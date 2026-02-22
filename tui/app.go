package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"

	"github.com/ivogarais/bronto-cli/spec"
)

type Model struct {
	Spec     *spec.AppSpec
	SpecPath string

	Charts map[string]barchart.Model
	Tables map[string]table.Model

	Width    int
	Height   int
	Status   string
	LoadedAt time.Time
}

func NewModel(s *spec.AppSpec, specPath string) Model {
	m := Model{
		Spec:     s,
		SpecPath: specPath,
		Charts:   map[string]barchart.Model{},
		Tables:   map[string]table.Model{},
		Status:   "Snapshot loaded",
		LoadedAt: time.Now(),
	}

	m.resolveComponents()
	m.resizeForLayout(120, 36)
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
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

	body := m.renderNode(m.Spec.Layout, width, height)
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

		bar := barchart.New(50, 12)
		setBarData(&bar, ds.Labels, ds.Values)
		m.Charts[chartID] = bar
	}

	m.Tables = map[string]table.Model{}
	for tableID, tableSpec := range m.Spec.Tables {
		ds, ok := m.Spec.Datasets[tableSpec.DatasetRef]
		if !ok {
			continue
		}

		cols := buildTableColumns(tableSpec, ds, 80)
		rows := buildTableRows(tableSpec, ds)

		t := table.New(
			table.WithColumns(cols),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithWidth(80),
			table.WithHeight(12),
		)
		m.Tables[tableID] = t
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
	m.resizeNode(m.Spec.Layout, width, height)
}

func (m *Model) resizeNode(n spec.Node, width, height int) {
	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return
		}
		gap := maxInt(0, n.Gap)
		childHeights := splitEven(maxInt(1, height-gap*(len(n.Children)-1)), len(n.Children))
		for i, ch := range n.Children {
			m.resizeNode(ch, width, childHeights[i])
		}

	case "row":
		if len(n.Children) == 0 {
			return
		}
		gap := maxInt(0, n.Gap)
		childWidths := splitByWeights(maxInt(1, width-gap*(len(n.Children)-1)), len(n.Children), n.Weights)
		for i, ch := range n.Children {
			m.resizeNode(ch, childWidths[i], height)
		}

	case "chart":
		c, ok := m.Charts[n.ChartRef]
		if !ok {
			return
		}
		chartW := maxInt(20, width-6)
		chartH := maxInt(8, height-4)
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

		tableW := maxInt(30, width-6)
		tableH := maxInt(5, height-4)
		t.SetColumns(buildTableColumns(tableSpec, ds, tableW))
		t.SetWidth(tableW)
		t.SetHeight(tableH)
		m.Tables[n.TableRef] = t
	}
}

func (m Model) renderNode(n spec.Node, width, height int) string {
	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return ""
		}

		gap := maxInt(0, n.Gap)
		childHeights := splitEven(maxInt(1, height-gap*(len(n.Children)-1)), len(n.Children))
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

		gap := maxInt(0, n.Gap)
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
		sub := n.SubTitle
		if sub == "" {
			sub = "snapshot view"
		}

		header := fmt.Sprintf("%s\n%s\nSpec: %s   Theme: %s/%s   Loaded: %s\n(press q to quit)",
			lipgloss.NewStyle().Bold(true).Render(title),
			sub,
			m.SpecPath,
			m.Spec.Theme.Brand,
			m.Spec.Theme.Density,
			m.LoadedAt.Format("15:04:05"),
		)

		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(width)
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

		content := "(missing chart)"
		if c, ok := m.Charts[n.ChartRef]; ok {
			content = c.View()
		} else if chartFound {
			if chartSpec.Family != "bar" {
				content = fmt.Sprintf("unsupported chart family %q (renderer currently implements bar only)", chartSpec.Family)
			} else {
				content = "(chart data unavailable)"
			}
		}
		return renderPanel(title, content, width)

	case "table":
		title := n.Title
		if title == "" {
			title = n.TableRef
		}
		content := "(missing table)"
		if t, ok := m.Tables[n.TableRef]; ok {
			content = t.View()
		}
		return renderPanel(title, content, width)

	case "text":
		st := lipgloss.NewStyle().Width(width)
		switch n.Variant {
		case "muted":
			st = st.Faint(true)
		case "danger":
			st = st.Bold(true)
		case "primary":
			st = st.Bold(true)
		}
		return st.Render(n.Text)
	}

	return renderPanel("Unsupported", fmt.Sprintf("node type %q is not supported", n.Type), width)
}

func renderPanel(title, body string, width int) string {
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width)
	return panel.Render(title + "\n\n" + body)
}

func buildTableColumns(t spec.TableSpec, ds spec.DatasetSpec, totalWidth int) []table.Column {
	widths := resolveColumnWidths(t, ds, totalWidth)
	cols := make([]table.Column, 0, len(t.Columns))
	for i, c := range t.Columns {
		w := 10
		if i < len(widths) {
			w = widths[i]
		}
		cols = append(cols, table.Column{Title: c.Title, Width: w})
	}
	return cols
}

func buildTableRows(t spec.TableSpec, ds spec.DatasetSpec) []table.Row {
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
			if idx, ok := indexByName[col.Key]; ok && idx < len(src) {
				dst[colIdx] = src[idx]
			}
		}
		rows = append(rows, dst)
	}
	return rows
}

func resolveColumnWidths(t spec.TableSpec, ds spec.DatasetSpec, totalWidth int) []int {
	n := len(t.Columns)
	if n == 0 {
		return nil
	}

	if totalWidth < n*6 {
		totalWidth = n * 6
	}

	widths := make([]int, n)
	flexIndexes := make([]int, 0, n)
	used := 0

	for i, c := range t.Columns {
		switch kind, fixed := parseWidthSpec(c.Width); kind {
		case "fixed":
			widths[i] = maxInt(1, fixed)
			used += widths[i]
		case "auto":
			widths[i] = estimateAutoWidth(c, ds)
			used += widths[i]
		case "flex":
			flexIndexes = append(flexIndexes, i)
		}
	}

	if len(flexIndexes) > 0 {
		remaining := totalWidth - used
		minFlexTotal := len(flexIndexes) * 8
		if remaining < minFlexTotal {
			remaining = minFlexTotal
		}
		per := remaining / len(flexIndexes)
		rem := remaining % len(flexIndexes)
		for i, idx := range flexIndexes {
			w := per
			if i < rem {
				w++
			}
			widths[idx] = maxInt(8, w)
		}
	} else {
		sum := 0
		for _, w := range widths {
			sum += w
		}
		if sum < totalWidth {
			widths[len(widths)-1] += totalWidth - sum
		}
	}

	shrinkWidthsToFit(widths, totalWidth, 6)
	return widths
}

func parseWidthSpec(w interface{}) (kind string, fixed int) {
	switch v := w.(type) {
	case string:
		if v == "flex" {
			return "flex", 0
		}
		return "auto", 0
	case float64:
		if v < 1 {
			return "fixed", 1
		}
		return "fixed", int(v)
	case nil:
		return "auto", 0
	default:
		return "auto", 0
	}
}

func estimateAutoWidth(col spec.TableColumnSpec, ds spec.DatasetSpec) int {
	w := maxInt(8, len(col.Title)+2)

	idx := -1
	for i, c := range ds.Columns {
		if c == col.Key {
			idx = i
			break
		}
	}
	if idx < 0 {
		return w
	}

	limit := len(ds.Rows)
	if limit > 50 {
		limit = 50
	}
	for i := 0; i < limit; i++ {
		if idx >= len(ds.Rows[i]) {
			continue
		}
		w = maxInt(w, len(ds.Rows[i][idx])+2)
	}
	if w > 40 {
		w = 40
	}
	return w
}

func shrinkWidthsToFit(widths []int, total, minWidth int) {
	sum := 0
	for _, w := range widths {
		sum += w
	}
	over := sum - total
	if over <= 0 {
		return
	}

	for over > 0 {
		shrunk := false
		for i := len(widths) - 1; i >= 0 && over > 0; i-- {
			if widths[i] > minWidth {
				widths[i]--
				over--
				shrunk = true
			}
		}
		if !shrunk {
			return
		}
	}
}

func setBarData(bar *barchart.Model, labels []string, values []float64) {
	data := make([]barchart.BarData, 0, len(labels))
	for i, l := range labels {
		v := 0.0
		if i < len(values) {
			v = values[i]
		}
		data = append(data, barchart.BarData{
			Label: l,
			Values: []barchart.BarValue{
				{Name: l, Value: v},
			},
		})
	}

	bar.Clear()
	bar.PushAll(data)
	bar.Draw()
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
