package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	composeIDFlag           string
	composePromptFlag       string
	composeMaxRoundsFlag    int
	composeConcurrencyFlag  int
	composeHierarchicalFlag bool
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Generate story outline",
	Long: `Generate a story outline with a rigid 3-level structure (parts → volumes → chapters).

The outline includes detailed information for each chapter:
  - Summary: What happens in this chapter
  - Characters: Who appears in this chapter
  - Location: Where the chapter takes place
  - Events: State changes (relationships, goals, items, etc.)
  - Beats: Key plot points
  - Conflict: Central tension
  - Pacing: Chapter rhythm

This command reads story/setup/story_setup.json and generates story/compose/outline.json.

Subcommands:
  gen     - Generate new outline
  regen   - Regenerate specific part/volume/chapter
  improve - Improve existing outline through AI review`,
}

var composeGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate new story outline",
	Long: `Generate a new story outline based on story setup.

This command reads story/setup/story_setup.json and uses AI to generate
a hierarchical outline structure based on the predefined structure in novel.json.

Examples:
  novelgen compose gen                      # Generate full outline (one-shot)
  novelgen compose gen --hierarchical       # Generate using hierarchical approach (better quality)`,
	RunE: runComposeGen,
}

var composeRegenCmd = &cobra.Command{
	Use:   "regen [id]",
	Short: "Regenerate specific part, volume, or chapter",
	Long: `Regenerate a specific part, volume, or chapter from the existing outline.

The ID format is:
  - "1"       - Part 1
  - "1_1"     - Part 1, Volume 1
  - "1_1_1"   - Part 1, Volume 1, Chapter 1

Examples:
  novelgen compose regen 1_1_1              # Regenerate chapter 1.1.1
  novelgen compose regen 1_1_1 --prompt "make it more intense"
  novelgen compose regen 1_1 --prompt "add more conflict"`,
	Args: cobra.ExactArgs(1),
	RunE: runComposeRegen,
}

var composeImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve existing outline through AI review",
	Long: `Improve an existing outline by running AI review and enhancement cycles.

This command loads the current outline and runs multiple rounds of AI self-review
to identify weaknesses and improve the story structure, pacing, and coherence.

Examples:
  novelgen compose improve                  # Improve outline with 1 round (one-shot)
  novelgen compose improve --max-rounds 3   # Run 3 improvement rounds (one-shot)
  novelgen compose improve --hierarchical   # Use hierarchical improvement (better quality)`,
	RunE: runComposeImprove,
}

func init() {
	composeCmd.AddCommand(composeGenCmd)
	composeCmd.AddCommand(composeRegenCmd)
	composeCmd.AddCommand(composeImproveCmd)

	composeGenCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical generation (better quality, slower)")
	composeRegenCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Suggestions for regeneration")
	composeImproveCmd.Flags().IntVar(&composeMaxRoundsFlag, "max-rounds", 1, "Maximum number of improvement rounds")
	composeImproveCmd.Flags().IntVar(&composeConcurrencyFlag, "concurrency", 3, "Maximum number of concurrent regeneration tasks")
	composeImproveCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical improvement (better quality, slower)")

	// Register compose command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return composeCmd
	})
}

func runComposeGen(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE GEN")

	// Check if we're in a novel project
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Check if story_setup.json exists
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Check if outline already exists
	outlinePath := filepath.Join("story", "compose", "outline.json")
	if _, err := os.Stat(outlinePath); err == nil {
		return fmt.Errorf("outline already exists at %s. Use 'novelgen compose regen' to regenerate or 'novelgen compose improve' to improve", outlinePath)
	}

	// AI generation mode
	var outline *models.Outline

	if composeHierarchicalFlag {
		logger.Info("Using hierarchical generation mode (better quality)")
		outline, err = generateOutlineHierarchical(setup, projectConfig)
	} else {
		logger.Info("Using one-shot generation mode (faster)")
		outline, err = generateOutlineWithAI(setup, projectConfig)
	}
	if err != nil {
		return fmt.Errorf("failed to generate outline with AI: %w", err)
	}

	// Save outline
	if err := outline.Save(outlinePath); err != nil {
		return fmt.Errorf("failed to save outline: %w", err)
	}

	// Create markdown version
	mdPath := filepath.Join("story", "compose", "outline.md")
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
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen compose improve' to improve the outline with AI review")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func runComposeRegen(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE REGEN")

	// Check if we're in a novel project
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Check if story_setup.json exists
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load existing outline
	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load existing outline: %w", err)
	}

	// Regenerate specific element
	if err := regenerateElement(outline, id, setup, projectConfig); err != nil {
		return fmt.Errorf("failed to regenerate element: %w", err)
	}

	// Save outline
	if err := outline.Save(outlinePath); err != nil {
		return fmt.Errorf("failed to save outline: %w", err)
	}

	// Create markdown version
	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	fmt.Printf("\n✓ Outline regenerated and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func runComposeImprove(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE IMPROVE")

	// Check if we're in a novel project
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Check if story_setup.json exists
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load existing outline
	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load existing outline: %w", err)
	}
	logger.Info("Loaded existing outline for improvement")

	// Run improvement
	if err := iterateOutlineImprovement(outline, setup, projectConfig, composeMaxRoundsFlag, composeConcurrencyFlag, composeHierarchicalFlag); err != nil {
		logger.Error("Improvement failed: %v", err)
		return fmt.Errorf("improvement failed: %w", err)
	}

	// Save improved outline
	if err := outline.Save(outlinePath); err != nil {
		return fmt.Errorf("failed to save improved outline: %w", err)
	}

	// Update markdown version
	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	fmt.Printf("\n✓ Outline improved and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

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
	agent.SetLanguage(projectConfig.Language)

	ctx := context.Background()
	input := agents.ComposeGenInput{
		Setup:     *setup,
		Structure: projectConfig.Structure,
	}
	output, err := agent.Generate(ctx, input)
	if err != nil {
		return nil, err
	}
	return &output.Outline, nil
}

// generateOutlineHierarchical generates outline using hierarchical approach
// Supports incremental save and resume
func generateOutlineHierarchical(setup *models.StorySetup, projectConfig *models.ProjectConfig) (*models.Outline, error) {
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

	// Check for existing partial outline (for resume)
	progressPath := filepath.Join("story", "compose", "outline_progress.json")

	var outline *models.Outline
	var resumeMode bool

	if _, err := os.Stat(progressPath); err == nil {
		// Found progress file, try to resume
		fmt.Println("📂 Found existing progress file. Resuming generation...")
		outline, err = loadPartialOutline(progressPath)
		if err != nil {
			fmt.Printf("⚠️  Failed to load progress: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			fmt.Println("✓ Resumed from saved progress")
			printProgressStatus(outline, projectConfig.Structure)
		}
	}

	// Create LLM client and agent
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	ctx := context.Background()

	// Save callback for incremental saves (used in both fresh and resume modes)
	onVolumeComplete := func(o *models.Outline, partIdx, volIdx, volumeCount int) {
		if err := savePartialOutline(o, progressPath); err != nil {
			logger.GetLogger().Warn("Failed to save progress: %v", err)
		} else {
			fmt.Printf("💾 Progress saved (%d volumes completed)\n", volumeCount)
		}
	}

	if !resumeMode {
		// Fresh generation
		fmt.Println("Using hierarchical generation:")
		fmt.Println("  Phase 1: Generate skeleton (parts and volumes)")
		fmt.Printf("  Phase 2: Generate chapters for each of %d volumes\n",
			projectConfig.Structure.TargetParts*projectConfig.Structure.TargetVolumes)
		fmt.Println()

		// Generate skeleton first
		skeletonInput := agents.ComposeSkeletonInput{
			Setup:     *setup,
			Structure: projectConfig.Structure,
		}
		skeletonOutput, err := agent.GenerateSkeleton(ctx, skeletonInput)
		if err != nil {
			return nil, fmt.Errorf("failed to generate skeleton: %w", err)
		}

		outline = &models.Outline{
			Parts: skeletonOutput.Parts,
		}

		// Save skeleton as initial progress
		if err := savePartialOutline(outline, progressPath); err != nil {
			logger.GetLogger().Warn("Failed to save initial progress: %v", err)
		}
		fmt.Println("💾 Skeleton saved")

		// Generate chapters with progress saving
		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save progress even on error
			if saveErr := savePartialOutline(outline, progressPath); saveErr != nil {
				logger.GetLogger().Warn("Failed to save progress on error: %v", saveErr)
			}
			return nil, err
		}

		// Remove progress file on successful completion
		os.Remove(progressPath)
		fmt.Println("\n✓ Generation complete! Progress file removed.")
	} else {
		// Resume generation - continue from where we left off
		fmt.Println("Continuing chapter generation...")
		fmt.Println()

		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save progress even on error
			if saveErr := savePartialOutline(outline, progressPath); saveErr != nil {
				logger.GetLogger().Warn("Failed to save progress on error: %v", saveErr)
			}
			return nil, err
		}

		// Remove progress file on successful completion
		os.Remove(progressPath)
		fmt.Println("\n✓ Generation complete! Progress file removed.")
	}

	return outline, nil
}

// loadPartialOutline loads a partially generated outline from progress file
func loadPartialOutline(path string) (*models.Outline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var outline models.Outline
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	return &outline, nil
}

// savePartialOutline saves the current progress to a file
func savePartialOutline(outline *models.Outline, path string) error {
	if outline == nil {
		return fmt.Errorf("cannot save nil outline")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outline: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	return nil
}

// printProgressStatus prints the current generation progress
func printProgressStatus(outline *models.Outline, structure models.StoryStructure) {
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	completedVolumes := 0
	totalChapters := 0

	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			if len(vol.Chapters) > 0 {
				completedVolumes++
				totalChapters += len(vol.Chapters)
			}
		}
	}

	fmt.Printf("\n📊 Progress: %d/%d volumes completed (%d chapters generated)\n",
		completedVolumes, totalVolumes, totalChapters)
	fmt.Println()
}

func regenerateElement(outline *models.Outline, id string, setup *models.StorySetup, projectConfig *models.ProjectConfig) error {
	parts := strings.Split(id, "_")

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Get user prompt for regeneration
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
	agent.SetLanguage(projectConfig.Language)

	// Create IDManager for ID resolution
	idManager := logic.NewIDManager(outline)

	ctx := context.Background()

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

		context := agent.BuildPartContext(part, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "part",
			ElementID:   part.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Part != nil {
			part.Title = output.Part.Title
			part.Summary = output.Part.Summary
		}
		return nil

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

		context := agent.BuildVolumeContext(volume, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "volume",
			ElementID:   volume.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Volume != nil {
			volume.Title = output.Volume.Title
			volume.Summary = output.Volume.Summary
			if len(output.Volume.Chapters) > 0 {
				volume.Chapters = output.Volume.Chapters
			}
		}
		return nil

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

		context := agent.BuildChapterContext(chapter, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "chapter",
			ElementID:   chapter.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Chapter != nil {
			chapter.Title = output.Chapter.Title
			chapter.Summary = output.Chapter.Summary
			chapter.Characters = output.Chapter.Characters
			chapter.Location = output.Chapter.Location
			chapter.Events = output.Chapter.Events
			chapter.Beats = output.Chapter.Beats
			chapter.OpeningBeat = output.Chapter.OpeningBeat
			chapter.ClosingBeat = output.Chapter.ClosingBeat
			chapter.Conflict = output.Chapter.Conflict
			chapter.Pacing = output.Chapter.Pacing
		}
		return nil

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
func iterateOutlineImprovement(outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, maxIterations int, concurrency int, hierarchical bool) error {
	logger.Section("Outline Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)
	if hierarchical {
		logger.Info("Mode: Hierarchical (review entire outline, improve volumes individually)")
	} else {
		logger.Info("Mode: One-shot (review and improve entire outline at once)")
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client and compose agent
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	ctx := context.Background()

	var improvedOutline *models.Outline
	var review *models.ReviewResult

	if hierarchical {
		// Use hierarchical iteration
		improvedOutline, review, err = agent.IterateHierarchical(ctx, outline, maxIterations, 80.0)
	} else {
		// Use one-shot iteration
		improvedOutline, review, err = agent.Iterate(ctx, outline, maxIterations, 80.0)
	}

	if err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	// Update the outline with improved version
	*outline = *improvedOutline

	// Save intermediate result
	outlinePath := filepath.Join("story", "compose", fmt.Sprintf("outline_iter_%d.json", maxIterations))
	if err := outline.Save(outlinePath); err != nil {
		logger.Error("Failed to save intermediate outline: %v", err)
	} else {
		logger.Info("Saved intermediate outline to %s", outlinePath)
	}

	logger.Section("Iteration Complete")
	logger.Info("Final Review Score: %.1f/100", review.OverallScore)
	return nil
}
