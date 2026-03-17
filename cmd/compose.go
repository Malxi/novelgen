package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/models"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	composeRegenFlag  string
	composePromptFlag string
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Generate a story outline",
	Long: `Generate a story outline with a rigid 3-level structure (parts → volumes → chapters),
including plot beats, conflict, and pacing to guide AI writing.

This command reads the story setup from config/init/story_setup.json and uses AI
to generate a hierarchical outline structure based on the predefined structure in novel.json.

Examples:
  novel compose                          # Generate full outline
  novel compose --regen 1_1_1            # Regenerate chapter 1.1.1
  novel compose --regen 1_1_1 --prompt "make it more intense"  # Regenerate with suggestion`,
	RunE: runCompose,
}

func init() {
	composeCmd.Flags().StringVar(&composeRegenFlag, "regen", "", "Regenerate a specific part, volume, or chapter (e.g., \"1\", \"1_1\", \"1_1_1\")")
	composeCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Suggestions for regeneration (used with --regen)")
	rootCmd.AddCommand(composeCmd)
}

func runCompose(cmd *cobra.Command, args []string) error {
	// Check if we're in a novel project
	if _, err := os.Stat("novel.json"); err != nil {
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novel init' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		return fmt.Errorf("failed to load novel.json: %w", err)
	}

	// Check if story_setup.json exists
	setupPath := filepath.Join("config", "init", "story_setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		return fmt.Errorf("story setup not found at %s. Run 'novel init' first", setupPath)
	}

	// Load story setup
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Check if outline already exists
	outlinePath := filepath.Join("config", "compose", "outline.json")
	var outline *models.Outline

	if _, err := os.Stat(outlinePath); err == nil && composeRegenFlag == "" {
		return fmt.Errorf("outline already exists at %s. Use --regen to regenerate specific parts", outlinePath)
	}

	if composeRegenFlag != "" {
		// Regenerate specific element
		outline, err = models.LoadOutline(outlinePath)
		if err != nil {
			return fmt.Errorf("failed to load existing outline: %w", err)
		}
		if err := regenerateElement(outline, composeRegenFlag, setup, projectConfig); err != nil {
			return fmt.Errorf("failed to regenerate element: %w", err)
		}
	} else {
		// AI generation mode (default)
		outline, err = generateOutlineWithAI(setup, projectConfig)
		if err != nil {
			return fmt.Errorf("failed to generate outline with AI: %w", err)
		}
	}

	// Save outline
	if err := outline.Save(outlinePath); err != nil {
		return fmt.Errorf("failed to save outline: %w", err)
	}

	// Create markdown version
	mdPath := filepath.Join("config", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	// Print summary
	fmt.Printf("\n✓ Story outline saved to %s\n", outlinePath)
	fmt.Printf("\n📊 Story Structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit config/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novel worldbuild' to create world elements")

	return nil
}

func generateOutlineWithAI(setup *models.StorySetup, projectConfig *models.ProjectConfig) (*models.Outline, error) {
	// Load or create LLM config
	llmConfig, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	fmt.Printf("Using model: %s at %s\n", llmConfig.Model, llmConfig.BaseURL)
	fmt.Printf("Story structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println()

	// Create LLM client and agent
	client := llmConfig.CreateClient()
	agent := agents.NewComposeAgent(client, llmConfig)

	return agent.GenerateOutlineWithStructure(setup, projectConfig.Structure, projectConfig.Language)
}

func regenerateElement(outline *models.Outline, id string, setup *models.StorySetup, projectConfig *models.ProjectConfig) error {
	parts := strings.Split(id, "_")

	// Load LLM config
	llmConfig, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Get user prompt for regeneration (from --prompt flag or interactive)
	userPrompt := composePromptFlag
	if userPrompt == "" {
		var err error
		userPrompt, err = getRegenPrompt()
		if err != nil {
			return fmt.Errorf("failed to get regeneration prompt: %w", err)
		}
	}

	client := llmConfig.CreateClient()
	agent := agents.NewComposeAgent(client, llmConfig)

	switch len(parts) {
	case 1:
		// Regenerate a part
		partID := "part_" + parts[0]
		part := outline.GetPartByID(partID)
		if part == nil {
			return fmt.Errorf("part %s not found", partID)
		}
		fmt.Printf("Regenerating part: %s\n", partID)
		return agent.RegeneratePart(part, outline, setup, projectConfig.Language, userPrompt)

	case 2:
		// Regenerate a volume
		volumeID := fmt.Sprintf("vol_%s_%s", parts[0], parts[1])
		volume := outline.GetVolumeByID(volumeID)
		if volume == nil {
			return fmt.Errorf("volume %s not found", volumeID)
		}
		fmt.Printf("Regenerating volume: %s\n", volumeID)
		return agent.RegenerateVolume(volume, outline, setup, projectConfig.Language, userPrompt)

	case 3:
		// Regenerate a chapter
		chapterID := fmt.Sprintf("chap_%s_%s_%s", parts[0], parts[1], parts[2])
		chapter := outline.GetChapterByID(chapterID)
		if chapter == nil {
			return fmt.Errorf("chapter %s not found", chapterID)
		}
		fmt.Printf("Regenerating chapter: %s\n", chapterID)
		return agent.RegenerateChapter(chapter, outline, setup, projectConfig.Language, userPrompt)

	default:
		return fmt.Errorf("invalid ID format: %s (expected format: \"1\", \"1_1\", or \"1_1_1\")", id)
	}
}

func getRegenPrompt() (string, error) {
	fmt.Println("\n💡 Regeneration Prompt")
	fmt.Println("======================")
	fmt.Println("Enter your suggestions for regeneration (e.g., 'make it more intense', 'add a plot twist')")
	fmt.Println("Press Enter to skip and use default regeneration:")

	promptPrompt := &survey.Multiline{
		Message: "Your suggestions:",
	}

	var prompt string
	if err := survey.AskOne(promptPrompt, &prompt); err != nil {
		return "", err
	}

	return strings.TrimSpace(prompt), nil
}

func createOutlineMarkdown(outline *models.Outline, path string) error {
	var content strings.Builder

	content.WriteString("# Story Outline\n\n")

	for _, part := range outline.Parts {
		content.WriteString(fmt.Sprintf("## %s: %s\n\n", part.ID, part.Title))
		content.WriteString(fmt.Sprintf("**Summary:** %s\n\n", part.Summary))

		for _, volume := range part.Volumes {
			content.WriteString(fmt.Sprintf("### %s: %s\n\n", volume.ID, volume.Title))
			content.WriteString(fmt.Sprintf("**Summary:** %s\n\n", volume.Summary))

			for _, chapter := range volume.Chapters {
				content.WriteString(fmt.Sprintf("#### %s: %s\n\n", chapter.ID, chapter.Title))
				content.WriteString(fmt.Sprintf("**Summary:** %s\n\n", chapter.Summary))

				if len(chapter.Beats) > 0 {
					content.WriteString("**Plot Beats:**\n")
					for _, beat := range chapter.Beats {
						content.WriteString(fmt.Sprintf("- %s\n", beat))
					}
					content.WriteString("\n")
				}

				if chapter.Conflict != "" {
					content.WriteString(fmt.Sprintf("**Conflict:** %s\n\n", chapter.Conflict))
				}

				if chapter.Pacing != "" {
					content.WriteString(fmt.Sprintf("**Pacing:** %s\n\n", chapter.Pacing))
				}
			}
		}
	}

	return os.WriteFile(path, []byte(content.String()), 0644)
}

func splitLinesAndTrim(s string) []string {
	parts := strings.Split(s, "\n")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
