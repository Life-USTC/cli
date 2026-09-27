package homework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
)

func homeworkContractServer(t *testing.T, denied bool) (*httptest.Server, *int) {
	t.Helper()
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/workspace/subscriptions/current" {
			_, _ = io.WriteString(w, `{"subscription":null}`)
			return
		}
		if r.Method != "GET" || (r.URL.Path != "/api/workspace/homeworks" && r.URL.Path != "/api/community/section-homeworks") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/api/community/section-homeworks" && r.URL.Query().Get("sectionId") != "7" {
			t.Errorf("section query %s", r.URL)
		}
		count++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if page < 1 || size < 1 || size > 100 {
			http.Error(w, "invalid pagination", http.StatusBadRequest)
			return
		}
		if denied && page == 2 {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		rows := []map[string]any{}
		for id := (page-1)*size + 1; id <= page*size && id <= 101; id++ {
			row := map[string]any{"id": fmt.Sprintf("h%d", id), "title": fmt.Sprintf("Assignment %d", id), "submissionDueAt": "2030-09-30T00:00:00Z", "completionRequired": true, "section": map[string]any{"id": 7, "code": "CS7", "course": map[string]any{"namePrimary": "Course"}}}
			if id <= 100 {
				row["completion"] = map[string]any{"completedAt": "2026-09-01T00:00:00Z"}
			} else {
				row["section"] = map[string]any{"id": 8, "code": "CS8", "course": map[string]any{"namePrimary": "Later Course"}}
			}
			rows = append(rows, row)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]int{"page": page, "pageSize": size, "total": 101, "totalPages": 2}})
	}))
	t.Cleanup(server.Close)
	if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}
	return server, &count
}

func runHomeworkContract(t *testing.T, cmd *cobra.Command, server string, args ...string) (string, error) {
	t.Helper()
	cmd.PersistentFlags().String("server", server, "")
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	return captureHomeworkContract(t, cmd.Execute)
}

func captureHomeworkContract(t *testing.T, run func() error) (string, error) {
	t.Helper()
	old, opts := os.Stdout, output.Current
	output.Current = &output.Opts{Format: "json", NoColor: true}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	done := make(chan string)
	go func() { data, _ := io.ReadAll(reader); done <- string(data) }()
	err = run()
	_ = writer.Close()
	os.Stdout = old
	output.Current = opts
	text := <-done
	_ = reader.Close()
	return text, err
}

func TestSpecHomeworkSectionList(t *testing.T) {
	t.Run("homework.cli-section-pagination", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server, count := homeworkContractServer(t, false)
		text, err := runHomeworkContract(t, NewCmdSectionHomework(), server.URL, "list", "7", "--page", "2", "--limit", "100", "--include-deleted")
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Data []struct {
				ID string `json:"id"`
			}
			Pagination struct{ Total, Page int }
		}
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatal(err)
		}
		if *count != 2 || len(body.Data) != 1 || body.Data[0].ID != "h101" || body.Pagination.Total != 101 || body.Pagination.Page != 2 {
			t.Fatalf("wrong page: count=%d body=%s", *count, text)
		}
	})
}

func TestSpecHomeworkFilteredList(t *testing.T) {
	t.Run("homework.cli-filter-after-pages", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, section := range []bool{false, true} {
			for _, denied := range []bool{false, true} {
				server, count := homeworkContractServer(t, denied)
				args := []string{"--pending", "--limit", "1"}
				if section {
					args = append(args, "--section-id", "7")
				}
				text, err := runHomeworkContract(t, NewCmdMyHomework(), server.URL, args...)
				if denied {
					if err == nil || text != "" {
						t.Fatalf("partial output %s %v", text, err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				var body struct {
					Data []struct {
						ID string `json:"id"`
					}
					Pagination struct{ Total int }
				}
				if err := json.Unmarshal([]byte(text), &body); err != nil {
					t.Fatal(err)
				}
				if *count != 2 || len(body.Data) != 1 || body.Data[0].ID != "h101" || body.Pagination.Total != 1 {
					t.Fatalf("filtered incomplete pages: %s", text)
				}
			}
		}
	})
}

func TestSpecHomeworkPicker(t *testing.T) {
	t.Run("homework.cli-picker-complete-input", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server, count := homeworkContractServer(t, false)
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		cmd.PersistentFlags().String("server", server.URL, "")
		rows, err := fetchHomeworkPickList(cmd, myHomeworkListOpts{pending: true, limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if *count != 2 || len(rows) != 1 || rows[0]["id"] != "h101" {
			t.Fatalf("picker omitted later match: %v", rows)
		}
	})
}

func TestSpecHomeworkSubscribedSections(t *testing.T) {
	t.Run("homework.cli-subscription-fallback", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server, count := homeworkContractServer(t, false)
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		cmd.PersistentFlags().String("server", server.URL, "")
		rows, err := loadSubscribedSections(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if *count != 2 || len(rows) != 2 || rows[1]["id"] != float64(8) {
			t.Fatalf("sections omitted later page or duplicates retained: %v", rows)
		}
	})
}

func TestSpecHomeworkCreatedIdentity(t *testing.T) {
	t.Run("homework.cli-created-identity", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/api/community/section-homeworks" {
				t.Errorf("unexpected %s %s", r.Method, r.URL)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"homework":{"id":"h-created","title":"Assignment"}}`)
		}))
		defer server.Close()
		if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
			t.Fatal(err)
		}
		text, err := runHomeworkContract(t, NewCmdSectionHomework(), server.URL, "create", "7", "--title", "Assignment")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, "h-created") {
			t.Fatalf("created identity lost: %q", text)
		}
	})
}
