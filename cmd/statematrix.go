package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	statematrixChapterFlag string
	statematrixVolumeFlag  string
	statematrixPartFlag    string
	statematrixFormatFlag  string
)

var statematrixCmd = &cobra.Command{
	Use:   "statematrix",
	Short: "Display state matrix for chapters",
	Long: `Display the state matrix for each chapter to analyze story progression.

This command reads the outline and displays the state changes for:
  - Characters: Character states and attributes
  - Relationships: Character relationship changes
  - Items: Item acquisitions and changes
  - Goals: Character goal changes
  - Storylines: Storyline progression
  - Gates: Gate/obstacle states
  - Status: Character status effects
  - Memories: Character memories/knowledge

Examples:
  # Show state matrix for all chapters
  novelgen statematrix

  # Show state matrix for a specific chapter
  novelgen statematrix --chapter P1-V1-C1

  # Show state matrix for all chapters in a volume
  novelgen statematrix --volume P1-V1

  # Show state matrix in JSON format
  novelgen statematrix --format json`,
	RunE: runStatematrix,
}

func init() {
	statematrixCmd.Flags().StringVar(&statematrixChapterFlag, "chapter", "", "Show state matrix for a specific chapter ID")
	statematrixCmd.Flags().StringVar(&statematrixVolumeFlag, "volume", "", "Show state matrix for all chapters in a volume")
	statematrixCmd.Flags().StringVar(&statematrixPartFlag, "part", "", "Show state matrix for all chapters in a part")
	statematrixCmd.Flags().StringVar(&statematrixFormatFlag, "format", "table", "Output format: table, json, or markdown")

	RegisterCommand(func() *cobra.Command {
		return statematrixCmd
	})
}

func runStatematrix(cmd *cobra.Command, args []string) error {
	logger.Section("State Matrix Analysis")

	// Load outline
	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Filter chapters based on flags
	chapters := filterChaptersForStateMatrix(outline)

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found matching the specified criteria")
	}

	logger.Info("Found %d chapters to analyze", len(chapters))

	// Display based on format
	switch statematrixFormatFlag {
	case "json":
		return displayStateMatrixAsJSON(chapters)
	case "markdown":
		return displayStateMatrixAsMarkdown(chapters)
	default:
		return displayStateMatrixAsTable(chapters)
	}
}

func filterChaptersForStateMatrix(outline *models.Outline) []chapterStateInfo {
	var chapters []chapterStateInfo

	for _, part := range outline.Parts {
		// Filter by part if specified
		if statematrixPartFlag != "" && part.ID != statematrixPartFlag {
			continue
		}

		for _, volume := range part.Volumes {
			// Filter by volume if specified
			if statematrixVolumeFlag != "" && volume.ID != statematrixVolumeFlag {
				continue
			}

			for i, chapter := range volume.Chapters {
				// Filter by chapter if specified
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

func displayStateMatrixAsTable(chapters []chapterStateInfo) error {
	for _, info := range chapters {
		fmt.Printf("\n%s %s %s\n", strings.Repeat("=", 60), info.Chapter.ID, strings.Repeat("=", 60))
		fmt.Printf("Title: %s\n", info.Chapter.Title)
		fmt.Printf("Location: %s\n", info.Chapter.Location)
		fmt.Printf("Characters: %s\n", strings.Join(info.Chapter.Characters, ", "))
		fmt.Println()

		// Display events (state changes)
		if len(info.Chapter.Events) > 0 {
			fmt.Println("State Changes (Events):")
			fmt.Println()

			// Group events by type
			eventsByType := make(map[string][]models.Event)
			for _, event := range info.Chapter.Events {
				eventsByType[event.Type] = append(eventsByType[event.Type], event)
			}

			// Display in order of importance
			eventTypes := []string{"premise", "status", "item", "relationship", "goal", "storyline", "gate", "memory"}
			for _, eventType := range eventTypes {
				if events, ok := eventsByType[eventType]; ok {
					fmt.Printf("  [%s]\n", strings.ToUpper(eventType))
					for _, event := range events {
						displayStateMatrixEvent(&event)
					}
					fmt.Println()
				}
			}

			// Display any other event types
			for eventType, events := range eventsByType {
				if !contains(eventTypes, eventType) {
					fmt.Printf("  [%s]\n", strings.ToUpper(eventType))
					for _, event := range events {
						displayStateMatrixEvent(&event)
					}
					fmt.Println()
				}
			}
		}

		// Display beats
		if len(info.Chapter.Beats) > 0 {
			fmt.Println("Plot Beats:")
			for i, beat := range info.Chapter.Beats {
				fmt.Printf("  %d. %s\n", i+1, beat)
			}
			fmt.Println()
		}

		// Display state change summary
		if info.Chapter.StateChange != "" {
			fmt.Printf("State Change Summary: %s\n", info.Chapter.StateChange)
		}
		if info.Chapter.Conflict != "" {
			fmt.Printf("Conflict: %s\n", info.Chapter.Conflict)
		}
		if info.Chapter.Pacing != "" {
			fmt.Printf("Pacing: %s\n", info.Chapter.Pacing)
		}

		fmt.Println(strings.Repeat("-", 140))
	}

	return nil
}

func displayStateMatrixEvent(event *models.Event) {
	fmt.Printf("    • %s: %s\n", event.Subject, event.Change)
	if event.Details != "" {
		// Wrap details to fit terminal width
		details := wrapText(event.Details, 70)
		for _, line := range strings.Split(details, "\n") {
			fmt.Printf("      %s\n", line)
		}
	}
}

func displayStateMatrixAsJSON(chapters []chapterStateInfo) error {
	output := struct {
		Chapters []chapterStateInfo `json:"chapters"`
	}{
		Chapters: chapters,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func displayStateMatrixAsMarkdown(chapters []chapterStateInfo) error {
	fmt.Println("# State Matrix Report")
	fmt.Println()

	for _, info := range chapters {
		fmt.Printf("## %s: %s\n\n", info.Chapter.ID, info.Chapter.Title)
		fmt.Printf("**Summary:** %s\n\n", info.Chapter.Summary)
		fmt.Printf("**Location:** %s\n\n", info.Chapter.Location)
		fmt.Printf("**Characters:** %s\n\n", strings.Join(info.Chapter.Characters, ", "))

		if len(info.Chapter.Events) > 0 {
			fmt.Println("### State Changes\n")

			// Group events by type
			eventsByType := make(map[string][]models.Event)
			for _, event := range info.Chapter.Events {
				eventsByType[event.Type] = append(eventsByType[event.Type], event)
			}

			// Sort event types
			var eventTypes []string
			for eventType := range eventsByType {
				eventTypes = append(eventTypes, eventType)
			}
			sort.Strings(eventTypes)

			for _, eventType := range eventTypes {
				fmt.Printf("#### %s\n\n", strings.Title(eventType))
				for _, event := range eventsByType[eventType] {
					fmt.Printf("- **%s**: %s\n", event.Subject, event.Change)
					if event.Details != "" {
						fmt.Printf("  - %s\n", event.Details)
					}
				}
				fmt.Println()
			}
		}

		if len(info.Chapter.Beats) > 0 {
			fmt.Println("### Plot Beats\n")
			for i, beat := range info.Chapter.Beats {
				fmt.Printf("%d. %s\n", i+1, beat)
			}
			fmt.Println()
		}

		if info.Chapter.StateChange != "" {
			fmt.Printf("**State Change:** %s\n\n", info.Chapter.StateChange)
		}
		if info.Chapter.Conflict != "" {
			fmt.Printf("**Conflict:** %s\n\n", info.Chapter.Conflict)
		}

		fmt.Println("---\n")
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > width {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(currentLine)
			currentLine = word
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}

	if currentLine != "" {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(currentLine)
	}

	return result.String()
}
