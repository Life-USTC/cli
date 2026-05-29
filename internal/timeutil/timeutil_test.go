package timeutil

import (
	"testing"
	"time"
)

func TestParseAPI(t *testing.T) {
	for _, raw := range []string{
		"2025-06-01T12:30:45.123456789Z",
		"2025-06-01T12:30:45+08:00",
		"2025-06-01T12:30:45",
	} {
		if _, ok := ParseAPI(raw); !ok {
			t.Fatalf("ParseAPI(%q) failed", raw)
		}
	}

	if _, ok := ParseAPI("2025-06-01"); ok {
		t.Fatal("ParseAPI accepted date-only input")
	}
	if _, ok := ParseAPI(time.Now().Format(time.DateOnly)); ok {
		t.Fatal("ParseAPI accepted DateOnly layout")
	}
}

func TestParseUserDateTimeAcceptsDateOnly(t *testing.T) {
	start, err := ParseUserDateTime("2025-06-01", false)
	if err != nil {
		t.Fatalf("ParseUserDateTime start: %v", err)
	}
	if got := start.Format("2006-01-02T15:04:05Z07:00"); got != "2025-06-01T00:00:00Z" {
		t.Fatalf("start = %s", got)
	}

	end, err := ParseUserDateTime("2025-06-01", true)
	if err != nil {
		t.Fatalf("ParseUserDateTime end: %v", err)
	}
	want := time.Date(2025, 6, 1, 23, 59, 59, 999999999, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestDateOnlyString(t *testing.T) {
	if got := DateOnlyString("2025-06-01T00:00:00Z"); got != "2025-06-01" {
		t.Fatalf("DateOnlyString = %q, want date", got)
	}
	if got := DateOnlyString("short"); got != "short" {
		t.Fatalf("DateOnlyString short = %q", got)
	}
}
