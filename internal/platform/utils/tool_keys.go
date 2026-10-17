package utils

import (
	"fmt"
	"strings"
)

// NormalizeKey canonicalizes an agent-facing section, target or category key.
// Tool queries, checks and mutations share this spelling rule so a lookup cannot
// succeed in one surface and fail in another.
func NormalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// StringMapValue returns a string value from a generic navigation map, trimmed
// for command use. Non-string values are rendered with fmt.Sprint, matching the
// historical tool-plane behavior for JSON-decoded numbers and booleans.
func StringMapValue(value map[string]interface{}, key string) string {
	if value == nil {
		return ""
	}
	raw, ok := value[key]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}
