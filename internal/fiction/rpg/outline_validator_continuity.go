package rpg

import (
	"fmt"
	"strings"
)

func (ov *OutlineValidator) validateCultivationContinuity() {
	if ov.Outline == nil {
		return
	}

	protagonist := ov.outlineProtagonistName()
	knownNames := outlineKnownCharacterNames(ov.Outline)
	chapters := outlineOrderedChapters(ov.Outline)
	states := make(map[string]cultivationSighting)

	for idx, chapter := range chapters {
		// 1. StateAnchor 仅追踪主角，且作为本章主角修为的权威来源。
		anchor := strings.TrimSpace(chapter.StateAnchor.Cultivation)
		if anchor != "" {
			ov.recordCultivationSighting(states, protagonist, anchor, idx, chapters)
		}
		// 2. StateChange 自然语言追踪任意角色；
		//    主角在同一章已有 state_anchor 时，忽略 StateChange 文本提取的修为
		//    （state_anchor 优先），避免"练气五层"（摘要句）与"练气四层中期"（锚点）
		//    这类跨来源矛盾造成的误报。
		for name, cultivation := range outlineExtractCultivationsFromStateChange(chapter.StateChange, knownNames) {
			if name == protagonist && anchor != "" {
				continue
			}
			ov.recordCultivationSighting(states, name, cultivation, idx, chapters)
		}
	}
}

// recordCultivationSighting 记录一次修为出现并做升降级检查
func (ov *OutlineValidator) recordCultivationSighting(states map[string]cultivationSighting, name, raw string, idx int, chapters []StoryChapter) {
	name = normalizeOutlineName(name)
	raw = strings.TrimSpace(raw)
	if name == "" || raw == "" {
		return
	}

	chapter := chapters[idx]
	sighting := cultivationSighting{raw: raw, chapter: chapter.ID, index: idx}
	sighting.tier, sighting.phase, sighting.parsed = cultivateTierRank(raw)
	sighting.layer = cultivateLayerRank(raw)

	prev, ok := states[name]
	states[name] = sighting
	if !ok || !prev.parsed || !sighting.parsed {
		return
	}

	cmp := cultivateCompareRanks(prev, sighting)
	switch {
	case cmp > 0:
		// 升级：缺少突破事件则警告（minor）
		if !outlineChapterHasBreakthroughExplanation(chapter, name) {
			ov.Warnings = append(ov.Warnings, OutlineWarning{
				Type:        "cultivation_continuity",
				Location:    chapter.ID,
				Description: fmt.Sprintf("角色 %s 修为从 %s 提升到 %s (%s → %s)，但缺少突破事件", name, prev.raw, raw, prev.chapter, chapter.ID),
				Suggestion:  "添加突破/进阶相关事件，或修正修为描述",
			})
		}
	case cmp < 0:
		// 降级：缺少跌境事件则报 major
		if !ov.outlineChapterHasDowngradeExplanation(chapters, prev.index, idx, name) {
			ov.Issues = append(ov.Issues, OutlineIssue{
				Type:        "cultivation_continuity",
				Severity:    "major",
				Location:    chapter.ID,
				Description: fmt.Sprintf("角色 %s 修为从 %s 跌到 %s (chapter %s → %s)，缺少跌境事件", name, prev.raw, raw, prev.chapter, chapter.ID),
				Impact:      "修为倒退缺乏合理解释会破坏修炼体系的一致性",
				Fix:         "添加跌境/跌落/修为倒退/被废等事件，或修正修为描述",
			})
		}
	}
}

// cultivateTierRank 解析修为字符串，返回大境界排名与小境界排名
func cultivateTierRank(s string) (tier int, phase int, ok bool) {
	s = normalizeOutlineValidatorText(strings.TrimSpace(s))
	if s == "" {
		return 0, 0, false
	}
	tier = -1
	for i, t := range outlineCultivationTiers {
		if strings.Contains(s, t) {
			tier = i
			break
		}
	}
	if tier < 0 {
		return 0, 0, false
	}
	phase = 0
	for i, p := range outlineCultivationPhases {
		if strings.Contains(s, p) {
			phase = len(outlineCultivationPhases) - 1 - i
			break
		}
	}
	return tier, phase, true
}

// cultivateLayerRank 解析修为中的层数（如“练气五层后期”返回5；无层数返回0）
func cultivateLayerRank(s string) int {
	runes := []rune(normalizeOutlineValidatorText(strings.TrimSpace(s)))
	for i := 1; i < len(runes); i++ {
		if runes[i] != '层' {
			continue
		}
		if runes[i-1] == '十' {
			if i >= 2 {
				if v, ok := outlineChineseNumberValue(runes[i-2]); ok && v < 10 {
					return v * 10
				}
			}
			return 10
		}
		if v, ok := outlineChineseNumberValue(runes[i-1]); ok {
			return v
		}
	}
	return 0
}

// outlineChineseNumberValue 中文数字转数值
func outlineChineseNumberValue(r rune) (int, bool) {
	switch r {
	case '一':
		return 1, true
	case '二', '两':
		return 2, true
	case '三':
		return 3, true
	case '四':
		return 4, true
	case '五':
		return 5, true
	case '六':
		return 6, true
	case '七':
		return 7, true
	case '八':
		return 8, true
	case '九':
		return 9, true
	case '十':
		return 10, true
	}
	return 0, false
}

// cultivateCompareRanks 比较两次修为：大境界 → 层数 → 小境界
func cultivateCompareRanks(prev, cur cultivationSighting) int {
	if cur.tier != prev.tier {
		return cur.tier - prev.tier
	}
	if prev.layer > 0 && cur.layer > 0 && cur.layer != prev.layer {
		return cur.layer - prev.layer
	}
	return cur.phase - prev.phase
}

// outlineExtractCultivationsFromStateChange 从 StateChange 自然语言中提取各角色修为
// 按标点切分句子，取每个分句中最后一个修为短语作为该分句提及角色的当前修为。
func outlineExtractCultivationsFromStateChange(text string, knownNames []string) map[string]string {
	result := make(map[string]string)
	text = strings.TrimSpace(text)
	if text == "" {
		return result
	}
	lower := normalizeOutlineValidatorText(text)
	if !containsAnyOutlineValidatorText(lower, "修为", "境界", "cultivation", "突破", "跌落", "跌境", "跌至", "掉到", "降至") {
		return result
	}

	for _, clause := range strings.FieldsFunc(text, outlineClauseSplit) {
		phrases := extractCultivationPhrases(clause)
		if len(phrases) == 0 {
			continue
		}
		clauseLower := normalizeOutlineValidatorText(clause)
		last := phrases[len(phrases)-1]
		for _, name := range knownNames {
			if name != "" && strings.Contains(clauseLower, normalizeOutlineValidatorText(name)) {
				result[name] = last
			}
		}
	}
	return result
}

// extractCultivationPhrases 从文本中提取修为短语（如：练气五层后期、筑基大圆满、金丹初期）
func extractCultivationPhrases(text string) []string {
	var phrases []string
	runes := []rune(text)
	n := len(runes)
	i := 0
	for i < n {
		found := -1
		foundLen := 0
		for _, tier := range outlineCultivationTiers {
			tr := []rune(tier)
			if i+len(tr) <= n && string(runes[i:i+len(tr)]) == tier {
				found = i
				foundLen = len(tr)
				break
			}
		}
		if found < 0 {
			i++
			continue
		}
		end := found + foundLen
		for end < n {
			progressed := false
			for _, phase := range outlineCultivationPhases {
				pr := []rune(phase)
				if end+len(pr) <= n && string(runes[end:end+len(pr)]) == phase {
					end += len(pr)
					progressed = true
					break
				}
			}
			if progressed {
				continue
			}
			if outlineIsCultivationSpecifierRune(runes[end]) {
				end++
				continue
			}
			break
		}
		phrases = append(phrases, string(runes[found:end]))
		i = end
	}
	return phrases
}

func outlineIsCultivationSpecifierRune(r rune) bool {
	switch r {
	case '一', '二', '三', '四', '五', '六', '七', '八', '九', '十', '百', '千', '万', '零', '两',
		'层', '期', '阶', '重', '品', '星', '段', '级':
		return true
	}
	return false
}

// outlineChapterHasBreakthroughExplanation 判断本章是否有突破事件解释修为上升
func outlineChapterHasBreakthroughExplanation(chapter StoryChapter, name string) bool {
	if outlineStateChangeExplains(chapter.StateChange, name, outlineBreakthroughKeywords) {
		return true
	}
	for _, evt := range chapter.Events {
		if storyEventSupportsCultivationChange(evt, "", "") {
			return true
		}
	}
	return false
}

// outlineChapterHasDowngradeExplanation 判断从 from 到 to 章节之间是否存在跌境解释
func (ov *OutlineValidator) outlineChapterHasDowngradeExplanation(chapters []StoryChapter, from, to int, name string) bool {
	if from < 0 {
		from = 0
	}
	if to >= len(chapters) {
		to = len(chapters) - 1
	}
	for i := from; i <= to; i++ {
		chapter := chapters[i]
		if outlineStateChangeExplains(chapter.StateChange, name, outlineDowngradeKeywords) {
			return true
		}
		for _, evt := range chapter.Events {
			evtText := outlineEventText(evt)
			if outlineEventMentionsName(evt, name) &&
				containsAnyOutlineValidatorText(evtText, outlineDowngradeKeywords...) &&
				outlineTextMentionsCultivation(evtText) {
				return true
			}
		}
	}
	return false
}

// outlineStateChangeExplains 判断 StateChange 是否包含解释关键词并提及修为相关文本
func outlineStateChangeExplains(stateChange, name string, keywords []string) bool {
	scText := normalizeOutlineValidatorText(stateChange)
	if !containsAnyOutlineValidatorText(scText, keywords...) {
		return false
	}
	if name != "" && strings.Contains(scText, normalizeOutlineValidatorText(name)) {
		return true
	}
	return outlineTextMentionsCultivation(scText)
}

// outlineTextMentionsCultivation 判断文本是否与修为相关（出现修为/境界关键词或任一境界名）
func outlineTextMentionsCultivation(text string) bool {
	if containsAnyOutlineValidatorText(text, "修为", "境界", "cultivation") {
		return true
	}
	for _, tier := range outlineCultivationTiers {
		if strings.Contains(text, tier) {
			return true
		}
	}
	return false
}

// outlineEventText 拼接事件全部文本用于关键词检查
func outlineEventText(evt StoryEvent) string {
	return normalizeOutlineValidatorText(strings.Join([]string{
		evt.Type,
		evt.Subject,
		evt.Change,
		evt.Details,
		evt.Actor,
		evt.Action,
		evt.Target,
		evt.TargetType,
		evt.Context,
		evt.Result,
	}, " "))
}

// outlineEventMentionsName 判断事件是否提到指定角色名
func outlineEventMentionsName(evt StoryEvent, name string) bool {
	name = normalizeOutlineValidatorText(name)
	if name == "" {
		return true
	}
	if strings.Contains(outlineEventText(evt), name) {
		return true
	}
	for _, c := range evt.Characters {
		if normalizeOutlineValidatorText(c) == name {
			return true
		}
	}
	return false
}

// outlineOrderedChapters 按顺序展开所有章节
func outlineOrderedChapters(outline *StoryOutline) []StoryChapter {
	var chapters []StoryChapter
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			chapters = append(chapters, volume.Chapters...)
		}
	}
	return chapters
}

// outlineProtagonistName 推断主角名（第一章第一个角色名，找不到时用“主角”）
func (ov *OutlineValidator) outlineProtagonistName() string {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, name := range chapter.Characters {
					if n := normalizeOutlineName(name); n != "" {
						return n
					}
				}
			}
		}
	}
	return "主角"
}

// outlineKnownCharacterNames 收集大纲中所有命名角色
func outlineKnownCharacterNames(outline *StoryOutline) []string {
	set := make(map[string]bool)
	var names []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !set[name] {
			set[name] = true
			names = append(names, name)
		}
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, name := range chapter.Characters {
					add(name)
				}
				for _, evt := range chapter.Events {
					for _, name := range evt.Characters {
						add(name)
					}
					add(evt.GetActor())
					targetType := normalizeOutlineName(evt.GetTargetType())
					if targetType == "" || targetType == "character" {
						add(evt.GetTarget())
					}
				}
				for _, enemy := range chapter.Enemies {
					add(enemy.Name)
				}
			}
		}
	}
	return names
}

// validateItemUsageContinuity 物品获得→使用连续性检查
// 记录每个物品首次获得章节；物品在获得前被使用即报 major 问题。
