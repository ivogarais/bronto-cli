package datasource

import "context"

type Kind string

const (
	KindBar   Kind = "bar"
	KindTable Kind = "table"
)

type Query struct {
	ID   string
	Kind Kind
	// Tool/Args are future (MCP). Kept now so your spec can match final design.
	Tool string
	Args map[string]any
}

type BarResult struct {
	Labels []string
	Values []float64
}

type TableResult struct {
	Columns []string
	Rows    [][]string
}

type Result struct {
	Kind  Kind
	Bar   *BarResult
	Table *TableResult
}

type DataSource interface {
	RunQuery(ctx context.Context, q Query) (Result, error)
}
