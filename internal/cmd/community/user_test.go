package community

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestFetchUserUsesIdentifierAndKeepsProfileFields(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/community/users/alice" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"user":{"id":"u1","username":"alice","name":"Alice","_count":{"comments":3}},"sectionCount":2,"totalContributions":3,"weeks":[[{"date":"2026-01-01","count":3}]]}`)
	}))
	defer server.Close()
	client, err := api.NewTypedClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fetchUser(client, "alice")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]any)
	if !ok || m["sectionCount"] != float64(2) {
		t.Fatalf("profile response = %#v", data)
	}
	if got := m["user"].(map[string]any)["username"]; got != "alice" {
		t.Fatalf("username = %#v", got)
	}
}
