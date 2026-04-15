package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// RPGEnhancedWriteAgent 集成RPG约束的写作Agent
type RPGEnhancedWriteAgent struct {
	base             *BaseAgent
	setup            *models.StorySetup
	outline          *models.Outline
	constraintSystem *rpg.ConstraintSystem
	rpgWorld         *rpg.GameWorld
	constraintReport *rpg.ConstraintReport
}

// NewRPGEnhancedWriteAgent 创建RPG增强的写作Agent
func NewRPGEnhancedWriteAgent(
	client llm.Client,
	config *llm.Config,
	projectLLM *models.ProjectLLM,
	setup *models.StorySetup,
	outline *models.Outline,
	projectPath string,
	bookName string,
) (*RPGEnhancedWriteAgent, error) {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "RPGEnhancedWriteAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	// 加载RPG世界数据
	project, err := rpg.LoadNovelgenProject(projectPath, bookName)
	if err != nil {
		logger.Warn("Failed to load RPG project: %v", err)
		// 继续运行，只是没有RPG约束
		return &RPGEnhancedWriteAgent{
			base:    base,
			setup:   setup,
			outline: outline,
		}, nil
	}

	world, err := project.ConvertToRPG()
	if err != nil {
		logger.Warn("Failed to convert to RPG world: %v", err)
		return &RPGEnhancedWriteAgent{
			base:    base,
			setup:   setup,
			outline: outline,
		}, nil
	}

	// 创建约束系统
	constraintSystem := rpg.NewConstraintSystem(world)
	constraintReport := constraintSystem.BuildFromRPGData()

	return &RPGEnhancedWriteAgent{
		base:             base,
		setup:            setup,
		outline:          outline,
		constraintSystem: constraintSystem,
		rpgWorld:         world,
		constraintReport: constraintReport,
	}, nil
}

// SetLanguage 设置输出语言
func (a *RPGEnhancedWriteAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// RPGWriteGenInput RPG增强的写作输入
type RPGWriteGenInput struct {
	StorySetup        CompactStorySetup        `json:"story_setup" md:"story_setup"`
	Chapter           models.Chapter           `json:"chapter" md:"chapter"`
	StateMatrix       string                   `json:"state_matrix" md:"state_matrix"`
	TargetWords       int                      `json:"target_words" md:"target_words"`
	Context           string                   `json:"context,omitempty" md:"context,omitempty"`
	Recap             string                   `json:"recap,omitempty" md:"recap,omitempty"`
	NextChapters      []NextChapterInfo        `json:"next_chapters,omitempty" md:"next_chapters,omitempty"`
	RPGConstraints    string                   `json:"rpg_constraints" md:"rpg_constraints"`
	CharacterProfiles []RPGCharacterProfile    `json:"character_profiles,omitempty" md:"character_profiles,omitempty"`
	WorldState        *RPGWorldState           `json:"world_state,omitempty" md:"world_state,omitempty"`
}

// RPGCharacterProfile RPG角色档案
type RPGCharacterProfile struct {
	Name        string            `json:"name"`
	Level       int               `json:"level"`
	HP          int               `json:"hp"`
	MP          int               `json:"mp"`
	Attack      int               `json:"attack"`
	Defense     int               `json:"defense"`
	Skills      []string          `json:"skills"`
	Status      string            `json:"status"`
	Constraints map[string]string `json:"constraints"`
}

// RPGWorldState RPG世界状态
type RPGWorldState struct {
	CurrentLocation string            `json:"current_location"`
	TimeOfDay       string            `json:"time_of_day"`
	Weather         string            `json:"weather"`
	ActiveQuests    []string          `json:"active_quests"`
	WorldEvents     []string          `json:"world_events"`
	PowerSystem     *rpg.PowerSystemConstraint `json:"power_system,omitempty"`
}

// RPGWriteGenOutput RPG增强的写作输出
type RPGWriteGenOutput struct {
	Content            string                     `json:"content" md:"content"`
	ConstraintViolations []rpg.ConstraintViolation `json:"violations,omitempty"`
	RPGStateChanges    map[string]interface{}     `json:"rpg_state_changes,omitempty"`
}

// GenerateChapter 生成章节内容（带RPG约束）
func (a *RPGEnhancedWriteAgent) GenerateChapter(
	ctx context.Context,
	chapter *models.Chapter,
	context *ChapterContext,
	state *models.StateMatrix,
	targetWords int,
) (string, error) {
	logger.Section("RPG ENHANCED WRITE AGENT - Chapter Generation")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)

	// 转换下一章信息
	var nextInfos []NextChapterInfo
	for _, nc := range context.Next {
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.Chapter.ID,
			Title:   nc.Chapter.Title,
			Summary: nc.Chapter.Summary,
		})
	}

	// 构建RPG约束提示词
	rpgConstraints := ""
	if a.constraintReport != nil {
		rpgConstraints = a.constraintSystem.ToPromptFormat(a.constraintReport)
	}

	// 构建角色档案
	characterProfiles := a.buildCharacterProfiles(chapter)

	// 构建世界状态
	worldState := a.buildWorldState(chapter)

	input := RPGWriteGenInput{
		StorySetup:        ToCompact(a.setup),
		Chapter:           *chapter,
		StateMatrix:       formatStateMatrixForWrite(state, chapter),
		TargetWords:       targetWords,
		Context:           formatChapterContext(context),
		Recap:             context.Recap,
		NextChapters:      nextInfos,
		RPGConstraints:    rpgConstraints,
		CharacterProfiles: characterProfiles,
		WorldState:        worldState,
	}

	var output RPGWriteGenOutput
	params := InvokeParams{
		Skills:  []string{"rpg-write-generate"},
		Command: "generate chapter with RPG constraints",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// 验证内容是否符合RPG约束
	if a.constraintSystem != nil {
		violations := a.constraintSystem.ValidateChapter(chapter.ID, output.Content)
		if len(violations) > 0 {
			logger.Warn("RPG constraint violations detected: %d", len(violations))
			for _, v := range violations {
				logger.Warn("  - %s: %s", v.Target, v.Issue)
			}
			
			// 尝试修正
			corrected, err := a.correctViolations(ctx, chapter, context, state, targetWords, output.Content, violations)
			if err != nil {
				logger.Warn("Failed to correct violations: %v", err)
			} else {
				output.Content = corrected
			}
		}
	}

	// 验证输出内容
	if strings.TrimSpace(output.Content) == "" {
		return "", fmt.Errorf("AI returned empty content for chapter %s", chapter.ID)
	}

	// 记录上下文
	if err := a.logWriteContext(chapter.ID, "rpg_final", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated RPG-enhanced chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// buildCharacterProfiles 构建角色档案
func (a *RPGEnhancedWriteAgent) buildCharacterProfiles(chapter *models.Chapter) []RPGCharacterProfile {
	profiles := make([]RPGCharacterProfile, 0)

	if a.rpgWorld == nil {
		return profiles
	}

	for _, charName := range chapter.Characters {
		char := a.rpgWorld.GetCharacterByName(charName)
		if char == nil {
			continue
		}

		profile := RPGCharacterProfile{
			Name:    char.Name,
			Level:   char.Level,
			HP:      char.CurrentStats.HP,
			MP:      char.CurrentStats.MP,
			Attack:  char.CurrentStats.Attack,
			Defense: char.CurrentStats.Defense,
			Skills:  char.Skills,
			Status:  string(char.State),
			Constraints: make(map[string]string),
		}

		// 添加约束信息
		if constraint, exists := a.constraintSystem.CharacterRules[char.Name]; exists {
			profile.Constraints["max_deaths"] = fmt.Sprintf("%d", constraint.MaxDeaths)
			profile.Constraints["max_resurrections"] = fmt.Sprintf("%d", constraint.MaxResurrections)
			profile.Constraints["power_change_rate"] = fmt.Sprintf("%.0f%%", constraint.PowerChangeRate*100)
		}

		profiles = append(profiles, profile)
	}

	return profiles
}

// buildWorldState 构建世界状态
func (a *RPGEnhancedWriteAgent) buildWorldState(chapter *models.Chapter) *RPGWorldState {
	if a.rpgWorld == nil {
		return nil
	}

	state := &RPGWorldState{
		CurrentLocation: chapter.Location,
		TimeOfDay:       "unknown",
		Weather:         "normal",
		ActiveQuests:    make([]string, 0),
		WorldEvents:     make([]string, 0),
	}

	// 添加战力系统约束
	if a.constraintReport != nil {
		state.PowerSystem = a.constraintReport.PowerConstraints
	}

	return state
}

// correctViolations 修正约束违反
func (a *RPGEnhancedWriteAgent) correctViolations(
	ctx context.Context,
	chapter *models.Chapter,
	context *ChapterContext,
	state *models.StateMatrix,
	targetWords int,
	currentContent string,
	violations []rpg.ConstraintViolation,
) (string, error) {
	logger.Section("RPG CONSTRAINT CORRECTION")
	logger.Info("Attempting to correct %d violations", len(violations))

	correctionPrompt := a.constraintSystem.GenerateCorrectionPrompt(violations)

	input := RPGWriteCorrectInput{
		StorySetup:     ToCompact(a.setup),
		Chapter:        *chapter,
		StateMatrix:    formatStateMatrixForWrite(state, chapter),
		TargetWords:    targetWords,
		CurrentContent: currentContent,
		Corrections:    correctionPrompt,
		Context:        formatChapterContext(context),
		Recap:          context.Recap,
	}

	var output RPGWriteCorrectOutput
	params := InvokeParams{
		Skills:  []string{"rpg-write-correct"},
		Command: "correct RPG constraint violations",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	logger.Info("✓ Corrected content: %d characters", len(output.Content))
	return output.Content, nil
}

// RPGWriteCorrectInput 修正输入
type RPGWriteCorrectInput struct {
	StorySetup     CompactStorySetup `json:"story_setup"`
	Chapter        models.Chapter    `json:"chapter"`
	StateMatrix    string            `json:"state_matrix"`
	TargetWords    int               `json:"target_words"`
	CurrentContent string            `json:"current_content"`
	Corrections    string            `json:"corrections"`
	Context        string            `json:"context,omitempty"`
	Recap          string            `json:"recap,omitempty"`
}

// RPGWriteCorrectOutput 修正输出
type RPGWriteCorrectOutput struct {
	Content string `json:"content"`
}

// ReviewChapterWithRPG RPG增强的章节评审
func (a *RPGEnhancedWriteAgent) ReviewChapterWithRPG(
	ctx context.Context,
	chapter *models.Chapter,
	context *ChapterContext,
	state *models.StateMatrix,
	content string,
	targetWords int,
	iteration int,
) (models.ReviewResult, error) {
	logger.Section("RPG ENHANCED REVIEW")

	// 先进行普通评审
	recap := ""
	if context != nil {
		recap = context.Recap
	}

	input := WriteReviewInput{
		StorySetup:     ToCompact(a.setup),
		StateMatrix:    formatStateMatrixForWrite(state, chapter),
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		Iteration:      iteration,
		Recap:          recap,
	}

	var output WriteReviewOutput
	params := InvokeParams{
		Skills:  []string{"write-review"},
		Command: "review chapter",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return models.ReviewResult{}, err
	}

	// 添加RPG约束检查
	if a.constraintSystem != nil {
		violations := a.constraintSystem.ValidateChapter(chapter.ID, content)
		if len(violations) > 0 {
			// 将约束违反添加到评审结果
			for _, v := range violations {
				suggestion := models.ReviewSuggestion{
					Category:   "rpg_constraint",
					Issue:      v.Issue,
					Suggestion: v.Suggestion,
					Priority:   a.mapSeverityToPriority(v.Severity),
				}
				output.Result.Suggestions = append(output.Result.Suggestions, suggestion)
			}

			// 降低总体分数
			penalty := len(violations) * 5
			if penalty > 20 {
				penalty = 20
			}
			output.Result.OverallScore -= float64(penalty)
			if output.Result.OverallScore < 0 {
				output.Result.OverallScore = 0
			}
		}
	}

	logger.Section("RPG Enhanced Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// mapSeverityToPriority 映射严重程度到优先级
func (a *RPGEnhancedWriteAgent) mapSeverityToPriority(severity string) string {
	switch severity {
	case "critical":
		return "high"
	case "error":
		return "high"
	case "warning":
		return "medium"
	default:
		return "low"
	}
}

// IterateChapterWithRPG RPG增强的迭代写作
func (a *RPGEnhancedWriteAgent) IterateChapterWithRPG(
	ctx context.Context,
	chapter *models.Chapter,
	context *ChapterContext,
	state *models.StateMatrix,
	targetWords int,
	initialContent string,
	maxIterations int,
	qualityThreshold float64,
) (string, *models.ReviewResult, error) {
	logger.Section("RPG ENHANCED ITERATION LOOP")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentContent := initialContent
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== RPG Enhanced Iteration %d/%d ===", i, maxIterations)

		// RPG增强的评审
		review, err := a.ReviewChapterWithRPG(ctx, chapter, context, state, currentContent, targetWords, i)
		if err != nil {
			return "", nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}
		finalReview = &review

		// 检查是否满足质量阈值
		if review.OverallScore >= qualityThreshold {
			// 检查是否有严重的RPG约束违反
			hasCriticalRPGViolation := false
			for _, s := range review.Suggestions {
				if s.Category == "rpg_constraint" && s.Priority == "high" {
					hasCriticalRPGViolation = true
					break
				}
			}

			if !hasCriticalRPGViolation {
				logger.Info("✓ Quality threshold met (%.1f >= %.1f) and no critical RPG violations", 
					review.OverallScore, qualityThreshold)
				break
			}
			logger.Info("Quality threshold met but critical RPG violations exist, continuing iteration")
		}

		// 检查是否达到最大迭代次数
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// 检查是否有高优先级建议
		hasHighPriority := false
		for _, s := range review.Suggestions {
			if s.Priority == "high" {
				hasHighPriority = true
				break
			}
		}
		if !hasHighPriority {
			logger.Info("No high priority issues, stopping iteration")
			break
		}

		// 改进
		suggestions := formatWriteSuggestions(review.Suggestions)
		improved, err := a.GenerateChapterWithRPGSuggestions(ctx, chapter, context, state, targetWords, currentContent, suggestions)
		if err != nil {
			return "", nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentContent = improved
	}

	return currentContent, finalReview, nil
}

// GenerateChapterWithRPGSuggestions 根据RPG建议改进章节
func (a *RPGEnhancedWriteAgent) GenerateChapterWithRPGSuggestions(
	ctx context.Context,
	chapter *models.Chapter,
	context *ChapterContext,
	state *models.StateMatrix,
	targetWords int,
	currentDraft string,
	suggestions string,
) (string, error) {
	logger.Section("RPG ENHANCED IMPROVEMENT")

	input := RPGWriteImproveInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrixForWrite(state, chapter),
		TargetWords:  targetWords,
		CurrentDraft: currentDraft,
		Suggestions:  suggestions,
		Context:      formatChapterContext(context),
		Recap:        context.Recap,
	}

	// 添加RPG约束
	if a.constraintReport != nil {
		input.RPGConstraints = a.constraintSystem.ToPromptFormat(a.constraintReport)
	}

	var output RPGWriteImproveOutput
	params := InvokeParams{
		Skills:  []string{"rpg-write-improve"},
		Command: "improve chapter with RPG constraints",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	if strings.TrimSpace(output.Content) == "" {
		return "", fmt.Errorf("AI returned empty content for chapter %s improvement", chapter.ID)
	}

	logger.Info("✓ Generated RPG-enhanced improved chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// RPGWriteImproveInput RPG改进输入
type RPGWriteImproveInput struct {
	StorySetup     CompactStorySetup `json:"story_setup"`
	Chapter        models.Chapter    `json:"chapter"`
	StateMatrix    string            `json:"state_matrix"`
	TargetWords    int               `json:"target_words"`
	CurrentDraft   string            `json:"current_draft"`
	Suggestions    string            `json:"suggestions"`
	Context        string            `json:"context,omitempty"`
	Recap          string            `json:"recap,omitempty"`
	RPGConstraints string            `json:"rpg_constraints,omitempty"`
}

// RPGWriteImproveOutput RPG改进输出
type RPGWriteImproveOutput struct {
	Content string `json:"content"`
}

// logWriteContext 记录写作上下文
func (a *RPGEnhancedWriteAgent) logWriteContext(chapterID, variant string, input interface{}, output string) error {
	debugDir := filepath.Join("logs", "rpg_write_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.md", chapterID, variant, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# RPG Write Context: %s (%s)\n\n", chapterID, variant))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## Input\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(fmt.Sprintf("%+v", input))
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Output\n\n")
	sb.WriteString("```\n")
	sb.WriteString(output)
	sb.WriteString("\n```\n")

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

// GetConstraintReport 获取约束报告
func (a *RPGEnhancedWriteAgent) GetConstraintReport() *rpg.ConstraintReport {
	return a.constraintReport
}

// ValidateOutlineWithRPG 使用RPG验证大纲
func (a *RPGEnhancedWriteAgent) ValidateOutlineWithRPG() (*rpg.RPGCheckResult, error) {
	if a.constraintSystem == nil {
		return nil, fmt.Errorf("constraint system not initialized")
	}

	// 这里可以调用RPG检查器来验证大纲
	checker := rpg.NewOutlineRPGChecker()
	
	// 需要从outline转换为StoryOutline
	// 这是一个简化实现
	storyOutline := rpg.StoryOutline{
		Parts: make([]rpg.StoryPart, 0),
	}

	return checker.Check(&storyOutline), nil
}
