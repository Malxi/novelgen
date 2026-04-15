package rpg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// ComposeBridge 连接Compose生成和RPG验证的桥梁
type ComposeBridge struct {
	ProjectPath string
	BookName    string
}

// NewComposeBridge 创建ComposeBridge
func NewComposeBridge(projectPath, bookName string) *ComposeBridge {
	return &ComposeBridge{
		ProjectPath: projectPath,
		BookName:    bookName,
	}
}

// ValidateOutline 验证大纲
func (cb *ComposeBridge) ValidateOutline() (*RPGCheckResult, error) {
	// 加载项目
	project, err := LoadNovelgenProject(cb.ProjectPath, cb.BookName)
	if err != nil {
		return nil, fmt.Errorf("加载项目失败: %v", err)
	}

	// 创建RPG检测器
	checker := NewOutlineRPGChecker()
	result := checker.Check(&project.Outline)

	return result, nil
}

// ValidateAndReport 验证并生成报告
func (cb *ComposeBridge) ValidateAndReport() (*ValidationReport, error) {
	result, err := cb.ValidateOutline()
	if err != nil {
		return nil, err
	}

	return &ValidationReport{
		Result:      *result,
		Suggestions: cb.generateSuggestions(result),
	}, nil
}

// generateSuggestions 生成建议
func (cb *ComposeBridge) generateSuggestions(result *RPGCheckResult) []Suggestion {
	return GenerateSuggestionsFromResult(result)
}

// GenerateSuggestionsFromResult 从验证结果生成建议（导出函数）
func GenerateSuggestionsFromResult(result *RPGCheckResult) []Suggestion {
	return generateSuggestionsFromResult(result)
}

// generateSuggestionsFromResult 内部实现
func generateSuggestionsFromResult(result *RPGCheckResult) []Suggestion {
	var suggestions []Suggestion

	// 基于Debuff生成建议
	for _, debuff := range result.Debuffs {
		switch debuff.Name {
		case "战力崩坏":
			suggestions = append(suggestions, Suggestion{
				Type:     "error",
				Category: "system",
				Message:  "战力系统变化过于频繁，读者难以建立稳定预期",
				Action:   "设定明确的升级规则，减少频繁的境界变化",
				Priority: debuff.Severity,
			})
		case "时间混乱":
			suggestions = append(suggestions, Suggestion{
				Type:     "warning",
				Category: "plot",
				Message:  "时间跳跃过于频繁，缺乏过渡",
				Action:   "增加时间过渡描述，让读者跟上节奏",
				Priority: debuff.Severity,
			})
		case "角色分散":
			suggestions = append(suggestions, Suggestion{
				Type:     "warning",
				Category: "character",
				Message:  "角色数量过多或分布不均",
				Action:   "合并相似角色，或给次要角色更多戏份",
				Priority: debuff.Severity,
			})
		}
	}

	// 基于分数生成建议
	if result.Stats.StructureIntegrity < 70 {
		suggestions = append(suggestions, Suggestion{
			Type:     "error",
			Category: "structure",
			Message:  "结构完整性不足，部分章节缺少必要信息",
			Action:   "完善章节的OpeningBeat、ClosingBeat、StateChange等字段",
			Priority: 8,
		})
	}

	if result.Stats.LogicConsistency < 70 {
		suggestions = append(suggestions, Suggestion{
			Type:     "error",
			Category: "logic",
			Message:  "逻辑一致性不足，存在角色出场不一致或事件矛盾",
			Action:   "检查角色是否在不合适的章节出现，事件因果关系是否清晰",
			Priority: 9,
		})
	}

	if result.Stats.CharacterBalance < 60 {
		suggestions = append(suggestions, Suggestion{
			Type:     "warning",
			Category: "character",
			Message:  "角色平衡性不足，主角出场率可能过高或过低",
			Action:   "调整主角出场频率，确保在30%-70%之间",
			Priority: 6,
		})
	}

	if result.Stats.PacingQuality < 60 {
		suggestions = append(suggestions, Suggestion{
			Type:     "warning",
			Category: "plot",
			Message:  "节奏质量不足，快慢节奏比例可能失衡",
			Action:   "调整章节节奏，快:慢比例建议在 3:7 到 5:5 之间",
			Priority: 5,
		})
	}

	// 如果总体评分高，给出正面反馈
	if result.TotalScore >= 80 {
		suggestions = append(suggestions, Suggestion{
			Type:     "info",
			Category: "general",
			Message:  "大纲质量优秀，可以直接进入写作阶段",
			Action:   "继续保持，关注细节完善",
			Priority: 1,
		})
	}

	return suggestions
}

// ExportValidationReport 导出验证报告
func (cb *ComposeBridge) ExportValidationReport(report *ValidationReport, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化报告失败: %v", err)
	}

	return ioutil.WriteFile(outputPath, data, 0644)
}

// QuickValidate 快速验证（不加载完整项目）
func QuickValidate(outlinePath string) (*RPGCheckResult, error) {
	// 读取大纲
	data, err := ioutil.ReadFile(outlinePath)
	if err != nil {
		return nil, fmt.Errorf("读取大纲失败: %v", err)
	}

	var outline StoryOutline
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, fmt.Errorf("解析大纲失败: %v", err)
	}

	// 创建检测器
	checker := NewOutlineRPGChecker()
	result := checker.Check(&outline)

	return result, nil
}

// ValidateOutlineFile 验证大纲文件并返回详细报告
func ValidateOutlineFile(outlinePath string) (*ValidationReport, error) {
	result, err := QuickValidate(outlinePath)
	if err != nil {
		return nil, err
	}

	// 创建bridge来生成建议
	bridge := &ComposeBridge{}

	return &ValidationReport{
		Result:      *result,
		Suggestions: bridge.generateSuggestions(result),
	}, nil
}

// PrintValidationSummary 打印验证摘要
func PrintValidationSummary(report *ValidationReport) {
	result := report.Result

	fmt.Println("\n========== RPG大纲验证结果 ==========")
	fmt.Printf("总评分: %d/100 (等级: %s)\n", result.TotalScore, result.Grade)
	fmt.Printf("诊断: %s\n\n", result.Diagnosis)

	fmt.Println("【基础属性】")
	fmt.Printf("  结构完整性: %d\n", result.Stats.StructureIntegrity)
	fmt.Printf("  逻辑一致性: %d\n", result.Stats.LogicConsistency)
	fmt.Printf("  角色平衡性: %d\n", result.Stats.CharacterBalance)
	fmt.Printf("  剧情连贯性: %d\n", result.Stats.PlotCoherence)
	fmt.Printf("  节奏质量: %d\n", result.Stats.PacingQuality)

	if len(result.Debuffs) > 0 {
		fmt.Println("\n【负面状态】")
		for _, debuff := range result.Debuffs {
			fmt.Printf("  ⚠️ %s (严重度: %d): %s\n", debuff.Name, debuff.Severity, debuff.Description)
		}
	}

	if len(result.Bosses) > 0 {
		fmt.Println("\n【BOSS级问题】")
		for _, boss := range result.Bosses {
			fmt.Printf("  👹 %s: %s\n", boss.Name, boss.Description)
			fmt.Printf("      弱点: %v\n", boss.Weaknesses)
		}
	}

	if len(report.Suggestions) > 0 {
		fmt.Println("\n【改进建议】")
		for _, suggestion := range report.Suggestions {
			typeIcon := "ℹ️"
			if suggestion.Type == "warning" {
				typeIcon = "⚠️"
			} else if suggestion.Type == "error" {
				typeIcon = "❌"
			}
			fmt.Printf("  %s [%s] %s\n", typeIcon, suggestion.Category, suggestion.Message)
			fmt.Printf("      建议: %s (优先级: %d)\n", suggestion.Action, suggestion.Priority)
		}
	}

	fmt.Println("\n=====================================")
}
