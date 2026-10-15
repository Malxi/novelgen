package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type legacyAILog struct {
	Name  string
	Path  string
	Agent string
	Time  time.Time
	Size  int64
}

func listAICalls(c *gin.Context) {
	projectPath := projectPathFromQuery(c)

	calls := make([]AICallSummary, 0)
	structured, err := listStructuredAICalls(projectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	calls = append(calls, structured...)
	calls = append(calls, listLegacyAICalls(projectPath)...)

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].StartedAt.After(calls[j].StartedAt)
	})

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: calls})
}

func getAICall(c *gin.Context) {
	projectPath := projectPathFromQuery(c)

	id := c.Param("id")
	if strings.HasPrefix(id, "structured:") {
		detail, err := readStructuredAICall(projectPath, strings.TrimPrefix(id, "structured:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	if strings.HasPrefix(id, "legacy-response:") {
		detail, err := readLegacyResponseOnlyAICall(projectPath, strings.TrimPrefix(id, "legacy-response:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	if strings.HasPrefix(id, "legacy:") {
		detail, err := readLegacyAICall(projectPath, strings.TrimPrefix(id, "legacy:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid AI call id"})
}

func listStructuredAICalls(projectPath string) ([]AICallSummary, error) {
	dir := filepath.Join(projectPath, "logs", "ai_calls")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	calls := make([]AICallSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		detail, err := readStructuredAICall(projectPath, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		calls = append(calls, detail.AICallSummary)
	}
	return calls, nil
}

func readStructuredAICall(projectPath, id string) (*AICallDetail, error) {
	if !safeLogName(id) {
		return nil, fmt.Errorf("invalid AI call id")
	}
	path := filepath.Join(projectPath, "logs", "ai_calls", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry aiCallLogFile
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.ID == "" {
		entry.ID = id
	}

	inputChars := len(entry.SystemPrompt) + len(entry.UserPrompt)
	detail := &AICallDetail{
		AICallSummary: AICallSummary{
			ID:               "structured:" + id,
			Agent:            entry.Agent,
			Command:          entry.Command,
			Model:            entry.Model,
			StartedAt:        entry.StartedAt,
			HasInput:         strings.TrimSpace(entry.SystemPrompt) != "" || strings.TrimSpace(entry.UserPrompt) != "",
			HasOutput:        strings.TrimSpace(entry.Response) != "",
			InputChars:       inputChars,
			OutputChars:      len(entry.Response),
			PromptTokens:     entry.Usage.PromptTokens,
			CompletionTokens: entry.Usage.CompletionTokens,
			TotalTokens:      entry.Usage.TotalTokens,
			Legacy:           false,
		},
		Skills:       entry.Skills,
		SystemPrompt: entry.SystemPrompt,
		UserPrompt:   entry.UserPrompt,
		Response:     entry.Response,
	}
	if detail.StartedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			detail.StartedAt = info.ModTime()
		}
	}
	return detail, nil
}

func listLegacyAICalls(projectPath string) []AICallSummary {
	prompts := readLegacyAILogs(filepath.Join(projectPath, "logs", "prompts"), ".md")
	responses := readLegacyAILogs(filepath.Join(projectPath, "logs", "responses"), ".md")
	pairs := pairLegacyAILogs(prompts, responses)

	calls := make([]AICallSummary, 0, len(prompts)+len(responses))
	for _, prompt := range prompts {
		response := pairs[prompt.Name]
		summary := AICallSummary{
			ID:          "legacy:" + prompt.Name,
			Agent:       prompt.Agent,
			StartedAt:   prompt.Time,
			HasInput:    true,
			HasOutput:   response != nil,
			InputChars:  int(prompt.Size),
			OutputChars: 0,
			Legacy:      true,
		}
		if response != nil {
			summary.OutputChars = int(response.Size)
		}
		calls = append(calls, summary)
	}

	matchedResponses := map[string]bool{}
	for _, response := range pairs {
		if response != nil {
			matchedResponses[response.Name] = true
		}
	}
	for _, response := range responses {
		if matchedResponses[response.Name] {
			continue
		}
		calls = append(calls, AICallSummary{
			ID:          "legacy-response:" + response.Name,
			Agent:       response.Agent,
			StartedAt:   response.Time,
			HasInput:    false,
			HasOutput:   true,
			OutputChars: int(response.Size),
			Legacy:      true,
		})
	}

	return calls
}

func readLegacyAICall(projectPath, promptName string) (*AICallDetail, error) {
	if !safeLogName(promptName) || !strings.HasSuffix(promptName, ".md") {
		return nil, fmt.Errorf("invalid prompt log name")
	}
	promptPath := filepath.Join(projectPath, "logs", "prompts", promptName)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}

	promptLog, err := legacyAILogFromPath(promptPath)
	if err != nil {
		return nil, err
	}
	responses := readLegacyAILogs(filepath.Join(projectPath, "logs", "responses"), ".md")
	pairs := pairLegacyAILogs([]legacyAILog{promptLog}, responses)
	response := pairs[promptLog.Name]

	systemPrompt, userPrompt := splitPromptLog(string(promptData))
	detail := &AICallDetail{
		AICallSummary: AICallSummary{
			ID:          "legacy:" + promptLog.Name,
			Agent:       promptLog.Agent,
			StartedAt:   promptLog.Time,
			HasInput:    true,
			HasOutput:   response != nil,
			InputChars:  len(systemPrompt) + len(userPrompt),
			OutputChars: 0,
			Legacy:      true,
		},
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		PromptPath:   promptPath,
	}
	if response != nil {
		responseData, _ := os.ReadFile(response.Path)
		detail.Response = extractAIResponse(string(responseData))
		detail.ResponsePath = response.Path
		detail.OutputChars = len(detail.Response)
	}
	return detail, nil
}

func readLegacyResponseOnlyAICall(projectPath, responseName string) (*AICallDetail, error) {
	if !safeLogName(responseName) || !strings.HasSuffix(responseName, ".md") {
		return nil, fmt.Errorf("invalid response log name")
	}
	responsePath := filepath.Join(projectPath, "logs", "responses", responseName)
	responseData, err := os.ReadFile(responsePath)
	if err != nil {
		return nil, err
	}
	responseLog, err := legacyAILogFromPath(responsePath)
	if err != nil {
		return nil, err
	}
	response := extractAIResponse(string(responseData))
	return &AICallDetail{
		AICallSummary: AICallSummary{
			ID:          "legacy-response:" + responseLog.Name,
			Agent:       responseLog.Agent,
			StartedAt:   responseLog.Time,
			HasInput:    false,
			HasOutput:   true,
			OutputChars: len(response),
			Legacy:      true,
		},
		Response:     response,
		ResponsePath: responsePath,
	}, nil
}

func readLegacyAILogs(dir, suffix string) []legacyAILog {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	logs := make([]legacyAILog, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		log, err := legacyAILogFromPath(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})
	return logs
}

func legacyAILogFromPath(path string) (legacyAILog, error) {
	name := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return legacyAILog{}, err
	}
	agent, loggedAt := parseAILogFilename(name, info.ModTime())
	return legacyAILog{
		Name:  name,
		Path:  path,
		Agent: agent,
		Time:  loggedAt,
		Size:  info.Size(),
	}, nil
}

func pairLegacyAILogs(prompts, responses []legacyAILog) map[string]*legacyAILog {
	pairs := map[string]*legacyAILog{}
	used := map[string]bool{}
	for _, prompt := range prompts {
		var best *legacyAILog
		for i := range responses {
			response := responses[i]
			if used[response.Name] || response.Agent != prompt.Agent || response.Time.Before(prompt.Time) {
				continue
			}
			if response.Time.Sub(prompt.Time) > 2*time.Hour {
				continue
			}
			if best == nil || response.Time.Before(best.Time) {
				candidate := response
				best = &candidate
			}
		}
		if best != nil {
			used[best.Name] = true
		}
		pairs[prompt.Name] = best
	}
	return pairs
}

func parseAILogFilename(name string, fallback time.Time) (string, time.Time) {
	matches := aiLogFilenamePattern.FindStringSubmatch(name)
	if len(matches) != 4 {
		return strings.TrimSuffix(name, filepath.Ext(name)), fallback
	}
	loggedAt, err := time.ParseInLocation("20060102_150405", matches[2], time.Local)
	if err != nil {
		loggedAt = fallback
	}
	return matches[1], loggedAt
}

func splitPromptLog(content string) (string, string) {
	systemMarker := "# SYSTEM PROMPT"
	userMarker := "# USER PROMPT"
	systemIndex := strings.Index(content, systemMarker)
	userIndex := strings.Index(content, userMarker)
	if systemIndex == -1 || userIndex == -1 || userIndex <= systemIndex {
		return content, ""
	}
	systemPrompt := strings.TrimSpace(content[systemIndex+len(systemMarker) : userIndex])
	systemPrompt = strings.Trim(systemPrompt, "- \n\r\t")
	userPrompt := strings.TrimSpace(content[userIndex+len(userMarker):])
	return strings.TrimSpace(systemPrompt), userPrompt
}

func extractAIResponse(content string) string {
	marker := "# AI RESPONSE"
	if idx := strings.Index(content, marker); idx != -1 {
		content = content[idx+len(marker):]
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if newline := strings.Index(content, "\n"); newline != -1 {
			content = content[newline+1:]
		}
		if end := strings.LastIndex(content, "```"); end != -1 {
			content = content[:end]
		}
	}
	return strings.TrimSpace(content)
}

func safeLogName(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
}
