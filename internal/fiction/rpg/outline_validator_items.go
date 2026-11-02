package rpg

import (
	"fmt"
	"strings"

	models "novelgen/internal/contract/story"
)

func (ov *OutlineValidator) validateItemUsageContinuity() {
	if ov.Outline == nil {
		return
	}

	candidates := outlineKnownItemNames(ov.Outline)
	acquired := make(map[string]string) // 规范化物品名 → 首次获得章节
	flagged := make(map[string]bool)    // 物品名|章节 → 已报过

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, evt := range chapter.Events {
					// 先记录获得（结构化 + details 文本）
					for _, name := range outlineEventAcquiredItems(evt, candidates) {
						if _, ok := acquired[name]; !ok {
							acquired[name] = chapter.ID
						}
					}
					// 再检查使用
					for _, name := range outlineEventUsedItems(evt, candidates) {
						if outlineIsNonItemConcept(name) {
							continue
						}
						if _, ok := acquired[name]; ok {
							continue
						}
						key := name + "\x00" + chapter.ID
						if flagged[key] {
							continue
						}
						flagged[key] = true
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type:        "item_usage_continuity",
							Severity:    "major",
							Location:    chapter.ID,
							Description: fmt.Sprintf("物品 %s 在 %s 被使用，但此前从未获得", name, chapter.ID),
							Impact:      "物品在获得前被使用会破坏资源与道具的连贯性",
							Fix:         "在前面章节添加获得该物品的事件，或移除本次使用",
						})
					}
				}
			}
		}
	}
}

// outlineNonItemConcepts 非物品概念/初始资源/能力名：这些词即使出现在 item 事件中
// 也跳过（不追踪、不报"使用前未获得"），例如灵石是初始资源、血色校验/神魂之力是能力。
var outlineNonItemConcepts = []string{
	"灵石", "血色校验", "神魂之力", "玉简使用次数", "回溯读取", "跨系统解析",
	"日志", "时间线重置", "时间线", "模拟器", "规则", "权限", "情报",
}

// outlineIsNonItemConcept 判断规范化后的物品名是否命中非物品概念/能力/初始资源。
func outlineIsNonItemConcept(name string) bool {
	name = normalizeOutlineName(name)
	if name == "" {
		return false
	}
	for _, c := range outlineNonItemConcepts {
		if strings.Contains(name, c) {
			return true
		}
	}
	return false
}

// normalizeOutlineItemName 规范化物品名作为追踪 KEY：
// 去引号/空白、去除括号注释（破阵符（签到应急奖励）→破阵符、隐息符（最后一张）→隐息符），
// 并去掉尾部数量后缀（隐息符×2→隐息符）。
func normalizeOutlineItemName(s string) string {
	s = normalizeOutlineName(s)
	for {
		start := -1
		closer := ""
		if i := strings.Index(s, "（"); i >= 0 {
			start, closer = i, "）"
		}
		if i := strings.Index(s, "("); i >= 0 && (start < 0 || i < start) {
			start, closer = i, ")"
		}
		if start < 0 {
			break
		}
		endRel := strings.Index(s[start+len(closer):], closer)
		if endRel < 0 {
			break
		}
		end := start + len(closer) + endRel + len(closer)
		s = s[:start] + s[end:]
	}
	s = strings.TrimSpace(s)
	for _, marker := range []string{"×", "x", "X", "*"} {
		if idx := strings.LastIndex(s, marker); idx >= 0 && outlineIsAllDigits(s[idx+len(marker):]) {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return strings.Trim(s, "\"'“”‘’「」『』")
}

func outlineIsAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// outlineKnownItemNames 收集大纲中的候选物品名
func outlineKnownItemNames(outline *StoryOutline) []string {
	set := make(map[string]bool)
	var names []string
	add := func(name string) {
		name = normalizeOutlineItemName(name)
		if name != "" && !outlineIsNonItemConcept(name) && !set[name] {
			set[name] = true
			names = append(names, name)
		}
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, name := range chapter.StateAnchor.KeyItems {
					add(name)
				}
				for _, entry := range chapter.ResourceLedger {
					add(entry.Item)
				}
				for _, evt := range chapter.Events {
					action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
					evtType := strings.ToLower(normalizeOutlineName(evt.Type))
					if evtType == "item" || action == models.ActionAcquire || action == models.ActionUse ||
						action == models.ActionConsume || action == models.ActionCraft || action == "consumed" ||
						action == "utilize" || action == "使用" || action == "消耗" {
						if outlineEventTargetIsItem(evt) {
							add(evt.GetTarget())
						}
					}
				}
			}
		}
	}
	return names
}

var outlineAcquireKeywords = []string{"获得", "得到", "捡到", "拿到", "入手"}
var outlineUseKeywords = []string{"使用", "消耗", "用掉"}

// outlineEventAcquiredItems 判定事件中获得的物品
func outlineEventAcquiredItems(evt StoryEvent, candidates []string) []string {
	var items []string
	add := func(name string) {
		name = normalizeOutlineItemName(name)
		if name != "" && !outlineIsNonItemConcept(name) && !contains(items, name) {
			items = append(items, name)
		}
	}

	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	target := normalizeOutlineName(evt.GetTarget())
	change := normalizeOutlineName(evt.Change)
	evtType := strings.ToLower(normalizeOutlineName(evt.Type))

	// 结构化获得：acquire / craft（含旧格式 item 事件推断出的动作）且目标非空。
	if (action == models.ActionAcquire || action == models.ActionCraft) &&
		target != "" && outlineEventTargetIsItem(evt) {
		add(target)
	}
	if evtType == "item" && (change == "acquired" || change == "obtained" || change == "got" ||
		change == "获得" || change == "得到") && target != "" {
		add(target)
	}

	// 仅当结构化 action 为 acquire 且 target 为空时，才回退到 details 文本解析。
	if action == models.ActionAcquire && target == "" {
		for _, name := range candidates {
			if outlineDetailsContainsKeywordNearItem(evt.Details, name, outlineAcquireKeywords) {
				add(name)
			}
		}
	}
	return items
}

// outlineEventUsedItems 判定事件中使用的物品
func outlineEventUsedItems(evt StoryEvent, candidates []string) []string {
	var items []string
	add := func(name string) {
		name = normalizeOutlineItemName(name)
		if name != "" && !outlineIsNonItemConcept(name) && !contains(items, name) {
			items = append(items, name)
		}
	}

	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	target := normalizeOutlineName(evt.GetTarget())
	// 结构化使用：use/consume/consumed/utilize（含中文别名 使用/消耗）且目标非空。
	if (action == models.ActionUse || action == "使用" || action == "消耗" || action == "utilize" ||
		strings.Contains(action, "consume")) &&
		target != "" && outlineEventTargetIsItem(evt) {
		add(target)
	}

	// 仅当结构化 action 为 use 且 target 为空时，才回退到 details 文本解析。
	if action == models.ActionUse && target == "" {
		for _, name := range candidates {
			if outlineDetailsContainsKeywordNearItem(evt.Details, name, outlineUseKeywords) {
				add(name)
			}
		}
	}
	return items
}

// outlineEventTargetIsItem 判断事件 target 是否可视为物品名
// 仅接受 target_type 为 item/空 或旧格式 type 为 item 的事件，避免把技能/知识/地点当作物品。
func outlineEventTargetIsItem(evt StoryEvent) bool {
	targetType := strings.ToLower(normalizeOutlineName(evt.GetTargetType()))
	return targetType == "" || targetType == "item"
}

// outlineDetailsContainsKeywordNearItem 判断 details 中某物品名是否与关键词出现在同一分句
func outlineDetailsContainsKeywordNearItem(details, name string, keywords []string) bool {
	details = normalizeOutlineValidatorText(details)
	name = normalizeOutlineValidatorText(name)
	if name == "" {
		return false
	}
	for _, clause := range strings.FieldsFunc(details, outlineClauseSplit) {
		if !strings.Contains(clause, name) {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(clause, kw) {
				return true
			}
		}
	}
	return false
}

// outlineClauseSplit 用于按标点/空白切分句子
func outlineClauseSplit(r rune) bool {
	switch r {
	case '，', '。', '；', '、', ',', '.', ';', '！', '？', '!', '?', '\n', '\r', '\t', ' ':
		return true
	}
	return false
}

// normalizeOutlineName 规范化角色/物品名：去首尾空白并剥离引号
func normalizeOutlineName(s string) string {
	return strings.Trim(strings.TrimSpace(s), "\"'“”‘’「」『』")
}

// generateSummary 生成验证摘要
func (ov *OutlineValidator) generateSummary() string {
	if len(ov.Issues) == 0 && len(ov.Warnings) == 0 {
		return "✓ 大纲验证通过，未发现明显问题"
	}

	summary := fmt.Sprintf("大纲验证完成：发现 %d 个问题，%d 个警告，%d 条建议\n",
		len(ov.Issues), len(ov.Warnings), len(ov.Suggestions))

	if len(ov.Issues) > 0 {
		criticalCount := 0
		majorCount := 0
		for _, issue := range ov.Issues {
			switch issue.Severity {
			case "critical":
				criticalCount++
			case "major":
				majorCount++
			}
		}

		if criticalCount > 0 {
			summary += fmt.Sprintf("  ⚠ 严重问题: %d 个（必须修复）\n", criticalCount)
		}
		if majorCount > 0 {
			summary += fmt.Sprintf("  ⚠ 主要问题: %d 个（建议修复）\n", majorCount)
		}
	}

	return summary
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// outlineCombatResultChanges 战斗事件的可接受结果语义（change 应为这些受控词，而非散文）
