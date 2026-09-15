package workspace

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/workspace/calendar/events" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("dateFrom") != "2026-09-01" || r.URL.Query().Get("dateTo") != "2026-09-30" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"young-event-1","type":"young_event","at":"2026-09-01T01:00:00Z","endsAt":null,"title":"Activity","location":null,"url":"/catalog/young-events/event-1","youngId":"event-1"}],"pagination":{"page":1,"pageSize":20,"total":1,"totalPages":1}}`)
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
}
