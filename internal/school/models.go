package school

import (
	"encoding/json"
	"sort"
	"strings"
)

type Credentials struct {
	Username string
	Password string
	TOTP     string
}

type Semester struct {
	ID         int    `json:"id"`
	Code       string `json:"code,omitempty"`
	SemesterCn string `json:"semesterCn,omitempty"`
	SemesterEn string `json:"semesterEn,omitempty"`
	StartDate  string `json:"startDate,omitempty"`
	EndDate    string `json:"endDate,omitempty"`
	IsLast     bool   `json:"isLast,omitempty"`
}

func (s Semester) Name() string {
	if s.SemesterCn != "" {
		return s.SemesterCn
	}
	if s.SemesterEn != "" {
		return s.SemesterEn
	}
	if s.Code != "" {
		return s.Code
	}
	return ""
}

type CurriculumItem struct {
	SemesterID int      `json:"semesterId"`
	LessonID   int      `json:"lessonId,omitempty"`
	LessonCode string   `json:"lessonCode,omitempty"`
	CourseCode string   `json:"courseCode,omitempty"`
	CourseName string   `json:"courseName,omitempty"`
	Credits    float64  `json:"credits,omitempty"`
	Teachers   []string `json:"teachers,omitempty"`
	Schedule   string   `json:"schedule,omitempty"`
}

func (i CurriculumItem) TeacherList() string {
	return strings.Join(i.Teachers, ", ")
}

type ExamItem struct {
	CourseName string `json:"courseName,omitempty"`
	LessonCode string `json:"lessonCode,omitempty"`
	ExamType   string `json:"examType,omitempty"`
	DateTime   string `json:"dateTime,omitempty"`
	Location   string `json:"location,omitempty"`
	Seat       string `json:"seat,omitempty"`
	Status     string `json:"status,omitempty"`
}

type ScoreReport struct {
	Summary json.RawMessage `json:"summary,omitempty"`
	Items   []ScoreItem     `json:"items"`
}

type ScoreItem struct {
	SemesterID   int     `json:"semesterId"`
	SemesterName string  `json:"semesterName,omitempty"`
	CourseName   string  `json:"courseName,omitempty"`
	LessonCode   string  `json:"lessonCode,omitempty"`
	CourseCode   string  `json:"courseCode,omitempty"`
	Credits      float64 `json:"credits,omitempty"`
	GradePoint   float64 `json:"gradePoint,omitempty"`
	Score        string  `json:"score,omitempty"`
	GradeText    string  `json:"gradeText,omitempty"`
}

type HomeworkItem struct {
	ID             string `json:"id,omitempty"`
	Title          string `json:"title,omitempty"`
	CourseName     string `json:"courseName,omitempty"`
	LessonCode     string `json:"lessonCode,omitempty"`
	SemesterCode   string `json:"semesterCode,omitempty"`
	ExternalItemID string `json:"externalItemId,omitempty"`
	StartAt        string `json:"startAt,omitempty"`
	EndAt          string `json:"endAt,omitempty"`
	Status         string `json:"status,omitempty"`
}

func SortCurriculum(items []CurriculumItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].LessonCode != items[j].LessonCode {
			return items[i].LessonCode < items[j].LessonCode
		}
		if items[i].CourseName != items[j].CourseName {
			return items[i].CourseName < items[j].CourseName
		}
		return items[i].LessonID < items[j].LessonID
	})
}

func SortExams(items []ExamItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].DateTime != items[j].DateTime {
			return items[i].DateTime < items[j].DateTime
		}
		if items[i].CourseName != items[j].CourseName {
			return items[i].CourseName < items[j].CourseName
		}
		return items[i].LessonCode < items[j].LessonCode
	})
}

func SortScores(items []ScoreItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].SemesterID != items[j].SemesterID {
			return items[i].SemesterID > items[j].SemesterID
		}
		if items[i].CourseName != items[j].CourseName {
			return items[i].CourseName < items[j].CourseName
		}
		return items[i].LessonCode < items[j].LessonCode
	})
}

func SortHomework(items []HomeworkItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].EndAt != items[j].EndAt {
			return items[i].EndAt < items[j].EndAt
		}
		if items[i].CourseName != items[j].CourseName {
			return items[i].CourseName < items[j].CourseName
		}
		return items[i].Title < items[j].Title
	})
}
