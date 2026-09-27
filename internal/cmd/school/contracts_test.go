package school

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	ustcschool "github.com/Life-USTC/CLI/internal/school"
	"github.com/spf13/cobra"
)

func TestSpecSchoolCompleteSemesters(t *testing.T) {
	t.Run("school.cli-complete-semesters", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, denied := range []bool{false, true} {
			pages := []int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
				pages = append(pages, page)
				if r.Method != "GET" || r.URL.Path != "/api/catalog/semesters" || page < 1 || size < 1 || size > 100 {
					http.Error(w, "invalid pagination", http.StatusBadRequest)
					return
				}
				if denied && page == 2 {
					http.Error(w, "denied", http.StatusForbidden)
					return
				}
				rows := []map[string]any{}
				for id := (page-1)*size + 1; id <= page*size && id <= 101; id++ {
					rows = append(rows, map[string]any{"id": id, "jwId": id, "code": fmt.Sprint(id), "nameCn": fmt.Sprintf("Semester %d", id)})
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]int{"page": page, "pageSize": size, "total": 101, "totalPages": 2}})
			}))
			client, err := api.NewTypedClient(server.URL, false)
			if err != nil {
				t.Fatal(err)
			}
			data, err := fetchAllLifeSemesters(context.Background(), client)
			server.Close()
			if len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
				t.Fatalf("pages=%v", pages)
			}
			if denied {
				if err == nil || data != nil {
					t.Fatalf("partial semesters escaped: %v %v", data, err)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if rows := cmdutil.NewListResult(data, "data").Rows; len(rows) != 101 {
				t.Fatalf("rows=%d", len(rows))
			}
			id, _, ok := resolveLifeSemester(data, ustcschool.Semester{ID: 101})
			if !ok || id != "101" {
				t.Fatalf("historical semester not mapped: %q %v", id, ok)
			}
		}
	})
}

func TestSpecSchoolExistingHomework(t *testing.T) {
	t.Run("school.cli-existing-homework", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, denied := range []bool{false, true} {
			pages, completions, creates := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/workspace/homeworks/h101/completion" && r.Method == "PUT" {
					completions++
					var body struct {
						Completed bool `json:"completed"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if !body.Completed {
						t.Error("submitted work must be completed")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"completed": true, "completedAt": "2026-09-01T00:00:00Z"})
					return
				}
				if r.Method != "GET" {
					creates++
					http.Error(w, "unexpected create", http.StatusBadRequest)
					return
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
				pages++
				if r.URL.Path != "/api/community/section-homeworks" || r.URL.Query().Get("sectionId") != "7" || r.URL.Query().Get("includeDeleted") != "false" || page < 1 || size < 1 || size > 100 {
					http.Error(w, "invalid query", http.StatusBadRequest)
					return
				}
				if denied && page == 2 {
					http.Error(w, "denied", http.StatusForbidden)
					return
				}
				rows := []map[string]any{}
				for id := (page-1)*size + 1; id <= page*size && id <= 101; id++ {
					rows = append(rows, map[string]any{"id": fmt.Sprintf("h%d", id), "title": fmt.Sprintf("Assignment %d", id), "submissionDueAt": "2026-09-30T00:00:00Z", "completionRequired": true})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]int{"page": page, "pageSize": size, "total": 101, "totalPages": 2}})
			}))
			client, err := api.NewTypedClient(server.URL, false)
			if err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			rows, err := fetchLifeHomeworksForSection(cmd, client, "7")
			if denied {
				server.Close()
				if err == nil || rows != nil || completions != 0 || creates != 0 {
					t.Fatalf("partial failure: %v %v %d %d", rows, err, completions, creates)
				}
				continue
			}
			if err != nil {
				server.Close()
				t.Fatal(err)
			}
			item := ustcschool.HomeworkItem{Title: "Assignment 101", EndAt: "2026-09-30T00:00:00Z", Status: "submitted"}
			result, err := syncLifeHomework(cmd, client, "7", map[string]any{"id": 7}, item, rows, false)
			server.Close()
			if err != nil {
				t.Fatal(err)
			}
			if pages != 2 || creates != 0 || completions != 1 || result.Action != "matched" || result.CompletionAction != "updated" || result.LifeHomework["id"] != "h101" {
				t.Fatalf("incorrect reuse: pages=%d creates=%d completions=%d result=%+v", pages, creates, completions, result)
			}
		}
	})
}
