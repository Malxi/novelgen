package writepipe

import (
	"context"
	"fmt"
	"strings"

	novelcont "novelgen/internal/continuity"
	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/toolplane"
)

// RecapExtractor is the recap-agent surface the write pipeline depends on. Both
// the chat and Agent SDK backends satisfy it.
type RecapExtractor interface {
	Extract(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error)
	ExtractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error)
	ExtractWithAgentSDK(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error)
	ExtractWithFeedbackAgentSDK(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error)
}

// ExtractAndSaveRecapWithGate extracts the chapter recap, retries once with
// gate feedback when the minimal validation fails, and persists the result.
func ExtractAndSaveRecapWithGate(ctx context.Context, recapAgent RecapExtractor, recapStore *recap.Store, chapter *models.Chapter, content string, workerID int, useAgentSDK bool) error {
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

		fb := novelcont.RecapGateFeedback(reasons, recapData)
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

	root := recapStore.Root()
	if root == nil {
		return fmt.Errorf("save recap: recap store has no project root")
	}
	if _, err := toolplane.NewMutationService(root).ReplaceRecap(ctx, recapData, toolplane.MutationOptions{
		Validate: false,
		Source:   "write-pipeline",
	}); err != nil {
		return fmt.Errorf("save recap: %w", err)
	}
	return nil
}

func extractRecap(ctx context.Context, recapAgent RecapExtractor, chapterID, title, content string, useAgentSDK bool) (*models.ChapterRecap, error) {
	if useAgentSDK {
		return recapAgent.ExtractWithAgentSDK(ctx, chapterID, title, content)
	}
	return recapAgent.Extract(ctx, chapterID, title, content)
}

func extractRecapWithFeedback(ctx context.Context, recapAgent RecapExtractor, chapterID, title, content, feedback string, useAgentSDK bool) (*models.ChapterRecap, error) {
	if useAgentSDK {
		return recapAgent.ExtractWithFeedbackAgentSDK(ctx, chapterID, title, content, feedback)
	}
	return recapAgent.ExtractWithFeedback(ctx, chapterID, title, content, feedback)
}
