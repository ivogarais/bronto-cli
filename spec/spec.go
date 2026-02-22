package spec

import (
	"encoding/json"
	"fmt"
	"os"
)

type Refresh struct {
	EveryMs int `json:"everyMs"`
}

type DashboardSpec struct {
	Version string  `json:"version"`
	Title   string  `json:"title"`
	Refresh Refresh `json:"refresh"`
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
	// refresh.everyMs is optional; if missing or 0 we'll use a default in the TUI.
	return nil
}
