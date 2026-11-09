package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// The run-metrics JSON is produced by the parent module (internal/platform/
// runmetrics). webui cannot import it, so these structs mirror the wire format.
// internal/architecture/schema_test.go compares the JSON tags on both sides; keep
// every field in step or the build fails.
type agentRunSummary struct {
	RunID            string            `json:"run_id"`
	Command          string            `json:"command,omitempty"`
	Status           string            `json:"status"`
	StartedAt        string            `json:"started_at,omitempty"`
	FinishedAt       string            `json:"finished_at,omitempty"`
	DurationMS       int64             `json:"duration_ms,omitempty"`
	Events           int               `json:"events"`
	AgentCalls       int               `json:"agent_calls"`
	ToolCalls        int               `json:"tool_calls"`
	ToolDenied       int               `json:"tool_denied,omitempty"`
	Retries          int               `json:"retries,omitempty"`
	StuckEvents      int               `json:"stuck_events,omitempty"`
	PromptTokens     int               `json:"prompt_tokens,omitempty"`
	CompletionTokens int               `json:"completion_tokens,omitempty"`
	TotalTokens      int               `json:"total_tokens,omitempty"`
	Agents           []string          `json:"agents,omitempty"`
	Models           []string          `json:"models,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Skills           []string          `json:"skills,omitempty"`
	Commands         []string          `json:"commands,omitempty"`
	Stuck            []stuckSignal     `json:"stuck,omitempty"`
	Errors           []string          `json:"errors,omitempty"`
	LiveLog          string            `json:"live_log,omitempty"`
}

type stuckSignal struct {
	Reason string `json:"reason"`
	Event  string `json:"event,omitempty"`
	Count  int    `json:"count,omitempty"`
}

type agentRunEvent struct {
	Version           int               `json:"version"`
	RunID             string            `json:"run_id"`
	SpanID            string            `json:"span_id,omitempty"`
	ParentSpanID      string            `json:"parent_span_id,omitempty"`
	Event             string            `json:"event"`
	Time              time.Time         `json:"time"`
	Status            string            `json:"status,omitempty"`
	Agent             string            `json:"agent,omitempty"`
	Model             string            `json:"model,omitempty"`
	Kind              string            `json:"kind,omitempty"`
	Command           string            `json:"command,omitempty"`
	NormalizedCommand string            `json:"normalized_command,omitempty"`
	Skills            []string          `json:"skills,omitempty"`
	LoadedSkills      []string          `json:"loaded_skills,omitempty"`
	MissingSkills     []string          `json:"missing_skills,omitempty"`
	ToolName          string            `json:"tool_name,omitempty"`
	Allowed           *bool             `json:"allowed,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	Error             string            `json:"error,omitempty"`
	DurationMS        int64             `json:"duration_ms,omitempty"`
	PromptTokens      int               `json:"prompt_tokens,omitempty"`
	CompletionTokens  int               `json:"completion_tokens,omitempty"`
	TotalTokens       int               `json:"total_tokens,omitempty"`
	PromptChars       int               `json:"prompt_chars,omitempty"`
	SystemPromptChars int               `json:"system_prompt_chars,omitempty"`
	ResponseChars     int               `json:"response_chars,omitempty"`
	LiveLogPath       string            `json:"live_log_path,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type agentRunDetail struct {
	Summary agentRunSummary `json:"summary"`
	Events  []agentRunEvent `json:"events"`
}

var runMetricsCLI = func(project string, args ...string) ([]byte, error) {
	cmdArgs := []string{"--dir", project, "project"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(getNovelGenPath(), cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return stdout.Bytes(), nil
}

func listRunMetrics(c *gin.Context) {
	project := projectPathFromQuery(c)
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			limit = parsed
		}
	}
	args := []string{"runs", "--json", "--limit", strconv.Itoa(limit)}
	if strings.EqualFold(c.Query("stuck"), "1") || strings.EqualFold(c.Query("stuck"), "true") {
		args = append(args, "--stuck")
	}
	data, err := runMetricsCLI(project, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	var runs []agentRunSummary
	if err := json.Unmarshal(data, &runs); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "run metrics returned invalid JSON"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: runs})
}

func getRunMetrics(c *gin.Context) {
	runID, err := safeSegment(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid run id"})
		return
	}
	project := projectPathFromQuery(c)
	data, err := runMetricsCLI(project, "run", runID, "--json")
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
		return
	}
	var detail agentRunDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "run detail returned invalid JSON"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
}
