package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/logic"
	"nolvegen/internal/models"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	composeRegenFlag   string
	composePromptFlag  string
	composeIterateFlag int
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Generate or improve a story outline",
	Long: `Generate a story outline with a rigid 3-level structure (parts → volumes → chapters),
including plot beats, conflict, and pacing to guide AI writing.

This command reads the story setup from config/init/story_setup.json and uses AI
to generate a hierarchical outline structure based on the predefined structure in novel.json.

Examples:
  novel compose                          # Generate full outline
  novel compose --regen 1_1_1            # Regenerate chapter 1.1.1
  novel compose --regen 1_1_1 --prompt "make it more intense"  # Regenerate with suggestion
  novel compose --iterate 3              # Generate and iterate 3 times for improvement
  novel compose --iterate 2              # Improve existing outline with 2 iterations`,
	RunE: runCompose,
}

func init() {
	composeCmd.Flags().StringVar(&composeRegenFlag, "regen", "", "Regenerate a specific part, volume, or chapter (e.g., \"1\", \"1_1\", \"1_1_1\")")
	composeCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Suggestions for regeneration (used with --regen)")
	composeCmd.Flags().IntVar(&composeIterateFlag, "iterate", 0, "Number of iterations for AI self-review and improvement (default: 0)")
	rootCmd.AddCommand(composeCmd)
}

func runCompose(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN COMPOSE")

	// Check if we're in a novel project
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novel init' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

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

	// Allow iteration on existing outline
	if _, err := os.Stat(outlinePath); err == nil && composeRegenFlag == "" && composeIterateFlag == 0 {
		return fmt.Errorf("outline already exists at %s. Use --regen to regenerate specific parts or --iterate to improve", outlinePath)
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
	} else if composeIterateFlag > 0 && outlineExists(outlinePath) {
		// Iterate on existing outline
		logger.Info("Loading existing outline for iteration")
		outline, err = models.LoadOutline(outlinePath)
		if err != nil {
			return fmt.Errorf("failed to load existing outline: %w", err)
		}
		if err := iterateOutlineImprovement(outline, setup, projectConfig, composeIterateFlag); err != nil {
			logger.Error("Iteration improvement failed: %v", err)
			return fmt.Errorf("iteration improvement failed: %w", err)
		}
	} else {
		// AI generation mode (default)
		outline, err = generateOutlineWithAI(setup, projectConfig)
		if err != nil {
			return fmt.Errorf("failed to generate outline with AI: %w", err)
		}

		// Iterative improvement if requested
		if composeIterateFlag > 0 {
			if err := iterateOutlineImprovement(outline, setup, projectConfig, composeIterateFlag); err != nil {
				logger.Error("Iteration improvement failed: %v", err)
				// Continue with original outline if iteration fails
			}
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
	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Get active provider and model
	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Story structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println()

	// Create LLM client and agent
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)

	return agent.GenerateOutlineWithStructure(setup, projectConfig.Structure, projectConfig.Language)
}

func regenerateElement(outline *models.Outline, id string, setup *models.StorySetup, projectConfig *models.ProjectConfig) error {
	parts := strings.Split(id, "_")

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
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

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)

	// Create IDManager for ID resolution
	idManager := logic.NewIDManager(outline)

	switch len(parts) {
	case 1:
		// Regenerate a part
		partNum, _ := strconv.Atoi(parts[0])
		partID := idManager.GeneratePartID(partNum)
		part := idManager.GetPartByID(partID)
		if part == nil {
			return fmt.Errorf("part %s not found", partID)
		}
		fmt.Printf("Regenerating part: %s\n", partID)
		return agent.RegeneratePart(part, outline, setup, projectConfig.Language, userPrompt)

	case 2:
		// Regenerate a volume
		partNum, _ := strconv.Atoi(parts[0])
		volNum, _ := strconv.Atoi(parts[1])
		volumeID := idManager.GenerateVolumeID(partNum, volNum)
		volume, _ := idManager.GetVolumeByID(volumeID)
		if volume == nil {
			return fmt.Errorf("volume %s not found", volumeID)
		}
		fmt.Printf("Regenerating volume: %s\n", volumeID)
		return agent.RegenerateVolume(volume, outline, setup, projectConfig.Language, userPrompt)

	case 3:
		// Regenerate a chapter
		partNum, _ := strconv.Atoi(parts[0])
		volNum, _ := strconv.Atoi(parts[1])
		chapNum, _ := strconv.Atoi(parts[2])
		chapterID := idManager.GenerateChapterID(partNum, volNum, chapNum)
		chapter, _, _ := idManager.GetChapterByID(chapterID)
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
	// Use the ToMarkdown method to ensure all fields are included
	return os.WriteFile(path, []byte(outline.ToMarkdown()), 0644)
}

func outlineExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// iterateOutlineImprovement runs the review-improvement loop
func iterateOutlineImprovement(outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, maxIterations int) error {
	logger.Section("Outline Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create iteration agent
	iterationAgent := agents.NewIterationAgent(client, cfg, &projectConfig.LLM)

	currentIteration := 0
	for currentIteration < maxIterations {
		currentIteration++
		logger.Section(fmt.Sprintf("Iteration %d/%d", currentIteration, maxIterations))

		// Review the outline
		review, err := iterationAgent.ReviewOutline(outline, setup, currentIteration)
		if err != nil {
			logger.Error("Review failed: %v", err)
			return err
		}

		// Check if we should continue
		if !agents.ShouldContinueIteration(review, currentIteration, maxIterations) {
			logger.Info("Stopping iteration - quality threshold met or no critical issues")
			break
		}

		// Apply improvements
		if err := iterationAgent.ApplyImprovements(outline, review, setup, projectConfig.Language); err != nil {
			logger.Error("Failed to apply improvements: %v", err)
			// Continue to next iteration even if some improvements fail
		}

		// Save intermediate result
		outlinePath := filepath.Join("config", "compose", fmt.Sprintf("outline_iter_%d.json", currentIteration))
		if err := outline.Save(outlinePath); err != nil {
			logger.Error("Failed to save intermediate outline: %v", err)
		} else {
			logger.Info("Saved intermediate outline to %s", outlinePath)
		}
	}

	logger.Section("Iteration Complete")
	logger.Info("Completed %d iterations", currentIteration)
	return nil
}
