package publication

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestFetchListMapsCanonicalFiltersAndKeepsJSON(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/publications" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{
			"type": "notice", "source": "official", "query": "scholarship", "page": "2", "pageSize": "20",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"p1","publicationType":"notice","revision":{"title":"Scholarship","bodyText":"full text"}}],"pagination":{"page":2,"pageSize":20,"total":1,"totalPages":1}}`)
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	typeValue, err := normalizeType("notice")
	if err != nil {
		t.Fatal(err)
	}
	data, err := fetchList(client, listOpts{
		typeName: "notice",
		source:   "official",
		query:    "scholarship",
		page:     2,
		pageSize: 20,
	}, typeValue)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("decoded response = %T, want object", data)
	}
	rows, ok := m["data"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("decoded data = %#v", m["data"])
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["id"] != "p1" {
		t.Fatalf("decoded row = %#v", rows[0])
	}
	if body := row["revision"].(map[string]any)["bodyText"]; body != "full text" {
		t.Fatalf("full JSON field = %#v", body)
	}
}

func TestFetchGetEscapesIdentifierAndReportsHTTPError(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/api/publications/a%2Fb" {
			t.Errorf("escaped path = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"publication not found"}`)
	}))
	defer server.Close()

	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fetchGet(client, "a/b")
	if err == nil || !strings.Contains(err.Error(), "GET /api/publications/a/b → 404: publication not found") {
		t.Fatalf("fetchGet error = %v", err)
	}
}

func TestNormalizeTypeRejectsUnknownValue(t *testing.T) {
	if _, err := normalizeType("press-release"); err == nil {
		t.Fatal("normalizeType accepted unknown publication type")
	}
}

func TestValidateListBoundsRejectsNegativePagination(t *testing.T) {
	if err := validateListBounds(listOpts{page: -1}); err == nil {
		t.Fatal("validateListBounds accepted a negative page")
	}
	if err := validateListBounds(listOpts{pageSize: -1}); err == nil {
		t.Fatal("validateListBounds accepted a negative limit")
	}
}
