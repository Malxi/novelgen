package agents

import (
	"context"
	"fmt"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// ChapterAnalysisInput 章节分析输入
type ChapterAnalysisInput struct {
	ChapterID       string `json:"chapter_id" md:"chapter_id" desc:"章节ID"`
	ChapterTitle    string `json:"chapter_title" md:"chapter_title" desc:"章节标题"`
	ChapterContent  string `json:"chapter_content" md:"chapter_content" desc:"章节完整内容"`
	PreviousContext string `json:"previous_context,omitempty" md:"previous_context,omitempty" desc:"前一章的上下文摘要"`
	StorySetup      string `json:"story_setup,omitempty" md:"story_setup,omitempty" desc:"故事设定（金手指规则等）"`
}

// ChapterAnalysisOutput 章节分析输出
type ChapterAnalysisOutput struct {
	Characters      []CharacterInfo   `json:"characters" md:"characters" desc:"本章节出现的角色"`
	Events          []EventInfo       `json:"events" md:"events" desc:"重要事件"`
	PowerChanges    []PowerChangeInfo `json:"power_changes" md:"power_changes" desc:"战力/修为变化"`
	TimelineInfo    TimelineInfo      `json:"timeline_info" md:"timeline_info" desc:"时间线信息"`
	Issues          []ChapterIssue    `json:"issues" md:"issues" desc:"发现的问题"`
	Summary         string            `json:"summary" md:"summary" desc:"章节摘要"`
}

// CharacterInfo 角色信息
type CharacterInfo struct {
	Name           string   `json:"name" md:"name" desc:"角色名"`
	State          string   `json:"state" md:"state" desc:"状态: alive, dead, injured, unconscious"`
	Cultivation    string   `json:"cultivation,omitempty" md:"cultivation,omitempty" desc:"修为境界"`
	PowerLevel     int      `json:"power_level,omitempty" md:"power_level,omitempty" desc:"战力等级"`
	HP             int      `json:"hp,omitempty" md:"hp,omitempty" desc:"生命值"`
	MaxHP          int      `json:"max_hp,omitempty" md:"max_hp,omitempty" desc:"最大生命值"`
	Location       string   `json:"location,omitempty" md:"location,omitempty" desc:"所在位置"`
	IsResurrected  bool     `json:"is_resurrected,omitempty" md:"is_resurrected,omitempty" desc:"是否在本章复活"`
	ResurrectionCost string `json:"resurrection_cost,omitempty" md:"resurrection_cost,omitempty" desc:"复活代价"`
	Appearances    int      `json:"appearances" md:"appearances" desc:"出场次数"`
}

// EventInfo 事件信息
type EventInfo struct {
	Type        string   `json:"type" md:"type" desc:"事件类型: combat, dialogue, cultivation, discovery, death, resurrection"`
	Description string   `json:"description" md:"description" desc:"事件描述"`
	Characters  []string `json:"characters" md:"characters" desc:"参与角色"`
	Location    string   `json:"location" md:"location" desc:"发生地点"`
	Importance  int      `json:"importance" md:"importance" desc:"重要性 1-10"`
}

// PowerChangeInfo 战力变化信息
type PowerChangeInfo struct {
	Character   string `json:"character" md:"character" desc:"角色名"`
	FromLevel   string `json:"from_level" md:"from_level" desc:"变化前境界"`
	ToLevel     string `json:"to_level" md:"to_level" desc:"变化后境界"`
	ChangeType  string `json:"change_type" md:"change_type" desc:"变化类型: breakthrough, regression, injury_recovery, item_boost"`
	Reason      string `json:"reason" md:"reason" desc:"变化原因"`
	IsLegitimate bool  `json:"is_legitimate" md:"is_legitimate" desc:"是否合理"`
}

// TimelineInfo 时间线信息
type TimelineInfo struct {
	StartTime      string `json:"start_time" md:"start_time" desc:"章节开始时间"`
	EndTime        string `json:"end_time" md:"end_time" desc:"章节结束时间"`
	Duration       string `json:"duration" md:"duration" desc:"持续时间"`
	TimeJumps      int    `json:"time_jumps" md:"time_jumps" desc:"时间跳跃次数"`
	IsContinuous   bool   `json:"is_continuous" md:"is_continuous" desc:"时间是否连续"`
}

// ChapterIssue 章节问题
type ChapterIssue struct {
	Category    string `json:"category" md:"category" desc:"问题类别: power, timeline, character, plot, consistency"`
	Severity    string `json:"severity" md:"severity" desc:"严重程度: critical, error, warning, info"`
	Target      string `json:"target" md:"target" desc:"问题对象"`
	Description string `json:"description" md:"description" desc:"问题描述"`
	Evidence    string `json:"evidence" md:"evidence" desc:"证据文本"`
	Suggestion  string `json:"suggestion" md:"suggestion" desc:"修改建议"`
}

// ChapterAnalysisAgent 章节分析Agent
type ChapterAnalysisAgent struct {
	base *BaseAgent
}

// NewChapterAnalysisAgent 创建章节分析Agent
func NewChapterAnalysisAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *ChapterAnalysisAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "ChapterAnalysisAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &ChapterAnalysisAgent{base: base}
}

// SetLanguage 设置输出语言
func (a *ChapterAnalysisAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// Analyze 分析单个章节
func (a *ChapterAnalysisAgent) Analyze(ctx context.Context, input ChapterAnalysisInput) (ChapterAnalysisOutput, error) {
	logger.Section("CHAPTER ANALYSIS AGENT - Chapter Analysis")
	logger.Info("Chapter: %s - %s", input.ChapterID, input.ChapterTitle)
	logger.Info("Content length: %d characters", len(input.ChapterContent))

	// 截断过长的内容（避免token超限）
	content := input.ChapterContent
	if len(content) > 15000 {
		logger.Warn("Chapter content too long (%d chars), truncating to 15000", len(content))
		content = content[:15000] + "\n... [内容已截断]"
		input.ChapterContent = content
	}

	var output ChapterAnalysisOutput
	params := InvokeParams{
		Skills:  []string{"chapter-analysis"},
		Command: "analyze the chapter and extract RPG data",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return ChapterAnalysisOutput{}, err
	}

	// 后处理验证
	a.validateOutput(&output)

	// 日志记录结果
	logger.Section("Chapter Analysis Result")
	logger.Info("Characters found: %d", len(output.Characters))
	logger.Info("Events found: %d", len(output.Events))
	logger.Info("Power changes: %d", len(output.PowerChanges))
	logger.Info("Issues detected: %d", len(output.Issues))

	if len(output.Issues) > 0 {
		logger.Warn("Detected %d issues:", len(output.Issues))
		for _, issue := range output.Issues {
			logger.Warn("  [%s/%s] %s: %s", issue.Severity, issue.Category, issue.Target, issue.Description)
		}
	}

	return output, nil
}

// AnalyzeBatch 批量分析多个章节
func (a *ChapterAnalysisAgent) AnalyzeBatch(ctx context.Context, inputs []ChapterAnalysisInput) ([]ChapterAnalysisOutput, error) {
	logger.Section("CHAPTER ANALYSIS AGENT - Batch Analysis")
	logger.Info("Total chapters to analyze: %d", len(inputs))

	outputs := make([]ChapterAnalysisOutput, 0, len(inputs))

	for i, input := range inputs {
		logger.Info("[%d/%d] Analyzing %s...", i+1, len(inputs), input.ChapterID)

		output, err := a.Analyze(ctx, input)
		if err != nil {
			logger.Error("Failed to analyze chapter %s: %v", input.ChapterID, err)
			// 继续分析其他章节，不中断
			continue
		}

		outputs = append(outputs, output)
	}

	logger.Info("Batch analysis completed: %d/%d chapters successful", len(outputs), len(inputs))
	return outputs, nil
}

// CrossChapterAnalysis 跨章分析（检测一致性问题）
func (a *ChapterAnalysisAgent) CrossChapterAnalysis(ctx context.Context, outputs []ChapterAnalysisOutput) ([]ChapterIssue, error) {
	logger.Section("CHAPTER ANALYSIS AGENT - Cross-Chapter Analysis")

	// 构建跨章分析输入
	input := struct {
		Chapters []ChapterSummary `json:"chapters"`
	}{
		Chapters: make([]ChapterSummary, 0, len(outputs)),
	}

	for _, out := range outputs {
		summary := ChapterSummary{
			Characters:   make([]string, 0, len(out.Characters)),
			PowerChanges: out.PowerChanges,
			Timeline:     out.TimelineInfo,
		}
		for _, char := range out.Characters {
			summary.Characters = append(summary.Characters, char.Name)
		}
		input.Chapters = append(input.Chapters, summary)
	}

	type CrossChapterOutput struct {
		Issues []ChapterIssue `json:"issues"`
	}

	var output CrossChapterOutput
	params := InvokeParams{
		Skills:  []string{"cross-chapter-analysis"},
		Command: "analyze cross-chapter consistency",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return nil, err
	}

	logger.Info("Cross-chapter analysis completed: %d issues found", len(output.Issues))
	return output.Issues, nil
}

// ChapterSummary 章节摘要（用于跨章分析）
type ChapterSummary struct {
	Characters   []string          `json:"characters"`
	PowerChanges []PowerChangeInfo `json:"power_changes"`
	Timeline     TimelineInfo      `json:"timeline"`
}

// validateOutput 验证和清理输出
func (a *ChapterAnalysisAgent) validateOutput(output *ChapterAnalysisOutput) {
	// 确保严重性级别有效
	validSeverities := map[string]bool{
		"critical": true, "error": true, "warning": true, "info": true,
	}

	for i := range output.Issues {
		if !validSeverities[output.Issues[i].Severity] {
			output.Issues[i].Severity = "warning"
		}
	}

	// 确保角色状态有效
	validStates := map[string]bool{
		"alive": true, "dead": true, "injured": true, "unconscious": true,
	}

	for i := range output.Characters {
		if !validStates[output.Characters[i].State] {
			output.Characters[i].State = "alive"
		}
	}

	// 如果摘要为空，生成一个简单的
	if output.Summary == "" && len(output.Events) > 0 {
		output.Summary = fmt.Sprintf("本章包含 %d 个事件，%d 个角色出场",
			len(output.Events), len(output.Characters))
	}
}
