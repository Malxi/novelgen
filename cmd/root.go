package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"novelgen/internal/platform/cli"
	"novelgen/internal/platform/envutil"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

// projectDirFlag overrides project discovery for a single invocation. It is the
// supported way to run a command against a project that is not the working
// directory, and it is how the commands stopped disagreeing about whether the
// working directory is the project root.
var projectDirFlag string

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
  6. export   - Export the completed novel to various formats

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
		return nil
	}
	root, err := store.LocateProject("")
	if err != nil {
		return nil
	}
	store.SetDefault(root)
	cmd.SetContext(store.WithRoot(cmd.Context(), root))
	return nil
}

// ExecuteContext runs the command tree with the given context so callers can
// cancel long-running workflows (for example on SIGINT).
func ExecuteContext(ctx context.Context) {
	cli.SetRootContext(ctx)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// init is called after all other init() functions in the cmd package
// Commands register themselves via RegisterCommand() in their init() functions
