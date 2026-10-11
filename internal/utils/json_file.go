package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveJSONMerging writes data as indented JSON to path. If path already holds a
// JSON object, its top-level keys are merged with the new data (new keys win),
// so callers can update a subset of fields without dropping the rest. A UTF-8
// BOM on the existing file is stripped before parsing.
func SaveJSONMerging(path string, data interface{}) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	existing := make(map[string]interface{})
	if fileData, err := os.ReadFile(path); err == nil {
		fileData = StripUTF8BOM(fileData)
		if err := json.Unmarshal(fileData, &existing); err != nil {
			return fmt.Errorf("failed to parse existing data: %w", err)
		}
	}

	newData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal new data: %w", err)
	}
	var newMap map[string]interface{}
	if err := json.Unmarshal(newData, &newMap); err != nil {
		return fmt.Errorf("failed to parse new data: %w", err)
	}
	for k, v := range newMap {
		existing[k] = v
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return os.WriteFile(path, output, 0644)
}
