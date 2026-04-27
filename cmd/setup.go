package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
	"novelgen/internal/rpg/dsl"

	"github.com/spf13/cobra"
)

var (
	setupRegenPrompt   string
	setupMaxRounds     int
	setupImprovePrompt string
	setupForceImprove  bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create story setup",
	Long: `Create or update the story setup for your novel.

This command generates story/setup/story_setup.json containing:
  - Genre(s) and subgenres
  - Core premise and logline
  - Story rules and mechanics
  - Themes and motifs
  - Tone and atmosphere
  - POV style and narrative voice

Subcommands:
  gen     - Generate story setup using AI from a prompt
  regen   - Regenerate story setup with optional guidance
  improve - Improve existing story setup through AI review
  import  - Import story setup from markdown file`,
}

var setupGenCmd = &cobra.Command{
	Use:   "gen [prompt]",
	Short: "Generate story setup from a prompt",
	Long: `Generate story setup using AI based on your story idea prompt.

Examples:
  novelgen setup gen "一个关于太空探险的故事"
  novelgen setup gen "赛博朋克背景下的侦探故事"`,
	Args: cobra.ExactArgs(1),
	RunE: runSetupGen,
}

var setupRegenCmd = &cobra.Command{
	Use:   "regen",
	Short: "Regenerate story setup",
	Long: `Regenerate the story setup with optional guidance.

This command reads the existing story setup and regenerates it
based on the optional prompt guidance.

Examples:
  novelgen setup regen                      # Regenerate with current setup
  novelgen setup regen --prompt "增加更多悬疑元素"
  novelgen setup regen --prompt "改为喜剧风格"`,
	RunE: runSetupRegen,
}

var setupImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve story setup through AI review",
	Long: `Improve the existing story setup through AI review and refinement.

This command analyzes the current story setup and suggests improvements
to make it more compelling, coherent, and complete.

Examples:
  novelgen setup improve                    # Improve with 1 round
  novelgen setup improve --max-rounds 3     # Improve with up to 3 rounds`,
	RunE: runSetupImprove,
}

var setupImportCmd = &cobra.Command{
	Use:   "import [markdown_file]",
	Short: "Import story setup from markdown file",
	Long: `Import story setup from a markdown file and save it as JSON.

This command reads a markdown file (e.g., story/setup/story_setup.md),
parses its content, and converts it to story_setup.json format.

Use this after manually editing the markdown file to update the JSON.

Examples:
  novelgen setup import                     # Import from story/setup/story_setup.md
  novelgen setup import my_setup.md         # Import from custom markdown file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetupImport,
}

func init() {

var setupCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate story setup for completeness and consistency",
	Long: `Run validation against story/setup/story_setup.json.

Checks include:
  - Required fields and format
  - Faction tier completeness (for enemy system)
  - Timeline event ordering
  - Resource scarcity consistency
  - Premise progression gaps

Examples:
  novelgen setup check`,
	RunE: runSetupCheck,
}
	// Register setup command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		setupCmd.AddCommand(setupGenCmd)
		setupCmd.AddCommand(setupRegenCmd)
		setupCmd.AddCommand(setupImproveCmd)
		setupCmd.AddCommand(setupImportCmd)
			setupCmd.AddCommand(setupCheckCmd)
		return setupCmd
	})

	// Regen flags
	setupRegenCmd.Flags().StringVar(&setupRegenPrompt, "prompt", "", "Guidance for regeneration")

	// Improve flags
	setupImproveCmd.Flags().IntVar(&setupMaxRounds, "max-rounds", 1, "Maximum improvement rounds")
	setupImproveCmd.Flags().StringVar(&setupImprovePrompt, "prompt", "", "Manual improvement guidance (if provided, uses manual mode)")
	setupImproveCmd.Flags().BoolVar(&setupForceImprove, "force", false, "Force improvement based on suggestions even if score meets threshold")
}

func runSetupGen(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN SETUP")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// AI generation mode
	logger.Info("AI generation mode with prompt: %s", prompt)
	logger.Info("Using language: %s", projectConfig.Language)
	setup, err := generateStorySetupWithAI(cmd.Context(), prompt, projectConfig.Language, &projectConfig.LLM)
	if err != nil {
		logger.Error("Failed to generate story setup with AI: %v", err)
		return fmt.Errorf("failed to generate story setup with AI: %w", err)
	}

	// Save story setup
	if err := saveStorySetup(setup); err != nil {
		return fmt.Errorf("failed to save story setup: %w", err)
	}

	fmt.Printf("\n✓ Story setup created successfully!\n")
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/setup/story_setup.json to refine your story setup")
	fmt.Println("  - Run 'novelgen compose gen' to generate the story outline")

	return nil
}

func runSetupRegen(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN SETUP REGEN")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Load existing story setup
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	existingSetup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		logger.Error("Failed to load existing story setup: %v", err)
		return fmt.Errorf("failed to load existing story setup: %w", err)
	}
	logger.Info("Loaded existing story setup")

	// Build prompt for regeneration with full context
	setupJSON, err := json.MarshalIndent(existingSetup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize existing setup: %w", err)
	}

	prompt := fmt.Sprintf("Current story setup:\n%s\n\nPlease regenerate this story setup", string(setupJSON))
	if setupRegenPrompt != "" {
		prompt = fmt.Sprintf("%s with the following guidance: %s", prompt, setupRegenPrompt)
		logger.Info("Using regeneration guidance: %s", setupRegenPrompt)
	}

	// Regenerate story setup
	logger.Info("Regenerating story setup...")
	setup, err := generateStorySetupWithAI(cmd.Context(), prompt, projectConfig.Language, &projectConfig.LLM)
	if err != nil {
		logger.Error("Failed to regenerate story setup: %v", err)
		return fmt.Errorf("failed to regenerate story setup: %w", err)
	}

	// Save story setup
	if err := saveStorySetup(setup); err != nil {
		return fmt.Errorf("failed to save story setup: %w", err)
	}

	fmt.Printf("\n✓ Story setup regenerated successfully!\n")
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func runSetupImprove(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN SETUP IMPROVE")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Load existing story setup
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		logger.Error("Failed to load existing story setup: %v", err)
		return fmt.Errorf("failed to load existing story setup: %w", err)
	}
	logger.Info("Loaded existing story setup")

	// Create LLM client and agent
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return fmt.Errorf("failed to get active LLM configuration")
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	agent := agents.NewSetupAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	// Improve story setup
	if setupImprovePrompt != "" {
		// Manual mode: use user's prompt for improvement
		fmt.Printf("\n📝 Starting manual improvement...\n")

		logger.Section("MANUAL IMPROVEMENT")
		logger.Info("Prompt: %s", setupImprovePrompt)

		// Create input with user prompt
		input := agents.SetupImproveInput{
			ExistingSetup: *setup,
			ReviewResult: models.ReviewResult{
				Summary: setupImprovePrompt,
				Suggestions: []models.ReviewSuggestion{
					{
						Category:   "user_guidance",
						Issue:      "User requested improvement",
						Suggestion: setupImprovePrompt,
						Priority:   "high",
					},
				},
			},
		}

		improveResult, err := agent.Improve(cmd.Context(), input)
		if err != nil {
			logger.Error("Failed to improve story setup: %v", err)
			return fmt.Errorf("failed to improve story setup: %w", err)
		}
		setup = &improveResult.Setup

		// Save improved setup
		if err := saveStorySetup(setup); err != nil {
			return fmt.Errorf("failed to save improved story setup: %w", err)
		}

		fmt.Printf("\n✓ Story setup improved with manual guidance!\n")
	} else {
		// Auto mode: use Iterate for Review → Improve loop
		fmt.Printf("\n🔄 Starting auto improvement (max %d rounds)...\n\n", setupMaxRounds)
		if setupForceImprove {
			fmt.Println("⚡ Force improve enabled: will improve based on suggestions even if score meets threshold")
		}
		improvedSetup, review, err := agent.Iterate(cmd.Context(), setup, setupMaxRounds, 80.0, setupForceImprove)
		if err != nil {
			logger.Error("Failed to improve story setup: %v", err)
			return fmt.Errorf("failed to improve story setup: %w", err)
		}
		setup = improvedSetup

		// Enrich with DSL simulation feedback
		dslBridge := dsl.NewSimulationBridge()
		dslAdapter := dsl.NewModelAdapter(setup, nil, nil, nil, nil)
		if dslIssues, simErr := dslAdapter.Simulate(dsl.PhaseSetup); simErr == nil && len(dslIssues) > 0 {
			dslBridge.MergeIntoReview(dslIssues, review)
			logger.Info("DSL simulation enriched review with %d additional issues", len(dslIssues))

			for _, iss := range dslIssues {
				if iss.Severity == dsl.SeverityCritical {
					extraInput := agents.SetupImproveInput{
						ExistingSetup: *improvedSetup,
						ReviewResult:  *review,
					}
					if extraResult, extraErr := agent.Improve(cmd.Context(), extraInput); extraErr == nil {
						setup = &extraResult.Setup
						logger.Info("Extra improve pass completed for DSL-critical issues")
					}
					break
				}
			}
		}

		// Enrich with cross-module feedback (compose→setup)
		if review != nil {
			crossIssues, crossWarnings := loadAndCrossCheckSetup(improvedSetup)
			for _, iss := range crossIssues {
				review.Suggestions = append(review.Suggestions, models.ReviewSuggestion{
					Category: "cross-module", TargetName: "setup", Issue: iss,
					Suggestion: "在 story_setup.json 中添加对应的声明", Priority: models.PriorityHigh,
				})
			}
			for _, w := range crossWarnings {
				review.Suggestions = append(review.Suggestions, models.ReviewSuggestion{
					Category: "cross-module", TargetName: "setup", Issue: w,
					Suggestion: "检查是否需要在 story_setup.json 中补充", Priority: models.PriorityMedium,
				})
			}
			if len(crossIssues)+len(crossWarnings) > 0 {
				logger.Info("Cross-module check added %d issues and %d warnings to review",
					len(crossIssues), len(crossWarnings))
			}
		}

		// Save improved setup
		if err := saveStorySetup(setup); err != nil {
			return fmt.Errorf("failed to save improved story setup: %w", err)
		}

		fmt.Printf("\n✓ Story setup improved successfully!\n")
		fmt.Printf("📊 Final Review Score: %.1f/100\n", review.OverallScore)
	}

	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func loadAndCrossCheckSetup(setup *models.StorySetup) (issues, warnings []string) {
	outlinePath := filepath.Join("story", "compose", "outline.json")
	data, err := os.ReadFile(outlinePath)
	if err != nil {
		return nil, nil // no outline yet, skip
	}
	var storyOutline rpg.StoryOutline
	if json.Unmarshal(data, &storyOutline) != nil {
		return nil, nil
	}
	return validateSetupOutlineCross(setup, &storyOutline)
}

func runSetupImport(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN SETUP IMPORT")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Determine markdown file path
	mdPath := filepath.Join("story", "setup", "story_setup.md")
	if len(args) > 0 {
		mdPath = args[0]
	}
	logger.Info("Importing from: %s", mdPath)

	// Read markdown file
	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		logger.Error("Failed to read markdown file: %v", err)
		return fmt.Errorf("failed to read markdown file %s: %w", mdPath, err)
	}

	// Parse markdown and create story setup
	setup, err := parseStorySetupFromMarkdown(string(mdContent))
	if err != nil {
		logger.Error("Failed to parse markdown: %v", err)
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Set project name from config if not found in markdown
	if setup.ProjectName == "" {
		setup.ProjectName = projectConfig.Name
	}

	// Save story setup
	if err := saveStorySetup(setup); err != nil {
		return fmt.Errorf("failed to save story setup: %w", err)
	}

	fmt.Printf("\n✓ Story setup imported successfully!\n")
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func parseStorySetupFromMarkdown(content string) (*models.StorySetup, error) {
	setup := &models.StorySetup{}

	lines := strings.Split(content, "\n")
	var currentSection string
	var sectionContent []string
	var inStorylines bool
	var inPremises bool
	var currentStoryline *models.Storyline
	var currentPremise *models.Premise
	var currentProgression *models.ProgressionStage

	flushSection := func() {
		if currentSection != "" && len(sectionContent) > 0 {
			fillSetupField(setup, currentSection, strings.Join(sectionContent, "\n"))
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Main title
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			flushSection()
			setup.ProjectName = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			currentSection = ""
			sectionContent = []string{}
			continue
		}

		// Section headers (##)
		if strings.HasPrefix(trimmed, "## ") {
			flushSection()
			sectionName := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			sectionLower := strings.ToLower(sectionName)
			inStorylines = strings.Contains(sectionLower, "storyline")
			inPremises = strings.Contains(sectionLower, "premise")
			currentSection = ""
			sectionContent = []string{}
			continue
		}

		// Subsection headers (###) - could be simple fields or storyline/premise items
		if strings.HasPrefix(trimmed, "### ") {
			flushSection()
			subSection := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))

			if inStorylines {
				// Save previous storyline
				if currentStoryline != nil && currentStoryline.Name != "" {
					setup.Storylines = append(setup.Storylines, *currentStoryline)
				}
				// Parse storyline name (format: "主线：xxx" or "副线：xxx")
				name := subSection
				if idx := strings.Index(subSection, "："); idx > 0 {
					name = strings.TrimSpace(subSection[idx+3:])
				} else if idx := strings.Index(subSection, ":"); idx > 0 {
					name = strings.TrimSpace(subSection[idx+1:])
				}
				currentStoryline = &models.Storyline{Name: name}
				currentSection = "storyline_item"
				sectionContent = []string{}
			} else if inPremises {
				// Save previous premise
				if currentPremise != nil && currentPremise.Name != "" {
					setup.Premises = append(setup.Premises, *currentPremise)
				}
				// Parse premise name (format: "xxx (category)")
				name := subSection
				category := ""
				if idx := strings.Index(subSection, "("); idx > 0 && strings.Contains(subSection, ")") {
					name = strings.TrimSpace(subSection[:idx])
					endIdx := strings.Index(subSection, ")")
					if endIdx > idx {
						category = strings.TrimSpace(subSection[idx+1 : endIdx])
					}
				}
				currentPremise = &models.Premise{Name: name, Category: category}
				currentSection = "premise_item"
				sectionContent = []string{}
			} else {
				// Regular field
				currentSection = subSection
				sectionContent = []string{}
			}
			continue
		}

		// Progression stage headers (Level X:)
		if strings.HasPrefix(trimmed, "**Level ") && currentPremise != nil {
			// Save previous progression stage
			if currentProgression != nil {
				currentPremise.Progression = append(currentPremise.Progression, *currentProgression)
			}
			// Parse level
			levelStr := trimmed
			levelStr = strings.TrimPrefix(levelStr, "**Level ")
			levelStr = strings.TrimSuffix(levelStr, "**")
			levelStr = strings.TrimSpace(levelStr)
			if idx := strings.Index(levelStr, ":"); idx > 0 {
				levelStr = strings.TrimSpace(levelStr[:idx])
			}
			var level int
			fmt.Sscanf(levelStr, "%d", &level)
			currentProgression = &models.ProgressionStage{Level: level}
			continue
		}

		// Parse progression stage fields
		if currentProgression != nil {
			if strings.HasPrefix(trimmed, "- Description:") || strings.HasPrefix(trimmed, "- **Description**:") {
				desc := trimmed
				desc = strings.TrimPrefix(desc, "- Description:")
				desc = strings.TrimPrefix(desc, "- **Description**:")
				currentProgression.Description = strings.TrimSpace(desc)
				continue
			}
			if strings.HasPrefix(trimmed, "- Requirements:") || strings.HasPrefix(trimmed, "- **Requirements**:") {
				req := trimmed
				req = strings.TrimPrefix(req, "- Requirements:")
				req = strings.TrimPrefix(req, "- **Requirements**:")
				currentProgression.Requirements = strings.TrimSpace(req)
				continue
			}
			// Check if next line is a new level or section
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(nextLine, "**Level ") || strings.HasPrefix(nextLine, "### ") || strings.HasPrefix(nextLine, "## ") {
					currentPremise.Progression = append(currentPremise.Progression, *currentProgression)
					currentProgression = nil
				}
			}
		}

		// Parse storyline fields
		if currentStoryline != nil && currentSection == "storyline_item" {
			if strings.HasPrefix(trimmed, "- **Type**:") || strings.HasPrefix(trimmed, "- Type:") {
				typ := trimmed
				typ = strings.TrimPrefix(typ, "- **Type**:")
				typ = strings.TrimPrefix(typ, "- Type:")
				currentStoryline.Type = strings.TrimSpace(typ)
				continue
			}
			if strings.HasPrefix(trimmed, "- **Importance**:") || strings.HasPrefix(trimmed, "- Importance:") {
				imp := trimmed
				imp = strings.TrimPrefix(imp, "- **Importance**:")
				imp = strings.TrimPrefix(imp, "- Importance:")
				imp = strings.TrimSpace(imp)
				imp = strings.TrimSuffix(imp, "/10")
				fmt.Sscanf(imp, "%d", &currentStoryline.Importance)
				continue
			}
			if strings.HasPrefix(trimmed, "- **Description**:") || strings.HasPrefix(trimmed, "- Description:") {
				desc := trimmed
				desc = strings.TrimPrefix(desc, "- **Description**:")
				desc = strings.TrimPrefix(desc, "- Description:")
				currentStoryline.Description = strings.TrimSpace(desc)
				continue
			}
		}

		// Parse premise description
		if currentPremise != nil && currentSection == "premise_item" && !strings.HasPrefix(trimmed, "**Progression**") && !strings.HasPrefix(trimmed, "**Level") {
			if trimmed != "" && !strings.HasPrefix(trimmed, "-") && currentPremise.Description == "" {
				currentPremise.Description = trimmed
				continue
			}
		}

		// Regular content for simple fields
		if currentSection != "" && currentSection != "storyline_item" && currentSection != "premise_item" {
			cleanLine := trimmed
			cleanLine = strings.TrimPrefix(cleanLine, "- ")
			cleanLine = strings.TrimPrefix(cleanLine, "* ")
			if cleanLine != "" {
				sectionContent = append(sectionContent, cleanLine)
			}
		}
	}

	// Save final items
	flushSection()
	if currentStoryline != nil && currentStoryline.Name != "" {
		setup.Storylines = append(setup.Storylines, *currentStoryline)
	}
	if currentPremise != nil && currentPremise.Name != "" {
		if currentProgression != nil {
			currentPremise.Progression = append(currentPremise.Progression, *currentProgression)
		}
		setup.Premises = append(setup.Premises, *currentPremise)
	}

	return setup, nil
}

func fillSetupField(setup *models.StorySetup, section, content string) {
	content = strings.TrimSpace(content)
	sectionLower := strings.ToLower(section)

	switch {
	case strings.Contains(sectionLower, "genre"):
		// Parse genres from list items
		genres := []string{}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" && line != "None" {
				genres = append(genres, line)
			}
		}
		setup.Genres = genres

	case strings.Contains(sectionLower, "premise") && !strings.Contains(sectionLower, "premises"):
		setup.Premise = content

	case strings.Contains(sectionLower, "theme"):
		setup.Theme = content

	case strings.Contains(sectionLower, "rule") || strings.Contains(sectionLower, "constraint"):
		rules := []string{}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" && line != "None" {
				rules = append(rules, line)
			}
		}
		setup.Rules = rules

	case strings.Contains(sectionLower, "target audience") || strings.Contains(sectionLower, "audience"):
		setup.TargetAudience = content

	case strings.Contains(sectionLower, "tone") || strings.Contains(sectionLower, "style"):
		setup.Tone = content

	case strings.Contains(sectionLower, "tense"):
		setup.Tense = content

	case strings.Contains(sectionLower, "pov"):
		setup.POVStyle = content
	}
}

func generateStorySetupWithAI(ctx context.Context, prompt, language string, projectLLM *models.ProjectLLM) (*models.StorySetup, error) {
	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Use project LLM settings from novel.json
	if projectLLM.Provider == "" {
		projectLLM.Provider = "ollama"
	}
	if projectLLM.Model == "" {
		if projectLLM.Provider == "openai" {
			projectLLM.Model = "gpt-5.2"
		} else {
			projectLLM.Model = "qwen3.5:4b"
		}
	}

	provider, model := cfg.GetActiveModel(projectLLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Language: %s\n", language)
	fmt.Println()

	// Create LLM client and agent
	client := cfg.CreateClient(projectLLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewSetupAgent(client, cfg, projectLLM)
	agent.SetLanguage(language)

	result, err := agent.Generate(ctx, agents.SetupGenInput{Idea: prompt})
	if err != nil {
		return nil, err
	}
	return &result.Setup, nil
}

func saveStorySetup(setup *models.StorySetup) error {
	// Create story_setup.json in story/setup/
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if err := setup.Save(setupPath); err != nil {
		return fmt.Errorf("failed to save story_setup.json: %w", err)
	}

	// Create story_setup.md (markdown version for easier editing)
	mdPath := filepath.Join("story", "setup", "story_setup.md")
	if err := createStorySetupMarkdown(setup, mdPath); err != nil {
		return fmt.Errorf("failed to save story_setup.md: %w", err)
	}

	return nil
}

func createStorySetupMarkdown(setup *models.StorySetup, path string) error {
	content := fmt.Sprintf(`# %s

## Story Setup

### Genre(s)
%s

### Core Premise
%s

### Theme
%s

### Story Rules/Constraints
%s

### Target Audience
%s

### Tone/Style
%s

### Narrative Tense
%s

### POV Style
%s

## Storylines
%s

## Premises
%s
`,
		setup.ProjectName,
		formatList(setup.Genres),
		setup.Premise,
		setup.Theme,
		formatList(setup.Rules),
		setup.TargetAudience,
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
		formatStorylines(setup.Storylines),
		formatPremises(setup.Premises),
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func formatStorylines(storylines []models.Storyline) string {
	if len(storylines) == 0 {
		return "No storylines defined."
	}
	var result strings.Builder
	for _, s := range storylines {
		result.WriteString(fmt.Sprintf("\n### %s\n", s.Name))
		result.WriteString(fmt.Sprintf("- **Type**: %s\n", s.Type))
		result.WriteString(fmt.Sprintf("- **Importance**: %d/10\n", s.Importance))
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", s.Description))
	}
	return result.String()
}

func formatPremises(premises []models.Premise) string {
	if len(premises) == 0 {
		return "No premises defined."
	}
	var result strings.Builder
	for _, p := range premises {
		result.WriteString(fmt.Sprintf("\n### %s (%s)\n", p.Name, p.Category))
		result.WriteString(fmt.Sprintf("%s\n\n", p.Description))
		if len(p.Progression) > 0 {
			result.WriteString("**Progression System:**\n\n")
			for _, stage := range p.Progression {
				result.WriteString(fmt.Sprintf("**Level %d: %s**\n", stage.Level, stage.Name))
				result.WriteString(fmt.Sprintf("- Description: %s\n", stage.Description))
				if stage.Requirements != "" {
					result.WriteString(fmt.Sprintf("- Requirements: %s\n", stage.Requirements))
				}
				result.WriteString("\n")
			}
		}
	}
	return result.String()
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "None"
	}
	var result strings.Builder
	for _, item := range items {
		result.WriteString(fmt.Sprintf("- %s\n", item))
	}
	return result.String()
}

func runSetupCheck(cmd *cobra.Command, args []string) error {
	logger.Section("NOVELGEN SETUP CHECK")

	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if _, err := os.Stat(setupPath); err != nil {
		return fmt.Errorf("story setup not found at %s", setupPath)
	}

	data, err := os.ReadFile(setupPath)
	if err != nil {
		return fmt.Errorf("failed to read setup: %w", err)
	}

	var setup models.StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return fmt.Errorf("failed to parse setup: %w", err)
	}

	issues, warnings, suggestions := 0, 0, 0

	// Check required fields
	if setup.ProjectName == "" {
		fmt.Println("  ✗ 缺少项目名称")
		issues++
	}
	if setup.Premise == "" {
		fmt.Println("  ✗ 缺少核心设定(premise)")
		issues++
	}

	// Check faction tier definitions (premises with category that looks like faction)
	fmt.Println()
	fmt.Printf("===== SETUP VALIDATION =====\n")
	fmt.Printf("Project: %s | Genres: %v\n", setup.ProjectName, setup.Genres)
	fmt.Printf("Premises: %d | Storylines: %d | Timeline events: %d | Resources: %d\n\n",
		len(setup.Premises), len(setup.Storylines), len(setup.WorldTimeline), len(setup.WorldResources))

	for _, p := range setup.Premises {
		if len(p.Progression) == 0 {
			fmt.Printf("  ⚠ [premise] '%s' 缺少 progression 等级体系\n", p.Name)
			warnings++
			continue
		}
		// Check progression gaps
		for i, stage := range p.Progression {
			if i > 0 && stage.Level != p.Progression[i-1].Level+1 {
				fmt.Printf("  ⚠ [premise] '%s' progression 层级跳跃: Lv%d → Lv%d\n",
					p.Name, p.Progression[i-1].Level, stage.Level)
				warnings++
			}
		}
	}

	// Check timeline ordering
	timelineWarned := false
	for _, t := range setup.WorldTimeline {
		if t.Year == "" || t.Event == "" {
			if !timelineWarned {
				fmt.Printf("  ⚠ [timeline] 事件缺少年份或描述\n")
				warnings++
				timelineWarned = true
			}
		}
	}

	// Check resource scarcity consistency
	uniqueResources := 0
	for _, r := range setup.WorldResources {
		if r.Scarcity == "独一无二" {
			uniqueResources++
		}
		if r.Name == "" || r.Category == "" {
			fmt.Printf("  ⚠ [resource] 资源缺少名称或类型\n")
			warnings++
		}
	}
	if uniqueResources > 1 {
		fmt.Printf("  💡 [resource] 有 %d 个'独一无二'资源，确认是否合理\n", uniqueResources)
		suggestions++
	}

	// Check setup has faction definitions for future outline
	hasFactions := false
	for _, p := range setup.Premises {
		if strings.Contains(p.Category, "zerg") || strings.Contains(p.Category, "ai_mech") ||
			strings.Contains(strings.ToLower(p.Name), "阵营") || strings.Contains(strings.ToLower(p.Name), "虫族") ||
			strings.Contains(strings.ToLower(p.Name), "机械") || strings.Contains(strings.ToLower(p.Name), "势力") {
			hasFactions = true
		}
	}
	if !hasFactions {
		fmt.Printf("  💡 [faction] 未检测到阵营定义，建议在 premises 中定义敌对阵营的等级体系\n")
		suggestions++
	}

	fmt.Printf("\n结果: %d 问题, %d 警告, %d 建议\n", issues, warnings, suggestions)
	return nil
}
