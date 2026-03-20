package cmd

import (
	"strings"

	"nolvegen/internal/logger"
	"nolvegen/internal/logic/continuity/transition"
	"nolvegen/internal/models"
)

type bridgeGenFunc func(suggestions string) (string, error)

// runTeleportBridgeAutoFix performs a minimal-change teleport fix by asking the model
// to generate ONLY a <TRANSITION_BRIDGE> patch, inserting it into the original text,
// and re-checking the teleport heuristic. It retries up to retries times.
func runTeleportBridgeAutoFix(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	prevRecap *models.ChapterRecap,
	original string,
	baseSuggestions string,
	retries int,
	gen bridgeGenFunc,
) (fixedText string, attempted bool, attemptsUsed int, passed bool) {
	if chapter == nil {
		return original, false, 0, false
	}
	if !strings.Contains(baseSuggestions, "【PATCH 请求：TRANSITION_BRIDGE】") {
		return original, false, 0, false
	}
	if gen == nil {
		return original, false, 0, false
	}

	cleanOrig := strings.TrimSpace(original)
	if cleanOrig == "" {
		cleanOrig = original
	}

	suggestions := baseSuggestions
	attempts := retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastInserted string = original

	for attempt := 0; attempt < attempts; attempt++ {
		attemptsUsed = attempt + 1
		out, err := gen(suggestions)
		if err != nil {
			if log != nil {
				log.Warn("[Worker %d] Bridge generation failed for %s (attempt %d/%d): %v", workerID, chapter.ID, attempt+1, attempts, err)
			}
			continue
		}

		if ok, reasons := transition.ValidateTransitionBridgeOutput(out); !ok {
			if log != nil {
				log.Warn("[Worker %d] Transition bridge validation failed for %s: %v", workerID, chapter.ID, reasons)
			}
			suggestions = suggestions + "\n\n## 自动验收失败（必须修复）\n" + formatReasons(reasons)
			continue
		}

		bridge, ok := transition.ExtractTransitionBridge(out)
		if !ok {
			if log != nil {
				log.Warn("[Worker %d] Transition bridge extraction failed for %s", workerID, chapter.ID)
			}
			continue
		}

		inserted := transition.InsertTransitionBridge(cleanOrig, bridge)
		lastInserted = inserted

		// If we have a previous recap, re-check teleport heuristic to ensure the fix actually worked.
		if prevRecap != nil {
			if res := transition.CheckTeleportOpening(prevRecap, chapter, inserted); res != nil && res.HasIssue {
				if log != nil {
					log.Warn("[Worker %d] Teleport still detected after bridge insert for %s: %v", workerID, chapter.ID, res.Issues)
				}
				suggestions = baseSuggestions + "\n\n" + transitionFixFeedback(res.Issues)
				continue
			}
		}

		return inserted, true, attemptsUsed, true
	}

	return lastInserted, true, attemptsUsed, false
}
