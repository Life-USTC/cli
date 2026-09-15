package young_workspace

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/spf13/cobra"
)

func TestEventSubscriptionSetSendsReminderFlags(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/workspace/young-event-subscriptions/event-1" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), `{"remindDeadline":false,"remindSignup":true,"remindStart":true,"subscribed":true}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"youngId":"event-1","subscribed":true,"remindSignup":true,"remindDeadline":false,"remindStart":true}`)
	}))
	defer server.Close()
	saveTestCredentials(t, server.URL)
	root := commandRoot(server.URL, NewCmdYoungEventSubscription())
	root.SetArgs([]string{"young-event-subscription", "set", "--subscribed", "true", "--remind-signup", "true", "--remind-deadline", "false", "--remind-start", "true", "event-1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer access-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestOrganizerSubscriptionListMapsPagination(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/workspace/young-organizer-subscriptions" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("pageSize") != "10" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"pagination":{"page":2,"pageSize":10,"total":0,"totalPages":1}}`)
	}))
	defer server.Close()
	saveTestCredentials(t, server.URL)
	root := commandRoot(server.URL, NewCmdYoungOrganizerSubscription())
	root.SetArgs([]string{"young-organizer-subscription", "list", "--page", "2", "--limit", "10"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationReadPostsToReadRoute(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/workspace/young-notifications/notification-1/read" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"notification-1","success":true}`)
	}))
	defer server.Close()
	saveTestCredentials(t, server.URL)
	root := commandRoot(server.URL, NewCmdYoungNotification())
	root.SetArgs([]string{"young-notification", "read", "notification-1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func commandRoot(server string, child *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("server", server, "")
	root.AddCommand(child)
	return root
}

func saveTestCredentials(t *testing.T, server string) {
	t.Helper()
	if err := config.SaveCredentials(server, &config.Credential{
		AccessToken: "access-token",
		ExpiresAt:   float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		t.Fatal(err)
	}
}
