package spec

import (
	"encoding/json"
	"fmt"
	"os"
)

type Refresh struct {
	EveryMs int `json:"everyMs"`
}

type Query struct {
	Kind string         `json:"kind"` // "bar" or "table"
	Tool string         `json:"tool,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type Widget struct {
	ID       string `json:"id"`
	Type     string `json:"type"`     // "barchart" or "table"
	Title    string `json:"title"`    // panel title
	QueryRef string `json:"queryRef"` // required now

	// Table-specific (optional; may be overridden by query results later)
	Columns []string `json:"columns,omitempty"`
}

type DashboardSpec struct {
	Version string           `json:"version"`
	Title   string           `json:"title"`
	Refresh Refresh          `json:"refresh"`
	Queries map[string]Query `json:"queries"`
	Widgets []Widget         `json:"widgets"`
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
	if len(s.Queries) == 0 {
		return fmt.Errorf("spec invalid: must include non-empty \"queries\"")
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

		if w.QueryRef == "" {
			return fmt.Errorf("spec invalid: widget %q missing required field \"queryRef\"", w.ID)
		}
		q, ok := s.Queries[w.QueryRef]
		if !ok {
			return fmt.Errorf("spec invalid: widget %q references missing query %q", w.ID, w.QueryRef)
		}

		// Type-kind consistency
		switch w.Type {
		case "barchart":
			if q.Kind != "bar" {
				return fmt.Errorf("spec invalid: widget %q is barchart but query %q kind is %q", w.ID, w.QueryRef, q.Kind)
			}
		case "table":
			if q.Kind != "table" {
				return fmt.Errorf("spec invalid: widget %q is table but query %q kind is %q", w.ID, w.QueryRef, q.Kind)
			}
		}
	}

	// Validate query kinds
	for id, q := range s.Queries {
		switch q.Kind {
		case "bar", "table":
		default:
			return fmt.Errorf("spec invalid: query %q has unsupported kind %q", id, q.Kind)
		}
	}

	return nil
}
