package comment

import (
	"strings"
	"testing"
)

func TestReportCommentBatchResults_AllSuccess(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"id": "c1", "success": true},
			map[string]any{"id": "c2", "success": true},
		},
	}
	rows := []map[string]any{
		{"id": "c1", "body": "First comment"},
		{"id": "c2", "body": "Second comment"},
	}
	if err := reportCommentBatchResults(data, rows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportCommentBatchResults_PartialFailure(t *testing.T) {
	data := map[string]any{
		"results": []any{
			map[string]any{"id": "c1", "success": true},
			map[string]any{"id": "c2", "success": false, "error": map[string]any{"message": "not found"}},
		},
	}
	err := reportCommentBatchResults(data, []map[string]any{{"id": "c1", "body": "First"}})
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "c2: not found") {
		t.Errorf("expected error to contain failed item, got %v", err)
	}
}

func TestCommentBatchLabel(t *testing.T) {
	if got := commentBatchLabel([]map[string]any{{"id": "c1", "body": "Hello"}}, 1); got != "Hello" {
		t.Errorf("single row label = %q, want %q", got, "Hello")
	}
	if got := commentBatchLabel([]map[string]any{{"id": "c1"}, {"id": "c2"}}, 2); got != "2 comments" {
		t.Errorf("multi row label = %q, want %q", got, "2 comments")
	}
	if got := commentBatchLabel(nil, 3); got != "3 comments" {
		t.Errorf("id-only label = %q, want %q", got, "3 comments")
	}
	if got := commentBatchLabel(nil, 1); got != "this comment" {
		t.Errorf("single id label = %q, want %q", got, "this comment")
	}
}
