package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
	"novelgen/internal/rpg/benchmark"
)

// AIRPGPipeline AI RPG 完整流程管道
// chapters -> AI Agent -> RPG DSL -> RPG Simulate -> Issues
type AIRPGPipeline struct {
	analysisAgent *ChapterAnalysisAgent
	llmClient     llm.Client
	llmConfig     *llm.Config
	projectLLM    *models.ProjectLLM
}

// PipelineResult 管道执行结果
type PipelineResult struct {
	ChapterID          string                        `json:"chapter_id"`
	ChapterTitle       string                        `json:"chapter_title"`
	AnalysisOutput     ChapterAnalysisOutput         `json:"analysis_output"`
	RPGData            *rpg.NovelRPGData             `json:"rpg_data"`
	SimulationReport   *rpg.SimulationReport         `json:"simulation_report"`
	ValidationIssues   []ValidationIssue             `json:"validation_issues"`
	CrossChapterIssues []benchmark.CrossChapterIssue `json:"cross_chapter_issues,omitempty"`
	ProcessingTime     time.Duration                 `json:"processing_time"`
}

// ValidationIssue 验证发现的问题
type ValidationIssue struct {
	Category    string `json:"category"`    // power, timeline, character, plot, economy
	Severity    string `json:"severity"`    // critical, error, warning
	Target      string `json:"target"`      // 问题对象
	Description string `json:"description"` // 问题描述
	Evidence    string `json:"evidence"`    // 证据
	Suggestion  string `json:"suggestion"`  // 建议
}

// ChapterInput 章节输入（用于批量处理）
type ChapterInput struct {
	ID      string
	Title   string
	Content string
}

// NewAIRPGPipeline 创建 AI RPG 管道
func NewAIRPGPipeline(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *AIRPGPipeline {
	return &AIRPGPipeline{
		analysisAgent: NewChapterAnalysisAgent(client, config, projectLLM),
		llmClient:     client,
		llmConfig:     config,
		projectLLM:    projectLLM,
	}
}

// ProcessChapter 处理单个章节
func (p *AIRPGPipeline) ProcessChapter(ctx context.Context, chapterID, chapterTitle, content string) (*PipelineResult, error) {
	start := time.Now()

	// Step 1: AI Agent 分析章节
	fmt.Printf("🤖 Step 1: AI Agent 分析章节 %s...\n", chapterID)

	analysisInput := ChapterAnalysisInput{
		ChapterID:      chapterID,
		ChapterTitle:   chapterTitle,
		ChapterContent: content,
		StorySetup:     "主角有复活金手指，每次复活掉一层修为",
	}

	analysisOutput, err := p.analysisAgent.Analyze(ctx, analysisInput)
	if err != nil {
		return nil, fmt.Errorf("AI分析失败: %w", err)
	}

	fmt.Printf("   ✓ 提取到 %d 个角色, %d 个事件, %d 个问题\n",
		len(analysisOutput.Characters),
		len(analysisOutput.Events),
		len(analysisOutput.Issues))

	// Step 2: 转换为 RPG DSL
	fmt.Printf("🔄 Step 2: 转换为 RPG DSL...\n")

	rpgData := p.convertToRPGData(&analysisOutput)

	fmt.Printf("   ✓ 生成 RPG 数据: %d 角色, %d 物品\n",
		len(rpgData.Characters),
		len(rpgData.Items))

	// Step 3: RPG 模拟验证
	fmt.Printf("🎮 Step 3: RPG 模拟验证...\n")

	simulator := rpg.NewNovelSimulator(rpgData)
	simReport := simulator.Simulate()

	fmt.Printf("   ✓ 模拟完成: %d 条日志, %d 个验证结果\n",
		len(simReport.SimulationLog),
		len(simReport.ValidationResults))

	// Step 4: 整合问题
	fmt.Printf("🔍 Step 4: 整合验证问题...\n")

	validationIssues := p.aggregateIssues(&analysisOutput, simReport)

	fmt.Printf("   ✓ 发现 %d 个验证问题\n", len(validationIssues))

	result := &PipelineResult{
		ChapterID:        chapterID,
		ChapterTitle:     chapterTitle,
		AnalysisOutput:   analysisOutput,
		RPGData:          rpgData,
		SimulationReport: simReport,
		ValidationIssues: validationIssues,
		ProcessingTime:   time.Since(start),
	}

	return result, nil
}

// ProcessChaptersBatch 批量处理多个章节
func (p *AIRPGPipeline) ProcessChaptersBatch(ctx context.Context, chapters []ChapterInput) ([]*PipelineResult, error) {
	fmt.Printf("🚀 开始批量处理 %d 个章节...\n\n", len(chapters))

	results := make([]*PipelineResult, 0, len(chapters))

	for i, chapter := range chapters {
		fmt.Printf("[%d/%d] 处理章节 %s...\n", i+1, len(chapters), chapter.ID)

		result, err := p.ProcessChapter(ctx, chapter.ID, chapter.Title, chapter.Content)
		if err != nil {
			fmt.Printf("   ❌ 失败: %v\n", err)
			continue
		}

		results = append(results, result)
		fmt.Printf("   ✅ 完成 (%s)\n\n", result.ProcessingTime)
	}

	// 跨章一致性检测
	fmt.Printf("🔗 执行跨章一致性检测...\n")
	crossChapterIssues := p.detectCrossChapterIssues(results)

	for _, result := range results {
		result.CrossChapterIssues = crossChapterIssues
	}

	fmt.Printf("✅ 批量处理完成: %d/%d 章节成功\n", len(results), len(chapters))

	return results, nil
}

// convertToRPGData 将 AI 分析结果转换为 RPG DSL
func (p *AIRPGPipeline) convertToRPGData(analysis *ChapterAnalysisOutput) *rpg.NovelRPGData {
	rpgData := &rpg.NovelRPGData{
		Characters: make([]*rpg.CharacterTemplate, 0),
		Items:      make([]*rpg.Item, 0),
		Skills:     make([]*rpg.Skill, 0),
		Locations:  make([]*rpg.Map, 0),
	}

	// 转换角色
	for _, charInfo := range analysis.Characters {
		charTemplate := &rpg.CharacterTemplate{
			Name:        charInfo.Name,
			Description: fmt.Sprintf("%s，状态：%s", charInfo.Name, charInfo.State),
			BaseStats: rpg.BaseStats{
				HP:      charInfo.MaxHP,
				MP:      100,
				Attack:  10,
				Defense: 10,
			},
		}

		// 从修为解析等级
		if charInfo.Cultivation != "" {
			// 简单解析等级
			if level := parseLevelFromCultivation(charInfo.Cultivation); level > 0 {
				// 等级信息可以存储在 Description 中
				charTemplate.Description = fmt.Sprintf("%s，修为：%s", charTemplate.Description, charInfo.Cultivation)
			}
		}

		rpgData.Characters = append(rpgData.Characters, charTemplate)
	}

	// 从事件中提取物品
	itemSet := make(map[string]bool)
	for _, event := range analysis.Events {
		// 简单提取物品关键词
		itemKeywords := []string{"灵石", "丹药", "法宝", "武器", "秘籍", "灵草"}
		for _, keyword := range itemKeywords {
			if strings.Contains(event.Description, keyword) && !itemSet[keyword] {
				itemSet[keyword] = true
				item := &rpg.Item{
					Name:        keyword,
					Description: fmt.Sprintf("从事件'%s'中提取的物品", event.Description),
					Type:        "misc",
				}
				rpgData.Items = append(rpgData.Items, item)
			}
		}
	}

	// 从事件中提取地点
	locationSet := make(map[string]bool)
	for _, event := range analysis.Events {
		if event.Location != "" && !locationSet[event.Location] {
			locationSet[event.Location] = true
			location := &rpg.Map{
				Name:        event.Location,
				Description: event.Location,
			}
			rpgData.Locations = append(rpgData.Locations, location)
		}
	}

	return rpgData
}

// aggregateIssues 整合 AI 问题和模拟问题
func (p *AIRPGPipeline) aggregateIssues(analysis *ChapterAnalysisOutput, simReport *rpg.SimulationReport) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// 转换 AI 检测到的问题
	for _, aiIssue := range analysis.Issues {
		// 跳过 info 级别的问题（如设定说明）
		if aiIssue.Severity == "info" {
			continue
		}

		issue := ValidationIssue{
			Category:    aiIssue.Category,
			Severity:    aiIssue.Severity,
			Target:      aiIssue.Target,
			Description: aiIssue.Description,
			Evidence:    aiIssue.Evidence,
			Suggestion:  aiIssue.Suggestion,
		}
		issues = append(issues, issue)
	}

	// 转换模拟器验证结果
	for _, result := range simReport.ValidationResults {
		if !result.Passed {
			for _, issueDetail := range result.Issues {
				issue := ValidationIssue{
					Category:    result.Category,
					Severity:    "error",
					Target:      result.Category,
					Description: issueDetail,
					Evidence:    fmt.Sprintf("验证类别: %s", result.Category),
					Suggestion:  "请参考验证规则修复",
				}
				issues = append(issues, issue)
			}
		}
	}

	// 添加战力变化检测
	for _, change := range analysis.PowerChanges {
		if !change.IsLegitimate {
			issue := ValidationIssue{
				Category:    "power",
				Severity:    "warning",
				Target:      change.Character,
				Description: fmt.Sprintf("不合理的战力变化: %s -> %s", change.FromLevel, change.ToLevel),
				Evidence:    change.Reason,
				Suggestion:  "检查战力变化的合理性，确保有充分铺垫",
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// detectCrossChapterIssues 检测跨章问题
func (p *AIRPGPipeline) detectCrossChapterIssues(results []*PipelineResult) []benchmark.CrossChapterIssue {
	issues := make([]benchmark.CrossChapterIssue, 0)

	if len(results) < 2 {
		return issues
	}

	// 简单的跨章检测逻辑
	characterStates := make(map[string]*CharacterInfo)

	for _, result := range results {
		for _, char := range result.AnalysisOutput.Characters {
			if prevState, exists := characterStates[char.Name]; exists {
				// 检查死亡后复活
				if prevState.State == "dead" && char.State == "alive" && !char.IsResurrected {
					issues = append(issues, benchmark.CrossChapterIssue{
						Type:        "inconsistency",
						Category:    "character",
						Severity:    "critical",
						ChapterFrom: result.ChapterID,
						ChapterTo:   result.ChapterID,
						Target:      char.Name,
						Description: fmt.Sprintf("角色 %s 死亡后无复活说明再次出现", char.Name),
					})
				}
			}

			// 更新状态
			characterStates[char.Name] = &char
		}
	}

	return issues
}

// FormatPipelineReport 格式化管道报告
func (p *AIRPGPipeline) FormatPipelineReport(result *PipelineResult) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║           AI RPG Pipeline 分析报告                        ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n\n")

	sb.WriteString(fmt.Sprintf("📖 章节: %s - %s\n", result.ChapterID, result.ChapterTitle))
	sb.WriteString(fmt.Sprintf("⏱️  处理时间: %s\n\n", result.ProcessingTime))

	// AI 分析结果
	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│  Step 1: AI Agent 分析                                   │\n")
	sb.WriteString(fmt.Sprintf("│  • 角色: %d 个                                          │\n", len(result.AnalysisOutput.Characters)))
	sb.WriteString(fmt.Sprintf("│  • 事件: %d 个                                          │\n", len(result.AnalysisOutput.Events)))
	sb.WriteString(fmt.Sprintf("│  • 战力变化: %d 次                                      │\n", len(result.AnalysisOutput.PowerChanges)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// RPG DSL
	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│  Step 2: RPG DSL 转换                                    │\n")
	sb.WriteString(fmt.Sprintf("│  • 角色模板: %d 个                                      │\n", len(result.RPGData.Characters)))
	sb.WriteString(fmt.Sprintf("│  • 物品: %d 个                                          │\n", len(result.RPGData.Items)))
	sb.WriteString(fmt.Sprintf("│  • 地点: %d 个                                          │\n", len(result.RPGData.Locations)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// 模拟结果
	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│  Step 3: RPG 模拟验证                                    │\n")
	sb.WriteString(fmt.Sprintf("│  • 模拟日志: %d 条                                      │\n", len(result.SimulationReport.SimulationLog)))
	sb.WriteString(fmt.Sprintf("│  • 验证通过: %d/%d                                      │\n",
		countPassedValidations(result.SimulationReport.ValidationResults),
		len(result.SimulationReport.ValidationResults)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// 问题汇总
	if len(result.ValidationIssues) > 0 {
		sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
		sb.WriteString("│  Step 4: 发现的问题                                      │\n")
		sb.WriteString("├─────────────────────────────────────────────────────────┤\n")

		for _, issue := range result.ValidationIssues {
			severityEmoji := "⚠️"
			if issue.Severity == "critical" {
				severityEmoji = "🔴"
			} else if issue.Severity == "error" {
				severityEmoji = "🟠"
			}
			sb.WriteString(fmt.Sprintf("│ %s [%s] %s: %s\n", severityEmoji, issue.Category, issue.Target, issue.Description))
		}
		sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")
	} else {
		sb.WriteString("✅ 未发现问题\n\n")
	}

	return sb.String()
}

// Helper functions

func parseLevelFromCultivation(cultivation string) int {
	// 简单解析修为等级
	levels := map[string]int{
		"练气": 1, "筑基": 2, "金丹": 3, "元婴": 4,
		"化神": 5, "合体": 6, "大乘": 7, "渡劫": 8,
	}
	for name, level := range levels {
		if strings.Contains(cultivation, name) {
			return level
		}
	}
	return 0
}

func countPassedValidations(validations []rpg.SimulatorValidationResult) int {
	count := 0
	for _, v := range validations {
		if v.Passed {
			count++
		}
	}
	return count
}
