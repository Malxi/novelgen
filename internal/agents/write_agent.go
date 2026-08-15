package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
)

const compactStyleReferenceRuneLimit = 2000

// CompactStorySetup is a minimal version of StorySetup for chapter generation
// Only includes essential fields needed for writing
type CompactStorySetup struct {
	Genres       []string            `json:"genres" md:"genres" desc:"Story genres (2-4 specific genres)"`
	Premise      string              `json:"premise" md:"premise" desc:"Core story premise (2-4 sentences)"`
	Theme        string              `json:"theme" md:"theme" desc:"Story theme (clear statement)"`
	Rules        []string            `json:"rules" md:"rules" desc:"World rules (3-7 enforceable rules)"`
	Tone         string              `json:"tone" md:"tone" desc:"Writing tone (2-4 adjectives, comma-separated)"`
	Tense        string              `json:"tense" md:"tense" desc:"Narrative tense (past or present)"`
	POVStyle     string              `json:"pov_style" md:"pov_style" desc:"POV style (first person, third person limited, or third person omniscient)"`
	WritingStyle models.WritingStyle `json:"writing_style,omitempty" md:"writing_style,omitempty" desc:"Optional prose style instructions and reference excerpt; use as style signal only, not story facts"`
}

// ToCompact converts a full StorySetup to CompactStorySetup
func ToCompact(setup *models.StorySetup) CompactStorySetup {
	if setup == nil {
		return CompactStorySetup{}
	}
	return CompactStorySetup{
		Genres:       setup.Genres,
		Premise:      setup.Premise,
		Theme:        setup.Theme,
		Rules:        setup.Rules,
		Tone:         setup.Tone,
		Tense:        setup.Tense,
		POVStyle:     setup.POVStyle,
		WritingStyle: setup.WritingStyle.CompactReference(compactStyleReferenceRuneLimit),
	}
}

// WriteGenInput is the input for final chapter generation
type WriteGenInput struct {
	StorySetup   CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	Continuity   string            `json:"continuity" md:"continuity" desc:"Current story continuity including character statuses, relationships, goals, resources, and recent state changes"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	HistoryMode  string            `json:"history_mode,omitempty" md:"history_mode,omitempty" desc:"When present, concise instructions for using copied prompt/response/agent-live logs as creative continuation history"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
}

// WriteGenOutput is the output for final chapter generation
type WriteGenOutput struct {
	Content string `json:"content" md:"content" desc:"The final polished chapter content"`
}

// WriteImproveInput is the input for chapter improvement
type WriteImproveInput struct {
	StorySetup   CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	Continuity   string            `json:"continuity" md:"continuity" desc:"Current story continuity including character statuses, relationships, goals, resources, and recent state changes"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	Iteration    int               `json:"iteration" md:"iteration" desc:"Current improvement iteration number"`
	ApplyPatches bool              `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch chapter --apply after a successful dry-run"`
	CurrentDraft string            `json:"current_draft" md:"current_draft" desc:"The current draft content to be improved"`
	Suggestions  string            `json:"suggestions" md:"suggestions" desc:"Review suggestions for improvement"`
	HistoryMode  string            `json:"history_mode,omitempty" md:"history_mode,omitempty" desc:"When present, concise instructions for using copied prompt/response/agent-live logs as creative continuation history"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
}

// WriteImproveOutput is the output for chapter improvement
type WriteImproveOutput struct {
	Content string `json:"content" md:"content" desc:"The improved chapter content"`
}

// WriteReviewInput is the input for chapter review
type WriteReviewInput struct {
	StorySetup     CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	Continuity     string            `json:"continuity" md:"continuity" desc:"Current story continuity including character statuses, relationships, goals, resources, and recent state changes"`
	Chapter        models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	ChapterContent string            `json:"chapter_content" md:"chapter_content" desc:"The chapter content to be reviewed"`
	TargetWords    int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	ChapterStats   string            `json:"chapter_stats,omitempty" md:"chapter_stats,omitempty" desc:"Deterministic chapter length statistics; trust this over manual word-count estimates"`
	Iteration      int               `json:"iteration" md:"iteration" desc:"Current iteration number"`
	Context        string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap          string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter for continuity checking"`
	NextChapters   []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing and hook checks"`
}

// WriteReviewOutput is the output for chapter review
type WriteReviewOutput struct {
	Result models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result including scores and improvement suggestions"`
}

type writeAgentSDKReviewOutput struct {
	Result writeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact chapter review result"`
}

type writeAgentSDKReviewResult struct {
	OverallScore float64                   `json:"overall_score" md:"overall_score" desc:"0-100 chapter quality score"`
	Summary      string                    `json:"summary" md:"summary" desc:"Compact Chinese review summary under 500 characters"`
	Strengths    []string                  `json:"strengths,omitempty" md:"strengths" desc:"At most 4 concrete strengths"`
	Weaknesses   []string                  `json:"weaknesses,omitempty" md:"weaknesses" desc:"At most 4 concrete weaknesses"`
	Suggestions  []models.ReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"At most 8 concrete suggestions for write improve"`
}

// ChapterContext holds surrounding chapter information for continuity
type ChapterContext struct {
	Previous []*ContextChapter
	Current  *models.Chapter
	Next     []*ContextChapter
	Recap    string
	Craft    string
}

// ContextChapter represents a chapter with its content
type ContextChapter struct {
	Chapter *models.Chapter
	Content string
}

// VolumeReviewChapterInput represents a chapter in volume review
type VolumeReviewChapterInput struct {
	ChapterID    string   `json:"chapter_id" md:"chapter_id"`
	ChapterTitle string   `json:"chapter_title" md:"chapter_title"`
	Summary      string   `json:"summary" md:"summary"`
	Content      string   `json:"content" md:"content"`
	Beats        []string `json:"beats" md:"beats"`
}

// VolumeReviewInput is the input for volume-level review
type VolumeReviewInput struct {
	StorySetup            CompactStorySetup          `json:"story_setup" md:"story_setup"`
	VolumeID              string                     `json:"volume_id" md:"volume_id"`
	VolumeTitle           string                     `json:"volume_title" md:"volume_title"`
	VolumeSummary         string                     `json:"volume_summary" md:"volume_summary"`
	Chapters              []VolumeReviewChapterInput `json:"chapters" md:"chapters"`
	TargetWordsPerChapter int                        `json:"target_words_per_chapter" md:"target_words_per_chapter"`
}

// VolumeChapterReview represents review result for a single chapter in volume review
type VolumeChapterReview struct {
	ChapterID          string   `json:"chapter_id" md:"chapter_id" desc:"Chapter ID"`
	ChapterScore       float64  `json:"chapter_score" md:"chapter_score" desc:"Chapter score 0-10, can be decimal like 8.5"`
	ChapterRole        string   `json:"chapter_role" md:"chapter_role" desc:"Chapter role in volume: setup/development/turning_point/climax/resolution"`
	ContinuityWithPrev string   `json:"continuity_with_previous" md:"continuity_with_previous" desc:"Continuity evaluation with previous chapter"`
	ContinuityWithNext string   `json:"continuity_with_next" md:"continuity_with_next" desc:"Continuity evaluation with next chapter"`
	Issues             []string `json:"issues" md:"issues" desc:"List of issues found in this chapter"`
	Suggestions        []string `json:"suggestions" md:"suggestions" desc:"List of improvement suggestions for this chapter"`
}

// VolumeStructureAnalysis represents the volume structure analysis
type VolumeStructureAnalysis struct {
	OpeningHook  string `json:"opening_hook" md:"opening_hook" desc:"Evaluation of the opening hook"`
	RisingAction string `json:"rising_action" md:"rising_action" desc:"Evaluation of rising action distribution"`
	Climax       string `json:"climax" md:"climax" desc:"Evaluation of the climax"`
	Resolution   string `json:"resolution" md:"resolution" desc:"Evaluation of the resolution"`
}

// VolumeReviewResult represents the result of volume-level review
type VolumeReviewResult struct {
	OverallScore            int                     `json:"overall_score" md:"overall_score" desc:"Overall volume quality score 0-100"`
	VolumeStructureAnalysis VolumeStructureAnalysis `json:"volume_structure_analysis" md:"volume_structure_analysis" desc:"Analysis of volume structure: opening, rising action, climax, resolution"`
	ChapterReviews          []VolumeChapterReview   `json:"chapter_reviews" md:"chapter_reviews" desc:"Review results for each chapter"`
	VolumeLevelIssues       []string                `json:"volume_level_issues" md:"volume_level_issues" desc:"Issues that affect the entire volume"`
	VolumeLevelSuggestions  []string                `json:"volume_level_suggestions" md:"volume_level_suggestions" desc:"Suggestions for improving the entire volume"`
}

// VolumeReviewOutput is the output for volume review
type VolumeReviewOutput struct {
	Result VolumeReviewResult `json:"volume_review_result" md:"volume_review_result"`
}

// WriteAgent generates final chapter content with continuity
// It wraps BaseAgent to provide type-safe methods
type WriteAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewWriteAgent creates a new WriteAgent
func NewWriteAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *WriteAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "WriteAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &WriteAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *WriteAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// GenerateChapter generates final chapter content with continuity
func (a *WriteAgent) GenerateChapter(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, targetWords int) (string, error) {
	logger.Section("WRITE AGENT - Final Chapter Generation")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	nextInfos := buildNextChapterInfos(context)
	recap := ""
	if context != nil {
		recap = context.Recap
	}

	input := WriteGenInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		Continuity:   formatContinuityForWrite(continuity, chapter),
		TargetWords:  targetWords,
		Context:      formatChapterContext(context),
		Recap:        recap,
		NextChapters: nextInfos,
	}

	var output WriteGenOutput
	params := InvokeParams{
		Skills:  []string{"write-generate"},
		Command: "generate final chapter content",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s length warning: %s", chapter.ID, warn)
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "final", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("[ok] Generated final chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateChapterWithAgentSDK generates final chapter content through the Agent SDK.
// The agent may query focused project context, but Go remains the only writer.
func (a *WriteAgent) GenerateChapterWithAgentSDK(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, targetWords int, useHistory ...bool) (string, error) {
	logger.Section("WRITE AGENT SDK - Final Chapter Generation")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	nextInfos := buildNextChapterInfos(context)
	recap := ""
	if context != nil {
		recap = context.Recap
	}

	input := WriteGenInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		Continuity:   formatContinuityForWrite(continuity, chapter),
		TargetWords:  targetWords,
		HistoryMode:  writeAgentSDKHistoryMode(writeHistoryRequested(useHistory)),
		Context:      appendAgentSDKInitialLengthContext(formatChapterContext(context), targetWords),
		Recap:        recap,
		NextChapters: nextInfos,
	}

	params := writeAgentSDKParams("generate final chapter content using focused project tools", chapter.ID, writeHistoryRequested(useHistory), targetWords)
	var output WriteGenOutput
	var err error
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		output, err = a.executeAgentSDKChapterGeneration(ctx, params, input, chapter, targetWords)
		canRetry := attempt < maxAttempts-1
		if err == nil && agentSDKWriteShouldRetryLength(output.Content, targetWords) && canRetry {
			logger.Warn("Chapter %s agent-sdk generation exceeded preferred hard length; retrying with stricter length instruction (attempt %d/%d)", chapter.ID, attempt+1, maxAttempts-1)
			input.Context = appendAgentSDKGenerationLengthRetryContext(input.Context, targetWords, attempt+1)
			continue
		}
		if err == nil && agentSDKWriteShouldRetryShortfall(output.Content, targetWords) && canRetry {
			logger.Warn("Chapter %s agent-sdk generation fell below preferred length; retrying with fuller scene instruction (attempt %d/%d)", chapter.ID, attempt+1, maxAttempts-1)
			input.Context = appendAgentSDKGenerationShortfallRetryContext(input.Context, targetWords)
			continue
		}
		if err == nil || !isAgentSDKWriteLengthOvershootError(err) || targetWords <= 0 || !canRetry {
			break
		}
		logger.Warn("Chapter %s agent-sdk generation exceeded target length; retrying with stricter length instruction (attempt %d/%d)", chapter.ID, attempt+1, maxAttempts-1)
		input.Context = appendAgentSDKGenerationLengthRetryContext(input.Context, targetWords, attempt+1)
	}
	if err != nil {
		return "", err
	}
	if err := validateAgentSDKWriteFinalMinimum(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s agent-sdk length warning: %s", chapter.ID, warn)
	}
	if err := a.logWriteContext(chapter.ID, "final-agent-sdk", input, output.Content); err != nil {
		logger.Warn("Failed to log write agent-sdk context: %v", err)
	}
	logger.Info("[ok] Agent SDK generated final chapter: %d characters", len(output.Content))
	return output.Content, nil
}

func (a *WriteAgent) executeAgentSDKChapterGeneration(ctx context.Context, params InvokeParams, input WriteGenInput, chapter *models.Chapter, targetWords int) (WriteGenOutput, error) {
	var output WriteGenOutput
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return output, err
	}
	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return output, err
	}
	if err := validateAgentSDKWriteTargetLength(chapter, output.Content, targetWords); err != nil {
		return output, err
	}
	return output, nil
}

func appendAgentSDKGenerationLengthRetryContext(contextText string, targetWords int, attempt int) string {
	instruction := agentSDKWriteLengthRetryInstruction(targetWords, attempt)
	if strings.TrimSpace(contextText) == "" {
		return "## Agent SDK Length Retry\n\n" + instruction
	}
	return strings.TrimSpace(contextText) + "\n\n## Agent SDK Length Retry\n\n" + instruction
}

func appendAgentSDKGenerationShortfallRetryContext(contextText string, targetWords int) string {
	instruction := agentSDKWriteShortfallRetryInstruction(targetWords)
	if strings.TrimSpace(contextText) == "" {
		return "## Agent SDK Length Shortfall Retry\n\n" + instruction
	}
	return strings.TrimSpace(contextText) + "\n\n## Agent SDK Length Shortfall Retry\n\n" + instruction
}

func appendAgentSDKInitialLengthContext(contextText string, targetWords int) string {
	instruction := agentSDKWriteInitialLengthInstruction(targetWords)
	if instruction == "" {
		return contextText
	}
	if strings.TrimSpace(contextText) == "" {
		return "## Agent SDK Length Budget\n\n" + instruction
	}
	return strings.TrimSpace(contextText) + "\n\n## Agent SDK Length Budget\n\n" + instruction
}

func isAgentSDKWriteLengthOvershootError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "agent-sdk returned too much content")
}

func agentSDKWriteShouldRetryLength(content string, targetWords int) bool {
	if targetWords <= 0 {
		return false
	}
	return narrativeUnitCount(content) > agentSDKWritePreferredHardMax(targetWords)
}

func agentSDKWriteShouldRetryShortfall(content string, targetWords int) bool {
	if targetWords <= 0 {
		return false
	}
	return narrativeUnitCount(content) < agentSDKWritePreferredSoftMin(targetWords)
}

func agentSDKWriteInitialLengthInstruction(targetWords int) string {
	if targetWords <= 0 {
		return ""
	}
	low := int(float64(targetWords) * 0.95)
	high := int(float64(targetWords) * 1.05)
	preferredHardMax := agentSDKWritePreferredHardMax(targetWords)
	absoluteHardMax := agentSDKWriteAbsoluteHardMax(targetWords)
	minParagraphs, maxParagraphs, paraLow, paraHigh := agentSDKWriteParagraphBudget(targetWords)
	return fmt.Sprintf("First response length budget: target_words=%d narrative units, preferred range %d-%d, initial hard max %d, absolute validation max %d. Use a compact scene budget of %d-%d paragraphs, about %d-%d narrative units per paragraph. Cover required beats by merging actions into dense paragraphs instead of adding new scenes. Stay inside the preferred range when possible; do not use history, future hooks, world exposition, extra dialogue, or explanatory summaries to expand beyond the target. The final JSON object must contain only the content field; do not add chapter_id, title, word_count, notes, or metadata fields.", targetWords, low, high, preferredHardMax, absoluteHardMax, minParagraphs, maxParagraphs, paraLow, paraHigh)
}

func agentSDKWriteParagraphBudget(targetWords int) (int, int, int, int) {
	if targetWords <= 0 {
		return 0, 0, 0, 0
	}
	minParagraphs := targetWords / 190
	if minParagraphs < 4 {
		minParagraphs = 4
	}
	if minParagraphs > 12 {
		minParagraphs = 12
	}
	maxParagraphs := minParagraphs + 2
	if maxParagraphs > 14 {
		maxParagraphs = 14
	}
	paraLow := targetWords / maxParagraphs
	paraHigh := targetWords / minParagraphs
	return minParagraphs, maxParagraphs, paraLow, paraHigh
}

func agentSDKWriteLengthRetryInstruction(targetWords int, attempt int) string {
	if targetWords <= 0 {
		return "Rewrite shorter and return only the final JSON content field. Tool results are already visible in the conversation; do not read Claude tool-results temporary files, use shell wrappers, or output prose through Bash/echo/printf."
	}
	low := int(float64(targetWords) * 0.9)
	high := int(float64(targetWords) * 1.1)
	hardMax := agentSDKWritePreferredHardMax(targetWords)
	if attempt >= 2 {
		compressedLow := int(float64(targetWords) * 0.75)
		compressedHigh := int(float64(targetWords) * 0.95)
		failMax := targetWords + 100
		minParagraphs, maxParagraphs, paraLow, paraHigh := agentSDKWriteParagraphBudget(targetWords)
		return fmt.Sprintf("The previous retry was still too long. This is the final compression attempt: produce %d-%d narrative units, and treat %d as a fail-fast ceiling even though validation hard max is %d. Use only %d-%d paragraphs, about %d-%d narrative units per paragraph. Preserve only required events and payoff; cut side description, repeated analysis, extra dialogue, history-derived expansion, world exposition, setup recaps, and future-hook explanation. Do not run any Bash command to output, count, store, echo, printf, or inspect the prose. Tool results are already visible in the conversation; do not read Claude tool-results temporary files or use shell wrappers. Return only a JSON object with the content field; do not add chapter_id, title, word_count, notes, or metadata.", compressedLow, compressedHigh, failMax, hardMax, minParagraphs, maxParagraphs, paraLow, paraHigh)
	}
	minParagraphs, maxParagraphs, paraLow, paraHigh := agentSDKWriteParagraphBudget(targetWords)
	return fmt.Sprintf("The previous attempt was too long. Rewrite the chapter near %d narrative units, preferably %d-%d, and never exceed %d. Use %d-%d compact paragraphs, about %d-%d narrative units each. Treat target_words as a hard budget; history, context, and future hooks must not increase the chapter length. Do not run Bash/echo/printf to output, count, store, or inspect prose. Return only the final JSON content field; do not add extra scenes, appendices, analysis, command logs, metadata fields, or shell output.", targetWords, low, high, hardMax, minParagraphs, maxParagraphs, paraLow, paraHigh)
}

func agentSDKWriteShortfallRetryInstruction(targetWords int) string {
	if targetWords <= 0 {
		return "The previous attempt was too short. Add necessary scene-level prose while preserving the same chapter events, then return only the final JSON content field."
	}
	low := int(float64(targetWords) * 0.95)
	high := int(float64(targetWords) * 1.05)
	hardMax := agentSDKWritePreferredHardMax(targetWords)
	minParagraphs, maxParagraphs, paraLow, paraHigh := agentSDKWriteParagraphBudget(targetWords)
	return fmt.Sprintf("The previous attempt was too short for the requested chapter budget. Rewrite near %d narrative units, preferably %d-%d, without exceeding %d. Preserve the exact outline beats and do not add new plot events. Add only useful scene-level texture: concrete reactions, transitions, system-log observations, emotional stakes, and cause/effect connective tissue. Use %d-%d compact paragraphs, about %d-%d narrative units each. Do not run Bash/echo/printf to output, count, store, or inspect prose. Return only the final JSON content field; do not add metadata fields or shell output.", targetWords, low, high, hardMax, minParagraphs, maxParagraphs, paraLow, paraHigh)
}

func writeAgentSDKParams(command string, chapterID string, useHistory bool, targetWords ...int) InvokeParams {
	command = appendWriteAgentSDKToolRules(command)
	if instruction := agentSDKWriteInitialLengthInstruction(optionalFirstInt(targetWords)); instruction != "" {
		command = strings.TrimSpace(command) + " " + instruction
	}
	chapterID = strings.TrimSpace(chapterID)
	requiredContextQuery := writeGenRequiredContextQuery(chapterID)
	if requiredContextQuery != "" {
		command = fmt.Sprintf("%s Before writing, run exactly this small context sanity query: `%s`. Use the typed input for chapter facts, recap, adjacent context, and next-chapter hook; do not run the plain chapter-write brief query, --view full, or any file/temp-result read.", command, requiredContextQuery)
	}
	if useHistory {
		command = strings.TrimSpace(command) + " History continuation is requested: first inspect copied logs with `novelgen tool query logs --view index --limit 5`; by default use the index summaries only, and read at most one exact log brief if the index clearly shows a relevant completed prior run. Prefer prompt/response history when agent-live logs are absent. Do not read log content for chapter generation. History is reference only and must not increase target_words."
	}
	evidence := ToolEvidenceRequirement{MinContextQueryCalls: 1}
	evidence.RequireNoDeniedTools = true
	if requiredContextQuery != "" {
		evidence.RequiredToolCommands = []string{requiredContextQuery}
	}
	if useHistory {
		evidence.MinQueryCalls = 2
		evidence.RequiredToolCommands = append(evidence.RequiredToolCommands, "novelgen tool query logs --view index")
	}
	return InvokeParams{
		SDKSkills:      []string{"write-chapter-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  writeGenToolAllowlist(chapterID, useHistory),
		ToolEvidence:   evidence,
		MaxTurns:       18,
		Timeout:        600,
		Command:        command,
	}
}

func appendWriteAgentSDKToolRules(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "use focused project tools"
	}
	return command + ". Tool outputs are already returned in the conversation; never read Claude tool-results temporary files, run Get-Content/type/cat, pipe, redirect, create temp files, use echo/printf to output JSON, or wrap an allowed novelgen command in shell syntax. After a required query succeeds, do not rerun it, filter it with head/Select-String, summarize it with shell commands, or inspect temp tool output. If a tool is denied, use the last allowed tool result and return the required JSON directly as the assistant response."
}

func optionalFirstInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func writeHistoryRequested(values []bool) bool {
	return len(values) > 0 && values[0]
}

func writeAgentSDKHistoryMode(useHistory bool) string {
	if !useHistory {
		return ""
	}
	return "历史续写已启用。先查询 agent-live 日志索引；默认只使用 index 里的摘要，只有索引明确显示某条已完成旧运行高度相关时，最多读取 1 条 brief，不要读取 content。历史只作为创作意图、风格、失败教训和工具策略参考，不能增加 target_words，不要复制旧输出，不要把命令记录写进正文。"
}

// GenerateChapterWithSuggestions generates improved chapter content with review suggestions
func (a *WriteAgent) GenerateChapterWithSuggestions(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentDraft string, suggestions string, iteration ...int) (string, error) {
	logger.Section("WRITE AGENT - Chapter Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	iter := 1
	if len(iteration) > 0 && iteration[0] > 0 {
		iter = iteration[0]
	}

	input := WriteImproveInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		Continuity:   formatContinuityForWrite(continuity, chapter),
		TargetWords:  targetWords,
		Iteration:    iter,
		CurrentDraft: currentDraft,
		Suggestions:  suggestions,
		Context:      formatChapterContext(context),
		Recap:        recapForContext(context),
		NextChapters: buildNextChapterInfos(context),
	}

	var output WriteImproveOutput
	params := InvokeParams{
		Skills:  []string{"write-improve"},
		Command: "improve chapter based on suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s improvement length warning: %s", chapter.ID, warn)
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "improve", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated improved chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateChapterWithSuggestionsAgentSDK improves chapter content through the Agent SDK.
// The agent may query focused facts and checks, but Go remains the only writer.
func (a *WriteAgent) GenerateChapterWithSuggestionsAgentSDK(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentDraft string, suggestions string, applyPatches bool, useHistory bool, iteration ...int) (string, error) {
	logger.Section("WRITE AGENT SDK - Chapter Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)
	if applyPatches {
		logger.Info("Agent apply enabled: SDK may write through validated chapter patch tools")
	}

	iter := 1
	if len(iteration) > 0 && iteration[0] > 0 {
		iter = iteration[0]
	}

	input := WriteImproveInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		Continuity:   formatContinuityForWrite(continuity, chapter),
		TargetWords:  targetWords,
		Iteration:    iter,
		ApplyPatches: applyPatches,
		CurrentDraft: currentDraft,
		Suggestions:  suggestions,
		HistoryMode:  writeAgentSDKHistoryMode(useHistory),
		Context:      appendAgentSDKInitialLengthContext(formatChapterContext(context), targetWords),
		Recap:        recapForContext(context),
		NextChapters: buildNextChapterInfos(context),
	}

	var output WriteImproveOutput
	params := writeImproveAgentSDKParams("improve final chapter content using focused project tools", chapter.ID, applyPatches, useHistory, targetWords)
	runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, input, &output)
	if err != nil {
		return "", err
	}
	if applyPatches && agentSDKPatchApplyCount(runtimeResult) == 0 {
		logger.Info("[ok] Agent SDK made no patch apply for %s; preserving current draft", chapter.ID)
		return currentDraft, nil
	}

	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if err := validateAgentSDKWriteTargetLength(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s agent-sdk improvement length warning: %s", chapter.ID, warn)
	}

	if err := a.logWriteContext(chapter.ID, "improve-agent-sdk", input, output.Content); err != nil {
		logger.Warn("Failed to log write agent-sdk improve context: %v", err)
	}

	logger.Info("[ok] Agent SDK generated improved chapter: %d characters", len(output.Content))
	return output.Content, nil
}

func agentSDKPatchApplyCount(result *agentruntime.Result) int {
	if result == nil || result.LiveSummary == nil {
		return 0
	}
	return result.LiveSummary.PatchApplies
}

func writeImproveAgentSDKParams(command string, chapterID string, applyPatches bool, useHistory bool, targetWords ...int) InvokeParams {
	targetWordCount := optionalFirstInt(targetWords)
	command = appendWriteAgentSDKToolRules(command)
	if instruction := agentSDKWriteInitialLengthInstruction(targetWordCount); instruction != "" {
		command = strings.TrimSpace(command) + " " + instruction
	}
	chapterID = strings.TrimSpace(chapterID)
	if chapterID != "" {
		checkCommand := fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID)
		if targetWordCount > 0 {
			checkCommand += fmt.Sprintf(" --target-words %d", targetWordCount)
		}
		command = fmt.Sprintf("%s. First inspect the focused chapter with `novelgen tool query context --type chapter-repair --id %q --view brief`. Before any patch-buffer or patch command, run `%s`.", command, chapterID, checkCommand)
	}
	evidence := ToolEvidenceRequirement{
		MinContextQueryCalls: 1,
		MinCheckCalls:        1,
	}
	if applyPatches {
		command += " apply_patches=true: do not run patch-buffer or patch commands until the exact focused chapter check above, including target_words when present, has completed. If the user request is conditional, exploratory, or says to keep the text when no issue is found, you may return final JSON without patching after the focused chapter/context fact query proves no edit should be made. IMPORTANT: if the focused chapter check reports zero issues (no critical/high/medium) and no concrete defect is named, YOU ARE DONE — return final JSON immediately with the current draft unchanged; do not re-query, do not re-check, do not loop. If the user request says not to modify when named characters, abilities, items, or settings are absent from project facts, verify those entities from chapter-repair/chapter-write context and exact craft-character/craft-item/craft-location queries before any patch; there is no craft-ability query. When verification fails, return the current draft unchanged and do not repair unrelated length/style/check issues. Only patch a clean chapter when the input suggestions or user request names a concrete prose defect or exact edit target; then create a minimal chapter patch, dry-run it, and apply the same validated patch with --apply --refresh-derived before returning final JSON. Never emit plans, status JSON, prose, or final JSON through Bash/printf/echo. After --apply --refresh-derived, use the returned check result directly; do not read task/tool-result temp files, run type/Get-Content/cat, run tool refresh again, or inspect saved chapter files."
		if targetWordCount > 0 {
			command += fmt.Sprintf(" Because target_words=%d is active, every `novelgen tool patch chapter` dry-run and apply command must include `--target-words %d`; patch-buffer clear/append commands do not take this flag.", targetWordCount, targetWordCount)
		}
		evidence.MinCheckCalls = 0
		evidence.RequirePatchApplyFollowupCheck = true
		evidence.RequireNoDeniedTools = true
	}
	if useHistory {
		command += " History continuation is requested: inspect copied logs with `novelgen tool query logs --view index --limit 5` before deciding the improvement strategy; by default use the index summaries only, and read at most one exact log brief if the index clearly shows a relevant completed prior run. Prefer prompt/response history when agent-live logs are absent. Do not read log content. History is reference only and must not increase target_words."
		evidence.MinQueryCalls = maxInt(evidence.MinQueryCalls, 2)
		evidence.RequiredToolCommands = append(evidence.RequiredToolCommands, "novelgen tool query logs --view index")
	}
	return InvokeParams{
		SDKSkills:      []string{"novel-tools-core", "write-improve-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  writeImproveToolAllowlist(chapterID, applyPatches, targetWordCount),
		ToolEvidence:   evidence,
		MaxTurns:       writeImproveAgentSDKMaxTurns(applyPatches),
		Timeout:        600,
		Command:        command,
	}
}

func writeImproveAgentSDKMaxTurns(applyPatches bool) int {
	if applyPatches {
		return 64
	}
	return 32
}

func writeGenToolAllowlist(chapterID string, useHistory bool) []string {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return nil
	}
	allowlist := []string{
		writeGenRequiredContextQuery(chapterID),
	}
	if useHistory {
		allowlist = append(allowlist, agentSDKLogToolAllowlist()...)
	}
	return dedupeWriteToolAllowlist(allowlist)
}

func writeGenRequiredContextQuery(chapterID string) string {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ""
	}
	return fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief --fields path,navigation,stats,warnings", chapterID)
}

func writeImproveToolAllowlist(chapterID string, applyPatches bool, targetWords ...int) []string {
	allowlist := writeFocusedQueryAllowlist(chapterID)
	chapterID = strings.TrimSpace(chapterID)
	if chapterID != "" {
		targetWordCount := optionalFirstInt(targetWords)
		targetWordsFlag := ""
		if targetWordCount > 0 {
			targetWordsFlag = fmt.Sprintf(" --target-words %d", targetWordCount)
		}
		bufferID := chapterID + "-draft"
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query chapter --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool query chapter --id %q --content", chapterID),
			fmt.Sprintf("novelgen tool query chapter --id %q --content --view brief", chapterID),
			fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --max-issues 8", chapterID),
			fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --max-issues 8", chapterID),
			fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
			fmt.Sprintf("novelgen tool patch-buffer --id %q", bufferID),
		)
		if targetWordCount > 0 {
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool check quality --target chapter --scope chapter --id %q --max-issues 8%s", chapterID, targetWordsFlag),
				fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --max-issues 8%s", chapterID, targetWordsFlag),
				fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12%s", chapterID, targetWordsFlag),
				fmt.Sprintf("novelgen tool patch chapter --id %q%s", chapterID, targetWordsFlag),
				fmt.Sprintf("novelgen tool patch chapter --id %q --patch-buffer %q%s", chapterID, bufferID, targetWordsFlag),
			)
		} else {
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool check quality --target chapter --scope chapter --id %q --max-issues 8", chapterID),
				fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --max-issues 8", chapterID),
				fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
				fmt.Sprintf("novelgen tool patch chapter --id %q", chapterID),
				fmt.Sprintf("novelgen tool patch chapter --id %q --patch-buffer %q", chapterID, bufferID),
			)
		}
		if applyPatches {
			if targetWordCount > 0 {
				allowlist = append(allowlist,
					fmt.Sprintf("novelgen tool patch chapter --id %q --apply --refresh-derived%s", chapterID, targetWordsFlag),
					fmt.Sprintf("novelgen tool patch chapter --id %q --patch-buffer %q --apply --refresh-derived%s", chapterID, bufferID, targetWordsFlag),
				)
			} else {
				allowlist = append(allowlist,
					fmt.Sprintf("novelgen tool patch chapter --id %q --apply --refresh-derived", chapterID),
					fmt.Sprintf("novelgen tool patch chapter --id %q --patch-buffer %q --apply --refresh-derived", chapterID, bufferID),
				)
			}
		}
	}
	return dedupeWriteToolAllowlist(allowlist)
}

func writeFocusedQueryAllowlist(chapterID string) []string {
	chapterID = strings.TrimSpace(chapterID)
	allowlist := []string{
		"novelgen tool query context --type craft-character",
		"novelgen tool query context --type craft-item",
		"novelgen tool query context --type craft-location",
	}
	allowlist = append(allowlist, agentSDKLogToolAllowlist()...)
	if chapterID != "" {
		allowlist = append([]string{
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view index", chapterID),
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief --fields existing_chapter_excerpt", chapterID),
			fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q", chapterID),
		}, allowlist...)
	}
	return dedupeWriteToolAllowlist(allowlist)
}

func dedupeWriteToolAllowlist(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// ReviewChapter reviews a chapter and provides improvement suggestions
func (a *WriteAgent) ReviewChapter(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, content string, targetWords int, iteration int) (models.ReviewResult, error) {
	logger.Section("WRITE AGENT - Chapter Review")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Iteration: %d", iteration)
	logger.Info("Language: %s", a.base.language)

	recap := ""
	contextText := ""
	nextInfos := []NextChapterInfo(nil)
	if context != nil {
		recap = context.Recap
		contextText = formatChapterContext(context)
		nextInfos = buildNextChapterInfos(context)
	}

	input := WriteReviewInput{
		StorySetup:     ToCompact(a.setup),
		Continuity:     formatContinuityForWrite(continuity, chapter),
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		ChapterStats:   formatWriteReviewChapterStats(content, targetWords),
		Iteration:      iteration,
		Context:        contextText,
		Recap:          recap,
		NextChapters:   nextInfos,
	}

	var output WriteReviewOutput
	params := InvokeParams{
		Skills:  []string{"write-review"},
		Command: "review chapter and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Chapter Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// ReviewChapterWithAgentSDK reviews a saved final chapter through the Agent SDK.
// The agent may query focused project facts and checks, but it cannot write files.
func (a *WriteAgent) ReviewChapterWithAgentSDK(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, content string, targetWords int, iteration int) (models.ReviewResult, error) {
	return a.ReviewChapterWithAgentSDKFocus(ctx, chapter, context, continuity, content, targetWords, iteration, "")
}

// ReviewChapterWithAgentSDKFocus 同 ReviewChapterWithAgentSDK, 但支持 --focus 审查视角(复用 compose review 的 focus, 如 deai/protagonist)。
func (a *WriteAgent) ReviewChapterWithAgentSDKFocus(ctx context.Context, chapter *models.Chapter, context *ChapterContext, continuity *models.ChapterContinuity, content string, targetWords int, iteration int, focus string) (models.ReviewResult, error) {
	logger.Section("WRITE AGENT SDK - Chapter Review")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Iteration: %d", iteration)
	logger.Info("Language: %s", a.base.language)

	recap := ""
	contextText := ""
	nextInfos := []NextChapterInfo(nil)
	if context != nil {
		recap = context.Recap
		contextText = formatChapterContext(context)
		nextInfos = buildNextChapterInfos(context)
	}

	input := WriteReviewInput{
		StorySetup:     ToCompact(a.setup),
		Continuity:     formatContinuityForWrite(continuity, chapter),
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		ChapterStats:   formatWriteReviewChapterStats(content, targetWords),
		Iteration:      iteration,
		Context:        contextText,
		Recap:          recap,
		NextChapters:   nextInfos,
	}

	var output writeAgentSDKReviewOutput
	reviewCommand := "review final chapter content using focused project tools"
	if strings.TrimSpace(focus) != "" {
		focusPrompt := ResolveReviewFocusPrompt(focus)
		if strings.TrimSpace(focusPrompt) != "" {
			reviewCommand += ". Additional review focus (最高优先级审查视角): " + focusPrompt
		}
	}
	params := writeReviewAgentSDKParams(reviewCommand, chapter.ID, targetWords)
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return models.ReviewResult{}, err
	}

	result := output.Result.toModelReviewResult()
	result.NormalizeScoreScale()
	logger.Section("Chapter Agent SDK Review Result")
	logger.Info("Overall Score: %.1f/100", result.OverallScore)
	logger.Info("Suggestions: %d", len(result.Suggestions))
	return result, nil
}

func (r writeAgentSDKReviewResult) toModelReviewResult() models.ReviewResult {
	result := models.ReviewResult{
		OverallScore: r.OverallScore,
		Summary:      clipForPrompt(r.Summary, 500),
	}
	for _, strength := range r.Strengths {
		if strings.TrimSpace(strength) == "" {
			continue
		}
		result.Strengths = append(result.Strengths, clipForPrompt(strength, 180))
		if len(result.Strengths) >= 4 {
			break
		}
	}
	for _, weakness := range r.Weaknesses {
		if strings.TrimSpace(weakness) == "" {
			continue
		}
		result.Weaknesses = append(result.Weaknesses, clipForPrompt(weakness, 220))
		if len(result.Weaknesses) >= 4 {
			break
		}
	}
	for _, suggestion := range r.Suggestions {
		result.Suggestions = append(result.Suggestions, models.ReviewSuggestion{
			Category:   strings.TrimSpace(suggestion.Category),
			TargetID:   strings.TrimSpace(suggestion.TargetID),
			TargetName: clipForPrompt(suggestion.TargetName, 120),
			Issue:      clipForPrompt(suggestion.Issue, 360),
			Suggestion: clipForPrompt(suggestion.Suggestion, 420),
			Priority:   strings.TrimSpace(suggestion.Priority),
		})
		if len(result.Suggestions) >= 8 {
			break
		}
	}
	return result
}

func formatWriteReviewChapterStats(content string, targetWords int) string {
	count := narrativeUnitCount(content)
	if targetWords <= 0 {
		return fmt.Sprintf("current_narrative_units=%d; target_words=not_set; length_review_rule=do_not_estimate_length_manually", count)
	}
	low := int(float64(targetWords) * 0.95)
	high := int(float64(targetWords) * 1.05)
	hardMax := agentSDKWriteAbsoluteHardMax(targetWords)
	hardMin := agentSDKWriteAbsoluteHardMin(targetWords)
	status := "within_preferred_range"
	if count < hardMin {
		status = "below_hard_min"
	} else if count > hardMax {
		status = "above_hard_max"
	} else if count < low {
		status = "below_preferred_range"
	} else if count > high {
		status = "above_preferred_range"
	}
	return fmt.Sprintf("current_narrative_units=%d; target_words=%d; preferred_range=%d-%d; hard_range=%d-%d; status=%s; length_review_rule=trust_these_counts_and_tool_check_do_not_estimate_length_manually", count, targetWords, low, high, hardMin, hardMax, status)
}

func writeReviewAgentSDKParams(command string, chapterID string, targetWords ...int) InvokeParams {
	allowlist := writeFocusedQueryAllowlist(chapterID)
	chapterID = strings.TrimSpace(chapterID)
	if chapterID != "" {
		targetWordCount := optionalFirstInt(targetWords)
		targetWordsFlag := ""
		if targetWordCount > 0 {
			targetWordsFlag = fmt.Sprintf(" --target-words %d", targetWordCount)
			command = fmt.Sprintf("%s Treat target_words=%d as the chapter length budget for this review.", command, targetWordCount)
		}
		checkCommand := fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --max-issues 8%s", chapterID, targetWordsFlag)
		command = fmt.Sprintf("%s. Before reviewing, run exactly these tools first: `novelgen tool query context --type chapter-repair --id %q --view brief`, then `%s`. Do not run alternate check command shapes.", command, chapterID, checkCommand)
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query outline --type chapter --id %q", chapterID),
			fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q", chapterID),
			fmt.Sprintf("novelgen tool check quality --target chapter --scope chapter --id %q --max-issues 8%s", chapterID, targetWordsFlag),
			fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --max-issues 8", chapterID),
			checkCommand,
			fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --max-issues 8", chapterID),
			fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
		)
	}
	return InvokeParams{
		SDKSkills:      []string{"novel-tools-core", "write-review-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  dedupeWriteToolAllowlist(allowlist),
		ToolEvidence:   ToolEvidenceRequirement{MinContextQueryCalls: 1, MinCheckCalls: 1, RequireNoDeniedTools: true},
		MaxTurns:       18,
		Timeout:        600,
		Command:        command,
	}
}

// ReviewVolume performs a holistic review of all chapters in a volume
func (a *WriteAgent) ReviewVolume(ctx context.Context, volume *models.Volume, chapters []*models.Chapter, chapterContents map[string]string, targetWords int) (VolumeReviewResult, error) {
	logger.Section("WRITE AGENT - Volume Review")
	logger.Info("Volume: %s - %s", volume.ID, volume.Title)
	logger.Info("Chapters: %d", len(chapters))
	logger.Info("Language: %s", a.base.language)

	// Build chapter inputs
	var chapterInputs []VolumeReviewChapterInput
	for _, chapter := range chapters {
		content := chapterContents[chapter.ID]
		if content == "" {
			logger.Warn("No content for chapter %s, skipping in volume review", chapter.ID)
			continue
		}

		// Beats are already strings
		beats := chapter.GetBeats()

		chapterInputs = append(chapterInputs, VolumeReviewChapterInput{
			ChapterID:    chapter.ID,
			ChapterTitle: chapter.Title,
			Summary:      chapter.Summary,
			Content:      content,
			Beats:        beats,
		})
	}

	if len(chapterInputs) == 0 {
		return VolumeReviewResult{}, fmt.Errorf("no chapters with content to review")
	}

	input := VolumeReviewInput{
		StorySetup:            ToCompact(a.setup),
		VolumeID:              volume.ID,
		VolumeTitle:           volume.Title,
		VolumeSummary:         volume.Summary,
		Chapters:              chapterInputs,
		TargetWordsPerChapter: targetWords,
	}

	var output VolumeReviewOutput
	params := InvokeParams{
		Skills:  []string{"volume-review"},
		Command: "review entire volume holistically and provide chapter-level suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return VolumeReviewResult{}, err
	}

	logger.Section("Volume Review Result")
	logger.Info("Overall Volume Score: %d/100", output.Result.OverallScore)
	logger.Info("Chapters Reviewed: %d", len(output.Result.ChapterReviews))
	logger.Info("Volume Level Issues: %d", len(output.Result.VolumeLevelIssues))
	logger.Info("Volume Level Suggestions: %d", len(output.Result.VolumeLevelSuggestions))

	return output.Result, nil
}

// logWriteContext logs the write context to a markdown file for debugging
func (a *WriteAgent) logWriteContext(chapterID, variant string, input interface{}, output string) error {
	debugDir := filepath.Join("logs", "write_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.md", chapterID, variant, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Write Context: %s (%s)\n\n", chapterID, variant))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## Input\n\n")
	sb.WriteString("```json\n")
	if b, err := json.MarshalIndent(input, "", "  "); err == nil {
		sb.Write(b)
	} else {
		sb.WriteString(fmt.Sprintf("%+v", input))
	}
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Output\n\n")
	sb.WriteString("```\n")
	sb.WriteString(output)
	sb.WriteString("\n```\n")

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

// formatChapterContext formats the chapter context for the prompt
func formatChapterContext(context *ChapterContext) string {
	if context == nil {
		return ""
	}
	var sb strings.Builder
	if context == nil {
		return ""
	}

	if strings.TrimSpace(context.Craft) != "" {
		sb.WriteString("CRAFT CONTEXT:\n")
		sb.WriteString(strings.TrimSpace(context.Craft))
		sb.WriteString("\n")
	}

	if len(context.Previous) > 0 {
		sb.WriteString("PREVIOUS CHAPTERS:\n")
		for _, prev := range context.Previous {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", prev.Chapter.ID, prev.Chapter.Title))
			sb.WriteString(prev.Content)
			sb.WriteString("\n")
		}
	}

	if len(context.Next) > 0 {
		sb.WriteString("\nUPCOMING CHAPTERS (for foreshadowing):\n")
		for _, next := range context.Next {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", next.Chapter.ID, next.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", next.Chapter.Summary))
		}
	}

	return sb.String()
}

func buildNextChapterInfos(context *ChapterContext) []NextChapterInfo {
	if context == nil || len(context.Next) == 0 {
		return nil
	}
	nextInfos := make([]NextChapterInfo, 0, len(context.Next))
	for _, nc := range context.Next {
		if nc == nil || nc.Chapter == nil {
			continue
		}
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.Chapter.ID,
			Title:   nc.Chapter.Title,
			Summary: nc.Chapter.Summary,
		})
	}
	return nextInfos
}

func recapForContext(context *ChapterContext) string {
	if context == nil {
		return ""
	}
	return context.Recap
}

func validateWriteContent(chapter *models.Chapter, content string, targetWords int) error {
	chapterID := ""
	if chapter != nil {
		chapterID = chapter.ID
	}
	clean := strings.TrimSpace(content)
	if clean == "" {
		return fmt.Errorf("AI returned empty content for chapter %s", chapterID)
	}
	if strings.HasPrefix(clean, "{") && strings.Contains(clean, `"content"`) {
		return fmt.Errorf("AI returned JSON text as chapter prose for chapter %s", chapterID)
	}
	if strings.Contains(clean, "```") {
		return fmt.Errorf("AI returned markdown code fence in chapter prose for chapter %s", chapterID)
	}
	if targetWords > 0 && narrativeUnitCount(clean) < minAcceptableNarrativeUnits(targetWords) {
		return fmt.Errorf("AI returned too little content for chapter %s: got %d narrative units, target %d", chapterID, narrativeUnitCount(clean), targetWords)
	}
	return nil
}

func validateWriteTargetLength(content string, targetWords int) string {
	if targetWords <= 0 {
		return ""
	}
	count := narrativeUnitCount(content)
	low := int(float64(targetWords) * 0.95)
	high := int(float64(targetWords) * 1.05)
	if count < low || count > high {
		return fmt.Sprintf("got %d narrative units, target %d, preferred range %d-%d", count, targetWords, low, high)
	}
	return ""
}

func validateAgentSDKWriteTargetLength(chapter *models.Chapter, content string, targetWords int) error {
	if targetWords <= 0 {
		return nil
	}
	count := narrativeUnitCount(content)
	hardMax := agentSDKWriteAbsoluteHardMax(targetWords)
	if count > hardMax {
		chapterID := ""
		if chapter != nil {
			chapterID = chapter.ID
		}
		return fmt.Errorf("agent-sdk returned too much content for chapter %s: got %d narrative units, target %d, hard max %d", chapterID, count, targetWords, hardMax)
	}
	return nil
}

func validateAgentSDKWriteFinalMinimum(chapter *models.Chapter, content string, targetWords int) error {
	if targetWords <= 0 {
		return nil
	}
	count := narrativeUnitCount(content)
	hardMin := agentSDKWriteAbsoluteHardMin(targetWords)
	if count < hardMin {
		chapterID := ""
		if chapter != nil {
			chapterID = chapter.ID
		}
		return fmt.Errorf("agent-sdk returned too little content for chapter %s after retries: got %d narrative units, target %d, hard min %d", chapterID, count, targetWords, hardMin)
	}
	return nil
}

func agentSDKWritePreferredHardMax(targetWords int) int {
	return int(float64(targetWords) * 1.2)
}

func agentSDKWritePreferredSoftMin(targetWords int) int {
	return int(float64(targetWords) * 0.9)
}

func agentSDKWriteAbsoluteHardMax(targetWords int) int {
	return int(float64(targetWords) * 1.5)
}

func agentSDKWriteAbsoluteHardMin(targetWords int) int {
	min := int(float64(targetWords) * 0.8)
	if floor := minAcceptableNarrativeUnits(targetWords); min < floor {
		min = floor
	}
	return min
}

func minAcceptableNarrativeUnits(targetWords int) int {
	min := targetWords / 2
	if min < 120 {
		min = 120
	}
	return min
}

func narrativeUnitCount(content string) int {
	hasCJK := false
	cjkCount := 0
	for _, r := range content {
		if isCJK(r) {
			hasCJK = true
			cjkCount++
		}
	}
	if hasCJK {
		return cjkCount
	}
	return len(strings.Fields(content))
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// formatContinuityForWrite formats the continuity snapshot for the prompt.
func formatContinuityForWrite(continuity *models.ChapterContinuity, chapter *models.Chapter) string {
	return logic.FormatChapterContinuity(continuity, chapter)
}
