package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
	"novelgen/internal/rpg/dsl"
	"novelgen/internal/utils"

	"github.com/spf13/cobra"
)

var (
	setupRegenPrompt   string
	setupMaxRounds     int
	setupImprovePrompt string
	setupForceImprove  bool
	setupAgentSDK      bool
	setupAgentApply    bool
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
  - Optional writing style and reference excerpt
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

With --agent-sdk, regeneration runs as a focused setup repair workflow:
the agent queries/checks the current setup through novelgen tools and
returns or applies a validated setup patch instead of receiving the full
setup JSON in prompt context.

Examples:
  novelgen setup regen                      # Regenerate with current setup
  novelgen setup regen --prompt "add more mystery pressure"
  novelgen setup regen --prompt "make the tone more comedic"
  novelgen setup regen --agent-sdk --prompt "sharpen the protagonist promise"
  novelgen setup regen --agent-sdk --agent-apply --prompt "repair setup check issues"`,
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

func init() {
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
	setupRegenCmd.Flags().BoolVar(&setupAgentSDK, "agent-sdk", false, "Use Claude Agent SDK workflow with setup query/check/patch tools")
	setupRegenCmd.Flags().BoolVar(&setupAgentApply, "agent-apply", false, "With --agent-sdk, let the agent write setup through validated patch tools")

	// Improve flags
	setupImproveCmd.Flags().IntVar(&setupMaxRounds, "max-rounds", 1, "Maximum improvement rounds")
	setupImproveCmd.Flags().StringVar(&setupImprovePrompt, "prompt", "", "Manual improvement guidance (if provided, uses manual mode)")
	setupImproveCmd.Flags().BoolVar(&setupForceImprove, "force", false, "Force improvement based on suggestions even if score meets threshold")
	setupImproveCmd.Flags().BoolVar(&setupAgentSDK, "agent-sdk", false, "Use Claude Agent SDK workflow with setup query/check/patch tools")
	setupImproveCmd.Flags().BoolVar(&setupAgentApply, "agent-apply", false, "With --agent-sdk, let the agent write setup through validated patch tools")
}

func validateSetupAgentApplyOption(agentSDK, agentApply bool) error {
	if agentApply && !agentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
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

	setup, gate, err := repairSetupWithQualityGate(cmd.Context(), setup, projectConfig, "generation")
	if err != nil {
		return err
	}
	logQualityGateResult("setup", gate)

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
	if err := validateSetupAgentApplyOption(setupAgentSDK, setupAgentApply); err != nil {
		return err
	}

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

	if setupAgentSDK {
		agent, err := newSetupAgentForProject(projectConfig)
		if err != nil {
			return err
		}
		prompt := strings.TrimSpace(setupRegenPrompt)
		if prompt == "" {
			prompt = "Regenerate and tighten the story setup using the current project facts. Preserve stable project identity, repair setup check issues, and make only justified patch changes."
		}
		improved, review, changed, err := runSetupImproveAgentSDK(cmd.Context(), agent, existingSetup, setupPath, 1, prompt, true, setupAgentApply)
		if err != nil {
			logger.Error("Failed to regenerate story setup with Agent SDK: %v", err)
			return fmt.Errorf("failed to regenerate story setup with Agent SDK: %w", err)
		}
		if !setupAgentApply && changed {
			if _, err := saveToolSetupPatchCheckpoint(".", *existingSetup); err != nil {
				return fmt.Errorf("failed to checkpoint original story setup: %w", err)
			}
			if err := saveStorySetup(improved); err != nil {
				return fmt.Errorf("failed to save regenerated story setup: %w", err)
			}
		}
		if changed {
			fmt.Printf("\n[ok] Story setup regenerated with Agent SDK!\n")
		} else {
			fmt.Printf("\n[ok] Story setup checked with Agent SDK; no effective changes were applied.\n")
		}
		if review != nil {
			fmt.Printf("Final Agent SDK Review Score: %.1f/100\n", review.OverallScore)
		}
		fmt.Printf("\nProject: %s\n", improved.ProjectName)
		fmt.Printf("Genre(s): %s\n", strings.Join(improved.Genres, ", "))
		fmt.Printf("Premise: %.100s...\n", improved.Premise)
		return nil
	}

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

	setup, gate, err := repairSetupWithQualityGate(cmd.Context(), setup, projectConfig, "regeneration")
	if err != nil {
		return err
	}
	logQualityGateResult("setup", gate)

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
	if err := validateSetupAgentApplyOption(setupAgentSDK, setupAgentApply); err != nil {
		return err
	}

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
	agent, err := newSetupAgentForProject(projectConfig)
	if err != nil {
		return err
	}

	if setupAgentSDK {
		originalSetup := cloneStorySetup(setup)
		improved, review, changed, err := runSetupImproveAgentSDK(cmd.Context(), agent, setup, setupPath, setupMaxRounds, setupImprovePrompt, setupForceImprove, setupAgentApply)
		if err != nil {
			logger.Error("Failed to improve story setup with Agent SDK: %v", err)
			return fmt.Errorf("failed to improve story setup with Agent SDK: %w", err)
		}
		setup = improved
		if !setupAgentApply && changed {
			if _, err := saveToolSetupPatchCheckpoint(".", originalSetup); err != nil {
				return fmt.Errorf("failed to checkpoint original story setup: %w", err)
			}
			if err := saveStorySetup(setup); err != nil {
				return fmt.Errorf("failed to save improved story setup: %w", err)
			}
		}
		if changed {
			fmt.Printf("\n[ok] Story setup improved with Agent SDK!\n")
		} else {
			fmt.Printf("\n[ok] Story setup checked with Agent SDK; no effective changes were applied.\n")
		}
		if review != nil {
			fmt.Printf("Final Agent SDK Review Score: %.1f/100\n", review.OverallScore)
		}
		fmt.Printf("\nProject: %s\n", setup.ProjectName)
		fmt.Printf("Genre(s): %s\n", strings.Join(setup.Genres, ", "))
		fmt.Printf("Premise: %.100s...\n", setup.Premise)
		return nil
	}

	// Improve story setup
	if setupImprovePrompt != "" {
		// Manual mode: use user's prompt for improvement
		fmt.Printf("\n📝 Starting manual improvement...\n")

		logger.Section("MANUAL IMPROVEMENT")
		logger.Info("Prompt: %s", setupImprovePrompt)

		// Create input with user prompt
		review := models.ReviewResult{
			Summary: setupImprovePrompt,
			Suggestions: []models.ReviewSuggestion{
				{
					Category:   "user_guidance",
					Issue:      "User requested improvement",
					Suggestion: setupImprovePrompt,
					Priority:   "high",
				},
			},
		}
		session := agents.NewRevisionSession("setup", "Manual setup improvement from user guidance.")
		session.AddUserGuidance(1, setupImprovePrompt)
		session.AddReview(1, review)
		input := agents.SetupImproveInput{
			ExistingSetup:   *setup,
			ReviewResult:    review,
			RevisionContext: session.Prompt(),
		}

		improveResult, err := agent.Improve(cmd.Context(), input)
		if err != nil {
			logger.Error("Failed to improve story setup: %v", err)
			return fmt.Errorf("failed to improve story setup: %w", err)
		}
		setup = &improveResult.Setup

		setup, gate, err := repairSetupWithQualityGate(cmd.Context(), setup, projectConfig, "manual improvement")
		if err != nil {
			return err
		}
		logQualityGateResult("setup", gate)

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

			if review != nil {
				session := agents.NewRevisionSession("setup", "Repair setup after DSL simulation feedback.")
				session.AddReview(review.Iteration+1, *review)
				extraInput := agents.SetupImproveInput{
					ExistingSetup:   *improvedSetup,
					ReviewResult:    *review,
					RevisionContext: session.Prompt(),
				}
				if extraResult, extraErr := agent.Improve(cmd.Context(), extraInput); extraErr == nil {
					setup = &extraResult.Setup
					improvedSetup = setup
					logger.Info("Extra improve pass completed with DSL setup-contract feedback")
				} else {
					logger.Warn("Extra improve pass with DSL feedback failed: %v", extraErr)
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
					Suggestion: `在 story_setup.json 的 world_resources 中添加: {"name": "能量晶核", "category": "能源", "scarcity": "稀有", "description": "虫族核心能量结晶"}`, Priority: models.PriorityHigh,
				})
			}
			if len(crossIssues)+len(crossWarnings) > 0 {
				logger.Info("Cross-module check added %d issues and %d warnings to review",
					len(crossIssues), len(crossWarnings))
				session := agents.NewRevisionSession("setup", "Repair setup after setup/outline cross-module feedback.")
				session.AddReview(review.Iteration+1, *review)
				extraInput := agents.SetupImproveInput{
					ExistingSetup:   *setup,
					ReviewResult:    *review,
					RevisionContext: session.Prompt(),
				}
				if extraResult, extraErr := agent.Improve(cmd.Context(), extraInput); extraErr == nil {
					setup = &extraResult.Setup
					improvedSetup = setup
					logger.Info("Extra improve pass completed with setup/outline cross feedback")
				} else {
					logger.Warn("Extra improve pass with setup/outline cross feedback failed: %v", extraErr)
				}
			}
		}

		setup, gate, err := repairSetupWithQualityGate(cmd.Context(), setup, projectConfig, "auto improvement")
		if err != nil {
			return err
		}
		logQualityGateResult("setup", gate)

		// Save improved setup
		if err := saveStorySetup(setup); err != nil {
			return fmt.Errorf("failed to save improved story setup: %w", err)
		}

		fmt.Printf("\n✓ Story setup improved successfully!\n")
		if review != nil {
			fmt.Printf("📊 Final Review Score: %.1f/100\n", review.OverallScore)
		}
	}

	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func newSetupAgentForProject(projectConfig *models.ProjectConfig) (*agents.SetupAgent, error) {
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}

	agent := agents.NewSetupAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	return agent, nil
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

func runSetupImproveAgentSDK(ctx context.Context, agent *agents.SetupAgent, setup *models.StorySetup, setupPath string, maxRounds int, userPrompt string, forceImprove bool, agentApply bool) (*models.StorySetup, *models.ReviewResult, bool, error) {
	if setup == nil {
		return nil, nil, false, fmt.Errorf("setup is nil")
	}
	if maxRounds <= 0 {
		maxRounds = 1
	}
	logger.Section("SETUP AGENT SDK - Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxRounds)
	if agentApply {
		logger.Info("Agent apply enabled: SDK may write setup patches through validated tool apply")
	}

	current := cloneStorySetup(setup)
	original := cloneStorySetup(setup)
	changed := false
	var finalReview *models.ReviewResult

	for round := 1; round <= maxRounds; round++ {
		check, err := runToolSetupCheck("all", &current)
		if err != nil {
			return &current, finalReview, changed, err
		}
		review := setupToolCheckReview("Setup Agent SDK pre-check for targeted repair.", check)
		if strings.TrimSpace(userPrompt) != "" {
			review.Suggestions = append([]models.ReviewSuggestion{{
				Category:   "user_guidance",
				TargetID:   "setup",
				TargetName: "setup",
				Issue:      "User requested setup improvement",
				Suggestion: userPrompt,
				Priority:   models.PriorityHigh,
			}}, review.Suggestions...)
			review.Summary = strings.TrimSpace(userPrompt) + "\n" + review.Summary
		}

		forceIssueRepair := forceImprove || strings.TrimSpace(userPrompt) != "" || len(review.Suggestions) > 0
		output, err := agent.ImproveWithAgentSDK(ctx, agents.BuildSetupAgentSDKImprovePromptInput(review, userPrompt, agentApply, forceIssueRepair))
		if err != nil {
			return &current, finalReview, changed, fmt.Errorf("agent SDK setup improve round %d failed: %w", round, err)
		}
		output.ReviewResult.Iteration = round
		finalReview = &output.ReviewResult

		if !hasSetupPatch(output.SetupPatch) {
			logger.Info("Agent SDK returned empty setup patch in round %d", round)
			break
		}

		if agentApply {
			reloaded, err := models.LoadStorySetup(setupPath)
			if err != nil {
				return &current, finalReview, changed, fmt.Errorf("failed to reload setup after agent apply: %w", err)
			}
			if !reflect.DeepEqual(current, *reloaded) {
				changed = true
			}
			current = *reloaded
		} else {
			merged, check, changes, err := applySetupAgentSDKPatch(&current, output.SetupPatch)
			if err != nil {
				return &current, finalReview, changed, fmt.Errorf("failed to apply Agent SDK setup patch: %w", err)
			}
			if check != nil && check.Blocking {
				return &current, finalReview, changed, fmt.Errorf("setup patch rejected: quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
			}
			if len(changes) > 0 {
				current = *merged
				changed = true
				logger.Info("Agent SDK setup patch changed %d top-level field(s)", len(changes))
			}
		}

		if round == maxRounds {
			logger.Warn("Max Agent SDK setup iterations reached")
			break
		}
	}

	if !changed && !reflect.DeepEqual(original, current) {
		changed = true
	}
	return &current, finalReview, changed, nil
}

func setupToolCheckReview(summary string, check *toolCheckResult) models.ReviewResult {
	if check == nil {
		return models.ReviewResult{Summary: summary}
	}
	return models.ReviewResult{
		OverallScore: check.Score,
		Summary:      summary,
		Suggestions:  append([]models.ReviewSuggestion(nil), check.Issues...),
	}
}

func hasSetupPatch(patch map[string]interface{}) bool {
	for key, value := range patch {
		if strings.TrimSpace(key) == "" || key == "id" {
			continue
		}
		if value != nil {
			return true
		}
	}
	return false
}

func applySetupAgentSDKPatch(setup *models.StorySetup, patch map[string]interface{}) (*models.StorySetup, *toolCheckResult, []toolPatchChange, error) {
	if setup == nil {
		return nil, nil, nil, fmt.Errorf("setup is nil")
	}
	if err := utils.ValidateNoSuspiciousPatchText(patch); err != nil {
		return nil, nil, nil, fmt.Errorf("setup patch rejected: %w", err)
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateToolPatchBytes(patchBytes); err != nil {
		return nil, nil, nil, fmt.Errorf("patch rejected: %w", err)
	}
	before := cloneStorySetup(setup)
	merged, err := mergeJSONPatchObject(*setup, patchBytes)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return nil, nil, nil, fmt.Errorf("setup patch rejected: %w", err)
	}
	models.NormalizeStorySetup(&merged)
	check, err := runToolSetupCheck("all", &merged)
	if err != nil {
		return nil, nil, nil, err
	}
	changes := diffStructFields("setup", before, merged)
	return &merged, check, changes, nil
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
	var inCoreCast bool
	var inStorylines bool
	var inPremises bool
	var currentCoreCast *models.CoreCastSeed
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
			inCoreCast = strings.Contains(sectionLower, "core cast") || strings.Contains(sectionLower, "character seed")
			inStorylines = strings.Contains(sectionLower, "storyline")
			inPremises = strings.Contains(sectionLower, "premise")
			currentSection = ""
			if strings.Contains(sectionLower, "long form") || strings.Contains(sectionLower, "long_form") || strings.Contains(sectionLower, "serial plan") {
				currentSection = sectionName
			}
			sectionContent = []string{}
			continue
		}

		// Subsection headers (###) - could be simple fields or storyline/premise items
		if strings.HasPrefix(trimmed, "### ") {
			flushSection()
			subSection := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))

			if inCoreCast {
				if currentCoreCast != nil && currentCoreCast.Name != "" {
					setup.CoreCast = append(setup.CoreCast, *currentCoreCast)
				}
				name := subSection
				if idx := strings.Index(subSection, ":"); idx > 0 {
					name = strings.TrimSpace(subSection[idx+1:])
				}
				currentCoreCast = &models.CoreCastSeed{Name: name}
				currentSection = "core_cast_item"
				sectionContent = []string{}
			} else if inStorylines {
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

		if currentCoreCast != nil && currentSection == "core_cast_item" {
			if strings.HasPrefix(trimmed, "- **ID**:") || strings.HasPrefix(trimmed, "- ID:") {
				currentCoreCast.ID = trimMarkdownField(trimmed, "ID")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Role**:") || strings.HasPrefix(trimmed, "- Role:") {
				currentCoreCast.Role = trimMarkdownField(trimmed, "Role")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Importance**:") || strings.HasPrefix(trimmed, "- Importance:") {
				imp := trimMarkdownField(trimmed, "Importance")
				imp = strings.TrimSuffix(imp, "/10")
				fmt.Sscanf(imp, "%d", &currentCoreCast.Importance)
				continue
			}
			if strings.HasPrefix(trimmed, "- **Story Function**:") || strings.HasPrefix(trimmed, "- Story Function:") {
				currentCoreCast.StoryFunction = trimMarkdownField(trimmed, "Story Function")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Relationship To Lead**:") || strings.HasPrefix(trimmed, "- Relationship To Lead:") {
				currentCoreCast.RelationshipToLead = trimMarkdownField(trimmed, "Relationship To Lead")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Relationship Arc**:") || strings.HasPrefix(trimmed, "- Relationship Arc:") {
				currentCoreCast.RelationshipArc = trimMarkdownField(trimmed, "Relationship Arc")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Entry Phase**:") || strings.HasPrefix(trimmed, "- Entry Phase:") {
				currentCoreCast.EntryPhase = trimMarkdownField(trimmed, "Entry Phase")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Payoff**:") || strings.HasPrefix(trimmed, "- Payoff:") {
				currentCoreCast.Payoff = trimMarkdownField(trimmed, "Payoff")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Storyline Refs**:") || strings.HasPrefix(trimmed, "- Storyline Refs:") {
				currentCoreCast.StorylineRefs = splitInlineList(trimMarkdownField(trimmed, "Storyline Refs"))
				continue
			}
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
			if strings.HasPrefix(trimmed, "- **Scope**:") || strings.HasPrefix(trimmed, "- Scope:") {
				currentStoryline.Scope = trimMarkdownField(trimmed, "Scope")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Payoff Style**:") || strings.HasPrefix(trimmed, "- Payoff Style:") {
				currentStoryline.PayoffStyle = trimMarkdownField(trimmed, "Payoff Style")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Setup Role**:") || strings.HasPrefix(trimmed, "- Setup Role:") {
				currentStoryline.SetupRole = trimMarkdownField(trimmed, "Setup Role")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Repeatable Pressure**:") || strings.HasPrefix(trimmed, "- Repeatable Pressure:") {
				currentStoryline.RepeatablePressure = trimMarkdownField(trimmed, "Repeatable Pressure")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Payoff Cadence**:") || strings.HasPrefix(trimmed, "- Payoff Cadence:") {
				currentStoryline.PayoffCadence = trimMarkdownField(trimmed, "Payoff Cadence")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Mutation**:") || strings.HasPrefix(trimmed, "- Mutation:") {
				currentStoryline.Mutation = trimMarkdownField(trimmed, "Mutation")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Failure Mode**:") || strings.HasPrefix(trimmed, "- Failure Mode:") {
				currentStoryline.FailureMode = trimMarkdownField(trimmed, "Failure Mode")
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
			if strings.HasPrefix(trimmed, "- **Desire**:") || strings.HasPrefix(trimmed, "- Desire:") {
				currentStoryline.Desire = trimMarkdownField(trimmed, "Desire")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Opposition**:") || strings.HasPrefix(trimmed, "- Opposition:") {
				currentStoryline.Opposition = trimMarkdownField(trimmed, "Opposition")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Stakes**:") || strings.HasPrefix(trimmed, "- Stakes:") {
				currentStoryline.Stakes = trimMarkdownField(trimmed, "Stakes")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Turn**:") || strings.HasPrefix(trimmed, "- Turn:") {
				currentStoryline.Turn = trimMarkdownField(trimmed, "Turn")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Payoff**:") || strings.HasPrefix(trimmed, "- Payoff:") {
				currentStoryline.Payoff = trimMarkdownField(trimmed, "Payoff")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Open Question**:") || strings.HasPrefix(trimmed, "- Open Question:") {
				currentStoryline.OpenQuestion = trimMarkdownField(trimmed, "Open Question")
				continue
			}
			if strings.HasPrefix(trimmed, "- **Pressure Points**:") || strings.HasPrefix(trimmed, "- Pressure Points:") {
				points := trimMarkdownField(trimmed, "Pressure Points")
				currentStoryline.PressurePoints = splitInlineList(points)
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
		if currentSection != "" && currentSection != "core_cast_item" && currentSection != "storyline_item" && currentSection != "premise_item" {
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
	if currentCoreCast != nil && currentCoreCast.Name != "" {
		setup.CoreCast = append(setup.CoreCast, *currentCoreCast)
	}
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

	case strings.Contains(sectionLower, "writing style") || strings.Contains(sectionLower, "writing_style") ||
		strings.Contains(sectionLower, "写作风格") || strings.Contains(sectionLower, "文风"):
		setup.WritingStyle = parseWritingStyleMarkdown(content)

	case strings.Contains(sectionLower, "long form") || strings.Contains(sectionLower, "long_form") || strings.Contains(sectionLower, "serial plan"):
		setup.LongFormPlan = parseLongFormPlanMarkdown(content)

	case strings.Contains(sectionLower, "tense"):
		setup.Tense = content

	case strings.Contains(sectionLower, "pov"):
		setup.POVStyle = content

	case strings.Contains(sectionLower, "tone") || strings.Contains(sectionLower, "style"):
		setup.Tone = content
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

func repairSetupWithQualityGate(ctx context.Context, setup *models.StorySetup, projectConfig *models.ProjectConfig, stage string) (*models.StorySetup, qualityGateResult, error) {
	gate := runSetupQualityGate(setup)
	if !gate.Blocking {
		return setup, gate, nil
	}
	if setup == nil {
		return nil, gate, fmt.Errorf("setup quality gate failed: setup is nil")
	}

	logger.Info("Setup quality gate found blocking issues after %s; running one repair pass", stage)
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return setup, gate, fmt.Errorf("failed to load LLM config for setup repair: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return setup, gate, fmt.Errorf("failed to create LLM client for setup repair")
	}

	agent := agents.NewSetupAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	review := qualityGateReviewResult("Repair setup quality gate findings before saving project state.", gate)
	session := agents.NewRevisionSession("setup", fmt.Sprintf("Repair %s quality-gate issues before saving project state.", stage))
	session.AddReview(1, *review)
	output, err := agent.Improve(ctx, agents.SetupImproveInput{
		ExistingSetup:   *setup,
		ReviewResult:    *review,
		RevisionContext: session.Prompt(),
	})
	if err != nil {
		return setup, gate, fmt.Errorf("setup quality gate repair failed: %w", err)
	}

	repaired := &output.Setup
	finalGate := runSetupQualityGate(repaired)
	return repaired, finalGate, nil
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

### Writing Style
%s

## Long Form Plan
%s

## Core Cast
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
		formatWritingStyle(setup.WritingStyle),
		formatLongFormPlan(setup.LongFormPlan),
		formatCoreCast(setup.CoreCast),
		formatStorylines(setup.Storylines),
		formatPremises(setup.Premises),
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func formatLongFormPlan(plan *models.LongFormPlan) string {
	if plan == nil || plan.IsZero() {
		return "No long-form plan defined."
	}
	var result strings.Builder
	if plan.TargetChapters > 0 {
		result.WriteString(fmt.Sprintf("- **Target Chapters**: %d\n", plan.TargetChapters))
	}
	if plan.TargetVolumes > 0 {
		result.WriteString(fmt.Sprintf("- **Target Volumes**: %d\n", plan.TargetVolumes))
	}
	if plan.MainLoop != "" {
		result.WriteString(fmt.Sprintf("- **Main Loop**: %s\n", plan.MainLoop))
	}
	if len(plan.EscalationLadder) > 0 {
		result.WriteString(fmt.Sprintf("- **Escalation Ladder**: %s\n", strings.Join(plan.EscalationLadder, "; ")))
	}
	if len(plan.ReaderPromises) > 0 {
		result.WriteString(fmt.Sprintf("- **Reader Promises**: %s\n", strings.Join(plan.ReaderPromises, "; ")))
	}
	if plan.PayoffCadence != "" {
		result.WriteString(fmt.Sprintf("- **Payoff Cadence**: %s\n", plan.PayoffCadence))
	}
	if len(plan.VolumePattern) > 0 {
		result.WriteString(fmt.Sprintf("- **Volume Pattern**: %s\n", strings.Join(plan.VolumePattern, "; ")))
	}
	if plan.MidpointMutation != "" {
		result.WriteString(fmt.Sprintf("- **Midpoint Mutation**: %s\n", plan.MidpointMutation))
	}
	if plan.EndgamePromise != "" {
		result.WriteString(fmt.Sprintf("- **Endgame Promise**: %s\n", plan.EndgamePromise))
	}
	return result.String()
}

func formatCoreCast(seeds []models.CoreCastSeed) string {
	if len(seeds) == 0 {
		return "No core cast seeds defined."
	}
	var result strings.Builder
	for _, seed := range seeds {
		result.WriteString(fmt.Sprintf("\n### %s\n", seed.Name))
		if seed.ID != "" {
			result.WriteString(fmt.Sprintf("- **ID**: %s\n", seed.ID))
		}
		result.WriteString(fmt.Sprintf("- **Role**: %s\n", seed.Role))
		result.WriteString(fmt.Sprintf("- **Importance**: %d/10\n", seed.Importance))
		result.WriteString(fmt.Sprintf("- **Story Function**: %s\n", seed.StoryFunction))
		if seed.RelationshipToLead != "" {
			result.WriteString(fmt.Sprintf("- **Relationship To Lead**: %s\n", seed.RelationshipToLead))
		}
		if seed.RelationshipArc != "" {
			result.WriteString(fmt.Sprintf("- **Relationship Arc**: %s\n", seed.RelationshipArc))
		}
		if seed.EntryPhase != "" {
			result.WriteString(fmt.Sprintf("- **Entry Phase**: %s\n", seed.EntryPhase))
		}
		if seed.Payoff != "" {
			result.WriteString(fmt.Sprintf("- **Payoff**: %s\n", seed.Payoff))
		}
		if len(seed.StorylineRefs) > 0 {
			result.WriteString(fmt.Sprintf("- **Storyline Refs**: %s\n", strings.Join(seed.StorylineRefs, "; ")))
		}
	}
	return result.String()
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
		if s.Scope != "" {
			result.WriteString(fmt.Sprintf("- **Scope**: %s\n", s.Scope))
		}
		if s.PayoffStyle != "" {
			result.WriteString(fmt.Sprintf("- **Payoff Style**: %s\n", s.PayoffStyle))
		}
		if s.SetupRole != "" {
			result.WriteString(fmt.Sprintf("- **Setup Role**: %s\n", s.SetupRole))
		}
		if s.RepeatablePressure != "" {
			result.WriteString(fmt.Sprintf("- **Repeatable Pressure**: %s\n", s.RepeatablePressure))
		}
		if s.PayoffCadence != "" {
			result.WriteString(fmt.Sprintf("- **Payoff Cadence**: %s\n", s.PayoffCadence))
		}
		if s.Mutation != "" {
			result.WriteString(fmt.Sprintf("- **Mutation**: %s\n", s.Mutation))
		}
		if s.FailureMode != "" {
			result.WriteString(fmt.Sprintf("- **Failure Mode**: %s\n", s.FailureMode))
		}
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", s.Description))
		if s.Desire != "" {
			result.WriteString(fmt.Sprintf("- **Desire**: %s\n", s.Desire))
		}
		if s.Opposition != "" {
			result.WriteString(fmt.Sprintf("- **Opposition**: %s\n", s.Opposition))
		}
		if s.Stakes != "" {
			result.WriteString(fmt.Sprintf("- **Stakes**: %s\n", s.Stakes))
		}
		if s.Turn != "" {
			result.WriteString(fmt.Sprintf("- **Turn**: %s\n", s.Turn))
		}
		if s.Payoff != "" {
			result.WriteString(fmt.Sprintf("- **Payoff**: %s\n", s.Payoff))
		}
		if s.OpenQuestion != "" {
			result.WriteString(fmt.Sprintf("- **Open Question**: %s\n", s.OpenQuestion))
		}
		if len(s.PressurePoints) > 0 {
			result.WriteString(fmt.Sprintf("- **Pressure Points**: %s\n", strings.Join(s.PressurePoints, "; ")))
		}
	}
	return result.String()
}

func trimMarkdownField(line, field string) string {
	line = strings.TrimPrefix(line, "- **"+field+"**:")
	line = strings.TrimPrefix(line, "- "+field+":")
	return strings.TrimSpace(line)
}

func splitInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	separators := []string{"；", ";", "，", ","}
	parts := []string{value}
	for _, sep := range separators {
		var next []string
		for _, part := range parts {
			next = append(next, strings.Split(part, sep)...)
		}
		parts = next
	}

	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimPrefix(part, "-"))
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseWritingStyleMarkdown(content string) models.WritingStyle {
	var style models.WritingStyle
	var freeform []string
	var referenceLines []string
	currentList := ""
	inReference := false

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || line == "None" || line == "(待填写)" {
			continue
		}

		if key, value, ok := parseMarkdownLabel(line); ok {
			normalizedKey := normalizeMarkdownLabel(key)
			wasInReference := inReference
			currentList = ""
			inReference = false

			switch normalizedKey {
			case "name", "style name", "writing style name", "名称", "风格名称":
				style.Name = value
			case "description", "style description", "writing style description", "描述", "风格描述":
				style.Description = value
			case "principles", "style principles", "writing principles", "原则", "写作原则":
				style.Principles = append(style.Principles, splitInlineList(value)...)
				currentList = "principles"
			case "avoid", "avoidance", "dont", "don't", "避免", "禁忌", "避免事项":
				style.Avoid = append(style.Avoid, splitInlineList(value)...)
				currentList = "avoid"
			case "reference excerpt", "reference", "sample", "参考文章", "参考片段", "参考文":
				if value != "" {
					referenceLines = append(referenceLines, value)
				}
				inReference = true
			default:
				if wasInReference {
					referenceLines = append(referenceLines, line)
					inReference = true
				} else {
					freeform = append(freeform, line)
				}
			}
			continue
		}

		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		switch {
		case inReference:
			referenceLines = append(referenceLines, line)
		case currentList == "principles":
			style.Principles = append(style.Principles, line)
		case currentList == "avoid":
			style.Avoid = append(style.Avoid, line)
		default:
			freeform = append(freeform, line)
		}
	}

	if style.Description == "" && len(freeform) > 0 {
		style.Description = strings.Join(freeform, "\n")
	}
	style.ReferenceExcerpt = strings.TrimSpace(strings.Join(referenceLines, "\n"))
	return style.CompactReference(len([]rune(style.ReferenceExcerpt)))
}

func parseLongFormPlanMarkdown(content string) *models.LongFormPlan {
	var plan models.LongFormPlan
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := parseMarkdownLabel(line)
		if !ok {
			continue
		}
		switch normalizeMarkdownLabel(key) {
		case "target chapters":
			fmt.Sscanf(value, "%d", &plan.TargetChapters)
		case "target volumes":
			fmt.Sscanf(value, "%d", &plan.TargetVolumes)
		case "main loop":
			plan.MainLoop = value
		case "escalation ladder":
			plan.EscalationLadder = splitInlineList(value)
		case "reader promises":
			plan.ReaderPromises = splitInlineList(value)
		case "payoff cadence":
			plan.PayoffCadence = value
		case "volume pattern":
			plan.VolumePattern = splitInlineList(value)
		case "midpoint mutation":
			plan.MidpointMutation = value
		case "endgame promise":
			plan.EndgamePromise = value
		}
	}
	if plan.IsZero() {
		return nil
	}
	return &plan
}

func parseMarkdownLabel(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(line), "- "), "* "))
	if line == "" {
		return "", "", false
	}

	if strings.HasPrefix(line, "**") {
		rest := line[2:]
		if end := strings.Index(rest, "**"); end >= 0 {
			key = strings.TrimSpace(rest[:end])
			value = strings.TrimSpace(rest[end+2:])
			value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, ":"), "："))
			return key, value, key != ""
		}
	}

	colon := strings.Index(line, ":")
	sepLen := len(":")
	fullColon := strings.Index(line, "：")
	if colon < 0 || (fullColon >= 0 && fullColon < colon) {
		colon = fullColon
		sepLen = len("：")
	}
	if colon <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:colon]), strings.TrimSpace(line[colon+sepLen:]), true
}

func normalizeMarkdownLabel(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.Trim(key, "*` ")
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, "-", " ")
	key = strings.Join(strings.Fields(key), " ")
	return key
}

func formatWritingStyle(style models.WritingStyle) string {
	if style.IsZero() {
		return "None"
	}
	var result strings.Builder
	if style.Name != "" {
		result.WriteString(fmt.Sprintf("- **Name**: %s\n", style.Name))
	}
	if style.Description != "" {
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", style.Description))
	}
	if len(style.Principles) > 0 {
		result.WriteString("- **Principles**:\n")
		for _, item := range style.Principles {
			result.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	if len(style.Avoid) > 0 {
		result.WriteString("- **Avoid**:\n")
		for _, item := range style.Avoid {
			result.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	if style.ReferenceExcerpt != "" {
		result.WriteString("- **Reference Excerpt**:\n")
		result.WriteString(style.ReferenceExcerpt)
		if !strings.HasSuffix(style.ReferenceExcerpt, "\n") {
			result.WriteString("\n")
		}
	}
	return strings.TrimRight(result.String(), "\n")
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

	logQualityGateResult("setup", runSetupQualityGate(&setup))

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
	fmt.Printf("Premises: %d | Storylines: %d | Core cast: %d | Timeline events: %d | Resources: %d\n\n",
		len(setup.Premises), len(setup.Storylines), len(setup.CoreCast), len(setup.WorldTimeline), len(setup.WorldResources))

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

	for _, storyline := range setup.Storylines {
		texture := 0
		for _, value := range []string{
			storyline.Desire,
			storyline.Opposition,
			storyline.Stakes,
			storyline.Turn,
			storyline.Payoff,
			storyline.OpenQuestion,
			storyline.Scope,
			storyline.PayoffStyle,
			storyline.SetupRole,
		} {
			if strings.TrimSpace(value) != "" {
				texture++
			}
		}
		if len(storyline.PressurePoints) > 0 {
			texture++
		}
		if texture < 3 {
			fmt.Printf("  💡 [storyline] '%s' 可补充欲望、阻力、代价、反转或回收提示，让故事线更有压力\n", storyline.Name)
			suggestions++
		}
	}

	for _, seed := range setup.CoreCast {
		if seed.Importance >= 8 && (strings.TrimSpace(seed.StoryFunction) == "" || strings.TrimSpace(seed.Payoff) == "") {
			fmt.Printf("  馃挕 [core_cast] '%s' 鍙ˉ鍏呮晠浜嬪姛鑳戒笌 payoff锛岃 craft 鎵╁啓鏃朵笉浼氭紓绉籠n", seed.Name)
			suggestions++
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
