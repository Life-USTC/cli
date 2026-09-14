package calendar

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/config"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
)

func TestCalendarSubscriptionDetailsUseOnlyScopeGatedURL(t *testing.T) {
	const calendarURL = "https://life.example/api/calendar-feeds/private-token.ics"
	details := calendarSubscriptionDetails(map[string]any{
		"calendarUrl": calendarURL,
		"userId":      "must-not-be-used-to-build-a-url",
	})
	if got := details[0].Value; !strings.Contains(got.(string), calendarURL) {
		t.Fatalf("URL detail = %#v, want returned calendarUrl", got)
	}
}

func TestCalendarSubscriptionDetailsExplainMissingScopeGatedURL(t *testing.T) {
	for _, calendarURL := range []any{nil, ""} {
		details := calendarSubscriptionDetails(map[string]any{
			"calendarUrl": calendarURL,
			"userId":      "must-not-be-used-to-build-a-url",
		})
		got, _ := details[0].Value.(string)
		if !strings.Contains(got, "Unavailable") || !strings.Contains(got, "workspace.calendar-feed:read") {
			t.Fatalf("URL detail = %q, want clear missing-scope message", got)
		}
		if strings.Contains(got, "must-not-be-used") {
			t.Fatalf("URL detail synthesized a fallback feed: %q", got)
		}
	}
}

func TestValidSubscriptionKind(t *testing.T) {
	for _, value := range []string{"regular", "teaching_assistant", "auditor"} {
		if !openapi.SubscriptionKindUpdateRequestSchemaKind(value).Valid() {
			t.Errorf("validSubscriptionKind(%q) = false", value)
		}
	}
	if openapi.SubscriptionKindUpdateRequestSchemaKind("student").Valid() {
		t.Error("validSubscriptionKind accepted unknown kind")
	}
}

func TestSubscriptionSectionColumnsUseExternalIdentityAndLocalizedFields(t *testing.T) {
	columns := subscriptionSectionColumns()
	want := map[string]bool{
		"jwId":               false,
		"course.namePrimary": false,
		"semester.nameCn":    false,
		"kind":               false,
	}
	for _, column := range columns {
		if _, ok := want[column.Key]; ok {
			want[column.Key] = true
		}
		if column.Key == "id" {
			t.Fatal("subscription table exposes internal database ID")
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("subscription table missing %s", key)
		}
	}
}

func TestSetSubscriptionKindUsesExternalJWIDAndAuth(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/workspace/subscriptions/12345" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), `{"kind":"auditor"}`; got != want {
			t.Fatalf("request body = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"sectionJwId":12345,"kind":"auditor"}`)
	}))
	defer server.Close()
	if err := config.SaveCredentials(server.URL, &config.Credential{
		AccessToken: "access-token",
		ExpiresAt:   float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		t.Fatal(err)
	}
	client, err := api.NewTypedClient(server.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := setSubscriptionKind(client, 12345, openapi.SubscriptionKindUpdateRequestSchemaKindAuditor)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer access-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if got := data.(map[string]any)["sectionJwId"]; got != float64(12345) {
		t.Fatalf("response sectionJwId = %#v", got)
	}
}

func TestSetSubscriptionKindReturnsForbiddenAndNotFoundErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"message":"denied"}`)
			}))
			defer server.Close()
			if err := config.SaveCredentials(server.URL, &config.Credential{
				AccessToken: "access-token",
				ExpiresAt:   float64(time.Now().Add(time.Hour).Unix()),
			}); err != nil {
				t.Fatal(err)
			}
			client, err := api.NewTypedClient(server.URL, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := setSubscriptionKind(client, 12345, openapi.SubscriptionKindUpdateRequestSchemaKindRegular); err == nil {
				t.Fatalf("status %d returned nil error", status)
			}
		})
	}
}
