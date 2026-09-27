package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func descriptionContractResponse(content, history string) string {
	encoded, _ := json.Marshal(content)
	return fmt.Sprintf(`{"description":{"id":"description-1","content":%s,"renderedHtml":"","updatedAt":null,"lastEditedAt":null,"lastEditedBy":null},"history":%s,"viewer":{"userId":null,"name":null,"image":null,"isAuthenticated":false,"isAdmin":false,"isSuspended":false,"suspensionReason":null,"suspensionExpiresAt":null}}`, encoded, history)
}

func TestSpecDescriptionContentOutput(t *testing.T) {
	t.Run("description.cli-content-output", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, content := range []string{"Office hours: Monday afternoon.", ""} {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet || r.URL.Path != "/api/community/descriptions" || r.URL.Query().Get("targetType") != "course" || r.URL.Query().Get("targetId") != "42" || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected public description request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, descriptionContractResponse(content, "[]"))
			}))
			args := []string{"--server", server.URL, "community", "description", "get", "--target-type", "course", "--target-id", "42"}
			text, err := executeGeneratedCommand(t, args)
			if err != nil {
				server.Close()
				t.Fatal(err)
			}
			if content == "" {
				if !strings.Contains(text, "No description.") {
					t.Errorf("missing empty state: %q", text)
				}
			} else if !strings.Contains(text, content) || strings.Contains(text, "No description.") {
				t.Errorf("nested description content was lost: %q", text)
			}
			text, err = executeGeneratedCommand(t, append([]string{"--json"}, args...))
			server.Close()
			if err != nil {
				t.Fatal(err)
			}
			var response struct {
				Description struct {
					Content string `json:"content"`
					ID      string `json:"id"`
				} `json:"description"`
			}
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				t.Fatal(err)
			}
			if requests != 2 || response.Description.Content != content || response.Description.ID != "description-1" {
				t.Fatalf("description contract lost: requests=%d response=%s", requests, text)
			}
		}
	})
}

func TestSpecDescriptionHistoryOutput(t *testing.T) {
	t.Run("description.cli-history-output", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		history := `[{"id":"revision-1","createdAt":"2026-01-01T00:15:00Z","editor":{"id":"editor-1","name":"Editor Alice","username":null,"image":null},"nextContent":"First revision","previousContent":null},{"id":"revision-2","createdAt":"2026-01-02T00:30:00Z","editor":null,"nextContent":"Second revision","previousContent":"First revision"}]`
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.Method != http.MethodGet || r.URL.Path != "/api/community/descriptions" || r.URL.Query().Get("targetType") != "teacher" || r.URL.Query().Get("targetId") != "7" {
				t.Errorf("unexpected history request: %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, descriptionContractResponse("Second revision", history))
		}))
		defer server.Close()
		text, err := executeGeneratedCommand(t, []string{"--server", server.URL, "community", "description", "get", "--target-type", "teacher", "--target-id", "7"})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"History", "revision-1", "revision-2", "Editor Alice", "2026-01-01 00:15", "2026-01-02 00:30"} {
			if !strings.Contains(text, want) {
				t.Errorf("missing history field %q in %q", want, text)
			}
		}
		if requests != 1 {
			t.Fatalf("requests=%d, want 1", requests)
		}
	})
}
