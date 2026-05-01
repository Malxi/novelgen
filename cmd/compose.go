package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/rpg"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	composeIDFlag           string
	composePromptFlag       string
	composeMaxRoundsFlag    int
	composeConcurrencyFlag  int
	composeHierarchicalFlag bool
	composeOneShotFlag      bool
	composeForceImproveFlag bool
	composeForceGenFlag     bool
	composeCheckJSONFlag    bool
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
  novelgen compose gen                      # Generate using hierarchical approach with progress resume
  novelgen compose gen --one-shot           # Try one-shot generation, fallback to hierarchical on failure`,
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
  novelgen compose improve                  # Improve outline with hierarchical partial repair
  novelgen compose improve --max-rounds 3   # Run 3 improvement rounds
  novelgen compose improve --one-shot       # Try one-shot improvement, fallback to hierarchical on failure`,
	RunE: runComposeImprove,
}

var composeCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate outline for structural, continuity, and timeline issues",
	Long: `Run comprehensive validation against the current outline without calling AI.

Checks include:
  - Structure: part/volume/chapter completeness
  - Characters: presence consistency across chapters
  - Plot logic: events match state changes
  - Pacing: rhythm and tension distribution
  - Transitions: location changes and time jumps
  - Timeline: cross-chapter time consistency (NEW)
  - State Anchor: protagonist state at chapter boundaries (NEW)

Examples:
  novelgen compose check                  # Check entire outline
  novelgen compose check --json           # Output as JSON`,
	RunE: runComposeCheck,
}

var composeNormalizeCmd = &cobra.Command{
	Use:   "normalize",
	Short: "Apply deterministic outline cleanups without calling AI",
	Long: `Apply deterministic, schema-preserving cleanup rules to the current outline.

This does not call AI. It canonicalizes state anchors, syncs generated markdown,
and writes a normalization report under story/compose/.`,
	RunE: runComposeNormalize,
}

func init() {
	composeCheckCmd.Flags().BoolVar(&composeCheckJSONFlag, "json", false, "Output results as JSON")
	composeCmd.AddCommand(composeGenCmd)
	composeCmd.AddCommand(composeRegenCmd)
	composeCmd.AddCommand(composeImproveCmd)
	composeCmd.AddCommand(composeCheckCmd)
	composeCmd.AddCommand(composeNormalizeCmd)

	composeGenCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical generation (better quality, slower)")
	composeGenCmd.Flags().BoolVar(&composeOneShotFlag, "one-shot", false, "Try one-shot generation first (falls back to hierarchical on failure)")
	composeGenCmd.Flags().BoolVar(&composeForceGenFlag, "force", false, "Force regeneration even if outline exists (old outline will be backed up)")
	composeRegenCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Suggestions for regeneration")
	composeImproveCmd.Flags().IntVar(&composeMaxRoundsFlag, "max-rounds", 1, "Maximum number of improvement rounds")
	composeImproveCmd.Flags().IntVar(&composeConcurrencyFlag, "concurrency", 3, "Maximum number of concurrent regeneration tasks")
	composeImproveCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical improvement (better quality, slower)")
	composeImproveCmd.Flags().BoolVar(&composeOneShotFlag, "one-shot", false, "Try one-shot improvement first (falls back to hierarchical on failure)")
	composeImproveCmd.Flags().BoolVar(&composeForceImproveFlag, "force", false, "Force improvement based on suggestions even if score meets threshold")
	composeImproveCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Additional user suggestions for improvement")

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
		if !composeForceGenFlag {
			return fmt.Errorf("outline already exists at %s. Use 'novelgen compose regen' to regenerate, 'novelgen compose improve' to improve, or add --force to regenerate with backup", outlinePath)
		}

		// Backup existing outline
		logger.Info("Outline exists, creating backup...")
		backupDir := filepath.Join("story", "compose", "backups")
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		// Generate backup filename with timestamp
		timestamp := time.Now().Format("20060102_150405")
		backupPath := filepath.Join(backupDir, fmt.Sprintf("outline_%s.json", timestamp))

		// Read and backup current outline
		outlineData, err := os.ReadFile(outlinePath)
		if err != nil {
			return fmt.Errorf("failed to read existing outline for backup: %w", err)
		}

		if err := os.WriteFile(backupPath, outlineData, 0644); err != nil {
			return fmt.Errorf("failed to backup outline: %w", err)
		}

		// Also backup markdown if exists
		mdPath := filepath.Join("story", "compose", "outline.md")
		if _, err := os.Stat(mdPath); err == nil {
			mdData, _ := os.ReadFile(mdPath)
			mdBackupPath := filepath.Join(backupDir, fmt.Sprintf("outline_%s.md", timestamp))
			os.WriteFile(mdBackupPath, mdData, 0644)
		}

		logger.Info("Outline backed up to: %s", backupPath)
		fmt.Printf("\n📦 Existing outline backed up to: %s\n\n", backupPath)
	}

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}

	// AI generation mode. Default to hierarchical generation because it saves
	// progress per volume and avoids asking the model to emit one huge outline.
	var outline *models.Outline
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	if useHierarchical {
		logger.Info("Using hierarchical generation mode (better quality)")
		outline, err = generateOutlineHierarchical(setup, projectConfig)
	} else {
		logger.Info("Using one-shot generation mode (faster, less stable for long outlines)")
		outline, err = generateOutlineWithAI(setup, projectConfig)
		if err != nil {
			logger.Warn("One-shot outline generation failed; falling back to hierarchical generation: %v", err)
			outline, err = generateOutlineHierarchical(setup, projectConfig)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to generate outline with AI: %w", err)
	}

	logQualityGateResult("outline", runOutlineQualityGate(setup, outline))

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

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}

	// Default to hierarchical partial improvement for stability. --one-shot is
	// still available for smaller outlines and falls back to hierarchical if it
	// cannot produce valid output.
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	// Run improvement
	if err := iterateOutlineImprovement(outline, setup, projectConfig, composeMaxRoundsFlag, composeConcurrencyFlag, useHierarchical, composeForceImproveFlag, composePromptFlag); err != nil {
		logger.Error("Improvement failed: %v", err)
		return fmt.Errorf("improvement failed: %w", err)
	}

	logQualityGateResult("outline", runOutlineQualityGate(setup, outline))

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

func runComposeCheck(cmd *cobra.Command, args []string) error {
	logger.Section("NOVELGEN COMPOSE CHECK")

	outlinePath := filepath.Join("story", "compose", "outline.json")
	if _, err := os.Stat(outlinePath); err != nil {
		return fmt.Errorf("outline not found at %s. Run 'novelgen compose gen' first", outlinePath)
	}

	data, err := os.ReadFile(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to read outline: %w", err)
	}

	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(data, &storyOutline); err != nil {
		return fmt.Errorf("failed to parse outline: %w", err)
	}

	var modelOutline models.Outline
	if err := json.Unmarshal(data, &modelOutline); err == nil {
		setupPath := filepath.Join("story", "setup", "story_setup.json")
		var setupForGate *models.StorySetup
		if loadedSetup, loadErr := models.LoadStorySetup(setupPath); loadErr == nil {
			setupForGate = loadedSetup
		}
		logQualityGateResult("outline", runOutlineQualityGate(setupForGate, &modelOutline))
	}

	validator := rpg.NewOutlineValidator(&storyOutline)
	result := validator.Validate()

	if composeCheckJSONFlag {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("\n===== OUTLINE VALIDATION =====\n")
	if result.IsValid {
		fmt.Println("✓ Outline passed validation")
	} else {
		fmt.Printf("✗ Issues: %d | Warnings: %d | Suggestions: %d\n\n",
			result.IssueCount, result.WarningCount, len(result.Suggestions))
	}

	for _, issue := range result.Issues {
		icon := "✗"
		if issue.Severity == "critical" {
			icon = "⛔"
		}
		fmt.Printf("  %s [%s/%s] %s: %s\n", icon, issue.Type, issue.Severity, issue.Location, issue.Description)
		if issue.Fix != "" {
			fmt.Printf("       Fix: %s\n", issue.Fix)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n  --- Warnings (%d) ---\n", result.WarningCount)
	}
	for _, w := range result.Warnings {
		fmt.Printf("  ⚠ [%s] %s: %s\n", w.Type, w.Location, w.Description)
		if w.Suggestion != "" {
			fmt.Printf("     → %s\n", w.Suggestion)
		}
	}

	if len(result.Suggestions) > 0 {
		fmt.Printf("\n  --- Suggestions (%d) ---\n", len(result.Suggestions))
	}
	for _, s := range result.Suggestions {
		fmt.Printf("  💡 [%s] %s\n", s.Type, s.Location)
		fmt.Printf("     Current: %s\n", s.Current)
		fmt.Printf("     → %s\n", s.Suggested)
	}

	fmt.Printf("\n%s\n", result.Summary)

	// Cross-module: verify outline consistency with story setup
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if setupData, err := os.ReadFile(setupPath); err == nil {
		var setup models.StorySetup
		if json.Unmarshal(setupData, &setup) == nil {
			crossIssues, crossWarnings := validateSetupOutlineCross(&setup, &storyOutline)
			if len(crossIssues)+len(crossWarnings) > 0 {
				// Collect missing resources and auto-patch setup
				missingResources := make(map[string]bool)
				for _, part := range storyOutline.Parts {
					for _, vol := range part.Volumes {
						for _, ch := range vol.Chapters {
							for _, entry := range ch.ResourceLedger {
								found := false
								for _, r := range setup.WorldResources {
									if r.Name == entry.Item {
										found = true
										break
									}
								}
								if !found && entry.Item != "" {
									missingResources[entry.Item] = true
								}
							}
						}
					}
				}
				if len(missingResources) > 0 {
					for name := range missingResources {
						setup.WorldResources = append(setup.WorldResources, models.WorldResource{
							Name: name, Category: "通用", Scarcity: "稀有",
							Description: "(自动添加 — 请手动补充描述)",
						})
					}
					if saveErr := setup.Save(setupPath); saveErr == nil {
						fmt.Printf("\n  ✓ Auto-patched setup: added %d missing resources\n", len(missingResources))
					}
				}

				fmt.Printf("\n  --- Cross-Module (Setup↔Outline) ---\n")
				for _, w := range crossWarnings {
					fmt.Printf("  ⚠ [cross] %s\n", w)
				}
				for _, i := range crossIssues {
					fmt.Printf("  ✗ [cross] %s\n", i)
				}
			}
		}
	}

	return nil
}

func runComposeNormalize(cmd *cobra.Command, args []string) error {
	logger.Section("NOVELGEN COMPOSE NORMALIZE")

	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to load outline at %s: %w", outlinePath, err)
	}

	report := models.NormalizeOutline(outline)
	if !report.Changed() {
		fmt.Println("✓ Outline already normalized")
		return nil
	}

	if err := backupOutlineFiles(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal normalized outline: %w", err)
	}
	if err := os.WriteFile(outlinePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write normalized outline: %w", err)
	}

	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	reportPath := filepath.Join("story", "compose", "outline_normalization_manual.json")
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal normalization report: %w", err)
	}
	if err := os.WriteFile(reportPath, reportData, 0644); err != nil {
		return fmt.Errorf("failed to write normalization report: %w", err)
	}

	fmt.Printf("✓ Applied %d deterministic outline cleanup(s)\n", len(report.Changes))
	fmt.Printf("✓ Updated %s and %s\n", outlinePath, mdPath)
	fmt.Printf("✓ Report saved to %s\n", reportPath)
	return nil
}

func validateSetupOutlineCross(setup *models.StorySetup, outline *rpg.StoryOutline) (issues, warnings []string) {
	// Collect defined faction tiers from setup premises
	setupFactions := make(map[string]map[string]bool) // faction → tier → true
	for _, p := range setup.Premises {
		if p.Category == "" {
			continue
		}
		for _, faction := range setupFactionAliases(p.Category) {
			if setupFactions[faction] == nil {
				setupFactions[faction] = make(map[string]bool)
			}
			for tier := range setupPremiseTierAliases(p, faction) {
				setupFactions[faction][tier] = true
			}
		}
	}

	// Collect defined resources from setup
	setupResources := make(map[string]bool)
	for _, r := range setup.WorldResources {
		setupResources[r.Name] = true
	}

	// Check outline enemies against setup faction definitions
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, enemy := range ch.Enemies {
					if enemy.Faction != "" && len(setupFactions) > 0 {
						if tiers, ok := setupFactions[enemy.Faction]; ok {
							if enemy.Tier != "" && !tiers[enemy.Tier] {
								issues = append(issues, fmt.Sprintf(
									"%s: enemy '%s' tier '%s' not defined in setup faction '%s'",
									ch.ID, enemy.Name, enemy.Tier, enemy.Faction))
							}
						} else {
							warnings = append(warnings, fmt.Sprintf(
								"%s: enemy faction '%s' not defined in setup premises",
								ch.ID, enemy.Faction))
						}
					}
				}
				for _, entry := range ch.ResourceLedger {
					if entry.Item != "" && len(setupResources) > 0 {
						if !setupResources[entry.Item] {
							// Find closest match in setup to suggest
							closest := findClosestResource(entry.Item, setupResources)
							hint := fmt.Sprintf("%s: resource '%s' not declared in setup.world_resources",
								ch.ID, entry.Item)
							if closest != "" {
								hint += fmt.Sprintf(" (setup has: '%s' — consider using that name)", closest)
							}
							warnings = append(warnings, hint)
						}
					}
				}
			}
		}
	}
	return
}

func findClosestResource(target string, candidates map[string]bool) string {
	// Check if any candidate is a substring of target or vice versa
	targetLower := strings.ToLower(target)
	for name := range candidates {
		nameLower := strings.ToLower(name)
		// Simple shared-character heuristic: if they share >50% of characters
		shared := 0
		for _, r := range targetLower {
			if strings.ContainsRune(nameLower, r) {
				shared++
			}
		}
		if float64(shared)/float64(len([]rune(targetLower))) > 0.4 {
			return name
		}
	}
	return ""
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

func setupFactionAliases(category string) []string {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil
	}
	seen := map[string]bool{}
	var aliases []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		aliases = append(aliases, value)
	}
	add(category)
	for _, part := range strings.FieldsFunc(category, func(r rune) bool {
		return r == '/' || r == '\\' || r == '|' || r == ',' || r == '，' || r == ';' || r == '；' || r == ' '
	}) {
		add(part)
	}
	return aliases
}

func setupPremiseTierAliases(p models.Premise, faction string) map[string]bool {
	tiers := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			tiers[value] = true
		}
	}

	for _, stage := range p.Progression {
		add(stage.Name)
		text := stage.Name + " " + stage.Description + " " + stage.Requirements
		switch {
		case strings.Contains(text, "工虫"):
			add("drone")
		case strings.Contains(text, "兵虫"):
			add("soldier")
		case strings.Contains(text, "初级虫将"):
			add("captain")
		case strings.Contains(text, "高级虫统领"):
			add("commander")
		case strings.Contains(text, "母巢"):
			add("queen")
			add("hive")
		}
		if strings.Contains(text, "机甲") {
			add("mech")
		}
	}

	switch strings.ToLower(strings.TrimSpace(faction)) {
	case "zerg":
		add("drone")
		add("soldier")
		add("captain")
		add("commander")
		add("queen")
		add("hive")
	case "shen":
		add("agent")
		add("informant")
		add("soldier")
		add("mech")
	}

	return tiers
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

func backupOutlineFiles() error {
	outlinePath := filepath.Join("story", "compose", "outline.json")
	outlineData, err := os.ReadFile(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to read outline for backup: %w", err)
	}

	backupDir := filepath.Join("story", "compose", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("outline_%s.json", timestamp))
	if err := os.WriteFile(backupPath, outlineData, 0644); err != nil {
		return fmt.Errorf("failed to backup outline: %w", err)
	}

	mdPath := filepath.Join("story", "compose", "outline.md")
	if mdData, err := os.ReadFile(mdPath); err == nil {
		mdBackupPath := filepath.Join(backupDir, fmt.Sprintf("outline_%s.md", timestamp))
		if err := os.WriteFile(mdBackupPath, mdData, 0644); err != nil {
			return fmt.Errorf("failed to backup outline markdown: %w", err)
		}
	}

	logger.Info("Outline backed up to: %s", backupPath)
	return nil
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

// runOutlineValidatorOnModel converts models.Outline to rpg.StoryOutline,
// runs the outline validator, and returns issues as ReviewSuggestions.
func runOutlineValidatorOnModel(outline *models.Outline) []models.ReviewSuggestion {
	if outline == nil {
		return nil
	}
	// Convert via JSON roundtrip (models.Outline → rpg.StoryOutline)
	data, err := json.Marshal(outline)
	if err != nil {
		return nil
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(data, &storyOutline); err != nil {
		return nil
	}

	validator := rpg.NewOutlineValidator(&storyOutline)
	result := validator.Validate()

	var suggestions []models.ReviewSuggestion

	for _, issue := range result.Issues {
		suggestions = append(suggestions, models.ReviewSuggestion{
			Category:   issue.Type,
			TargetID:   issue.Location,
			TargetName: issue.Location,
			Issue:      issue.Description,
			Suggestion: issue.Fix,
			Priority:   severityToPriorityStr(issue.Severity),
		})
	}
	for _, w := range result.Warnings {
		suggestions = append(suggestions, models.ReviewSuggestion{
			Category:   w.Type,
			TargetID:   w.Location,
			TargetName: w.Location,
			Issue:      w.Description,
			Suggestion: w.Suggestion,
			Priority:   models.PriorityMedium,
		})
	}
	for _, s := range result.Suggestions {
		suggestions = append(suggestions, outlineSuggestionToReviewSuggestion(s))
	}
	return suggestions
}

func outlineSuggestionToReviewSuggestion(s rpg.OutlineSuggestion) models.ReviewSuggestion {
	issue := strings.TrimSpace(s.Reason)
	if issue == "" {
		issue = outlineSuggestionIssueFromType(s.Type)
	}
	if current := shortEvidence(s.Current, 120); current != "" {
		issue = fmt.Sprintf("%s；当前表现：%s", issue, current)
	}

	suggestion := strings.TrimSpace(s.Suggested)
	if suggestion == "" {
		suggestion = "按问题原因补充一个具体、可执行的章节调整。"
	}

	return models.ReviewSuggestion{
		Category:   s.Type,
		TargetID:   s.Location,
		TargetName: s.Location,
		Issue:      issue,
		Suggestion: suggestion,
		Priority:   models.PriorityLow,
	}
}

func outlineSuggestionIssueFromType(issueType string) string {
	switch strings.TrimSpace(issueType) {
	case "logic":
		return "章节逻辑需要更明确的因果或阶段性结果"
	case "conflict":
		return "章节冲突强度或落点不足"
	case "pacing":
		return "章节节奏需要调整"
	case "timeline":
		return "时间线锚点或过渡不够清晰"
	case "state_anchor":
		return "章节状态锚点缺少可追踪的变化依据"
	case "storyline_texture":
		return "故事线推进记录缺少戏剧压力、后果或具体变化"
	default:
		return "大纲存在可优化项"
	}
}

func shortEvidence(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "缺少") || strings.HasPrefix(value, "chapter has") || strings.Contains(value, "entry has no") {
		return value
	}
	runes := []rune(value)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return value
}

func severityToPriorityStr(severity string) string {
	switch severity {
	case "critical":
		return models.PriorityHigh
	case "major":
		return models.PriorityHigh
	case "minor":
		return models.PriorityMedium
	default:
		return models.PriorityLow
	}
}

// iterateOutlineImprovement runs the review-improvement loop
func iterateOutlineImprovement(outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, maxIterations int, concurrency int, hierarchical bool, forceImprove bool, userPrompt string) error {
	logger.Section("Outline Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)
	if hierarchical {
		logger.Info("Mode: Hierarchical (review entire outline, improve volumes individually)")
	} else {
		logger.Info("Mode: One-shot (review and improve entire outline at once)")
	}
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
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

	applyOutlineNormalization(outline, "pre_improve")

	var improvedOutline *models.Outline
	var review *models.ReviewResult

	if hierarchical {
		// Use hierarchical iteration
		improvedOutline, review, err = agent.IterateHierarchical(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
	} else {
		// Use one-shot iteration
		improvedOutline, review, err = agent.Iterate(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
		if err != nil {
			logger.Warn("One-shot outline improve failed; falling back to hierarchical partial improve: %v", err)
			improvedOutline, review, err = agent.IterateHierarchical(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
		}
	}

	if err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	// Update the outline with improved version
	applyOutlineNormalization(improvedOutline, "post_improve")
	*outline = *improvedOutline

	// Enrich with direct model checks + DSL simulation + outline validator feedback.
	if review != nil {
		gate := runOutlineQualityGate(setup, improvedOutline)
		if len(gate.Suggestions) > 0 {
			review.Suggestions = append(review.Suggestions, gate.Suggestions...)
			logger.Info("Quality gate added %d suggestions to review", len(gate.Suggestions))
		}

		// Feed DSL simulation feedback back into outline improve. Critical
		// findings force repair; softer story-contract findings get one direct
		// repair pass so the simulation loop can actually improve the outline.
		if gate.Blocking && maxIterations > 0 {
			logger.Info("Quality gate blocking feedback detected, running partial repair pass with enriched review")
			repairOutput, repairErr := agent.RepairByReview(ctx, improvedOutline, review, setup)
			if repairErr == nil {
				improvedOutline = repairOutput
				applyOutlineNormalization(improvedOutline, "post_repair")
				*outline = *improvedOutline
				logger.Info("Partial compose repair pass completed for DSL/validator feedback")
			} else {
				logger.Warn("Partial compose repair pass failed: %v", repairErr)
			}
		}
	}

	// Save intermediate result
	outlinePath := filepath.Join("story", "compose", fmt.Sprintf("outline_iter_%d.json", maxIterations))
	if err := outline.Save(outlinePath); err != nil {
		logger.Error("Failed to save intermediate outline: %v", err)
	} else {
		logger.Info("Saved intermediate outline to %s", outlinePath)
	}

	logger.Section("Iteration Complete")
	if review != nil {
		logger.Info("Final Review Score: %.1f/100", review.OverallScore)
	}
	return nil
}

func applyOutlineNormalization(outline *models.Outline, stage string) {
	report := models.NormalizeOutline(outline)
	if !report.Changed() {
		return
	}

	logger.Info("Outline normalizer applied %d changes at %s", len(report.Changes), stage)
	reportPath := filepath.Join("story", "compose", fmt.Sprintf("outline_normalization_%s.json", stage))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		logger.Warn("Failed to marshal outline normalization report: %v", err)
		return
	}
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		logger.Warn("Failed to save outline normalization report: %v", err)
	}
}
