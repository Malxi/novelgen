package agentruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentTimeoutOverride(t *testing.T) {
	t.Setenv("NOVELGEN_AGENT_TIMEOUT", "")
	if got := agentTimeoutOverride(300); got != 300 {
		t.Fatalf("agentTimeoutOverride(300) without env = %d, want 300", got)
	}
	t.Setenv("NOVELGEN_AGENT_TIMEOUT", "1500")
	if got := agentTimeoutOverride(300); got != 1500 {
		t.Fatalf("agentTimeoutOverride(300) with env 1500 = %d, want 1500", got)
	}
	t.Setenv("NOVELGEN_AGENT_TIMEOUT", "bogus")
	if got := agentTimeoutOverride(900); got != 900 {
		t.Fatalf("agentTimeoutOverride(900) with bogus env = %d, want 900", got)
	}
	t.Setenv("NOVELGEN_AGENT_TIMEOUT", "-5")
	if got := agentTimeoutOverride(900); got != 900 {
		t.Fatalf("agentTimeoutOverride(900) with negative env = %d, want 900", got)
	}
}

func TestConfigNormalizeAddsDefaultClaudeRuntime(t *testing.T) {
	cfg := &Config{}
	cfg.normalize()

	if cfg.DefaultRuntime != "claude" {
		t.Fatalf("DefaultRuntime = %q, want claude", cfg.DefaultRuntime)
	}
	if cfg.AgentHome == "" {
		t.Fatalf("AgentHome is empty")
	}
	runtime, ok := cfg.Runtimes["claude"]
	if !ok {
		t.Fatalf("default claude runtime missing")
	}
	if runtime.Type != "python_process" {
		t.Fatalf("runtime.Type = %q, want python_process", runtime.Type)
	}
	if runtime.Command == "" || len(runtime.Args) == 0 {
		t.Fatalf("runtime command/args not configured: %+v", runtime)
	}
	if runtime.MaxTurns != 8 {
		t.Fatalf("runtime.MaxTurns = %d, want 8", runtime.MaxTurns)
	}
}

func TestNewProcessRuntimeMaterializesEmbeddedClaudeRunner(t *testing.T) {
	agentHome := t.TempDir()
	missingRunner := filepath.Join(t.TempDir(), "claude_runner.py")

	runtime, err := NewProcessRuntime(agentHome, RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{missingRunner},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}
	if runtime.config.Args[0] == missingRunner {
		t.Fatalf("runner path was not materialized")
	}
	if _, err := os.Stat(runtime.config.Args[0]); err != nil {
		t.Fatalf("materialized runner missing: %v", err)
	}
	if filepath.Dir(runtime.config.Args[0]) != filepath.Join(agentHome, "runners") {
		t.Fatalf("runner dir = %q, want %q", filepath.Dir(runtime.config.Args[0]), filepath.Join(agentHome, "runners"))
	}

	skillsDir := filepath.Join(agentHome, "skills")
	if !containsPath(runtime.config.AddDirs, skillsDir) {
		t.Fatalf("embedded skills dir %q not added to AddDirs: %#v", skillsDir, runtime.config.AddDirs)
	}
	for _, skill := range []string{
		"novel-tools", "novel-tools-core", "protagonist-craft-workflow",
		"outline-compose-skeleton-workflow", "outline-compose-volume-workflow",
		"outline-global-repair-workflow", "outline-improve-volume-workflow",
		"craft-character-workflow", "craft-element-workflow", "recap-extract-workflow",
		"setup-improve-workflow", "write-chapter-workflow", "write-review-workflow",
		"write-improve-workflow", "translate-workflow",
	} {
		if _, err := os.Stat(filepath.Join(skillsDir, skill, "SKILL.md")); err != nil {
			t.Fatalf("materialized skill %s missing: %v", skill, err)
		}
	}

	assertSkillContains(t, skillsDir, "novel-tools-core",
		"name: novel-tools-core",
		"Novelgen Core Tool Rules",
		"## Log Context",
		"novelgen tool query logs --view index",
		"apply 前必须先",
		"2>&1",
	)
	assertSkillContains(t, skillsDir, "outline-improve-volume-workflow", "单卷大纲改进 Workflow", "不要使用 `tool patch outline --target chapter`", "最终 JSON 不要返回完整 `volume_patch`")
	assertSkillContains(t, skillsDir, "recap-extract-workflow", "Recap 抽取 Workflow", "apply_patches=false", "novelgen tool patch recap")
	assertSkillContains(t, skillsDir, "write-review-workflow", "summary.total=0", "不要再查询 outline/events/craft", "不要运行 `echo`", "tool check all --target chapter --scope chapter")
	assertSkillContains(t, skillsDir, "write-improve-workflow", "chapter-repair", "tool check all --target chapter", "chapter-write", "tool patch chapter")
	assertSkillContains(t, skillsDir, "translate-workflow", "Translate Workflow", "Do not call tools", "translation")
}

func TestConfigNormalizeSanitizesModelNames(t *testing.T) {
	cfg := &Config{Runtimes: map[string]RuntimeConfig{
		"claude": {
			Model: "deepseek-v4-pro[1m]",
			Env: map[string]string{
				"ANTHROPIC_MODEL":                "deepseek-v4-pro\x1b[0m",
				"ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-flash[1m]",
				"OTHER":                          "keep[1m]",
			},
		},
	}}

	cfg.normalize()

	runtime := cfg.Runtimes["claude"]
	if runtime.Model != "deepseek-v4-pro" {
		t.Fatalf("Model = %q, want sanitized model", runtime.Model)
	}
	if got := runtime.Env["ANTHROPIC_MODEL"]; got != "deepseek-v4-pro" {
		t.Fatalf("ANTHROPIC_MODEL = %q, want sanitized model", got)
	}
	if got := runtime.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"]; got != "deepseek-v4-flash" {
		t.Fatalf("ANTHROPIC_DEFAULT_SONNET_MODEL = %q, want sanitized model", got)
	}
	if got := runtime.Env["OTHER"]; got != "keep[1m]" {
		t.Fatalf("OTHER = %q, want untouched unrelated env", got)
	}
}

func TestTrimUTF8BOM(t *testing.T) {
	got := string(trimUTF8BOM([]byte{0xEF, 0xBB, 0xBF, '{', '}'}))
	if got != "{}" {
		t.Fatalf("trimUTF8BOM() = %q, want {}", got)
	}
}

func TestMaterializedWorkflowSkillsAreNotMojibake(t *testing.T) {
	agentHome := t.TempDir()
	missingRunner := filepath.Join(t.TempDir(), "claude_runner.py")

	_, err := NewProcessRuntime(agentHome, RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{missingRunner},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}
	skillsDir := filepath.Join(agentHome, "skills")
	for _, skill := range []string{
		"novel-tools-core", "protagonist-craft-workflow",
		"outline-compose-skeleton-workflow", "outline-compose-volume-workflow",
		"outline-global-repair-workflow", "outline-improve-volume-workflow",
		"craft-character-workflow", "craft-element-workflow", "recap-extract-workflow",
		"setup-improve-workflow", "write-chapter-workflow", "write-review-workflow",
		"write-improve-workflow", "translate-workflow",
	} {
		data, err := os.ReadFile(filepath.Join(skillsDir, skill, "SKILL.md"))
		if err != nil {
			t.Fatalf("read %s: %v", skill, err)
		}
		text := string(data)
		for _, bad := range []string{
			"锛", "銆", "鈧", "€", "浣犺", "鐨", "鍗", "绔犺", "鏌ヨ", "杈撳", "涓嶈", "鏈€",
			"娴ｈ", "閸楁", "缁旂", "娑撳", "閹惰", "鏉堟", "浣跨敤", "鍙", "乣", "乸", "乶", "沗",
		} {
			if strings.Contains(text, bad) {
				t.Fatalf("%s skill contains mojibake marker %q", skill, bad)
			}
		}
	}
}

func assertSkillContains(t *testing.T, skillsDir, skill string, wants ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(skillsDir, skill, "SKILL.md"))
	if err != nil {
		t.Fatalf("read %s: %v", skill, err)
	}
	text := string(data)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("materialized %s skill missing %q", skill, want)
		}
	}
}

func TestMaterializedWorkflowSkillsForbidShellRedirection(t *testing.T) {
	agentHome := t.TempDir()
	missingRunner := filepath.Join(t.TempDir(), "claude_runner.py")

	runtime, err := NewProcessRuntime(agentHome, RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{missingRunner},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}
	skillsDir := filepath.Join(agentHome, "skills")
	if !containsPath(runtime.config.AddDirs, skillsDir) {
		t.Fatalf("embedded skills dir %q not added to AddDirs: %#v", skillsDir, runtime.config.AddDirs)
	}

	for _, skill := range []string{"novel-tools", "novel-tools-core", "outline-improve-volume-workflow", "write-chapter-workflow", "write-improve-workflow"} {
		data, err := os.ReadFile(filepath.Join(skillsDir, skill, "SKILL.md"))
		if err != nil {
			t.Fatalf("read materialized skill %s: %v", skill, err)
		}
		text := string(data)
		if !strings.Contains(text, "2>&1") {
			t.Fatalf("materialized skill %s should forbid 2>&1", skill)
		}
	}
}

func TestProcessRuntimeApplyRuntimeDefaults(t *testing.T) {
	workspace := t.TempDir()
	runtime, err := NewProcessRuntime(t.TempDir(), RuntimeConfig{
		Type:           "python_process",
		Command:        "python",
		Args:           []string{defaultClaudeRunnerPath()},
		Settings:       "~/claude-settings.json",
		MaxTurns:       12,
		SettingSources: []string{"project"},
		SDKSkills:      []string{"novelgen"},
		AddDirs:        []string{"~/extra"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Read", "Grep"},
		PermissionMode: "dontAsk",
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}

	invocation := Invocation{AgentName: "CraftAgent", WorkspaceRoot: workspace}
	runtime.applyRuntimeDefaults(&invocation)

	if invocation.Settings == "" || invocation.Settings == "~/claude-settings.json" {
		t.Fatalf("Settings was not expanded: %q", invocation.Settings)
	}
	if got := invocation.SettingSources; len(got) != 1 || got[0] != "project" {
		t.Fatalf("SettingSources = %#v", got)
	}
	if got := invocation.SDKSkills; len(got) != 1 || got[0] != "novelgen" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	if got := invocation.AddDirs; len(got) < 1 || !containsPath(got, filepath.Join(home, "extra")) {
		t.Fatalf("AddDirs = %#v", got)
	}
	if got := invocation.AllowedTools; len(got) != 2 || got[0] != "Read" || got[1] != "Grep" {
		t.Fatalf("AllowedTools = %#v", got)
	}
	if got := invocation.Tools; len(got) != 1 || got[0] != "Bash" {
		t.Fatalf("Tools = %#v", got)
	}
	if invocation.PermissionMode != "dontAsk" {
		t.Fatalf("PermissionMode = %q, want dontAsk", invocation.PermissionMode)
	}
	if invocation.Options.MaxTurns != 12 {
		t.Fatalf("Options.MaxTurns = %d, want 12", invocation.Options.MaxTurns)
	}
	if invocation.LiveLogPath == "" {
		t.Fatalf("LiveLogPath is empty")
	}
	if filepath.Dir(invocation.LiveLogPath) != filepath.Join(workspace, "logs", "agent-live") {
		t.Fatalf("LiveLogPath dir = %q", filepath.Dir(invocation.LiveLogPath))
	}
}

func TestProcessRuntimeEnvForcesPythonUTF8ByDefault(t *testing.T) {
	runtime, err := NewProcessRuntime(t.TempDir(), RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{defaultClaudeRunnerPath()},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}

	env := envMap(runtime.env(Invocation{}))
	for key, want := range map[string]string{
		"PYTHONUTF8":       "1",
		"PYTHONIOENCODING": "utf-8",
		"LANG":             "C.UTF-8",
		"LC_ALL":           "C.UTF-8",
	} {
		if got := env[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestProcessRuntimeEnvAllowsExplicitOverride(t *testing.T) {
	runtime, err := NewProcessRuntime(t.TempDir(), RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{defaultClaudeRunnerPath()},
		Env: map[string]string{
			"PYTHONIOENCODING": "utf-8:replace",
		},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}

	env := envMap(runtime.env(Invocation{}))
	if got := env["PYTHONIOENCODING"]; got != "utf-8:replace" {
		t.Fatalf("PYTHONIOENCODING = %q, want explicit override", got)
	}
}

func TestProcessRuntimeEnvIncludesWorkspaceAndCliPath(t *testing.T) {
	workspace := t.TempDir()
	runtime, err := NewProcessRuntime(t.TempDir(), RuntimeConfig{
		Type:    "python_process",
		Command: "python",
		Args:    []string{defaultClaudeRunnerPath()},
	})
	if err != nil {
		t.Fatalf("NewProcessRuntime() error = %v", err)
	}

	env := envMap(runtime.env(Invocation{WorkspaceRoot: workspace}))
	if got := env["NOVELGEN_PROJECT_ROOT"]; got != workspace {
		t.Fatalf("NOVELGEN_PROJECT_ROOT = %q, want %q", got, workspace)
	}
	if got := env["NOVELGEN_WORKSPACE_ROOT"]; got != workspace {
		t.Fatalf("NOVELGEN_WORKSPACE_ROOT = %q, want %q", got, workspace)
	}
	if got := env["NOVELGEN_CLI_PATH"]; got == "" {
		t.Fatalf("NOVELGEN_CLI_PATH is empty")
	}
}

func TestSummarizeLiveLogCountsToolActivity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start","model":"deepseek-v4-flash","sdk_skills":["novel-tools-core","outline-improve-volume-workflow"],"loaded_sdk_skills":["novel-tools-core"],"missing_sdk_skills":["outline-improve-volume-workflow"],"sdk_skill_prompt_chars":1234}`,
		`{"event":"message","message_type":"AssistantMessage","text":"thinking"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query outline --type volume --id P1-V1 --view brief","allowed":true}`,
		`{"event":"tool_hook","hook":"PostToolUse","command":"novelgen tool query outline --type volume --id P1-V1 --view brief","duration_ms":125}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool check all --target outline --scope volume --id P1-V1","allowed":true}`,
		`{"event":"tool_hook","hook":"PostToolUse","command":"novelgen tool check all --target outline --scope volume --id P1-V1","duration_ms":2300}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool refresh chapter-dsl --id P1-V1-C1","allowed":true}`,
		`{"event":"tool_hook","hook":"PostToolUse","command":"novelgen tool refresh chapter-dsl --id P1-V1-C1","duration_ms":17000}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch outline --target volume --id P1-V1 --patch-json \"{}\"","allowed":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch recap --id P1-V1-C1 --patch-json \"{}\"","allowed":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-json \"{}\"","allowed":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query outline --type chapter --id P1-V1-C1 --view full","allowed":false}`,
		`{"event":"final","content":"{}","model":"deepseek-v4-flash"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.Events != 13 || got.Messages != 1 || got.FinalRecords != 1 {
		t.Fatalf("event counts = %+v", got)
	}
	if got.Model != "deepseek-v4-flash" || got.FinalModel != "deepseek-v4-flash" {
		t.Fatalf("model summary = %+v", got)
	}
	if strings.Join(got.SDKSkills, ",") != "novel-tools-core,outline-improve-volume-workflow" ||
		strings.Join(got.LoadedSDKSkills, ",") != "novel-tools-core" ||
		strings.Join(got.MissingSDKSkills, ",") != "outline-improve-volume-workflow" ||
		got.SDKSkillPromptChars != 1234 {
		t.Fatalf("skill summary = %+v", got)
	}
	if got.ToolCalls != 7 || got.ToolAllowed != 6 || got.ToolDenied != 1 {
		t.Fatalf("tool counts = %+v", got)
	}
	if len(got.DeniedToolCommands) != 1 || !strings.Contains(got.DeniedToolCommands[0], "--view full") {
		t.Fatalf("denied commands = %#v", got.DeniedToolCommands)
	}
	if len(got.AllowedToolCommands) < 3 ||
		!strings.Contains(strings.Join(got.AllowedToolCommands, "\n"), "tool check all --target outline --scope volume --id P1-V1") {
		t.Fatalf("allowed commands = %#v", got.AllowedToolCommands)
	}
	if got.QueryCalls != 1 || got.CheckCalls != 1 || got.RefreshCalls != 1 || got.PatchCalls != 3 {
		t.Fatalf("command counts = %+v", got)
	}
	if got.PatchApplies != 0 || got.ApplyWithoutFollowupCheck != 0 {
		t.Fatalf("apply counts = %+v", got)
	}
	if got.ToolDurationMS != 19425 || got.SlowestToolDurationMS != 17000 || !strings.Contains(got.SlowestToolCommand, "tool refresh chapter-dsl") {
		t.Fatalf("duration summary = %+v", got)
	}
}

func TestSummarizeLiveLogSeparatesWorkflowDenials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start","model":"deepseek-v4-flash"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch outline --target volume --id P1-V1","allowed":false,"workflow_denial":true,"reason":"This patch target already has a successful dry-run and this workflow does not allow --apply. Return final JSON now."}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query outline --type chapter --id P1-V1-C1 --view full","allowed":false}`,
		`{"event":"final","content":"{}","model":"deepseek-v4-flash"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.ToolDenied != 2 {
		t.Fatalf("ToolDenied = %d, want 2", got.ToolDenied)
	}
	if len(got.DeniedToolCommands) != 1 || !strings.Contains(got.DeniedToolCommands[0], "--view full") {
		t.Fatalf("permission denied commands = %#v", got.DeniedToolCommands)
	}
	if len(got.WorkflowDeniedToolCommands) != 1 || !strings.Contains(got.WorkflowDeniedToolCommands[0], "tool patch outline") {
		t.Fatalf("workflow denied commands = %#v", got.WorkflowDeniedToolCommands)
	}
}

func TestSummarizeLiveLogParsesStopGuardDenialsResolved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start","model":"deepseek-v4-flash"}`,
		`{"event":"stop_guard","decision":"block","denials_resolved":false}`,
		`{"event":"stop_guard","decision":"allow","denials_resolved":true}`,
		`{"event":"final","content":"{}","model":"deepseek-v4-flash"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if !got.DenialsResolved {
		t.Fatalf("DenialsResolved = false, want true (last stop_guard record wins)")
	}
}

func TestSummarizeLiveLogParsesHookErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start","model":"deepseek-v4-flash"}`,
		`{"event":"hook_error","hook":"Stop","error":"RuntimeError: hook exploded"}`,
		`{"event":"final","content":"{}","model":"deepseek-v4-flash"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if len(got.HookErrors) != 1 || !strings.Contains(got.HookErrors[0], "Stop") || !strings.Contains(got.HookErrors[0], "hook exploded") {
		t.Fatalf("HookErrors = %#v", got.HookErrors)
	}
}

func TestSummarizeLiveLogCountsQueryViewsAndContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query context --type outline-volume --id P1-V1 --view index","allowed":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query context --type outline-volume --id P1-V1 --view=brief","allowed":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query outline --type chapter --id P1-V1-C1 --view full","allowed":false}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool query story-setup --type search --name Lin","allowed":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.QueryCalls != 3 || got.ContextQueryCalls != 2 {
		t.Fatalf("query/context counts = %+v", got)
	}
	if got.QueryIndexCalls != 1 || got.QueryBriefCalls != 1 || got.QueryFullCalls != 0 {
		t.Fatalf("query view counts = %+v", got)
	}
}

func TestSummarizeLiveLogFlagsApplyWithoutFollowupCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-json \"{}\"","allowed":true,"patch_apply":false}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-json \"{}\" --apply","allowed":true,"patch_apply":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.PatchCalls != 2 || got.PatchApplies != 1 {
		t.Fatalf("patch counts = %+v", got)
	}
	if got.ApplyWithoutFollowupCheck != 1 {
		t.Fatalf("ApplyWithoutFollowupCheck = %d, want 1", got.ApplyWithoutFollowupCheck)
	}
}

func TestSummarizeLiveLogClearsApplyAfterFollowupCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-json \"{}\" --apply","allowed":true,"patch_apply":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool check all --target chapter --scope chapter --id P1-V1-C1","allowed":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.PatchApplies != 1 || got.CheckCalls != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if got.ApplyWithoutFollowupCheck != 0 {
		t.Fatalf("ApplyWithoutFollowupCheck = %d, want 0", got.ApplyWithoutFollowupCheck)
	}
}

func TestSummarizeLiveLogClearsOutlineApplyAfterGlobalFollowupCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch outline --target volume --id P1-V1 --patch-json \"{}\" --apply","allowed":true,"patch_apply":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool check all --target outline --scope all --category mysteries --min-priority low --max-issues 12","allowed":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.PatchApplies != 1 || got.CheckCalls != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if got.ApplyWithoutFollowupCheck != 0 {
		t.Fatalf("ApplyWithoutFollowupCheck = %d, want 0", got.ApplyWithoutFollowupCheck)
	}
}

func TestSummarizeLiveLogTreatsRefreshDerivedApplyAsFollowedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-buffer P1-V1-C1-draft --apply --refresh-derived","allowed":true,"patch_apply":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.PatchApplies != 1 {
		t.Fatalf("PatchApplies = %d, want 1", got.PatchApplies)
	}
	if got.ApplyWithoutFollowupCheck != 0 {
		t.Fatalf("ApplyWithoutFollowupCheck = %d, want 0", got.ApplyWithoutFollowupCheck)
	}
}

func TestSummarizeLiveLogRequiresMatchingFollowupCheckTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	content := strings.Join([]string{
		`{"event":"start"}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool patch chapter --id P1-V1-C1 --patch-json \"{}\" --apply","allowed":true,"patch_apply":true}`,
		`{"event":"tool_hook","hook":"PreToolUse","command":"novelgen tool check all --target chapter --scope chapter --id P1-V1-C2","allowed":true}`,
		`{"event":"final","content":"{}"}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write live log: %v", err)
	}

	got := summarizeLiveLog(path)
	if got == nil {
		t.Fatalf("summary is nil")
	}
	if got.PatchApplies != 1 || got.CheckCalls != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if got.ApplyWithoutFollowupCheck != 1 {
		t.Fatalf("ApplyWithoutFollowupCheck = %d, want 1", got.ApplyWithoutFollowupCheck)
	}
}

func TestLiveTargetKeysCoverPatchAndCheckCommands(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		check string
		want  string
	}{
		{
			name:  "chapter",
			patch: `novelgen tool patch chapter --id "P1-V1-C1" --patch-json "{}" --apply`,
			check: `novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"`,
			want:  "chapter:p1-v1-c1",
		},
		{
			name:  "outline volume",
			patch: `novelgen tool patch outline --target volume --id P1-V1 --patch-json "{}" --apply`,
			check: `novelgen tool check all --target outline --scope volume --id P1-V1`,
			want:  "outline:volume:p1-v1",
		},
		{
			name:  "craft preserves comparable id",
			patch: `novelgen tool patch craft --target character --id "Lin" --patch-json "{}" --apply`,
			check: `novelgen tool check schema --target craft --scope character --id "Lin"`,
			want:  "craft:character:lin",
		},
		{
			name:  "recap",
			patch: `novelgen tool patch recap --id P1-V1-C1 --patch-json "{}" --apply`,
			check: `novelgen tool check quality --target recap --scope chapter --id P1-V1-C1`,
			want:  "recap:p1-v1-c1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := livePatchTargetKey(tc.patch); got != tc.want {
				t.Fatalf("livePatchTargetKey() = %q, want %q", got, tc.want)
			}
			gotKeys := liveCheckTargetKeys(tc.check)
			if len(gotKeys) != 1 || gotKeys[0] != tc.want {
				t.Fatalf("liveCheckTargetKeys() = %#v, want [%q]", gotKeys, tc.want)
			}
		})
	}
}

func TestSummarizeToolCommandRedactsPatchJSON(t *testing.T) {
	got := summarizeToolCommand(`echo '{"name":"Lin","background":"` + strings.Repeat("long ", 80) + `"}' | novelgen tool patch craft --target character --id "Lin" --patch-json '{"notes":"secret"}' --apply`)
	if !strings.Contains(got, "novelgen tool patch craft --target character --id \"Lin\" --patch-json <json> --apply") {
		t.Fatalf("summary = %q", got)
	}
	if strings.Contains(got, "secret") || strings.Contains(got, "background") {
		t.Fatalf("summary leaked patch JSON: %q", got)
	}
}

func TestSummarizeToolCommandStartsAtActualNovelgenTool(t *testing.T) {
	got := summarizeToolCommand(`cd "C:\Temp\novelgen-fire-galaxy-live-test" && novelgen tool query context --type craft-character --name "Lin" --view brief 2>&1`)
	want := `novelgen tool query context --type craft-character --name "Lin" --view brief 2>&1`
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestSummarizeToolCommandRedactsPipedPatchBody(t *testing.T) {
	got := summarizeToolCommand(`printf '%s' '{"summary":"很长的中文补丁"}' | novelgen tool patch outline --target volume --id "P1-V1" --apply`)
	want := `novelgen tool patch outline --target volume --id "P1-V1" --apply`
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestSummarizeToolCommandRedactsPatchBufferStdinBody(t *testing.T) {
	got := summarizeToolCommand(`printf '%s' 'SECRET_CHAPTER_BODY 很长的正文' | novelgen tool patch-buffer append --id "P1-V1-C1-draft" --stdin`)
	want := `novelgen tool patch-buffer append --id "P1-V1-C1-draft" --stdin`
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if strings.Contains(got, "SECRET_CHAPTER_BODY") || strings.Contains(got, "printf") {
		t.Fatalf("summary leaked stdin body: %q", got)
	}
}

func TestSummarizeToolDenialReasonClipsUsefulGuidance(t *testing.T) {
	reason := "Patch commands must include real compact JSON, not an empty command or placeholder. " + strings.Repeat("detail ", 80)
	got := summarizeToolDenialReason(reason)
	if !strings.Contains(got, "real compact JSON") {
		t.Fatalf("reason lost guidance: %q", got)
	}
	if len([]rune(got)) > 223 {
		t.Fatalf("reason was not clipped: %d", len([]rune(got)))
	}
	if summarizeToolDenialReason(nil) != "" {
		t.Fatalf("nil reason should be empty")
	}
}

func TestTrimRunnerDetailTruncatesLargeSDKDumps(t *testing.T) {
	got := trimRunnerDetail(strings.Repeat("a", 5000) + "tail")
	if len([]rune(got)) >= 5004 {
		t.Fatalf("detail was not truncated")
	}
	if !strings.Contains(got, "agent runner output truncated") || !strings.HasSuffix(got, "tail") {
		t.Fatalf("truncated detail missing marker or tail: %q", got)
	}
}

func envMap(values []string) map[string]string {
	env := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if ok {
			env[key] = val
		}
	}
	return env
}
