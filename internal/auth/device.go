package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"golang.org/x/oauth2"
)

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

	deviceEndpoint := stringFromMap(meta, "device_authorization_endpoint")
	tokenEndpoint := stringFromMap(meta, "token_endpoint")
	regEndpoint := stringFromMap(meta, "registration_endpoint")

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

	conf := &oauth2.Config{
		ClientID: clientID,
		Scopes:   strings.Fields(oauthScope),
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: deviceEndpoint,
			TokenURL:      tokenEndpoint,
		},
	}

	ctx := oauth2Context(context.Background(), &http.Client{Timeout: 15 * time.Second})
	res, err := conf.DeviceAuth(ctx, oauth2.SetAuthURLParam("resource", server))
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}

	// Display instructions to user
	fmt.Println()
	fmt.Println("To sign in, visit:")
	if res.VerificationURIComplete != "" {
		fmt.Printf("  %s\n", res.VerificationURIComplete)
	} else {
		fmt.Printf("  %s\n", res.VerificationURI)
	}
	fmt.Println()
	fmt.Printf("And enter the code: %s\n\n", res.UserCode)

	// Try to open browser
	if res.VerificationURIComplete != "" {
		_ = openBrowser(res.VerificationURIComplete)
	} else {
		_ = openBrowser(res.VerificationURI)
	}

	fmt.Println("Waiting for authorization...")

	tok, err := conf.DeviceAccessToken(ctx, res, oauth2.SetAuthURLParam("resource", server))
	if err != nil {
		return nil, fmt.Errorf("device authorization failed: %w", err)
	}

	vt := newVerifiedToken(tok)
	issuer := stringFromMap(meta, "issuer")
	if issuer == "" {
		issuer = server
	}
	if err := vt.ValidateIDToken(issuer, server); err != nil {
		return nil, err
	}
	return verifiedTokenToCredential(clientID, server, vt, "", "", time.Now())
}
