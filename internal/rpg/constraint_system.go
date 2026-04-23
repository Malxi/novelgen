package rpg

import (
	"fmt"
	"strings"
)

// ConstraintViolation 约束违反记录
type ConstraintViolation struct {
	Type       string // 违反类型：power, timeline, character, plot
	Target     string // 违反目标：角色名、时间线等
	Issue      string // 问题描述
	Severity   string // 严重程度：critical, error, warning, info
	Suggestion string // 修复建议
}

// ConstraintSystem RPG约束系统
// 用于将RPG规则和验证结果转换为AI可理解的约束条件
type ConstraintSystem struct {
	World          *GameWorld
	Checker        *OutlineRPGChecker
	CharacterRules map[string]*CharacterConstraint
	PlotRules      *PlotConstraint
	PowerRules     *PowerSystemConstraint
}

// CharacterConstraint 角色约束
type CharacterConstraint struct {
	Name            string
	MaxDeaths       int
	MaxResurrections int
	PowerChangeRate float64 // 每章最大战力变化率
	RequiredPresence map[string]float64 // 在特定章节必须出现的比例
	Relationships   []RelationshipConstraint
}

// RelationshipConstraint 关系约束
type RelationshipConstraint struct {
	WithCharacter string
	MinInteractions int
	MaxInteractions int
	RelationshipType string // ally, enemy, neutral
}

// PlotConstraint 剧情约束
type PlotConstraint struct {
	MaxTimeJumpsPerChapter int
	MinPacingRatio         float64 // 慢节奏最小比例
	MaxPacingRatio         float64 // 快节奏最大比例
	RequiredTransitions    []string // 必须有的转场
	ForbiddenElements      []string // 禁止的元素
}

// PowerSystemConstraint 战力系统约束
type PowerSystemConstraint struct {
	MaxPowerChangesPerArc    int
	CooldownBetweenChanges   int // 章节数
	ResurrectionCost         string // 复活的代价描述
	PowerLevelConsistency    bool   // 是否保持战力等级一致性
	AllowedPowerTransitions  map[string][]string // 允许的战力跃迁路径
}

// ConstraintReport 约束报告
type ConstraintReport struct {
	CharacterConstraints []*CharacterConstraint
	PlotConstraints      *PlotConstraint
	PowerConstraints     *PowerSystemConstraint
	ValidationIssues     []ValidationIssue
	Suggestions          []ConstraintSuggestion
}

// ConstraintSuggestion 约束建议
type ConstraintSuggestion struct {
	Type        string // hard, soft, info
	Category    string // character, plot, power, timeline
	Target      string // 目标对象
	Constraint  string // 约束描述
	Reason      string // 原因
	Priority    int    // 1-10
}

// NewConstraintSystem 创建约束系统
func NewConstraintSystem(world *GameWorld) *ConstraintSystem {
	cs := &ConstraintSystem{
		World:          world,
		Checker:        NewOutlineRPGChecker(),
		CharacterRules: make(map[string]*CharacterConstraint),
		PlotRules:      &PlotConstraint{},
		PowerRules:     &PowerSystemConstraint{},
	}
	// 初始化默认约束规则（避免空规则导致检测遗漏）
	cs.PlotRules = cs.buildPlotConstraints()
	cs.PowerRules = cs.buildPowerConstraints()
	return cs
}

// BuildFromRPGData 从RPG数据构建约束
func (cs *ConstraintSystem) BuildFromRPGData() *ConstraintReport {
	report := &ConstraintReport{
		CharacterConstraints: make([]*CharacterConstraint, 0),
		PlotConstraints:      cs.buildPlotConstraints(),
		PowerConstraints:     cs.buildPowerConstraints(),
		ValidationIssues:     make([]ValidationIssue, 0),
		Suggestions:          make([]ConstraintSuggestion, 0),
	}

	// 为每个角色构建约束
	for _, char := range cs.World.Characters.GetAllCharacters() {
		constraint := cs.buildCharacterConstraint(char)
		report.CharacterConstraints = append(report.CharacterConstraints, constraint)
		cs.CharacterRules[char.Name] = constraint
	}

	// 生成约束建议
	report.Suggestions = cs.generateConstraintSuggestions(report)

	return report
}

// buildCharacterConstraint 构建角色约束
func (cs *ConstraintSystem) buildCharacterConstraint(char *Character) *CharacterConstraint {
	constraint := &CharacterConstraint{
		Name:             char.Name,
		MaxDeaths:        3,  // 默认最多死亡3次
		MaxResurrections: 2,  // 默认最多复活2次
		PowerChangeRate:  0.2, // 每章最多20%战力变化
		RequiredPresence: make(map[string]float64),
		Relationships:    make([]RelationshipConstraint, 0),
	}

	// 根据角色类型调整约束
	switch char.Type {
	case CharacterTypePlayer:
		constraint.MaxDeaths = 5
		constraint.MaxResurrections = 3
		constraint.RequiredPresence["main_arc"] = 0.8 // 主线必须80%出场
	case CharacterTypeEnemy:
		constraint.MaxDeaths = 1 // 反派通常只死一次
		constraint.MaxResurrections = 0
	}

	// 根据等级调整战力变化率
	if char.Level > 50 {
		constraint.PowerChangeRate = 0.1 // 高等级变化更慢
	}

	return constraint
}

// buildPlotConstraints 构建剧情约束
func (cs *ConstraintSystem) buildPlotConstraints() *PlotConstraint {
	return &PlotConstraint{
		MaxTimeJumpsPerChapter: 2,
		MinPacingRatio:         0.3, // 至少30%慢节奏
		MaxPacingRatio:         0.6, // 最多60%快节奏
		RequiredTransitions:    []string{},
		ForbiddenElements: []string{
			"无理由的复活",
			"战力突然暴涨",
			"角色无理由消失",
		},
	}
}

// buildPowerConstraints 构建战力约束
func (cs *ConstraintSystem) buildPowerConstraints() *PowerSystemConstraint {
	return &PowerSystemConstraint{
		MaxPowerChangesPerArc:  3,
		CooldownBetweenChanges: 5, // 至少5章间隔
		ResurrectionCost:       "每次复活消耗寿命或修为",
		PowerLevelConsistency:  true,
		AllowedPowerTransitions: map[string][]string{
			"练气": {"练气", "筑基"},
			"筑基": {"筑基", "金丹"},
			"金丹": {"金丹", "元婴"},
			"元婴": {"元婴", "化神"},
			"化神": {"化神", "合体"},
			"合体": {"合体", "大乘"},
			"大乘": {"大乘", "渡劫"},
			"渡劫": {"渡劫"},
		},
	}
}

// generateConstraintSuggestions 生成约束建议
func (cs *ConstraintSystem) generateConstraintSuggestions(report *ConstraintReport) []ConstraintSuggestion {
	suggestions := make([]ConstraintSuggestion, 0)

	// 角色约束建议
	for _, char := range report.CharacterConstraints {
		if char.MaxResurrections > 0 {
			suggestions = append(suggestions, ConstraintSuggestion{
				Type:       "hard",
				Category:   "character",
				Target:     char.Name,
				Constraint: fmt.Sprintf("最多复活%d次", char.MaxResurrections),
				Reason:     "保持死亡的严肃性，避免读者失去紧张感",
				Priority:   9,
			})
		}

		suggestions = append(suggestions, ConstraintSuggestion{
			Type:       "soft",
			Category:   "character",
			Target:     char.Name,
			Constraint: fmt.Sprintf("战力变化率不超过%.0f%%每章", char.PowerChangeRate*100),
			Reason:     "避免战力崩坏，保持系统稳定性",
			Priority:   7,
		})
	}

	// 剧情约束建议
	suggestions = append(suggestions, ConstraintSuggestion{
		Type:       "hard",
		Category:   "plot",
		Target:     "全局",
		Constraint: fmt.Sprintf("每章最多%d次时间跳跃", report.PlotConstraints.MaxTimeJumpsPerChapter),
		Reason:     "保持时间线清晰，避免读者混乱",
		Priority:   8,
	})

	suggestions = append(suggestions, ConstraintSuggestion{
		Type:       "soft",
		Category:   "plot",
		Target:     "全局",
		Constraint: fmt.Sprintf("快节奏比例控制在%.0f%%-%.0f%%", report.PlotConstraints.MinPacingRatio*100, report.PlotConstraints.MaxPacingRatio*100),
		Reason:     "保持节奏平衡，避免阅读疲劳",
		Priority:   6,
	})

	// 战力约束建议
	suggestions = append(suggestions, ConstraintSuggestion{
		Type:       "hard",
		Category:   "power",
		Target:     "战力系统",
		Constraint: fmt.Sprintf("每次战力变化至少间隔%d章", report.PowerConstraints.CooldownBetweenChanges),
		Reason:     "给读者建立稳定预期的时间",
		Priority:   9,
	})

	suggestions = append(suggestions, ConstraintSuggestion{
		Type:       "hard",
		Category:   "power",
		Target:     "复活机制",
		Constraint: report.PowerConstraints.ResurrectionCost,
		Reason:     "复活必须有代价，否则失去意义",
		Priority:   10,
	})

	return suggestions
}

// ToPromptFormat 转换为提示词格式
func (cs *ConstraintSystem) ToPromptFormat(report *ConstraintReport) string {
	var sb strings.Builder

	sb.WriteString("=== RPG系统约束规则 ===\n\n")

	// 角色约束
	if len(report.CharacterConstraints) > 0 {
		sb.WriteString("【角色约束】\n")
		for _, char := range report.CharacterConstraints {
			sb.WriteString(fmt.Sprintf("角色: %s\n", char.Name))
			sb.WriteString(fmt.Sprintf("  - 最大死亡次数: %d\n", char.MaxDeaths))
			sb.WriteString(fmt.Sprintf("  - 最大复活次数: %d\n", char.MaxResurrections))
			sb.WriteString(fmt.Sprintf("  - 战力变化率限制: %.0f%%每章\n", char.PowerChangeRate*100))
			if len(char.RequiredPresence) > 0 {
				sb.WriteString("  - 出场要求:\n")
				for arc, ratio := range char.RequiredPresence {
					sb.WriteString(fmt.Sprintf("    * %s: %.0f%%\n", arc, ratio*100))
				}
			}
			sb.WriteString("\n")
		}
	}

	// 剧情约束
	sb.WriteString("【剧情约束】\n")
	sb.WriteString(fmt.Sprintf("  - 每章最多%d次时间跳跃\n", report.PlotConstraints.MaxTimeJumpsPerChapter))
	sb.WriteString(fmt.Sprintf("  - 快节奏比例: %.0f%%-%.0f%%\n", 
		report.PlotConstraints.MinPacingRatio*100, 
		report.PlotConstraints.MaxPacingRatio*100))
	if len(report.PlotConstraints.ForbiddenElements) > 0 {
		sb.WriteString("  - 禁止元素:\n")
		for _, elem := range report.PlotConstraints.ForbiddenElements {
			sb.WriteString(fmt.Sprintf("    * %s\n", elem))
		}
	}
	sb.WriteString("\n")

	// 战力约束
	sb.WriteString("【战力系统约束】\n")
	sb.WriteString(fmt.Sprintf("  - 战力变化冷却: %d章\n", report.PowerConstraints.CooldownBetweenChanges))
	sb.WriteString(fmt.Sprintf("  - 复活代价: %s\n", report.PowerConstraints.ResurrectionCost))
	if report.PowerConstraints.PowerLevelConsistency {
		sb.WriteString("  - 必须保持战力等级一致性\n")
	}
	sb.WriteString("\n")

	// 约束建议
	if len(report.Suggestions) > 0 {
		sb.WriteString("【写作约束建议】\n")
		for _, sug := range report.Suggestions {
			typeStr := "[建议]"
			if sug.Type == "hard" {
				typeStr = "[硬性约束]"
			}
			sb.WriteString(fmt.Sprintf("%s %s (优先级:%d)\n", typeStr, sug.Constraint, sug.Priority))
			sb.WriteString(fmt.Sprintf("  原因: %s\n\n", sug.Reason))
		}
	}

	sb.WriteString("=== 约束规则结束 ===\n")

	return sb.String()
}

// ToSystemPrompt 转换为系统提示词
func (cs *ConstraintSystem) ToSystemPrompt(report *ConstraintReport) string {
	var sb strings.Builder

	sb.WriteString("RPG系统约束规则（必须遵守）:\n\n")

	// 硬性约束
	hardConstraints := make([]ConstraintSuggestion, 0)
	for _, sug := range report.Suggestions {
		if sug.Type == "hard" {
			hardConstraints = append(hardConstraints, sug)
		}
	}

	if len(hardConstraints) > 0 {
		sb.WriteString("硬性约束（违反会导致剧情崩坏）:\n")
		for i, sug := range hardConstraints {
			sb.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, sug.Target, sug.Constraint))
		}
		sb.WriteString("\n")
	}

	// 软性约束
	softConstraints := make([]ConstraintSuggestion, 0)
	for _, sug := range report.Suggestions {
		if sug.Type == "soft" {
			softConstraints = append(softConstraints, sug)
		}
	}

	if len(softConstraints) > 0 {
		sb.WriteString("软性约束（建议遵守，增强剧情质量）:\n")
		for i, sug := range softConstraints {
			sb.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, sug.Target, sug.Constraint))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("约束执行原则:\n")
	sb.WriteString("- 硬性约束必须严格遵守，不得违反\n")
	sb.WriteString("- 软性约束尽量遵守，特殊情况可灵活处理\n")
	sb.WriteString("- 所有约束的目的是保持剧情合理性和读者体验\n")

	return sb.String()
}

// ValidateChapter 验证章节是否符合约束
func (cs *ConstraintSystem) ValidateChapter(chapterID string, chapterText string) []ConstraintViolation {
	violations := make([]ConstraintViolation, 0)

	// 1. 检查战力变化 — 升级为多模式匹配 + 计数
	powerChanges := cs.detectPowerChanges(chapterText)
	if len(powerChanges) > cs.PowerRules.MaxPowerChangesPerArc {
		// 尝试确定涉及的角色
		target := "战力系统"
		for _, pc := range powerChanges {
			if pc.Character != "" {
				target = pc.Character
				break
			}
		}
		violations = append(violations, ConstraintViolation{
			Type:       "power",
			Target:     target,
			Issue:      fmt.Sprintf("一章内战力变化过于频繁: %d次", len(powerChanges)),
			Severity:   "critical",
			Suggestion: "减少突破次数，增加修炼过程的描写和困难",
		})
	}

	// 2. 检查复活 — 升级为角色感知 + 代价检测
	resurrections := cs.detectResurrectionsEnhanced(chapterText)
	for charName, info := range resurrections {
		maxAllowed := 0 // 默认不允许
		if constraint, exists := cs.CharacterRules[charName]; exists {
			maxAllowed = constraint.MaxResurrections
		}
		if info.Count > maxAllowed {
			violations = append(violations, ConstraintViolation{
				Type:       "resurrection",
				Target:     charName,
				Issue:      fmt.Sprintf("复活次数超限: %d次 (最大允许%d次)", info.Count, maxAllowed),
				Severity:   "critical",
				Suggestion: fmt.Sprintf("限制%s的复活次数，或增加复活代价", charName),
			})
		}
		// 检查复活代价
		if info.Count > 0 && !info.HasCost {
			violations = append(violations, ConstraintViolation{
				Type:       "resurrection",
				Target:     charName,
				Issue:      fmt.Sprintf("%s复活缺乏代价", charName),
				Severity:   "critical",
				Suggestion: "每次复活应消耗修为、寿命或其他代价",
			})
		}
	}

	// 3. 检查时间跳跃 — 升级为多模式 + 隐含表达
	timeJumps := cs.detectTimeJumpsEnhanced(chapterText)
	if len(timeJumps) > cs.PlotRules.MaxTimeJumpsPerChapter {
		violations = append(violations, ConstraintViolation{
			Type:       "timeline",
			Target:     "时间线",
			Issue:      fmt.Sprintf("时间跳跃过多: %d次 (最大允许%d次)", len(timeJumps), cs.PlotRules.MaxTimeJumpsPerChapter),
			Severity:   "critical",
			Suggestion: "减少时间跳跃，或增加跳跃之间的内容描写",
		})
	}

	// 4. 检查禁止元素 — 升级为多模式匹配
	for _, element := range cs.PlotRules.ForbiddenElements {
		if matches, _ := cs.matchForbiddenElement(chapterText, element); matches {
			violations = append(violations, ConstraintViolation{
				Type:       "plot",
				Target:     "剧情",
				Issue:      fmt.Sprintf("出现禁止元素: %s", element),
				Severity:   "critical",
				Suggestion: fmt.Sprintf("移除或修改%s相关内容，确保剧情合理性", element),
			})
		}
	}

	// 5. 检查角色位置矛盾
	locationConflicts := cs.detectLocationConflicts(chapterText)
	for _, conflict := range locationConflicts {
		violations = append(violations, ConstraintViolation{
			Type:       "character",
			Target:     conflict.Character,
			Issue:      fmt.Sprintf("%s同时出现在多个地点: %s", conflict.Character, conflict.Locations),
			Severity:   "critical",
			Suggestion: "确保角色在同一时间只出现在一个地点，或解释传送/分身等机制",
		})
	}

	// 6. 检查战力不一致（突然倒退/恢复）
	inconsistencies := cs.detectPowerInconsistencies(chapterText)
	for _, inc := range inconsistencies {
		violations = append(violations, ConstraintViolation{
			Type:       "power",
			Target:     inc.Character,
			Issue:      inc.Description,
			Severity:   "critical",
			Suggestion: "解释战力变化的合理原因，或增加过渡描写",
		})
	}

		// 7. 检查跨级秒杀
		crossLevelKills := cs.detectCrossLevelCombat(chapterText)
		for _, clk := range crossLevelKills {
			violations = append(violations, ConstraintViolation{
				Type:       "power",
				Target:     clk.Attacker,
				Issue:      clk.Description,
				Severity:   "critical",
				Suggestion: fmt.Sprintf("为%s增加战斗困难或失败经历，避免跨级秒杀", clk.Attacker),
			})
		}

		// 8. 检查突破缺乏困难
		noStruggles := cs.detectNoStruggleBreakthroughs(chapterText, powerChanges)
		for _, ns := range noStruggles {
			violations = append(violations, ConstraintViolation{
				Type:       "power",
				Target:     ns.Character,
				Issue:      ns.Description,
				Severity:   "error",
				Suggestion: "增加突破前的困难和挫折描写",
			})
		}

		// 9. 检查暗示性战力暴涨
		hiddenSurges := cs.detectHiddenPowerSurges(chapterText)
		for _, hs := range hiddenSurges {
			violations = append(violations, ConstraintViolation{
				Type:       "power",
				Target:     hs.Character,
				Issue:      hs.Description,
				Severity:   "error",
				Suggestion: "明确战力变化的原因和过程，避免暗示性暴涨",
			})
		}

		// 10. 检查角色关系突变
		relShifts := cs.detectRelationshipShifts(chapterText)
		for _, rs := range relShifts {
			violations = append(violations, ConstraintViolation{
				Type:       "character",
				Target:     rs.Character,
				Issue:      rs.Description,
				Severity:   "error",
				Suggestion: fmt.Sprintf("为%s的关系转变增加铺垫和过渡", rs.Character),
			})
		}

		// 11. 检查时间线矛盾
		paradoxes := cs.detectTimelineParadoxes(chapterText)
		for _, p := range paradoxes {
			violations = append(violations, ConstraintViolation{
				Type:       "timeline",
				Target:     "时间线",
				Issue:      p.Description,
				Severity:   "critical",
				Suggestion: "修复时间线矛盾，确保前后时间描述一致",
			})
		}

		// 12. 检查工具人角色
		toolChars := cs.detectToolCharacters(chapterText)
		for _, tc := range toolChars {
			violations = append(violations, ConstraintViolation{
				Type:       "character",
				Target:     tc.Name,
				Issue:      tc.Description,
				Severity:   "warning",
				Suggestion: fmt.Sprintf("丰富%s的角色，使其有更多出场和发展", tc.Name),
			})
		}

		return violations
	}

// PowerChange 战力变化记录
type PowerChange struct {
	Character string // 涉及角色
	Type      string // 突破/跌落/恢复
	FromLevel string // 变化前
	ToLevel   string // 变化后
	Evidence  string // 匹配到的原文
}

// detectPowerChanges 增强版战力变化检测
// 从只匹配关键词升级为：匹配多种表达 + 提取角色 + 计数
func (cs *ConstraintSystem) detectPowerChanges(text string) []PowerChange {
	changes := make([]PowerChange, 0)

	// 战力变化的多模式关键词
	// 格式: {模式, 变化类型}
	patterns := []struct {
		Keywords []string
		Type     string
	}{
		// 突破类
		{[]string{"突破", "踏入", "跨入", "进入", "成就", "证得"}, "breakthrough"},
		// 晋升类
		{[]string{"晋升", "升入", "提升到", "进阶", "修为精进"}, "promotion"},
		// 跌落类
		{[]string{"跌落", "降至", "退化", "修为倒退", "境界不稳"}, "regression"},
	}

	// 境界关键词（修仙小说常见）
	levels := []string{"练气", "筑基", "金丹", "元婴", "化神", "合体", "大乘", "渡劫",
		"炼气", "结丹", "元神", "分神", "洞虚", "渡劫"}

	// 提取文本中的角色名（简化：2-4字中文名 + 动词模式）
	knownChars := cs.extractKnownCharacters(text)

	for _, group := range patterns {
		for _, keyword := range group.Keywords {
			// 找到所有出现位置
			idx := 0
			for {
				pos := strings.Index(text[idx:], keyword)
				if pos == -1 {
					break
				}
				absPos := idx + pos

				// 提取上下文（前后30字）
				contextStart := absPos - 30
				if contextStart < 0 {
					contextStart = 0
				}
				contextEnd := absPos + len(keyword) + 30
				if contextEnd > len(text) {
					contextEnd = len(text)
				}
				context := text[contextStart:contextEnd]

				// 确定涉及角色
				char := cs.findNearestCharacter(context, knownChars)

				// 尝试提取境界信息
				fromLevel, toLevel := "", ""
				for _, lv := range levels {
					if strings.Contains(context, lv) {
						if toLevel == "" {
							toLevel = lv
						} else if fromLevel == "" {
							fromLevel = lv
						}
					}
				}

				evidence := keyword
				if len(context) < 50 {
					evidence = context
				}

				changes = append(changes, PowerChange{
					Character: char,
					Type:      group.Type,
					FromLevel: fromLevel,
					ToLevel:   toLevel,
					Evidence:  evidence,
				})

				idx = absPos + len(keyword)
			}
		}
	}

	return changes
}

// ResurrectionInfo 复活信息
type ResurrectionInfo struct {
	Count   int  // 复活次数
	HasCost bool // 是否有代价
}

// detectResurrectionsEnhanced 增强版复活检测
// 从全归主角升级为：角色感知 + 代价检测
func (cs *ConstraintSystem) detectResurrectionsEnhanced(text string) map[string]*ResurrectionInfo {
	result := make(map[string]*ResurrectionInfo)

	// 复活关键词
	resurrectionKeywords := []string{"复活", "苏醒", "重生", "死而复生", "起死回生",
		"复活了", "重新活了", "又活了过来", "从死亡中回来"}

	// 死亡关键词
	deathKeywords := []string{"死亡", "被杀", "身亡", "倒下", "陨落", "气绝",
		"一剑穿心", "殒命", "断了气息", "倒在了血泊"}

	// 代价关键词
	costKeywords := []string{"代价", "消耗", "修为", "寿命", "寿元", "根基",
		"折损", "倒退", "损失", "献祭", "献出"}

	// 提取已知角色
	knownChars := cs.extractKnownCharacters(text)

	// 检测死亡
	deathCount := 0
	deathChars := make(map[string]int)
	for _, kw := range deathKeywords {
		idx := 0
		for {
			pos := strings.Index(text[idx:], kw)
			if pos == -1 {
				break
			}
			absPos := idx + pos
			contextStart := absPos - 30
			if contextStart < 0 {
				contextStart = 0
			}
			contextEnd := absPos + len(kw) + 30
			if contextEnd > len(text) {
				contextEnd = len(text)
			}
			context := text[contextStart:contextEnd]
			char := cs.findNearestCharacter(context, knownChars)
			if char != "" {
				deathChars[char]++
			}
			deathCount++
			idx = absPos + len(kw)
		}
	}

	// 检测复活
	resurrectCount := 0
	for _, kw := range resurrectionKeywords {
		idx := 0
		for {
			pos := strings.Index(text[idx:], kw)
			if pos == -1 {
				break
			}
			absPos := idx + pos

			contextStart := absPos - 40
			if contextStart < 0 {
				contextStart = 0
			}
			contextEnd := absPos + len(kw) + 40
			if contextEnd > len(text) {
				contextEnd = len(text)
			}
			context := text[contextStart:contextEnd]

			char := cs.findNearestCharacter(context, knownChars)
			if char == "" {
				char = "未知角色"
			}

			if _, exists := result[char]; !exists {
				result[char] = &ResurrectionInfo{Count: 0, HasCost: false}
			}
			result[char].Count++
			resurrectCount++

			// 检查复活附近是否有代价描述
			if !result[char].HasCost {
				for _, cost := range costKeywords {
					if strings.Contains(context, cost) {
						result[char].HasCost = true
						break
					}
				}
			}

			idx = absPos + len(kw)
		}
	}

	// 如果有死亡但没有找到复活关键词，但文本中有"又"或"重新"暗示恢复
	if deathCount > 0 && resurrectCount == 0 {
		// 检查死亡后是否有暗示恢复的描述
		recoveryHints := []string{"又站了起来", "重新站起来", "仿佛什么都没发生", "毫发无伤"}
		for _, hint := range recoveryHints {
			if strings.Contains(text, hint) {
				for char := range deathChars {
					if _, exists := result[char]; !exists {
						result[char] = &ResurrectionInfo{Count: deathChars[char], HasCost: false}
					}
				}
				break
			}
		}
	}

	// 对没有角色信息的死亡/复活，归给已知角色中最可能的
	if len(result) == 0 && deathCount > 0 && resurrectCount > 0 {
		for _, char := range knownChars {
			result[char] = &ResurrectionInfo{Count: resurrectCount, HasCost: false}
			break // 只归给第一个找到的角色
		}
	}

	return result
}

// TimeJump 时间跳跃记录
type TimeJump struct {
	Expression string // 匹配到的时间表达
	IsImplicit bool   // 是否为隐含表达
}

// detectTimeJumpsEnhanced 增强版时间跳跃检测
// 从5个硬编码词升级为：标准表达 + 隐含表达
func (cs *ConstraintSystem) detectTimeJumpsEnhanced(text string) []TimeJump {
	jumps := make([]TimeJump, 0)

	// 标准时间跳跃模式（精确匹配）
	standardPatterns := []string{
		// 中文数字 + 时间单位
		"一天后", "两天后", "三天后", "数天后", "几天后",
		"一周后", "数周后",
		"一个月后", "两个月后", "数月后", "几个月后",
		"半年后", "一年后", "两年后", "数年后", "几年后",
		"十年后", "百年后",
		// 变体
		"过了三天", "过了七天", "过了一周",
		"过了一月", "过了一个月", "过了半年", "过了一年", "过了数年",
		"三天之后", "一个月之后", "半年之后", "一年之后",
	}

	for _, pattern := range standardPatterns {
		if strings.Contains(text, pattern) {
			jumps = append(jumps, TimeJump{
				Expression: pattern,
				IsImplicit: false,
			})
		}
	}

	// 隐含时间跳跃模式（修辞/文学性表达）
	implicitPatterns := []string{
		"春去秋来", "寒来暑往", "四季更迭", "岁月流转",
		"斗转星移", "日升月落", "光阴似箭", "时光飞逝",
		"不知不觉", "不知多少", "不知过了多久",
		"来年", "隔年", "次年", "翌年",
		"半生", "余生", "大半辈子",
		"转眼间", "一晃", "一转眼",
	}

	for _, pattern := range implicitPatterns {
		if strings.Contains(text, pattern) {
			jumps = append(jumps, TimeJump{
				Expression: pattern,
				IsImplicit: true,
			})
		}
	}

	return jumps
}

// LocationConflict 位置矛盾
type LocationConflict struct {
	Character string
	Locations string
}

// detectLocationConflicts 检测角色位置矛盾
func (cs *ConstraintSystem) detectLocationConflicts(text string) []LocationConflict {
	conflicts := make([]LocationConflict, 0)

	// 按行分析，检测同一角色在不同地点
	lines := strings.Split(text, "\n")
	charLocations := make(map[string][]string) // 角色名 -> 出现的地点

	// 地点指示词
	locationPatterns := []string{"在", "来到", "前往", "到达", "位于", "身处", "回到", "留在"}

	// 地点名词后缀
	locationSuffixes := []string{"矿场", "宗门", "城池", "山脉", "森林", "洞府",
		"秘境", "大殿", "广场", "山洞", "战场", "营寨", "宫中"}

	knownChars := cs.extractKnownCharacters(text)

	for _, line := range lines {
		lineChars := make(map[string]bool)
		lineLocs := make([]string, 0)

		// 找出本行出现的角色
		for _, char := range knownChars {
			if strings.Contains(line, char) {
				lineChars[char] = true
			}
		}

		// 找出本行的地点
		for _, pattern := range locationPatterns {
			idx := 0
			for {
				pos := strings.Index(line[idx:], pattern)
				if pos == -1 {
					break
				}
				absPos := idx + pos
				// 取关键词后的文本
				after := line[absPos+len(pattern):]
				if len(after) > 20 {
					after = after[:20]
				}
				for _, suffix := range locationSuffixes {
					if strings.Contains(after, suffix) {
						lineLocs = append(lineLocs, suffix)
						break
					}
				}
				idx = absPos + len(pattern)
			}
		}

		// 也直接检查地点后缀
		for _, suffix := range locationSuffixes {
			if strings.Contains(line, suffix) {
				found := false
				for _, existing := range lineLocs {
					if existing == suffix {
						found = true
						break
					}
				}
				if !found {
					lineLocs = append(lineLocs, suffix)
				}
			}
		}

		// 记录角色-地点映射
		for char := range lineChars {
			for _, loc := range lineLocs {
				charLocations[char] = append(charLocations[char], loc)
			}
		}
	}

	// 检测矛盾：同一角色在同一时间段出现在明确不同的地点
	// "与此同时" 是关键矛盾信号
	if strings.Contains(text, "与此同时") || strings.Contains(text, "与此同时") {
		for char, locs := range charLocations {
			uniqueLocs := make(map[string]bool)
			for _, loc := range locs {
				uniqueLocs[loc] = true
			}
			if len(uniqueLocs) > 1 {
				locList := make([]string, 0, len(uniqueLocs))
				for loc := range uniqueLocs {
					locList = append(locList, loc)
				}
				conflicts = append(conflicts, LocationConflict{
					Character: char,
					Locations: strings.Join(locList, " 和 "),
				})
			}
		}
	}

	return conflicts
}

// PowerInconsistency 战力不一致记录
type PowerInconsistency struct {
	Character   string
	Description string
}

// detectPowerInconsistencies 检测战力不一致（突然倒退又恢复）
func (cs *ConstraintSystem) detectPowerInconsistencies(text string) []PowerInconsistency {
	inconsistencies := make([]PowerInconsistency, 0)

	knownChars := cs.extractKnownCharacters(text)

	// 检测"跌落"+"恢复"模式
	regressionKeywords := []string{"跌落", "降至", "退化", "倒退", "突然下降"}
	recoveryKeywords := []string{"恢复", "又恢复", "重回", "重临", "仿佛刚才", "毫无影响"}

	for _, char := range knownChars {
		hasRegression := false
		hasRecovery := false
		regressionContext := ""

		for _, rk := range regressionKeywords {
			if strings.Contains(text, rk) {
				hasRegression = true
				regressionContext = rk
				break
			}
		}

		if !hasRegression {
			continue
		}

		for _, rvk := range recoveryKeywords {
			if strings.Contains(text, rvk) {
				hasRecovery = true
				break
			}
		}

		if hasRegression && hasRecovery {
			inconsistencies = append(inconsistencies, PowerInconsistency{
				Character:   char,
				Description: fmt.Sprintf("%s战力%s后又恢复，缺乏合理解释", char, regressionContext),
			})
		}
	}

	return inconsistencies
}

// matchForbiddenElement 匹配禁止元素（增强版）
func (cs *ConstraintSystem) matchForbiddenElement(text string, element string) (bool, string) {
	// 直接匹配
	if strings.Contains(text, element) {
		return true, element
	}

	// 同义词/近义词映射
	synonyms := map[string][]string{
		"无理由的复活": {"复活", "没有理由地复活", "毫无解释地复活", "仿佛什么都没发生"},
		"战力突然暴涨": {"战力暴涨", "突然变强", "实力暴增", "战力激增", "力量突然"},
		"角色无理由消失": {"消失了", "再也没出现", "不知去向"},
	}

	if syns, ok := synonyms[element]; ok {
		for _, syn := range syns {
			if strings.Contains(text, syn) {
				return true, syn
			}
		}
	}

	return false, ""
}

// CrossLevelKill 跨级击杀记录
type CrossLevelKill struct {
	Attacker    string
	Defender    string
	AttackerLvl string
	DefenderLvl string
	Description string
}

// detectCrossLevelCombat 检测跨级秒杀（低战力击败高战力无合理解释）
func (cs *ConstraintSystem) detectCrossLevelCombat(text string) []CrossLevelKill {
	kills := make([]CrossLevelKill, 0)

	// 境界等级映射
	levelOrder := []string{"练气", "炼气", "筑基", "金丹", "结丹", "元婴", "化神", "合体", "大乘", "渡劫"}
	levelRank := make(map[string]int)
	for i, lv := range levelOrder {
		levelRank[lv] = i
	}

	// 秒杀关键词
	instantKillKws := []string{"随手一击", "随手一挥", "灰飞烟灭", "一击毙命", "瞬间击杀",
		"秒杀", "轻易击败", "轻松击杀", "毫不费力", "一招制敌", "举手之劳"}

	// 战败关键词
	defeatKws := []string{"击败", "击杀", "战胜", "打败", "杀死了", "击毙", "斩杀"}

	knownChars := cs.extractKnownCharacters(text)

	// 查找文本中出现的所有境界
	charLevels := make(map[string]string) // 角色名 -> 境界
	for _, char := range knownChars {
		for _, lv := range levelOrder {
			if strings.Contains(text, char+lv) || strings.Contains(text, char+"的"+lv) {
				charLevels[char] = lv
				break
			}
		}
	}

	// 也从上下文提取：只有X境界 的模式
	for _, lv := range levelOrder {
		patterns := []string{"只有" + lv, lv + "的修为", lv + "期修士", lv + "期强者", lv + "修士"}
		for _, p := range patterns {
			idx := strings.Index(text, p)
			if idx != -1 {
				// 找附近的角色
				ctxStart := idx - 30
				if ctxStart < 0 {
					ctxStart = 0
				}
				ctxEnd := idx + len(p) + 10
				if ctxEnd > len(text) {
					ctxEnd = len(text)
				}
				ctx := text[ctxStart:ctxEnd]
				for _, char := range knownChars {
					if strings.Contains(ctx, char) {
						if _, exists := charLevels[char]; !exists {
							charLevels[char] = lv
						}
					}
				}
			}
		}
	}

	// 检测秒杀模式
	for _, ik := range instantKillKws {
		if !strings.Contains(text, ik) {
			continue
		}
		// 找到秒杀附近的角色和境界
		for _, char := range knownChars {
			attackerLvl, aOk := charLevels[char]
			if !aOk {
				continue
			}
			// 找被击败方的境界
			for _, lv := range levelOrder {
				if lv == attackerLvl {
					continue
				}
				defRank := levelRank[lv]
				attRank := levelRank[attackerLvl]
				if defRank > attRank+1 { // 跨超过1个大级
					for _, dk := range defeatKws {
						if strings.Contains(text, dk) {
							kills = append(kills, CrossLevelKill{
								Attacker:    char,
								Defender:    "对手",
								AttackerLvl: attackerLvl,
								DefenderLvl: lv,
								Description: fmt.Sprintf("%s(%s期)秒杀%s期强者，跨级过大", char, attackerLvl, lv),
							})
							break
						}
					}
					if len(kills) > 0 {
						break
					}
				}
			}
			if len(kills) > 0 {
				break
			}
		}
	}

	// 检测"只有X层...击败Y期"模式
	for _, char := range knownChars {
		lvl, ok := charLevels[char]
		if !ok {
			continue
		}
		rank := levelRank[lvl]
		for _, lv := range levelOrder {
			if levelRank[lv] > rank+1 {
				// 检查是否击败了更高境界
				for _, dk := range defeatKws {
					if strings.Contains(text, dk) && strings.Contains(text, lv) {
						kills = append(kills, CrossLevelKill{
							Attacker:    char,
							Defender:    "对手",
							AttackerLvl: lvl,
							DefenderLvl: lv,
							Description: fmt.Sprintf("%s(%s)击败%s期对手，跨级过大", char, lvl, lv),
						})
						break
					}
				}
			}
		}
	}

	return kills
}

// NoStruggleBreakthrough 突破缺乏困难记录
type NoStruggleBreakthrough struct {
	Character   string
	Description string
}

// detectNoStruggleBreakthroughs 检测突破缺乏困难和挣扎
func (cs *ConstraintSystem) detectNoStruggleBreakthroughs(text string, powerChanges []PowerChange) []NoStruggleBreakthrough {
	results := make([]NoStruggleBreakthrough, 0)

	// 挣扎/困难关键词
	struggleKws := []string{"困难", "艰难", "壁障", "卡住", "受阻", "瓶颈", "痛苦",
		"挣扎", "险些失败", "差点走火入魔", "九死一生", "苦修", "苦战"}

	// 轻松突破关键词
	easyKws := []string{"轻而易举", "轻松", "毫无阻碍", "轻描淡写", "轻易", "顺利",
		"一蹴而就", "水到渠成", "顺理成章", "毫不费力"}

	knownChars := cs.extractKnownCharacters(text)

	for _, char := range knownChars {
		charChanges := 0
		hasStruggle := false
		isEasy := false

		for _, pc := range powerChanges {
			if pc.Character == char && (pc.Type == "breakthrough" || pc.Type == "promotion") {
				charChanges++
			}
		}

		if charChanges == 0 {
			continue
		}

		// 检查文本中是否有困难描写
		for _, sk := range struggleKws {
			if strings.Contains(text, sk) {
				hasStruggle = true
				break
			}
		}

		// 检查是否被描述为轻松
		for _, ek := range easyKws {
			if strings.Contains(text, ek) {
				isEasy = true
				break
			}
		}

		// 多次突破但缺乏困难
		if charChanges >= 2 && !hasStruggle {
			results = append(results, NoStruggleBreakthrough{
				Character:   char,
				Description: fmt.Sprintf("%s一章内突破%d次，缺乏足够的铺垫和困难", char, charChanges),
			})
		} else if isEasy && !hasStruggle {
			results = append(results, NoStruggleBreakthrough{
				Character:   char,
				Description: fmt.Sprintf("%s的突破被描述为轻松，缺乏困难描写", char),
			})
		}
	}

	return results
}

// HiddenPowerSurge 暗示性战力暴涨记录
type HiddenPowerSurge struct {
	Character   string
	Description string
}

// detectHiddenPowerSurges 检测暗示性战力暴涨（不通过"突破"等关键词）
func (cs *ConstraintSystem) detectHiddenPowerSurges(text string) []HiddenPowerSurge {
	surges := make([]HiddenPowerSurge, 0)

	knownChars := cs.extractKnownCharacters(text)

	// 暗示性暴涨模式
	implicitSurgeKws := []string{"深不可测", "气息暴涨", "威压", "令人心悸", "颤抖着说不出话",
		"面面相觑", "不敢相信", "骇然", "震惊", "倒吸一口凉气"}

	// 对比模式：之前弱 + 现在强
	beforeWeakKws := []string{"昨日还是", "之前只是", "原本只是", "还是个", "不过是"}
	afterStrongKws := []string{"今日", "现在", "此刻", "已经", "已是"}

	for _, char := range knownChars {
		hasBeforeWeak := false
		hasAfterStrong := false
		hasSurgeHint := false
		weakContext := ""

		for _, bk := range beforeWeakKws {
			if strings.Contains(text, bk) {
				hasBeforeWeak = true
				weakContext = bk
				break
			}
		}

		for _, ak := range afterStrongKws {
			if strings.Contains(text, ak) {
				hasAfterStrong = true
				break
			}
		}

		for _, ik := range implicitSurgeKws {
			if strings.Contains(text, ik) {
				hasSurgeHint = true
				break
			}
		}

		// "之前弱" + "暗示性暴涨" = 隐含战力暴涨
		if hasBeforeWeak && hasSurgeHint {
			surges = append(surges, HiddenPowerSurge{
				Character:   char,
				Description: fmt.Sprintf("%s战力通过暗示突然暴涨（%s后气息深不可测）", char, weakContext),
			})
		}

		// "之前弱" + "现在强" = 对比性暴涨
		if hasBeforeWeak && hasAfterStrong && !hasSurgeHint {
			surges = append(surges, HiddenPowerSurge{
				Character:   char,
				Description: fmt.Sprintf("%s战力短时间内大幅提升，缺乏过程描写", char),
			})
		}
	}

	return surges
}

// RelationshipShift 角色关系突变记录
type RelationshipShift struct {
	Character   string
	FromRel     string
	ToRel       string
	Description string
}

// detectRelationshipShifts 检测角色关系突变
func (cs *ConstraintSystem) detectRelationshipShifts(text string) []RelationshipShift {
	shifts := make([]RelationshipShift, 0)

	knownChars := cs.extractKnownCharacters(text)

	// 敌对关键词
	hostileKws := []string{"死对头", "敌人", "仇人", "追杀", "刺杀", "陷害", "设计",
		"敌对", "死敌", "追杀过", "派人来刺杀"}

	// 友好/服从关键词
	friendlyKws := []string{"效忠", "忠诚", "盟友", "朋友", "追随", "归顺", "臣服",
		"投靠", "投奔", "愿意效忠", "欣然接受"}

	for _, char := range knownChars {
		isHostile := false
		isFriendly := false
		hostileKw := ""
		friendlyKw := ""

		for _, hk := range hostileKws {
			if strings.Contains(text, hk) {
				isHostile = true
				hostileKw = hk
				break
			}
		}

		for _, fk := range friendlyKws {
			if strings.Contains(text, fk) {
				isFriendly = true
				friendlyKw = fk
				break
			}
		}

		// 同一角色既被描述为敌对又被描述为友好
		if isHostile && isFriendly {
			shifts = append(shifts, RelationshipShift{
				Character:   char,
				FromRel:     hostileKw,
				ToRel:       friendlyKw,
				Description: fmt.Sprintf("%s从%s突然变为%s，缺乏转变铺垫", char, hostileKw, friendlyKw),
			})
		}
	}

	return shifts
}

// TimelineParadox 时间线矛盾记录
type TimelineParadox struct {
	Description string
}

// detectTimelineParadoxes 检测时间线矛盾
func (cs *ConstraintSystem) detectTimelineParadoxes(text string) []TimelineParadox {
	paradoxes := make([]TimelineParadox, 0)

	// 检测"第X天" + "Y天前" 矛盾
	type timeRef struct {
		value int
		unit  string // day, month, year
		raw   string
	}

	var refs []timeRef

	// 提取时间引用
	dayPatterns := []struct {
		pattern string
		value   int
	}{
		{"第二天", 2}, {"第三天", 3}, {"第二天", 2},
	}
	for _, dp := range dayPatterns {
		if strings.Contains(text, dp.pattern) {
			refs = append(refs, timeRef{value: dp.value, unit: "day", raw: dp.pattern})
		}
	}

	// 提取"X天前"模式
	beforePatterns := []string{"三天前", "几天前", "一个月前", "三个月前", "半年前"}
	for _, bp := range beforePatterns {
		if strings.Contains(text, bp) {
			refs = append(refs, timeRef{value: -1, unit: "before", raw: bp})
		}
	}

	// 检测"第X天"但回忆"Y个月前" — 如果Y个月 > X天，可能矛盾
	hasDayRef := false
	dayValue := 0
	for _, r := range refs {
		if r.unit == "day" {
			hasDayRef = true
			if r.value > dayValue {
				dayValue = r.value
			}
		}
	}

	if hasDayRef && dayValue <= 7 {
		// 如果故事发生在前几天内，但回忆了更长时间前的事
		longBeforeKws := []string{"三个月前", "半年前", "一年前", "几年前", "数年前"}
		for _, lb := range longBeforeKws {
			if strings.Contains(text, lb) {
				// 检查上下文是否暗示矛盾
				// "来到...第X天" + "回想起Y个月前" 但 "X天前才出发"
				departKws := []string{"才出发", "才离开", "才动身", "刚出发", "刚离开"}
				for _, dk := range departKws {
					if strings.Contains(text, dk) {
						paradoxes = append(paradoxes, TimelineParadox{
							Description: fmt.Sprintf("时间线矛盾：故事才第%d天但回忆了%s，且%s", dayValue, lb, dk),
						})
						break
					}
				}
				if len(paradoxes) > 0 {
					break
				}
			}
		}
	}

	// 检测"上一章"引用的时间矛盾
	if strings.Contains(text, "上一章") || strings.Contains(text, "前一章") {
		// 搜索上下文中的时间矛盾标记
		contradictionKws := []string{"哪来的", "怎么会", "不可能", "矛盾"}
		for _, ck := range contradictionKws {
			if strings.Contains(text, ck) {
				paradoxes = append(paradoxes, TimelineParadox{
					Description: "跨章节时间线不一致：当前时间与上一章时间描述矛盾",
				})
				break
			}
		}
	}

	return paradoxes
}

// ToolCharacter 工具人角色记录
type ToolCharacter struct {
	Name        string
	Description string
}

// detectToolCharacters 检测工具人角色（只出场一次就消失）
func (cs *ConstraintSystem) detectToolCharacters(text string) []ToolCharacter {
	results := make([]ToolCharacter, 0)

	knownChars := cs.extractKnownCharacters(text)

	for _, char := range knownChars {
		// 统计角色名出现次数
		count := 0
		idx := 0
		for {
			pos := strings.Index(text[idx:], char)
			if pos == -1 {
				break
			}
			count++
			idx = idx + pos + len(char)
		}

		// 只出现1-2次 且 文本中有"离开"、"走了"、"消失"等
		if count <= 2 {
			disappearKws := []string{"离开了", "走了", "然后离开了", "离开了", "再也没有出现", "消失了"}
			for _, dk := range disappearKws {
				if strings.Contains(text, dk) {
					results = append(results, ToolCharacter{
						Name:        char,
						Description: fmt.Sprintf("%s只出场%d次就消失，典型的工具人角色", char, count),
					})
					break
				}
			}
		}
	}

	return results
}

// extractKnownCharacters 从文本中提取已知角色名
func (cs *ConstraintSystem) extractKnownCharacters(text string) []string {
	chars := make([]string, 0)
	seen := make(map[string]bool)

	addChar := func(name string) {
		if name == "" || seen[name] || isCommonNonNameWord(name) {
			return
		}
		seen[name] = true
		chars = append(chars, name)
	}

	// 1. 从 ConstraintSystem 的角色规则中获取
	for name := range cs.CharacterRules {
		addChar(name)
	}

	// 2. 从 GameWorld 角色中获取
	if cs.World != nil && cs.World.Characters != nil {
		for _, char := range cs.World.Characters.GetAllCharacters() {
			addChar(char.Name)
		}
	}

	// 3. 从文本中提取角色名 — 只提取2字名（姓+名），这是最可靠的模式
	// 修仙小说常见姓氏
	surnames := []string{"林", "李", "王", "张", "刘", "陈", "杨", "赵", "黄", "周",
		"吴", "徐", "孙", "马", "朱", "胡", "韩", "萧", "叶",
		"苏", "楚", "秦", "沈", "白", "顾", "宋", "高", "田", "方", "许"}

	surnameSet := make(map[rune]bool)
	for _, sn := range surnames {
		for _, r := range sn {
			surnameSet[r] = true
		}
	}

	runes := []rune(text)
	for i := 0; i < len(runes)-1; i++ {
		// 检查是否为姓氏 + 汉字名（2字名：如"林砚"、"张三"、"李长老"）
		if !surnameSet[runes[i]] {
			continue
		}
		// 只取2字名（最可靠），如果后续也是汉字则尝试3字
		if !isChineseRune(runes[i+1]) {
			continue
		}
		// 2字名
		name2 := string(runes[i : i+2])
		addChar(name2)

		// 3字名（如"李长老"）- 只有当第3个也是汉字时
		if i+2 < len(runes) && isChineseRune(runes[i+2]) {
			name3 := string(runes[i : i+3])
			addChar(name3)
		}
	}

	return chars
}

// findNearestCharacter 在上下文中找最近的角色名
func (cs *ConstraintSystem) findNearestCharacter(context string, knownChars []string) string {
	bestChar := ""
	bestPos := len(context)

	for _, char := range knownChars {
		pos := strings.Index(context, char)
		if pos != -1 && pos < bestPos {
			bestPos = pos
			bestChar = char
		}
	}

	return bestChar
}

// isCommonNonNameWord 判断是否为常见的非人名词
func isCommonNonNameWord(word string) bool {
	nonNames := map[string]bool{
		"因此": true, "然后": true, "自己": true, "众人": true, "对方": true,
		"什么": true, "这个": true, "那个": true, "这里": true, "那里": true,
		"现在": true, "当时": true, "忽然": true, "突然": true, "终于": true,
		"但是": true, "虽然": true, "因为": true, "所以": true, "不过": true,
		"然而": true, "可是": true, "只见": true, "同时": true,
		"就在": true, "紧接着": true,
		"第二天": true, "第三天": true,
		"一个月": true, "三个月": true, "半年后": true,
	}
	return nonNames[word]
}

// isChineseRune 判断是否为汉字字符
// 支持 CJK 统一表意文字范围 (0x4E00-0x9FFF)
func isChineseRune(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// runeSliceEqual 判断两个 rune 切片是否相等
func runeSliceEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GenerateCorrectionPrompt 生成修正提示词
func (cs *ConstraintSystem) GenerateCorrectionPrompt(violations []ConstraintViolation) string {
	var sb strings.Builder

	sb.WriteString("=== RPG约束违反，需要修正 ===\n\n")

	for i, v := range violations {
		sb.WriteString(fmt.Sprintf("问题 %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  类型: %s\n", v.Type))
		sb.WriteString(fmt.Sprintf("  对象: %s\n", v.Target))
		sb.WriteString(fmt.Sprintf("  问题: %s\n", v.Issue))
		sb.WriteString(fmt.Sprintf("  严重程度: %s\n", v.Severity))
		sb.WriteString(fmt.Sprintf("  建议: %s\n\n", v.Suggestion))
	}

	sb.WriteString("请根据以上约束违反情况修改内容，确保符合RPG系统规则。\n")

	return sb.String()
}
