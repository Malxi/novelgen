package rpg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ============================================================
// 通用大纲分析框架
// ============================================================

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Rules       []ValidationRule       `json:"rules"`
	Patterns    map[string]string      `json:"patterns"`
	Thresholds  map[string]float64     `json:"thresholds"`
	CustomData  map[string]interface{} `json:"custom_data"`
}

// ValidationRule 验证规则
type ValidationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // frequency, pattern, consistency, relationship, threshold
	Target      string            `json:"target"` // events, characters, locations, beats, etc.
	Condition   RuleCondition     `json:"condition"`
	Severity    string            `json:"severity"` // fatal, critical, warning, info
	Message     string            `json:"message"`
	Suggestion  string            `json:"suggestion"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte, contains, matches, exists
	Field    string      `json:"field"`    // 要检查的字段
	Value    interface{} `json:"value"`    // 比较值
	Pattern  string      `json:"pattern"`  // 正则表达式
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	Config      AnalyzerConfig  `json:"config"`
	Issues      []AnalysisIssue `json:"issues"`
	Metrics     AnalysisMetrics `json:"metrics"`
	Summary     string          `json:"summary"`
	RiskLevel   string          `json:"risk_level"`
}

// AnalysisIssue 分析问题
type AnalysisIssue struct {
	RuleID      string                 `json:"rule_id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Location    string                 `json:"location"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details"`
	Suggestion  string                 `json:"suggestion"`
}

// AnalysisMetrics 分析指标
type AnalysisMetrics struct {
	TotalChapters      int                `json:"total_chapters"`
	TotalCharacters    int                `json:"total_characters"`
	TotalLocations     int                `json:"total_locations"`
	TotalEvents        int                `json:"total_events"`
	CharacterFrequency map[string]int     `json:"character_frequency"`
	EventTypeCounts    map[string]int     `json:"event_type_counts"`
	LocationTransitions []LocationTransition `json:"location_transitions"`
	CustomMetrics      map[string]float64 `json:"custom_metrics"`
}

// LocationTransition 地点转换
type LocationTransition struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Chapter  string `json:"chapter"`
	HasTransition bool `json:"has_transition"`
}

// ============================================================
// 通用分析器
// ============================================================

// OutlineAnalyzer 大纲分析器
type OutlineAnalyzer struct {
	config   *AnalyzerConfig
	outline  *StoryOutline
	result   *AnalysisResult
}

// NewOutlineAnalyzer 创建新的分析器
func NewOutlineAnalyzer(config *AnalyzerConfig) *OutlineAnalyzer {
	return &OutlineAnalyzer{
		config: config,
		result: &AnalysisResult{
			Config: *config,
			Issues: []AnalysisIssue{},
			Metrics: AnalysisMetrics{
				CharacterFrequency:  make(map[string]int),
				EventTypeCounts:     make(map[string]int),
				LocationTransitions: []LocationTransition{},
				CustomMetrics:       make(map[string]float64),
			},
		},
	}
}

// Analyze 执行分析
func (oa *OutlineAnalyzer) Analyze(outline *StoryOutline) *AnalysisResult {
	oa.outline = outline
	
	// 1. 计算基础指标
	oa.calculateMetrics()
	
	// 2. 执行所有规则检查
	for _, rule := range oa.config.Rules {
		oa.executeRule(rule)
	}
	
	// 3. 计算风险等级
	oa.calculateRiskLevel()
	
	// 4. 生成摘要
	oa.generateSummary()
	
	return oa.result
}

// calculateMetrics 计算基础指标
func (oa *OutlineAnalyzer) calculateMetrics() {
	chapters := oa.collectChapters()
	oa.result.Metrics.TotalChapters = len(chapters)
	
	characterSet := make(map[string]bool)
	locationSet := make(map[string]bool)
	
	for _, ch := range chapters {
		// 统计角色
		for _, char := range ch.Characters {
			characterSet[char] = true
			oa.result.Metrics.CharacterFrequency[char]++
		}
		
		// 统计事件中的角色
		for _, evt := range ch.Events {
			oa.result.Metrics.TotalEvents++
			oa.result.Metrics.EventTypeCounts[evt.Type]++
			
			for _, char := range evt.Characters {
				characterSet[char] = true
				oa.result.Metrics.CharacterFrequency[char]++
			}
		}
		
		// 统计地点
		if ch.Location != "" {
			locationSet[ch.Location] = true
		}
	}
	
	oa.result.Metrics.TotalCharacters = len(characterSet)
	oa.result.Metrics.TotalLocations = len(locationSet)
	
	// 分析地点转换
	oa.analyzeLocationTransitions(chapters)
}

// collectChapters 收集所有章节
func (oa *OutlineAnalyzer) collectChapters() []StoryChapter {
	var chapters []StoryChapter
	for _, part := range oa.outline.Parts {
		for _, volume := range part.Volumes {
			chapters = append(chapters, volume.Chapters...)
		}
	}
	return chapters
}

// analyzeLocationTransitions 分析地点转换
func (oa *OutlineAnalyzer) analyzeLocationTransitions(chapters []StoryChapter) {
	for i := 1; i < len(chapters); i++ {
		prevCh := chapters[i-1]
		currCh := chapters[i]
		
		if prevCh.Location != currCh.Location {
			transition := LocationTransition{
				From:     prevCh.Location,
				To:       currCh.Location,
				Chapter:  currCh.ID,
			}
			
			// 检查是否有过渡描述
			transition.HasTransition = oa.hasLocationTransition(prevCh, currCh)
			oa.result.Metrics.LocationTransitions = append(
				oa.result.Metrics.LocationTransitions, 
				transition,
			)
		}
	}
}

// hasLocationTransition 检查是否有地点过渡
func (oa *OutlineAnalyzer) hasLocationTransition(prevCh, currCh StoryChapter) bool {
	transitionKeywords := []string{
		"回到", "前往", "来到", "进入", "退出", "离开", 
		"赶到", "走向", "返回", "抵达", "出发",
	}
	
	for _, keyword := range transitionKeywords {
		if strings.Contains(currCh.Beats[0], keyword) {
			return true
		}
	}
	return false
}

// executeRule 执行单个规则
func (oa *OutlineAnalyzer) executeRule(rule ValidationRule) {
	switch rule.Type {
	case "frequency":
		oa.checkFrequencyRule(rule)
	case "pattern":
		oa.checkPatternRule(rule)
	case "consistency":
		oa.checkConsistencyRule(rule)
	case "relationship":
		oa.checkRelationshipRule(rule)
	case "threshold":
		oa.checkThresholdRule(rule)
	case "custom":
		oa.checkCustomRule(rule)
	}
}

// checkFrequencyRule 频率规则检查
func (oa *OutlineAnalyzer) checkFrequencyRule(rule ValidationRule) {
	chapters := oa.collectChapters()
	
	switch rule.Target {
	case "events":
		// 统计特定事件的频率
		count := 0
		pattern := rule.Condition.Pattern
		
		for _, ch := range chapters {
			for _, evt := range ch.Events {
				if matchesPattern(evt, pattern) {
					count++
				}
			}
		}
		
		// 检查是否超过阈值
		if threshold, ok := rule.Condition.Value.(float64); ok {
			if compareValues(float64(count), threshold, rule.Condition.Operator) {
				oa.addIssue(rule, fmt.Sprintf("%s (实际: %d)", rule.Message, count), map[string]interface{}{
					"count":     count,
					"threshold": threshold,
				})
			}
		}
		
	case "character_appearances":
		// 检查角色出场频率
		for char, freq := range oa.result.Metrics.CharacterFrequency {
			if threshold, ok := rule.Condition.Value.(float64); ok {
				if compareValues(float64(freq), threshold, rule.Condition.Operator) {
					oa.addIssue(rule, fmt.Sprintf("角色 '%s' %s (出场: %d次)", char, rule.Message, freq), map[string]interface{}{
						"character": char,
						"frequency": freq,
					})
				}
			}
		}
	}
}

// checkPatternRule 模式规则检查
func (oa *OutlineAnalyzer) checkPatternRule(rule ValidationRule) {
	chapters := oa.collectChapters()
	pattern := regexp.MustCompile(rule.Condition.Pattern)
	
	for _, ch := range chapters {
		switch rule.Target {
		case "beats":
			for _, beat := range ch.Beats {
				if pattern.MatchString(beat) {
					// 根据操作符决定是否报告
					if rule.Condition.Operator == "exists" {
						oa.addIssue(rule, rule.Message, map[string]interface{}{
							"chapter": ch.ID,
							"match":   beat,
						})
					}
				}
			}
			
		case "events":
			for _, evt := range ch.Events {
				text := evt.Change + " " + evt.Details
				if pattern.MatchString(text) {
					if rule.Condition.Operator == "exists" {
						oa.addIssue(rule, rule.Message, map[string]interface{}{
							"chapter": ch.ID,
							"event":   evt,
						})
					}
				}
			}
		}
	}
}

// checkConsistencyRule 一致性规则检查
func (oa *OutlineAnalyzer) checkConsistencyRule(rule ValidationRule) {
	chapters := oa.collectChapters()
	
	switch rule.Target {
	case "character_list":
		// 检查事件中的角色是否在章节角色列表中
		for _, ch := range chapters {
			charSet := make(map[string]bool)
			for _, char := range ch.Characters {
				charSet[char] = true
			}
			
			for _, evt := range ch.Events {
				for _, char := range evt.Characters {
					if !charSet[char] {
						oa.addIssue(rule, fmt.Sprintf("事件中的角色 '%s' 不在章节角色列表中", char), map[string]interface{}{
							"chapter":   ch.ID,
							"character": char,
						})
					}
				}
			}
		}
		
	case "location_transition":
		// 检查地点转换是否有过渡
		for _, transition := range oa.result.Metrics.LocationTransitions {
			if !transition.HasTransition {
				oa.addIssue(rule, fmt.Sprintf("地点从 '%s' 变为 '%s' 缺乏过渡说明", transition.From, transition.To), map[string]interface{}{
					"from":     transition.From,
					"to":       transition.To,
					"chapter":  transition.Chapter,
				})
			}
		}
	}
}

// checkRelationshipRule 关系规则检查
func (oa *OutlineAnalyzer) checkRelationshipRule(rule ValidationRule) {
	// 检查角色关系、事件因果关系等
	// 这里可以实现更复杂的关系图分析
}

// checkThresholdRule 阈值规则检查
func (oa *OutlineAnalyzer) checkThresholdRule(rule ValidationRule) {
	// 检查各种指标是否超过阈值
	metricName := rule.Condition.Field
	
	var value float64
	switch metricName {
	case "total_chapters":
		value = float64(oa.result.Metrics.TotalChapters)
	case "total_characters":
		value = float64(oa.result.Metrics.TotalCharacters)
	case "total_events":
		value = float64(oa.result.Metrics.TotalEvents)
	default:
		if v, ok := oa.result.Metrics.CustomMetrics[metricName]; ok {
			value = v
		}
	}
	
	if threshold, ok := rule.Condition.Value.(float64); ok {
		if compareValues(value, threshold, rule.Condition.Operator) {
			oa.addIssue(rule, fmt.Sprintf("%s (实际: %.0f, 阈值: %.0f)", rule.Message, value, threshold), map[string]interface{}{
				"value":     value,
				"threshold": threshold,
			})
		}
	}
}

// checkCustomRule 自定义规则检查
func (oa *OutlineAnalyzer) checkCustomRule(rule ValidationRule) {
	// 预留接口，可以通过配置注入自定义检查逻辑
}

// addIssue 添加问题
func (oa *OutlineAnalyzer) addIssue(rule ValidationRule, message string, details map[string]interface{}) {
	issue := AnalysisIssue{
		RuleID:     rule.ID,
		Type:       rule.Type,
		Severity:   rule.Severity,
		Message:    message,
		Details:    details,
		Suggestion: rule.Suggestion,
	}
	
	if loc, ok := details["chapter"]; ok {
		issue.Location = fmt.Sprintf("%v", loc)
	}
	
	oa.result.Issues = append(oa.result.Issues, issue)
}

// calculateRiskLevel 计算风险等级
func (oa *OutlineAnalyzer) calculateRiskLevel() {
	score := 0
	
	for _, issue := range oa.result.Issues {
		switch issue.Severity {
		case "fatal":
			score += 100
		case "critical":
			score += 50
		case "warning":
			score += 20
		case "info":
			score += 5
		}
	}
	
	// 根据阈值判断风险等级
	if fatalThreshold, ok := oa.config.Thresholds["fatal_score"]; ok && score >= int(fatalThreshold) {
		oa.result.RiskLevel = "极高风险 - 需要重大修改"
	} else if criticalThreshold, ok := oa.config.Thresholds["critical_score"]; ok && score >= int(criticalThreshold) {
		oa.result.RiskLevel = "高风险 - 需要较多修改"
	} else if warningThreshold, ok := oa.config.Thresholds["warning_score"]; ok && score >= int(warningThreshold) {
		oa.result.RiskLevel = "中等风险 - 需要部分修改"
	} else {
		oa.result.RiskLevel = "低风险 - 结构良好"
	}
}

// generateSummary 生成摘要
func (oa *OutlineAnalyzer) generateSummary() {
	fatalCount := 0
	criticalCount := 0
	warningCount := 0
	infoCount := 0
	
	for _, issue := range oa.result.Issues {
		switch issue.Severity {
		case "fatal":
			fatalCount++
		case "critical":
			criticalCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}
	
	oa.result.Summary = fmt.Sprintf(
		"分析完成: 发现 %d 个致命问题, %d 个严重问题, %d 个警告, %d 个建议。风险等级: %s",
		fatalCount, criticalCount, warningCount, infoCount, oa.result.RiskLevel,
	)
}

// ============================================================
// 辅助函数
// ============================================================

// matchesPattern 检查事件是否匹配模式
func matchesPattern(evt StoryEvent, pattern string) bool {
	text := evt.Type + " " + evt.Change + " " + evt.Details + " " + evt.Subject
	matched, _ := regexp.MatchString(pattern, text)
	return matched
}

// compareValues 比较两个值
func compareValues(actual, expected float64, operator string) bool {
	switch operator {
	case "eq":
		return actual == expected
	case "ne":
		return actual != expected
	case "gt":
		return actual > expected
	case "lt":
		return actual < expected
	case "gte":
		return actual >= expected
	case "lte":
		return actual <= expected
	default:
		return false
	}
}

// ============================================================
// 预置配置
// ============================================================

// GetNovelAnalysisConfig 获取小说分析配置
func GetNovelAnalysisConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		Name:        "通用小说大纲分析器",
		Description: "适用于各类网络小说的通用大纲结构分析",
		Rules: []ValidationRule{
			// 1. 角色一致性检查
			{
				ID:         "character_consistency",
				Name:       "角色列表一致性",
				Type:       "consistency",
				Target:     "character_list",
				Severity:   "warning",
				Message:    "角色出场逻辑不一致",
				Suggestion: "将角色添加到章节角色列表或修改事件",
			},
			
			// 2. 地点转换检查
			{
				ID:         "location_transition",
				Name:       "地点转换过渡",
				Type:       "consistency",
				Target:     "location_transition",
				Severity:   "info",
				Message:    "地点转换缺乏过渡描述",
				Suggestion: "在开场节拍中添加地点转换说明",
			},
			
			// 3. 角色出场频率检查（工具人检测）
			{
				ID:       "character_frequency_low",
				Name:     "角色出场频率过低",
				Type:     "frequency",
				Target:   "character_appearances",
				Condition: RuleCondition{
					Operator: "lte",
					Value:    1.0,
				},
				Severity:   "info",
				Message:    "角色仅出场一次，可能是工具人",
				Suggestion: "考虑给角色更多戏份或合并角色",
			},
			
			// 4. 章节事件数量检查
			{
				ID:       "chapter_event_count",
				Name:     "章节事件数量",
				Type:     "threshold",
				Target:   "chapter",
				Condition: RuleCondition{
					Field:    "events_per_chapter",
					Operator: "lt",
					Value:    1.0,
				},
				Severity:   "warning",
				Message:    "章节事件过少",
				Suggestion: "增加章节内的事件推动剧情",
			},
		},
		Thresholds: map[string]float64{
			"fatal_score":    100,
			"critical_score": 70,
			"warning_score":  40,
		},
	}
}

// GetXianxiaAnalysisConfig 获取修仙小说专用配置
func GetXianxiaAnalysisConfig() *AnalyzerConfig {
	config := GetNovelAnalysisConfig()
	config.Name = "修仙小说大纲分析器"
	config.Description = "专门针对修仙小说的战力体系、境界突破等规则检查"
	
	// 添加修仙专用规则
	xianxiaRules := []ValidationRule{
		// 修为变化频率检查
		{
			ID:       "cultivation_change_frequency",
			Name:     "修为变化频率",
			Type:     "frequency",
			Target:   "events",
			Condition: RuleCondition{
				Pattern:  "(修为|境界|练气|筑基|金丹|元婴)",
				Operator: "gt",
				Value:    10.0,
			},
			Severity:   "warning",
			Message:    "修为变化过于频繁，可能导致战力体系崩坏",
			Suggestion: "减少修为变化次数，建立稳定的战力预期",
		},
		
		// 突破合理性检查
		{
			ID:       "breakthrough_pattern",
			Name:     "突破模式检查",
			Type:     "pattern",
			Target:   "events",
			Condition: RuleCondition{
				Pattern:  "突破.*跌落|跌落.*突破",
				Operator: "exists",
			},
			Severity:   "info",
			Message:    "发现突破-跌落循环模式",
			Suggestion: "确保每次跌落都有合理的恢复机制",
		},
	}
	
	config.Rules = append(config.Rules, xianxiaRules...)
	return config
}

// GetInfiniteFlowAnalysisConfig 获取无限流小说配置
func GetInfiniteFlowAnalysisConfig() *AnalyzerConfig {
	config := GetNovelAnalysisConfig()
	config.Name = "无限流小说大纲分析器"
	config.Description = "针对无限流小说的副本结构、死亡机制等规则检查"
	
	// 添加无限流专用规则
	infiniteRules := []ValidationRule{
		// 死亡复活机制检查
		{
			ID:       "death_resurrection_frequency",
			Name:     "死亡复活频率",
			Type:     "frequency",
			Target:   "events",
			Condition: RuleCondition{
				Pattern:  "复活|重生|死亡",
				Operator: "gt",
				Value:    5.0,
			},
			Severity:   "critical",
			Message:    "死亡复活次数过多，可能降低紧张感",
			Suggestion: "限制复活次数或增加复活代价",
		},
		
		// 副本结构检查
		{
			ID:       "instance_structure",
			Name:     "副本结构完整性",
			Type:     "pattern",
			Target:   "events",
			Condition: RuleCondition{
				Pattern:  "副本|世界|任务",
				Operator: "exists",
			},
			Severity:   "info",
			Message:    "检测到副本相关事件",
			Suggestion: "确保副本有明确的进入、进行、退出结构",
		},
	}
	
	config.Rules = append(config.Rules, infiniteRules...)
	return config
}

// LoadAnalyzerConfig 从JSON加载配置
func LoadAnalyzerConfig(data []byte) (*AnalyzerConfig, error) {
	var config AnalyzerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveAnalyzerConfig 保存配置到JSON
func SaveAnalyzerConfig(config *AnalyzerConfig) ([]byte, error) {
	return json.MarshalIndent(config, "", "  ")
}
