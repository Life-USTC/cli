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
)

const oauthScope = "openid profile email offline_access"

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

	for _, path := range []string{
		"/.well-known/oauth-authorization-server",
		"/.well-known/openid-configuration",
	} {
		resp, err := client.Get(strings.TrimRight(server, "/") + path)
		if err != nil {
			continue
		}
		if resp.StatusCode == 200 {
			var meta map[string]any
			decodeErr := json.NewDecoder(resp.Body).Decode(&meta)
			_ = resp.Body.Close()
			if decodeErr != nil {
				continue
			}
			return meta, nil
		}
		_ = resp.Body.Close()
	}
	return nil, fmt.Errorf("could not discover OAuth metadata from %s", server)
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

	authEndpoint, _ := meta["authorization_endpoint"].(string)
	tokenEndpoint, _ := meta["token_endpoint"].(string)
	regEndpoint, _ := meta["registration_endpoint"].(string)
	if regEndpoint == "" {
		return nil, fmt.Errorf("server does not advertise a registration_endpoint")
	}

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

	// Build auth URL
	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {oauthScope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"resource":              {server},
	}
	authURL := authEndpoint + "?" + params.Encode()

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

	// Exchange code for tokens
	tokenData := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {result.code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
		"resource":      {server},
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	tokenResp, err := httpClient.PostForm(tokenEndpoint, tokenData)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tokenResp.Body.Close() }()

	tokenBody, _ := io.ReadAll(tokenResp.Body)
	if tokenResp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (%d): %s", tokenResp.StatusCode, string(tokenBody))
	}

	var tokens map[string]any
	if err := json.Unmarshal(tokenBody, &tokens); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return credentialFromTokens(clientID, server, tokens, "", "")
}

// RefreshToken attempts to refresh the access token.
func RefreshToken(server string, cred *config.Credential) (*config.Credential, error) {
	if cred.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}
	meta, err := discoverOAuthMetadata(server)
	if err != nil {
		return nil, err
	}
	tokenEndpoint, _ := meta["token_endpoint"].(string)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {cred.ClientID},
		"refresh_token": {cred.RefreshToken},
	}
	if cred.Resource != "" {
		data.Set("resource", cred.Resource)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(tokenEndpoint, data)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(b))
	}

	var tokens map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return credentialFromTokens(cred.ClientID, cred.Resource, tokens, cred.RefreshToken, cred.Scope)
}

func requiredString(values map[string]any, key string) (string, error) {
	if s, ok := values[key].(string); ok && s != "" {
		return s, nil
	}
	return "", fmt.Errorf("token response missing %q", key)
}

func strDefault(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func credentialFromTokens(clientID, resource string, tokens map[string]any, fallbackRefresh, fallbackScope string) (*config.Credential, error) {
	accessToken, err := requiredString(tokens, "access_token")
	if err != nil {
		return nil, err
	}
	expiresIn := 3600.0
	if ei, ok := tokens["expires_in"].(float64); ok {
		expiresIn = ei
	}
	refreshToken := strDefault(tokens["refresh_token"])
	if refreshToken == "" {
		refreshToken = fallbackRefresh
	}
	scope := strDefault(tokens["scope"])
	if scope == "" {
		scope = fallbackScope
	}
	return &config.Credential{
		ClientID:     clientID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    strDefault(tokens["token_type"]),
		ExpiresAt:    float64(time.Now().Unix()) + expiresIn,
		Scope:        scope,
		Resource:     resource,
	}, nil
}
