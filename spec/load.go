package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func LoadStrict(path string) (*AppSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()

	var s AppSpec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse spec JSON (strict): %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse spec JSON (strict): trailing JSON data")
		}
		return nil, fmt.Errorf("parse spec JSON (strict): %w", err)
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}
