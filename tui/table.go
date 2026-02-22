package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
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

func formatValueWithUnit(v float64, unit string) string {
	base := formatMetricValue(v)
	u := strings.TrimSpace(unit)
	if u == "" {
		return base
	}
	if u == "%" {
		return base + u
	}
	return base + " " + u
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
