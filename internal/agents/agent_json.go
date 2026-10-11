package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (a *BaseAgent) repairJSONResponse(ctx context.Context, malformed string, output interface{}, parseErr error) error {
	if strings.TrimSpace(malformed) == "" {
		return fmt.Errorf("cannot repair empty AI response")
	}

	repairOptions := *a.chatOptions()
	repairOptions.Temperature = 0.1

	schema := a.buildOutputRequirements(output)
	systemPrompt := `You are a JSON repair tool.
Return ONLY valid JSON.
Preserve the original semantic content and field values as much as possible.
Do not add markdown fences, explanations, comments, or new creative content.
The repaired JSON must match the requested structure.`

	userPrompt := fmt.Sprintf("The previous response failed JSON parsing.\n\nParse error:\n%s\n\n%s\n\nMalformed response:\n%s",
		parseErr.Error(), schema, malformed)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	logger.Info("[%s] Sending malformed JSON to repair pass...", a.name)
	repairStartedAt := time.Now()
	resp, err := a.invokeAI(ctx, InvokeParams{Command: "repair malformed JSON"}, messages, &repairOptions, schema, output)
	if err != nil {
		a.recordUsage(InvokeParams{Command: "repair malformed JSON"}, "json-repair", "", llm.Usage{}, time.Since(repairStartedAt), err)
		return fmt.Errorf("JSON repair request failed: %w", err)
	}
	a.recordUsage(InvokeParams{Command: "repair malformed JSON"}, "json-repair", resp.Model, resp.Usage, time.Since(repairStartedAt), nil)
	logger.Info("[%s] JSON repair response received (%d tokens used)", a.name, resp.Usage.TotalTokens)
	if err := a.saveResponseToFile(a.name+"_JSONRepair", resp.Content); err != nil {
		logger.Debug("[%s] Failed to save JSON repair response: %v", a.name, err)
	}

	if err := a.parseResponse(resp.Content, output); err != nil {
		return fmt.Errorf("repaired response still invalid: %w", err)
	}
	return nil
}

// parseResponse parses the AI response into the output struct
func (a *BaseAgent) parseResponse(content string, output interface{}) error {
	// Check for empty content
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("AI response is empty")
	}

	// Check for common error patterns in AI response
	contentPreview := clipResponsePreview(content, 200)
	logger.Debug("[%s] Parsing response (length: %d), preview: %s", a.name, len(content), contentPreview)

	// Try to parse as JSON directly
	if err := json.Unmarshal([]byte(content), output); err != nil {
		if repaired, repairErr := tryParseDeterministicallyRepairedJSON(content, output); repairErr == nil {
			logger.Warn("[%s] JSON parse failed, recovered via deterministic JSON type repair: %s", a.name, repaired)
			return nil
		}

		// Try to extract JSON from markdown code block
		jsonContent := extractJSONFromMarkdown(content)
		logger.Debug("[%s] Extracted JSON from markdown (length: %d)", a.name, len(jsonContent))

		if strings.TrimSpace(jsonContent) == "" {
			// Log the problematic response for debugging
			logger.Error("[%s] Extracted JSON content is empty. Raw response preview: %s", a.name, contentPreview)
			return fmt.Errorf("extracted JSON content is empty - AI may have returned non-JSON content\nOriginal response length: %d", len(content))
		}

		if err := json.Unmarshal([]byte(jsonContent), output); err != nil {
			if a.trySetDSLContentFromMalformed(content, output) || a.trySetDSLContentFromMalformed(jsonContent, output) {
				logger.Warn("[%s] JSON parse failed, recovered DSL content via malformed-JSON fallback", a.name)
				return nil
			}
			if repaired, repairErr := tryParseDeterministicallyRepairedJSON(jsonContent, output); repairErr == nil {
				logger.Warn("[%s] JSON parse failed, recovered via deterministic JSON type repair: %s", a.name, repaired)
				return nil
			}

			logger.Error("[%s] Failed to parse AI response as JSON: %v", a.name, err)
			// Log more context about the error
			errorPreview := clipResponsePreview(jsonContent, 500)
			logger.Error("[%s] JSON content that failed to parse: %s", a.name, errorPreview)
			return fmt.Errorf("failed to parse AI response as JSON: %w\nResponse length: %d, JSON extraction length: %d", err, len(content), len(jsonContent))
		}
	}
	return nil
}

func clipResponsePreview(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func tryParseDeterministicallyRepairedJSON(content string, output interface{}) (string, error) {
	var data interface{}
	quoteChanges := 0
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		repairedQuotes, changes := escapeLikelyUnescapedStringQuotes(content)
		if changes == 0 {
			return "", err
		}
		if quoteErr := json.Unmarshal([]byte(repairedQuotes), &data); quoteErr != nil {
			return "", err
		}
		quoteChanges = changes
	}
	numericChanges := repairJSONNumericStrings(data)
	if quoteChanges == 0 && numericChanges == 0 {
		return "", fmt.Errorf("no deterministic JSON type repairs available")
	}
	repaired, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(repaired, output); err != nil {
		return "", err
	}
	parts := []string{}
	if quoteChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d unescaped quote(s)", quoteChanges))
	}
	if numericChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d numeric string field(s)", numericChanges))
	}
	return strings.Join(parts, ", "), nil
}

func escapeLikelyUnescapedStringQuotes(content string) (string, int) {
	var b strings.Builder
	b.Grow(len(content))
	inString := false
	escaped := false
	changes := 0
	for i, r := range content {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inString {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if r != '"' {
			b.WriteRune(r)
			continue
		}
		if !inString {
			inString = true
			b.WriteRune(r)
			continue
		}
		if isLikelyJSONStringTerminator(content[i+1:]) {
			inString = false
			b.WriteRune(r)
			continue
		}
		b.WriteString(`\"`)
		changes++
	}
	return b.String(), changes
}

func isLikelyJSONStringTerminator(tail string) bool {
	for _, r := range tail {
		if unicode.IsSpace(r) {
			continue
		}
		switch r {
		case ':', ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}

func repairJSONNumericStrings(value interface{}) int {
	switch typed := value.(type) {
	case map[string]interface{}:
		changes := 0
		for key, child := range typed {
			if raw, ok := child.(string); ok && shouldRepairIntegerJSONField(key) {
				if n, ok := parseStrictJSONInt(raw); ok {
					typed[key] = float64(n)
					changes++
					continue
				}
			}
			if raw, ok := child.(float64); ok && shouldRepairIntegerJSONField(key) && raw != math.Trunc(raw) {
				typed[key] = float64(int64(math.Round(raw)))
				changes++
				continue
			}
			changes += repairJSONNumericStrings(child)
		}
		changes += repairResourceLedgerArithmetic(typed)
		return changes
	case []interface{}:
		changes := 0
		for _, child := range typed {
			changes += repairJSONNumericStrings(child)
		}
		return changes
	default:
		return 0
	}
}

func shouldRepairIntegerJSONField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "count", "level", "start", "delta", "end":
		return true
	default:
		return false
	}
}

func repairResourceLedgerArithmetic(value map[string]interface{}) int {
	if _, hasItem := value["item"]; !hasItem {
		return 0
	}
	start, okStart := jsonNumberToInt(value["start"])
	delta, okDelta := jsonNumberToInt(value["delta"])
	end, okEnd := jsonNumberToInt(value["end"])
	if !okStart || !okDelta || !okEnd {
		return 0
	}
	expected := start + delta
	if end == expected {
		return 0
	}
	value["end"] = float64(expected)
	return 1
}

func jsonNumberToInt(value interface{}) (int64, bool) {
	n, ok := value.(float64)
	if !ok || n != math.Trunc(n) {
		return 0, false
	}
	return int64(n), true
}

func parseStrictJSONInt(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (a *BaseAgent) trySetDSLContentFromMalformed(content string, output interface{}) bool {
	dslContent, ok := extractDSLContentFromMalformed(content)
	if !ok {
		return false
	}

	v := reflect.ValueOf(output)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return false
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}

	field := elem.FieldByName("DSLContent")
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.String {
		return false
	}
	field.SetString(dslContent)
	return true
}

func extractDSLContentFromMalformed(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}

	// Direct DSL fallback
	if isLikelyDSL(trimmed) {
		return trimmed, true
	}

	keyIdx := strings.Index(trimmed, "\"dsl_content\"")
	if keyIdx == -1 {
		return "", false
	}

	body := trimmed[keyIdx:]
	start := findFirstDSLBlockStart(body)
	if start == -1 {
		return "", false
	}

	candidate := strings.TrimSpace(body[start:])
	candidate = strings.TrimSuffix(candidate, "```")
	candidate = strings.TrimSpace(candidate)

	// Remove common loose-JSON tails
	for _, suffix := range []string{"\"}", "\"\n}", "\"\r\n}", "\",\n}", "\",\r\n}", "\""} {
		if strings.HasSuffix(candidate, suffix) {
			candidate = strings.TrimSpace(strings.TrimSuffix(candidate, suffix))
		}
	}

	if !isLikelyDSL(candidate) {
		return "", false
	}
	return candidate, true
}

func findFirstDSLBlockStart(content string) int {
	keywords := []string{"metadata {", "world {", "characters {", "storyline {"}
	best := -1
	for _, kw := range keywords {
		if idx := strings.Index(content, kw); idx != -1 && (best == -1 || idx < best) {
			best = idx
		}
	}
	return best
}

func isLikelyDSL(content string) bool {
	required := []string{"{", "}"}
	for _, token := range required {
		if !strings.Contains(content, token) {
			return false
		}
	}
	// At least one top-level DSL block keyword
	return strings.Contains(content, "metadata {") ||
		strings.Contains(content, "world {") ||
		strings.Contains(content, "characters {") ||
		strings.Contains(content, "storyline {")
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	// Look for ```json ... ``` or ``` ... ``` blocks
	startIdx := strings.Index(content, "```json")
	if startIdx == -1 {
		startIdx = strings.Index(content, "```")
	}
	if startIdx != -1 {
		// Find the end of the opening marker
		codeStart := strings.Index(content[startIdx:], "\n")
		if codeStart != -1 {
			codeStart += startIdx + 1

			// Find the closing marker
			endIdx := strings.Index(content[codeStart:], "```")
			if endIdx != -1 {
				return strings.TrimSpace(content[codeStart : codeStart+endIdx])
			}
			return strings.TrimSpace(content[codeStart:])
		}
	}

	// If no code block found, try to find JSON boundaries
	// Look for the first '{' and the last '}'
	startIdx = strings.Index(content, "{")
	if startIdx == -1 {
		return content
	}

	// Find the matching closing brace
	braceCount := 0
	endIdx := -1
	for i := startIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				endIdx = i
				break
			}
		}
		if braceCount == 0 && endIdx != -1 {
			break
		}
	}

	if endIdx == -1 {
		return content[startIdx:]
	}

	return strings.TrimSpace(content[startIdx : endIdx+1])
}
