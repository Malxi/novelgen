package rpg

import (
	"fmt"
	"math"
)

// NovelSimulator 小说模拟器 - 用于验证提取的RPG数据
type NovelSimulator struct {
	world             *GameWorld
	extractedData     *NovelRPGData
	simulationLog     []SimulationLogEntry
	validationResults []SimulatorValidationResult
}

// SimulationLogEntry 模拟日志条目
type SimulationLogEntry struct {
	Time   int      `json:"time"`
	Type   string   `json:"type"`
	Actor  string   `json:"actor"`
	Action string   `json:"action"`
	Target string   `json:"target"`
	Result string   `json:"result"`
	Issues []string `json:"issues"`
}

// SimulatorValidationResult 模拟器验证结果
type SimulatorValidationResult struct {
	Category    string   `json:"category"`
	Passed      bool     `json:"passed"`
	Score       float64  `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// NewNovelSimulator 创建小说模拟器
func NewNovelSimulator(data *NovelRPGData) *NovelSimulator {
	return &NovelSimulator{
		world:             NewGameWorld(),
		extractedData:     data,
		simulationLog:     make([]SimulationLogEntry, 0),
		validationResults: make([]SimulatorValidationResult, 0),
	}
}

// Simulate 执行完整模拟
func (ns *NovelSimulator) Simulate() *SimulationReport {
	// 1. 初始化世界
	ns.initializeWorld()

	// 2. 按时间线模拟事件
	ns.simulateTimeline()

	// 3. 执行各种验证
	ns.validateCharacterConsistency()
	ns.validatePowerSystem()
	ns.validateEconomySystem()
	ns.validateCombatBalance()
	ns.validatePlotLogic()
	ns.validatePacing()

	// 4. 生成报告
	return ns.generateReport()
}

// initializeWorld 初始化世界
func (ns *NovelSimulator) initializeWorld() {
	// 添加角色模板到世界
	for _, charTemplate := range ns.extractedData.Characters {
		ns.world.Characters.AddTemplate(charTemplate)
		// 创建实例
		instance := NewCharacter(charTemplate, charTemplate.Name)
		ns.world.Characters.AddCharacterInstance(instance)
	}

	// 添加物品
	for _, item := range ns.extractedData.Items {
		ns.world.Items.AddItem(item)
	}

	// 添加技能
	for _, skill := range ns.extractedData.Skills {
		ns.world.Skills.AddSkill(skill)
	}

	// 添加地图
	for _, gameMap := range ns.extractedData.Locations {
		ns.world.Maps.AddMap(gameMap)
	}
}

// simulateTimeline 模拟时间线
func (ns *NovelSimulator) simulateTimeline() {
	// 按时间顺序处理事件
	for _, timelineEvent := range ns.extractedData.Timeline {
		ns.processTimelineEvent(timelineEvent)
	}
}

// processTimelineEvent 处理时间线事件
func (ns *NovelSimulator) processTimelineEvent(event *TimelineEvent) {
	for _, eventType := range event.Events {
		switch eventType {
		case "death":
			ns.simulateDeath(event)
		case "resurrection":
			ns.simulateResurrection(event)
		case "breakthrough":
			ns.simulateBreakthrough(event)
		case "combat":
			ns.simulateCombat(event)
		}
	}

	// 处理战力变化
	for charID, powerChange := range event.PowerChanges {
		ns.simulatePowerChange(charID, powerChange)
	}
}

// simulateDeath 模拟死亡
func (ns *NovelSimulator) simulateDeath(event *TimelineEvent) {
	for _, charName := range event.Characters {
		char := ns.world.GetCharacterByName(charName)
		if char == nil {
			continue
		}

		// 检查角色是否可以死亡
		issues := ns.validateDeath(char)

		logEntry := SimulationLogEntry{
			Time:   event.Time,
			Type:   "death",
			Actor:  charName,
			Action: "die",
			Result: fmt.Sprintf("%s 死亡", charName),
			Issues: issues,
		}
		ns.simulationLog = append(ns.simulationLog, logEntry)
	}
}

// simulateResurrection 模拟复活
func (ns *NovelSimulator) simulateResurrection(event *TimelineEvent) {
	for _, charName := range event.Characters {
		char := ns.world.GetCharacterByName(charName)
		if char == nil {
			continue
		}

		// 检查复活合理性
		issues := ns.validateResurrection(char, event)

		logEntry := SimulationLogEntry{
			Time:   event.Time,
			Type:   "resurrection",
			Actor:  charName,
			Action: "resurrect",
			Result: fmt.Sprintf("%s 复活", charName),
			Issues: issues,
		}
		ns.simulationLog = append(ns.simulationLog, logEntry)
	}
}

// simulateBreakthrough 模拟突破
func (ns *NovelSimulator) simulateBreakthrough(event *TimelineEvent) {
	for _, charName := range event.Characters {
		char := ns.world.GetCharacterByName(charName)
		if char == nil {
			continue
		}

		// 检查突破合理性
		issues := ns.validateBreakthrough(char)

		logEntry := SimulationLogEntry{
			Time:   event.Time,
			Type:   "breakthrough",
			Actor:  charName,
			Action: "breakthrough",
			Result: fmt.Sprintf("%s 突破", charName),
			Issues: issues,
		}
		ns.simulationLog = append(ns.simulationLog, logEntry)
	}
}

// simulateCombat 模拟战斗
func (ns *NovelSimulator) simulateCombat(event *TimelineEvent) {
	if len(event.Characters) < 2 {
		return
	}

	// 模拟角色之间的战斗
	for i := 0; i < len(event.Characters)-1; i++ {
		for j := i + 1; j < len(event.Characters); j++ {
			char1 := ns.world.GetCharacterByName(event.Characters[i])
			char2 := ns.world.GetCharacterByName(event.Characters[j])

			if char1 == nil || char2 == nil {
				continue
			}

			// 检查战斗平衡性
			issues := ns.validateCombatBetweenChars(char1, char2)

			logEntry := SimulationLogEntry{
				Time:   event.Time,
				Type:   "combat",
				Actor:  event.Characters[i],
				Action: "attack",
				Target: event.Characters[j],
				Result: fmt.Sprintf("%s vs %s", event.Characters[i], event.Characters[j]),
				Issues: issues,
			}
			ns.simulationLog = append(ns.simulationLog, logEntry)
		}
	}
}

// simulatePowerChange 模拟战力变化
func (ns *NovelSimulator) simulatePowerChange(charID string, change int) {
	char := ns.world.Characters.GetCharacter(charID)
	if char == nil {
		return
	}

	// 记录战力变化
	logEntry := SimulationLogEntry{
		Time:   0,
		Type:   "power_change",
		Actor:  char.Name,
		Action: "power_change",
		Result: fmt.Sprintf("战力变化: %+d", change),
	}
	ns.simulationLog = append(ns.simulationLog, logEntry)
}

// validateDeath 验证死亡合理性
func (ns *NovelSimulator) validateDeath(char *Character) []string {
	issues := make([]string, 0)

	// 检查主角死亡次数 - 通过名字判断主角（通常是第一个创建的角色）
	if char.Name == ns.world.Player.Name {
		deathCount := ns.countDeathEvents(char.Name)
		if deathCount > 3 {
			issues = append(issues, fmt.Sprintf("主角死亡次数过多: %d次", deathCount))
		}
	}

	return issues
}

// validateResurrection 验证复活合理性
func (ns *NovelSimulator) validateResurrection(char *Character, event *TimelineEvent) []string {
	issues := make([]string, 0)

	// 检查复活次数
	resurrectCount := ns.countResurrectionEvents(char.Name)

	if resurrectCount > 5 {
		issues = append(issues, fmt.Sprintf("角色%s复活次数过多: %d次", char.Name, resurrectCount))
	}

	if resurrectCount > 10 {
		issues = append(issues, "复活次数严重超标，建议重新设计剧情")
	}

	// 检查复活代价
	if !ns.hasResurrectionCost(char.Name) {
		issues = append(issues, fmt.Sprintf("角色%s复活缺乏代价设定", char.Name))
	}

	return issues
}

// validateBreakthrough 验证突破合理性
func (ns *NovelSimulator) validateBreakthrough(char *Character) []string {
	issues := make([]string, 0)

	// 检查突破频率
	breakthroughCount := ns.countBreakthroughEvents(char.Name)
	if breakthroughCount > 5 {
		issues = append(issues, fmt.Sprintf("角色%s突破过于频繁: %d次", char.Name, breakthroughCount))
	}

	// 检查突破间隔
	avgInterval := ns.calculateAverageBreakthroughInterval(char.Name)
	if avgInterval < 3 { // 少于3个时间单位
		issues = append(issues, fmt.Sprintf("角色%s突破间隔过短: 平均%.1f个时间单位", char.Name, avgInterval))
	}

	return issues
}

// validateCharacterConsistency 验证角色一致性
func (ns *NovelSimulator) validateCharacterConsistency() {
	result := SimulatorValidationResult{
		Category:    "character_consistency",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 检查角色属性一致性
	characters := ns.world.Characters.GetAllCharacters()
	for _, char := range characters {
		// 检查属性范围
		if char.CurrentStats.HP < 0 || char.CurrentStats.HP > 100000 {
			result.Issues = append(result.Issues, fmt.Sprintf("角色%s生命值异常: %d", char.Name, char.CurrentStats.HP))
			result.Passed = false
		}

		if char.CurrentStats.Attack < 0 || char.CurrentStats.Attack > 100000 {
			result.Issues = append(result.Issues, fmt.Sprintf("角色%s攻击力异常: %d", char.Name, char.CurrentStats.Attack))
			result.Passed = false
		}
	}

	// 计算得分
	if len(result.Issues) > 0 {
		result.Score = math.Max(0, 100-float64(len(result.Issues))*10)
	}

	ns.validationResults = append(ns.validationResults, result)
}

// validatePowerSystem 验证战力系统
func (ns *NovelSimulator) validatePowerSystem() {
	result := SimulatorValidationResult{
		Category:    "power_system",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 统计战力系统问题
	resurrectionCount := 0
	powerChangeCount := 0
	breakthroughCount := 0

	for _, entry := range ns.simulationLog {
		switch entry.Type {
		case "resurrection":
			resurrectionCount++
		case "power_change":
			powerChangeCount++
		case "breakthrough":
			breakthroughCount++
		}
	}

	// 检查复活次数
	if resurrectionCount > 5 {
		result.Issues = append(result.Issues, fmt.Sprintf("复活次数过多: %d次", resurrectionCount))
		result.Suggestions = append(result.Suggestions, "建议限制复活次数，每次复活增加寿命代价")
		result.Passed = false
	}

	// 检查战力变化
	if powerChangeCount > 20 {
		result.Issues = append(result.Issues, fmt.Sprintf("战力变化过于频繁: %d次", powerChangeCount))
		result.Suggestions = append(result.Suggestions, "建议稳定战力系统，增加修炼过程的描写")
		result.Passed = false
	}

	// 检查突破频率
	if breakthroughCount > 10 {
		result.Issues = append(result.Issues, fmt.Sprintf("突破次数过多: %d次", breakthroughCount))
		result.Suggestions = append(result.Suggestions, "建议减少突破次数，增加战斗和剧情内容")
		result.Passed = false
	}

	// 计算得分
	if len(result.Issues) > 0 {
		result.Score = math.Max(0, 100-float64(len(result.Issues))*15)
	}

	ns.validationResults = append(ns.validationResults, result)
}

// validateEconomySystem 验证经济系统
func (ns *NovelSimulator) validateEconomySystem() {
	result := SimulatorValidationResult{
		Category:    "economy_system",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 检查物品价值一致性
	itemValues := make(map[string]int)
	for _, item := range ns.extractedData.Items {
		if item.Value < 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("物品%s价值为负: %d", item.Name, item.Value))
			result.Passed = false
		}
		itemValues[item.Name] = item.Value
	}

	// 检查货币流通 - 通过物品名称判断
	currencyItems := make([]*Item, 0)
	for _, item := range ns.extractedData.Items {
		if item.Name == "灵石" || item.Name == "金币" || item.Name == "货币" {
			currencyItems = append(currencyItems, item)
		}
	}

	if len(currencyItems) == 0 {
		result.Issues = append(result.Issues, "未检测到货币系统")
		result.Suggestions = append(result.Suggestions, "建议添加货币物品（如灵石、金币等）")
	}

	ns.validationResults = append(ns.validationResults, result)
}

// validateCombatBalance 验证战斗平衡
func (ns *NovelSimulator) validateCombatBalance() {
	result := SimulatorValidationResult{
		Category:    "combat_balance",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 检查角色战力差距 - 使用等级作为战力指标
	characters := ns.world.Characters.GetAllCharacters()
	if len(characters) >= 2 {
		minLevel := characters[0].Level
		maxLevel := characters[0].Level

		for _, char := range characters {
			if char.Level < minLevel {
				minLevel = char.Level
			}
			if char.Level > maxLevel {
				maxLevel = char.Level
			}
		}

		levelGap := maxLevel - minLevel
		if levelGap > 50 {
			result.Issues = append(result.Issues, fmt.Sprintf("角色等级差距过大: %d级", levelGap))
			result.Suggestions = append(result.Suggestions, "建议平衡角色等级，或合理解释等级差距")
			result.Passed = false
		}
	}

	ns.validationResults = append(ns.validationResults, result)
}

// validateCombatBetweenChars 验证两个角色之间的战斗平衡
func (ns *NovelSimulator) validateCombatBetweenChars(char1, char2 *Character) []string {
	issues := make([]string, 0)

	levelDiff := math.Abs(float64(char1.Level - char2.Level))
	if levelDiff > 20 {
		issues = append(issues, fmt.Sprintf("等级差距过大: %s(%d) vs %s(%d)",
			char1.Name, char1.Level, char2.Name, char2.Level))
	}

	return issues
}

// validatePlotLogic 验证剧情逻辑
func (ns *NovelSimulator) validatePlotLogic() {
	result := SimulatorValidationResult{
		Category:    "plot_logic",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 检查时间线一致性
	locationTimeline := make(map[string][]string)
	for _, entry := range ns.simulationLog {
		if entry.Type == "death" || entry.Type == "resurrection" {
			// 检查角色是否在同一时间出现在多个地点
			for _, timelineEvent := range ns.extractedData.Timeline {
				for _, char := range timelineEvent.Characters {
					if char == entry.Actor {
						locationTimeline[char] = append(locationTimeline[char], timelineEvent.Location)
					}
				}
			}
		}
	}

	// 检查地点矛盾
	for char, locations := range locationTimeline {
		if len(locations) > 1 {
			uniqueLocs := make(map[string]bool)
			for _, loc := range locations {
				uniqueLocs[loc] = true
			}
			if len(uniqueLocs) > 1 {
				result.Issues = append(result.Issues, fmt.Sprintf("角色%s出现在多个地点", char))
				result.Passed = false
			}
		}
	}

	ns.validationResults = append(ns.validationResults, result)
}

// validatePacing 验证节奏
func (ns *NovelSimulator) validatePacing() {
	result := SimulatorValidationResult{
		Category:    "pacing",
		Passed:      true,
		Score:       100,
		Issues:      make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 统计事件类型分布
	eventTypes := make(map[string]int)
	for _, entry := range ns.simulationLog {
		eventTypes[entry.Type]++
	}

	// 检查事件分布是否合理
	totalEvents := len(ns.simulationLog)
	if totalEvents > 0 {
		combatRatio := float64(eventTypes["combat"]) / float64(totalEvents)
		deathRatio := float64(eventTypes["death"]) / float64(totalEvents)

		if combatRatio > 0.5 {
			result.Issues = append(result.Issues, "战斗事件占比过高")
			result.Suggestions = append(result.Suggestions, "建议增加剧情发展和角色互动内容")
		}

		if deathRatio > 0.3 {
			result.Issues = append(result.Issues, "死亡事件占比过高")
			result.Suggestions = append(result.Suggestions, "建议减少死亡次数，增加紧张感的其他方式")
		}
	}

	ns.validationResults = append(ns.validationResults, result)
}

// generateReport 生成模拟报告
func (ns *NovelSimulator) generateReport() *SimulationReport {
	report := &SimulationReport{
		Summary:           ns.generateSummary(),
		SimulationLog:     ns.simulationLog,
		ValidationResults: ns.validationResults,
		Recommendations:   ns.generateRecommendations(),
	}

	return report
}

// SimulationReport 模拟报告
type SimulationReport struct {
	Summary           SimulationSummary           `json:"summary"`
	SimulationLog     []SimulationLogEntry        `json:"simulation_log"`
	ValidationResults []SimulatorValidationResult `json:"validation_results"`
	Recommendations   []string                    `json:"recommendations"`
}

// SimulationSummary 模拟摘要
type SimulationSummary struct {
	TotalEvents        int     `json:"total_events"`
	CharactersInvolved int     `json:"characters_involved"`
	IssuesFound        int     `json:"issues_found"`
	CriticalIssues     int     `json:"critical_issues"`
	OverallScore       float64 `json:"overall_score"`
	Grade              string  `json:"grade"`
}

// generateSummary 生成摘要
func (ns *NovelSimulator) generateSummary() SimulationSummary {
	characters := ns.world.Characters.GetAllCharacters()
	summary := SimulationSummary{
		TotalEvents:        len(ns.simulationLog),
		CharactersInvolved: len(characters),
	}

	// 统计问题
	for _, result := range ns.validationResults {
		summary.IssuesFound += len(result.Issues)
		if !result.Passed {
			summary.CriticalIssues++
		}
	}

	// 计算总体得分
	totalScore := 0.0
	for _, result := range ns.validationResults {
		totalScore += result.Score
	}
	if len(ns.validationResults) > 0 {
		summary.OverallScore = totalScore / float64(len(ns.validationResults))
	}

	// 评级
	summary.Grade = ns.calculateGrade(summary.OverallScore)

	return summary
}

// calculateGrade 计算评级
func (ns *NovelSimulator) calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "S"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

// generateRecommendations 生成建议
func (ns *NovelSimulator) generateRecommendations() []string {
	recommendations := make([]string, 0)

	for _, result := range ns.validationResults {
		recommendations = append(recommendations, result.Suggestions...)
	}

	// 去重
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, rec := range recommendations {
		if !seen[rec] {
			seen[rec] = true
			unique = append(unique, rec)
		}
	}

	return unique
}

// 辅助方法
func (ns *NovelSimulator) countDeathEvents(charName string) int {
	count := 0
	for _, entry := range ns.simulationLog {
		if entry.Type == "death" && entry.Actor == charName {
			count++
		}
	}
	return count
}

func (ns *NovelSimulator) countResurrectionEvents(charName string) int {
	count := 0
	for _, entry := range ns.simulationLog {
		if entry.Type == "resurrection" && entry.Actor == charName {
			count++
		}
	}
	return count
}

func (ns *NovelSimulator) countBreakthroughEvents(charName string) int {
	count := 0
	for _, entry := range ns.simulationLog {
		if entry.Type == "breakthrough" && entry.Actor == charName {
			count++
		}
	}
	return count
}

func (ns *NovelSimulator) hasResurrectionCost(charName string) bool {
	// 检查是否有复活代价的设定
	// 这里简化处理，实际应该检查文本中是否有寿命、修为等代价描述
	return true // 默认假设有代价
}

func (ns *NovelSimulator) calculateAverageBreakthroughInterval(charName string) float64 {
	times := make([]int, 0)
	for _, entry := range ns.simulationLog {
		if entry.Type == "breakthrough" && entry.Actor == charName {
			times = append(times, entry.Time)
		}
	}

	if len(times) < 2 {
		return 100 // 默认值
	}

	totalInterval := 0
	for i := 1; i < len(times); i++ {
		totalInterval += times[i] - times[i-1]
	}

	return float64(totalInterval) / float64(len(times)-1)
}

// GetCharacterByName 通过名称获取角色
func (w *GameWorld) GetCharacterByName(name string) *Character {
	characters := w.Characters.GetAllCharacters()
	for _, char := range characters {
		if char.Name == name {
			return char
		}
	}
	return nil
}
