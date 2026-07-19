package section

import (
	"testing"
)

// ── normalizeScheduleRow – weekday ────────────────────────────────────────────

func TestNormalizeScheduleRow_Weekday1_Mon(t *testing.T) {
	row := map[string]any{"weekday": float64(1)}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != "Mon" {
		t.Errorf("got %v, want Mon", got)
	}
}

func TestNormalizeScheduleRow_Weekday5_Fri(t *testing.T) {
	row := map[string]any{"weekday": float64(5)}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != "Fri" {
		t.Errorf("got %v, want Fri", got)
	}
}

func TestNormalizeScheduleRow_Weekday7_Sun(t *testing.T) {
	row := map[string]any{"weekday": float64(7)}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != "Sun" {
		t.Errorf("got %v, want Sun", got)
	}
}

func TestNormalizeScheduleRow_Weekday8_Unmapped(t *testing.T) {
	// 8 is not in the map; value must remain unchanged as float64(8)
	row := map[string]any{"weekday": float64(8)}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != float64(8) {
		t.Errorf("got %v, want float64(8)", got)
	}
}

func TestNormalizeScheduleRow_Weekday0_Unmapped(t *testing.T) {
	// 0 is not in the map; value must remain unchanged
	row := map[string]any{"weekday": float64(0)}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != float64(0) {
		t.Errorf("got %v, want float64(0)", got)
	}
}

func TestNormalizeScheduleRow_WeekdayString_NoChange(t *testing.T) {
	// already a string: type assertion to float64 fails, must be no-op
	row := map[string]any{"weekday": "Mon"}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != "Mon" {
		t.Errorf("got %v, want \"Mon\"", got)
	}
}

// ── normalizeScheduleRow – time fields ────────────────────────────────────────

func TestNormalizeScheduleRow_StartTime830(t *testing.T) {
	row := map[string]any{"startTime": float64(830)}
	normalizeScheduleRow(row)
	if got := row["startTime"]; got != "08:30" {
		t.Errorf("got %v, want \"08:30\"", got)
	}
}

func TestNormalizeScheduleRow_StartTime1230(t *testing.T) {
	row := map[string]any{"startTime": float64(1230)}
	normalizeScheduleRow(row)
	if got := row["startTime"]; got != "12:30" {
		t.Errorf("got %v, want \"12:30\"", got)
	}
}

func TestNormalizeScheduleRow_StartTime0(t *testing.T) {
	row := map[string]any{"startTime": float64(0)}
	normalizeScheduleRow(row)
	if got := row["startTime"]; got != "00:00" {
		t.Errorf("got %v, want \"00:00\"", got)
	}
}

func TestNormalizeScheduleRow_EndTime1700(t *testing.T) {
	row := map[string]any{"endTime": float64(1700)}
	normalizeScheduleRow(row)
	if got := row["endTime"]; got != "17:00" {
		t.Errorf("got %v, want \"17:00\"", got)
	}
}

func TestNormalizeScheduleRow_StartTimeString_NoChange(t *testing.T) {
	// already a string: must be a no-op
	row := map[string]any{"startTime": "08:30"}
	normalizeScheduleRow(row)
	if got := row["startTime"]; got != "08:30" {
		t.Errorf("got %v, want \"08:30\"", got)
	}
}

// ── normalizeScheduleRow – combined row ───────────────────────────────────────

func TestNormalizeScheduleRow_FullRow(t *testing.T) {
	row := map[string]any{
		"weekday":   float64(3),
		"startTime": float64(900),
		"endTime":   float64(1040),
	}
	normalizeScheduleRow(row)
	if got := row["weekday"]; got != "Wed" {
		t.Errorf("weekday: got %v, want Wed", got)
	}
	if got := row["startTime"]; got != "09:00" {
		t.Errorf("startTime: got %v, want 09:00", got)
	}
	if got := row["endTime"]; got != "10:40" {
		t.Errorf("endTime: got %v, want 10:40", got)
	}
}

func TestSectionListUsesViewIdentifier(t *testing.T) {
	columns := sectionListColumns()
	last := columns[len(columns)-1]
	if last.Header != "JW ID" || last.Key != "jwId" {
		t.Fatalf("last column = %#v, want JW ID backed by jwId", last)
	}
}
