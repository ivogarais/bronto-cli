package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
)

func (m *Model) resolveComponents() {
	if m.Spec == nil {
		return
	}

	m.Charts = map[string]barchart.Model{}
	m.Lines = map[string]linechart.Model{}
	for chartID, chartSpec := range m.Spec.Charts {
		ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]
		if !ok {
			continue
		}

		switch chartSpec.Family {
		case "bar":
			opts := []barchart.Option{
				barchart.WithStyles(m.Theme.ChartAxis, m.Theme.ChartLabel),
				barchart.WithBarGap(barGapForDensity(m.Theme.Density)),
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

		case "line":
			lc, ok := buildLineChartModel(m.Theme, chartSpec, ds, 50, 12)
			if ok {
				m.Lines[chartID] = lc
			}
		}
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
		chartSpec, hasSpec := m.Spec.Charts[n.ChartRef]
		if !hasSpec {
			return
		}
		ds, hasDS := m.Spec.Datasets[chartSpec.DatasetRef]
		if !hasDS {
			return
		}

		padV, padH := panelPadding(width, height, m.Theme.Density)
		chartW := maxInt(1, width-2-(2*padH))
		baseH := maxInt(1, height-2-(2*padV)-2)
		chartH := baseH

		metaMaxLines := m.chartMetaMaxLines(n, height)
		metaLines := m.chartMetadataLineEstimate(chartSpec, ds, width, metaMaxLines, m.isFocusedNode(n))
		if metaLines > 0 && baseH > 4 {
			reserve := minInt(metaLines+1, maxInt(2, baseH-3))
			chartH = maxInt(3, baseH-reserve)
		}

		switch chartSpec.Family {
		case "bar":
			c, ok := m.Charts[n.ChartRef]
			if !ok {
				return
			}
			if chartSpec.Render.ShowAxis != nil {
				c.SetShowAxis(*chartSpec.Render.ShowAxis)
			}
			labelMax := axisLabelMaxWidth(chartW)
			axisLabels := buildAxisLabels(ds.Labels, labelMax)
			setBarData(&c, m.Theme.ChartBar, m.Theme.ChartDanger, ds.Labels, ds.Values, labelMax)
			c.Resize(chartW, chartH)
			c.Draw()
			drawHorizontalBarValueLabels(&c, m.Theme, axisLabels, ds.Values, ds.Unit)
			m.Charts[n.ChartRef] = c

		case "line":
			lc, ok := buildLineChartModel(m.Theme, chartSpec, ds, chartW, chartH)
			if !ok {
				return
			}
			m.Lines[n.ChartRef] = lc

		case "pie":
			// Pie rendering is stateless and generated directly in View().
			return
		}

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
		var meta string
		chartW := 0
		chartH := 0
		if chartFound {
			if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
				chartW, chartH = m.chartRenderSize(n, chartSpec, ds, width, height)
			}
		}
		if chartFound {
			switch chartSpec.Family {
			case "bar":
				if c, ok := m.Charts[n.ChartRef]; ok {
					content = c.View()
				} else {
					content = "(chart data unavailable)"
				}
			case "line":
				if lc, ok := m.Lines[n.ChartRef]; ok {
					content = lc.View()
				} else {
					content = "(chart data unavailable)"
				}
			case "pie":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderPieChart(m.Theme, ds, width, height)
				} else {
					content = "(chart data unavailable)"
				}
			case "scatter":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderScatterChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "waveline":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderWavelineChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "streamline":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderStreamlineChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "sparkline":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderSparklineChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "heatmap":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderHeatmapChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "timeseries":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderTimeseriesChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			case "ohlc":
				if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
					content = renderOHLCChart(m.Theme, chartSpec, ds, chartW, chartH)
				} else {
					content = "(chart data unavailable)"
				}
			default:
				content = fmt.Sprintf("unsupported chart family %q", chartSpec.Family)
			}
		}
		if chartFound {
			if ds, ok := m.Spec.Datasets[chartSpec.DatasetRef]; ok {
				metaMaxLines := m.chartMetaMaxLines(n, height)
				meta = renderChartMetadata(
					m.Theme,
					chartSpec,
					ds,
					width,
					metaMaxLines,
					m.isFocusedNode(n),
				)
			}
		}
		if meta != "" {
			content += "\n" + meta
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
