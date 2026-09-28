// Package specification implements the repository's executable acceptance contracts.
// It is used by native tests and the evidence command, never by product requests.
package specification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type Binding struct {
	File string `json:"file"`
	Name string `json:"name"`
}
type Requirement struct {
	ID           string  `json:"id"`
	Scope        string  `json:"scope"`
	Rationale    string  `json:"rationale"`
	Test         Binding `json:"test"`
	Expectations struct {
		Kind  string `json:"kind"`
		Cases []Case `json:"cases"`
	} `json:"expectations"`
}

// Each action has a closed schema. Raw want fields are read only by the family
// checker; its destructive reader detects fields that never reach an assertion.
type Case struct {
	raw           map[string]json.RawMessage
	ID            string                     `json:"id"`
	Action        string                     `json:"action"`
	Operation     string                     `json:"operation,omitempty"`
	Arguments     []string                   `json:"arguments,omitempty"`
	Format        string                     `json:"format,omitempty"`
	Fixture       string                     `json:"fixture,omitempty"`
	Count         int                        `json:"count,omitempty"`
	FailureStatus int                        `json:"failureStatus,omitempty"`
	FailurePage   int                        `json:"failurePage,omitempty"`
	Value         string                     `json:"value,omitempty"`
	Malformed     bool                       `json:"malformed,omitempty"`
	Want          map[string]json.RawMessage `json:"want"`
}

func (c *Case) UnmarshalJSON(data []byte) error {
	type plain Case
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = Case(value)
	return json.Unmarshal(data, &c.raw)
}

type Document struct {
	Schema  string `json:"$schema"`
	Version int    `json:"version"`
	Owner   string `json:"owner"`
	Source  struct {
		OpenAPI    string `json:"openapi"`
		Provenance string `json:"provenance"`
	} `json:"source"`
	Requirements []Requirement `json:"requirements"`
}
type Provenance struct {
	WorkingTreeClean bool   `json:"workingTreeClean"`
	Commit           string `json:"commit"`
	SpecSHA256       string `json:"specSha256"`
	SchemaSHA256     string `json:"schemaSha256"`
	SourceSHA256     string `json:"sourceSha256"`
	SourceCommit     string `json:"sourceCommit"`
}
type Repository struct {
	Root       string
	Document   Document
	API        *openapi3.T
	Router     routers.Router
	Provenance Provenance
	Operations map[string]*openapi3.Operation
}

func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs/specifications/contracts.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find CLI specifications")
		}
		dir = parent
	}
}
func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON document")
	}
	return nil
}
func ValidateDocument(spec, schema []byte) (Document, error) {
	var document Document
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(spec))
	if err != nil {
		return document, err
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return document, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("https://life-ustc.github.io/cli/contracts.schema.json", schemaValue); err != nil {
		return document, err
	}
	compiled, err := compiler.Compile("https://life-ustc.github.io/cli/contracts.schema.json")
	if err != nil {
		return document, err
	}
	if err := compiled.Validate(value); err != nil {
		return document, err
	}
	if err := decode(spec, &document); err != nil {
		return document, err
	}
	ids, tests := map[string]bool{}, map[string]bool{}
	for _, r := range document.Requirements {
		if ids[r.ID] || tests[r.Test.Name] || !strings.HasSuffix(r.Test.Name, "/"+r.ID) {
			return document, fmt.Errorf("duplicate or invalid canonical binding: %s", r.ID)
		}
		ids[r.ID] = true
		tests[r.Test.Name] = true
		cases := map[string]bool{}
		for _, c := range r.Expectations.Cases {
			if cases[c.ID] {
				return document, fmt.Errorf("duplicate case %s/%s", r.ID, c.ID)
			}
			cases[c.ID] = true
		}
	}
	return document, nil
}
func Load(root string) (*Repository, error) {
	spec, err := os.ReadFile(filepath.Join(root, "docs/specifications/contracts.json"))
	if err != nil {
		return nil, err
	}
	schema, err := os.ReadFile(filepath.Join(root, "docs/specifications/contracts.schema.json"))
	if err != nil {
		return nil, err
	}
	document, err := ValidateDocument(spec, schema)
	if err != nil {
		return nil, err
	}
	rawAPI, err := os.ReadFile(filepath.Join(root, document.Source.OpenAPI))
	if err != nil {
		return nil, err
	}
	rawPin, err := os.ReadFile(filepath.Join(root, document.Source.Provenance))
	if err != nil {
		return nil, err
	}
	var pin struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		SHA256     string `json:"sha256"`
	}
	if err := decode(rawPin, &pin); err != nil {
		return nil, err
	}
	if pin.Repository != "Life-USTC/server" || len(pin.Commit) != 40 || pin.SHA256 != digest(rawAPI) {
		return nil, fmt.Errorf("OpenAPI provenance mismatch")
	}
	loader := openapi3.NewLoader()
	api, err := loader.LoadFromFile(filepath.Join(root, document.Source.OpenAPI))
	if err != nil {
		return nil, err
	}
	router, err := legacy.NewRouter(api)
	if err != nil {
		return nil, err
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	repo := &Repository{Root: root, Document: document, API: api, Router: router, Provenance: Provenance{len(bytes.TrimSpace(status)) == 0, strings.TrimSpace(string(head)), digest(spec), digest(schema), digest(rawAPI), pin.Commit}, Operations: map[string]*openapi3.Operation{}}
	for _, p := range api.Paths.Map() {
		for _, op := range p.Operations() {
			if op.OperationID != "" {
				if repo.Operations[op.OperationID] != nil {
					return nil, fmt.Errorf("duplicate OpenAPI operation %s", op.OperationID)
				}
				repo.Operations[op.OperationID] = op
			}
		}
	}
	for _, r := range document.Requirements {
		for _, c := range r.Expectations.Cases {
			if c.Operation != "" && repo.Operations[c.Operation] == nil {
				return nil, fmt.Errorf("%s/%s references missing operation %s", r.ID, c.ID, c.Operation)
			}
		}
	}
	return repo, nil
}
func (r *Repository) Requirement(name string) (Requirement, error) {
	for _, v := range r.Document.Requirements {
		if v.Test.Name == name {
			return v, nil
		}
	}
	return Requirement{}, fmt.Errorf("no canonical requirement for %s", name)
}
