package bus

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout during fn and returns what was printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	_ = r.Close()
	return buf.String()
}

// stripANSI removes ANSI escape sequences so assertions are colour-independent.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ── renderBus ─────────────────────────────────────────────────────────────────

func TestRenderBus_NilData(t *testing.T) {
	out := stripANSI(captureStdout(t, func() { renderBus(nil) }))
	if !strings.Contains(out, "No bus schedules found.") {
		t.Errorf("nil data: expected 'No bus schedules found.', got %q", out)
	}
}

func TestRenderBus_EmptyMap(t *testing.T) {
	out := stripANSI(captureStdout(t, func() { renderBus(map[string]any{}) }))
	if !strings.Contains(out, "No bus schedules found.") {
		t.Errorf("empty map: expected 'No bus schedules found.', got %q", out)
	}
}

func TestRenderBus_EmptyTripsList(t *testing.T) {
	data := map[string]any{
		"routes": []any{},
		"trips":  []any{},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "No bus schedules found.") {
		t.Errorf("empty trips: expected 'No bus schedules found.', got %q", out)
	}
}

func TestRenderBus_RoutesButNoTrips(t *testing.T) {
	data := map[string]any{
		"routes": []any{
			map[string]any{"id": float64(1), "nameCn": "东区-西区"},
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "No bus schedules found.") {
		t.Errorf("routes but no trips: expected 'No bus schedules found.', got %q", out)
	}
}

func TestRenderBus_UnknownRouteIdFallback(t *testing.T) {
	data := map[string]any{
		"routes": []any{},
		"trips": []any{
			map[string]any{
				"routeId":       float64(99),
				"departureTime": "08:00",
			},
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "Route 99") {
		t.Errorf("unknown routeId: expected 'Route 99', got %q", out)
	}
}

func TestRenderBus_NameCnPreferredOverNamePrimary(t *testing.T) {
	data := map[string]any{
		"routes": []any{
			map[string]any{
				"id":          float64(3),
				"nameCn":      "中区班车",
				"namePrimary": "Central Bus",
			},
		},
		"trips": []any{
			map[string]any{"routeId": float64(3), "departureTime": "09:00"},
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "中区班车") {
		t.Errorf("expected nameCn %q to be used, got %q", "中区班车", out)
	}
}

func TestRenderBus_NamePrimaryFallbackWhenNoCn(t *testing.T) {
	data := map[string]any{
		"routes": []any{
			map[string]any{
				"id":          float64(5),
				"namePrimary": "North Loop",
			},
		},
		"trips": []any{
			map[string]any{"routeId": float64(5), "departureTime": "10:00"},
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "North Loop") {
		t.Errorf("expected namePrimary fallback, got %q", out)
	}
}

func TestRenderBus_TripsGroupedByRoute(t *testing.T) {
	data := map[string]any{
		"routes": []any{
			map[string]any{"id": float64(1), "nameCn": "Route-One"},
			map[string]any{"id": float64(2), "nameCn": "Route-Two"},
		},
		"trips": []any{
			map[string]any{"routeId": float64(1), "departureTime": "08:00"},
			map[string]any{"routeId": float64(2), "departureTime": "09:00"},
			map[string]any{"routeId": float64(1), "departureTime": "10:00"},
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	// Both route headers appear
	if !strings.Contains(out, "Route-One") {
		t.Errorf("expected 'Route-One' header, got %q", out)
	}
	if !strings.Contains(out, "Route-Two") {
		t.Errorf("expected 'Route-Two' header, got %q", out)
	}
	// Both trip times appear
	if !strings.Contains(out, "08:00") || !strings.Contains(out, "10:00") {
		t.Errorf("expected both trips for Route-One, got %q", out)
	}
}

func TestRenderBus_NoticeShown(t *testing.T) {
	data := map[string]any{
		"trips": []any{
			map[string]any{"routeId": float64(1), "departureTime": "08:00"},
		},
		"notice": map[string]any{
			"message": "Service may be disrupted today.",
		},
	}
	out := stripANSI(captureStdout(t, func() { renderBus(data) }))
	if !strings.Contains(out, "Service may be disrupted today.") {
		t.Errorf("expected notice message, got %q", out)
	}
}

// Bug: data["notice"] is a non-map value — should not panic.
func TestRenderBus_NoticeNotAMap_NoPanic(t *testing.T) {
	data := map[string]any{
		"trips":  []any{},
		"notice": "plain string notice",
	}
	// Must not panic; notice is silently dropped.
	captureStdout(t, func() { renderBus(data) })
}

// ── printTripLine ─────────────────────────────────────────────────────────────

func TestPrintTripLine_BothDepAndArr(t *testing.T) {
	trip := map[string]any{
		"departureTime": "08:00",
		"arrivalTime":   "09:00",
	}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "") }))
	if !strings.Contains(out, "08:00 → 09:00") {
		t.Errorf("expected dep→arr format, got %q", out)
	}
}

func TestPrintTripLine_OnlyDep(t *testing.T) {
	trip := map[string]any{"departureTime": "08:30"}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "") }))
	if !strings.Contains(out, "08:30") {
		t.Errorf("expected dep time, got %q", out)
	}
	if strings.Contains(out, "→") {
		t.Errorf("unexpected arrow with only dep, got %q", out)
	}
}

func TestPrintTripLine_OnlyArr(t *testing.T) {
	trip := map[string]any{"arrivalTime": "09:30"}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "") }))
	if !strings.Contains(out, "09:30") {
		t.Errorf("expected arrival time, got %q", out)
	}
	if strings.Contains(out, "→") {
		t.Errorf("unexpected arrow with only arr, got %q", out)
	}
}

func TestPrintTripLine_PassThroughStopsSkipped(t *testing.T) {
	trip := map[string]any{
		"departureTime": "08:00",
		"stopTimes": []any{
			map[string]any{"campusName": "East", "isPassThrough": false},
			map[string]any{"campusName": "Mid", "isPassThrough": true},
			map[string]any{"campusName": "West", "isPassThrough": false},
		},
	}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "") }))
	if strings.Contains(out, "Mid") {
		t.Errorf("pass-through stop 'Mid' should not appear, got %q", out)
	}
	if !strings.Contains(out, "East") || !strings.Contains(out, "West") {
		t.Errorf("non-pass-through stops should appear, got %q", out)
	}
}

func TestPrintTripLine_EmptyStopTimes(t *testing.T) {
	trip := map[string]any{
		"departureTime": "07:00",
		"stopTimes":     []any{},
	}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "") }))
	if !strings.Contains(out, "07:00") {
		t.Errorf("expected dep time, got %q", out)
	}
	if strings.Contains(out, "(") {
		t.Errorf("no stop parens expected for empty stopTimes, got %q", out)
	}
}

func TestPrintTripLine_LabelShown(t *testing.T) {
	trip := map[string]any{"departureTime": "08:00"}
	out := stripANSI(captureStdout(t, func() { printTripLine(trip, false, "next") }))
	if !strings.Contains(out, "next") {
		t.Errorf("expected label 'next', got %q", out)
	}
}

func TestPrintTripLine_HighlightDoesNotPanic(t *testing.T) {
	trip := map[string]any{"departureTime": "08:00"}
	// highlight=true should not panic; output differences are terminal-dependent.
	captureStdout(t, func() { printTripLine(trip, true, "") })
	captureStdout(t, func() { printTripLine(trip, false, "") })
}
