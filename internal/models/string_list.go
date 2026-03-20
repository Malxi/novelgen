package models

import (
	"encoding/json"
)

// StringList unmarshals from either a JSON string or a JSON array of strings.
// This makes model outputs more tolerant (some prompts return secrets as a string,
// others as a list).
type StringList []string

func (sl *StringList) UnmarshalJSON(b []byte) error {
	// Try []string first
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		*sl = arr
		return nil
	}

	// Then try string
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" {
			*sl = nil
			return nil
		}
		*sl = []string{s}
		return nil
	}

	// Fallback: leave error from []string attempt (more informative)
	return json.Unmarshal(b, &arr)
}
