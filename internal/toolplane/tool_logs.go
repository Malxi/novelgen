package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/envutil"
	"novelgen/internal/store"
	"novelgen/internal/toolcmd"
	"novelgen/internal/utils"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// QueryLogs is the string-root entry point for callers that hold a resolved
// project directory.
func QueryLogs(rootDir, queryType, id, name string, includeContent bool) ToolResponse {
	return queryLogsAt(store.At(rootDir), queryType, id, name, includeContent)
}

func queryLogsAt(root *store.Root, queryType, id, name string, includeContent bool) ToolResponse {
	logsRoot := root.LogsDir()
	if !root.IsDir(logsRoot) {
		return ToolResponse{OK: true, Section: "logs", Count: 0, Results: nil, Warnings: []string{"logs directory not found"}}
	}
	kindFilter := NormalizeLogKind(queryType)
	id = filepath.ToSlash(strings.TrimSpace(id))
	name = strings.ToLower(strings.TrimSpace(name))
	historyCutoff, hasHistoryCutoff := logHistoryCutoff()

	hits := []LogHit{}
	err := root.Walk(logsRoot, func(path string, info os.FileInfo) error {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(logsRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		kind := logKindFromRel(rel)
		if kindFilter != "" && kind != kindFilter {
			return nil
		}
		if id == "" && kindFilter == "" && kind != "prompts" && kind != "responses" && kind != "agent-live" {
			return nil
		}
		if id == "" && hasHistoryCutoff && info.ModTime().After(historyCutoff) {
			return nil
		}
		if id != "" && !strings.EqualFold(rel, id) {
			return nil
		}
		agent := logAgentFromFilename(filepath.Base(path))
		searchHaystack := strings.ToLower(rel + " " + agent)
		if name != "" && !strings.Contains(searchHaystack, name) {
			return nil
		}
		summary := map[string]interface{}(nil)
		if kind == "agent-live" {
			summary = summarizeAgentLiveLogFile(root.FS(), path)
			if id == "" && !agentLiveLogSummaryComplete(summary) {
				return nil
			}
		}
		hit := LogHit{
			ID:          rel,
			Kind:        kind,
			Agent:       agent,
			Path:        rel,
			SizeBytes:   info.Size(),
			ModifiedAt:  info.ModTime(),
			DetailQuery: toolcmd.QueryLogs(toolcmd.QueryOptions{ID: rel, View: "brief"}),
		}
		if kind == "agent-live" {
			hit.Summary = summary
		}
		if includeContent && id != "" {
			hit.Content, hit.Truncated = readLogExcerptForQuery(root.FS(), path, kind, 12000)
		} else {
			hit.Preview, hit.Truncated = readLogPreviewForQuery(root.FS(), path, kind, 1000)
		}
		hits = append(hits, hit)
		return nil
	})
	if err != nil {
		return ToolResponse{OK: false, Section: "logs", Count: 0, Warnings: []string{err.Error()}}
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].ModifiedAt.After(hits[j].ModifiedAt)
	})
	meta := map[string]interface{}{
		"content_policy": "content is returned only when --content is used with an exact --id; excerpts are capped",
		"next_queries": []string{
			"novelgen tool query logs --view index",
			"novelgen tool query logs --type agent-live --view index",
			"novelgen tool query logs --type prompts --name <agent> --view brief --limit 5",
			"novelgen tool query logs --id <relative_log_path> --content --view brief",
		},
	}
	warnings := []string{}
	if len(hits) == 0 {
		if id != "" {
			warnings = append(warnings, fmt.Sprintf("log %q not found", id))
		} else if kindFilter != "" {
			warnings = append(warnings, fmt.Sprintf("no logs matched type %q", kindFilter))
		} else if name != "" {
			warnings = append(warnings, "no logs matched the name filter")
		}
	}
	return ToolResponse{OK: true, Section: "logs", Count: len(hits), Results: hits, Warnings: warnings, Meta: meta}
}

func logHistoryCutoff() (time.Time, bool) {
	return envutil.TimestampRFC3339("NOVELGEN_LOG_HISTORY_CUTOFF")
}

func agentLiveLogSummaryComplete(summary map[string]interface{}) bool {
	return summaryInt(summary, "final_records") > 0
}

func summaryInt(summary map[string]interface{}, key string) int {
	if summary == nil {
		return 0
	}
	switch value := summary[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func NormalizeLogKind(value string) string {
	switch NormalizeKey(value) {
	case "", "all":
		return ""
	case "prompt", "prompts":
		return "prompts"
	case "response", "responses":
		return "responses"
	case "agent-live", "agent_live", "live", "livelog", "live-log":
		return "agent-live"
	default:
		return NormalizeKey(value)
	}
}

func logKindFromRel(rel string) string {
	first, _, ok := strings.Cut(filepath.ToSlash(rel), "/")
	if !ok {
		return "other"
	}
	return NormalizeLogKind(first)
}

func logAgentFromFilename(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, marker := range []string{"_20", "-20"} {
		if idx := strings.Index(base, marker); idx > 0 {
			return base[:idx]
		}
	}
	if idx := strings.Index(base, "_"); idx > 0 {
		return base[:idx]
	}
	return base
}

func readLogPreviewForQuery(fs store.FS, path string, kind string, maxRunes int) (string, bool) {
	content, truncated := readLogExcerptForQuery(fs, path, kind, maxRunes)
	return logPreviewFromExcerpt(content, truncated)
}

func logPreviewFromExcerpt(content string, truncated bool) (string, bool) {
	if content == "" {
		return "", truncated
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	kept := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isLogPreviewMetadataLine(line) {
			continue
		}
		kept = append(kept, line)
		if len(kept) == 3 {
			break
		}
	}
	return strings.Join(kept, "\n"), truncated
}

func isLogPreviewMetadataLine(line string) bool {
	normalized := strings.ToLower(strings.TrimSpace(line))
	if normalized == "" {
		return true
	}
	if normalized == "---" || normalized == "{" || normalized == "[" {
		return true
	}
	if strings.HasPrefix(normalized, "```") {
		return true
	}
	for _, prefix := range []string{
		"# agent:",
		"# time:",
		"# ai response",
		"# system prompt",
		"# user prompt",
		"# response",
		"# prompt",
	} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func readLogExcerptForQuery(fs store.FS, path string, kind string, maxRunes int) (string, bool) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return "", false
	}
	text := string(utils.StripUTF8BOM(data))
	if NormalizeLogKind(kind) == "agent-live" {
		text = sanitizeAgentLiveLogText(text)
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text, false
	}
	return string(runes[:maxRunes]), true
}

func sanitizeAgentLiveLogText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = sanitizeAgentLiveLogLine(line)
	}
	return strings.Join(lines, "\n")
}

func summarizeAgentLiveLogFile(fs store.FS, path string) map[string]interface{} {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil
	}
	return summarizeAgentLiveLogText(string(utils.StripUTF8BOM(data)))
}

func summarizeAgentLiveLogText(text string) map[string]interface{} {
	summary := map[string]interface{}{}
	allowedCommands := []string{}
	deniedCommands := []string{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		incrementSummaryInt(summary, "events")
		event := strings.TrimSpace(fmt.Sprint(record["event"]))
		switch event {
		case "start":
			if model := liveLogRecordString(record, "model"); model != "" {
				summary["model"] = model
			}
			addSummaryStringSlice(summary, "sdk_skills", liveLogRecordStringSlice(record, "sdk_skills"))
			addSummaryStringSlice(summary, "loaded_sdk_skills", liveLogRecordStringSlice(record, "loaded_sdk_skills"))
			addSummaryStringSlice(summary, "missing_sdk_skills", liveLogRecordStringSlice(record, "missing_sdk_skills"))
		case "message":
			incrementSummaryInt(summary, "messages")
		case "final":
			incrementSummaryInt(summary, "final_records")
			if model := liveLogRecordString(record, "model"); model != "" {
				summary["final_model"] = model
			}
		case "tool_hook", "tool_permission":
			hook := strings.TrimSpace(fmt.Sprint(record["hook"]))
			if hook != "PreToolUse" && event != "tool_permission" {
				continue
			}
			command := summarizeAgentLiveToolCommand(liveLogRecordString(record, "command"))
			if command == "" {
				continue
			}
			if allowed, ok := record["allowed"].(bool); ok {
				incrementSummaryInt(summary, "tool_calls")
				if allowed {
					incrementSummaryInt(summary, "tool_allowed")
					countAgentLiveSummaryCommand(summary, command)
					appendUniqueSummaryCommand(&allowedCommands, command, 12)
				} else {
					incrementSummaryInt(summary, "tool_denied")
					appendUniqueSummaryCommand(&deniedCommands, command, 8)
				}
			}
		}
	}
	if _, ok := summary["final_model"]; !ok {
		if model, ok := summary["model"]; ok {
			summary["final_model"] = model
		}
	}
	if len(allowedCommands) > 0 {
		summary["allowed_tool_commands"] = allowedCommands
	}
	if len(deniedCommands) > 0 {
		summary["denied_tool_commands"] = deniedCommands
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func incrementSummaryInt(summary map[string]interface{}, key string) {
	current, _ := summary[key].(int)
	summary[key] = current + 1
}

func addSummaryStringSlice(summary map[string]interface{}, key string, values []string) {
	if len(values) == 0 {
		return
	}
	summary[key] = values
}

func liveLogRecordString(record map[string]interface{}, key string) string {
	if record == nil {
		return ""
	}
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func liveLogRecordStringSlice(record map[string]interface{}, key string) []string {
	value, ok := record[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func countAgentLiveSummaryCommand(summary map[string]interface{}, command string) {
	normalized := strings.ToLower(command)
	switch {
	case strings.Contains(normalized, " tool query "):
		incrementSummaryInt(summary, "query_calls")
	case strings.Contains(normalized, " tool check "):
		incrementSummaryInt(summary, "check_calls")
	case strings.Contains(normalized, " tool refresh "):
		incrementSummaryInt(summary, "refresh_calls")
	case strings.Contains(normalized, " tool patch "), strings.Contains(normalized, " tool patch-buffer "):
		incrementSummaryInt(summary, "patch_calls")
		if strings.Contains(normalized, " --apply") {
			incrementSummaryInt(summary, "patch_applies")
		}
	}
}

func appendUniqueSummaryCommand(commands *[]string, command string, max int) {
	if command == "" || len(*commands) >= max {
		return
	}
	for _, existing := range *commands {
		if existing == command {
			return
		}
	}
	*commands = append(*commands, command)
}

func sanitizeAgentLiveLogLine(line string) string {
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(line), &record); err == nil {
		if command, ok := record["command"].(string); ok {
			record["command"] = summarizeAgentLiveToolCommand(command)
			if encoded, err := json.Marshal(record); err == nil {
				return string(encoded)
			}
		}
		return line
	}
	return sanitizeAgentLiveRawCommandText(line)
}

func sanitizeAgentLiveRawCommandText(line string) string {
	lower := strings.ToLower(line)
	idx := novelgenToolCommandIndexForQuery(lower)
	if idx < 0 {
		return line
	}
	command := summarizeAgentLiveToolCommand(line[idx:])
	if command == "" {
		return "<command>"
	}
	return command
}

func summarizeAgentLiveToolCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	lower := strings.ToLower(command)
	if isClaudeTempOutputReadCommand(lower) {
		return "powershell Get-Content <claude-temp-tool-output>"
	}
	if idx := novelgenToolCommandIndexForQuery(lower); idx >= 0 {
		command = strings.TrimSpace(command[idx:])
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--patch-json"); idx >= 0 {
		suffix := ""
		if strings.Contains(lower[idx:], " --apply") {
			suffix = " --apply"
		}
		command = strings.TrimSpace(command[:idx]) + " --patch-json <json>" + suffix
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--text"); idx >= 0 && strings.Contains(lower, " tool patch-buffer ") {
		command = strings.TrimSpace(command[:idx]) + " --text <text>"
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--stdin"); idx >= 0 && strings.Contains(lower, " tool patch-buffer ") {
		command = strings.TrimSpace(command[:idx]) + " --stdin <stdin>"
	}
	return clipToolString(command, 240)
}

func isClaudeTempOutputReadCommand(lowerCommand string) bool {
	return strings.Contains(lowerCommand, "get-content") &&
		strings.Contains(lowerCommand, `\temp\claude\`) &&
		strings.Contains(lowerCommand, `\tasks\`) &&
		strings.Contains(lowerCommand, ".output")
}

func novelgenToolCommandIndexForQuery(lowerCommand string) int {
	best := -1
	for _, marker := range []string{
		"novelgen tool",
		"novelgen.exe tool",
		"novelgen.exe' tool",
		"novelgen.exe\" tool",
	} {
		if idx := strings.Index(lowerCommand, marker); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func briefLogResults(results interface{}, indexOnly bool) interface{} {
	hits, ok := results.([]LogHit)
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		item := map[string]interface{}{
			"id":           hit.ID,
			"kind":         hit.Kind,
			"agent":        hit.Agent,
			"size_bytes":   hit.SizeBytes,
			"modified_at":  hit.ModifiedAt,
			"detail_query": hit.DetailQuery,
		}
		if len(hit.Summary) > 0 {
			item["summary"] = hit.Summary
		}
		if !indexOnly {
			if hit.Preview != "" {
				item["preview"] = clipToolString(hit.Preview, 1000)
			}
			if hit.Content != "" {
				item["content"] = hit.Content
			}
			if hit.Truncated {
				item["truncated"] = true
			}
		}
		out = append(out, item)
	}
	return out
}
