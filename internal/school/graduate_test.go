package school

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	graduateTestScheduleApp  = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/xskbcx"
	graduateTestScheduleTerm = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/modules/xskbcx/xnxqxxcx.do"
	graduateTestScheduleRows = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/modules/xskbcx/xskbxxcx.do"
	graduateTestExamApp      = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/xskssbcx"
	graduateTestExamTerm     = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/modules/xskssbcx/hqxnxq.do"
	graduateTestExamRows     = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/modules/xskssbcx/xscxkssbxx.do"
	graduateTestScoreApp     = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/wdcj"
	graduateTestScoreInfo    = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/queryInfo.do"
	graduateTestScoreProfile = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/hqxh.do"
	graduateTestScoreRows    = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/xscjcx.do"
	graduateTestHomeworkApp  = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/wdzy"
	graduateTestHomeworkTerm = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/modules/wdzy/hqxnxq.do"
	graduateTestHomeworkRows = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/modules/wdzy/wdzycx.do"
	graduateTestTerms        = `{"code":"0","datas":{"terms":{"rows":[{"DM":"20251","MC":"2025年秋季学期","SFDQXQ":"0","QSSJ":"2025-09-01","JZSJ":"2026-01-20"},{"DM":"20252","SFDQXQ":"1","QSSJ":"2026-02-23","JZSJ":"2026-07-05"}]}}}`
)

type graduateRequestFixture struct {
	url, referer, body      string
	form                    url.Values
	get                     bool
	status                  int
	networkError, readError error
}

func graduateFixtureClient(t *testing.T, fixtures []graduateRequestFixture) *http.Client {
	t.Helper()
	calls := 0
	t.Cleanup(func() {
		if calls != len(fixtures) {
			t.Errorf("graduate requests = %d, want %d", calls, len(fixtures))
		}
	})
	return &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		if calls >= len(fixtures) {
			t.Fatalf("unexpected graduate request: %s %s", r.Method, r.URL)
		}
		fixture := fixtures[calls]
		calls++
		method := http.MethodPost
		if fixture.get {
			method = http.MethodGet
		}
		if r.Method != method || r.URL.String() != fixture.url || r.Header.Get("Referer") != fixture.referer || r.Header.Get("User-Agent") != schoolUserAgent {
			t.Fatalf("graduate request %d = %s %s headers=%v, want %s %s referer=%s", calls, r.Method, r.URL, r.Header, method, fixture.url, fixture.referer)
		}
		if !fixture.get {
			for name, want := range map[string]string{
				"Accept":           "application/json, text/javascript, */*; q=0.01",
				"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
				"Origin":           "https://yjs1.ustc.edu.cn",
				"X-Requested-With": "XMLHttpRequest",
			} {
				if r.Header.Get(name) != want {
					t.Fatalf("graduate header %s = %q, want %q", name, r.Header.Get(name), want)
				}
			}
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != fixture.form.Encode() {
				t.Fatalf("graduate form = %q, %v; want %q", body, err, fixture.form.Encode())
			}
		}
		if fixture.networkError != nil {
			return nil, fixture.networkError
		}
		status := fixture.status
		if status == 0 {
			status = http.StatusOK
		}
		response := importParserResponse(r, status, fixture.body)
		if fixture.readError != nil {
			response.Body = io.NopCloser(importParserReadError{err: fixture.readError})
		}
		return response, nil
	})}
}

func graduateScheduleFixtures(rows string) []graduateRequestFixture {
	return []graduateRequestFixture{
		{url: graduateTestScheduleTerm, referer: graduateTestScheduleApp, form: url.Values{"SFSY": {"1"}, "pageSize": {"100"}, "pageNumber": {"1"}}, body: graduateTestTerms},
		{url: graduateTestScheduleRows, referer: graduateTestScheduleApp, form: url.Values{"querySetting": {`[{"name":"XNXQDM","builder":"equal","linkOpt":"AND","builderList":"cbl_String","value":"20252"}]`}, "pageSize": {"999"}, "pageNumber": {"1"}}, body: rows},
	}
}

func graduateExamFixtures(rows string) []graduateRequestFixture {
	return []graduateRequestFixture{
		{url: graduateTestExamTerm, referer: graduateTestExamApp, body: graduateTestTerms},
		{url: graduateTestExamRows, referer: graduateTestExamApp, form: url.Values{"querySetting": {`[{"name":"XNXQDM","caption":"学年学期","builder":"m_value_equal","linkOpt":"AND","value":"20252"},{"name":"SBZT","caption":"申报状态","builder":"equal","linkOpt":"AND","value":"1"}]`}, "pageSize": {"999"}, "pageNumber": {"1"}}, body: rows},
	}
}

func graduateScoreFixtures(info, profile, rows string) []graduateRequestFixture {
	return []graduateRequestFixture{
		{url: graduateTestScoreInfo, referer: graduateTestScoreApp, body: info},
		{url: graduateTestScoreProfile, referer: graduateTestScoreApp, body: profile},
		{url: graduateTestScoreRows, referer: graduateTestScoreApp, form: url.Values{"pageSize": {"999"}, "pageNumber": {"1"}}, body: rows},
	}
}

func graduateHomeworkFixtures(rows string) []graduateRequestFixture {
	return []graduateRequestFixture{
		{url: graduateTestHomeworkTerm, referer: graduateTestHomeworkApp, form: url.Values{"*order": {"-XNDM,-XQDM"}}, body: graduateTestTerms},
		{url: graduateTestHomeworkApp, referer: graduateTestHomeworkApp, get: true, body: `<script>var student={"USERID":"PB +&/1"}</script>`},
		{url: graduateTestHomeworkRows, referer: graduateTestHomeworkApp, form: url.Values{"XH": {"PB +&/1"}, "querySetting": {`[{"name":"XNXQDM","builder":"m_value_equal","value":"20252"}]`}, "pageSize": {"999"}, "pageNumber": {"1"}}, body: rows},
	}
}

func graduateTestClient(client *http.Client) *Client {
	c := NewClient(Credentials{}, ProgramGraduate)
	c.graduateScheduleClient = client
	return c
}

func TestGraduateYJS1ScheduleRequestsCacheAndOutput(t *testing.T) {
	t.Parallel()
	fixtures := graduateScheduleFixtures(`{"datas":{"schedule":{"rows":[
		{"WID":"abc-row","BJMC":" SA25113014.01 ","KCDM":" Y0203220 ","KCMC":" 软件体系结构分析与设计 ","PKSJDD":"周一 3-4节 软件楼101; 周三 7-8节 软件楼102","RKJS":"张三、李四、张三","ZCMC":"1~8; 9~16"},
		{"WID":"42-extra","BJMC":" A.01 ","KCDM":" MATH ","KCMC":" Math  Course ","PKSJDD":" 周四 1-2节 ","RKJS":" B / A ","ZCMC":"1~16"}
	]}}} {"ignored":true}`)
	fixtures[0].body += ` {"ignored":true}`
	c := graduateTestClient(graduateFixtureClient(t, fixtures))
	semesters, err := c.FetchSemesters(context.Background())
	wantSemesters := []Semester{
		{ID: 20251, Code: "20251", SemesterCn: "2025年秋季学期", SemesterEn: "2025年秋季学期", StartDate: "2025-09-01", EndDate: "2026-01-20"},
		{ID: 20252, Code: "20252", SemesterCn: "2026年春季学期", SemesterEn: "2026年春季学期", StartDate: "2026-02-23", EndDate: "2026-07-05", IsLast: true},
	}
	if err != nil || !reflect.DeepEqual(semesters, wantSemesters) {
		t.Fatalf("graduate semesters = %#v, %v; want %#v", semesters, err, wantSemesters)
	}
	selected, items, err := c.FetchCurriculum(context.Background(), 0)
	wantItems := []CurriculumItem{
		{SemesterID: 20252, LessonID: 42, LessonCode: "A.01", CourseCode: "MATH", CourseName: "Math Course", Teachers: []string{"A", "B"}, Schedule: "周四 1-2节 [1-16周]"},
		{SemesterID: 20252, LessonCode: "SA25113014.01", CourseCode: "Y0203220", CourseName: "软件体系结构分析与设计", Teachers: []string{"张三", "李四"}, Schedule: "周一 3-4节 软件楼101 [1-8周]; 周三 7-8节 软件楼102 [9-16周]"},
	}
	if err != nil || selected != wantSemesters[1] || !reflect.DeepEqual(items, wantItems) {
		t.Fatalf("graduate curriculum = %#v, %#v, %v; want %#v", selected, items, err, wantItems)
	}
	again, err := c.FetchSemesters(context.Background())
	if err != nil || !reflect.DeepEqual(again, wantSemesters) {
		t.Fatalf("cached graduate semesters = %#v, %v", again, err)
	}
}

func TestGraduateYJS1SemesterSelectionAndEmptyCache(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		requested           int
		current             bool
		wantCode, wantError string
	}{
		{name: "current", current: true, wantCode: "20252"},
		{name: "first source term", wantCode: "20251"},
		{name: "explicit older term", requested: 20251, current: true, wantCode: "20251"},
		{name: "missing term", requested: 17, current: true, wantError: "graduate semester 17 was not found"},
		{name: "negative is explicit", requested: -1, current: true, wantError: "graduate semester -1 was not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixtures := graduateScheduleFixtures(`{}`)
			if !tc.current {
				fixtures[0].body = strings.ReplaceAll(fixtures[0].body, `"SFDQXQ":"1"`, `"SFDQXQ":"0"`)
			}
			if tc.wantError != "" {
				fixtures = fixtures[:1]
			} else {
				fixtures[1].form.Set("querySetting", strings.ReplaceAll(fixtures[1].form.Get("querySetting"), "20252", tc.wantCode))
			}
			selected, items, err := graduateTestClient(graduateFixtureClient(t, fixtures)).FetchCurriculum(context.Background(), tc.requested)
			if tc.wantError != "" {
				if err == nil || err.Error() != tc.wantError || selected != (Semester{}) || items != nil {
					t.Fatalf("selection = %#v, %#v, %v", selected, items, err)
				}
			} else if err != nil || selected.Code != tc.wantCode || items == nil || len(items) != 0 {
				t.Fatalf("selection = %#v, %#v, %v", selected, items, err)
			}
		})
	}
	for _, tc := range []struct {
		name, body string
		cached     bool
	}{
		{"missing datas", `{}`, false}, {"null response", `null`, false}, {"null rows", `{"datas":{"x":{"rows":null}}}`, false}, {"empty rows", `{"datas":{"x":{"rows":[]}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := graduateScheduleFixtures("")[0]
			fixture.body = tc.body
			fixtures := []graduateRequestFixture{fixture}
			if !tc.cached {
				fixtures = append(fixtures, fixture, fixture)
			}
			c := graduateTestClient(graduateFixtureClient(t, fixtures))
			for range 2 {
				items, err := c.FetchSemesters(context.Background())
				encoded, _ := json.Marshal(items)
				if err != nil || string(encoded) != "[]" {
					t.Fatalf("empty semesters = %s, %v", encoded, err)
				}
			}
			_, items, err := c.FetchCurriculum(context.Background(), 0)
			if err == nil || err.Error() != "no graduate semesters available" || items != nil {
				t.Fatalf("empty curriculum = %#v, %v", items, err)
			}
		})
	}
}

func TestGraduateYJS1ScoreRequestsAndOutput(t *testing.T) {
	t.Parallel()
	fixtures := graduateScoreFixtures(
		`{"datas":{"info":{"rows":[{"XH":" info-id ","PYCCDM_DISPLAY":"硕士","ZYDM_DISPLAY":" 信息专业 ","YXDM_DISPLAY":"信息学院","DSXM":" 信息导师 ","NJDM_DISPLAY":"2025","ZCZT_DISPLAY":"在籍","YJBYSJ":"2028-06"},{"XH":"ignored"}]}}} {"ignored":true}`,
		`{"code":"0","XH":" profile-id ","xscjxx":{"PYCCDM_DISPLAY":" 博士 ","ZYDM_DISPLAY":" 优先专业 ","PJJDZ":" 3.7 "}} {"ignored":true}`,
		`{"datas":{"scores":{"rows":[{"XNXQDM":"20251","KCDM":" OLD ","KCMC":"Old","CJ":"通过","JDZ":null},{"XNXQDM":"20252","XNXQDM_DISPLAY":"2026年春季学期","BJMC":"SA25113014.01","KCDM":"Y0203220","KCMC":" 软件体系结构分析与设计 ","CJFZDM_DISPLAY":"百分制","CJJL_DISPLAY":"正常考试","XF":2,"CJ":87,"JDZ":3.7},{"XNXQDM":"20252","KCDM":"ZERO","KCMC":"A","CJFZDM_DISPLAY":"通过","CJJL_DISPLAY":"通过","XF":"0","CJ":0,"JDZ":"bad"}]}}} {"ignored":true}`)
	report, err := fetchGraduateYJS1ScoreReport(context.Background(), graduateFixtureClient(t, fixtures))
	want := []ScoreItem{
		{SemesterID: 20252, SemesterName: "2026年春季学期", LessonCode: "ZERO", CourseCode: "ZERO", CourseName: "A", Score: "0", GradeText: "通过"},
		{SemesterID: 20252, SemesterName: "2026年春季学期", LessonCode: "SA25113014.01", CourseCode: "Y0203220", CourseName: "软件体系结构分析与设计", Credits: 2, GradePoint: 3.7, Score: "87", GradeText: "百分制 / 正常考试"},
		{SemesterID: 20251, SemesterName: "2025年秋季学期", LessonCode: "OLD", CourseCode: "OLD", CourseName: "Old", Score: "通过"},
	}
	if err != nil || !reflect.DeepEqual(report.Items, want) {
		t.Fatalf("graduate scores = %#v, %v; want %#v", report, err, want)
	}
	var summary map[string]any
	if err := json.Unmarshal(report.Summary, &summary); err != nil {
		t.Fatal(err)
	}
	wantSummary := map[string]any{"studentId": "profile-id", "program": "博士", "major": "优先专业", "department": "信息学院", "advisor": "信息导师", "grade": "2025", "status": "在籍", "expectedGraduation": "2028-06", "averageGradePoint": 3.7}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("graduate summary = %#v, want %#v", summary, wantSummary)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if _, ok := output.Items[0]["gradePoint"]; ok || output.Items[0]["score"] != "0" || output.Items[1]["gradePoint"] != 3.7 {
		t.Fatalf("numeric CLI output changed: %s", encoded)
	}
}

func TestGraduateYJS1ExamRequestsAndOutput(t *testing.T) {
	t.Parallel()
	fixtures := graduateExamFixtures(`{"datas":{"exams":{"rows":[{"BJMC":" B.01 ","KCDM":"ignored","KCMC":" Course B ","KSDD":" Room 1 ","KSKSSJ":" 2026-06-01 09:00 ","KSJSSJ":" 2026-06-01 11:00 ","KSXS":"raw","KSXS_DISPLAY":" 闭卷 ","SBZT":"1","SBZT_DISPLAY":" 已申报 ","ZWH":" 42 "},{"KCDM":" A.01 ","KCMC":"Course A","KSKSSJ":"2026-05-01","KSXS":"raw type","SBZT":"raw status"}]}}} {"ignored":true}`)
	items, err := fetchGraduateYJS1Exams(context.Background(), graduateFixtureClient(t, fixtures), 0)
	want := []ExamItem{
		{CourseName: "Course A", LessonCode: "A.01", DateTime: "2026-05-01", ExamType: "raw type", Status: "raw status"},
		{CourseName: "Course B", LessonCode: "B.01", DateTime: "2026-06-01 09:00 - 2026-06-01 11:00", Location: "Room 1", ExamType: "闭卷", Status: "已申报", Seat: "42"},
	}
	if err != nil || !reflect.DeepEqual(items, want) {
		t.Fatalf("graduate exams = %#v, %v; want %#v", items, err, want)
	}
}

func TestGraduateYJS1HomeworkRequestsAndLocalTimezone(t *testing.T) {
	previous := time.Local
	time.Local = time.FixedZone("graduate-fixture", 9*60*60)
	t.Cleanup(func() { time.Local = previous })
	now := time.Now()
	past := now.Add(-time.Hour).Format("2006-01-02 15:04:05")
	future := now.Add(time.Hour).Format("2006-01-02 15:04")
	rows := []map[string]any{
		{"WID": "1", "KCMC": " Course A ", "ZYMC": " Overdue ", "ZYJZSJ": past},
		{"WID": "2", "XSZYWID": " sub-2 ", "KCMC": "Course B", "ZYMC": "Submitted", "ZYJZSJ": future},
		{"WID": "3", "KCMC": "Course C", "ZYMC": "Graded", "ZYJZSJ": future, "CJ": 0},
		{"WID": "4", "KCMC": "Course D", "ZYMC": "Later", "ZYKSSJ": future, "ZYJZSJ": future},
		{"WID": "5", "KCMC": "Course E", "ZYMC": "Pending", "ZYJZSJ": future},
	}
	encoded, err := json.Marshal(map[string]any{"datas": map[string]any{"homework": map[string]any{"rows": rows}}})
	if err != nil {
		t.Fatal(err)
	}
	fixtures := graduateHomeworkFixtures(string(encoded) + ` {"ignored":true}`)
	items, err := fetchGraduateYJS1Homework(context.Background(), graduateFixtureClient(t, fixtures))
	want := []HomeworkItem{
		{ID: "1", CourseName: "Course A", Title: "Overdue", EndAt: past, Status: "overdue"},
		{ID: "sub-2", CourseName: "Course B", Title: "Submitted", EndAt: future, Status: "submitted"},
		{ID: "3", CourseName: "Course C", Title: "Graded", EndAt: future, Status: "graded"},
		{ID: "4", CourseName: "Course D", Title: "Later", StartAt: future, EndAt: future, Status: "not started"},
		{ID: "5", CourseName: "Course E", Title: "Pending", EndAt: future, Status: "pending"},
	}
	if err != nil || !reflect.DeepEqual(items, want) {
		t.Fatalf("graduate homework = %#v, %v; want %#v", items, err, want)
	}
}

func TestGraduateYJS1EmptyOutputs(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `null`, `{"datas":{"x":{"rows":null}}}`, `{"datas":{"x":{"rows":[]}}}`} {
		t.Run(body, func(t *testing.T) {
			_, curriculum, err := graduateTestClient(graduateFixtureClient(t, graduateScheduleFixtures(body))).FetchCurriculum(context.Background(), 0)
			if err != nil || curriculum == nil || len(curriculum) != 0 {
				t.Fatalf("empty curriculum = %#v, %v", curriculum, err)
			}
			exams, err := fetchGraduateYJS1Exams(context.Background(), graduateFixtureClient(t, graduateExamFixtures(body)), 0)
			if err != nil || exams == nil || len(exams) != 0 {
				t.Fatalf("empty exams = %#v, %v", exams, err)
			}
			homework, err := fetchGraduateYJS1Homework(context.Background(), graduateFixtureClient(t, graduateHomeworkFixtures(body)))
			if err != nil || homework == nil || len(homework) != 0 {
				t.Fatalf("empty homework = %#v, %v", homework, err)
			}
			scores, err := fetchGraduateYJS1ScoreReport(context.Background(), graduateFixtureClient(t, graduateScoreFixtures(body, `null`, body)))
			encoded, _ := json.Marshal(scores)
			if err != nil || string(encoded) != `{"items":[]}` {
				t.Fatalf("empty scores = %s, %v", encoded, err)
			}
		})
	}
}

func TestGraduateYJS1RequestFailuresStopTheFlow(t *testing.T) {
	t.Parallel()
	flows := []struct {
		name     string
		fixtures []graduateRequestFixture
		fetch    func(*http.Client) error
	}{
		{"schedule", graduateScheduleFixtures(`{}`), func(client *http.Client) error {
			_, _, err := graduateTestClient(client).FetchCurriculum(context.Background(), 0)
			return err
		}},
		{"exams", graduateExamFixtures(`{}`), func(client *http.Client) error {
			_, err := fetchGraduateYJS1Exams(context.Background(), client, 0)
			return err
		}},
		{"scores", graduateScoreFixtures(`{}`, `{}`, `{}`), func(client *http.Client) error {
			_, err := fetchGraduateYJS1ScoreReport(context.Background(), client)
			return err
		}},
		{"homework", graduateHomeworkFixtures(`{}`), func(client *http.Client) error {
			_, err := fetchGraduateYJS1Homework(context.Background(), client)
			return err
		}},
	}
	for _, flow := range flows {
		for index, endpoint := range flow.fixtures {
			failures := []string{"network", "read", "HTTP", "invalid JSON", "wrong type", "code"}
			if endpoint.get {
				failures = []string{"network", "read", "HTTP", "missing ID"}
			}
			for _, failure := range failures {
				t.Run(flow.name+"/"+endpoint.url+"/"+failure, func(t *testing.T) {
					fixtures := append([]graduateRequestFixture(nil), flow.fixtures[:index+1]...)
					fixture := &fixtures[index]
					sentinel := errors.New("graduate fixture failure")
					switch failure {
					case "network":
						fixture.networkError = sentinel
					case "read":
						fixture.readError = sentinel
					case "HTTP":
						fixture.status = http.StatusServiceUnavailable
						fixture.body = " unavailable "
					case "invalid JSON":
						fixture.body = "!"
					case "wrong type":
						fixture.body = `{"code":9}`
					case "code":
						fixture.body = `{"code":"9","msg":"fixture rejected"}`
					case "missing ID":
						fixture.body = `{"USERID" : "ignored"}`
					}
					err := flow.fetch(graduateFixtureClient(t, fixtures))
					if err == nil {
						t.Fatal("expected request failure")
					}
					switch failure {
					case "network", "read":
						if !errors.Is(err, sentinel) {
							t.Fatalf("lost failure cause: %v", err)
						}
					case "HTTP":
						method := "POST"
						if fixture.get {
							method = "GET"
						}
						want := method + " " + fixture.url + " returned 503 Service Unavailable: unavailable"
						if err.Error() != want {
							t.Fatalf("HTTP error = %q, want %q", err, want)
						}
					case "invalid JSON":
						var target *json.SyntaxError
						if !errors.As(err, &target) {
							t.Fatalf("JSON error = %T %v", err, err)
						}
					case "wrong type":
						var target *json.UnmarshalTypeError
						if !errors.As(err, &target) {
							t.Fatalf("type error = %T %v", err, err)
						}
					case "code":
						want := "graduate yjs1 request failed: fixture rejected"
						if fixture.url == graduateTestScoreProfile {
							want = "graduate score profile request failed: fixture rejected"
						}
						if err.Error() != want {
							t.Fatalf("code error = %q, want %q", err, want)
						}
					case "missing ID":
						if err.Error() != "could not resolve graduate student ID from "+graduateTestHomeworkApp {
							t.Fatalf("ID error = %v", err)
						}
					}
				})
			}
		}
	}
}
