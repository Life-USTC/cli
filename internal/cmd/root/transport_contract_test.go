package root

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
)

func TestSpecAdminSessionBoundary(t *testing.T) {
	t.Run("cli.admin-session-boundary", func(t *testing.T) {
		root := NewCmdRoot()
		for _, cmd := range root.Commands() {
			if cmd.Name() == "admin" {
				t.Fatal("OAuth CLI must not advertise browser-session administration")
			}
		}
		_, err := executeGeneratedCommand(t, []string{"admin", "user", "list"})
		if err == nil {
			t.Fatal("unsupported administration accepted")
		}
	})
}

func TestSpecBusPreferencePartialUpdate(t *testing.T) {
	t.Run("cli.bus-preference-partial-update", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, denied := range []bool{false, true} {
			calls := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method)
				if r.URL.Path != "/api/workspace/bus-preferences" || r.Header.Get("Authorization") != "Bearer owner" {
					t.Errorf("unexpected request %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					if denied {
						http.Error(w, "denied", http.StatusForbidden)
						return
					}
					_, _ = io.WriteString(w, `{"preference":{"preferredOriginCampusId":1,"preferredDestinationCampusId":2,"showDepartedTrips":true}}`)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != http.MethodPost || body["preferredOriginCampusId"] != float64(1) || body["preferredDestinationCampusId"] != float64(3) || body["showDepartedTrips"] != true || len(body) != 3 {
					t.Errorf("partial flag overwrote state: %#v", body)
				}
				_, _ = io.WriteString(w, `{"preference":{"preferredOriginCampusId":1,"preferredDestinationCampusId":3,"showDepartedTrips":true}}`)
			}))
			if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
				t.Fatal(err)
			}
			_, err := executeGeneratedCommand(t, []string{"--server", server.URL, "workspace", "bus-preferences", "set", "--destination", "3"})
			server.Close()
			if denied {
				if err == nil || len(calls) != 1 {
					t.Fatalf("failed GET allowed mutation: %v %v", calls, err)
				}
			} else if err != nil || len(calls) != 2 || calls[0] != "GET" || calls[1] != "POST" {
				t.Fatalf("update sequence %v %v", calls, err)
			}
		}
	})
}

func TestSpecBusPreferenceJSONInput(t *testing.T) {
	t.Run("cli.bus-preference-json-input", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.Method != http.MethodPost || r.URL.Path != "/api/workspace/bus-preferences" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["preferredOriginCampusId"] != float64(1) || body["showDepartedTrips"] != true {
				t.Errorf("unexpected body %#v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"preference":{"preferredOriginCampusId":1,"preferredDestinationCampusId":null,"showDepartedTrips":true}}`)
		}))
		defer server.Close()
		if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{`null`, `[]`, `{"unknown":true}`, `{"showDepartedTrips":"true"}`, `{"showDepartedTrips":true} {}`, `{"showDepartedTrips":true} trailing`} {
			_, err := executeGeneratedCommand(t, []string{"--server", server.URL, "workspace", "bus-preferences", "set", "--raw-json", value})
			if err == nil || requests != 0 {
				t.Fatalf("invalid JSON %q accepted: requests=%d err=%v", value, requests, err)
			}
		}
		_, err := executeGeneratedCommand(t, []string{"--server", server.URL, "workspace", "bus-preferences", "set", "--raw-json", `{"preferredOriginCampusId":1,"preferredDestinationCampusId":null,"showDepartedTrips":true}`})
		if err != nil || requests != 1 {
			t.Fatalf("valid JSON: requests=%d err=%v", requests, err)
		}
	})
}

func TestSpecExamMonitorIdentityOutput(t *testing.T) {
	t.Run("exam.cli-monitor-identity", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/workspace/exams" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"id":1,"jwId":2,"monitors":[{"jwId":7301,"nameCn":"监考老师","nameEn":"Invigilator"}]}],"pagination":{"page":1,"pageSize":100,"total":1,"totalPages":1}}`)
		}))
		defer server.Close()
		if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
			t.Fatal(err)
		}
		text, err := executeGeneratedCommand(t, []string{"--server", server.URL, "--json", "workspace", "exam"})
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Data []struct {
				Monitors []struct {
					JwID   int    `json:"jwId"`
					NameCn string `json:"nameCn"`
					NameEn string `json:"nameEn"`
				} `json:"monitors"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Data) != 1 || len(body.Data[0].Monitors) != 1 || body.Data[0].Monitors[0].JwID != 7301 || body.Data[0].Monitors[0].NameCn != "监考老师" || body.Data[0].Monitors[0].NameEn != "Invigilator" {
			t.Fatalf("monitor identity lost: %s", text)
		}
	})
}

func TestSpecYoungParticipationOutput(t *testing.T) {
	t.Run("young.cli-participation-fields", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		fields := map[string]any{
			"youngId": "event-1", "name": "Campus event", "contactName": "Organizer", "contactTel": "0551-12345678",
			"onlineMeetingInfo": "Meeting 123", "isOnline": true, "requiresSignupInfo": false,
			"allowedAttachmentTypes": []any{"pdf"}, "signupScopeCode": "department", "signupDepartmentIds": []any{"dept-1"},
			"upstreamOrganizerIds": []any{"org-1"}, "upstreamSponsorIds": []any{"sponsor-1"}, "tagIds": []any{"tag-1"},
			"appliedCount": float64(0), "participationNotes": "Bring student card", "rawJson": map[string]any{"sourceDetail": "retained"},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/young-events/event-1" || r.Header.Get("Authorization") != "" {
				t.Errorf("unexpected public request %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(fields)
		}))
		defer server.Close()
		text, err := executeGeneratedCommand(t, []string{"--server", server.URL, "--json", "catalog", "young-event", "get", "event-1"})
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatal(err)
		}
		for field, expected := range fields {
			want, _ := json.Marshal(expected)
			got, _ := json.Marshal(body[field])
			if string(got) != string(want) {
				t.Errorf("%s = %s, want %s", field, got, want)
			}
		}
	})
}
