// Package api provides the HTTP client for the Life@USTC API.
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Life-USTC/CLI/internal/auth"
	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
)

// Client wraps net/http with automatic auth header injection and token refresh.
type Client struct {
	Server string

	HTTPClient *http.Client
}

type refreshTokenFunc func(server string, cred *config.Credential) (*config.Credential, error)

// authTransport implements http.RoundTripper.
// It injects the Bearer token and retries once on 401 after a token refresh.
type authTransport struct {
	mu      sync.Mutex
	server  string
	cred    *config.Credential
	base    http.RoundTripper
	refresh refreshTokenFunc
}

// NewClient creates a client, optionally requiring auth.
func NewClient(server string, requireAuth bool) (*Client, error) {
	return NewClientWithRefresh(server, requireAuth, nil)
}

// NewClientWithRefresh creates a Client with a custom token refresh function.
// Pass nil for refresh to use the default auth.RefreshToken; this is intended
// for tests that want to inject a mock refresh without hitting the real server.
func NewClientWithRefresh(server string, requireAuth bool, refresh refreshTokenFunc) (*Client, error) {
	server = strings.TrimRight(server, "/")
	cred, err := config.LoadCredentials(server)
	if err != nil {
		return nil, err
	}
	if requireAuth && cred == nil {
		return nil, fmt.Errorf("not logged in. Run `life-ustc auth login` first")
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			server:  server,
			cred:    cred,
			base:    http.DefaultTransport,
			refresh: refresh,
		},
	}
	return &Client{Server: server, HTTPClient: httpClient}, nil
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	refreshed := t.ensureToken()

	t.mu.Lock()
	cred := t.cred
	t.mu.Unlock()

	if cred != nil {
		req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	}

	output.VerboseF("→ %s %s", req.Method, req.URL)
	start := time.Now()

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		output.VerboseF("← error: %s (%dms)", err, time.Since(start).Milliseconds())
		return nil, err
	}

	output.VerboseF("← %d %s (%dms)", resp.StatusCode, http.StatusText(resp.StatusCode), time.Since(start).Milliseconds())

	if resp.StatusCode == 401 && cred != nil {
		_ = resp.Body.Close()
		if refreshed {
			return nil, fmt.Errorf("session expired. Please run `life-ustc auth login` again")
		}
		t.mu.Lock()
		newCred, refreshErr := t.refreshToken()
		if refreshErr == nil && newCred != nil {
			t.cred = newCred
			_ = config.SaveCredentials(t.server, newCred)
		}
		t.mu.Unlock()
		if refreshErr != nil || newCred == nil {
			return nil, fmt.Errorf("session expired while refreshing token. Please run `life-ustc auth login` again")
		}

		// Clone the request with a fresh body and the new token.
		req2 := req.Clone(req.Context())
		if req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, bodyErr
			}
			req2.Body = body
		} else if req.Body != nil {
			return nil, fmt.Errorf("session expired while sending a non-replayable request body; please retry")
		}
		req2.Header.Set("Authorization", "Bearer "+t.cred.AccessToken)
		resp, err = t.base.RoundTrip(req2)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 401 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("session expired. Please run `life-ustc auth login` again")
		}
	}

	return resp, nil
}

func (t *authTransport) ensureToken() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cred == nil || !config.IsTokenExpired(t.cred) {
		return false
	}
	newCred, err := t.refreshToken()
	if err != nil || newCred == nil {
		return false
	}
	t.cred = newCred
	_ = config.SaveCredentials(t.server, newCred)
	return true
}

func (t *authTransport) refreshToken() (*config.Credential, error) {
	refresh := t.refresh
	if refresh == nil {
		refresh = auth.RefreshToken
	}
	return refresh(t.server, t.cred)
}

// DoRaw performs an HTTP request and returns the raw response.
func (c *Client) DoRaw(ctx context.Context, method, path string, params url.Values, body io.Reader, contentType string, headers http.Header) (*http.Response, error) {
	u := c.Server + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header[k] = append([]string(nil), v...)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.HTTPClient.Do(req)
}
