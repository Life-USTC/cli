package school

import (
	"reflect"
	"strings"
	"testing"
)

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
