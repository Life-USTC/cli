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

	"github.com/Life-USTC/client-core/ustc"
	"github.com/PuerkitoBio/goquery"
)

const (
	catalogSemesterURL = "https://catalog.ustc.edu.cn/api/teach/semester/list"
	catalogLessonURL   = "https://catalog.ustc.edu.cn/api/teach/lesson/list-for-teach/%d?page=%d&pageSize=%d"
)

var errNoLessonIDs = errors.New("jw returned no lesson ids")

func IsNoLessonData(err error) bool {
	return errors.Is(err, errNoLessonIDs)
}

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
	return fetchJWExams(ctx, jwClient)
}

func fetchJWExams(ctx context.Context, client *http.Client) ([]ExamItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.ExamArrangementURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch exams: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}
	return parseExamsHTML(res.Body)
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

	return fetchBlackboardHomework(ctx, bbClient)
}

func fetchBlackboardHomework(ctx context.Context, bbClient *http.Client) ([]HomeworkItem, error) {
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
	for _, batch := range ustc.BatchBlackboardCalendarIDs(calendarIDs, ustc.BlackboardCalendarIDsPerRequestMax) {
		var events []ustc.BlackboardCalendarEvent
		if err := withSchoolDebugStep(fmt.Sprintf("Blackboard fetch events calendars=%d", len(batch)), func() error {
			var err error
			events, err = fetchBlackboardCalendarEvents(ctx, bbClient, batch)
			return err
		}); err != nil {
			return nil, err
		}
		debugLog("Blackboard events=%d", len(events))
		for _, event := range events {
			lessonCode, semesterCode := ustc.SplitBlackboardCalendarID(event.CalendarID)
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

func fetchBlackboardCourseCalendarIDs(ctx context.Context, client *http.Client) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.BlackboardCalendarsURL, nil)
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
	ids, err := ustc.ParseBlackboardCalendarIDs(body)
	if err != nil {
		return nil, fmt.Errorf("decode Blackboard calendars: %w: %s", err, responseSnippet(body))
	}
	return ids, nil
}

func fetchBlackboardCalendarEvents(ctx context.Context, client *http.Client, calendarIDs []string) ([]ustc.BlackboardCalendarEvent, error) {
	if len(calendarIDs) == 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.BlackboardCalendarEventsURL(calendarIDs), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard homework: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	var body json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode Blackboard homework: %w", err)
	}
	events, err := ustc.ParseBlackboardCalendarEvents(body)
	if err != nil {
		return nil, fmt.Errorf("decode Blackboard homework: %w", err)
	}
	return events, nil
}

func fetchBlackboardCourseIDs(ctx context.Context, client *http.Client) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.BlackboardCoursesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard portal courses: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	ids, err := ustc.ParseBlackboardCourseIDs(res.Body)
	if err != nil {
		return nil, fmt.Errorf("parse Blackboard portal courses: %w", err)
	}
	return ids, nil
}

func fetchBlackboardGradeStatuses(ctx context.Context, client *http.Client, courseID string) (map[string]string, error) {
	rawURL := ustc.BlackboardGradesURL(courseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Blackboard grades: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, responseError(res)
	}

	return ustc.ParseBlackboardGradeStatuses(res.Body), nil
}

func fetchStudentID(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.CourseTablePageURL, nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch course table page: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return "", responseError(res)
	}

	return ustc.StudentIDFromCourseTableURL(res.Request.URL.String())
}

func fetchCurrentCourseTable(ctx context.Context, client *http.Client, semesterID int, studentID string) ([]int, []catalogLesson, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ustc.CourseTableDataURL(semesterID, studentID), nil)
	if err != nil {
		return nil, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch course table data: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, nil, responseError(res)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read course table data: %w", err)
	}
	table, err := ustc.ParseCourseTable(data)
	if err != nil {
		return nil, nil, err
	}
	if len(table.LessonIDs) == 0 {
		return nil, nil, fmt.Errorf("%w for semester %d", errNoLessonIDs, semesterID)
	}
	lessons := make([]catalogLesson, len(table.Lessons))
	for i, lesson := range table.Lessons {
		lessons[i] = catalogLesson(lesson)
	}
	return table.LessonIDs, lessons, nil
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
			_ = res.Body.Close()
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
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
	parsed, err := fetchJWScoreList(ctx, client, types[0].ID, semesterIDs, semesterNames)
	if err != nil {
		return ScoreReport{}, err
	}

	report := ScoreReport{Summary: parsed.Summary, Items: make([]ScoreItem, len(parsed.Items))}
	for i, item := range parsed.Items {
		// The numeric profile produces integer identifiers and a grade point,
		// including zero for the CLI's existing missing-value behavior.
		semesterID, _ := strconv.Atoi(item.SemesterID)
		report.Items[i] = ScoreItem{
			SemesterID:   semesterID,
			SemesterName: item.SemesterName,
			CourseName:   item.CourseName,
			LessonCode:   item.LessonCode,
			CourseCode:   item.CourseCode,
			Credits:      item.Credits,
			GradePoint:   *item.GradePoint,
			Score:        item.Score,
			GradeText:    item.GradeText,
		}
	}
	return report, nil
}

func fetchJWScoreSemesters(ctx context.Context, client *http.Client) ([]Semester, error) {
	var semesters []Semester
	if err := fetchJSON(ctx, client, ustc.ScoreSemestersURL, nil, &semesters); err != nil {
		return nil, fmt.Errorf("fetch score semesters: %w", err)
	}
	return semesters, nil
}

func fetchJWScoreTypes(ctx context.Context, client *http.Client) ([]scoreSheetType, error) {
	var types []scoreSheetType
	if err := fetchJSON(ctx, client, ustc.ScoreTypesURL, nil, &types); err != nil {
		return nil, fmt.Errorf("fetch score transcript types: %w", err)
	}
	return types, nil
}

func fetchJWScoreList(ctx context.Context, client *http.Client, trainTypeID int, semesterIDs []int, semesterNames map[int]string) (ustc.ScoreReport, error) {
	rawURL := ustc.NumericScoreListURL(trainTypeID, semesterIDs)
	var body json.RawMessage
	if err := fetchJSON(ctx, client, rawURL, nil, &body); err != nil {
		return ustc.ScoreReport{}, fmt.Errorf("fetch score list: %w", err)
	}
	report, err := ustc.ParseNumericScores(body, semesterNames)
	if err != nil {
		return ustc.ScoreReport{}, fmt.Errorf("fetch score list: decode %s: %w", rawURL, err)
	}
	return report, nil
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
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return responseError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

func parseExamsHTML(r io.Reader) ([]ExamItem, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse exams page: %w", err)
	}
	parsed, err := ustc.ParseExamStatusTable(body)
	if err != nil {
		return nil, err
	}
	items := make([]ExamItem, len(parsed))
	for i, item := range parsed {
		items[i] = ExamItem{
			CourseName: item.CourseName,
			LessonCode: item.LessonCode,
			ExamType:   item.ExamType,
			DateTime:   item.DateTime,
			Location:   item.Location,
			Seat:       item.Seat,
			Status:     item.Status,
		}
	}
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

type pageResponse[T any] struct {
	Rows      []T `json:"rows"`
	TotalPage int `json:"totalPage"`
}

type catalogLesson ustc.Lesson

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

type scoreSheetType struct {
	ID int `json:"id"`
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
