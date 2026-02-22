package tui

import (
	"strings"

	"github.com/ivogarais/bronto-cli/spec"
)

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
