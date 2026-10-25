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

const catalogTestSemestersURL = "https://catalog.ustc.edu.cn/api/teach/semester/list"

func TestCatalogSemesterRequestAndMetadataProjection(t *testing.T) {
	t.Parallel()
	body := `[{"id":301,"code":" code ","semesterCn":" 中文 ","semesterEn":" English ","startDate":"start","endDate":"end","isLast":true},{"id":101},{"id":301}] trailing`
	want := []Semester{
		{ID: 301, Code: " code ", SemesterCn: " 中文 ", SemesterEn: " English ", StartDate: "start", EndDate: "end", IsLast: true},
		{ID: 101}, {ID: 301},
	}
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != catalogTestSemestersURL || r.Method != http.MethodGet || r.Header.Get("Accept") != "" || r.Header.Get("User-Agent") != "" {
			t.Fatalf("catalog semester request changed: %s %s %v", r.Method, r.URL, r.Header)
		}
		return importParserResponse(r, http.StatusOK, body), nil
	})}
	got, err := fetchCatalogSemesters(context.Background(), client, func() (*http.Client, error) { t.Fatal("unexpected JW fallback"); return nil, nil })
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("semesters = %#v, %v; want %#v", got, err, want)
	}
	jw := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters" || r.Header.Get("Accept") != "application/json, text/plain, */*" || r.Header.Get("User-Agent") != schoolUserAgent {
			t.Fatalf("JW metadata request changed: %s %v", r.URL, r.Header)
		}
		return importParserResponse(r, http.StatusOK, body), nil
	})}
	got, err = fetchJWScoreSemesters(context.Background(), jw)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("JW semesters = %#v, %v; want %#v", got, err, want)
	}
}

func TestCatalogSemesterFallbackRemainsLazy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		authError bool
		wantError string
		wantCalls int
	}{
		{name: "success", status: 200, body: `[]`, wantCalls: 1},
		{name: "invalid JSON is not a fallback", status: 200, body: `!`, wantError: "decode semesters:", wantCalls: 1},
		{name: "wrong shape is not a fallback", status: 200, body: `{}`, wantError: "decode semesters:", wantCalls: 1},
		{name: "HTTP fallback", status: 403, body: "catalog denied", wantCalls: 2},
		{name: "fallback login failure retains catalog error", status: 403, body: "catalog denied", authError: true, wantError: "GET " + catalogTestSemestersURL + " returned 403 Forbidden: catalog denied", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			transport := courseTableTransport(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.String())
				if r.URL.String() == catalogTestSemestersURL {
					return importParserResponse(r, tc.status, tc.body), nil
				}
				if r.URL.String() != "https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters" {
					t.Fatalf("unexpected fallback URL %s", r.URL)
				}
				return importParserResponse(r, 200, `[{"id":17}]`), nil
			})
			client := &http.Client{Transport: transport}
			factories := 0
			items, err := fetchCatalogSemesters(context.Background(), client, func() (*http.Client, error) {
				factories++
				if len(requests) != 1 {
					t.Fatalf("fallback invoked before catalog request: %v", requests)
				}
				if tc.authError {
					return nil, errors.New("login failed")
				}
				return client, nil
			})
			if len(requests) != tc.wantCalls || (tc.status >= 300 && factories != 1) || (tc.status < 300 && factories != 0) {
				t.Fatalf("fallback requests=%v factories=%d", requests, factories)
			}
			if tc.wantError != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantError) || items != nil {
					t.Fatalf("fallback error = %#v, %v", items, err)
				}
			} else if err != nil {
				t.Fatal(err)
			} else if tc.status >= 300 && !reflect.DeepEqual(items, []Semester{{ID: 17}}) {
				t.Fatalf("fallback items = %#v", items)
			}
		})
	}
}

func TestCatalogAndScoreMetadataEmptyArrays(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`null`, `[]`} {
		t.Run(body, func(t *testing.T) {
			client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) { return importParserResponse(r, 200, body), nil })}
			catalog, err := fetchCatalogSemesters(context.Background(), client, func() (*http.Client, error) { t.Fatal("unexpected fallback"); return nil, nil })
			if err != nil {
				t.Fatal(err)
			}
			semesters, err := fetchJWScoreSemesters(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			types, err := fetchJWScoreTypes(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []any{catalog, semesters, types} {
				encoded, _ := json.Marshal(value)
				if string(encoded) != body {
					t.Fatalf("metadata output = %s, want %s", encoded, body)
				}
			}
		})
	}
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		return importParserResponse(r, 200, `[{"id":7},{"id":3},{"id":7}] {}`), nil
	})}
	types, err := fetchJWScoreTypes(context.Background(), client)
	if err != nil || len(types) != 3 || types[0].ID != 7 || types[1].ID != 3 || types[2].ID != 7 {
		t.Fatalf("transcript order = %#v, %v", types, err)
	}
}

func TestCatalogLessonPagesAndDirectResponses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		pages     []string
		ids       []int
		nilResult bool
	}{
		{"paged order and duplicates", []string{`{"rows":[{"id":3},{"id":1}],"totalPage":2}`, `{"rows":[{"id":3},{"id":2}],"totalPage":2}`}, []int{3, 1, 3, 2}, false},
		{"direct replaces previous page", []string{`{"rows":[{"id":9}],"totalPage":2}`, `[{"id":1}]`}, []int{1}, false},
		{"direct null replaces previous page", []string{`{"rows":[{"id":9}],"totalPage":2}`, `null`}, nil, true},
		{"direct empty", []string{`[]`}, nil, false},
		{"direct null", []string{`null`}, nil, true},
		{"paged empty", []string{`{"rows":[],"totalPage":0}`}, nil, true},
		{"missing total", []string{`{"rows":[{"id":5}]}`}, []int{5}, false},
		{"negative total", []string{`{"rows":[{"id":5}],"totalPage":-1}`}, []int{5}, false},
		{"empty page still advances", []string{`{"rows":[],"totalPage":2}`, `{"rows":[{"id":4}],"totalPage":2}`}, []int{4}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				want := fmt.Sprintf("https://catalog.ustc.edu.cn/api/teach/lesson/list-for-teach/301?page=%d&pageSize=500", calls)
				if r.Method != http.MethodGet || r.URL.String() != want || calls > len(tc.pages) {
					t.Fatalf("page request = %s %s, want %s", r.Method, r.URL, want)
				}
				return importParserResponse(r, 200, tc.pages[calls-1]), nil
			})}
			lessons, err := fetchCatalogLessons(context.Background(), client, 301)
			if err != nil || calls != len(tc.pages) || (lessons == nil) != tc.nilResult {
				t.Fatalf("pages = %#v, %v, calls=%d", lessons, err, calls)
			}
			var ids []int
			for _, lesson := range lessons {
				ids = append(ids, lesson.ID)
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("lesson IDs = %v, want %v", ids, tc.ids)
			}
		})
	}
}

func TestCatalogLessonFieldsReachCLIProjection(t *testing.T) {
	t.Parallel()
	body := `[{"id":42,"code":" A.01 ","courseCode":"OUT","courseName":"Outer","credits":4,"dateTimePlacePersonText":{"cn":" 中文安排 ","en":"English"},"course":{"code":"INNER","nameZh":"中文","nameEn":"English","cn":"CN","en":"EN","credits":2},"teacherAssignmentList":[{"cn":" C ","en":" E ","teacher":{"nameZh":" 中 ","nameEn":" English "}},{"cn":"C"}]},{"id":5,"course":{"code":"INNER","nameEn":"English","credits":2},"dateTimePlacePersonText":" raw  schedule ","teacherAssignmentList":[{"en":"EN"}]}]`
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) { return importParserResponse(r, 200, body), nil })}
	lessons, err := fetchCatalogLessons(context.Background(), client, 301)
	if err != nil || len(lessons) != 2 {
		t.Fatalf("lessons = %#v, %v", lessons, err)
	}
	want := []CurriculumItem{
		{SemesterID: 301, LessonID: 42, LessonCode: " A.01 ", CourseCode: "OUT", CourseName: "Outer", Credits: 4, Schedule: "中文安排", Teachers: []string{"中", "C"}},
		{SemesterID: 301, LessonID: 5, CourseCode: "INNER", CourseName: "English", Credits: 2, Schedule: "raw schedule", Teachers: []string{"EN"}},
	}
	for i, lesson := range lessons {
		if got := lesson.toCurriculumItem(301); !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("CLI lesson = %#v, want %#v", got, want[i])
		}
	}
	if lessons[0].Course.CN != "CN" || lessons[0].TeacherAssignmentList[0].EN != " E " || lessons[0].TeacherAssignmentList[0].Teacher.NameEn != " English " {
		t.Fatalf("nested fields lost: %#v", lessons[0])
	}
}

func TestCatalogAndMetadataFailuresKeepContext(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []struct {
		name, prefix, url string
		fetch             func(*http.Client) error
		catalogPage       bool
	}{
		{"catalog semesters", "decode semesters: ", catalogTestSemestersURL, func(c *http.Client) error {
			_, err := fetchCatalogSemesters(context.Background(), c, func() (*http.Client, error) { t.Fatal("unexpected fallback"); return nil, nil })
			return err
		}, false},
		{"score semesters", "fetch score semesters: decode https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters: ", "https://jw.ustc.edu.cn/for-std/grade/sheet/getSemesters", func(c *http.Client) error { _, err := fetchJWScoreSemesters(context.Background(), c); return err }, false},
		{"score types", "fetch score transcript types: decode https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeSheetTypes: ", "https://jw.ustc.edu.cn/for-std/grade/sheet/getGradeSheetTypes", func(c *http.Client) error { _, err := fetchJWScoreTypes(context.Background(), c); return err }, false},
		{"catalog page", "decode catalog lessons: ", "https://catalog.ustc.edu.cn/api/teach/lesson/list-for-teach/301?page=2&pageSize=500", func(c *http.Client) error {
			rows, err := fetchCatalogLessons(context.Background(), c, 301)
			if err != nil && rows != nil {
				t.Errorf("returned partial lessons on failure: %#v", rows)
			}
			return err
		}, true},
	} {
		failures := []string{"network", "read", "syntax", "type"}
		if endpoint.catalogPage {
			failures = append(failures, "HTTP", "trailing document")
		}
		for _, failure := range failures {
			t.Run(endpoint.name+"/"+failure, func(t *testing.T) {
				calls := 0
				sentinel := errors.New("catalog fixture failure")
				client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					if endpoint.catalogPage && calls == 1 {
						return importParserResponse(r, 200, `{"rows":[{"id":7}],"totalPage":2}`), nil
					}
					if r.URL.String() != endpoint.url {
						t.Fatalf("failure URL = %s, want %s", r.URL, endpoint.url)
					}
					if failure == "network" {
						return nil, sentinel
					}
					response := importParserResponse(r, 200, `!`)
					switch failure {
					case "read":
						response.Body = io.NopCloser(importParserReadError{err: sentinel})
					case "type":
						response.Body = io.NopCloser(strings.NewReader(`[{"id":"7"}]`))
					case "HTTP":
						response = importParserResponse(r, 503, " unavailable ")
					case "trailing document":
						response.Body = io.NopCloser(strings.NewReader(`[] {}`))
					}
					return response, nil
				})}
				err := endpoint.fetch(client)
				if err == nil {
					t.Fatal("expected failure")
				}
				if endpoint.catalogPage && calls != 2 {
					t.Fatalf("page calls = %d", calls)
				}
				switch failure {
				case "network", "read":
					if !errors.Is(err, sentinel) {
						t.Fatalf("lost failure cause: %v", err)
					}
				case "syntax", "type", "trailing document":
					if !strings.HasPrefix(err.Error(), endpoint.prefix) {
						t.Fatalf("decode context = %v, want %s", err, endpoint.prefix)
					}
				case "HTTP":
					if err.Error() != "GET "+endpoint.url+" returned 503 Service Unavailable: unavailable" {
						t.Fatalf("HTTP context = %v", err)
					}
				}
			})
		}
	}
}
