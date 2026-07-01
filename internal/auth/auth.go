// Package auth implements OAuth2 Authorization Code + PKCE for CLI login.
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"golang.org/x/oauth2"
)

var oauthScope = strings.Join([]string{
	"openid",
	"profile",
	"email",
	"offline_access",
	"me:read",
	"me:write",
	"todo:read",
	"todo:write",
	"homework:read",
	"homework:write",
	"subscription:read",
	"subscription:write",
	"comment:read",
	"comment:write",
	"description:read",
	"description:write",
	"upload:read",
	"upload:write",
	"dashboard:read",
	"dashboard:write",
	"bus:read",
	"bus:write",
	"course:read",
	"course:write",
	"section:read",
	"section:write",
	"teacher:read",
	"teacher:write",
	"schedule:read",
	"schedule:write",
	"exam:read",
	"exam:write",
}, " ")

func b64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to read secure random bytes: %w", err)
	}
	verifier = b64url(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = b64url(h[:])
	return verifier, challenge, nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read secure random bytes: %w", err)
	}
	return b64url(b), nil
}

func discoverOAuthMetadata(server string) (map[string]any, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	var lastErr error
	for _, path := range []string{
		"/.well-known/oauth-authorization-server/api/auth",
		"/api/auth/.well-known/openid-configuration",
		"/.well-known/oauth-authorization-server",
		"/.well-known/openid-configuration",
	} {
		resp, err := client.Get(strings.TrimRight(server, "/") + path)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", path, err)
			continue
		}
		if resp.StatusCode == 200 {
			var meta map[string]any
			decodeErr := json.NewDecoder(resp.Body).Decode(&meta)
			_ = resp.Body.Close()
			if decodeErr != nil {
				lastErr = fmt.Errorf("%s: decode metadata: %w", path, decodeErr)
				continue
			}
			return meta, nil
		}
		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
		_ = resp.Body.Close()
	}
	if lastErr != nil {
		return nil, fmt.Errorf("could not discover OAuth metadata from %s: %w", server, lastErr)
	}
	return nil, fmt.Errorf("could not discover OAuth metadata from %s", server)
}

func oauthResource(server string, meta map[string]any) string {
	if issuer := strings.TrimRight(stringFromMap(meta, "issuer"), "/"); issuer != "" {
		return issuer
	}
	return strings.TrimRight(server, "/")
}

func registerPublicClient(endpoint string, redirectURIs, grantTypes, responseTypes []string) (map[string]any, error) {
	body := map[string]any{
		"client_name":                "life-ustc-cli",
		"redirect_uris":              redirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                grantTypes,
		"response_types":             responseTypes,
		"scope":                      oauthScope,
	}
	data, _ := json.Marshal(body)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client registration failed (%d): %s", resp.StatusCode, string(b))
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode client registration response: %w", err)
	}
	return result, nil
}

func registerClient(endpoint, redirectURI string) (map[string]any, error) {
	return registerPublicClient(
		endpoint,
		[]string{redirectURI},
		[]string{"authorization_code", "refresh_token"},
		[]string{"code"},
	)
}

func refreshTokenRequest(ctx context.Context, client *http.Client, endpoint, clientID, refreshToken, scope, resource string) (*oauth2.Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	if scope = strings.TrimSpace(scope); scope != "" {
		form.Set("scope", scope)
	}
	if resource = strings.TrimSpace(resource); resource != "" {
		form.Set("resource", resource)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("token refresh returned %d with invalid JSON", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if desc, _ := raw["error_description"].(string); strings.TrimSpace(desc) != "" {
			return nil, fmt.Errorf("%s", strings.TrimSpace(desc))
		}
		if code, _ := raw["error"].(string); strings.TrimSpace(code) != "" {
			return nil, fmt.Errorf("%s", strings.TrimSpace(code))
		}
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	accessToken, _ := raw["access_token"].(string)
	tokenType, _ := raw["token_type"].(string)
	nextRefreshToken, _ := raw["refresh_token"].(string)
	expiresIn := tokenExpiresInFromRaw(raw)
	tok := &oauth2.Token{
		AccessToken:  strings.TrimSpace(accessToken),
		TokenType:    strings.TrimSpace(tokenType),
		RefreshToken: strings.TrimSpace(nextRefreshToken),
		ExpiresIn:    int64(expiresIn),
	}
	if expiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	return tok.WithExtra(raw), nil
}

func tokenExpiresInFromRaw(raw map[string]any) int {
	switch v := raw["expires_in"].(type) {
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
	return 0
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// Login runs the full OAuth2 Authorization Code + PKCE flow.
// Returns a credential to store.
func Login(server string) (*config.Credential, error) {
	server = strings.TrimRight(server, "/")
	fmt.Printf("Logging in to %s ...\n", server)

	meta, err := discoverOAuthMetadata(server)
	if err != nil {
		return nil, err
	}

	authEndpoint := stringFromMap(meta, "authorization_endpoint")
	tokenEndpoint := stringFromMap(meta, "token_endpoint")
	regEndpoint := stringFromMap(meta, "registration_endpoint")
	if regEndpoint == "" {
		return nil, fmt.Errorf("server does not advertise a registration_endpoint")
	}
	resource := oauthResource(server, meta)

	// Start local callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	// Register client
	clientInfo, err := registerClient(regEndpoint, redirectURI)
	if err != nil {
		return nil, err
	}
	clientID, _ := clientInfo["client_id"].(string)

	// PKCE
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	conf := &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURI,
		Scopes:      strings.Fields(oauthScope),
		Endpoint: oauth2.Endpoint{
			AuthURL:  authEndpoint,
			TokenURL: tokenEndpoint,
		},
	}

	ctx := oauth2Context(context.Background(), &http.Client{Timeout: 15 * time.Second})

	authURL := conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	// Channel for callback result
	type callbackResult struct {
		code  string
		state string
		err   string
	}
	ch := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			ch <- callbackResult{err: e}
			_, _ = w.Write([]byte("<html><body><h2>Authentication failed</h2><p>You can close this tab.</p></body></html>"))
			return
		}
		ch <- callbackResult{code: q.Get("code"), state: q.Get("state")}
		_, _ = w.Write([]byte("<html><body><h2>Authentication successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>"))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Open browser
	fmt.Println()
	if err := openBrowser(authURL); err != nil {
		fmt.Println("Could not open browser automatically.")
	}
	fmt.Println("If the browser didn't open, visit this URL:")
	fmt.Printf("  %s\n\n", authURL)
	fmt.Println("Waiting for authentication...")

	// Wait for callback (5 min timeout)
	var result callbackResult
	select {
	case result = <-ch:
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}

	if result.err != "" {
		return nil, fmt.Errorf("authentication failed: %s", result.err)
	}
	if result.state != state {
		return nil, fmt.Errorf("state mismatch — possible CSRF attack")
	}

	tok, err := conf.Exchange(ctx, result.code,
		oauth2.VerifierOption(verifier),
		oauth2.SetAuthURLParam("resource", resource),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	vt := newVerifiedToken(tok)
	if err := requireIDTokenForOpenID(oauthScope, vt.IDToken); err != nil {
		return nil, err
	}
	issuer := stringFromMap(meta, "issuer")
	if issuer == "" {
		issuer = server
	}
	if err := vt.ValidateIDToken(issuer, clientID); err != nil {
		return nil, err
	}
	return verifiedTokenToCredential(clientID, resource, vt, "", oauthScope, time.Now())
}

// RefreshToken attempts to refresh the access token.
func RefreshToken(server string, cred *config.Credential) (*config.Credential, error) {
	server = strings.TrimRight(server, "/")
	if cred.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}
	meta, err := discoverOAuthMetadata(server)
	if err != nil {
		return nil, err
	}
	tokenEndpoint := stringFromMap(meta, "token_endpoint")
	resource := strings.TrimSpace(cred.Resource)
	if resource == "" {
		resource = oauthResource(server, meta)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	tok, err := refreshTokenRequest(context.Background(), client, tokenEndpoint, cred.ClientID, cred.RefreshToken, cred.Scope, resource)
	if err != nil {
		return nil, fmt.Errorf("refresh failed: %w", err)
	}

	vt := newVerifiedToken(tok)
	if err := requireIDTokenForOpenID(oauthScope, vt.IDToken); err != nil {
		return nil, err
	}
	issuer := stringFromMap(meta, "issuer")
	if issuer == "" {
		issuer = server
	}
	if err := vt.ValidateIDToken(issuer, cred.ClientID); err != nil {
		return nil, err
	}
	return verifiedTokenToCredential(cred.ClientID, resource, vt, cred.RefreshToken, cred.Scope, time.Now())
}
