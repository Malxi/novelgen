package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"novelgen/internal/platform/cli"
	"novelgen/internal/platform/envutil"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/runmetrics"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

// projectDirFlag overrides project discovery for a single invocation. It is the
// supported way to run a command against a project that is not the working
// directory, and it is how the commands stopped disagreeing about whether the
// working directory is the project root.
var projectDirFlag string

var activeRun struct {
	mu  sync.Mutex
	run *runmetrics.Run
}

var rootCmd = &cobra.Command{
	Use:   "novelgen",
	Short: "A CLI tool for AI-assisted novel generation",
	Long: `Novelgen is a command-line tool for AI-assisted novel creation.

It provides a structured workflow to guide you from initial idea to complete novel:
  1. init     - Initialize a new novel project
  2. setup    - Create story setup (genre, premise, theme, etc.)
  3. compose  - Generate story outline (parts -> volumes -> chapters)
  4. craft    - Create detailed world elements (characters, locations, items)
  5. write    - Generate, review, improve, recap, and emit RPG DSL for final chapters
  6. agent    - Run a goal-level autonomous agent with skills and project tools
  7. export   - Export the completed novel to various formats

Legacy note: draft commands still exist for old projects, but the recommended
workflow writes final chapters directly with "novelgen write pipeline".

Use "novelgen <command> --help" for more information about a command.`,
	PersistentPreRunE: resolveProjectRoot,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&projectDirFlag, "dir", "",
		"Project directory to operate on (default: nearest novel.json above the working directory)")
}

// resolveProjectRoot installs the project root for the invocation, so both the
// command tree (through the command context) and library code reached without a
// command handle (through the ambient store default) resolve the same project.
//
// A missing project is not an error here: `init` creates one, and commands that
// require an existing project report their own, better error.
func resolveProjectRoot(cmd *cobra.Command, _ []string) error {
	// Commands may be executed repeatedly in one process (tests, embedding).
	// Clear the previous invocation's ambient root before resolving this one so
	// a "no project here" result cannot silently inherit an older project.
	store.ResetDefault()

	dir := strings.TrimSpace(projectDirFlag)
	if dir == "" {
		dir = strings.TrimSpace(envutil.String("NOVELGEN_PROJECT_DIR"))
	}
	if dir != "" {
		root, err := store.OpenProject(dir)
		if err != nil {
			return err
		}
		if hostDir, ok := root.HostDir(); ok && !store.DirExists(hostDir) {
			return fmt.Errorf("project directory %s does not exist", dir)
		}
		store.SetDefault(root)
		cmd.SetContext(store.WithRoot(cmd.Context(), root))
		startRunMetrics(cmd, root)
		return nil
	}
	root, err := store.LocateProject("")
	if err != nil {
		return nil
	}
	store.SetDefault(root)
	cmd.SetContext(store.WithRoot(cmd.Context(), root))
	startRunMetrics(cmd, root)
	return nil
}

func startRunMetrics(cmd *cobra.Command, root *store.Root) {
	if cmd == nil || root == nil || isRunMetricsReadCommand(cmd.CommandPath()) {
		return
	}
	if strings.TrimSpace(envutil.String("NOVELGEN_RUN_ID")) != "" {
		// Agent SDK tool commands are child processes of an already-recorded run.
		// The parent records the tool call and duration; starting another run
		// here would turn every query into a misleading standalone job.
		return
	}
	ctx, run, err := runmetrics.StartWithMetadata(cmd.Context(), root, cmd.CommandPath(), runMetadata())
	if err != nil {
		logger.Warn("Failed to start run metrics: %v", err)
		return
	}
	cmd.SetContext(ctx)
	activeRun.mu.Lock()
	activeRun.run = run
	activeRun.mu.Unlock()
}

func runMetadata() map[string]string {
	metadata := map[string]string{}
	for envName, key := range map[string]string{
		"NOVELGEN_ORIGIN":            "origin",
		"NOVELGEN_TOOL":              "tool",
		"NOVELGEN_SECTION":           "section",
		"NOVELGEN_MCP_SESSION":       "mcp_session",
		"NOVELGEN_WORKBUDDY_SESSION": "workbuddy_session",
		"NOVELGEN_INPUT_BYTES":       "input_bytes",
	} {
		if value := strings.TrimSpace(envutil.String(envName)); value != "" {
			metadata[key] = value
		}
	}
	return metadata
}

func isRunMetricsReadCommand(commandPath string) bool {
	path := strings.TrimSpace(commandPath)
	// Harness hooks and the long-lived MCP stdio server only read project state;
	// they must not create standalone run records. Each MCP tool child records
	// its own origin/tool/session metadata.
	if path == "novelgen hook" || strings.HasPrefix(path, "novelgen hook ") ||
		path == "novelgen mcp serve" {
		return true
	}
	switch path {
	case "novelgen project run", "novelgen project runs", "novelgen project usage", "novelgen project doctor":
		return true
	default:
		return false
	}
}

// ExecuteContext runs the command tree with the given context so callers can
// cancel long-running workflows (for example on SIGINT). It returns the command
// error instead of calling os.Exit so embedding callers and tests can choose
// their own exit behaviour; main.go maps it to the process exit code.
func ExecuteContext(ctx context.Context) error {
	cli.SetRootContext(ctx)
	err := rootCmd.ExecuteContext(ctx)
	activeRun.mu.Lock()
	run := activeRun.run
	activeRun.run = nil
	activeRun.mu.Unlock()
	run.Finish("", err)
	return err
}

// init is called after all other init() functions in the cmd package
// Commands register themselves via RegisterCommand() in their init() functions
