package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestVerifiedTokenToCredentialUsesFallbacks(t *testing.T) {
	vt := &VerifiedToken{
		AccessToken: "access",
		ExpiresIn:   120,
	}
	cred, err := verifiedTokenToCredential("client", "https://example.test", vt, "refresh", "openid", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if cred.ClientID != "client" || cred.AccessToken != "access" || cred.RefreshToken != "refresh" || cred.Scope != "openid" {
		t.Fatalf("credential = %#v", cred)
	}
	if cred.ExpiresAt == 0 {
		t.Fatal("ExpiresAt was not populated")
	}
	if cred.Resource != "https://example.test" {
		t.Fatalf("resource = %q, want %q", cred.Resource, "https://example.test")
	}
}

func TestVerifiedTokenToCredentialRequiresAccessToken(t *testing.T) {
	vt := &VerifiedToken{}
	if _, err := verifiedTokenToCredential("client", "resource", vt, "", "", time.Now()); err == nil {
		t.Fatal("expected missing access token error")
	}
}

func TestVerifiedTokenToCredentialNilGuard(t *testing.T) {
	if _, err := verifiedTokenToCredential("client", "resource", nil, "", "", time.Now()); err == nil {
		t.Fatal("expected error for nil token")
	}
}

func TestRequireIDTokenForOpenID(t *testing.T) {
	if err := requireIDTokenForOpenID("openid profile", ""); err == nil {
		t.Fatal("expected error when openid scope requested without id_token")
	}
	if err := requireIDTokenForOpenID("profile email", ""); err != nil {
		t.Fatalf("unexpected error when openid not requested: %v", err)
	}
	if err := requireIDTokenForOpenID("openid", "idtoken"); err != nil {
		t.Fatalf("unexpected error when id_token present: %v", err)
	}
}

func TestValidateIDTokenAudienceIsClientID(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	build := func(aud any) string {
		t.Helper()
		claims := map[string]any{
			"iss": "https://issuer.test",
			"aud": aud,
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		s, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	vt := &VerifiedToken{IDToken: build("client-id-123")}
	if err := vt.ValidateIDToken("https://issuer.test", "client-id-123"); err != nil {
		t.Fatalf("expected client_id audience to validate: %v", err)
	}

	vt.IDToken = build("https://server.test")
	if err := vt.ValidateIDToken("https://issuer.test", "client-id-123"); err == nil {
		t.Fatal("expected server URL audience to fail against client_id expectation")
	}

	vt.IDToken = build([]string{"client-id-123", "other"})
	if err := vt.ValidateIDToken("https://issuer.test", "client-id-123"); err != nil {
		t.Fatalf("expected audience list containing client_id to validate: %v", err)
	}
}
