package school

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type courseTableTransport func(*http.Request) (*http.Response, error)

func (f courseTableTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCourseTableTransportAndEmbeddedProjection(t *testing.T) {
	t.Parallel()
	var requests []string
	client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String())
		responseRequest := r.Clone(r.Context())
		body := ""
		if r.URL.Path == "/for-std/course-table" {
			responseRequest.URL, _ = url.Parse("https://jw.ustc.edu.cn/for-std/course-table/info/0012345")
		} else {
			body = `{"studentTableVm":{"lessonIds":[30,10,30],"lessons":[{"id":30,"code":"CS.01","course":{"code":"CS","nameZh":"计算机","credits":4},"dateTimePlacePersonText":{"cn":"星期一"},"teacherAssignmentList":[{"teacher":{"nameZh":"张老师"}}]}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: responseRequest}, nil
	})}
	student, err := fetchStudentID(context.Background(), client)
	if err != nil || student != "0012345" {
		t.Fatalf("student=%q err=%v", student, err)
	}
	ids, lessons, err := fetchCurrentCourseTable(context.Background(), client, 301, student)
	if err != nil || !reflect.DeepEqual(ids, []int{10, 30}) || len(lessons) != 1 {
		t.Fatalf("ids=%v lessons=%v err=%v", ids, lessons, err)
	}
	item := lessons[0].toCurriculumItem(301)
	if item.CourseName != "计算机" || item.CourseCode != "CS" || item.Credits != 4 {
		t.Fatalf("projection=%+v", item)
	}
	want := []string{"https://jw.ustc.edu.cn/for-std/course-table", "https://jw.ustc.edu.cn/for-std/course-table/get-data?bizTypeId=2&semesterId=301&dataId=0012345"}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests=%v", requests)
	}
}

func TestCourseTableRetainsEmptySemesterAndTransportFailures(t *testing.T) {
	t.Parallel()
	networkErr := errors.New("offline")
	for _, tc := range []struct {
		name, body   string
		status       int
		transportErr error
		empty        bool
	}{
		{name: "empty semester", body: `{"lessonIds":[]}`, status: 200, empty: true},
		{name: "malformed response", body: `{`, status: 200},
		{name: "upstream failure", body: `unavailable`, status: 503},
		{name: "transport failure", transportErr: networkErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: courseTableTransport(func(r *http.Request) (*http.Response, error) {
				if tc.transportErr != nil {
					return nil, tc.transportErr
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header), Request: r}, nil
			})}
			_, _, err := fetchCurrentCourseTable(context.Background(), client, 301, "12345")
			if err == nil {
				t.Fatal("expected error")
			}
			if IsNoLessonData(err) != tc.empty {
				t.Fatalf("empty-semester classification: %v", err)
			}
			if tc.transportErr != nil && !errors.Is(err, networkErr) {
				t.Fatalf("lost transport error: %v", err)
			}
		})
	}
}
