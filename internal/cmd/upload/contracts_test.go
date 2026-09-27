package upload

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

func uploadContractServer(t *testing.T, denied bool) (*httptest.Server, *int) {
	t.Helper()
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if r.Method != "GET" || r.URL.Path != "/api/workspace/uploads" || r.Header.Get("Authorization") != "Bearer owner" || page < 1 || size < 1 || size > 100 {
			http.Error(w, "invalid query", http.StatusBadRequest)
			return
		}
		if denied && page == 2 {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		rows := []map[string]any{}
		for id := (page-1)*size + 1; id <= page*size && id <= 101; id++ {
			rows = append(rows, map[string]any{"id": fmt.Sprintf("u%d", id), "filename": fmt.Sprintf("File-%d.txt", id), "key": fmt.Sprintf("key%d", id), "size": id, "createdAt": "2026-09-01T00:00:00Z"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows, "pagination": map[string]int{"page": page, "pageSize": size, "total": 101, "totalPages": 2}, "meta": map[string]int{"usedBytes": 5151, "quotaBytes": 10000, "maxFileSizeBytes": 999}})
	}))
	t.Cleanup(server.Close)
	if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
		t.Fatal(err)
	}
	return server, &pages
}

func captureUploadContract(t *testing.T, format string, run func() error) (string, error) {
	t.Helper()
	old, opts := os.Stdout, output.Current
	output.Current = &output.Opts{Format: format, NoColor: true}
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

func TestSpecUploadCompleteList(t *testing.T) {
	t.Run("upload.cli-complete-list", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, format := range []string{"json", "table"} {
			for _, denied := range []bool{false, true} {
				server, pages := uploadContractServer(t, denied)
				cmd := NewCmdUpload()
				cmd.PersistentFlags().String("server", server.URL, "")
				cmd.SetArgs([]string{})
				cmd.SilenceErrors, cmd.SilenceUsage = true, true
				text, err := captureUploadContract(t, format, cmd.Execute)
				if *pages != 2 {
					t.Fatalf("pages=%d", *pages)
				}
				if denied {
					if err == nil || text != "" {
						t.Fatalf("partial list %q %v", text, err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if format == "json" {
					var body struct {
						Data []struct {
							ID string `json:"id"`
						}
						Meta       struct{ UsedBytes int }
						Pagination struct{ Total int }
					}
					if err := json.Unmarshal([]byte(text), &body); err != nil {
						t.Fatal(err)
					}
					if len(body.Data) != 101 || body.Data[100].ID != "u101" || body.Meta.UsedBytes != 5151 || body.Pagination.Total != 101 {
						t.Fatalf("incomplete uploads %s", text)
					}
				} else if !strings.Contains(text, "File-101.txt") || !strings.Contains(text, "Usage:") || strings.Contains(text, "Type") {
					t.Fatalf("wrong table: %s", text)
				}
			}
		}
	})
}

func TestSpecUploadPicker(t *testing.T) {
	t.Run("upload.cli-picker-complete-input", func(t *testing.T) {
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		server, pages := uploadContractServer(t, false)
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		cmd.PersistentFlags().String("server", server.URL, "")
		input, err := os.CreateTemp(t.TempDir(), "choice")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = input.Close() }()
		_, _ = input.WriteString("101\n")
		_, _ = input.Seek(0, 0)
		old := os.Stdin
		os.Stdin = input
		defer func() { os.Stdin = old }()
		var row map[string]any
		text, err := captureUploadContract(t, "table", func() error { var err error; row, err = promptUploadPick(cmd, "Select upload"); return err })
		if err != nil {
			t.Fatal(err)
		}
		if *pages != 2 || row["id"] != "u101" || row["filename"] != "File-101.txt" || !strings.Contains(text, "File-101.txt") {
			t.Fatalf("picker incomplete row=%v text=%s", row, text)
		}
	})
}
