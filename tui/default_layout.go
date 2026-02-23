package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ivogarais/bronto-cli/spec"
)

const (
	tabCharts = "charts"
	tabLogs   = "logs"
)

func (m *Model) applyDefaultLayoutStructure() {
	if m.Spec == nil {
		return
	}
	chartTitles, tableTitles := extractPanelTitles(m.Spec.Layout)

	chartIDs := sortedChartIDs(m.Spec.Charts)
	tableIDs := sortedTableIDs(m.Spec.Tables)

	m.HasChartsTab = len(chartIDs) > 0
	m.HasLogsTab = len(tableIDs) > 0

	if m.HasChartsTab {
		panels := make([]spec.Node, 0, len(chartIDs))
		for _, chartID := range chartIDs {
			chartTitle := strings.TrimSpace(chartTitles[chartID])
			if chartTitle == "" {
				chartTitle = strings.TrimSpace(m.Spec.Charts[chartID].Title)
			}
			if chartTitle == "" {
				chartTitle = chartID
			}
			panels = append(panels, spec.Node{
				Type:     "chart",
				ID:       "chart_" + chartID,
				Title:    chartTitle,
				ChartRef: chartID,
			})
		}
		m.chartsLayout = buildDefaultGrid("charts", panels, chartPanelsPerRow)
	}

	if m.HasLogsTab {
		panels := make([]spec.Node, 0, len(tableIDs))
		for _, tableID := range tableIDs {
			tableTitle := strings.TrimSpace(tableTitles[tableID])
			if tableTitle == "" {
				tableTitle = strings.TrimSpace(m.Spec.Tables[tableID].Title)
			}
			if tableTitle == "" {
				tableTitle = tableID
			}
			panels = append(panels, spec.Node{
				Type:     "table",
				ID:       "table_" + tableID,
				Title:    tableTitle,
				TableRef: tableID,
			})
		}
		m.logsLayout = buildDefaultGrid("logs", panels, logPanelsPerRow)
	}

	switch {
	case m.HasChartsTab:
		m.ActiveTab = tabCharts
	case m.HasLogsTab:
		m.ActiveTab = tabLogs
	default:
		m.ActiveTab = ""
	}

	m.rebuildRootLayout()
}

func (m Model) hasTabs() bool {
	return m.HasChartsTab && m.HasLogsTab
}

func (m Model) tabLine() string {
	if !m.hasTabs() {
		return ""
	}
	if m.ActiveTab == tabLogs {
		return " Charts  [Logs]  (use c/l)"
	}
	return "[Charts]  Logs  (use c/l)"
}

func (m *Model) rebuildRootLayout() {
	if m.Spec == nil {
		return
	}

	children := []spec.Node{
		{
			Type:     "header",
			ID:       "hdr",
			TitleRef: "$title",
			SubTitle: m.statusSubtitle(),
		},
	}

	if m.hasTabs() {
		children = append(children, spec.Node{
			Type:    "text",
			ID:      "tabs",
			Text:    m.tabLine(),
			Variant: "primary",
		})
	}

	switch {
	case m.ActiveTab == tabLogs && m.HasLogsTab:
		children = append(children, m.logsLayout)
	case m.HasChartsTab:
		children = append(children, m.chartsLayout)
	case m.HasLogsTab:
		children = append(children, m.logsLayout)
	default:
		children = append(children, spec.Node{
			Type:    "text",
			ID:      "empty",
			Text:    "No charts or logs available.",
			Variant: "muted",
		})
	}

	m.Spec.Layout = spec.Node{
		Type:     "col",
		ID:       "root",
		Gap:      1,
		Children: children,
	}
}

func (m *Model) switchTab(target string) bool {
	if !m.hasTabs() || target == m.ActiveTab {
		return false
	}
	if target == tabCharts && !m.HasChartsTab {
		return false
	}
	if target == tabLogs && !m.HasLogsTab {
		return false
	}

	m.ActiveTab = target
	m.FocusedPanel = 0
	m.FocusScrollY = 0
	m.ScrollY = 0
	m.TableFilterMode = false

	m.rebuildRootLayout()
	m.indexFocusablePanels()
	m.resizeForLayout(m.Width, m.Height)
	return true
}

func buildDefaultGrid(prefix string, panels []spec.Node, perRow int) spec.Node {
	if perRow <= 0 {
		perRow = 1
	}
	if len(panels) == 0 {
		return spec.Node{
			Type:    "text",
			ID:      prefix + "_empty",
			Text:    "No panels available.",
			Variant: "muted",
		}
	}

	rows := make([]spec.Node, 0, (len(panels)+perRow-1)/perRow)
	for i := 0; i < len(panels); i += perRow {
		end := minInt(len(panels), i+perRow)
		chunk := append([]spec.Node(nil), panels[i:end]...)
		rows = append(rows, spec.Node{
			Type:     "row",
			ID:       fmt.Sprintf("%s_row_%d", prefix, len(rows)+1),
			Gap:      1,
			Weights:  equalWeights(len(chunk)),
			Children: chunk,
		})
	}

	if len(rows) == 1 {
		return rows[0]
	}
	return spec.Node{
		Type:     "col",
		ID:       prefix + "_grid",
		Gap:      1,
		Children: rows,
	}
}

func equalWeights(n int) []int {
	if n <= 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = 1
	}
	return out
}

func sortedChartIDs(charts map[string]spec.ChartSpec) []string {
	out := make([]string, 0, len(charts))
	for id := range charts {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sortedTableIDs(tables map[string]spec.TableSpec) []string {
	out := make([]string, 0, len(tables))
	for id := range tables {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func extractPanelTitles(root spec.Node) (map[string]string, map[string]string) {
	chartTitles := map[string]string{}
	tableTitles := map[string]string{}

	var walk func(node spec.Node)
	walk = func(node spec.Node) {
		switch node.Type {
		case "chart":
			if node.ChartRef != "" && strings.TrimSpace(node.Title) != "" {
				chartTitles[node.ChartRef] = strings.TrimSpace(node.Title)
			}
		case "table":
			if node.TableRef != "" && strings.TrimSpace(node.Title) != "" {
				tableTitles[node.TableRef] = strings.TrimSpace(node.Title)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return chartTitles, tableTitles
}
