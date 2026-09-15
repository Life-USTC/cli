package comment

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/spf13/cobra"
)

func TestYoungEventCommentTargetRequiresYoungID(t *testing.T) {
	if err := validateTarget(commentTarget{targetType: "young-event"}, false); err == nil {
		t.Fatal("young-event target without young ID was accepted")
	}
	if err := validateTarget(commentTarget{targetType: "young-event", youngID: "young-1"}, false); err != nil {
		t.Fatal(err)
	}
}

func TestYoungEventCommentListUsesYoungIDQuery(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	seenPages := map[string]bool{}
	var seenPagesMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/community/comments" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if got := r.URL.Query().Get("targetType"); got != "young-event" {
			t.Fatalf("targetType = %q", got)
		}
		if got := r.URL.Query().Get("youngId"); got != "young-1" {
			t.Fatalf("youngId = %q", got)
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
			_, _ = io.WriteString(w, `{"data":[{"id":"comment-1","body":"one"}],"pagination":{"page":1,"pageSize":100,"total":2,"totalPages":2},"meta":{}}`)
		case "2":
			_, _ = io.WriteString(w, `{"data":[{"id":"comment-2","body":"two"}],"pagination":{"page":2,"pageSize":100,"total":2,"totalPages":2},"meta":{}}`)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	cmd := commandWithServer(server.URL)
	if err := runCommentList(cmd, commentTarget{targetType: "young-event", youngID: "young-1"}); err != nil {
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

func TestYoungEventCommentCreateUsesYoungIDBody(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/community/comments" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), `{"body":"hello","isAnonymous":false,"targetType":"young-event","visibility":"public","youngId":"young-1"}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"comment-1"}`)
	}))
	defer server.Close()
	if err := config.SaveCredentials(server.URL, &config.Credential{
		AccessToken: "access-token",
		ExpiresAt:   float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		t.Fatal(err)
	}
	cmd := commandWithServer(server.URL)
	if err := runCommentCreate(cmd, commentTarget{targetType: "young-event", youngID: "young-1"}, "hello", "public", "", false); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer access-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func commandWithServer(server string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("server", server, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestReportCommentBatchResults_AllSuccess(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"id": "c1", "success": true},
			map[string]any{"id": "c2", "success": true},
		},
	}
	rows := []map[string]any{
		{"id": "c1", "body": "First comment"},
		{"id": "c2", "body": "Second comment"},
	}
	if err := reportCommentBatchResults(data, rows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportCommentBatchResults_PartialFailure(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"id": "c1", "success": true},
			map[string]any{"id": "c2", "success": false, "error": map[string]any{"message": "not found"}},
		},
	}
	err := reportCommentBatchResults(data, []map[string]any{{"id": "c1", "body": "First"}})
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "c2: not found") {
		t.Errorf("expected error to contain failed item, got %v", err)
	}
}

func TestCommentBatchLabel(t *testing.T) {
	if got := commentBatchLabel([]map[string]any{{"id": "c1", "body": "Hello"}}, 1); got != "Hello" {
		t.Errorf("single row label = %q, want %q", got, "Hello")
	}
	if got := commentBatchLabel([]map[string]any{{"id": "c1"}, {"id": "c2"}}, 2); got != "2 comments" {
		t.Errorf("multi row label = %q, want %q", got, "2 comments")
	}
	if got := commentBatchLabel(nil, 3); got != "3 comments" {
		t.Errorf("id-only label = %q, want %q", got, "3 comments")
	}
	if got := commentBatchLabel(nil, 1); got != "this comment" {
		t.Errorf("single id label = %q, want %q", got, "this comment")
	}
}
