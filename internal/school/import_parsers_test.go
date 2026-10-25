package school

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func importParserResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    request,
	}
}

func TestJWScoreImportRequestsAndCLIProjection(t *testing.T) {
	t.Parallel()
	const scoreResponse = `{
		"overview":{"passedCredits":10,"notPassedCredits":2,"gpa":3,"weightedScore":80,"arithmeticScore":81},
		"stdGradeRank":{"startSemesterName":"Start","endSemesterName":"End","gpa":2.5,"grade":"2024","majorName":"Math","majorRank":3,"majorStdCount":10},
		"semesters":[
			{"id":101,"scores":[{"courseName":" Zeta ","courseCode":" COURSE ","lessonCode":" 001 ","credits":0,"course":{"credits":4},"scoreCh":" A+ ","gradeDetail":" retake "}]},
			{"id":303,"scores":[
				{"semesterId":505,"semesterAssoc":404,"semesterName":" Override ","courseName":" Beta ","courseNameCh":"ignored","lessonCode":"B.01","credits":3,"course":{"credits":8},"gp":4.3,"scoreCh":" ","score":" 95 ","scoreText":" Details "},
				{"semesterAssoc":404,"semesterCh":" Display ","courseNameCh":" Alpha ","course":{"code":" INNER ","nameZh":"ignored","credits":2},"gp":2.5,"scoreCh":" 80 "},
				{"semester":{"semesterCn":"ignored"},"course":{"nameZh":" Gamma ","code":" G ","credits":5},"scoreText":" Pass "}
			]}
		]
	}`
	var requests []string
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String())
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/json, text/plain, */*" || r.Header.Get("User-Agent") != schoolUserAgent {
			t.Fatalf("unexpected score request: method=%s headers=%v", r.Method, r.Header)
		}
		var body string
		switch r.URL.Path {
		case "/for-std/grade/sheet/getSemesters":
			body = `[{"id":101,"semesterCn":" 旧学期 "},{"id":303,"semesterEn":"Current"},{"id":202,"code":"CODE"}]`
		case "/for-std/grade/sheet/getGradeSheetTypes":
			body = `[{"id":7},{"id":9}]`
		case "/for-std/grade/sheet/getGradeList":
			body = scoreResponse + ` {"ignored":true}`
		default:
			t.Fatalf("unexpected score URL: %s", r.URL)
		}
		return importParserResponse(r, http.StatusOK, body), nil
	})}
	report, err := fetchJWScoreReport(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	wantRequests := []string{
		"https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters",
		"https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeSheetTypes",
		"https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeList?semesterIds=101%2C303%2C202&trainTypeId=7",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("score requests = %v, want %v", requests, wantRequests)
	}
	wantItems := []ScoreItem{
		{SemesterID: 505, SemesterName: "Override", CourseName: "Beta", LessonCode: "B.01", Credits: 3, GradePoint: 4.3, Score: "95", GradeText: "Details"},
		{SemesterID: 404, SemesterName: "Display", CourseName: "Alpha", CourseCode: "INNER", Credits: 2, GradePoint: 2.5, Score: "80"},
		{SemesterID: 303, SemesterName: "Current", CourseName: "Gamma", CourseCode: "G", Credits: 5, Score: "Pass", GradeText: "Pass"},
		{SemesterID: 101, SemesterName: "旧学期", CourseName: "Zeta", LessonCode: " 001 ", CourseCode: "COURSE", Credits: 4, Score: "A+", GradeText: "retake"},
	}
	if !reflect.DeepEqual(report.Items, wantItems) {
		t.Fatalf("score items = %#v, want %#v", report.Items, wantItems)
	}
	var summary map[string]any
	if err := json.Unmarshal(report.Summary, &summary); err != nil {
		t.Fatal(err)
	}
	wantSummary := map[string]any{
		"totalCredits": float64(12), "earnedCredits": float64(10), "failedCredits": float64(2),
		"gpa": float64(3), "weightedAverage": float64(80), "averageScore": float64(81),
		"fromSemester": "Start", "toSemester": "End", "periodGPA": float64(2.5),
		"ranking": "全校2024年级Math专业 GPA 排名 3/10",
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("score summary = %#v, want %#v", summary, wantSummary)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 || output["summary"] == nil || output["items"] == nil {
		t.Fatalf("CLI report fields changed: %s", encoded)
	}
	var items []map[string]any
	if err := json.Unmarshal(output["items"], &items); err != nil {
		t.Fatal(err)
	}
	if items[0]["semesterId"] != float64(505) || items[0]["gradePoint"] != 4.3 {
		t.Fatalf("CLI numeric fields changed: %s", encoded)
	}
	if _, present := items[2]["gradePoint"]; present {
		t.Fatalf("zero grade point must retain omitempty: %s", encoded)
	}
}

func TestJWScoreImportEmptySelectionsAndSummary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, semesters, types, body, wantError, summary string
		requestCount                                     int
	}{
		{name: "no semesters", semesters: `[]`, wantError: "jw returned no score semesters", requestCount: 1},
		{name: "no transcript types", semesters: `[{"id":301}]`, types: `[]`, wantError: "jw returned no score transcript types", requestCount: 2},
		{name: "empty scores", semesters: `[{"id":301}]`, types: `[{"id":7}]`, body: `{}`, requestCount: 3},
		{name: "null scores", semesters: `[{"id":301}]`, types: `[{"id":7}]`, body: `null`, requestCount: 3},
		{name: "zero overview", semesters: `[{"id":301}]`, types: `[{"id":7}]`, body: `{"overview":{"passedCredits":0,"gpa":0}}`, requestCount: 3},
		{name: "empty rank remains present", semesters: `[{"id":301}]`, types: `[{"id":7}]`, body: `{"stdGradeRank":{}}`, summary: `{"fromSemester":"","periodGPA":0,"toSemester":""}`, requestCount: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				var body string
				switch r.URL.Path {
				case "/for-std/grade/sheet/getSemesters":
					body = tc.semesters
				case "/for-std/grade/sheet/getGradeSheetTypes":
					body = tc.types
				case "/for-std/grade/sheet/getGradeList":
					body = tc.body
				default:
					t.Fatalf("unexpected score URL: %s", r.URL)
				}
				return importParserResponse(r, http.StatusOK, body), nil
			})}
			report, err := fetchJWScoreReport(context.Background(), client)
			if requests != tc.requestCount {
				t.Fatalf("score request count = %d, want %d", requests, tc.requestCount)
			}
			if tc.wantError != "" {
				if err == nil || err.Error() != tc.wantError || !reflect.DeepEqual(report, ScoreReport{}) {
					t.Fatalf("score result = %#v, %v; want empty report, %q", report, err, tc.wantError)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(report.Items, []ScoreItem{}) || string(report.Summary) != tc.summary {
				t.Fatalf("score result = %#v, %v; want empty items and summary %q", report, err, tc.summary)
			}
			if tc.summary == "" {
				encoded, err := json.Marshal(report)
				if err != nil || string(encoded) != `{"items":[]}` {
					t.Fatalf("empty CLI score report = %s, %v", encoded, err)
				}
			}
		})
	}
}

func TestJWExamImportHTTPProjectionAndEmptyTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       []ExamItem
	}{
		{
			name: "status table fields and ordering",
			body: `<table><tbody>
				<tr><td>Zeta</td><td>Z.01</td><td> Final </td><td>2026-12-01 09:00</td><td> Room 2 </td><td> 18 </td><td> Pending </td><td>ignored detail</td></tr>
				<tr><td> <span>Math</span> Theory </td><td>M.01</td><td>Midterm</td><td>2026-11-01 09:00</td><td>Room 1</td><td>27</td><td>Ready</td></tr>
				<tr><td>Alpha</td><td>A.01</td><td>Quiz</td><td>2026-11-01 09:00</td></tr>
				<tr><td></td><td>ignored</td><td>Quiz</td><td>earlier</td></tr>
			</tbody></table>`,
			want: []ExamItem{
				{CourseName: "Alpha", LessonCode: "A.01", ExamType: "Quiz", DateTime: "2026-11-01 09:00"},
				{CourseName: "Math Theory", LessonCode: "M.01", ExamType: "Midterm", DateTime: "2026-11-01 09:00", Location: "Room 1", Seat: "27", Status: "Ready"},
				{CourseName: "Zeta", LessonCode: "Z.01", ExamType: "Final", DateTime: "2026-12-01 09:00", Location: "Room 2", Seat: "18", Status: "Pending"},
			},
		},
		{name: "empty table", body: `<table><tbody></tbody></table>`, want: []ExamItem{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != http.MethodGet || r.URL.String() != "https://jw.ustc.edu.cn/for-std/exam-arrange" {
					t.Fatalf("unexpected exam request: %s %s", r.Method, r.URL)
				}
				return importParserResponse(r, http.StatusOK, tc.body), nil
			})}
			got, err := fetchJWExams(context.Background(), client)
			if err != nil || requests != 1 || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("exams = %#v, %v; requests=%d, want %#v", got, err, requests, tc.want)
			}
		})
	}
}

func TestBlackboardHomeworkImportAssociationsAndOrdering(t *testing.T) {
	t.Parallel()
	var requests []string
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String())
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected Blackboard method: %s", r.Method)
		}
		var body, accept string
		switch r.URL.Path {
		case "/webapps/calendar/calendarData/calendars":
			accept = "application/json"
			body = `{"calendars":[{"id":"MATH.01.2026FA"},{"id":"PERSONAL"},{"id":"CS.01.2026FA"},{"id":"MATH.01.2026FA"},{"id":"INSTITUTION"}]}`
		case "/webapps/portal/execute/tabs/tabAction":
			accept = "text/html"
			body = `<a href="/webapps/blackboard/execute/launcher?type=Course&amp;id=_123_1">CS.01.2026FA: Computing</a><a href="/webapps/blackboard/execute/launcher?type=Course&amp;id=_123_1">MATH.01.2026FA: Math</a>`
		case "/webapps/calendar/calendarData/events":
			accept = "application/json"
			body = `[
				{"id":"later","calendarId":"CS.01.2026FA","itemSourceId":"_10_1","title":"Late assignment","calendarName":"Computing","start":" raw start ","end":"2026-12-01T00:00:00Z","eventType":"Assignment"},
				{"id":"early","calendarId":"MATH.01.2026FA","itemSourceId":"_20_1","title":"Early assignment","calendarName":"Zeta","start":"2026-10-20T08:00:00+08:00","end":"2026-11-01T00:00:00Z","eventType":"Not started"},
				{"id":"duplicate","calendarId":"CS.01.2026FA","title":"Old version","calendarName":"Discarded","start":"old","end":"old","eventType":"Old"},
				{"id":"duplicate","calendarId":"MATH.01.2026FA","title":"Final version","calendarName":"Alpha","start":" 2026-10-21T08:00:00+08:00 ","end":"2026-11-01T00:00:00Z","eventType":"New"}
			] {"ignored":true}`
		case "/webapps/gradebook/do/student/viewGrades":
			accept = "text/html"
			body = `<div id="grades_wrapper"><div id="10" class="sortable_item_row"><div class="cell activity"><span class="activityType"> Needs
			Grading </span></div><div class="cell gradeStatus">ignored fallback</div></div></div>`
		default:
			t.Fatalf("unexpected Blackboard URL: %s", r.URL)
		}
		if r.Header.Get("Accept") != accept {
			t.Fatalf("Blackboard Accept = %q, want %q", r.Header.Get("Accept"), accept)
		}
		return importParserResponse(r, http.StatusOK, body), nil
	})}
	items, err := fetchBlackboardHomework(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	wantRequests := []string{
		"https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/calendars?mode=personal&course_id=",
		"https://www.bb.ustc.edu.cn/webapps/portal/execute/tabs/tabAction?tab_tab_group_id=_1_1",
		"https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/events?start=0&end=4102444800000&course_id=&calendarIds=CS.01.2026FA,MATH.01.2026FA",
		"https://www.bb.ustc.edu.cn/webapps/gradebook/do/student/viewGrades?course_id=_123_1",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("Blackboard requests = %v, want %v", requests, wantRequests)
	}
	wantItems := []HomeworkItem{
		{ID: "duplicate", Title: "Final version", CourseName: "Alpha", LessonCode: "MATH.01", SemesterCode: "2026FA", StartAt: " 2026-10-21T08:00:00+08:00 ", EndAt: "2026-11-01T00:00:00Z", Status: "New"},
		{ID: "early", Title: "Early assignment", CourseName: "Zeta", LessonCode: "MATH.01", SemesterCode: "2026FA", ExternalItemID: "_20_1", StartAt: "2026-10-20T08:00:00+08:00", EndAt: "2026-11-01T00:00:00Z", Status: "Not started"},
		{ID: "later", Title: "Late assignment", CourseName: "Computing", LessonCode: "CS.01", SemesterCode: "2026FA", ExternalItemID: "_10_1", StartAt: " raw start ", EndAt: "2026-12-01T00:00:00Z", Status: "Needs Grading"},
	}
	if !reflect.DeepEqual(items, wantItems) {
		t.Fatalf("Blackboard homework = %#v, want %#v", items, wantItems)
	}
}

func TestBlackboardHomeworkEmptyCalendarImport(t *testing.T) {
	t.Parallel()
	var paths []string
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/webapps/calendar/calendarData/calendars":
			return importParserResponse(r, http.StatusOK, `{"calendars":[]}`), nil
		case "/webapps/portal/execute/tabs/tabAction":
			return importParserResponse(r, http.StatusOK, ""), nil
		default:
			t.Fatalf("empty calendar import requested %s", r.URL)
			return nil, nil
		}
	})}
	items, err := fetchBlackboardHomework(context.Background(), client)
	wantPaths := []string{"/webapps/calendar/calendarData/calendars", "/webapps/portal/execute/tabs/tabAction"}
	if err != nil || !reflect.DeepEqual(items, []HomeworkItem{}) || !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("empty Blackboard import = %#v, %v; paths=%v", items, err, paths)
	}
}

type importParserReadError struct {
	err error
}

func (reader importParserReadError) Read([]byte) (int, error) { return 0, reader.err }

func TestSchoolImportTransportAndDecodeFailures(t *testing.T) {
	t.Parallel()
	responses := map[string]string{
		"/for-std/grade/sheet/getSemesters":        `[{"id":301}]`,
		"/for-std/grade/sheet/getGradeSheetTypes":  `[{"id":7}]`,
		"/for-std/grade/sheet/getGradeList":        `{}`,
		"/for-std/exam-arrange":                    `<table><tbody></tbody></table>`,
		"/webapps/calendar/calendarData/calendars": `{"calendars":[{"id":"CS.01.2026FA"}]}`,
		"/webapps/portal/execute/tabs/tabAction":   `<a href="/webapps/blackboard/execute/launcher?type=Course&amp;id=_123_1">CS.01.2026FA: Computing</a>`,
		"/webapps/calendar/calendarData/events":    `[{"id":"item","calendarId":"CS.01.2026FA","itemSourceId":"_10_1","eventType":"Assignment"}]`,
		"/webapps/gradebook/do/student/viewGrades": `<div id="grades_wrapper"></div>`,
	}
	scores := func(client *http.Client) error {
		_, err := fetchJWScoreReport(context.Background(), client)
		return err
	}
	exams := func(client *http.Client) error {
		_, err := fetchJWExams(context.Background(), client)
		return err
	}
	homework := func(client *http.Client) error {
		_, err := fetchBlackboardHomework(context.Background(), client)
		return err
	}
	for _, endpoint := range []struct {
		name, path, networkPrefix, readPrefix, decodePrefix, wrongType string
		fetch                                                          func(*http.Client) error
	}{
		{name: "score list", path: "/for-std/grade/sheet/getGradeList", networkPrefix: "fetch score list:", readPrefix: "fetch score list: decode https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeList?semesterIds=301&trainTypeId=7:", decodePrefix: "fetch score list: decode https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeList?semesterIds=301&trainTypeId=7:", wrongType: `{"semesters":[{"scores":[{"credits":"bad"}]}]}`, fetch: scores},
		{name: "exams", path: "/for-std/exam-arrange", networkPrefix: "fetch exams:", readPrefix: "parse exams page:", fetch: exams},
		{name: "Blackboard calendars", path: "/webapps/calendar/calendarData/calendars", networkPrefix: "fetch Blackboard calendars:", readPrefix: "read Blackboard calendars:", decodePrefix: "decode Blackboard calendars:", wrongType: `{"calendars":[{"id":42}]}`, fetch: homework},
		{name: "Blackboard portal", path: "/webapps/portal/execute/tabs/tabAction", networkPrefix: "fetch Blackboard portal courses:", readPrefix: "parse Blackboard portal courses:", fetch: homework},
		{name: "Blackboard events", path: "/webapps/calendar/calendarData/events", networkPrefix: "fetch Blackboard homework:", readPrefix: "decode Blackboard homework:", decodePrefix: "decode Blackboard homework:", wrongType: `[{"title":true}]`, fetch: homework},
		{name: "Blackboard grades", path: "/webapps/gradebook/do/student/viewGrades", networkPrefix: "fetch Blackboard grades:", fetch: homework},
	} {
		failures := []string{"network", "HTTP status"}
		if endpoint.readPrefix != "" {
			failures = append(failures, "body read")
		}
		if endpoint.decodePrefix != "" {
			failures = append(failures, "malformed JSON", "wrong JSON type")
		}
		for _, failure := range failures {
			t.Run(endpoint.name+"/"+failure, func(t *testing.T) {
				sentinel := errors.New("import fixture unavailable")
				targetURL := ""
				client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
					body, exists := responses[r.URL.Path]
					if !exists {
						t.Fatalf("unexpected import URL: %s", r.URL)
					}
					if r.URL.Path != endpoint.path {
						return importParserResponse(r, http.StatusOK, body), nil
					}
					targetURL = r.URL.String()
					switch failure {
					case "network":
						return nil, sentinel
					case "HTTP status":
						return importParserResponse(r, http.StatusServiceUnavailable, "unavailable"), nil
					case "body read":
						response := importParserResponse(r, http.StatusOK, "")
						response.Body = io.NopCloser(importParserReadError{err: sentinel})
						return response, nil
					case "malformed JSON":
						return importParserResponse(r, http.StatusOK, "{"), nil
					case "wrong JSON type":
						return importParserResponse(r, http.StatusOK, endpoint.wrongType), nil
					default:
						t.Fatalf("unknown failure: %s", failure)
						return nil, nil
					}
				})}
				err := endpoint.fetch(client)
				if targetURL == "" || err == nil {
					t.Fatalf("failure did not reach %s: target=%q err=%v", endpoint.path, targetURL, err)
				}
				var wantPrefix string
				switch failure {
				case "network":
					wantPrefix = endpoint.networkPrefix
				case "body read":
					wantPrefix = endpoint.readPrefix
				case "HTTP status":
					want := "GET " + targetURL + " returned 503 Service Unavailable: unavailable"
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("HTTP error = %v, want context %q", err, want)
					}
				case "malformed JSON", "wrong JSON type":
					wantPrefix = endpoint.decodePrefix
				}
				if wantPrefix != "" && !strings.HasPrefix(err.Error(), wantPrefix) {
					t.Fatalf("import error = %v, want prefix %q", err, wantPrefix)
				}
				if (failure == "network" || failure == "body read") && !errors.Is(err, sentinel) {
					t.Fatalf("import error lost cause: %v", err)
				}
			})
		}
	}
}

func TestBlackboardGradeReadFailureRetainsEmptyStatus(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("grade fixture read failed")
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://www.bb.ustc.edu.cn/webapps/gradebook/do/student/viewGrades?course_id=_123_1" || r.Header.Get("Accept") != "text/html" {
			t.Fatalf("unexpected grade request: %s headers=%v", r.URL, r.Header)
		}
		response := importParserResponse(r, http.StatusOK, "")
		response.Body = io.NopCloser(importParserReadError{err: sentinel})
		return response, nil
	})}
	statuses, err := fetchBlackboardGradeStatuses(context.Background(), client, "_123_1")
	if err != nil || statuses != nil {
		t.Fatalf("unreadable grade page = %#v, %v; want nil status map without error", statuses, err)
	}
}
