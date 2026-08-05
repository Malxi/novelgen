package agentruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/logger"
)

// ProcessRuntime invokes an SDK runner process over JSON stdin/stdout.
type ProcessRuntime struct {
	agentHome string
	config    RuntimeConfig
}

// NewProcessRuntime creates a process-backed runtime.
func NewProcessRuntime(agentHome string, config RuntimeConfig) (*ProcessRuntime, error) {
	agentHome = expandHome(agentHome)
	if strings.TrimSpace(config.Command) == "" {
		config.Command = defaultPythonCommand()
	}
	if len(config.Args) == 0 {
		config.Args = []string{defaultClaudeRunnerPath()}
	}
	if len(config.Args) > 0 && isClaudeRunnerArg(config.Args[0]) {
		runnerPath, err := resolveClaudeRunnerPath(agentHome, config.Args[0])
		if err != nil {
			return nil, err
		}
		config.Args[0] = runnerPath
	}
	if config.Timeout == 0 {
		config.Timeout = 120
	}
	if config.MaxTurns == 0 {
		config.MaxTurns = 8
	}
	skillsDir, err := materializeEmbeddedSkills(agentHome)
	if err != nil {
		return nil, err
	}
	if skillsDir != "" && !containsPath(config.AddDirs, skillsDir) {
		config.AddDirs = append(config.AddDirs, skillsDir)
	}
	return &ProcessRuntime{agentHome: agentHome, config: config}, nil
}

// Invoke sends an invocation to the runner and decodes the normalized result.
func (r *ProcessRuntime) Invoke(ctx context.Context, invocation Invocation) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if invocation.Options.Model == "" {
		invocation.Options.Model = r.config.Model
	}
	if invocation.Options.Timeout == 0 {
		invocation.Options.Timeout = r.config.Timeout
	}
	r.applyRuntimeDefaults(&invocation)

	timeout := time.Duration(invocation.Options.Timeout) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := json.Marshal(invocation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent invocation: %w", err)
	}

	var stdout, stderr bytes.Buffer
	var runErr error
	for _, candidate := range r.commandCandidates() {
		stdout.Reset()
		stderr.Reset()
		cmd := exec.CommandContext(runCtx, candidate.command, append(candidate.args, r.config.Args...)...)
		cmd.Stdin = bytes.NewReader(payload)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = r.env(invocation)
		if dir := cleanExistingDir(invocation.WorkspaceRoot); dir != "" {
			cmd.Dir = dir
		}
		runErr = cmd.Start()
		var stopTail func()
		if runErr == nil {
			stopTail = startAgentLiveProgressTailer(runCtx, invocation.LiveLogPath, invocation.AgentName)
			runErr = cmd.Wait()
			if stopTail != nil {
				stopTail()
			}
		}
		if runErr == nil {
			break
		}
		if !looksLikeMissingExecutable(runErr) {
			break
		}
	}
	if runErr != nil {
		detail := trimRunnerDetail(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("agent runner failed: %w: %s", runErr, detail)
		}
		return nil, fmt.Errorf("agent runner failed: %w", runErr)
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse agent runner result: %w; stdout=%s; stderr=%s", err, trimRunnerDetail(stdout.String()), trimRunnerDetail(stderr.String()))
	}
	if strings.TrimSpace(result.Content) == "" {
		return nil, fmt.Errorf("agent runner returned empty content; stderr=%s", trimRunnerDetail(stderr.String()))
	}
	result.LiveLogPath = invocation.LiveLogPath
	result.LiveSummary = summarizeLiveLog(invocation.LiveLogPath)
	return &result, nil
}

func trimRunnerDetail(value string) string {
	value = strings.TrimSpace(value)
	const maxRunes = 4000
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	head := string(runes[:3000])
	tail := string(runes[len(runes)-800:])
	return head + "\n... [agent runner output truncated] ...\n" + tail
}

type liveProgressTailer struct {
	path         string
	agentName    string
	offset       int64
	partial      string
	seenStart    bool
	messageCount int
	nextMessage  int
}

func startAgentLiveProgressTailer(ctx context.Context, path, agentName string) func() {
	path = strings.TrimSpace(path)
	if path == "" {
		return func() {}
	}
	tailCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	tailer := &liveProgressTailer{path: path, agentName: strings.TrimSpace(agentName), nextMessage: 500}
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		tailer.poll()
		for {
			select {
			case <-tailCtx.Done():
				tailer.poll()
				return
			case <-ticker.C:
				tailer.poll()
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (t *liveProgressTailer) poll() {
	info, err := os.Stat(t.path)
	if err != nil || info.IsDir() {
		return
	}
	if info.Size() < t.offset {
		t.offset = 0
		t.partial = ""
	}
	f, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return
	}
	t.offset += int64(len(data))
	chunk := t.partial + string(data)
	lines := strings.Split(chunk, "\n")
	if !strings.HasSuffix(chunk, "\n") {
		t.partial = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	} else {
		t.partial = ""
	}
	for _, line := range lines {
		t.handleLine(strings.TrimSpace(line))
	}
}

func (t *liveProgressTailer) handleLine(line string) {
	if line == "" {
		return
	}
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return
	}
	agent := t.agentName
	if agent == "" {
		agent = strings.TrimSpace(fmt.Sprint(record["agent_name"]))
	}
	if agent == "" {
		agent = "Agent"
	}
	switch strings.TrimSpace(fmt.Sprint(record["event"])) {
	case "start":
		if t.seenStart {
			return
		}
		t.seenStart = true
		logger.Info("[%s] Agent live log: %s", agent, t.path)
	case "message":
		t.messageCount++
		if t.messageCount >= t.nextMessage {
			logger.Info("[%s] Agent live progress: messages=%d", agent, t.messageCount)
			for t.nextMessage <= t.messageCount {
				t.nextMessage += 500
			}
		}
	case "tool_hook":
		if strings.TrimSpace(fmt.Sprint(record["hook"])) != "PreToolUse" {
			return
		}
		command := summarizeToolCommand(strings.TrimSpace(fmt.Sprint(record["command"])))
		allowed, hasAllowed := record["allowed"].(bool)
		if hasAllowed && !allowed {
			workflowDenial, _ := record["workflow_denial"].(bool)
			reason := summarizeToolDenialReason(record["reason"])
			if workflowDenial {
				if reason != "" {
					logger.Info("[%s] Agent workflow guard blocked tool: %s reason=%s", agent, command, reason)
				} else {
					logger.Info("[%s] Agent workflow guard blocked tool: %s", agent, command)
				}
			} else if reason != "" {
				logger.Warn("[%s] Agent denied tool: %s reason=%s", agent, command, reason)
			} else {
				logger.Warn("[%s] Agent denied tool: %s", agent, command)
			}
			return
		}
		logger.Info("[%s] Agent tool: %s", agent, command)
	case "tool_permission":
		command := summarizeToolCommand(strings.TrimSpace(fmt.Sprint(record["command"])))
		if allowed, ok := record["allowed"].(bool); ok && !allowed {
			if reason := summarizeToolDenialReason(record["reason"]); reason != "" {
				logger.Warn("[%s] Agent denied tool permission: %s reason=%s", agent, command, reason)
			} else {
				logger.Warn("[%s] Agent denied tool permission: %s", agent, command)
			}
		}
	case "error":
		detail := strings.TrimSpace(fmt.Sprint(record["detail"]))
		if detail == "" {
			detail = strings.TrimSpace(fmt.Sprint(record["type"]))
		}
		logger.Warn("[%s] Agent live error: %s", agent, clipLiveProgressText(detail, 220))
	case "final":
		logger.Info("[%s] Agent live final record received", agent)
	}
}

func summarizeToolCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "<empty>"
	}
	lower := strings.ToLower(command)
	if idx := novelgenToolCommandIndex(lower); idx >= 0 {
		command = command[idx:]
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--patch-json"); idx >= 0 {
		suffix := ""
		if strings.Contains(lower[idx:], " --apply") {
			suffix = " --apply"
		}
		command = strings.TrimSpace(command[:idx]) + " --patch-json <json>" + suffix
	}
	lower = strings.ToLower(command)
	if idx := strings.Index(lower, "--text"); idx >= 0 && strings.Contains(lower, " tool patch-buffer ") {
		command = strings.TrimSpace(command[:idx]) + " --text <text>"
	}
	return clipLiveProgressText(command, 180)
}

func summarizeToolDenialReason(value interface{}) string {
	reason := strings.TrimSpace(fmt.Sprint(value))
	if reason == "" || reason == "<nil>" {
		return ""
	}
	return clipLiveProgressText(reason, 220)
}

func novelgenToolCommandIndex(lowerCommand string) int {
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

func clipLiveProgressText(value string, maxRunes int) string {
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

func (r *ProcessRuntime) applyRuntimeDefaults(invocation *Invocation) {
	if invocation == nil {
		return
	}
	if strings.TrimSpace(invocation.Settings) == "" {
		invocation.Settings = expandHome(r.config.Settings)
	}
	if len(invocation.SettingSources) == 0 {
		invocation.SettingSources = append([]string(nil), r.config.SettingSources...)
	}
	if len(invocation.SettingSources) == 0 {
		invocation.SettingSources = []string{"project", "local", "user"}
	}
	if len(invocation.SDKSkills) == 0 {
		invocation.SDKSkills = append([]string(nil), r.config.SDKSkills...)
	}
	if len(invocation.AddDirs) == 0 {
		invocation.AddDirs = expandHomeList(r.config.AddDirs)
	}
	if len(invocation.Tools) == 0 {
		invocation.Tools = append([]string(nil), r.config.Tools...)
	}
	if len(invocation.AllowedTools) == 0 {
		invocation.AllowedTools = append([]string(nil), r.config.AllowedTools...)
	}
	if strings.TrimSpace(invocation.PermissionMode) == "" {
		invocation.PermissionMode = r.config.PermissionMode
	}
	if invocation.Options.MaxTurns == 0 {
		invocation.Options.MaxTurns = r.config.MaxTurns
	}
	if strings.TrimSpace(invocation.WorkspaceRoot) != "" {
		invocation.WorkspaceRoot = expandHome(invocation.WorkspaceRoot)
	}
	if r.liveOutputEnabled() && strings.TrimSpace(invocation.LiveLogPath) == "" {
		invocation.LiveLogPath = r.defaultLiveLogPath(*invocation)
	}
}

func (r *ProcessRuntime) liveOutputEnabled() bool {
	if r.config.LiveOutput == nil {
		return true
	}
	return *r.config.LiveOutput
}

func (r *ProcessRuntime) defaultLiveLogPath(invocation Invocation) string {
	root := cleanExistingDir(invocation.WorkspaceRoot)
	if root == "" {
		root = r.agentHome
	}
	name := strings.TrimSpace(invocation.AgentName)
	if name == "" {
		name = "agent"
	}
	name = sanitizeLogName(name)
	timestamp := time.Now().Format("20060102_150405_000000000")
	return filepath.Join(root, "logs", "agent-live", fmt.Sprintf("%s_%s.jsonl", name, timestamp))
}

func sanitizeLogName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "agent"
	}
	return out
}

func summarizeLiveLog(path string) *LiveSummary {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	summary := &LiveSummary{}
	pendingApplyChecks := map[string]int{}
	pendingUnknownApplyChecks := 0
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)
	for scanner.Scan() {
		var record map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		summary.Events++
		switch strings.TrimSpace(fmt.Sprint(record["event"])) {
		case "start":
			summary.Model = liveRecordString(record, "model")
			summary.SDKSkills = liveRecordStringSlice(record, "sdk_skills")
			summary.LoadedSDKSkills = liveRecordStringSlice(record, "loaded_sdk_skills")
			summary.MissingSDKSkills = liveRecordStringSlice(record, "missing_sdk_skills")
			summary.SDKSkillPromptChars = liveRecordInt(record, "sdk_skill_prompt_chars")
		case "message":
			summary.Messages++
		case "final":
			summary.FinalRecords++
			summary.FinalModel = firstNonEmpty(liveRecordString(record, "model"), summary.FinalModel)
		case "stop_guard":
			summary.DenialsResolved = liveRecordBool(record, "denials_resolved")
		case "hook_error":
			addHookError(summary, record)
		case "tool_hook", "tool_permission":
			hook := strings.TrimSpace(fmt.Sprint(record["hook"]))
			command := strings.TrimSpace(fmt.Sprint(record["command"]))
			if hook == "PostToolUse" {
				duration := liveRecordInt(record, "duration_ms")
				if duration > 0 {
					summary.ToolDurationMS += duration
					if duration > summary.SlowestToolDurationMS {
						summary.SlowestToolDurationMS = duration
						summary.SlowestToolCommand = command
					}
				}
			}
			if hook == "PreToolUse" || strings.TrimSpace(fmt.Sprint(record["event"])) == "tool_permission" {
				summary.ToolCalls++
				if allowed, ok := record["allowed"].(bool); ok {
					if allowed {
						summary.ToolAllowed++
						addAllowedToolCommand(summary, command)
						countToolCommand(summary, command)
					} else {
						summary.ToolDenied++
						if workflowDenied, ok := record["workflow_denial"].(bool); ok && workflowDenied {
							addWorkflowDeniedToolCommand(summary, command)
						} else {
							addDeniedToolCommand(summary, command)
						}
					}
				}
				if allowed, ok := record["allowed"].(bool); ok && allowed {
					if isPatchApplyCommand(command, record) {
						if patchApplyIncludesRefreshDerived(command) {
							continue
						}
						if key := livePatchTargetKey(command); key != "" {
							pendingApplyChecks[key]++
						} else {
							pendingUnknownApplyChecks++
						}
					} else if isToolCheckCommand(command) {
						checkKeys := liveCheckTargetKeys(command)
						if len(checkKeys) == 0 && pendingUnknownApplyChecks > 0 {
							pendingUnknownApplyChecks = 0
						}
						for _, key := range checkKeys {
							if key == "outline" {
								for pendingKey := range pendingApplyChecks {
									if strings.HasPrefix(pendingKey, "outline:") {
										delete(pendingApplyChecks, pendingKey)
									}
								}
								continue
							}
							delete(pendingApplyChecks, key)
						}
					}
				}
			}
		}
	}
	summary.ApplyWithoutFollowupCheck = pendingUnknownApplyChecks
	for _, count := range pendingApplyChecks {
		summary.ApplyWithoutFollowupCheck += count
	}
	if summary.Events == 0 {
		return nil
	}
	if summary.FinalModel == "" {
		summary.FinalModel = summary.Model
	}
	return summary
}

func liveRecordString(record map[string]interface{}, key string) string {
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

func addAllowedToolCommand(summary *LiveSummary, command string) {
	if summary == nil {
		return
	}
	command = summarizeToolCommand(command)
	if command == "" {
		return
	}
	for _, existing := range summary.AllowedToolCommands {
		if existing == command {
			return
		}
	}
	if len(summary.AllowedToolCommands) >= 20 {
		return
	}
	summary.AllowedToolCommands = append(summary.AllowedToolCommands, command)
}

func liveRecordStringSlice(record map[string]interface{}, key string) []string {
	if record == nil {
		return nil
	}
	value, ok := record[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
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

func addDeniedToolCommand(summary *LiveSummary, command string) {
	if summary == nil {
		return
	}
	command = summarizeToolCommand(command)
	if command == "" {
		return
	}
	for _, existing := range summary.DeniedToolCommands {
		if existing == command {
			return
		}
	}
	if len(summary.DeniedToolCommands) >= 5 {
		return
	}
	summary.DeniedToolCommands = append(summary.DeniedToolCommands, command)
}

func addWorkflowDeniedToolCommand(summary *LiveSummary, command string) {
	if summary == nil {
		return
	}
	command = summarizeToolCommand(command)
	if command == "" {
		return
	}
	for _, existing := range summary.WorkflowDeniedToolCommands {
		if existing == command {
			return
		}
	}
	if len(summary.WorkflowDeniedToolCommands) >= 5 {
		return
	}
	summary.WorkflowDeniedToolCommands = append(summary.WorkflowDeniedToolCommands, command)
}

func addHookError(summary *LiveSummary, record map[string]interface{}) {
	if summary == nil {
		return
	}
	hook := strings.TrimSpace(fmt.Sprint(record["hook"]))
	detail := strings.TrimSpace(fmt.Sprint(record["error"]))
	if detail == "" {
		detail = strings.TrimSpace(fmt.Sprint(record["repr"]))
	}
	entry := hook
	if detail != "" {
		entry += ": " + detail
	}
	entry = clipLiveProgressText(entry, 220)
	if entry == "" {
		return
	}
	for _, existing := range summary.HookErrors {
		if existing == entry {
			return
		}
	}
	if len(summary.HookErrors) >= 5 {
		return
	}
	summary.HookErrors = append(summary.HookErrors, entry)
}

func liveRecordInt(record map[string]interface{}, key string) int {
	if record == nil {
		return 0
	}
	switch value := record[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := strconv.Atoi(value.String())
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func liveRecordBool(record map[string]interface{}, key string) bool {
	if record == nil {
		return false
	}
	value, ok := record[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func countToolCommand(summary *LiveSummary, command string) {
	if summary == nil {
		return
	}
	normalized := strings.ToLower(command)
	switch {
	case strings.Contains(normalized, " tool query "):
		summary.QueryCalls++
		countToolQueryShape(summary, normalized)
	case strings.Contains(normalized, " tool check "):
		summary.CheckCalls++
	case strings.Contains(normalized, " tool refresh "):
		summary.RefreshCalls++
	case strings.Contains(normalized, " tool patch outline"), strings.Contains(normalized, " tool patch craft"), strings.Contains(normalized, " tool patch setup"), strings.Contains(normalized, " tool patch recap"), strings.Contains(normalized, " tool patch chapter"), strings.Contains(normalized, " tool patch-buffer "):
		summary.PatchCalls++
		if strings.Contains(normalized, " --apply") {
			summary.PatchApplies++
		}
	}
}

func countToolQueryShape(summary *LiveSummary, normalizedCommand string) {
	if summary == nil {
		return
	}
	if strings.Contains(normalizedCommand, " tool query context ") ||
		strings.Contains(normalizedCommand, " tool query context --") {
		summary.ContextQueryCalls++
	}
	switch {
	case commandHasView(normalizedCommand, "index"):
		summary.QueryIndexCalls++
	case commandHasView(normalizedCommand, "brief"):
		summary.QueryBriefCalls++
	case commandHasView(normalizedCommand, "full"):
		summary.QueryFullCalls++
	}
}

func commandHasView(command, view string) bool {
	view = strings.ToLower(strings.TrimSpace(view))
	if view == "" {
		return false
	}
	return strings.Contains(command, " --view "+view) ||
		strings.Contains(command, " --view="+view) ||
		strings.Contains(command, " --view=\""+view+"\"") ||
		strings.Contains(command, " --view='"+view+"'")
}

func isToolCheckCommand(command string) bool {
	return strings.Contains(strings.ToLower(command), " tool check ")
}

func isPatchApplyCommand(command string, record map[string]interface{}) bool {
	if patchApply, ok := record["patch_apply"].(bool); ok {
		return patchApply
	}
	normalized := strings.ToLower(command)
	if !strings.Contains(normalized, " --apply") {
		return false
	}
	return strings.Contains(normalized, " tool patch outline") ||
		strings.Contains(normalized, " tool patch craft") ||
		strings.Contains(normalized, " tool patch setup") ||
		strings.Contains(normalized, " tool patch recap") ||
		strings.Contains(normalized, " tool patch chapter")
}

func patchApplyIncludesRefreshDerived(command string) bool {
	normalized := strings.ToLower(command)
	return strings.Contains(normalized, " tool patch chapter") &&
		strings.Contains(normalized, " --apply") &&
		strings.Contains(normalized, " --refresh-derived")
}

func livePatchTargetKey(command string) string {
	normalized := strings.ToLower(command)
	switch {
	case strings.Contains(normalized, " tool patch setup"):
		return "setup"
	case strings.Contains(normalized, " tool patch recap"):
		return liveScopedTargetKey("recap", "", commandFlagValue(command, "--id"))
	case strings.Contains(normalized, " tool patch chapter"):
		return liveScopedTargetKey("chapter", "", commandFlagValue(command, "--id"))
	case strings.Contains(normalized, " tool patch outline"):
		scope := strings.ToLower(commandFlagValue(command, "--target"))
		if scope == "" {
			scope = "chapter"
		}
		return liveScopedTargetKey("outline", scope, commandFlagValue(command, "--id"))
	case strings.Contains(normalized, " tool patch craft"):
		return liveScopedTargetKey("craft", strings.ToLower(commandFlagValue(command, "--target")), commandFlagValue(command, "--id"))
	default:
		return ""
	}
}

func liveCheckTargetKeys(command string) []string {
	if !isToolCheckCommand(command) {
		return nil
	}
	target := strings.ToLower(commandFlagValue(command, "--target"))
	scope := strings.ToLower(commandFlagValue(command, "--scope"))
	id := commandFlagValue(command, "--id")
	switch target {
	case "setup":
		return []string{"setup"}
	case "recap":
		if key := liveScopedTargetKey("recap", "", id); key != "" {
			return []string{key}
		}
	case "chapter":
		if key := liveScopedTargetKey("chapter", "", id); key != "" {
			return []string{key}
		}
	case "outline":
		if scope == "all" && id == "" {
			return []string{"outline"}
		}
		if key := liveScopedTargetKey("outline", scope, id); key != "" {
			return []string{key}
		}
	case "craft":
		if key := liveScopedTargetKey("craft", scope, id); key != "" {
			return []string{key}
		}
	}
	return nil
}

func liveScopedTargetKey(target, scope, id string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	scope = strings.ToLower(strings.TrimSpace(scope))
	id = strings.ToLower(strings.TrimSpace(id))
	if target == "" {
		return ""
	}
	if scope != "" && id != "" {
		return target + ":" + scope + ":" + id
	}
	if id != "" {
		return target + ":" + id
	}
	return ""
}

func commandFlagValue(command, flag string) string {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return ""
	}
	pattern := `(?i)(?:^|\s)` + regexp.QuoteMeta(flag) + `\s+("[^"]*"|'[^']*'|\S+)`
	match := regexp.MustCompile(pattern).FindStringSubmatch(command)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(match[1]), `"'`)
}

type commandCandidate struct {
	command string
	args    []string
}

func (r *ProcessRuntime) commandCandidates() []commandCandidate {
	candidates := []commandCandidate{{command: r.config.Command}}
	if runtime.GOOS == "windows" && strings.EqualFold(strings.TrimSpace(r.config.Command), "python") {
		candidates = append(candidates, commandCandidate{command: "py", args: []string{"-3"}})
	}
	return candidates
}

func cleanExistingDir(path string) string {
	path = expandHome(path)
	if strings.TrimSpace(path) == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return ""
	}
	return abs
}

func expandHomeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	expanded := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			expanded = append(expanded, expandHome(trimmed))
		}
	}
	return expanded
}

func containsPath(values []string, target string) bool {
	target = expandHome(target)
	for _, value := range values {
		if strings.EqualFold(filepath.Clean(expandHome(value)), filepath.Clean(target)) {
			return true
		}
	}
	return false
}

func looksLikeMissingExecutable(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "executable file not found")
}

func (r *ProcessRuntime) env(invocation Invocation) []string {
	env := os.Environ()
	appendKV := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			env = append(env, key+"="+value)
		}
	}
	prependPathDir := func(dir string) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		pathKey := "PATH"
		if runtime.GOOS == "windows" {
			pathKey = "Path"
		}
		current := os.Getenv(pathKey)
		if strings.TrimSpace(current) == "" && pathKey != "PATH" {
			current = os.Getenv("PATH")
		}
		env = append(env, pathKey+"="+dir+string(os.PathListSeparator)+current)
	}
	appendKV("NOVELGEN_AGENT_HOME", r.agentHome)
	if exe, err := os.Executable(); err == nil {
		appendKV("NOVELGEN_CLI_PATH", exe)
		prependPathDir(filepath.Dir(exe))
	}
	if root := cleanExistingDir(invocation.WorkspaceRoot); root != "" {
		appendKV("NOVELGEN_PROJECT_ROOT", root)
		appendKV("NOVELGEN_WORKSPACE_ROOT", root)
	}
	appendKV("PYTHONUTF8", "1")
	appendKV("PYTHONIOENCODING", "utf-8")
	appendKV("LANG", "C.UTF-8")
	appendKV("LC_ALL", "C.UTF-8")
	appendKV("ANTHROPIC_BASE_URL", r.config.BaseURL)
	appendKV("ANTHROPIC_AUTH_TOKEN", r.config.APIKey)
	appendKV("ANTHROPIC_API_KEY", r.config.APIKey)
	appendKV("ANTHROPIC_MODEL", r.config.Model)
	if r.config.Timeout > 0 {
		appendKV("NOVELGEN_AGENT_TIMEOUT", strconv.Itoa(r.config.Timeout))
	}
	if r.config.MaxTurns > 0 {
		appendKV("NOVELGEN_AGENT_MAX_TURNS", strconv.Itoa(r.config.MaxTurns))
	}
	for key, value := range r.config.Env {
		appendKV(key, value)
	}
	return env
}
