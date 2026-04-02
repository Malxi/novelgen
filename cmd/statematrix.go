package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	statematrixChapterFlag string
	statematrixVolumeFlag  string
	statematrixPartFlag    string
)

var statematrixCmd = &cobra.Command{
	Use:   "statematrix",
	Short: "Display state matrix for chapters",
	Long: `Display the state matrix for each chapter to analyze story progression.

Examples:
  novelgen statematrix
  novelgen statematrix --chapter P1-V1-C1
  novelgen statematrix --volume P1-V1`,
	RunE: runStatematrix,
}

func init() {
	statematrixCmd.Flags().StringVar(&statematrixChapterFlag, "chapter", "", "Show state matrix for a specific chapter ID")
	statematrixCmd.Flags().StringVar(&statematrixVolumeFlag, "volume", "", "Show state matrix for all chapters in a volume")
	statematrixCmd.Flags().StringVar(&statematrixPartFlag, "part", "", "Show state matrix for all chapters in a part")

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

	for _, info := range chapters {
		state := manager.CalculateStateMatrix(outline, &info.Chapter)
		displayChapterState(info, state)
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

func displayChapterState(info chapterStateInfo, state *models.StateMatrix) {
	fmt.Printf("\n%s %s %s\n", strings.Repeat("=", 60), info.Chapter.ID, strings.Repeat("=", 60))
	fmt.Printf("Title: %s\n", info.Chapter.Title)
	fmt.Printf("Location: %s\n", info.Chapter.Location)
	fmt.Printf("Characters: %s\n", strings.Join(info.Chapter.Characters, ", "))
	fmt.Println()

	// Display current chapter's events
	if len(info.Chapter.Events) > 0 {
		fmt.Println("Chapter Events:")
		for _, event := range info.Chapter.Events {
			fmt.Printf("  [%s] %s: %s\n", event.Type, event.Subject, event.Change)
		}
		fmt.Println()
	}

	// Display cumulative state
	if state != nil {
		displayCumulativeState(state)
	}

	fmt.Println(strings.Repeat("-", 140))
}

func displayCumulativeState(state *models.StateMatrix) {
	fmt.Println("Cumulative State:")

	if len(state.Goals) > 0 {
		fmt.Println("  Goals:")
		var chars []string
		for char := range state.Goals {
			chars = append(chars, char)
		}
		sort.Strings(chars)
		for _, char := range chars {
			for _, goal := range state.Goals[char] {
				fmt.Printf("    - %s: %s\n", char, goal)
			}
		}
	}

	if len(state.Items) > 0 {
		fmt.Println("  Items:")
		for name, item := range state.Items {
			if item.Owner != "" {
				fmt.Printf("    - %s (owned by %s)\n", name, item.Owner)
			}
		}
	}

	if len(state.Status) > 0 {
		fmt.Println("  Statuses:")
		var keys []string
		for key := range state.Status {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			status := state.Status[key]
			// Try to extract character and status type from key
			// Key format: either "char_statusType" or "statusDescription"
			if idx := strings.Index(key, "_"); idx > 0 {
				charName := key[:idx]
				statusType := key[idx+1:]
				if statusType == charName {
					// If statusType is same as charName, just show charName
					fmt.Printf("    - %s: %s\n", charName, status.State)
				} else {
					fmt.Printf("    - %s (%s): %s\n", charName, statusType, status.State)
				}
			} else {
				// No underscore, show as is
				fmt.Printf("    - %s: %s\n", key, status.State)
			}
		}
	}

	if len(state.Relationships) > 0 {
		fmt.Println("  Relationships:")
		for key, rel := range state.Relationships {
			parts := strings.Split(key, "_")
			if len(parts) == 2 {
				fmt.Printf("    - %s ↔ %s: %s\n", parts[0], parts[1], rel)
			}
		}
	}

	fmt.Println()
}
