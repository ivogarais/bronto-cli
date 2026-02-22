package spec

import (
	"fmt"
)

func (s *AppSpec) Validate() error {
	if s.Version != "bronto-tui/v1" {
		return fmt.Errorf("spec invalid: version must be %q", "bronto-tui/v1")
	}
	if s.Title == "" {
		return fmt.Errorf("spec invalid: missing title")
	}

	// Theme
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

	// Require maps
	if len(s.Datasets) == 0 {
		return fmt.Errorf("spec invalid: datasets must be non-empty")
	}
	if len(s.Charts) == 0 && len(s.Tables) == 0 {
		return fmt.Errorf("spec invalid: at least one of charts or tables must be non-empty")
	}

	// Validate datasets
	for id, d := range s.Datasets {
		if id == "" {
			return fmt.Errorf("spec invalid: dataset id cannot be empty")
		}
		if err := validateDataset(id, d); err != nil {
			return err
		}
	}

	// Validate charts
	for id, c := range s.Charts {
		if id == "" {
			return fmt.Errorf("spec invalid: chart id cannot be empty")
		}
		normalized, err := validateChart(id, c, s.Datasets)
		if err != nil {
			return err
		}
		s.Charts[id] = normalized
	}

	// Validate tables
	for id, t := range s.Tables {
		if id == "" {
			return fmt.Errorf("spec invalid: table id cannot be empty")
		}
		normalized, err := validateTable(id, t, s.Datasets)
		if err != nil {
			return err
		}
		s.Tables[id] = normalized
	}

	// Validate layout tree references
	seen := map[string]bool{}
	if err := validateNode(s.Layout, s.Charts, s.Tables, seen); err != nil {
		return err
	}

	return nil
}

func validateDataset(id string, d DatasetSpec) error {
	switch d.Kind {
	case "categorySeries":
		if len(d.Labels) == 0 {
			return fmt.Errorf("spec invalid: dataset %q categorySeries labels must be non-empty", id)
		}
		if len(d.Values) == 0 {
			return fmt.Errorf("spec invalid: dataset %q categorySeries values must be non-empty", id)
		}
		if len(d.Labels) != len(d.Values) {
			return fmt.Errorf("spec invalid: dataset %q categorySeries labels/values length mismatch", id)
		}
	case "table":
		if len(d.Columns) == 0 {
			return fmt.Errorf("spec invalid: dataset %q table columns must be non-empty", id)
		}
		// rows can be empty (still render)
		for i, r := range d.Rows {
			if len(r) != len(d.Columns) {
				return fmt.Errorf("spec invalid: dataset %q row %d has %d cells; expected %d", id, i, len(r), len(d.Columns))
			}
		}
	default:
		return fmt.Errorf("spec invalid: dataset %q has unsupported kind %q", id, d.Kind)
	}
	return nil
}

func validateChart(id string, c ChartSpec, datasets map[string]DatasetSpec) (ChartSpec, error) {
	switch c.Family {
	case "bar":
		if c.Orientation == "" {
			c.Orientation = "horizontal"
		}
		if c.Orientation != "horizontal" && c.Orientation != "vertical" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q bar orientation must be horizontal|vertical", id)
		}
		if c.DatasetRef == "" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q missing datasetRef", id)
		}
		d, ok := datasets[c.DatasetRef]
		if !ok {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q references missing dataset %q", id, c.DatasetRef)
		}
		if d.Kind != "categorySeries" {
			return ChartSpec{}, fmt.Errorf("spec invalid: chart %q expects categorySeries dataset; got %q", id, d.Kind)
		}
	default:
		return ChartSpec{}, fmt.Errorf("spec invalid: chart %q unsupported family %q", id, c.Family)
	}
	return c, nil
}

func validateTable(id string, t TableSpec, datasets map[string]DatasetSpec) (TableSpec, error) {
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
	if t.RowLimit == 0 {
		t.RowLimit = 200
	}
	for i, c := range t.Columns {
		if c.Key == "" || c.Title == "" {
			return TableSpec{}, fmt.Errorf("spec invalid: table %q column[%d] missing key/title", id, i)
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
		if len(n.Weights) > 0 && len(n.Weights) != len(n.Children) {
			return fmt.Errorf("spec invalid: row %q weights length must equal children length", n.ID)
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
	default:
		return fmt.Errorf("spec invalid: unsupported layout node type %q", n.Type)
	}
	return nil
}
