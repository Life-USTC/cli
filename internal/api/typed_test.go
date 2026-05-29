package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthTransportRetriesWithFreshBody(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())

	calls := 0
	transport := &authTransport{
		server: "https://example.test",
		cred: &config.Credential{
			ClientID:     "client",
			AccessToken:  "old-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    float64(time.Now().Add(time.Hour).Unix()),
		},
		refresh: func(server string, cred *config.Credential) (*config.Credential, error) {
			return &config.Credential{
				ClientID:     cred.ClientID,
				AccessToken:  "new-token",
				RefreshToken: cred.RefreshToken,
				ExpiresAt:    float64(time.Now().Add(time.Hour).Unix()),
			}, nil
		},
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body on call %d: %v", calls, err)
			}
			_ = req.Body.Close()

			switch calls {
			case 1:
				if got := req.Header.Get("Authorization"); got != "Bearer old-token" {
					t.Fatalf("first Authorization = %q", got)
				}
				if string(body) != `{"title":"first"}` {
					t.Fatalf("first body = %q", body)
				}
				return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
			case 2:
				if got := req.Header.Get("Authorization"); got != "Bearer new-token" {
					t.Fatalf("retry Authorization = %q", got)
				}
				if string(body) != `{"title":"first"}` {
					t.Fatalf("retry body = %q", body)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
			default:
				t.Fatalf("unexpected call %d", calls)
				return nil, nil
			}
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.test/api/todos", strings.NewReader(`{"title":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestReadResponseFormatsJSONErrors(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/problem+json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"bad input","ignored":true}`)),
		Request: &http.Request{
			Method: http.MethodPost,
			URL:    &url.URL{Path: "/api/todos"},
		},
	}

	_, _, err := ReadResponse(resp, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "POST /api/todos → 400: bad input"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReadResponseFormatsEmptyErrors(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Path: "/api/missing"},
		},
	}

	_, _, err := ReadResponse(resp, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "GET /api/missing → 404"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestIsJSONContentType(t *testing.T) {
	for _, contentType := range []string{
		"application/json",
		"application/json; charset=utf-8",
		"application/problem+json",
		"APPLICATION/PROBLEM+JSON",
	} {
		if !IsJSONContentType(contentType) {
			t.Fatalf("%q should be JSON", contentType)
		}
	}
	for _, contentType := range []string{"text/json", "application/jsonp", ""} {
		if IsJSONContentType(contentType) {
			t.Fatalf("%q should not be JSON", contentType)
		}
	}
}

func TestDecodeResponseBody(t *testing.T) {
	got, err := DecodeResponseBody([]byte(`{"ok":true}`), "application/problem+json", false)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[string]any)
	if !ok || m["ok"] != true {
		t.Fatalf("decoded = %#v, want JSON map", got)
	}

	got, err = DecodeResponseBody([]byte("plain"), "text/plain", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain" {
		t.Fatalf("decoded = %#v, want plain", got)
	}
}

func TestDecodeResponseBodyMalformedJSONFallback(t *testing.T) {
	if _, err := DecodeResponseBody([]byte("{bad"), "application/json", false); err == nil {
		t.Fatal("expected strict JSON decode error")
	}

	got, err := DecodeResponseBody([]byte("{bad"), "application/json", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "{bad" {
		t.Fatalf("decoded = %#v, want fallback text", got)
	}
}
