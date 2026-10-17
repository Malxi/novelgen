// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/fiction/agents"
)

var (
	composePromptFlag            string
	composeReviewFocusFlag       string
	composeReviewMatrixFlag      bool
	composeCrossVolumeFlag       bool
	composeReviewFromVolumeFlag  int
	composeReviewToVolumeFlag    int
	composeReviewModelsFlag      string
	composeReviewSampleFlag      int
	composeReviewSeedFlag        int64
	composeReviewParallelFlag    int
	composeMaxRoundsFlag         int
	composeConcurrencyFlag       int
	composeHierarchicalFlag      bool
	composeOneShotFlag           bool
	composeAgentSDKFlag          bool
	composeAgentApplyFlag        bool
	composeRepairBudgetFlag      int
	composeForceImproveFlag      bool
	composeForceGenFlag          bool
	composeCheckJSONFlag         bool
	composeCheckSuggestionsOut   string
	composeImproveVolume         int
	composeImproveFromVol        int
	composeImproveToVol          int
	composeSuggestionsFlag       string
	composeCrossVolumeAllFlag    bool
	composeReviewOutFlag         string
	composeReviewSurfaceOnlyFlag bool
	composeModelFlag             string
	composePipelineFromVol       int
	composePipelineToVol         int
	composePipelineMaxRounds     int
	composePipelineForce         bool
	composePipelineSkipGen       bool
	composePipelineSkipImprove   bool
	composePipelineSkipCross     bool
	composeNormalizeForceFlag    bool
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
	composeNormalizeCmd.Flags().BoolVar(&composeNormalizeForceFlag, "force", false, "Rewrite the outline JSON and derived markdown even when no normalization change is needed")
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
	composeReviewCmd.Flags().StringVar(&composeReviewFocusFlag, "focus", "", "Built-in review focus: "+strings.Join(agents.ListReviewFocusNames(), ",")+" (comma-separated, or 'all')")
	composeReviewCmd.Flags().BoolVar(&composeReviewMatrixFlag, "matrix", false, "Run a multi-model x multi-focus review matrix with clustered sampling")
	composeReviewCmd.Flags().BoolVar(&composeCrossVolumeFlag, "cross-volume", false, "Review cross-volume continuity across a volume range in one agent session")
	composeReviewCmd.Flags().IntVar(&composeReviewFromVolumeFlag, "from-volume", 0, "Cross-volume review start (1-based global volume index)")
	composeReviewCmd.Flags().IntVar(&composeReviewToVolumeFlag, "to-volume", 0, "Cross-volume review end (1-based global volume index, default: last)")
	composeReviewCmd.Flags().StringVar(&composeReviewModelsFlag, "models", "", "Comma-separated model list for --matrix (default: project model)")
	composeReviewCmd.Flags().BoolVar(&composeReviewSurfaceOnlyFlag, "surface-only", false, "With --cross-volume, review only reader-visible fields (title/summary/opening_beat/scenes); skip events/payoff_contract/state_anchor internal panels (for judging reader experience, not author panels)")
	composeReviewCmd.Flags().IntVar(&composeReviewSampleFlag, "sample", 10, "Stratified sample size for --matrix output")
	composeReviewCmd.Flags().Int64Var(&composeReviewSeedFlag, "seed", 42, "Random seed for --matrix sampling")
	composeReviewCmd.Flags().IntVar(&composeReviewParallelFlag, "parallel", 4, "Concurrent reviews in --matrix mode")
	composeReviewCmd.Flags().IntVar(&composeImproveVolume, "volume", 0, "Review one 1-based global volume index")
	composeReviewCmd.Flags().StringVar(&composeReviewOutFlag, "out", "",
		"Review report output path (default: story/compose/outline_review.json inside the project)")
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

// composeAgentSDKImproveResumeAttempts bounds how many times an Agent SDK
// improvement run is retried after a transient runner/network failure. The
// checkpoint file (outline_improve_progress.json) makes each retry resume from
// the last completed volume instead of restarting the whole range.
const composeAgentSDKImproveResumeAttempts = 3

type composeImproveResumeProgress struct {
	Iteration     int      `json:"iteration"`
	TargetVolumes []string `json:"target_volumes"`
}

func NewCommand() *cobra.Command {
	return composeCmd
}
