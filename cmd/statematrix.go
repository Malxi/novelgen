package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/prompts"

	"github.com/spf13/cobra"
)

var (
	statematrixChapterFlag string
	statematrixVolumeFlag  string
	statematrixPartFlag    string
	statematrixOutputFlag  string
)

var statematrixCmd = &cobra.Command{
	Use:   "statematrix",
	Short: "Generate state matrix for chapters (same format used by write command)",
	Long: `Generate the state matrix for each chapter in the same format used by the write command.

This helps you preview what context the AI will receive when writing each chapter.

Examples:
  novelgen statematrix                    # Generate for all chapters
  novelgen statematrix --chapter P1-V1-C1 # Generate for specific chapter
  novelgen statematrix --volume P1-V1     # Generate for specific volume
  novelgen statematrix --output state.md  # Save to file`,
	RunE: runStatematrix,
}

func init() {
	statematrixCmd.Flags().StringVar(&statematrixChapterFlag, "chapter", "", "Show state matrix for a specific chapter ID")
	statematrixCmd.Flags().StringVar(&statematrixVolumeFlag, "volume", "", "Show state matrix for all chapters in a volume")
	statematrixCmd.Flags().StringVar(&statematrixPartFlag, "part", "", "Show state matrix for all chapters in a part")
	statematrixCmd.Flags().StringVarP(&statematrixOutputFlag, "output", "o", "", "Output file path (default: stdout)")

	RegisterCommand(func() *cobra.Command {
		return statematrixCmd
	})
}

func runStatematrix(cmd *cobra.Command, args []string) error {
	logger.Section("State Matrix Analysis")

	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	chapters := filterChaptersForStateMatrix(outline)
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found")
	}

	logger.Info("Found %d chapters to analyze", len(chapters))

	manager := logic.NewStateMatrixManager("")

	var output strings.Builder

	for _, info := range chapters {
		state := manager.CalculateStateMatrix(outline, &info.Chapter)

		// Generate the same format used by write command
		stateText := prompts.FormatStateMatrix(state, &info.Chapter)

		output.WriteString(fmt.Sprintf("\n%s %s %s\n", strings.Repeat("=", 40), info.Chapter.ID, strings.Repeat("=", 40)))
		output.WriteString(fmt.Sprintf("Title: %s\n", info.Chapter.Title))
		output.WriteString(fmt.Sprintf("Part: %s | Volume: %s\n", info.PartID, info.VolumeID))
		output.WriteString("\n")
		output.WriteString(stateText)
		output.WriteString("\n")
	}

	result := output.String()

	// Output to file or stdout
	if statematrixOutputFlag != "" {
		err := os.WriteFile(statematrixOutputFlag, []byte(result), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		logger.Info("State matrix saved to: %s", statematrixOutputFlag)
	} else {
		fmt.Println(result)
	}

	return nil
}

func filterChaptersForStateMatrix(outline *models.Outline) []chapterStateInfo {
	var chapters []chapterStateInfo

	for _, part := range outline.Parts {
		if statematrixPartFlag != "" && part.ID != statematrixPartFlag {
			continue
		}

		for _, volume := range part.Volumes {
			if statematrixVolumeFlag != "" && volume.ID != statematrixVolumeFlag {
				continue
			}

			for i, chapter := range volume.Chapters {
				if statematrixChapterFlag != "" && chapter.ID != statematrixChapterFlag {
					continue
				}

				chapters = append(chapters, chapterStateInfo{
					PartID:    part.ID,
					VolumeID:  volume.ID,
					Chapter:   chapter,
					ChapterNo: i + 1,
				})
			}
		}
	}

	return chapters
}

type chapterStateInfo struct {
	PartID    string
	VolumeID  string
	Chapter   models.Chapter
	ChapterNo int
}
