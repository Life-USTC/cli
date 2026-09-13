package homework

import (
	"strings"
	"testing"
)

func TestFilterHomeworkRows_UsesCompletionWhenIsCompletedMissing(t *testing.T) {
	rows := []map[string]any{
		{"id": "done", "completion": map[string]any{"completedAt": "2025-06-01T00:00:00Z"}},
		{"id": "pending", "completion": nil},
	}

	cases := []struct {
		name string
		opts myHomeworkListOpts
		want string
	}{
		{name: "done", opts: myHomeworkListOpts{done: true}, want: "done"},
		{name: "pending", opts: myHomeworkListOpts{pending: true}, want: "pending"},
	}

	for _, tc := range cases {
		got, err := filterHomeworkRows(rows, tc.opts)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if len(got) != 1 || got[0]["id"] != tc.want {
			t.Fatalf("%s: got %#v, want only %q", tc.name, got, tc.want)
		}
	}
}

func TestAnnotateHomeworkRows_NoCompletionRequiredIgnoresState(t *testing.T) {
	rows := []map[string]any{
		{
			"id":                 "ta-overdue",
			"completion":         map[string]any{"completedAt": "2000-06-01T00:00:00Z"},
			"completionRequired": false,
			"submissionDueAt":    "2000-06-02T23:59:00Z",
		},
	}

	annotateHomeworkRows(rows)
	if got := rows[0]["_done"]; got != "无需完成" {
		t.Fatalf("TA homework status = %v, want 无需完成", got)
	}
	if got := rows[0]["_due"]; got == "-" {
		t.Fatal("TA homework deadline was hidden")
	}
}

func TestFilterHomeworkRows_TAPendingUsesDeadline(t *testing.T) {
	rows := []map[string]any{
		{
			"id":                 "ta-overdue",
			"completionRequired": false,
			"submissionDueAt":    "2000-06-02T23:59:00Z",
		},
		{
			"id":                 "ta-upcoming",
			"completionRequired": false,
			"submissionDueAt":    "2999-06-02T23:59:00Z",
		},
		{
			"id":                 "ta-no-deadline",
			"completionRequired": false,
		},
	}

	got, err := filterHomeworkRows(rows, myHomeworkListOpts{pending: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0]["id"] != "ta-upcoming" || got[1]["id"] != "ta-no-deadline" {
		t.Fatalf("TA pending rows = %#v, want upcoming and no-deadline", got)
	}
}

func TestReportHomeworkBatchResults_AllSuccess(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"homeworkId": "h1", "success": true},
			map[string]any{"homeworkId": "h2", "success": true},
		},
	}
	rows := []map[string]any{
		{"id": "h1", "title": "PS1"},
		{"id": "h2", "title": "PS2"},
	}
	if err := reportHomeworkBatchResults(data, rows, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportHomeworkBatchResults_PartialFailure(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"homeworkId": "h1", "success": true},
			map[string]any{"homeworkId": "h2", "success": false, "error": map[string]any{"message": "not found"}},
		},
	}
	err := reportHomeworkBatchResults(data, []map[string]any{{"id": "h1", "title": "PS1"}}, true)
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "h2: not found") {
		t.Errorf("expected error to contain failed item, got %v", err)
	}
}
