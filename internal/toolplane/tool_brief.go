// The index/brief projection dispatch and the text/clip primitives every brief is built from.

package toolplane

import (
	"encoding/json"
	"fmt"
	"strings"
)

func briefToolResults(section string, results interface{}) interface{} {
	switch NormalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, false)
	case "outline":
		return briefOutlineResults(results, false)
	case "chapter", "chapters":
		return briefChapterResults(results, false)
	case "craft":
		return briefCraftResults(results, false)
	case "context":
		return briefContextResults(results, false)
	case "logs", "log":
		return briefLogResults(results, false)
	default:
		return results
	}
}

func indexToolResults(section string, results interface{}) interface{} {
	switch NormalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, true)
	case "outline":
		return briefOutlineResults(results, true)
	case "chapter", "chapters":
		return briefChapterResults(results, true)
	case "craft":
		return briefCraftResults(results, true)
	case "context":
		return briefContextResults(results, true)
	case "logs", "log":
		return briefLogResults(results, true)
	default:
		return results
	}
}

func clipToolStrings(values []string, maxItems, maxRunes int) []string {
	if len(values) == 0 {
		return nil
	}
	if maxItems > 0 && len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if clipped := clipToolString(value, maxRunes); clipped != "" {
			out = append(out, clipped)
		}
	}
	return out
}

func firstNStrings(values []string, maxItems int) []string {
	if maxItems > 0 && len(values) > maxItems {
		return values[:maxItems]
	}
	return values
}

func clipToolString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func clipToolTailString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return "..." + string(runes[len(runes)-maxRunes:])
}

func lastNonEmptyLine(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func applyToolLimit(resp *ToolResponse, limit int) {
	if resp == nil || limit <= 0 || resp.Results == nil {
		return
	}
	data, err := json.Marshal(resp.Results)
	if err != nil {
		return
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		resp.Warnings = append(resp.Warnings, "results are object-shaped; --limit only applies to list results, so it was ignored")
		return
	}
	if len(values) <= limit {
		return
	}
	total := len(values)
	resp.Results = values[:limit]
	resp.Count = limit
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["total_count"] = total
	resp.Warnings = append(resp.Warnings, fmt.Sprintf("results truncated from %d to --limit=%d", total, limit))
}
