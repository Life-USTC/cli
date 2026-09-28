package specification

import (
	"encoding/json"
	"fmt"
	"github.com/go-openapi/jsonpointer"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

type reporter interface {
	Helper()
	Error(...any)
	Errorf(string, ...any)
}

type assertions struct {
	remaining map[string]json.RawMessage
	consumed  []string
}

func newAssertions(m map[string]json.RawMessage) *assertions {
	remaining := map[string]json.RawMessage{}
	for k, v := range m {
		remaining[k] = v
	}
	return &assertions{remaining: remaining}
}
func (a *assertions) equal(t reporter, key string, actual any) {
	t.Helper()
	raw, ok := a.remaining[key]
	if !ok {
		t.Errorf("checker expected undeclared field %s", key)
		return
	}
	delete(a.remaining, key)
	a.consumed = append(a.consumed, "want."+key)
	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Error(err)
		return
	}
	var want, got any
	_ = json.Unmarshal(raw, &want)
	_ = json.Unmarshal(encoded, &got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s: got %s, want %s", key, encoded, raw)
	}
}
func (a *assertions) strings(t reporter, key string, check func(string) bool) {
	t.Helper()
	raw, ok := a.remaining[key]
	if !ok {
		t.Errorf("missing %s", key)
		return
	}
	delete(a.remaining, key)
	a.consumed = append(a.consumed, "want."+key)
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Error(err)
		return
	}
	for _, v := range values {
		if !check(v) {
			t.Errorf("%s: unsatisfied %q", key, v)
		}
	}
}
func (a *assertions) finish() error {
	if len(a.remaining) > 0 {
		keys := []string{}
		for k := range a.remaining {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return fmt.Errorf("unconsumed expectation fields: %v", keys)
	}
	return nil
}
func pointer(value any, path string) (any, error) {
	p, err := jsonpointer.New(path)
	if err != nil {
		return nil, err
	}
	v, _, err := p.Get(value)
	return v, err
}
func jsonValue(value any) any {
	raw, _ := json.Marshal(value)
	var result any
	_ = json.Unmarshal(raw, &result)
	return result
}
func resultData(o Observation) any {
	if o.Data != nil {
		return jsonValue(o.Data)
	}
	var data any
	_ = json.Unmarshal([]byte(o.Output), &data)
	return data
}
func stringID(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func resultRows(value any) []any {
	if m, ok := value.(map[string]any); ok {
		return resultRows(m["data"])
	}
	if rows, ok := value.([]any); ok {
		return rows
	}
	return nil
}
func field(value any, key string) any {
	if m, ok := value.(map[string]any); ok {
		return m[key]
	}
	return nil
}
func containsToken(s, word string) bool { return slices.Contains(strings.Fields(s), word) }

func evaluate(t reporter, kind string, c Case, h *harness, o Observation, a *assertions) {
	t.Helper()
	if kind != "architecture" || c.Action != "architecture" {
		if _, ok := a.remaining["error"]; ok {
			a.equal(t, "error", o.Err != nil)
		}
	}
	if _, ok := a.remaining["requests"]; ok {
		a.equal(t, "requests", len(h.requests))
	}
	if _, ok := a.remaining["outputEmpty"]; ok {
		a.equal(t, "outputEmpty", strings.TrimSpace(o.Output) == "" && o.Data == nil)
	}
	switch kind {
	case "collection":
		if o.Err != nil {
			return
		}
		data := resultData(o)
		rows := resultRows(data)
		tableTotal := 0
		if c.Format == "table" {
			var err error
			rows, tableTotal, err = uploadTableRows(o.Output)
			if err != nil {
				t.Error(err)
			}
		}
		if _, ok := a.remaining["resolved"]; ok {
			a.equal(t, "resolved", o.Identity)
		}
		a.equal(t, "rowCount", len(rows))
		total := len(rows)
		if c.Format == "table" {
			total = tableTotal
		}
		if pg, ok := field(data, "pagination").(map[string]any); ok {
			if n, ok := pg["total"].(float64); ok {
				total = int(n)
			}
		}
		a.equal(t, "total", total)
		if _, ok := a.remaining["page"]; ok {
			a.equal(t, "page", field(field(data, "pagination"), "page"))
		}
		if _, ok := a.remaining["filenames"]; ok {
			a.strings(t, "filenames", func(name string) bool {
				return strings.Contains(o.Output, name) && slices.ContainsFunc(rows, func(row any) bool { return field(row, "filename") == name })
			})
		}
		ids := []string{}
		for _, row := range rows {
			ids = append(ids, stringID(field(row, "id")))
		}
		a.exactStrings(t, "identities", ids)
		if _, ok := a.remaining["preserveQuota"]; ok {
			preserved := false
			if c.Format == "json" {
				preserved = reflect.DeepEqual(field(data, "meta"), field(h.base, "meta"))
			} else {
				preserved = strings.Contains(o.Output, "Usage:") && !containsToken(o.Output, "Type")
			}
			a.equal(t, "preserveQuota", preserved)
		}

	case "input":
	case "normalization":
		a.equal(t, "value", o.Output)
	case "availability":
		a.strings(t, "absent", func(command string) bool { return !slices.Contains(o.Commands, command) })
	case "projection":
		data := resultData(o)
		a.strings(t, "preserve", func(path string) bool {
			want, err := pointer(h.base, path)
			if err != nil {
				t.Error(err)
				return false
			}
			if err := h.validatePointer(path); err != nil {
				t.Error(err)
				return false
			}
			if c.Format == "table" {
				return strings.Contains(o.Output, fmt.Sprint(want))
			}
			got, err := pointer(data, path)
			return err == nil && reflect.DeepEqual(got, want)
		})
	case "mutation":
		writes := 0
		for _, req := range h.requests {
			if req.Method != "GET" {
				writes++
			}
		}
		a.equal(t, "writes", writes)
		if c.Action == "school-homework" {
			a.equal(t, "completed", o.Completed)
			a.equal(t, "identity", o.Identity)
			if _, ok := a.remaining["completionAction"]; ok {
				a.equal(t, "completionAction", o.CompletionAction)
			}
			for _, request := range h.requests {
				if request.Operation == "setHomeworkCompletion" {
					var body map[string]any
					if err := json.Unmarshal(request.Body, &body); err != nil {
						t.Error(err)
					}
					if body["completed"] != o.Completed || !strings.HasSuffix(request.Path, "/"+o.Identity+"/completion") {
						t.Error("completion request differs from reused homework identity or state")
					}
				}
			}
			if o.Err == nil && o.Action != "matched" {
				t.Errorf("existing homework was not reused: %s", o.Action)
			}
		} else {
			body := map[string]any{}
			for _, req := range h.requests {
				if req.Method != "GET" {
					_ = json.Unmarshal(req.Body, &body)
				}
			}
			a.strings(t, "preserve", func(path string) bool {
				initial, err := pointer(field(h.base, "preference"), path)
				if err != nil {
					t.Error(err)
					return false
				}
				actual, err := pointer(body, path)
				return err == nil && reflect.DeepEqual(initial, actual)
			})
			if raw, ok := a.remaining["changed"]; ok {
				delete(a.remaining, "changed")
				a.consumed = append(a.consumed, "want.changed")
				var changes map[string]any
				_ = json.Unmarshal(raw, &changes)
				for k, v := range changes {
					if !reflect.DeepEqual(body[k], v) {
						t.Errorf("changed %s=%v, want %v", k, body[k], v)
					}
				}
			}
		}
	case "description":
		data := resultData(o)
		content := field(field(h.base, "description"), "content")
		if c.Format == "json" {
			if !reflect.DeepEqual(field(data, "description"), field(h.base, "description")) {
				t.Error("nested description response lost")
			}
		} else if content != "" && !strings.Contains(o.Output, fmt.Sprint(content)) {
			t.Error("description content missing")
		}
		a.equal(t, "content", content)
		a.equal(t, "empty", content == "")
		if c.Format == "table" && ((content == "") != strings.Contains(o.Output, "No description.")) {
			t.Error("description empty state incorrect")
		}
		a.equal(t, "history", len(resultRows(field(h.base, "history"))) > 0)
		for _, key := range []string{"revisionIDs", "editors", "timestamps"} {
			a.strings(t, key, func(value string) bool { return strings.Contains(o.Output, value) })
		}
	case "architecture":
		if c.Action == "architecture" {
			a.equal(t, "violations", o.Violations)
			raw, ok := a.remaining["minimumCalls"]
			if !ok {
				t.Error("missing minimumCalls")
			} else {
				delete(a.remaining, "minimumCalls")
				a.consumed = append(a.consumed, "want.minimumCalls")
				var min int
				_ = json.Unmarshal(raw, &min)
				if o.Calls < min {
					t.Errorf("only %d generated calls, need %d", o.Calls, min)
				}
			}
		}
		if _, ok := a.remaining["preserve"]; ok {
			data := resultData(o)
			a.strings(t, "preserve", func(path string) bool {
				expected, err := pointer(h.base, path)
				if err != nil {
					return false
				}
				actual, err := pointer(data, path)
				return err == nil && reflect.DeepEqual(actual, expected)
			})
		}
	default:
		t.Errorf("unsupported expectation kind %s", kind)
	}
}

// exactStrings compares the complete multiset. Unknown, duplicated, replaced,
// missing and extra identities all fail without filtering the actual output.
func (a *assertions) exactStrings(t reporter, key string, actual []string) {
	raw, ok := a.remaining[key]
	if !ok {
		t.Errorf("missing %s", key)
		return
	}
	delete(a.remaining, key)
	a.consumed = append(a.consumed, "want."+key)
	var expected []string
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Error(err)
		return
	}
	got := append([]string{}, actual...)
	slices.Sort(expected)
	slices.Sort(got)
	if !slices.Equal(got, expected) {
		t.Errorf("%s: got %v, want %v", key, got, expected)
	}
}

// Read every rendered upload row. Only the known presentation metadata is
// skipped; no expected ID or fixture count participates in parsing output.
func uploadTableRows(output string) ([]any, int, error) {
	rows := []any{}
	total, header, usage, summary := 0, false, false, false
	for _, line := range strings.Split(output, "\n") {
		cells := strings.Fields(line)
		if len(cells) == 0 {
			continue
		}
		if !usage && cells[0] == "Usage:" {
			usage = true
			continue
		}
		if !summary && len(cells) == 2 && cells[1] == "result(s)" {
			var err error
			total, err = strconv.Atoi(cells[0])
			if err != nil {
				return rows, total, err
			}
			summary = true
			continue
		}
		if !header && slices.Equal(cells, []string{"ID", "FILENAME", "SIZE"}) {
			header = true
			continue
		}
		if !header || len(cells) < 3 {
			return rows, total, fmt.Errorf("unexpected upload table line %q", line)
		}
		rows = append(rows, map[string]any{"id": cells[0], "filename": strings.Join(cells[1:len(cells)-1], " ")})
	}
	if !header || !summary {
		return rows, total, fmt.Errorf("upload table omitted its header or result count")
	}
	return rows, total, nil
}
