package benchmark

import (
	"fmt"
	"time"

	"novelgen/internal/rpg"
)

// ============================================================
// Benchmark 主入口 — 运行评估并生成报告
// ============================================================

// BenchmarkRunner 评估运行器
type BenchmarkRunner struct {
	TestCases []BenchmarkTestCase
	Results   []MatchResult
	Metrics   Metrics
	Duration  time.Duration
}

// NewBenchmarkRunner 创建评估运行器
func NewBenchmarkRunner() *BenchmarkRunner {
	return &BenchmarkRunner{
		TestCases: StandardTestCases(),
		Results:   make([]MatchResult, 0),
	}
}

// Run 执行评估
func (br *BenchmarkRunner) Run() {
	start := time.Now()
	br.Results = make([]MatchResult, 0, len(br.TestCases))

	for _, tc := range br.TestCases {
		result := br.runTestCase(tc)
		br.Results = append(br.Results, result)
	}

	br.Duration = time.Since(start)
	br.Metrics = CalculateMetricsWithDifficulty(br.Results, br.TestCases)
}

// RunOnly 运行指定测试用例
func (br *BenchmarkRunner) RunOnly(names []string) {
	start := time.Now()
	br.Results = make([]MatchResult, 0)
	var runTestCases []BenchmarkTestCase

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	for _, tc := range br.TestCases {
		if nameSet[tc.Name] {
			result := br.runTestCase(tc)
			br.Results = append(br.Results, result)
			runTestCases = append(runTestCases, tc)
		}
	}

	br.Duration = time.Since(start)
	br.Metrics = CalculateMetricsWithDifficulty(br.Results, runTestCases)
}

// runTestCase 运行单个测试用例
func (br *BenchmarkRunner) runTestCase(tc BenchmarkTestCase) MatchResult {
	var detected []DetectedIssue

	// 1. 如果有章节文本，用 ConstraintSystem 检测
	if tc.ChapterText != "" {
		detected = append(detected, br.detectFromText(tc)...)
	}

	// 2. 如果有大纲快照，用 OutlineRPGChecker + OutlineValidator 检测
	if tc.Outline != nil {
		detected = append(detected, br.detectFromOutline(tc)...)
	}

	// 3. 匹配检出结果与标注问题
	return MatchDetectedToKnown(tc, detected)
}

// detectFromText 从文本中检测问题
func (br *BenchmarkRunner) detectFromText(tc BenchmarkTestCase) []DetectedIssue {
	// 创建一个最小化的 GameWorld 用于约束检测
	world := rpg.NewGameWorld()

	// 创建主角
	template := &rpg.CharacterTemplate{
		ID:        "char_林砚",
		Name:      "林砚",
		Type:      rpg.CharacterTypePlayer,
		BaseStats: rpg.BaseStats{HP: 100, MP: 50, Attack: 12, Defense: 10, Speed: 10, Luck: 10},
	}
	world.Characters.AddTemplate(template)
	player := rpg.NewCharacter(template, "林砚")
	world.SetPlayer(player)

	// 创建约束系统
	cs := rpg.NewConstraintSystem(world)
	report := cs.BuildFromRPGData()

	// 运行验证
	violations := cs.ValidateChapter("test_chapter", tc.ChapterText)

	detected := make([]DetectedIssue, 0, len(violations))
	for _, v := range violations {
		detected = append(detected, DetectedIssue{
			Category:    v.Type,
			Target:      v.Target,
			Description: v.Issue,
			Severity:    v.Severity,
		})
	}

	// 也用 NovelSimulator 检测
	detected = append(detected, br.detectFromSimulator(tc, world)...)

	_ = report // 后续会用来做更多检测
	return detected
}

// detectFromSimulator 使用 NovelSimulator 检测
// 注意：模拟器需要完整的 GameWorld，对文本测试用例不太适用，
// 所以这里仅做安全提取，如果世界不完整则跳过
func (br *BenchmarkRunner) detectFromSimulator(tc BenchmarkTestCase, world *rpg.GameWorld) []DetectedIssue {
	// 需要一个有 Player 的世界才能运行模拟器
	if world.Player == nil {
		return nil
	}

	// 构造 NovelRPGData 用于模拟器
	data := &rpg.NovelRPGData{
		Characters:       make([]*rpg.CharacterTemplate, 0),
		Items:            make([]*rpg.Item, 0),
		Skills:           make([]*rpg.Skill, 0),
		Locations:        make([]*rpg.Map, 0),
		Events:           make([]*rpg.Event, 0),
		Quests:           make([]*rpg.Quest, 0),
		Timeline:         make([]*rpg.TimelineEvent, 0),
		ValidationIssues: make([]rpg.ValidationIssue, 0),
	}

	// 添加主角到数据
	template := &rpg.CharacterTemplate{
		ID:        "char_林砚",
		Name:      "林砚",
		Type:      rpg.CharacterTypePlayer,
		BaseStats: rpg.BaseStats{HP: 100, MP: 50, Attack: 12, Defense: 10, Speed: 10, Luck: 10},
	}
	data.Characters = append(data.Characters, template)

	// 从文本提取时间线事件
	data.Timeline = extractTimelineFromText(tc.ChapterText)

	// 安全运行模拟器
	detected := make([]DetectedIssue, 0)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// 模拟器崩溃时不影响基准测试，跳过
			}
		}()
		sim := rpg.NewNovelSimulator(data)
		report := sim.Simulate()

		for _, vr := range report.ValidationResults {
			if vr.Passed {
				continue
			}
			for _, issue := range vr.Issues {
				detected = append(detected, DetectedIssue{
					Category:    vr.Category,
					Description: issue,
					Severity:    "warning",
				})
			}
		}
	}()

	return detected
}

// detectFromOutline 从大纲快照中检测问题
func (br *BenchmarkRunner) detectFromOutline(tc BenchmarkTestCase) []DetectedIssue {
	if tc.Outline == nil {
		return nil
	}

	// 转换为 rpg.StoryOutline
	outline := convertOutlineSnapshot(tc.Outline)

	detected := make([]DetectedIssue, 0)

	// 用 OutlineRPGChecker 检测
	checker := rpg.NewOutlineRPGChecker()
	result := checker.Check(&outline)

	// 从 Debuffs 提取
	for _, debuff := range result.Debuffs {
		cat := mapDebuffToCategory(debuff.Name)
		detected = append(detected, DetectedIssue{
			Category:    cat,
			Description: debuff.Description,
			Severity:    mapSeverity(debuff.Severity),
		})
	}

	// 从 Bosses 提取
	for _, boss := range result.Bosses {
		cat := mapBossToCategory(boss.Name)
		detected = append(detected, DetectedIssue{
			Category:    cat,
			Description: boss.Description,
			Severity:    "critical",
		})
	}

	// 也用 OutlineValidator 检测
	validator := rpg.NewOutlineValidator(&outline)
	vr := validator.Validate()

	for _, issue := range vr.Issues {
		detected = append(detected, DetectedIssue{
			Category:    issue.Type,
			Description: issue.Description,
			Severity:    issue.Severity,
		})
	}

	for _, warning := range vr.Warnings {
		detected = append(detected, DetectedIssue{
			Category:    warning.Type,
			Description: warning.Description,
			Severity:    "warning",
		})
	}

	return detected
}

// ============================================================
// 辅助转换函数
// ============================================================

func convertOutlineSnapshot(snap *StoryOutlineSnapshot) rpg.StoryOutline {
	outline := rpg.StoryOutline{
		Parts: make([]rpg.StoryPart, 0, len(snap.Parts)),
	}

	for _, ps := range snap.Parts {
		part := rpg.StoryPart{
			ID:      ps.ID,
			Title:   ps.Title,
			Volumes: make([]rpg.StoryVolume, 0, len(ps.Volumes)),
		}
		for _, vs := range ps.Volumes {
			vol := rpg.StoryVolume{
				ID:       vs.ID,
				Title:    vs.Title,
				Chapters: make([]rpg.StoryChapter, 0, len(vs.Chapters)),
			}
			for _, cs := range vs.Chapters {
				ch := rpg.StoryChapter{
					ID:          cs.ID,
					Title:       cs.Title,
					Summary:     cs.Summary,
					Characters:  cs.Characters,
					Location:    cs.Location,
					Beats:       cs.Beats,
					OpeningBeat: cs.OpeningBeat,
					ClosingBeat: cs.ClosingBeat,
					StateChange: cs.StateChange,
					Conflict:    cs.Conflict,
					Pacing:      cs.Pacing,
					Events:      make([]rpg.StoryEvent, 0, len(cs.Events)),
				}
				for _, es := range cs.Events {
					ch.Events = append(ch.Events, rpg.StoryEvent{
						Type:       es.Type,
						Characters: es.Characters,
						Subject:    es.Subject,
						Change:     es.Change,
						Details:    es.Details,
						Actor:      es.Actor,
						Action:     es.Action,
						Target:     es.Target,
						TargetType: es.TargetType,
						Context:    es.Context,
						Result:     es.Result,
					})
				}
				vol.Chapters = append(vol.Chapters, ch)
			}
			part.Volumes = append(part.Volumes, vol)
		}
		outline.Parts = append(outline.Parts, part)
	}

	return outline
}

func extractTimelineFromText(text string) []*rpg.TimelineEvent {
	// 简化的时间线提取 — 检测关键词后构造时间线事件
	// 实际优化后这里会被更好的提取替代
	events := make([]*rpg.TimelineEvent, 0)

	event := &rpg.TimelineEvent{
		Time:         0,
		Characters:   []string{"林砚"},
		Location:     "unknown",
		Events:       make([]string, 0),
		PowerChanges: make(map[string]int),
	}

	// 检测死亡/复活
	if containsSubstring(text, "死亡") || containsSubstring(text, "被杀") || containsSubstring(text, "穿心") {
		event.Events = append(event.Events, "death")
	}
	if containsSubstring(text, "复活") {
		event.Events = append(event.Events, "resurrection")
	}
	if containsSubstring(text, "突破") || containsSubstring(text, "踏入") {
		event.Events = append(event.Events, "breakthrough")
	}
	if containsSubstring(text, "战斗") || containsSubstring(text, "激战") || containsSubstring(text, "击杀") {
		event.Events = append(event.Events, "combat")
	}

	if len(event.Events) > 0 {
		events = append(events, event)
	}

	return events
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func mapDebuffToCategory(name string) string {
	switch name {
	case "战力崩坏":
		return "power"
	case "时间混乱":
		return "timeline"
	case "角色分散":
		return "character"
	case "节奏失控":
		return "pacing"
	case "逻辑断裂":
		return "plot"
	case "寿命耗尽":
		return "resurrection"
	default:
		return name
	}
}

func mapBossToCategory(name string) string {
	switch name {
	case "复活滥用者":
		return "resurrection"
	case "战力崩坏王":
		return "power"
	case "工具人军团":
		return "character"
	default:
		return name
	}
}

func mapSeverity(severity int) string {
	if severity >= 8 {
		return "critical"
	} else if severity >= 5 {
		return "error"
	}
	return "warning"
}

// ============================================================
// 跨章一致性检测
// ============================================================

// CrossChapterTestInput 跨章测试输入
type CrossChapterTestInput struct {
	Chapters []ChapterStateInput
}

// ChapterStateInput 章节状态输入
type ChapterStateInput struct {
	ChapterID       string
	CharacterStates map[string]CharacterStateInput
}

// CharacterStateInput 角色状态输入
type CharacterStateInput struct {
	Name     string
	HP       int
	MP       int
	Level    int
	Location string
	Alive    bool
}

// RunCrossChapterTest 运行跨章一致性测试（简化版）
func (br *BenchmarkRunner) RunCrossChapterTest(input CrossChapterTestInput) float64 {
	if len(input.Chapters) < 2 {
		return 100
	}

	// 转换为新的跨章检查器格式
	checker := NewCrossChapterChecker()

	for i, chInput := range input.Chapters {
		state := ChapterState{
			ChapterID:       chInput.ChapterID,
			ChapterNum:      i + 1,
			CharacterStates: make(map[string]*CharacterSnapshot),
		}

		for charName, charInput := range chInput.CharacterStates {
			status := "alive"
			if !charInput.Alive {
				status = "dead"
			}
			state.CharacterStates[charName] = &CharacterSnapshot{
				Name:     charName,
				HP:       charInput.HP,
				MaxHP:    charInput.HP,
				MP:       charInput.MP,
				Location: charInput.Location,
				Status:   status,
				PowerLevel: charInput.Level * 100,
			}
		}

		checker.AddChapter(state)
	}

	issues := checker.CheckConsistency()
	metrics := CalculateCrossChapterMetrics(issues)

	return metrics.ConsistencyScore
}

// RunCrossChapterAnalysis 运行完整的跨章分析
func (br *BenchmarkRunner) RunCrossChapterAnalysis(chapters []ChapterAnalysisInput) CrossChapterMetrics {
	checker := NewCrossChapterChecker()

	for i, ch := range chapters {
		state := ExtractChapterStateFromText(ch.ChapterID, i+1, ch.Text)
		checker.AddChapter(state)
	}

	issues := checker.CheckConsistency()
	return CalculateCrossChapterMetrics(issues)
}

// ChapterAnalysisInput 章节分析输入
type ChapterAnalysisInput struct {
	ChapterID string
	Text      string
}

// ============================================================
// 报告
// ============================================================

// GenerateReport 生成完整评估报告
func (br *BenchmarkRunner) GenerateReport() string {
	var s string
	s += FormatMetrics(br.Metrics)
	s += "\n"

	s += fmt.Sprintf("  测试用例数: %d\n", len(br.Results))
	s += fmt.Sprintf("  执行耗时: %v\n\n", br.Duration)

	// 详细匹配结果
	s += "┌──────────────────────────────────────────────────────┐\n"
	s += "│              各用例匹配详情                           │\n"
	s += "├──────────────────────────────────────────────────────┤\n"

	for _, r := range br.Results {
		s += fmt.Sprintf("│ %-40s             │\n", r.TestCaseName)
		s += fmt.Sprintf("│   已知: %d  检出: %d  命中: %d  漏检: %d  误报: %d │\n",
			r.KnownIssues, r.DetectedIssues, r.TruePositives, r.FalseNegatives, r.FalsePositives)

		if len(r.MissedKnown) > 0 {
			s += "│   漏检:                                              │\n"
			for _, m := range r.MissedKnown {
				s += fmt.Sprintf("│     - [%s] %s: %s             │\n", m.Category, m.Target, m.Description)
			}
		}

		if len(r.FalseAlarms) > 0 {
			s += "│   误报:                                              │\n"
			for _, f := range r.FalseAlarms {
				s += fmt.Sprintf("│     - [%s] %s             │\n", f.Category, f.Description)
			}
		}
	}

	s += "└──────────────────────────────────────────────────────┘\n"

	return s
}
