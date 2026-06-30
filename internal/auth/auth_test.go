package auth

import (
	"testing"
	"time"
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
