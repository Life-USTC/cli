package semester

import (
	"testing"
)

func TestSemesterDateOnly_FullISO(t *testing.T) {
	m := map[string]any{"startDate": "2025-06-01T00:00:00Z"}
	semesterDateOnly(m)
	if got := m["startDate"]; got != "2025-06-01" {
		t.Errorf("got %q, want %q", got, "2025-06-01")
	}
}

func TestSemesterDateOnly_ExactlyTenChars(t *testing.T) {
	m := map[string]any{"startDate": "2025-06-01"}
	semesterDateOnly(m)
	if got := m["startDate"]; got != "2025-06-01" {
		t.Errorf("got %q, want %q", got, "2025-06-01")
	}
}

func TestSemesterDateOnly_NineChars_NoTrim(t *testing.T) {
	m := map[string]any{"startDate": "2025-06-0"}
	semesterDateOnly(m)
	// len < 10: must remain unchanged
	if got := m["startDate"]; got != "2025-06-0" {
		t.Errorf("got %q, want %q", got, "2025-06-0")
	}
}

func TestSemesterDateOnly_EmptyString_NoPanic(t *testing.T) {
	m := map[string]any{"startDate": ""}
	// must not panic
	semesterDateOnly(m)
	if got := m["startDate"]; got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

func TestSemesterDateOnly_NonStringValue_NoPanic(t *testing.T) {
	m := map[string]any{"startDate": 20250601}
	// type assertion to string fails: must be a no-op, no panic
	semesterDateOnly(m)
	if got := m["startDate"]; got != 20250601 {
		t.Errorf("got %v, want %v", got, 20250601)
	}
}

func TestSemesterDateOnly_KeyAbsent_NoPanic(t *testing.T) {
	m := map[string]any{"other": "value"}
	// startDate/endDate not present: must be no-op
	semesterDateOnly(m)
	if _, ok := m["startDate"]; ok {
		t.Error("unexpected key startDate inserted into map")
	}
}

func TestSemesterDateOnly_BothDates(t *testing.T) {
	m := map[string]any{
		"startDate": "2025-02-01T00:00:00Z",
		"endDate":   "2025-06-30T23:59:59Z",
	}
	semesterDateOnly(m)
	if got := m["startDate"]; got != "2025-02-01" {
		t.Errorf("startDate: got %q, want %q", got, "2025-02-01")
	}
	if got := m["endDate"]; got != "2025-06-30" {
		t.Errorf("endDate: got %q, want %q", got, "2025-06-30")
	}
}
