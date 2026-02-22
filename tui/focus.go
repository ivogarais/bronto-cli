package tui

import (
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/ivogarais/bronto-cli/spec"
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

func (m Model) isFocusedNode(n spec.Node) bool {
	number := m.panelNumberForNode(n)
	return number > 0 && number == m.FocusedPanel
}

func (m Model) chartMetaMaxLines(n spec.Node, panelHeight int) int {
	innerH := maxInt(3, panelHeight-4)
	if m.isFocusedNode(n) {
		return maxInt(4, innerH/2)
	}
	return 2
}

func (m Model) chartMetadataLineEstimate(chartSpec spec.ChartSpec, ds spec.DatasetSpec, panelWidth, maxLines int, focused bool) int {
	if maxLines <= 0 {
		return 0
	}
	if ds.Kind == "categorySeries" && focused {
		totalItems := len(ds.Labels)
		if totalItems > len(ds.Values) {
			totalItems = len(ds.Values)
		}
		visible := legendVisibleItemCount(totalItems, panelWidth, maxLines)
		return legendLineCount(visible, totalItems)
	}

	tokens := chartSummaryTokens(chartSpec, ds)
	lines, _ := packSummaryTokens(tokens, maxInt(12, panelWidth-8), maxLines)
	return len(lines)
}

func (m Model) chartRenderSize(n spec.Node, chartSpec spec.ChartSpec, ds spec.DatasetSpec, panelWidth, panelHeight int) (int, int) {
	padV, padH := panelPadding(panelWidth, panelHeight, m.Theme.Density)
	chartW := maxInt(1, panelWidth-2-(2*padH))
	baseH := maxInt(1, panelHeight-2-(2*padV)-2)
	chartH := baseH

	metaMaxLines := m.chartMetaMaxLines(n, panelHeight)
	metaLines := m.chartMetadataLineEstimate(chartSpec, ds, panelWidth, metaMaxLines, m.isFocusedNode(n))
	if metaLines > 0 && baseH > 4 {
		reserve := minInt(metaLines+1, maxInt(2, baseH-3))
		chartH = maxInt(3, baseH-reserve)
	}
	return chartW, chartH
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
