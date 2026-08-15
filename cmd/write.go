package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/logic/style"
	"novelgen/internal/models"
	"novelgen/internal/rpg/dsl"

	"github.com/spf13/cobra"
)

type writeErrorCollector struct {
	mu   sync.Mutex
	errs []error
}

func (c *writeErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = append(c.errs, err)
}

func (c *writeErrorCollector) Addf(format string, args ...interface{}) {
	c.Add(fmt.Errorf(format, args...))
}

func (c *writeErrorCollector) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.errs) == 0 {
		return nil
	}
	return errors.Join(c.errs...)
}

func validateWriteMinScoreFlag() error {
	if writeMinScoreFlag < 0 || writeMinScoreFlag > 100 {
		return fmt.Errorf("--min-score must be between 0 and 100, got %d", writeMinScoreFlag)
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

var writeGenCmd = &cobra.Command{
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
  novelgen write gen --chapter 5 --context 3`,
	RunE: runWriteGen,
}

var writeImproveCmd = &cobra.Command{
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

var writeReviewCmd = &cobra.Command{
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

var writePipelineCmd = &cobra.Command{
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
	writeCmd.AddCommand(writeGenCmd)
	writeCmd.AddCommand(writeImproveCmd)
	writeCmd.AddCommand(writeReviewCmd)
	writeCmd.AddCommand(writePipelineCmd)

	writeGenCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'P1-V1-C1')")
	writeGenCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume number for context (e.g., '1', 'P1-V1')")
	writeGenCmd.Flags().StringVar(&writePartFlag, "part", "", "Part number for context (e.g., '1', 'P1')")
	writeGenCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	writeGenCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Generate content for all chapters")
	writeGenCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	writeGenCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")
	writeGenCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter generation")
	writeGenCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before writing")
	writeGenCmd.Flags().BoolVar(&writeRecapAgentSDKFlag, "recap-agent-sdk", false, "Use Claude Agent SDK workflow for automatic recap extraction")

	writeImproveCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	writeImproveCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	writeImproveCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	writeImproveCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the improved chapter")
	writeImproveCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	writeImproveCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Minimum acceptable score (0-100)")
	writeImproveCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")
	writeImproveCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	writeImproveCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	writeImproveCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	writeImproveCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	writeImproveCmd.Flags().StringVar(&writePromptFlag, "prompt", "", "Additional user instructions for improvement")
	writeImproveCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks and humanization suggestions")
	writeImproveCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before improvement is suggested (0-100)")
	writeImproveCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter improvement")
	writeImproveCmd.Flags().BoolVar(&writeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write final chapter markdown through validated patch tools")
	writeImproveCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before improving")

	writeReviewCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to review (e.g., '1' or 'P1-V1-C1')")
	writeReviewCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to review (e.g., '1', 'P1-V1')")
	writeReviewCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to review (e.g., '1', 'P1')")
	writeReviewCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Review all chapters")
	writeReviewCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count used when reviewing chapter length")
	writeReviewCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent reviews")
	writeReviewCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks in write review")
	writeReviewCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before review marks style issues (0-100)")
	writeReviewCmd.Flags().StringVar(&writeFocusFlag, "focus", "", "Review focus (comma-separated, e.g. deai,protagonist; 'all' for all). Uses the same AI review focuses as compose review (see --focus list). Empty = generic chapter review.")
	writeReviewCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter review")

	writePipelineCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to process (e.g., '1' or 'P1-V1-C1')")
	writePipelineCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to process (e.g., '1', 'P1-V1')")
	writePipelineCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to process (e.g., '1', 'P1')")
	writePipelineCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Process all chapters")
	writePipelineCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	writePipelineCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 2, "Maximum improvement rounds")
	writePipelineCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 70, "Minimum acceptable score (0-100)")
	writePipelineCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	writePipelineCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent operations")
	_ = writePipelineCmd.Flags().MarkHidden("concurrency")
	writePipelineCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	writePipelineCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	writePipelineCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	writePipelineCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	writePipelineCmd.Flags().BoolVar(&writeEmitRPGDSLFlag, "emit-rpg-dsl", true, "Update story/rpg/04_chapters.rpg from chapter markdown plus optional recaps")
	writePipelineCmd.Flags().IntVar(&writeRPGBatchSizeFlag, "rpg-batch-size", 10, "Chapter markdown batch size for AI -> RPG DSL conversion")
	writePipelineCmd.Flags().BoolVar(&writeHumanizeFlag, "humanize", true, "Enable deterministic AI-flavor checks and humanization suggestions")
	writePipelineCmd.Flags().IntVar(&writeHumanizeThresholdFlag, "humanize-threshold", 75, "Minimum humanization score before improvement is suggested (0-100)")
	writePipelineCmd.Flags().BoolVar(&writeAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for chapter generation, review, and improvement")
	writePipelineCmd.Flags().BoolVar(&writeAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent write final chapter markdown through validated patch tools")
	writePipelineCmd.Flags().BoolVar(&writeAgentHistoryFlag, "agent-history", false, "With --agent-sdk, let the agent consult copied prompt/response/agent-live logs before writing or improving")
	writePipelineCmd.Flags().BoolVar(&writeRecapAgentSDKFlag, "recap-agent-sdk", false, "Use Claude Agent SDK workflow for automatic recap extraction")

	// Register write command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return writeCmd
	})
}

func runWriteGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := validateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
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

	// Get project root for continuity builder
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
	setupWriteRunLogging(root, "write gen")

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilder(root)
	// Create recap agent + store (auto-persist recaps for continuity)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)
	var errc writeErrorCollector

	// Get list of chapters to generate
	chapters, err := getChaptersToGenerate(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating final content for %d chapter(s) with concurrency %d", len(chapters), writeConcurrencyFlag)
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

	// Create work channel and wait group
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Generating content for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load context drafts (previous and next chapters)
				context := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate chapter continuity
				continuity := continuityBuilder.BuildBefore(outline, chapter)

				// Generate final content
				var content string
				var err error
				if writeAgentSDKFlag {
					content, err = agent.GenerateChapterWithAgentSDK(ctx, chapter, context, continuity, targetWords, writeAgentHistoryFlag)
				} else {
					content, err = agent.GenerateChapter(ctx, chapter, context, continuity, targetWords)
				}
				if err != nil {
					log.Error("Failed to generate content for chapter %s: %v", chapter.ID, err)
					errc.Addf("%s: generate failed: %w", chapter.ID, err)
					continue
				}

				// Save final content
				if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("Failed to save content for chapter %s: %v", chapter.ID, err)
					errc.Addf("%s: save final chapter failed: %w", chapter.ID, err)
					continue
				}
				if savedContent := loadFinalChapterContent(chapter); strings.TrimSpace(savedContent) != "" {
					content = savedContent
				}
				if writeAgentSDKFlag {
					if result, err := runAgentSDKChapterPostSaveCheck(ctx, chapter, targetWords); err != nil {
						log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
					} else {
						logAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, result)
						if result.Blocking {
							log.Warn("[Worker %d] Agent SDK generated chapter %s still has blocking post-save issues; attempting one validated agent repair", workerID, chapter.ID)
							repaired, repairedCheck, repairErr := repairAgentSDKGeneratedChapterPostSave(ctx, log, workerID, agent, chapter, context, continuity, targetWords, content, result, writeAgentHistoryFlag)
							if repairErr != nil {
								log.Error("[Worker %d] Agent SDK post-save repair failed for %s: %v", workerID, chapter.ID, repairErr)
								errc.Addf("%s: post-save repair failed: %w", chapter.ID, repairErr)
								continue
							}
							content = repaired
							logAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, repairedCheck)
							if repairedCheck != nil && repairedCheck.Blocking {
								errc.Addf("%s: post-save repair still has blocking issues", chapter.ID)
								continue
							}
						}
					}
				}

				// Auto-extract + persist recap for this final chapter (best-effort)
				if err := extractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, writeRecapAgentSDKFlag); err != nil {
					log.Warn("[Worker %d] Recap gate failed for %s: %v", workerID, chapter.ID, err)
				}

				log.Info("[Worker %d] Content saved for chapter %s: %d narrative units", workerID, chapter.ID, chapterNarrativeUnitsForLog(chapter, content))
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Chapter writing complete")
	return nil
}

// loadChapterContent loads chapter content, preferring final version over draft
func loadChapterContent(chapterID string) string {
	// Try to load final chapter first
	root, err := findProjectRoot()
	if err != nil {
		return loadDraftContent(chapterID)
	}

	// Check for final chapter. Prefer full chapter IDs, but keep the old
	// numeric filename as a compatibility fallback for existing projects.
	for _, finalPath := range []string{
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterID)),
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", extractChapterNumber(chapterID))),
	} {
		if data, err := os.ReadFile(finalPath); err == nil {
			return string(data)
		}
	}

	// Fallback to draft
	return loadDraftContent(chapterID)
}

// loadChapterContext loads surrounding chapter content for context
// Prefers final chapters over drafts when available
func loadChapterContext(outline *models.Outline, targetChapter *models.Chapter, contextCount int) *agents.ChapterContext {
	context := &agents.ChapterContext{
		Current:  targetChapter,
		Previous: make([]*agents.ContextChapter, 0),
		Next:     make([]*agents.ContextChapter, 0),
		Recap:    "",
	}
	if _, _, _, organizations, err := loadAllElements(); err == nil {
		context.Craft = buildOrganizationWriteContext(targetChapter, organizations)
	}

	allChapters := getAllChapters(outline)

	// Find target chapter index
	targetIndex := -1
	for i, ch := range allChapters {
		if ch.ID == targetChapter.ID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return context
	}

	if recapJSON := loadPreviousRecapJSON(outline, targetChapter); strings.TrimSpace(recapJSON) != "" {
		context.Recap = recapJSON
	} else if draftRecap := loadPreviousDraftRecap(outline, targetChapter); strings.TrimSpace(draftRecap) != "" {
		context.Recap = draftRecap
	}

	// Load previous chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex - i
		if idx >= 0 {
			ch := allChapters[idx]
			content := loadChapterContent(ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				// Ensure there's at least an offline recap persisted for the previous
				// chapter so later steps (e.g., transition checks / future drafts) can
				// use it even when generation is out-of-order.
				if i == 1 && content != "" {
					persistOfflineRecapIfMissing(ch, content)
				}

				// Note: Recap extraction is handled separately via recap command
				context.Previous = append([]*agents.ContextChapter{{
					Chapter: ch,
					Content: content,
				}}, context.Previous...)
			}
		}
	}

	// Load next chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex + i
		if idx < len(allChapters) {
			ch := allChapters[idx]
			content := loadChapterContent(ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				context.Next = append(context.Next, &agents.ContextChapter{
					Chapter: ch,
					Content: content,
				})
			}
		}
	}

	return context
}

func buildOrganizationWriteContext(chapter *models.Chapter, organizations map[string]*models.Organization) string {
	if chapter == nil || len(organizations) == 0 {
		return ""
	}

	haystackParts := []string{
		chapter.ID,
		chapter.Title,
		chapter.Summary,
		chapter.Location,
		chapter.StateAnchor.Location,
		strings.Join(chapter.Characters, " "),
		strings.Join(chapter.GetBeats(), " "),
		chapter.Conflict,
		chapter.StateChange,
	}
	for _, scene := range chapter.Scenes {
		haystackParts = append(haystackParts, scene.POV, scene.Goal, scene.Location, strings.Join(scene.Characters, " "), strings.Join(scene.Beats, " "))
	}
	for _, enemy := range chapter.Enemies {
		haystackParts = append(haystackParts, enemy.Name, enemy.Faction, enemy.Tier, enemy.BossID, enemy.Status, enemy.Context)
	}
	for _, entry := range chapter.ResourceLedger {
		haystackParts = append(haystackParts, entry.Item, entry.Reason)
	}
	for _, advance := range chapter.StorylineAdvances {
		haystackParts = append(haystackParts, advance.StorylineName, advance.Stage, advance.Change, advance.Consequence, advance.Pressure)
	}
	for _, planted := range chapter.Mysteries.Planted {
		haystackParts = append(haystackParts, planted.ID, planted.Clue)
	}
	for _, resolved := range chapter.Mysteries.Resolved {
		haystackParts = append(haystackParts, resolved.ID, resolved.Resolution)
	}
	for _, event := range chapter.Events {
		haystackParts = append(haystackParts,
			event.GetActor(),
			event.GetAction(),
			event.GetTarget(),
			event.GetTargetType(),
			strings.Join(event.Characters, " "),
			event.Context,
			event.Details,
			event.Result,
		)
	}
	haystack := strings.ToLower(strings.Join(haystackParts, " "))

	type organizationMatch struct {
		id    string
		org   *models.Organization
		score int
	}
	matches := make([]organizationMatch, 0, len(organizations))
	for id, org := range organizations {
		if org == nil {
			continue
		}
		score := 0
		candidates := []string{id, org.Name, org.Headquarters, org.Leadership}
		candidates = append(candidates, org.Members...)
		candidates = append(candidates, org.Allies...)
		candidates = append(candidates, org.Enemies...)
		candidates = append(candidates, org.DSLTags...)
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate != "" && strings.Contains(haystack, strings.ToLower(candidate)) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		matches = append(matches, organizationMatch{id: id, org: org, score: score})
	}
	if len(matches) == 0 {
		return ""
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].id < matches[j].id
	})

	if len(matches) > 8 {
		matches = matches[:8]
	}
	var sb strings.Builder
	sb.WriteString("ORGANIZATIONS:\n")
	for _, match := range matches {
		org := match.org
		name := strings.TrimSpace(org.Name)
		if name == "" {
			name = match.id
		}
		sb.WriteString(fmt.Sprintf("- %s", name))
		if strings.TrimSpace(org.Type) != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.TrimSpace(org.Type)))
		}
		sb.WriteString(": ")
		parts := []string{}
		appendPart := func(label, value string) {
			value = strings.TrimSpace(value)
			if value == "" {
				return
			}
			if label == "" {
				parts = append(parts, value)
			} else {
				parts = append(parts, label+"="+value)
			}
		}
		appendPart("", org.Description)
		appendPart("leadership", org.Leadership)
		appendPart("headquarters", org.Headquarters)
		appendPart("goals", strings.Join(org.Goals, " | "))
		appendPart("resources", strings.Join(org.Resources, " | "))
		appendPart("allies", strings.Join(org.Allies, " | "))
		appendPart("enemies", strings.Join(org.Enemies, " | "))
		appendPart("", org.Significance)
		sb.WriteString(strings.Join(parts, "; "))
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

// buildChapterContentFromOutline builds chapter content from outline data
// Used as fallback when no draft or final content exists
func buildChapterContentFromOutline(chapter *models.Chapter) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chapter: %s\n", chapter.ID))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", chapter.Summary))
	if len(chapter.GetBeats()) > 0 {
		sb.WriteString("Beats:\n")
		for i, beat := range chapter.GetBeats() {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, beat))
		}
	}
	return sb.String()
}

// saveFinalChapter saves the generated final chapter content
func saveFinalChapter(chapter *models.Chapter, content string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		return fmt.Errorf("failed to create chapters directory: %w", err)
	}

	// Format: chapter-<full-id>.md, e.g. chapter-P1-V1-C1.md.
	filename := filepath.Join(chaptersDir, fmt.Sprintf("chapter-%s.md", chapter.ID))

	content = normalizeChapterPatchContent(chapter, content)
	return os.WriteFile(filename, []byte(content), 0644)
}

func finalChapterContentChanged(chapter *models.Chapter, before, after string) bool {
	before = strings.TrimSpace(normalizeChapterPatchContent(chapter, before))
	after = strings.TrimSpace(normalizeChapterPatchContent(chapter, after))
	return before != after
}

func chapterNarrativeUnitsForLog(chapter *models.Chapter, content string) int {
	title := ""
	if chapter != nil {
		title = chapter.Title
	}
	return toolNarrativeUnitCount(stripToolChapterMarkdownTitle(content, title))
}

func saveWriteReviewResult(chapter *models.Chapter, review models.ReviewResult) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}

	reviewDir := filepath.Join(root, "story", "reviews")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		return fmt.Errorf("failed to create reviews directory: %w", err)
	}

	reviewPath := filepath.Join(reviewDir, chapter.ID+"_write_review.json")
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal write review: %w", err)
	}
	return os.WriteFile(reviewPath, data, 0644)
}

type recapExtractor interface {
	Extract(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error)
	ExtractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error)
	ExtractWithAgentSDK(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error)
	ExtractWithFeedbackAgentSDK(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error)
}

func extractAndSaveRecapWithGate(ctx context.Context, recapAgent recapExtractor, recapStore *recap.Store, chapter *models.Chapter, content string, workerID int, useAgentSDK bool) error {
	if recapAgent == nil || recapStore == nil {
		return fmt.Errorf("recap dependencies are not initialized")
	}
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}

	recapData, err := extractRecap(ctx, recapAgent, chapter.ID, chapter.Title, content, useAgentSDK)
	if err != nil {
		return fmt.Errorf("extract recap: %w", err)
	}

	log := logger.GetLogger()
	if ok, reasons := recap.ValidateMinimal(recapData); !ok {
		log.Warn("[Worker %d] Recap minimal validation failed for %s: %v", workerID, chapter.ID, reasons)

		fb := recapGateFeedback(reasons, recapData)
		recap2, err := extractRecapWithFeedback(ctx, recapAgent, chapter.ID, chapter.Title, content, fb, useAgentSDK)
		if err != nil {
			return fmt.Errorf("retry recap extraction: %w", err)
		}
		if okR, reasonsR := recap.ValidateMinimal(recap2); !okR {
			return fmt.Errorf("recap retry still failed minimal validation: %s", strings.Join(reasonsR, "; "))
		}
		recapData = recap2
	}

	if ok, reasons := recap.ValidateConsistency(recapData); !ok {
		log.Warn("[Worker %d] Recap consistency validation warning for %s: %v", workerID, chapter.ID, reasons)
	}

	if err := recapStore.Save(recapData); err != nil {
		return fmt.Errorf("save recap: %w", err)
	}
	return nil
}

func extractRecap(ctx context.Context, recapAgent recapExtractor, chapterID, title, content string, useAgentSDK bool) (*models.ChapterRecap, error) {
	if useAgentSDK {
		return recapAgent.ExtractWithAgentSDK(ctx, chapterID, title, content)
	}
	return recapAgent.Extract(ctx, chapterID, title, content)
}

func extractRecapWithFeedback(ctx context.Context, recapAgent recapExtractor, chapterID, title, content, feedback string, useAgentSDK bool) (*models.ChapterRecap, error) {
	if useAgentSDK {
		return recapAgent.ExtractWithFeedbackAgentSDK(ctx, chapterID, title, content, feedback)
	}
	return recapAgent.ExtractWithFeedback(ctx, chapterID, title, content, feedback)
}

func loadFinalChapterContentsForVolume(volume *models.Volume) map[string]string {
	contents := make(map[string]string)
	if volume == nil {
		return contents
	}
	for i := range volume.Chapters {
		ch := &volume.Chapters[i]
		if content := loadFinalChapterContent(ch); strings.TrimSpace(content) != "" {
			contents[ch.ID] = content
		}
	}
	return contents
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

func mergeVolumeReviewByChapter(existing, incoming *agents.VolumeReview) *agents.VolumeReview {
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
		if !containsStr(review.PacingReview.Issues, msg) {
			review.PacingReview.Issues = append(review.PacingReview.Issues, msg)
		}
	}
	for _, suggestion := range result.Suggestions {
		msg := "【去AI味】" + suggestion
		if !containsStr(review.PacingReview.Suggestions, msg) {
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
			if !containsStr(review.Suggestions, summary) {
				review.Suggestions = append(review.Suggestions, summary)
			}
		}
	}
}

func applyHumanizeChecksToVolume(volume *models.Volume, contents map[string]string, review *agents.VolumeReview) {
	if !writeHumanizeFlag || volume == nil || review == nil {
		return
	}
	contentByID := contents
	if contentByID == nil {
		contentByID = loadFinalChapterContentsForVolume(volume)
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
	if err := validateWriteAgentApplyOption(writeAgentSDKFlag, writeAgentApplyFlag); err != nil {
		return err
	}
	if err := validateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
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

	// Get project root for continuity builder
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
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
	volumes := getVolumesForDraft(outline, writeVolumeFlag, writeChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	forcedAgentSDKImprove := writeAgentSDKFlag && (strings.TrimSpace(writeChapterFlag) != "" || strings.TrimSpace(writePromptFlag) != "")

	// Auto-review flag - set to true if any volume needs review
	autoReviewNeeded := false

	// Check if reviews exist for all volumes
	for _, volume := range volumes {
		_, err := loadVolumeReview(volume.ID)
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
			_, err := loadVolumeReview(volume.ID)
			if err != nil {
				log.Info("Auto-reviewing volume: %s", volume.ID)
				if err := reviewVolumeWithWriteAgent(ctx, writeAgent, continuityBuilder, volume, outline, targetWords, writeConcurrencyFlag); err != nil {
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
			review, err := loadVolumeReview(volume.ID)
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
				chaptersToImprove = getWriteChaptersNeedingImprovement(review, outline, writeMinScoreFlag)
				if len(chaptersToImprove) == 0 {
					log.Info("Volume %s: All chapters meet quality threshold", volume.ID)
					continue
				}
				log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))
			}

			// Improve chapters concurrently
			improved, err := improveChaptersWithWriteAgent(ctx, writeAgent, recapAgent, recapStore, chaptersToImprove, review.Reviews, outline, continuityBuilder, writeConcurrencyFlag, targetWords, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag)
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

func validateWriteAgentApplyOption(useAgentSDK, agentApply bool) error {
	if agentApply && !useAgentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
}

func validateWriteAgentHistoryOption(useAgentSDK, agentHistory bool) error {
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
func improveChaptersWithWriteAgent(ctx context.Context, agent *agents.WriteAgent, recapAgent recapExtractor, recapStore *recap.Store, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, continuityBuilder *logic.ChapterContinuityBuilder, concurrency int, targetWords int, useAgentSDK bool, agentApply bool, useAgentHistory bool) (int, error) {
	log := logger.GetLogger()
	var errc writeErrorCollector

	// Pre-build DSL simulation for enrichment (shared across goroutines)
	var dslIssues []dsl.SimulationIssue
	dslBridge := dsl.NewSimulationBridge()
	if charModels, locModels, itemModels, orgModels, elErr := loadAllElements(); elErr == nil {
		setup, _ := loadStorySetup()
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

	// Create work channel and wait group
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup
	improvedCount := 0
	var mu sync.Mutex

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				review := reviewMap[chapter.ID]
				if review == nil {
					log.Warn("[Worker %d] No existing review for forced chapter %s, using focused check suggestions", workerID, chapter.ID)
					review = fallbackDraftReviewForForcedImprove(chapter)
				}

				log.Info("[Worker %d] Improving chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load current chapter content
				currentContent := loadFinalChapterContent(chapter)
				if currentContent == "" {
					if useAgentSDK && agentApply {
						log.Warn("[Worker %d] No existing content for chapter %s; Agent SDK apply may create it through validated chapter patch", workerID, chapter.ID)
					} else {
						log.Error("[Worker %d] No existing content for chapter %s, skipping improvement", workerID, chapter.ID)
						errc.Addf("%s: no existing content for improvement", chapter.ID)
						continue
					}
				}
				applyHumanizeCheckToReview(chapter, currentContent, review)

				// Build improvement suggestions
				suggestions := buildImprovementSuggestions(review)

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
					if checkSuggestions := buildAgentSDKChapterCheckSuggestions(chapter, targetWords); checkSuggestions != "" {
						suggestions += "\n\n" + checkSuggestions
					}
				}

				// Load context drafts
				context := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate chapter continuity
				continuity := continuityBuilder.BuildBefore(outline, chapter)

				// Generate improved content with suggestions
				content, err := generateImprovedChapter(ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, useAgentSDK, agentApply, useAgentHistory)
				if err != nil {
					log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
					errc.Addf("%s: improve failed: %w", chapter.ID, err)
					continue
				}
				content, agentApplied := resolveAgentAppliedChapterContent(log, workerID, chapter, currentContent, content, useAgentSDK, agentApply)

				// Apply enabled minimal-change fixers (teleport bridge, character presence)
				knownChars := collectKnownCharactersFromOutline(outline)
				fixed, sum := applyImproveFixesWrite(
					log,
					workerID,
					chapter,
					outline,
					content,
					suggestions,
					writeTeleportFixFlag,
					writeBridgeRetriesFlag,
					func(s string) (string, error) {
						return generateImprovedChapter(ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
					},
					writeCharacterFixFlag,
					writeCharacterPatchRetriesFlag,
					knownChars,
					func(s string) (string, error) {
						return generateImprovedChapter(ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
					},
				)
				content = fixed
				log.Info("[Worker %d] Fix summary for %s: %s", workerID, chapter.ID, sum.String())

				if !agentApplied && !finalChapterContentChanged(chapter, currentContent, content) {
					log.Info("[Worker %d] No content changes for chapter %s; skipping save, post-save check, and recap refresh", workerID, chapter.ID)
					continue
				}

				// Save improved content
				if agentApplied && strings.TrimSpace(loadFinalChapterContent(chapter)) == strings.TrimSpace(content) {
					log.Info("[Worker %d] Agent patch already saved chapter %s through tool patch chapter --apply", workerID, chapter.ID)
				} else if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("[Worker %d] Failed to save improved chapter %s: %v", workerID, chapter.ID, err)
					errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
					continue
				}
				if useAgentSDK {
					postCheck, err := runAgentSDKChapterPostSaveCheck(ctx, chapter, targetWords)
					if err != nil {
						log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
					} else {
						logAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, postCheck)
						appendAgentSDKPostSaveReviewSuggestions(review, postCheck)
					}
				}
				if savedContent := loadFinalChapterContent(chapter); strings.TrimSpace(savedContent) != "" {
					content = savedContent
				}
				if err := extractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, useAgentSDK); err != nil {
					log.Warn("[Worker %d] Recap refresh failed for improved chapter %s: %v", workerID, chapter.ID, err)
				} else {
					log.Info("[Worker %d] Updated recap for improved chapter %s", workerID, chapter.ID)
				}

				mu.Lock()
				improvedCount++
				mu.Unlock()

				log.Info("[Worker %d] Improved chapter saved: %s", workerID, chapter.ID)
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

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

func generateImprovedChapter(ctx context.Context, agent *agents.WriteAgent, chapter *models.Chapter, context *agents.ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentContent string, suggestions string, useAgentSDK bool, agentApply bool, useAgentHistory bool, iteration ...int) (string, error) {
	if useAgentSDK {
		effectiveTargetWords := effectiveAgentSDKImproveTargetWords(chapter, targetWords, currentContent, suggestions, useAgentSDK)
		content, err := agent.GenerateChapterWithSuggestionsAgentSDK(ctx, chapter, context, continuity, effectiveTargetWords, currentContent, suggestions, agentApply, useAgentHistory, iteration...)
		if err != nil && isAgentSDKLengthOvershootError(err) {
			logger.GetLogger().Warn("Agent SDK chapter improvement overshot target length; retrying once with strict minimal-repair length guidance")
			content, err = agent.GenerateChapterWithSuggestionsAgentSDK(ctx, chapter, context, continuity, effectiveTargetWords, currentContent, appendAgentSDKLengthRetrySuggestions(suggestions, effectiveTargetWords), agentApply, useAgentHistory, iteration...)
		}
		if err != nil {
			if recovered, ok := recoverAgentAppliedChapterContentAfterError(chapter, currentContent, effectiveTargetWords, agentApply); ok {
				logger.GetLogger().Warn("Agent SDK returned an error after applying a chapter patch; recovering saved content for %s and continuing Go validation: %v", chapter.ID, err)
				return recovered, nil
			}
		}
		return content, err
	}
	return agent.GenerateChapterWithSuggestions(ctx, chapter, context, continuity, targetWords, currentContent, suggestions, iteration...)
}

func repairAgentSDKGeneratedChapterPostSave(ctx context.Context, log logger.LoggerInterface, workerID int, agent *agents.WriteAgent, chapter *models.Chapter, context *agents.ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentContent string, check *toolCheckResult, useAgentHistory bool) (string, *toolCheckResult, error) {
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
		formatAgentSDKChapterCheckSuggestions(check)

	repaired, err := generateImprovedChapter(ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, true, true, useAgentHistory, 1)
	if err != nil {
		return "", nil, err
	}
	repaired, agentApplied := resolveAgentAppliedChapterContent(log, workerID, chapter, currentContent, repaired, true, true)
	if !agentApplied {
		if err := saveFinalChapter(chapter, repaired); err != nil {
			return "", nil, fmt.Errorf("save repaired final chapter: %w", err)
		}
		if saved := loadFinalChapterContent(chapter); strings.TrimSpace(saved) != "" {
			repaired = saved
		}
	}
	recheck, err := runAgentSDKChapterPostSaveCheck(ctx, chapter, targetWords)
	if err != nil {
		return repaired, nil, err
	}
	return repaired, recheck, nil
}

func effectiveAgentSDKImproveTargetWords(chapter *models.Chapter, targetWords int, currentContent string, suggestions string, useAgentSDK bool) int {
	if !useAgentSDK || targetWords <= 0 || suggestionsRequestLengthExpansion(suggestions) {
		return targetWords
	}
	currentUnits := chapterNarrativeUnitsForLog(chapter, currentContent)
	if currentUnits <= 0 {
		return targetWords
	}
	if currentUnits > targetWords {
		return targetWords
	}
	return currentUnits
}

func suggestionsRequestLengthExpansion(suggestions string) bool {
	text := strings.ToLower(strings.TrimSpace(suggestions))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"too little content",
		"too short",
		"shortfall",
		"under target",
		"length",
		"word count",
		"字数",
		"篇幅",
		"过短",
		"太短",
		"不足",
		"扩写",
		"补足",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func recoverAgentAppliedChapterContentAfterError(chapter *models.Chapter, previousContent string, targetWords int, agentApply bool) (string, bool) {
	if !agentApply || chapter == nil {
		return "", false
	}
	saved := loadFinalChapterContent(chapter)
	if strings.TrimSpace(saved) == "" || strings.TrimSpace(saved) == strings.TrimSpace(previousContent) {
		return "", false
	}
	if err := validateRecoveredAgentSDKChapterLength(chapter, saved, targetWords); err != nil {
		logger.GetLogger().Warn("Agent SDK saved content exists but is not recoverable: %v", err)
		return "", false
	}
	return saved, true
}

func validateRecoveredAgentSDKChapterLength(chapter *models.Chapter, content string, targetWords int) error {
	if targetWords <= 0 {
		return nil
	}
	count := toolNarrativeUnitCount(content)
	hardMax := int(float64(targetWords) * 1.35)
	if targetWords+300 > hardMax {
		hardMax = targetWords + 300
	}
	if count > hardMax {
		chapterID := ""
		if chapter != nil {
			chapterID = chapter.ID
		}
		return fmt.Errorf("saved agent patch is too long for chapter %s: got %d narrative units, target %d, hard max %d", chapterID, count, targetWords, hardMax)
	}
	return nil
}

func resolveAgentAppliedChapterContent(log logger.LoggerInterface, workerID int, chapter *models.Chapter, previousContent, agentContent string, useAgentSDK, agentApply bool) (string, bool) {
	if !useAgentSDK || !agentApply || chapter == nil {
		return agentContent, false
	}
	saved := loadFinalChapterContent(chapter)
	if strings.TrimSpace(saved) == "" || strings.TrimSpace(saved) == strings.TrimSpace(previousContent) {
		return agentContent, false
	}
	if strings.TrimSpace(saved) != strings.TrimSpace(agentContent) {
		log.Info("[Worker %d] Agent apply changed %s; using saved patch content instead of returned JSON content", workerID, chapter.ID)
	}
	return saved, true
}

func isAgentSDKLengthOvershootError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "agent-sdk returned too much content")
}

func appendAgentSDKLengthRetrySuggestions(suggestions string, targetWords int) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(suggestions))
	if sb.Len() > 0 {
		sb.WriteString("\n\n")
	}
	sb.WriteString("## Agent SDK Length Retry\n\n")
	sb.WriteString("The previous Agent SDK attempt was rejected because it produced too much content. ")
	sb.WriteString("Retry with minimal repair only: preserve the existing chapter shape, replace only the paragraphs needed to fix the reported check issues, and do not expand the scene list. ")
	if targetWords > 0 {
		low := int(float64(targetWords) * 0.9)
		high := int(float64(targetWords) * 1.1)
		sb.WriteString(fmt.Sprintf("The returned `content` must be close to %d narrative units, preferably %d-%d, and must not exceed %d. ", targetWords, low, high, int(float64(targetWords)*1.35)))
	}
	sb.WriteString("Return only the final JSON object with the revised chapter content.")
	return sb.String()
}

func buildAgentSDKChapterCheckSuggestions(chapter *models.Chapter, targetWords int) string {
	if chapter == nil || strings.TrimSpace(chapter.ID) == "" {
		return ""
	}
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not run focused chapter check before improvement: %v", err)
	}
	result, err := runToolChapterCheckWithTargetWords(root, "all", "chapter", chapter.ID, targetWords)
	if err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not run focused chapter check before improvement: %v", err)
	}
	if err := applyToolCheckIssueFilters(result, "low", "", 8); err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not filter focused chapter check before improvement: %v", err)
	}
	return formatAgentSDKChapterCheckSuggestions(result)
}

func formatAgentSDKChapterCheckSuggestions(result *toolCheckResult) string {
	if result == nil {
		return ""
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not encode focused chapter check before improvement: %v", err)
	}
	return "## Agent SDK Chapter Check\n\n" +
		"Treat this deterministic `tool check --target chapter` result as the primary repair task list. " +
		"If an issue contains `navigation.refresh_query`, run it first, then run `navigation.post_refresh_check_query` before deciding whether prose needs repair. " +
		"Before rewriting, execute each remaining issue's `navigation.repair_route_query` when present; it returns an index-sized route and next_actions. " +
		"Only execute `navigation.repair_context_query` when the route says detailed facts or excerpts are needed. " +
		"Do not query full setup, full outline, or full chapter files unless the focused repair context is insufficient.\n\n" +
		"```json\n" + string(data) + "\n```"
}

func runAgentSDKChapterPostSaveCheck(ctx context.Context, chapter *models.Chapter, targetWords int) (*toolCheckResult, error) {
	if chapter == nil || strings.TrimSpace(chapter.ID) == "" {
		return nil, fmt.Errorf("chapter is nil or has no ID")
	}
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	restoreLogger := suppressToolRefreshLogs()
	_, refreshErr := refreshToolChapterDSL(ctx, root, chapter.ID, toolRefreshFlags.BatchSize)
	restoreLogger()
	if refreshErr != nil {
		return nil, fmt.Errorf("refresh chapter RPG DSL before post-save check: %w", refreshErr)
	}
	result, err := runToolChapterCheckWithTargetWords(root, "all", "chapter", chapter.ID, targetWords)
	if err != nil {
		return nil, err
	}
	if err := applyToolCheckIssueFilters(result, "low", "", 12); err != nil {
		return nil, err
	}
	return result, nil
}

func logAgentSDKChapterPostSaveCheck(log logger.LoggerInterface, workerID int, chapterID string, result *toolCheckResult) {
	if result == nil {
		return
	}
	log.Info("[Worker %d] Agent SDK post-save check for %s: ok=%t blocking=%t score=%.0f issues=%d (critical=%d high=%d medium=%d low=%d)",
		workerID,
		chapterID,
		result.OK,
		result.Blocking,
		result.Score,
		result.Summary.Total,
		result.Summary.Critical,
		result.Summary.High,
		result.Summary.Medium,
		result.Summary.Low,
	)
	if result.Blocking {
		for _, issue := range firstNReviewSuggestions(result.Issues, 3) {
			log.Warn("[Worker %d] Remaining chapter issue for %s: [%s/%s] %s", workerID, chapterID, issue.Priority, issue.Category, issue.Issue)
		}
	}
}

func appendAgentSDKPostSaveReviewSuggestions(review *agents.DraftReview, result *toolCheckResult) {
	if review == nil || result == nil {
		return
	}
	if result.Blocking {
		review.NeedsRevision = true
	}
	for _, issue := range result.Issues {
		text := formatAgentSDKPostSaveIssueSuggestion(result.Kind, result.Target, result.Scope, issue)
		if text != "" && !containsStr(review.Suggestions, text) {
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
	nav := toolIssueNavigation(kind, target, scope, issue.TargetID, issue, 0)
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

func firstNReviewSuggestions(items []models.ReviewSuggestion, n int) []models.ReviewSuggestion {
	if n < 0 {
		n = 0
	}
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func getWriteChaptersNeedingImprovement(review *agents.VolumeReview, outline *models.Outline, minScorePercent int) []*models.Chapter {
	if review == nil || outline == nil {
		return nil
	}
	chapters := make([]*models.Chapter, 0)
	for _, r := range review.Reviews {
		if r.NeedsRevision || draftReviewScorePercent(r) < minScorePercent {
			if chapter := outline.GetChapterByID(r.ChapterID); chapter != nil {
				chapters = append(chapters, chapter)
			}
		}
	}
	return chapters
}

func draftReviewScorePercent(review agents.DraftReview) int {
	if review.OverallScore <= 10 {
		return review.OverallScore * 10
	}
	return review.OverallScore
}

func reviewScorePercentInt(score float64) int {
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

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
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

	// Get project root for continuity builder
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
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
	var errc writeErrorCollector

	// Review chapters concurrently
	chapterChan := make(chan *models.Chapter, len(chaptersToReview))
	var wg sync.WaitGroup
	var mu sync.Mutex

	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chaptersToReview) {
		concurrency = len(chaptersToReview)
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load final chapter content
				content := loadFinalChapterContent(chapter)
				if content == "" {
					log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
					errc.Addf("%s: no final chapter content found for review", chapter.ID)
					continue
				}

				// Load context with recap for continuity checking
				chapterContext := loadChapterContext(outline, chapter, writeContextFlag)

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
					continue
				}
				if err := saveWriteReviewResult(chapter, reviewResult); err != nil {
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
					OverallScore:  reviewScorePercentInt(reviewResult.OverallScore),
					NeedsRevision: reviewResult.OverallScore < 70,
					Suggestions:   suggestionStrings,
				}

				// Get volume ID for this chapter
				volumeID := getVolumeIDFromChapter(chapter.ID)
				volume := outline.GetVolumeByID(volumeID)

				mu.Lock()
				if _, exists := volumeContents[volumeID]; !exists {
					volumeContents[volumeID] = make(map[string]string)
				}
				volumeContents[volumeID][chapter.ID] = content
				// Create volume review if not exists
				if _, exists := volumeReviews[volumeID]; !exists {
					volumeTitle := ""
					if volume != nil {
						volumeTitle = volume.Title
					}
					volumeReviews[volumeID] = &agents.VolumeReview{
						VolumeID:    volumeID,
						VolumeTitle: volumeTitle,
						Reviews:     make([]agents.DraftReview, 0),
					}
				}
				volumeReviews[volumeID].Reviews = append(volumeReviews[volumeID].Reviews, draftReview)
				mu.Unlock()

				log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chaptersToReview {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	// Save all volume reviews
	for volumeID, review := range volumeReviews {
		if existing, err := loadVolumeReview(volumeID); err == nil {
			review = mergeVolumeReviewByChapter(existing, review)
		} else if !os.IsNotExist(err) {
			log.Warn("Failed to load existing review for volume %s before merge: %v", volumeID, err)
		}
		if volume := outline.GetVolumeByID(volumeID); volume != nil {
			contentByID := loadFinalChapterContentsForVolume(volume)
			for chapterID, content := range volumeContents[volumeID] {
				contentByID[chapterID] = content
			}
			applyHeuristicTransitionChecks(volume, contentByID, review)
			applyHumanizeChecksToVolume(volume, contentByID, review)
		}
		if err := saveVolumeReview(review); err != nil {
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
		return getAllChapters(outline)
	}

	// If chapter flag specified, return specific chapter(s)
	if chapterFlag != "" {
		chapters, err := getChaptersToGenerate(outline, chapterFlag, volumeFlag, partFlag, false)
		if err == nil {
			return chapters
		}
	}

	// If volume flag specified, return all chapters in volume
	if volumeFlag != "" {
		var chapters []*models.Chapter
		volumeID := resolveVolumeID(outline, volumeFlag, partFlag)
		for _, ch := range getAllChapters(outline) {
			if strings.HasPrefix(ch.ID, volumeID) {
				chapters = append(chapters, ch)
			}
		}
		return chapters
	}

	// Default: return all chapters
	return getAllChapters(outline)
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

// resolveVolumeID resolves volume flag to full volume ID
func resolveVolumeID(outline *models.Outline, volumeFlag, partFlag string) string {
	// If already full ID, return as is
	if strings.Contains(volumeFlag, "-") {
		return volumeFlag
	}

	// Try to resolve with part flag
	partID := partFlag
	if partID == "" {
		partID = "P1" // Default to first part
	}
	if !strings.HasPrefix(partID, "P") {
		partID = "P" + partID
	}

	volumeID := volumeFlag
	if !strings.HasPrefix(volumeID, "V") {
		volumeID = "V" + volumeID
	}

	return partID + "-" + volumeID
}

// reviewVolumeWithWriteAgent reviews all chapters in a volume using the write agent
func reviewVolumeWithWriteAgent(ctx context.Context, writeAgent *agents.WriteAgent, continuityBuilder *logic.ChapterContinuityBuilder, volume *models.Volume, outline *models.Outline, targetWords int, concurrency int) error {
	log := logger.GetLogger()

	// Get all chapters in this volume
	var chapters []*models.Chapter
	for _, ch := range getAllChapters(outline) {
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
	var errc writeErrorCollector

	// Review chapters concurrently
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup
	var mu sync.Mutex

	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load final chapter content
				content := loadFinalChapterContent(chapter)
				if content == "" {
					log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
					errc.Addf("%s: no final chapter content found for review", chapter.ID)
					continue
				}

				// Load context with recap for continuity checking
				chapterContext := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate continuity snapshot for continuity checking
				continuity := continuityBuilder.BuildBefore(outline, chapter)

				// Review chapter
				reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
				if err != nil {
					log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
					errc.Addf("%s: review failed: %w", chapter.ID, err)
					continue
				}
				if err := saveWriteReviewResult(chapter, reviewResult); err != nil {
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
					OverallScore:  reviewScorePercentInt(reviewResult.OverallScore),
					NeedsRevision: reviewResult.OverallScore < 70,
					Suggestions:   suggestionStrings,
				}

				mu.Lock()
				volumeContents[chapter.ID] = content
				volumeReview.Reviews = append(volumeReview.Reviews, draftReview)
				mu.Unlock()

				log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	// Save volume review
	applyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
	applyHumanizeChecksToVolume(volume, volumeContents, volumeReview)
	if err := saveVolumeReview(volumeReview); err != nil {
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
	if err := validateWriteAgentApplyOption(writeAgentSDKFlag, writeAgentApplyFlag); err != nil {
		return err
	}
	if err := validateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
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

	// Get project root for continuity builder
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
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
	log.Info("RPG DSL export: enabled=%t batch_size=%d output=%s", writeEmitRPGDSLFlag, writeRPGBatchSizeFlag, filepath.Join(root, "story", "rpg", "04_chapters.rpg"))
	minScorePercent := writeMinScorePercent()
	log.Info("Minimum acceptable score: %.0f/100", minScorePercent)
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

	// RPG DSL issues are refreshed after chapter content changes. Running this
	// before generation would analyze stale or missing chapter files.
	var rpgDSLIssues []dsl.SimulationIssue
	knownChars := collectKnownCharactersFromOutline(outline)
	var errc writeErrorCollector

	// Process each chapter completely before moving to the next
	for chapterIdx, chapter := range chaptersToProcess {
		log.Info("\n%s", strings.Repeat("=", 60))
		log.Info("Processing Chapter %d/%d: %s - %s", chapterIdx+1, len(chaptersToProcess), chapter.ID, chapter.Title)
		log.Info("%s", strings.Repeat("=", 60))

		// Step 1: Generate chapter (if not exists)
		log.Info("\n[Step 1/4] Generating chapter...")
		content := loadFinalChapterContent(chapter)
		if content == "" {
			log.Info("Generating chapter: %s - %s", chapter.ID, chapter.Title)
			chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			var generatedContent string
			var err error
			if writeAgentSDKFlag {
				generatedContent, err = writeAgent.GenerateChapterWithAgentSDK(ctx, chapter, chapterContext, continuity, targetWords, writeAgentHistoryFlag)
			} else {
				generatedContent, err = writeAgent.GenerateChapter(ctx, chapter, chapterContext, continuity, targetWords)
			}
			if err != nil {
				log.Error("Failed to generate chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: generate failed: %w", chapter.ID, err)
				continue
			}
			if err := saveFinalChapter(chapter, generatedContent); err != nil {
				log.Error("Failed to save chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save generated chapter failed: %w", chapter.ID, err)
				continue
			}
			content = loadFinalChapterContent(chapter)
			if strings.TrimSpace(content) == "" {
				content = generatedContent
			}
			log.Info("Generated chapter output: %s", finalChapterPath(root, chapter))
		} else {
			log.Info("Chapter %s already exists, using: %s", chapter.ID, finalChapterPath(root, chapter))
		}

		// Step 2: Review chapter
		log.Info("\n[Step 2/4] Reviewing chapter...")
		chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
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
		if err := saveWriteReviewResult(chapter, reviewResult); err != nil {
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
			OverallScore:  reviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: reviewResult.OverallScore < minScorePercent,
			Suggestions:   suggestionStrings,
		}
		volumeID := getVolumeIDFromChapter(chapter.ID)
		volume := outline.GetVolumeByID(volumeID)
		var volumeReview *agents.VolumeReview
		reviewPath := filepath.Join(root, "story", "reviews", volumeID+"_review.json")
		if data, err := os.ReadFile(reviewPath); err == nil {
			var existingReview agents.VolumeReview
			if err := json.Unmarshal(data, &existingReview); err == nil {
				volumeReview = &existingReview
			}
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
			volumeContents := loadFinalChapterContentsForVolume(volume)
			applyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
			applyHumanizeChecksToVolume(volume, volumeContents, volumeReview)
			if updated := findDraftReview(volumeReview, chapter.ID); updated != nil {
				draftReview = *updated
			}
		}
		if err := saveVolumeReview(volumeReview); err != nil {
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
			chapterRPGIssues := GetChapterSpecificSuggestions(rpgDSLIssues, chapter.ID)
			hasRPGIssues := chapterRPGIssues != ""

			if len(chapterReview.Suggestions) == 0 && !chapterReview.NeedsRevision && !hasRPGIssues {
				log.Info("Chapter %s: no suggestions and no revision needed, skipping improvement", chapter.ID)
				break
			}

			log.Info("Improvement round %d/%d (score: %d, RPG issues: %v)", round, writeMaxRoundsFlag, chapterReview.OverallScore, hasRPGIssues)
			suggestions := buildImprovementSuggestions(chapterReview)

			// Append RPG DSL issues as suggestions if any
			if hasRPGIssues {
				suggestions += "\n\n" + chapterRPGIssues
			}
			if writeAgentSDKFlag {
				if checkSuggestions := buildAgentSDKChapterCheckSuggestions(chapter, targetWords); checkSuggestions != "" {
					suggestions += "\n\n" + checkSuggestions
				}
			}
			chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			improvedContent, err := generateImprovedChapter(ctx, writeAgent, chapter, chapterContext, continuity, targetWords, currentContent, suggestions, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
			if err != nil {
				log.Error("Failed to improve chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: improve round %d failed: %w", chapter.ID, round, err)
				break
			}
			improvedContent, agentApplied := resolveAgentAppliedChapterContent(log, 0, chapter, currentContent, improvedContent, writeAgentSDKFlag, writeAgentApplyFlag)

			fixedContent, sum := applyImproveFixesWrite(
				log,
				0,
				chapter,
				outline,
				improvedContent,
				suggestions,
				writeTeleportFixFlag,
				writeBridgeRetriesFlag,
				func(s string) (string, error) {
					return generateImprovedChapter(ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
				writeCharacterFixFlag,
				writeCharacterPatchRetriesFlag,
				knownChars,
				func(s string) (string, error) {
					return generateImprovedChapter(ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
			)
			improvedContent = fixedContent
			log.Info("Fix summary for %s: %s", chapter.ID, sum.String())

			if !agentApplied && !finalChapterContentChanged(chapter, currentContent, improvedContent) {
				log.Info("No content changes for chapter %s; skipping save and post-save check", chapter.ID)
				break
			}

			if agentApplied && strings.TrimSpace(loadFinalChapterContent(chapter)) == strings.TrimSpace(improvedContent) {
				log.Info("Agent patch already saved chapter %s through tool patch chapter --apply", chapter.ID)
			} else if err := saveFinalChapter(chapter, improvedContent); err != nil {
				log.Error("Failed to save improved chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
				break
			}
			if writeAgentSDKFlag {
				postCheck, err := runAgentSDKChapterPostSaveCheck(ctx, chapter, targetWords)
				if err != nil {
					log.Warn("Agent SDK post-save check failed for %s: %v", chapter.ID, err)
				} else {
					logAgentSDKChapterPostSaveCheck(log, 0, chapter.ID, postCheck)
					appendAgentSDKPostSaveReviewSuggestions(chapterReview, postCheck)
					if err := saveVolumeReview(volumeReview); err != nil {
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
				if err := saveWriteReviewResult(chapter, reviewResult); err != nil {
					log.Warn("Failed to save full write review for %s: %v", chapter.ID, err)
				}
				var suggestionStrings []string
				for _, s := range reviewResult.Suggestions {
					suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
				}
				chapterReview.OverallScore = reviewScorePercentInt(reviewResult.OverallScore)
				chapterReview.NeedsRevision = reviewResult.OverallScore < minScorePercent
				chapterReview.Suggestions = suggestionStrings
				if volume != nil {
					volumeContents := loadFinalChapterContentsForVolume(volume)
					applyHeuristicTransitionChecks(volume, volumeContents, volumeReview)
					applyHumanizeChecksToVolume(volume, volumeContents, volumeReview)
					if updated := findDraftReview(volumeReview, chapter.ID); updated != nil {
						chapterReview = updated
					}
				}
				// Save updated review
				if err := saveVolumeReview(volumeReview); err != nil {
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
		if err := extractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, currentContent, 0, usePipelineRecapAgentSDK); err != nil {
			log.Error("Failed to generate/save recap for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: final recap failed: %w", chapter.ID, err)
		} else {
			log.Info("Recap output: %s", filepath.Join(root, "story", "recaps", chapter.ID+".json"))
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
) (writeRPGDSLResult, error) {
	log := logger.GetLogger()
	log.Info("\n[RPG DSL] Refreshing chapter DSL (%s)...", reason)
	result, err := emitChapterRPGDSLForProject(ctx, root, filepath.Base(root), config, cfg, setup, client, batchSize)
	if err != nil {
		return result, err
	}
	log.Info("[RPG DSL] Output: %s", result.DSLPath)
	log.Info("[RPG DSL] Batches: %d reused, %d generated", result.ReusedBatches, result.GeneratedBatches)
	log.Info("[RPG DSL] Issues detected: %d total, %d important", len(result.Issues), len(FilterImportantIssues(result.Issues)))
	return result, nil
}
