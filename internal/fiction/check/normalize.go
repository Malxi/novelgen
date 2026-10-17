package check

import "novelgen/internal/platform/utils"

// NormalizeKey is the check-local spelling used by the moved check code. The
// canonical implementation lives in platform/utils so queries, checks and
// mutations cannot drift.
func NormalizeKey(value string) string {
	return utils.NormalizeKey(value)
}

// StringMapValue reads a navigation/command value from a generic map.
func StringMapValue(value map[string]interface{}, key string) string {
	return utils.StringMapValue(value, key)
}
