package writecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"novelgen/internal/agents"
	novelcont "novelgen/internal/continuity"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/logic/style"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/reviewstore"
	"novelgen/internal/rpg/dsl"
	"novelgen/internal/store"
	"novelgen/internal/toolplane"
	"novelgen/internal/workerpool"
	"novelgen/internal/writepipe"
	"novelgen/internal/writersdl"

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
	Long: `Improve final chapters by reviewing and regenerating content that doesn't meet quality standards.

This command will:
1. Load existing reviews for the specified chapters/volumes
2. Identify chapters that need improvement (below min-score)
3. Regenerate those chapters with improvement suggestions
4. Repeat for the specified number of rounds

Examples:
  # Improve all chapters in volume 1
  novelgen write improve --volume 1

  # Improve with max 3 rounds
  novelgen write improve --volume 1 --max-rounds 3

  # Only improve chapters with score below 70
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

	WriteImproveCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	WriteImproveCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	WriteImproveCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	WriteImproveCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the improved chapter")
	WriteImproveCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	WriteImproveCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Minimum acceptable score (0-100)")
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

	WritePipelineCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to process (e.g., '1' or 'P1-V1-C1')")
	WritePipelineCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to process (e.g., '1', 'P1-V1')")
	WritePipelineCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to process (e.g., '1', 'P1')")
	WritePipelineCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Process all chapters")
	WritePipelineCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	WritePipelineCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 2, "Maximum improvement rounds")
	WritePipelineCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Minimum acceptable score (0-100)")
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

}

// NewCommand returns the `novelgen write` command tree.
func NewCommand() *cobra.Command {
	return writeCmd
}

func runWriteGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := ValidateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	if err := ValidateWriteBestOfFlag(); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	agent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	root := book.Dir()
	setupWriteRunLogging(root, "write gen")

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilder(root)
	// Create recap agent + store (auto-persist recaps for continuity)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)
	// Create lightweight best-of scorer (used only when --best-of is enabled)
	scorer := agents.NewChapterScorerAgent(client, cfg, &config.LLM)
	scorer.SetLanguage(config.Language)
	var errc writepipe.WriteErrorCollector

	// Get list of chapters to generate
	chapters, err := project.SelectChapters(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating final content for %d chapter(s) with concurrency %d", len(chapters), writeConcurrencyFlag)
	if WriteBestOfFlag > 1 {
		log.Info("Best-of mode enabled: %d independent copies per chapter, scored and best saved", WriteBestOfFlag)
	}
	if writeAgentSDKFlag {
		log.Info("Chapter generation will use Agent SDK; Go still validates and saves final markdown")
		if writeAgentHistoryFlag {
			log.Info("Agent history enabled: SDK must inspect queryable logs before chapter generation")
		}
	}
	if writeRecapAgentSDKFlag {
		log.Info("Automatic recap extraction will use Agent SDK; Go still validates and saves recap JSON")
	}

	// Use worker pool for concurrent generation
	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	_, _ = workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (struct{}, error) {
		log.Info("[Worker %d] Generating content for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load context drafts (previous and next chapters)
		context := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)

		// Calculate chapter continuity
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Generate final content (optionally best-of-N with AI scoring)
		content, err := GenerateChapterContentForRun(ctx, agent, scorer, chapter, context, continuity, targetWords, WriteBestOfFlag, writeAgentSDKFlag, writeAgentHistoryFlag)
		if err != nil {
			log.Error("Failed to generate content for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: generate failed: %w", chapter.ID, err)
			return struct{}{}, nil
		}

		// Save final content
		if err := book.SaveFinalChapter(chapter, content); err != nil {
			log.Error("Failed to save content for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: save final chapter failed: %w", chapter.ID, err)
			return struct{}{}, nil
		}
		savedContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("Failed to read back chapter %s: %v", chapter.ID, readErr)
			errc.Addf("%s: read back final chapter failed: %w", chapter.ID, readErr)
			return struct{}{}, nil
		}
		if strings.TrimSpace(savedContent) != "" {
			content = savedContent
		}
		if writeAgentSDKFlag {
			if result, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords); err != nil {
				log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
			} else {
				writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, result)
				if result.Blocking {
					log.Warn("[Worker %d] Agent SDK generated chapter %s still has blocking post-save issues; attempting one validated agent repair", workerID, chapter.ID)
					repaired, repairedCheck, repairErr := repairAgentSDKGeneratedChapterPostSave(ctx, book, log, workerID, agent, chapter, context, continuity, targetWords, content, result, writeAgentHistoryFlag)
					if repairErr != nil {
						log.Error("[Worker %d] Agent SDK post-save repair failed for %s: %v", workerID, chapter.ID, repairErr)
						errc.Addf("%s: post-save repair failed: %w", chapter.ID, repairErr)
						return struct{}{}, nil
					}
					content = repaired
					writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, repairedCheck)
					if repairedCheck != nil && repairedCheck.Blocking {
						errc.Addf("%s: post-save repair still has blocking issues", chapter.ID)
						return struct{}{}, nil
					}
				}
			}
		}

		// Auto-extract + persist recap for this final chapter (best-effort)
		if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, writeRecapAgentSDKFlag); err != nil {
			log.Warn("[Worker %d] Recap gate failed for %s: %v", workerID, chapter.ID, err)
		}

		log.Info("[Worker %d] Content saved for chapter %s: %d narrative units", workerID, chapter.ID, writepipe.ChapterNarrativeUnitsForLog(chapter, content))
		return struct{}{}, nil
	})

	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Chapter writing complete")
	return nil
}

// GenerateChapterContentForRun produces chapter content either through the
// existing single-copy path or, when --best-of is enabled, through the
// parallel best-of-N generation + AI scoring flow.
func GenerateChapterContentForRun(
	ctx context.Context,
	agent *agents.WriteAgent,
	scorer *agents.ChapterScorerAgent,
	chapter *models.Chapter,
	context *agents.ChapterContext,
	continuity *models.ChapterContinuity,
	targetWords int,
	bestOf int,
	useAgentSDK bool,
	useHistory bool,
) (string, error) {
	if bestOf <= 1 {
		if useAgentSDK {
			return agent.GenerateChapterWithAgentSDK(ctx, chapter, context, continuity, targetWords, useHistory)
		}
		return agent.GenerateChapter(ctx, chapter, context, continuity, targetWords)
	}
	return GenerateChapterBestOf(ctx, agent, scorer, chapter, context, continuity, targetWords, bestOf, useAgentSDK, useHistory)
}

// GenerateChapterBestOf generates N independent copies of the same chapter in
// parallel, scores each successful copy with the lightweight reader scorer,
// and returns the highest-scoring content. Failures are isolated per copy:
// a failed copy is skipped, and if every scoring attempt fails the first
// successfully generated copy is used so generation can still proceed.
func GenerateChapterBestOf(
	ctx context.Context,
	agent *agents.WriteAgent,
	scorer *agents.ChapterScorerAgent,
	chapter *models.Chapter,
	context *agents.ChapterContext,
	continuity *models.ChapterContinuity,
	targetWords int,
	bestOf int,
	useAgentSDK bool,
	useHistory bool,
) (string, error) {
	log := logger.GetLogger()

	type generationResult struct {
		index   int
		content string
		err     error
	}
	results := make([]generationResult, bestOf)
	var genWg sync.WaitGroup
	for i := 0; i < bestOf; i++ {
		genWg.Add(1)
		go func(index int) {
			defer genWg.Done()
			log.Info("[BestOf] Generating copy %d/%d for chapter %s", index+1, bestOf, chapter.ID)
			var content string
			var err error
			if useAgentSDK {
				content, err = agent.GenerateChapterWithAgentSDK(ctx, chapter, context, continuity, targetWords, useHistory)
			} else {
				content, err = agent.GenerateChapter(ctx, chapter, context, continuity, targetWords)
			}
			results[index] = generationResult{index: index, content: content, err: err}
		}(i)
	}
	genWg.Wait()

	type scoredResult struct {
		index   int
		content string
		score   int
		reason  string
		err     error
	}
	copies := make([]scoredResult, 0, bestOf)
	for _, result := range results {
		if result.err != nil {
			log.Error("[BestOf] Copy %d/%d generation failed for chapter %s: %v", result.index+1, bestOf, chapter.ID, result.err)
			continue
		}
		copies = append(copies, scoredResult{index: result.index, content: result.content})
	}
	if len(copies) == 0 {
		return "", fmt.Errorf("all %d best-of copies failed to generate for chapter %s", bestOf, chapter.ID)
	}

	// Score all successful copies in parallel; each scoring call is one cheap
	// LLM request.
	var scoreWg sync.WaitGroup
	for i := range copies {
		scoreWg.Add(1)
		go func(c *scoredResult) {
			defer scoreWg.Done()
			output, err := scorer.ScoreChapter(ctx, BuildChapterScoreInput(chapter, c.content, context))
			if err != nil {
				c.err = err
				return
			}
			c.score = int(output.Score)
			c.reason = output.Reason
		}(&copies[i])
	}
	scoreWg.Wait()

	scored := make([]agents.ScoredChapterCopy, 0, len(copies))
	for _, c := range copies {
		if c.err != nil {
			log.Error("[BestOf] Scoring copy %d/%d failed for chapter %s: %v", c.index+1, bestOf, chapter.ID, c.err)
			continue
		}
		scored = append(scored, agents.ScoredChapterCopy{
			CopyIndex: c.index,
			Content:   c.content,
			Score:     c.score,
			Reason:    c.reason,
		})
	}

	if len(scored) == 0 {
		log.Warn("[BestOf] All scoring attempts failed for chapter %s; falling back to copy %d/%d", chapter.ID, copies[0].index+1, bestOf)
		return copies[0].content, nil
	}

	best, err := agents.SelectBestScoredChapter(scored)
	if err != nil {
		return "", fmt.Errorf("best-of selection failed for chapter %s: %w", chapter.ID, err)
	}

	log.Info("[BestOf] Chapter %s copy score comparison:", chapter.ID)
	for _, c := range scored {
		marker := " "
		if c.CopyIndex == best.CopyIndex {
			marker = "*"
		}
		log.Info("[BestOf]   %s copy %d/%d: score %d/100 - %s", marker, c.CopyIndex+1, bestOf, c.Score, c.Reason)
	}
	log.Info("[BestOf] Selected copy %d/%d (score %d/100) for chapter %s", best.CopyIndex+1, bestOf, best.Score, chapter.ID)
	return best.Content, nil
}

// BuildChapterScoreInput builds the lightweight scorer input from the outline
// chapter and its generated content. Outline info is optional: only the
// chapter's own summary/beats and the immediate next chapter's summary are
// included so the scorer can judge beat coverage and the ending hook.
func BuildChapterScoreInput(chapter *models.Chapter, content string, context *agents.ChapterContext) agents.ChapterScoreInput {
	input := agents.ChapterScoreInput{
		ChapterID:      chapter.ID,
		ChapterTitle:   chapter.Title,
		ChapterContent: content,
		ChapterSummary: chapter.Summary,
		ChapterBeats:   chapter.GetBeats(),
	}
	if context != nil && len(context.Next) > 0 && context.Next[0] != nil && context.Next[0].Chapter != nil {
		input.NextChapterSummary = context.Next[0].Chapter.Summary
	}
	return input
}

func FinalChapterContentChanged(chapter *models.Chapter, before, after string) bool {
	return project.ChapterContentChanged(chapter, before, after)
}

func saveWriteReviewResult(book *project.Book, chapter *models.Chapter, review models.ReviewResult) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	root, err := store.Default()
	if err != nil {
		return err
	}
	return root.WriteJSON(root.ReviewFile(chapter.ID+"_write_review.json"), review)
}

func loadFinalChapterContentsForVolume(book *project.Book, volume *models.Volume) (map[string]string, error) {
	contents := make(map[string]string)
	if volume == nil {
		return contents, nil
	}
	for i := range volume.Chapters {
		ch := &volume.Chapters[i]
		content, readErr := book.LoadFinalChapterContent(ch)
		if readErr != nil {
			return nil, readErr
		}
		if strings.TrimSpace(content) != "" {
			contents[ch.ID] = content
		}
	}
	return contents, nil
}

func findDraftReview(review *agents.VolumeReview, chapterID string) *agents.DraftReview {
	if review == nil {
		return nil
	}
	for i := range review.Reviews {
		if review.Reviews[i].ChapterID == chapterID {
			return &review.Reviews[i]
		}
	}
	return nil
}

func MergeVolumeReviewByChapter(existing, incoming *agents.VolumeReview) *agents.VolumeReview {
	if incoming == nil {
		return existing
	}
	if existing == nil || existing.VolumeID == "" {
		return incoming
	}
	if incoming.VolumeID != "" && existing.VolumeID != incoming.VolumeID {
		return incoming
	}

	merged := *existing
	if incoming.VolumeID != "" {
		merged.VolumeID = incoming.VolumeID
	}
	if incoming.VolumeTitle != "" {
		merged.VolumeTitle = incoming.VolumeTitle
	}
	if incoming.Summary != "" {
		merged.Summary = incoming.Summary
	}

	indexByChapter := make(map[string]int, len(merged.Reviews))
	for i := range merged.Reviews {
		indexByChapter[merged.Reviews[i].ChapterID] = i
	}
	for _, review := range incoming.Reviews {
		if i, ok := indexByChapter[review.ChapterID]; ok {
			merged.Reviews[i] = review
			continue
		}
		indexByChapter[review.ChapterID] = len(merged.Reviews)
		merged.Reviews = append(merged.Reviews, review)
	}
	return &merged
}

func applyHumanizeCheckToReview(chapter *models.Chapter, content string, review *agents.DraftReview) {
	if !writeHumanizeFlag || review == nil || strings.TrimSpace(content) == "" {
		return
	}
	result := style.CheckAIFlavor(content, writeHumanizeThresholdFlag)
	if len(result.Issues) == 0 && len(result.Suggestions) == 0 {
		return
	}

	if review.PacingReview.Score == 0 {
		review.PacingReview.Score = 8
	}
	for _, issue := range result.Issues {
		msg := fmt.Sprintf("去AI味检查（score=%d）：%s", result.Score, issue)
		if !novelcont.ContainsStr(review.PacingReview.Issues, msg) {
			review.PacingReview.Issues = append(review.PacingReview.Issues, msg)
		}
	}
	for _, suggestion := range result.Suggestions {
		msg := "【去AI味】" + suggestion
		if !novelcont.ContainsStr(review.PacingReview.Suggestions, msg) {
			review.PacingReview.Suggestions = append(review.PacingReview.Suggestions, msg)
		}
	}
	if result.HasIssue {
		review.NeedsRevision = true
		if review.OverallScore == 0 || review.OverallScore > 7 {
			review.OverallScore = 7
		}
		if chapter != nil {
			summary := fmt.Sprintf("[medium] style: 本章%s去AI味得分 %d，建议按“去AI味”条目改写公式化表达。", chapter.ID, result.Score)
			if !novelcont.ContainsStr(review.Suggestions, summary) {
				review.Suggestions = append(review.Suggestions, summary)
			}
		}
	}
}

func applyHumanizeChecksToVolume(book *project.Book, volume *models.Volume, contents map[string]string, review *agents.VolumeReview) {
	if !writeHumanizeFlag || volume == nil || review == nil {
		return
	}
	contentByID := contents
	if contentByID == nil {
		loaded, err := loadFinalChapterContentsForVolume(book, volume)
		if err != nil {
			logger.GetLogger().Warn("Cannot load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
			loaded = map[string]string{}
		}
		contentByID = loaded
	}
	chapterByID := make(map[string]*models.Chapter)
	for i := range volume.Chapters {
		chapterByID[volume.Chapters[i].ID] = &volume.Chapters[i]
	}
	for i := range review.Reviews {
		r := &review.Reviews[i]
		applyHumanizeCheckToReview(chapterByID[r.ChapterID], contentByID[r.ChapterID], r)
	}
}

// runWriteImprove improves final chapters based on review
func runWriteImprove(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := validateWriteMinScoreFlag(); err != nil {
		return err
	}
	if err := ValidateWriteBestOfFlag(); err != nil {
		return err
	}
	if err := ValidateWriteAgentApplyOption(writeAgentSDKFlag, writeAgentApplyFlag); err != nil {
		return err
	}
	if err := ValidateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

	root := book.Dir()
	setupWriteRunLogging(root, "write improve")
	if writeAgentSDKFlag {
		log.Info("Chapter improvement will use Agent SDK; Go still validates and saves final markdown")
	}
	if writeAgentApplyFlag {
		log.Info("Agent apply enabled: SDK may write final markdown through validated chapter patch tools")
	}

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilder(root)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)

	// Get volumes to improve
	volumes := project.VolumesForSelection(outline, writeVolumeFlag, writeChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	forcedAgentSDKImprove := writeAgentSDKFlag && (strings.TrimSpace(writeChapterFlag) != "" || strings.TrimSpace(writePromptFlag) != "")

	// Auto-review flag - set to true if any volume needs review
	autoReviewNeeded := false

	// Check if reviews exist for all volumes
	for _, volume := range volumes {
		_, err := reviewstore.LoadVolumeReviewAt(book, volume.ID)
		if err != nil {
			if forcedAgentSDKImprove {
				log.Info("No review found for volume %s; Agent SDK forced improvement will use focused chapter checks instead of auto-review", volume.ID)
			} else {
				log.Info("No review found for volume %s, will auto-review", volume.ID)
				autoReviewNeeded = true
			}
		}
	}

	// Auto-review if needed
	if autoReviewNeeded {
		log.Info("=== Auto-Review Phase ===")
		log.Info("Some volumes don't have reviews, running auto-review first...")

		// Run review for volumes that need it
		for _, volume := range volumes {
			_, err := reviewstore.LoadVolumeReviewAt(book, volume.ID)
			if err != nil {
				log.Info("Auto-reviewing volume: %s", volume.ID)
				if err := reviewVolumeWithWriteAgent(ctx, book, writeAgent, continuityBuilder, volume, outline, targetWords, writeConcurrencyFlag); err != nil {
					log.Error("Failed to auto-review volume %s: %v", volume.ID, err)
					return err
				}
			}
		}
		log.Info("Auto-review phase complete")
	}

	// Run improvement rounds
	for round := 1; round <= writeMaxRoundsFlag; round++ {
		log.Info("=== Improvement Round %d/%d ===", round, writeMaxRoundsFlag)

		improvedCount := 0

		for _, volume := range volumes {
			// Load review for this volume
			review, err := reviewstore.LoadVolumeReviewAt(book, volume.ID)
			if err != nil {
				if forcedAgentSDKImprove {
					log.Warn("No review found for volume %s; continuing forced Agent SDK improvement with focused check suggestions", volume.ID)
					review = &agents.VolumeReview{
						VolumeID:    volume.ID,
						VolumeTitle: volume.Title,
						Reviews:     []agents.DraftReview{},
					}
				} else {
					log.Warn("No review found for volume %s, skipping", volume.ID)
					continue
				}
			}

			// Get chapters that need improvement
			// If user prompt is provided or specific chapter is specified, force improvement regardless of score
			var chaptersToImprove []*models.Chapter
			if writePromptFlag != "" || writeChapterFlag != "" {
				// Force improvement: get chapters specified by flags
				chaptersToImprove = getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, false)
				if writePromptFlag != "" {
					log.Info("Volume %s: Force improving %d chapters (user prompt provided)", volume.ID, len(chaptersToImprove))
				} else {
					log.Info("Volume %s: Force improving %d chapters (chapter specified)", volume.ID, len(chaptersToImprove))
				}
			} else {
				chaptersToImprove = GetWriteChaptersNeedingImprovement(review, outline, writeMinScoreFlag)
				if len(chaptersToImprove) == 0 {
					log.Info("Volume %s: All chapters meet quality threshold", volume.ID)
					continue
				}
				log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))
			}

			// Improve chapters concurrently
			improved, err := improveChaptersWithWriteAgent(ctx, book, writeAgent, recapAgent, recapStore, chaptersToImprove, review.Reviews, outline, continuityBuilder, writeConcurrencyFlag, targetWords, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag)
			if err != nil {
				log.Error("Failed to improve some chapters in volume %s: %v", volume.ID, err)
				return err
			}
			improvedCount += improved
		}

		log.Info("Round %d complete: %d chapters improved", round, improvedCount)

		if improvedCount == 0 {
			log.Info("No more chapters need improvement")
			break
		}

		// Re-review after improvement (if not last round)
		if round < writeMaxRoundsFlag {
			log.Info("Re-reviewing after improvements...")
			if err := runWriteReview(cmd, args); err != nil {
				log.Error("Re-review failed: %v", err)
			}
		}
	}

	log.Info("Improvement process complete")
	return nil
}

func ValidateWriteAgentApplyOption(useAgentSDK, agentApply bool) error {
	if agentApply && !useAgentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
}

func ValidateWriteAgentHistoryOption(useAgentSDK, agentHistory bool) error {
	if agentHistory && !useAgentSDK {
		return fmt.Errorf("--agent-history requires --agent-sdk")
	}
	return nil
}

func setupWriteAgentHistoryCutoff(agentHistory bool) {
	if !agentHistory {
		return
	}
	_ = os.Setenv("NOVELGEN_LOG_HISTORY_CUTOFF", time.Now().Format(time.RFC3339Nano))
}

// improveChaptersWithWriteAgent improves chapters using the write agent
func improveChaptersWithWriteAgent(ctx context.Context, book *project.Book, agent *agents.WriteAgent, recapAgent writepipe.RecapExtractor, recapStore *recap.Store, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, continuityBuilder *logic.ChapterContinuityBuilder, concurrency int, targetWords int, useAgentSDK bool, agentApply bool, useAgentHistory bool) (int, error) {
	log := logger.GetLogger()
	var errc writepipe.WriteErrorCollector

	// Pre-build DSL simulation for enrichment (shared across goroutines)
	var dslIssues []dsl.SimulationIssue
	dslBridge := dsl.NewSimulationBridge()
	if charModels, locModels, itemModels, orgModels, elErr := book.LoadElements(); elErr == nil {
		setup, _ := book.LoadSetup()
		dslAdapter := dsl.NewModelAdapterWithOrganizations(setup, outline, charModels, locModels, itemModels, orgModels)
		dslIssues, _ = dslAdapter.Simulate(dsl.PhaseCraft)
		if len(dslIssues) > 0 {
			log.Info("DSL simulation loaded %d issues for improvement enrichment", len(dslIssues))
		}
	}

	// Create review map for quick lookup
	reviewMap := make(map[string]*agents.DraftReview)
	for i := range reviews {
		reviewMap[reviews[i].ChapterID] = &reviews[i]
	}

	improved, _ := workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (bool, error) {
		review := reviewMap[chapter.ID]
		if review == nil {
			log.Warn("[Worker %d] No existing review for forced chapter %s, using focused check suggestions", workerID, chapter.ID)
			review = fallbackDraftReviewForForcedImprove(chapter)
		}

		log.Info("[Worker %d] Improving chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load current chapter content
		currentContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if currentContent == "" {
			if useAgentSDK && agentApply {
				log.Warn("[Worker %d] No existing content for chapter %s; Agent SDK apply may create it through validated chapter patch", workerID, chapter.ID)
			} else {
				log.Error("[Worker %d] No existing content for chapter %s, skipping improvement", workerID, chapter.ID)
				errc.Addf("%s: no existing content for improvement", chapter.ID)
				return false, nil
			}
		}
		applyHumanizeCheckToReview(chapter, currentContent, review)

		// Build improvement suggestions
		suggestions := reviewstore.BuildImprovementSuggestions(review)

		// Append DSL simulation feedback for this chapter
		if len(dslIssues) > 0 {
			chapterIssues := dslBridge.IssuesForChapter(dslIssues, chapter.ID)
			if len(chapterIssues) > 0 {
				dslFeedback := dslBridge.FormatAsString(chapterIssues)
				if dslFeedback != "" {
					suggestions += "\n\n" + dslFeedback
				}
			}
		}

		// Append user prompt if provided
		if writePromptFlag != "" {
			suggestions += "\n\n## 用户要求\n\n" + writePromptFlag
		}
		if useAgentSDK {
			if checkSuggestions := writepipe.BuildAgentSDKChapterCheckSuggestions(book, chapter, targetWords); checkSuggestions != "" {
				suggestions += "\n\n" + checkSuggestions
			}
		}

		// Load context drafts
		context := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)

		// Calculate chapter continuity
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Generate improved content with suggestions
		content, err := writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, useAgentSDK, agentApply, useAgentHistory)
		if err != nil {
			log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: improve failed: %w", chapter.ID, err)
			return false, nil
		}
		content, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, workerID, chapter, currentContent, content, useAgentSDK, agentApply)

		// Apply enabled minimal-change fixers (teleport bridge, character presence)
		knownChars := project.KnownCharacterNames(outline)
		fixed, sum := novelcont.ApplyImproveFixesWrite(
			log,
			workerID,
			chapter,
			outline,
			content,
			suggestions,
			writeTeleportFixFlag,
			writeBridgeRetriesFlag,
			func(s string) (string, error) {
				return writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
			},
			writeCharacterFixFlag,
			writeCharacterPatchRetriesFlag,
			knownChars,
			func(s string) (string, error) {
				return writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
			},
		)
		content = fixed
		log.Info("[Worker %d] Fix summary for %s: %s", workerID, chapter.ID, sum.String())

		if !agentApplied && !FinalChapterContentChanged(chapter, currentContent, content) {
			log.Info("[Worker %d] No content changes for chapter %s; skipping save, post-save check, and recap refresh", workerID, chapter.ID)
			return false, nil
		}

		// Save improved content
		savedContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if agentApplied && strings.TrimSpace(savedContent) == strings.TrimSpace(content) {
			log.Info("[Worker %d] Agent patch already saved chapter %s through tool patch chapter --apply", workerID, chapter.ID)
		} else if err := book.SaveFinalChapter(chapter, content); err != nil {
			log.Error("[Worker %d] Failed to save improved chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
			return false, nil
		}
		if useAgentSDK {
			postCheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
			if err != nil {
				log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
			} else {
				writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, postCheck)
				AppendAgentSDKPostSaveReviewSuggestions(review, postCheck)
			}
		}
		savedContent, readErr = book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if strings.TrimSpace(savedContent) != "" {
			content = savedContent
		}
		if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, useAgentSDK); err != nil {
			log.Warn("[Worker %d] Recap refresh failed for improved chapter %s: %v", workerID, chapter.ID, err)
		} else {
			log.Info("[Worker %d] Updated recap for improved chapter %s", workerID, chapter.ID)
		}

		log.Info("[Worker %d] Improved chapter saved: %s", workerID, chapter.ID)
		return true, nil
	})

	improvedCount := 0
	for _, ok := range improved {
		if ok {
			improvedCount++
		}
	}

	return improvedCount, errc.Err()
}

func fallbackDraftReviewForForcedImprove(chapter *models.Chapter) *agents.DraftReview {
	review := &agents.DraftReview{
		OverallScore:  7,
		NeedsRevision: true,
		Suggestions: []string{
			"User selected this chapter for focused improvement. Preserve the established outline facts and repair only issues found by deterministic chapter checks.",
		},
	}
	if chapter != nil {
		review.ChapterID = strings.TrimSpace(chapter.ID)
		review.ChapterTitle = strings.TrimSpace(chapter.Title)
	}
	return review
}

func repairAgentSDKGeneratedChapterPostSave(ctx context.Context, book *project.Book, log logger.LoggerInterface, workerID int, agent *agents.WriteAgent, chapter *models.Chapter, context *agents.ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentContent string, check *toolplane.ToolCheckResult, useAgentHistory bool) (string, *toolplane.ToolCheckResult, error) {
	if agent == nil {
		return "", nil, fmt.Errorf("write agent is nil")
	}
	if chapter == nil {
		return "", nil, fmt.Errorf("chapter is nil")
	}
	suggestions := "## Agent SDK Post-save Repair\n\n" +
		"The just-generated chapter was saved, but deterministic post-save checks still found blocking issues. " +
		"Repair only the returned check issues. Use validated `tool patch chapter` dry-run and then the matching `--apply --refresh-derived` command. " +
		"Keep the existing chapter shape and prose volume unless the check issue is explicitly about length.\n\n" +
		writepipe.FormatAgentSDKChapterCheckSuggestions(check)

	repaired, err := writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, true, true, useAgentHistory, 1)
	if err != nil {
		return "", nil, err
	}
	repaired, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, workerID, chapter, currentContent, repaired, true, true)
	if !agentApplied {
		if err := book.SaveFinalChapter(chapter, repaired); err != nil {
			return "", nil, fmt.Errorf("save repaired final chapter: %w", err)
		}
		saved, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			return "", nil, fmt.Errorf("read repaired final chapter: %w", readErr)
		}
		if strings.TrimSpace(saved) != "" {
			repaired = saved
		}
	}
	recheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
	if err != nil {
		return repaired, nil, err
	}
	return repaired, recheck, nil
}

func AppendAgentSDKPostSaveReviewSuggestions(review *agents.DraftReview, result *toolplane.ToolCheckResult) {
	if review == nil || result == nil {
		return
	}
	if result.Blocking {
		review.NeedsRevision = true
	}
	for _, issue := range result.Issues {
		text := formatAgentSDKPostSaveIssueSuggestion(result.Kind, result.Target, result.Scope, issue)
		if text != "" && !novelcont.ContainsStr(review.Suggestions, text) {
			review.Suggestions = append(review.Suggestions, text)
		}
	}
}

func formatAgentSDKPostSaveIssueSuggestion(kind, target, scope string, issue models.ReviewSuggestion) string {
	parts := []string{"Agent SDK post-save check still reports"}
	if issue.Priority != "" {
		parts = append(parts, "priority="+string(issue.Priority))
	}
	if strings.TrimSpace(issue.Category) != "" {
		parts = append(parts, "category="+strings.TrimSpace(issue.Category))
	}
	if strings.TrimSpace(issue.TargetID) != "" {
		parts = append(parts, "target="+strings.TrimSpace(issue.TargetID))
	}
	if strings.TrimSpace(issue.Issue) != "" {
		parts = append(parts, "issue="+strings.TrimSpace(issue.Issue))
	}
	if strings.TrimSpace(issue.Suggestion) != "" {
		parts = append(parts, "suggestion="+strings.TrimSpace(issue.Suggestion))
	}
	nav := toolplane.ToolIssueNavigation(kind, target, scope, issue.TargetID, issue, 0)
	if route, ok := nav["repair_route_query"].(string); ok && strings.TrimSpace(route) != "" {
		parts = append(parts, "repair_route_query="+route)
	}
	if repair, ok := nav["repair_context_query"].(string); ok && strings.TrimSpace(repair) != "" {
		parts = append(parts, "repair_context_query="+repair)
	}
	if check, ok := nav["focused_check_query"].(string); ok && strings.TrimSpace(check) != "" {
		parts = append(parts, "focused_check_query="+check)
	}
	return strings.Join(parts, "; ")
}

func GetWriteChaptersNeedingImprovement(review *agents.VolumeReview, outline *models.Outline, minScorePercent int) []*models.Chapter {
	if review == nil || outline == nil {
		return nil
	}
	chapters := make([]*models.Chapter, 0)
	for _, r := range review.Reviews {
		if r.NeedsRevision || DraftReviewScorePercent(r) < minScorePercent {
			if chapter := outline.GetChapterByID(r.ChapterID); chapter != nil {
				chapters = append(chapters, chapter)
			}
		}
	}
	return chapters
}

func DraftReviewScorePercent(review agents.DraftReview) int {
	if review.OverallScore <= 10 {
		return review.OverallScore * 10
	}
	return review.OverallScore
}

func ReviewScorePercentInt(score float64) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return int(score + 0.5)
}

// runWriteReview reviews final chapters and saves review results
func runWriteReview(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

	root := book.Dir()
	setupWriteRunLogging(root, "write review")
	if writeAgentSDKFlag {
		log.Info("Chapter review will use Agent SDK; Go still saves review JSON")
	}

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilder(root)

	// Get chapters to review based on flags
	chaptersToReview := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToReview) == 0 {
		return fmt.Errorf("no chapters found to review")
	}

	log.Info("Reviewing %d chapter(s)", len(chaptersToReview))

	// Group chapters by volume for saving reviews
	volumeReviews := make(map[string]*agents.VolumeReview)
	volumeContents := make(map[string]map[string]string)
	var errc writepipe.WriteErrorCollector

	// Review chapters concurrently
	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chaptersToReview) {
		concurrency = len(chaptersToReview)
	}

	type volumeReviewResult struct {
		volumeID    string
		volumeTitle string
		chapterID   string
		content     string
		review      agents.DraftReview
	}

	reviewResults, _ := workerpool.Map(ctx, concurrency, chaptersToReview, func(ctx context.Context, workerID int, chapter *models.Chapter) (*volumeReviewResult, error) {
		log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load final chapter content
		content, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return nil, nil
		}
		if content == "" {
			log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
			errc.Addf("%s: no final chapter content found for review", chapter.ID)
			return nil, nil
		}

		// Load context with recap for continuity checking
		chapterContext := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)

		// Calculate continuity snapshot for continuity checking
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Review chapter
		var reviewResult models.ReviewResult
		var err error
		if writeAgentSDKFlag {
			reviewResult, err = writeAgent.ReviewChapterWithAgentSDKFocus(ctx, chapter, chapterContext, continuity, content, targetWords, 1, writeFocusFlag)
		} else {
			reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		}
		if err != nil {
			log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: review failed: %w", chapter.ID, err)
			return nil, nil
		}
		if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
			log.Warn("[Worker %d] Failed to save full write review for %s: %v", workerID, chapter.ID, err)
		}

		// Convert suggestions to strings
		var suggestionStrings []string
		for _, s := range reviewResult.Suggestions {
			suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
		}

		// Convert to DraftReview
		draftReview := agents.DraftReview{
			ChapterID:     chapter.ID,
			ChapterTitle:  chapter.Title,
			OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: reviewResult.OverallScore < 70,
			Suggestions:   suggestionStrings,
		}

		// Get volume ID for this chapter
		volumeID := getVolumeIDFromChapter(chapter.ID)
		volumeTitle := ""
		if volume := outline.GetVolumeByID(volumeID); volume != nil {
			volumeTitle = volume.Title
		}

		log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
		return &volumeReviewResult{
			volumeID:    volumeID,
			volumeTitle: volumeTitle,
			chapterID:   chapter.ID,
			content:     content,
			review:      draftReview,
		}, nil
	})

	// Merge in input order so review lists are deterministic.
	for _, result := range reviewResults {
		if result == nil {
			continue
		}
		if _, exists := volumeContents[result.volumeID]; !exists {
			volumeContents[result.volumeID] = make(map[string]string)
		}
		volumeContents[result.volumeID][result.chapterID] = result.content
		if _, exists := volumeReviews[result.volumeID]; !exists {
			volumeReviews[result.volumeID] = &agents.VolumeReview{
				VolumeID:    result.volumeID,
				VolumeTitle: result.volumeTitle,
				Reviews:     make([]agents.DraftReview, 0),
			}
		}
		volumeReviews[result.volumeID].Reviews = append(volumeReviews[result.volumeID].Reviews, result.review)
	}

	// Save all volume reviews
	for volumeID, review := range volumeReviews {
		if existing, err := reviewstore.LoadVolumeReviewAt(book, volumeID); err == nil {
			review = MergeVolumeReviewByChapter(existing, review)
		} else if !os.IsNotExist(err) {
			log.Warn("Failed to load existing review for volume %s before merge: %v", volumeID, err)
		}
		if volume := outline.GetVolumeByID(volumeID); volume != nil {
			contentByID, err := loadFinalChapterContentsForVolume(book, volume)
			if err != nil {
				log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volumeID, err)
			} else {
				for chapterID, content := range volumeContents[volumeID] {
					contentByID[chapterID] = content
				}
				novelcont.ApplyHeuristicTransitionChecks(volume, contentByID, review)
				applyHumanizeChecksToVolume(book, volume, contentByID, review)
			}
		}
		if err := reviewstore.SaveVolumeReviewAt(book, review); err != nil {
			log.Error("Failed to save review for volume %s: %v", volumeID, err)
			errc.Addf("%s: save volume review failed: %w", volumeID, err)
			continue
		}
		log.Info("Review saved for volume %s: %d chapters reviewed", volumeID, len(review.Reviews))
	}

	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Review process complete")
	return nil
}

// getChaptersToReview returns chapters to review based on flags
func getChaptersToReview(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) []*models.Chapter {
	// If --all flag, return all chapters
	if allFlag {
		return project.AllChapters(outline)
	}

	// If chapter flag specified, return specific chapter(s)
	if chapterFlag != "" {
		chapters, err := project.SelectChapters(outline, chapterFlag, volumeFlag, partFlag, false)
		if err == nil {
			return chapters
		}
	}

	// If volume flag specified, return all chapters in volume
	if volumeFlag != "" {
		var chapters []*models.Chapter
		volumeID := writepipe.ResolveVolumeID(outline, volumeFlag, partFlag)
		for _, ch := range project.AllChapters(outline) {
			if strings.HasPrefix(ch.ID, volumeID) {
				chapters = append(chapters, ch)
			}
		}
		return chapters
	}

	// Default: return all chapters
	return project.AllChapters(outline)
}

// getVolumeIDFromChapter extracts volume ID from chapter ID
// e.g., "P1-V1-C1" -> "P1-V1"
func getVolumeIDFromChapter(chapterID string) string {
	parts := strings.Split(chapterID, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "-")
	}
	return ""
}

// reviewVolumeWithWriteAgent reviews all chapters in a volume using the write agent
func reviewVolumeWithWriteAgent(ctx context.Context, book *project.Book, writeAgent *agents.WriteAgent, continuityBuilder *logic.ChapterContinuityBuilder, volume *models.Volume, outline *models.Outline, targetWords int, concurrency int) error {
	log := logger.GetLogger()

	// Get all chapters in this volume
	var chapters []*models.Chapter
	for _, ch := range project.AllChapters(outline) {
		if strings.HasPrefix(ch.ID, volume.ID) {
			chapters = append(chapters, ch)
		}
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found in volume %s", volume.ID)
	}

	// Create volume review
	volumeReview := &agents.VolumeReview{
		VolumeID:    volume.ID,
		VolumeTitle: volume.Title,
		Reviews:     make([]agents.DraftReview, 0),
	}
	volumeContents := make(map[string]string)
	var errc writepipe.WriteErrorCollector

	// Review chapters concurrently
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	type chapterReviewResult struct {
		chapterID string
		content   string
		review    agents.DraftReview
	}

	reviewResults, _ := workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (*chapterReviewResult, error) {
		log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load final chapter content
		content, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return nil, nil
		}
		if content == "" {
			log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
			errc.Addf("%s: no final chapter content found for review", chapter.ID)
			return nil, nil
		}

		// Load context with recap for continuity checking
		chapterContext := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)

		// Calculate continuity snapshot for continuity checking
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Review chapter
		reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		if err != nil {
			log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: review failed: %w", chapter.ID, err)
			return nil, nil
		}
		if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
			log.Warn("[Worker %d] Failed to save full write review for %s: %v", workerID, chapter.ID, err)
		}

		// Convert suggestions to strings
		var suggestionStrings []string
		for _, s := range reviewResult.Suggestions {
			suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
		}

		// Convert to DraftReview
		draftReview := agents.DraftReview{
			ChapterID:     chapter.ID,
			ChapterTitle:  chapter.Title,
			OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: reviewResult.OverallScore < 70,
			Suggestions:   suggestionStrings,
		}

		log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
		return &chapterReviewResult{chapterID: chapter.ID, content: content, review: draftReview}, nil
	})

	// Merge in input order so the saved review list is deterministic.
	for _, result := range reviewResults {
		if result == nil {
			continue
		}
		volumeContents[result.chapterID] = result.content
		volumeReview.Reviews = append(volumeReview.Reviews, result.review)
	}

	// Save volume review
	novelcont.ApplyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
	applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
	if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
		return fmt.Errorf("failed to save review for volume %s: %w", volume.ID, err)
	}
	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Review saved for volume %s: %d chapters reviewed", volume.ID, len(volumeReview.Reviews))
	return nil
}

// runWritePipeline runs the complete writing pipeline: gen -> review -> improve -> recap
func runWritePipeline(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := validateWriteMinScoreFlag(); err != nil {
		return err
	}
	if err := ValidateWriteAgentApplyOption(writeAgentSDKFlag, writeAgentApplyFlag); err != nil {
		return err
	}
	if err := ValidateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

	root := book.Dir()
	setupWriteRunLogging(root, "write pipeline")

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilder(root)

	// Get chapters to process
	chaptersToProcess := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToProcess) == 0 {
		return fmt.Errorf("no chapters found to process")
	}

	log.Info("=== WRITE PIPELINE ===")
	log.Info("Processing %d chapter(s)", len(chaptersToProcess))
	log.Info("RPG DSL export: enabled=%t batch_size=%d output=%s", writeEmitRPGDSLFlag, writeRPGBatchSizeFlag, store.At(root).RPGChaptersDSL())
	minScorePercent := writeMinScorePercent()
	log.Info("Minimum acceptable score: %.0f/100", minScorePercent)
	if WriteBestOfFlag > 1 {
		log.Info("Best-of mode enabled: %d independent copies per chapter, scored and best saved", WriteBestOfFlag)
	}
	if writeAgentSDKFlag {
		log.Info("Chapter generation/review/improvement will use Agent SDK; Go still validates and saves final markdown and review JSON")
		if writeAgentHistoryFlag {
			log.Info("Agent history enabled: SDK must inspect queryable logs before writing/improving")
		}
	}
	if writeAgentApplyFlag {
		log.Info("Agent apply enabled: SDK may write final markdown through validated chapter patch tools")
	}
	usePipelineRecapAgentSDK := writeRecapAgentSDKFlag || writeAgentSDKFlag
	if usePipelineRecapAgentSDK {
		log.Info("Final recap extraction will use Agent SDK; Go still validates and saves recap JSON")
	}

	// Create recap agent for all chapters
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)
	// Create lightweight best-of scorer (used only when --best-of is enabled)
	scorer := agents.NewChapterScorerAgent(client, cfg, &config.LLM)
	scorer.SetLanguage(config.Language)

	// RPG DSL issues are refreshed after chapter content changes. Running this
	// before generation would analyze stale or missing chapter files.
	var rpgDSLIssues []dsl.SimulationIssue
	knownChars := project.KnownCharacterNames(outline)
	var errc writepipe.WriteErrorCollector

	// Process each chapter completely before moving to the next
	for chapterIdx, chapter := range chaptersToProcess {
		log.Info("\n%s", strings.Repeat("=", 60))
		log.Info("Processing Chapter %d/%d: %s - %s", chapterIdx+1, len(chaptersToProcess), chapter.ID, chapter.Title)
		log.Info("%s", strings.Repeat("=", 60))

		// Step 1: Generate chapter (if not exists)
		log.Info("\n[Step 1/4] Generating chapter...")
		content, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("Failed to read chapter %s: %v", chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			continue
		}
		if content == "" {
			log.Info("Generating chapter: %s - %s", chapter.ID, chapter.Title)
			chapterContext := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			generatedContent, err := GenerateChapterContentForRun(ctx, writeAgent, scorer, chapter, chapterContext, continuity, targetWords, WriteBestOfFlag, writeAgentSDKFlag, writeAgentHistoryFlag)
			if err != nil {
				log.Error("Failed to generate chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: generate failed: %w", chapter.ID, err)
				continue
			}
			if err := book.SaveFinalChapter(chapter, generatedContent); err != nil {
				log.Error("Failed to save chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save generated chapter failed: %w", chapter.ID, err)
				continue
			}
			content, readErr = book.LoadFinalChapterContent(chapter)
			if readErr != nil {
				log.Error("Failed to read back chapter %s: %v", chapter.ID, readErr)
				errc.Addf("%s: read back final chapter failed: %w", chapter.ID, readErr)
				continue
			}
			if strings.TrimSpace(content) == "" {
				content = generatedContent
			}
			log.Info("Generated chapter output: %s", finalChapterPath(root, chapter))
		} else {
			log.Info("Chapter %s already exists, using: %s", chapter.ID, finalChapterPath(root, chapter))
		}

		// Step 2: Review chapter
		log.Info("\n[Step 2/4] Reviewing chapter...")
		chapterContext := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)
		continuity := continuityBuilder.BuildBefore(outline, chapter)
		var reviewResult models.ReviewResult
		var err error
		if writeAgentSDKFlag {
			reviewResult, err = writeAgent.ReviewChapterWithAgentSDK(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		} else {
			reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		}
		if err != nil {
			log.Error("Failed to review chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: review failed: %w", chapter.ID, err)
			continue
		}
		if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
			log.Warn("Failed to save full write review for %s: %v", chapter.ID, err)
		}

		// Save review
		var suggestionStrings []string
		for _, s := range reviewResult.Suggestions {
			suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
		}
		draftReview := agents.DraftReview{
			ChapterID:     chapter.ID,
			ChapterTitle:  chapter.Title,
			OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: reviewResult.OverallScore < minScorePercent,
			Suggestions:   suggestionStrings,
		}
		volumeID := getVolumeIDFromChapter(chapter.ID)
		volume := outline.GetVolumeByID(volumeID)
		var volumeReview *agents.VolumeReview
		reviewPath := store.At(root).VolumeReviewFile(volumeID)
		var existingReview agents.VolumeReview
		if err := store.At(root).ReadJSON(reviewPath, &existingReview); err == nil {
			volumeReview = &existingReview
		}
		if volumeReview == nil {
			volumeTitle := ""
			if volume != nil {
				volumeTitle = volume.Title
			}
			volumeReview = &agents.VolumeReview{
				VolumeID:    volumeID,
				VolumeTitle: volumeTitle,
				Reviews:     make([]agents.DraftReview, 0),
			}
		}
		found := false
		for i := range volumeReview.Reviews {
			if volumeReview.Reviews[i].ChapterID == chapter.ID {
				volumeReview.Reviews[i] = draftReview
				found = true
				break
			}
		}
		if !found {
			volumeReview.Reviews = append(volumeReview.Reviews, draftReview)
		}
		if volume != nil {
			volumeContents, err := loadFinalChapterContentsForVolume(book, volume)
			if err != nil {
				log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
			} else {
				novelcont.ApplyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
				applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
				if updated := findDraftReview(volumeReview, chapter.ID); updated != nil {
					draftReview = *updated
				}
			}
		}
		if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
			log.Error("Failed to save review for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: save volume review failed: %w", chapter.ID, err)
		}
		log.Info("Review output: %s", reviewPath)
		log.Info("Reviewed chapter %s: score %.1f", chapter.ID, reviewResult.OverallScore)

		if writeEmitRPGDSLFlag && writeMaxRoundsFlag > 0 {
			result, err := refreshWriteRPGDSL(ctx, root, config, cfg, setup, client, writeRPGBatchSizeFlag, "before improvement")
			if err != nil {
				log.Error("Failed to refresh RPG DSL before improving chapter %s: %v", chapter.ID, err)
			} else {
				rpgDSLIssues = result.Issues
			}
		}

		// Step 3: Improve chapter (iterative)
		log.Info("\n[Step 3/4] Improving chapter...")
		currentContent := content
		chapterReview := findDraftReview(volumeReview, chapter.ID)
		if chapterReview == nil {
			chapterReview = &draftReview
		}
		for round := 1; round <= writeMaxRoundsFlag; round++ {
			// Check for RPG DSL issues for this chapter
			chapterRPGIssues := writersdl.GetChapterSpecificSuggestions(rpgDSLIssues, chapter.ID)
			hasRPGIssues := chapterRPGIssues != ""

			if len(chapterReview.Suggestions) == 0 && !chapterReview.NeedsRevision && !hasRPGIssues {
				log.Info("Chapter %s: no suggestions and no revision needed, skipping improvement", chapter.ID)
				break
			}

			log.Info("Improvement round %d/%d (score: %d, RPG issues: %v)", round, writeMaxRoundsFlag, chapterReview.OverallScore, hasRPGIssues)
			suggestions := reviewstore.BuildImprovementSuggestions(chapterReview)

			// Append RPG DSL issues as suggestions if any
			if hasRPGIssues {
				suggestions += "\n\n" + chapterRPGIssues
			}
			if writeAgentSDKFlag {
				if checkSuggestions := writepipe.BuildAgentSDKChapterCheckSuggestions(book, chapter, targetWords); checkSuggestions != "" {
					suggestions += "\n\n" + checkSuggestions
				}
			}
			chapterContext := writepipe.LoadChapterContext(book, outline, chapter, writeContextFlag)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			improvedContent, err := writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, currentContent, suggestions, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
			if err != nil {
				log.Error("Failed to improve chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: improve round %d failed: %w", chapter.ID, round, err)
				break
			}
			improvedContent, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, 0, chapter, currentContent, improvedContent, writeAgentSDKFlag, writeAgentApplyFlag)

			fixedContent, sum := novelcont.ApplyImproveFixesWrite(
				log,
				0,
				chapter,
				outline,
				improvedContent,
				suggestions,
				writeTeleportFixFlag,
				writeBridgeRetriesFlag,
				func(s string) (string, error) {
					return writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
				writeCharacterFixFlag,
				writeCharacterPatchRetriesFlag,
				knownChars,
				func(s string) (string, error) {
					return writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
			)
			improvedContent = fixedContent
			log.Info("Fix summary for %s: %s", chapter.ID, sum.String())

			if !agentApplied && !FinalChapterContentChanged(chapter, currentContent, improvedContent) {
				log.Info("No content changes for chapter %s; skipping save and post-save check", chapter.ID)
				break
			}

			savedContent, readErr := book.LoadFinalChapterContent(chapter)
			if readErr != nil {
				log.Error("Failed to read chapter %s: %v", chapter.ID, readErr)
				errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
				break
			}
			if agentApplied && strings.TrimSpace(savedContent) == strings.TrimSpace(improvedContent) {
				log.Info("Agent patch already saved chapter %s through tool patch chapter --apply", chapter.ID)
			} else if err := book.SaveFinalChapter(chapter, improvedContent); err != nil {
				log.Error("Failed to save improved chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
				break
			}
			if writeAgentSDKFlag {
				postCheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
				if err != nil {
					log.Warn("Agent SDK post-save check failed for %s: %v", chapter.ID, err)
				} else {
					writepipe.LogAgentSDKChapterPostSaveCheck(log, 0, chapter.ID, postCheck)
					AppendAgentSDKPostSaveReviewSuggestions(chapterReview, postCheck)
					if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
						log.Warn("Failed to save Agent SDK post-save review suggestions for %s: %v", chapter.ID, err)
					}
				}
			}
			currentContent = improvedContent
			log.Info("Improved chapter output: %s", finalChapterPath(root, chapter))

			// Re-review after improvement (if not last round)
			if round < writeMaxRoundsFlag {
				log.Info("Re-reviewing chapter %s after improvement...", chapter.ID)
				if writeAgentSDKFlag {
					reviewResult, err = writeAgent.ReviewChapterWithAgentSDK(ctx, chapter, chapterContext, continuity, currentContent, targetWords, 1)
				} else {
					reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, currentContent, targetWords, 1)
				}
				if err != nil {
					log.Error("Failed to re-review chapter %s: %v", chapter.ID, err)
					errc.Addf("%s: re-review after improvement failed: %w", chapter.ID, err)
					break
				}
				if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
					log.Warn("Failed to save full write review for %s: %v", chapter.ID, err)
				}
				var suggestionStrings []string
				for _, s := range reviewResult.Suggestions {
					suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
				}
				chapterReview.OverallScore = ReviewScorePercentInt(reviewResult.OverallScore)
				chapterReview.NeedsRevision = reviewResult.OverallScore < minScorePercent
				chapterReview.Suggestions = suggestionStrings
				if volume != nil {
					volumeContents, err := loadFinalChapterContentsForVolume(book, volume)
					if err != nil {
						log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
					} else {
						novelcont.ApplyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
						applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
						if updated := findDraftReview(volumeReview, chapter.ID); updated != nil {
							chapterReview = updated
						}
					}
				}
				// Save updated review
				if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
					log.Error("Failed to save updated review for chapter %s: %v", chapter.ID, err)
					errc.Addf("%s: save updated review failed: %w", chapter.ID, err)
				}
				// Check if quality threshold met
				if reviewResult.OverallScore >= minScorePercent {
					log.Info("Quality threshold met (%.1f >= %.0f), stopping improvement", reviewResult.OverallScore, minScorePercent)
					break
				}
			}
		}

		// Step 4: Generate recap
		log.Info("\n[Step 4/4] Generating recap...")
		if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, currentContent, 0, usePipelineRecapAgentSDK); err != nil {
			log.Error("Failed to generate/save recap for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: final recap failed: %w", chapter.ID, err)
		} else {
			log.Info("Recap output: %s", store.At(root).RecapFile(chapter.ID))
		}

		if writeEmitRPGDSLFlag {
			result, err := refreshWriteRPGDSL(ctx, root, config, cfg, setup, client, writeRPGBatchSizeFlag, "after final recap")
			if err != nil {
				log.Error("Failed to refresh final RPG DSL after chapter %s recap: %v", chapter.ID, err)
			} else {
				rpgDSLIssues = result.Issues
			}
		}

		log.Info("\nCompleted chapter: %s", chapter.ID)
	}

	if err := errc.Err(); err != nil {
		log.Info("\n=== PIPELINE COMPLETE WITH ERRORS ===")
		return err
	}

	log.Info("\n=== PIPELINE COMPLETE ===")
	log.Info("All phases completed successfully!")
	return nil
}

func refreshWriteRPGDSL(
	ctx context.Context,
	root string,
	config *models.ProjectConfig,
	cfg *llm.Config,
	setup *models.StorySetup,
	client llm.Client,
	batchSize int,
	reason string,
) (writersdl.WriteRPGDSLResult, error) {
	log := logger.GetLogger()
	log.Info("\n[RPG DSL] Refreshing chapter DSL (%s)...", reason)
	result, err := writersdl.EmitChapterRPGDSLForProject(ctx, root, filepath.Base(root), config, cfg, setup, client, batchSize)
	if err != nil {
		return result, err
	}
	log.Info("[RPG DSL] Output: %s", result.DSLPath)
	log.Info("[RPG DSL] Batches: %d reused, %d generated", result.ReusedBatches, result.GeneratedBatches)
	log.Info("[RPG DSL] Issues detected: %d total, %d important", len(result.Issues), len(writersdl.FilterImportantIssues(result.Issues)))
	return result, nil
}
