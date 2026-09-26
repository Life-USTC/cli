package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
)

func executeExam(t *testing.T, serverURL, format string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	if err := config.SaveCredentials(serverURL, &config.Credential{
		AccessToken: "exam-token", ExpiresAt: float64(time.Now().Add(time.Hour).Unix()),
	}); err != nil {
		t.Fatal(err)
	}
	previousOutput, previousStdout := output.Current, os.Stdout
	output.Current.Format = format
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	done := make(chan string)
	go func() { data, _ := io.ReadAll(reader); done <- string(data) }()
	root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("server", serverURL, "")
	root.AddCommand(newCmdExam())
	root.SetArgs(append([]string{"exam"}, args...))
	execErr := root.Execute()
	_ = writer.Close()
	os.Stdout, output.Current = previousStdout, previousOutput
	result := <-done
	_ = reader.Close()
	return result, execErr
}

func examPage(total, page, pageSize int) map[string]any {
	rows := []map[string]any{}
	for i := (page - 1) * pageSize; i < page*pageSize && i < total; i++ {
		rows = append(rows, map[string]any{
			"id": i + 1, "examDate": "2026-09-15T08:00:00+08:00",
			"section": map[string]any{
				"course":   map[string]any{"namePrimary": fmt.Sprintf("Course %d", i+1)},
				"semester": map[string]any{"nameCn": "2026 Fall"},
			},
		})
	}
	return map[string]any{"data": rows, "pagination": map[string]any{
		"page": page, "pageSize": pageSize, "total": total, "totalPages": (total + pageSize - 1) / pageSize,
	}}
}

func TestExamReadsEveryPageAndPreservesTotal(t *testing.T) {
	var mu sync.Mutex
	pages := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspace/exams" || r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer exam-token" {
			t.Error("missing exam bearer")
		}
		if r.URL.Query().Get("pageSize") != "100" {
			t.Error("unexpected page size")
		}
		if r.URL.Query().Get("includeDateUnknown") != "true" {
			t.Error("unknown dates must be included by default")
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		mu.Lock()
		pages = append(pages, page)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(examPage(104, page, 100))
	}))
	defer server.Close()
	text, err := executeExam(t, server.URL, "json")
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Data       []map[string]any
		Pagination struct {
			Total    int
			PageSize int
		}
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 104 || payload.Pagination.Total != 104 || payload.Pagination.PageSize != 104 {
		t.Fatalf("incomplete result: %d rows, pagination %+v", len(payload.Data), payload.Pagination)
	}
	if payload.Data[103]["id"] != float64(104) {
		t.Fatal("last page missing")
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(pages) != "[1 2]" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestExamExplicitPaginationAndFilters(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		args                  []string
		page, pageSize, total int
	}{
		{"page", []string{"--page", "2"}, 2, 20, 24},
		{"limit", []string{"--limit", "2"}, 1, 2, 4},
		{"both", []string{"--page", "2", "--limit", "2"}, 2, 2, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests++
				mu.Unlock()
				if r.URL.Path != "/api/workspace/exams" {
					t.Error("exam list used the wrong endpoint")
				}
				for key, want := range map[string]string{"semesterId": "7", "dateFrom": "2026-09-01", "dateTo": "2026-09-30", "includeDateUnknown": "false"} {
					if r.URL.Query().Get(key) != want {
						t.Errorf("%s = %q", key, r.URL.Query().Get(key))
					}
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page == 0 {
					page = 1
				}
				size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
				if size == 0 {
					size = 20
				}
				if page != tc.page || size != tc.pageSize {
					t.Errorf("page/size = %d/%d", page, size)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(examPage(tc.total, page, size))
			}))
			defer server.Close()
			args := append(tc.args, "--semester-id", "7", "--date-from", "2026-09-01", "--date-to", "2026-09-30", "--include-date-unknown=false")
			text, err := executeExam(t, server.URL, "table", args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(text, "2026 Fall") || !strings.Contains(text, fmt.Sprintf("of %d", tc.total)) {
				t.Fatalf("missing semester or true total in %q", text)
			}
			mu.Lock()
			defer mu.Unlock()
			if requests != 1 {
				t.Fatalf("explicit pagination made %d requests", requests)
			}
		})
	}
}

func TestExamLaterPageFailureDoesNotPrintPartialSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			_ = json.NewEncoder(w).Encode(examPage(104, 1, 100))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"exam access revoked"}`)
	}))
	defer server.Close()
	text, err := executeExam(t, server.URL, "json")
	if err == nil {
		t.Fatal("later page failure was ignored")
	}
	if strings.TrimSpace(text) != "" {
		t.Fatalf("partial output printed: %s", text)
	}
}

func TestExamRejectsInvalidExplicitPaginationAndSemester(t *testing.T) {
	for _, args := range [][]string{{"--page", "0"}, {"--limit", "0"}, {"--semester-id", "0"}, {"--page", "-1"}, {"--limit", "101"}, {"--semester-id", "-1"}} {
		cmd := newCmdExam()
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}
