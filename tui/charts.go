package tui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/heatmap"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/streamlinechart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/wavelinechart"
	"github.com/NimbleMarkets/ntcharts/v2/sparkline"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
)

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

func buildAxisLabels(labels []string, axisLabelLimit int) []string {
	out := make([]string, len(labels))
	for i, l := range labels {
		if axisLabelLimit > 0 {
			out[i] = truncateCell(l, axisLabelLimit, colDefault)
		} else {
			out[i] = l
		}
	}
	return out
}

func setBarData(bar *barchart.Model, barStyle lipgloss.Style, dangerBarStyle lipgloss.Style, labels []string, values []float64, axisLabelLimit int) {
	axisLabels := buildAxisLabels(labels, axisLabelLimit)
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
		if i < len(axisLabels) {
			axisLabel = axisLabels[i]
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

type lineSeriesSelection struct {
	Name    string
	Variant string
	Points  []spec.XYPoint
}

func buildLineChartModel(
	th brontotheme.BrontoTheme,
	chartSpec spec.ChartSpec,
	ds spec.DatasetSpec,
	width int,
	height int,
) (linechart.Model, bool) {
	if ds.Kind != "xySeries" {
		return linechart.Model{}, false
	}
	if width < 8 || height < 6 {
		return linechart.Model{}, false
	}

	series := selectLineSeries(chartSpec, ds)
	if len(series) == 0 {
		return linechart.Model{}, false
	}

	minX, maxX, minY, maxY, ok := lineSeriesBounds(series)
	if !ok {
		return linechart.Model{}, false
	}
	if minX == maxX {
		minX -= 1
		maxX += 1
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	showAxis := chartSpec.Render.ShowAxis == nil || *chartSpec.Render.ShowAxis
	xStep := 2
	yStep := 2
	if !showAxis {
		xStep = 0
		yStep = 0
	}

	opts := []linechart.Option{
		linechart.WithXYSteps(xStep, yStep),
		linechart.WithStyles(th.ChartAxis, th.ChartLabel, th.ChartBar),
		linechart.WithYLabelFormatter(yAxisLabelFormatter(ds)),
	}
	if fmter := xyTimeXLabelFormatter(series); fmter != nil {
		opts = append(opts, linechart.WithXLabelFormatter(fmter))
	}

	lc := linechart.New(
		width,
		height,
		minX,
		maxX,
		minY,
		maxY,
		opts...,
	)

	lc.Clear()
	if showAxis {
		lc.DrawXYAxisAndLabel()
	}

	lineStyle := runes.ArcLineStyle
	if chartSpec.Line != nil && chartSpec.Line.Style.Interpolation == "step" {
		lineStyle = runes.ThinLineStyle
	}

	for i, seriesData := range series {
		style := lineSeriesStyle(th, seriesData.Variant, i)
		if len(seriesData.Points) == 1 {
			p := canvas.Float64Point{X: seriesData.Points[0].X, Y: seriesData.Points[0].Y}
			lc.DrawRuneWithStyle(p, '•', style)
			continue
		}

		for j := 1; j < len(seriesData.Points); j++ {
			p1 := canvas.Float64Point{X: seriesData.Points[j-1].X, Y: seriesData.Points[j-1].Y}
			p2 := canvas.Float64Point{X: seriesData.Points[j].X, Y: seriesData.Points[j].Y}

			if chartSpec.Render.Mode == "braille" {
				lc.DrawBrailleLineWithStyle(p1, p2, style)
				continue
			}
			lc.DrawLineWithStyle(p1, p2, lineStyle, style)
		}

		if chartSpec.Line != nil && chartSpec.Line.Style.Markers {
			for _, p := range seriesData.Points {
				lc.DrawRuneWithStyle(canvas.Float64Point{X: p.X, Y: p.Y}, '•', style)
			}
		}
	}

	return lc, true
}

func selectLineSeries(chartSpec spec.ChartSpec, ds spec.DatasetSpec) []lineSeriesSelection {
	if chartSpec.Line == nil {
		return selectXYSeries(ds, nil)
	}
	return selectXYSeries(ds, chartSpec.Line.Series)
}

func lineSeriesBounds(series []lineSeriesSelection) (float64, float64, float64, float64, bool) {
	if len(series) == 0 {
		return 0, 0, 0, 0, false
	}

	initialized := false
	minX := 0.0
	maxX := 0.0
	minY := 0.0
	maxY := 0.0

	for _, s := range series {
		for _, p := range s.Points {
			if !initialized {
				minX, maxX = p.X, p.X
				minY, maxY = p.Y, p.Y
				initialized = true
				continue
			}
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	return minX, maxX, minY, maxY, initialized
}

func lineSeriesStyle(th brontotheme.BrontoTheme, variant string, idx int) lipgloss.Style {
	switch variant {
	case "danger":
		return th.ChartDanger
	case "muted":
		return th.Muted
	case "primary":
		return th.ChartBar
	}

	palette := []lipgloss.Style{
		th.ChartBar,
		th.ChartDanger.Copy().Bold(false),
		th.Text,
		th.Primary.Copy().Bold(false),
	}
	return palette[idx%len(palette)]
}

type timeSeriesSelection struct {
	Name    string
	Variant string
	Points  []spec.TimePoint
}

func renderScatterChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "xySeries" {
		return "(unsupported dataset for scatter chart)"
	}
	if width < 8 || height < 6 {
		return "(scatter chart too small)"
	}

	series := selectXYSeries(ds, nil)
	if len(series) == 0 {
		return "(no scatter data)"
	}

	minX, maxX, minY, maxY, ok := lineSeriesBounds(series)
	if !ok {
		return "(no scatter points)"
	}
	if minX == maxX {
		minX -= 1
		maxX += 1
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	xStep := 2
	yStep := 2
	if !chartShowAxis(chartSpec) {
		xStep = 0
		yStep = 0
	}

	lc := linechart.New(
		width,
		height,
		minX,
		maxX,
		minY,
		maxY,
		linechart.WithXYSteps(xStep, yStep),
		linechart.WithStyles(th.ChartAxis, th.ChartLabel, th.ChartBar),
		linechart.WithYLabelFormatter(yAxisLabelFormatter(ds)),
	)
	if fmter := xyTimeXLabelFormatter(series); fmter != nil {
		lc.XLabelFormatter = fmter
	}
	lc.Clear()
	if chartShowAxis(chartSpec) {
		lc.DrawXYAxisAndLabel()
	}

	pointRune := '•'
	if chartSpec.Scatter != nil && chartSpec.Scatter.PointRune != "" {
		r := []rune(chartSpec.Scatter.PointRune)
		if len(r) > 0 {
			pointRune = r[0]
		}
	}

	for i, s := range series {
		style := lineSeriesStyle(th, s.Variant, i)
		for _, p := range s.Points {
			fp := canvas.Float64Point{X: p.X, Y: p.Y}
			if chartSpec.Render.Mode == "braille" {
				lc.DrawBrailleLineWithStyle(fp, fp, style)
			} else {
				lc.DrawRuneWithStyle(fp, pointRune, style)
			}
		}
	}
	return lc.View()
}

func renderWavelineChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "xySeries" {
		return "(unsupported dataset for waveline chart)"
	}
	if width < 8 || height < 6 {
		return "(waveline chart too small)"
	}

	var refs []spec.SeriesRef
	if chartSpec.Waveline != nil {
		refs = chartSpec.Waveline.Series
	}
	series := selectXYSeries(ds, refs)
	if len(series) == 0 {
		return "(no waveline data)"
	}

	minX, maxX, minY, maxY, ok := lineSeriesBounds(series)
	if !ok {
		return "(no waveline points)"
	}
	if minX == maxX {
		minX -= 1
		maxX += 1
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	xStep := 2
	yStep := 2
	if !chartShowAxis(chartSpec) {
		xStep = 0
		yStep = 0
	}

	wlc := wavelinechart.New(
		width,
		height,
		wavelinechart.WithXYRange(minX, maxX, minY, maxY),
		wavelinechart.WithXYSteps(xStep, yStep),
		wavelinechart.WithAxesStyles(th.ChartAxis, th.ChartLabel),
		wavelinechart.WithStyles(runes.ArcLineStyle, th.ChartBar),
	)
	wlc.YLabelFormatter = yAxisLabelFormatter(ds)
	if fmter := xyTimeXLabelFormatter(series); fmter != nil {
		wlc.XLabelFormatter = fmter
	}

	names := make([]string, 0, len(series))
	for i, s := range series {
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("series_%d", i+1)
		}
		names = append(names, name)
		wlc.SetDataSetStyles(name, runes.ArcLineStyle, lineSeriesStyle(th, s.Variant, i))
		for _, p := range s.Points {
			wlc.PlotDataSet(name, canvas.Float64Point{X: p.X, Y: p.Y})
		}
	}
	wlc.DrawDataSets(names)
	return wlc.View()
}

func renderStreamlineChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "valueSeries" {
		return "(unsupported dataset for streamline chart)"
	}
	if width < 8 || height < 6 {
		return "(streamline chart too small)"
	}

	values := windowedValues(ds.Value, streamWindow(chartSpec))
	if len(values) == 0 {
		return "(no streamline data)"
	}

	minY, maxY := valuesMinMax(values)
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	xStep := 0
	yStep := 2
	if !chartShowAxis(chartSpec) {
		yStep = 0
	}

	slc := streamlinechart.New(
		width,
		height,
		streamlinechart.WithYRange(minY, maxY),
		streamlinechart.WithXYSteps(xStep, yStep),
		streamlinechart.WithAxesStyles(th.ChartAxis, th.ChartLabel),
		streamlinechart.WithStyles(runes.ArcLineStyle, th.ChartBar),
	)
	slc.YLabelFormatter = yAxisLabelFormatter(ds)
	for _, v := range values {
		slc.Push(v)
	}
	slc.Draw()
	return slc.View()
}

func renderSparklineChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "valueSeries" {
		return "(unsupported dataset for sparkline chart)"
	}
	if width < 8 || height < 4 {
		return "(sparkline chart too small)"
	}

	values := windowedValues(ds.Value, sparkWindow(chartSpec))
	if len(values) == 0 {
		return "(no sparkline data)"
	}

	sl := sparkline.New(
		width,
		height,
		sparkline.WithStyle(th.ChartBar),
	)
	sl.PushAll(values)
	if chartSpec.Render.Mode == "braille" {
		sl.DrawBraille()
	} else {
		sl.Draw()
	}
	return sl.View()
}

func renderHeatmapChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "heatmapCells" || ds.Heatmap == nil {
		return "(unsupported dataset for heatmap chart)"
	}
	if width < 8 || height < 6 {
		return "(heatmap chart too small)"
	}

	points, minX, maxX, minY, maxY, minV, maxV := extractHeatmapPoints(ds)
	if len(points) == 0 {
		return "(no heatmap data)"
	}
	if chartSpec.Heatmap != nil {
		if chartSpec.Heatmap.Min != nil {
			minV = *chartSpec.Heatmap.Min
		}
		if chartSpec.Heatmap.Max != nil {
			maxV = *chartSpec.Heatmap.Max
		}
	}
	if minX == maxX {
		maxX = minX + 1
	}
	if minY == maxY {
		maxY = minY + 1
	}
	if minV == maxV {
		maxV = minV + 1
	}

	// Official ntcharts flow: New -> SetXYRange -> Push -> Draw -> View.
	hm := heatmap.New(
		width,
		height,
		heatmap.WithColorScale(brontoHeatmapScale()),
		heatmap.WithValueRange(minV, maxV),
	)
	hm.SetXYRange(minX, maxX, minY, maxY)
	if chartShowAxis(chartSpec) {
		hm.SetXStep(2)
		hm.SetYStep(2)
		hm.DrawXYAxisAndLabel()
	} else {
		hm.SetXStep(0)
		hm.SetYStep(0)
	}
	for _, p := range points {
		hm.Push(p)
	}
	hm.Draw()
	return hm.View()
}

func renderTimeseriesChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "timeSeries" {
		return "(unsupported dataset for timeseries chart)"
	}
	if width < 8 || height < 6 {
		return "(timeseries chart too small)"
	}

	var refs []spec.SeriesRef
	timeFormat := ""
	if chartSpec.TimeSeries != nil {
		refs = chartSpec.TimeSeries.Series
		timeFormat = chartSpec.TimeSeries.TimeFormat
	}
	series := selectTimeSeries(ds, refs)
	if len(series) == 0 {
		return "(no timeseries data)"
	}

	minT, maxT, minY, maxY, ok := timeSeriesBounds(series)
	if !ok {
		return "(invalid timeseries data)"
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	xStep := 2
	yStep := 2
	if !chartShowAxis(chartSpec) {
		xStep = 0
		yStep = 0
	}

	opts := []timeserieslinechart.Option{
		timeserieslinechart.WithTimeRange(minT, maxT),
		timeserieslinechart.WithYRange(minY, maxY),
		timeserieslinechart.WithXYSteps(xStep, yStep),
		timeserieslinechart.WithAxesStyles(th.ChartAxis, th.ChartLabel),
		timeserieslinechart.WithStyle(th.ChartBar),
		timeserieslinechart.WithLineStyle(runes.ArcLineStyle),
		timeserieslinechart.WithYLabelFormatter(yAxisLabelFormatter(ds)),
	}
	if timeFormat != "" {
		layout := timeFormat
		opts = append(opts, timeserieslinechart.WithXLabelFormatter(func(i int, v float64) string {
			return time.Unix(int64(v), 0).UTC().Format(layout)
		}))
	} else {
		opts = append(opts, timeserieslinechart.WithXLabelFormatter(timeserieslinechart.HourTimeLabelFormatter()))
	}

	tslc := timeserieslinechart.New(width, height, opts...)
	names := make([]string, 0, len(series))
	for i, s := range series {
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("series_%d", i+1)
		}
		names = append(names, name)
		tslc.SetDataSetStyle(name, lineSeriesStyle(th, s.Variant, i))
		tslc.SetDataSetLineStyle(name, runes.ArcLineStyle)
		for _, p := range s.Points {
			parsed, err := time.Parse(time.RFC3339, p.T)
			if err != nil {
				continue
			}
			tslc.PushDataSet(name, timeserieslinechart.TimePoint{
				Time:  parsed,
				Value: p.V,
			})
		}
	}

	if chartSpec.Render.Mode == "braille" {
		tslc.DrawBrailleDataSets(names)
	} else {
		tslc.DrawDataSets(names)
	}
	return tslc.View()
}

func renderOHLCChart(th brontotheme.BrontoTheme, chartSpec spec.ChartSpec, ds spec.DatasetSpec, width, height int) string {
	if ds.Kind != "ohlcSeries" {
		return "(unsupported dataset for ohlc chart)"
	}
	if width < 8 || height < 6 {
		return "(ohlc chart too small)"
	}
	if len(ds.Candles) == 0 {
		return "(no ohlc data)"
	}

	minT, maxT, minY, maxY, ok := candleBounds(ds.Candles)
	if !ok {
		return "(invalid ohlc data)"
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	xStep := 2
	yStep := 2
	if !chartShowAxis(chartSpec) {
		xStep = 0
		yStep = 0
	}

	tslc := timeserieslinechart.New(
		width,
		height,
		timeserieslinechart.WithTimeRange(minT, maxT),
		timeserieslinechart.WithYRange(minY, maxY),
		timeserieslinechart.WithXYSteps(xStep, yStep),
		timeserieslinechart.WithAxesStyles(th.ChartAxis, th.ChartLabel),
		timeserieslinechart.WithXLabelFormatter(timeserieslinechart.HourTimeLabelFormatter()),
		timeserieslinechart.WithYLabelFormatter(yAxisLabelFormatter(ds)),
	)

	const (
		openDS  = "open"
		highDS  = "high"
		lowDS   = "low"
		closeDS = "close"
	)

	for _, c := range ds.Candles {
		parsed, err := time.Parse(time.RFC3339, c.T)
		if err != nil {
			continue
		}
		tslc.PushDataSet(openDS, timeserieslinechart.TimePoint{Time: parsed, Value: c.Open})
		tslc.PushDataSet(highDS, timeserieslinechart.TimePoint{Time: parsed, Value: c.High})
		tslc.PushDataSet(lowDS, timeserieslinechart.TimePoint{Time: parsed, Value: c.Low})
		tslc.PushDataSet(closeDS, timeserieslinechart.TimePoint{Time: parsed, Value: c.Close})
	}

	bull := th.ChartBar
	bear := th.ChartDanger
	if chartSpec.OHLC != nil && chartSpec.OHLC.Style == "ohlc" {
		bull = th.Text
		bear = th.Muted
	}
	tslc.DrawCandle(openDS, highDS, lowDS, closeDS, bull, bear)
	return tslc.View()
}

func selectXYSeries(ds spec.DatasetSpec, refs []spec.SeriesRef) []lineSeriesSelection {
	byName := make(map[string]spec.XYSeries, len(ds.XY))
	for _, series := range ds.XY {
		byName[series.Name] = series
	}
	if len(refs) == 0 {
		out := make([]lineSeriesSelection, 0, len(ds.XY))
		for _, series := range ds.XY {
			out = append(out, lineSeriesSelection{
				Name:   series.Name,
				Points: series.Points,
			})
		}
		return out
	}

	out := make([]lineSeriesSelection, 0, len(refs))
	for _, ref := range refs {
		series, ok := byName[ref.Name]
		if !ok {
			continue
		}
		out = append(out, lineSeriesSelection{
			Name:    series.Name,
			Variant: ref.Variant,
			Points:  series.Points,
		})
	}
	return out
}

func xyTimeXLabelFormatter(series []lineSeriesSelection) linechart.LabelFormatter {
	mode := detectXYTimeMode(series)
	if mode == "" {
		return nil
	}
	return func(_ int, v float64) string {
		if mode == "ms" {
			return time.UnixMilli(int64(v)).UTC().Format("15:04")
		}
		return time.Unix(int64(v), 0).UTC().Format("15:04")
	}
}

func detectXYTimeMode(series []lineSeriesSelection) string {
	total := 0
	ms := 0
	sec := 0
	for _, s := range series {
		for _, p := range s.Points {
			total++
			if isUnixMillis(p.X) {
				ms++
			} else if isUnixSeconds(p.X) {
				sec++
			}
		}
	}
	if total == 0 {
		return ""
	}
	if ms*100/total >= 80 {
		return "ms"
	}
	if sec*100/total >= 80 {
		return "sec"
	}
	return ""
}

func isUnixSeconds(v float64) bool {
	return v >= 946684800 && v <= 4102444800
}

func isUnixMillis(v float64) bool {
	return v >= 946684800000 && v <= 4102444800000
}

func yAxisLabelFormatter(ds spec.DatasetSpec) linechart.LabelFormatter {
	return func(_ int, v float64) string {
		return formatAxisValue(v, ds.Format, ds.Unit)
	}
}

func formatAxisValue(v float64, format string, unit string) string {
	switch format {
	case "bytes":
		return formatBytes(v)
	case "duration":
		return formatDuration(v, unit)
	default:
		value := compactNumber(v)
		u := strings.TrimSpace(unit)
		if u == "" {
			return value
		}
		return value + u
	}
}

func compactNumber(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%s%.1fB", sign, abs/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%s%.1fM", sign, abs/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%s%.1fk", sign, abs/1_000)
	case abs >= 10:
		return fmt.Sprintf("%s%.0f", sign, abs)
	case abs >= 1:
		return fmt.Sprintf("%s%.1f", sign, abs)
	default:
		return fmt.Sprintf("%s%.2f", sign, abs)
	}
}

func formatBytes(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	idx := 0
	for abs >= 1024 && idx < len(units)-1 {
		abs /= 1024
		idx++
	}
	if abs >= 10 || idx == 0 {
		return fmt.Sprintf("%s%.0f%s", sign, abs, units[idx])
	}
	return fmt.Sprintf("%s%.1f%s", sign, abs, units[idx])
}

func formatDuration(v float64, unit string) string {
	multiplier := time.Millisecond
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "ns", "nanosecond", "nanoseconds":
		multiplier = time.Nanosecond
	case "us", "µs", "microsecond", "microseconds":
		multiplier = time.Microsecond
	case "s", "sec", "second", "seconds":
		multiplier = time.Second
	case "m", "min", "minute", "minutes":
		multiplier = time.Minute
	case "h", "hour", "hours":
		multiplier = time.Hour
	default:
		multiplier = time.Millisecond
	}
	d := time.Duration(v * float64(multiplier))
	if d < 0 {
		return "-" + formatDuration(-v, unit)
	}
	if d >= time.Hour {
		h := d / time.Hour
		d -= h * time.Hour
		m := d / time.Minute
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if d >= time.Minute {
		m := d / time.Minute
		d -= m * time.Minute
		s := d / time.Second
		return fmt.Sprintf("%dm%ds", m, s)
	}
	if d >= time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d >= time.Millisecond {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d >= time.Microsecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return strconv.FormatInt(d.Nanoseconds(), 10) + "ns"
}

func selectTimeSeries(ds spec.DatasetSpec, refs []spec.SeriesRef) []timeSeriesSelection {
	byName := make(map[string]spec.TimeSeries, len(ds.Time))
	for _, series := range ds.Time {
		byName[series.Name] = series
	}
	if len(refs) == 0 {
		out := make([]timeSeriesSelection, 0, len(ds.Time))
		for _, series := range ds.Time {
			out = append(out, timeSeriesSelection{
				Name:   series.Name,
				Points: series.Points,
			})
		}
		return out
	}

	out := make([]timeSeriesSelection, 0, len(refs))
	for _, ref := range refs {
		series, ok := byName[ref.Name]
		if !ok {
			continue
		}
		out = append(out, timeSeriesSelection{
			Name:    series.Name,
			Variant: ref.Variant,
			Points:  series.Points,
		})
	}
	return out
}

func timeSeriesBounds(series []timeSeriesSelection) (time.Time, time.Time, float64, float64, bool) {
	initialized := false
	var minT time.Time
	var maxT time.Time
	minY := 0.0
	maxY := 0.0

	for _, s := range series {
		for _, p := range s.Points {
			parsed, err := time.Parse(time.RFC3339, p.T)
			if err != nil {
				continue
			}
			if !initialized {
				minT, maxT = parsed, parsed
				minY, maxY = p.V, p.V
				initialized = true
				continue
			}
			if parsed.Before(minT) {
				minT = parsed
			}
			if parsed.After(maxT) {
				maxT = parsed
			}
			if p.V < minY {
				minY = p.V
			}
			if p.V > maxY {
				maxY = p.V
			}
		}
	}
	if !initialized {
		return time.Time{}, time.Time{}, 0, 0, false
	}
	if !minT.Before(maxT) {
		maxT = minT.Add(time.Second)
	}
	return minT, maxT, minY, maxY, true
}

func candleBounds(candles []spec.Candle) (time.Time, time.Time, float64, float64, bool) {
	if len(candles) == 0 {
		return time.Time{}, time.Time{}, 0, 0, false
	}

	minY := candles[0].Low
	maxY := candles[0].High
	minT, err := time.Parse(time.RFC3339, candles[0].T)
	if err != nil {
		return time.Time{}, time.Time{}, 0, 0, false
	}
	maxT := minT

	for _, c := range candles {
		parsed, err := time.Parse(time.RFC3339, c.T)
		if err != nil {
			continue
		}
		if parsed.Before(minT) {
			minT = parsed
		}
		if parsed.After(maxT) {
			maxT = parsed
		}
		if c.Low < minY {
			minY = c.Low
		}
		if c.High > maxY {
			maxY = c.High
		}
	}
	if !minT.Before(maxT) {
		maxT = minT.Add(time.Second)
	}
	return minT, maxT, minY, maxY, true
}

func extractHeatmapPoints(ds spec.DatasetSpec) ([]heatmap.HeatPoint, float64, float64, float64, float64, float64, float64) {
	h := ds.Heatmap
	if h == nil {
		return nil, 0, 0, 0, 0, 0, 0
	}

	points := make([]heatmap.HeatPoint, 0)
	minX := 0.0
	maxX := 0.0
	minY := 0.0
	maxY := 0.0
	minV := 0.0
	maxV := 0.0
	initialized := false

	push := func(x, y, v float64) {
		points = append(points, heatmap.NewHeatPoint(x, y, v))
		if !initialized {
			minX, maxX = x, x
			minY, maxY = y, y
			minV, maxV = v, v
			initialized = true
			return
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	if h.Width > 0 && h.Height > 0 && len(h.Values) > 0 {
		for y := 0; y < h.Height; y++ {
			for x := 0; x < h.Width; x++ {
				idx := y*h.Width + x
				if idx < 0 || idx >= len(h.Values) {
					continue
				}
				push(float64(x), float64(y), h.Values[idx])
			}
		}
	} else {
		for _, c := range h.Cells {
			push(float64(c.X), float64(c.Y), c.V)
		}
	}

	if !initialized {
		return nil, 0, 0, 0, 0, 0, 0
	}
	return points, minX, maxX, minY, maxY, minV, maxV
}

func brontoHeatmapScale() []color.Color {
	return []color.Color{
		lipgloss.Color("#1D3557"),
		lipgloss.Color("#457B9D"),
		lipgloss.Color("#2A9D8F"),
		lipgloss.Color("#8AB17D"),
		lipgloss.Color("#E9C46A"),
		lipgloss.Color("#F4A261"),
		lipgloss.Color("#E76F51"),
		lipgloss.Color("#D62828"),
	}
}

func valuesMinMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	minV := values[0]
	maxV := values[0]
	for _, v := range values {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	return minV, maxV
}

func sparkWindow(chartSpec spec.ChartSpec) int {
	if chartSpec.Sparkline == nil || chartSpec.Sparkline.Window <= 0 {
		return 0
	}
	return chartSpec.Sparkline.Window
}

func streamWindow(chartSpec spec.ChartSpec) int {
	if chartSpec.Stream == nil || chartSpec.Stream.Window <= 0 {
		return 0
	}
	return chartSpec.Stream.Window
}

func windowedValues(values []float64, window int) []float64 {
	if window <= 0 || len(values) <= window {
		return values
	}
	return values[len(values)-window:]
}

func chartShowAxis(chartSpec spec.ChartSpec) bool {
	return chartSpec.Render.ShowAxis == nil || *chartSpec.Render.ShowAxis
}

func drawHorizontalBarValueLabels(
	bar *barchart.Model,
	th brontotheme.BrontoTheme,
	axisLabels []string,
	values []float64,
	unit string,
) {
	if bar == nil || !bar.Horizontal() {
		return
	}
	if len(values) == 0 {
		return
	}

	labelMax := 0
	for _, l := range axisLabels {
		if w := runeWidth(l); w > labelMax {
			labelMax = w
		}
	}

	startX := 0
	if bar.ShowAxis() {
		startX = labelMax + 1
	}
	graphW := bar.Width() - startX
	if graphW <= 1 {
		return
	}

	barWidth := maxInt(1, bar.BarWidth())
	barGap := maxInt(0, bar.BarGap())
	scale := bar.Scale()
	maxTextW := clampInt(graphW/3, 3, 14)
	textStyle := th.Text.Copy().Foreground(lipgloss.Color(brontotheme.BrontoBlack)).Bold(true)

	limit := len(values)
	maxRows := (bar.Height() + barGap) / maxInt(1, barWidth+barGap)
	if limit > maxRows {
		limit = maxRows
	}

	for i := 0; i < limit; i++ {
		v := values[i]
		if v <= 0 {
			continue
		}
		fill := int(math.Round(v * scale))
		if fill <= 1 {
			continue
		}
		if fill > graphW {
			fill = graphW
		}

		rowStart := i * (barWidth + barGap)
		if rowStart >= bar.Height() {
			break
		}
		y := rowStart + (barWidth / 2)
		if y >= bar.Height() {
			y = bar.Height() - 1
		}

		text := formatValueWithUnit(v, unit)
		text = truncateCell(text, minInt(maxTextW, fill-1), colNumeric)
		textW := runeWidth(text)
		if textW <= 0 || textW >= fill {
			continue
		}

		x := startX + fill - textW
		if x < startX {
			x = startX
		}
		bar.Canvas.SetStringWithStyle(canvas.Point{X: x, Y: y}, text, textStyle)
	}
}
