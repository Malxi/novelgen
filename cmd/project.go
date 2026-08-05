package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"novelgen/internal/agentruntime"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	projectCloneSource   string
	projectCloneBook     string
	projectCloneName     string
	projectCloneWithLogs bool
	projectDoctorJSON    bool
)

var (
	loadAgentRuntimeConfig     = agentruntime.LoadConfig
	embeddedAgentRuntimeSkills = agentruntime.EmbeddedSkillNames
	lookAgentRuntimeCommand    = exec.LookPath
	probeClaudeAgentSDKPackage = defaultProbeClaudeAgentSDKPackage
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage novel projects",
}

var projectCloneCmd = &cobra.Command{
	Use:   "clone <target_dir>",
	Short: "Clone the current novel project into a safe creative sandbox",
	Long: `Clone a novel project so Agent SDK experiments can continue from existing
story state without modifying the source project.

By default logs are not copied. Use --with-logs when the clone should preserve
prompt, response, and agent-live logs for debugging or creative continuation.
Use --book system-log --with-logs from the repository root to seed a fresh
creative sandbox from books/system-log history.`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectClone,
}

var projectRenameCmd = &cobra.Command{
	Use:   "rename <name>",
	Short: "Rename the current novel project in novel.json",
	Long:  `Rename only the current project's novel.json name field. Story files, chapters, and logs are not modified.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectRename,
}

var projectDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check whether the current project is ready for Agent SDK creative work",
	Long:  `Run a read-only project preflight for Agent SDK creative work. It checks project metadata, setup, outline, and log availability without modifying files.`,
	Args:  cobra.NoArgs,
	RunE:  runProjectDoctor,
}

func init() {
	projectCloneCmd.Flags().StringVar(&projectCloneSource, "source", "", "Source project directory (default: current project root)")
	projectCloneCmd.Flags().StringVar(&projectCloneBook, "book", "", "Source book name under a books/ directory, e.g. system-log")
	projectCloneCmd.Flags().StringVar(&projectCloneName, "name", "", "Override cloned project name in novel.json")
	projectCloneCmd.Flags().BoolVar(&projectCloneWithLogs, "with-logs", false, "Copy logs/ into the cloned project")
	projectDoctorCmd.Flags().BoolVar(&projectDoctorJSON, "json", false, "Print machine-readable JSON report")
	projectCmd.AddCommand(projectCloneCmd)
	projectCmd.AddCommand(projectRenameCmd)
	projectCmd.AddCommand(projectDoctorCmd)
	RegisterCommand(func() *cobra.Command {
		return projectCmd
	})
}

type projectCloneOptions struct {
	SourceRoot string
	TargetRoot string
	Name       string
	WithLogs   bool
	Now        time.Time
}

type projectCloneManifest struct {
	SourceRoot string    `json:"source_root"`
	TargetRoot string    `json:"target_root"`
	SourceName string    `json:"source_name"`
	TargetName string    `json:"target_name"`
	WithLogs   bool      `json:"with_logs"`
	CreatedAt  time.Time `json:"created_at"`
}

type projectDoctorReport struct {
	Root   string               `json:"root"`
	Name   string               `json:"name,omitempty"`
	Status string               `json:"status"`
	Checks []projectDoctorCheck `json:"checks"`
	Counts map[string]int       `json:"counts,omitempty"`
}

type projectDoctorCheck struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type agentRuntimeCommand struct {
	Command string
	Args    []string
}

func runProjectClone(cmd *cobra.Command, args []string) error {
	source, err := resolveProjectCloneSource(projectCloneSource, projectCloneBook)
	if err != nil {
		return err
	}
	if source == "" {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		source = root
	}
	target := args[0]
	if !filepath.IsAbs(target) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		target = filepath.Join(wd, target)
	}

	manifest, err := cloneProject(projectCloneOptions{
		SourceRoot: source,
		TargetRoot: target,
		Name:       projectCloneName,
		WithLogs:   projectCloneWithLogs,
		Now:        time.Now(),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Cloned project %q to %s\n", manifest.TargetName, manifest.TargetRoot)
	if !manifest.WithLogs {
		fmt.Println("Logs were skipped. Use --with-logs to preserve prompt/response/agent-live history.")
	} else {
		fmt.Println("Logs were copied. Agent history mode can use prompt/response/agent-live context from the source project.")
	}
	fmt.Printf("Next: cd %s\n", manifest.TargetRoot)
	if next := nextAgentHistoryWriteCommand(manifest.TargetRoot); next != "" {
		fmt.Printf("Then: %s\n", next)
	}
	return nil
}

func resolveProjectCloneSource(sourceFlag, bookFlag string) (string, error) {
	sourceFlag = strings.TrimSpace(sourceFlag)
	bookFlag = strings.TrimSpace(bookFlag)
	if sourceFlag != "" && bookFlag != "" {
		return "", fmt.Errorf("--source and --book are mutually exclusive")
	}
	if bookFlag != "" {
		return resolveBookProjectRoot(bookFlag)
	}
	return sourceFlag, nil
}

func resolveBookProjectRoot(book string) (string, error) {
	book = strings.TrimSpace(book)
	if book == "" {
		return "", fmt.Errorf("book name cannot be empty")
	}
	if filepath.IsAbs(book) || strings.ContainsAny(book, `/\`) {
		return "", fmt.Errorf("--book expects a book directory name under books/, got %q", book)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "books", book)
		if _, err := os.Stat(filepath.Join(candidate, "novel.json")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("book %q not found under any parent books/ directory", book)
}

func runProjectRename(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	oldName, err := renameProject(root, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Renamed project %q to %q\n", oldName, strings.TrimSpace(args[0]))
	return nil
}

func runProjectDoctor(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	report := inspectProjectForAgentSDK(root)
	if projectDoctorJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\nRoot: %s\nStatus: %s\n", report.Name, report.Root, report.Status)
		for _, check := range report.Checks {
			marker := "ok"
			if !check.OK {
				marker = check.Severity
			}
			fmt.Fprintf(cmd.OutOrStdout(), "- [%s] %s: %s\n", marker, check.Name, check.Message)
		}
	}
	if report.Status == "error" {
		return fmt.Errorf("project doctor found blocking issues")
	}
	return nil
}

func renameProject(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("project name cannot be empty")
	}
	configPath := filepath.Join(root, "novel.json")
	config, err := models.LoadProjectConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("load novel.json: %w", err)
	}
	oldName := config.Name
	config.Name = name
	if err := config.Save(configPath); err != nil {
		return "", fmt.Errorf("save novel.json: %w", err)
	}
	return oldName, nil
}

func inspectProjectForAgentSDK(root string) projectDoctorReport {
	report := projectDoctorReport{
		Root:   root,
		Status: "ok",
		Counts: map[string]int{},
	}
	addCheck := func(name string, ok bool, severity, message string) {
		report.Checks = append(report.Checks, projectDoctorCheck{
			Name:     name,
			OK:       ok,
			Severity: severity,
			Message:  message,
		})
		if !ok {
			if severity == "error" {
				report.Status = "error"
			} else if report.Status == "ok" {
				report.Status = "warning"
			}
		}
	}

	configPath := filepath.Join(root, "novel.json")
	config, err := models.LoadProjectConfig(configPath)
	if err != nil {
		addCheck("novel_json", false, "error", fmt.Sprintf("cannot load novel.json: %v", err))
		return report
	}
	report.Name = config.Name
	if err := config.Validate(); err != nil {
		addCheck("novel_json", false, "error", fmt.Sprintf("invalid project config: %v", err))
	} else {
		addCheck("novel_json", true, "info", fmt.Sprintf("project %q uses %s/%s", config.Name, config.LLM.Provider, config.LLM.Model))
	}

	setupPath := filepath.Join(root, "story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		addCheck("story_setup", false, "error", fmt.Sprintf("cannot load story setup: %v", err))
	} else {
		report.Counts["core_cast"] = len(setup.CoreCast)
		report.Counts["storylines"] = len(setup.Storylines)
		report.Counts["premises"] = len(setup.Premises)
		if strings.TrimSpace(setup.Premise) == "" {
			addCheck("story_setup", false, "warning", "story setup loads but premise is empty")
		} else {
			addCheck("story_setup", true, "info", "story setup is readable")
		}
	}

	outlinePath := filepath.Join(root, "story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		if os.IsNotExist(err) {
			addCheck("outline", false, "warning", "outline.json is missing; run compose gen/pipeline before write workflows")
		} else {
			addCheck("outline", false, "error", fmt.Sprintf("cannot load outline.json: %v", err))
		}
	} else {
		parts, volumes, chapters := outlineShapeCounts(outline)
		report.Counts["parts"] = parts
		report.Counts["volumes"] = volumes
		report.Counts["chapters"] = chapters
		if parts == 0 || volumes == 0 || chapters == 0 {
			addCheck("outline", false, "warning", "outline loads but has no complete part/volume/chapter structure")
		} else {
			addCheck("outline", true, "info", fmt.Sprintf("outline has %d parts, %d volumes, %d chapters", parts, volumes, chapters))
		}
	}

	logResp := queryLogs(root, "", "", "", false)
	report.Counts["logs"] = logResp.Count
	for kind, count := range logKindCounts(logResp) {
		report.Counts[kind] = count
	}
	if logResp.Count == 0 {
		addCheck("logs", false, "warning", "no logs found; history-aware continuation will have no prompt/response/live context")
	} else {
		addCheck("logs", true, "info", fmt.Sprintf("found %d queryable log files (prompts=%d responses=%d agent-live=%d)", logResp.Count, report.Counts["logs_prompts"], report.Counts["logs_responses"], report.Counts["logs_agent_live"]))
	}
	if _, err := os.Stat(filepath.Join(root, "logs", "clone_manifest.json")); err == nil {
		addCheck("clone_manifest", true, "info", "clone manifest is present")
	}
	addAgentSDKReadinessChecks(report, addCheck, config.LLM.Model)
	return report
}

func logKindCounts(resp toolResponse) map[string]int {
	counts := map[string]int{}
	hits, ok := resp.Results.([]logHit)
	if !ok {
		return counts
	}
	for _, hit := range hits {
		switch normalizeLogKind(hit.Kind) {
		case "prompts":
			counts["logs_prompts"]++
		case "responses":
			counts["logs_responses"]++
		case "agent-live":
			counts["logs_agent_live"]++
		default:
			counts["logs_other"]++
		}
	}
	return counts
}

func addAgentSDKReadinessChecks(report projectDoctorReport, addCheck func(name string, ok bool, severity, message string), projectModel string) {
	skillNames, err := embeddedAgentRuntimeSkills()
	if err != nil {
		addCheck("agent_sdk_skills", false, "error", err.Error())
	} else {
		report.Counts["agent_sdk_skills"] = len(skillNames)
		missing := missingRequiredAgentSDKSkills(skillNames)
		if len(missing) > 0 {
			addCheck("agent_sdk_skills", false, "error", fmt.Sprintf("embedded Agent SDK skills missing: %s", strings.Join(missing, ", ")))
		} else {
			addCheck("agent_sdk_skills", true, "info", fmt.Sprintf("embedded Agent SDK skills available (%d)", len(skillNames)))
		}
	}

	cfg, err := loadAgentRuntimeConfig()
	if err != nil {
		addCheck("agent_sdk_config", false, "error", fmt.Sprintf("cannot load Agent SDK runtime config: %v", err))
		return
	}
	cfgName := strings.TrimSpace(cfg.DefaultRuntime)
	if cfgName == "" {
		cfgName = agentruntime.DefaultRuntimeName
	}
	rc, ok := cfg.Runtimes[cfgName]
	if !ok {
		addCheck("agent_sdk_config", false, "error", fmt.Sprintf("default Agent SDK runtime %q is not configured", cfgName))
		return
	}
	if runtimeType := strings.TrimSpace(rc.Type); runtimeType != "" && runtimeType != "python_process" && runtimeType != "process" {
		addCheck("agent_sdk_config", false, "error", fmt.Sprintf("Agent SDK runtime %q has unsupported type %q", cfgName, runtimeType))
		return
	}
	runtimeModel := strings.TrimSpace(rc.Model)
	addCheck("agent_sdk_config", true, "info", fmt.Sprintf("Agent SDK runtime %q is configured for model %q", cfgName, runtimeModel))
	projectModel = strings.TrimSpace(projectModel)
	if projectModel != "" {
		if runtimeModel == "" || !strings.EqualFold(runtimeModel, projectModel) {
			fallback := runtimeModel
			if fallback == "" {
				fallback = "<unset>"
			}
			addCheck("agent_sdk_model", true, "info", fmt.Sprintf("Agent SDK calls will request project model %q per call; runtime fallback model is %q", projectModel, fallback))
		} else {
			addCheck("agent_sdk_model", true, "info", fmt.Sprintf("Agent SDK runtime model matches project model %q", projectModel))
		}
	} else if runtimeModel == "" {
		addCheck("agent_sdk_model", false, "warning", fmt.Sprintf("project and Agent SDK runtime %q do not set a model", cfgName))
	}

	command := strings.TrimSpace(rc.Command)
	if command == "" {
		command = defaultAgentRuntimePythonCommand()
	}
	candidates := agentRuntimePythonCandidates(command)
	available, err := firstAvailableAgentRuntimeCommand(candidates)
	if err != nil {
		addCheck("agent_sdk_python", false, "error", err.Error())
		return
	}
	addCheck("agent_sdk_python", true, "info", fmt.Sprintf("Python command %q is available for Agent SDK runner", available.Command))
	if err := probeClaudeAgentSDKPackage(available); err != nil {
		addCheck("agent_sdk_python_package", false, "error", fmt.Sprintf("Python package claude_agent_sdk is not importable via %q: %v", available.Command, err))
		return
	}
	addCheck("agent_sdk_python_package", true, "info", fmt.Sprintf("Python package claude_agent_sdk is importable via %q", available.Command))
}

func missingRequiredAgentSDKSkills(available []string) []string {
	required := []string{
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
	}
	have := make(map[string]bool, len(available))
	for _, name := range available {
		have[strings.TrimSpace(name)] = true
	}
	var missing []string
	for _, name := range required {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func defaultAgentRuntimePythonCommand() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func agentRuntimePythonCandidates(command string) []agentRuntimeCommand {
	command = strings.TrimSpace(command)
	if command == "" {
		command = defaultAgentRuntimePythonCommand()
	}
	candidates := []agentRuntimeCommand{{Command: command}}
	if runtime.GOOS == "windows" && strings.EqualFold(command, "python") {
		candidates = append(candidates, agentRuntimeCommand{Command: "py", Args: []string{"-3"}})
	}
	return candidates
}

func firstAvailableAgentRuntimeCommand(candidates []agentRuntimeCommand) (agentRuntimeCommand, error) {
	var missing []string
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Command) == "" {
			continue
		}
		if _, err := lookAgentRuntimeCommand(candidate.Command); err == nil {
			return candidate, nil
		}
		missing = append(missing, candidate.Command)
	}
	if len(missing) == 0 {
		missing = append(missing, defaultAgentRuntimePythonCommand())
	}
	return agentRuntimeCommand{}, fmt.Errorf("no Python command found for Agent SDK runner (tried %s)", strings.Join(missing, ", "))
}

func defaultProbeClaudeAgentSDKPackage(command agentRuntimeCommand) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := append(append([]string{}, command.Args...), "-c", "import claude_agent_sdk")
	cmd := exec.CommandContext(ctx, command.Command, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

func outlineShapeCounts(outline *models.Outline) (parts, volumes, chapters int) {
	if outline == nil {
		return 0, 0, 0
	}
	parts = len(outline.Parts)
	for _, part := range outline.Parts {
		volumes += len(part.Volumes)
		for _, volume := range part.Volumes {
			chapters += len(volume.Chapters)
		}
	}
	return parts, volumes, chapters
}

func cloneProject(opts projectCloneOptions) (*projectCloneManifest, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	sourceRoot, err := filepath.Abs(opts.SourceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve source path: %w", err)
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve target path: %w", err)
	}
	sourceRoot, err = filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return nil, fmt.Errorf("source project not found: %w", err)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "novel.json")); err != nil {
		return nil, fmt.Errorf("source is not a novel project: %w", err)
	}
	if _, err := os.Stat(targetRoot); err == nil {
		return nil, fmt.Errorf("target already exists: %s", targetRoot)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("check target: %w", err)
	}
	if sameOrChildPath(targetRoot, sourceRoot) {
		return nil, fmt.Errorf("target must not be the source project or inside it")
	}

	if err := copyProjectTree(sourceRoot, targetRoot, opts.WithLogs); err != nil {
		_ = os.RemoveAll(targetRoot)
		return nil, err
	}

	configPath := filepath.Join(targetRoot, "novel.json")
	config, err := models.LoadProjectConfig(configPath)
	if err != nil {
		_ = os.RemoveAll(targetRoot)
		return nil, fmt.Errorf("load cloned novel.json: %w", err)
	}
	sourceName := config.Name
	if opts.Name != "" {
		config.Name = opts.Name
	}
	if err := config.Save(configPath); err != nil {
		_ = os.RemoveAll(targetRoot)
		return nil, fmt.Errorf("save cloned novel.json: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(targetRoot, "logs"), 0755); err != nil {
		_ = os.RemoveAll(targetRoot)
		return nil, fmt.Errorf("create cloned logs directory: %w", err)
	}

	manifest := &projectCloneManifest{
		SourceRoot: sourceRoot,
		TargetRoot: targetRoot,
		SourceName: sourceName,
		TargetName: config.Name,
		WithLogs:   opts.WithLogs,
		CreatedAt:  opts.Now,
	}
	if err := writeProjectCloneManifest(targetRoot, manifest); err != nil {
		_ = os.RemoveAll(targetRoot)
		return nil, err
	}
	return manifest, nil
}

func nextAgentHistoryWriteCommand(root string) string {
	chapterID := firstUnwrittenOutlineChapterID(root)
	if chapterID == "" {
		return "novelgen write gen --agent-sdk --agent-history --chapter <chapter_id>"
	}
	return fmt.Sprintf("novelgen write gen --agent-sdk --agent-history --chapter %s", chapterID)
}

func firstUnwrittenOutlineChapterID(root string) string {
	outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json"))
	if err != nil || outline == nil {
		return ""
	}
	first := ""
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				id := strings.TrimSpace(chapter.ID)
				if id == "" {
					continue
				}
				if first == "" {
					first = id
				}
				if !projectChapterMarkdownExists(root, id) {
					return id
				}
			}
		}
	}
	return first
}

func projectChapterMarkdownExists(root, chapterID string) bool {
	for _, path := range []string{
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterID)),
		filepath.Join(root, "chapters", chapterID+".md"),
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", extractChapterNumber(chapterID))),
	} {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return true
		}
	}
	return false
}

func copyProjectTree(sourceRoot, targetRoot string, withLogs bool) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(targetRoot, 0755)
		}
		if rel == ".novelgen" && entry.IsDir() {
			return filepath.SkipDir
		}
		if rel == "logs" && entry.IsDir() && !withLogs {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to clone symlink: %s", path)
		}
		targetPath := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		return copyProjectFile(path, targetPath, info.Mode().Perm(), info.ModTime())
	})
}

func copyProjectFile(sourcePath, targetPath string, perm os.FileMode, modTime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !modTime.IsZero() {
		return os.Chtimes(targetPath, modTime, modTime)
	}
	return nil
}

func writeProjectCloneManifest(targetRoot string, manifest *projectCloneManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(targetRoot, "logs", "clone_manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write clone manifest: %w", err)
	}
	return nil
}

func sameOrChildPath(path, parent string) bool {
	cleanPath := filepath.Clean(path)
	cleanParent := filepath.Clean(parent)
	if strings.EqualFold(cleanPath, cleanParent) {
		return true
	}
	rel, err := filepath.Rel(cleanParent, cleanPath)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
