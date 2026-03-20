package cmd

import (
	"strings"

	"nolvegen/internal/logger"
	"nolvegen/internal/logic/continuity/character"
	"nolvegen/internal/models"
)

type patchGenFunc func(suggestions string) (string, error)

// runCharacterPresenceAutoFix attempts a minimal-change fix for character presence issues.
// It asks the model to output ONLY a <CHARACTER_PRESENCE_PATCH> block, inserts it into the
// original text, then re-runs the heuristic check.
func runCharacterPresenceAutoFix(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	original string,
	knownChars []string,
	baseSuggestions string,
	retries int,
	gen patchGenFunc,
) (fixedText string, attempted bool, attemptsUsed int, passed bool) {
	if chapter == nil || gen == nil {
		return original, false, 0, false
	}
	if !strings.Contains(baseSuggestions, "【PATCH 请求：CHARACTER_PRESENCE_PATCH】") {
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

	lastInserted := original

	for attempt := 0; attempt < attempts; attempt++ {
		attemptsUsed = attempt + 1
		out, err := gen(suggestions)
		if err != nil {
			if log != nil {
				log.Warn("[Worker %d] Character patch generation failed for %s (attempt %d/%d): %v", workerID, chapter.ID, attempt+1, attempts, err)
			}
			continue
		}
		if ok, reasons := character.ValidateCharacterPresencePatchOutput(out); !ok {
			suggestions = suggestions + "\n\n## 自动验收失败（必须修复）\n" + formatReasons(reasons)
			continue
		}
		patch, ok := character.ExtractCharacterPresencePatch(out)
		if !ok {
			continue
		}

		inserted := character.InsertCharacterPresencePatch(cleanOrig, patch)
		lastInserted = inserted

		// Re-check heuristic
		if res := character.CheckCharacterPresence(chapter, inserted, knownChars); res != nil && res.HasIssue {
			// still failing, provide feedback and retry
			suggestions = baseSuggestions + "\n\n## 角色出场复检失败（必须修复）\n" + strings.Join(res.Issues, "\n")
			continue
		}

		return inserted, true, attemptsUsed, true
	}

	return lastInserted, true, attemptsUsed, false
}
