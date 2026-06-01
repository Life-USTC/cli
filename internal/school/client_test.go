package school

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilterBlackboardCourseCalendarIDs(t *testing.T) {
	t.Parallel()

	calendars := []blackboardCalendar{
		{ID: "PERSONAL"},
		{ID: "CS1001.01.2024FA"},
		{ID: "INSTITUTION"},
		{ID: "MATH2001.02.2024FA"},
		{ID: "CS1001.01.2024FA"},
		{ID: ""},
	}

	got := filterBlackboardCourseCalendarIDs(calendars)
	want := []string{"CS1001.01.2024FA", "MATH2001.02.2024FA"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterBlackboardCourseCalendarIDs() = %v, want %v", got, want)
	}
}

func TestBatchCalendarIDs(t *testing.T) {
	t.Parallel()

	calendarIDs := []string{"A123", "BC456", "DEF789", "GHIJ000"}
	got := batchCalendarIDs(calendarIDs, 9)
	want := [][]string{{"A123"}, {"BC456"}, {"DEF789"}, {"GHIJ000"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batchCalendarIDs() = %v, want %v", got, want)
	}
}

func TestBuildBlackboardCalendarEventsURL(t *testing.T) {
	t.Parallel()

	got := buildBlackboardCalendarEventsURL([]string{"CS1001.01.2024FA", "MATH2001.02.2024FA"})
	want := "https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/events?start=0&end=4102444800000&course_id=&calendarIds=CS1001.01.2024FA,MATH2001.02.2024FA"
	if got != want {
		t.Fatalf("buildBlackboardCalendarEventsURL() = %q, want %q", got, want)
	}
}

func TestSplitBlackboardCalendarID(t *testing.T) {
	t.Parallel()

	code, semester := splitBlackboardCalendarID("MATH3011.01.2023FA")
	if code != "MATH3011.01" || semester != "2023FA" {
		t.Fatalf("splitBlackboardCalendarID() = (%q, %q), want (%q, %q)", code, semester, "MATH3011.01", "2023FA")
	}
}

func TestFetchBlackboardGradeStatusesParsesRows(t *testing.T) {
	t.Parallel()

	html := `<div id="grades_wrapper">
<div id="77235" class="sortable_item_row"><div class="cell activity timestamp"><span class="activityType">已评分</span></div></div>
<div id="59232" class="sortable_item_row"><div class="cell activity timestamp"><span class="activityType">尚未评分</span></div></div>
</div>`
	got := blackboardGradeStatusesFromHTML(strings.NewReader(html))
	if got["_77235_1"] != "已评分" {
		t.Fatalf("status _77235_1 = %q, want 已评分", got["_77235_1"])
	}
	if got["_59232_1"] != "尚未评分" {
		t.Fatalf("status _59232_1 = %q, want 尚未评分", got["_59232_1"])
	}
}

func TestParseExamsHTMLEmptyTableReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	items, err := parseExamsHTML(strings.NewReader(`
		<table>
			<tbody></tbody>
		</table>
	`))
	if err != nil {
		t.Fatalf("parseExamsHTML returned error: %v", err)
	}
	if items == nil {
		t.Fatal("parseExamsHTML returned nil slice, want empty slice")
	}
	if len(items) != 0 {
		t.Fatalf("parseExamsHTML returned %d items, want 0", len(items))
	}
}

func TestParseExamsHTMLParsesAndSortsRows(t *testing.T) {
	t.Parallel()

	items, err := parseExamsHTML(strings.NewReader(`
		<table>
			<tbody>
				<tr>
					<td>高等数学</td>
					<td>MATH1001.01</td>
					<td>期末</td>
					<td>2025-07-02 08:00</td>
					<td>三教3A101</td>
					<td>18</td>
					<td>待参加</td>
					<td>闭卷</td>
				</tr>
				<tr>
					<td>程序设计基础</td>
					<td>CS1001.01</td>
					<td>期中</td>
					<td>2025-06-30 14:00</td>
					<td>东区2A201</td>
					<td>27</td>
					<td>待参加</td>
					<td>机考</td>
				</tr>
			</tbody>
		</table>
	`))
	if err != nil {
		t.Fatalf("parseExamsHTML returned error: %v", err)
	}

	want := []ExamItem{
		{
			CourseName: "程序设计基础",
			LessonCode: "CS1001.01",
			ExamType:   "期中",
			DateTime:   "2025-06-30 14:00",
			Location:   "东区2A201",
			Seat:       "27",
			Status:     "待参加",
		},
		{
			CourseName: "高等数学",
			LessonCode: "MATH1001.01",
			ExamType:   "期末",
			DateTime:   "2025-07-02 08:00",
			Location:   "三教3A101",
			Seat:       "18",
			Status:     "待参加",
		},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("parseExamsHTML() = %#v, want %#v", items, want)
	}
}
