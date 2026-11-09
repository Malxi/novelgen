// Package bookcmd exposes the agent-facing book management commands.
package bookcmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/projectinit"
	"novelgen/internal/toolplane"
)

var (
	bookStatusJSON bool

	bookInitParts    int
	bookInitVolumes  int
	bookInitChapter  int
	bookInitGenre    string
	bookInitMode     string
	bookInitProvider string
	bookInitLanguage string

	bookWriteMode        string
	bookWriteTarget      string
	bookWriteID          string
	bookWriteApply       bool
	bookWriteDryRun      bool
	bookWriteRefresh     bool
	bookWriteMaxIssues   int
	bookWriteTargetWords int
)

var bookCmd = &cobra.Command{
	Use:   "book",
	Short: "Manage one Novelgen book",
	Long: `Manage one Novelgen book.

  init      create a project
  status    print project progress and the next step
  read      read setup/outline/craft/chapter/context/logs
  write     patch or replace a complete artifact
  refresh   rebuild derived chapter DSL or narrative memory

The older "novelgen tool ..." commands remain available as compatibility
aliases.`,
}

var bookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print project progress and the next step",
	Args:  cobra.NoArgs,
	RunE:  runBookStatus,
}

var bookInitCmd = &cobra.Command{
	Use:   "init [book_name]",
	Short: "Initialize a new novel project",
	Args:  cobra.ExactArgs(1),
	RunE:  runBookInit,
}

var bookReadCmd = &cobra.Command{
	Use:   "read <setup|outline|craft|chapter|context|logs>",
	Short: "Read project state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		section := strings.ToLower(strings.TrimSpace(args[0]))
		if section == "ideas" {
			return fmt.Errorf("use `novelgen ideas draw` for content ideas, not `book read`")
		}
		if section == "setup" {
			section = "story-setup"
		}
		return toolplane.ToolQueryCmd.RunE(cmd, []string{section})
	},
}

var bookWriteCmd = &cobra.Command{
	Use:   "write <setup|outline|craft|chapter|recap>",
	Short: "Patch or replace one complete artifact",
	Long: `Patch or replace one complete artifact.

The artifact is read from stdin. Patch mode is incremental; replace mode writes
the complete artifact. Both validate before writing and default to a dry run
unless --apply is passed.`,
	Args: cobra.ExactArgs(1),
	RunE: runBookWrite,
}

var bookRefreshCmd = &cobra.Command{
	Use:   "refresh <chapter-dsl|narrative-memory>",
	Short: "Rebuild derived project artifacts",
	Args:  cobra.ExactArgs(1),
	RunE:  toolplane.ToolRefreshCmd.RunE,
}

func init() {
	bookStatusCmd.Flags().BoolVar(&bookStatusJSON, "json", false, "Print the status as JSON")

	bookInitCmd.Flags().IntVar(&bookInitParts, "parts", 1, "Number of parts (部)")
	bookInitCmd.Flags().IntVar(&bookInitVolumes, "volumes", 1, "Number of volumes per part (卷)")
	bookInitCmd.Flags().IntVar(&bookInitChapter, "chapter", 20, "Number of chapters per volume (章)")
	bookInitCmd.Flags().StringVar(&bookInitGenre, "genre", "", "Genre(s), comma-separated")
	bookInitCmd.Flags().StringVar(&bookInitMode, "mode", "", "LLM model to use")
	bookInitCmd.Flags().StringVar(&bookInitProvider, "provider", models.ProviderClaude, "LLM provider")
	bookInitCmd.Flags().StringVar(&bookInitLanguage, "language", "zh", "Story language")

	bookReadCmd.Flags().AddFlagSet(toolplane.ToolQueryCmd.Flags())

	bookWriteCmd.Flags().StringVar(&bookWriteMode, "mode", "patch", "Write mode: patch or replace")
	bookWriteCmd.Flags().StringVar(&bookWriteTarget, "target", "", "Section-specific target (craft kind, outline volume/chapter)")
	bookWriteCmd.Flags().StringVar(&bookWriteID, "id", "", "Target artifact ID")
	bookWriteCmd.Flags().BoolVar(&bookWriteApply, "apply", false, "Write the artifact; default is a dry run")
	bookWriteCmd.Flags().BoolVar(&bookWriteDryRun, "dry-run", false, "Force a dry run even with --apply")
	bookWriteCmd.Flags().BoolVar(&bookWriteRefresh, "refresh-derived", false, "Refresh derived chapter DSL after a chapter patch")
	bookWriteCmd.Flags().IntVar(&bookWriteMaxIssues, "max-issues", 0, "Cap returned check issues (0 = no cap)")
	bookWriteCmd.Flags().IntVar(&bookWriteTargetWords, "target-words", 0, "Override target narrative units for chapter checks")

	bookRefreshCmd.Flags().AddFlagSet(toolplane.ToolRefreshCmd.Flags())

	bookCmd.AddCommand(bookInitCmd)
	bookCmd.AddCommand(bookStatusCmd)
	bookCmd.AddCommand(bookReadCmd)
	bookCmd.AddCommand(bookWriteCmd)
	bookCmd.AddCommand(bookRefreshCmd)
}

// NewCommand returns the `novelgen book` command tree.
func NewCommand() *cobra.Command {
	return bookCmd
}

func runBookInit(cmd *cobra.Command, args []string) error {
	if bookInitParts <= 0 || bookInitVolumes <= 0 || bookInitChapter <= 0 {
		return fmt.Errorf("--parts, --volumes, and --chapter must be positive")
	}
	target, err := project.CommandDir(cmd)
	if err != nil {
		return err
	}
	book := project.Open(target)
	config, err := projectinit.Create(book, projectinit.Options{
		Name:     args[0],
		Parts:    bookInitParts,
		Volumes:  bookInitVolumes,
		Chapters: bookInitChapter,
		Genre:    bookInitGenre,
		Provider: bookInitProvider,
		Model:    bookInitMode,
		Language: bookInitLanguage,
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Created %s: %d parts x %d volumes x %d chapters (%d total)\n",
		config.Name, config.Structure.TargetParts, config.Structure.TargetVolumes,
		config.Structure.TargetChapters, config.Structure.TotalChapters())
	fmt.Fprintf(out, "Project: %s\n", book.Root().Dir())
	fmt.Fprintln(out, "Next: generate setup, then outline, then chapters.")
	return nil
}

func runBookStatus(cmd *cobra.Command, _ []string) error {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	status, err := book.Status()
	if err != nil {
		return err
	}
	if bookStatusJSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	written := 0
	for _, chapter := range status.Chapters {
		if chapter.Written {
			written++
		}
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Project: %s (%s)\n", status.Name, status.Root)
	fmt.Fprintf(out, "Setup: %t\n", status.Setup)
	fmt.Fprintf(out, "Structure: %d parts / %d volumes / %d chapters, %d written\n",
		status.Outline.Parts, status.Outline.Volumes, status.Outline.Chapters, written)
	fmt.Fprintf(out, "Next: %s", status.Next.Stage)
	if status.Next.TargetID != "" {
		fmt.Fprintf(out, " %s", status.Next.TargetID)
	}
	if status.Next.Reason != "" {
		fmt.Fprintf(out, " (%s)", status.Next.Reason)
	}
	fmt.Fprintln(out)
	for _, warning := range status.Warnings {
		fmt.Fprintf(out, "warning: %s\n", warning)
	}
	return nil
}

func runBookWrite(cmd *cobra.Command, args []string) error {
	section := toolplane.NormalizeKey(args[0])
	switch section {
	case "setup", "outline", "craft", "chapter", "recap":
	default:
		return fmt.Errorf("unsupported write section %q; valid sections: setup, outline, craft, chapter, recap", section)
	}
	mode := strings.ToLower(strings.TrimSpace(bookWriteMode))
	if mode == "" {
		mode = "patch"
	}
	target := strings.TrimSpace(bookWriteTarget)
	id := strings.TrimSpace(bookWriteID)
	apply := bookWriteApply && !bookWriteDryRun

	switch mode {
	case "replace":
		toolplane.ToolReplaceFlags.Target = target
		toolplane.ToolReplaceFlags.ID = id
		toolplane.ToolReplaceFlags.Apply = apply
		toolplane.ToolReplaceFlags.DryRun = bookWriteDryRun
		toolplane.ToolReplaceFlags.MaxIssues = bookWriteMaxIssues
		return toolplane.ToolReplaceCmd.RunE(cmd, []string{section})
	case "patch":
		toolplane.ToolPatchFlags.Target = target
		toolplane.ToolPatchFlags.ID = id
		toolplane.ToolPatchFlags.PatchJSON = ""
		toolplane.ToolPatchFlags.PatchBuffer = ""
		toolplane.ToolPatchFlags.Task = ""
		toolplane.ToolPatchFlags.Apply = apply
		toolplane.ToolPatchFlags.DryRun = bookWriteDryRun
		toolplane.ToolPatchFlags.RefreshDerived = bookWriteRefresh
		toolplane.ToolPatchFlags.TargetWords = bookWriteTargetWords
		toolplane.ToolPatchMaxIssues = bookWriteMaxIssues
		return toolplane.ToolPatchCmd.RunE(cmd, []string{section})
	default:
		return fmt.Errorf("unsupported --mode %q; valid modes: patch, replace", mode)
	}
}
