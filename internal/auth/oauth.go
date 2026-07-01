package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/oauth2"
)

// VerifiedToken wraps an oauth2.Token and preserves the optional ID token.
type VerifiedToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	ExpiresIn    int
	Scope        string
	IDToken      string
}

func newVerifiedToken(tok *oauth2.Token) *VerifiedToken {
	if tok == nil {
		return nil
	}
	return &VerifiedToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		ExpiresIn:    tokenExpiresIn(tok, 0),
		Scope:        tokenExtraString(tok, "scope"),
		IDToken:      tokenExtraString(tok, "id_token"),
	}
}

func tokenExtraString(tok *oauth2.Token, key string) string {
	if tok == nil {
		return ""
	}
	if s, ok := tok.Extra(key).(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// ValidateIDToken checks the ID token's issuer and audience claims.
// It does not verify the JWT signature; callers should fetch the issuer's
// JWKS and verify the signature when required.
// Empty issuer or audience is treated as an error so checks are never
// silently skipped.
func (t *VerifiedToken) ValidateIDToken(issuer, audience string) error {
	if t == nil || t.IDToken == "" {
		return nil
	}
	if issuer == "" {
		return errors.New("id_token issuer required")
	}
	if audience == "" {
		return errors.New("id_token audience required")
	}
	parsed, err := jwt.ParseSigned(t.IDToken, []jose.SignatureAlgorithm{jose.RS256, jose.ES256, jose.EdDSA})
	if err != nil {
		return fmt.Errorf("invalid id_token: %w", err)
	}
	claims := map[string]any{}
	if err := parsed.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return fmt.Errorf("invalid id_token claims: %w", err)
	}
	if iss, _ := claims["iss"].(string); strings.TrimSpace(iss) != issuer {
		return fmt.Errorf("invalid issuer %q, expected %q", iss, issuer)
	}
	if !audienceMatches(claims["aud"], audience) {
		return fmt.Errorf("invalid audience, expected %q", audience)
	}
	if exp, ok := expiresAtFromClaim(claims["exp"]); ok && !exp.After(time.Now()) {
		return errors.New("id_token expired")
	}
	return nil
}

func audienceMatches(audClaim any, expected string) bool {
	if s, ok := audClaim.(string); ok {
		return strings.TrimSpace(s) == expected
	}
	if list, ok := audClaim.([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok && strings.TrimSpace(s) == expected {
				return true
			}
		}
	}
	return false
}

func expiresAtFromClaim(expClaim any) (time.Time, bool) {
	switch v := expClaim.(type) {
	case float64:
		return time.Unix(int64(v), 0), true
	case int:
		return time.Unix(int64(v), 0), true
	case int64:
		return time.Unix(v, 0), true
	case string:
		if n, err := parseIntString(v); err == nil {
			return time.Unix(n, 0), true
		}
	}
	return time.Time{}, false
}

func parseIntString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	return strconv.ParseInt(s, 10, 64)
}

// scopeIncludes reports whether a space-delimited OAuth scope contains value.
func scopeIncludes(scope, value string) bool {
	for _, s := range strings.Fields(scope) {
		if s == value {
			return true
		}
	}
	return false
}

// requireIDTokenForOpenID returns an error when the openid scope was requested
// but the token response does not contain an ID token.
func requireIDTokenForOpenID(scope, idToken string) error {
	if scopeIncludes(scope, "openid") && strings.TrimSpace(idToken) == "" {
		return errors.New("openid scope requested but token response missing id_token")
	}
	return nil
}

func verifiedTokenToCredential(clientID, resource string, vt *VerifiedToken, fallbackRefresh, fallbackScope string, now time.Time) (*config.Credential, error) {
	if vt == nil {
		return nil, errors.New("token response is nil")
	}
	accessToken := strings.TrimSpace(vt.AccessToken)
	if accessToken == "" {
		return nil, errors.New("token response missing access_token")
	}
	expiresIn := vt.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	refreshToken := strings.TrimSpace(vt.RefreshToken)
	if refreshToken == "" {
		refreshToken = fallbackRefresh
	}
	scope := strings.TrimSpace(vt.Scope)
	if scope == "" {
		scope = fallbackScope
	}
	return &config.Credential{
		ClientID:     clientID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    strings.TrimSpace(vt.TokenType),
		ExpiresAt:    float64(now.Add(time.Duration(expiresIn) * time.Second).Unix()),
		Scope:        scope,
		Resource:     resource,
	}, nil
}

func oauth2Context(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}

func stringFromMap(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func tokenExpiresIn(tok *oauth2.Token, fallback int) int {
	if tok == nil {
		return fallback
	}
	if extra := tok.Extra("expires_in"); extra != nil {
		switch v := extra.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			if v > 0 {
				return int(v)
			}
		case string:
			if n, err := parseIntString(v); err == nil && n > 0 {
				return int(n)
			}
		}
	}
	if !tok.Expiry.IsZero() {
		secs := int(time.Until(tok.Expiry).Seconds())
		if secs > 0 {
			return secs
		}
	}
	return fallback
}
