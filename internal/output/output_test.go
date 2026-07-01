package output

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var testAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return testAnsiRe.ReplaceAllString(s, "") }

func TestResolveNameFallbacks(t *testing.T) {
	row := map[string]any{
		"category": map[string]any{"nameCn": "通识课"},
		"course":   map[string]any{"nameCn": "数学分析", "nameEn": "Mathematical Analysis"},
	}

	if got := Resolve(row, "category.name"); got != "通识课" {
		t.Fatalf("category.name fallback = %#v, want nameCn", got)
	}
	if got := Resolve(row, "course.namePrimary"); got != "数学分析" {
		t.Fatalf("course.namePrimary fallback = %#v, want nameCn", got)
	}
	if got := Resolve(row, "course.nameSecondary"); got != "Mathematical Analysis" {
		t.Fatalf("course.nameSecondary fallback = %#v, want nameEn", got)
	}
}

func TestJSONReturnsJQErrors(t *testing.T) {
	old := *Current
	defer func() { *Current = old }()

	Current.JQ = ".["
	if err := JSON(map[string]any{"ok": true}); err == nil {
		t.Fatal("JSON with invalid jq returned nil error")
	}
}

// --- Resolve edge cases ---

func TestResolveEdgeCases(t *testing.T) {
	t.Run("key not found returns nil", func(t *testing.T) {
		if got := Resolve(map[string]any{"foo": "bar"}, "missing"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("direct existing key", func(t *testing.T) {
		if got := Resolve(map[string]any{"foo": "bar"}, "foo"); got != "bar" {
			t.Errorf("got %#v, want bar", got)
		}
	})

	t.Run("nil map returns nil", func(t *testing.T) {
		if got := Resolve(nil, "foo"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("empty map returns nil", func(t *testing.T) {
		if got := Resolve(map[string]any{}, "foo"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("dotted path exists", func(t *testing.T) {
		row := map[string]any{"course": map[string]any{"code": "MATH001"}}
		if got := Resolve(row, "course.code"); got != "MATH001" {
			t.Errorf("got %#v, want MATH001", got)
		}
	})

	t.Run("dotted path intermediate missing", func(t *testing.T) {
		row := map[string]any{"course": map[string]any{"code": "MATH001"}}
		if got := Resolve(row, "missing.code"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("dotted path parent is string", func(t *testing.T) {
		row := map[string]any{"course": "not-a-map"}
		if got := Resolve(row, "course.code"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("dotted path parent is number", func(t *testing.T) {
		row := map[string]any{"course": float64(42)}
		if got := Resolve(row, "course.code"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})
}

// TestResolveNameFallbackChain tests the two-level virtual key fallback chain.
func TestResolveNameFallbackChain(t *testing.T) {
	t.Run("name prefers nameCn over nameEn", func(t *testing.T) {
		row := map[string]any{"nameCn": "数学", "nameEn": "Math"}
		if got := Resolve(row, "name"); got != "数学" {
			t.Errorf("got %#v, want 数学", got)
		}
	})

	t.Run("name falls back to nameEn when nameCn missing", func(t *testing.T) {
		row := map[string]any{"nameEn": "Math"}
		if got := Resolve(row, "name"); got != "Math" {
			t.Errorf("got %#v, want Math", got)
		}
	})

	t.Run("namePrimary falls back to nameEn when nameCn missing", func(t *testing.T) {
		row := map[string]any{"nameEn": "Math"}
		if got := Resolve(row, "namePrimary"); got != "Math" {
			t.Errorf("got %#v, want Math", got)
		}
	})

	t.Run("nameSecondary prefers nameEn over nameCn", func(t *testing.T) {
		row := map[string]any{"nameCn": "数学", "nameEn": "Math"}
		if got := Resolve(row, "nameSecondary"); got != "Math" {
			t.Errorf("got %#v, want Math", got)
		}
	})

	t.Run("nameSecondary falls back to nameCn when nameEn missing", func(t *testing.T) {
		row := map[string]any{"nameCn": "数学"}
		if got := Resolve(row, "nameSecondary"); got != "数学" {
			t.Errorf("got %#v, want 数学", got)
		}
	})

	t.Run("name returns nil when both missing", func(t *testing.T) {
		if got := Resolve(map[string]any{}, "name"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("namePrimary returns nil when both missing", func(t *testing.T) {
		if got := Resolve(map[string]any{}, "namePrimary"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})

	t.Run("nameSecondary returns nil when both missing", func(t *testing.T) {
		if got := Resolve(map[string]any{}, "nameSecondary"); got != nil {
			t.Errorf("got %#v, want nil", got)
		}
	})
}

// --- ApplyJQ ---

func TestApplyJQErrors(t *testing.T) {
	data := map[string]any{"key": "value", "num": float64(42)}

	if err := ApplyJQ(data, ".key"); err != nil {
		t.Errorf("valid .key expr returned error: %v", err)
	}
	if err := ApplyJQ(data, ".missing"); err != nil {
		t.Errorf("null-returning expr returned error: %v", err)
	}
	if err := ApplyJQ(data, ".num"); err != nil {
		t.Errorf("numeric expr returned error: %v", err)
	}
	if err := ApplyJQ([]any{1, 2, 3}, ".[]"); err != nil {
		t.Errorf("array iteration expr returned error: %v", err)
	}
	if err := ApplyJQ([]struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{{ID: 1, Name: "one"}}, "length"); err != nil {
		t.Errorf("typed struct slice expr returned error: %v", err)
	}
	if err := ApplyJQ(data, ".invalid-["); err == nil {
		t.Error("invalid expr returned nil error")
	}
}

// --- FormatRelativeTime ---

func TestFormatRelativeTime(t *testing.T) {
	t.Run("empty string returned unchanged", func(t *testing.T) {
		if got := FormatRelativeTime(""); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("invalid input returned unchanged", func(t *testing.T) {
		if got := FormatRelativeTime("not-a-date"); got != "not-a-date" {
			t.Errorf("got %q, want original string", got)
		}
	})

	t.Run("recent past is just now", func(t *testing.T) {
		s := time.Now().Add(-10 * time.Second).Format(time.RFC3339)
		if got := FormatRelativeTime(s); got != "just now" {
			t.Errorf("10s ago got %q, want 'just now'", got)
		}
	})

	t.Run("minutes ago", func(t *testing.T) {
		s := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
		got := FormatRelativeTime(s)
		if !strings.HasSuffix(got, "m ago") {
			t.Errorf("30m ago got %q, want Xm ago", got)
		}
	})

	t.Run("hours ago", func(t *testing.T) {
		s := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
		got := FormatRelativeTime(s)
		if !strings.HasSuffix(got, "h ago") {
			t.Errorf("3h ago got %q, want Xh ago", got)
		}
	})

	t.Run("days ago", func(t *testing.T) {
		s := time.Now().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
		got := FormatRelativeTime(s)
		if !strings.HasSuffix(got, "d ago") {
			t.Errorf("5d ago got %q, want Xd ago", got)
		}
	})

	t.Run("far past returns date format", func(t *testing.T) {
		s := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
		got := FormatRelativeTime(s)
		if len(got) != 10 || got[4] != '-' || got[7] != '-' {
			t.Errorf("60d ago got %q, want YYYY-MM-DD", got)
		}
	})

	t.Run("future hours returns in X format", func(t *testing.T) {
		s := time.Now().Add(3 * time.Hour).Format(time.RFC3339)
		got := FormatRelativeTime(s)
		if !strings.HasPrefix(got, "in ") {
			t.Errorf("3h future got %q, want 'in ...'", got)
		}
	})

	t.Run("near future is in <1m", func(t *testing.T) {
		s := time.Now().Add(30 * time.Second).Format(time.RFC3339)
		if got := FormatRelativeTime(s); got != "in <1m" {
			t.Errorf("30s future got %q, want 'in <1m'", got)
		}
	})
}

// --- TableTo ---

// TestTableToEmptyWritesToWriter ensures the empty message goes to the provided writer.
func TestTableToEmptyWritesToWriter(t *testing.T) {
	var buf strings.Builder
	TableTo(&buf, nil, []Column{{Header: "Name", Key: "name"}}, "Custom empty message")
	out := stripANSI(buf.String())
	if !strings.Contains(out, "Custom empty message") {
		t.Errorf("empty message not in writer output: %q", buf.String())
	}
}

// TestTableToNilCellValues ensures nil column values are rendered as "-".
func TestTableToNilCellValues(t *testing.T) {
	var buf strings.Builder
	rows := []map[string]any{{"code": "MATH001", "name": nil}}
	cols := []Column{
		{Header: "Code", Key: "code"},
		{Header: "Name", Key: "name"},
	}
	TableTo(&buf, rows, cols, "No results.")
	out := stripANSI(buf.String())
	if !strings.Contains(out, "MATH001") {
		t.Errorf("expected MATH001 in output: %q", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("expected '-' placeholder for nil value: %q", out)
	}
}

// TestTableToMissingColumnKey ensures missing column keys don't panic.
func TestTableToMissingColumnKey(t *testing.T) {
	var buf strings.Builder
	rows := []map[string]any{{"code": "MATH001"}}
	cols := []Column{
		{Header: "Code", Key: "code"},
		{Header: "Missing", Key: "nonexistent"},
	}
	TableTo(&buf, rows, cols, "No results.")
	out := stripANSI(buf.String())
	if !strings.Contains(out, "MATH001") {
		t.Errorf("expected MATH001 in output: %q", out)
	}
}

// --- OutputList ---

// TestOutputListEmptyZeroTotal verifies the empty state with total=0 doesn't error.
func TestOutputListEmptyZeroTotal(t *testing.T) {
	old := *Current
	defer func() { *Current = old }()
	Current.Format = "table"
	if err := OutputList(nil, nil, nil, 0, 0); err != nil {
		t.Errorf("OutputList empty got error: %v", err)
	}
}

// ── FormatCellPlain ────────────────────────────────────────────────────────────

func TestFormatCellPlain(t *testing.T) {
	cases := []struct {
		input any
		want  string
	}{
		{nil, "-"},
		{true, "yes"},
		{false, "no"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{"hello", "hello"},
		{"", "-"},
	}
	for _, tc := range cases {
		if got := FormatCellPlain(tc.input); got != tc.want {
			t.Errorf("FormatCellPlain(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
