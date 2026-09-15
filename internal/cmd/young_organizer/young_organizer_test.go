package young_organizer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/spf13/cobra"
)

func TestBuildListParamsMapsSearchAndPagination(t *testing.T) {
	params, err := buildListParams(listOpts{search: "学生会", page: 2, pageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"search": "学生会", "page": "2", "pageSize": "25"} {
		if got := params.Get(key); got != want {
			t.Errorf("params[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestBuildListParamsRejectsNegativePagination(t *testing.T) {
	if _, err := buildListParams(listOpts{page: -1}); err == nil {
		t.Fatal("negative page accepted")
	}
	if _, err := buildListParams(listOpts{pageSize: -1}); err == nil {
		t.Fatal("negative limit accepted")
	}
}

func TestOrganizerListUsesPublicEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/young-organizers" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if got := r.URL.Query().Get("search"); got != "学生会" {
			t.Fatalf("search = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"pagination":{"page":1,"pageSize":20,"total":0,"totalPages":1}}`)
	}))
	defer server.Close()
	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	params, err := buildListParams(listOpts{search: "学生会"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.DoJSON(t.Context(), http.MethodGet, "/api/catalog/young-organizers", params, nil); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizerGetLoadsEventsFromPaginatedCatalog(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/catalog/young-organizers/org-1":
			_, _ = io.WriteString(w, `{"id":"org-1","name":"Students Union","normalizedName":"students union","activeCount":1,"upcomingCount":1,"historyCount":0}`)
		case "/api/catalog/young-events":
			if r.URL.Query().Get("organizerId") != "org-1" {
				t.Fatalf("organizerId = %q", r.URL.Query().Get("organizerId"))
			}
			if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("pageSize") != "100" {
				t.Fatalf("pagination = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"data":[{"youngId":"event-1","name":"Activity","category":"single","startAt":"2026-09-01T09:00:00+08:00","endAt":null,"isActive":true}],"pagination":{"page":1,"pageSize":100,"total":1,"totalPages":1},"unknownDates":[],"source":{"status":"fresh","lastSyncedAt":null}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("server", server.URL, "")
	root.AddCommand(NewCmdYoungOrganizer())
	root.SetArgs([]string{"young-organizer", "get", "org-1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
