package auth

import "testing"

func TestCredentialFromTokensUsesFallbacks(t *testing.T) {
	cred, err := credentialFromTokens("client", "https://example.test", map[string]any{
		"access_token": "access",
		"expires_in":   float64(120),
	}, "refresh", "openid")
	if err != nil {
		t.Fatal(err)
	}
	if cred.ClientID != "client" || cred.AccessToken != "access" || cred.RefreshToken != "refresh" || cred.Scope != "openid" {
		t.Fatalf("credential = %#v", cred)
	}
	if cred.ExpiresAt == 0 {
		t.Fatal("ExpiresAt was not populated")
	}
}

func TestCredentialFromTokensRequiresAccessToken(t *testing.T) {
	if _, err := credentialFromTokens("client", "resource", map[string]any{}, "", ""); err == nil {
		t.Fatal("expected missing access token error")
	}
}
