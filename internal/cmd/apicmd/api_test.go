package apicmd

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestNormalizeAPIPath(t *testing.T) {
	cases := map[string]string{
		"catalog/metadata":                  "/api/catalog/metadata",
		"api/catalog/metadata":              "/api/catalog/metadata",
		"/api/catalog/metadata":             "/api/catalog/metadata",
		"/.well-known/openid-configuration": "/.well-known/openid-configuration",
		// Empty string produces a trailing slash; callers should avoid passing "".
		"": "/api/",
		// Paths starting with "." but no leading slash are treated as bare paths and
		// get the /api/ prefix.  Users must supply a leading slash to reach
		// /.well-known/* endpoints without the /api prefix.
		".well-known/openid-configuration": "/api/.well-known/openid-configuration",
		// A path that already starts with "/" is returned verbatim, even double-slashes.
		"//double-slash": "//double-slash",
	}
	for in, want := range cases {
		if got := normalizeAPIPath(in); got != want {
			t.Fatalf("normalizeAPIPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildFieldsParsesTypedAndRawValues(t *testing.T) {
	got, err := buildFields([]string{"count=3", "enabled=true", "ratio=1.5", "empty=null"}, []string{"name=3"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name":    "3",
		"count":   int64(3),
		"enabled": true,
		"ratio":   1.5,
		"empty":   nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestParseHeaders(t *testing.T) {
	got, err := parseHeaders([]string{"Accept: application/json", "X-Test: one", "X-Test: two"})
	if err != nil {
		t.Fatal(err)
	}
	want := http.Header{
		"Accept": []string{"application/json"},
		"X-Test": []string{"one", "two"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("headers = %#v, want %#v", got, want)
	}
}

func TestBuildFieldsInvalidInputs(t *testing.T) {
	// A field with no '=' must produce an error.
	if _, err := buildFields(nil, []string{"noequals"}); err == nil {
		t.Fatal("expected error for raw field without '=', got nil")
	}
	// A field with an empty key must produce an error.
	if _, err := buildFields(nil, []string{"=value"}); err == nil {
		t.Fatal("expected error for raw field with empty key, got nil")
	}
	// Typed field with no '=' must also produce an error.
	if _, err := buildFields([]string{"noequals"}, nil); err == nil {
		t.Fatal("expected error for typed field without '=', got nil")
	}
}

func TestBuildFieldsTypedFieldOverridesRaw(t *testing.T) {
	// When the same key appears in both rawFields and typedFields, the typed value wins
	// because buildFields processes rawFields first, then typed fields overwrite.
	got, err := buildFields([]string{"count=42"}, []string{"count=99"})
	if err != nil {
		t.Fatal(err)
	}
	if got["count"] != int64(42) {
		t.Fatalf("expected typed int64(42) to win, got %v (%T)", got["count"], got["count"])
	}
}

func TestParseHeadersErrors(t *testing.T) {
	// A header with no ':' must produce an error.
	if _, err := parseHeaders([]string{"InvalidHeader"}); err == nil {
		t.Fatal("expected error for header without ':', got nil")
	}
	// A header with an empty key must produce an error.
	if _, err := parseHeaders([]string{": value"}); err == nil {
		t.Fatal("expected error for header with empty key, got nil")
	}
}

func TestParseHeadersNoSpaceAfterColon(t *testing.T) {
	// TrimSpace must handle headers without the conventional space after ":".
	got, err := parseHeaders([]string{"X-Custom:value"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("X-Custom") != "value" {
		t.Fatalf("expected 'value', got %q", got.Get("X-Custom"))
	}
}

func TestIsJSONResponse(t *testing.T) {
	trueCases := []string{
		"application/json",
		"application/json; charset=utf-8",
		"application/problem+json",
	}
	for _, ct := range trueCases {
		if !api.IsJSONContentType(ct) {
			t.Fatalf("%q should be JSON", ct)
		}
	}
	falseCases := []string{
		"text/json",
		// Must NOT match: uses HasSuffix("+json"), so "jsonp" suffix is correctly rejected.
		"application/jsonp",
		"",
	}
	for _, ct := range falseCases {
		if api.IsJSONContentType(ct) {
			t.Fatalf("%q should not be treated as JSON", ct)
		}
	}
}
