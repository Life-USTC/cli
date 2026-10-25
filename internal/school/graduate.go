package school

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Life-USTC/client-core/ustc"
)

func (c *Client) fetchGraduateSemesters(ctx context.Context) ([]Semester, error) {
	_, terms, err := c.graduateScheduleSession(ctx)
	if err != nil {
		return nil, err
	}
	semesters := make([]Semester, 0, len(terms))
	for _, term := range terms {
		semesters = append(semesters, Semester(term))
	}
	return semesters, nil
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
	data, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1ScheduleAppURL, ustc.YJS1ScheduleRowsURL, ustc.YJS1ScheduleRowsForm(selected.Code))
	if err != nil {
		return Semester{}, nil, err
	}
	rows, err := ustc.ParseYJS1Curriculum(data, selected.ID)
	if err != nil {
		return Semester{}, nil, err
	}
	items := make([]CurriculumItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, CurriculumItem(row))
	}
	return selected, items, nil
}

func (c *Client) graduateScheduleSession(ctx context.Context) (*http.Client, []ustc.YJS1Semester, error) {
	if c.graduateScheduleClient == nil {
		client, err := newGraduateYJS1Client(ctx, c.creds, ustc.YJS1ScheduleAppURL)
		if err != nil {
			return nil, nil, err
		}
		c.graduateScheduleClient = client
	}
	if c.graduateScheduleTerms == nil {
		terms, err := fetchGraduateYJS1Terms(ctx, c.graduateScheduleClient, ustc.YJS1ScheduleAppURL, ustc.YJS1ScheduleTermURL, ustc.YJS1ScheduleTermsForm())
		if err != nil {
			return nil, nil, err
		}
		c.graduateScheduleTerms = terms
	}
	return c.graduateScheduleClient, c.graduateScheduleTerms, nil
}

func (c *Client) fetchGraduateExams(ctx context.Context, semesterID int) ([]ExamItem, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, ustc.YJS1ExamAppURL)
	if err != nil {
		return nil, err
	}
	return fetchGraduateYJS1Exams(ctx, client, semesterID)
}

func fetchGraduateYJS1Exams(ctx context.Context, client *http.Client, semesterID int) ([]ExamItem, error) {
	terms, err := fetchGraduateYJS1Terms(ctx, client, ustc.YJS1ExamAppURL, ustc.YJS1ExamTermURL, nil)
	if err != nil {
		return nil, err
	}
	selected, err := pickGraduateYJS1Term(terms, semesterID)
	if err != nil {
		return nil, err
	}
	data, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1ExamAppURL, ustc.YJS1ExamRowsURL, ustc.YJS1ExamRowsForm(selected.Code))
	if err != nil {
		return nil, err
	}
	rows, err := ustc.ParseYJS1Exams(data)
	if err != nil {
		return nil, err
	}
	items := make([]ExamItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ExamItem(row))
	}
	return items, nil
}

func (c *Client) fetchGraduateScores(ctx context.Context) (ScoreReport, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, ustc.YJS1ScoreAppURL)
	if err != nil {
		return ScoreReport{}, err
	}
	return fetchGraduateYJS1ScoreReport(ctx, client)
}

func fetchGraduateYJS1ScoreReport(ctx context.Context, client *http.Client) (ScoreReport, error) {
	infoData, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1ScoreAppURL, ustc.YJS1ScoreInfoURL, nil)
	if err != nil {
		return ScoreReport{}, err
	}
	info, err := ustc.ParseYJS1ScoreInfo(infoData)
	if err != nil {
		return ScoreReport{}, err
	}
	profileData, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1ScoreAppURL, ustc.YJS1ScoreProfileURL, nil)
	if err != nil {
		return ScoreReport{}, err
	}
	profile, err := ustc.ParseYJS1ScoreProfile(profileData)
	if err != nil {
		return ScoreReport{}, err
	}
	data, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1ScoreAppURL, ustc.YJS1ScoreRowsURL, ustc.YJS1ScoreRowsForm())
	if err != nil {
		return ScoreReport{}, err
	}
	parsed, err := ustc.ParseYJS1Scores(data, info, profile)
	if err != nil {
		return ScoreReport{}, err
	}
	report := ScoreReport{Summary: parsed.Summary, Items: make([]ScoreItem, 0, len(parsed.Items))}
	for _, item := range parsed.Items {
		report.Items = append(report.Items, ScoreItem(item))
	}
	return report, nil
}

func (c *Client) fetchGraduateHomework(ctx context.Context) ([]HomeworkItem, error) {
	client, err := newGraduateYJS1Client(ctx, c.creds, ustc.YJS1HomeworkAppURL)
	if err != nil {
		return nil, err
	}
	return fetchGraduateYJS1Homework(ctx, client)
}

func fetchGraduateYJS1Homework(ctx context.Context, client *http.Client) ([]HomeworkItem, error) {
	terms, err := fetchGraduateYJS1Terms(ctx, client, ustc.YJS1HomeworkAppURL, ustc.YJS1HomeworkTermURL, ustc.YJS1HomeworkTermsForm())
	if err != nil {
		return nil, err
	}
	selected, err := pickGraduateYJS1Term(terms, 0)
	if err != nil {
		return nil, err
	}
	userID, err := fetchGraduateYJS1UserID(ctx, client, ustc.YJS1HomeworkAppURL)
	if err != nil {
		return nil, err
	}
	data, err := fetchGraduateYJS1JSON(ctx, client, ustc.YJS1HomeworkAppURL, ustc.YJS1HomeworkRowsURL, ustc.YJS1HomeworkRowsForm(selected.Code, userID))
	if err != nil {
		return nil, err
	}
	rows, err := ustc.ParseYJS1Homework(data, time.Now(), time.Local)
	if err != nil {
		return nil, err
	}
	items := make([]HomeworkItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, HomeworkItem{
			ID: row.ID, Title: row.Title, CourseName: row.CourseName,
			StartAt: row.StartAt, EndAt: row.EndAt, Status: row.Status,
		})
	}
	return items, nil
}

func fetchGraduateYJS1Terms(ctx context.Context, client *http.Client, appPageURL, rawURL string, form url.Values) ([]ustc.YJS1Semester, error) {
	data, err := fetchGraduateYJS1JSON(ctx, client, appPageURL, rawURL, form)
	if err != nil {
		return nil, err
	}
	return ustc.ParseYJS1Terms(data)
}

func fetchGraduateYJS1JSON(ctx context.Context, client *http.Client, appPageURL, rawURL string, form url.Values) (json.RawMessage, error) {
	var data json.RawMessage
	if err := postGraduateYJS1FormJSON(ctx, client, appPageURL, rawURL, form, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func newGraduateYJS1Client(ctx context.Context, creds Credentials, appPageURL string) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      ustc.CASServiceLoginURL(appPageURL),
		expectedHost:  "yjs1.ustc.edu.cn",
		postLoginURLs: []string{appPageURL},
	})
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

	return ustc.ParseYJS1UserID(body, appPageURL)
}

func pickGraduateYJS1Term(terms []ustc.YJS1Semester, semesterID int) (Semester, error) {
	if len(terms) == 0 {
		return Semester{}, fmt.Errorf("no graduate semesters available")
	}
	if semesterID == 0 {
		for _, term := range terms {
			if term.IsLast {
				return Semester(term), nil
			}
		}
		return Semester(terms[0]), nil
	}
	for _, term := range terms {
		if term.ID == semesterID {
			return Semester(term), nil
		}
	}
	return Semester{}, fmt.Errorf("graduate semester %d was not found", semesterID)
}
