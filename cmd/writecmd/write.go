package writecmd

import (
	"fmt"

	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/project"
	writepipe "novelgen/internal/fiction/pipeline"

	"github.com/spf13/cobra"
)

func validateWriteMinScoreFlag() error {
	if writeMinScoreFlag < 0 || writeMinScoreFlag > 100 {
		return fmt.Errorf("--min-score must be between 0 and 100, got %d", writeMinScoreFlag)
	}
	return nil
}

func ValidateWriteBestOfFlag() error {
	if WriteBestOfFlag < 0 || WriteBestOfFlag > 5 {
		return fmt.Errorf("--best-of must be between 0 and 5, got %d", WriteBestOfFlag)
	}
	return nil
}

func writeMinScorePercent() float64 {
	return float64(writeMinScoreFlag)
}

// writeTargetWords resolves the chapter word budget. The flag default is only a
// fallback: an explicit --words value must win over the project config even
// when it happens to equal the historical default (2000).
func writeTargetWords(cmd *cobra.Command, config *models.ProjectConfig) int {
	targetWords := writeWordsFlag
	if cmd != nil && cmd.Flags().Changed("words") {
		return targetWords
	}
	if config != nil && config.ChapterConfig.TargetWordsPerChapter > 0 {
		return config.ChapterConfig.TargetWordsPerChapter
	}
	return targetWords
}

func loadWriteChapterContext(book *project.Book, outline *models.Outline, chapter *models.Chapter) *agents.ChapterContext {
	return writepipe.LoadChapterContextWithOptions(book, outline, chapter, writeContextFlag, writepipe.ChapterContextOptions{
		DisableRecap: writeNoRecapContextFlag,
	})
}

var (
	writeChapterFlag               string
	writeVolumeFlag                string
	writePartFlag                  string
	writeWordsFlag                 int
	writeAllFlag                   bool
	writeContextFlag               int
	writeConcurrencyFlag           int
	writeMaxRoundsFlag             int
	writeMinScoreFlag              int
	writeBridgeRetriesFlag         int
	writeTeleportFixFlag           bool
	writeCharacterPatchRetriesFlag int
	writeCharacterFixFlag          bool
	writePromptFlag                string
	writeEmitRPGDSLFlag            bool
	writeRPGBatchSizeFlag          int
	writeHumanizeFlag              bool
	writeHumanizeThresholdFlag     int
	writeFocusFlag                 string
	writeAgentSDKFlag              bool
	writeAgentApplyFlag            bool
	writeAgentHistoryFlag          bool
	writeRecapAgentSDKFlag         bool
	writeNoRecapContextFlag        bool
	WriteBestOfFlag                int
)

var writeCmd = &cobra.Command{
	Use:   "write",
	Short: "Generate, review, improve, and validate final chapters",
	Long: `Generate polished final chapter content directly from outline, story setup,
RPG state, recaps, and neighboring chapter context.

The recommended default is "write pipeline": it writes final chapters, reviews
and improves them, extracts recaps, and updates chapter-level RPG DSL.

Features:
  - Context-aware generation (includes surrounding chapters)
  - RPG state tracking (character states, relationships, items, resources)
  - Recap extraction for continuity
  - Chapter RPG DSL export for simulation checks
  - Consistent voice and style across chapters

Final chapters are saved to the chapters/ directory.

Subcommands:
  pipeline - Recommended direct-to-final flow
  gen      - Generate final chapters
  review   - Review final chapters
  improve  - Improve final chapters based on review`,
}

var WriteGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate final chapter content",
	Long: `Generate final chapter content with continuity from outline state, recaps,
and surrounding final chapters when available.

Examples:
  # Generate final content for chapter 1
  novelgen write gen --chapter 1

  # Generate final content for chapters 1 to 5
  novelgen write gen --chapter 1-5

  # Generate final content for all chapters
  novelgen write gen --all

  # Generate with custom word count
  novelgen write gen --chapter 1 --words 2000

  # Generate with 3 chapters of context on each side
  novelgen write gen --chapter 5 --context 3

  # Generate 3 independent copies and save the highest-scoring one
  novelgen write gen --chapter 1 --best-of 3`,
	RunE: runWriteGen,
}

var WriteImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve final chapters based on review",
	Long: `Improve final chapters that have concrete review or deterministic-check findings.

This command will:
1. Load or refresh reviews for the specified chapters/volumes
2. Identify chapters with actionable findings
3. Regenerate those chapters with improvement suggestions
4. Repeat for the specified number of rounds

Scores are diagnostic. --min-score is a secondary floor for chapters that
already contain findings; a score alone does not force a rewrite. A prompt
combined with --volume only narrows work to chapters selected by findings.
Use --chapter to force one chapter through improvement.

Examples:
  # Improve all chapters in volume 1
  novelgen write improve --volume 1

  # Improve with max 3 rounds
  novelgen write improve --volume 1 --max-rounds 3

  # Apply findings below the diagnostic 70-point floor
  novelgen write improve --volume 1 --min-score 70`,
	RunE: runWriteImprove,
}

var WriteReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review final chapter content",
	Long: `Review final chapter content and generate review reports.

This command will:
1. Load existing final chapters
2. Review each chapter for quality
3. Save review results to story/reviews/

Examples:
  # Review chapter 1
  novelgen write review --chapter 1

  # Review all chapters in volume 1
  novelgen write review --volume 1

  # Review using Agent SDK focused tools
  novelgen write review --chapter 1 --agent-sdk

  # Review all chapters
  novelgen write review --all`,
	RunE: runWriteReview,
}

var WritePipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run complete writing pipeline: gen -> review -> improve -> recap",
	Long: `Run the complete writing pipeline for a chapter or volume.

This command will:
1. Generate final chapter content (if not exists)
2. Review the generated content
3. Improve content based on review (iterative)
4. Generate recap for continuity

Examples:
  # Run pipeline for chapter 1
  novelgen write pipeline --chapter 1

  # Run pipeline for volume 1 with 3 improvement rounds
  novelgen write pipeline --volume 1 --max-rounds 3

  # Run pipeline with custom target words
  novelgen write pipeline --chapter 1 --words 3000`,
	RunE: runWritePipeline,
}

func init() {
	writeCmd.AddCommand(WriteGenCmd)
	writeCmd.AddCommand(WriteImproveCmd)
	writeCmd.AddCommand(WriteReviewCmd)
	writeCmd.AddCommand(WritePipelineCmd)

	WriteGenCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'P1-V1-C1')")
	WriteGenCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume number for context (e.g., '1', 'P1-V1')")
	WriteGenCmd.Flags().StringVar(&writePartFlag, "part", "", "Part number for context (e.g., '1', 'P1')")
	WriteGenCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	WriteGenCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Generate content for all chapters")
	WriteGenCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	WriteGenCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")
	WriteGenCmd.Flags().IntVar(&WriteBestOfFlag, "best-of", 0, "Generate N independent copies of each chapter, score them, and save the best (0 disables, max 5)")
	WriteGenCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter generation")
	WriteGenCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before writing")
	WriteGenCmd.Flags().BoolVar(&writeRecapAgentSDKFlag, "recap-agent-sdk", false, "Use Claude Agent SDK workflow for automatic recap extraction")
	WriteGenCmd.Flags().BoolVar(&writeNoRecapContextFlag, "no-recap-context", false, "Omit the previous-chapter recap from prompt context; previous chapter text and canonical state remain available")

	WriteImproveCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	WriteImproveCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	WriteImproveCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	WriteImproveCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the improved chapter")
	WriteImproveCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	WriteImproveCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Score floor for chapters that also have concrete review findings (0-100; diagnostic, not a release gate)")
	WriteImproveCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")
	WriteImproveCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	WriteImproveCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	WriteImproveCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	WriteImproveCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	WriteImproveCmd.Flags().StringVar(&writePromptFlag, "prompt", "", "Additional user instructions for improvement")
	WriteImproveCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks and humanization suggestions")
	WriteImproveCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before improvement is suggested (0-100)")
	WriteImproveCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter improvement")
	WriteImproveCmd.Flags().BoolVar(&writeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write final chapter markdown through validated patch tools")
	WriteImproveCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before improving")
	WriteImproveCmd.Flags().BoolVar(&writeNoRecapContextFlag, "no-recap-context", false, "Omit the previous-chapter recap from prompt context; previous chapter text and canonical state remain available")

	WriteReviewCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to review (e.g., '1' or 'P1-V1-C1')")
	WriteReviewCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to review (e.g., '1', 'P1-V1')")
	WriteReviewCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to review (e.g., '1', 'P1')")
	WriteReviewCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Review all chapters")
	WriteReviewCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count used when reviewing chapter length")
	WriteReviewCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent reviews")
	WriteReviewCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks in write review")
	WriteReviewCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before review marks style issues (0-100)")
	WriteReviewCmd.Flags().StringVar(&writeFocusFlag, "focus", "", "Review focus (comma-separated, e.g. deai,protagonist; 'all' for all). Uses the same AI review focuses as compose review (see --focus list). Empty = generic chapter review.")
	WriteReviewCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter review")
	WriteReviewCmd.Flags().BoolVar(&writeNoRecapContextFlag, "no-recap-context", false, "Omit the previous-chapter recap from prompt context; previous chapter text and canonical state remain available")

	WritePipelineCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to process (e.g., '1' or 'P1-V1-C1')")
	WritePipelineCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to process (e.g., '1', 'P1-V1')")
	WritePipelineCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to process (e.g., '1', 'P1')")
	WritePipelineCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Process all chapters")
	WritePipelineCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	WritePipelineCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 2, "Maximum improvement rounds")
	WritePipelineCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Score floor for chapters that also have concrete review findings (0-100; diagnostic, not a release gate)")
	WritePipelineCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	WritePipelineCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent operations")
	_ = WritePipelineCmd.Flags().MarkHidden("concurrency")
	WritePipelineCmd.Flags().IntVar(&WriteBestOfFlag, "best-of", 0, "Generate N independent copies of each chapter, score them, and save the best (0 disables, max 5)")
	WritePipelineCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	WritePipelineCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	WritePipelineCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	WritePipelineCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	WritePipelineCmd.Flags().BoolVar(&writeEmitRPGDSLFlag, "emit-rpg-dsl", true, "Update story/rpg/04_chapters.rpg from chapter markdown plus optional recaps")
	WritePipelineCmd.Flags().IntVar(&writeRPGBatchSizeFlag, "rpg-batch-size", 10, "Chapter markdown batch size for AI -> RPG DSL conversion")
	WritePipelineCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks and humanization suggestions")
	WritePipelineCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before improvement is suggested (0-100)")
	WritePipelineCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter generation, review, and improvement")
	WritePipelineCmd.Flags().BoolVar(&writeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write final chapter markdown through validated patch tools")
	WritePipelineCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before writing or improving")
	WritePipelineCmd.Flags().BoolVar(&writeRecapAgentSDKFlag, "recap-agent-sdk", false, "Use Claude Agent SDK workflow for automatic recap extraction")
	WritePipelineCmd.Flags().BoolVar(&writeNoRecapContextFlag, "no-recap-context", false, "Omit the previous-chapter recap from prompt context; previous chapter text and canonical state remain available")

}

// NewCommand returns the `novelgen write` command tree.
func NewCommand() *cobra.Command {
	return writeCmd
}
