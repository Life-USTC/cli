package young_event

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/youngutil"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
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
	client, err := api.NewTypedClient(server.URL, false)
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

func TestDateViewUnknownDateHintPreservesFilters(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := map[string][]string{
			"active": {"false"}, "category": {"Science & Tech"}, "search": {"freshmen's fair"},
			"organizerId": {"org/科大+1"}, "timeBasis": {"registration"},
			"dateFrom": {"2026-09-15"}, "dateTo": {"2026-09-15"}, "page": {"1"}, "pageSize": {"100"},
		}
		if r.URL.Path != "/api/catalog/young-events" || !reflect.DeepEqual(map[string][]string(r.URL.Query()), want) {
			t.Errorf("request = %s, want filters %#v", r.URL, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"pagination":{"page":1,"pageSize":100,"total":0,"totalPages":1},"meta":{"unknownDateCount":2,"source":{"status":"unknown","lastSyncedAt":null}}}`)
	}))
	defer server.Close()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.PersistentFlags().String("server", server.URL, "")
	file, err := os.Create(filepath.Join(t.TempDir(), "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldOutput := os.Stdout, output.Current
	os.Stdout, output.Current = file, &output.Opts{Format: "table", NoColor: true}
	t.Cleanup(func() { os.Stdout, output.Current = oldStdout, oldOutput; _ = file.Close() })
	if err := runDateView(cmd, youngutil.DateDay, "2026-09-15", dateOpts{
		active: "false", category: "Science & Tech", search: "freshmen's fair",
		organizerID: "org/科大+1", timeBasis: "registration",
	}); err != nil {
		t.Fatal(err)
	}
	printed, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "Unknown-date activities: life-ustc catalog young-event list --date-unknown true --active 'false' --category 'Science & Tech' --search 'freshmen'\\''s fair' --organizer-id 'org/科大+1' --time-basis 'registration'\n"
	if !strings.Contains(string(printed), want) {
		t.Fatalf("output = %s\nwant hint = %s", printed, want)
	}
}
