package school

import (
	"slices"
	"testing"

	"github.com/Life-USTC/CLI/internal/openapi"
	ustcschool "github.com/Life-USTC/CLI/internal/school"
)

func TestResolveLifeSemesterMatchesJWID(t *testing.T) {
	t.Parallel()

	id, row, ok := resolveLifeSemester(map[string]any{
		"data": []any{
			map[string]any{
				"id":     float64(77),
				"jwId":   float64(362),
				"code":   "362",
				"nameCn": "2024年秋季学期",
			},
		},
	}, ustcschool.Semester{ID: 362, Code: "20241", SemesterCn: "2024年秋季学期"})
	if !ok {
		t.Fatal("resolveLifeSemester() did not find a matching semester")
	}
	if id != "77" {
		t.Fatalf("resolveLifeSemester() id = %q, want %q", id, "77")
	}
	if got := row["nameCn"]; got != "2024年秋季学期" {
		t.Fatalf("resolveLifeSemester() nameCn = %v, want %q", got, "2024年秋季学期")
	}
}

func TestResolveLifeSemesterFallsBackToSemesterIDCode(t *testing.T) {
	t.Parallel()

	id, _, ok := resolveLifeSemester(map[string]any{
		"data": []any{
			map[string]any{
				"id":     "77",
				"code":   "362",
				"nameCn": "2024年秋季学期",
			},
		},
	}, ustcschool.Semester{ID: 362, Code: "20241"})
	if !ok {
		t.Fatal("resolveLifeSemester() did not match by semester ID fallback")
	}
	if id != "77" {
		t.Fatalf("resolveLifeSemester() id = %q, want %q", id, "77")
	}
}

func TestResolveLifeSemesterReturnsFalseWithoutMatch(t *testing.T) {
	t.Parallel()

	if _, _, ok := resolveLifeSemester(map[string]any{
		"data": []any{
			map[string]any{
				"id":     float64(71),
				"jwId":   float64(421),
				"code":   "421",
				"nameCn": "2026年春季学期",
			},
		},
	}, ustcschool.Semester{ID: 362, Code: "20241"}); ok {
		t.Fatal("resolveLifeSemester() unexpectedly found a match")
	}
}

func TestNewSemesterScopedSubscriptionSetBody(t *testing.T) {
	t.Parallel()

	sectionIDs := []int{11, 12}
	body := newSemesterScopedSubscriptionSetBody(sectionIDs, "77")
	if body.Action != openapi.CalendarSubscriptionBatchRequestSchemaActionSet {
		t.Fatalf("body.Action = %q, want set", body.Action)
	}
	if body.SectionIds == nil || !slices.Equal(*body.SectionIds, sectionIDs) {
		t.Fatalf("body.SectionIds = %v, want %v", body.SectionIds, sectionIDs)
	}
	if body.SemesterId == nil {
		t.Fatal("body.SemesterId is nil")
	}
	value, err := body.SemesterId.AsCalendarSubscriptionBatchRequestSchemaSemesterId0()
	if err != nil {
		t.Fatalf("semester ID is not a string: %v", err)
	}
	if value != "77" {
		t.Fatalf("semester ID = %q, want 77", value)
	}
}

func TestSchoolHomeworkCompleted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   bool
	}{
		{status: "submitted", want: true},
		{status: "graded", want: true},
		{status: "pending", want: false},
		{status: "overdue", want: false},
	}
	for _, tc := range tests {
		got := schoolHomeworkCompleted(ustcschool.HomeworkItem{Status: tc.status})
		if got != tc.want {
			t.Fatalf("schoolHomeworkCompleted(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestSchoolHomeworkCompletionKnown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status       string
		wantComplete bool
		wantKnown    bool
	}{
		{status: "submitted", wantComplete: true, wantKnown: true},
		{status: "graded", wantComplete: true, wantKnown: true},
		{status: "pending", wantComplete: false, wantKnown: true},
		{status: "overdue", wantComplete: false, wantKnown: true},
		{status: "已评分", wantComplete: true, wantKnown: true},
		{status: "已提交", wantComplete: true, wantKnown: true},
		{status: "需要评分", wantComplete: true, wantKnown: true},
		{status: "尚未评分", wantComplete: false, wantKnown: true},
		{status: "作业", wantComplete: false, wantKnown: false},
	}
	for _, tc := range tests {
		gotComplete, gotKnown := schoolHomeworkCompletion(ustcschool.HomeworkItem{Status: tc.status})
		if gotComplete != tc.wantComplete || gotKnown != tc.wantKnown {
			t.Fatalf("schoolHomeworkCompletion(%q) = (%v, %v), want (%v, %v)", tc.status, gotComplete, gotKnown, tc.wantComplete, tc.wantKnown)
		}
	}
}

func TestParseBlackboardSemesterCode(t *testing.T) {
	t.Parallel()

	year, term, ok := parseBlackboardSemesterCode("2023FA")
	if !ok {
		t.Fatal("parseBlackboardSemesterCode() did not parse valid code")
	}
	if year != "2023" || term != "秋季" {
		t.Fatalf("parseBlackboardSemesterCode() = (%q, %q), want (%q, %q)", year, term, "2023", "秋季")
	}
}

func TestMatchLifeHomeworkByTitleAndDueTime(t *testing.T) {
	t.Parallel()

	existing := []map[string]any{
		{
			"id":              "hw-1",
			"title":           "  组合数学第四次作业 ",
			"submissionDueAt": "2026-06-02T23:59:00+08:00",
		},
	}
	got := matchLifeHomework(existing, ustcschool.HomeworkItem{
		Title: "组合数学第四次作业",
		EndAt: "2026-06-02 23:59:00",
	})
	if got == nil {
		t.Fatal("matchLifeHomework() did not find expected homework")
	}
	if got["id"] != "hw-1" {
		t.Fatalf("matchLifeHomework() id = %v, want hw-1", got["id"])
	}
}

func TestMatchLifeHomeworkRequiresSameDueTime(t *testing.T) {
	t.Parallel()

	existing := []map[string]any{
		{
			"id":              "hw-1",
			"title":           "组合数学第四次作业",
			"submissionDueAt": "2026-06-03T23:59:00+08:00",
		},
	}
	got := matchLifeHomework(existing, ustcschool.HomeworkItem{
		Title: "组合数学第四次作业",
		EndAt: "2026-06-02 23:59:00",
	})
	if got != nil {
		t.Fatalf("matchLifeHomework() = %v, want nil", got)
	}
}
