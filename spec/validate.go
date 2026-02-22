package spec

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *AppSpec) Validate() error {
	if s.Version != "bronto-tui/v1" {
		return fmt.Errorf("spec invalid: version must be %q", "bronto-tui/v1")
	}
	if s.Title == "" {
		return fmt.Errorf("spec invalid: missing title")
	}
	if s.Name != "" {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("spec invalid: name must not be whitespace-only")
		}
	}

	if s.Meta.GeneratedAt != "" {
		if _, err := time.Parse(time.RFC3339, s.Meta.GeneratedAt); err != nil {
			return fmt.Errorf("spec invalid: meta.generatedAt must be RFC3339")
		}
	}

	if s.Defaults.ChartRender.Mode != "" && s.Defaults.ChartRender.Mode != "ascii" && s.Defaults.ChartRender.Mode != "braille" {
		return fmt.Errorf("spec invalid: defaults.chartRender.mode must be ascii|braille")
	}
	if s.Defaults.Table.RowLimit < 0 {
		return fmt.Errorf("spec invalid: defaults.table.rowLimit must be >= 0")
	}

	if s.Theme.Brand == "" {
		s.Theme.Brand = "bronto"
	}
	switch s.Theme.Density {
	case "", "comfortable":
		s.Theme.Density = "comfortable"
	case "compact":
	default:
		return fmt.Errorf("spec invalid: theme.density must be compact|comfortable")
	}

	if len(s.Datasets) == 0 {
		return fmt.Errorf("spec invalid: datasets must be non-empty")
	}
	if len(s.Charts) == 0 && len(s.Tables) == 0 {
		return fmt.Errorf("spec invalid: at least one of charts or tables must be non-empty")
	}

	for id, d := range s.Datasets {
		if id == "" {
			return fmt.Errorf("spec invalid: dataset id cannot be empty")
		}
		normalized, err := validateDataset(id, d)
		if err != nil {
			return err
		}
		s.Datasets[id] = normalized
	}

	for id, c := range s.Charts {
		if id == "" {
			return fmt.Errorf("spec invalid: chart id cannot be empty")
		}
		normalized, err := validateChart(id, c, s.Datasets, s.Defaults.ChartRender)
		if err != nil {
			return err
		}
		s.Charts[id] = normalized
	}

	for id, t := range s.Tables {
		if id == "" {
			return fmt.Errorf("spec invalid: table id cannot be empty")
		}
		normalized, err := validateTable(id, t, s.Datasets, s.Defaults.Table.RowLimit)
		if err != nil {
			return err
		}
		s.Tables[id] = normalized
	}

	return nil
}

func validateDataset(id string, d DatasetSpec) (DatasetSpec, error) {
	switch d.Format {
	case "", "number", "bytes", "duration":
	default:
		return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q format must be number|bytes|duration", id)
	}
	if d.Unit != "" {
		if strings.TrimSpace(d.Unit) == "" {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q unit must not be whitespace-only", id)
		}
		if len(d.Unit) > 16 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q unit length must be <= 16", id)
		}
	}

	switch d.Kind {
	case "categorySeries":
		if len(d.Labels) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q categorySeries labels must be non-empty", id)
		}
		if len(d.Values) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q categorySeries values must be non-empty", id)
		}
		if len(d.Labels) != len(d.Values) {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q categorySeries labels/values length mismatch", id)
		}

	case "table":
		if len(d.Columns) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q table columns must be non-empty", id)
		}
		seenCols := map[string]bool{}
		for i, c := range d.Columns {
			if c == "" {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q table column[%d] must be non-empty", id, i)
			}
			if seenCols[c] {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q table has duplicate column %q", id, c)
			}
			seenCols[c] = true
		}
		for i, r := range d.Rows {
			if len(r) != len(d.Columns) {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q row %d has %d cells; expected %d", id, i, len(r), len(d.Columns))
			}
		}

	case "xySeries":
		if len(d.XY) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q xySeries must include non-empty \"xy\"", id)
		}
		seen := map[string]bool{}
		for i, series := range d.XY {
			if series.Name == "" {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q xy[%d] missing series name", id, i)
			}
			if seen[series.Name] {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q duplicate xy series %q", id, series.Name)
			}
			seen[series.Name] = true
			if len(series.Points) == 0 {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q xy series %q must have at least one point", id, series.Name)
			}
		}

	case "timeSeries":
		if len(d.Time) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q timeSeries must include non-empty \"time\"", id)
		}
		seen := map[string]bool{}
		for i := range d.Time {
			series := d.Time[i]
			if series.Name == "" {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q time[%d] missing series name", id, i)
			}
			if seen[series.Name] {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q duplicate time series %q", id, series.Name)
			}
			seen[series.Name] = true
			if len(series.Points) == 0 {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q time series %q must have at least one point", id, series.Name)
			}
			type timedPoint struct {
				t time.Time
				p TimePoint
			}
			timed := make([]timedPoint, len(series.Points))
			for j, p := range series.Points {
				if p.T == "" {
					return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q time series %q point[%d] missing t", id, series.Name, j)
				}
				parsed, err := time.Parse(time.RFC3339, p.T)
				if err != nil {
					return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q time series %q point[%d] invalid RFC3339 timestamp %q", id, series.Name, j, p.T)
				}
				timed[j] = timedPoint{t: parsed, p: p}
			}

			sort.Slice(timed, func(a, b int) bool {
				return timed[a].t.Before(timed[b].t)
			})
			sorted := make([]TimePoint, len(timed))
			for j := range timed {
				sorted[j] = timed[j].p
			}
			series.Points = sorted
			d.Time[i] = series
		}

	case "ohlcSeries":
		if len(d.Candles) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q ohlcSeries must include non-empty \"candles\"", id)
		}
		type timedCandle struct {
			t time.Time
			c Candle
		}
		timed := make([]timedCandle, len(d.Candles))
		for i, c := range d.Candles {
			if c.T == "" {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q candles[%d] missing t", id, i)
			}
			parsed, err := time.Parse(time.RFC3339, c.T)
			if err != nil {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q candles[%d] invalid RFC3339 timestamp %q", id, i, c.T)
			}
			if c.High < c.Low {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q candles[%d] high must be >= low", id, i)
			}
			if c.Open < c.Low || c.Open > c.High {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q candles[%d] open must be in [low, high]", id, i)
			}
			if c.Close < c.Low || c.Close > c.High {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q candles[%d] close must be in [low, high]", id, i)
			}
			timed[i] = timedCandle{t: parsed, c: c}
		}
		sort.Slice(timed, func(i, j int) bool {
			return timed[i].t.Before(timed[j].t)
		})
		sorted := make([]Candle, len(timed))
		for i := range timed {
			sorted[i] = timed[i].c
		}
		d.Candles = sorted

	case "heatmapCells":
		if d.Heatmap == nil {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmapCells requires non-null heatmap", id)
		}
		h := d.Heatmap
		dense := h.Width != 0 || h.Height != 0 || len(h.Values) > 0
		sparse := len(h.Cells) > 0
		if dense && sparse {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmap must be dense or sparse, not both", id)
		}
		if !dense && !sparse {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmap must define dense values or sparse cells", id)
		}
		if dense {
			if h.Width <= 0 || h.Height <= 0 {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q dense heatmap requires width>0 and height>0", id)
			}
			expected := h.Width * h.Height
			if len(h.Values) != expected {
				return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q dense heatmap values length must be width*height (%d)", id, expected)
			}
		}
		if sparse {
			for i, cell := range h.Cells {
				if cell.X < 0 || cell.Y < 0 {
					return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmap cell[%d] x/y must be >= 0", id, i)
				}
				if h.Width > 0 && cell.X >= h.Width {
					return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmap cell[%d] x out of range", id, i)
				}
				if h.Height > 0 && cell.Y >= h.Height {
					return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q heatmap cell[%d] y out of range", id, i)
				}
			}
		}

	case "valueSeries":
		if len(d.Value) == 0 {
			return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q valueSeries must include non-empty \"value\"", id)
		}

	default:
		return DatasetSpec{}, fmt.Errorf("spec invalid: dataset %q has unsupported kind %q", id, d.Kind)
	}

	return d, nil
}

func validateChart(id string, c ChartSpec, datasets map[string]DatasetSpec, defaults ChartRender) (ChartSpec, error) {
	if c.Render.Mode == "" && defaults.Mode != "" {
		c.Render.Mode = defaults.Mode
	}
	if c.Render.ShowAxis == nil && defaults.ShowAxis != nil {
		c.Render.ShowAxis = boolPtr(*defaults.ShowAxis)
	}
	if c.Render.Mode == "" {
		c.Render.Mode = "ascii"
	}
	if c.Render.ShowAxis == nil {
		c.Render.ShowAxis = boolPtr(true)
	}

	if c.Render.Mode != "ascii" && c.Render.Mode != "braille" {
		return ChartSpec{}, fmt.Errorf("spec invalid: chart %q render.mode must be ascii|braille", id)
	}
	if c.DatasetRef == "" {
		return ChartSpec{}, fmt.Errorf("spec invalid: chart %q missing datasetRef", id)
	}
	d, ok := datasets[c.DatasetRef]
	if !ok {
		return ChartSpec{}, fmt.Errorf("spec invalid: chart %q references missing dataset %q", id, c.DatasetRef)
	}

	optionCount := countChartOptionBlocks(c)

	switch c.Family {
	case "bar":
		if c.Bar == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"bar\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Bar.Orientation == "" {
			c.Bar.Orientation = "vertical"
		}
		if c.Bar.Orientation != "horizontal" && c.Bar.Orientation != "vertical" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q bar.orientation must be horizontal|vertical", id)
		}
		// Renderer policy: bar charts are always vertical.
		if c.Bar.Orientation == "horizontal" {
			c.Bar.Orientation = "vertical"
		}
		if d.Kind != "categorySeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects categorySeries dataset; got %q", id, d.Kind)
		}

	case "heatmap":
		if c.Heatmap == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"heatmap\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Heatmap.Min != nil && c.Heatmap.Max != nil && *c.Heatmap.Min > *c.Heatmap.Max {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q heatmap.min must be <= heatmap.max", id)
		}
		if d.Kind != "heatmapCells" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects heatmapCells dataset; got %q", id, d.Kind)
		}

	case "line":
		if c.Line == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"line\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Line.Style.Interpolation != "" && c.Line.Style.Interpolation != "linear" && c.Line.Style.Interpolation != "step" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q line.style.interpolation must be linear|step", id)
		}
		if d.Kind != "xySeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects xySeries dataset; got %q", id, d.Kind)
		}
		if err := validateSeriesRefs(c.Line.Series, fmt.Sprintf("chart %q line.series", id), namesFromXY(d), true); err != nil {
			return ChartSpec{}, err
		}

	case "ohlc":
		if c.OHLC == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"ohlc\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.OHLC.Style == "" {
			c.OHLC.Style = "candle"
		}
		if c.OHLC.Style != "candle" && c.OHLC.Style != "ohlc" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q ohlc.style must be candle|ohlc", id)
		}
		if d.Kind != "ohlcSeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects ohlcSeries dataset; got %q", id, d.Kind)
		}

	case "scatter":
		if c.Scatter == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"scatter\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Scatter.PointRune != "" && utf8.RuneCountInString(c.Scatter.PointRune) != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q scatter.pointRune must be a single rune", id)
		}
		if d.Kind != "xySeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects xySeries dataset; got %q", id, d.Kind)
		}

	case "streamline":
		if c.Stream == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"streamline\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Stream.Window < 0 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q streamline.window must be >= 0", id)
		}
		if c.Stream.Window == 0 {
			c.Stream.Window = 120
		}
		if d.Kind != "valueSeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects valueSeries dataset; got %q", id, d.Kind)
		}

	case "timeseries":
		if c.TimeSeries == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"timeseries\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if d.Kind != "timeSeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects timeSeries dataset; got %q", id, d.Kind)
		}
		if err := validateSeriesRefs(c.TimeSeries.Series, fmt.Sprintf("chart %q timeseries.series", id), namesFromTime(d), true); err != nil {
			return ChartSpec{}, err
		}

	case "waveline":
		if c.Waveline == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"waveline\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if d.Kind != "xySeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects xySeries dataset; got %q", id, d.Kind)
		}
		if err := validateSeriesRefs(c.Waveline.Series, fmt.Sprintf("chart %q waveline.series", id), namesFromXY(d), true); err != nil {
			return ChartSpec{}, err
		}

	case "sparkline":
		if c.Sparkline == nil {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q family %q requires \"sparkline\" options", id, c.Family)
		}
		if optionCount != 1 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q must define only the %q options block", id, c.Family)
		}
		if c.Sparkline.Window < 0 {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q sparkline.window must be >= 0", id)
		}
		if c.Sparkline.Window == 0 {
			c.Sparkline.Window = 120
		}
		if d.Kind != "valueSeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects valueSeries dataset; got %q", id, d.Kind)
		}

	default:
		return ChartSpec{}, fmt.Errorf("spec invalid: chart %q unsupported family %q", id, c.Family)
	}

	return c, nil
}

func validateSeriesRefs(refs []SeriesRef, field string, allowed map[string]bool, required bool) error {
	if required && len(refs) == 0 {
		return fmt.Errorf("spec invalid: %s must be non-empty", field)
	}

	seen := map[string]bool{}
	for i, r := range refs {
		if r.Name == "" {
			return fmt.Errorf("spec invalid: %s[%d] missing name", field, i)
		}
		if seen[r.Name] {
			return fmt.Errorf("spec invalid: %s has duplicate series name %q", field, r.Name)
		}
		seen[r.Name] = true
		if allowed != nil && !allowed[r.Name] {
			return fmt.Errorf("spec invalid: %s[%d] references missing dataset series %q", field, i, r.Name)
		}
		if r.Variant != "" && r.Variant != "primary" && r.Variant != "muted" && r.Variant != "danger" {
			return fmt.Errorf("spec invalid: %s[%d] variant must be primary|muted|danger", field, i)
		}
	}
	return nil
}

func namesFromXY(d DatasetSpec) map[string]bool {
	out := make(map[string]bool, len(d.XY))
	for _, s := range d.XY {
		out[s.Name] = true
	}
	return out
}

func namesFromTime(d DatasetSpec) map[string]bool {
	out := make(map[string]bool, len(d.Time))
	for _, s := range d.Time {
		out[s.Name] = true
	}
	return out
}

func countChartOptionBlocks(c ChartSpec) int {
	count := 0
	if c.Bar != nil {
		count++
	}
	if c.Heatmap != nil {
		count++
	}
	if c.Line != nil {
		count++
	}
	if c.OHLC != nil {
		count++
	}
	if c.Scatter != nil {
		count++
	}
	if c.Stream != nil {
		count++
	}
	if c.TimeSeries != nil {
		count++
	}
	if c.Waveline != nil {
		count++
	}
	if c.Sparkline != nil {
		count++
	}
	return count
}

func validateTable(id string, t TableSpec, datasets map[string]DatasetSpec, defaultRowLimit int) (TableSpec, error) {
	if t.DatasetRef == "" {
		return TableSpec{}, fmt.Errorf("spec invalid: table %q missing datasetRef", id)
	}
	d, ok := datasets[t.DatasetRef]
	if !ok {
		return TableSpec{}, fmt.Errorf("spec invalid: table %q references missing dataset %q", id, t.DatasetRef)
	}
	if d.Kind != "table" {
		return TableSpec{}, fmt.Errorf("spec invalid: table %q expects table dataset; got %q", id, d.Kind)
	}
	if len(t.Columns) == 0 {
		return TableSpec{}, fmt.Errorf("spec invalid: table %q columns must be non-empty", id)
	}
	if t.RowLimit < 0 {
		return TableSpec{}, fmt.Errorf("spec invalid: table %q rowLimit must be >= 0", id)
	}
	if t.RowLimit == 0 {
		if defaultRowLimit > 0 {
			t.RowLimit = defaultRowLimit
		} else {
			t.RowLimit = 200
		}
	}

	datasetCols := map[string]bool{}
	for _, c := range d.Columns {
		datasetCols[c] = true
	}

	for i, c := range t.Columns {
		if c.Key == "" || c.Title == "" {
			return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] missing key/title", id, i)
		}
		if !datasetCols[c.Key] {
			return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] key %q not found in dataset columns", id, i, c.Key)
		}
		switch w := c.Width.(type) {
		case string:
			if w != "auto" && w != "flex" {
				return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] width must be auto|flex|number", id, i)
			}
		case float64:
			if w < 1 {
				return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] width must be >= 1", id, i)
			}
		case nil:
			// allow missing -> treat as auto in renderer
		default:
			return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] width must be auto|flex|number", id, i)
		}
	}
	return t, nil
}

func validateNode(n Node, charts map[string]ChartSpec, tables map[string]TableSpec, seenIDs map[string]bool) error {
	if n.Type == "" {
		return fmt.Errorf("spec invalid: layout node missing type")
	}
	if n.ID == "" {
		return fmt.Errorf("spec invalid: layout node type %q missing id", n.Type)
	}
	if seenIDs[n.ID] {
		return fmt.Errorf("spec invalid: duplicate layout node id %q", n.ID)
	}
	seenIDs[n.ID] = true

	if n.Gap < 0 {
		return fmt.Errorf("spec invalid: layout node %q gap must be >= 0", n.ID)
	}

	switch n.Type {
	case "col":
		if len(n.Children) == 0 {
			return fmt.Errorf("spec invalid: col %q must have children", n.ID)
		}
		for _, ch := range n.Children {
			if err := validateNode(ch, charts, tables, seenIDs); err != nil {
				return err
			}
		}

	case "row":
		if len(n.Children) == 0 {
			return fmt.Errorf("spec invalid: row %q must have children", n.ID)
		}
		if len(n.Weights) > 0 {
			if len(n.Weights) != len(n.Children) {
				return fmt.Errorf("spec invalid: row %q weights length must equal children length", n.ID)
			}
			for i, w := range n.Weights {
				if w <= 0 {
					return fmt.Errorf("spec invalid: row %q weights[%d] must be > 0", n.ID, i)
				}
			}
		}
		for _, ch := range n.Children {
			if err := validateNode(ch, charts, tables, seenIDs); err != nil {
				return err
			}
		}

	case "header":
		if n.TitleRef != "$title" {
			return fmt.Errorf("spec invalid: header %q titleRef must be %q", n.ID, "$title")
		}

	case "chart":
		if n.ChartRef == "" {
			return fmt.Errorf("spec invalid: chart node %q missing chartRef", n.ID)
		}
		if _, ok := charts[n.ChartRef]; !ok {
			return fmt.Errorf("spec invalid: chart node %q references missing chart %q", n.ID, n.ChartRef)
		}

	case "table":
		if n.TableRef == "" {
			return fmt.Errorf("spec invalid: table node %q missing tableRef", n.ID)
		}
		if _, ok := tables[n.TableRef]; !ok {
			return fmt.Errorf("spec invalid: table node %q references missing table %q", n.ID, n.TableRef)
		}

	case "text":
		if n.Text == "" {
			return fmt.Errorf("spec invalid: text node %q missing text", n.ID)
		}
		if n.Variant != "" && n.Variant != "muted" && n.Variant != "primary" && n.Variant != "danger" {
			return fmt.Errorf("spec invalid: text node %q variant must be muted|primary|danger", n.ID)
		}

	default:
		return fmt.Errorf("spec invalid: unsupported layout node type %q", n.Type)
	}

	return nil
}

func boolPtr(v bool) *bool {
	b := v
	return &b
}
