package specification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testDocument(t *testing.T) ([]byte, []byte) {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join(root, "docs/specifications/contracts.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(root, "docs/specifications/contracts.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	return spec, schema
}
func mutateJSON(t *testing.T, data []byte, fn func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	fn(value)
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func firstCase(value map[string]any) map[string]any {
	return value["requirements"].([]any)[0].(map[string]any)["expectations"].(map[string]any)["cases"].([]any)[0].(map[string]any)
}
func TestContractSchemaRejectsInvalidDeclarations(t *testing.T) {
	spec, schema := testDocument(t)
	for name, mutation := range map[string]func(map[string]any){
		"unknown expectation":    func(v map[string]any) { firstCase(v)["want"].(map[string]any)["ignored"] = true },
		"missing expectation":    func(v map[string]any) { delete(firstCase(v)["want"].(map[string]any), "requests") },
		"unknown scenario input": func(v map[string]any) { firstCase(v)["ignored"] = true },
		"missing case": func(v map[string]any) {
			e := v["requirements"].([]any)[0].(map[string]any)["expectations"].(map[string]any)
			e["cases"] = e["cases"].([]any)[1:]
		},
		"duplicate case": func(v map[string]any) {
			e := v["requirements"].([]any)[0].(map[string]any)["expectations"].(map[string]any)
			cs := e["cases"].([]any)
			cs[1] = cs[0]
		},
		"orphan case":           func(v map[string]any) { firstCase(v)["id"] = "unregistered" },
		"fractional count":      func(v map[string]any) { firstCase(v)["count"] = 0.5 },
		"duplicate requirement": func(v map[string]any) { rs := v["requirements"].([]any); rs[1] = rs[0] },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateDocument(mutateJSON(t, spec, mutation), schema); err == nil {
				t.Fatal("invalid declaration passed")
			}
		})
	}
}

type failures struct{ messages []string }

func (f *failures) Helper()           {}
func (f *failures) Error(args ...any) { f.messages = append(f.messages, fmt.Sprint(args...)) }
func (f *failures) Errorf(format string, args ...any) {
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}
func TestChangedExpectationChangesBehavioralResult(t *testing.T) {
	for _, value := range []string{"1", "2"} {
		a := newAssertions(map[string]json.RawMessage{"rowCount": json.RawMessage(value)})
		r := &failures{}
		a.equal(r, "rowCount", 1)
		if (len(r.messages) == 0) != (value == "1") {
			t.Fatalf("constraint mutation was ignored: %v", r.messages)
		}
		if err := a.finish(); err != nil {
			t.Fatal(err)
		}
	}
	a := newAssertions(map[string]json.RawMessage{"rowCount": json.RawMessage("1"), "unused": json.RawMessage("true")})
	a.equal(&failures{}, "rowCount", 1)
	if a.finish() == nil {
		t.Fatal("unconsumed expectation passed")
	}
	inputs := newInputs(Case{raw: map[string]json.RawMessage{"addedButUnused": json.RawMessage("true")}})
	if inputs.finish() == nil {
		t.Fatal("unconsumed input passed")
	}
}
func nativeEvidence(repo *Repository, runID string) []NativeEvent {
	events := []NativeEvent{}
	for _, r := range repo.Document.Requirements {
		pkg := "github.com/Life-USTC/CLI/" + filepath.ToSlash(filepath.Dir(r.Test.File))
		for _, c := range r.Expectations.Cases {
			name := r.Test.Name + "/" + c.ID
			receipt := Receipt{r.ID, c.ID, name, runID, repo.Provenance, CaseFields(c)}
			raw, _ := json.Marshal(receipt)
			events = append(events, NativeEvent{Action: "run", Package: pkg, Test: name}, NativeEvent{Action: "output", Package: pkg, Test: name, Output: "SPEC_EVIDENCE " + string(raw) + "\n"}, NativeEvent{Action: "pass", Package: pkg, Test: name})
		}
		events = append(events, NativeEvent{Action: "pass", Package: pkg, Test: r.Test.Name})
	}
	return events
}
func eventStream(events []NativeEvent) *bytes.Buffer {
	buf := new(bytes.Buffer)
	for _, e := range events {
		_ = json.NewEncoder(buf).Encode(e)
	}
	return buf
}
func TestEvidenceRequiresNativePassAndCompleteConsumption(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	valid := nativeEvidence(repo, "run-1")
	report, err := Collect(repo, eventStream(valid), "run-1")
	if err != nil || !report.GatePassed {
		t.Fatalf("valid evidence: %+v %v", report, err)
	}
	for name, mutation := range map[string]func([]NativeEvent) []NativeEvent{
		"missing native pass":   func(es []NativeEvent) []NativeEvent { return append(es[:2], es[3:]...) },
		"skipped":               func(es []NativeEvent) []NativeEvent { es[2].Action = "skip"; return es },
		"duplicate native pass": func(es []NativeEvent) []NativeEvent { return append(es, es[2]) },
		"missing receipt":       func(es []NativeEvent) []NativeEvent { return append(es[:1], es[2:]...) },
		"duplicate receipt":     func(es []NativeEvent) []NativeEvent { return append(es, es[1]) },
		"unknown native case":   func(es []NativeEvent) []NativeEvent { es[0].Test += "-unknown"; return es },
		"stale run": func(es []NativeEvent) []NativeEvent {
			es[1].Output = strings.ReplaceAll(es[1].Output, "run-1", "old-run")
			return es
		},
		"stale source": func(es []NativeEvent) []NativeEvent {
			es[1].Output = strings.ReplaceAll(es[1].Output, repo.Provenance.SourceSHA256, strings.Repeat("0", 64))
			return es
		},
		"stale spec": func(es []NativeEvent) []NativeEvent {
			es[1].Output = strings.ReplaceAll(es[1].Output, repo.Provenance.SpecSHA256, strings.Repeat("0", 64))
			return es
		},
		"stale commit": func(es []NativeEvent) []NativeEvent {
			es[1].Output = strings.ReplaceAll(es[1].Output, repo.Provenance.Commit, strings.Repeat("0", 40))
			return es
		},
		"missing consumed field": func(es []NativeEvent) []NativeEvent {
			var receipt Receipt
			_ = json.Unmarshal([]byte(strings.TrimPrefix(es[1].Output, "SPEC_EVIDENCE ")), &receipt)
			receipt.Consumed = receipt.Consumed[1:]
			raw, _ := json.Marshal(receipt)
			es[1].Output = "SPEC_EVIDENCE " + string(raw) + "\n"
			return es
		},
	} {
		t.Run(name, func(t *testing.T) {
			events := append([]NativeEvent(nil), valid...)
			if _, err := Collect(repo, eventStream(mutation(events)), "run-1"); err == nil {
				t.Fatal("invalid evidence passed")
			}
		})
	}
}

func TestWireValidationRejectsRequestsOutsidePinnedSource(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	c := Case{ID: "source-bound", Action: "homework-section", Operation: "community_section_homework_list", Fixture: "homework", Count: 1}
	for _, size := range []int{50, 100} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			h, err := newHarness(repo, c, newInputs(c))
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("GET", fmt.Sprintf("/api/community/section-homeworks?sectionId=7&page=1&pageSize=%d", size), nil)
			err = h.serve(httptest.NewRecorder(), request)
			if (err == nil) != (size == 50) {
				t.Fatalf("pageSize %d: %v", size, err)
			}
			if size == 100 && !strings.Contains(err.Error(), "invalid actual request") {
				t.Fatalf("wrong rejection: %v", err)
			}
		})
	}
	h, err := newHarness(repo, c, newInputs(c))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/data/0/id", "/data/0/undeclared"} {
		if err := h.validatePointer(path); (err == nil) != (path == "/data/0/id") {
			t.Fatalf("projection %s: %v", path, err)
		}
	}
	schema, _, err := responseSchema(repo.Operations[c.Operation])
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"missing required field": func(row map[string]any) { delete(row, "title") },
		"wrong field type":       func(row map[string]any) { row["title"] = false },
	} {
		t.Run(name, func(t *testing.T) {
			body := clone(h.base)
			mutate(resultRows(body)[0].(map[string]any))
			if err := schema.VisitJSON(body); err == nil {
				t.Fatal("source-invalid fixture passed")
			}
		})
	}
}

func TestIncompleteNumericSourceBoundsFailClosed(t *testing.T) {
	for _, schema := range []*openapi3.Schema{{ExclusiveMin: true}, {ExclusiveMax: true}} {
		if err := checkNumericBounds(schema, map[*openapi3.Schema]bool{}); err == nil {
			t.Fatal("incomplete source bound passed")
		}
	}
	minimum := float64(1)
	if err := checkNumericBounds(&openapi3.Schema{Min: &minimum}, map[*openapi3.Schema]bool{}); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteIdentityAssertionsRejectCorruptedCollections(t *testing.T) {
	for name, ids := range map[string][]string{
		"correct":   {"u1", "u2", "u3"},
		"extra":     {"u1", "u2", "u3", "u4"},
		"missing":   {"u1", "u3"},
		"replaced":  {"u1", "unknown", "u3"},
		"duplicate": {"u1", "u1", "u3"},
	} {
		for _, format := range []string{"json", "table"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				observed := Observation{}
				rows := []any{}
				text := "Usage: 6 B / 10 B\n3 result(s)\nID FILENAME SIZE\n"
				for _, id := range ids {
					rows = append(rows, map[string]any{"id": id})
					text += id + " File.txt 1\n"
				}
				if format == "table" {
					observed.Output = text
				} else {
					observed.Data = map[string]any{"data": rows, "pagination": map[string]any{"total": 3}}
				}
				assertions := newAssertions(map[string]json.RawMessage{"rowCount": json.RawMessage("3"), "total": json.RawMessage("3"), "identities": json.RawMessage(`["u1","u2","u3"]`)})
				reporter := &failures{}
				evaluate(reporter, "collection", Case{Format: format}, &harness{}, observed, assertions)
				if (len(reporter.messages) == 0) != (name == "correct") {
					t.Fatalf("%s %s: %v", format, name, reporter.messages)
				}
				if err := assertions.finish(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestEvidenceReassemblesSplitNativeOutput(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	repo, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	runID := strings.Repeat("r", 1100)
	events := nativeEvidence(repo, runID)
	split := []NativeEvent{}
	for _, event := range events {
		if event.Output == "" {
			split = append(split, event)
			continue
		}
		// Split inside the marker and JSON as well as across the native 1024-byte
		// boundary. Package/test identity must survive interleaved native events.
		for len(event.Output) > 0 {
			length := min(7, len(event.Output))
			part := event
			part.Output = event.Output[:length]
			split = append(split, part)
			event.Output = event.Output[length:]
		}
	}
	if _, err := Collect(repo, eventStream(split), runID); err != nil {
		t.Fatalf("split receipt: %v", err)
	}
	first, second := events[1], events[4]
	firstRest, secondRest := first, second
	first.Output, firstRest.Output = first.Output[:1024], first.Output[1024:]
	second.Output, secondRest.Output = second.Output[:1024], second.Output[1024:]
	interleaved := []NativeEvent{events[0], events[3], first, second, firstRest, secondRest, events[2], events[5]}
	interleaved = append(interleaved, events[6:]...)
	if _, err := Collect(repo, eventStream(interleaved), runID); err != nil {
		t.Fatalf("interleaved 1024-byte receipt fragments: %v", err)
	}
	for name, mutate := range map[string]func([]NativeEvent){
		"truncated JSON":        func(es []NativeEvent) { es[1].Output = es[1].Output[:len(es[1].Output)-3] },
		"missing final newline": func(es []NativeEvent) { es[1].Output = strings.TrimSuffix(es[1].Output, "\n") },
	} {
		t.Run(name, func(t *testing.T) {
			altered := append([]NativeEvent(nil), events...)
			mutate(altered)
			if _, err := Collect(repo, eventStream(altered), runID); err == nil {
				t.Fatal("truncated receipt passed")
			}
		})
	}
}
