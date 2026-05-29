package todo

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/fatih/color"
)

func TestMain(m *testing.M) {
	color.NoColor = true // strip ANSI codes so string comparisons are deterministic
	os.Exit(m.Run())
}

// --- validTodoPriority ---

func TestValidTodoPriority(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"low", true},
		{"medium", true},
		{"high", true},
		{"invalid", false},
		{"", false},
		{"HIGH", false},   // case-sensitive: uppercase is invalid
		{"Low", false},    // case-sensitive: mixed case is invalid
		{"MEDIUM", false}, // case-sensitive: uppercase is invalid
		{" low", false},   // leading space is invalid
		{"low ", false},   // trailing space is invalid
	}
	for _, tc := range cases {
		if got := validTodoPriority(tc.input); got != tc.want {
			t.Errorf("validTodoPriority(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- priorityRank ---

func TestPriorityRank(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"", 0}, // unknown → 0
		{"invalid", 0},
		{"HIGH", 0}, // case-sensitive: uppercase unknown → 0
		{"Low", 0},
	}
	for _, tc := range cases {
		if got := priorityRank(tc.input); got != tc.want {
			t.Errorf("priorityRank(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestPriorityRankOrderedCorrectly(t *testing.T) {
	if priorityRank("high") <= priorityRank("medium") {
		t.Error("high should outrank medium")
	}
	if priorityRank("medium") <= priorityRank("low") {
		t.Error("medium should outrank low")
	}
	if priorityRank("low") <= priorityRank("") {
		t.Error("low should outrank unknown/empty")
	}
}

// --- stringValue ---

func TestStringValue(t *testing.T) {
	row := map[string]any{
		"str":  "hello",
		"num":  42,
		"bool": true,
		"nil":  nil,
	}
	if got := stringValue(row, "str"); got != "hello" {
		t.Errorf("string key: got %q, want %q", got, "hello")
	}
	if got := stringValue(row, "num"); got != "" {
		t.Errorf("int key: got %q, want empty string", got)
	}
	if got := stringValue(row, "bool"); got != "" {
		t.Errorf("bool key: got %q, want empty string", got)
	}
	if got := stringValue(row, "nil"); got != "" {
		t.Errorf("nil key: got %q, want empty string", got)
	}
	if got := stringValue(row, "missing"); got != "" {
		t.Errorf("missing key: got %q, want empty string", got)
	}
}

// --- applyTodoClientListOptions ---

func makeRows(ids ...string) []map[string]any {
	rows := make([]map[string]any, len(ids))
	for i, id := range ids {
		rows[i] = map[string]any{"id": id}
	}
	return rows
}

func rowIDs(rows []map[string]any) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i], _ = r["id"].(string)
	}
	return ids
}

func applyTodoClientListOptions(rows []map[string]any, opts todoListOpts, total, page int) ([]map[string]any, int, int, error) {
	list, err := applyTodoListOptions(cmdutil.NewListResult(nil, "todos").WithRows(rows, total, page), opts)
	return list.Rows, list.Total, list.Page, err
}

func TestApplyTodoClientListOptions_NoOptsPreservesOrder(t *testing.T) {
	rows := makeRows("a", "b", "c")
	got, total, _, err := applyTodoClientListOptions(rows, todoListOpts{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids := rowIDs(got); strings.Join(ids, ",") != "a,b,c" {
		t.Errorf("order changed: %v", ids)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (fallback to len(rows))", total)
	}
}

func TestApplyTodoClientListOptions_TotalPreservedFromServer(t *testing.T) {
	rows := makeRows("a", "b")
	_, total, _, err := applyTodoClientListOptions(rows, todoListOpts{}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 100 {
		t.Errorf("total = %d, want 100 (server-provided total must be preserved)", total)
	}
}

func TestApplyTodoClientListOptions_SortByCreated(t *testing.T) {
	rows := []map[string]any{
		{"id": "b", "createdAt": "2025-02-01T00:00:00Z"},
		{"id": "a", "createdAt": "2025-01-01T00:00:00Z"},
		{"id": "c", "createdAt": "2025-03-01T00:00:00Z"},
	}
	got, _, _, err := applyTodoClientListOptions(rows, todoListOpts{sort: "created"}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids := rowIDs(got); strings.Join(ids, ",") != "a,b,c" {
		t.Errorf("sort by created: got %v, want a,b,c", ids)
	}
}

func TestApplyTodoClientListOptions_SortByDue(t *testing.T) {
	rows := []map[string]any{
		{"id": "b", "dueAt": "2025-06-15T00:00:00Z"},
		{"id": "a", "dueAt": "2025-06-01T00:00:00Z"},
		{"id": "c", "dueAt": "2025-06-30T00:00:00Z"},
	}
	got, _, _, err := applyTodoClientListOptions(rows, todoListOpts{sort: "due"}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids := rowIDs(got); strings.Join(ids, ",") != "a,b,c" {
		t.Errorf("sort by due: got %v, want a,b,c", ids)
	}
}

func TestApplyTodoClientListOptions_SortByDue_MissingOrInvalidDueDatesLast(t *testing.T) {
	rows := []map[string]any{
		{"id": "b", "dueAt": "2025-06-15T00:00:00Z"},
		{"id": "invalid", "dueAt": "not-a-date"},
		{"id": "a", "dueAt": "2025-06-01T00:00:00Z"},
		{"id": "noduedate"},
	}
	got, _, _, err := applyTodoClientListOptions(rows, todoListOpts{sort: "due"}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids := rowIDs(got); strings.Join(ids, ",") != "a,b,invalid,noduedate" {
		t.Errorf("sort by due with missing/invalid values: got %v, want a,b,invalid,noduedate", ids)
	}
}

func TestApplyTodoClientListOptions_SortByPriority(t *testing.T) {
	rows := []map[string]any{
		{"id": "low", "priority": "low"},
		{"id": "high", "priority": "high"},
		{"id": "med", "priority": "medium"},
		{"id": "none"}, // unknown priority → rank 0
	}
	got, _, _, err := applyTodoClientListOptions(rows, todoListOpts{sort: "priority"}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// highest rank first: high(3) > medium(2) > low(1) > unknown(0)
	if ids := rowIDs(got); strings.Join(ids, ",") != "high,med,low,none" {
		t.Errorf("sort by priority: got %v, want high,med,low,none", ids)
	}
}

func TestApplyTodoClientListOptions_InvalidSort(t *testing.T) {
	_, _, _, err := applyTodoClientListOptions(makeRows("a"), todoListOpts{sort: "name"}, 0, 0)
	if err == nil {
		t.Fatal("expected error for invalid --sort, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --sort") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestApplyTodoClientListOptions_Pagination(t *testing.T) {
	rows := []map[string]any{
		{"id": "1"}, {"id": "2"}, {"id": "3"}, {"id": "4"}, {"id": "5"},
	}
	got, total, page, err := applyTodoClientListOptions(rows, todoListOpts{limit: 2, page: 2}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 || page != 2 || len(got) != 2 {
		t.Errorf("total=%d page=%d len=%d, want total=5 page=2 len=2", total, page, len(got))
	}
	if got[0]["id"] != "3" || got[1]["id"] != "4" {
		t.Errorf("wrong page slice: %v", rowIDs(got))
	}
}

func TestApplyTodoClientListOptions_PageWithoutLimit(t *testing.T) {
	_, _, _, err := applyTodoClientListOptions(makeRows("a", "b"), todoListOpts{page: 2}, 0, 0)
	if err == nil {
		t.Fatal("expected error for --page without --limit, got nil")
	}
	if !strings.Contains(err.Error(), "--page requires --limit") {
		t.Errorf("unexpected error text: %v", err)
	}
}

func TestApplyTodoClientListOptions_LimitOnly(t *testing.T) {
	rows := makeRows("a", "b", "c", "d")
	got, total, page, err := applyTodoClientListOptions(rows, todoListOpts{limit: 2}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || total != 4 || page != 1 {
		t.Errorf("limit only: len=%d total=%d page=%d, want len=2 total=4 page=1", len(got), total, page)
	}
}

func TestApplyTodoClientListOptions_PageBeyondEnd(t *testing.T) {
	rows := makeRows("a", "b")
	got, total, _, err := applyTodoClientListOptions(rows, todoListOpts{limit: 2, page: 99}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 || total != 2 {
		t.Errorf("out-of-range page: len=%d total=%d, want len=0 total=2", len(got), total)
	}
}

func TestApplyTodoClientListOptions_SortAndPaginate(t *testing.T) {
	rows := []map[string]any{
		{"id": "c", "priority": "low"},
		{"id": "a", "priority": "high"},
		{"id": "b", "priority": "medium"},
	}
	// sort by priority then take page 1 limit 2 → high, medium
	got, _, _, err := applyTodoClientListOptions(rows, todoListOpts{sort: "priority", limit: 2, page: 1}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids := rowIDs(got); strings.Join(ids, ",") != "a,b" {
		t.Errorf("sort+paginate: got %v, want a,b", ids)
	}
}

// --- annotateTodoRows ---

func TestAnnotateTodoRows_Completed(t *testing.T) {
	rows := []map[string]any{{"completed": true, "priority": "high"}}
	annotateTodoRows(rows)
	row := rows[0]

	if row["_done"] != "✓" {
		t.Errorf("_done = %q, want ✓", row["_done"])
	}
	if row["_priority"] != "high" {
		t.Errorf("_priority = %q, want high", row["_priority"])
	}
	if row["_due"] != "-" {
		t.Errorf("_due for no-dueAt completed = %q, want -", row["_due"])
	}
}

func TestAnnotateTodoRows_CompletedWithOverdueDue(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	rows := []map[string]any{{"completed": true, "dueAt": past}}
	annotateTodoRows(rows)
	row := rows[0]

	// completed takes precedence over overdue: _done must be ✓ not ✗
	if row["_done"] != "✓" {
		t.Errorf("_done = %q, want ✓ (completed overrides overdue status)", row["_done"])
	}
	// _timeLeft for completed items should be "-"
	if row["_timeLeft"] != "-" {
		t.Errorf("_timeLeft = %q, want - for completed item", row["_timeLeft"])
	}
	// _due should show the date (not "-")
	if row["_due"] == "-" || row["_due"] == "" {
		t.Errorf("_due = %q, want a formatted date for completed item with dueAt", row["_due"])
	}
}

func TestAnnotateTodoRows_Overdue(t *testing.T) {
	past := time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339)
	rows := []map[string]any{{"completed": false, "dueAt": past}}
	annotateTodoRows(rows)
	row := rows[0]

	if row["_done"] != "✗" {
		t.Errorf("_done = %q, want ✗", row["_done"])
	}
	if row["_due"] == "-" || row["_due"] == "" {
		t.Errorf("_due for overdue = %q, want formatted date", row["_due"])
	}
	if row["_timeLeft"] == "-" || row["_timeLeft"] == "" {
		t.Errorf("_timeLeft for overdue = %q, want relative time string", row["_timeLeft"])
	}
}

func TestAnnotateTodoRows_DueSoon(t *testing.T) {
	soon := time.Now().Add(6 * time.Hour).UTC().Format(time.RFC3339)
	rows := []map[string]any{{"completed": false, "dueAt": soon}}
	annotateTodoRows(rows)
	row := rows[0]

	if row["_done"] != "✗" {
		t.Errorf("_done = %q, want ✗", row["_done"])
	}
	if row["_due"] == "-" || row["_due"] == "" {
		t.Errorf("_due for due-soon = %q, want formatted date", row["_due"])
	}
}

func TestAnnotateTodoRows_DueFarFuture(t *testing.T) {
	future := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows := []map[string]any{{"completed": false, "dueAt": future}}
	annotateTodoRows(rows)
	row := rows[0]

	if row["_done"] != "✗" {
		t.Errorf("_done = %q, want ✗", row["_done"])
	}
}

func TestAnnotateTodoRows_NoDueDate(t *testing.T) {
	rows := []map[string]any{{"completed": false}}
	annotateTodoRows(rows)
	row := rows[0]

	if row["_due"] != "-" {
		t.Errorf("_due = %q, want -", row["_due"])
	}
	if row["_timeLeft"] != "-" {
		t.Errorf("_timeLeft = %q, want -", row["_timeLeft"])
	}
}

func TestAnnotateTodoRows_InvalidDueDate(t *testing.T) {
	rows := []map[string]any{{"completed": false, "dueAt": "not-a-date"}}
	annotateTodoRows(rows)
	row := rows[0]

	// Invalid RFC3339 → hasDue = false → treated as no due date
	if row["_due"] != "-" {
		t.Errorf("_due for bad dueAt = %q, want - (invalid date treated as absent)", row["_due"])
	}
	if row["_timeLeft"] != "-" {
		t.Errorf("_timeLeft for bad dueAt = %q, want -", row["_timeLeft"])
	}
}

func TestAnnotateTodoRows_PriorityAnnotation(t *testing.T) {
	rows := []map[string]any{
		{"priority": "high"},
		{"priority": "medium"},
		{"priority": "low"},
		{"priority": ""},
		{"priority": "unknown"},
		{}, // missing priority key
	}
	annotateTodoRows(rows)

	if rows[0]["_priority"] != "high" {
		t.Errorf("[high] _priority = %q, want high", rows[0]["_priority"])
	}
	if rows[1]["_priority"] != "medium" {
		t.Errorf("[medium] _priority = %q, want medium", rows[1]["_priority"])
	}
	if rows[2]["_priority"] != "low" {
		t.Errorf("[low] _priority = %q, want low", rows[2]["_priority"])
	}
	if rows[3]["_priority"] != "-" {
		t.Errorf("[empty] _priority = %q, want -", rows[3]["_priority"])
	}
	if rows[4]["_priority"] != "-" {
		t.Errorf("[unknown] _priority = %q, want -", rows[4]["_priority"])
	}
	if rows[5]["_priority"] != "-" {
		t.Errorf("[missing] _priority = %q, want -", rows[5]["_priority"])
	}
}

func TestAnnotateTodoRows_AllAnnotationKeysSet(t *testing.T) {
	rows := []map[string]any{{"completed": false, "priority": "low"}}
	annotateTodoRows(rows)
	row := rows[0]

	for _, key := range []string{"_done", "_priority", "_due", "_timeLeft"} {
		if _, ok := row[key]; !ok {
			t.Errorf("annotation key %q not set", key)
		}
	}
}

func TestAnnotateTodoRows_EmptySlice(t *testing.T) {
	// must not panic
	annotateTodoRows([]map[string]any{})
}
