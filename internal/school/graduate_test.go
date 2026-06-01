package school

import (
	"reflect"
	"testing"
	"time"
)

func TestGraduateYJS1SemestersFromTermsMarksCurrent(t *testing.T) {
	t.Parallel()

	items := graduateYJS1SemestersFromTerms([]graduateYJS1Term{
		{DM: "20251", MC: "2025年秋季学期", SFDQXQ: "0", QSSJ: "2025-09-01", JZSJ: "2026-01-20"},
		{DM: "20252", MC: "2026年春季学期", SFDQXQ: "1", QSSJ: "2026-02-23", JZSJ: "2026-07-05"},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 semesters, got %d", len(items))
	}
	if items[0].IsLast {
		t.Fatal("first semester should not be current")
	}
	if !items[1].IsLast {
		t.Fatal("second semester should be current")
	}
}

func TestGraduateYJS1CurriculumItemsFromRows(t *testing.T) {
	t.Parallel()

	items := graduateYJS1CurriculumItemsFromRows(Semester{ID: 20252}, []graduateYJS1ScheduleRow{
		{
			WID:    "abc-row",
			BJMC:   "SA25113014.01",
			KCDM:   "Y0203220",
			KCMC:   "软件体系结构分析与设计",
			PKSJDD: "周一 3-4节 软件楼101; 周三 7-8节 软件楼102",
			RKJS:   "张三、李四",
			ZCMC:   "1~8; 9~16",
		},
	})

	want := []CurriculumItem{
		{
			SemesterID: 20252,
			LessonCode: "SA25113014.01",
			CourseCode: "Y0203220",
			CourseName: "软件体系结构分析与设计",
			Teachers:   []string{"张三", "李四"},
			Schedule:   "周一 3-4节 软件楼101 [1-8周]; 周三 7-8节 软件楼102 [9-16周]",
		},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("graduateYJS1CurriculumItemsFromRows() = %#v, want %#v", items, want)
	}
}

func TestGraduateYJS1ScoreItemsFromRows(t *testing.T) {
	t.Parallel()

	items := graduateYJS1ScoreItemsFromRows([]graduateYJS1ScoreRow{
		{
			XNXQDM:        "20252",
			XNXQDMDisplay: "2026年春季学期",
			BJMC:          "SA25113014.01",
			KCDM:          "Y0203220",
			KCMC:          "软件体系结构分析与设计",
			CJFZDMDisplay: "百分制",
			CJJLDisplay:   "正常考试",
			XF:            2.0,
			CJ:            87.0,
			JDZ:           3.7,
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected 1 score item, got %d", len(items))
	}
	if items[0].SemesterID != 20252 {
		t.Fatalf("unexpected semester ID: %d", items[0].SemesterID)
	}
	if items[0].Score != "87" {
		t.Fatalf("unexpected score string: %q", items[0].Score)
	}
	if items[0].GradeText != "百分制 / 正常考试" {
		t.Fatalf("unexpected grade text: %q", items[0].GradeText)
	}
	if items[0].Credits != 2 || items[0].GradePoint != 3.7 {
		t.Fatalf("unexpected numeric mapping: credits=%v gradePoint=%v", items[0].Credits, items[0].GradePoint)
	}
}

func TestGraduateYJS1HomeworkItemsFromRows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.Local)
	items := graduateYJS1HomeworkItemsFromRows([]graduateYJS1HomeworkRow{
		{
			WID:    "1",
			KCMC:   "课程A",
			ZYMC:   "作业A",
			ZYKSSJ: "2026-03-01 10:00:00",
			ZYJZSJ: "2026-03-09 23:59:00",
		},
		{
			WID:     "2",
			XSZYWID: "sub-2",
			KCMC:    "课程B",
			ZYMC:    "作业B",
			ZYKSSJ:  "2026-03-01 10:00:00",
			ZYJZSJ:  "2026-03-20 23:59:00",
		},
		{
			WID:    "3",
			KCMC:   "课程C",
			ZYMC:   "作业C",
			ZYKSSJ: "2026-03-01 10:00:00",
			ZYJZSJ: "2026-03-20 23:59:00",
			CJ:     95.0,
		},
	}, now)

	if len(items) != 3 {
		t.Fatalf("expected 3 homework items, got %d", len(items))
	}
	if items[0].Status != "overdue" {
		t.Fatalf("expected overdue homework first, got %q", items[0].Status)
	}
	if items[1].Status != "submitted" {
		t.Fatalf("expected submitted homework second after sorting by deadline, got %q", items[1].Status)
	}
	if items[2].Status != "graded" {
		t.Fatalf("expected graded homework last, got %q", items[2].Status)
	}
}
