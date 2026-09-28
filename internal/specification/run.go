package specification

import (
	"encoding/json"
	"fmt"
	"github.com/Life-USTC/CLI/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

type Input struct {
	ServerURL string
	Arguments []string
	Format    string
	Value     string
	Action    string
}
type Observation struct {
	Output           string
	Data             any
	Err              error
	Commands         []string
	Violations       int
	Calls            int
	Identity         string
	Completed        bool
	Action           string
	CompletionAction string
}
type Adapter func(*testing.T, Input) Observation

type Receipt struct {
	Requirement string     `json:"requirement"`
	Case        string     `json:"case"`
	NativeName  string     `json:"nativeName"`
	RunID       string     `json:"runId"`
	Provenance  Provenance `json:"provenance"`
	Consumed    []string   `json:"consumed"`
}

var repositoryOnce sync.Once
var repository *Repository
var repositoryErr error

func currentRepository() (*Repository, error) {
	repositoryOnce.Do(func() {
		root, err := Root()
		if err != nil {
			repositoryErr = err
			return
		}
		repository, repositoryErr = Load(root)
	})
	return repository, repositoryErr
}

// Run owns the full case set and every expectation assertion. Adapters execute
// production entrypoints; they cannot claim a field or case was consumed.
func Run(t *testing.T, execute Adapter) {
	t.Helper()
	repo, err := currentRepository()
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := repo.Requirement(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range requirement.Expectations.Cases {
		t.Run(c.ID, func(t *testing.T) {
			t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
			inputs := newInputs(c)
			inputs.read("id", &c.ID)
			inputs.read("action", &c.Action)
			inputs.read("arguments", &c.Arguments)
			inputs.read("format", &c.Format)
			inputs.read("value", &c.Value)
			harness, err := newHarness(repo, c, inputs)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requirement.Scope == "public" && r.Header.Get("Authorization") != "" {
					t.Error("public command sent credentials")
				}
				if requirement.Scope == "private" && r.Header.Get("Authorization") != "Bearer owner" {
					t.Error("private command omitted owner bearer")
				}
				if err := harness.serve(w, r); err != nil {
					t.Errorf("source-backed fixture: %v", err)
					http.Error(w, "fixture rejected", http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			if requirement.Scope == "private" {
				if err := config.SaveCredentials(server.URL, &config.Credential{AccessToken: "owner", ExpiresAt: float64(time.Now().Add(time.Hour).Unix())}); err != nil {
					t.Fatal(err)
				}
			}
			observed := execute(t, Input{server.URL, c.Arguments, c.Format, c.Value, c.Action})
			server.Close()
			want := newAssertions(c.Want)
			evaluate(t, requirement.Expectations.Kind, c, harness, observed, want)
			if err := inputs.finish(); err != nil {
				t.Error(err)
			}
			if err := want.finish(); err != nil {
				t.Error(err)
			}
			if t.Failed() {
				return
			}
			consumed := append(inputs.consumed, want.consumed...)
			slices.Sort(consumed)
			receipt := Receipt{requirement.ID, c.ID, t.Name(), os.Getenv("SPEC_RUN_ID"), repo.Provenance, consumed}
			encoded, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("SPEC_EVIDENCE " + string(encoded))
		})
	}
}

// CaseFields enumerates schema-validated declarative inputs plus expected fields.
// Every input configures Run's fixture or production adapter; expected values
// reach the assertions reader, which rejects any unhandled expectation.
func CaseFields(c Case) []string {
	m := c.raw
	if m == nil {
		data, _ := json.Marshal(c)
		_ = json.Unmarshal(data, &m)
	}
	fields := []string{}
	for k := range m {
		if k == "want" {
			for w := range c.Want {
				fields = append(fields, "want."+w)
			}
		} else {
			fields = append(fields, k)
		}
	}
	slices.Sort(fields)
	return fields
}

type inputReader struct {
	remaining map[string]json.RawMessage
	consumed  []string
	err       error
}

func newInputs(c Case) *inputReader {
	m := map[string]json.RawMessage{}
	for k, v := range c.raw {
		if k != "want" {
			m[k] = v
		}
	}
	return &inputReader{remaining: m}
}
func (i *inputReader) read(key string, target any) {
	if value, ok := i.remaining[key]; ok {
		if err := json.Unmarshal(value, target); err != nil {
			i.err = err
		}
		delete(i.remaining, key)
		i.consumed = append(i.consumed, key)
	}
}
func (i *inputReader) finish() error {
	if i.err != nil {
		return i.err
	}
	if len(i.remaining) > 0 {
		return fmt.Errorf("unconsumed scenario inputs: %v", i.remaining)
	}
	return nil
}
