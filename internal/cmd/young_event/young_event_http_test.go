package young_event

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestFetchListMapsAllServerFilters(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/young-events" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{
			"active": "true", "category": "系列项目", "search": "robotics", "page": "2", "pageSize": "20",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"pagination":{"page":2,"pageSize":20,"total":0,"totalPages":0}}`)
	}))
	defer server.Close()
	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	params, err := buildListParams(listOpts{
		active:   "true",
		category: "系列项目",
		search:   "robotics",
		page:     2,
		pageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fetchList(context.Background(), client, params); err != nil {
		t.Fatal(err)
	}
}
