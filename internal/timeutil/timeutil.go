package timeutil

import (
	"fmt"
	"strings"
	"time"
)

// ParseAPI parses timestamp strings returned by the Life@USTC API.
func ParseAPI(s string) (time.Time, bool) {
	if len(s) < 19 || s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ParseUserDateTime accepts a full RFC3339 timestamp or a YYYY-MM-DD date.
// Date-only values are interpreted in UTC (not local time). This ensures
// --before 2025-06-01 consistently means "before end of June 1 UTC", which
// matches the API's server-side filtering. If endOfDay is true, date-only
// values resolve to the last nanosecond of that day.
func ParseUserDateTime(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation(time.DateOnly, raw, time.UTC); err == nil {
		if endOfDay {
			return t.Add(24*time.Hour - time.Nanosecond), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 timestamp or YYYY-MM-DD date")
}

func DateOnlyString(value string) string {
	if len(value) >= len(time.DateOnly) {
		return value[:len(time.DateOnly)]
	}
	return value
}
