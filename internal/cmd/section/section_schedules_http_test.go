package section

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
)

const scheduleTeacherOneJSON = `{
	"id":11,"jwId":9011,"personId":1011,"code":"T11",
	"nameCn":"示例教师一","nameEn":"Teacher One",
	"namePrimary":"Teacher One","nameSecondary":"示例教师一","department":null
}`

const scheduleTeacherTwoJSON = `{
	"id":12,"jwId":9012,"personId":null,"code":null,
	"nameCn":"示例教师二","nameEn":null,
	"namePrimary":"示例教师二","nameSecondary":null,"department":null
}`

const sectionSchedulesJSON = `[{
	"id":101,"date":"2026-09-24T08:00:00+08:00","weekday":4,"weekIndex":4,
	"startTime":"09:00","endTime":"10:40","startUnit":1,"endUnit":2,
	"periods":2.5,"roomId":null,"room":null,"customPlace":"Teaching Building 1",
	"sectionId":301,"scheduleGroupId":201,
	"experiment":null,"exerciseClass":false,"lessonType":null,
	"teachers":[` + scheduleTeacherOneJSON + `,` + scheduleTeacherTwoJSON + `],
	"teacherParticipations":[
		{"teacher":` + scheduleTeacherOneJSON + `,"periods":2,"exerciseClass":false},
		{"teacher":` + scheduleTeacherTwoJSON + `,"periods":0.5,"exerciseClass":true}
	],
	"scheduleGroup":{
		"id":201,"jwId":9201,"no":1,"limitCount":60,"stdCount":45,
		"actualPeriods":32,"isDefault":true,"sectionId":301
	}
}]`

func TestSchedulesCommandUsesCanonicalRouteAndPreservesOutput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format string
		body   string
		status int
	}{
		{name: "array JSON", format: "json", body: sectionSchedulesJSON, status: http.StatusOK},
		{name: "array table", format: "table", body: sectionSchedulesJSON, status: http.StatusOK},
		{name: "empty JSON", format: "json", body: `[]`, status: http.StatusOK},
		{name: "empty table", format: "table", body: `[]`, status: http.StatusOK},
		{name: "not found", format: "json", body: `{"message":"Section not found"}`, status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/sections/12345/schedules" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				if r.URL.RawQuery != "" {
					t.Errorf("query = %q, want no pagination or filters", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			printed, err := runSchedulesCommand(t, server.URL, tc.format)
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
			if tc.status == http.StatusNotFound {
				const want = "GET /api/catalog/sections/12345/schedules → 404: Section not found"
				if err == nil || err.Error() != want {
					t.Fatalf("error = %v, want %q", err, want)
				}
				if printed != "" {
					t.Fatalf("failed command printed success output: %q", printed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if tc.format == "json" {
				var got, want []map[string]any
				if err := json.Unmarshal([]byte(printed), &got); err != nil {
					t.Fatalf("decode command output: %v\n%s", err, printed)
				}
				if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
					t.Fatal(err)
				}
				if len(want) > 0 {
					// The command already normalizes weekdays in JSON as well as tables.
					want[0]["weekday"] = "Thu"
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("JSON output = %#v, want %#v", got, want)
				}
				return
			}

			if tc.body == `[]` {
				if !strings.Contains(printed, "No results found.") {
					t.Fatalf("empty table output = %q", printed)
				}
				return
			}
			for _, want := range []string{"ID", "DAY", "START", "END", "PLACE", "101", "Thu", "09:00", "10:40", "Teaching Building 1"} {
				if !strings.Contains(printed, want) {
					t.Errorf("table output missing %q:\n%s", want, printed)
				}
			}
		})
	}
}

func TestSchedulesResponseDecodesArrayAndPaginatedContracts(t *testing.T) {
	page, pageSize := int64(2), int64(1)
	for _, tc := range []struct {
		name      string
		params    openapi.GetSectionSchedulesParams
		query     url.Values
		body      string
		wantTotal int
		wantPage  int
	}{
		{
			name: "array", params: openapi.GetSectionSchedulesParams{},
			query: url.Values{}, body: sectionSchedulesJSON,
		},
		{
			name: "paginated", params: openapi.GetSectionSchedulesParams{Page: &page, PageSize: &pageSize},
			query:     url.Values{"page": {"2"}, "pageSize": {"1"}},
			body:      `{"data":` + sectionSchedulesJSON + `,"pagination":{"page":2,"pageSize":1,"total":2,"totalPages":2}}`,
			wantTotal: 2, wantPage: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/sections/12345/schedules" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				if !reflect.DeepEqual(r.URL.Query(), tc.query) {
					t.Errorf("query = %v, want %v", r.URL.Query(), tc.query)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := api.NewTypedClient(server.URL, false)
			if err != nil {
				t.Fatal(err)
			}
			data, err := api.ParseResponse[openapi.SectionSchedulesReadResponseSchema](client.GetSectionSchedules(api.Ctx(), 12345, &tc.params))
			if err != nil {
				t.Fatal(err)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
			var want any
			if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(data, want) {
				t.Fatalf("decoded response = %#v, want %#v", data, want)
			}
			_, rows, total, gotPage := cmdutil.ExtractList(data)
			if len(rows) != 1 || rows[0]["id"] != float64(101) || total != tc.wantTotal || gotPage != tc.wantPage {
				t.Fatalf("list = %#v, total = %d, page = %d", rows, total, gotPage)
			}
		})
	}
}

func runSchedulesCommand(t *testing.T, server, format string) (string, error) {
	t.Helper()
	printed, err := os.CreateTemp(t.TempDir(), "schedules-output-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = printed.Close() }()
	previousStdout, previousOutput := os.Stdout, output.Current
	os.Stdout = printed
	output.Current = &output.Opts{Format: format, NoColor: true}
	defer func() {
		os.Stdout = previousStdout
		output.Current = previousOutput
	}()

	root := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("server", server, "")
	root.AddCommand(NewCmdSection())
	root.SetArgs([]string{"section", "schedules", "12345"})
	commandErr := root.Execute()
	if _, err := printed.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(printed)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), commandErr
}
