// Package api provides the HTTP client for the Life@USTC API.
//
// typed.go bridges the generated OpenAPI client with the existing
// auth/token-refresh logic by injecting an authTransport into the
// standard http.Client used by the generated code.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	openapi "github.com/Life-USTC/CLI/internal/openapi"
)

// TypedClient wraps the generated OpenAPI client with auth support.
type TypedClient struct {
	*openapi.Client
}

// NewTypedClient creates a TypedClient with auth, using the generated OpenAPI client.
func NewTypedClient(server string, requireAuth bool) (*TypedClient, error) {
	client, err := NewClientWithRefresh(server, requireAuth, nil)
	if err != nil {
		return nil, err
	}
	oapiClient, err := openapi.NewClient(client.Server, openapi.WithHTTPClient(client.HTTPClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &TypedClient{Client: oapiClient}, nil
}

// ParseResponseRaw reads a response and returns it as map[string]any for
// backward compatibility with the output module.
func ParseResponseRaw(resp *http.Response, err error) (any, error) {
	body, ct, err := ReadResponse(resp, err)
	if err != nil {
		return nil, err
	}

	return DecodeResponseBody(body, ct, false)
}

func DecodeResponseBody(body []byte, contentType string, fallbackToText bool) (any, error) {
	if !IsJSONContentType(contentType) {
		return string(body), nil
	}
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		if fallbackToText {
			return string(body), nil
		}
		return nil, err
	}
	return result, nil
}

func ReadResponse(resp *http.Response, err error) ([]byte, string, error) {
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, "", readErr
	}

	if resp.StatusCode >= 400 {
		msg := extractErrorMessage(body, resp.StatusCode, resp.Request.Method, resp.Request.URL.Path)
		return nil, "", fmt.Errorf("%s", msg)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func IsJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func extractErrorMessage(body []byte, status int, method, path string) string {
	var parsed map[string]any
	msg := ""
	if json.Unmarshal(body, &parsed) == nil {
		if m, ok := parsed["message"].(string); ok {
			msg = m
		} else if e, ok := parsed["error"].(string); ok {
			msg = e
		}
	}
	if msg == "" && len(body) > 0 {
		msg = string(body)
		if len(msg) > 200 {
			msg = msg[:200]
		}
	}
	if msg != "" {
		return fmt.Sprintf("%s %s → %d: %s", method, path, status, msg)
	}
	return fmt.Sprintf("%s %s → %d", method, path, status)
}

// Ctx returns a background context (convenience for CLI commands).
func Ctx() context.Context {
	return context.Background()
}
