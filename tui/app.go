package tui

import (
	"fmt"
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

	Width    int
	Height   int
	Status   string
	LoadedAt time.Time
}

func NewModel(s *spec.AppSpec, specPath string) Model {
	density := "comfortable"
	if s != nil {
		density = s.Theme.Density
	}

	m := Model{
		Spec:     s,
		SpecPath: specPath,
		Theme:    brontotheme.NewBrontoTheme(density),
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
		setBarData(&bar, m.Theme.ChartBar, m.Theme.ChartDanger, ds.Labels, ds.Values)
		m.Charts[chartID] = bar
	}

	m.Tables = map[string]table.Model{}
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
		gap := layoutGap(m.Theme.Density, height)
		childHeights := splitEven(maxInt(1, height-gap*(len(n.Children)-1)), len(n.Children))
		for i, ch := range n.Children {
			m.resizeNode(ch, width, childHeights[i])
		}

	case "row":
		if len(n.Children) == 0 {
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
		chartH := maxInt(1, height-2-(2*padV)-2)
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
		t.SetRows(buildTableRowsForColumns(tableSpec, ds, cols))
		t.SetWidth(tableW)
		t.SetHeight(tableH)
		t.SetStyles(buildTableStyles(m.Theme))
		m.Tables[n.TableRef] = t
	}
}

func (m Model) renderNode(n spec.Node, width, height int) string {
	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return ""
		}

		gap := layoutGap(m.Theme.Density, height)
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
		sub := n.SubTitle
		if sub == "" {
			sub = "snapshot view"
		}
		header := fmt.Sprintf("%s\n%s\n%s\n%s",
			m.Theme.PanelAccent.Render("▌ ")+m.Theme.AppTitle.Render(title),
			m.Theme.Muted.Render(sub),
			m.Theme.Muted.Render(fmt.Sprintf("Spec: %s   Theme: %s/%s   Loaded: %s",
				m.SpecPath,
				m.Spec.Theme.Brand,
				m.Spec.Theme.Density,
				m.LoadedAt.Format("15:04:05"),
			)),
			m.Theme.Muted.Render("(press q to quit)"),
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
		if width < minChartPanelWidth || height < minChartPanelHeight {
			content = panelTooSmallMessage(width, height, minChartPanelWidth, minChartPanelHeight)
		}
		return renderPanel(m.Theme, title, content, width, height)

	case "table":
		title := n.Title
		if title == "" {
			title = n.TableRef
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
	minChartPanelWidth  = 24
	minChartPanelHeight = 8
	minTablePanelWidth  = 30
	minTablePanelHeight = 6

	minAnyWidth   = 6
	minAutoWidth  = 8
	minFixedWidth = 6
	minFlexWidth  = 12
)

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
	if width < 40 || height < 8 {
		return 0, 1
	}
	return 1, 1
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

func setBarData(bar *barchart.Model, barStyle lipgloss.Style, dangerBarStyle lipgloss.Style, labels []string, values []float64) {
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
		data = append(data, barchart.BarData{
			Label: l,
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
