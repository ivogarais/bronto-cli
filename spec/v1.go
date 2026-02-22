package spec

// ---------- Top-level ----------

type AppSpec struct {
	Version  string                 `json:"version"`
	Title    string                 `json:"title"`
	Theme    ThemeSpec              `json:"theme"`
	Layout   Node                   `json:"layout"`
	Charts   map[string]ChartSpec   `json:"charts"`
	Tables   map[string]TableSpec   `json:"tables"`
	Datasets map[string]DatasetSpec `json:"datasets"`
}

type ThemeSpec struct {
	Brand   string `json:"brand"`   // "bronto"
	Density string `json:"density"` // "compact"|"comfortable"
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
	Family      string      `json:"family"`                // "bar" only for v1 impl
	Orientation string      `json:"orientation,omitempty"` // "horizontal"|"vertical"
	DatasetRef  string      `json:"datasetRef"`            // dataset id
	Render      ChartRender `json:"render,omitempty"`
	// (future: line series config, etc.)
}

type ChartRender struct {
	Mode     string `json:"mode,omitempty"`     // "ascii" (v1)
	ShowAxis *bool  `json:"showAxis,omitempty"` // default true
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
	Width interface{} `json:"width"` // "auto"|"flex"|number
}

// ---------- Datasets (tagged union) ----------

type DatasetSpec struct {
	Kind string `json:"kind"`

	// categorySeries
	Labels []string  `json:"labels,omitempty"`
	Values []float64 `json:"values,omitempty"`

	// table
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`

	// future:
	// sparkSeries: Values []
	// timeSeries: series points...
}
