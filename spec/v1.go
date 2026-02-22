package spec

// ---------- Top-level ----------

type AppSpec struct {
	Version  string                 `json:"version"`
	Name     string                 `json:"name,omitempty"` // optional friendly id, e.g. "errors-dashboard"
	Title    string                 `json:"title"`
	Theme    ThemeSpec              `json:"theme"`
	Defaults DefaultsSpec           `json:"defaults,omitempty"`
	Meta     MetaSpec               `json:"meta,omitempty"`
	Layout   Node                   `json:"layout,omitempty"` // renderer-owned; accepted but ignored by default UI builder
	Charts   map[string]ChartSpec   `json:"charts"`
	Tables   map[string]TableSpec   `json:"tables"`
	Datasets map[string]DatasetSpec `json:"datasets"`
}

type ThemeSpec struct {
	Brand   string `json:"brand"`   // "bronto"
	Density string `json:"density"` // "compact"|"comfortable"
}

type DefaultsSpec struct {
	ChartRender ChartRender   `json:"chartRender,omitempty"`
	Table       TableDefaults `json:"table,omitempty"`
}

type TableDefaults struct {
	RowLimit int `json:"rowLimit,omitempty"`
}

type MetaSpec struct {
	GeneratedBy string `json:"generatedBy,omitempty"` // "codex", "claude", etc.
	GeneratedAt string `json:"generatedAt,omitempty"` // RFC3339
	RequestID   string `json:"requestId,omitempty"`
}

// ---------- Layout Node (tagged union) ----------

type Node struct {
	Type string `json:"type"`

	// Common
	ID string `json:"id,omitempty"`

	// Containers
	Gap      int    `json:"gap,omitempty"`
	Weights  []int  `json:"weights,omitempty"`
	Children []Node `json:"children,omitempty"`

	// Header
	TitleRef string `json:"titleRef,omitempty"` // "$title"
	SubTitle string `json:"subTitle,omitempty"`

	// Leaves
	Title    string `json:"title,omitempty"`
	ChartRef string `json:"chartRef,omitempty"`
	TableRef string `json:"tableRef,omitempty"`

	// Text leaf
	Text    string `json:"text,omitempty"`
	Variant string `json:"variant,omitempty"` // "muted"|"primary"|"danger"
}

// ---------- Charts ----------

type ChartSpec struct {
	Family string `json:"family"` // bar|heatmap|line|ohlc|scatter|streamline|timeseries|waveline|sparkline

	Title      string `json:"title,omitempty"`      // optional override for leaf title
	DatasetRef string `json:"datasetRef,omitempty"` // almost all charts use datasets

	Render ChartRender `json:"render,omitempty"`

	Bar        *BarChartOptions   `json:"bar,omitempty"`
	Heatmap    *HeatmapOptions    `json:"heatmap,omitempty"`
	Line       *LineChartOptions  `json:"line,omitempty"`
	OHLC       *OHLCOptions       `json:"ohlc,omitempty"`
	Scatter    *ScatterOptions    `json:"scatter,omitempty"`
	Stream     *StreamlineOptions `json:"streamline,omitempty"`
	TimeSeries *TimeSeriesOptions `json:"timeseries,omitempty"`
	Waveline   *WavelineOptions   `json:"waveline,omitempty"`
	Sparkline  *SparklineOptions  `json:"sparkline,omitempty"`
}

type ChartRender struct {
	Mode string `json:"mode,omitempty"` // ascii|braille

	ShowAxis *bool `json:"showAxis,omitempty"` // default true where applicable
}

type BarChartOptions struct {
	Orientation string `json:"orientation,omitempty"` // horizontal|vertical (inputs are normalized to vertical by renderer policy)
	Stacked     bool   `json:"stacked,omitempty"`     // future-friendly
}

type HeatmapOptions struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type LineChartOptions struct {
	Series []SeriesRef `json:"series,omitempty"`
	Style  LineStyle   `json:"style,omitempty"`
}

type WavelineOptions struct {
	Series []SeriesRef `json:"series,omitempty"`
}

type SeriesRef struct {
	Name    string `json:"name"`
	Variant string `json:"variant,omitempty"` // primary|muted|danger
}

type LineStyle struct {
	Interpolation string `json:"interpolation,omitempty"` // linear|step
	Markers       bool   `json:"markers,omitempty"`
}

type ScatterOptions struct {
	PointRune string `json:"pointRune,omitempty"`
}

type OHLCOptions struct {
	Style string `json:"style,omitempty"` // candle|ohlc
}

type StreamlineOptions struct {
	Window int `json:"window,omitempty"`
}

type TimeSeriesOptions struct {
	Series     []SeriesRef `json:"series,omitempty"`
	TimeFormat string      `json:"timeFormat,omitempty"`
}

type SparklineOptions struct {
	Window int `json:"window,omitempty"`
}

// ---------- Tables ----------

type TableSpec struct {
	DatasetRef string            `json:"datasetRef"`
	Columns    []TableColumnSpec `json:"columns"`
	RowLimit   int               `json:"rowLimit,omitempty"` // default 200
}

type TableColumnSpec struct {
	Key   string      `json:"key"`
	Title string      `json:"title"`
	Width interface{} `json:"width,omitempty"` // omit or null means auto; also supports "auto"|"flex"|number
}

// ---------- Datasets (tagged union) ----------

type DatasetSpec struct {
	Kind string `json:"kind"` // categorySeries|table|xySeries|timeSeries|ohlcSeries|heatmapCells|valueSeries

	Unit   string         `json:"unit,omitempty"`   // e.g. "ms", "%", "count"
	Format string         `json:"format,omitempty"` // number|bytes|duration
	Live   *LiveQuerySpec `json:"liveQuery,omitempty"`

	// categorySeries (bar)
	Labels []string  `json:"labels,omitempty"`
	Values []float64 `json:"values,omitempty"`

	// table
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`

	// xySeries (line/waveline/scatter)
	XY []XYSeries `json:"xy,omitempty"`

	// timeSeries
	Time []TimeSeries `json:"time,omitempty"`

	// ohlcSeries
	Candles []Candle `json:"candles,omitempty"`

	// heatmapCells
	Heatmap *HeatmapData `json:"heatmap,omitempty"`

	// valueSeries
	Value []float64 `json:"value,omitempty"`
}

type LiveQuerySpec struct {
	Mode            string   `json:"mode,omitempty"` // metrics|logs
	LogIDs          []string `json:"logIds,omitempty"`
	MetricFunctions []string `json:"metricFunctions,omitempty"`
	SearchFilter    string   `json:"searchFilter,omitempty"`
	GroupByKeys     []string `json:"groupByKeys,omitempty"`
	LookbackSec     int      `json:"lookbackSec,omitempty"`
	Limit           int      `json:"limit,omitempty"`
}

type XYSeries struct {
	Name   string    `json:"name"`
	Points []XYPoint `json:"points"`
}

type XYPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type TimeSeries struct {
	Name   string      `json:"name"`
	Points []TimePoint `json:"points"`
}

type TimePoint struct {
	T string  `json:"t"` // RFC3339
	V float64 `json:"v"`
}

type Candle struct {
	T     string  `json:"t"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type HeatmapData struct {
	Width  int       `json:"width,omitempty"`
	Height int       `json:"height,omitempty"`
	Values []float64 `json:"values,omitempty"` // len == width*height for dense form

	Cells []HeatCell `json:"cells,omitempty"` // sparse form
}

type HeatCell struct {
	X int     `json:"x"`
	Y int     `json:"y"`
	V float64 `json:"v"`
}
