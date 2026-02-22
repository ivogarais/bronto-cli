package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/mattn/go-runewidth"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
)

type Model struct {
	Spec     *spec.AppSpec
	SpecPath string
	Theme    brontotheme.BrontoTheme

	Charts map[string]barchart.Model
	Tables map[string]table.Model

	FocusablePanels []focusPanel
	PanelNumberByID map[string]int
	FocusedPanel    int
	FocusScrollY    int

	TableBaseRows   map[string][]table.Row
	TableFilter     map[string]string
	TableFilterMode bool

	Width    int
	Height   int
	ContentH int
	ScrollY  int
	Status   string
	LoadedAt time.Time
}

func NewModel(s *spec.AppSpec, specPath string) Model {
	density := "comfortable"
	if s != nil {
		density = s.Theme.Density
	}

	m := Model{
		Spec:            s,
		SpecPath:        specPath,
		Theme:           brontotheme.NewBrontoTheme(density),
		Charts:          map[string]barchart.Model{},
		Tables:          map[string]table.Model{},
		PanelNumberByID: map[string]int{},
		TableBaseRows:   map[string][]table.Row{},
		TableFilter:     map[string]string{},
		Status:          "Snapshot loaded",
		LoadedAt:        time.Now(),
	}

	m.resolveComponents()
	m.indexFocusablePanels()
	m.resizeForLayout(120, 36)
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.TableFilterMode {
			if tableRef, ok := m.focusedTableRef(); ok {
				if m.handleTableFilterInput(tableRef, msg) {
					return m, nil
				}
			} else {
				m.TableFilterMode = false
			}
		}

		if number, ok := parseFocusNumberKey(msg); ok {
			if _, exists := m.focusPanelByNumber(number); exists {
				m.FocusedPanel = number
				m.ScrollY = 0
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "0":
			if m.FocusedPanel > 0 {
				m.FocusedPanel = 0
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
				return m, nil
			}
			if msg.String() == "esc" {
				return m, tea.Quit
			}
		case "tab":
			if len(m.FocusablePanels) > 0 {
				next := m.FocusedPanel + 1
				if next > len(m.FocusablePanels) || next <= 0 {
					next = 1
				}
				m.FocusedPanel = next
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		case "shift+tab":
			if len(m.FocusablePanels) > 0 {
				prev := m.FocusedPanel - 1
				if prev <= 0 || prev > len(m.FocusablePanels) {
					prev = len(m.FocusablePanels)
				}
				m.FocusedPanel = prev
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		}

		if tableRef, ok := m.focusedTableRef(); ok {
			switch msg.String() {
			case "/":
				m.TableFilterMode = true
				if _, exists := m.TableFilter[tableRef]; !exists {
					m.TableFilter[tableRef] = ""
				}
				return m, nil
			case "c":
				if m.TableFilter[tableRef] != "" {
					m.TableFilter[tableRef] = ""
					m.applyTableFilter(tableRef)
				}
				return m, nil
			}

			t, exists := m.Tables[tableRef]
			if !exists {
				return m, nil
			}
			var cmd tea.Cmd
			t, cmd = t.Update(msg)
			m.Tables[tableRef] = t
			return m, cmd
		}

		if m.FocusedPanel > 0 {
			switch msg.String() {
			case "down", "j", "ctrl+n":
				m.focusScrollBy(1)
				return m, nil
			case "up", "k", "ctrl+p":
				m.focusScrollBy(-1)
				return m, nil
			case "pgdown", "ctrl+f", "ctrl+d":
				m.focusScrollBy(maxInt(1, m.Height-4))
				return m, nil
			case "pgup", "ctrl+b", "ctrl+u":
				m.focusScrollBy(-maxInt(1, m.Height-4))
				return m, nil
			case "home", "g":
				m.FocusScrollY = 0
				return m, nil
			case "end", "G":
				m.FocusScrollY = m.focusMaxScroll()
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "down", "j", "ctrl+n":
			m.scrollBy(1)
			return m, nil
		case "up", "k", "ctrl+p":
			m.scrollBy(-1)
			return m, nil
		case "pgdown", "ctrl+f", "ctrl+d":
			m.scrollBy(maxInt(1, m.Height-4))
			return m, nil
		case "pgup", "ctrl+b", "ctrl+u":
			m.scrollBy(-maxInt(1, m.Height-4))
			return m, nil
		case "home", "g":
			m.ScrollY = 0
			return m, nil
		case "end", "G":
			m.ScrollY = m.maxScroll()
			return m, nil
		}

		for id, t := range m.Tables {
			var cmd tea.Cmd
			t, cmd = t.Update(msg)
			m.Tables[id] = t
			if cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.resizeForLayout(msg.Width, msg.Height)
		return m, nil
	}

	return m, nil
}

func (m Model) View() tea.View {
	if m.Spec == nil {
		v := tea.NewView("No spec loaded.\n")
		v.AltScreen = true
		return v
	}

	width := m.Width
	height := m.Height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 36
	}

	body := ""
	if panel, ok := m.focusPanelByNumber(m.FocusedPanel); ok {
		hints := []string{
			m.Theme.Muted.Render("(focus mode: 0/esc exit | tab next | shift+tab prev | q quit)"),
		}
		if panel.Node.Type == "table" {
			tableRef := panel.Node.TableRef
			query := m.TableFilter[tableRef]
			switch {
			case m.TableFilterMode:
				hints = append(hints, m.Theme.Primary.Render(fmt.Sprintf("filter> %s_", query)))
			case query != "":
				hints = append(hints, m.Theme.Muted.Render(fmt.Sprintf("filter: %q ( / edit | c clear )", query)))
			default:
				hints = append(hints, m.Theme.Muted.Render("table controls: arrows/pgup/pgdn | / filter | c clear"))
			}
		} else {
			hints = append(hints, m.Theme.Muted.Render("scroll focused panel: up/down | pgup/pgdn | home/end"))
		}

		hintBlock := strings.Join(hints, "\n")
		viewH := maxInt(1, height-len(hints))

		renderH := viewH
		if panel.Node.Type != "table" {
			renderH = m.focusRenderHeight(panel.Node, width, viewH)
		}

		panelBody := m.renderNode(panel.Node, width, renderH)
		if panel.Node.Type != "table" {
			panelBody = clampViewport(panelBody, m.FocusScrollY, viewH)
		}
		body = hintBlock + "\n" + panelBody
	} else {
		contentH := m.ContentH
		if contentH <= 0 {
			contentH = maxInt(height, m.preferredNodeHeight(m.Spec.Layout, width))
		}
		body = m.renderNode(m.Spec.Layout, width, contentH)
		body = clampViewport(body, m.ScrollY, height)
	}
	body = m.Theme.AppBg.Copy().Render(body)
	v := tea.NewView(body + "\n")
	v.AltScreen = true
	return v
}

func (m *Model) resolveComponents() {
	if m.Spec == nil {
		return
	}

	m.Charts = map[string]barchart.Model{}
	for chartID, chartSpec := range m.Spec.Charts {
		if chartSpec.Family != "bar" {
			continue
		}

		ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]
		if !ok {
			continue
		}

		opts := []barchart.Option{
			barchart.WithStyles(m.Theme.ChartAxis, m.Theme.ChartLabel),
			barchart.WithBarGap(barGapForDensity(m.Theme.Density)),
		}
		if chartSpec.Bar != nil && chartSpec.Bar.Orientation == "horizontal" {
			opts = append(opts, barchart.WithHorizontalBars())
		}
		if chartSpec.Render.ShowAxis != nil && !*chartSpec.Render.ShowAxis {
			opts = append(opts, barchart.WithNoAxis())
		}

		bar := barchart.New(50, 12, opts...)
		if chartSpec.Render.ShowAxis != nil {
			bar.SetShowAxis(*chartSpec.Render.ShowAxis)
		}
		setBarData(&bar, m.Theme.ChartBar, m.Theme.ChartDanger, ds.Labels, ds.Values, axisLabelMaxWidth(50))
		m.Charts[chartID] = bar
	}

	m.Tables = map[string]table.Model{}
	m.TableBaseRows = map[string][]table.Row{}
	m.TableFilter = map[string]string{}
	for tableID, tableSpec := range m.Spec.Tables {
		ds, ok := m.Spec.Datasets[tableSpec.DatasetRef]
		if !ok {
			continue
		}

		cols := buildTableColumns(tableSpec, ds, 80)
		rows := buildTableRowsForColumns(tableSpec, ds, cols)

		t := table.New(
			table.WithColumns(cols),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithStyles(buildTableStyles(m.Theme)),
			table.WithWidth(80),
			table.WithHeight(12),
		)
		m.Tables[tableID] = t
		m.TableBaseRows[tableID] = cloneRows(rows)
		m.TableFilter[tableID] = ""
	}
}

func (m *Model) resizeForLayout(width, height int) {
	if m.Spec == nil {
		return
	}

	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 36
	}

	m.Width = width
	m.Height = height
	m.ContentH = maxInt(height, m.preferredNodeHeight(m.Spec.Layout, width))
	m.resizeNode(m.Spec.Layout, width, m.ContentH)
	if panel, ok := m.focusPanelByNumber(m.FocusedPanel); ok {
		m.resizeNode(panel.Node, width, maxInt(1, height-2))
		m.FocusScrollY = clampInt(m.FocusScrollY, 0, m.focusMaxScroll())
	} else {
		m.FocusedPanel = 0
		m.FocusScrollY = 0
		m.TableFilterMode = false
	}
	m.ScrollY = clampInt(m.ScrollY, 0, m.maxScroll())
}

func (m *Model) resizeNode(n spec.Node, width, height int) {
	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return
		}
		gap := layoutGap(m.Theme.Density, height)
		childHeights := splitByPreferences(
			maxInt(1, height-gap*(len(n.Children)-1)),
			m.preferredChildHeightsForColumn(n.Children, width),
		)
		for i, ch := range n.Children {
			m.resizeNode(ch, width, childHeights[i])
		}

	case "row":
		if len(n.Children) == 0 {
			return
		}
		wrapped := wrapRowChildren(n.Children, n.Weights, maxPanelsPerRow)
		if len(wrapped) > 1 {
			gap := layoutGap(m.Theme.Density, height)
			rowHeights := splitByPreferences(
				maxInt(1, height-gap*(len(wrapped)-1)),
				m.preferredHeightsForWrappedRows(wrapped, width),
			)
			for i, group := range wrapped {
				rowNode := spec.Node{
					Type:     "row",
					Children: group.Children,
					Weights:  group.Weights,
				}
				m.resizeNode(rowNode, width, rowHeights[i])
			}
			return
		}
		gap := layoutGap(m.Theme.Density, width)
		childWidths := splitByWeights(maxInt(1, width-gap*(len(n.Children)-1)), len(n.Children), n.Weights)
		for i, ch := range n.Children {
			m.resizeNode(ch, childWidths[i], height)
		}

	case "chart":
		c, ok := m.Charts[n.ChartRef]
		if !ok {
			return
		}
		chartSpec, hasSpec := m.Spec.Charts[n.ChartRef]
		if hasSpec && chartSpec.Render.ShowAxis != nil {
			c.SetShowAxis(*chartSpec.Render.ShowAxis)
		}
		padV, padH := panelPadding(width, height, m.Theme.Density)
		chartW := maxInt(1, width-2-(2*padH))
		baseH := maxInt(1, height-2-(2*padV)-2)
		chartH := baseH

		if hasSpec && chartSpec.Family == "bar" {
			if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
				labelMax := axisLabelMaxWidth(chartW)
				setBarData(&c, m.Theme.ChartBar, m.Theme.ChartDanger, ds.Labels, ds.Values, labelMax)

				maxLegendLines := m.legendMaxLinesForChart(n, height)
				legendLines := m.chartLegendLines(n.ChartRef, width, maxLegendLines)
				if legendLines > 0 && baseH > 4 {
					reserve := minInt(legendLines+1, maxInt(2, baseH-3))
					chartH = maxInt(3, baseH-reserve)
				}
			}
		}
		c.Resize(chartW, chartH)
		c.Draw()
		m.Charts[n.ChartRef] = c

	case "table":
		t, ok := m.Tables[n.TableRef]
		if !ok || m.Spec == nil {
			return
		}

		tableSpec, ok := m.Spec.Tables[n.TableRef]
		if !ok {
			return
		}
		ds, ok := m.Spec.Datasets[tableSpec.DatasetRef]
		if !ok {
			return
		}

		padV, padH := panelPadding(width, height, m.Theme.Density)
		tableW := maxInt(1, width-2-(2*padH))
		tableH := maxInt(1, height-2-(2*padV)-2)
		cols := buildTableColumns(tableSpec, ds, tableW)
		t.SetColumns(cols)
		t.SetWidth(tableW)
		t.SetHeight(tableH)
		t.SetStyles(buildTableStyles(m.Theme))
		rows := buildTableRowsForColumns(tableSpec, ds, cols)
		m.TableBaseRows[n.TableRef] = cloneRows(rows)
		m.Tables[n.TableRef] = t
		m.applyTableFilter(n.TableRef)
	}
}

func (m Model) renderNode(n spec.Node, width, height int) string {
	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return ""
		}

		gap := layoutGap(m.Theme.Density, height)
		childHeights := splitByPreferences(
			maxInt(1, height-gap*(len(n.Children)-1)),
			m.preferredChildHeightsForColumn(n.Children, width),
		)
		parts := make([]string, 0, len(n.Children))
		for i, ch := range n.Children {
			child := m.renderNode(ch, width, childHeights[i])
			child = lipgloss.NewStyle().Width(width).MaxWidth(width).Render(child)
			parts = append(parts, child)
		}
		return strings.Join(parts, strings.Repeat("\n", gap+1))

	case "row":
		if len(n.Children) == 0 {
			return ""
		}

		wrapped := wrapRowChildren(n.Children, n.Weights, maxPanelsPerRow)
		if len(wrapped) > 1 {
			gap := layoutGap(m.Theme.Density, height)
			rowHeights := splitByPreferences(
				maxInt(1, height-gap*(len(wrapped)-1)),
				m.preferredHeightsForWrappedRows(wrapped, width),
			)
			rows := make([]string, 0, len(wrapped))
			for i, group := range wrapped {
				rowNode := spec.Node{
					Type:     "row",
					Children: group.Children,
					Weights:  group.Weights,
				}
				row := m.renderNode(rowNode, width, rowHeights[i])
				rows = append(rows, lipgloss.NewStyle().Width(width).MaxWidth(width).Render(row))
			}
			return strings.Join(rows, strings.Repeat("\n", gap+1))
		}

		gap := layoutGap(m.Theme.Density, width)
		childWidths := splitByWeights(maxInt(1, width-gap*(len(n.Children)-1)), len(n.Children), n.Weights)
		parts := make([]string, 0, len(n.Children))
		for i, ch := range n.Children {
			child := m.renderNode(ch, childWidths[i], height)
			child = lipgloss.NewStyle().Width(childWidths[i]).MaxWidth(childWidths[i]).Render(child)
			if i < len(n.Children)-1 && gap > 0 {
				child = lipgloss.NewStyle().MarginRight(gap).Render(child)
			}
			parts = append(parts, child)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	case "header":
		title := "Dashboard"
		if m.Spec != nil && n.TitleRef == "$title" {
			title = m.Spec.Title
		}
		header := fmt.Sprintf("%s\n%s",
			m.Theme.PanelAccent.Render("▌ ")+m.Theme.AppTitle.Render(title),
			m.Theme.Muted.Render("(q quit | up/down scroll | 1-9 focus panel | 0/esc exit focus)"),
		)
		header = trimTrailingWhitespace(header)

		box := m.Theme.HeaderBox.Copy().Width(width)
		return box.Render(header)

	case "chart":
		title := n.Title
		chartSpec, chartFound := m.Spec.Charts[n.ChartRef]
		if title == "" && chartFound && chartSpec.Title != "" {
			title = chartSpec.Title
		}
		if title == "" {
			title = n.ChartRef
		}
		if number := m.panelNumberForNode(n); number > 0 {
			title = fmt.Sprintf("[%d] %s", number, title)
		}

		content := "(missing chart)"
		var legend string
		if c, ok := m.Charts[n.ChartRef]; ok {
			content = c.View()
			if chartFound && chartSpec.Family == "bar" {
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					maxLegendLines := m.legendMaxLinesForChart(n, height)
					legend = renderBarLegend(m.Theme, ds, width, maxLegendLines)
				}
			}
		} else if chartFound {
			if chartSpec.Family != "bar" {
				content = fmt.Sprintf("unsupported chart family %q (renderer currently implements bar only)", chartSpec.Family)
			} else {
				content = "(chart data unavailable)"
			}
		}
		if legend != "" {
			content += "\n" + legend
		}
		if width < minChartPanelWidth || height < minChartPanelHeight {
			content = panelTooSmallMessage(width, height, minChartPanelWidth, minChartPanelHeight)
		}
		return renderPanel(m.Theme, title, content, width, height)

	case "table":
		title := n.Title
		if title == "" {
			title = n.TableRef
		}
		if number := m.panelNumberForNode(n); number > 0 {
			title = fmt.Sprintf("[%d] %s", number, title)
		}
		content := "(missing table)"
		if t, ok := m.Tables[n.TableRef]; ok {
			content = t.View()
		}
		if width < minTablePanelWidth || height < minTablePanelHeight {
			content = panelTooSmallMessage(width, height, minTablePanelWidth, minTablePanelHeight)
		}
		return renderPanel(m.Theme, title, content, width, height)

	case "text":
		st := m.Theme.Text.Copy().Width(width)
		switch n.Variant {
		case "muted":
			st = m.Theme.Muted.Copy().Width(width)
		case "danger":
			st = m.Theme.Danger.Copy().Width(width)
		case "primary":
			st = m.Theme.Primary.Copy().Width(width)
		}
		return st.Render(n.Text)
	}

	return renderPanel(m.Theme, "Unsupported", fmt.Sprintf("node type %q is not supported", n.Type), width, height)
}

func renderPanel(th brontotheme.BrontoTheme, title, body string, width, height int) string {
	padV, padH := panelPadding(width, height, th.Density)
	spacing := "\n\n"
	if width < 50 || height < 10 || th.Density == "compact" {
		spacing = "\n"
	}

	panel := th.Panel.Copy().
		Padding(padV, padH).
		Width(width)
	titleLine := th.PanelAccent.Render("▌ ") + th.PanelTitle.Render(title)
	innerW := maxInt(1, width-2-(2*padH))
	body = lipgloss.NewStyle().
		Width(innerW).
		MaxWidth(innerW).
		Render(body)
	body = trimTrailingWhitespace(body)
	return panel.Render(titleLine + spacing + body)
}

func trimTrailingWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

const (
	maxPanelsPerRow = 3

	preferredChartPanelHeight = 14
	preferredTablePanelHeight = 12
	preferredHeaderHeight     = 4

	minChartPanelWidth  = 24
	minChartPanelHeight = 8
	minTablePanelWidth  = 30
	minTablePanelHeight = 6

	minAnyWidth   = 6
	minAutoWidth  = 8
	minFixedWidth = 6
	minFlexWidth  = 12
)

type rowGroup struct {
	Children []spec.Node
	Weights  []int
}

type focusPanel struct {
	Number int
	Node   spec.Node
}

type colType int

const (
	colDefault colType = iota
	colTimestamp
	colLevel
	colService
	colID
	colMessage
	colNumeric
)

func parseFocusNumberKey(msg tea.KeyMsg) (int, bool) {
	s := msg.String()
	if len(s) != 1 {
		return 0, false
	}
	ch := s[0]
	if ch < '1' || ch > '9' {
		return 0, false
	}
	return int(ch - '0'), true
}

func (m *Model) indexFocusablePanels() {
	m.FocusablePanels = nil
	m.PanelNumberByID = map[string]int{}
	m.FocusedPanel = 0

	if m.Spec == nil {
		return
	}

	next := 1
	var walk func(n spec.Node)
	walk = func(n spec.Node) {
		switch n.Type {
		case "chart", "table":
			m.FocusablePanels = append(m.FocusablePanels, focusPanel{
				Number: next,
				Node:   n,
			})
			if n.ID != "" {
				m.PanelNumberByID[n.ID] = next
			}
			next++
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(m.Spec.Layout)
}

func (m Model) focusPanelByNumber(number int) (focusPanel, bool) {
	if number <= 0 {
		return focusPanel{}, false
	}
	index := number - 1
	if index < 0 || index >= len(m.FocusablePanels) {
		return focusPanel{}, false
	}
	return m.FocusablePanels[index], true
}

func (m Model) panelNumberForNode(n spec.Node) int {
	if n.ID == "" || m.PanelNumberByID == nil {
		return 0
	}
	return m.PanelNumberByID[n.ID]
}

func (m Model) focusedTableRef() (string, bool) {
	panel, ok := m.focusPanelByNumber(m.FocusedPanel)
	if !ok || panel.Node.Type != "table" || panel.Node.TableRef == "" {
		return "", false
	}
	return panel.Node.TableRef, true
}

func (m *Model) handleTableFilterInput(tableRef string, msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		m.TableFilterMode = false
		return true
	case "enter":
		m.TableFilterMode = false
		return true
	case "backspace", "ctrl+h":
		m.TableFilter[tableRef] = removeLastRune(m.TableFilter[tableRef])
		m.applyTableFilter(tableRef)
		return true
	case "ctrl+u":
		m.TableFilter[tableRef] = ""
		m.applyTableFilter(tableRef)
		return true
	}

	if text, ok := keyText(msg); ok {
		m.TableFilter[tableRef] += text
		m.applyTableFilter(tableRef)
		return true
	}
	return false
}

func keyText(msg tea.KeyMsg) (string, bool) {
	s := msg.String()
	if len(s) != 1 {
		return "", false
	}
	if strings.HasPrefix(s, " ") {
		return s, true
	}
	ch := s[0]
	if ch < 32 || ch == 127 {
		return "", false
	}
	return s, true
}

func removeLastRune(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func (m *Model) applyTableFilter(tableRef string) {
	t, ok := m.Tables[tableRef]
	if !ok {
		return
	}

	base := m.TableBaseRows[tableRef]
	query := strings.TrimSpace(strings.ToLower(m.TableFilter[tableRef]))
	if query == "" {
		t.SetRows(cloneRows(base))
		m.Tables[tableRef] = t
		return
	}

	tokens := strings.Fields(query)
	filtered := make([]table.Row, 0, len(base))
	for _, row := range base {
		matched := true
		rowText := strings.ToLower(strings.Join(row, " "))
		for _, tok := range tokens {
			if !strings.Contains(rowText, tok) {
				matched = false
				break
			}
		}
		if matched {
			filtered = append(filtered, cloneRow(row))
		}
	}
	t.SetRows(filtered)
	m.Tables[tableRef] = t
}

func cloneRow(row table.Row) table.Row {
	copied := make(table.Row, len(row))
	copy(copied, row)
	return copied
}

func cloneRows(rows []table.Row) []table.Row {
	out := make([]table.Row, len(rows))
	for i := range rows {
		out[i] = cloneRow(rows[i])
	}
	return out
}

func (m *Model) focusScrollBy(delta int) {
	if delta == 0 {
		return
	}
	m.FocusScrollY = clampInt(m.FocusScrollY+delta, 0, m.focusMaxScroll())
}

func (m Model) focusMaxScroll() int {
	panel, ok := m.focusPanelByNumber(m.FocusedPanel)
	if !ok || panel.Node.Type == "table" {
		return 0
	}
	viewH := maxInt(1, m.Height-2)
	contentH := m.focusRenderHeight(panel.Node, m.Width, viewH)
	return maxInt(0, contentH-viewH)
}

func (m Model) legendMaxLinesForChart(n spec.Node, panelHeight int) int {
	innerH := maxInt(3, panelHeight-4)
	if m.panelNumberForNode(n) == m.FocusedPanel {
		return maxInt(4, innerH/2)
	}
	return maxInt(3, innerH/3)
}

func (m Model) chartLegendLines(chartRef string, panelWidth, maxLines int) int {
	if m.Spec == nil {
		return 0
	}
	chartSpec, ok := m.Spec.Charts[chartRef]
	if !ok || chartSpec.Family != "bar" {
		return 0
	}
	ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]
	if !ok {
		return 0
	}

	totalItems := len(ds.Labels)
	if totalItems > len(ds.Values) {
		totalItems = len(ds.Values)
	}
	if totalItems <= 0 {
		return 0
	}

	visible := legendVisibleItemCount(totalItems, panelWidth, maxLines)
	if visible <= 0 {
		return 0
	}
	return legendLineCount(visible, totalItems)
}

func (m Model) focusRenderHeight(n spec.Node, width, viewH int) int {
	if n.Type == "chart" {
		return viewH
	}
	contentH := m.preferredNodeHeight(n, width) + 4
	return maxInt(viewH, contentH)
}

func (m *Model) scrollBy(delta int) {
	if delta == 0 {
		return
	}
	m.ScrollY = clampInt(m.ScrollY+delta, 0, m.maxScroll())
}

func (m Model) maxScroll() int {
	if m.ContentH <= m.Height {
		return 0
	}
	return maxInt(0, m.ContentH-m.Height)
}

func clampViewport(content string, start, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	maxStart := maxInt(0, len(lines)-maxLines)
	start = clampInt(start, 0, maxStart)
	end := minInt(len(lines), start+maxLines)
	visible := lines[start:end]
	for len(visible) < maxLines {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

func wrapRowChildren(children []spec.Node, weights []int, perRow int) []rowGroup {
	if perRow <= 0 {
		perRow = maxPanelsPerRow
	}
	if len(children) <= perRow {
		return []rowGroup{{Children: children, Weights: weights}}
	}

	useWeights := len(weights) == len(children)
	groups := make([]rowGroup, 0, (len(children)+perRow-1)/perRow)
	for start := 0; start < len(children); start += perRow {
		end := minInt(len(children), start+perRow)
		group := rowGroup{
			Children: append([]spec.Node(nil), children[start:end]...),
		}
		if useWeights {
			group.Weights = append([]int(nil), weights[start:end]...)
		}
		groups = append(groups, group)
	}
	return groups
}

func splitByPreferences(total int, prefs []int) []int {
	if len(prefs) == 0 {
		return nil
	}
	if total < len(prefs) {
		total = len(prefs)
	}

	normalized := make([]int, len(prefs))
	sumPref := 0
	for i, p := range prefs {
		if p < 1 {
			p = 1
		}
		normalized[i] = p
		sumPref += p
	}
	if sumPref <= 0 {
		return splitEven(total, len(prefs))
	}

	if sumPref <= total {
		out := append([]int(nil), normalized...)
		out[len(out)-1] += total - sumPref
		return out
	}

	return splitByWeights(total, len(normalized), normalized)
}

func (m Model) preferredChildHeightsForColumn(children []spec.Node, width int) []int {
	prefs := make([]int, len(children))
	for i, child := range children {
		prefs[i] = m.preferredNodeHeight(child, width)
	}
	return prefs
}

func (m Model) preferredHeightsForWrappedRows(groups []rowGroup, width int) []int {
	prefs := make([]int, len(groups))
	for i, group := range groups {
		panelWidth := maxInt(1, width/maxInt(1, len(group.Children)))
		rowPref := 1
		for _, child := range group.Children {
			rowPref = maxInt(rowPref, m.preferredNodeHeight(child, panelWidth))
		}
		prefs[i] = rowPref
	}
	return prefs
}

func (m Model) preferredNodeHeight(n spec.Node, width int) int {
	switch n.Type {
	case "header":
		if m.Theme.Density == "compact" {
			return 3
		}
		return preferredHeaderHeight
	case "chart":
		if m.Theme.Density == "compact" {
			return maxInt(minChartPanelHeight, preferredChartPanelHeight-2)
		}
		return preferredChartPanelHeight
	case "table":
		if m.Theme.Density == "compact" {
			return maxInt(minTablePanelHeight, preferredTablePanelHeight-2)
		}
		return preferredTablePanelHeight
	case "text":
		lines := strings.Count(n.Text, "\n") + 1
		return maxInt(1, lines)
	case "row":
		if len(n.Children) == 0 {
			return 1
		}
		wrapped := wrapRowChildren(n.Children, n.Weights, maxPanelsPerRow)
		if len(wrapped) == 1 {
			panelWidth := maxInt(1, width/maxInt(1, len(n.Children)))
			rowPref := 1
			for _, child := range n.Children {
				rowPref = maxInt(rowPref, m.preferredNodeHeight(child, panelWidth))
			}
			return rowPref
		}

		gap := layoutGap(m.Theme.Density, 80)
		total := 0
		for i, group := range wrapped {
			panelWidth := maxInt(1, width/maxInt(1, len(group.Children)))
			rowPref := 1
			for _, child := range group.Children {
				rowPref = maxInt(rowPref, m.preferredNodeHeight(child, panelWidth))
			}
			total += rowPref
			if i < len(wrapped)-1 {
				total += gap
			}
		}
		return total
	case "col":
		if len(n.Children) == 0 {
			return 1
		}
		gap := layoutGap(m.Theme.Density, 80)
		total := 0
		for i, child := range n.Children {
			total += m.preferredNodeHeight(child, width)
			if i < len(n.Children)-1 {
				total += gap
			}
		}
		return total
	default:
		return maxInt(minChartPanelHeight, preferredChartPanelHeight)
	}
}

func renderBarLegend(th brontotheme.BrontoTheme, ds spec.DatasetSpec, width, maxLines int) string {
	if len(ds.Labels) == 0 || len(ds.Values) == 0 {
		return ""
	}

	totalItems := len(ds.Labels)
	if totalItems > len(ds.Values) {
		totalItems = len(ds.Values)
	}
	visible := legendVisibleItemCount(totalItems, width, maxLines)
	if visible <= 0 {
		return ""
	}

	grandTotal := 0.0
	for _, v := range ds.Values {
		grandTotal += v
	}

	innerW := maxInt(24, width-8)
	valueW := 10
	pctW := 7
	spacing := 4 // between columns
	labelW := innerW - valueW - pctW - spacing
	if labelW < 8 {
		labelW = 8
	}

	header := fmt.Sprintf("%-*s %*s %*s", labelW, "label", valueW, "value", pctW, "pct")
	separator := strings.Repeat("─", minInt(innerW, runeWidth(header)))

	lines := make([]string, 0, visible+5)
	lines = append(lines, th.Muted.Render(header))
	lines = append(lines, th.Divider.Render(separator))
	for i := 0; i < visible; i++ {
		label := truncateCell(ds.Labels[i], labelW, colDefault)
		value := formatMetricValue(ds.Values[i])
		pct := "0.0%"
		if grandTotal > 0 {
			pct = fmt.Sprintf("%.1f%%", (ds.Values[i]/grandTotal)*100.0)
		}
		lines = append(lines, th.Text.Render(fmt.Sprintf("%-*s %*s %*s", labelW, label, valueW, value, pctW, pct)))
	}
	if totalItems > visible {
		lines = append(lines, th.Muted.Render(fmt.Sprintf("+%d more", totalItems-visible)))
	}
	lines = append(lines, th.Muted.Render(fmt.Sprintf("Total: %s", formatMetricValue(grandTotal))))
	return strings.Join(lines, "\n")
}

func buildTableColumns(t spec.TableSpec, ds spec.DatasetSpec, totalWidth int) []table.Column {
	colsSpec := ensureFlexColumn(cloneTableColumns(t.Columns))
	widths := resolveColumnWidths(colsSpec, ds, totalWidth)
	cols := make([]table.Column, 0, len(colsSpec))
	for i, c := range colsSpec {
		title := c.Title
		if title == "" {
			title = c.Key
		}

		w := 10
		if i < len(widths) {
			w = widths[i]
		}
		cols = append(cols, table.Column{Title: title, Width: w})
	}
	return cols
}

func buildTableRowsForColumns(t spec.TableSpec, ds spec.DatasetSpec, cols []table.Column) []table.Row {
	indexByName := make(map[string]int, len(ds.Columns))
	for i, c := range ds.Columns {
		indexByName[c] = i
	}

	limit := t.RowLimit
	if limit <= 0 {
		limit = 200
	}
	if limit > len(ds.Rows) {
		limit = len(ds.Rows)
	}

	rows := make([]table.Row, 0, limit)
	for i := 0; i < limit; i++ {
		src := ds.Rows[i]
		dst := make(table.Row, len(t.Columns))
		for colIdx, col := range t.Columns {
			colWidth := minAnyWidth
			if colIdx < len(cols) && cols[colIdx].Width > 0 {
				colWidth = cols[colIdx].Width
			}
			colType := detectColType(col.Key, col.Title)
			value := ""
			if idx, ok := indexByName[col.Key]; ok && idx < len(src) {
				value = src[idx]
			}
			dst[colIdx] = truncateCell(value, colWidth, colType)
		}
		rows = append(rows, dst)
	}
	return rows
}

func resolveColumnWidths(cols []spec.TableColumnSpec, ds spec.DatasetSpec, totalWidth int) []int {
	n := len(cols)
	if n == 0 {
		return nil
	}

	separators := maxInt(0, n-1)
	available := maxInt(n*minAnyWidth+separators, totalWidth-2)
	if available <= 0 {
		available = n*minAnyWidth + separators
	}

	indexByName := make(map[string]int, len(ds.Columns))
	for i, c := range ds.Columns {
		indexByName[c] = i
	}

	maxAuto := minInt(40, maxInt(minAutoWidth, (available*6)/10))

	widths := make([]int, n)
	kinds := make([]string, n)
	colTypes := make([]colType, n)
	minWidths := make([]int, n)
	flexIndexes := make([]int, 0, n)
	used := separators

	for i, c := range cols {
		ct := detectColType(c.Key, c.Title)
		colTypes[i] = ct
		isoTimestamp := ct == colTimestamp && isISOTimeColumn(c, ds, indexByName)
		minWidths[i] = minWidthForType(ct, isoTimestamp)
		natural := estimateAutoWidth(c, ds, indexByName, maxAuto, ct, isoTimestamp)
		kind, fixed := parseWidthSpec(c.Width)
		kinds[i] = kind

		switch kind {
		case "fixed":
			maxFixed := minInt(60, maxInt(minFixedWidth, available-6))
			widths[i] = clampInt(fixed, minFixedWidth, maxFixed)
			widths[i] = maxInt(widths[i], minWidths[i])
			used += widths[i]
		case "auto":
			widths[i] = maxInt(natural, minWidths[i])
			used += widths[i]
		case "flex":
			flexIndexes = append(flexIndexes, i)
		}
	}

	if len(flexIndexes) == 0 {
		remaining := available - used
		if remaining > 0 {
			widths[n-1] += remaining
		}
	} else {
		for _, idx := range flexIndexes {
			widths[idx] = maxInt(minFlexWidth, minWidths[idx])
			used += widths[idx]
		}
		remaining := available - used
		if remaining > 0 {
			per := remaining / len(flexIndexes)
			rem := remaining % len(flexIndexes)
			for i, idx := range flexIndexes {
				widths[idx] += per
				if i < rem {
					widths[idx]++
				}
			}
		}
	}

	effectiveMins := relaxMinimumsToFit(minWidths, available, separators)
	shrinkWidthsToFitPriority(widths, kinds, colTypes, effectiveMins, available, separators)

	if len(flexIndexes) == 0 {
		total := separators
		for _, w := range widths {
			total += w
		}
		if total < available {
			widths[n-1] += available - total
		}
	}

	return widths
}

func parseWidthSpec(w interface{}) (kind string, fixed int) {
	switch v := w.(type) {
	case nil:
		return "auto", 0
	case string:
		switch v {
		case "auto":
			return "auto", 0
		case "flex":
			return "flex", 0
		default:
			return "auto", 0
		}
	case float64:
		if v < 1 {
			return "fixed", 1
		}
		return "fixed", int(v)
	default:
		return "auto", 0
	}
}

func estimateAutoWidth(col spec.TableColumnSpec, ds spec.DatasetSpec, indexByName map[string]int, maxAuto int, ct colType, isoTimestamp bool) int {
	title := col.Title
	if title == "" {
		title = col.Key
	}
	w := maxInt(minAutoWidth, runeWidth(title)+2)

	idx, ok := indexByName[col.Key]
	if ok {
		limit := len(ds.Rows)
		if limit > 50 {
			limit = 50
		}
		for i := 0; i < limit; i++ {
			if idx >= len(ds.Rows[i]) {
				continue
			}
			w = maxInt(w, runeWidth(ds.Rows[i][idx])+2)
		}
	}

	switch ct {
	case colTimestamp:
		if isoTimestamp {
			return clampInt(w, 20, minInt(30, maxInt(20, maxAuto)))
		}
		return clampInt(w, minAutoWidth, maxAuto)
	case colID:
		return clampInt(w, 8, minInt(16, maxInt(8, maxAuto)))
	case colService:
		return clampInt(w, minAutoWidth, minInt(20, maxInt(10, maxAuto)))
	case colLevel:
		return clampInt(w, 7, minInt(10, maxInt(7, maxAuto)))
	case colMessage:
		return maxInt(12, clampInt(w, minAutoWidth, maxAuto))
	default:
		return clampInt(w, minAutoWidth, maxAuto)
	}
}

func relaxMinimumsToFit(mins []int, available, separators int) []int {
	effective := append([]int(nil), mins...)
	totalMin := separators
	for _, w := range effective {
		totalMin += w
	}
	for totalMin > available {
		changed := false
		for i := len(effective) - 1; i >= 0 && totalMin > available; i-- {
			if effective[i] > minAnyWidth {
				effective[i]--
				totalMin--
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return effective
}

func shrinkWidthsToFitPriority(widths []int, kinds []string, types []colType, mins []int, available, separators int) {
	total := separators
	for _, w := range widths {
		total += w
	}
	over := total - available
	if over <= 0 {
		return
	}

	shrinkGroup := func(indexes []int) {
		for over > 0 {
			changed := false
			for _, idx := range indexes {
				if idx < 0 || idx >= len(widths) {
					continue
				}
				if widths[idx] > mins[idx] {
					widths[idx]--
					over--
					changed = true
					if over <= 0 {
						return
					}
				}
			}
			if !changed {
				return
			}
		}
	}

	groupAutoNonKey := make([]int, 0, len(widths))
	groupFixed := make([]int, 0, len(widths))
	groupFlex := make([]int, 0, len(widths))
	groupAll := make([]int, 0, len(widths))
	for i := range widths {
		groupAll = append(groupAll, i)
		switch kinds[i] {
		case "auto":
			if !isKeyColumn(types[i]) {
				groupAutoNonKey = append(groupAutoNonKey, i)
			}
		case "fixed":
			groupFixed = append(groupFixed, i)
		case "flex":
			groupFlex = append(groupFlex, i)
		}
	}

	shrinkGroup(groupAutoNonKey)
	shrinkGroup(groupFixed)
	shrinkGroup(groupFlex)
	shrinkGroup(groupAll)
}

func isKeyColumn(ct colType) bool {
	return ct == colTimestamp || ct == colID
}

func minWidthForType(ct colType, isoTimestamp bool) int {
	switch ct {
	case colTimestamp:
		if isoTimestamp {
			return 20
		}
		return minAnyWidth
	case colMessage:
		return 12
	case colID:
		return 8
	case colLevel:
		return 7
	default:
		return minAnyWidth
	}
}

func detectColType(key, title string) colType {
	s := strings.ToLower(strings.TrimSpace(key + " " + title))
	switch {
	case strings.Contains(s, "ts") || strings.Contains(s, "time") || strings.Contains(s, "timestamp") || strings.Contains(s, "date"):
		return colTimestamp
	case strings.Contains(s, "level") || strings.Contains(s, "sev") || strings.Contains(s, "severity"):
		return colLevel
	case strings.Contains(s, "service") || strings.Contains(s, "component") || strings.Contains(s, "module"):
		return colService
	case strings.Contains(s, "trace") || strings.Contains(s, "span") || strings.Contains(s, "uuid") ||
		strings.Contains(s, "request") || strings.Contains(s, "req") || strings.Contains(s, "hash") ||
		strings.HasSuffix(strings.ToLower(key), "id"):
		return colID
	case strings.Contains(s, "message") || strings.Contains(s, "error") || strings.Contains(s, "summary") ||
		strings.Contains(s, "desc") || strings.Contains(s, "description"):
		return colMessage
	case strings.Contains(s, "count") || strings.Contains(s, "total") || strings.Contains(s, "avg") ||
		strings.Contains(s, "mean") || strings.Contains(s, "rate") || strings.Contains(s, "pct") || strings.Contains(s, "percent"):
		return colNumeric
	default:
		return colDefault
	}
}

func isISOTimeColumn(col spec.TableColumnSpec, ds spec.DatasetSpec, indexByName map[string]int) bool {
	idx, ok := indexByName[col.Key]
	if !ok {
		return false
	}
	checked := 0
	for i := 0; i < len(ds.Rows) && checked < 5; i++ {
		if idx >= len(ds.Rows[i]) {
			continue
		}
		v := strings.TrimSpace(ds.Rows[i][idx])
		if v == "" {
			continue
		}
		checked++
		if _, err := time.Parse(time.RFC3339, v); err == nil {
			return true
		}
	}
	return false
}

func ensureFlexColumn(cols []spec.TableColumnSpec) []spec.TableColumnSpec {
	hasFlex := false
	for _, c := range cols {
		if s, ok := c.Width.(string); ok && s == "flex" {
			hasFlex = true
			break
		}
	}
	if hasFlex || len(cols) == 0 {
		return cols
	}

	best := -1
	for i, c := range cols {
		k := strings.ToLower(c.Key + " " + c.Title)
		if strings.Contains(k, "message") || strings.Contains(k, "error") ||
			strings.Contains(k, "summary") || strings.Contains(k, "desc") {
			best = i
			break
		}
	}
	if best == -1 {
		best = len(cols) - 1
	}
	cols[best].Width = "flex"
	return cols
}

func cloneTableColumns(cols []spec.TableColumnSpec) []spec.TableColumnSpec {
	out := make([]spec.TableColumnSpec, len(cols))
	copy(out, cols)
	return out
}

func layoutGap(density string, available int) int {
	if density == "compact" || available < 24 {
		return 0
	}
	return 1
}

func panelPadding(width, height int, density string) (vertical, horizontal int) {
	if density == "compact" {
		return 0, 0
	}

	vertical = 1
	horizontal = 1

	if height < 14 {
		vertical = 0
	}
	if width < 72 {
		horizontal = 0
	}
	if width < 46 || height < 10 {
		return 0, 0
	}
	return vertical, horizontal
}

func panelTooSmallMessage(width, height, minW, minH int) string {
	return fmt.Sprintf("panel too small (%dx%d). need at least %dx%d", width, height, minW, minH)
}

func truncateCell(v string, width int, ct colType) string {
	if width <= 0 {
		return ""
	}
	if runeWidth(v) <= width {
		return v
	}

	switch ct {
	case colID:
		return middleEllipsis(v, width)
	default:
		return endEllipsis(v, width)
	}
}

func endEllipsis(v string, width int) string {
	if width <= 1 {
		return "…"
	}
	runes := []rune(v)
	if len(runes) <= width {
		return v
	}
	return string(runes[:width-1]) + "…"
}

func middleEllipsis(v string, width int) string {
	if width <= 1 {
		return "…"
	}
	if width <= 3 {
		return endEllipsis(v, width)
	}

	runes := []rune(v)
	if len(runes) <= width {
		return v
	}
	left := (width - 1) / 2
	right := (width - 1) - left
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func formatMetricValue(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.000001 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func buildTableStyles(th brontotheme.BrontoTheme) table.Styles {
	s := table.DefaultStyles()
	s.Header = th.TableHeader.Copy().
		Background(lipgloss.Color(brontotheme.BrontoPanelBg))
	s.Cell = th.TableCell.Copy().
		Background(lipgloss.Color(brontotheme.BrontoPanelBg))
	s.Selected = th.TableCell.Copy().
		Background(lipgloss.Color(brontotheme.BrontoPanelBg))
	return s
}

func runeWidth(s string) int {
	return runewidth.StringWidth(s)
}

func clampInt(v, minV, maxV int) int {
	if maxV < minV {
		maxV = minV
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func axisLabelMaxWidth(chartWidth int) int {
	if chartWidth <= 0 {
		return 8
	}
	limit := chartWidth / 3
	return clampInt(limit, 8, 28)
}

func legendItemLimit(width int) int {
	switch {
	case width >= 150:
		return 14
	case width >= 110:
		return 12
	case width >= 80:
		return 10
	default:
		return 8
	}
}

func legendLineCount(visibleItems, totalItems int) int {
	if visibleItems <= 0 {
		return 0
	}
	lines := visibleItems + 3 // header + divider + total
	if totalItems > visibleItems {
		lines++
	}
	return lines
}

func legendVisibleItemCount(totalItems, width, maxLines int) int {
	if totalItems <= 0 {
		return 0
	}
	visible := totalItems
	widthCap := legendItemLimit(width)
	if visible > widthCap {
		visible = widthCap
	}
	if maxLines > 0 {
		for visible > 0 && legendLineCount(visible, totalItems) > maxLines {
			visible--
		}
	}
	return visible
}

func setBarData(bar *barchart.Model, barStyle lipgloss.Style, dangerBarStyle lipgloss.Style, labels []string, values []float64, axisLabelLimit int) {
	data := make([]barchart.BarData, 0, len(labels))
	for i, l := range labels {
		v := 0.0
		if i < len(values) {
			v = values[i]
		}

		style := barStyle
		if isDangerLabel(l) {
			style = dangerBarStyle
		}
		axisLabel := l
		if axisLabelLimit > 0 {
			axisLabel = truncateCell(l, axisLabelLimit, colDefault)
		}
		data = append(data, barchart.BarData{
			Label: axisLabel,
			Values: []barchart.BarValue{
				{Name: l, Value: v, Style: style},
			},
		})
	}

	bar.Clear()
	bar.PushAll(data)
	bar.Draw()
}

func isDangerLabel(label string) bool {
	s := strings.ToLower(strings.TrimSpace(label))
	return strings.Contains(s, "critical") ||
		strings.Contains(s, "fatal") ||
		strings.Contains(s, "panic") ||
		strings.Contains(s, "sev1") ||
		strings.Contains(s, "p0") ||
		strings.Contains(s, "error")
}

func barGapForDensity(density string) int {
	if density == "compact" {
		return 0
	}
	return 1
}

func splitByWeights(total, count int, weights []int) []int {
	if count <= 0 {
		return nil
	}
	if total < count {
		total = count
	}

	normalized := make([]int, count)
	if len(weights) != count {
		for i := range normalized {
			normalized[i] = 1
		}
	} else {
		for i, w := range weights {
			if w <= 0 {
				normalized[i] = 1
				continue
			}
			normalized[i] = w
		}
	}

	weightSum := 0
	for _, w := range normalized {
		weightSum += w
	}
	if weightSum <= 0 {
		return splitEven(total, count)
	}

	parts := make([]int, count)
	assigned := 0
	for i, w := range normalized {
		parts[i] = total * w / weightSum
		assigned += parts[i]
	}

	rem := total - assigned
	for i := 0; i < count && rem > 0; i++ {
		parts[i]++
		rem--
	}

	for i := range parts {
		if parts[i] < 1 {
			parts[i] = 1
		}
	}
	return parts
}

func splitEven(total, count int) []int {
	if count <= 0 {
		return nil
	}
	if total < count {
		total = count
	}

	base := total / count
	rem := total % count
	parts := make([]int, count)
	for i := range parts {
		parts[i] = base
		if i < rem {
			parts[i]++
		}
	}
	return parts
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
