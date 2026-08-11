package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	composeIDFlag               string
	composePromptFlag           string
	composeReviewFocusFlag      string
	composeReviewMatrixFlag     bool
	composeCrossVolumeFlag      bool
	composeReviewFromVolumeFlag int
	composeReviewToVolumeFlag   int
	composeReviewModelsFlag     string
	composeReviewSampleFlag     int
	composeReviewSeedFlag       int64
	composeReviewParallelFlag   int
	composeMaxRoundsFlag        int
	composeConcurrencyFlag      int
	composeHierarchicalFlag     bool
	composeOneShotFlag          bool
	composeAgentSDKFlag         bool
	composeAgentApplyFlag       bool
	composeRepairBudgetFlag     int
	composeForceImproveFlag     bool
	composeForceGenFlag         bool
	composeCheckJSONFlag        bool
	composeCheckSuggestionsOut  string
	composeImproveVolume        int
	composeImproveFromVol       int
	composeImproveToVol         int
	composeSuggestionsFlag      string
	composeCrossVolumeAllFlag   bool
	composeReviewOutFlag        string
	composeModelFlag            string
	composePipelineFromVol      int
	composePipelineToVol        int
	composePipelineMaxRounds    int
	composePipelineForce        bool
	composePipelineSkipGen      bool
	composePipelineSkipImprove  bool
	composePipelineSkipCross    bool
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Generate story outline",
	Long: `Generate a story outline with a rigid 3-level structure (parts -> volumes -> chapters).

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
  improve - Improve existing outline through AI review
  skeleton-review - Review parts/volumes before chapter generation
  skeleton-improve - Improve parts/volumes while preserving chapters
  pipeline - Run checkpointed per-volume compose workflow`,
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

var composeSkeletonReviewCmd = &cobra.Command{
	Use:   "skeleton-review",
	Short: "Review the outline skeleton without requiring chapters",
	Long: `Review the current outline skeleton before chapter generation.

This command checks only parts, volumes, summaries, volume-to-volume causality,
and payoff contracts. It does not require chapters and does not modify files.`,
	RunE: runComposeSkeletonReview,
}

var composeReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review the outline and save an AI review report",
	Long: `Review the current outline and save a review report without modifying files.

The default path uses the legacy single-shot review. With --agent-sdk the review
runs through the agent runtime: the model reads the outline with read-only
novelgen tool queries and returns open-ended review_result suggestions.

Examples:
  novelgen compose review --agent-sdk
  novelgen compose review --agent-sdk --prompt "重点检查角色动机和伏笔回收节奏"
  novelgen compose review --agent-sdk --volume 2
  novelgen compose review --agent-sdk --out story/reviews/outline_review.json`,
	RunE: runComposeReview,
}

var composeSkeletonImproveCmd = &cobra.Command{
	Use:   "skeleton-improve",
	Short: "Improve the outline skeleton without generating chapters",
	Long: `Improve the current outline skeleton through AI review and refinement.

This command only changes part/volume titles, summaries, and payoff contracts.
It preserves part IDs, volume IDs, and any existing chapter arrays.`,
	RunE: runComposeSkeletonImprove,
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

var composePipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run checkpointed compose generation one volume at a time",
	Long: `Run the compose workflow as a resumable per-volume pipeline.

The pipeline keeps story/compose/outline.json as the canonical state:
  1. create or load the outline skeleton
  2. normalize the skeleton/state checkpoint
  3. generate one selected empty volume
  4. improve that generated volume
  5. apply deterministic setup/outline cross patches

Examples:
  novelgen compose pipeline --from-volume 1 --to-volume 1
  novelgen compose pipeline --from-volume 2 --to-volume 7 --max-rounds 1`,
	RunE: runComposePipeline,
}

func init() {
	composeCheckCmd.Flags().BoolVar(&composeCheckJSONFlag, "json", false, "Output results as JSON")
	composeCheckCmd.Flags().StringVar(&composeCheckSuggestionsOut, "suggestions-out", "", "Write deterministic check suggestions as a ReviewResult JSON report to this path")
	composeCmd.AddCommand(composeGenCmd)
	composeCmd.AddCommand(composeRegenCmd)
	composeCmd.AddCommand(composeImproveCmd)
	composeCmd.AddCommand(composeReviewCmd)
	composeCmd.AddCommand(composeSkeletonReviewCmd)
	composeCmd.AddCommand(composeSkeletonImproveCmd)
	composeCmd.AddCommand(composeCheckCmd)
	composeCmd.AddCommand(composeNormalizeCmd)
	composeCmd.AddCommand(composePipelineCmd)

	composeGenCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical generation (better quality, slower)")
	composeGenCmd.Flags().BoolVar(&composeOneShotFlag, "one-shot", false, "Try one-shot generation first (falls back to hierarchical on failure)")
	composeGenCmd.Flags().BoolVar(&composeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow with read-only project query tools")
	composeGenCmd.Flags().BoolVar(&composeForceGenFlag, "force", false, "Force regeneration even if outline exists (old outline will be backed up)")
	composeRegenCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Suggestions for regeneration")
	composeImproveCmd.Flags().IntVar(&composeMaxRoundsFlag, "max-rounds", 1, "Maximum number of improvement rounds")
	composeImproveCmd.Flags().IntVar(&composeConcurrencyFlag, "concurrency", 3, "Maximum number of concurrent regeneration tasks")
	composeImproveCmd.Flags().BoolVar(&composeHierarchicalFlag, "hierarchical", false, "Use hierarchical improvement (better quality, slower)")
	composeImproveCmd.Flags().BoolVar(&composeOneShotFlag, "one-shot", false, "Try one-shot improvement first (falls back to hierarchical on failure)")
	composeImproveCmd.Flags().BoolVar(&composeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow with read-only project query tools")
	composeImproveCmd.Flags().BoolVar(&composeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write outline patches through validated patch tools")
	composeImproveCmd.Flags().IntVar(&composeRepairBudgetFlag, "repair-budget", 20, "Maximum number of repair suggestions per Agent SDK repair pass")
	composeImproveCmd.Flags().BoolVar(&composeForceImproveFlag, "force", false, "Force improvement based on suggestions even if score meets threshold")
	composeImproveCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Additional user suggestions for improvement")
	composeImproveCmd.Flags().IntVar(&composeImproveVolume, "volume", 0, "Improve one 1-based global volume index")
	composeImproveCmd.Flags().IntVar(&composeImproveFromVol, "from-volume", 0, "Improve from this 1-based global volume index")
	composeImproveCmd.Flags().IntVar(&composeImproveToVol, "to-volume", 0, "Improve through this 1-based global volume index")
	composeImproveCmd.Flags().StringVar(&composeSuggestionsFlag, "suggestions", "", "Comma-separated ReviewResult JSON report paths whose suggestions seed Agent SDK improve (requires --agent-sdk)")
	composeImproveCmd.Flags().BoolVar(&composeCrossVolumeAllFlag, "cross-volume-all", false, "With --agent-apply, allow the Agent SDK session to query/check/patch EVERY volume in the outline, not just adjacent ones (for book-level consistency issues)")
	composeImproveCmd.Flags().StringVar(&composeModelFlag, "model", "", "Override the project model for this improve run (e.g. deepseek-v4-pro)")

	composeReviewCmd.Flags().BoolVar(&composeAgentSDKFlag, "agent-sdk", false, "Use Agent SDK workflow with read-only project query tools")
	composeReviewCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Additional user suggestions for review focus")
	composeReviewCmd.Flags().StringVar(&composeReviewFocusFlag, "focus", "", "Built-in review focus: reader,logic,character,commercial,storyline,deai,protagonist,foreshadowing,setup-fidelity,emotion,power-system,novelty,shuangwen (comma-separated, or 'all')")
	composeReviewCmd.Flags().BoolVar(&composeReviewMatrixFlag, "matrix", false, "Run a multi-model x multi-focus review matrix with clustered sampling")
	composeReviewCmd.Flags().BoolVar(&composeCrossVolumeFlag, "cross-volume", false, "Review cross-volume continuity across a volume range in one agent session")
	composeReviewCmd.Flags().IntVar(&composeReviewFromVolumeFlag, "from-volume", 0, "Cross-volume review start (1-based global volume index)")
	composeReviewCmd.Flags().IntVar(&composeReviewToVolumeFlag, "to-volume", 0, "Cross-volume review end (1-based global volume index, default: last)")
	composeReviewCmd.Flags().StringVar(&composeReviewModelsFlag, "models", "", "Comma-separated model list for --matrix (default: project model)")
	composeReviewCmd.Flags().IntVar(&composeReviewSampleFlag, "sample", 10, "Stratified sample size for --matrix output")
	composeReviewCmd.Flags().Int64Var(&composeReviewSeedFlag, "seed", 42, "Random seed for --matrix sampling")
	composeReviewCmd.Flags().IntVar(&composeReviewParallelFlag, "parallel", 4, "Concurrent reviews in --matrix mode")
	composeReviewCmd.Flags().IntVar(&composeImproveVolume, "volume", 0, "Review one 1-based global volume index")
	composeReviewCmd.Flags().StringVar(&composeReviewOutFlag, "out", "story/compose/outline_review.json", "Review report output path")
	composeReviewCmd.Flags().StringVar(&composeModelFlag, "model", "", "Override the project model for this review run (e.g. deepseek-v4-pro)")
	composeSkeletonReviewCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Additional review focus")
	composeSkeletonImproveCmd.Flags().IntVar(&composeMaxRoundsFlag, "max-rounds", 1, "Maximum skeleton improvement rounds")
	composeSkeletonImproveCmd.Flags().BoolVar(&composeForceImproveFlag, "force", false, "Force improvement based on suggestions even if score meets threshold")
	composeSkeletonImproveCmd.Flags().StringVar(&composePromptFlag, "prompt", "", "Additional user suggestions for skeleton improvement")
	composePipelineCmd.Flags().IntVar(&composePipelineFromVol, "from-volume", 1, "Start at this 1-based global volume index")
	composePipelineCmd.Flags().IntVar(&composePipelineToVol, "to-volume", 0, "Stop at this 1-based global volume index (default: all volumes)")
	composePipelineCmd.Flags().IntVar(&composePipelineMaxRounds, "max-rounds", 1, "Maximum improvement rounds per generated volume")
	composePipelineCmd.Flags().BoolVar(&composePipelineForce, "force", false, "Force regeneration/improvement of selected volumes")
	composePipelineCmd.Flags().BoolVar(&composePipelineSkipGen, "skip-gen", false, "Skip volume chapter generation")
	composePipelineCmd.Flags().BoolVar(&composePipelineSkipImprove, "skip-improve", false, "Skip per-volume improvement")
	composePipelineCmd.Flags().BoolVar(&composePipelineSkipCross, "skip-cross", false, "Skip deterministic setup/outline cross patch")
	composePipelineCmd.Flags().BoolVar(&composeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow with read-only project query tools")
	composePipelineCmd.Flags().BoolVar(&composeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write outline patches through validated patch tools during improve")

	// Register compose command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return composeCmd
	})
}

func validateComposeAgentSDKOption(agentSDK, oneShot bool) error {
	if agentSDK && oneShot {
		return fmt.Errorf("--agent-sdk cannot be used with --one-shot")
	}
	return nil
}

func validateComposeAgentApplyOption(agentSDK, agentApply bool) error {
	if agentApply && !agentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
}

func runComposeGen(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE GEN")
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

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

	// Check if outline already exists. Hierarchical generation can resume from
	// an incomplete outline.json whose later volumes still have empty chapters.
	outlinePath := filepath.Join("story", "compose", "outline.json")
	if _, err := os.Stat(outlinePath); err == nil {
		if !composeForceGenFlag && !composeOneShotFlag && canResumeOutlineGeneration(outlinePath, projectConfig.Structure) {
			logger.Info("Found incomplete outline at %s; resuming chapter generation", outlinePath)
		} else if !composeForceGenFlag {
			return fmt.Errorf("outline already exists at %s. Use 'novelgen compose regen' to regenerate, 'novelgen compose improve' to improve, or add --force to regenerate with backup", outlinePath)
		} else {

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
	}

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

	// AI generation mode. Default to hierarchical generation because it saves
	// progress per volume and avoids asking the model to emit one huge outline.
	var outline *models.Outline
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	if composeAgentSDKFlag {
		logger.Info("Using Claude Agent SDK hierarchical generation mode")
		outline, err = generateOutlineHierarchicalAgentSDK(setup, projectConfig)
	} else if useHierarchical {
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

	var gate qualityGateResult
	if composeAgentSDKFlag {
		outline, gate, err = repairOutlineWithQualityGateAgentSDK(cmd.Context(), outline, setup, projectConfig, "generation", false, false, "", nil)
	} else {
		outline, gate, err = repairOutlineWithQualityGate(cmd.Context(), outline, setup, projectConfig, "generation", false)
	}
	if err != nil {
		return err
	}
	logQualityGateResult("outline", gate)

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
	fmt.Printf("\n[ok] Story outline saved to %s\n", outlinePath)
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

	fmt.Printf("\n[ok] Outline regenerated and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func runComposeImprove(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE IMPROVE")
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

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
	originalOutline := cloneOutline(outline)
	if countGeneratedVolumes(outline) == 0 {
		return fmt.Errorf("outline has no generated volumes to improve; run 'novelgen compose gen' first")
	}

	// Backup the current outline before any improvement pass mutates it. Both
	// legacy and Agent SDK improve paths overwrite outline.json in place; the
	// in-memory clone above only guards against a no-op run, so a disk backup
	// is the only way to recover the pre-improve outline after a bad iteration.
	if err := backupOutlineFiles(); err != nil {
		logger.Error("Failed to backup outline before improve: %v", err)
		return err
	}
	logger.Info("Backed up outline before improvement")

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}
	if err := validateComposeAgentApplyOption(composeAgentSDKFlag, composeAgentApplyFlag); err != nil {
		return err
	}

	// Default to hierarchical partial improvement for stability. --one-shot is
	// still available for smaller outlines and falls back to hierarchical if it
	// cannot produce valid output.
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	improveOutline := outline
	selectedImprove := composeImproveVolume > 0 || composeImproveFromVol > 0 || composeImproveToVol > 0
	if selectedImprove {
		improveOutline, err = outlineWithImproveVolumeSelection(outline, composeImproveVolume, composeImproveFromVol, composeImproveToVol)
		if err != nil {
			return err
		}
		logger.Info("Improving selected generated volumes only")
	} else if countEmptyVolumes(outline) > 0 {
		logger.Info("Outline contains empty volumes; improving only generated volumes")
		improveOutline = outlineWithGeneratedVolumes(outline)
	}

	// Run improvement
	if composeAgentSDKFlag {
		// A fresh run (no resume checkpoint) starts an empty improvement report.
		// A resume run keeps the existing report file so already-completed
		// volumes are not re-appended after a transient failure.
		progressPath := filepath.Join("story", "compose", "outline_improve_progress.json")
		keepReport := resumeProgressMatchesRun(progressPath, improveOutline, composeMaxRoundsFlag)
		if !keepReport {
			_ = os.Remove(filepath.Join("story", "compose", "outline_improve_report.jsonl"))
		}
		agentSDKForceImprove := composeForceImproveFlag || selectedImprove
		var suggestions []models.ReviewSuggestion
		if strings.TrimSpace(composeSuggestionsFlag) != "" {
			suggestions, err = loadReviewSuggestionReports(composeSuggestionsFlag)
			if err != nil {
				return err
			}
		}
		// When the run is scoped to selected volume(s), the iteration outline
		// only contains those volumes. Pass the full outline as the
		// cross-volume context so apply mode can patch adjacent volumes in the
		// same session.
		var improveCrossOutline *models.Outline
		if selectedImprove {
			improveCrossOutline = outline
		}
		err = runAgentSDKImproveWithResume(improveOutline, setup, projectConfig, composeMaxRoundsFlag, agentSDKForceImprove, composePromptFlag, composeAgentApplyFlag, improveOutline != outline, suggestions, composeModelFlag, improveCrossOutline)
	} else {
		err = iterateOutlineImprovement(improveOutline, setup, projectConfig, composeMaxRoundsFlag, composeConcurrencyFlag, useHierarchical, composeForceImproveFlag, composePromptFlag)
	}
	if err != nil {
		logger.Error("Improvement failed: %v", err)
		return fmt.Errorf("improvement failed: %w", err)
	}
	var gate qualityGateResult
	if improveOutline != outline {
		if composeAgentSDKFlag {
			var repairCrossOutline *models.Outline
			if selectedImprove {
				repairCrossOutline = outline
			}
			improveOutline, gate, err = repairOutlineWithQualityGateAgentSDK(cmd.Context(), improveOutline, setup, projectConfig, "improvement", composeAgentApplyFlag, true, composePromptFlag, repairCrossOutline)
		} else {
			improveOutline, gate, err = repairOutlineWithQualityGate(cmd.Context(), improveOutline, setup, projectConfig, "improvement", true)
		}
		if err != nil {
			return err
		}
		logQualityGateResult("outline", gate)
		mergeGeneratedVolumes(outline, improveOutline)
	} else {
		if composeAgentSDKFlag {
			outline, gate, err = repairOutlineWithQualityGateAgentSDK(cmd.Context(), outline, setup, projectConfig, "improvement", composeAgentApplyFlag, false, composePromptFlag, nil)
		} else {
			outline, gate, err = repairOutlineWithQualityGate(cmd.Context(), outline, setup, projectConfig, "improvement", false)
		}
		if err != nil {
			return err
		}
		logQualityGateResult("outline", gate)
	}

	// Save improved outline. In Agent SDK selected-volume mode the agent returns
	// a bounded patch; avoid whole-outline normalization here so unselected
	// volumes are not rewritten as a side effect of saving.
	if composeAgentSDKFlag && selectedImprove && outlinesSemanticallyEqual(originalOutline, outline) {
		logger.Info("Agent SDK selected-volume improve made no effective outline changes; skipping outline.json and outline.md rewrite")
		if reportErr := writeComposeImproveReport(-1); reportErr != nil {
			logger.Warn("Failed to write compose improve report: %v", reportErr)
		}
		fmt.Printf("\n[ok] Outline checked; no effective changes were applied to %s\n", outlinePath)
		return nil
	}
	if composeAgentSDKFlag && selectedImprove {
		err = savePartialOutline(outline, outlinePath)
	} else {
		err = outline.Save(outlinePath)
	}
	if err != nil {
		return fmt.Errorf("failed to save improved outline: %w", err)
	}

	// Update markdown version
	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	if composeAgentSDKFlag {
		remainingGateIssues := len(filterMediumOrHigherOutlineTargetSuggestions(gate.Suggestions))
		if reportErr := writeComposeImproveReport(remainingGateIssues); reportErr != nil {
			logger.Warn("Failed to write compose improve report: %v", reportErr)
		}
	}

	fmt.Printf("\n[ok] Outline improved and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

// composeAgentSDKImproveResumeAttempts bounds how many times an Agent SDK
// improvement run is retried after a transient runner/network failure. The
// checkpoint file (outline_improve_progress.json) makes each retry resume from
// the last completed volume instead of restarting the whole range.
const composeAgentSDKImproveResumeAttempts = 3

func runAgentSDKImproveWithResume(
	improveOutline *models.Outline,
	setup *models.StorySetup,
	projectConfig *models.ProjectConfig,
	maxRounds int,
	forceImprove bool,
	userPrompt string,
	applyPatches bool,
	scoped bool,
	suggestions []models.ReviewSuggestion,
	modelOverride string,
	crossOutline *models.Outline,
) error {
	return retryAgentSDKImproveWithResume(composeAgentSDKImproveResumeAttempts, func() error {
		return iterateOutlineImprovementAgentSDK(improveOutline, setup, projectConfig, maxRounds, forceImprove, userPrompt, applyPatches, scoped, suggestions, modelOverride, crossOutline)
	})
}

// loadReviewSuggestionReports reads comma-separated ReviewResult JSON report
// paths and returns their combined suggestions. Both compose review and
// compose check --suggestions-out emit this shape, so any combination of AI
// review and deterministic check reports can seed an Agent SDK improve run.
func loadReviewSuggestionReports(raw string) ([]models.ReviewSuggestion, error) {
	var out []models.ReviewSuggestion
	for _, part := range strings.Split(raw, ",") {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read suggestion report %s: %w", path, err)
		}
		var report models.ReviewResult
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("failed to parse suggestion report %s: %w", path, err)
		}
		out = append(out, report.Suggestions...)
	}
	return out, nil
}

func retryAgentSDKImproveWithResume(attempts int, run func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = run()
		if lastErr == nil {
			return nil
		}
		if attempt < attempts {
			logger.Warn("Agent SDK outline improvement failed (attempt %d/%d): %v; resuming from saved checkpoint", attempt, attempts, lastErr)
		}
	}
	return lastErr
}

// repairAgentSDKWithTransientRetry runs the Agent SDK repair pass, retrying up
// to maxRepairAttempts when the failure looks like a transient CLI/SDK
// transport error (e.g. the CLI control-channel "Stream closed" crash family).
// The repair pass writes its own per-iteration checkpoint, so each retry
// resumes from the last completed volume instead of restarting the whole
// batch. Permanent failures return immediately, and a leftover repair
// checkpoint is removed so a later invocation never resumes stale work.
func repairAgentSDKWithTransientRetry(repair func() (*models.Outline, error)) (*models.Outline, error) {
	const maxRepairAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxRepairAttempts; attempt++ {
		output, err := repair()
		if err == nil {
			return output, nil
		}
		lastErr = err
		if attempt < maxRepairAttempts && isTransientAgentSDKError(err) {
			logger.Warn("Agent SDK repair pass failed with transient CLI/SDK error (attempt %d/%d): %v; resuming from repair checkpoint", attempt, maxRepairAttempts, err)
			continue
		}
		break
	}
	// The repair pass's checkpoint (iteration 0) is only removed on success;
	// clear it on permanent failure so the next invocation starts clean.
	if removeErr := os.Remove("story/compose/outline_improve_progress.json"); removeErr != nil && !os.IsNotExist(removeErr) {
		logger.Warn("Failed to remove stale Agent SDK repair checkpoint: %v", removeErr)
	}
	return nil, lastErr
}

// isTransientAgentSDKError reports whether an Agent SDK runner failure is a
// transient CLI/SDK transport issue that a retry can recover from (the CLI
// control-channel "Stream closed" crash family and network resets), as opposed
// to a deterministic prompt/parse failure that will fail the same way again.
func isTransientAgentSDKError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"error in hook callback",
		"stream closed",
		"connection reset",
		"broken pipe",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

type composeImproveResumeProgress struct {
	Iteration     int      `json:"iteration"`
	TargetVolumes []string `json:"target_volumes"`
}

// resumeProgressMatchesRun reports whether the on-disk improvement progress
// belongs to this command invocation (same iteration window and same target
// volumes). Only then is the improvement report jsonl kept across a restart.
func resumeProgressMatchesRun(progressPath string, outline *models.Outline, maxRounds int) bool {
	data, err := os.ReadFile(progressPath)
	if err != nil {
		return false
	}
	var progress composeImproveResumeProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return false
	}
	if progress.Iteration < 1 || progress.Iteration > maxInt(1, maxRounds) {
		return false
	}
	targets := make(map[string]bool, len(progress.TargetVolumes))
	for _, target := range progress.TargetVolumes {
		targets[target] = true
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if !targets[volume.ID] {
				return false
			}
		}
	}
	return len(progress.TargetVolumes) == outlineVolumeCount(outline)
}

func outlineVolumeCount(outline *models.Outline) int {
	count := 0
	if outline == nil {
		return count
	}
	for _, part := range outline.Parts {
		count += len(part.Volumes)
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func loadComposeImproveReportEntries(path string) ([]agents.ComposeImproveReportEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entriesByKey := make(map[string]agents.ComposeImproveReportEntry)
	order := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry agents.ComposeImproveReportEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			logger.Warn("Skipping malformed improve report line: %v", err)
			continue
		}
		key := fmt.Sprintf("%d:%s", entry.Iteration, entry.VolumeID)
		if _, seen := entriesByKey[key]; !seen {
			order = append(order, key)
		}
		entriesByKey[key] = entry
	}
	entries := make([]agents.ComposeImproveReportEntry, 0, len(order))
	for _, key := range order {
		entries = append(entries, entriesByKey[key])
	}
	return entries, nil
}

// writeComposeImproveReport aggregates the per-volume report entries recorded
// by the Agent SDK improvement run into a Markdown file under logs/. Pass a
// negative remainingGateIssues when no gate result is available.
func writeComposeImproveReport(remainingGateIssues int) error {
	entries, err := loadComposeImproveReportEntries(filepath.Join("story", "compose", "outline_improve_report.jsonl"))
	if err != nil {
		return err
	}
	if len(entries) == 0 && remainingGateIssues < 0 {
		return nil
	}
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return err
	}
	now := time.Now()
	reportPath := filepath.Join("logs", fmt.Sprintf("compose_improve_report_%s.md", now.Format("20060102_150405")))

	var builder strings.Builder
	builder.WriteString("# Compose Improve 报告\n\n")
	fmt.Fprintf(&builder, "- 生成时间: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&builder, "- 改进卷数: %d\n\n", len(entries))

	for index, entry := range entries {
		status := "已应用修复"
		if entry.Skipped {
			status = "跳过（预检无中高优问题）"
		} else if !entry.Changed {
			status = "评审通过（无有效改动）"
		}
		fmt.Fprintf(&builder, "## %d. %s（%s）\n\n", index+1, entry.VolumeTitle, entry.VolumeID)
		fmt.Fprintf(&builder, "- 状态: %s\n", status)
		if !entry.Skipped {
			fmt.Fprintf(&builder, "- 评分: %.1f\n", entry.Score)
			fmt.Fprintf(&builder, "- 剩余中高优问题: %d\n", entry.RemainingMediumPlus)
		}
		if summary := strings.TrimSpace(entry.Summary); summary != "" {
			fmt.Fprintf(&builder, "- 摘要: %s\n", summary)
		}
		builder.WriteString("\n")
	}
	if remainingGateIssues >= 0 {
		fmt.Fprintf(&builder, "## 门禁修复后\n\n- 剩余中高优问题: %d\n", remainingGateIssues)
	}

	if err := os.WriteFile(reportPath, []byte(builder.String()), 0o644); err != nil {
		return err
	}
	logger.Info("Compose improve report written: %s", reportPath)
	fmt.Printf("\n[report] 改进报告: %s\n", reportPath)
	return nil
}

func runComposeSkeletonReview(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE SKELETON REVIEW")

	projectConfig, setup, outline, err := loadComposeProjectState()
	if err != nil {
		return err
	}

	agent, err := newComposeAgentForProject(projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)

	review, err := agent.ReviewSkeleton(cmd.Context(), agents.ComposeSkeletonReviewInput{
		ExistingOutline: *outline,
		Setup:           *setup,
		Structure:       projectConfig.Structure,
		UserPrompt:      composePromptFlag,
	})
	if err != nil {
		return fmt.Errorf("skeleton review failed: %w", err)
	}

	reportPath := filepath.Join("story", "compose", "skeleton_review.json")
	if err := saveReviewResult(reportPath, review.Result); err != nil {
		return err
	}
	printReviewResult("Outline skeleton review", review.Result)
	fmt.Printf("[ok] Skeleton review saved to %s\n", reportPath)
	return nil
}

func runComposeReview(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE REVIEW")

	if composeReviewMatrixFlag {
		return runComposeReviewMatrix(cmd, args)
	}

	if composeCrossVolumeFlag {
		return runComposeCrossVolumeReview(cmd, args)
	}

	projectConfig, setup, outline, err := loadComposeProjectState()
	if err != nil {
		return err
	}
	agent, err := newComposeAgentForProject(projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(composeModelFlag) != "" {
		agent.SetModelOverride(composeModelFlag)
	}

	var review models.ReviewResult
	// --focus 提供内置审查视角，与 --prompt 自由文本可组合使用。
	userPrompt := composePromptFlag
	if focusPrompt := agents.ResolveReviewFocusPrompt(composeReviewFocusFlag); focusPrompt != "" {
		if userPrompt != "" {
			userPrompt = focusPrompt + "\n\n" + userPrompt
		} else {
			userPrompt = focusPrompt
		}
	}
	if composeAgentSDKFlag {
		input := agents.ComposeOutlineReviewInput{
			Outline:    *outline,
			Setup:      *setup,
			UserPrompt: userPrompt,
		}
		if composeImproveVolume > 0 {
			volumeID, resolveErr := resolveGlobalVolumeID(outline, composeImproveVolume)
			if resolveErr != nil {
				return resolveErr
			}
			input.VolumeID = volumeID
		}
		review, err = agent.ReviewOutlineWithAgentSDK(cmd.Context(), input)
		if err != nil {
			return fmt.Errorf("outline review failed: %w", err)
		}
	} else {
		result, reviewErr := agent.Review(cmd.Context(), agents.ComposeReviewInput{
			ExistingOutline: *outline,
			Setup:           *setup,
			UserPrompt:      userPrompt,
		})
		if reviewErr != nil {
			return fmt.Errorf("outline review failed: %w", reviewErr)
		}
		review = result.Result
	}

	reportPath := strings.TrimSpace(composeReviewOutFlag)
	if reportPath == "" {
		reportPath = filepath.Join("story", "compose", "outline_review.json")
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return fmt.Errorf("failed to create review report directory: %w", err)
	}
	if err := saveReviewResult(reportPath, review); err != nil {
		return err
	}
	printReviewResult("Outline review", review)
	fmt.Printf("[ok] Outline review saved to %s\n", reportPath)
	return nil
}

// runComposeReviewMatrix 运行多模型 × 多 focus 的 review 矩阵:
// 每个 (model, focus) 组合跑一轮只读 review, 合并所有建议, 按主题聚类,
// 分层抽样后输出 sampled JSON (可直接喂 --suggestions) 和合理化 prompt。
func runComposeReviewMatrix(cmd *cobra.Command, args []string) error {
	projectConfig, setup, outline, err := loadComposeProjectState()
	if err != nil {
		return err
	}

	// 解析模型列表
	var modelList []string
	if strings.TrimSpace(composeReviewModelsFlag) != "" {
		for _, m := range strings.Split(composeReviewModelsFlag, ",") {
			if m = strings.TrimSpace(m); m != "" {
				modelList = append(modelList, m)
			}
		}
	}
	if len(modelList) == 0 && strings.TrimSpace(projectConfig.LLM.Model) != "" {
		modelList = []string{strings.TrimSpace(projectConfig.LLM.Model)}
	}
	if len(modelList) == 0 {
		return fmt.Errorf("no models to run matrix with; pass --models or set project model")
	}

	// 解析 focus 列表
	var focusNames []string
	rawFocus := strings.TrimSpace(composeReviewFocusFlag)
	if rawFocus == "" || rawFocus == "all" {
		focusNames = agents.ListReviewFocusNames()
	} else {
		for _, f := range strings.Split(rawFocus, ",") {
			if f = strings.TrimSpace(f); f != "" {
				focusNames = append(focusNames, f)
			}
		}
	}
	if len(focusNames) == 0 {
		return fmt.Errorf("no focus selected for matrix")
	}

	totalRuns := len(modelList) * len(focusNames)
	logger.Info("Review matrix: %d models × %d focuses = %d runs", len(modelList), len(focusNames), totalRuns)
	fmt.Printf("=== Review matrix: %d models × %d focuses = %d runs ===\n", len(modelList), len(focusNames), totalRuns)

	// 并发跑 review
	parallel := composeReviewParallelFlag
	if parallel <= 0 {
		parallel = 4
	}
	if parallel > totalRuns {
		parallel = totalRuns
	}
	sem := make(chan struct{}, parallel)
	type runResult struct {
		model  string
		focus  string
		score  float64
		review models.ReviewResult
		err    error
	}
	results := make([]runResult, 0, totalRuns)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, model := range modelList {
		for _, focus := range focusNames {
			wg.Add(1)
			go func(model, focus string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				agent, aerr := newComposeAgentForProject(projectConfig)
				if aerr != nil {
					mu.Lock()
					results = append(results, runResult{model: model, focus: focus, err: aerr})
					mu.Unlock()
					return
				}
				agent.SetLanguage(projectConfig.Language)
				if model != "" {
					agent.SetModelOverride(model)
				}
				focusPrompt := agents.ResolveReviewFocusPrompt(focus)
				userPrompt := focusPrompt
				if strings.TrimSpace(composePromptFlag) != "" {
					if userPrompt != "" {
						userPrompt = userPrompt + "\n\n" + composePromptFlag
					} else {
						userPrompt = composePromptFlag
					}
				}
				input := agents.ComposeOutlineReviewInput{
					Outline:    *outline,
					Setup:      *setup,
					UserPrompt: userPrompt,
				}
				if composeImproveVolume > 0 {
					volumeID, resolveErr := resolveGlobalVolumeID(outline, composeImproveVolume)
					if resolveErr != nil {
						mu.Lock()
						results = append(results, runResult{model: model, focus: focus, err: resolveErr})
						mu.Unlock()
						return
					}
					input.VolumeID = volumeID
				}
				review, rerr := agent.ReviewOutlineWithAgentSDK(cmd.Context(), input)
				mu.Lock()
				results = append(results, runResult{model: model, focus: focus, score: review.OverallScore, review: review, err: rerr})
				mu.Unlock()
			}(model, focus)
		}
	}
	wg.Wait()

	// 汇总: 收集所有建议
	var allSugs []agents.ReviewMatrixSuggestion
	okCount := 0
	for _, r := range results {
		status := "ok"
		if r.err != nil {
			status = fmt.Sprintf("ERR: %v", r.err)
			logger.Error("Matrix run %s/%s failed: %v", r.model, r.focus, r.err)
		} else {
			okCount++
			for _, s := range r.review.Suggestions {
				allSugs = append(allSugs, agents.ReviewMatrixSuggestion{
					ReviewSuggestion: s,
					Src:              r.model + "/" + r.focus,
					ReviewCore:       int(r.score),
				})
			}
		}
		fmt.Printf("  [%s/%s] score=%v %s\n", r.model, r.focus, r.score, status)
	}
	fmt.Printf("成功 %d/%d 轮\n", okCount, totalRuns)
	if len(allSugs) == 0 {
		return fmt.Errorf("matrix collected no suggestions (all runs failed)")
	}

	agents.AnnotateMatrixSuggestions(allSugs)

	// 主题分布
	themeCount := map[string]int{}
	for _, s := range allSugs {
		if len(s.Themes) > 0 {
			themeCount[s.Themes[0]]++
		}
	}
	fmt.Printf("合并建议总数: %d\n", len(allSugs))
	fmt.Printf("主题分布: %v\n", themeCount)

	// 输出目录: story/reviews/matrix_<timestamp>/
	outDir := filepath.Join("story", "reviews", fmt.Sprintf("matrix_%s", time.Now().Format("20060102_150405")))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// 保存全部建议
	allPath := filepath.Join(outDir, "all_suggestions.json")
	if data, err := json.MarshalIndent(allSugs, "", "  "); err == nil {
		if err := os.WriteFile(allPath, data, 0644); err != nil {
			return err
		}
	}

	// 分层抽样
	sampleN := composeReviewSampleFlag
	if sampleN <= 0 {
		sampleN = 10
	}
	picked := agents.StratifiedSample(allSugs, sampleN, composeReviewSeedFlag)
	var sampled []agents.ReviewMatrixSuggestion
	for _, idx := range picked {
		sampled = append(sampled, allSugs[idx])
	}

	samplePath := filepath.Join(outDir, fmt.Sprintf("sampled_%d.json", len(sampled)))
	if data, err := json.MarshalIndent(sampled, "", "  "); err == nil {
		if err := os.WriteFile(samplePath, data, 0644); err != nil {
			return err
		}
	}

	// 合理化 prompt
	rp := agents.BuildRationalizePrompt(sampled)
	rpPath := filepath.Join(outDir, "rationalize_prompt.txt")
	if err := os.WriteFile(rpPath, []byte(rp), 0644); err != nil {
		return err
	}

	// 汇总报告
	reportPath := filepath.Join(outDir, "matrix_report.txt")
	var rb strings.Builder
	fmt.Fprintf(&rb, "review 矩阵: %d 模型 × %d focus = %d 轮\n", len(modelList), len(focusNames), totalRuns)
	fmt.Fprintf(&rb, "成功: %d/%d\n", okCount, totalRuns)
	fmt.Fprintf(&rb, "合并建议: %d\n", len(allSugs))
	fmt.Fprintf(&rb, "主题分布: %v\n", themeCount)
	fmt.Fprintf(&rb, "抽样(分层): %d 条, seed=%d\n", len(sampled), composeReviewSeedFlag)
	sampleThemes := map[string]int{}
	for _, s := range sampled {
		primary := "其他"
		if len(s.Themes) > 0 {
			primary = s.Themes[0]
		}
		sampleThemes[primary]++
	}
	fmt.Fprintf(&rb, "抽样主题覆盖: %v\n", sampleThemes)
	if err := os.WriteFile(reportPath, []byte(rb.String()), 0644); err != nil {
		return err
	}

	fmt.Printf("\n[ok] Matrix review complete. Outputs in %s\n", outDir)
	fmt.Printf("  - all suggestions:   %s\n", allPath)
	fmt.Printf("  - stratified sample: %s\n", samplePath)
	fmt.Printf("  - rationalize:       %s\n", rpPath)
	fmt.Printf("  - report:            %s\n", reportPath)
	fmt.Printf("\n下一步: novelgen compose improve --agent-sdk --suggestions %s\n", samplePath)
	return nil
}

// runComposeCrossVolumeReview 跨卷审查: 单 agent session 内审查卷范围的连续性。
func runComposeCrossVolumeReview(cmd *cobra.Command, args []string) error {
	projectConfig, setup, outline, err := loadComposeProjectState()
	if err != nil {
		return err
	}
	fromIdx := composeReviewFromVolumeFlag
	if fromIdx <= 0 {
		return fmt.Errorf("--from-volume is required for --cross-volume review")
	}
	toIdx := composeReviewToVolumeFlag
	if toIdx > 0 && toIdx < fromIdx {
		return fmt.Errorf("--to-volume %d must be >= --from-volume %d", toIdx, fromIdx)
	}

	agent, err := newComposeAgentForProject(projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(composeModelFlag) != "" {
		agent.SetModelOverride(composeModelFlag)
	}

	input := agents.ComposeOutlineReviewInput{
		Outline:         *outline,
		Setup:           *setup,
		UserPrompt:      composePromptFlag,
		CrossVolume:     true,
		FromVolumeIndex: fromIdx,
		ToVolumeIndex:   toIdx,
	}
	// --focus 拼进 prompt
	if focusPrompt := agents.ResolveReviewFocusPrompt(composeReviewFocusFlag); focusPrompt != "" {
		if input.UserPrompt != "" {
			input.UserPrompt = focusPrompt + "\n\n" + input.UserPrompt
		} else {
			input.UserPrompt = focusPrompt
		}
	}

	review, err := agent.ReviewOutlineWithAgentSDK(cmd.Context(), input)
	if err != nil {
		return fmt.Errorf("cross-volume review failed: %w", err)
	}

	reportPath := strings.TrimSpace(composeReviewOutFlag)
	if reportPath == "" {
		reportPath = filepath.Join("story", "reviews", "cross_volume_review.json")
	}
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return fmt.Errorf("failed to create review report directory: %w", err)
	}
	if err := saveReviewResult(reportPath, review); err != nil {
		return err
	}
	printReviewResult("Cross-volume review", review)
	fmt.Printf("[ok] Cross-volume review saved to %s\n", reportPath)
	return nil
}

func resolveGlobalVolumeID(outline *models.Outline, index int) (string, error) {
	if outline == nil || index <= 0 {
		return "", fmt.Errorf("invalid volume index %d", index)
	}
	seen := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			seen++
			if seen == index {
				vid := strings.TrimSpace(volume.ID)
				if vid == "" {
					return "", fmt.Errorf("volume %d has no ID", index)
				}
				return vid, nil
			}
		}
	}
	return "", fmt.Errorf("volume index %d out of range (total %d)", index, seen)
}

func runComposeSkeletonImprove(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE SKELETON IMPROVE")

	projectConfig, setup, outline, err := loadComposeProjectState()
	if err != nil {
		return err
	}
	if composeMaxRoundsFlag < 1 {
		return fmt.Errorf("--max-rounds must be at least 1")
	}

	agent, err := newComposeAgentForProject(projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)

	improved, review, err := agent.IterateSkeleton(cmd.Context(), outline, setup, projectConfig.Structure, composeMaxRoundsFlag, 80.0, composeForceImproveFlag, composePromptFlag)
	if err != nil {
		return fmt.Errorf("skeleton improvement failed: %w", err)
	}

	applyOutlineNormalization(improved, "skeleton_improve")
	if err := backupOutlineFiles(); err != nil {
		return err
	}

	outlinePath := filepath.Join("story", "compose", "outline.json")
	if err := improved.Save(outlinePath); err != nil {
		return fmt.Errorf("failed to save improved skeleton outline: %w", err)
	}
	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(improved, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	if review != nil {
		reportPath := filepath.Join("story", "compose", "skeleton_improve_review.json")
		if err := saveReviewResult(reportPath, *review); err != nil {
			return err
		}
		printReviewResult("Outline skeleton improve review", *review)
		fmt.Printf("[ok] Skeleton improve review saved to %s\n", reportPath)
	}

	fmt.Printf("[ok] Skeleton outline improved and saved to %s\n", outlinePath)
	fmt.Printf("[ok] Markdown updated at %s\n", mdPath)
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

	var modelOutline models.Outline
	if err := json.Unmarshal(data, &modelOutline); err != nil {
		return fmt.Errorf("failed to parse outline: %w", err)
	}

	checkOutline := &modelOutline
	partialCheck := countGeneratedVolumes(&modelOutline) > 0 && countEmptyVolumes(&modelOutline) > 0
	if partialCheck {
		checkOutline = outlineWithGeneratedVolumes(&modelOutline)
	}

	checkData, err := json.Marshal(checkOutline)
	if err != nil {
		return fmt.Errorf("failed to prepare outline for validation: %w", err)
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(checkData, &storyOutline); err != nil {
		return fmt.Errorf("failed to parse outline: %w", err)
	}

	if checkOutline != nil {
		setupPath := filepath.Join("story", "setup", "story_setup.json")
		var setupForGate *models.StorySetup
		if loadedSetup, loadErr := models.LoadStorySetup(setupPath); loadErr == nil {
			setupForGate = loadedSetup
		}
		logQualityGateResult("outline", runOutlineQualityGate(setupForGate, checkOutline))
		if strings.TrimSpace(composeCheckSuggestionsOut) != "" {
			gate := runOutlineCombinedGateForScope(setupForGate, checkOutline, false)
			report := qualityGateReviewResult("Deterministic outline check suggestions.", gate)
			if err := saveReviewResult(composeCheckSuggestionsOut, *report); err != nil {
				return err
			}
			fmt.Printf("[ok] Check suggestions report saved to %s\n", composeCheckSuggestionsOut)
		}
	}

	validator := rpg.NewOutlineValidator(&storyOutline)
	result := validator.Validate()

	if composeCheckJSONFlag {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("\n===== OUTLINE VALIDATION =====\n")
	if partialCheck {
		fmt.Printf("Partial outline: checking %d generated volume(s), skipping %d empty future volume(s).\n",
			countGeneratedVolumes(&modelOutline), countEmptyVolumes(&modelOutline))
	}
	if result.IsValid {
		fmt.Println("[ok] Outline passed validation")
	} else {
		fmt.Printf("[fail] Issues: %d | Warnings: %d | Suggestions: %d\n\n",
			result.IssueCount, result.WarningCount, len(result.Suggestions))
	}

	for _, issue := range result.Issues {
		icon := "[fail]"
		if issue.Severity == "critical" {
			icon = "[critical]"
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
			fmt.Printf("     -> %s\n", w.Suggestion)
		}
	}

	if len(result.Suggestions) > 0 {
		fmt.Printf("\n  --- Suggestions (%d) ---\n", len(result.Suggestions))
	}
	for _, s := range result.Suggestions {
		fmt.Printf("  💡 [%s] %s\n", s.Type, s.Location)
		fmt.Printf("     Current: %s\n", s.Current)
		fmt.Printf("     -> %s\n", s.Suggested)
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
						fmt.Printf("\n  [ok] Auto-patched setup: added %d missing resources\n", len(missingResources))
					}
				}

				fmt.Printf("\n  --- Cross-Module (Setup↔Outline) ---\n")
				for _, w := range crossWarnings {
					fmt.Printf("  ⚠ [cross] %s\n", w)
				}
				for _, i := range crossIssues {
					fmt.Printf("  [fail] [cross] %s\n", i)
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
		fmt.Println("[ok] Outline already normalized")
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

	fmt.Printf("[ok] Applied %d deterministic outline cleanup(s)\n", len(report.Changes))
	fmt.Printf("[ok] Updated %s and %s\n", outlinePath, mdPath)
	fmt.Printf("[ok] Report saved to %s\n", reportPath)
	return nil
}

func runComposePipeline(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE PIPELINE")

	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, false); err != nil {
		return err
	}
	if err := validateComposeAgentApplyOption(composeAgentSDKFlag, composeAgentApplyFlag); err != nil {
		return err
	}

	if _, err := os.Stat("novel.json"); err != nil {
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}

	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup at %s: %w", setupPath, err)
	}

	totalVolumes := projectConfig.Structure.TargetParts * projectConfig.Structure.TargetVolumes
	if totalVolumes <= 0 {
		return fmt.Errorf("project structure must define positive part and volume counts")
	}
	fromVolume := composePipelineFromVol
	toVolume := composePipelineToVol
	if toVolume == 0 {
		toVolume = totalVolumes
	}
	if fromVolume < 1 || toVolume < 1 || fromVolume > toVolume || toVolume > totalVolumes {
		return fmt.Errorf("invalid volume range %d..%d; valid range is 1..%d", fromVolume, toVolume, totalVolumes)
	}
	if composePipelineMaxRounds < 1 && !composePipelineSkipImprove {
		return fmt.Errorf("--max-rounds must be at least 1 unless --skip-improve is set")
	}

	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	ctx := context.Background()
	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := ensureComposePipelineSkeleton(ctx, agent, setup, projectConfig, outlinePath, composeAgentSDKFlag)
	if err != nil {
		return err
	}
	if err := saveComposePipelineOutline(outline, outlinePath); err != nil {
		return err
	}
	fmt.Printf("Pipeline range: volumes %d..%d (%d total)\n", fromVolume, toVolume, totalVolumes)

	for globalVolume := fromVolume; globalVolume <= toVolume; globalVolume++ {
		partIdx, volIdx, err := outlineVolumePosition(outline, globalVolume)
		if err != nil {
			return err
		}
		volume := &outline.Parts[partIdx].Volumes[volIdx]
		fmt.Printf("\n=== Pipeline volume %d: %s ===\n", globalVolume, volume.Title)

		if !composePipelineSkipGen {
			if len(volume.Chapters) > 0 && !composePipelineForce {
				fmt.Printf("Generation skipped: volume already has %d chapter(s)\n", len(volume.Chapters))
			} else {
				if composeAgentSDKFlag {
					err = generateComposePipelineVolumeAgentSDK(ctx, agent, setup, projectConfig, outline, partIdx, volIdx, globalVolume, totalVolumes)
				} else {
					err = generateComposePipelineVolume(ctx, agent, setup, projectConfig, outline, partIdx, volIdx, globalVolume, totalVolumes)
				}
				if err != nil {
					return err
				}
				if err := saveComposePipelineOutline(outline, outlinePath); err != nil {
					return err
				}
				fmt.Printf("Saved after generation: %s\n", outlinePath)
			}
		}

		if !composePipelineSkipImprove {
			if len(volume.Chapters) == 0 {
				return fmt.Errorf("volume %d has no chapters to improve; generate it first or remove --skip-gen", globalVolume)
			}
			improveOutline, err := outlineWithImproveVolumeSelection(outline, globalVolume, 0, 0)
			if err != nil {
				return err
			}
			if composeAgentSDKFlag {
				err = iterateOutlineImprovementAgentSDK(improveOutline, setup, projectConfig, composePipelineMaxRounds, composePipelineForce, composePromptFlag, composeAgentApplyFlag, true, nil, "", outline)
			} else {
				err = iterateOutlineImprovement(improveOutline, setup, projectConfig, composePipelineMaxRounds, 1, true, composePipelineForce, composePromptFlag)
			}
			if err != nil {
				return fmt.Errorf("failed to improve volume %d: %w", globalVolume, err)
			}
			mergeGeneratedVolumes(outline, improveOutline)
			if err := saveComposePipelineOutline(outline, outlinePath); err != nil {
				return err
			}
			fmt.Printf("Saved after improve: %s\n", outlinePath)
		}

		if !composePipelineSkipCross {
			patched, issues, warnings, err := applySetupOutlineCrossPatch(setupPath, setup, outline)
			if err != nil {
				return err
			}
			if patched > 0 {
				fmt.Printf("Cross setup patch: added %d missing resource(s)\n", patched)
			}
			if len(issues)+len(warnings) > 0 {
				fmt.Printf("Cross check: %d issue(s), %d warning(s)\n", len(issues), len(warnings))
			}
		}
	}

	logQualityGateResult("outline", runOutlineQualityGate(setup, outline))
	fmt.Printf("\nPipeline complete. Current outline: %s\n", outlinePath)
	fmt.Printf("Generated volumes: %d, empty future volumes: %d\n", countGeneratedVolumes(outline), countEmptyVolumes(outline))
	return nil
}

func ensureComposePipelineSkeleton(ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outlinePath string, useAgentSDK bool) (*models.Outline, error) {
	if _, err := os.Stat(outlinePath); err == nil {
		outline, err := models.LoadOutline(outlinePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing outline: %w", err)
		}
		applyOutlineNormalization(outline, "pipeline_skeleton")
		return outline, nil
	}

	legacyProgressPath := filepath.Join("story", "compose", "outline_progress.json")
	if _, err := os.Stat(legacyProgressPath); err == nil {
		outline, err := loadPartialOutline(legacyProgressPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load legacy outline progress: %w", err)
		}
		applyOutlineNormalization(outline, "pipeline_skeleton")
		return outline, nil
	}

	fmt.Println("No outline.json found; generating compose skeleton")
	skeletonInput := agents.ComposeSkeletonInput{
		Setup:     *setup,
		Structure: projectConfig.Structure,
	}
	var skeletonOutput agents.ComposeSkeletonOutput
	var err error
	if useAgentSDK {
		skeletonOutput, err = agent.GenerateSkeletonWithAgentSDK(ctx, skeletonInput)
	} else {
		skeletonOutput, err = agent.GenerateSkeleton(ctx, skeletonInput)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate skeleton: %w", err)
	}
	outline := &models.Outline{Parts: skeletonOutput.Parts}
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(outline, "pipeline_skeleton")
	return outline, nil
}

func generateComposePipelineVolume(ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outline *models.Outline, partIdx, volIdx, globalVolume, totalVolumes int) error {
	if outline == nil || setup == nil || projectConfig == nil {
		return fmt.Errorf("pipeline generation requires setup, project config, and outline")
	}
	volume := &outline.Parts[partIdx].Volumes[volIdx]
	var outlineContext string
	if partIdx > 0 || volIdx > 0 {
		outlineContext = agent.BuildHierarchicalContext(outline, partIdx, volIdx)
	}

	var previousVolume *models.Volume
	if volIdx > 0 {
		previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
	} else if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		if len(prevPart.Volumes) > 0 {
			previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
		}
	}

	output, err := agent.GenerateChaptersForVolume(ctx, agents.ComposeChaptersInput{
		Setup:          *setup,
		Part:           outline.Parts[partIdx],
		Volume:         *volume,
		VolumeIndex:    globalVolume,
		TotalVolumes:   totalVolumes,
		ChaptersPerVol: projectConfig.Structure.TargetChapters,
		PreviousVolume: previousVolume,
		OutlineContext: outlineContext,
	})
	if err != nil {
		return fmt.Errorf("failed to generate chapters for volume %d: %w", globalVolume, err)
	}

	volume.Chapters = output.Chapters
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(outline, fmt.Sprintf("pipeline_gen_v%02d", globalVolume))
	return nil
}

func generateComposePipelineVolumeAgentSDK(ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outline *models.Outline, partIdx, volIdx, globalVolume, totalVolumes int) error {
	if outline == nil || setup == nil || projectConfig == nil {
		return fmt.Errorf("pipeline generation requires setup, project config, and outline")
	}
	volume := &outline.Parts[partIdx].Volumes[volIdx]
	var outlineContext string
	if partIdx > 0 || volIdx > 0 {
		outlineContext = agent.BuildHierarchicalContext(outline, partIdx, volIdx)
	}

	var previousVolume *models.Volume
	if volIdx > 0 {
		previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
	} else if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		if len(prevPart.Volumes) > 0 {
			previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
		}
	}

	output, err := agent.GenerateChaptersForVolumeWithAgentSDK(ctx, agents.ComposeChaptersInput{
		Setup:          *setup,
		Part:           outline.Parts[partIdx],
		Volume:         *volume,
		VolumeIndex:    globalVolume,
		TotalVolumes:   totalVolumes,
		ChaptersPerVol: projectConfig.Structure.TargetChapters,
		PreviousVolume: previousVolume,
		OutlineContext: outlineContext,
	})
	if err != nil {
		return fmt.Errorf("failed to generate chapters for volume %d: %w", globalVolume, err)
	}

	outline.Parts[partIdx].Volumes[volIdx] = output.Volume
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(outline, fmt.Sprintf("pipeline_agent_sdk_gen_v%02d", globalVolume))
	return nil
}

func saveComposePipelineOutline(outline *models.Outline, outlinePath string) error {
	if err := savePartialOutline(outline, outlinePath); err != nil {
		return err
	}
	mdPath := filepath.Join("story", "compose", "outline.md")
	if err := createOutlineMarkdown(outline, mdPath); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}
	return nil
}

func outlineVolumePosition(outline *models.Outline, globalVolume int) (int, int, error) {
	if outline == nil {
		return 0, 0, fmt.Errorf("outline is nil")
	}
	if globalVolume < 1 {
		return 0, 0, fmt.Errorf("volume index must be positive")
	}
	current := 0
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			current++
			if current == globalVolume {
				return partIdx, volIdx, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("volume %d not found in outline (%d volume(s) available)", globalVolume, current)
}

func applySetupOutlineCrossPatch(setupPath string, setup *models.StorySetup, outline *models.Outline) (int, []string, []string, error) {
	if setup == nil || outline == nil {
		return 0, nil, nil, nil
	}
	checkOutline := outlineWithGeneratedVolumes(outline)
	if countGeneratedVolumes(checkOutline) == 0 {
		checkOutline = outline
	}
	checkData, err := json.Marshal(checkOutline)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to prepare outline for cross check: %w", err)
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(checkData, &storyOutline); err != nil {
		return 0, nil, nil, fmt.Errorf("failed to parse outline for cross check: %w", err)
	}

	issues, warnings := validateSetupOutlineCross(setup, &storyOutline)
	missingResources := missingSetupResources(setup, &storyOutline)
	if len(missingResources) == 0 {
		return 0, issues, warnings, nil
	}
	for _, name := range missingResources {
		setup.WorldResources = append(setup.WorldResources, models.WorldResource{
			Name:        name,
			Category:    "general",
			Scarcity:    "rare",
			Description: "Auto-added by compose pipeline cross check; refine manually.",
		})
	}
	if err := setup.Save(setupPath); err != nil {
		return 0, issues, warnings, fmt.Errorf("failed to save cross-patched setup: %w", err)
	}
	return len(missingResources), issues, warnings, nil
}

func missingSetupResources(setup *models.StorySetup, outline *rpg.StoryOutline) []string {
	if setup == nil || outline == nil {
		return nil
	}
	defined := map[string]bool{}
	for _, resource := range setup.WorldResources {
		name := strings.TrimSpace(resource.Name)
		if name != "" {
			defined[name] = true
		}
	}
	missing := map[string]bool{}
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, entry := range ch.ResourceLedger {
					name := strings.TrimSpace(entry.Item)
					if name != "" && !defined[name] {
						missing[name] = true
					}
				}
			}
		}
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateSetupOutlineCross(setup *models.StorySetup, outline *rpg.StoryOutline) (issues, warnings []string) {
	// Collect defined faction tiers from setup premises
	setupFactions := make(map[string]map[string]bool) // faction -> tier -> true
	for _, p := range setup.Premises {
		for _, faction := range setupPremiseFactionAliases(p) {
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

func repairOutlineWithQualityGate(ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, stage string, scoped bool) (*models.Outline, qualityGateResult, error) {
	applyOutlineNormalization(outline, "pre_gate_"+stage)
	gate := runOutlineQualityGateForScope(setup, outline, scoped)
	if !gate.Blocking {
		return outline, gate, nil
	}
	if outline == nil {
		return nil, gate, fmt.Errorf("outline quality gate failed: outline is nil")
	}

	logger.Info("Outline quality gate found blocking issues after %s; running one bounded repair pass", stage)
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return outline, gate, fmt.Errorf("failed to load LLM config for outline repair: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return outline, gate, fmt.Errorf("failed to create LLM client for outline repair")
	}

	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	review := qualityGateReviewResult("Repair outline quality gate findings before saving project state.", gate)
	repaired, err := agent.RepairByReview(ctx, outline, review, setup)
	if err != nil {
		return outline, gate, fmt.Errorf("outline quality gate repair failed: %w", err)
	}
	applyOutlineNormalization(repaired, "post_gate_"+stage)
	finalGate := runOutlineQualityGateForScope(setup, repaired, scoped)
	return repaired, finalGate, nil
}

func repairOutlineWithQualityGateAgentSDK(ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, stage string, agentApply bool, scoped bool, userPrompt string, crossOutline *models.Outline) (*models.Outline, qualityGateResult, error) {
	applyOutlineNormalization(outline, "pre_gate_"+stage)
	gate := runOutlineQualityGateForScope(setup, outline, scoped)
	repairGate := filterQualityGateForAgentSDKPromptBoundary(gate, outline, userPrompt, "quality-gate repair")
	if !gate.Blocking {
		if agentApply && outlineGateHasPatchableGlobalIssues(repairGate, outline) {
			repairedOutline, repairedGate, err := repairOutlineGlobalIssuesAgentSDK(ctx, outline, setup, projectConfig, repairGate, stage)
			if err != nil {
				return outline, gate, err
			}
			return repairedOutline, repairedGate, nil
		}
		return outline, gate, nil
	}
	if outline == nil {
		return nil, gate, fmt.Errorf("outline quality gate failed: outline is nil")
	}

	logger.Info("Outline quality gate found blocking issues after %s; running one Agent SDK repair pass", stage)
	if !repairGate.Blocking {
		logger.Info("Skipping Agent SDK blocking repair after %s: user prompt boundary excludes blocking issue(s)", stage)
		return outline, gate, nil
	}
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return outline, gate, fmt.Errorf("failed to load LLM config for outline repair: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return outline, gate, fmt.Errorf("failed to create LLM client for outline repair")
	}

	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	review := qualityGateReviewResult("Repair outline quality gate findings before saving project state.", repairGate)
	repaired, err := agent.RepairByReviewAgentSDK(ctx, outline, review, setup, agentApply, crossOutline, composeCrossVolumeAllFlag)
	if err != nil {
		return outline, gate, fmt.Errorf("outline quality gate Agent SDK repair failed: %w", err)
	}
	applyOutlineNormalization(repaired, "post_gate_"+stage)
	finalGate := runOutlineQualityGateForScope(setup, repaired, scoped)
	return repaired, finalGate, nil
}

func runOutlineQualityGateForScope(setup *models.StorySetup, outline *models.Outline, scoped bool) qualityGateResult {
	if scoped {
		return runScopedOutlineQualityGate(setup, outline)
	}
	return runOutlineQualityGate(setup, outline)
}

func outlineGateHasPatchableGlobalIssues(gate qualityGateResult, outline *models.Outline) bool {
	mysteryThreads := collectOutlineMysteryThreads(outline)
	for _, suggestion := range gate.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID != "" && !strings.EqualFold(targetID, "global") && normalizeKey(suggestion.Category) != "faction_tier" {
			continue
		}
		nav := toolIssueNavigation("all", "outline", "all", "", suggestion, 0)
		if stringMapValue(nav, "patch_query") != "" && nav["patch_shape"] != nil {
			return true
		}
		if normalizeKey(suggestion.Category) == "mysteries" && firstPatchableOutlineMysteryThread(mysteryThreads) != nil {
			return true
		}
	}
	return false
}

func repairOutlineGlobalIssuesAgentSDK(ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, gate qualityGateResult, stage string) (*models.Outline, qualityGateResult, error) {
	logger.Info("Outline quality gate found patchable global issues after %s; running one Agent SDK global repair pass", stage)
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return outline, gate, fmt.Errorf("failed to load LLM config for global outline repair: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return outline, gate, fmt.Errorf("failed to create LLM client for global outline repair")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	review := qualityGateReviewResult("Repair patchable global outline quality findings before saving project state.", gate)
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(ctx, *review, true); err != nil {
		return outline, gate, fmt.Errorf("global outline quality gate Agent SDK repair failed: %w", err)
	}
	reloadedSetup := setup
	if loaded, loadErr := models.LoadStorySetup(filepath.Join("story", "setup", "story_setup.json")); loadErr == nil {
		reloadedSetup = loaded
	} else {
		logger.Warn("Failed to reload story setup after global repair: %v", loadErr)
	}
	reloadedOutline := outline
	if loaded, loadErr := models.LoadOutline(filepath.Join("story", "compose", "outline.json")); loadErr == nil {
		reloadedOutline = loaded
	} else {
		logger.Warn("Failed to reload outline after global repair; using in-memory outline: %v", loadErr)
	}
	return reloadedOutline, runOutlineQualityGate(reloadedSetup, reloadedOutline), nil
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

	outlinePath := filepath.Join("story", "compose", "outline.json")
	legacyProgressPath := filepath.Join("story", "compose", "outline_progress.json")

	var outline *models.Outline
	var resumeMode bool

	if _, err := os.Stat(outlinePath); err == nil && !composeForceGenFlag {
		fmt.Println("📂 Found existing outline.json. Resuming empty-volume chapter generation...")
		outline, err = loadPartialOutline(outlinePath)
		if err != nil {
			fmt.Printf("⚠️  Failed to load outline: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			fmt.Println("[ok] Resumed from outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	} else if _, err := os.Stat(legacyProgressPath); err == nil && !composeForceGenFlag {
		fmt.Println("📂 Found legacy outline_progress.json. Migrating to outline.json and resuming...")
		outline, err = loadPartialOutline(legacyProgressPath)
		if err != nil {
			fmt.Printf("⚠️  Failed to load legacy progress: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			if err := savePartialOutline(outline, outlinePath); err != nil {
				return nil, fmt.Errorf("failed to migrate legacy progress to outline.json: %w", err)
			}
			fmt.Println("[ok] Migrated legacy progress to outline.json")
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
		if err := savePartialOutline(o, outlinePath); err != nil {
			logger.GetLogger().Warn("Failed to save outline progress: %v", err)
		} else {
			fmt.Printf("💾 outline.json saved (%d volumes completed)\n", volumeCount)
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
		logic.NewIDManager(outline).AssignIDsToOutline()
		applyOutlineNormalization(outline, "fresh_skeleton")

		// Save skeleton as initial outline with empty chapter arrays.
		if err := savePartialOutline(outline, outlinePath); err != nil {
			logger.GetLogger().Warn("Failed to save initial outline: %v", err)
		}
		fmt.Println("💾 Skeleton saved to outline.json")

		// Generate chapters with incremental outline.json saves.
		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save outline even on error
			if saveErr := savePartialOutline(outline, outlinePath); saveErr != nil {
				logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
			}
			return nil, err
		}

		os.Remove(legacyProgressPath)
		fmt.Println("\n[ok] Generation complete! outline.json is the canonical state.")
	} else {
		// Resume generation - continue from where we left off
		fmt.Println("Continuing chapter generation...")
		fmt.Println()

		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save outline even on error
			if saveErr := savePartialOutline(outline, outlinePath); saveErr != nil {
				logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
			}
			return nil, err
		}

		os.Remove(legacyProgressPath)
		fmt.Println("\n[ok] Generation complete! outline.json is the canonical state.")
	}

	return outline, nil
}

func generateOutlineHierarchicalAgentSDK(setup *models.StorySetup, projectConfig *models.ProjectConfig) (*models.Outline, error) {
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}
	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Story structure: %d parts x %d volumes x %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println("Mode: Claude Agent SDK with read-only novelgen tool queries")
	fmt.Println()

	outlinePath := filepath.Join("story", "compose", "outline.json")
	legacyProgressPath := filepath.Join("story", "compose", "outline_progress.json")

	var outline *models.Outline
	var resumeMode bool
	if _, err := os.Stat(outlinePath); err == nil && !composeForceGenFlag {
		fmt.Println("[resume] Found existing outline.json. Resuming empty-volume Agent SDK chapter generation...")
		outline, err = loadPartialOutline(outlinePath)
		if err != nil {
			fmt.Printf("[warn] Failed to load outline: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			fmt.Println("[ok] Resumed from outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	} else if _, err := os.Stat(legacyProgressPath); err == nil && !composeForceGenFlag {
		fmt.Println("[resume] Found legacy outline_progress.json. Migrating to outline.json and resuming...")
		outline, err = loadPartialOutline(legacyProgressPath)
		if err != nil {
			fmt.Printf("[warn] Failed to load legacy progress: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			if err := savePartialOutline(outline, outlinePath); err != nil {
				return nil, fmt.Errorf("failed to migrate legacy progress to outline.json: %w", err)
			}
			fmt.Println("[ok] Migrated legacy progress to outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	ctx := context.Background()

	onVolumeComplete := func(o *models.Outline, partIdx, volIdx, volumeCount int) {
		if err := savePartialOutline(o, outlinePath); err != nil {
			logger.GetLogger().Warn("Failed to save outline progress: %v", err)
		} else {
			fmt.Printf("[save] outline.json saved (%d volumes completed)\n", volumeCount)
		}
		if err := createOutlineMarkdown(o, filepath.Join("story", "compose", "outline.md")); err != nil {
			logger.GetLogger().Warn("Failed to save outline markdown progress: %v", err)
		}
	}

	if !resumeMode {
		fmt.Println("Using Agent SDK hierarchical generation:")
		fmt.Println("  Phase 1: Generate skeleton (parts and volumes)")
		fmt.Printf("  Phase 2: Generate chapters for each of %d volumes\n",
			projectConfig.Structure.TargetParts*projectConfig.Structure.TargetVolumes)
		fmt.Println()

		skeletonOutput, err := agent.GenerateSkeletonWithAgentSDK(ctx, agents.ComposeSkeletonInput{
			Setup:     *setup,
			Structure: projectConfig.Structure,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate skeleton: %w", err)
		}
		outline = &models.Outline{Parts: skeletonOutput.Parts}
		logic.NewIDManager(outline).AssignIDsToOutline()
		applyOutlineNormalization(outline, "agent_sdk_fresh_skeleton")
		if err := savePartialOutline(outline, outlinePath); err != nil {
			logger.GetLogger().Warn("Failed to save initial outline: %v", err)
		}
		fmt.Println("[save] Skeleton saved to outline.json")
	} else {
		fmt.Println("Continuing Agent SDK chapter generation...")
		fmt.Println()
	}

	outline, err = agent.GenerateChaptersHierarchicalAgentSDK(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
	if err != nil {
		if saveErr := savePartialOutline(outline, outlinePath); saveErr != nil {
			logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
		}
		return nil, err
	}

	os.Remove(legacyProgressPath)
	fmt.Println("\n[ok] Agent SDK generation complete. outline.json is the canonical state.")
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

func canResumeOutlineGeneration(path string, structure models.StoryStructure) bool {
	outline, err := loadPartialOutline(path)
	if err != nil {
		logger.GetLogger().Warn("Failed to inspect existing outline for resume: %v", err)
		return false
	}
	if len(outline.Parts) == 0 {
		return false
	}
	expectedVolumes := structure.TargetParts * structure.TargetVolumes
	actualVolumes := 0
	for _, part := range outline.Parts {
		actualVolumes += len(part.Volumes)
	}
	if expectedVolumes > 0 && actualVolumes != expectedVolumes {
		return false
	}
	return countEmptyVolumes(outline) > 0
}

func countGeneratedVolumes(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	count := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if len(volume.Chapters) > 0 {
				count++
			}
		}
	}
	return count
}

func countEmptyVolumes(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	count := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if len(volume.Chapters) == 0 {
				count++
			}
		}
	}
	return count
}

func outlineWithGeneratedVolumes(outline *models.Outline) *models.Outline {
	if outline == nil {
		return nil
	}
	filtered := &models.Outline{}
	for _, part := range outline.Parts {
		nextPart := part
		nextPart.Volumes = nil
		for _, volume := range part.Volumes {
			if len(volume.Chapters) > 0 {
				nextPart.Volumes = append(nextPart.Volumes, volume)
			}
		}
		if len(nextPart.Volumes) > 0 {
			filtered.Parts = append(filtered.Parts, nextPart)
		}
	}
	return filtered
}

func outlineWithImproveVolumeSelection(outline *models.Outline, volume, fromVolume, toVolume int) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if volume > 0 && (fromVolume > 0 || toVolume > 0) {
		return nil, fmt.Errorf("--volume cannot be combined with --from-volume or --to-volume")
	}
	if volume < 0 || fromVolume < 0 || toVolume < 0 {
		return nil, fmt.Errorf("volume indexes must be positive")
	}

	start, end := fromVolume, toVolume
	if volume > 0 {
		start, end = volume, volume
	}
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = start
	}
	if start > end {
		return nil, fmt.Errorf("--from-volume must be less than or equal to --to-volume")
	}

	filtered := &models.Outline{}
	globalVolume := 0
	selected := 0
	for _, part := range outline.Parts {
		nextPart := part
		nextPart.Volumes = nil
		for _, vol := range part.Volumes {
			globalVolume++
			if globalVolume < start || globalVolume > end || len(vol.Chapters) == 0 {
				continue
			}
			nextPart.Volumes = append(nextPart.Volumes, vol)
			selected++
		}
		if len(nextPart.Volumes) > 0 {
			filtered.Parts = append(filtered.Parts, nextPart)
		}
	}
	if selected == 0 {
		return nil, fmt.Errorf("selected range has no generated volumes to improve")
	}
	return filtered, nil
}

func mergeGeneratedVolumes(target *models.Outline, improved *models.Outline) {
	if target == nil || improved == nil {
		return
	}
	byID := map[string]models.Volume{}
	for _, part := range improved.Parts {
		for _, volume := range part.Volumes {
			if strings.TrimSpace(volume.ID) != "" {
				byID[volume.ID] = volume
			}
		}
	}
	for partIdx := range target.Parts {
		for volIdx := range target.Parts[partIdx].Volumes {
			volume := &target.Parts[partIdx].Volumes[volIdx]
			if improvedVolume, ok := byID[volume.ID]; ok {
				preserveImprovedVolumeIdentityCmd(volume, &improvedVolume)
				target.Parts[partIdx].Volumes[volIdx] = improvedVolume
			}
		}
	}
}

func preserveImprovedVolumeIdentityCmd(original *models.Volume, improved *models.Volume) {
	if original == nil || improved == nil {
		return
	}
	improved.ID = original.ID
	for i := range improved.Chapters {
		if i >= len(original.Chapters) {
			break
		}
		improved.Chapters[i].ID = original.Chapters[i].ID
	}
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

func setupPremiseFactionAliases(p models.Premise) []string {
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
	for _, alias := range setupFactionAliases(p.Category) {
		add(alias)
	}
	text := strings.ToLower(p.Name + " " + p.Description + " " + p.Category)
	for _, known := range []string{"zerg", "shen"} {
		if strings.Contains(text, known) {
			add(known)
		}
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
			existingID := chapter.ID
			*chapter = *output.Chapter
			chapter.ID = existingID
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

func loadComposeProjectState() (*models.ProjectConfig, *models.StorySetup, *models.Outline, error) {
	if _, err := os.Stat("novel.json"); err != nil {
		return nil, nil, nil, fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	setupPath := filepath.Join("story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load story setup at %s: %w", setupPath, err)
	}

	outlinePath := filepath.Join("story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load outline at %s: %w", outlinePath, err)
	}
	return projectConfig, setup, outline, nil
}

func newComposeAgentForProject(projectConfig *models.ProjectConfig) (*agents.ComposeAgent, error) {
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	return agents.NewComposeAgent(client, cfg, &projectConfig.LLM), nil
}

func saveReviewResult(path string, result models.ReviewResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review result: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write review result to %s: %w", path, err)
	}
	return nil
}

func printReviewResult(label string, result models.ReviewResult) {
	fmt.Printf("\n%s: %.1f/100\n", label, result.OverallScore)
	if strings.TrimSpace(result.Summary) != "" {
		fmt.Printf("Summary: %s\n", result.Summary)
	}
	if len(result.Suggestions) > 0 {
		fmt.Println("Top suggestions:")
		limit := len(result.Suggestions)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			s := result.Suggestions[i]
			fmt.Printf("- [%s] %s: %s\n", s.Priority, s.TargetName, s.Suggestion)
		}
	}
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

// runOutlineValidatorOnModel converts models.Outline to rpg.StoryOutline,
// runs the outline validator, and returns issues as ReviewSuggestions.
func runOutlineValidatorOnModel(outline *models.Outline) []models.ReviewSuggestion {
	if outline == nil {
		return nil
	}
	// Convert via JSON roundtrip (models.Outline -> rpg.StoryOutline)
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
		if s.Type == "faction_tier" {
			// The outline-only validator cannot see story_setup.json, so its
			// "define faction tiers in setup" hint is not self-verifying. The
			// setup+outline cross-check owns that contract.
			continue
		}
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
			logger.Info("Quality gate added %d suggestion(s) to review", len(gate.Suggestions))
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

func iterateOutlineImprovementAgentSDK(outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, maxIterations int, forceImprove bool, userPrompt string, agentApply bool, scoped bool, suggestions []models.ReviewSuggestion, modelOverride string, crossOutline *models.Outline) error {
	logger.Section("Outline Agent SDK Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)
	logger.Info("Mode: Agent SDK hierarchical per-volume review/improve")
	if forceImprove {
		logger.Info("Force improve enabled: will apply SDK volume output even if score meets threshold")
	}
	if agentApply {
		logger.Info("Agent apply enabled: SDK may write outline patches through validated tool apply")
	}

	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}
	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(modelOverride) != "" {
		agent.SetModelOverride(modelOverride)
	}

	ctx := context.Background()
	beforeOutline := cloneOutline(outline)
	applyOutlineNormalization(outline, "pre_agent_sdk_improve")

	preGate := runOutlineCombinedGateForScope(setup, outline, scoped)
	var seedReview *models.ReviewResult
	if len(preGate.Suggestions) > 0 {
		seedReview = qualityGateReviewResult("Pre-check issues for targeted Agent SDK outline improvement.", preGate)
		seedReview = filterAgentSDKReviewForPromptBoundary(seedReview, outline, userPrompt, "pre-check seed review")
		logger.Info("Pre-check supplied %d quality/simulation issue(s) to Agent SDK", len(seedReview.Suggestions))
	}
	if len(suggestions) > 0 {
		merged := qualityGateResult{Suggestions: append([]models.ReviewSuggestion(nil), suggestions...)}
		if seedReview != nil {
			merged.Suggestions = append(merged.Suggestions, seedReview.Suggestions...)
		}
		merged.dedup()
		seedReview = &models.ReviewResult{
			OverallScore: scoreFromGate(merged),
			Summary:      fmt.Sprintf("Agent SDK outline improvement seeded by %d suggestion report item(s).", len(merged.Suggestions)),
			Suggestions:  merged.Suggestions,
		}
		seedReview = filterAgentSDKReviewForPromptBoundary(seedReview, outline, userPrompt, "suggestion report seed")
		logger.Info("Suggestion report(s) provided %d issue(s) to Agent SDK", len(merged.Suggestions))
	}

	improvedOutline, review, err := agent.IterateHierarchicalAgentSDK(ctx, outline, crossOutline, seedReview, maxIterations, 80.0, forceImprove, userPrompt, setup, agentApply, composeCrossVolumeAllFlag)
	if err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	applyOutlineNormalization(improvedOutline, "post_agent_sdk_improve")
	*outline = *improvedOutline

	if review != nil {
		gate := runOutlineCombinedGateForScope(setup, improvedOutline, scoped)
		if len(gate.Suggestions) > 0 {
			review.Suggestions = append(review.Suggestions, gate.Suggestions...)
			logger.Info("Quality/simulation gate added %d suggestion(s) to Agent SDK review", len(gate.Suggestions))
		}
		repairReview := qualityGateMediumReviewResult("Post-check medium-or-higher issues for Agent SDK targeted repair.", gate)
		repairReview = filterAgentSDKReviewForPromptBoundary(repairReview, improvedOutline, userPrompt, "post-check repair pass")
		if len(repairReview.Suggestions) > 0 && maxIterations > 0 {
			repairBatch := limitReviewSuggestionsForAgentSDKRepair(repairReview, composeRepairBudgetFlag)
			logger.Info("Quality/simulation gate found %d targetable medium-or-higher issue(s), running Agent SDK repair pass on %d issue(s)", len(repairReview.Suggestions), len(repairBatch.Suggestions))
			repairOutput, repairErr := repairAgentSDKWithTransientRetry(func() (*models.Outline, error) {
				return agent.RepairByReviewAgentSDK(ctx, improvedOutline, repairBatch, setup, agentApply, crossOutline, composeCrossVolumeAllFlag)
			})
			if repairErr == nil {
				improvedOutline = repairOutput
				applyOutlineNormalization(improvedOutline, "post_agent_sdk_repair")
				*outline = *improvedOutline
				repairGate := runOutlineCombinedGateForScope(setup, improvedOutline, scoped)
				remaining := filterMediumOrHigherOutlineTargetSuggestions(repairGate.Suggestions)
				if len(remaining) > 0 {
					logger.Warn("Agent SDK repair completed, but %d targetable medium-or-higher quality/simulation issue(s) remain", len(remaining))
				} else {
					logger.Info("Agent SDK partial compose repair pass completed; no targetable medium-or-higher quality/simulation issues remain")
				}
			} else {
				logger.Warn("Agent SDK partial compose repair pass failed: %v", repairErr)
			}
		}
	}

	if outlinesSemanticallyEqual(beforeOutline, outline) {
		logger.Info("Agent SDK iteration made no effective outline changes; skipping intermediate outline_iter_%d.json", maxIterations)
	} else {
		outlinePath := filepath.Join("story", "compose", fmt.Sprintf("outline_iter_%d.json", maxIterations))
		if err := savePartialOutline(outline, outlinePath); err != nil {
			logger.Error("Failed to save intermediate outline: %v", err)
		} else {
			logger.Info("Saved intermediate outline to %s", outlinePath)
		}
	}

	logger.Section("Agent SDK Iteration Complete")
	if review != nil {
		logger.Info("Final Agent SDK Review Score: %.1f/100", review.OverallScore)
	}
	return nil
}

func filterAgentSDKReviewForPromptBoundary(review *models.ReviewResult, outline *models.Outline, userPrompt, purpose string) *models.ReviewResult {
	if review == nil || outline == nil || strings.TrimSpace(userPrompt) == "" {
		return review
	}
	boundaryIDs := collectAgentSDKRepairBoundaryChapterIDs(outline, userPrompt)
	if len(boundaryIDs) == 0 {
		return review
	}
	filtered := make([]models.ReviewSuggestion, 0, len(review.Suggestions))
	for _, suggestion := range review.Suggestions {
		if boundaryIDs[strings.TrimSpace(suggestion.TargetID)] {
			filtered = append(filtered, suggestion)
		}
	}
	limited := *review
	limited.Suggestions = filtered
	limited.Summary = fmt.Sprintf("%s Prompt boundary retained %d of %d targetable issue(s).", strings.TrimSpace(review.Summary), len(filtered), len(review.Suggestions))
	if len(filtered) == 0 && len(review.Suggestions) > 0 {
		if strings.TrimSpace(purpose) == "" {
			purpose = "review"
		}
		logger.Info("Skipping Agent SDK %s: user prompt boundary does not include remaining targetable issue(s)", purpose)
	}
	return &limited
}

func filterQualityGateForAgentSDKPromptBoundary(gate qualityGateResult, outline *models.Outline, userPrompt, purpose string) qualityGateResult {
	review := qualityGateReviewResult("Filter quality gate by Agent SDK prompt boundary.", gate)
	filtered := filterAgentSDKReviewForPromptBoundary(review, outline, userPrompt, purpose)
	if filtered == nil {
		return gate
	}
	limited := gate
	limited.Suggestions = append([]models.ReviewSuggestion(nil), filtered.Suggestions...)
	limited.Blocking = hasBlockingSuggestions(limited.Suggestions)
	return limited
}

func collectAgentSDKRepairBoundaryChapterIDs(outline *models.Outline, userPrompt string) map[string]bool {
	ids := map[string]bool{}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, id := range agents.AgentSDKImproveBoundaryChapterIDs(volume, userPrompt) {
				if id = strings.TrimSpace(id); id != "" {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

func limitReviewSuggestionsForAgentSDKRepair(review *models.ReviewResult, limit int) *models.ReviewResult {
	if review == nil {
		return nil
	}
	if limit <= 0 || len(review.Suggestions) <= limit {
		return review
	}
	limited := *review
	limited.Suggestions = append([]models.ReviewSuggestion(nil), review.Suggestions...)
	sort.SliceStable(limited.Suggestions, func(i, j int) bool {
		left := reviewSuggestionPriorityRank(limited.Suggestions[i].Priority)
		right := reviewSuggestionPriorityRank(limited.Suggestions[j].Priority)
		if left != right {
			return left < right
		}
		leftTarget := strings.TrimSpace(limited.Suggestions[i].TargetID)
		rightTarget := strings.TrimSpace(limited.Suggestions[j].TargetID)
		if leftTarget != rightTarget {
			return leftTarget < rightTarget
		}
		return strings.TrimSpace(limited.Suggestions[i].Category) < strings.TrimSpace(limited.Suggestions[j].Category)
	})
	limited.Suggestions = limited.Suggestions[:limit]
	limited.Summary = fmt.Sprintf("%s Repairing the first %d of %d targetable issues in this pass.", strings.TrimSpace(review.Summary), limit, len(review.Suggestions))
	return &limited
}

func reviewSuggestionPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case models.PriorityCritical:
		return 0
	case models.PriorityHigh:
		return 1
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 3
	default:
		return 4
	}
}

func outlinesSemanticallyEqual(left, right *models.Outline) bool {
	if left == nil || right == nil {
		return left == right
	}
	return reflect.DeepEqual(left, right)
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
