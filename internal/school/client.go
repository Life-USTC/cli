package school

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	catalogSemesterURL      = "https://catalog.ustc.edu.cn/api/teach/semester/list"
	catalogLessonURL        = "https://catalog.ustc.edu.cn/api/teach/lesson/list-for-teach/%d?page=%d&pageSize=%d"
	jwCourseTableURL        = "https://jw.ustc.edu.cn/for-std/course-table"
	jwCourseDataURL         = "https://jw.ustc.edu.cn/for-std/course-table/get-data?bizTypeId=2&semesterId=%d&dataId=%s"
	jwExamURL               = "https://jw.ustc.edu.cn/for-std/exam-arrange"
	jwScoreSemestersURL     = "https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters"
	jwScoreTypesURL         = "https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeSheetTypes"
	jwScoreListURL          = "https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeList"
	bbCalendarPageURL       = "https://www.bb.ustc.edu.cn/webapps/calendar/viewPersonal"
	bbCalendarListURL       = "https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/calendars?mode=personal&course_id="
	bbCalendarEventsURLBase = "https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/events"
)

var errNoLessonIDs = errors.New("jw returned no lesson ids")

func IsNoLessonData(err error) bool {
	return errors.Is(err, errNoLessonIDs)
}

const (
	bbCalendarEventsStartMillis int64 = 0
	bbCalendarEventsEndMillis   int64 = 4102444800000
	bbCalendarIDsPerRequestMax        = 1800
)

type Client struct {
	creds                  Credentials
	program                Program
	graduateScheduleClient *http.Client
	graduateScheduleTerms  []graduateYJS1Term
}

func NewClient(creds Credentials, programs ...Program) *Client {
	program := ProgramUndergraduate
	if len(programs) > 0 {
		program = programs[0]
	}
	return &Client{creds: creds, program: program}
}

func (c *Client) FetchSemesters(ctx context.Context) ([]Semester, error) {
	if c.program.IsGraduate() {
		return c.fetchGraduateSemesters(ctx)
	}
	client, err := newCatalogClient(ctx, c.creds)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogSemesterURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch semesters: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		jwClient, err := newJWClient(ctx, c.creds)
		if err != nil {
			return nil, responseError(res)
		}
		return fetchJWScoreSemesters(ctx, jwClient)
	}

	var semesters []Semester
	if err := json.NewDecoder(res.Body).Decode(&semesters); err != nil {
		return nil, fmt.Errorf("decode semesters: %w", err)
	}
	return semesters, nil
}

func PickSemester(semesters []Semester, semesterID int) (Semester, error) {
	if semesterID > 0 {
		for _, semester := range semesters {
			if semester.ID == semesterID {
				return semester, nil
			}
		}
		return Semester{}, fmt.Errorf("semester %d not found", semesterID)
	}
	for _, semester := range semesters {
		if semester.IsLast {
			return semester, nil
		}
	}
	if len(semesters) == 0 {
		return Semester{}, fmt.Errorf("no semester data returned from catalog")
	}
	current := semesters[0]
	for _, semester := range semesters[1:] {
		if semester.ID > current.ID {
			current = semester
		}
	}
	return current, nil
}

func (c *Client) FetchCurriculum(ctx context.Context, semesterID int) (Semester, []CurriculumItem, error) {
	if c.program.IsGraduate() {
		return c.fetchGraduateCurriculum(ctx, semesterID)
	}

	semesters, err := c.FetchSemesters(ctx)
	if err != nil {
		return Semester{}, nil, err
	}

	jwClient, err := newJWClient(ctx, c.creds)
	if err != nil {
		return Semester{}, nil, err
	}
	studentID, err := fetchStudentID(ctx, jwClient)
	if err != nil {
		return Semester{}, nil, err
	}

	semester, lessonIDs, jwLessons, err := pickCurriculumSemester(ctx, jwClient, semesters, semesterID, studentID)
	if err != nil {
		return Semester{}, nil, err
	}
	lessons := jwLessons
	if semester.Name() != "" {
		catalogClient, err := newCatalogClient(ctx, c.creds)
		if err != nil {
			return Semester{}, nil, err
		}
		catalogLessons, err := fetchCatalogLessons(ctx, catalogClient, semester.ID)
		if err != nil {
			if len(jwLessons) == 0 {
				return Semester{}, nil, err
			}
		} else {
			lessons = catalogLessons
		}
	}
	if len(lessons) == 0 {
		return Semester{}, nil, fmt.Errorf("no lesson details returned for semester %d", semester.ID)
	}

	set := make(map[int]struct{}, len(lessonIDs))
	for _, lessonID := range lessonIDs {
		set[lessonID] = struct{}{}
	}

	items := make([]CurriculumItem, 0, len(set))
	for _, lesson := range lessons {
		if _, ok := set[lesson.ID]; !ok {
			continue
		}
		items = append(items, lesson.toCurriculumItem(semester.ID))
	}
	SortCurriculum(items)
	return semester, items, nil
}

func pickCurriculumSemester(ctx context.Context, client *http.Client, semesters []Semester, semesterID int, studentID string) (Semester, []int, []catalogLesson, error) {
	if semesterID > 0 {
		semester, err := PickSemester(semesters, semesterID)
		if err != nil {
			return Semester{}, nil, nil, err
		}
		lessonIDs, lessons, err := fetchCurrentCourseTable(ctx, client, semester.ID, studentID)
		return semester, lessonIDs, lessons, err
	}

	selected, err := PickSemester(semesters, 0)
	if err != nil {
		return Semester{}, nil, nil, err
	}

	candidates := append([]Semester{selected}, semesters...)
	slices.SortFunc(candidates[1:], func(a, b Semester) int {
		return b.ID - a.ID
	})

	seen := map[int]struct{}{}
	var lastNoLessons error
	for _, semester := range candidates {
		if _, ok := seen[semester.ID]; ok {
			continue
		}
		seen[semester.ID] = struct{}{}

		lessonIDs, lessons, err := fetchCurrentCourseTable(ctx, client, semester.ID, studentID)
		if err == nil {
			return semester, lessonIDs, lessons, nil
		}
		if errors.Is(err, errNoLessonIDs) {
			lastNoLessons = err
			continue
		}
		return Semester{}, nil, nil, err
	}
	if lastNoLessons != nil {
		return Semester{}, nil, nil, lastNoLessons
	}
	return Semester{}, nil, nil, fmt.Errorf("no semester data returned from jw course table")
}

func (c *Client) FetchExams(ctx context.Context, semesterIDs ...int) ([]ExamItem, error) {
	if c.program.IsGraduate() {
		semesterID := 0
		if len(semesterIDs) > 0 {
			semesterID = semesterIDs[0]
		}
		return c.fetchGraduateExams(ctx, semesterID)
	}

	jwClient, err := newJWClient(ctx, c.creds)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwExamURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := jwClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch exams: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	items, err := parseExamsHTML(res.Body)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) FetchScores(ctx context.Context) (ScoreReport, error) {
	if c.program.IsGraduate() {
		return c.fetchGraduateScores(ctx)
	}

	jwClient, err := newJWClient(ctx, c.creds)
	if err != nil {
		return ScoreReport{}, err
	}
	report, err := fetchJWScoreReport(ctx, jwClient)
	if err != nil {
		return ScoreReport{}, err
	}
	return report, nil
}

func (c *Client) FetchHomework(ctx context.Context) ([]HomeworkItem, error) {
	if c.program.IsGraduate() {
		return c.fetchGraduateHomework(ctx)
	}

	var bbClient *http.Client
	if err := withSchoolDebugStep("Blackboard authenticate", func() error {
		var err error
		bbClient, err = newBlackboardClient(ctx, c.creds)
		return err
	}); err != nil {
		return nil, err
	}

	var calendarIDs []string
	if err := withSchoolDebugStep("Blackboard fetch calendars", func() error {
		var err error
		calendarIDs, err = fetchBlackboardCourseCalendarIDs(ctx, bbClient)
		return err
	}); err != nil {
		return nil, err
	}
	debugLog("Blackboard calendars=%d", len(calendarIDs))

	var courseIDs map[string]string
	if err := withSchoolDebugStep("Blackboard fetch course ids", func() error {
		var err error
		courseIDs, err = fetchBlackboardCourseIDs(ctx, bbClient)
		return err
	}); err != nil {
		return nil, err
	}
	debugLog("Blackboard course ids=%d", len(courseIDs))
	statusesByCourse := map[string]map[string]string{}

	itemsByID := make(map[string]HomeworkItem)
	for _, batch := range batchCalendarIDs(calendarIDs, bbCalendarIDsPerRequestMax) {
		var events []blackboardCalendarEvent
		if err := withSchoolDebugStep(fmt.Sprintf("Blackboard fetch events calendars=%d", len(batch)), func() error {
			var err error
			events, err = fetchBlackboardCalendarEvents(ctx, bbClient, batch)
			return err
		}); err != nil {
			return nil, err
		}
		debugLog("Blackboard events=%d", len(events))
		for _, event := range events {
			lessonCode, semesterCode := splitBlackboardCalendarID(event.CalendarID)
			status := event.EventType
			internalCourseID := courseIDs[event.CalendarID]
			if internalCourseID != "" && event.ItemSourceID != "" {
				statuses, ok := statusesByCourse[internalCourseID]
				if !ok {
					if err := withSchoolDebugStep(fmt.Sprintf("Blackboard fetch grades course=%s internal=%s", event.CalendarID, internalCourseID), func() error {
						var err error
						statuses, err = fetchBlackboardGradeStatuses(ctx, bbClient, internalCourseID)
						return err
					}); err != nil {
						return nil, err
					}
					statusesByCourse[internalCourseID] = statuses
				}
				if gradeStatus := statuses[event.ItemSourceID]; gradeStatus != "" {
					status = gradeStatus
				}
			}
			itemsByID[event.ID] = HomeworkItem{
				ID:             event.ID,
				Title:          event.Title,
				CourseName:     event.CalendarName,
				LessonCode:     lessonCode,
				SemesterCode:   semesterCode,
				ExternalItemID: event.ItemSourceID,
				StartAt:        event.Start,
				EndAt:          event.End,
				Status:         status,
			}
		}
	}

	items := make([]HomeworkItem, 0, len(itemsByID))
	for _, item := range itemsByID {
		items = append(items, item)
	}
	SortHomework(items)
	return items, nil
}

type blackboardCalendarPayload struct {
	Calendars []blackboardCalendar `json:"calendars"`
}

type blackboardCalendar struct {
	ID string `json:"id"`
}

type blackboardCalendarEvent struct {
	ID           string `json:"id"`
	CalendarID   string `json:"calendarId"`
	ItemSourceID string `json:"itemSourceId"`
	Title        string `json:"title"`
	Start        string `json:"start"`
	End          string `json:"end"`
	EventType    string `json:"eventType"`
	CalendarName string `json:"calendarName"`
}

func fetchBlackboardCourseCalendarIDs(ctx context.Context, client *http.Client) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bbCalendarListURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard calendars: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Blackboard calendars: %w", err)
	}
	var payload blackboardCalendarPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Blackboard calendars: %w: %s", err, responseSnippet(body))
	}
	return filterBlackboardCourseCalendarIDs(payload.Calendars), nil
}

func fetchBlackboardCalendarEvents(ctx context.Context, client *http.Client, calendarIDs []string) ([]blackboardCalendarEvent, error) {
	if len(calendarIDs) == 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildBlackboardCalendarEventsURL(calendarIDs), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard homework: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	var events []blackboardCalendarEvent
	if err := json.NewDecoder(res.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("decode Blackboard homework: %w", err)
	}
	return events, nil
}

func fetchBlackboardCourseIDs(ctx context.Context, client *http.Client) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.bb.ustc.edu.cn/webapps/portal/execute/tabs/tabAction?tab_tab_group_id=_1_1", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard portal courses: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("parse Blackboard portal courses: %w", err)
	}
	out := map[string]string{}
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		rawCourseID := strings.TrimSpace(strings.Split(strings.TrimSpace(s.Text()), ":")[0])
		if rawCourseID == "" {
			return
		}
		parsed, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			return
		}
		if parsed.Path != "/webapps/blackboard/execute/launcher" || parsed.Query().Get("type") != "Course" {
			return
		}
		internalID := parsed.Query().Get("id")
		if internalID != "" {
			out[rawCourseID] = internalID
		}
	})
	return out, nil
}

func fetchBlackboardGradeStatuses(ctx context.Context, client *http.Client, courseID string) (map[string]string, error) {
	rawURL := "https://www.bb.ustc.edu.cn/webapps/gradebook/do/student/viewGrades?course_id=" + url.QueryEscape(courseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard grades: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	return blackboardGradeStatusesFromHTML(res.Body), nil
}

func blackboardGradeStatusesFromHTML(reader io.Reader) map[string]string {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	doc.Find("#grades_wrapper .sortable_item_row").Each(func(_ int, s *goquery.Selection) {
		id := strings.TrimSpace(s.AttrOr("id", ""))
		if id == "" {
			return
		}
		status := strings.TrimSpace(s.Find(".cell.activity .activityType").First().Text())
		if status == "" {
			status = strings.TrimSpace(s.Find(".cell.gradeStatus").First().Text())
		}
		if status != "" {
			out["_"+id+"_1"] = compactWhitespace(status)
		}
	})
	return out
}

func splitBlackboardCalendarID(calendarID string) (string, string) {
	parts := strings.Split(strings.TrimSpace(calendarID), ".")
	if len(parts) < 3 {
		return strings.TrimSpace(calendarID), ""
	}
	semesterCode := parts[len(parts)-1]
	if len(semesterCode) != 6 {
		return strings.TrimSpace(calendarID), ""
	}
	for _, r := range semesterCode[:4] {
		if r < '0' || r > '9' {
			return strings.TrimSpace(calendarID), ""
		}
	}
	switch semesterCode[4:] {
	case "SP", "SU", "FA":
		return strings.Join(parts[:len(parts)-1], "."), semesterCode
	default:
		return strings.TrimSpace(calendarID), ""
	}
}

func batchCalendarIDs(calendarIDs []string, maxQueryLength int) [][]string {
	if len(calendarIDs) == 0 {
		return nil
	}
	if maxQueryLength <= 0 {
		return [][]string{append([]string(nil), calendarIDs...)}
	}

	batches := make([][]string, 0, 1)
	current := make([]string, 0, len(calendarIDs))
	currentLen := 0
	for _, calendarID := range calendarIDs {
		extraLen := len(calendarID)
		if len(current) > 0 {
			extraLen++
		}
		if currentLen > 0 && currentLen+extraLen > maxQueryLength {
			batches = append(batches, current)
			current = make([]string, 0, len(calendarIDs))
			currentLen = 0
		}
		current = append(current, calendarID)
		currentLen += extraLen
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func buildBlackboardCalendarEventsURL(calendarIDs []string) string {
	return fmt.Sprintf(
		"%s?start=%d&end=%d&course_id=&calendarIds=%s",
		bbCalendarEventsURLBase,
		bbCalendarEventsStartMillis,
		bbCalendarEventsEndMillis,
		strings.Join(calendarIDs, ","),
	)
}

func filterBlackboardCourseCalendarIDs(calendars []blackboardCalendar) []string {
	ids := make([]string, 0, len(calendars))
	for _, calendar := range calendars {
		if calendar.ID == "" || calendar.ID == "PERSONAL" || calendar.ID == "INSTITUTION" {
			continue
		}
		ids = append(ids, calendar.ID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func fetchStudentID(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwCourseTableURL, nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch course table page: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", responseError(res)
	}

	return extractStudentIDFromURL(res.Request.URL.String())
}

func fetchCurrentCourseTable(ctx context.Context, client *http.Client, semesterID int, studentID string) ([]int, []catalogLesson, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(jwCourseDataURL, semesterID, studentID), nil)
	if err != nil {
		return nil, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch course table data: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, nil, responseError(res)
	}

	var payload struct {
		StudentTableVm struct {
			LessonIDs []int           `json:"lessonIds"`
			Lessons   []catalogLesson `json:"lessons"`
		} `json:"studentTableVm"`
		LessonIDs       []int                    `json:"lessonIds"`
		Lessons         []catalogLesson          `json:"lessons"`
		LessonID2Lesson map[string]catalogLesson `json:"lessonId2Lesson"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("decode course table data: %w", err)
	}

	lessonIDs := payload.StudentTableVm.LessonIDs
	if len(lessonIDs) == 0 {
		lessonIDs = payload.LessonIDs
	}
	if len(lessonIDs) == 0 {
		return nil, nil, fmt.Errorf("%w for semester %d", errNoLessonIDs, semesterID)
	}

	slices.Sort(lessonIDs)
	lessons := payload.StudentTableVm.Lessons
	if len(lessons) == 0 {
		lessons = payload.Lessons
	}
	if len(lessons) == 0 && len(payload.LessonID2Lesson) > 0 {
		for _, lessonID := range lessonIDs {
			if lesson, ok := payload.LessonID2Lesson[strconv.Itoa(lessonID)]; ok {
				lessons = append(lessons, lesson)
			}
		}
	}
	return slices.Compact(lessonIDs), lessons, nil
}

func fetchCatalogLessons(ctx context.Context, client *http.Client, semesterID int) ([]catalogLesson, error) {
	var lessons []catalogLesson
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(catalogLessonURL, semesterID, page, 500), nil)
		if err != nil {
			return nil, err
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch catalog lessons: %w", err)
		}
		if res.StatusCode >= 300 {
			err := responseError(res)
			res.Body.Close()
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read catalog lessons: %w", err)
		}

		var direct []catalogLesson
		if err := json.Unmarshal(body, &direct); err == nil {
			return direct, nil
		}

		var payload pageResponse[catalogLesson]
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("decode catalog lessons: %w", err)
		}
		lessons = append(lessons, payload.Rows...)
		if payload.TotalPage == 0 || page >= payload.TotalPage {
			break
		}
	}
	return lessons, nil
}

func fetchJWScoreReport(ctx context.Context, client *http.Client) (ScoreReport, error) {
	semesters, err := fetchJWScoreSemesters(ctx, client)
	if err != nil {
		return ScoreReport{}, err
	}
	if len(semesters) == 0 {
		return ScoreReport{}, fmt.Errorf("jw returned no score semesters")
	}

	types, err := fetchJWScoreTypes(ctx, client)
	if err != nil {
		return ScoreReport{}, err
	}
	if len(types) == 0 {
		return ScoreReport{}, fmt.Errorf("jw returned no score transcript types")
	}

	semesterIDs := make([]int, 0, len(semesters))
	semesterNames := make(map[int]string, len(semesters))
	for _, semester := range semesters {
		semesterIDs = append(semesterIDs, semester.ID)
		semesterNames[semester.ID] = semester.Name()
	}

	payload, err := fetchJWScoreList(ctx, client, types[0].ID, semesterIDs)
	if err != nil {
		return ScoreReport{}, err
	}

	summary, err := buildJWScoreSummary(payload.Overview, payload.StdGradeRank)
	if err != nil {
		return ScoreReport{}, err
	}

	report := ScoreReport{
		Summary: summary,
		Items:   flattenJWScoreItems(payload.Semesters, semesterNames),
	}
	SortScores(report.Items)
	return report, nil
}

func fetchJWScoreSemesters(ctx context.Context, client *http.Client) ([]Semester, error) {
	var semesters []Semester
	if err := fetchJSON(ctx, client, jwScoreSemestersURL, nil, &semesters); err != nil {
		return nil, fmt.Errorf("fetch score semesters: %w", err)
	}
	return semesters, nil
}

func fetchJWScoreTypes(ctx context.Context, client *http.Client) ([]scoreSheetType, error) {
	var types []scoreSheetType
	if err := fetchJSON(ctx, client, jwScoreTypesURL, nil, &types); err != nil {
		return nil, fmt.Errorf("fetch score transcript types: %w", err)
	}
	return types, nil
}

func fetchJWScoreList(ctx context.Context, client *http.Client, trainTypeID int, semesterIDs []int) (scoreListResponse, error) {
	query := url.Values{}
	query.Set("trainTypeId", strconv.Itoa(trainTypeID))
	query.Set("semesterIds", joinInts(semesterIDs))

	var payload scoreListResponse
	if err := fetchJSON(ctx, client, jwScoreListURL, query, &payload); err != nil {
		return scoreListResponse{}, fmt.Errorf("fetch score list: %w", err)
	}
	return payload, nil
}

func buildJWScoreSummary(overview scoreOverview, rank *scoreRankInfo) (json.RawMessage, error) {
	summary := map[string]any{}
	if overview.PassedCredits != 0 || overview.NotPassedCredits != 0 || overview.GPA != 0 || overview.WeightedScore != 0 || overview.ArithmeticScore != 0 {
		summary["totalCredits"] = overview.PassedCredits + overview.NotPassedCredits
		summary["earnedCredits"] = overview.PassedCredits
		summary["failedCredits"] = overview.NotPassedCredits
		summary["gpa"] = overview.GPA
		summary["weightedAverage"] = overview.WeightedScore
		summary["averageScore"] = overview.ArithmeticScore
	}
	if rank != nil {
		summary["fromSemester"] = rank.StartSemesterName
		summary["toSemester"] = rank.EndSemesterName
		summary["periodGPA"] = rank.GPA
		if rank.MajorName != "" && rank.MajorRank != 0 && rank.MajorStdCount != 0 {
			summary["ranking"] = fmt.Sprintf("全校%s年级%s专业 GPA 排名 %d/%d", rank.Grade, rank.MajorName, rank.MajorRank, rank.MajorStdCount)
		}
	}
	if len(summary) == 0 {
		return nil, nil
	}
	return json.Marshal(summary)
}

func flattenJWScoreItems(semesters []scoreSemesterScores, semesterNames map[int]string) []ScoreItem {
	out := make([]ScoreItem, 0)
	for _, semester := range semesters {
		out = append(out, flattenScoreItems(semester.Scores, semester.ID, semesterNames[semester.ID])...)
	}
	return out
}

func flattenScoreItems(items []scoreAPIItem, defaultSemesterID int, defaultSemesterName string) []ScoreItem {
	out := make([]ScoreItem, 0, len(items))
	for _, item := range items {
		semesterID := firstNonZeroInt(item.SemesterID, item.SemesterAssoc, defaultSemesterID)
		out = append(out, ScoreItem{
			SemesterID:   semesterID,
			SemesterName: firstNonEmpty(item.SemesterName, item.SemesterCh, defaultSemesterName, item.Semester.SemesterCn, item.Semester.SemesterEn),
			CourseName:   firstNonEmpty(item.CourseName, item.CourseNameCh, item.Course.NameZh, item.Course.NameEn, item.CourseNameEn),
			LessonCode:   item.LessonCode,
			CourseCode:   firstNonEmpty(item.CourseCode, item.Course.Code),
			Credits:      firstNonZero(item.Credits, item.Course.Credits),
			GradePoint:   item.GradePoint,
			Score:        firstNonEmpty(item.ScoreCh, item.Score, item.ScoreText, item.Grade, item.ScoreEn),
			GradeText:    firstNonEmpty(item.GradeText, item.ScoreText),
		})
	}
	return out
}

func fetchJSON[T any](ctx context.Context, client *http.Client, rawURL string, query url.Values, out *T) error {
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", schoolUserAgent)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return responseError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}

func parseExamsHTML(r io.Reader) ([]ExamItem, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse exams page: %w", err)
	}

	items := make([]ExamItem, 0)
	doc.Find("table tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 4 {
			return
		}
		values := make([]string, 0, cells.Length())
		cells.Each(func(_ int, cell *goquery.Selection) {
			values = append(values, compactWhitespace(cell.Text()))
		})
		item := ExamItem{}
		if len(values) > 0 {
			item.CourseName = values[0]
		}
		if len(values) > 1 {
			item.LessonCode = values[1]
		}
		if len(values) > 2 {
			item.ExamType = values[2]
		}
		if len(values) > 3 {
			item.DateTime = values[3]
		}
		if len(values) > 4 {
			item.Location = values[4]
		}
		if len(values) > 5 {
			item.Seat = values[5]
		}
		if len(values) > 6 {
			item.Status = values[6]
		}
		if item.CourseName != "" {
			items = append(items, item)
		}
	})

	SortExams(items)
	return items, nil
}

func parseScoreReportHTML(r io.Reader) (ScoreReport, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return ScoreReport{}, fmt.Errorf("parse rendered score page: %w", err)
	}

	report := ScoreReport{}
	if summary := parseScoreSummary(doc); len(summary) > 0 {
		report.Summary, err = json.Marshal(summary)
		if err != nil {
			return ScoreReport{}, fmt.Errorf("marshal score summary: %w", err)
		}
	}

	semesterIDs := map[string]int{}
	doc.Find(".history-table thead select option").Each(func(_ int, option *goquery.Selection) {
		name := strings.TrimSpace(option.Text())
		if name == "" {
			return
		}
		id, convErr := strconv.Atoi(strings.TrimSpace(attrOr(option, "value", "")))
		if convErr != nil || id == 0 {
			return
		}
		semesterIDs[name] = id
	})

	doc.Find(".tab-content .tab-pane.active .semesters .semester").Each(func(_ int, semester *goquery.Selection) {
		semesterName := normalizeSpace(semester.Find("h4").First().Text())
		semesterID := semesterIDs[semesterName]

		semester.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() < 5 {
				return
			}

			courseName, courseCode := parseScoreCourseCell(cells.Eq(0))
			if courseName == "" {
				return
			}

			report.Items = append(report.Items, ScoreItem{
				SemesterID:   semesterID,
				SemesterName: semesterName,
				CourseName:   courseName,
				CourseCode:   courseCode,
				Credits:      parseScoreNumber(cells.Eq(2).Text()),
				GradePoint:   parseScoreNumber(cells.Eq(3).Text()),
				Score:        normalizeSpace(cells.Eq(4).Text()),
			})
		})
	})

	if len(report.Items) == 0 {
		return ScoreReport{}, fmt.Errorf("parse rendered score page: no score rows found")
	}

	SortScores(report.Items)
	return report, nil
}

func parseScoreSummary(doc *goquery.Document) map[string]any {
	summary := map[string]any{}

	rankText := normalizeSpace(doc.Find(".rankinfo").First().Text())
	if rankText != "" {
		pattern := regexp.MustCompile(`^(.*?)\s*[–-]\s*(.*?)平均学分绩点（GPA）为\s*([0-9.]+)，排名情况：\s*(.+)$`)
		if parts := pattern.FindStringSubmatch(rankText); len(parts) == 5 {
			summary["fromSemester"] = normalizeSpace(parts[1])
			summary["toSemester"] = normalizeSpace(parts[2])
			summary["periodGPA"] = parseSummaryValue(parts[3])
			summary["ranking"] = normalizeSpace(parts[4])
		} else {
			summary["rankingText"] = rankText
		}
	}

	fields := map[string]string{
		"总学分":   "totalCredits",
		"已获学分":  "earnedCredits",
		"不及格学分": "failedCredits",
		"GPA":   "gpa",
		"加权平均分": "weightedAverage",
		"算术平均分": "averageScore",
	}
	doc.Find(".tab-content .tab-pane.active .overview li").Each(func(_ int, item *goquery.Selection) {
		spans := item.Find("span")
		if spans.Length() < 2 {
			return
		}
		label := normalizeSpace(spans.Eq(0).Text())
		key, ok := fields[label]
		if !ok {
			return
		}
		summary[key] = parseSummaryValue(spans.Eq(1).Text())
	})

	return summary
}

func parseScoreCourseCell(cell *goquery.Selection) (string, string) {
	clone := cell.Clone()
	code := normalizeSpace(clone.Find("small").First().Text())
	clone.Find("small").Remove()
	return normalizeSpace(clone.Text()), code
}

func parseSummaryValue(value string) any {
	value = normalizeSpace(value)
	if value == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number
	}
	return value
}

func parseScoreNumber(value string) float64 {
	value = normalizeSpace(value)
	if value == "" {
		return 0
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return number
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func splitTeachers(value string) []string {
	parts := regexp.MustCompile(`[、,，;/；]+`).Split(value, -1)
	teachers := make([]string, 0, len(parts))
	for _, part := range parts {
		name := compactWhitespace(part)
		if name != "" {
			teachers = append(teachers, name)
		}
	}
	slices.Sort(teachers)
	return slices.Compact(teachers)
}

func parseLeadingInt(value string) int {
	match := regexp.MustCompile(`^\d+`).FindString(strings.TrimSpace(value))
	if match == "" {
		return 0
	}
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return parsed
}

func attrOr(selection *goquery.Selection, name, fallback string) string {
	if value, ok := selection.Attr(name); ok {
		return value
	}
	return fallback
}

func extractStudentIDFromURL(rawURL string) (string, error) {
	re := regexp.MustCompile(`/course-table/(?:info/)?(\d+)`)
	match := re.FindStringSubmatch(rawURL)
	if len(match) != 2 {
		return "", fmt.Errorf("could not extract jw student id from %s", rawURL)
	}
	return match[1], nil
}

type pageResponse[T any] struct {
	Rows      []T `json:"rows"`
	TotalPage int `json:"totalPage"`
}

type catalogLesson struct {
	ID                      int                        `json:"id"`
	Code                    string                     `json:"code"`
	DateTimePlacePersonText localizedOrString          `json:"dateTimePlacePersonText"`
	CourseCode              string                     `json:"courseCode"`
	CourseName              string                     `json:"courseName"`
	Credits                 float64                    `json:"credits"`
	Course                  catalogCourse              `json:"course"`
	TeacherAssignmentList   []catalogTeacherAssignment `json:"teacherAssignmentList"`
}

type catalogTeacherAssignment struct {
	CN      string `json:"cn"`
	EN      string `json:"en"`
	Teacher struct {
		NameZh string `json:"nameZh"`
		NameEn string `json:"nameEn"`
	} `json:"teacher"`
}

type catalogCourse struct {
	Code    string  `json:"code"`
	NameZh  string  `json:"nameZh"`
	NameEn  string  `json:"nameEn"`
	CN      string  `json:"cn"`
	EN      string  `json:"en"`
	Credits float64 `json:"credits"`
}

type localizedOrString struct {
	Value string
}

func (l *localizedOrString) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		l.Value = text
		return nil
	}

	var localized struct {
		CN string `json:"cn"`
		EN string `json:"en"`
	}
	if err := json.Unmarshal(data, &localized); err != nil {
		return err
	}
	l.Value = firstNonEmpty(localized.CN, localized.EN)
	return nil
}

func (l catalogLesson) toCurriculumItem(semesterID int) CurriculumItem {
	teachers := make([]string, 0, len(l.TeacherAssignmentList))
	for _, assignment := range l.TeacherAssignmentList {
		name := firstNonEmpty(assignment.Teacher.NameZh, assignment.Teacher.NameEn, assignment.CN, assignment.EN)
		if name != "" {
			teachers = append(teachers, name)
		}
	}
	return CurriculumItem{
		SemesterID: semesterID,
		LessonID:   l.ID,
		LessonCode: l.Code,
		CourseCode: firstNonEmpty(l.CourseCode, l.Course.Code),
		CourseName: firstNonEmpty(l.CourseName, l.Course.NameZh, l.Course.NameEn, l.Course.CN, l.Course.EN),
		Credits:    firstNonZero(l.Credits, l.Course.Credits),
		Teachers:   slices.Compact(teachers),
		Schedule:   compactWhitespace(l.DateTimePlacePersonText.Value),
	}
}

type scoreAPIItem struct {
	SemesterID    int            `json:"semesterId"`
	SemesterAssoc int            `json:"semesterAssoc"`
	SemesterName  string         `json:"semesterName"`
	SemesterCh    string         `json:"semesterCh"`
	SemesterEn    string         `json:"semesterEn"`
	LessonCode    string         `json:"lessonCode"`
	CourseCode    string         `json:"courseCode"`
	CourseName    string         `json:"courseName"`
	CourseNameCh  string         `json:"courseNameCh"`
	CourseNameEn  string         `json:"courseNameEn"`
	Credits       float64        `json:"credits"`
	GradePoint    float64        `json:"gp"`
	Score         string         `json:"score"`
	ScoreCh       string         `json:"scoreCh"`
	ScoreEn       string         `json:"scoreEn"`
	ScoreText     string         `json:"scoreText"`
	Grade         string         `json:"grade"`
	GradeText     string         `json:"gradeDetail"`
	Course        scoreAPICourse `json:"course"`
	Semester      Semester       `json:"semester"`
}

type scoreAPICourse struct {
	Code    string  `json:"code"`
	NameZh  string  `json:"nameZh"`
	NameEn  string  `json:"nameEn"`
	Credits float64 `json:"credits"`
}

type scoreSheetType struct {
	ID int `json:"id"`
}

type scoreListResponse struct {
	Overview     scoreOverview         `json:"overview"`
	Semesters    []scoreSemesterScores `json:"semesters"`
	StdGradeRank *scoreRankInfo        `json:"stdGradeRank"`
}

type scoreSemesterScores struct {
	ID     int            `json:"id"`
	Scores []scoreAPIItem `json:"scores"`
}

type scoreOverview struct {
	PassedCredits    float64 `json:"passedCredits"`
	GPA              float64 `json:"gpa"`
	WeightedScore    float64 `json:"weightedScore"`
	NotPassedCredits float64 `json:"notPassedCredits"`
	ArithmeticScore  float64 `json:"arithmeticScore"`
}

type scoreRankInfo struct {
	StartSemesterName string  `json:"startSemesterName"`
	EndSemesterName   string  `json:"endSemesterName"`
	GPA               float64 `json:"gpa"`
	Grade             string  `json:"grade"`
	MajorName         string  `json:"majorName"`
	MajorRank         int     `json:"majorRank"`
	MajorStdCount     int     `json:"majorStdCount"`
}

func responseError(res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	return fmt.Errorf("%s %s returned %s: %s", res.Request.Method, res.Request.URL, res.Status, strings.TrimSpace(string(body)))
}

func responseSnippet(body []byte) string {
	text := compactWhitespace(string(body))
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
