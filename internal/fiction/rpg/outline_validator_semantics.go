package rpg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var outlineCombatResultChanges = []string{
	"progressed", "completed", "achieved", "started", "defeated", "resolved",
	"worsened", "transformed", "activated", "injured", "climax", "solidified",
	"temporary_death", "deepened", "launched", "escalated", "ended", "won",
	"lost", "retreated", "escaped", "survived", "victory", "defeat", "pyrrhic",
	"confronted", "surrendered", "captured", "repelled", "stalemate", "partial",
}

// validateEventSemantics 事件语义完整性检查
// 目标：确保事件的结构化字段（action/change/target）携带机器可读的语义，
// 而不是把语义丢在 details 散文里。这样下游验证器（死亡/修为/物品）与模拟器
// 都能直接消费事件，而非靠关键词猜。检测三类问题：
//  1. combat 事件的 change 是散文（不在受控结果词表）→ 缺战斗结果语义
//  2. acquire 事件缺 target（不知道获得了什么）→ 缺物品名
//  3. use 事件的 target 是"工具/资源/能力"而非消耗性物品 → use 语义误用
func (ov *OutlineValidator) validateEventSemantics() {
	if ov.Outline == nil {
		return
	}
	flagged := make(map[string]bool) // 事件描述|章节 → 已报过

	// 非消耗性 target：use 的目标是工具/资源/能力时，use 应改为 utilize/operate/employ
	nonConsumableTargets := []string{"日志", "模拟器", "系统", "权限", "经脉", "阵眼", "阵法",
		"执法堂", "情报", "规则", "能力", "功法", "深度扫描", "反侦察干扰", "血幕大阵",
		"优先出发权", "灵脉", "通道", "空间", "残页", "灵石"}

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, evt := range chapter.Events {
					action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
					change := normalizeOutlineName(evt.Change)
					target := normalizeOutlineName(evt.GetTarget())
					key := chapter.ID + "\x00" + action + "\x00" + change + "\x00" + target

					switch action {
					case "combat":
						if change == "" {
							continue // 无 change 交给其他检查
						}
						if !containsAnyOutlineValidatorText(change, outlineCombatResultChanges...) &&
							!outlineTextHasNonDeathContext(change) {
							if flagged[key] {
								continue
							}
							flagged[key] = true
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type:        "event_semantics",
								Severity:    "minor",
								Location:    chapter.ID,
								Description: fmt.Sprintf("combat 事件 change 是散文（%q），缺少受控结果语义（如 defeated/injured/escaped/completed）", change),
								Impact:      "战斗结果无法被下游验证器与模拟器消费，死亡/伤势追踪会漏报或误报",
								Fix:         "将 change 改为受控结果词（defeated/injured/escaped/completed/achieved），细节保留在 details 中",
							})
						}
					case "acquire":
						if target == "" {
							if flagged[key] {
								continue
							}
							flagged[key] = true
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type:        "event_semantics",
								Severity:    "minor",
								Location:    chapter.ID,
								Description: "acquire 事件缺少 target（不知道获得了什么物品/资源）",
								Impact:      "物品获得链无法建立，物品使用校验会漏报或误报",
								Fix:         "在 target 字段填写获得的物品/资源名，details 保留具体描述",
							})
						}
					case "use":
						if target == "" {
							continue
						}
						if outlineIsNonItemConcept(target) {
							continue
						}
						isConsumable := false
						for _, suf := range []string{"符", "丹", "药", "散", "液", "髓", "残片", "碎片", "果", "草", "石"} {
							if strings.HasSuffix(target, suf) {
								isConsumable = true
								break
							}
						}
						if isConsumable {
							// 消耗品（符/丹/药/残片）用 use 语义不精确——应使用 consume（一次性消耗）
							if flagged[key] {
								continue
							}
							flagged[key] = true
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type:        "event_semantics",
								Severity:    "minor",
								Location:    chapter.ID,
								Description: fmt.Sprintf("use 事件 target %q 是消耗性物品（符/丹/药），应改用 consume 语义（一次性消耗）", target),
								Impact:      "use 与 consume 混用使物品消耗链无法机器追踪（acquire→consume），库存与损耗校验失真",
								Fix:         "消耗性物品（符/丹/药/残片/灵石）的消耗用 consume；use 仅保留给不消耗的工具/能力/资源操作",
							})
							continue
						}
						// 非消耗 target：use 可接受，但若命中工具/资源/能力词，提示用 utilize/operate/employ 更精确
						for _, nt := range nonConsumableTargets {
							if strings.Contains(target, nt) {
								if flagged[key] {
									break
								}
								flagged[key] = true
								ov.Issues = append(ov.Issues, OutlineIssue{
									Type:        "event_semantics",
									Severity:    "minor",
									Location:    chapter.ID,
									Description: fmt.Sprintf("use 事件 target %q 是工具/资源/能力而非消耗性物品，use 语义被误用", target),
									Impact:      "物品使用链被非消耗目标污染，无法区分'消耗了物品'与'使用了工具'",
									Fix:         "使用工具/资源/能力改用 utilize/operate/employ/leverage；消耗性物品（符/丹/药）用 consume",
								})
								break
							}
						}
					case "consume":
						// consume 只应作用于消耗性物品
						if target == "" {
							continue
						}
						if outlineIsNonItemConcept(target) {
							if flagged[key] {
								continue
							}
							flagged[key] = true
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type:        "event_semantics",
								Severity:    "minor",
								Location:    chapter.ID,
								Description: fmt.Sprintf("consume 事件 target %q 是非物品概念（能力/资源/初始物），不应消耗", target),
								Impact:      "消耗语义被用于非消耗目标，追踪失真",
								Fix:         "consume 只用于一次性消耗品（符/丹/药/残片/灵石）；能力/工具消耗改用 use/utilize",
							})
							continue
						}
						_ = target // 消耗品 consume 正常，无需处理
					}
				}
			}
		}
	}
}

// outlineParseDayNumber 从时间锚点文本中提取"第N天"的数字，解析失败返回 0
func outlineParseDayNumber(anchor string) int {
	m := regexp.MustCompile(`第(\d+)天`).FindStringSubmatch(anchor)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// validateTimelineMonotonicity 检查相邻章节时间是否倒流（第N天必须单调不减）
func (ov *OutlineValidator) validateTimelineMonotonicity() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			var prevDay int
			hasPrev := false
			for i, chapter := range volume.Chapters {
				day := outlineParseDayNumber(chapter.Timeline.Anchor)
				if day == 0 {
					continue // 无天数锚点，跳过
				}
				if hasPrev && day < prevDay {
					// 倒叙/回溯章不报: duration/transition 提到回溯、回顾、倒叙、闪回时时间倒退是设计使然
					ctx := chapter.Timeline.Duration + " " + chapter.Timeline.Transition + " " + chapter.Summary
					if containsAnyOutlineValidatorText(ctx, "回溯", "回顾", "倒叙", "闪回", "回放", "回忆") {
						prevDay, hasPrev = day, true
						continue
					}
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "timeline_monotonicity",
						Severity:    "major",
						Location:    chapter.ID,
						Description: fmt.Sprintf("时间倒流: 本章时间锚点 %q (第%d天) 早于上一章 %q (第%d天)", chapter.Timeline.Anchor, day, volume.Chapters[i-1].Timeline.Anchor, prevDay),
						Impact:      "读者盯着倒计时读，天数倒退直接暴露编排粗糙",
						Fix:         "修正本章 anchor 为不小于上一章的天数；若确为倒叙，需在 transition 说明",
					})
				}
				prevDay, hasPrev = day, true
			}
		}
	}
}

// validateTitleUniqueness 检查章节标题是否重复（重名会让读者和系统都无法定位）
func (ov *OutlineValidator) validateTitleUniqueness() {
	stripTitle := func(title string) string {
		// 只去掉"第N章："前缀（章号不同、标题本体相同也算重名）
		// 注意: （一）（二）（三）序号后缀是区分手段, 不 strip
		return regexp.MustCompile(`^第\d+章[：:]`).ReplaceAllString(title, "")
	}
	seen := make(map[string]string)
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				base := stripTitle(chapter.Title)
				if base == "" {
					continue
				}
				if prevID, ok := seen[base]; ok {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "title_uniqueness",
						Severity:    "major",
						Location:    chapter.ID,
						Description: fmt.Sprintf("章节标题与 %s 重复: %q", prevID, base),
						Impact:      "重名章节无法区分，读者和工具引用都会错位",
						Fix:         "给同标题连续章节加序号后缀（一）（二）（三）或改写标题",
					})
				} else {
					seen[base] = chapter.ID
				}
			}
		}
	}
}

// validateAnchorFormat 检查 timeline.anchor 是否用了"第N章"等错误格式（应为"第N天"或日期/时间）
func (ov *OutlineValidator) validateAnchorFormat() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				anchor := chapter.Timeline.Anchor
				if anchor == "" {
					continue
				}
				if regexp.MustCompile(`^第\d+章$`).MatchString(anchor) {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "anchor_format",
						Severity:    "minor",
						Location:    chapter.ID,
						Description: fmt.Sprintf("时间锚点格式错误: %q 是章号不是时间", anchor),
						Impact:      "锚点无法参与时间连续性校验，且暴露批量生成的粗糙",
						Fix:         "改为日期/天数锚点，如: 第N天",
					})
				}
			}
		}
	}
}

// validateLocationContinuity 检查角色位置瞬移（跨大区域移动但无过渡说明）
// 结构化优先: 只查 timeline.transition 是否填写（这是生成时本就该填的结构化字段）
// 不用 beats 文本匹配移动动词——那会误报（用户原则: 缺失结构化字段=缺陷, 喂 improve 补, 不猜文本）
func (ov *OutlineValidator) validateLocationContinuity() {
	// 区域提取: "玄云宗·外门·灵药园" → "玄云宗"; 只比较顶层区域, 区域内移动(外门内不同地点)不报
	regionOf := func(loc string) string {
		if i := strings.Index(loc, "·"); i > 0 {
			return strings.TrimSpace(loc[:i])
		}
		return strings.TrimSpace(loc)
	}
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for i, chapter := range volume.Chapters {
				if i == 0 || chapter.StateAnchor.Location == "" {
					continue
				}
				prev := volume.Chapters[i-1].StateAnchor.Location
				if prev == "" {
					continue
				}
				prevRegion := regionOf(prev)
				curRegion := regionOf(chapter.StateAnchor.Location)
				if prevRegion == "" || prevRegion == curRegion {
					continue // 同区域或未知区域，不报
				}
				// 跨大区域移动: 结构化字段 timeline.transition 必须非空
				if chapter.Timeline.Transition == "" {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "location_continuity",
						Severity:    "medium",
						Location:    chapter.ID,
						Description: fmt.Sprintf("跨区域移动缺过渡说明: 上一章在 %q，本章在 %q，但 timeline.transition 为空", prev, chapter.StateAnchor.Location),
						Impact:      "角色瞬间跨越大区域破坏空间连贯性",
						Fix:         "在 timeline.transition 填写移动过程（前往/抵达/传送/离开），供后续阶段参考",
					})
				}
			}
		}
	}
}
