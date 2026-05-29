package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Life-USTC/CLI/internal/output"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type SearchForm struct {
	Title       string
	Description string
	SearchLabel string
	Search      string
	Limit       int
}

type SearchResult struct {
	Search string
	Limit  int
	Page   int
}

type TableResult struct {
	Rows  []map[string]any
	Total int
	Page  int
}

type SearchTable struct {
	Form         SearchForm
	Columns      []output.Column
	Fetch        func(SearchResult) (TableResult, error)
	EmptyMessage string
}

type tableMode int

const (
	tableModeSearch tableMode = iota
	tableModeLoading
	tableModeResults
)

type tableModel struct {
	spec     SearchTable
	inputs   []textinput.Model
	focus    int
	err      string
	mode     tableMode
	query    SearchResult
	result   TableResult
	selected int
	scroll   int
	width    int
	height   int
}

type tableFetchMsg struct {
	result TableResult
	err    error
}

func RunSearchTable(spec SearchTable) error {
	if spec.Fetch == nil {
		return fmt.Errorf("missing TUI fetch function")
	}
	model := newTableModel(spec)
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func newTableModel(spec SearchTable) tableModel {
	inputs, form := newSearchInputs(spec.Form)
	spec.Form = form
	return tableModel{
		spec:   spec,
		inputs: inputs,
		width:  96,
		height: 24,
	}
}

func newSearchInputs(form SearchForm) ([]textinput.Model, SearchForm) {
	if form.SearchLabel == "" {
		form.SearchLabel = "Search"
	}
	search := textinput.New()
	search.Prompt = form.SearchLabel + ": "
	search.Placeholder = "keyword, code, or blank"
	search.SetValue(form.Search)
	search.CharLimit = 120
	search.Width = 48
	search.Focus()

	limit := textinput.New()
	limit.Prompt = "Limit: "
	limit.Placeholder = "20"
	if form.Limit > 0 {
		limit.SetValue(strconv.Itoa(form.Limit))
	} else {
		limit.SetValue("20")
	}
	limit.CharLimit = 4
	limit.Width = 12

	return []textinput.Model{search, limit}, form
}

func (m tableModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentWidth := m.searchContentWidth()
		m.inputs[0].Width = inputWidth(contentWidth, m.inputs[0].Prompt, 60)
		m.inputs[1].Width = inputWidth(contentWidth, m.inputs[1].Prompt, 12)
		return m, nil
	case tableFetchMsg:
		m.mode = tableModeResults
		m.err = ""
		m.result = msg.result
		m.selected = 0
		m.scroll = 0
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case tableModeSearch:
			return m.updateSearch(msg)
		case tableModeLoading:
			switch msg.String() {
			case "ctrl+c", "esc", "q":
				return m, tea.Quit
			}
		case tableModeResults:
			return m.updateResults(msg)
		}
	}

	if m.mode != tableModeSearch {
		return m, nil
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m tableModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "enter":
		if m.focus < len(m.inputs)-1 {
			return m.focusNext(), nil
		}
		result, err := m.resultFromInputs()
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.query = result
		m.err = ""
		m.mode = tableModeLoading
		return m, m.fetch(result)
	case "tab", "shift+tab", "up", "down":
		return m.focusNext(), nil
	}
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m tableModel) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "/", "e", "enter":
		m.mode = tableModeSearch
		m.err = ""
		m = m.focusInput(0)
		return m, nil
	case "r":
		m.mode = tableModeLoading
		m.err = ""
		return m, m.fetch(m.query)
	case "right", "l", "n":
		if m.canPageNext() {
			m.query.Page = m.currentPage() + 1
			m.mode = tableModeLoading
			m.err = ""
			return m, m.fetch(m.query)
		}
	case "left", "h", "p":
		if m.currentPage() > 1 {
			m.query.Page = m.currentPage() - 1
			m.mode = tableModeLoading
			m.err = ""
			return m, m.fetch(m.query)
		}
	case "up", "k":
		if len(m.result.Rows) > 0 && m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if len(m.result.Rows) > 0 && m.selected < len(m.result.Rows)-1 {
			m.selected++
		}
	case "pgup":
		if len(m.result.Rows) > 0 {
			m.selected = max(0, m.selected-m.resultRowsAvailable())
		}
	case "pgdown":
		if len(m.result.Rows) > 0 {
			m.selected = min(len(m.result.Rows)-1, m.selected+m.resultRowsAvailable())
		}
	case "home":
		m.selected = 0
	case "end":
		if len(m.result.Rows) > 0 {
			m.selected = len(m.result.Rows) - 1
		}
	}
	m.syncScroll()
	return m, nil
}

func (m tableModel) fetch(query SearchResult) tea.Cmd {
	return func() tea.Msg {
		result, err := m.spec.Fetch(query)
		return tableFetchMsg{result: result, err: err}
	}
}

func (m tableModel) View() string {
	switch m.mode {
	case tableModeLoading:
		return m.loadingView()
	case tableModeResults:
		return m.resultsView()
	default:
		return m.searchView()
	}
}

func (m tableModel) searchView() string {
	title := formTitleStyle.Render(m.spec.Form.Title)
	contentWidth := m.searchContentWidth()
	fields := lipgloss.JoinVertical(lipgloss.Left, m.inputs[0].View(), m.inputs[1].View())

	compact := m.height > 0 && m.height < 16
	parts := []string{title}
	if !compact && m.spec.Form.Description != "" {
		description := mutedStyle.Width(contentWidth).Render(m.spec.Form.Description)
		parts = append(parts, description, "")
	}
	parts = append(parts, fields)

	if m.err != "" {
		parts = append(parts, errorStyle.Width(contentWidth).Render(m.err))
	}

	helpText := "enter search  •  tab switch field  •  esc quit"
	if compact {
		helpText = "enter search  •  tab  •  esc quit"
	}
	help := mutedStyle.Width(contentWidth).Render(helpText)
	if compact {
		parts = append(parts, help)
	} else {
		parts = append(parts, "", help)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return formBoxStyle.Width(contentWidth).Render(body)
}

func (m tableModel) loadingView() string {
	contentWidth := m.resultsContentWidth()
	query := querySummary(m.query)
	body := lipgloss.JoinVertical(lipgloss.Left,
		formTitleStyle.Render(m.spec.Form.Title),
		mutedStyle.Width(contentWidth).Render(query),
		"",
		"Loading results...",
		"",
		mutedStyle.Render("esc quit"),
	)
	return formBoxStyle.Width(contentWidth).Render(body)
}

func (m tableModel) resultsView() string {
	contentWidth := m.resultsContentWidth()
	compact := m.height > 0 && m.height < 16
	parts := []string{formTitleStyle.Render(m.spec.Form.Title)}
	if !compact {
		parts = append(parts, mutedStyle.Width(contentWidth).Render(querySummary(m.query)))
	}
	if m.err != "" {
		errText := m.err
		helpText := "r retry  •  / edit search  •  esc quit"
		if compact {
			errText = runewidth.Truncate(errText, contentWidth, "…")
			helpText = "r retry  •  / edit  •  esc quit"
		}
		parts = append(parts,
			errorStyle.Width(contentWidth).Render(errText),
			mutedStyle.Width(contentWidth).Render(helpText),
		)
		body := lipgloss.JoinVertical(lipgloss.Left, parts...)
		return formBoxStyle.Width(contentWidth).Render(body)
	}

	parts = append(parts, mutedStyle.Render(resultSummary(m.result)))
	if len(m.result.Rows) == 0 {
		msg := m.spec.EmptyMessage
		if msg == "" {
			msg = "No results found."
		}
		if !compact {
			parts = append(parts, "")
		}
		parts = append(parts, mutedStyle.Width(contentWidth).Render(msg))
	} else {
		if !compact {
			parts = append(parts, "")
		}
		parts = append(parts, m.renderTable(contentWidth))
	}
	helpText := m.resultsHelpText(compact)
	if compact {
		parts = append(parts, mutedStyle.Width(contentWidth).Render(helpText))
	} else {
		parts = append(parts, "", mutedStyle.Width(contentWidth).Render(helpText))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return formBoxStyle.Width(contentWidth).Render(body)
}

func (m tableModel) renderTable(width int) string {
	visibleRows := m.visibleRows()
	cols := tableColumnsForWidth(m.spec.Columns, width)
	if len(cols) == 0 {
		return mutedStyle.Render("No columns configured.")
	}
	gap := tableColumnGapFor(width, len(cols))
	colWidths := tableColumnWidths(m.result.Rows, cols, width, gap)
	lines := []string{tableLine(cols, nil, colWidths, gap, width, true)}
	for i, row := range visibleRows {
		absolute := m.scroll + i
		line := tableLine(cols, row, colWidths, gap, width, false)
		if absolute == m.selected {
			line = selectedRowStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	if len(m.result.Rows) > len(visibleRows) {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("Rows %d-%d of %d", m.scroll+1, m.scroll+len(visibleRows), len(m.result.Rows))))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m tableModel) visibleRows() []map[string]any {
	if len(m.result.Rows) == 0 {
		return nil
	}
	available := m.resultRowsAvailable()
	if available <= 0 {
		available = 1
	}
	end := min(len(m.result.Rows), m.scroll+available)
	return m.result.Rows[m.scroll:end]
}

func (m tableModel) resultRowsAvailable() int {
	if m.height <= 0 {
		return 8
	}
	return max(1, m.height-14)
}

func (m *tableModel) syncScroll() {
	available := m.resultRowsAvailable()
	if m.selected < m.scroll {
		m.scroll = m.selected
	}
	if m.selected >= m.scroll+available {
		m.scroll = m.selected - available + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m tableModel) currentPage() int {
	if m.result.Page > 0 {
		return m.result.Page
	}
	if m.query.Page > 0 {
		return m.query.Page
	}
	return 1
}

func (m tableModel) canPageNext() bool {
	if m.result.Total <= 0 || m.query.Limit <= 0 {
		return false
	}
	return m.currentPage()*m.query.Limit < m.result.Total
}

func (m tableModel) resultsHelpText(compact bool) string {
	if compact {
		if m.canPageNext() || m.currentPage() > 1 {
			return "p/n page  •  / edit  •  esc"
		}
		return "↑/↓  •  r  •  / edit  •  esc"
	}
	parts := []string{"↑/↓ move", "r refresh"}
	if m.currentPage() > 1 {
		parts = append(parts, "p prev")
	}
	if m.canPageNext() {
		parts = append(parts, "n next")
	}
	parts = append(parts, "/ edit search", "esc quit")
	return strings.Join(parts, "  •  ")
}

func (m tableModel) focusNext() tableModel {
	m.inputs[m.focus].Blur()
	m.focus = (m.focus + 1) % len(m.inputs)
	m.inputs[m.focus].Focus()
	m.err = ""
	return m
}

func (m tableModel) focusInput(index int) tableModel {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.focus = max(0, min(index, len(m.inputs)-1))
	m.inputs[m.focus].Focus()
	return m
}

func (m tableModel) resultFromInputs() (SearchResult, error) {
	limitRaw := strings.TrimSpace(m.inputs[1].Value())
	limit := 20
	if limitRaw != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil || parsed < 0 {
			return SearchResult{}, fmt.Errorf("limit must be a non-negative number")
		}
		limit = parsed
	}
	return SearchResult{
		Search: strings.TrimSpace(m.inputs[0].Value()),
		Limit:  limit,
		Page:   0,
	}, nil
}



func (m tableModel) searchContentWidth() int {
	return contentWidth(m.width, maxFormOuterWidth)
}

func (m tableModel) resultsContentWidth() int {
	return contentWidth(m.width, maxResultsOuterWidth)
}

func contentWidth(width, maxOuterWidth int) int {
	termWidth := width
	if termWidth <= 0 {
		termWidth = 72
	}
	outerWidth := min(maxOuterWidth, termWidth)
	if termWidth > maxOuterWidth+4 {
		outerWidth = maxOuterWidth
	} else if termWidth > 44 {
		outerWidth = termWidth - 4
	}
	return max(1, outerWidth-formBoxStyle.GetHorizontalFrameSize())
}

func inputWidth(contentWidth int, prompt string, maxWidth int) int {
	return max(1, min(maxWidth, contentWidth-lipgloss.Width(prompt)-1))
}

func querySummary(query SearchResult) string {
	search := query.Search
	if search == "" {
		search = "recent results"
	}
	parts := []string{fmt.Sprintf("Search: %s", search), fmt.Sprintf("Limit: %d", query.Limit)}
	if query.Page > 0 {
		parts = append(parts, fmt.Sprintf("Page: %d", query.Page))
	}
	return strings.Join(parts, "  •  ")
}

func resultSummary(result TableResult) string {
	if result.Total > 0 && len(result.Rows) > 0 {
		if result.Total > len(result.Rows) {
			if result.Page > 0 {
				return fmt.Sprintf("Showing %d of %d results  •  page %d", len(result.Rows), result.Total, result.Page)
			}
			return fmt.Sprintf("Showing %d of %d results", len(result.Rows), result.Total)
		}
		return fmt.Sprintf("%d result(s)", result.Total)
	}
	return fmt.Sprintf("%d result(s)", len(result.Rows))
}

func tableColumnsForWidth(cols []output.Column, width int) []output.Column {
	if width <= 0 || len(cols) == 0 {
		return nil
	}
	if len(cols) <= width {
		return cols
	}
	return cols[:width]
}

func tableColumnGapFor(width, colCount int) int {
	if colCount <= 1 {
		return 0
	}
	if colCount+(tableColumnGap*(colCount-1)) <= width {
		return tableColumnGap
	}
	if colCount+(colCount-1) <= width {
		return 1
	}
	return 0
}

func tableColumnWidths(rows []map[string]any, cols []output.Column, width, gap int) []int {
	if len(cols) == 0 {
		return nil
	}
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = max(1, runewidth.StringWidth(strings.ToUpper(col.Header)))
	}
	for _, row := range rows {
		for i, col := range cols {
			widths[i] = max(widths[i], runewidth.StringWidth(tableCell(row, col)))
		}
	}
	for tableWidth(widths, gap) > width {
		widest := 0
		for i := range widths {
			if widths[i] > widths[widest] {
				widest = i
			}
		}
		if widths[widest] <= 1 {
			break
		}
		widths[widest]--
	}
	return widths
}

func tableWidth(widths []int, gap int) int {
	total := 0
	for i, width := range widths {
		total += width
		if i < len(widths)-1 {
			total += gap
		}
	}
	return total
}

func tableLine(cols []output.Column, row map[string]any, widths []int, gap, width int, header bool) string {
	cells := make([]string, len(cols))
	for i, col := range cols {
		value := strings.ToUpper(col.Header)
		if !header {
			value = tableCell(row, col)
		}
		cells[i] = padCell(value, widths[i])
	}
	line := strings.Join(cells, strings.Repeat(" ", gap))
	if runewidth.StringWidth(line) > width {
		line = truncateCell(line, width)
	}
	if header {
		line = tableHeaderStyle.Render(line)
	}
	return line
}

func tableCell(row map[string]any, col output.Column) string {
	return output.FormatCellPlain(output.Resolve(row, col.Key))
}


func padCell(value string, width int) string {
	if runewidth.StringWidth(value) > width {
		value = truncateCell(value, width)
	}
	return value + strings.Repeat(" ", max(0, width-runewidth.StringWidth(value)))
}

func truncateCell(value string, width int) string {
	if width <= 0 {
		return ""
	}
	tail := "…"
	if width <= runewidth.StringWidth(tail) {
		tail = ""
	}
	return runewidth.Truncate(value, width, tail)
}

const (
	maxFormOuterWidth    = 76
	maxResultsOuterWidth = 120
	tableColumnGap       = 2
)

var (
	formBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42"))
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("246"))
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62"))
)
