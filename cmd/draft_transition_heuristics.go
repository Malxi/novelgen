package cmd

import (
	"sort"
	"strings"

	"nolvegen/internal/agents"
	"nolvegen/internal/logic/continuity/character"
	"nolvegen/internal/logic/continuity/recap"
	"nolvegen/internal/logic/continuity/transition"
	"nolvegen/internal/models"
)

// applyHeuristicTransitionChecks runs deterministic, cheap heuristics over draft text
// and merges results into the LLM review output. This is designed to:
// 1) catch obvious teleport openings even if the LLM misses them
// 2) feed concrete fix suggestions into draft improve / write improve flows
//
// Best-effort: it never returns an error; it only augments review in-place.
func applyHeuristicTransitionChecks(volume *models.Volume, drafts map[string]string, review *agents.VolumeReview) {
	if volume == nil || review == nil {
		return
	}

	root, _ := findProjectRoot()

	// Map chapter ID -> recap (for prev chapter)
	idToRecap := map[string]*models.ChapterRecap{}
	for i := range volume.Chapters {
		ch := &volume.Chapters[i]
		text := strings.TrimSpace(drafts[ch.ID])
		if text == "" {
			continue
		}
		idToRecap[ch.ID] = buildOfflineRecap(ch, text)
	}

	// Map review by chapter ID for quick merge
	reviewByID := map[string]*agents.DraftReview{}
	for i := range review.Reviews {
		r := &review.Reviews[i]
		reviewByID[r.ChapterID] = r
	}

	// Build a canonical list of known character names for this volume
	knownChars := collectKnownCharacters(volume)

	// Walk chapters in order within volume and check current opening vs prev recap
	for i := 1; i < len(volume.Chapters); i++ {
		prev := &volume.Chapters[i-1]
		cur := &volume.Chapters[i]

		curText := strings.TrimSpace(drafts[cur.ID])
		if curText == "" {
			continue
		}

		prevRecap := idToRecap[prev.ID]
		if prevRecap == nil {
			continue
		}

		r := reviewByID[cur.ID]
		if r == nil {
			continue
		}

		// Recap quality gate (persisted recap existence + minimal fields)
		if r.RecapQuality.Score == 0 && strings.TrimSpace(root) != "" {
			if ok, issues, sugs := recap.CheckQuality(root, cur.ID); !ok {
				r.RecapQuality.Score = 4
				for _, it := range issues {
					if it != "" && !containsStr(r.RecapQuality.Issues, it) {
						r.RecapQuality.Issues = append(r.RecapQuality.Issues, it)
					}
				}
				for _, s := range sugs {
					if s != "" && !containsStr(r.RecapQuality.Suggestions, s) {
						r.RecapQuality.Suggestions = append(r.RecapQuality.Suggestions, s)
					}
					if s != "" && !containsStr(r.Suggestions, s) {
						r.Suggestions = append(r.Suggestions, s)
					}
				}
			} else {
				r.RecapQuality.Score = 8
			}
		}

		// Character presence heuristic check (conservative)
		if pres, details := character.CheckCharacterPresenceDetailed(cur, curText, knownChars); pres != nil && pres.HasIssue {
			for _, issue := range pres.Issues {
				if issue != "" && !containsStr(r.CharacterPresence.Issues, issue) {
					r.CharacterPresence.Issues = append(r.CharacterPresence.Issues, issue)
				}
			}
			for _, sug := range pres.Suggestions {
				if sug != "" && !containsStr(r.CharacterPresence.Suggestions, sug) {
					r.CharacterPresence.Suggestions = append(r.CharacterPresence.Suggestions, sug)
				}
				if sug != "" && !containsStr(r.Suggestions, sug) {
					r.Suggestions = append(r.Suggestions, sug)
				}
			}
			// If we have structured details, add a patch request to top-level suggestions
			// so improve can auto-fix with minimal changes.
			if details != nil {
				patchReq := buildCharacterPresencePatchRequest(details.MissingExpected, details.UnexpectedInOpen)
				if patchReq != "" && !containsStr(r.Suggestions, patchReq) {
					r.Suggestions = append(r.Suggestions, patchReq)
				}
			}
			if r.CharacterPresence.Score == 0 {
				r.CharacterPresence.Score = 7
			}
			if r.OverallScore > 7 {
				r.OverallScore = 7
			}
		}

		// Teleport opening heuristic check
		res := transition.CheckTeleportOpening(prevRecap, cur, curText)
		if res == nil || !res.HasIssue {
			continue
		}

		// Merge issues/suggestions (dedupe lightly)
		for _, issue := range res.Issues {
			if issue != "" && !containsStr(r.SceneContinuity.Issues, issue) {
				r.SceneContinuity.Issues = append(r.SceneContinuity.Issues, issue)
			}
		}
		for _, sug := range res.Suggestions {
			if sug != "" && !containsStr(r.SceneContinuity.Suggestions, sug) {
				r.SceneContinuity.Suggestions = append(r.SceneContinuity.Suggestions, sug)
			}
			// Also bubble into top-level suggestions so improve definitely sees it
			if sug != "" && !containsStr(r.Suggestions, sug) {
				r.Suggestions = append(r.Suggestions, sug)
			}
		}

		// Add a strict, structured instruction block to help improve reliably patch
		// the opening with a transition bridge.
		strict := buildTeleportFixInstruction(prevRecap)
		if strict != "" {
			if !containsStr(r.SceneContinuity.Suggestions, strict) {
				r.SceneContinuity.Suggestions = append(r.SceneContinuity.Suggestions, strict)
			}
			if !containsStr(r.Suggestions, strict) {
				r.Suggestions = append(r.Suggestions, strict)
			}
		}

		// Also add a patch-style instruction block so the model can generate a concrete
		// bridge segment and then insert it verbatim at the start.
		patch := buildTransitionBridgePatchRequest(prevRecap)
		if patch != "" {
			if !containsStr(r.SceneContinuity.Suggestions, patch) {
				r.SceneContinuity.Suggestions = append(r.SceneContinuity.Suggestions, patch)
			}
			if !containsStr(r.Suggestions, patch) {
				r.Suggestions = append(r.Suggestions, patch)
			}
		}

		// If LLM forgot to score, give a conservative nudge down.
		if r.SceneContinuity.Score == 0 {
			r.SceneContinuity.Score = 6
		} else if r.SceneContinuity.Score > 7 {
			r.SceneContinuity.Score = 7
		}

		// If overall score is high but we found a hard continuity break, gently nudge.
		if r.OverallScore > 7 {
			r.OverallScore = 7
		}
	}
}

func containsStr(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func collectKnownCharacters(volume *models.Volume) []string {
	if volume == nil {
		return nil
	}
	set := map[string]bool{}
	for i := range volume.Chapters {
		ch := &volume.Chapters[i]
		for _, n := range ch.Characters {
			n = strings.TrimSpace(n)
			if n != "" {
				set[n] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func buildTeleportFixInstruction(prevRecap *models.ChapterRecap) string {
	if prevRecap == nil {
		return ""
	}
	// Keep it short but unambiguous. This will be fed into improve prompts.
	// We intentionally reference recap.last_line to force continuity.
	return "" +
		"【硬性修复指令：补转场桥段】\n" +
		"- 目标：修复本章开头‘瞬移转场’问题。\n" +
		"- 必须在正文最开头插入 120–260 字‘转场桥段’，再进入原有剧情。\n" +
		"- 桥段必须以‘上一章最后一幕/最后一句’为承接点开始（优先直接复用/轻改写 recap.last_line）。\n" +
		"- 桥段必须交代：为何离开上一地点、如何到达新地点、经过了多久；若是切镜头叙事，必须明确写出‘与此同时/另一边/镜头一转’来合法化。\n" +
		"- 禁止：直接在第一段就写‘地点: 新地点’而不解释。\n" +
		"- 不要新增大事件，只做连续性补丁。"
}

func buildTransitionBridgePatchRequest(prevRecap *models.ChapterRecap) string {
	if prevRecap == nil {
		return ""
	}
	// This is a stronger, more "program-like" instruction that asks the model to
	// produce a concrete bridge segment that can be inserted as a patch.
	return "" +
		"【PATCH 请求：TRANSITION_BRIDGE】\n" +
		"你需要先输出一段可直接插入正文开头的‘转场桥段’（160–220字），并用下面格式包裹：\n" +
		"<TRANSITION_BRIDGE>\n" +
		"...桥段正文...\n" +
		"</TRANSITION_BRIDGE>\n" +
		"要求：\n" +
		"1) 第一行必须承接上一章最后一句/最后一幕（优先复用或轻改写 recap.last_line）。\n" +
		"2) 交代离开原因、到达方式、耗时；或明确‘与此同时/另一边/镜头一转’作为切镜合法化。\n" +
		"3) 禁止在桥段第一段直接写‘地点: 新地点’而不解释。\n" +
		"4) 桥段不得引入新剧情大事件，只做连续性补丁。\n" +
		"5) 仅输出 TRANSITION_BRIDGE 补丁块，不要重写完整章节正文。"
}
