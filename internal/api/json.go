package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DoJSON performs a JSON request through the authenticated transport and
// decodes the response using the same error handling as the generated client.
// It is used for endpoints whose generated OpenAPI client is refreshed with
// the server contract independently from the CLI command implementation.
func (c *Client) DoJSON(
	ctx context.Context,
	method string,
	path string,
	params url.Values,
	body any,
) (any, error) {
	var requestBody io.Reader
	contentType := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode JSON request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	resp, err := c.DoRaw(ctx, method, path, params, requestBody, contentType, http.Header{})
	return ParseResponseRaw(resp, err)
}
