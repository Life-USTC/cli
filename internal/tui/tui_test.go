package tui

import (
	"strings"
	"testing"

	"github.com/Life-USTC/CLI/internal/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/muesli/termenv"
)

func TestSearchResultFromInputs(t *testing.T) {
	m := newTableModel(SearchTable{Form: SearchForm{Title: "Courses", Search: "math", Limit: 15}})
	result, err := m.resultFromInputs()
	if err != nil {
		t.Fatal(err)
	}
	if result.Search != "math" || result.Limit != 15 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSearchResultDefaultsLimit(t *testing.T) {
	m := newTableModel(SearchTable{Form: SearchForm{Title: "Courses"}})
	m.inputs[1].SetValue("")
	result, err := m.resultFromInputs()
	if err != nil {
		t.Fatal(err)
	}
	if result.Limit != 20 {
		t.Fatalf("limit = %d, want 20", result.Limit)
	}
}

func TestSearchResultRejectsInvalidLimit(t *testing.T) {
	m := newTableModel(SearchTable{Form: SearchForm{Title: "Courses"}})
	m.inputs[1].SetValue("many")
	if _, err := m.resultFromInputs(); err == nil {
		t.Fatal("expected invalid limit error")
	}
}

func TestSearchViewFitsWindowWidths(t *testing.T) {
	for _, width := range []int{24, 30, 40, 80, 120} {
		m := newTableModel(SearchTable{
			Form: SearchForm{
				Title:       "Section Search",
				Description: "Search by course name, section code, teacher, or leave blank for recent results.",
				SearchLabel: "Section",
				Limit:       20,
			},
		})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
		rendered := updated.(tableModel).searchView()
		if renderedWidth := lipgloss.Width(rendered); renderedWidth > width {
			t.Fatalf("rendered width = %d, want <= %d", renderedWidth, width)
		}
	}
}

func TestSearchViewCompactsForShortWindows(t *testing.T) {
	m := newTableModel(SearchTable{
		Form: SearchForm{
			Title:       "Teacher Search",
			Description: "Search by teacher name, code, department, or leave blank for recent results.",
			SearchLabel: "Teacher",
			Limit:       20,
		},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	rendered := updated.(tableModel).searchView()
	if renderedHeight := lipgloss.Height(rendered); renderedHeight > 12 {
		t.Fatalf("rendered height = %d, want <= 12", renderedHeight)
	}
}

func TestSearchTableSubmitsAndShowsResults(t *testing.T) {
	var got SearchResult
	m := newTableModel(SearchTable{
		Form: SearchForm{Title: "Course Search", SearchLabel: "Course", Search: "math", Limit: 2},
		Columns: []output.Column{
			{Header: "Code", Key: "code"},
			{Header: "Name", Key: "namePrimary"},
		},
		Fetch: func(result SearchResult) (TableResult, error) {
			got = result
			return TableResult{
				Rows: []map[string]any{
					{"code": "MATH101", "namePrimary": "Calculus"},
					{"code": "MATH202", "namePrimary": "Linear Algebra"},
				},
				Total: 2,
				Page:  1,
			}, nil
		},
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tableModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tableModel)
	if m.mode != tableModeLoading {
		t.Fatalf("mode = %v, want loading", m.mode)
	}
	msg := cmd().(tableFetchMsg)
	updated, _ = m.Update(msg)
	m = updated.(tableModel)

	if got.Search != "math" || got.Limit != 2 {
		t.Fatalf("fetch query = %#v", got)
	}
	rendered := m.View()
	if m.mode != tableModeResults || !strings.Contains(rendered, "Calculus") || !strings.Contains(rendered, "/ edit search") {
		t.Fatalf("results view did not render expected content:\n%s", rendered)
	}
}

func TestSearchTableEditReturnsToSearchWithoutQuitting(t *testing.T) {
	m := newTableModel(SearchTable{
		Form:    SearchForm{Title: "Teacher Search", SearchLabel: "Teacher"},
		Columns: []output.Column{{Header: "Name", Key: "namePrimary"}},
		Fetch: func(SearchResult) (TableResult, error) {
			return TableResult{Rows: []map[string]any{{"namePrimary": "Ada"}}}, nil
		},
	})
	m.mode = tableModeResults
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(tableModel)
	if cmd != nil {
		t.Fatal("edit should not quit or fetch")
	}
	if m.mode != tableModeSearch {
		t.Fatalf("mode = %v, want search", m.mode)
	}
	if m.focus != 0 || !m.inputs[0].Focused() {
		t.Fatalf("focus = %d, search focused = %v", m.focus, m.inputs[0].Focused())
	}
}

func TestSearchTableEnterSelectsHighlightedRow(t *testing.T) {
	m := newTableModel(SearchTable{
		OnSelect: func(map[string]any) error { return nil },
	})
	m.mode = tableModeResults
	m.selected = 1
	m.result.Rows = []map[string]any{
		{"jwId": float64(101)},
		{"jwId": float64(202)},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tableModel)
	if cmd == nil {
		t.Fatal("enter should quit the TUI before opening the selected result")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("enter command = %T, want tea.QuitMsg", cmd())
	}
	if got := m.selection["jwId"]; got != float64(202) {
		t.Fatalf("selected jwId = %v, want 202", got)
	}
}

func TestSearchTableNoColorUsesAsciiProfile(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	previousNoColor := output.Current.NoColor
	previousFatihNoColor := color.NoColor
	lipgloss.SetColorProfile(termenv.ANSI256)
	output.Current.NoColor = true
	color.NoColor = false
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
		output.Current.NoColor = previousNoColor
		color.NoColor = previousFatihNoColor
	})

	restore := configureSearchTableColors()
	if got := lipgloss.ColorProfile(); got != termenv.Ascii {
		t.Fatalf("color profile = %v, want Ascii", got)
	}
	restore()
	if got := lipgloss.ColorProfile(); got != termenv.ANSI256 {
		t.Fatalf("restored color profile = %v, want ANSI256", got)
	}
}

func TestSearchTableCanPageInsideResults(t *testing.T) {
	var got SearchResult
	m := newTableModel(SearchTable{
		Form:    SearchForm{Title: "Course Search", SearchLabel: "Course", Limit: 20},
		Columns: []output.Column{{Header: "Code", Key: "code"}},
		Fetch: func(result SearchResult) (TableResult, error) {
			got = result
			return TableResult{
				Rows:  []map[string]any{{"code": "MATH202"}},
				Total: 50,
				Page:  result.Page,
			}, nil
		},
	})
	m.mode = tableModeResults
	m.query = SearchResult{Limit: 20}
	m.result = TableResult{
		Rows:  []map[string]any{{"code": "MATH101"}},
		Total: 50,
		Page:  1,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(tableModel)
	if m.mode != tableModeLoading || cmd == nil {
		t.Fatalf("mode = %v, cmd nil = %v", m.mode, cmd == nil)
	}
	msg := cmd().(tableFetchMsg)
	updated, _ = m.Update(msg)
	m = updated.(tableModel)
	if got.Page != 2 || m.result.Page != 2 {
		t.Fatalf("page request = %d, result page = %d; want 2", got.Page, m.result.Page)
	}
}

func TestSearchTableResultsFitCompactWindow(t *testing.T) {
	m := newTableModel(SearchTable{
		Form: SearchForm{Title: "Section Search", SearchLabel: "Section"},
		Columns: []output.Column{
			{Header: "Code", Key: "code"},
			{Header: "Course", Key: "course.namePrimary"},
			{Header: "Semester", Key: "semester.name"},
			{Header: "ID", Key: "id"},
		},
		Fetch: func(SearchResult) (TableResult, error) {
			return TableResult{}, nil
		},
		OnSelect: func(map[string]any) error { return nil },
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(tableModel)
	m.mode = tableModeResults
	m.query = SearchResult{Search: "calculus", Limit: 20}
	m.result = TableResult{
		Rows: []map[string]any{
			{
				"code":     "SECTION-001",
				"course":   map[string]any{"namePrimary": "A very long course name that needs truncation"},
				"semester": map[string]any{"name": "Spring 2026"},
				"id":       "section-id",
			},
		},
		Total: 1,
		Page:  1,
	}

	rendered := m.View()
	if renderedWidth := lipgloss.Width(rendered); renderedWidth > 40 {
		t.Fatalf("rendered width = %d, want <= 40\n%s", renderedWidth, rendered)
	}
	if renderedHeight := lipgloss.Height(rendered); renderedHeight > 12 {
		t.Fatalf("rendered height = %d, want <= 12\n%s", renderedHeight, rendered)
	}
	if !strings.Contains(rendered, "enter open") {
		t.Fatalf("compact results did not show the selection action:\n%s", rendered)
	}
}

func TestSearchTableSelectedCJKRowDoesNotWrapWithColor(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previousProfile)
	})

	m := newTableModel(SearchTable{
		Columns: []output.Column{
			{Header: "Code", Key: "code"},
			{Header: "Name", Key: "namePrimary"},
			{Header: "Level", Key: "educationLevel.name"},
			{Header: "JW ID", Key: "jwId"},
		},
		OnSelect: func(map[string]any) error { return nil },
	})
	m.mode = tableModeResults
	m.result = TableResult{
		Rows: []map[string]any{
			{
				"code":           "MATH101",
				"namePrimary":    "数学分析(B1)",
				"educationLevel": map[string]any{"name": "Undergraduate"},
				"jwId":           "course-jw-id-1",
			},
			{
				"code":           "MATH102",
				"namePrimary":    "线性代数(A)",
				"educationLevel": map[string]any{"name": "Undergraduate"},
				"jwId":           "course-jw-id-2",
			},
		},
		Total: 2,
		Page:  1,
	}

	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 40, height: 12},
	} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		sized := updated.(tableModel)
		selected := sized.View()
		sized.selected = len(sized.result.Rows)
		unselected := sized.View()
		if selectedHeight, unselectedHeight := lipgloss.Height(selected), lipgloss.Height(unselected); selectedHeight != unselectedHeight {
			t.Fatalf("%dx%d selected view height = %d, unselected view height = %d\nselected:\n%s\nunselected:\n%s", size.width, size.height, selectedHeight, unselectedHeight, selected, unselected)
		}
		if selectedHeight := lipgloss.Height(selected); selectedHeight > size.height {
			t.Fatalf("%dx%d selected view height = %d\n%s", size.width, size.height, selectedHeight, selected)
		}
		if selectedWidth := lipgloss.Width(selected); selectedWidth > size.width {
			t.Fatalf("%dx%d selected view width = %d\n%s", size.width, size.height, selectedWidth, selected)
		}
	}
}

func TestSearchTableErrorFitsCompactWindow(t *testing.T) {
	m := newTableModel(SearchTable{
		Form:    SearchForm{Title: "Course Search", SearchLabel: "Course"},
		Columns: []output.Column{{Header: "Code", Key: "code"}},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(tableModel)
	m.mode = tableModeResults
	m.query = SearchResult{Limit: 20}
	m.err = `Get "http://127.0.0.1:1/api/courses?limit=20": dial tcp 127.0.0.1:1: socket: operation not permitted`

	rendered := m.View()
	if renderedWidth := lipgloss.Width(rendered); renderedWidth > 40 {
		t.Fatalf("rendered width = %d, want <= 40\n%s", renderedWidth, rendered)
	}
	if renderedHeight := lipgloss.Height(rendered); renderedHeight > 12 {
		t.Fatalf("rendered height = %d, want <= 12\n%s", renderedHeight, rendered)
	}
}
