package utils

import "strings"

// SplitCommaList splits a comma-separated string, trimming surrounding
// whitespace and dropping empty entries. An empty input returns nil.
func SplitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
