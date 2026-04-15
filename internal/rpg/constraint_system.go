package rpg

import (
	"fmt"
	"strings"
)

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
	return &ConstraintSystem{
		World:          world,
		Checker:        NewOutlineRPGChecker(),
		CharacterRules: make(map[string]*CharacterConstraint),
		PlotRules:      &PlotConstraint{},
		PowerRules:     &PowerSystemConstraint{},
	}
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

	// 检查战力变化
	powerChanges := cs.detectPowerChanges(chapterText)
	if len(powerChanges) > cs.PowerRules.MaxPowerChangesPerArc {
		violations = append(violations, ConstraintViolation{
			Type:     "power",
			Target:   "战力系统",
			Issue:    fmt.Sprintf("战力变化过于频繁: %d次", len(powerChanges)),
			Severity: "critical",
			Suggestion: "减少突破次数，增加修炼过程的描写",
		})
	}

	// 检查复活次数
	resurrectionCount := cs.detectResurrections(chapterText)
	for charName, count := range resurrectionCount {
		if constraint, exists := cs.CharacterRules[charName]; exists {
			if count > constraint.MaxResurrections {
				violations = append(violations, ConstraintViolation{
					Type:     "character",
					Target:   charName,
					Issue:    fmt.Sprintf("复活次数超限: %d/%d", count, constraint.MaxResurrections),
					Severity: "critical",
					Suggestion: fmt.Sprintf("限制%s的复活次数，或增加复活代价", charName),
				})
			}
		}
	}

	// 检查时间跳跃
	timeJumps := cs.detectTimeJumps(chapterText)
	if len(timeJumps) > cs.PlotRules.MaxTimeJumpsPerChapter {
		violations = append(violations, ConstraintViolation{
			Type:     "timeline",
			Target:   "时间线",
			Issue:    fmt.Sprintf("时间跳跃过多: %d次", len(timeJumps)),
			Severity: "warning",
			Suggestion: "减少时间跳跃，或增加过渡描述",
		})
	}

	// 检查禁止元素
	for _, element := range cs.PlotRules.ForbiddenElements {
		if strings.Contains(chapterText, element) {
			violations = append(violations, ConstraintViolation{
				Type:     "plot",
				Target:   "剧情",
				Issue:    fmt.Sprintf("出现禁止元素: %s", element),
				Severity: "error",
				Suggestion: fmt.Sprintf("移除或修改%s相关内容", element),
			})
		}
	}

	return violations
}

// ConstraintViolation 约束违反
type ConstraintViolation struct {
	Type       string
	Target     string
	Issue      string
	Severity   string // critical, error, warning
	Suggestion string
}

// detectPowerChanges 检测战力变化
func (cs *ConstraintSystem) detectPowerChanges(text string) []string {
	changes := make([]string, 0)
	patterns := []string{"突破", "晋升", "跌落", "降至", "提升到"}
	
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			changes = append(changes, pattern)
		}
	}
	
	return changes
}

// detectResurrections 检测复活
func (cs *ConstraintSystem) detectResurrections(text string) map[string]int {
	count := make(map[string]int)
	// 简化实现，实际应该更精确
	if strings.Contains(text, "复活") {
		count["主角"] = strings.Count(text, "复活")
	}
	return count
}

// detectTimeJumps 检测时间跳跃
func (cs *ConstraintSystem) detectTimeJumps(text string) []string {
	jumps := make([]string, 0)
	patterns := []string{"三天后", "一个月后", "半年后", "一年后", "数年后"}
	
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			jumps = append(jumps, pattern)
		}
	}
	
	return jumps
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
