package benchmark

import (
	"fmt"
	"strings"
)

// ============================================================
// 指标体系
// ============================================================

// Metrics 评估指标集
type Metrics struct {
	// Issue Recall: 检出真实问题的比例
	// = 正确检出数 / 总真实问题数
	IssueRecall float64

	// Issue Precision: 检出结果中正确问题的比例
	// = 正确检出数 / 总检出数
	IssuePrecision float64

	// False Positive Rate: 误报率
	// = 误报数 / (误报数 + 正确检出数)
	FalsePositiveRate float64

	// CategoryCoverage: 按类别统计的检出率
	CategoryCoverage map[string]CategoryMetric

	// DifficultyMetrics: 按难度级别统计的指标
	DifficultyMetrics map[DifficultyLevel]DifficultyMetric

	// PostWriteViolationRate: 写作后残留违反率（需要写作流程数据）
	PostWriteViolationRate float64

	// CrossChapterConsistency: 跨章一致性得分 (0-100)
	CrossChapterConsistency float64

	// OverallScore: 加权总分 (0-100)
	OverallScore float64

	// 测试统计
	TotalTestCases   int
	PassedTestCases  int
	FailedTestCases  int
	SkippedTestCases int

	// 问题统计
	TotalKnownIssues   int
	TotalDetectedIssues int
	TruePositives      int
	FalsePositives     int
	FalseNegatives     int
}

// CategoryMetric 按类别的指标
type CategoryMetric struct {
	Category    string
	TotalKnown  int     // 该类已知问题数
	Detected    int     // 检出数
	FalsePos    int     // 误报数
	Recall      float64 // 检出率
	Precision   float64 // 精确率
}

// DifficultyMetric 按难度的指标
type DifficultyMetric struct {
	Level           DifficultyLevel
	TotalTestCases  int
	PassedCases     int
	TotalKnown      int
	Detected        int
	FalsePos        int
	Recall          float64
	Precision       float64
	AvgIssuesPerCase float64
}

// ============================================================
// 检出结果匹配
// ============================================================

// DetectedIssue 系统检出的问题
type DetectedIssue struct {
	Category    string
	SubCategory string
	Target      string
	Description string
	Severity    string
}

// MatchResult 单个测试用例的匹配结果
type MatchResult struct {
	TestCaseName   string
	KnownIssues    int
	DetectedIssues int
	TruePositives  int  // 正确检出
	FalsePositives int  // 误报
	FalseNegatives int  // 漏检
	MatchedKnown   []KnownIssue    // 被成功检出的已知问题
	MissedKnown    []KnownIssue    // 被漏检的已知问题
	FalseAlarms    []DetectedIssue // 误报的问题
}

// MatchDetectedToKnown 将检出结果与标注问题匹配
func MatchDetectedToKnown(tc BenchmarkTestCase, detected []DetectedIssue) MatchResult {
	result := MatchResult{
		TestCaseName:   tc.Name,
		KnownIssues:    len(tc.KnownIssues),
		DetectedIssues: len(detected),
	}

	matchedKnown := make(map[int]bool)
	matchedDetected := make(map[int]bool)

	// 对每个已知问题，检查是否有检出问题能匹配
	for ki, known := range tc.KnownIssues {
		for di, det := range detected {
			if matchedDetected[di] {
				continue
			}
			if matches(known, det) {
				result.TruePositives++
				matchedKnown[ki] = true
				matchedDetected[di] = true
				result.MatchedKnown = append(result.MatchedKnown, known)
				break
			}
		}
	}

	// 收集漏检
	for ki, known := range tc.KnownIssues {
		if !matchedKnown[ki] {
			result.FalseNegatives++
			result.MissedKnown = append(result.MissedKnown, known)
		}
	}

	// 收集误报（没有匹配到任何已知问题的检出）
	for di, det := range detected {
		if !matchedDetected[di] {
			// 检查是否误报：检出的问题在正常内容范围内
			isFalsePositive := isFalseAlarm(det, tc.ValidContent)
			if isFalsePositive || len(tc.KnownIssues) == 0 {
				result.FalsePositives++
				result.FalseAlarms = append(result.FalseAlarms, det)
			} else {
				// 可能是合理的检出，但没对应到标注 — 算作额外检出
				// 暂时也视为误报以保守估计精确率
				result.FalsePositives++
				result.FalseAlarms = append(result.FalseAlarms, det)
			}
		}
	}

	return result
}

// matches 判断检出问题是否匹配已知问题
func matches(known KnownIssue, detected DetectedIssue) bool {
	// 类别必须匹配
	if known.Category != detected.Category {
		return false
	}

	// 子类别匹配（如果标注了）
	if known.SubCategory != "" && detected.SubCategory != "" {
		if known.SubCategory != detected.SubCategory {
			// 子类别不完全相同，但语义上可能相关
			if !subCategoryRelated(known.SubCategory, detected.SubCategory) {
				return false
			}
		}
	}

	// 目标匹配
	if known.Target != "" && detected.Target != "" {
		if known.Target != detected.Target {
			return false
		}
	}

	// 严重程度匹配（允许升级/降级一级）
	if known.Severity != "" && detected.Severity != "" {
		if known.Severity != detected.Severity {
			if !severityClose(known.Severity, detected.Severity) {
				return false
			}
		}
	}

	return true
}

// subCategoryRelated 判断子类别是否语义相关
func subCategoryRelated(a, b string) bool {
	relatedGroups := [][]string{
		{"too_frequent", "no_struggle", "inconsistency"},
		{"too_frequent", "no_cost"},
		{"excessive_jumps", "implicit_jumps", "no_transition"},
		{"location_contradiction", "tool_character"},
		{"forbidden_element", "no_cost"},
		{"all_fast", "no_transition"},
	}

	for _, group := range relatedGroups {
		hasA, hasB := false, false
		for _, item := range group {
			if item == a {
				hasA = true
			}
			if item == b {
				hasB = true
			}
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

// severityClose 判断严重程度是否接近
func severityClose(a, b string) bool {
	levels := map[string]int{"critical": 3, "error": 2, "warning": 1, "info": 0}
	aLevel, okA := levels[a]
	bLevel, okB := levels[b]
	if !okA || !okB {
		return true // 未知级别不做判断
	}
	diff := aLevel - bLevel
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1 // 允许差一级
}

// isFalseAlarm 判断检出问题是否是误报
func isFalseAlarm(detected DetectedIssue, validContent []string) bool {
	for _, content := range validContent {
		if strings.Contains(strings.ToLower(detected.Description), strings.ToLower(content)) {
			return true
		}
	}
	return false
}

// ============================================================
// 指标计算
// ============================================================

// CalculateMetrics 从所有匹配结果计算指标
func CalculateMetrics(results []MatchResult) Metrics {
	return CalculateMetricsWithDifficulty(results, nil)
}

// CalculateMetricsWithDifficulty 从匹配结果计算指标（包含难度信息）
func CalculateMetricsWithDifficulty(results []MatchResult, testCases []BenchmarkTestCase) Metrics {
	m := Metrics{
		CategoryCoverage:  make(map[string]CategoryMetric),
		DifficultyMetrics: make(map[DifficultyLevel]DifficultyMetric),
	}

	totalKnown := 0
	totalDetected := 0
	totalTP := 0
	totalFP := 0
	totalFN := 0

	// 按类别收集
	categoryStats := make(map[string]*categoryAccum)

	// 按难度收集
	difficultyStats := make(map[DifficultyLevel]*difficultyAccum)

	for i, r := range results {
		totalKnown += r.KnownIssues
		totalDetected += r.DetectedIssues
		totalTP += r.TruePositives
		totalFP += r.FalsePositives
		totalFN += r.FalseNegatives

		// 类别统计
		for _, matched := range r.MatchedKnown {
			acc := getCategoryAccum(categoryStats, matched.Category)
			acc.detected++
		}
		for _, missed := range r.MissedKnown {
			acc := getCategoryAccum(categoryStats, missed.Category)
			acc.missed++
		}
		for _, fp := range r.FalseAlarms {
			acc := getCategoryAccum(categoryStats, fp.Category)
			acc.falsePos++
		}

		// 难度统计
		if testCases != nil && i < len(testCases) {
			level := testCases[i].Difficulty
			acc := getDifficultyAccum(difficultyStats, level)
			acc.totalCases++
			acc.totalKnown += r.KnownIssues
			acc.detected += r.TruePositives
			acc.falsePos += r.FalsePositives

			// 如果检出率>80%认为通过
			if r.KnownIssues > 0 && float64(r.TruePositives)/float64(r.KnownIssues) > 0.8 {
				acc.passedCases++
			} else if r.KnownIssues == 0 && r.FalsePositives == 0 {
				acc.passedCases++
			}
		}
	}

	// 填充基本统计
	m.TotalTestCases = len(results)
	m.TotalKnownIssues = totalKnown
	m.TotalDetectedIssues = totalDetected
	m.TruePositives = totalTP
	m.FalsePositives = totalFP
	m.FalseNegatives = totalFN

	// Recall
	if totalKnown > 0 {
		m.IssueRecall = float64(totalTP) / float64(totalKnown)
	}

	// Precision
	if totalDetected > 0 {
		m.IssuePrecision = float64(totalTP) / float64(totalDetected)
	}

	// False Positive Rate
	if totalTP+totalFP > 0 {
		m.FalsePositiveRate = float64(totalFP) / float64(totalTP+totalFP)
	}

	// Category Coverage
	for cat, acc := range categoryStats {
		total := acc.detected + acc.missed
		cm := CategoryMetric{
			Category:   cat,
			TotalKnown: total,
			Detected:   acc.detected,
			FalsePos:   acc.falsePos,
		}
		if total > 0 {
			cm.Recall = float64(acc.detected) / float64(total)
		}
		detectedTotal := acc.detected + acc.falsePos
		if detectedTotal > 0 {
			cm.Precision = float64(acc.detected) / float64(detectedTotal)
		}
		m.CategoryCoverage[cat] = cm
	}

	// Difficulty Metrics
	for level, acc := range difficultyStats {
		dm := DifficultyMetric{
			Level:          level,
			TotalTestCases: acc.totalCases,
			PassedCases:    acc.passedCases,
			TotalKnown:     acc.totalKnown,
			Detected:       acc.detected,
			FalsePos:       acc.falsePos,
		}
		if acc.totalCases > 0 {
			dm.AvgIssuesPerCase = float64(acc.totalKnown) / float64(acc.totalCases)
		}
		if acc.totalKnown > 0 {
			dm.Recall = float64(acc.detected) / float64(acc.totalKnown)
		}
		detectedTotal := acc.detected + acc.falsePos
		if detectedTotal > 0 {
			dm.Precision = float64(acc.detected) / float64(detectedTotal)
		}
		m.DifficultyMetrics[level] = dm
	}

	// Overall Score: 加权
	// Recall 40%, Precision 40%, FPR penalty 20%
	m.OverallScore = m.IssueRecall*40 + m.IssuePrecision*40 - m.FalsePositiveRate*20
	if m.OverallScore < 0 {
		m.OverallScore = 0
	}

	return m
}

type categoryAccum struct {
	detected int
	missed   int
	falsePos int
}

func getCategoryAccum(m map[string]*categoryAccum, cat string) *categoryAccum {
	if _, ok := m[cat]; !ok {
		m[cat] = &categoryAccum{}
	}
	return m[cat]
}

type difficultyAccum struct {
	totalCases int
	passedCases int
	totalKnown int
	detected   int
	falsePos   int
}

func getDifficultyAccum(m map[DifficultyLevel]*difficultyAccum, level DifficultyLevel) *difficultyAccum {
	if _, ok := m[level]; !ok {
		m[level] = &difficultyAccum{}
	}
	return m[level]
}

// ============================================================
// 指标格式化
// ============================================================

// FormatMetrics 格式化输出指标
func FormatMetrics(m Metrics) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║           RPG 模拟系统 Benchmark 报告              ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n\n")

	sb.WriteString(fmt.Sprintf("  总体评分: %.1f / 100\n\n", m.OverallScore))

	// 测试统计
	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│              测试统计                    │\n")
	sb.WriteString("├─────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 总测试用例:     %3d                     │\n", m.TotalTestCases))
	sb.WriteString(fmt.Sprintf("│ 已知问题数:     %3d                     │\n", m.TotalKnownIssues))
	sb.WriteString(fmt.Sprintf("│ 检出总数:       %3d                     │\n", m.TotalDetectedIssues))
	sb.WriteString(fmt.Sprintf("│ 正确检出 (TP):  %3d                     │\n", m.TruePositives))
	sb.WriteString(fmt.Sprintf("│ 漏检 (FN):      %3d                     │\n", m.FalseNegatives))
	sb.WriteString(fmt.Sprintf("│ 误报 (FP):      %3d                     │\n", m.FalsePositives))
	sb.WriteString("└─────────────────────────────────────────┘\n\n")

	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│              核心指标                    │\n")
	sb.WriteString("├─────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ Issue Recall:      %6.1f%% (%5.1f/40) │\n",
		m.IssueRecall*100, m.IssueRecall*40))
	sb.WriteString(fmt.Sprintf("│ Issue Precision:   %6.1f%% (%5.1f/40) │\n",
		m.IssuePrecision*100, m.IssuePrecision*40))
	sb.WriteString(fmt.Sprintf("│ False Positive:    %6.1f%% (%5.1f/20) │\n",
		m.FalsePositiveRate*100, m.FalsePositiveRate*20))
	sb.WriteString("└─────────────────────────────────────────┘\n\n")

	// 按难度统计
	if len(m.DifficultyMetrics) > 0 {
		sb.WriteString("┌─────────────────────────────────────────────────┐\n")
		sb.WriteString("│              按难度级别统计                      │\n")
		sb.WriteString("├──────────┬───────┬───────┬────────┬────────────┤\n")
		sb.WriteString("│ 难度     │ 用例数│ 通过  │ Recall │ Precision  │\n")
		sb.WriteString("├──────────┼───────┼───────┼────────┼────────────┤\n")

		difficultyOrder := []DifficultyLevel{DifficultyEasy, DifficultyMedium, DifficultyHard, DifficultyExpert}
		difficultyNames := map[DifficultyLevel]string{
			DifficultyEasy:   "简单",
			DifficultyMedium: "中等",
			DifficultyHard:   "困难",
			DifficultyExpert: "专家",
		}

		for _, level := range difficultyOrder {
			if dm, ok := m.DifficultyMetrics[level]; ok {
				sb.WriteString(fmt.Sprintf("│ %-8s │ %5d │ %5d │ %5.1f%% │ %8.1f%%  │\n",
					difficultyNames[level],
					dm.TotalTestCases,
					dm.PassedCases,
					dm.Recall*100,
					dm.Precision*100))
			}
		}
		sb.WriteString("└──────────┴───────┴───────┴────────┴────────────┘\n\n")
	}

	// 按类别统计
	if len(m.CategoryCoverage) > 0 {
		sb.WriteString("┌─────────────────────────────────────────┐\n")
		sb.WriteString("│           按问题类别统计                  │\n")
		sb.WriteString("├──────────┬────────┬────────┬────────────┤\n")
		sb.WriteString("│ 类别     │ Recall │ Prec.  │ 误报数     │\n")
		sb.WriteString("├──────────┼────────┼────────┼────────────┤\n")
		for cat, cm := range m.CategoryCoverage {
			sb.WriteString(fmt.Sprintf("│ %-8s │ %5.1f%% │ %5.1f%% │ %8d   │\n",
				cat, cm.Recall*100, cm.Precision*100, cm.FalsePos))
		}
		sb.WriteString("└──────────┴────────┴────────┴────────────┘\n\n")
	}

	// 其他指标
	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│           其他一致性指标                 │\n")
	sb.WriteString("├─────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 跨章一致性得分:    %6.1f/100          │\n", m.CrossChapterConsistency))
	sb.WriteString(fmt.Sprintf("│ 写作后残留违反率:  %6.1f%%             │\n", m.PostWriteViolationRate*100))
	sb.WriteString("└─────────────────────────────────────────┘\n")

	return sb.String()
}
