package school

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseScoreReportHTML(t *testing.T) {
	const page = `
<html><body>
  <div class="rankinfo"><div><em>2021年秋季学期</em>–<em>2024年秋季学期</em>平均学分绩点（GPA）为 <b>3.01</b>，排名情况：<br>全校2021年级信息与计算科学专业 GPA 排名 <b>63</b>/90</div></div>
  <div class="tab-content">
    <div class="tab-pane active">
      <div class="overview"><ul>
        <li><span>总学分</span><span>160</span></li>
        <li><span>已获学分</span><span>160</span></li>
        <li><span>不及格学分</span><span>0</span></li>
        <li><span>GPA</span><span>3.08</span></li>
        <li><span>加权平均分</span><span>81.05</span></li>
        <li><span>算术平均分</span><span>81.68</span></li>
      </ul></div>
      <div class="semesters">
        <div class="semester">
          <h4>2025年春季学期</h4>
          <table class="course-table"><tbody>
            <tr><td class="course-name">毕业论文<small>THESIS</small></td><td>320</td><td>8</td><td>4.3</td><td>A+</td></tr>
          </tbody></table>
        </div>
        <div class="semester">
          <h4>2024年秋季学期</h4>
          <table class="course-table"><tbody>
            <tr><td class="course-name">芯片科技概论<small>IC1901</small></td><td>40</td><td>2</td><td>4.3</td><td>97</td></tr>
            <tr><td class="course-name">思想政治理论课实践<small>MARX1005</small></td><td>80</td><td>2</td><td></td><td>通过</td></tr>
          </tbody></table>
        </div>
      </div>
    </div>
  </div>
  <table class="history-table"><thead><tr><th class="semesterName"><select>
    <option value="" selected="selected">所有学期</option>
    <option value="381">2025年春季学期</option>
    <option value="362">2024年秋季学期</option>
  </select></th></tr></thead></table>
</body></html>`

	report, err := parseScoreReportHTML(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parseScoreReportHTML returned error: %v", err)
	}

	if len(report.Items) != 3 {
		t.Fatalf("expected 3 score items, got %d", len(report.Items))
	}
	if report.Items[0].SemesterID != 381 || report.Items[0].CourseName != "毕业论文" || report.Items[0].CourseCode != "THESIS" {
		t.Fatalf("unexpected first item: %+v", report.Items[0])
	}
	if report.Items[1].SemesterID != 362 || report.Items[1].CourseName != "思想政治理论课实践" || report.Items[1].Score != "通过" {
		t.Fatalf("unexpected second item: %+v", report.Items[1])
	}
	if report.Items[1].GradePoint != 0 {
		t.Fatalf("expected empty grade point to parse as zero, got %v", report.Items[1].GradePoint)
	}

	var summary map[string]any
	if err := json.Unmarshal(report.Summary, &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if summary["fromSemester"] != "2021年秋季学期" || summary["toSemester"] != "2024年秋季学期" {
		t.Fatalf("unexpected summary semesters: %+v", summary)
	}
	if summary["ranking"] != "全校2021年级信息与计算科学专业 GPA 排名 63 /90" && summary["ranking"] != "全校2021年级信息与计算科学专业 GPA 排名 63/90" {
		t.Fatalf("unexpected ranking summary: %+v", summary["ranking"])
	}
	if summary["gpa"] != 3.08 {
		t.Fatalf("unexpected gpa summary: %+v", summary["gpa"])
	}
}
