package tui

import "github.com/ivogarais/bronto-cli/spec"

const (
	maxPanelsPerRow = 3

	preferredChartPanelHeight = 14
	preferredTablePanelHeight = 12
	preferredHeaderHeight     = 4

	minChartPanelWidth  = 24
	minChartPanelHeight = 8
	minTablePanelWidth  = 30
	minTablePanelHeight = 6

	minAnyWidth   = 6
	minAutoWidth  = 8
	minFixedWidth = 6
	minFlexWidth  = 12
)

type rowGroup struct {
	Children []spec.Node
	Weights  []int
}

type focusPanel struct {
	Number int
	Node   spec.Node
}

type colType int

const (
	colDefault colType = iota
	colTimestamp
	colLevel
	colService
	colID
	colMessage
	colNumeric
)
