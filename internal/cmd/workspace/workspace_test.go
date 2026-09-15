package workspace

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/spf13/cobra"
)

func TestBuildCalendarEventParamsRequiresPairedBounds(t *testing.T) {
	if _, err := buildCalendarEventParams(calendarEventOpts{dateFrom: "2026-09-01"}); err == nil {
		t.Fatal("calendar params accepted an unpaired date bound")
	}
}

func TestCalendarEventsUsesCompletePersonalCalendarEndpoint(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	seenPages := map[string]bool{}
	var seenPagesMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/workspace/calendar/events" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("dateFrom") != "2026-09-01" || r.URL.Query().Get("dateTo") != "2026-09-30" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("pageSize") != "100" {
			t.Fatalf("pageSize = %q", r.URL.Query().Get("pageSize"))
		}
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		seenPagesMu.Lock()
		seenPages[page] = true
		seenPagesMu.Unlock()
		switch page {
		case "1":
			_, _ = io.WriteString(w, `{"data":[{"id":"young-event-1","type":"young_event","at":"2026-09-01T01:00:00Z","endsAt":null,"title":"Activity 1","location":null,"url":"/catalog/young-events/event-1","youngId":"event-1"}],"pagination":{"page":1,"pageSize":100,"total":2,"totalPages":2}}`)
		case "2":
			_, _ = io.WriteString(w, `{"data":[{"id":"young-event-2","type":"young_event","at":"2026-09-02T01:00:00Z","endsAt":null,"title":"Activity 2","location":null,"url":"/catalog/young-events/event-2","youngId":"event-2"}],"pagination":{"page":2,"pageSize":100,"total":2,"totalPages":2}}`)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	if err := config.SaveCredentials(server.URL, &config.Credential{
		AccessToken: "access-token",
		ExpiresAt:   float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("server", server.URL, "")
	root.AddCommand(newCmdCalendar())
	root.SetArgs([]string{"calendar", "events", "--date-from", "2026-09-01", "--date-to", "2026-09-30"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	seenPagesMu.Lock()
	pageCount := len(seenPages)
	pageOne, pageTwo := seenPages["1"], seenPages["2"]
	seenPagesMu.Unlock()
	if !pageOne || !pageTwo || pageCount != 2 {
		t.Fatalf("pages fetched = %v (page 1: %v, page 2: %v), want pages 1 and 2", pageCount, pageOne, pageTwo)
	}
}
