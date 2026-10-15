package writepipe

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/toolplane"
)

// Try to load final chapter first

// Check for final chapter. Prefer full chapter IDs, but keep the old
// numeric filename as a compatibility fallback for existing projects.

// Fallback to draft

// LoadChapterContext loads surrounding chapter content for context.
// Prefers final chapters over drafts when available.

// Find target chapter index

// Load previous chapters (prefer final, fallback to draft, then outline)

// If no final/draft, build from outline

// Ensure there's at least an offline recap persisted for the previous
// chapter so later steps (e.g., transition checks / future drafts) can
// use it even when generation is out-of-order.

// Note: Recap extraction is handled separately via recap command

// Load next chapters (prefer final, fallback to draft, then outline)

// If no final/draft, build from outline

// buildChapterContentFromOutline builds chapter content from outline data
// Used as fallback when no draft or final content exists

// SaveFinalChapter saves the generated final chapter content
// SaveFinalChapter saves the generated final chapter content.

// ChapterNarrativeUnitsForLog counts the narrative units the project reports
// for a chapter, used to compare generated length against the target.

// RecapExtractor is the recap-agent surface the write pipeline depends on. Both
// the chat and Agent SDK backends satisfy it.

// ExtractAndSaveRecapWithGate extracts the chapter recap, retries once with
// gate feedback when the minimal validation fails, and persists the result.

// GenerateImprovedChapter runs one improvement pass, applying the Agent SDK
// length-overshoot retry and post-error recovery rules when the SDK path is
// used.
func GenerateImprovedChapter(ctx context.Context, agent *agents.WriteAgent, chapter *models.Chapter, context *agents.ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentContent string, suggestions string, useAgentSDK bool, agentApply bool, useAgentHistory bool, iteration ...int) (string, error) {
	if useAgentSDK {
		effectiveTargetWords := EffectiveAgentSDKImproveTargetWords(chapter, targetWords, currentContent, suggestions, useAgentSDK)
		content, err := agent.GenerateChapterWithSuggestionsAgentSDK(ctx, chapter, context, continuity, effectiveTargetWords, currentContent, suggestions, agentApply, useAgentHistory, iteration...)
		if err != nil && IsAgentSDKLengthOvershootError(err) {
			logger.GetLogger().Warn("Agent SDK chapter improvement overshot target length; retrying once with strict minimal-repair length guidance")
			content, err = agent.GenerateChapterWithSuggestionsAgentSDK(ctx, chapter, context, continuity, effectiveTargetWords, currentContent, AppendAgentSDKLengthRetrySuggestions(suggestions, effectiveTargetWords), agentApply, useAgentHistory, iteration...)
		}
		if err != nil {
			if recovered, ok := RecoverAgentAppliedChapterContentAfterError(chapter, currentContent, effectiveTargetWords, agentApply); ok {
				logger.GetLogger().Warn("Agent SDK returned an error after applying a chapter patch; recovering saved content for %s and continuing Go validation: %v", chapter.ID, err)
				return recovered, nil
			}
		}
		return content, err
	}
	return agent.GenerateChapterWithSuggestions(ctx, chapter, context, continuity, targetWords, currentContent, suggestions, iteration...)
}

func EffectiveAgentSDKImproveTargetWords(chapter *models.Chapter, targetWords int, currentContent string, suggestions string, useAgentSDK bool) int {
	if !useAgentSDK || targetWords <= 0 || suggestionsRequestLengthExpansion(suggestions) {
		return targetWords
	}
	currentUnits := ChapterNarrativeUnitsForLog(chapter, currentContent)
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

func RecoverAgentAppliedChapterContentAfterError(chapter *models.Chapter, previousContent string, targetWords int, agentApply bool) (string, bool) {
	if !agentApply || chapter == nil {
		return "", false
	}
	saved := project.LoadFinalChapterContent(chapter)
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
	count := toolplane.ToolNarrativeUnitCount(content)
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

func ResolveAgentAppliedChapterContent(log logger.LoggerInterface, workerID int, chapter *models.Chapter, previousContent, agentContent string, useAgentSDK, agentApply bool) (string, bool) {
	if !useAgentSDK || !agentApply || chapter == nil {
		return agentContent, false
	}
	saved := project.LoadFinalChapterContent(chapter)
	if strings.TrimSpace(saved) == "" || strings.TrimSpace(saved) == strings.TrimSpace(previousContent) {
		return agentContent, false
	}
	if strings.TrimSpace(saved) != strings.TrimSpace(agentContent) {
		log.Info("[Worker %d] Agent apply changed %s; using saved patch content instead of returned JSON content", workerID, chapter.ID)
	}
	return saved, true
}

func IsAgentSDKLengthOvershootError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "agent-sdk returned too much content")
}

func AppendAgentSDKLengthRetrySuggestions(suggestions string, targetWords int) string {
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

// RunAgentSDKChapterPostSaveCheck refreshes the chapter RPG DSL and runs the
// deterministic post-save checks that gate further repair.

// ResolveVolumeID resolves volume flag to full volume ID.

// If already full ID, return as is

// Try to resolve with part flag

// Default to first part
