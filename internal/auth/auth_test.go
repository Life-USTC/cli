package auth

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"golang.org/x/oauth2"
)

func TestRegisterPublicClientUsesNativeApplicationType(t *testing.T) {
	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"client_id":"client-1"}`))
	}))
	t.Cleanup(server.Close)

	_, err := registerPublicClient(
		server.URL,
		[]string{"offline_access", "workspace.todo:read"},
		[]string{"http://127.0.0.1:46289/callback"},
		[]string{"authorization_code", "refresh_token"},
		[]string{"code"},
	)
	if err != nil {
		t.Fatal(err)
	}
	body := <-requests
	if body["application_type"] != "native" {
		t.Fatalf("application_type = %#v, want native", body["application_type"])
	}
	if body["scope"] != "offline_access workspace.todo:read" {
		t.Fatalf("scope = %#v", body["scope"])
	}
	redirectURIs, ok := body["redirect_uris"].([]any)
	if !ok || len(redirectURIs) != 1 || redirectURIs[0] != "http://127.0.0.1:46289/callback" {
		t.Fatalf("redirect_uris = %#v", body["redirect_uris"])
	}
}

func TestRegisterPublicClientOmitsUnusedDeviceRedirectMetadata(t *testing.T) {
	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"client_id":"device-client"}`))
	}))
	t.Cleanup(server.Close)

	_, err := registerPublicClient(
		server.URL,
		[]string{"offline_access", "workspace.todo:read"},
		nil,
		[]string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	body := <-requests
	if body["application_type"] != "native" {
		t.Fatalf("application_type = %#v, want native", body["application_type"])
	}
	if body["scope"] != "offline_access workspace.todo:read" {
		t.Fatalf("scope = %#v", body["scope"])
	}
	if _, ok := body["redirect_uris"]; ok {
		t.Fatalf("redirect_uris should be omitted, got %#v", body["redirect_uris"])
	}
	if _, ok := body["response_types"]; ok {
		t.Fatalf("response_types should be omitted, got %#v", body["response_types"])
	}
}

func TestOAuthScopesFromMetadata(t *testing.T) {
	advertised := make([]any, 0, len(cliOAuthScopes)+3)
	advertised = append(advertised,
		"openid",
		"profile",
		"admin:write",
	)
	for _, scope := range cliOAuthScopes {
		advertised = append(advertised, scope)
	}
	scopes, err := oauthScopesFromMetadata(map[string]any{
		"scopes_supported": advertised,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(scopes, " ") != strings.Join(cliOAuthScopes, " ") {
		t.Fatalf("scopes = %#v, want %#v", scopes, cliOAuthScopes)
	}
	granted := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		granted[scope] = true
	}
	for _, required := range []string{
		"account.client-activity:read",
		"workspace.subscription:write",
		"workspace.calendar:read",
		"workspace.young-subscription:read",
		"workspace.young-subscription:write",
		"workspace.young-notification:read",
		"workspace.young-notification:write",
	} {
		if !granted[required] {
			t.Fatalf("command scope missing: %q", required)
		}
	}
	for _, forbidden := range []string{"admin:write", "openid", "profile"} {
		if granted[forbidden] {
			t.Fatalf("scopes unexpectedly include %q: %#v", forbidden, scopes)
		}
	}
}

func TestOAuthScopesFromMetadataRequiresCommandScopes(t *testing.T) {
	for _, missing := range []string{"email", "workspace.calendar-feed:read", "workspace.exam:read", "account.client-activity:read"} {
		t.Run(missing, func(t *testing.T) {
			advertised := make([]any, 0, len(cliOAuthScopes)-1)
			for _, scope := range cliOAuthScopes {
				if scope != missing {
					advertised = append(advertised, scope)
				}
			}
			_, err := oauthScopesFromMetadata(map[string]any{"scopes_supported": advertised})
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("error = %v, want missing required scope %q", err, missing)
			}
		})
	}
}

func TestOAuthScopesFromMetadataRejectsMissingOrInvalidScopes(t *testing.T) {
	tests := []struct {
		name string
		meta map[string]any
	}{
		{name: "missing", meta: map[string]any{}},
		{name: "wrong type", meta: map[string]any{"scopes_supported": "openid profile"}},
		{name: "empty", meta: map[string]any{"scopes_supported": []any{}}},
		{name: "invalid item", meta: map[string]any{"scopes_supported": []any{"offline_access", 42}}},
		{name: "blank item", meta: map[string]any{"scopes_supported": []any{"offline_access", " "}}},
		{name: "missing required", meta: map[string]any{"scopes_supported": []any{"openid", "profile", "email"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := oauthScopesFromMetadata(tt.meta); err == nil {
				t.Fatal("expected invalid metadata error")
			}
		})
	}
}

func TestLoginDeviceCodeAcceptsOAuthTokenWithoutIDToken(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	registrationScopes := make(chan string, 1)
	deviceScopes := make(chan string, 1)
	tokenScopes := make(chan string, 1)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                        server.URL + "/api/auth",
				"registration_endpoint":         server.URL + "/api/auth/oauth2/register",
				"device_authorization_endpoint": server.URL + "/api/auth/oauth2/device-authorization",
				"token_endpoint":                server.URL + "/api/auth/oauth2/token",
				"scopes_supported":              append([]string{"openid", "profile", "account.client-activity:read"}, cliOAuthScopes...),
			})
		case "/api/auth/oauth2/register":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			registrationScope, _ := body["scope"].(string)
			registrationScopes <- registrationScope
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"client_id":"device-client"}`))
		case "/api/auth/oauth2/device-authorization":
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			deviceScopes <- r.Form.Get("scope")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "device-code",
				"user_code":                 "TEST-CODE",
				"verification_uri":          server.URL + "/oauth/device",
				"verification_uri_complete": server.URL + "/oauth/device?code=TEST-CODE",
				"expires_in":                60,
				"interval":                  1,
			})
		case "/api/auth/oauth2/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			tokenScopes <- r.Form.Get("scope")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "device-access",
				"refresh_token": "device-refresh",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"scope":         strings.Join(cliOAuthScopes, " "),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cred, err := LoginDeviceCode(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "device-access" || cred.RefreshToken != "device-refresh" {
		t.Fatalf("credential = %#v", cred)
	}
	wantScope := strings.Join(cliOAuthScopes, " ")
	if got := <-registrationScopes; got != wantScope {
		t.Fatalf("registration scope = %q, want %q", got, wantScope)
	}
	if got := <-deviceScopes; got != wantScope {
		t.Fatalf("device authorization scope = %q, want %q", got, wantScope)
	}
	if got := <-tokenScopes; got != wantScope {
		t.Fatalf("token scope = %q, want %q", got, wantScope)
	}
	if cred.Scope != wantScope {
		t.Fatalf("credential scope = %q, want %q", cred.Scope, wantScope)
	}
}

func TestBrowserAuthorizationURLForcesFreshLogin(t *testing.T) {
	conf := &oauth2.Config{
		ClientID: "client-1",
		Endpoint: oauth2.Endpoint{AuthURL: "https://example.test/oauth/authorize"},
	}
	authURL, err := url.Parse(browserAuthorizationURL(conf, "state-1", "challenge-1"))
	if err != nil {
		t.Fatal(err)
	}
	query := authURL.Query()
	if query.Get("prompt") != "login" {
		t.Fatalf("prompt = %q, want login", query.Get("prompt"))
	}
	if query.Get("code_challenge") != "challenge-1" || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE query = %q", authURL.RawQuery)
	}
}

func TestOAuthCallbackHandlerDeliversOnlyFirstRequest(t *testing.T) {
	results := make(chan callbackResult, 1)
	handler := oauthCallbackHandler(results)

	first := httptest.NewRecorder()
	handler(first, httptest.NewRequest(http.MethodGet, "/callback?code=first&state=state-1", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}
	result := <-results
	if result.code != "first" || result.state != "state-1" || result.err != "" {
		t.Fatalf("result = %#v", result)
	}

	repeated := httptest.NewRecorder()
	handler(repeated, httptest.NewRequest(http.MethodGet, "/callback?code=second&state=state-2", nil))
	if repeated.Code != http.StatusConflict {
		t.Fatalf("repeated status = %d, want %d", repeated.Code, http.StatusConflict)
	}
	select {
	case extra := <-results:
		t.Fatalf("unexpected repeated result: %#v", extra)
	default:
	}
}

func TestOAuthCallbackHandlerDoesNotBlockWhenResultBufferIsFull(t *testing.T) {
	results := make(chan callbackResult, 1)
	results <- callbackResult{code: "occupied"}
	handler := oauthCallbackHandler(results)
	done := make(chan struct{})
	response := httptest.NewRecorder()
	go func() {
		handler(response, httptest.NewRequest(http.MethodGet, "/callback?error=denied", nil))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callback handler blocked on a full result channel")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestCallbackRedirectURIMatchesLoopbackListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	redirect, err := url.Parse(callbackRedirectURI(listener.Addr()))
	if err != nil {
		t.Fatal(err)
	}
	if redirect.Hostname() != "127.0.0.1" {
		t.Fatalf("redirect hostname = %q, want 127.0.0.1", redirect.Hostname())
	}
	if redirect.Port() != strconv.Itoa(listener.Addr().(*net.TCPAddr).Port) {
		t.Fatalf("redirect port = %q, listener = %q", redirect.Port(), listener.Addr())
	}
	if redirect.Path != "/callback" {
		t.Fatalf("redirect path = %q, want /callback", redirect.Path)
	}
}

func TestOAuthTokenToCredentialUsesFallbacks(t *testing.T) {
	token := &oauthToken{
		AccessToken: "access",
		ExpiresIn:   120,
	}
	cred, err := oauthTokenToCredential("client", "https://example.test", token, "refresh", "openid", time.Now())
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

func TestOAuthTokenToCredentialRequiresAccessToken(t *testing.T) {
	token := &oauthToken{}
	if _, err := oauthTokenToCredential("client", "resource", token, "", "", time.Now()); err == nil {
		t.Fatal("expected missing access token error")
	}
}

func TestOAuthTokenToCredentialNilGuard(t *testing.T) {
	if _, err := oauthTokenToCredential("client", "resource", nil, "", "", time.Now()); err == nil {
		t.Fatal("expected error for nil token")
	}
}

func TestEffectiveTokenScopePrefersGrantedScope(t *testing.T) {
	token := &oauthToken{Scope: "profile workspace.todo:read"}
	if got := effectiveTokenScope(token, "openid profile workspace.todo:read"); got != "profile workspace.todo:read" {
		t.Fatalf("effective scope = %q", got)
	}
}

func TestOAuthExpiresInSecondsRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
		ok    bool
	}{
		{name: "int", value: 3600, want: 3600, ok: true},
		{name: "int64", value: int64(3600), want: 3600, ok: true},
		{name: "float", value: float64(3600), want: 3600, ok: true},
		{name: "string", value: " 3600 ", want: 3600, ok: true},
		{name: "zero", value: 0},
		{name: "negative", value: int64(-1)},
		{name: "fractional", value: 1.5},
		{name: "duration overflow", value: int64(math.MaxInt64/int64(time.Second) + 1)},
		{name: "integer overflow string", value: "9223372036854775808"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := oauthExpiresInSeconds(tt.value)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("oauthExpiresInSeconds(%v) = (%d, %t), want (%d, %t)", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTokenExpiresInUsesFallbackForUnsafeExtra(t *testing.T) {
	token := (&oauth2.Token{}).WithExtra(map[string]any{
		"expires_in": "9223372036854775808",
	})
	if got := tokenExpiresIn(token, 900); got != 900 {
		t.Fatalf("tokenExpiresIn() = %d, want fallback 900", got)
	}
}

func TestRefreshTokenDoesNotRequireNewIDToken(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":         server.URL + "/api/auth",
				"token_endpoint": server.URL + "/api/auth/oauth2/token",
			})
		case "/api/auth/oauth2/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.Form.Get("grant_type") != "refresh_token" {
				http.Error(w, "unexpected grant", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "next-access",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cred, err := RefreshToken(server.URL, &config.Credential{
		ClientID:     "client-1",
		RefreshToken: "refresh-1",
		Scope:        "offline_access workspace.todo:read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "next-access" || cred.RefreshToken != "refresh-1" {
		t.Fatalf("credential = %#v", cred)
	}
	if cred.Scope != "offline_access workspace.todo:read" {
		t.Fatalf("scope = %q", cred.Scope)
	}
}

func TestOAuthAccessTokenDoesNotRequireIDToken(t *testing.T) {
	token := (&oauth2.Token{
		AccessToken:  "device-access",
		RefreshToken: "device-refresh",
		TokenType:    "Bearer",
	}).WithExtra(map[string]any{
		"expires_in": 3600,
		"scope":      "openid profile workspace.todo:read",
	})
	cred, err := oauthTokenToCredential(
		"device-client",
		"https://life.example/api/auth",
		newOAuthToken(token),
		"",
		"",
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "device-access" || cred.RefreshToken != "device-refresh" {
		t.Fatalf("credential = %#v", cred)
	}
	if cred.Scope != "openid profile workspace.todo:read" {
		t.Fatalf("scope = %q", cred.Scope)
	}
}
