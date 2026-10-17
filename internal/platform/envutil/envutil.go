// Package envutil centralizes reading and parsing of Novelgen's `NOVELGEN_*`
// environment variables.
//
// Keeping the parsing in one place means every tuning knob behaves the same
// way (trimmed, validated, predictable fallback) and the full set of knobs is
// easy to audit. Callers keep passing the same variable names so existing
// deployments do not change.
package envutil

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the trimmed value of key, or "" when unset or blank.
func String(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// StringDefault returns the trimmed value of key, or fallback when unset/blank.
func StringDefault(key, fallback string) string {
	if value := String(key); value != "" {
		return value
	}
	return fallback
}

// Bool reports the boolean value of key. Recognized true values are
// 1/true/yes/on and false values are 0/false/no/off (case-insensitive).
// Unset or unrecognized values return fallback.
func Bool(key string, fallback bool) bool {
	switch strings.ToLower(String(key)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// PositiveInt returns the parsed integer value of key when it parses and is
// greater than zero, otherwise fallback.
func PositiveInt(key string, fallback int) int {
	raw := String(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// PositiveIntMilliseconds parses key as a positive integer number of
// milliseconds into a time.Duration, returning fallback when unset or invalid.
func PositiveIntMilliseconds(key string, fallback time.Duration) time.Duration {
	raw := String(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}

// TimestampRFC3339 parses key as an RFC3339 timestamp (nanosecond precision
// first). ok is false when the variable is unset or unparsable.
func TimestampRFC3339(key string) (time.Time, bool) {
	raw := String(key)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}
