package specification

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

type NativeEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
}
type Coverage struct {
	Provenance   Provenance `json:"provenance"`
	RunID        string     `json:"runId"`
	Requirements int        `json:"requirements"`
	Cases        int        `json:"cases"`
	Passed       int        `json:"passed"`
	GatePassed   bool       `json:"gatePassed"`
	Receipts     []Receipt  `json:"receipts"`
}

// Collect joins declarations with independent native Go execution events. A log
// receipt alone never proves a test ran or passed.
func Collect(repo *Repository, reader io.Reader, runID string) (Coverage, error) {
	report := Coverage{Provenance: repo.Provenance, RunID: runID, Requirements: len(repo.Document.Requirements), Receipts: []Receipt{}}
	if runID == "" {
		return report, fmt.Errorf("native evidence needs a nonempty run ID")
	}
	type expectedCase struct {
		requirement string
		id          string
		fields      []string
		name        string
	}
	expected := map[string]expectedCase{}
	canonical := map[string]int{}
	for _, r := range repo.Document.Requirements {
		pkg := "github.com/Life-USTC/CLI/" + filepath.ToSlash(filepath.Dir(r.Test.File))
		canonical[pkg+"\n"+r.Test.Name] = 0
		for _, c := range r.Expectations.Cases {
			name := r.Test.Name + "/" + c.ID
			expected[pkg+"\n"+name] = expectedCase{r.ID, c.ID, CaseFields(c), name}
			report.Cases++
		}
	}
	passes := map[string]int{}
	caseRuns := map[string]int{}
	canonicalRuns := map[string]int{}
	receipts := map[string]Receipt{}
	outputLines := map[string]string{}
	consumeLine := func(key, line string) error {
		pos := strings.Index(line, "SPEC_EVIDENCE ")
		if pos < 0 {
			return nil
		}
		var receipt Receipt
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[pos+len("SPEC_EVIDENCE "):])), &receipt); err != nil {
			return fmt.Errorf("invalid consumption receipt: %w", err)
		}
		wanted, ok := expected[key]
		if !ok {
			return fmt.Errorf("receipt outside declared native case: %s", key)
		}
		if _, duplicate := receipts[key]; duplicate {
			return fmt.Errorf("duplicate receipt: %s", key)
		}
		if receipt.Requirement != wanted.requirement || receipt.Case != wanted.id || receipt.NativeName != wanted.name || receipt.RunID != runID || receipt.Provenance != repo.Provenance {
			return fmt.Errorf("receipt provenance or case mismatch: %s", key)
		}
		actual := append([]string(nil), receipt.Consumed...)
		slices.Sort(actual)
		if !reflect.DeepEqual(actual, wanted.fields) {
			return fmt.Errorf("missing, duplicate or unknown consumed field: %s", key)
		}
		receipts[key] = receipt
		return nil
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var event NativeEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return report, fmt.Errorf("invalid native event: %w", err)
		}
		key := event.Package + "\n" + event.Test
		if event.Action == "fail" {
			return report, fmt.Errorf("native test failure: %s", key)
		}
		if _, known := canonical[key]; known {
			if event.Action == "pass" {
				canonical[key]++
			}
			if event.Action == "run" {
				canonicalRuns[key]++
			}
		}
		if _, known := expected[key]; known {
			if event.Action == "run" {
				caseRuns[key]++
			}
			if event.Action == "skip" {
				return report, fmt.Errorf("mandatory case skipped: %s", key)
			}
			if event.Action == "pass" {
				passes[key]++
			}
		}
		for parent := range canonical {
			if strings.HasPrefix(key, parent+"/") {
				if _, known := expected[key]; !known {
					return report, fmt.Errorf("undeclared native case: %s", key)
				}
			}
		}
		if event.Output != "" {
			outputLines[key] += event.Output
			for {
				line, rest, complete := strings.Cut(outputLines[key], "\n")
				if !complete {
					break
				}
				outputLines[key] = rest
				if err := consumeLine(key, line); err != nil {
					return report, err
				}
			}
		}

	}
	if err := scanner.Err(); err != nil {
		return report, err
	}
	for key, remaining := range outputLines {
		if remaining != "" {
			return report, fmt.Errorf("truncated native output line: %s", key)
		}
	}
	for name, count := range canonical {
		if canonicalRuns[name] != 1 {
			return report, fmt.Errorf("canonical needs exactly one native run (%d): %s", canonicalRuns[name], name)
		}
		if count != 1 {
			return report, fmt.Errorf("canonical needs exactly one native pass (%d): %s", count, name)
		}
	}
	keys := []string{}
	for name := range expected {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		if caseRuns[name] != 1 {
			return report, fmt.Errorf("case needs exactly one native run (%d): %s", caseRuns[name], name)
		}
		if passes[name] != 1 {
			return report, fmt.Errorf("case needs exactly one native pass (%d): %s", passes[name], name)
		}
		receipt, ok := receipts[name]
		if !ok {
			return report, fmt.Errorf("missing consumption receipt: %s", name)
		}
		report.Receipts = append(report.Receipts, receipt)
		report.Passed++
	}
	report.GatePassed = true
	return report, nil
}
