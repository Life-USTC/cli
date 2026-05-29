package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldUseInteractiveDisabled(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	use, err := ShouldUseInteractive(cmd, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if use {
		t.Fatal("disabled interactive should not be used")
	}
}

func TestShouldUseInteractiveForceRequiresTTY(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if _, err := ShouldUseInteractive(cmd, true, false); err == nil {
		t.Fatal("expected force interactive to require a terminal")
	}
}

// ── ExtractList ───────────────────────────────────────────────────────────────

func TestExtractListNilInput(t *testing.T) {
	_, rows, total, page := ExtractList(nil)
	if len(rows) != 0 || total != 0 || page != 0 {
		t.Fatalf("nil: rows=%v total=%d page=%d", rows, total, page)
	}
}

func TestExtractListNonMapNonArray(t *testing.T) {
	_, rows, total, page := ExtractList("hello")
	if len(rows) != 0 || total != 0 || page != 0 {
		t.Fatalf("string: rows=%v total=%d page=%d", rows, total, page)
	}
}

func TestExtractListBareArray(t *testing.T) {
	input := []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}}
	_, rows, total, page := ExtractList(input)
	if len(rows) != 2 || total != 0 || page != 0 {
		t.Fatalf("bare array: rows=%v total=%d page=%d", rows, total, page)
	}
	if rows[0]["id"] != "a" || rows[1]["id"] != "b" {
		t.Fatalf("bare array values wrong: %v", rows)
	}
}

func TestExtractListDataKey(t *testing.T) {
	input := map[string]any{
		"data": []any{map[string]any{"id": "1"}},
	}
	_, rows, _, _ := ExtractList(input)
	if len(rows) != 1 || rows[0]["id"] != "1" {
		t.Fatalf("data key: rows=%v", rows)
	}
}

func TestExtractListItemsKey(t *testing.T) {
	input := map[string]any{
		"items": []any{map[string]any{"id": "x"}},
	}
	_, rows, _, _ := ExtractList(input)
	if len(rows) != 1 || rows[0]["id"] != "x" {
		t.Fatalf("items key: rows=%v", rows)
	}
}

func TestExtractListCustomKey(t *testing.T) {
	input := map[string]any{
		"todos": []any{map[string]any{"id": "t1"}, map[string]any{"id": "t2"}},
	}
	_, rows, _, _ := ExtractList(input, "todos")
	if len(rows) != 2 {
		t.Fatalf("custom key 'todos': got %d rows, want 2", len(rows))
	}
}

func TestExtractListEmptyArray(t *testing.T) {
	input := map[string]any{
		"data": []any{},
	}
	_, rows, total, page := ExtractList(input)
	if rows == nil {
		t.Fatal("rows should not be nil for empty array (got nil slice)")
	}
	if len(rows) != 0 || total != 0 || page != 0 {
		t.Fatalf("empty array: rows=%v total=%d page=%d", rows, total, page)
	}
}

func TestExtractListNestedPagination(t *testing.T) {
	input := map[string]any{
		"data": []any{map[string]any{"id": "1"}},
		"pagination": map[string]any{
			"total": float64(42),
			"page":  float64(3),
		},
	}
	_, rows, total, page := ExtractList(input)
	if len(rows) != 1 {
		t.Fatalf("nested pagination: rows=%v", rows)
	}
	if total != 42 {
		t.Fatalf("nested pagination: total=%d, want 42", total)
	}
	if page != 3 {
		t.Fatalf("nested pagination: page=%d, want 3", page)
	}
}

func TestExtractListFlatPagination(t *testing.T) {
	input := map[string]any{
		"data":  []any{map[string]any{"id": "1"}},
		"total": float64(10),
		"page":  float64(2),
	}
	_, rows, total, page := ExtractList(input)
	if len(rows) != 1 {
		t.Fatalf("flat pagination: rows=%v", rows)
	}
	if total != 10 {
		t.Fatalf("flat pagination: total=%d, want 10", total)
	}
	if page != 2 {
		t.Fatalf("flat pagination: page=%d, want 2", page)
	}
}

// Maps without a recognized list key return empty rows and zero pagination.
func TestExtractListMapNoMatchingKey(t *testing.T) {
	input := map[string]any{"unrelated": "value"}
	_, rows, total, page := ExtractList(input)
	if len(rows) != 0 || total != 0 || page != 0 {
		t.Fatalf("map without recognized list key: rows=%v total=%d page=%d", rows, total, page)
	}
}

// ── PaginateRows ──────────────────────────────────────────────────────────────

func TestWithListRowsUsesJSONFriendlySlice(t *testing.T) {
	data := map[string]any{"todos": []any{}}
	out := WithListRows(data, "todos", []map[string]any{{"id": "a"}}, 1, 1)
	m := out.(map[string]any)
	if _, ok := m["todos"].([]any); !ok {
		t.Fatalf("todos has type %T, want []any", m["todos"])
	}
}

func TestListResultServerPagedClampsRows(t *testing.T) {
	data := map[string]any{
		"data": []any{
			map[string]any{"id": "1"},
			map[string]any{"id": "2"},
			map[string]any{"id": "3"},
		},
		"pagination": map[string]any{"total": float64(30), "page": float64(2)},
	}
	list := NewListResult(data, "data").FinalizeServerSide(2)
	if len(list.Rows) != 2 || list.Total != 30 || list.Page != 2 {
		t.Fatalf("rows=%v total=%d page=%d", list.Rows, list.Total, list.Page)
	}
	if got := list.Raw.(map[string]any)["data"].([]any); len(got) != 2 {
		t.Fatalf("raw data length = %d, want 2", len(got))
	}
}

func TestListResultClientSidePaginatesRows(t *testing.T) {
	data := map[string]any{
		"todos": []any{
			map[string]any{"id": "1"},
			map[string]any{"id": "2"},
			map[string]any{"id": "3"},
		},
	}
	list, err := NewListResult(data, "todos").FinalizeClientSide(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) != 1 || list.Rows[0]["id"] != "3" || list.Total != 3 || list.Page != 2 {
		t.Fatalf("rows=%v total=%d page=%d", list.Rows, list.Total, list.Page)
	}
	pagination := list.Raw.(map[string]any)["pagination"].(map[string]any)
	if pagination["total"] != 3 || pagination["page"] != 2 {
		t.Fatalf("pagination=%v", pagination)
	}
}

func TestListResultWithRowsPaginatesAndUpdatesRaw(t *testing.T) {
	data := map[string]any{"todos": []any{}}
	rows := []map[string]any{{"id": "1"}, {"id": "2"}, {"id": "3"}}

	list, err := NewListResult(data, "todos").WithRows(rows, 0, 0).FinalizeClientSide(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) != 1 || list.Rows[0]["id"] != "3" || list.Total != 3 || list.Page != 2 {
		t.Fatalf("rows=%v total=%d page=%d", list.Rows, list.Total, list.Page)
	}
	got := list.Raw.(map[string]any)["todos"].([]any)
	if len(got) != 1 || got[0].(map[string]any)["id"] != "3" {
		t.Fatalf("raw todos=%v", got)
	}
}

func TestPaginateRows(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}}
	if _, _, _, err := PaginateRows(rows, 2, 0); err == nil {
		t.Fatal("PaginateRows accepted --page without --limit")
	}

	pageRows, total, page, err := PaginateRows(rows, 2, 2)
	if err != nil {
		t.Fatalf("PaginateRows: %v", err)
	}
	if total != 3 || page != 2 || len(pageRows) != 1 || pageRows[0]["id"] != 3 {
		t.Fatalf("rows=%v total=%d page=%d", pageRows, total, page)
	}
}

func TestPaginateRowsPage1(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}, {"id": 5}}
	got, total, page, err := PaginateRows(rows, 1, 2)
	if err != nil {
		t.Fatalf("PaginateRows page=1: %v", err)
	}
	if total != 5 || page != 1 || len(got) != 2 {
		t.Fatalf("page=1: rows=%v total=%d page=%d", got, total, page)
	}
	if got[0]["id"] != 1 || got[1]["id"] != 2 {
		t.Fatalf("page=1 values: %v", got)
	}
}

func TestPaginateRowsZeroPageTreatedAsOne(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}}
	// page=0 with limit>0 should be treated as page=1
	got, total, page, err := PaginateRows(rows, 0, 2)
	if err != nil {
		t.Fatalf("PaginateRows page=0,limit=2: %v", err)
	}
	if total != 3 || page != 1 || len(got) != 2 {
		t.Fatalf("page=0,limit=2: rows=%v total=%d page=%d", got, total, page)
	}
	if got[0]["id"] != 1 || got[1]["id"] != 2 {
		t.Fatalf("page=0,limit=2 values: %v", got)
	}
}

func TestPaginateRowsNoPagination(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}}
	got, total, page, err := PaginateRows(rows, 0, 0)
	if err != nil {
		t.Fatalf("PaginateRows no pagination: %v", err)
	}
	if total != 3 || page != 0 || len(got) != 3 {
		t.Fatalf("no pagination: rows=%v total=%d page=%d", got, total, page)
	}
}

func TestPaginateRowsOutOfBounds(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}}
	// 3 items, limit=2 → only pages 1 and 2 have data; page 3 is out of bounds
	got, total, page, err := PaginateRows(rows, 3, 2)
	if err != nil {
		t.Fatalf("PaginateRows out-of-bounds: %v", err)
	}
	if total != 3 || page != 3 || len(got) != 0 {
		t.Fatalf("out-of-bounds: rows=%v total=%d page=%d", got, total, page)
	}
}

func TestPaginateRowsNegativePage(t *testing.T) {
	rows := []map[string]any{{"id": 1}}
	if _, _, _, err := PaginateRows(rows, -1, 2); err == nil {
		t.Fatal("expected error for negative page")
	}
}

func TestPaginateRowsNegativeLimit(t *testing.T) {
	rows := []map[string]any{{"id": 1}}
	if _, _, _, err := PaginateRows(rows, 0, -1); err == nil {
		t.Fatal("expected error for negative limit")
	}
}

// ── WithListRows ──────────────────────────────────────────────────────────────

func TestWithListRowsNonMapData(t *testing.T) {
	// When data is not a map, WithListRows should return the items slice directly.
	out := WithListRows("not-a-map", "data", []map[string]any{{"id": "1"}}, 1, 1)
	items, ok := out.([]any)
	if !ok {
		t.Fatalf("non-map data: got %T, want []any", out)
	}
	if len(items) != 1 {
		t.Fatalf("non-map data: got %d items, want 1", len(items))
	}
}

func TestWithListRowsPreservesOtherFields(t *testing.T) {
	data := map[string]any{
		"status": "ok",
		"todos":  []any{},
	}
	out := WithListRows(data, "todos", []map[string]any{{"id": "t1"}}, 5, 2)
	m := out.(map[string]any)
	if m["status"] != "ok" {
		t.Fatalf("status field lost: %v", m["status"])
	}
	pg, ok := m["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("pagination missing or wrong type: %T", m["pagination"])
	}
	if pg["total"] != 5 || pg["page"] != 2 {
		t.Fatalf("pagination values wrong: %v", pg)
	}
}

func TestWithListRowsZeroTotalAndPage(t *testing.T) {
	// When both total and page are zero, pagination block is skipped.
	data := map[string]any{"data": []any{}}
	out := WithListRows(data, "data", []map[string]any{}, 0, 0)
	m := out.(map[string]any)
	if _, hasPg := m["pagination"]; hasPg {
		t.Fatal("pagination should not be set when total=0 and page=0")
	}
}

// ── AsMap ─────────────────────────────────────────────────────────────────────

func TestAsMapNil(t *testing.T) {
	if AsMap(nil) != nil {
		t.Fatal("AsMap(nil) should return nil")
	}
}

func TestAsMapNonMap(t *testing.T) {
	if AsMap("string") != nil {
		t.Fatal("AsMap(string) should return nil")
	}
	if AsMap(42) != nil {
		t.Fatal("AsMap(int) should return nil")
	}
}

func TestAsMapValid(t *testing.T) {
	input := map[string]any{"key": "value"}
	got := AsMap(input)
	if got == nil {
		t.Fatal("AsMap(map) should not return nil")
	}
	if got["key"] != "value" {
		t.Fatalf("AsMap: got %v, want key=value", got)
	}
}

// ── ClampRowsToLimit ──────────────────────────────────────────────────────────

func TestClampRowsToLimitZero(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}}
	got := ClampRowsToLimit(rows, 0)
	if len(got) != 3 {
		t.Fatalf("limit=0 should return all rows, got %d", len(got))
	}
}

func TestClampRowsToLimitGreaterThanLen(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}}
	got := ClampRowsToLimit(rows, 10)
	if len(got) != 2 {
		t.Fatalf("limit > len: got %d rows, want 2", len(got))
	}
}

func TestClampRowsToLimitLessThanLen(t *testing.T) {
	rows := []map[string]any{{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}}
	got := ClampRowsToLimit(rows, 2)
	if len(got) != 2 {
		t.Fatalf("limit < len: got %d rows, want 2", len(got))
	}
	if got[0]["id"] != 1 || got[1]["id"] != 2 {
		t.Fatalf("limit < len: wrong values: %v", got)
	}
}

// ── PromptPick (unit tests for selection logic) ────────────────────────────

func TestPromptPickNumberSelection(t *testing.T) {
	rows := []map[string]any{
		{"id": "abc123", "title": "First"},
		{"id": "def456", "title": "Second"},
	}

	// Number 1 selects first row
	result := pickByInput(rows, "1", "id")
	if result == nil || result["id"] != "abc123" {
		t.Fatalf("pick 1: got %v", result)
	}

	// Number 2 selects second row
	result = pickByInput(rows, "2", "id")
	if result == nil || result["id"] != "def456" {
		t.Fatalf("pick 2: got %v", result)
	}

	// ID direct match
	result = pickByInput(rows, "abc123", "id")
	if result == nil || result["id"] != "abc123" {
		t.Fatalf("pick by id: got %v", result)
	}

	// Unknown input → synthetic row with just the ID
	result = pickByInput(rows, "unknown-id", "id")
	if result == nil || result["id"] != "unknown-id" {
		t.Fatalf("unknown input: got %v", result)
	}

	// Numeric ID as float64
	rowsFloat := []map[string]any{{"id": float64(42), "title": "Numbered"}}
	result = pickByInput(rowsFloat, "42", "id")
	if result == nil || result["title"] != "Numbered" {
		t.Fatalf("float64 id: got %v", result)
	}

	// Out-of-range number → synthetic row
	result = pickByInput(rows, "3", "id")
	if result == nil || result["id"] != "3" {
		t.Fatalf("out-of-range number: got %v", result)
	}

	// Zero number → synthetic row
	result = pickByInput(rows, "0", "id")
	if result == nil || result["id"] != "0" {
		t.Fatalf("zero number: got %v", result)
	}
}

func TestPromptPickEmptyRows(t *testing.T) {
	picked, err := PromptPick(nil, nil, "id", "test")
	if err != nil {
		t.Fatal(err)
	}
	if picked != nil {
		t.Fatalf("empty rows should return nil")
	}
}

func TestNewListResultBareArrayFallback(t *testing.T) {
	// When the API returns a bare array (not wrapped in a keyed object),
	// NewListResult should still extract rows. Total is 0 because there
	// is no pagination metadata; FinalizeClientSide/ServerSide sets it.
	input := []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}}
	list := NewListResult(input, "todos")
	if len(list.Rows) != 2 {
		t.Fatalf("bare array: got %d rows, want 2", len(list.Rows))
	}
	if list.Rows[0]["id"] != "a" {
		t.Fatalf("bare array: first row id = %v, want a", list.Rows[0]["id"])
	}
	// Bare arrays have no pagination metadata, so total starts at 0.
	// FinalizeServerSide/FinalizeClientSide fills it in from len(rows).
	if list.Total != 0 {
		t.Fatalf("bare array: total = %d, want 0 (no pagination in response)", list.Total)
	}
	list = list.FinalizeServerSide(0)
	if list.Total != 2 {
		t.Fatalf("bare array after finalize: total = %d, want 2", list.Total)
	}
}
