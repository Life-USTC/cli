package school

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	graduateYJS1ScheduleAppURL  = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/xskbcx"
	graduateYJS1ScheduleTermURL = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/modules/xskbcx/xnxqxxcx.do"
	graduateYJS1ScheduleRowsURL = "https://yjs1.ustc.edu.cn/gsapp/sys/kbcxappustc/modules/xskbcx/xskbxxcx.do"

	graduateYJS1ScoreAppURL     = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/wdcj"
	graduateYJS1ScoreInfoURL    = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/queryInfo.do"
	graduateYJS1ScoreProfileURL = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/hqxh.do"
	graduateYJS1ScoreRowsURL    = "https://yjs1.ustc.edu.cn/gsapp/sys/wdcjapp/modules/wdcj/xscjcx.do"

	graduateYJS1ExamAppURL  = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/xskssbcx"
	graduateYJS1ExamTermURL = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/modules/xskssbcx/hqxnxq.do"
	graduateYJS1ExamRowsURL = "https://yjs1.ustc.edu.cn/gsapp/sys/kssbappustc/modules/xskssbcx/xscxkssbxx.do"

	graduateYJS1HomeworkAppURL  = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/*default/index.do?THEME=blue&EMAP_LANG=zh&min=1#/wdzy"
	graduateYJS1HomeworkTermURL = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/modules/wdzy/hqxnxq.do"
	graduateYJS1HomeworkRowsURL = "https://yjs1.ustc.edu.cn/gsapp/sys/wdzyappustc/modules/wdzy/wdzycx.do"
)

var graduateYJS1UserIDPattern = regexp.MustCompile(`"USERID":"([^"]+)"`)

type graduateYJS1RowsResponse[T any] struct {
	Code  string                                    `json:"code"`
	Msg   string                                    `json:"msg"`
	Datas map[string]graduateYJS1RowsResponseSet[T] `json:"datas"`
}

type graduateYJS1RowsResponseSet[T any] struct {
	Rows []T `json:"rows"`
}

type graduateYJS1Term struct {
	DM     string `json:"DM"`
	MC     string `json:"MC"`
	SFDQXQ string `json:"SFDQXQ"`
	QSSJ   string `json:"QSSJ"`
	JZSJ   string `json:"JZSJ"`
}

type graduateYJS1ScheduleRow struct {
	WID    string `json:"WID"`
	BJMC   string `json:"BJMC"`
	KCDM   string `json:"KCDM"`
	KCMC   string `json:"KCMC"`
	PKSJDD string `json:"PKSJDD"`
	RKJS   string `json:"RKJS"`
	ZCMC   string `json:"ZCMC"`
}

type graduateYJS1ScoreInfoRow struct {
	XH            string `json:"XH"`
	PYCCDMDisplay string `json:"PYCCDM_DISPLAY"`
	ZYDMDisplay   string `json:"ZYDM_DISPLAY"`
	YXDMDisplay   string `json:"YXDM_DISPLAY"`
	DSXM          string `json:"DSXM"`
	NJDMDisplay   string `json:"NJDM_DISPLAY"`
	ZCZTDisplay   string `json:"ZCZT_DISPLAY"`
	YJBYSJ        string `json:"YJBYSJ"`
}

type graduateYJS1ScoreProfile struct {
	Code   string                      `json:"code"`
	Msg    string                      `json:"msg"`
	XH     string                      `json:"XH"`
	XSCJXX graduateYJS1ScoreProfileRow `json:"xscjxx"`
}

type graduateYJS1ScoreProfileRow struct {
	PYCCDMDisplay string `json:"PYCCDM_DISPLAY"`
	ZYDMDisplay   string `json:"ZYDM_DISPLAY"`
	YXDMDisplay   string `json:"YXDM_DISPLAY"`
	DSXM          string `json:"DSXM"`
	NJDMDisplay   string `json:"NJDM_DISPLAY"`
	ZCZTDisplay   string `json:"ZCZT_DISPLAY"`
	YJBYSJ        string `json:"YJBYSJ"`
	PJJDZ         string `json:"PJJDZ"`
}

type graduateYJS1ScoreRow struct {
	XNXQDM        string `json:"XNXQDM"`
	XNXQDMDisplay string `json:"XNXQDM_DISPLAY"`
	BJMC          string `json:"BJMC"`
	KCDM          string `json:"KCDM"`
	KCMC          string `json:"KCMC"`
	CJFZDMDisplay string `json:"CJFZDM_DISPLAY"`
	CJJLDisplay   string `json:"CJJL_DISPLAY"`
	XF            any    `json:"XF"`
	CJ            any    `json:"CJ"`
	JDZ           any    `json:"JDZ"`
}

type graduateYJS1ExamRow struct {
	BJMC        string `json:"BJMC"`
	KCDM        string `json:"KCDM"`
	KCMC        string `json:"KCMC"`
	KSDD        string `json:"KSDD"`
	KSKSSJ      string `json:"KSKSSJ"`
	KSJSSJ      string `json:"KSJSSJ"`
	KSXS        string `json:"KSXS"`
	KSXSDisplay string `json:"KSXS_DISPLAY"`
	SBZT        string `json:"SBZT"`
	SBZTDisplay string `json:"SBZT_DISPLAY"`
	ZWH         string `json:"ZWH"`
}

type graduateYJS1HomeworkRow struct {
	WID     string `json:"WID"`
	XSZYWID string `json:"XSZYWID"`
	KCMC    string `json:"KCMC"`
	ZYMC    string `json:"ZYMC"`
	ZYKSSJ  string `json:"ZYKSSJ"`
	ZYJZSJ  string `json:"ZYJZSJ"`
	ZYSCSJ  string `json:"ZYSCSJ"`
	CJ      any    `json:"CJ"`
}

func (c *Client) fetchGraduateSemesters(ctx context.Context) ([]Semester, error) {
	_, terms, err := c.graduateScheduleSession(ctx)
	if err != nil {
		return nil, err
	}

	return graduateYJS1SemestersFromTerms(terms), nil
}

func (c *Client) fetchGraduateCurriculum(ctx context.Context, semesterID int) (Semester, []CurriculumItem, error) {
	client, terms, err := c.graduateScheduleSession(ctx)
	if err != nil {
		return Semester{}, nil, err
	}

	selected, err := pickGraduateYJS1Term(terms, semesterID)
	if err != nil {
		return Semester{}, nil, err
	}

	querySetting, err := graduateYJS1QuerySetting([]graduateYJS1Query{
		{Name: "XNXQDM", Builder: "equal", LinkOpt: "AND", BuilderList: "cbl_String", Value: selected.Code},
	})
	if err != nil {
		return Semester{}, nil, err
	}

	rows, err := fetchGraduateYJS1Rows[graduateYJS1ScheduleRow](ctx, client, graduateYJS1ScheduleAppURL, graduateYJS1ScheduleRowsURL, url.Values{
		"querySetting": {querySetting},
		"pageSize":     {"999"},
		"pageNumber":   {"1"},
	})
	if err != nil {
		return Semester{}, nil, err
	}

	return selected, graduateYJS1CurriculumItemsFromRows(selected, rows), nil
}

func (c *Client) graduateScheduleSession(ctx context.Context) (*http.Client, []graduateYJS1Term, error) {
	if c.graduateScheduleClient == nil {
		client, err := newGraduateYJS1Client(ctx, c.creds, graduateYJS1ScheduleAppURL)
		if err != nil {
			return nil, nil, err
		}
		c.graduateScheduleClient = client
	}
	if c.graduateScheduleTerms == nil {
		terms, err := fetchGraduateYJS1Terms(ctx, c.graduateScheduleClient, graduateYJS1ScheduleAppURL, graduateYJS1ScheduleTermURL, url.Values{
			"SFSY":       {"1"},
			"pageSize":   {"100"},
			"pageNumber": {"1"},
		})
		if err != nil {
			return nil, nil, err
		}
		c.graduateScheduleTerms = terms
	}
	return c.graduateScheduleClient, c.graduateScheduleTerms, nil
}

func (c *Client) fetchGraduateExams(ctx context.Context, semesterID int) ([]ExamItem, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, graduateYJS1ExamAppURL)
	if err != nil {
		return nil, err
	}

	terms, err := fetchGraduateYJS1Terms(ctx, client, graduateYJS1ExamAppURL, graduateYJS1ExamTermURL, nil)
	if err != nil {
		return nil, err
	}
	selected, err := pickGraduateYJS1Term(terms, semesterID)
	if err != nil {
		return nil, err
	}

	querySetting, err := graduateYJS1QuerySetting([]graduateYJS1Query{
		{Name: "XNXQDM", Caption: "学年学期", Builder: "m_value_equal", LinkOpt: "AND", Value: selected.Code},
		{Name: "SBZT", Caption: "申报状态", Builder: "equal", LinkOpt: "AND", Value: "1"},
	})
	if err != nil {
		return nil, err
	}

	rows, err := fetchGraduateYJS1Rows[graduateYJS1ExamRow](ctx, client, graduateYJS1ExamAppURL, graduateYJS1ExamRowsURL, url.Values{
		"querySetting": {querySetting},
		"pageSize":     {"999"},
		"pageNumber":   {"1"},
	})
	if err != nil {
		return nil, err
	}

	return graduateYJS1ExamItemsFromRows(rows), nil
}

func (c *Client) fetchGraduateScores(ctx context.Context) (ScoreReport, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, graduateYJS1ScoreAppURL)
	if err != nil {
		return ScoreReport{}, err
	}

	infoRows, err := fetchGraduateYJS1Rows[graduateYJS1ScoreInfoRow](ctx, client, graduateYJS1ScoreAppURL, graduateYJS1ScoreInfoURL, nil)
	if err != nil {
		return ScoreReport{}, err
	}

	var profile graduateYJS1ScoreProfile
	if err := postGraduateYJS1FormJSON(ctx, client, graduateYJS1ScoreAppURL, graduateYJS1ScoreProfileURL, nil, &profile); err != nil {
		return ScoreReport{}, err
	}
	if profile.Code != "" && profile.Code != "0" {
		return ScoreReport{}, fmt.Errorf("graduate score profile request failed: %s", profile.Msg)
	}

	rows, err := fetchGraduateYJS1Rows[graduateYJS1ScoreRow](ctx, client, graduateYJS1ScoreAppURL, graduateYJS1ScoreRowsURL, url.Values{
		"pageSize":   {"999"},
		"pageNumber": {"1"},
	})
	if err != nil {
		return ScoreReport{}, err
	}

	report := ScoreReport{
		Items:   graduateYJS1ScoreItemsFromRows(rows),
		Summary: buildGraduateYJS1ScoreSummary(firstGraduateYJS1ScoreInfoRow(infoRows), profile),
	}
	SortScores(report.Items)
	return report, nil
}

func (c *Client) fetchGraduateHomework(ctx context.Context) ([]HomeworkItem, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, graduateYJS1HomeworkAppURL)
	if err != nil {
		return nil, err
	}

	terms, err := fetchGraduateYJS1Terms(ctx, client, graduateYJS1HomeworkAppURL, graduateYJS1HomeworkTermURL, url.Values{
		"*order": {"-XNDM,-XQDM"},
	})
	if err != nil {
		return nil, err
	}
	selected, err := pickGraduateYJS1Term(terms, 0)
	if err != nil {
		return nil, err
	}

	userID, err := fetchGraduateYJS1UserID(ctx, client, graduateYJS1HomeworkAppURL)
	if err != nil {
		return nil, err
	}

	querySetting, err := graduateYJS1QuerySetting([]graduateYJS1Query{
		{Name: "XNXQDM", Builder: "m_value_equal", Value: selected.Code},
	})
	if err != nil {
		return nil, err
	}

	rows, err := fetchGraduateYJS1Rows[graduateYJS1HomeworkRow](ctx, client, graduateYJS1HomeworkAppURL, graduateYJS1HomeworkRowsURL, url.Values{
		"XH":           {userID},
		"querySetting": {querySetting},
		"pageSize":     {"999"},
		"pageNumber":   {"1"},
	})
	if err != nil {
		return nil, err
	}

	return graduateYJS1HomeworkItemsFromRows(rows, time.Now()), nil
}

type graduateYJS1Query struct {
	Name        string `json:"name"`
	Caption     string `json:"caption,omitempty"`
	Builder     string `json:"builder,omitempty"`
	LinkOpt     string `json:"linkOpt,omitempty"`
	BuilderList string `json:"builderList,omitempty"`
	Value       string `json:"value,omitempty"`
}

func graduateYJS1QuerySetting(filters []graduateYJS1Query) (string, error) {
	data, err := json.Marshal(filters)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fetchGraduateYJS1Terms(ctx context.Context, client *http.Client, appPageURL, rawURL string, form url.Values) ([]graduateYJS1Term, error) {
	return fetchGraduateYJS1Rows[graduateYJS1Term](ctx, client, appPageURL, rawURL, form)
}

func fetchGraduateYJS1Rows[T any](ctx context.Context, client *http.Client, appPageURL, rawURL string, form url.Values) ([]T, error) {
	var response graduateYJS1RowsResponse[T]
	if err := postGraduateYJS1FormJSON(ctx, client, appPageURL, rawURL, form, &response); err != nil {
		return nil, err
	}
	if response.Code != "" && response.Code != "0" {
		return nil, fmt.Errorf("graduate yjs1 request failed: %s", response.Msg)
	}
	for _, data := range response.Datas {
		return data.Rows, nil
	}
	return nil, nil
}

func newGraduateYJS1Client(ctx context.Context, creds Credentials, appPageURL string) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      graduateYJS1LoginURL(appPageURL),
		expectedHost:  "yjs1.ustc.edu.cn",
		postLoginURLs: []string{appPageURL},
	})
}

func graduateYJS1LoginURL(appPageURL string) string {
	return "https://id.ustc.edu.cn/cas/login?service=" + url.QueryEscape(appPageURL)
}

func postGraduateYJS1FormJSON(ctx context.Context, client *http.Client, appPageURL, rawURL string, form url.Values, dest any) error {
	body := ""
	if form != nil {
		body = form.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://yjs1.ustc.edu.cn")
	req.Header.Set("Referer", appPageURL)
	req.Header.Set("User-Agent", schoolUserAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return responseError(res)
	}
	return json.NewDecoder(res.Body).Decode(dest)
}

func fetchGraduateYJS1UserID(ctx context.Context, client *http.Client, appPageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appPageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", appPageURL)
	req.Header.Set("User-Agent", schoolUserAgent)

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", responseError(res)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	match := graduateYJS1UserIDPattern.FindSubmatch(body)
	if len(match) < 2 {
		return "", fmt.Errorf("could not resolve graduate student ID from %s", appPageURL)
	}
	return string(match[1]), nil
}

func pickGraduateYJS1Term(terms []graduateYJS1Term, semesterID int) (Semester, error) {
	semesters := graduateYJS1SemestersFromTerms(terms)
	if len(semesters) == 0 {
		return Semester{}, fmt.Errorf("no graduate semesters available")
	}
	if semesterID == 0 {
		for i, term := range terms {
			if term.SFDQXQ == "1" {
				return semesters[i], nil
			}
		}
		return semesters[0], nil
	}
	for _, semester := range semesters {
		if semester.ID == semesterID {
			return semester, nil
		}
	}
	return Semester{}, fmt.Errorf("graduate semester %d was not found", semesterID)
}

func graduateYJS1SemestersFromTerms(rows []graduateYJS1Term) []Semester {
	semesters := make([]Semester, 0, len(rows))
	for _, row := range rows {
		id, _ := strconv.Atoi(row.DM)
		name := firstNonEmpty(row.MC, graduateYJS1SemesterNameFromCode(row.DM))
		semesters = append(semesters, Semester{
			ID:         id,
			Code:       row.DM,
			SemesterCn: name,
			SemesterEn: name,
			StartDate:  row.QSSJ,
			EndDate:    row.JZSJ,
			IsLast:     row.SFDQXQ == "1",
		})
	}
	return semesters
}

func graduateYJS1SemesterNameFromCode(code string) string {
	if len(code) < 5 {
		return code
	}
	year, err := strconv.Atoi(code[:4])
	if err != nil {
		return code
	}
	switch code[4:] {
	case "1":
		return fmt.Sprintf("%d年秋季学期", year)
	case "2":
		return fmt.Sprintf("%d年春季学期", year+1)
	case "3":
		return fmt.Sprintf("%d年夏季学期", year+1)
	default:
		return code
	}
}

func graduateYJS1CurriculumItemsFromRows(semester Semester, rows []graduateYJS1ScheduleRow) []CurriculumItem {
	items := make([]CurriculumItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, CurriculumItem{
			LessonID:   parseLeadingInt(row.WID),
			SemesterID: semester.ID,
			LessonCode: compactWhitespace(firstNonEmpty(row.BJMC, row.WID)),
			CourseCode: compactWhitespace(row.KCDM),
			CourseName: compactWhitespace(row.KCMC),
			Teachers:   splitTeachers(row.RKJS),
			Schedule:   graduateYJS1ScheduleString(row.PKSJDD, row.ZCMC),
		})
	}
	SortCurriculum(items)
	return items
}

func graduateYJS1ScheduleString(lessonSchedule, weekRanges string) string {
	lessonSchedule = compactWhitespace(lessonSchedule)
	weekRanges = compactWhitespace(strings.ReplaceAll(weekRanges, "~", "-"))
	if lessonSchedule == "" {
		return weekRanges
	}
	if weekRanges == "" {
		return lessonSchedule
	}

	scheduleParts := splitSemicolonParts(lessonSchedule)
	weekParts := splitSemicolonParts(weekRanges)
	if len(scheduleParts) == len(weekParts) {
		parts := make([]string, 0, len(scheduleParts))
		for i, schedule := range scheduleParts {
			if week := weekParts[i]; week != "" {
				parts = append(parts, schedule+" ["+week+"周]")
				continue
			}
			parts = append(parts, schedule)
		}
		return strings.Join(parts, "; ")
	}

	return lessonSchedule + " [" + weekRanges + "周]"
}

func graduateYJS1ScoreItemsFromRows(rows []graduateYJS1ScoreRow) []ScoreItem {
	items := make([]ScoreItem, 0, len(rows))
	for _, row := range rows {
		semesterID, _ := strconv.Atoi(row.XNXQDM)
		items = append(items, ScoreItem{
			SemesterID:   semesterID,
			SemesterName: compactWhitespace(firstNonEmpty(row.XNXQDMDisplay, graduateYJS1SemesterNameFromCode(row.XNXQDM))),
			CourseName:   compactWhitespace(row.KCMC),
			LessonCode:   compactWhitespace(firstNonEmpty(row.BJMC, row.KCDM)),
			CourseCode:   compactWhitespace(row.KCDM),
			Credits:      graduateYJS1Float(row.XF),
			GradePoint:   graduateYJS1Float(row.JDZ),
			Score:        graduateYJS1String(row.CJ),
			GradeText:    graduateYJS1Join(" / ", row.CJFZDMDisplay, row.CJJLDisplay),
		})
	}
	return items
}

func firstGraduateYJS1ScoreInfoRow(rows []graduateYJS1ScoreInfoRow) graduateYJS1ScoreInfoRow {
	if len(rows) == 0 {
		return graduateYJS1ScoreInfoRow{}
	}
	return rows[0]
}

func buildGraduateYJS1ScoreSummary(info graduateYJS1ScoreInfoRow, profile graduateYJS1ScoreProfile) json.RawMessage {
	summary := map[string]any{}
	if studentID := compactWhitespace(firstNonEmpty(profile.XH, info.XH)); studentID != "" {
		summary["studentId"] = studentID
	}
	if program := compactWhitespace(firstNonEmpty(profile.XSCJXX.PYCCDMDisplay, info.PYCCDMDisplay)); program != "" {
		summary["program"] = program
	}
	if major := compactWhitespace(firstNonEmpty(profile.XSCJXX.ZYDMDisplay, info.ZYDMDisplay)); major != "" {
		summary["major"] = major
	}
	if department := compactWhitespace(firstNonEmpty(profile.XSCJXX.YXDMDisplay, info.YXDMDisplay)); department != "" {
		summary["department"] = department
	}
	if advisor := compactWhitespace(firstNonEmpty(profile.XSCJXX.DSXM, info.DSXM)); advisor != "" {
		summary["advisor"] = advisor
	}
	if grade := compactWhitespace(firstNonEmpty(profile.XSCJXX.NJDMDisplay, info.NJDMDisplay)); grade != "" {
		summary["grade"] = grade
	}
	if status := compactWhitespace(firstNonEmpty(profile.XSCJXX.ZCZTDisplay, info.ZCZTDisplay)); status != "" {
		summary["status"] = status
	}
	if graduation := compactWhitespace(firstNonEmpty(profile.XSCJXX.YJBYSJ, info.YJBYSJ)); graduation != "" {
		summary["expectedGraduation"] = graduation
	}
	if gpa := strings.TrimSpace(profile.XSCJXX.PJJDZ); gpa != "" {
		if value, err := strconv.ParseFloat(gpa, 64); err == nil {
			summary["averageGradePoint"] = value
		} else {
			summary["averageGradePoint"] = gpa
		}
	}
	if len(summary) == 0 {
		return nil
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil
	}
	return data
}

func graduateYJS1ExamItemsFromRows(rows []graduateYJS1ExamRow) []ExamItem {
	items := make([]ExamItem, 0, len(rows))
	for _, row := range rows {
		dateTime := compactWhitespace(strings.Trim(strings.Join([]string{
			strings.TrimSpace(row.KSKSSJ),
			strings.TrimSpace(row.KSJSSJ),
		}, " - "), " -"))
		items = append(items, ExamItem{
			CourseName: compactWhitespace(row.KCMC),
			LessonCode: compactWhitespace(firstNonEmpty(row.BJMC, row.KCDM)),
			DateTime:   dateTime,
			Location:   compactWhitespace(row.KSDD),
			ExamType:   compactWhitespace(firstNonEmpty(row.KSXSDisplay, row.KSXS)),
			Status:     compactWhitespace(firstNonEmpty(row.SBZTDisplay, row.SBZT)),
			Seat:       compactWhitespace(row.ZWH),
		})
	}
	SortExams(items)
	return items
}

func graduateYJS1HomeworkItemsFromRows(rows []graduateYJS1HomeworkRow, now time.Time) []HomeworkItem {
	items := make([]HomeworkItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, HomeworkItem{
			ID:         firstNonEmpty(row.XSZYWID, row.WID),
			Title:      compactWhitespace(row.ZYMC),
			CourseName: compactWhitespace(row.KCMC),
			StartAt:    compactWhitespace(row.ZYKSSJ),
			EndAt:      compactWhitespace(row.ZYJZSJ),
			Status:     graduateYJS1HomeworkStatus(row, now),
		})
	}
	SortHomework(items)
	return items
}

func graduateYJS1HomeworkStatus(row graduateYJS1HomeworkRow, now time.Time) string {
	if graduateYJS1String(row.CJ) != "" {
		return "graded"
	}
	if strings.TrimSpace(row.XSZYWID) != "" || strings.TrimSpace(row.ZYSCSJ) != "" {
		return "submitted"
	}
	if deadline, ok := graduateYJS1ParseTime(row.ZYJZSJ); ok && now.After(deadline) {
		return "overdue"
	}
	if start, ok := graduateYJS1ParseTime(row.ZYKSSJ); ok && now.Before(start) {
		return "not started"
	}
	return "pending"
}

func graduateYJS1ParseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func graduateYJS1String(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return compactWhitespace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return compactWhitespace(fmt.Sprint(v))
	}
}

func graduateYJS1Float(value any) float64 {
	switch v := value.(type) {
	case nil:
		return 0
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return f
	}
}

func graduateYJS1Join(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = compactWhitespace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	return strings.Join(parts, sep)
}

func splitSemicolonParts(value string) []string {
	rawParts := strings.Split(value, ";")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = compactWhitespace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
