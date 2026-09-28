package specification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type requestRecord struct {
	Method    string
	Operation string
	Body      []byte
	Path      string
}
type harness struct {
	repo     *Repository
	c        Case
	base     any
	requests []requestRecord
	status   int
}

func newHarness(repo *Repository, c Case, inputs *inputReader) (*harness, error) {
	inputs.read("operation", &c.Operation)
	inputs.read("fixture", &c.Fixture)
	inputs.read("count", &c.Count)
	inputs.read("failurePage", &c.FailurePage)
	inputs.read("failureStatus", &c.FailureStatus)
	inputs.read("malformed", &c.Malformed)
	h := &harness{repo: repo, c: c, status: http.StatusOK}
	if c.Fixture == "" {
		return h, nil
	}
	name := c.Fixture
	if name == "description-empty" {
		name = "description"
	}
	data, err := os.ReadFile(filepath.Join(repo.Root, "internal/specification/fixtures", name+".json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &h.base); err != nil {
		return nil, err
	}
	if c.Fixture == "description-empty" {
		field(h.base, "description").(map[string]any)["content"] = ""
	}
	op := repo.Operations[c.Operation]
	if op == nil {
		return nil, fmt.Errorf("unknown fixture operation %s", c.Operation)
	}
	schema, status, err := responseSchema(op)
	if err != nil {
		return nil, err
	}
	h.status = status
	if err := schema.VisitJSON(h.base); err != nil {
		return nil, fmt.Errorf("%s fixture is invalid: %w", c.Fixture, err)
	}
	return h, nil
}
func responseSchema(op *openapi3.Operation) (*openapi3.Schema, int, error) {
	for _, status := range []string{"200", "201"} {
		if response := op.Responses.Value(status); response != nil {
			if media := response.Value.Content["application/json"]; media != nil && media.Schema != nil {
				n, _ := strconv.Atoi(status)
				return media.Schema.Value, n, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("operation %s has no JSON success response", op.OperationID)
}
func (h *harness) validatePointer(path string) error {
	schema, _, err := responseSchema(h.repo.Operations[h.c.Operation])
	if err != nil {
		return err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if schema.Type.Is("array") {
			if _, err := strconv.Atoi(part); err != nil {
				return err
			}
			schema = schema.Items.Value
		} else {
			ref := propertySchema(schema, part)
			if ref == nil {
				return fmt.Errorf("%s references undeclared response field %s", h.c.ID, path)
			}
			schema = ref.Value
		}
	}
	return nil
}
func clone(v any) any { return jsonValue(v) }
func (h *harness) serve(w http.ResponseWriter, r *http.Request) error {
	route, params, err := h.repo.Router.FindRoute(r)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	h.requests = append(h.requests, requestRecord{r.Method, route.Operation.OperationID, body, r.URL.Path})
	input := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil }}}
	if route.Operation.RequestBody != nil {
		for _, media := range route.Operation.RequestBody.Value.Content {
			if media.Schema != nil {
				if err := checkNumericBounds(media.Schema.Value, map[*openapi3.Schema]bool{}); err != nil {
					return err
				}
			}
		}
	}
	if err := openapi3filter.ValidateRequest(context.Background(), input); err != nil {
		return fmt.Errorf("invalid actual request %s: %w", r.URL, err)
	}
	op := route.Operation.OperationID
	allowed := op == h.c.Operation || (h.c.Action == "bus-partial" && op == "workspace_bus_preferences_set") || (h.c.Action == "school-homework" && op == "setHomeworkCompletion") || (h.c.Action == "homework-sections" && op == "getCurrentCalendarSubscription")
	if !allowed {
		return fmt.Errorf("unexpected operation %s, expected %s", op, h.c.Operation)
	}
	if err := h.checkCLIQuery(r); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	if h.c.FailurePage > 0 && page == h.c.FailurePage {
		response := route.Operation.Responses.Value(strconv.Itoa(h.c.FailureStatus))
		if response == nil || response.Value.Content["application/json"] == nil {
			return fmt.Errorf("operation %s has no declared denial response", op)
		}
		denied := map[string]any{"error": "denied"}
		if err := response.Value.Content["application/json"].Schema.Value.VisitJSON(denied); err != nil {
			return err
		}
		w.WriteHeader(h.c.FailureStatus)
		return json.NewEncoder(w).Encode(denied)
	}
	base := clone(h.base)
	if op == "getCurrentCalendarSubscription" {
		base = map[string]any{"subscription": nil}
	} else if op == "setHomeworkCompletion" {
		base = map[string]any{"completed": true, "completedAt": "2026-09-01T00:00:00Z"}
	} else if op == "workspace_bus_preferences_set" {
		var request any
		if err := json.Unmarshal(body, &request); err != nil {
			return err
		}
		base = map[string]any{"preference": request}
	} else if h.c.Fixture == "exam" || h.c.Fixture == "semester" || h.c.Fixture == "homework" || h.c.Fixture == "subscribed" || h.c.Fixture == "upload" {
		size := 20
		if value := r.URL.Query().Get("pageSize"); value != "" {
			size, _ = strconv.Atoi(value)
		}
		rows := []any{}
		template := resultRows(h.base)[0]
		for id := (page-1)*size + 1; id <= page*size && id <= h.c.Count; id++ {
			row := clone(template).(map[string]any)
			switch h.c.Fixture {
			case "exam":
				row["id"] = id
				field(field(row, "section"), "course").(map[string]any)["namePrimary"] = fmt.Sprintf("Course %d", id)
			case "semester":
				row["id"] = id
				row["jwId"] = id
				row["code"] = strconv.Itoa(id)
				row["nameCn"] = fmt.Sprintf("Semester %d", id)
			case "homework", "subscribed":
				row["id"] = fmt.Sprintf("h%d", id)
				row["title"] = fmt.Sprintf("Assignment %d", id)
				row["completion"] = nil
				if id < h.c.Count {
					row["completion"] = map[string]any{"completedAt": "2026-09-01T00:00:00Z"}
				}
				if h.c.Fixture == "subscribed" && id == h.c.Count {
					field(row, "section").(map[string]any)["id"] = 8
				}
			case "upload":
				row["id"] = fmt.Sprintf("u%d", id)
				row["filename"] = fmt.Sprintf("File-%d.txt", id)
				row["key"] = fmt.Sprintf("key%d", id)
				row["size"] = id
			}
			rows = append(rows, row)
		}
		pages := (h.c.Count + size - 1) / size
		if pages == 0 {
			pages = 1
		}
		base.(map[string]any)["data"] = rows
		base.(map[string]any)["pagination"] = map[string]any{"page": page, "pageSize": size, "total": h.c.Count, "totalPages": pages}
	}
	schema, status, err := responseSchema(route.Operation)
	if err != nil {
		return err
	}
	if err := schema.VisitJSON(base); err != nil {
		return fmt.Errorf("invalid outgoing %s fixture: %w", op, err)
	}
	if h.c.Malformed {
		base.(map[string]any)["pagination"].(map[string]any)["page"] = "wrong-type"
		if schema.VisitJSON(base) == nil {
			return fmt.Errorf("malformed fixture no longer violates its source")
		}
	}
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(base)
}

func propertySchema(s *openapi3.Schema, key string) *openapi3.SchemaRef {
	if ref := s.Properties[key]; ref != nil {
		return ref
	}
	for _, part := range s.AllOf {
		if ref := propertySchema(part.Value, key); ref != nil {
			return ref
		}
	}
	return nil
}
func checkNumericBounds(s *openapi3.Schema, seen map[*openapi3.Schema]bool) error {
	if s == nil || seen[s] {
		return nil
	}
	seen[s] = true
	if s.ExclusiveMin && s.Min == nil {
		return fmt.Errorf("source has exclusiveMinimum without minimum")
	}
	if s.ExclusiveMax && s.Max == nil {
		return fmt.Errorf("source has exclusiveMaximum without maximum")
	}
	for _, ref := range s.Properties {
		if err := checkNumericBounds(ref.Value, seen); err != nil {
			return err
		}
	}
	for _, refs := range []openapi3.SchemaRefs{s.OneOf, s.AnyOf, s.AllOf} {
		for _, ref := range refs {
			if err := checkNumericBounds(ref.Value, seen); err != nil {
				return err
			}
		}
	}
	if s.Items != nil {
		return checkNumericBounds(s.Items.Value, seen)
	}
	return nil
}

func (h *harness) checkCLIQuery(r *http.Request) error {
	if h.c.Operation == "community_section_homework_list" && r.Method == "GET" {
		section := stringID(field(resultRows(h.base)[0], "sectionId"))
		if r.URL.Query().Get("sectionId") != section {
			return fmt.Errorf("wrong homework section: %s", r.URL)
		}
		if h.c.Action == "school-homework" && r.URL.Query().Get("includeDeleted") != "false" {
			return fmt.Errorf("school sync included deleted homework: %s", r.URL)
		}
	}
	mapping := map[string]string{}
	switch h.c.Action {
	case "exam":
		mapping = map[string]string{"--page": "page", "--limit": "pageSize", "--semester-id": "semesterId", "--date-from": "dateFrom", "--date-to": "dateTo", "--include-date-unknown": "includeDateUnknown"}
	case "publication":
		mapping = map[string]string{"--type": "type", "--limit": "pageSize"}
	case "homework-section":
		mapping = map[string]string{"--include-deleted": "includeDeleted"}
	case "description":
		mapping = map[string]string{"--target-type": "targetType", "--target-id": "targetId"}
	}
	for i, arg := range h.c.Arguments {
		flag, value, explicit := strings.Cut(arg, "=")
		param, ok := mapping[flag]
		if !ok {
			continue
		}
		if !explicit {
			if flag == "--include-deleted" {
				value = "true"
			} else if i+1 < len(h.c.Arguments) {
				value = h.c.Arguments[i+1]
			}
		}
		if r.URL.Query().Get(param) != value {
			return fmt.Errorf("CLI flag %s expected %s=%q, request had %q", flag, param, value, r.URL.Query().Get(param))
		}
	}
	if h.c.Action == "exam" && !slices.ContainsFunc(h.c.Arguments, func(v string) bool { return strings.HasPrefix(v, "--include-date-unknown") }) && r.URL.Query().Get("includeDateUnknown") != "true" {
		return fmt.Errorf("exam command omitted unknown-date inclusion")
	}
	return nil
}
