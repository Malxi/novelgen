package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/internal/agentruntime"
	"novelgen/internal/models"
)

func TestCloneProjectSkipsLogsByDefaultAndWritesManifest(t *testing.T) {
	source := writeCloneProjectFixture(t)
	target := filepath.Join(t.TempDir(), "clone")

	manifest, err := cloneProject(projectCloneOptions{
		SourceRoot: source,
		TargetRoot: target,
		Now:        time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("cloneProject() error = %v", err)
	}
	if manifest.WithLogs {
		t.Fatalf("WithLogs = true, want false")
	}
	if _, err := os.Stat(filepath.Join(target, "story", "setup", "story_setup.json")); err != nil {
		t.Fatalf("story state not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "logs", "prompts", "old.md")); !os.IsNotExist(err) {
		t.Fatalf("source logs should be skipped by default, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "logs", "clone_manifest.json")); err != nil {
		t.Fatalf("clone manifest missing: %v", err)
	}
}

func TestCloneProjectWithLogsAndNameOverride(t *testing.T) {
	source := writeCloneProjectFixture(t)
	target := filepath.Join(t.TempDir(), "clone")
	oldLogPath := filepath.Join(source, "logs", "prompts", "old.md")
	oldLogModTime := time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(oldLogPath, oldLogModTime, oldLogModTime); err != nil {
		t.Fatalf("set source log mtime: %v", err)
	}

	manifest, err := cloneProject(projectCloneOptions{
		SourceRoot: source,
		TargetRoot: target,
		Name:       "System Log Sandbox",
		WithLogs:   true,
		Now:        time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("cloneProject() error = %v", err)
	}
	if !manifest.WithLogs {
		t.Fatalf("WithLogs = false, want true")
	}
	if manifest.SourceName != "System Log" || manifest.TargetName != "System Log Sandbox" {
		t.Fatalf("manifest names = %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(target, "logs", "prompts", "old.md")); err != nil {
		t.Fatalf("source logs not copied with --with-logs: %v", err)
	}
	clonedLogInfo, err := os.Stat(filepath.Join(target, "logs", "prompts", "old.md"))
	if err != nil {
		t.Fatalf("stat cloned log: %v", err)
	}
	if !clonedLogInfo.ModTime().Equal(oldLogModTime) {
		t.Fatalf("cloned log mtime = %s, want %s", clonedLogInfo.ModTime(), oldLogModTime)
	}
	if _, err := os.Stat(filepath.Join(target, ".novelgen", "agent-patches", "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale agent patch buffers should not be cloned, stat err=%v", err)
	}
	config, err := models.LoadProjectConfig(filepath.Join(target, "novel.json"))
	if err != nil {
		t.Fatalf("load cloned config: %v", err)
	}
	if config.Name != "System Log Sandbox" {
		t.Fatalf("config.Name = %q", config.Name)
	}
}

func TestResolveProjectCloneSourceFromBookName(t *testing.T) {
	workspace := t.TempDir()
	source := filepath.Join(workspace, "books", "system-log")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "novel.json"), []byte(`{"name":"System Log","version":"1.0.0","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":1},"chapter_config":{"target_words_per_chapter":800},"llm":{"provider":"claude","model":"deepseek-v4-flash"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(workspace, "scratch", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	got, err := resolveProjectCloneSource("", "system-log")
	if err != nil {
		t.Fatalf("resolveProjectCloneSource() error = %v", err)
	}
	if got != source {
		t.Fatalf("source = %q, want %q", got, source)
	}
	if _, err := resolveProjectCloneSource(source, "system-log"); err == nil {
		t.Fatalf("expected --source/--book conflict")
	}
	if _, err := resolveProjectCloneSource("", filepath.Join("nested", "book")); err == nil {
		t.Fatalf("expected nested --book value to fail")
	}
}

func TestNextAgentHistoryWriteCommandUsesFirstUnwrittenChapter(t *testing.T) {
	root := writeCloneProjectFixture(t)
	outlinePath := filepath.Join(root, "story", "compose", "outline.json")
	outlineJSON := `{"parts":[{"id":"P1","title":"Part One","summary":"Opening arc","volumes":[{"id":"P1-V1","title":"Volume One","summary":"The first log window opens.","chapters":[{"id":"P1-V1-C1","title":"First Log","summary":"Written.","characters":["Lead"],"location":"Dorm","events":[],"conflict":"Trusting the impossible log","pacing":"fast"},{"id":"P1-V1-C2","title":"Second Log","summary":"Unwritten.","characters":["Lead"],"location":"Dorm","events":[],"conflict":"Using the log","pacing":"fast"}]}]}]}`
	if err := os.WriteFile(outlinePath, []byte(outlineJSON), 0644); err != nil {
		t.Fatal(err)
	}

	got := nextAgentHistoryWriteCommand(root)
	want := "novelgen write gen --agent-sdk --agent-history --chapter P1-V1-C2"
	if got != want {
		t.Fatalf("next command = %q, want %q", got, want)
	}
}

func TestCloneProjectRejectsTargetInsideSource(t *testing.T) {
	source := writeCloneProjectFixture(t)
	target := filepath.Join(source, "sandbox")

	if _, err := cloneProject(projectCloneOptions{SourceRoot: source, TargetRoot: target}); err == nil {
		t.Fatalf("expected target-inside-source clone to fail")
	}
}

func TestRenameProjectUpdatesOnlyProjectName(t *testing.T) {
	root := writeCloneProjectFixture(t)
	setupPath := filepath.Join(root, "story", "setup", "story_setup.json")
	beforeSetup, err := os.ReadFile(setupPath)
	if err != nil {
		t.Fatal(err)
	}

	oldName, err := renameProject(root, "Clean System Log")
	if err != nil {
		t.Fatalf("renameProject() error = %v", err)
	}
	if oldName != "System Log" {
		t.Fatalf("oldName = %q", oldName)
	}
	config, err := models.LoadProjectConfig(filepath.Join(root, "novel.json"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Name != "Clean System Log" {
		t.Fatalf("config.Name = %q", config.Name)
	}
	afterSetup, err := os.ReadFile(setupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterSetup) != string(beforeSetup) {
		t.Fatalf("rename should not modify story setup")
	}
}

func TestRenameProjectRejectsEmptyName(t *testing.T) {
	root := writeCloneProjectFixture(t)
	if _, err := renameProject(root, "  "); err == nil {
		t.Fatalf("expected empty project name to fail")
	}
}

func TestProjectDoctorReadyOnCloneFixture(t *testing.T) {
	withReadyAgentRuntime(t)
	root := writeCloneProjectFixture(t)

	report := inspectProjectForAgentSDK(root)
	if report.Status != "ok" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if report.Name != "System Log" {
		t.Fatalf("Name = %q", report.Name)
	}
	if report.Counts["parts"] != 1 || report.Counts["volumes"] != 1 || report.Counts["chapters"] != 1 || report.Counts["logs"] != 3 {
		t.Fatalf("Counts = %#v", report.Counts)
	}
	if report.Counts["logs_prompts"] != 1 || report.Counts["logs_responses"] != 1 || report.Counts["logs_agent_live"] != 1 {
		t.Fatalf("log kind counts = %#v", report.Counts)
	}
	if report.Counts["agent_sdk_skills"] == 0 {
		t.Fatalf("agent SDK skill count missing: %#v", report.Counts)
	}
	for _, name := range []string{"agent_sdk_config", "agent_sdk_model", "agent_sdk_python", "agent_sdk_python_package", "agent_sdk_skills"} {
		if !doctorHasCheck(report, name, true, "info") {
			t.Fatalf("missing ready check %q in %#v", name, report.Checks)
		}
	}
}

func TestProjectDoctorMissingOutlineIsWarning(t *testing.T) {
	withReadyAgentRuntime(t)
	root := writeCloneProjectFixture(t)
	if err := os.Remove(filepath.Join(root, "story", "compose", "outline.json")); err != nil {
		t.Fatal(err)
	}

	report := inspectProjectForAgentSDK(root)
	if report.Status != "warning" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "outline", false, "warning") {
		t.Fatalf("missing outline warning in %#v", report.Checks)
	}
}

func TestProjectDoctorInvalidSetupIsError(t *testing.T) {
	withReadyAgentRuntime(t)
	root := writeCloneProjectFixture(t)
	if err := os.WriteFile(filepath.Join(root, "story", "setup", "story_setup.json"), []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}

	report := inspectProjectForAgentSDK(root)
	if report.Status != "error" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "story_setup", false, "error") {
		t.Fatalf("missing setup error in %#v", report.Checks)
	}
}

func TestProjectDoctorMissingAgentSDKConfigIsError(t *testing.T) {
	withAgentRuntimeStubs(t, func() (*agentruntime.Config, error) {
		return nil, os.ErrNotExist
	}, allAgentSDKSkillNames, func(command string) (string, error) {
		return command, nil
	}, func(command agentRuntimeCommand) error {
		return nil
	})
	root := writeCloneProjectFixture(t)

	report := inspectProjectForAgentSDK(root)
	if report.Status != "error" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "agent_sdk_config", false, "error") {
		t.Fatalf("missing agent_sdk_config error in %#v", report.Checks)
	}
}

func TestProjectDoctorMissingAgentSDKSkillIsError(t *testing.T) {
	withAgentRuntimeStubs(t, readyAgentRuntimeConfig, func() ([]string, error) {
		return []string{"novel-tools-core", "write-chapter-workflow"}, nil
	}, func(command string) (string, error) {
		return command, nil
	}, func(command agentRuntimeCommand) error {
		return nil
	})
	root := writeCloneProjectFixture(t)

	report := inspectProjectForAgentSDK(root)
	if report.Status != "error" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "agent_sdk_skills", false, "error") {
		t.Fatalf("missing agent_sdk_skills error in %#v", report.Checks)
	}
}

func TestProjectDoctorAgentSDKModelMismatchUsesProjectModelPerCall(t *testing.T) {
	withAgentRuntimeStubs(t, func() (*agentruntime.Config, error) {
		cfg, err := readyAgentRuntimeConfig()
		if err != nil {
			return nil, err
		}
		cfg.Runtimes[agentruntime.DefaultRuntimeName] = agentruntime.RuntimeConfig{
			Type:    "python_process",
			Command: "python",
			Model:   "deepseek-v4-pro",
		}
		return cfg, nil
	}, allAgentSDKSkillNames, func(command string) (string, error) {
		return command, nil
	}, func(command agentRuntimeCommand) error {
		return nil
	})
	root := writeCloneProjectFixture(t)

	report := inspectProjectForAgentSDK(root)
	if report.Status != "ok" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "agent_sdk_model", true, "info") {
		t.Fatalf("missing agent_sdk_model info in %#v", report.Checks)
	}
	if !doctorCheckMessageContains(report, "agent_sdk_model", "project model") ||
		!doctorCheckMessageContains(report, "agent_sdk_model", "runtime fallback") {
		t.Fatalf("agent_sdk_model message should explain project override/fallback: %#v", report.Checks)
	}
}

func TestProjectDoctorMissingProjectAndRuntimeModelIsConfigError(t *testing.T) {
	withAgentRuntimeStubs(t, func() (*agentruntime.Config, error) {
		cfg, err := readyAgentRuntimeConfig()
		if err != nil {
			return nil, err
		}
		cfg.Runtimes[agentruntime.DefaultRuntimeName] = agentruntime.RuntimeConfig{
			Type:    "python_process",
			Command: "python",
		}
		return cfg, nil
	}, allAgentSDKSkillNames, func(command string) (string, error) {
		return command, nil
	}, func(command agentRuntimeCommand) error {
		return nil
	})
	root := writeCloneProjectFixture(t)
	config, err := models.LoadProjectConfig(filepath.Join(root, "novel.json"))
	if err != nil {
		t.Fatal(err)
	}
	config.LLM.Model = ""
	if err := config.Save(filepath.Join(root, "novel.json")); err != nil {
		t.Fatal(err)
	}

	report := inspectProjectForAgentSDK(root)
	if report.Status != "error" {
		t.Fatalf("Status = %q, checks = %#v", report.Status, report.Checks)
	}
	if !doctorHasCheck(report, "novel_json", false, "error") {
		t.Fatalf("missing novel_json error in %#v", report.Checks)
	}
	if !doctorHasCheck(report, "agent_sdk_model", false, "warning") {
		t.Fatalf("missing agent_sdk_model warning in %#v", report.Checks)
	}
}

func doctorHasCheck(report projectDoctorReport, name string, ok bool, severity string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.OK == ok && check.Severity == severity {
			return true
		}
	}
	return false
}

func doctorCheckMessageContains(report projectDoctorReport, name string, want string) bool {
	for _, check := range report.Checks {
		if check.Name == name && strings.Contains(check.Message, want) {
			return true
		}
	}
	return false
}

func withReadyAgentRuntime(t *testing.T) {
	t.Helper()
	withAgentRuntimeStubs(t, readyAgentRuntimeConfig, allAgentSDKSkillNames, func(command string) (string, error) {
		return command, nil
	}, func(command agentRuntimeCommand) error {
		return nil
	})
}

func withAgentRuntimeStubs(t *testing.T, loadConfig func() (*agentruntime.Config, error), skillNames func() ([]string, error), lookPath func(string) (string, error), probePackage func(agentRuntimeCommand) error) {
	t.Helper()
	oldLoadConfig := loadAgentRuntimeConfig
	oldSkills := embeddedAgentRuntimeSkills
	oldLookPath := lookAgentRuntimeCommand
	oldProbe := probeClaudeAgentSDKPackage
	loadAgentRuntimeConfig = loadConfig
	embeddedAgentRuntimeSkills = skillNames
	lookAgentRuntimeCommand = lookPath
	probeClaudeAgentSDKPackage = probePackage
	t.Cleanup(func() {
		loadAgentRuntimeConfig = oldLoadConfig
		embeddedAgentRuntimeSkills = oldSkills
		lookAgentRuntimeCommand = oldLookPath
		probeClaudeAgentSDKPackage = oldProbe
	})
}

func readyAgentRuntimeConfig() (*agentruntime.Config, error) {
	return &agentruntime.Config{
		DefaultRuntime: agentruntime.DefaultRuntimeName,
		AgentHome:      "agents",
		Runtimes: map[string]agentruntime.RuntimeConfig{
			agentruntime.DefaultRuntimeName: {
				Type:    "python_process",
				Command: "python",
				Model:   "deepseek-v4-flash",
			},
		},
	}, nil
}

func allAgentSDKSkillNames() ([]string, error) {
	return []string{
		"novel-tools-core",
		"outline-compose-skeleton-workflow",
		"outline-compose-volume-workflow",
		"outline-global-repair-workflow",
		"outline-improve-volume-workflow",
		"protagonist-craft-workflow",
		"craft-character-workflow",
		"craft-element-workflow",
		"setup-improve-workflow",
		"write-review-workflow",
		"write-chapter-workflow",
		"write-improve-workflow",
		"recap-extract-workflow",
		"translate-workflow",
	}, nil
}

func writeCloneProjectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	config := &models.ProjectConfig{
		Name:      "System Log",
		Version:   "1.0.0",
		CreatedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Language:  "zh",
		Structure: models.StoryStructure{
			TargetParts:    1,
			TargetVolumes:  1,
			TargetChapters: 2,
		},
		ChapterConfig: models.DefaultChapterConfig(),
		LLM:           models.ProjectLLM{Provider: "claude", Model: "deepseek-v4-flash"},
	}
	if err := config.Save(filepath.Join(root, "novel.json")); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}
	for _, dir := range []string{
		filepath.Join(root, "story", "setup"),
		filepath.Join(root, "story", "compose"),
		filepath.Join(root, "chapters"),
		filepath.Join(root, "logs", "prompts"),
		filepath.Join(root, "logs", "responses"),
		filepath.Join(root, "logs", "agent-live"),
		filepath.Join(root, ".novelgen", "agent-patches"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	files := map[string]string{
		filepath.Join(root, "story", "setup", "story_setup.json"):      `{"project_name":"System Log","genres":["sci-fi"],"premise":"A protagonist can inspect host system logs.","theme":"Power becomes durable when observation creates leverage.","rules":["Logs reveal hidden causal chains."],"target_audience":"web novel readers","tone":"sharp, escalating","tense":"past","pov_style":"third_person_limited"}`,
		filepath.Join(root, "story", "compose", "outline.json"):        `{"parts":[{"id":"P1","title":"Part One","summary":"Opening arc","volumes":[{"id":"P1-V1","title":"Volume One","summary":"The first log window opens.","chapters":[{"id":"P1-V1-C1","title":"First Log","summary":"The protagonist sees the first impossible system log.","characters":["Lead"],"location":"Dorm","events":[],"conflict":"Trusting the impossible log","pacing":"fast"}]}]}]}`,
		filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"):         "chapter",
		filepath.Join(root, "logs", "prompts", "old.md"):               "prompt",
		filepath.Join(root, "logs", "responses", "old.md"):             "response",
		filepath.Join(root, "logs", "agent-live", "old.jsonl"):         `{"event":"start","model":"deepseek-v4-flash"}` + "\n" + `{"event":"final","model":"deepseek-v4-flash"}`,
		filepath.Join(root, ".novelgen", "agent-patches", "stale.txt"): "stale patch buffer",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}
