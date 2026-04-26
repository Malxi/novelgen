package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// init is called after all other init() functions in the cmd package
// Commands register themselves via RegisterCommand() in their init() functions
