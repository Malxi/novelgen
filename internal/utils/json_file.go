package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MergeTopLevelJSON merges value's top-level keys into the JSON object in
// existing (new keys win) and returns the indented result. A UTF-8 BOM is
// tolerated. The single merge rule is shared by internal/store (which writes the
// result atomically) and SaveJSONMerging (which writes to a caller-chosen path),
// so the two cannot drift.
func MergeTopLevelJSON(existing []byte, value any) ([]byte, error) {
	current := make(map[string]any)
	if len(existing) > 0 {
		if err := json.Unmarshal(StripUTF8BOM(existing), &current); err != nil {
			return nil, fmt.Errorf("failed to parse existing data: %w", err)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new data: %w", err)
	}
	var incoming map[string]any
	if err := json.Unmarshal(encoded, &incoming); err != nil {
		return nil, fmt.Errorf("failed to parse new data: %w", err)
	}
	for key, item := range incoming {
		current[key] = item
	}

	output, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	return output, nil
}

// SaveJSONMerging writes data as indented JSON to path, merging its top-level
// keys into whatever object already lives there (new keys win). It is for
// caller-chosen output paths outside the project layout; project artifacts go
// through internal/store, which writes atomically. A missing file starts from an
// empty object; a file that exists but cannot be read or parsed is an error.
func SaveJSONMerging(path string, data interface{}) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	var existing []byte
	if fileData, err := os.ReadFile(path); err == nil {
		existing = fileData
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing data: %w", err)
	}

	output, err := MergeTopLevelJSON(existing, data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, output, 0644)
}
