package utils

import "strings"

// FirstNonEmpty returns the first value that is non-blank after trimming, with
// surrounding whitespace removed. It returns "" when every value is blank.
//
// Stages used to carry private copies of this helper in several packages; the
// shared version keeps the trimming behavior identical everywhere.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// MaxInt returns the larger of a and b.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CompactStrings trims each value, drops blanks and duplicates (keeping the
// first occurrence), and returns the result in input order. It always returns a
// new slice so callers never observe their input being rewritten in place.
func CompactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
