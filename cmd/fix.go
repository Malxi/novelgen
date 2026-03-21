package cmd

import (
	"fmt"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/character"
	"novelgen/internal/logic/continuity/transition"
	"novelgen/internal/models"
)

// ==================== Fix Summary ====================

// FixSummary tracks the results of auto-fix attempts.
type FixSummary struct {
	TeleportAttempted  bool
	TeleportAttempts   int
	TeleportPassed     bool
	CharacterAttempted bool
	CharacterAttempts  int
	CharacterPassed    bool
}

func (s FixSummary) String() string {
	return fmt.Sprintf("teleport_fix=%v attempts=%d passed=%v | character_fix=%v attempts=%d passed=%v",
		s.TeleportAttempted, s.TeleportAttempts, s.TeleportPassed,
		s.CharacterAttempted, s.CharacterAttempts, s.CharacterPassed)
}

// ==================== Fix Pipeline ====================

type patchGenFunc func(suggestions string) (string, error)
type bridgeGenFunc func(suggestions string) (string, error)

// applyImproveFixesDraft runs enabled minimal-change fixers for draft improve.
// Order matters: teleport bridge first, then character presence.
func applyImproveFixesDraft(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	outline *models.Outline,
	original string,
	suggestions string,
	teleportEnabled bool,
	teleportRetries int,
	teleportGen bridgeGenFunc,
	charEnabled bool,
	charRetries int,
	charKnown []string,
	charGen patchGenFunc,
) (string, FixSummary) {
	out := original
	sum := FixSummary{}

	if teleportEnabled {
		prevRecap := loadPreviousRecapStruct(outline, chapter)
		fixed, attempted, attemptsUsed, passed := runTeleportBridgeAutoFix(log, workerID, chapter, prevRecap, out, suggestions, teleportRetries, teleportGen)
		sum.TeleportAttempted = attempted
		sum.TeleportAttempts = attemptsUsed
		sum.TeleportPassed = passed
		if attempted {
			out = fixed
		}
	}

	if charEnabled {
		fixed, attempted, attemptsUsed, passed := runCharacterPresenceAutoFix(log, workerID, chapter, out, charKnown, suggestions, charRetries, charGen)
		sum.CharacterAttempted = attempted
		sum.CharacterAttempts = attemptsUsed
		sum.CharacterPassed = passed
		if attempted {
			out = fixed
		}
	}

	return out, sum
}

// applyImproveFixesWrite runs enabled minimal-change fixers for write improve.
func applyImproveFixesWrite(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	outline *models.Outline,
	original string,
	suggestions string,
	teleportEnabled bool,
	teleportRetries int,
	teleportGen bridgeGenFunc,
	charEnabled bool,
	charRetries int,
	charKnown []string,
	charGen patchGenFunc,
) (string, FixSummary) {
	out := original
	sum := FixSummary{}
	if teleportEnabled {
		prevRecap := loadPreviousRecapStruct(outline, chapter)
		fixed, attempted, attemptsUsed, passed := runTeleportBridgeAutoFix(log, workerID, chapter, prevRecap, out, suggestions, teleportRetries, teleportGen)
		sum.TeleportAttempted = attempted
		sum.TeleportAttempts = attemptsUsed
		sum.TeleportPassed = passed
		if attempted {
			out = fixed
		}
	}
	if charEnabled {
		fixed, attempted, attemptsUsed, passed := runCharacterPresenceAutoFix(log, workerID, chapter, out, charKnown, suggestions, charRetries, charGen)
		sum.CharacterAttempted = attempted
		sum.CharacterAttempts = attemptsUsed
		sum.CharacterPassed = passed
		if attempted {
			out = fixed
		}
	}
	return out, sum
}

// ==================== Character Fix ====================

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

func buildCharacterPresencePatchRequest(missing []string, unexpected []string) string {
	var sb strings.Builder
	sb.WriteString("【PATCH 请求：CHARACTER_PRESENCE_PATCH】\n")
	sb.WriteString("你需要仅输出一段可直接插入正文开头的'角色出场补丁段'（120–220字），并用下面格式包裹：\n")
	sb.WriteString("<CHARACTER_PRESENCE_PATCH>\n")
	sb.WriteString("...补丁正文...\n")
	sb.WriteString("</CHARACTER_PRESENCE_PATCH>\n")
	sb.WriteString("要求：\n")
	sb.WriteString("1) 目标：让大纲 characters 列表中的角色在正文中真实出场（至少一句动作/台词/被提及的明确描写）。\n")
	sb.WriteString("2) 补丁不得引入新剧情大事件，只做补出场/补承接。\n")
	sb.WriteString("3) 语气与本章一致，尽量自然地嵌入，不要写元评论。\n")
	sb.WriteString("4) 仅输出补丁块，不要重写完整章节。\n")
	if len(missing) > 0 {
		sb.WriteString("必须补出场的角色：" + strings.Join(missing, ", ") + "\n")
	}
	if len(unexpected) > 0 {
		sb.WriteString("若开头出现不该出现的角色，请用一句切镜/并线说明或降低其出镜：" + strings.Join(unexpected, ", ") + "\n")
	}
	return sb.String()
}

// ==================== Transition Fix ====================

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

func transitionFixFeedback(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Teleport 复检失败（必须修复）\n")
	for _, it := range issues {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		sb.WriteString("- " + it + "\n")
	}
	sb.WriteString("\n请重新生成 TRANSITION_BRIDGE，确保开头不再触发瞬移判定。")
	return sb.String()
}
