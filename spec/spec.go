package spec

import (
	"encoding/json"
	"fmt"
	"os"
)

type Refresh struct {
	EveryMs int `json:"everyMs"`
}

type Widget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`  // "barchart" or "table" (for now)
	Title string `json:"title"` // panel title
	// Table-specific (optional)
	Columns []string `json:"columns,omitempty"`
}

type DashboardSpec struct {
	Version string   `json:"version"`
	Title   string   `json:"title"`
	Refresh Refresh  `json:"refresh"`
	Widgets []Widget `json:"widgets"`
}

func Load(path string) (*DashboardSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}

	var s DashboardSpec
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse spec JSON: %w", err)
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s DashboardSpec) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("spec invalid: missing required field \"version\"")
	}
	if s.Title == "" {
		return fmt.Errorf("spec invalid: missing required field \"title\"")
	}
	if len(s.Widgets) == 0 {
		return fmt.Errorf("spec invalid: must include at least one widget in \"widgets\"")
	}

	seen := map[string]bool{}
	for i, w := range s.Widgets {
		if w.ID == "" {
			return fmt.Errorf("spec invalid: widgets[%d] missing required field \"id\"", i)
		}
		if seen[w.ID] {
			return fmt.Errorf("spec invalid: duplicate widget id %q", w.ID)
		}
		seen[w.ID] = true

		if w.Type == "" {
			return fmt.Errorf("spec invalid: widgets[%d] missing required field \"type\"", i)
		}
		switch w.Type {
		case "barchart", "table":
			// ok
		default:
			return fmt.Errorf("spec invalid: widgets[%d] has unsupported type %q", i, w.Type)
		}

		if w.Title == "" {
			return fmt.Errorf("spec invalid: widgets[%d] missing required field \"title\"", i)
		}

		if w.Type == "table" && len(w.Columns) == 0 {
			return fmt.Errorf("spec invalid: table widget %q must include non-empty \"columns\"", w.ID)
		}
	}

	return nil
}
