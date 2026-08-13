package rpg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"novelgen/internal/models"
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

// ValidationReport 验证报告 (用于 compose_bridge.go 兼容)
type ValidationReport struct {
	Result      RPGCheckResult `json:"result"`
	Suggestions []Suggestion   `json:"suggestions"`
}

// Suggestion 建议项 (用于 compose_bridge.go 兼容)
type Suggestion struct {
	Type     string `json:"type"`     // error, warning, info
	Category string `json:"category"` // system, plot, character, etc.
	Message  string `json:"message"`
	Action   string `json:"action"`
	Priority int    `json:"priority"` // 1-10
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

func (ov *OutlineValidator) validateEnemies() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.Name == "" {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "enemy", Severity: "major", Location: chapter.ID,
							Description: "敌人清单中存在空名称",
							Fix:         "为每个敌人设置明确的名称",
						})
					}
					if enemy.Count <= 0 {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "enemy", Location: chapter.ID,
							Description: fmt.Sprintf("敌人 '%s' 数量为 %d，已自动设为1", enemy.Name, enemy.Count),
							Suggestion:  "明确敌人的出现数量",
						})
					}
				}
				// Check: if chapter has combat events but no enemies listed
				hasCombat := false
				for _, evt := range chapter.Events {
					if evt.Type == "combat" || evt.Action == "combat" {
						hasCombat = true
						break
					}
				}
				if hasCombat && len(chapter.Enemies) == 0 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type: "enemy", Location: chapter.ID,
						Description: "章节有 combat 事件但未声明敌人清单",
						Suggestion:  "在 enemies 中列出本章出现的敌人及其数量",
					})
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateResourceLedger() {
	var prevLedger map[string]int // item → end count from previous chapter

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if len(chapter.ResourceLedger) == 0 {
					prevLedger = nil
					continue
				}
				if prevLedger == nil {
					prevLedger = make(map[string]int)
				}

				for _, entry := range chapter.ResourceLedger {
					// Arithmetic check: Start + Delta should equal End
					expected := entry.Start + entry.Delta
					if expected != entry.End {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "resource_ledger", Severity: "critical",
							Location:    chapter.ID,
							Description: fmt.Sprintf("资源 '%s' 账目不对: %d + %d = %d ≠ %d", entry.Item, entry.Start, entry.Delta, expected, entry.End),
							Fix:         fmt.Sprintf("修正 start/delta/end 使等式成立: %d + %d = %d", entry.Start, entry.Delta, entry.Start+entry.Delta),
						})
					}

					// Continuity: this chapter's Start should match previous chapter's End
					if prevEnd, ok := prevLedger[entry.Item]; ok && prevEnd != entry.Start {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "resource_ledger", Location: chapter.ID,
							Description: fmt.Sprintf("资源 '%s' 与上一章不一致: 上一章结束=%d, 本章开始=%d", entry.Item, prevEnd, entry.Start),
							Suggestion:  fmt.Sprintf("确保本章 start 等于上一章 end (%d)", prevEnd),
						})
					}
					prevLedger[entry.Item] = entry.End
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateScenes() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if len(chapter.Scenes) == 0 {
					ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
						Type: "scenes", Location: chapter.ID,
						Current:   "章节未拆分为场景",
						Suggested: "将章节拆分为2-3个场景，每个场景指定POV/目标/地点/字数",
						Reason:    "场景拆分帮助写作者聚焦每个场景的目标，避免跑题或漏角色",
					})
					continue
				}

				// Check scene order continuity
				for i, scene := range chapter.Scenes {
					if scene.Order != i+1 {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景序号不连续: 期望 %d, 实际 %d", i+1, scene.Order),
							Suggestion:  "确保场景序号从1开始递增",
						})
					}
					if scene.POV == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景 %d 缺少POV角色", scene.Order),
							Suggestion:  "为每个场景指定视角角色",
						})
					}
					if scene.Goal == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景 %d 缺少目标(goal)", scene.Order),
							Suggestion:  "明确这个场景要推进什么、达成什么",
						})
					}
				}

				// Scene characters should be a subset of chapter characters
				chapterChars := make(map[string]bool)
				for _, name := range chapter.Characters {
					chapterChars[name] = true
				}
				for _, scene := range chapter.Scenes {
					for _, name := range scene.Characters {
						if !chapterChars[name] {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "scenes", Location: chapter.ID,
								Description: fmt.Sprintf("场景角色 '%s' 不在章节角色列表中", name),
								Suggestion:  fmt.Sprintf("将 '%s' 添加到章节角色列表", name),
							})
						}
					}
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateMysteries() {
	planted := make(map[string]string) // id → chapter where planted
	resolved := make(map[string]bool)  // id → resolved

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, p := range chapter.Mysteries.Planted {
					if p.ID == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: "planted谜题缺少ID",
							Suggestion:  "为每个planted谜题设置唯一ID",
						})
						continue
					}
					if prevCh, ok := planted[p.ID]; ok {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "mysteries", Severity: "major", Location: chapter.ID,
							Description: fmt.Sprintf("谜题 '%s' 已在 %s 中planted，重复planted", p.ID, prevCh),
							Fix:         "确认是追加线索还是新谜题，追加线索应使用不同的ID",
						})
					}
					planted[p.ID] = chapter.ID
				}
				for _, r := range chapter.Mysteries.Resolved {
					if r.ID == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: "resolved谜题缺少ID",
							Suggestion:  "为每个resolved谜题设置唯一ID",
						})
						continue
					}
					if _, wasPlanted := planted[r.ID]; !wasPlanted {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: fmt.Sprintf("谜题 '%s' 被resolved但之前从未被planted", r.ID),
							Suggestion:  "在前面章节中添加对应的planted，或修改ID",
						})
					}
					resolved[r.ID] = true
				}
			}
		}
	}

	if len(planted) > 0 {
		unresolved := make([]string, 0)
		for id := range planted {
			if !resolved[id] {
				unresolved = append(unresolved, id)
			}
		}
		if len(unresolved) > 0 {
			ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
				Type:      "mysteries",
				Current:   fmt.Sprintf("存在%d个未回收的伏笔", len(unresolved)),
				Suggested: fmt.Sprintf("确认这些伏笔是否计划在后续章节回收: %s", strings.Join(unresolved, ", ")),
				Reason:    "未回收的伏笔可能导致读者感觉故事不完整",
			})
		}
	}
}

func (ov *OutlineValidator) validateBossContinuity() {
	bosses := make(map[string]string) // boss_id → last seen chapter + status

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.BossID == "" {
						continue
					}
					status := enemy.Status
					if status == "" {
						status = "engaged"
					}

					if prev, ok := bosses[enemy.BossID]; ok {
						parts := strings.SplitN(prev, "|", 2)
						prevCh, prevStatus := parts[0], ""
						if len(parts) > 1 {
							prevStatus = parts[1]
						}

						// defeated after engaged = normal progression
						if prevStatus == "defeated" || prevStatus == "escaped" {
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type: "boss_continuity", Severity: "major",
								Location:    chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 在 %s 已 %s，本章不应再出现", enemy.Name, prevCh, prevStatus),
								Fix:         "如果boss未死，请将状态改为escaped并说明去向；如果死亡，请移除后续章节的出场",
							})
						}

						// engaged → engaged without notes = missing transition
						if prevStatus == "engaged" && status == "engaged" {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "boss_continuity", Location: chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 跨章持续战斗中 (%s → %s)，但state_anchor缺少残血说明", enemy.Name, prevCh, chapter.ID),
								Suggestion:  fmt.Sprintf("在 %s 的 state_anchor.notes 中添加boss当前状态，如：'%s残血，甲壳碎裂'", chapter.ID, enemy.Name),
							})
						}
					} else {
						if status == "defeated" || status == "escaped" {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "boss_continuity", Location: chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 首次出场即 %s，缺少前置 engaged 章节", enemy.Name, status),
								Suggestion:  "如果是首次出场即被击败，请将status改为new并在前一章或本章前半部分铺垫",
							})
						}
					}

					bosses[enemy.BossID] = chapter.ID + "|" + status
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateFactionTiers() {
	// Collect faction→tier→first_appearance mapping
	type tierInfo struct {
		chapter   string
		enemyName string
	}
	factions := make(map[string]map[string]tierInfo)

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.Faction == "" {
						continue
					}
					if factions[enemy.Faction] == nil {
						factions[enemy.Faction] = make(map[string]tierInfo)
					}
					if _, seen := factions[enemy.Faction][enemy.Tier]; !seen {
						factions[enemy.Faction][enemy.Tier] = tierInfo{chapter: chapter.ID, enemyName: enemy.Name}
					}
				}
			}
		}
	}

	for faction, tiers := range factions {
		if len(tiers) == 0 {
			continue
		}

		// Suggest defining faction tiers in story setup
		var tierNames []string
		for t := range tiers {
			tierNames = append(tierNames, t)
		}
		if len(tierNames) > 1 {
			ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
				Type:      "faction_tier",
				Location:  faction,
				Current:   fmt.Sprintf("阵营 '%s' 使用了以下tier: %s", faction, strings.Join(tierNames, ", ")),
				Suggested: fmt.Sprintf("在 story_setup.json 的 premises 中定义 '%s' 阵营的完整等级体系", faction),
				Reason:    "统一管理阵营等级体系，便于validator做跨章一致性检查",
			})
		}

		// Check: enemy name should match [faction]_[tier] pattern for consistency
		for tier, info := range tiers {
			if tier == "" {
				ov.Warnings = append(ov.Warnings, OutlineWarning{
					Type: "faction_tier", Location: info.chapter,
					Description: fmt.Sprintf("敌人 '%s' 有faction但没有tier", info.enemyName),
					Suggestion:  fmt.Sprintf("为该敌人指定tier，如：%s_drone", info.enemyName),
				})
			}
		}
	}
}

// validateStorylineTexture adds soft hints for chapters that would benefit from
// clearer arc movement. It never creates issues or warnings.
func (ov *OutlineValidator) validateStorylineTexture() {
	if ov.Outline == nil {
		return
	}

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				hasStorylineEvent := false
				for _, event := range chapter.Events {
					if event.Type == "storyline" || event.GetTargetType() == "storyline" {
						hasStorylineEvent = true
						break
					}
				}

				if hasStorylineEvent && len(chapter.StorylineAdvances) == 0 {
					ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
						Type:      "storyline_texture",
						Location:  chapter.ID,
						Current:   "chapter has storyline event but no storyline_advances",
						Suggested: "Optionally add one storyline_advances entry if the chapter creates a real reveal, pressure, choice, or consequence.",
						Reason:    "This keeps important story arcs from becoming thin while staying optional for chapters where the event is self-evident.",
					})
				}

				for _, advance := range chapter.StorylineAdvances {
					if strings.TrimSpace(advance.StorylineName) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   "storyline_advances entry has no storyline_name",
							Suggested: "Name the setup storyline being moved, or remove the entry if it does not map to a meaningful arc.",
							Reason:    "A named arc helps later agents carry pressure and payoff forward without forcing a rigid template.",
						})
					}
					if strings.TrimSpace(advance.Change) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   "storyline_advances entry has no change",
							Suggested: "Describe the concrete shift in the arc, such as a reveal, setback, choice, escalation, or payoff.",
							Reason:    "A storyline note is useful only when it records an actual dramatic movement.",
						})
						continue
					}
					if strings.TrimSpace(advance.Consequence) == "" && strings.TrimSpace(advance.Pressure) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   advance.Change,
							Suggested: "If useful, add either consequence or pressure so the change creates forward energy.",
							Reason:    "Storyline advancement is stronger when it changes what characters risk, know, want, or must do next.",
						})
					}
				}
			}
		}
	}
}

// deathInfo 角色死亡记录
type deathInfo struct {
	chapterID string
	how       string
}

// 死亡关键词（用于 defeat/combat 结果与 kill/die/devour/swallow 动作）
var outlineDeathKeywords = []string{"吞噬", "死亡", "击杀", "杀死", "陨落", "毙命", "灰飞烟灭", "身亡"}

// 文本死亡关键词（details/result 中提到角色名 + 以下关键词判定死亡）
var outlineDeathTextKeywords = []string{"吞噬成功", "已死", "死亡", "陨落", "灰飞烟灭"}

// 非真实死亡语境：模拟/推演/预演/幻境/梦境/测试中的"死亡"不算死亡
var outlineNonDeathContexts = []string{"模拟死亡", "推演死亡", "预演死亡", "幻境中死亡", "梦里死亡", "梦境死亡", "模拟器", "推演中", "预演中", "幻境", "梦境", "模拟场景", "测试死亡"}

// 吞噬类能力/系统词：出现这些词时"吞噬"是能力名而非死亡动作
var outlineDevourAbilityWords = []string{"吞噬系统", "吞噬能力", "吞噬外挂", "吞噬属性", "吞噬之力", "吞噬功法", "吞噬能量", "吞噬气运"}

// outlineTextHasNonDeathContext 判断文本是否处于非真实死亡语境（模拟/推演/幻境等）
func outlineTextHasNonDeathContext(text string) bool {
	for _, ctx := range outlineNonDeathContexts {
		if strings.Contains(text, ctx) {
			return true
		}
	}
	return false
}

// outlineTextHasDevourAbility 判断文本中的"吞噬"是否只是能力/系统名（非死亡动作）
func outlineTextHasDevourAbility(text string) bool {
	for _, w := range outlineDevourAbilityWords {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

// validateCharacterDeathContinuity 角色死亡/复活连续性检查
// 扫描全部章节，记录已死亡角色；后续章节中该角色再次出场即报 major 问题；
// 若后续事件明确复活该角色，则清除死亡标记。
func (ov *OutlineValidator) validateCharacterDeathContinuity() {
	if ov.Outline == nil {
		return
	}

	deaths := make(map[string]deathInfo) // 规范化角色名 → 死亡信息
	flagged := make(map[string]bool)     // 角色名|章节 → 已报过

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				chapterChars := outlineChapterCharacterNames(chapter)

				// 先处理本章复活事件：复活后同章出场不报问题
				for _, evt := range chapter.Events {
					// 时间线重置/回档 = 群体复活，清空全部死亡标记
					evtText := normalizeOutlineValidatorText(evt.Details + " " + evt.Result + " " + evt.Change)
					if strings.Contains(evtText, "时间线重置") || strings.Contains(evtText, "回档") {
						deaths = make(map[string]deathInfo)
						break
					}
					for _, revived := range outlineEventRevivedCharacters(evt, chapterChars) {
						delete(deaths, revived)
					}
				}

				// 检查本章出场的已死亡角色
				for _, name := range chapterChars {
					info, ok := deaths[name]
					if !ok {
						continue
					}
					key := name + "\x00" + chapter.ID
					if flagged[key] {
						continue
					}
					flagged[key] = true
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "death_continuity",
						Severity:    "major",
						Location:    chapter.ID,
						Description: fmt.Sprintf("角色 %s 已在 %s 死亡/被吞噬，本章不应再出场", name, info.chapterID),
						Impact:      "死亡角色再次出场会破坏死亡事件的严肃性与剧情连贯性",
						Fix:         "为该角色补充复活/重生事件，或移除本章中的出场",
					})
				}

				// 按事件顺序更新死亡/复活标记
				for _, evt := range chapter.Events {
					for _, victim := range outlineEventDeadVictims(evt, chapterChars) {
						deaths[victim] = deathInfo{chapterID: chapter.ID, how: normalizeOutlineValidatorText(evt.GetAction())}
					}
					for _, revived := range outlineEventRevivedCharacters(evt, chapterChars) {
						delete(deaths, revived)
					}
				}
			}
		}
	}
}

// outlineChapterCharacterNames 收集章节中所有出场角色名（结构化字段，不含 summary）
func outlineChapterCharacterNames(chapter StoryChapter) []string {
	var names []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range chapter.Characters {
		add(name)
	}
	for _, evt := range chapter.Events {
		for _, name := range evt.Characters {
			add(name)
		}
		add(evt.GetActor())
		add(evt.GetTarget())
		for _, enemy := range evt.Enemies {
			add(enemy.Name)
		}
	}
	for _, enemy := range chapter.Enemies {
		add(enemy.Name)
	}
	return names
}

// outlineEventDeadVictims 判定事件中的死亡角色
// 概念/机制名：出现在 events 里但不是真实角色（不应参与死亡追踪）
var outlineNonCharacterConcepts = []string{"死亡回档", "回档", "时间线", "日志系统", "系统", "天道", "规则", "世界意志"}

// outlineIsConceptName 判断名字是否为概念/机制名（非真实角色）
func outlineIsConceptName(name string) bool {
	for _, c := range outlineNonCharacterConcepts {
		if name == c {
			return true
		}
	}
	return false
}

func outlineEventDeadVictims(evt StoryEvent, chapterChars []string) []string {
	var victims []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !outlineIsConceptName(name) && !contains(victims, name) {
			victims = append(victims, name)
		}
	}

	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	target := normalizeOutlineName(evt.GetTarget())
	change := normalizeOutlineName(evt.Change)
	text := normalizeOutlineValidatorText(evt.Details + " " + evt.Result)

	// 文本层守卫：非真实死亡语境（模拟/推演/幻境/梦境）中的"死亡"不算死亡。
	// 结构化字段（action/change）是主信号，因此该守卫现在很少触发。
	if outlineTextHasNonDeathContext(text) {
		return victims
	}
	// 文本层守卫："吞噬"仅是能力/系统名（吞噬系统/吞噬能力/吞噬外挂）时不算死亡动作。
	if outlineTextHasDevourAbility(text) {
		return victims
	}

	// 1. 结构化动作：kill/die/devour/swallow/defeat 直接判定死亡。
	//    die 类动作死亡的是执行者，其余动作死亡的是目标。
	switch {
	case strings.Contains(action, "die"):
		add(evt.GetActor())
	case strings.Contains(action, "kill"), strings.Contains(action, "devour"),
		strings.Contains(action, "swallow"), strings.Contains(action, "defeat"):
		add(target)
	}

	// 2. 结构化 change 明确为死亡结果：目标死亡（若 actor==target 且为自我死亡则记执行者）。
	if outlineChangeImpliesDeath(change) {
		if outlineChangeImpliesSelfDeath(change) && target != "" && normalizeOutlineName(evt.GetActor()) == target {
			add(evt.GetActor())
		} else {
			add(target)
		}
	}

	// 3. change 含"吞噬"且目标是真实角色名（且文本未把它当能力名）→ 目标被吞噬死亡。
	if strings.Contains(change, "吞噬") && target != "" && !outlineTextHasDevourAbility(text) {
		add(target)
	}

	// 4. 仅作为兜底：combat/defeat 动作 + 结果文本含死亡关键词。
	//    文本路径必须通过上面的非真实死亡语境守卫；不再扫描章节角色名共现。
	if (action == "combat" || action == "defeat") &&
		containsAnyOutlineValidatorText(text, outlineDeathTextKeywords...) {
		if outlineEventActorDied(evt, text) {
			add(evt.GetActor())
		} else {
			add(target)
		}
	}

	return victims
}

// outlineChangeImpliesDeath 判断结构化的 change 是否为死亡结果。
func outlineChangeImpliesDeath(change string) bool {
	switch strings.ToLower(strings.TrimSpace(change)) {
	case "temporary_death", "defeated", "died", "dead", "killed",
		"死亡", "战死", "身亡", "陨落", "被吞噬", "吞噬成功":
		return true
	}
	return false
}

// outlineChangeImpliesSelfDeath 判断该 change 是否表示执行者自身的死亡（actor==target 时适用）。
func outlineChangeImpliesSelfDeath(change string) bool {
	switch strings.ToLower(strings.TrimSpace(change)) {
	case "temporary_death", "died", "dead", "killed",
		"死亡", "战死", "身亡", "陨落", "被吞噬", "吞噬成功":
		return true
	}
	return false
}

// outlineEventActorDied 判断事件文本是否明确表明执行者死亡
func outlineEventActorDied(evt StoryEvent, text string) bool {
	actor := normalizeOutlineName(evt.GetActor())
	if actor == "" {
		return false
	}
	actorLower := normalizeOutlineValidatorText(actor)
	if strings.Contains(text, actorLower+"被") && containsAnyOutlineValidatorText(text, outlineDeathKeywords...) {
		return true
	}
	return containsAnyOutlineValidatorText(text, actorLower+"已死", actorLower+"身亡", actorLower+"陨落")
}

var outlineReviveKeywords = []string{"revive", "复活", "重生", "时间线重置", "回档"}

// outlineEventRevivedCharacters 返回复活事件中被复活的角色
// 判定依据：action 含 revive/复活/重生，或事件文本（change/details/result）含复活/重生标记
// 且文本中出现角色名（结构化字段优先，其次按文本共现归因）。
func outlineEventRevivedCharacters(evt StoryEvent, chapterChars []string) []string {
	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	text := normalizeOutlineValidatorText(strings.Join([]string{evt.Change, evt.Details, evt.Result}, " "))
	if !containsAnyOutlineValidatorText(action, outlineReviveKeywords...) &&
		!containsAnyOutlineValidatorText(text, "复活", "重生") {
		return nil
	}
	var revived []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !contains(revived, name) {
			revived = append(revived, name)
		}
	}
	add(evt.GetTarget())
	add(evt.GetActor())
	for _, name := range evt.Characters {
		add(name)
	}
	// 事件文本中同现角色名与复活/重生标记
	if containsAnyOutlineValidatorText(text, "复活", "重生") {
		for _, name := range chapterChars {
			if strings.Contains(text, normalizeOutlineValidatorText(name)) {
				add(name)
			}
		}
	}
	return revived
}

// cultivationSighting 角色修为记录
type cultivationSighting struct {
	raw     string
	tier    int
	layer   int
	phase   int
	parsed  bool
	chapter string
	index   int
}

// 修为大境界（由低到高）与小境界（由低到高）
var outlineCultivationTiers = []string{"练气", "筑基", "金丹", "元婴", "化神", "炼虚", "合体", "大乘", "渡劫"}
var outlineCultivationPhases = []string{"大圆满", "巅峰", "后期", "中期", "初期"}

var outlineBreakthroughKeywords = []string{"突破", "breakthrough", "进阶", "晋升", "升级", "upgrade", "觉醒", "awaken", "进化", "evolution"}
var outlineDowngradeKeywords = []string{"跌境", "跌落", "修为倒退", "修为被废", "反噬修为受损", "掉到", "跌至", "降至", "压制", "被废", "反噬损失修为"}

// validateCultivationContinuity 全角色修为单调性检查
// 来源：chapter.StateAnchor.Cultivation（仅主角）与 chapter.StateChange 自然语言（任意角色）。
// 修为下降且缺少跌境事件报 major；修为上升且缺少突破事件报 minor 警告。
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
