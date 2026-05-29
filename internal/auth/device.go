package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
)

// deviceAuthResponse matches the RFC 8628 device authorization response.
type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// LoginDeviceCode runs the RFC 8628 Device Authorization Grant flow.
// It prints a user code for the user to enter in a browser, then polls
// the token endpoint until approved, denied, or expired.
func LoginDeviceCode(server string) (*config.Credential, error) {
	server = strings.TrimRight(server, "/")
	fmt.Printf("Logging in to %s via device code ...\n", server)

	meta, err := discoverOAuthMetadata(server)
	if err != nil {
		return nil, err
	}

	deviceEndpoint, _ := meta["device_authorization_endpoint"].(string)
	tokenEndpoint, _ := meta["token_endpoint"].(string)
	regEndpoint, _ := meta["registration_endpoint"].(string)

	if deviceEndpoint == "" {
		return nil, fmt.Errorf("server does not support device authorization (no device_authorization_endpoint in metadata)")
	}
	if regEndpoint == "" {
		return nil, fmt.Errorf("server does not advertise a registration_endpoint")
	}

	// Register client
	clientInfo, err := registerPublicClient(
		regEndpoint,
		[]string{"http://localhost/callback"},
		[]string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		[]string{"code"},
	)
	if err != nil {
		return nil, err
	}
	clientID, _ := clientInfo["client_id"].(string)

	// Request device code
	httpClient := &http.Client{Timeout: 15 * time.Second}
	deviceResp, err := httpClient.PostForm(deviceEndpoint, url.Values{
		"client_id": {clientID},
		"scope":     {oauthScope},
	})
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}
	defer func() { _ = deviceResp.Body.Close() }()

	if deviceResp.StatusCode != 200 {
		b, _ := io.ReadAll(deviceResp.Body)
		return nil, fmt.Errorf("device authorization failed (%d): %s", deviceResp.StatusCode, string(b))
	}

	var devAuth deviceAuthResponse
	if err := json.NewDecoder(deviceResp.Body).Decode(&devAuth); err != nil {
		return nil, fmt.Errorf("failed to decode device authorization response: %w", err)
	}

	// Display instructions to user
	fmt.Println()
	fmt.Println("To sign in, visit:")
	if devAuth.VerificationURIComplete != "" {
		fmt.Printf("  %s\n", devAuth.VerificationURIComplete)
	} else {
		fmt.Printf("  %s\n", devAuth.VerificationURI)
	}
	fmt.Println()
	fmt.Printf("And enter the code: %s\n\n", devAuth.UserCode)

	// Try to open browser
	if devAuth.VerificationURIComplete != "" {
		_ = openBrowser(devAuth.VerificationURIComplete)
	} else {
		_ = openBrowser(devAuth.VerificationURI)
	}

	fmt.Println("Waiting for authorization...")

	// Poll token endpoint
	interval := time.Duration(devAuth.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(devAuth.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		tokenData := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {clientID},
			"device_code": {devAuth.DeviceCode},
		}

		tokenResp, err := httpClient.PostForm(tokenEndpoint, tokenData)
		if err != nil {
			return nil, fmt.Errorf("token poll failed: %w", err)
		}

		tokenBody, _ := io.ReadAll(tokenResp.Body)
		_ = tokenResp.Body.Close()

		if tokenResp.StatusCode == 200 {
			var tokens map[string]any
			if err := json.Unmarshal(tokenBody, &tokens); err != nil {
				return nil, fmt.Errorf("failed to decode token response: %w", err)
			}

			return credentialFromTokens(clientID, server, tokens, "", "")
		}

		// Parse error response
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(tokenBody, &errResp)

		switch errResp.Error {
		case "authorization_pending":
			// keep polling
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired — please try again")
		case "access_denied":
			return nil, fmt.Errorf("authorization denied by user")
		default:
			return nil, fmt.Errorf("token request failed: %s (HTTP %d)", string(tokenBody), tokenResp.StatusCode)
		}
	}

	return nil, fmt.Errorf("device code expired — please try again")
}
