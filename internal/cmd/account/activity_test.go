package account

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestFetchActivityMapsCursorAndLimit(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/account/client-activity" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if got := r.URL.Query().Get("cursor"); got != "cursor-1" {
			t.Errorf("cursor = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"id":"a1","action":"workspace.todo.list","outcome":"success","channel":"rest","createdAt":"2026-01-01T00:00:00Z","targetType":"todo"}],"nextCursor":"cursor-2"}`)
	}))
	defer server.Close()

	client, err := api.NewTypedClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fetchActivity(client, "cursor-1", 20)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]any)
	if !ok || m["nextCursor"] != "cursor-2" {
		t.Fatalf("activity response = %#v", data)
	}
	items, ok := m["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("activity items = %#v", m["items"])
	}
}

func TestFetchActivityRejectsNegativeLimit(t *testing.T) {
	if _, err := fetchActivity(nil, "", -1); err == nil {
		t.Fatal("fetchActivity accepted a negative limit")
	}
}
