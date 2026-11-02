package rpg

import (
	"fmt"
	"strings"
)

// OutlineValidator 大纲验证器
type OutlineValidator struct {
	Outline     *StoryOutline
	Issues      []OutlineIssue
	Warnings    []OutlineWarning
	Suggestions []OutlineSuggestion
}

// OutlineIssue 大纲问题（严重）
type OutlineIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"` // critical, major, minor
	Location    string `json:"location"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Fix         string `json:"fix"`
}

// OutlineWarning 大纲警告（需要注意）
type OutlineWarning struct {
	Type        string `json:"type"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// OutlineSuggestion 改进建议
type OutlineSuggestion struct {
	Type      string `json:"type"`
	Location  string `json:"location"`
	Current   string `json:"current"`
	Suggested string `json:"suggested"`
	Reason    string `json:"reason"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid      bool                `json:"is_valid"`
	IssueCount   int                 `json:"issue_count"`
	WarningCount int                 `json:"warning_count"`
	Issues       []OutlineIssue      `json:"issues"`
	Warnings     []OutlineWarning    `json:"warnings"`
	Suggestions  []OutlineSuggestion `json:"suggestions"`
	Summary      string              `json:"summary"`
}

// NewOutlineValidator 创建验证器
func NewOutlineValidator(outline *StoryOutline) *OutlineValidator {
	return &OutlineValidator{
		Outline:     outline,
		Issues:      make([]OutlineIssue, 0),
		Warnings:    make([]OutlineWarning, 0),
		Suggestions: make([]OutlineSuggestion, 0),
	}
}

func outlineChapterBeats(chapter StoryChapter) []string {
	if len(chapter.Beats) > 0 {
		return chapter.Beats
	}

	var beats []string
	for _, scene := range chapter.Scenes {
		beats = append(beats, scene.Beats...)
	}
	return beats
}

func outlineChapterHasStateChange(chapter StoryChapter) bool {
	if strings.TrimSpace(chapter.StateChange) != "" {
		return true
	}
	if len(chapter.ResourceLedger) > 0 || len(chapter.StorylineAdvances) > 0 {
		return true
	}
	if len(chapter.Mysteries.Planted) > 0 || len(chapter.Mysteries.Resolved) > 0 {
		return true
	}
	for _, event := range chapter.Events {
		if storyEventHasStateChange(event) {
			return true
		}
	}
	return false
}

func storyEventHasStateChange(event StoryEvent) bool {
	if strings.TrimSpace(event.Result) != "" || strings.TrimSpace(event.Change) != "" || strings.TrimSpace(event.Details) != "" {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(event.GetAction())) {
	case "acquire", "use", "lose", "move", "enter", "leave", "combat", "defeat", "escape",
		"learn", "awaken", "upgrade", "master", "discover", "reveal", "meet", "befriend",
		"set", "progress", "achieve", "activate", "transform", "recover", "afflict", "establish":
		return true
	}

	return false
}

// Validate 执行完整验证
func (ov *OutlineValidator) Validate() *ValidationResult {
	// 1. 结构完整性检查
	ov.validateStructure()

	// 2. 角色一致性检查
	ov.validateCharacterConsistency()

	// 3. 剧情逻辑检查
	ov.validatePlotLogic()

	// 4. 节奏和张力检查
	ov.validatePacing()

	// 5. 冲突和动机检查
	ov.validateConflict()

	// 6. 转换合理性检查
	ov.validateTransitions()

	// 7. 重复和冗余检查
	ov.validateRedundancy()

	// 8. 可行性检查（RPG推演角度）
	ov.validateFeasibility()

	// 9. 时间线一致性检查
	ov.validateTimeline()

	// 10. 状态锚点一致性检查
	ov.validateStateAnchor()

	// 11. 敌人清单检查
	ov.validateEnemies()

	// 12. 资源账本检查
	ov.validateResourceLedger()

	// 13. 场景拆分检查
	ov.validateScenes()

	// 14. 伏笔/谜题追踪检查
	ov.validateMysteries()

	// 15. Boss跨章连续性检查
	ov.validateBossContinuity()

	// 16. 阵营/等级体系检查
	ov.validateFactionTiers()

	// 17. 角色死亡/复活连续性检查
	ov.validateCharacterDeathContinuity()

	// 18. 全角色修为单调性检查
	ov.validateCultivationContinuity()

	// 19. 物品获得→使用连续性检查
	ov.validateItemUsageContinuity()

	// 20. 事件语义完整性检查（结构化字段缺失/误用）
	ov.validateEventSemantics()

	// 21. 时间单调性检查（相邻章天数倒流）
	ov.validateTimelineMonotonicity()

	// 22. 标题唯一性检查（章节重名）
	ov.validateTitleUniqueness()

	// 23. 时间锚点格式检查（"第N章"误用为锚点）
	ov.validateAnchorFormat()

	// 24. 位置连续性检查（角色瞬移）
	ov.validateLocationContinuity()

	// Optional storyline texture hints. These stay as suggestions only, so the
	// outline can remain loose when a chapter does not need explicit arc notes.
	ov.validateStorylineTexture()

	return &ValidationResult{
		IsValid:      len(ov.Issues) == 0,
		IssueCount:   len(ov.Issues),
		WarningCount: len(ov.Warnings),
		Issues:       ov.Issues,
		Warnings:     ov.Warnings,
		Suggestions:  ov.Suggestions,
		Summary:      ov.generateSummary(),
	}
}

// validateStructure 验证结构完整性
func (ov *OutlineValidator) validateStructure() {
	if ov.Outline == nil {
		ov.Issues = append(ov.Issues, OutlineIssue{
			Type:        "structure",
			Severity:    "critical",
			Location:    "root",
			Description: "大纲为空",
			Impact:      "无法进行任何推演",
			Fix:         "创建有效的大纲结构",
		})
		return
	}

	if len(ov.Outline.Parts) == 0 {
		ov.Issues = append(ov.Issues, OutlineIssue{
			Type:        "structure",
			Severity:    "critical",
			Location:    "parts",
			Description: "大纲缺少部分(Parts)",
			Impact:      "故事没有主要结构",
			Fix:         "至少添加一个Part",
		})
	}

	// 检查每个Part
	for pi, part := range ov.Outline.Parts {
		if part.ID == "" {
			ov.Issues = append(ov.Issues, OutlineIssue{
				Type:        "structure",
				Severity:    "major",
				Location:    fmt.Sprintf("Part[%d]", pi),
				Description: "Part缺少ID",
				Impact:      "无法引用和追踪该部分",
				Fix:         "为Part添加唯一ID",
			})
		}

		if part.Title == "" {
			ov.Warnings = append(ov.Warnings, OutlineWarning{
				Type:        "structure",
				Location:    fmt.Sprintf("Part[%d]", pi),
				Description: "Part缺少标题",
				Suggestion:  "添加描述性标题",
			})
		}

		// 检查Volume
		if len(part.Volumes) == 0 {
			ov.Warnings = append(ov.Warnings, OutlineWarning{
				Type:        "structure",
				Location:    fmt.Sprintf("Part[%d].%s", pi, part.ID),
				Description: "Part缺少卷(Volumes)",
				Suggestion:  "至少添加一个Volume",
			})
		}

		for vi, volume := range part.Volumes {
			if volume.ID == "" {
				ov.Issues = append(ov.Issues, OutlineIssue{
					Type:        "structure",
					Severity:    "major",
					Location:    fmt.Sprintf("Part[%d].Volume[%d]", pi, vi),
					Description: "Volume缺少ID",
					Impact:      "无法引用该卷",
					Fix:         "为Volume添加唯一ID",
				})
			}

			// 检查Chapter
			if len(volume.Chapters) == 0 {
				ov.Warnings = append(ov.Warnings, OutlineWarning{
					Type:        "structure",
					Location:    fmt.Sprintf("Part[%d].Volume[%d].%s", pi, vi, volume.ID),
					Description: "Volume缺少章节",
					Suggestion:  "至少添加一个Chapter",
				})
			}

			for ci, chapter := range volume.Chapters {
				location := fmt.Sprintf("Part[%d].Volume[%d].Chapter[%d].%s", pi, vi, ci, chapter.ID)
				beats := outlineChapterBeats(chapter)

				if chapter.ID == "" {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "structure",
						Severity:    "major",
						Location:    location,
						Description: "Chapter缺少ID",
						Impact:      "无法引用该章节",
						Fix:         "为Chapter添加唯一ID",
					})
				}

				if chapter.Title == "" {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "structure",
						Severity:    "minor",
						Location:    location,
						Description: "Chapter缺少标题",
						Impact:      "难以识别章节内容",
						Fix:         "添加章节标题",
					})
				}

				// 检查关键字段
				if len(beats) > 0 && beats[0] == "" {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: "Chapter缺少开场节拍",
						Suggestion:  "添加开场节拍以明确章节起点",
					})
				}

				if len(beats) > 0 && beats[len(beats)-1] == "" {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: "Chapter缺少结束节拍",
						Suggestion:  "添加结束节拍以明确章节终点",
					})
				}

				if !outlineChapterHasStateChange(chapter) {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: "Chapter缺少状态变化",
						Suggestion:  "明确章节带来的状态变化",
					})
				}

				if chapter.Conflict == "" {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: "Chapter缺少冲突描述",
						Suggestion:  "添加冲突以驱动剧情",
					})
				}

				// 检查节拍数量
				if len(beats) == 0 {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "content",
						Severity:    "major",
						Location:    location,
						Description: "Chapter缺少情节节拍",
						Impact:      "无法进行剧情推演",
						Fix:         "至少添加一个情节节拍",
					})
				} else if len(beats) < 3 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: fmt.Sprintf("Chapter节拍过少(%d个)", len(beats)),
						Suggestion:  "建议至少3-5个节拍以支撑完整场景",
					})
				} else if len(beats) > 10 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "content",
						Location:    location,
						Description: fmt.Sprintf("Chapter节拍过多(%d个)", len(beats)),
						Suggestion:  "考虑拆分为多个章节",
					})
				}
			}
		}
	}
}

// validateCharacterConsistency 验证角色一致性
func (ov *OutlineValidator) validateCharacterConsistency() {
	characterAppearances := make(map[string][]string)

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				location := chapter.ID

				// 记录角色出场
				for _, char := range chapter.Characters {
					characterAppearances[char] = append(characterAppearances[char], location)
				}

				// 检查事件中的角色
				for _, event := range chapter.Events {
					for _, char := range event.Characters {
						if !contains(chapter.Characters, char) {
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type:        "character",
								Severity:    "major",
								Location:    location,
								Description: fmt.Sprintf("事件中的角色 '%s' 不在章节角色列表中", char),
								Impact:      "角色出场逻辑不一致",
								Fix:         "将角色添加到章节角色列表或修改事件",
							})
						}
					}
				}
			}
		}
	}

	// 检查角色是否突然出现（没有铺垫）
	for char, appearances := range characterAppearances {
		if len(appearances) > 0 {
			// 检查首次出场是否有介绍
			firstAppearance := appearances[0]
			// 这里可以进一步检查首次出场章节是否有角色介绍
			_ = firstAppearance
			_ = char
		}
	}
}

// validatePlotLogic 验证剧情逻辑
func (ov *OutlineValidator) validatePlotLogic() {
	prevState := ""

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for ci, chapter := range volume.Chapters {
				location := chapter.ID
				beats := outlineChapterBeats(chapter)

				// 检查状态连续性
				if ci > 0 && prevState != "" {
					if len(beats) == 0 {
						continue
					}
					// 检查当前章节的opening_beat是否承接上一章的closing_beat
					if !strings.Contains(beats[0], "继续") &&
						!strings.Contains(beats[0], "随后") &&
						!strings.Contains(beats[0], "紧接着") {
						// 可能缺少过渡
						if chapter.StateChange != "" && !strings.Contains(chapter.StateChange, prevState) {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type:        "logic",
								Location:    location,
								Description: "章节开场可能缺少与上一章的过渡",
								Suggestion:  "确保剧情连贯性",
							})
						}
					}
				}

				// 检查状态变化是否合理
				if !outlineChapterHasStateChange(chapter) && len(chapter.Events) > 0 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "logic",
						Location:    location,
						Description: "有事件但无状态变化",
						Suggestion:  "明确事件带来的状态变化",
					})
				}

				// 检查冲突是否解决
				if chapter.Conflict != "" && len(beats) > 1 {
					if !outlineChapterHasConflictOutcome(chapter, beats[len(beats)-1]) {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "logic",
							Location:  location,
							Current:   beats[len(beats)-1],
							Suggested: beats[len(beats)-1] + "（冲突得到解决或升级）",
							Reason:    "确保冲突有明确的阶段性结果",
						})
					}
				}

				prevState = chapter.StateChange
			}
		}
	}
}

func outlineChapterHasConflictOutcome(chapter StoryChapter, closingBeat string) bool {
	textParts := []string{
		closingBeat,
		chapter.StateChange,
		chapter.Summary,
	}
	for _, event := range chapter.Events {
		textParts = append(textParts, event.Change, event.Result, event.Details, event.Context)
	}
	for _, advance := range chapter.StorylineAdvances {
		textParts = append(textParts, advance.Stage, advance.Change, advance.Consequence, advance.Pressure)
	}
	for _, resolved := range chapter.Mysteries.Resolved {
		textParts = append(textParts, resolved.Resolution)
	}
	text := normalizeOutlineValidatorText(strings.Join(textParts, " "))
	if strings.TrimSpace(text) == "" {
		return false
	}
	return containsAnyOutlineValidatorText(text,
		"解决", "结束", "胜利", "失败", "击退", "击败", "逃生", "撤离", "脱困", "完成",
		"升级", "恶化", "暴露", "追杀", "代价", "压力", "后果", "新局面", "决定", "获得",
		"resolved", "completed", "defeated", "escaped", "survived", "upgraded", "revealed", "consequence", "pressure")
}

// validatePacing 验证节奏和张力
func (ov *OutlineValidator) validatePacing() {
	fastCount := 0
	slowCount := 0

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				switch chapter.Pacing {
				case "fast":
					fastCount++
				case "slow":
					slowCount++
				case "":
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "pacing",
						Location:    chapter.ID,
						Description: "章节缺少节奏标记",
						Suggestion:  "标记为 fast/normal/slow",
					})
				}

				// 检查连续快节奏
				if fastCount > 3 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "pacing",
						Location:    chapter.ID,
						Description: "连续多个快节奏章节",
						Suggestion:  "插入慢节奏章节让读者喘息",
					})
					fastCount = 0
				}

				// 检查连续慢节奏
				if slowCount > 3 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "pacing",
						Location:    chapter.ID,
						Description: "连续多个慢节奏章节",
						Suggestion:  "加快节奏或增加冲突",
					})
					slowCount = 0
				}
			}
		}
	}
}

// validateConflict 验证冲突和动机
func (ov *OutlineValidator) validateConflict() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				// 检查冲突强度
				if chapter.Conflict != "" {
					conflictLower := strings.ToLower(chapter.Conflict)

					weakIndicators := []string{"有点", "轻微", "小", "简单"}
					for _, indicator := range weakIndicators {
						if strings.Contains(conflictLower, indicator) {
							ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
								Type:      "conflict",
								Location:  chapter.ID,
								Current:   chapter.Conflict,
								Suggested: "增强冲突强度",
								Reason:    "冲突过于温和，难以驱动剧情",
							})
							break
						}
					}
				}

				// 检查事件与冲突的关联
				if chapter.Conflict != "" && len(chapter.Events) == 0 {
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "conflict",
						Severity:    "major",
						Location:    chapter.ID,
						Description: "有冲突描述但无具体事件",
						Impact:      "冲突无法落地执行",
						Fix:         "添加具体事件来体现冲突",
					})
				}
			}
		}
	}
}

// validateTransitions 验证转换合理性
func (ov *OutlineValidator) validateTransitions() {
	// 检查场景转换是否突兀
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for ci := 1; ci < len(volume.Chapters); ci++ {
				prevChapter := volume.Chapters[ci-1]
				currChapter := volume.Chapters[ci]

				// 检查地点转换
				if prevChapter.Location != "" && currChapter.Location != "" &&
					prevChapter.Location != currChapter.Location {
					// 地点变化，检查是否有合理的转换
					firstBeat := ""
					currBeats := outlineChapterBeats(currChapter)
					if len(currBeats) > 0 {
						firstBeat = currBeats[0]
					}
					if !outlineBeatHasLocationTransition(firstBeat) {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type:     "transition",
							Location: currChapter.ID,
							Description: fmt.Sprintf("地点从 '%s' 变为 '%s' 但缺少过渡描述",
								prevChapter.Location, currChapter.Location),
							Suggestion: "在开场节拍中添加地点转换说明",
						})
					}
				}
			}
		}
	}
}

func outlineBeatHasLocationTransition(beat string) bool {
	beat = strings.TrimSpace(beat)
	if beat == "" {
		return false
	}
	for _, marker := range []string{"来到", "前往", "到达", "回到", "返回", "抵达", "赶到", "进入"} {
		if strings.Contains(beat, marker) {
			return true
		}
	}
	return false
}

// validateRedundancy 验证重复和冗余
func (ov *OutlineValidator) validateRedundancy() {
	// 检查重复的事件类型
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			eventTypeCount := make(map[string]int)

			for _, chapter := range volume.Chapters {
				for _, event := range chapter.Events {
					eventType := strings.TrimSpace(event.GetAction())
					if eventType == "" {
						eventType = strings.TrimSpace(event.Type)
					}
					if eventType == "" {
						continue
					}
					eventTypeCount[eventType]++
				}
			}

			// 检查过度使用的事件类型
			for eventType, count := range eventTypeCount {
				if count > len(volume.Chapters)/2 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "redundancy",
						Location:    volume.ID,
						Description: fmt.Sprintf("事件类型 '%s' 使用过于频繁(%d次)", eventType, count),
						Suggestion:  "增加事件类型多样性",
					})
				}
			}
		}
	}
}

// validateFeasibility 验证可行性（RPG推演角度）
func (ov *OutlineValidator) validateFeasibility() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				// 检查战斗场景的可行性
				combatEvents := 0
				for _, event := range chapter.Events {
					action := strings.ToLower(strings.TrimSpace(event.GetAction()))
					eventType := strings.ToLower(strings.TrimSpace(event.Type))
					if action == "combat" || action == "defeat" || eventType == "combat" || eventType == "battle" {
						combatEvents++
					}
				}

				if combatEvents > 3 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "feasibility",
						Location:    chapter.ID,
						Description: fmt.Sprintf("单章节包含%d场战斗", combatEvents),
						Suggestion:  "考虑分散战斗或合并，避免疲劳",
					})
				}

				// 检查角色数量
				if len(chapter.Characters) > 10 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "feasibility",
						Location:    chapter.ID,
						Description: fmt.Sprintf("章节角色过多(%d个)", len(chapter.Characters)),
						Suggestion:  "减少角色数量或分散到多个章节",
					})
				}
			}
		}
	}
}

// validateTimeline 验证时间线一致性
func (ov *OutlineValidator) validateTimeline() {
	var prevAnchor string

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for i, chapter := range volume.Chapters {
				// 检查是否有时间线信息
				if chapter.Timeline.Anchor == "" {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "timeline",
						Location:    chapter.ID,
						Description: "章节缺少时间锚点(timeline.anchor)",
						Suggestion:  "添加 anchor 字段，如：\"第3天傍晚\"、\"三个月后\"",
					})
				}

				// 检查时间跳跃是否有过渡说明
				if chapter.Timeline.TimeJump && chapter.Timeline.Transition == "" {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type:        "timeline",
						Location:    chapter.ID,
						Description: "时间跳跃缺少过渡说明(time_jump=true 但 transition 为空)",
						Suggestion:  "添加 transition 字段说明跳跃期间发生了什么",
					})
				}

				// 检查与上一章的时间连续性
				if i > 0 && prevAnchor != "" && chapter.Timeline.Anchor != "" {
					// TODO: 更智能的时间解析和比较
					// 目前只是简单检查是否有时间信息
				}

				// 检查第一章的时间锚点
				if i == 0 && chapter.Timeline.Anchor == "" {
					ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
						Type:      "timeline",
						Location:  chapter.ID,
						Current:   "缺少时间锚点",
						Suggested: "建议设置 anchor 为 \"第1天\" 或 \"故事开始\"",
						Reason:    "第一章需要为整个故事建立时间基准",
					})
				}

				prevAnchor = chapter.Timeline.Anchor
			}
		}
	}
}

func (ov *OutlineValidator) validateStateAnchor() {
	var prev StoryStateAnchor
	firstChapter := true

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for i, chapter := range volume.Chapters {
				sa := chapter.StateAnchor

				// Check chapter 1 has baseline
				if i == 0 && firstChapter {
					if sa.Cultivation == "" && len(sa.Allies) == 0 && sa.SpiritStones == 0 {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "state_anchor",
							Location:  chapter.ID,
							Current:   "缺少初始状态锚点",
							Suggested: "为第一章设置 state_anchor：修炼境界、资源数量、初始盟友",
							Reason:    "第一章需要建立状态基准线，后续章节才能对比变化",
						})
					}
					firstChapter = false
				}

				// Cross-chapter cultivation consistency
				if !firstChapter && sa.Cultivation != "" && prev.Cultivation != "" {
					if sa.Cultivation != prev.Cultivation {
						// Cultivation changed — check if there was a breakthrough event
						if !hasCultivationChangeEvent(chapter.Events, prev.Cultivation, sa.Cultivation) {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type:        "state_anchor",
								Location:    chapter.ID,
								Description: fmt.Sprintf("修炼境界从 '%s' 变为 '%s'，但本章事件中未发现对应的突破事件", prev.Cultivation, sa.Cultivation),
								Suggestion:  "添加一个突破相关的事件，或修正 state_anchor",
							})
						}
					}
				}

				// Resource arithmetic check
				if !firstChapter && sa.SpiritStones > 0 && prev.SpiritStones > 0 {
					delta := sa.SpiritStones - prev.SpiritStones
					if delta < 0 {
						// Loss — check if there's a spend/give-away event
						hasSpend := false
						for _, evt := range chapter.Events {
							if evt.Type == "item" && (evt.Change == "lost" || evt.Change == "used" || evt.Change == "consumed") {
								hasSpend = true
								break
							}
						}
						if !hasSpend {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type:        "state_anchor",
								Location:    chapter.ID,
								Description: fmt.Sprintf("灵石从 %d 减少到 %d（- %d），但前一章事件中未发现消耗/失去", prev.SpiritStones, sa.SpiritStones, -delta),
								Suggestion:  "添加灵石消耗事件，或修正 state_anchor 数值",
							})
						}
					}
				}

				// Injury continuity
				if len(prev.Injuries) > 0 && len(sa.Injuries) == 0 {
					hasRecovery := false
					for _, evt := range chapter.Events {
						if evt.Type == "status" && (evt.Change == "resolved" || evt.Change == "recovered" || evt.Change == "healed") {
							hasRecovery = true
							break
						}
					}
					if !hasRecovery {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type:        "state_anchor",
							Location:    chapter.ID,
							Description: fmt.Sprintf("上一章存在伤势 %v，本章 state_anchor 却无伤势且无恢复事件", prev.Injuries),
							Suggestion:  "添加伤势恢复事件，或在 state_anchor 中保留伤势",
						})
					}
				}

				prev = sa
			}
		}
	}
}

func hasCultivationChangeEvent(events []StoryEvent, from, to string) bool {
	for _, evt := range events {
		if storyEventSupportsCultivationChange(evt, from, to) {
			return true
		}
	}
	return false
}

func storyEventSupportsCultivationChange(evt StoryEvent, from, to string) bool {
	eventType := normalizeOutlineValidatorText(evt.Type)
	change := normalizeOutlineValidatorText(evt.Change)
	subject := normalizeOutlineValidatorText(evt.Subject)
	action := normalizeOutlineValidatorText(evt.Action)
	targetType := normalizeOutlineValidatorText(evt.TargetType)
	text := normalizeOutlineValidatorText(strings.Join([]string{
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

	if eventType == "status" && (strings.Contains(change, "突破") ||
		strings.Contains(subject, "修为") || strings.Contains(subject, "境界")) {
		return true
	}

	if !containsAnyOutlineValidatorText(text,
		"突破", "breakthrough", "进阶", "晋升", "升级", "upgrade", "觉醒", "awaken", "进化", "evolution") {
		return false
	}

	if action == "breakthrough" || action == "upgrade" || action == "awaken" || action == "transform" {
		return true
	}
	if eventType == "status" || eventType == "gate" || eventType == "premise" ||
		targetType == "status" || targetType == "skill" || targetType == "premise" {
		return true
	}
	if containsAnyOutlineValidatorText(text, "修为", "境界", "cultivation", "等级", "基因", "适配", "能力") {
		return true
	}
	return cultivationTextMentionsEndpoint(text, from) || cultivationTextMentionsEndpoint(text, to)
}

func normalizeOutlineValidatorText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsAnyOutlineValidatorText(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, normalizeOutlineValidatorText(needle)) {
			return true
		}
	}
	return false
}

func cultivationTextMentionsEndpoint(text, endpoint string) bool {
	endpoint = normalizeOutlineValidatorText(endpoint)
	if endpoint == "" {
		return false
	}
	if strings.Contains(text, endpoint) {
		return true
	}
	for _, token := range strings.FieldsFunc(endpoint, func(r rune) bool {
		return r == '（' || r == '）' || r == '(' || r == ')' || r == ' ' || r == '，' || r == ',' || r == '/' || r == '、'
	}) {
		token = normalizeOutlineValidatorText(token)
		if len([]rune(token)) >= 2 && strings.Contains(text, token) {
			return true
		}
	}
	return false
}
