package benchmark

import (
	"fmt"
	"testing"
)

// ============================================================
// Go testing 基准测试
// ============================================================

// BenchmarkStandardTestCases 标准测试用例基准测试
func BenchmarkStandardTestCases(b *testing.B) {
	runner := NewBenchmarkRunner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runner.Run()
	}
}

// BenchmarkSingleTestCase 单个测试用例基准测试
func BenchmarkSingleTestCase(b *testing.B) {
	testCases := StandardTestCases()
	if len(testCases) == 0 {
		b.Skip("没有测试用例")
	}

	runner := NewBenchmarkRunner()
	runner.TestCases = []BenchmarkTestCase{testCases[0]}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runner.Run()
	}
}

// BenchmarkTextExtraction 文本提取基准测试
func BenchmarkTextExtraction(b *testing.B) {
	testCases := StandardTestCases()
	if len(testCases) == 0 {
		b.Skip("没有测试用例")
	}

	longText := testCases[0].ChapterText

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractCharacterNames(longText)
	}
}

// BenchmarkCrossChapterCheck 跨章检查基准测试
func BenchmarkCrossChapterCheck(b *testing.B) {
	checker := NewCrossChapterChecker()

	// 添加一些章节
	for i := 0; i < 5; i++ {
		state := ChapterState{
			ChapterID:       fmt.Sprintf("ch%d", i),
			ChapterNum:      i + 1,
			CharacterStates: make(map[string]*CharacterSnapshot),
		}
		state.CharacterStates["林砚"] = &CharacterSnapshot{
			Name:        "林砚",
			HP:          100,
			MaxHP:       100,
			Cultivation: "练气" + fmt.Sprintf("%d", i+1),
			Status:      "alive",
		}
		checker.AddChapter(state)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckConsistency()
	}
}

// BenchmarkFuzzGeneration 模糊测试生成基准测试
func BenchmarkFuzzGeneration(b *testing.B) {
	gen := NewFuzzGenerator()
	gen.SetSeed(12345) // 固定种子保证可重复性

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.GenerateTestCase()
	}
}

// BenchmarkMetricsCalculation 指标计算基准测试
func BenchmarkMetricsCalculation(b *testing.B) {
	// 创建一些测试结果
	results := []MatchResult{
		{
			TestCaseName:   "test1",
			KnownIssues:    3,
			DetectedIssues: 3,
			TruePositives:  3,
			FalsePositives: 0,
			FalseNegatives: 0,
		},
		{
			TestCaseName:   "test2",
			KnownIssues:    2,
			DetectedIssues: 1,
			TruePositives:  1,
			FalsePositives: 0,
			FalseNegatives: 1,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateMetrics(results)
	}
}

// BenchmarkIssueMatching 问题匹配基准测试
func BenchmarkIssueMatching(b *testing.B) {
	tc := BenchmarkTestCase{
		Name: "benchmark_test",
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "too_frequent", Target: "林砚", Description: "突破太频繁", Severity: "critical"},
			{Category: "timeline", SubCategory: "jump", Target: "时间", Description: "时间跳跃", Severity: "warning"},
		},
	}

	detected := []DetectedIssue{
		{Category: "power", SubCategory: "too_frequent", Target: "林砚", Description: "突破太频繁", Severity: "critical"},
		{Category: "character", SubCategory: "tool", Target: "张三", Description: "工具人", Severity: "warning"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchDetectedToKnown(tc, detected)
	}
}
