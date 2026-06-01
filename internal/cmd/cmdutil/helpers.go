// Package cmdutil provides shared helpers for all commands.
package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Life-USTC/CLI/internal/config"
	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ServerFromCmd extracts the --server flag, falling back to the configured default.
func ServerFromCmd(cmd *cobra.Command) string {
	s, _ := cmd.Root().PersistentFlags().GetString("server")
	if s == "" {
		return config.GetDefaultServer()
	}
	return s
}

// StringPtrIfSet returns a pointer for non-empty flag values.
func StringPtrIfSet(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// IntStringPtrIfPositive returns a decimal string pointer for positive flag values.
func IntStringPtrIfPositive(value int) *string {
	if value <= 0 {
		return nil
	}
	result := strconv.Itoa(value)
	return &result
}

// Int64PtrIfPositive returns a pointer for positive integer flag values.
func Int64PtrIfPositive(value int) *int64 {
	if value <= 0 {
		return nil
	}
	result := int64(value)
	return &result
}

// Int64PtrIfSet parses a non-empty integer flag value.
func Int64PtrIfSet(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer %q", value)
	}
	return &parsed, nil
}

// ShouldUseInteractive decides whether a command should open a TUI.
//
// Default behavior: in a TTY with no explicit list/filter flags set, the TUI
// opens automatically so users can browse interactively. Any changed flag
// (--search, --limit, etc.) switches to plain table output for scripting.
//
// Override with --interactive to force the TUI (requires a terminal), or
// --no-interactive to suppress it (useful in aliases or scripts).
func ShouldUseInteractive(cmd *cobra.Command, force, disabled bool, optionFlags ...string) (bool, error) {
	if disabled {
		return false, nil
	}
	if force {
		if !IsInteractive() {
			return false, fmt.Errorf("--interactive requires a terminal")
		}
		return true, nil
	}
	if !IsInteractive() || output.IsJSON() {
		return false, nil
	}
	for _, flag := range optionFlags {
		if cmd.Flags().Changed(flag) {
			return false, nil
		}
	}
	return true, nil
}

// FormatWeekday converts API weekday values (1=Mon, 7=Sun) to short names.
func FormatWeekday(value any) (string, bool) {
	var day int
	switch v := value.(type) {
	case float64:
		day = int(v)
	case int:
		day = v
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", false
		}
		day = n
	default:
		return "", false
	}

	switch day {
	case 1:
		return "Mon", true
	case 2:
		return "Tue", true
	case 3:
		return "Wed", true
	case 4:
		return "Thu", true
	case 5:
		return "Fri", true
	case 6:
		return "Sat", true
	case 7:
		return "Sun", true
	default:
		return "", false
	}
}

// FormatHHMM converts numeric API times such as 830 to "08:30".
// Already-formatted "08:30" strings pass through unchanged.
func FormatHHMM(value any) (string, bool) {
	var raw int
	switch v := value.(type) {
	case float64:
		raw = int(v)
	case int:
		raw = v
	case string:
		if len(v) == 5 && v[2] == ':' {
			return v, true
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", false
		}
		raw = n
	default:
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", raw/100, raw%100), true
}

// AddListFlags registers the standard --limit/-L and --page/-p flags on a command.
func AddListFlags(cmd *cobra.Command, page, limit *int) {
	cmd.Flags().IntVarP(limit, "limit", "L", 0, "Maximum number of results to fetch")
	cmd.Flags().IntVarP(page, "page", "p", 0, "Page number for paginated results")
}

// DoneVerb returns the appropriate verb for completed/uncompleted toggles.
func DoneVerb(undo bool) string {
	if undo {
		return "reopen"
	}
	return "complete"
}

// IsInteractive reports whether stdin and stdout are both terminals.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// PromptText reads a single trimmed line from stdin after printing a label.
func PromptText(label string) string {
	fmt.Printf("%s ", outputPrompt(label))
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// PromptSelect lets a user choose by 1-based index or by exact choice text.
func PromptSelect(label string, choices []string) string {
	if len(choices) == 0 {
		return ""
	}
	output.Panel(label)
	for i, c := range choices {
		fmt.Printf("  %d) %s\n", i+1, c)
	}
	fmt.Print(outputPrompt("Choice") + " ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		for i, c := range choices {
			if text == strconv.Itoa(i+1) || strings.EqualFold(text, c) {
				return c
			}
		}
		return text
	}
	return choices[0]
}

// PromptPick displays a table and lets the user pick an item by number
// (1-based) or by typing an ID directly. Returns the selected row, or nil
// if cancelled. The idKey specifies which column holds the unique identifier.
func PromptPick(rows []map[string]any, cols []output.Column, idKey string, prompt string) (map[string]any, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	output.Panel(prompt, fmt.Sprintf("Enter a number (1-%d) or paste an ID", len(rows)))
	output.Table(rows, cols)

	input := PromptText("Choice")
	if input == "" {
		output.Warning("Cancelled.")
		return nil, nil
	}
	return pickByInput(rows, input, idKey), nil
}

// pickByInput resolves a user's typed input against a list of rows.
// It tries number (1-based) first, then ID match, then falls back to a
// synthetic row with just the idKey filled in.
func pickByInput(rows []map[string]any, input string, idKey string) map[string]any {
	// Try number first
	if n, err := strconv.Atoi(input); err == nil && n >= 1 && n <= len(rows) {
		return rows[n-1]
	}

	// Try ID match
	for _, row := range rows {
		if id, ok := row[idKey].(string); ok && id == input {
			return row
		}
		if fmt.Sprint(row[idKey]) == input {
			return row
		}
	}

	// No match — return a synthetic row with just the ID
	return map[string]any{idKey: input}
}

// Confirm asks for a y/N confirmation unless yes is already true.
func Confirm(prompt string, yes bool) bool {
	if yes {
		return true
	}
	fmt.Printf("%s ", outputPrompt(prompt+" (y/N)"))
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
		return true
	}
	output.Warning("Cancelled.")
	return false
}

func outputPrompt(label string) string {
	return fmt.Sprintf("%s %s:", outputPromptMarker(), label)
}

func outputPromptMarker() string {
	return ">"
}

// ExtractList pulls rows and pagination info from a standard API list response.
// The API may return pagination at the top level or nested under a "pagination" key:
//
//	{"data": [...], "pagination": {"total": N, "page": N, "totalPages": N}}
//	{"todos": [...]}
func ExtractList(data any, listKeys ...string) (raw any, rows []map[string]any, total int, page int) {
	raw = data
	m, ok := data.(map[string]any)
	if !ok {
		if arr, ok := data.([]any); ok {
			rows = toRows(arr)
		}
		return
	}

	keys := listKeys
	if len(keys) == 0 {
		keys = []string{"items", "data", "results"}
	}

	for _, key := range keys {
		if list, ok := m[key]; ok {
			if arr, ok := list.([]any); ok {
				rows = toRows(arr)
				break
			}
		}
	}

	// Extract pagination — check nested "pagination" object first, then top-level
	pg := m
	if nested, ok := m["pagination"].(map[string]any); ok {
		pg = nested
	}
	if t, ok := pg["total"].(float64); ok {
		total = int(t)
	}
	if p, ok := pg["page"].(float64); ok {
		page = int(p)
	}
	return
}

type ListResult struct {
	Raw   any
	Rows  []map[string]any
	Total int
	Page  int
	Key   string
}

// NewListResult extracts rows and pagination metadata from a list response.
// If the explicit key yields no rows, it falls back to treating the entire
// response as an array — matching the previous ExtractList behavior.
func NewListResult(data any, key string) ListResult {
	keys := []string{}
	if key != "" {
		keys = append(keys, key)
	}
	raw, rows, total, page := ExtractList(data, keys...)
	if key == "" {
		key = "data"
	}
	// Fallback: bare array response not wrapped in a keyed object.
	if len(rows) == 0 {
		if arr, ok := data.([]any); ok {
			rows = toRows(arr)
			total = len(rows)
		}
	}
	return ListResult{Raw: raw, Rows: rows, Total: total, Page: page, Key: key}
}

// WithRows returns a result with filtered/sorted rows. The Raw field is not
// updated — call FinalizeClientSide or FinalizeServerSide afterward to
// synchronize Raw with the new rows for consistent JSON/jq output.
func (r ListResult) WithRows(rows []map[string]any, total, page int) ListResult {
	r.Rows = rows
	r.Total = total
	r.Page = page
	return r
}

// FinalizeClientSide applies client-side pagination and rewrites Raw so JSON
// and table output show the same rows.
func (r ListResult) FinalizeClientSide(page, limit int) (ListResult, error) {
	var err error
	if page > 0 || limit > 0 {
		r.Rows, r.Total, r.Page, err = PaginateRows(r.Rows, page, limit)
		if err != nil {
			return ListResult{}, err
		}
	} else if r.Total == 0 {
		r.Total = len(r.Rows)
	}
	r.Raw = WithListRows(r.Raw, r.Key, r.Rows, r.Total, r.Page)
	return r, nil
}

// FinalizeServerSide clamps rows when a server ignores limit and rewrites Raw so
// JSON and table output show the same rows.
func (r ListResult) FinalizeServerSide(limit int) ListResult {
	r.Rows = ClampRowsToLimit(r.Rows, limit)
	if r.Total == 0 {
		r.Total = len(r.Rows)
	}
	r.Raw = WithListRows(r.Raw, r.Key, r.Rows, r.Total, r.Page)
	return r
}

func toRows(arr []any) []map[string]any {
	rows := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// RowsFromAny converts an API array value into table rows, ignoring non-object
// entries to match the existing detail-rendering behavior.
func RowsFromAny(data any) []map[string]any {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}
	return toRows(arr)
}

// PaginateRows applies client-side pagination for endpoints that do not expose
// page/limit parameters. A zero page means page 1 when a limit is set.
func PaginateRows(rows []map[string]any, page, limit int) ([]map[string]any, int, int, error) {
	if page < 0 {
		return nil, 0, 0, fmt.Errorf("--page must be non-negative")
	}
	if limit < 0 {
		return nil, 0, 0, fmt.Errorf("--limit must be non-negative")
	}
	if page > 0 && limit == 0 {
		return nil, 0, 0, fmt.Errorf("--page requires --limit for client-side pagination")
	}
	total := len(rows)
	if limit == 0 {
		return rows, total, page, nil
	}
	if page == 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= total {
		return []map[string]any{}, total, page, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total, page, nil
}

// ClampRowsToLimit trims rows when a server ignores --limit. It is harmless when
// the server already honored the requested limit.
func ClampRowsToLimit(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

// WithListRows returns data with key replaced by rows and pagination metadata
// updated. It preserves other top-level response fields when possible.
func WithListRows(data any, key string, rows []map[string]any, total, page int) any {
	items := RowsAsAny(rows)
	m, ok := data.(map[string]any)
	if !ok {
		return items
	}
	m[key] = items
	if total > 0 || page > 0 {
		pg, _ := m["pagination"].(map[string]any)
		if pg == nil {
			pg = map[string]any{}
		}
		pg["total"] = total
		if page > 0 {
			pg["page"] = page
		}
		m["pagination"] = pg
	}
	return m
}

// RowsAsAny converts []map[string]any to []any for JSON/jq compatibility.
func RowsAsAny(rows []map[string]any) []any {
	items := make([]any, len(rows))
	for i, row := range rows {
		items[i] = row
	}
	return items
}

// AsMap safely casts any to map[string]any.
func AsMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
