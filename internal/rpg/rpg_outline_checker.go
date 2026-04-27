package rpg

import (
	"fmt"
	"math"
	"strings"
)

// ============================================================
// RPG风格的大纲检测系统
// 将大纲问题转化为RPG数值异常来检测
// ============================================================

// OutlineRPGStats 大纲的RPG属性
type OutlineRPGStats struct {
	// 基础属性
	StructureIntegrity int `json:"structure_integrity"` // 结构完整性 (0-100)
	LogicConsistency   int `json:"logic_consistency"`   // 逻辑一致性 (0-100)
	CharacterBalance   int `json:"character_balance"`   // 角色平衡性 (0-100)
	PlotCoherence      int `json:"plot_coherence"`      // 剧情连贯性 (0-100)
	PacingQuality      int `json:"pacing_quality"`      // 节奏质量 (0-100)
	
	// 战斗属性（对抗崩坏）
	PowerSystemDefense int `json:"power_system_defense"` // 战力系统防御
	TimelineStability  int `json:"timeline_stability"`   // 时间线稳定性
	CharacterFocus     int `json:"character_focus"`      // 角色聚焦度
	ConflictIntensity  int `json:"conflict_intensity"`   // 冲突强度
	
	// 资源属性
	LifeSpan           int `json:"life_span"`            // 寿命/可持续性
	PlotArmor          int `json:"plot_armor"`           // 剧情护甲
	SuspensionOfDisbelief int `json:"suspension_of_disbelief"` //  suspension of disbelief
}

// OutlineDebuff 大纲负面状态
type OutlineDebuff struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    int    `json:"severity"` // 1-10
	Effect      string `json:"effect"`
	Location    string `json:"location"`
}

// OutlineBoss 大纲BOSS（严重问题）
type OutlineBoss struct {
	Name         string   `json:"name"`
	HP           int      `json:"hp"`
	MaxHP        int      `json:"max_hp"`
	Attack       int      `json:"attack"`
	Defense      int      `json:"defense"`
	Weaknesses   []string `json:"weaknesses"`
	Location     string   `json:"location"`
	Description  string   `json:"description"`
	Drops        []string `json:"drops"` // 修复建议
}

// RPGCheckResult RPG检测结果
type RPGCheckResult struct {
	Stats       OutlineRPGStats   `json:"stats"`
	Debuffs     []OutlineDebuff   `json:"debuffs"`
	Bosses      []OutlineBoss     `json:"bosses"`
	TotalScore  int               `json:"total_score"`
	Grade       string            `json:"grade"`
	Diagnosis   string            `json:"diagnosis"`
	Prescription []string          `json:"prescription"` // 治疗方案
}

// OutlineRPGChecker RPG风格的大纲检测器
type OutlineRPGChecker struct {
	outline *StoryOutline
	result  *RPGCheckResult
}

// NewOutlineRPGChecker 创建RPG检测器
func NewOutlineRPGChecker() *OutlineRPGChecker {
	return &OutlineRPGChecker{
		result: &RPGCheckResult{
			Stats:        OutlineRPGStats{},
			Debuffs:      []OutlineDebuff{},
			Bosses:       []OutlineBoss{},
			Prescription: []string{},
		},
	}
}

// Check 执行RPG风格检测
func (orc *OutlineRPGChecker) Check(outline *StoryOutline) *RPGCheckResult {
	orc.outline = outline
	
	// 1. 计算基础属性
	orc.calculateBaseStats()
	
	// 2. 检测负面状态
	orc.detectDebuffs()
	
	// 3. 检测BOSS级问题
	orc.detectBosses()
	
	// 4. 计算总评分
	orc.calculateTotalScore()
	
	// 5. 生成诊断
	orc.generateDiagnosis()
	
	return orc.result
}

// calculateBaseStats 计算基础属性
func (orc *OutlineRPGChecker) calculateBaseStats() {
	chapters := orc.collectChapters()
	
	// 结构完整性 = 章节完整性 × 地点完整性 × 角色完整性
	orc.result.Stats.StructureIntegrity = orc.calculateStructureIntegrity(chapters)
	
	// 逻辑一致性 = 事件逻辑 × 时间线逻辑 × 因果关系
	orc.result.Stats.LogicConsistency = orc.calculateLogicConsistency(chapters)
	
	// 角色平衡性 = 主角出场率 × 角色分布均匀度
	orc.result.Stats.CharacterBalance = orc.calculateCharacterBalance(chapters)
	
	// 剧情连贯性 = 章节过渡 × 事件衔接
	orc.result.Stats.PlotCoherence = orc.calculatePlotCoherence(chapters)
	
	// 节奏质量 = 快慢交替合理性
	orc.result.Stats.PacingQuality = orc.calculatePacingQuality(chapters)
	
	// 战力系统防御（修仙小说专用）
	orc.result.Stats.PowerSystemDefense = orc.calculatePowerSystemDefense(chapters)
	
	// 时间线稳定性
	orc.result.Stats.TimelineStability = orc.calculateTimelineStability(chapters)
	
	// 角色聚焦度
	orc.result.Stats.CharacterFocus = orc.calculateCharacterFocus(chapters)
	
	// 冲突强度
	orc.result.Stats.ConflictIntensity = orc.calculateConflictIntensity(chapters)
	
	// 寿命/可持续性
	orc.result.Stats.LifeSpan = orc.calculateLifeSpan(chapters)
	
	// 剧情护甲
	orc.result.Stats.PlotArmor = orc.calculatePlotArmor(chapters)
	
	// Suspension of disbelief
	orc.result.Stats.SuspensionOfDisbelief = orc.calculateSuspensionOfDisbelief(chapters)
}

// calculateStructureIntegrity 计算结构完整性
func (orc *OutlineRPGChecker) calculateStructureIntegrity(chapters []StoryChapter) int {
	score := 100
	
	// 检查章节完整性
	for _, ch := range chapters {
		// 缺少必要字段扣分
		if ch.Beats[0] == "" {
			score -= 5
		}
		if len(ch.Beats) > 0 && ch.Beats[len(ch.Beats)-1] == "" {
			score -= 5
		}
		if ch.StateChange == "" {
			score -= 3
		}
		if ch.Conflict == "" {
			score -= 3
		}
	}
	
	return max(0, score)
}

// calculateLogicConsistency 计算逻辑一致性
func (orc *OutlineRPGChecker) calculateLogicConsistency(chapters []StoryChapter) int {
	score := 100
	
	// 检查角色一致性
	for _, ch := range chapters {
		charSet := make(map[string]bool)
		for _, char := range ch.Characters {
			charSet[char] = true
		}
		
		for _, evt := range ch.Events {
			for _, char := range evt.Characters {
				if !charSet[char] {
					score -= 2
				}
			}
		}
	}
	
	return max(0, score)
}

// calculateCharacterBalance 计算角色平衡性
func (orc *OutlineRPGChecker) calculateCharacterBalance(chapters []StoryChapter) int {
	charFreq := make(map[string]int)
	totalEvents := 0
	
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			totalEvents++
			for _, char := range evt.Characters {
				charFreq[char]++
			}
		}
	}
	
	if totalEvents == 0 {
		return 50
	}
	
	// 计算主角出场率（假设主角是出场最多的）
	maxFreq := 0
	for _, freq := range charFreq {
		if freq > maxFreq {
			maxFreq = freq
		}
	}
	
	mcPresence := float64(maxFreq) / float64(totalEvents) * 100
	
	// 主角出场率应该在30%-70%之间
	if mcPresence < 30 {
		return int(mcPresence * 2) // 出场太少
	} else if mcPresence > 70 {
		return int((100 - mcPresence) * 2) // 出场太多，其他角色没戏份
	}
	
	return 100 - int(math.Abs(mcPresence-50)) // 越接近50%越好
}

// calculatePlotCoherence 计算剧情连贯性
func (orc *OutlineRPGChecker) calculatePlotCoherence(chapters []StoryChapter) int {
	score := 100
	
	// 检查地点转换
	for i := 1; i < len(chapters); i++ {
		prevCh := chapters[i-1]
		currCh := chapters[i]
		
		if prevCh.Location != currCh.Location {
			// 检查是否有过渡描述
			hasTransition := false
			transitionKeywords := []string{"回到", "前往", "来到", "进入", "离开", "赶到"}
			for _, keyword := range transitionKeywords {
				if strings.Contains(currCh.Beats[0], keyword) {
					hasTransition = true
					break
				}
			}
			
			if !hasTransition {
				score -= 1
			}
		}
	}
	
	return max(0, score)
}

// calculatePacingQuality 计算节奏质量
func (orc *OutlineRPGChecker) calculatePacingQuality(chapters []StoryChapter) int {
	if len(chapters) < 2 {
		return 50
	}
	
	// 检查快慢节奏交替
	fastCount := 0
	slowCount := 0
	
	for _, ch := range chapters {
		switch ch.Pacing {
		case "fast":
			fastCount++
		case "slow":
			slowCount++
		default:
			slowCount++ // 默认算慢节奏
		}
	}
	
	// 理想比例是快:慢 = 3:7 到 5:5
	total := fastCount + slowCount
	if total == 0 {
		return 50
	}
	
	fastRatio := float64(fastCount) / float64(total) * 100
	
	// 理想值40%，偏离越多扣分
	return max(0, 100-int(math.Abs(fastRatio-40)*2))
}

// calculatePowerSystemDefense 计算战力系统防御
func (orc *OutlineRPGChecker) calculatePowerSystemDefense(chapters []StoryChapter) int {
	score := 100
	
	// 统计修为/境界相关事件
	powerChanges := 0
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			text := evt.Change + " " + evt.Details
			if strings.Contains(text, "修为") || 
			   strings.Contains(text, "练气") || 
			   strings.Contains(text, "筑基") ||
			   strings.Contains(text, "境界") {
				powerChanges++
			}
		}
	}
	
	// 每章平均修为变化次数
	avgChanges := float64(powerChanges) / float64(len(chapters))
	
	// 理想值是每3章变化1次，即0.33
	if avgChanges > 0.5 {
		score -= int((avgChanges - 0.33) * 100)
	}
	
	return max(0, score)
}

// calculateTimelineStability 计算时间线稳定性
func (orc *OutlineRPGChecker) calculateTimelineStability(chapters []StoryChapter) int {
	score := 100
	
	// 检查时间跳跃是否合理
	timeJumps := 0
	for _, ch := range chapters {
		for _, beat := range ch.Beats {
			if strings.Contains(beat, "三天后") || 
			   strings.Contains(beat, "一个月后") ||
			   strings.Contains(beat, "半年后") {
				timeJumps++
			}
		}
	}
	
	// 时间跳跃太多会降低稳定性
	if timeJumps > len(chapters)/3 {
		score -= (timeJumps - len(chapters)/3) * 5
	}
	
	return max(0, score)
}

// calculateCharacterFocus 计算角色聚焦度
func (orc *OutlineRPGChecker) calculateCharacterFocus(chapters []StoryChapter) int {
	// 统计主要角色数量
	charFreq := make(map[string]int)
	
	for _, ch := range chapters {
		for _, char := range ch.Characters {
			charFreq[char]++
		}
	}
	
	// 出场超过3次的算主要角色
	mainChars := 0
	for _, freq := range charFreq {
		if freq >= 3 {
			mainChars++
		}
	}
	
	// 理想的主要角色数量是5-10个
	if mainChars < 5 {
		return 60 + mainChars*8 // 角色太少
	} else if mainChars > 15 {
		return 100 - (mainChars-15)*3 // 角色太多，分散
	}
	
	return 100
}

// calculateConflictIntensity 计算冲突强度
func (orc *OutlineRPGChecker) calculateConflictIntensity(chapters []StoryChapter) int {
	if len(chapters) == 0 {
		return 50
	}
	
	conflictCount := 0
	for _, ch := range chapters {
		if ch.Conflict != "" {
			conflictCount++
		}
	}
	
	// 理想比例是70%的章节有冲突
	ratio := float64(conflictCount) / float64(len(chapters)) * 100
	
	return 100 - int(math.Abs(ratio-70))
}

// calculateLifeSpan 计算寿命/可持续性
func (orc *OutlineRPGChecker) calculateLifeSpan(chapters []StoryChapter) int {
	score := 100
	
	// 检测死亡/复活次数
	deathCount := 0
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			if strings.Contains(evt.Change, "死亡") || 
			   strings.Contains(evt.Change, "复活") {
				deathCount++
			}
		}
	}
	
	// 死亡次数过多会降低可持续性
	if deathCount > 10 {
		score -= (deathCount - 10) * 5
	}
	
	return max(0, score)
}

// calculatePlotArmor 计算剧情护甲
func (orc *OutlineRPGChecker) calculatePlotArmor(chapters []StoryChapter) int {
	score := 100
	
	// 检测主角是否过于顺利
	mcSuccessCount := 0
	mcFailureCount := 0
	
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			if strings.Contains(evt.Subject, "林砚") {
				if strings.Contains(evt.Change, "成功") || 
				   strings.Contains(evt.Change, "获得") ||
				   strings.Contains(evt.Change, "提升") {
					mcSuccessCount++
				}
				if strings.Contains(evt.Change, "失败") || 
				   strings.Contains(evt.Change, "损失") ||
				   strings.Contains(evt.Change, "跌落") {
					mcFailureCount++
				}
			}
		}
	}
	
	// 理想的成功:失败比例是 3:1
	if mcFailureCount == 0 {
		score -= 30 // 主角太顺利，缺乏挫折
	} else {
		ratio := float64(mcSuccessCount) / float64(mcFailureCount)
		if ratio > 5 {
			score -= int((ratio - 3) * 10)
		}
	}
	
	return max(0, score)
}

// calculateSuspensionOfDisbelief 计算可信度
func (orc *OutlineRPGChecker) calculateSuspensionOfDisbelief(chapters []StoryChapter) int {
	score := 100
	
	// 检测过于牵强的情节
	for _, ch := range chapters {
		// 检查巧合次数
		coincidenceCount := 0
		for _, beat := range ch.Beats {
			if strings.Contains(beat, "恰好") || 
			   strings.Contains(beat, "正好") ||
			   strings.Contains(beat, "刚好") ||
			   strings.Contains(beat, "偶然") {
				coincidenceCount++
			}
		}
		
		// 每章最多1个巧合
		if coincidenceCount > 1 {
			score -= (coincidenceCount - 1) * 5
		}
	}
	
	return max(0, score)
}

// detectDebuffs 检测负面状态
func (orc *OutlineRPGChecker) detectDebuffs() {
	chapters := orc.collectChapters()
	
	// Debuff 1: 战力崩坏
	if orc.result.Stats.PowerSystemDefense < 50 {
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "战力崩坏",
			Description: "修为/境界变化过于频繁，读者难以建立稳定预期",
			Severity:    (100 - orc.result.Stats.PowerSystemDefense) / 10,
			Effect:      "读者流失风险 +30%",
		})
	}
	
	// Debuff 2: 时间混乱
	if orc.result.Stats.TimelineStability < 60 {
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "时间混乱",
			Description: "时间跳跃过于频繁或缺乏过渡",
			Severity:    (100 - orc.result.Stats.TimelineStability) / 10,
			Effect:      "剧情理解难度 +25%",
		})
	}
	
	// Debuff 3: 角色分散
	if orc.result.Stats.CharacterFocus < 50 {
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "角色分散",
			Description: "角色数量过多或分布不均",
			Severity:    (100 - orc.result.Stats.CharacterFocus) / 10,
			Effect:      "角色记忆难度 +20%",
		})
	}
	
	// Debuff 4: 节奏失控
	if orc.result.Stats.PacingQuality < 50 {
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "节奏失控",
			Description: "快慢节奏比例失衡",
			Severity:    (100 - orc.result.Stats.PacingQuality) / 10,
			Effect:      "阅读疲劳度 +25%",
		})
	}
	
	// Debuff 5: 逻辑断裂
	if orc.result.Stats.LogicConsistency < 60 {
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "逻辑断裂",
			Description: "角色出场不一致或事件逻辑矛盾",
			Severity:    (100 - orc.result.Stats.LogicConsistency) / 10,
			Effect:      "吐槽概率 +40%",
		})
	}
	
	// Debuff 6: 寿命耗尽（针对复活机制）
	if orc.result.Stats.LifeSpan < 50 {
		deathCount := 0
		for _, ch := range chapters {
			for _, evt := range ch.Events {
				if strings.Contains(evt.Change, "复活") {
					deathCount++
				}
			}
		}
		
		orc.result.Debuffs = append(orc.result.Debuffs, OutlineDebuff{
			Name:        "寿命耗尽",
			Description: fmt.Sprintf("主角已死亡/复活 %d 次，系统可持续性极低", deathCount),
			Severity:    (100 - orc.result.Stats.LifeSpan) / 10,
			Effect:      "紧张感 -50%，读者免疫",
		})
	}
}

// detectBosses 检测BOSS级问题
func (orc *OutlineRPGChecker) detectBosses() {
	chapters := orc.collectChapters()
	
	// BOSS 1: 复活滥用
	deathCount := 0
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			if strings.Contains(evt.Change, "复活") {
				deathCount++
			}
		}
	}
	
	if deathCount > 10 {
		orc.result.Bosses = append(orc.result.Bosses, OutlineBoss{
			Name:        "复活滥用者",
			HP:          deathCount * 10,
			MaxHP:       deathCount * 10,
			Attack:      80,
			Defense:     60,
			Weaknesses:  []string{"限制复活次数", "增加死亡代价", "引入真正死亡威胁"},
			Description: fmt.Sprintf("主角已复活 %d 次，读者对死亡失去敬畏", deathCount),
			Drops: []string{
				"复活次数限制器",
				"寿命消耗机制",
				"永久性死亡设定",
			},
		})
	}
	
	// BOSS 2: 战力崩坏王
	powerChanges := 0
	for _, ch := range chapters {
		for _, evt := range ch.Events {
			text := evt.Change + " " + evt.Details
			if strings.Contains(text, "修为") || strings.Contains(text, "练气") {
				powerChanges++
			}
		}
	}
	
	if powerChanges > 20 {
		orc.result.Bosses = append(orc.result.Bosses, OutlineBoss{
			Name:        "战力崩坏王",
			HP:          powerChanges * 5,
			MaxHP:       powerChanges * 5,
			Attack:      90,
			Defense:     70,
			Weaknesses:  []string{"建立战力体系文档", "设定升级规则", "减少频繁变化"},
			Description: fmt.Sprintf("修为变化 %d 次，战力体系完全崩坏", powerChanges),
			Drops: []string{
				"战力体系规则书",
				"升级冷却机制",
				"境界瓶颈设定",
			},
		})
	}
	
	// BOSS 3: 工具人军团
	toolChars := 0
	charFreq := make(map[string]int)
	for _, ch := range chapters {
		for _, char := range ch.Characters {
			charFreq[char]++
		}
	}
	for _, freq := range charFreq {
		if freq == 1 {
			toolChars++
		}
	}
	
	if toolChars > 10 {
		orc.result.Bosses = append(orc.result.Bosses, OutlineBoss{
			Name:        "工具人军团",
			HP:          toolChars * 15,
			MaxHP:       toolChars * 15,
			Attack:      50,
			Defense:     80,
			Weaknesses:  []string{"合并角色", "给工具人增加背景", "延长角色出场时间"},
			Description: fmt.Sprintf("发现 %d 个只出场一次的工具人角色", toolChars),
			Drops: []string{
				"角色背景生成器",
				"角色关系网",
				"角色合并卷轴",
			},
		})
	}
}

// calculateTotalScore 计算总评分
func (orc *OutlineRPGChecker) calculateTotalScore() {
	stats := orc.result.Stats
	
	// 加权计算总分
	score := int(
		float64(stats.StructureIntegrity)*0.15 +
		float64(stats.LogicConsistency)*0.20 +
		float64(stats.CharacterBalance)*0.15 +
		float64(stats.PlotCoherence)*0.15 +
		float64(stats.PacingQuality)*0.10 +
		float64(stats.PowerSystemDefense)*0.10 +
		float64(stats.TimelineStability)*0.10 +
		float64(stats.LifeSpan)*0.05,
	)
	
	// 负面状态惩罚
	for _, debuff := range orc.result.Debuffs {
		score -= debuff.Severity * 3
	}
	
	// BOSS惩罚
	for _, boss := range orc.result.Bosses {
		score -= boss.Attack / 5
	}
	
	orc.result.TotalScore = max(0, min(100, score))
	
	// 评级
	switch {
	case orc.result.TotalScore >= 90:
		orc.result.Grade = "S"
	case orc.result.TotalScore >= 80:
		orc.result.Grade = "A"
	case orc.result.TotalScore >= 70:
		orc.result.Grade = "B"
	case orc.result.TotalScore >= 60:
		orc.result.Grade = "C"
	case orc.result.TotalScore >= 50:
		orc.result.Grade = "D"
	default:
		orc.result.Grade = "F"
	}
}

// generateDiagnosis 生成诊断
func (orc *OutlineRPGChecker) generateDiagnosis() {
	// 根据评分生成诊断
	switch orc.result.Grade {
	case "S":
		orc.result.Diagnosis = "大纲状态完美，结构完整，逻辑严密，可以直接进入写作阶段"
	case "A":
		orc.result.Diagnosis = "大纲状态良好，少量细节需要优化，整体可接受"
	case "B":
		orc.result.Diagnosis = "大纲状态一般，存在一些问题需要修复，建议优化后再写作"
	case "C":
		orc.result.Diagnosis = "大纲状态较差，存在明显缺陷，必须修复后才能开始写作"
	case "D":
		orc.result.Diagnosis = "大纲状态危险，存在严重问题，需要大规模重构"
	case "F":
		orc.result.Diagnosis = "大纲状态崩溃，建议重新设计大纲结构"
	}
	
	// 生成治疗方案
	for _, debuff := range orc.result.Debuffs {
		orc.result.Prescription = append(orc.result.Prescription, 
			fmt.Sprintf("【%s】%s", debuff.Name, debuff.Description))
	}
	
	for _, boss := range orc.result.Bosses {
		orc.result.Prescription = append(orc.result.Prescription,
			fmt.Sprintf("【BOSS战：%s】弱点：%v", boss.Name, boss.Weaknesses))
	}
}

// collectChapters 收集所有章节
func (orc *OutlineRPGChecker) collectChapters() []StoryChapter {
	var chapters []StoryChapter
	for _, part := range orc.outline.Parts {
		for _, volume := range part.Volumes {
			chapters = append(chapters, volume.Chapters...)
		}
	}
	return chapters
}

