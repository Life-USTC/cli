package schedule

import "testing"

func TestNormalizeScheduleListRowFormatsWeekdayAndTimeRange(t *testing.T) {
	row := map[string]any{
		"weekday":   "3",
		"startTime": "08:30",
		"endTime":   "10:10",
	}

	normalizeScheduleListRow(row)

	if got := row["weekday"]; got != "Wed" {
		t.Fatalf("weekday = %v, want Wed", got)
	}
	if got := row["timeRange"]; got != "08:30-10:10" {
		t.Fatalf("timeRange = %v, want 08:30-10:10", got)
	}
}

func TestNormalizeScheduleListRowFormatsFloatWeekday(t *testing.T) {
	row := map[string]any{"weekday": float64(7)}

	normalizeScheduleListRow(row)

	if got := row["weekday"]; got != "Sun" {
		t.Fatalf("weekday = %v, want Sun", got)
	}
}

func TestNormalizeScheduleListRowUsesRoomFallback(t *testing.T) {
	row := map[string]any{
		"customPlace": nil,
		"room": map[string]any{
			"namePrimary": "Teaching Building 101",
		},
	}

	normalizeScheduleListRow(row)

	if got := row["customPlace"]; got != "Teaching Building 101" {
		t.Fatalf("customPlace = %v, want room name", got)
	}
}

func TestNormalizeScheduleListRowFormatsNumericTimes(t *testing.T) {
	row := map[string]any{
		"startTime": float64(830),
		"endTime":   float64(1010),
	}

	normalizeScheduleListRow(row)

	if got := row["timeRange"]; got != "08:30-10:10" {
		t.Fatalf("timeRange = %v, want 08:30-10:10", got)
	}
}

func TestNormalizeScheduleListRowFormatsIntTimes(t *testing.T) {
	row := map[string]any{
		"startTime": 830,
		"endTime":   1010,
	}

	normalizeScheduleListRow(row)

	if got := row["timeRange"]; got != "08:30-10:10" {
		t.Fatalf("timeRange = %v, want 08:30-10:10", got)
	}
}

func TestNormalizeScheduleListRowFormatsStringNumericTimes(t *testing.T) {
	row := map[string]any{
		"startTime": "830",
		"endTime":   "1010",
	}

	normalizeScheduleListRow(row)

	if got := row["timeRange"]; got != "08:30-10:10" {
		t.Fatalf("timeRange = %v, want 08:30-10:10", got)
	}
}
