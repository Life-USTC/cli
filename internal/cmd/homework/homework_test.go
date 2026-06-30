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
