package tui

import (
	"fmt"
	"strings"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
)

func renderChartMetadata(
	th brontotheme.BrontoTheme,
	chartSpec spec.ChartSpec,
	ds spec.DatasetSpec,
	width int,
	maxLines int,
	focused bool,
) string {
	if maxLines <= 0 {
		return ""
	}
	if ds.Kind == "categorySeries" && focused {
		return renderBarLegend(th, ds, width, maxLines)
	}
	return renderCompactSummary(th, chartSummaryTokens(chartSpec, ds), width, maxLines)
}

func chartSummaryTokens(chartSpec spec.ChartSpec, ds spec.DatasetSpec) []string {
	switch ds.Kind {
	case "categorySeries":
		totalItems := len(ds.Labels)
		if totalItems > len(ds.Values) {
			totalItems = len(ds.Values)
		}
		if totalItems <= 0 {
			return nil
		}
		total := 0.0
		for i := 0; i < totalItems; i++ {
			total += ds.Values[i]
		}

		tokens := make([]string, 0, totalItems)
		for i := 0; i < totalItems; i++ {
			label := truncateCell(ds.Labels[i], 18, colDefault)
			value := formatValueWithUnit(ds.Values[i], ds.Unit)
			if total > 0 {
				tokens = append(tokens, fmt.Sprintf("%s=%s (%.1f%%)", label, value, (ds.Values[i]/total)*100.0))
				continue
			}
			tokens = append(tokens, fmt.Sprintf("%s=%s", label, value))
		}
		return tokens

	case "valueSeries":
		if len(ds.Value) == 0 {
			return nil
		}
		minV := ds.Value[0]
		maxV := ds.Value[0]
		for _, v := range ds.Value {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		last := ds.Value[len(ds.Value)-1]
		return []string{
			"latest=" + formatValueWithUnit(last, ds.Unit),
			"min=" + formatValueWithUnit(minV, ds.Unit),
			"max=" + formatValueWithUnit(maxV, ds.Unit),
			fmt.Sprintf("n=%d", len(ds.Value)),
		}

	case "xySeries":
		tokens := make([]string, 0, len(ds.XY))
		for _, series := range ds.XY {
			if len(series.Points) == 0 {
				continue
			}
			last := series.Points[len(series.Points)-1].Y
			name := series.Name
			if name == "" {
				name = "series"
			}
			tokens = append(tokens, fmt.Sprintf("%s=%s", truncateCell(name, 18, colDefault), formatValueWithUnit(last, ds.Unit)))
		}
		return tokens

	case "timeSeries":
		tokens := make([]string, 0, len(ds.Time))
		for _, series := range ds.Time {
			if len(series.Points) == 0 {
				continue
			}
			last := series.Points[len(series.Points)-1]
			name := series.Name
			if name == "" {
				name = "series"
			}
			tokens = append(tokens, fmt.Sprintf("%s=%s", truncateCell(name, 18, colDefault), formatValueWithUnit(last.V, ds.Unit)))
		}
		return tokens

	case "ohlcSeries":
		if len(ds.Candles) == 0 {
			return nil
		}
		last := ds.Candles[len(ds.Candles)-1]
		return []string{
			"O=" + formatValueWithUnit(last.Open, ds.Unit),
			"H=" + formatValueWithUnit(last.High, ds.Unit),
			"L=" + formatValueWithUnit(last.Low, ds.Unit),
			"C=" + formatValueWithUnit(last.Close, ds.Unit),
		}

	case "heatmapCells":
		if ds.Heatmap == nil {
			return nil
		}
		values := make([]float64, 0)
		if len(ds.Heatmap.Values) > 0 {
			values = append(values, ds.Heatmap.Values...)
		} else if len(ds.Heatmap.Cells) > 0 {
			for _, c := range ds.Heatmap.Cells {
				values = append(values, c.V)
			}
		}
		if len(values) == 0 {
			return nil
		}
		minV := values[0]
		maxV := values[0]
		sum := 0.0
		for _, v := range values {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
			sum += v
		}
		avg := sum / float64(len(values))
		return []string{
			"min=" + formatValueWithUnit(minV, ds.Unit),
			"avg=" + formatValueWithUnit(avg, ds.Unit),
			"max=" + formatValueWithUnit(maxV, ds.Unit),
			fmt.Sprintf("cells=%d", len(values)),
		}
	}

	// Unknown/future dataset kinds: still show some diagnostic metadata.
	return []string{
		"family=" + chartSpec.Family,
		"dataset=" + ds.Kind,
	}
}

func renderCompactSummary(th brontotheme.BrontoTheme, tokens []string, width, maxLines int) string {
	if len(tokens) == 0 || maxLines <= 0 {
		return ""
	}
	lines, _ := packSummaryTokens(tokens, maxInt(12, width-8), maxLines)
	if len(lines) == 0 {
		return ""
	}
	styled := make([]string, len(lines))
	for i := range lines {
		styled[i] = th.Text.Render(lines[i])
	}
	return strings.Join(styled, "\n")
}

func packSummaryTokens(tokens []string, width, maxLines int) ([]string, int) {
	if len(tokens) == 0 || maxLines <= 0 {
		return nil, 0
	}
	if width < 8 {
		width = 8
	}

	sanitized := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		sanitized = append(sanitized, truncateCell(tok, width, colDefault))
	}
	if len(sanitized) == 0 {
		return nil, 0
	}

	lines := make([]string, 0, maxLines)
	i := 0
	for i < len(sanitized) && len(lines) < maxLines {
		line := sanitized[i]
		i++
		for i < len(sanitized) {
			candidate := line + " | " + sanitized[i]
			if runeWidth(candidate) > width {
				break
			}
			line = candidate
			i++
		}
		lines = append(lines, line)
	}

	hidden := len(sanitized) - i
	if hidden > 0 && len(lines) > 0 {
		suffix := fmt.Sprintf(" +%d", hidden)
		last := len(lines) - 1
		candidate := lines[last] + suffix
		if runeWidth(candidate) <= width {
			lines[last] = candidate
		} else {
			keep := maxInt(4, width-runeWidth(suffix))
			lines[last] = truncateCell(lines[last], keep, colDefault) + suffix
		}
	}
	return lines, hidden
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
