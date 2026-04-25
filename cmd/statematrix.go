package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	statematrixChapterFlag            string
	statematrixVolumeFlag             string
	statematrixPartFlag               string
	statematrixOutputFlag             string
	statematrixRPGOnlyFlag            bool
	statematrixCompareFlag            bool
	statematrixJSONFlag               bool
	statematrixCompareMinSeverityFlag string
)

var statematrixCmd = &cobra.Command{
	Use:     "statematrix",
	Aliases: []string{"rpgstate", "state"},
	Short:   "Generate state matrix for chapters (same format used by write command)",
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
	statematrixCmd.Flags().BoolVar(&statematrixRPGOnlyFlag, "rpg-only", false, "Only output the structured RPG state section")
	statematrixCmd.Flags().BoolVar(&statematrixCompareFlag, "compare", false, "Compare outline-expected RPG state with DSL-observed RPG state")
	statematrixCmd.Flags().BoolVar(&statematrixJSONFlag, "json", false, "Output structured JSON instead of formatted text")
	statematrixCmd.Flags().StringVar(&statematrixCompareMinSeverityFlag, "compare-min-severity", "warning", "Minimum drift severity for --compare: info, warning, critical")

	RegisterCommand(func() *cobra.Command {
		return statematrixCmd
	})
}

func runStatematrix(cmd *cobra.Command, args []string) error {
	if !statematrixJSONFlag {
		logger.Section("State Matrix Analysis")
	}

	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	chapters := filterChaptersForStateMatrix(outline)
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found")
	}

	if !statematrixJSONFlag {
		logger.Info("Found %d chapters to analyze", len(chapters))
	}

	manager := logic.NewStateMatrixManager(".")
	expectedManager := logic.NewStateMatrixManager(".")
	expectedManager.SetUseRPGDSL(false)

	var output strings.Builder
	var records []stateMatrixOutputRecord

	for _, info := range chapters {
		state := manager.CalculateStateMatrix(outline, &info.Chapter)

		stateText := logic.FormatStateMatrix(state, &info.Chapter)
		if statematrixRPGOnlyFlag {
			stateText = logic.FormatRPGState(state.RPG, &info.Chapter)
		}
		if statematrixCompareFlag {
			expected := expectedManager.CalculateStateMatrix(outline, &info.Chapter)
			comparison := logic.CompareRPGStates(expected.RPG, state.RPG, &info.Chapter)
			comparison = logic.FilterRPGStateComparison(comparison, statematrixCompareMinSeverityFlag)
			if statematrixJSONFlag {
				records = append(records, stateMatrixOutputRecord{
					ChapterID:  info.Chapter.ID,
					Title:      info.Chapter.Title,
					PartID:     info.PartID,
					VolumeID:   info.VolumeID,
					Comparison: &comparison,
				})
				continue
			}
			stateText = logic.FormatRPGStateComparison(comparison)
		} else if statematrixJSONFlag {
			record := stateMatrixOutputRecord{
				ChapterID: info.Chapter.ID,
				Title:     info.Chapter.Title,
				PartID:    info.PartID,
				VolumeID:  info.VolumeID,
			}
			if statematrixRPGOnlyFlag {
				record.RPG = state.RPG
			} else {
				record.State = state
			}
			records = append(records, record)
			continue
		}

		output.WriteString(fmt.Sprintf("\n%s %s %s\n", strings.Repeat("=", 40), info.Chapter.ID, strings.Repeat("=", 40)))
		output.WriteString(fmt.Sprintf("Title: %s\n", info.Chapter.Title))
		output.WriteString(fmt.Sprintf("Part: %s | Volume: %s\n", info.PartID, info.VolumeID))
		output.WriteString("\n")
		output.WriteString(stateText)
		output.WriteString("\n")
	}

	result := output.String()
	if statematrixJSONFlag {
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		result = string(data) + "\n"
	}

	// Output to file or stdout
	if statematrixOutputFlag != "" {
		err := os.WriteFile(statematrixOutputFlag, []byte(result), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		if !statematrixJSONFlag {
			logger.Info("State matrix saved to: %s", statematrixOutputFlag)
		}
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

type stateMatrixOutputRecord struct {
	ChapterID  string                    `json:"chapter_id"`
	Title      string                    `json:"title,omitempty"`
	PartID     string                    `json:"part_id,omitempty"`
	VolumeID   string                    `json:"volume_id,omitempty"`
	State      *models.StateMatrix       `json:"state,omitempty"`
	RPG        *models.RPGState          `json:"rpg,omitempty"`
	Comparison *logic.RPGStateComparison `json:"comparison,omitempty"`
}
