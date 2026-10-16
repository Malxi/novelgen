// Package export implements the `novelgen export` command tree.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"

	"github.com/spf13/cobra"
)

var (
	exportFormatFlag string
	exportOutputFlag string
)

// reBlankRun collapses the blank-line runs left behind when markdown rules are
// dropped, so a rule between two paragraphs still yields a single paragraph
// break instead of a double one.
var reBlankRun = regexp.MustCompile(`\n{3,}`)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export novel to various formats",
	Long: `Export the completed novel to various formats.

This command reads all generated final chapters from the chapters/ directory
and compiles them into a single file with proper formatting.

Supported formats:
  - markdown (.md): Formatted with headers and metadata
  - text (.txt): Plain text format

Subcommands:
  novel - Export the complete novel`,
}

var exportNovelCmd = &cobra.Command{
	Use:   "novel",
	Short: "Export the complete novel",
	Long: `Export all chapters as a complete novel.

Examples:
  # Export to markdown (default)
  novelgen export novel

  # Export to text format
  novelgen export novel --format txt

  # Export with custom filename
  novelgen export novel --output my_novel.md

  # Export to specific directory
  novelgen export novel --output ./exports/my_novel.md`,
	RunE: runExportNovel,
}

func init() {
	exportCmd.AddCommand(exportNovelCmd)

	exportNovelCmd.Flags().StringVar(&exportFormatFlag, "format", "md", "Export format (md, txt)")
	exportNovelCmd.Flags().StringVar(&exportOutputFlag, "output", "", "Output file path (default: <project_name>.<format>)")

}

// NewCommand returns the `novelgen export` command tree.
func NewCommand() *cobra.Command {
	return exportCmd
}

func runExportNovel(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := project.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup for metadata
	setup, err := project.LoadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline for chapter order
	outline, err := project.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Get all chapters in order
	chapters := project.AllChapters(outline)
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found in outline")
	}

	log.Info("Exporting %d chapters", len(chapters))

	// Build novel content
	content, err := buildNovelContent(config, setup, outline, chapters, exportFormatFlag)
	if err != nil {
		return fmt.Errorf("failed to build novel content: %w", err)
	}

	// Determine output path
	outputPath := exportOutputFlag
	if outputPath == "" {
		ext := exportFormatFlag
		if ext == "txt" {
			outputPath = fmt.Sprintf("%s.txt", config.Name)
		} else {
			outputPath = fmt.Sprintf("%s.md", config.Name)
		}
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." && outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	// Count words
	wordCount := len(strings.Fields(content))
	log.Info("Novel exported successfully: %s (%d words)", outputPath, wordCount)

	return nil
}

// buildNovelContent compiles all chapters into a single document
func buildNovelContent(config *models.ProjectConfig, setup *models.StorySetup, outline *models.Outline, chapters []*models.Chapter, format string) (string, error) {
	var sb strings.Builder

	isTXT := format == "txt"

	// Write title
	if isTXT {
		sb.WriteString(fmt.Sprintf("%s\n\n", setup.ProjectName))
	} else {
		sb.WriteString(fmt.Sprintf("# %s\n\n", setup.ProjectName))
	}

	// Write each chapter
	root, err := project.FindRoot()
	if err != nil {
		return "", err
	}

	for i, chapter := range chapters {
		// Add blank line between chapters
		if i > 0 {
			sb.WriteString("\n\n")
		}

		// Load chapter content
		chapterContent, err := loadChapterFile(root, chapter.ID)
		if err != nil {
			// Skip if chapter not generated
			continue
		}

		// Add chapter number and title
		chapterNum := i + 1
		if isTXT {
			sb.WriteString(fmt.Sprintf("第%d章 %s\n\n", chapterNum, chapter.Title))
			// Remove markdown headers and formatting for plain text
			cleanContent := cleanMarkdownForTXT(chapterContent)
			sb.WriteString(cleanContent)
		} else {
			sb.WriteString(fmt.Sprintf("## 第%d章 %s\n\n", chapterNum, chapter.Title))
			sb.WriteString(chapterContent)
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// cleanMarkdownForTXT removes markdown formatting for plain text output
func cleanMarkdownForTXT(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines at the start
		if trimmed == "" && result.Len() == 0 {
			continue
		}

		// Remove markdown headers (# ## ###)
		if strings.HasPrefix(trimmed, "#") {
			// Keep the text after the #
			text := strings.TrimLeft(trimmed, "#")
			text = strings.TrimSpace(text)
			if text != "" {
				result.WriteString(text)
				result.WriteString("\n")
			}
			continue
		}

		// Drop horizontal rules. This must happen before the emphasis markers
		// are stripped, otherwise "***" would turn into a blank paragraph.
		if isMarkdownHorizontalRule(trimmed) {
			continue
		}

		// Keep blank lines: they are the paragraph breaks of the source file.
		if trimmed == "" {
			result.WriteString("\n")
			continue
		}

		// Remove bold/italic markers
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "__", "")
		line = strings.ReplaceAll(line, "_", "")

		result.WriteString(line)
		result.WriteString("\n")
	}

	return reBlankRun.ReplaceAllString(strings.TrimSpace(result.String()), "\n\n")
}

// isMarkdownHorizontalRule reports whether a trimmed line is a markdown rule
// such as "---", "* * *" or "___".
func isMarkdownHorizontalRule(trimmed string) bool {
	compact := strings.ReplaceAll(strings.ReplaceAll(trimmed, " ", ""), "\t", "")
	if len(compact) < 3 {
		return false
	}
	for _, marker := range []string{"-", "*", "_"} {
		if strings.Trim(compact, marker) == "" {
			return true
		}
	}
	return false
}

// loadChapterFile loads a chapter file from the chapters directory
func loadChapterFile(root, chapterID string) (string, error) {
	project := store.At(root)
	content, _, err := project.LoadChapterContent(chapterID)
	if err != nil {
		return "", err
	}
	if content != "" {
		return content, nil
	}

	return "", fmt.Errorf("chapter file not found: %s", chapterID)
}
