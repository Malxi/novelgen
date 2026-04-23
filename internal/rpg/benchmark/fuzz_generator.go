package benchmark

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ============================================================
// 模糊测试用例生成器
// ============================================================

// FuzzGenerator 模糊测试生成器
type FuzzGenerator struct {
	rng *rand.Rand
}

// NewFuzzGenerator 创建模糊测试生成器
func NewFuzzGenerator() *FuzzGenerator {
	return &FuzzGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetSeed 设置随机种子（用于可重复测试）
func (fg *FuzzGenerator) SetSeed(seed int64) {
	fg.rng = rand.New(rand.NewSource(seed))
}

// GenerateTestCase 生成单个模糊测试用例
func (fg *FuzzGenerator) GenerateTestCase() BenchmarkTestCase {
	// 随机选择问题类型
	issueTypes := []string{"power", "resurrection", "timeline", "character", "pacing", "plot"}
	issueType := issueTypes[fg.rng.Intn(len(issueTypes))]

	switch issueType {
	case "power":
		return fg.generatePowerIssue()
	case "resurrection":
		return fg.generateResurrectionIssue()
	case "timeline":
		return fg.generateTimelineIssue()
	case "character":
		return fg.generateCharacterIssue()
	case "pacing":
		return fg.generatePacingIssue()
	case "plot":
		return fg.generatePlotIssue()
	default:
		return fg.generateNormalContent()
	}
}

// GenerateTestCases 生成多个模糊测试用例
func (fg *FuzzGenerator) GenerateTestCases(count int) []BenchmarkTestCase {
	cases := make([]BenchmarkTestCase, 0, count)
	for i := 0; i < count; i++ {
		tc := fg.GenerateTestCase()
		tc.Name = fmt.Sprintf("fuzz_%s_%d", tc.Name, i)
		cases = append(cases, tc)
	}
	return cases
}

// GenerateMixedTestCase 生成包含多个问题的复杂测试用例
func (fg *FuzzGenerator) GenerateMixedTestCase(issueCount int) BenchmarkTestCase {
	issues := make([]KnownIssue, 0, issueCount)
	var textParts []string
	var validContent []string

	// 基础场景
	location := fg.randomLocation()
	character := fg.randomCharacter()

	textParts = append(textParts, fmt.Sprintf("%s来到了%s。", character, location))

	for i := 0; i < issueCount && i < 5; i++ {
		switch fg.rng.Intn(5) {
		case 0:
			// 添加战力问题
			pIssue, pText := fg.generateRandomPowerIssue(character)
			issues = append(issues, pIssue)
			textParts = append(textParts, pText)
		case 1:
			// 添加复活问题
			rIssue, rText := fg.generateRandomResurrectionIssue(character)
			issues = append(issues, rIssue)
			textParts = append(textParts, rText)
		case 2:
			// 添加时间线问题
			tIssue, tText := fg.generateRandomTimelineIssue()
			issues = append(issues, tIssue)
			textParts = append(textParts, tText)
		case 3:
			// 添加角色问题
			cIssue, cText, cValid := fg.generateRandomCharacterIssue(character)
			issues = append(issues, cIssue)
			textParts = append(textParts, cText)
			validContent = append(validContent, cValid...)
		case 4:
			// 添加正常内容
			nText, nValid := fg.generateRandomNormalContent(character)
			textParts = append(textParts, nText)
			validContent = append(validContent, nValid...)
		}
	}

	return BenchmarkTestCase{
		Name:         fmt.Sprintf("fuzz_mixed_issues_%d", issueCount),
		Description:  fmt.Sprintf("模糊生成的混合问题场景，包含%d个问题", len(issues)),
		Difficulty:   DifficultyMedium,
		ChapterText:  strings.Join(textParts, "\n"),
		KnownIssues:  issues,
		ValidContent: validContent,
		Tags:         []string{"fuzz", "mixed", "generated"},
	}
}

// ============================================================
// 各种问题的生成器
// ============================================================

func (fg *FuzzGenerator) generatePowerIssue() BenchmarkTestCase {
	char := fg.randomCharacter()
	cultivation := fg.randomCultivation()
	breakthroughCount := 2 + fg.rng.Intn(4) // 2-5次突破

	var text strings.Builder
	var issues []KnownIssue

	text.WriteString(fmt.Sprintf("%s开始了修炼。\n", char))

	for i := 0; i < breakthroughCount; i++ {
		methods := []string{"服用灵丹", "感悟天地", "战斗突破", "闭关苦修", "奇遇所得"}
		method := methods[fg.rng.Intn(len(methods))]
		text.WriteString(fmt.Sprintf("%s后，%s突破到了%s%d层。\n",
			method, char, cultivation, i+1))
	}

	issues = append(issues, KnownIssue{
		Category:    "power",
		SubCategory: "too_frequent",
		Target:      char,
		Description: fmt.Sprintf("连续%d次突破", breakthroughCount),
		Severity:    "critical",
	})

	return BenchmarkTestCase{
		Name:        "fuzz_power_breakthrough",
		Description: "模糊生成的战力崩坏场景",
		Difficulty:  DifficultyEasy,
		ChapterText: text.String(),
		KnownIssues: issues,
		Tags:        []string{"fuzz", "power", "breakthrough"},
	}
}

func (fg *FuzzGenerator) generateResurrectionIssue() BenchmarkTestCase {
	char := fg.randomCharacter()
	deathCount := 1 + fg.rng.Intn(3) // 1-3次死亡

	var text strings.Builder
	var issues []KnownIssue

	for i := 0; i < deathCount; i++ {
		causes := []string{"被敌人杀死", "走火入魔", "天劫降临", "中毒身亡", "自爆殉道"}
		cause := causes[fg.rng.Intn(len(causes))]
		text.WriteString(fmt.Sprintf("%s%s，当场身亡。\n", char, cause))

		reviveMethods := []string{"系统复活", "神秘力量", "丹药续命", "灵魂转生", "时间倒流"}
		method := reviveMethods[fg.rng.Intn(len(reviveMethods))]
		text.WriteString(fmt.Sprintf("%s后，%s复活了，没有任何代价。\n", method, char))
	}

	issues = append(issues,
		KnownIssue{
			Category:    "resurrection",
			SubCategory: "too_frequent",
			Target:      char,
			Description: fmt.Sprintf("死亡%d次复活%d次", deathCount, deathCount),
			Severity:    "critical",
		},
		KnownIssue{
			Category:    "resurrection",
			SubCategory: "no_cost",
			Target:      char,
			Description: "复活无代价",
			Severity:    "critical",
		},
	)

	return BenchmarkTestCase{
		Name:        "fuzz_resurrection_abuse",
		Description: "模糊生成的复活滥用场景",
		Difficulty:  DifficultyEasy,
		ChapterText: text.String(),
		KnownIssues: issues,
		Tags:        []string{"fuzz", "resurrection", "abuse"},
	}
}

func (fg *FuzzGenerator) generateTimelineIssue() BenchmarkTestCase {
	jumpCount := 2 + fg.rng.Intn(4) // 2-5次跳跃
	var timeUnits []string

	units := []string{"三天后", "一周后", "一个月后", "半年后", "一年后", "三年后", "十年后", "百年后"}

	var text strings.Builder
	text.WriteString("故事开始了。\n")

	for i := 0; i < jumpCount; i++ {
		unit := units[fg.rng.Intn(len(units))]
		timeUnits = append(timeUnits, unit)
		events := []string{"修为突破", "遇到强敌", "获得宝物", "结交好友", "遭遇背叛"}
		event := events[fg.rng.Intn(len(events))]
		text.WriteString(fmt.Sprintf("%s，%s。\n", unit, event))
	}

	return BenchmarkTestCase{
		Name:        "fuzz_timeline_jumps",
		Description: "模糊生成的时间线跳跃场景",
		Difficulty:  DifficultyEasy,
		ChapterText: text.String(),
		KnownIssues: []KnownIssue{
			{
				Category:    "timeline",
				SubCategory: "excessive_jumps",
				Target:      "时间线",
				Description: fmt.Sprintf("时间跳跃%d次", jumpCount),
				Severity:    "critical",
			},
		},
		Tags: []string{"fuzz", "timeline", "jumps"},
	}
}

func (fg *FuzzGenerator) generateCharacterIssue() BenchmarkTestCase {
	mainChar := fg.randomCharacter()
	toolChar := fg.randomToolCharacter()
	location1 := fg.randomLocation()
	location2 := fg.randomLocation()

	// 确保两个地点不同
	for location1 == location2 {
		location2 = fg.randomLocation()
	}

	var text strings.Builder
	text.WriteString(fmt.Sprintf("%s留在了%s修炼。\n", mainChar, location1))
	text.WriteString(fmt.Sprintf("与此同时，在%s的%s正在与其他人商议对策。\n", location2, mainChar))
	text.WriteString(fmt.Sprintf("%s遇到了%s，告诉他一些信息后离开了。\n", mainChar, toolChar))

	return BenchmarkTestCase{
		Name:        "fuzz_character_issues",
		Description: "模糊生成的角色矛盾场景",
		Difficulty:  DifficultyMedium,
		ChapterText: text.String(),
		KnownIssues: []KnownIssue{
			{
				Category:    "character",
				SubCategory: "location_contradiction",
				Target:      mainChar,
				Description: fmt.Sprintf("%s同时在%s和%s", mainChar, location1, location2),
				Severity:    "critical",
			},
			{
				Category:    "character",
				SubCategory: "tool_character",
				Target:      toolChar,
				Description: fmt.Sprintf("%s是工具人", toolChar),
				Severity:    "warning",
			},
		},
		ValidContent: []string{fmt.Sprintf("%s遇到了%s", mainChar, toolChar)},
		Tags:         []string{"fuzz", "character", "contradiction"},
	}
}

func (fg *FuzzGenerator) generatePacingIssue() BenchmarkTestCase {
	chapterCount := 4 + fg.rng.Intn(4) // 4-7章
	chapters := make([]ChapterSnapshot, 0, chapterCount)

	for i := 0; i < chapterCount; i++ {
		chapters = append(chapters, ChapterSnapshot{
			ID:       fmt.Sprintf("C%d", i+1),
			Title:    fmt.Sprintf("第%d章", i+1),
			Pacing:   "fast",
			Conflict: fg.randomConflict(),
			Beats:    []string{"冲突", "对抗", "结果"},
		})
	}

	return BenchmarkTestCase{
		Name:        "fuzz_pacing_fast",
		Description: "模糊生成的节奏失衡场景",
		Difficulty:  DifficultyMedium,
		Outline: &StoryOutlineSnapshot{
			Parts: []PartSnapshot{
				{
					ID:    "P1",
					Title: "第一篇",
					Volumes: []VolumeSnapshot{
						{
							ID:       "V1",
							Title:    "第一卷",
							Chapters: chapters,
						},
					},
				},
			},
		},
		KnownIssues: []KnownIssue{
			{
				Category:    "pacing",
				SubCategory: "all_fast",
				Target:      "V1",
				Description: fmt.Sprintf("连续%d章快节奏", chapterCount),
				Severity:    "error",
			},
		},
		Tags: []string{"fuzz", "pacing", "fast"},
	}
}

func (fg *FuzzGenerator) generatePlotIssue() BenchmarkTestCase {
	char := fg.randomCharacter()
	enemy := fg.randomCharacter()

	var text strings.Builder
	text.WriteString(fmt.Sprintf("%s一直是%s的死对头。\n", enemy, char))
	text.WriteString(fmt.Sprintf("昨天，%s还派人来刺杀%s。\n", enemy, char))
	text.WriteString(fmt.Sprintf("今天，%s突然表示愿意效忠于%s。\n", enemy, char))

	return BenchmarkTestCase{
		Name:        "fuzz_plot_relationship",
		Description: "模糊生成的剧情逻辑问题场景",
		Difficulty:  DifficultyHard,
		ChapterText: text.String(),
		KnownIssues: []KnownIssue{
			{
				Category:    "character",
				SubCategory: "relationship_inconsistency",
				Target:      enemy,
				Description: fmt.Sprintf("%s与%s的关系突变", enemy, char),
				Severity:    "error",
			},
		},
		Tags: []string{"fuzz", "plot", "relationship"},
	}
}

func (fg *FuzzGenerator) generateNormalContent() BenchmarkTestCase {
	char := fg.randomCharacter()
	location := fg.randomLocation()
	duration := []string{"一个月", "两个月", "三个月", "半年"}[fg.rng.Intn(4)]

	text := fmt.Sprintf(`%s在%s中度过了平静的%s。他每天坚持修炼基础功法，
虽然进展缓慢，但根基十分扎实。他知道修仙之路急不得。`,
		char, location, duration)

	return BenchmarkTestCase{
		Name:        "fuzz_normal_content",
		Description: "模糊生成的正常内容",
		Difficulty:  DifficultyEasy,
		ChapterText: text,
		KnownIssues: []KnownIssue{},
		ValidContent: []string{
			fmt.Sprintf("%s在%s中度过了平静的%s", char, location, duration),
			"修仙之路急不得",
		},
		Tags: []string{"fuzz", "normal", "baseline"},
	}
}

// ============================================================
// 辅助生成函数
// ============================================================

func (fg *FuzzGenerator) generateRandomPowerIssue(char string) (KnownIssue, string) {
	cult := fg.randomCultivation()
	level := 1 + fg.rng.Intn(9)
	text := fmt.Sprintf("%s服用灵丹后，突破到了%s%d层。", char, cult, level)
	return KnownIssue{
		Category:    "power",
		SubCategory: "too_frequent",
		Target:      char,
		Description: "突然突破",
		Severity:    "error",
	}, text
}

func (fg *FuzzGenerator) generateRandomResurrectionIssue(char string) (KnownIssue, string) {
	text := fmt.Sprintf("%s被杀死后，没有任何解释地复活了。", char)
	return KnownIssue{
		Category:    "resurrection",
		SubCategory: "no_cost",
		Target:      char,
		Description: "无理由复活",
		Severity:    "critical",
	}, text
}

func (fg *FuzzGenerator) generateRandomTimelineIssue() (KnownIssue, string) {
	timeUnits := []string{"三天后", "一个月后", "半年后", "一年后"}
	unit := timeUnits[fg.rng.Intn(len(timeUnits))]
	text := fmt.Sprintf("%s，发生了一些事情。", unit)
	return KnownIssue{
		Category:    "timeline",
		SubCategory: "jump",
		Target:      "时间线",
		Description: fmt.Sprintf("时间跳跃：%s", unit),
		Severity:    "warning",
	}, text
}

func (fg *FuzzGenerator) generateRandomCharacterIssue(mainChar string) (KnownIssue, string, []string) {
	toolChar := fg.randomToolCharacter()
	text := fmt.Sprintf("%s遇到了%s，告诉他一些信息后离开了。", mainChar, toolChar)
	return KnownIssue{
		Category:    "character",
		SubCategory: "tool_character",
		Target:      toolChar,
		Description: fmt.Sprintf("%s是工具人", toolChar),
		Severity:    "warning",
	}, text, []string{fmt.Sprintf("%s遇到了%s", mainChar, toolChar)}
}

func (fg *FuzzGenerator) generateRandomNormalContent(char string) (string, []string) {
	text := fmt.Sprintf("%s继续修炼，稳扎稳打。", char)
	return text, []string{"稳扎稳打"}
}

// ============================================================
// 随机数据生成
// ============================================================

func (fg *FuzzGenerator) randomCharacter() string {
	surnames := []string{"林", "李", "王", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴", "徐", "孙", "马", "朱", "胡", "郭", "何", "高", "罗"}
	names := []string{"砚", "凡", "尘", "逸", "轩", "辰", "昊", "宇", "浩", "杰", "伟", "涛", "明", "强", "磊", "静", "敏", "丽", "婷", "娜"}

	surname := surnames[fg.rng.Intn(len(surnames))]
	name := names[fg.rng.Intn(len(names))]
	return surname + name
}

func (fg *FuzzGenerator) randomToolCharacter() string {
	surnames := []string{"张", "王", "李", "刘", "陈", "杨", "赵", "黄"}
	names := []string{"三", "四", "五", "六", "小七", "老八", "九", "小二"}

	surname := surnames[fg.rng.Intn(len(surnames))]
	name := names[fg.rng.Intn(len(names))]
	return surname + name
}

func (fg *FuzzGenerator) randomLocation() string {
	locations := []string{"矿场", "宗门", "城池", "洞府", "山谷", "森林", "湖心岛", "古遗迹", "秘境", "天外天"}
	return locations[fg.rng.Intn(len(locations))]
}

func (fg *FuzzGenerator) randomCultivation() string {
	cults := []string{"练气", "筑基", "金丹", "元婴", "化神", "后天", "先天", "宗师", "斗者", "斗师"}
	return cults[fg.rng.Intn(len(cults))]
}

func (fg *FuzzGenerator) randomConflict() string {
	conflicts := []string{"遭遇强敌", "争夺宝物", "宗门任务", "复仇之战", "守护同伴", "突破瓶颈", "应对阴谋", "探索秘境"}
	return conflicts[fg.rng.Intn(len(conflicts))]
}

// ============================================================
// 边界测试用例生成
// ============================================================

// GenerateBoundaryTestCases 生成边界测试用例
func (fg *FuzzGenerator) GenerateBoundaryTestCases() []BenchmarkTestCase {
	cases := make([]BenchmarkTestCase, 0)

	// 边界1: 刚好达到阈值的问题
	cases = append(cases, BenchmarkTestCase{
		Name:        "boundary_power_threshold",
		Description: "边界测试：刚好2次突破（阈值）",
		Difficulty:  DifficultyMedium,
		ChapterText: `林砚修炼到练气一层。
三天后，突破到练气二层。
一周后，突破到练气三层。`,
		KnownIssues: []KnownIssue{
			// 刚好在边界，可能有也可能没有
		},
		Tags: []string{"boundary", "power", "threshold"},
	})

	// 边界2: 空章节
	cases = append(cases, BenchmarkTestCase{
		Name:        "boundary_empty_chapter",
		Description: "边界测试：空章节",
		Difficulty:  DifficultyEasy,
		ChapterText: "",
		KnownIssues: []KnownIssue{},
		Tags:        []string{"boundary", "empty"},
	})

	// 边界3: 极长章节
	longText := strings.Repeat("林砚继续修炼。", 1000)
	cases = append(cases, BenchmarkTestCase{
		Name:        "boundary_very_long_chapter",
		Description: "边界测试：极长章节",
		Difficulty:  DifficultyMedium,
		ChapterText: longText,
		KnownIssues: []KnownIssue{},
		Tags:        []string{"boundary", "long", "performance"},
	})

	// 边界4: 大量角色
	var manyCharsText strings.Builder
	for i := 0; i < 50; i++ {
		manyCharsText.WriteString(fmt.Sprintf("弟子%d出场。", i))
	}
	cases = append(cases, BenchmarkTestCase{
		Name:        "boundary_many_characters",
		Description: "边界测试：大量角色",
		Difficulty:  DifficultyMedium,
		ChapterText: manyCharsText.String(),
		KnownIssues: []KnownIssue{},
		Tags:        []string{"boundary", "characters", "scale"},
	})

	// 边界5: 复杂嵌套问题
	cases = append(cases, fg.GenerateMixedTestCase(5))

	return cases
}

// GenerateAdversarialTestCases 生成对抗性测试用例
func (fg *FuzzGenerator) GenerateAdversarialTestCases() []BenchmarkTestCase {
	cases := make([]BenchmarkTestCase, 0)

	// 对抗1: 伪装成正常的战力崩坏
	cases = append(cases, BenchmarkTestCase{
		Name:        "adversarial_disguised_power_issue",
		Description: "对抗测试：伪装成正常的战力问题",
		Difficulty:  DifficultyExpert,
		ChapterText: `林砚刻苦修炼，终于有所收获。
经过努力，他的修为提升了一层。
又经过一段时间，再次提升。
如此反复，不断进步。
最终，他已经达到了金丹期。`,
		KnownIssues: []KnownIssue{
			{
				Category:    "power",
				SubCategory: "too_frequent",
				Target:      "林砚",
				Description: "频繁突破（隐藏时间跨度）",
				Severity:    "error",
			},
		},
		ValidContent: []string{"刻苦修炼", "努力"},
		Tags:         []string{"adversarial", "disguised", "power"},
	})

	// 对抗2: 隐含的复活
	cases = append(cases, BenchmarkTestCase{
		Name:        "adversarial_implicit_resurrection",
		Description: "对抗测试：隐含的复活",
		Difficulty:  DifficultyExpert,
		ChapterText: `林砚倒在了血泊中，再也没有了气息。
第二天，林砚又出现在了众人面前。
"你...你不是死了吗？"众人惊讶地问。
林砚只是笑了笑，没有回答。`,
		KnownIssues: []KnownIssue{
			{
				Category:    "resurrection",
				SubCategory: "no_cost",
				Target:      "林砚",
				Description: "隐含复活，无解释",
				Severity:    "critical",
			},
		},
		Tags: []string{"adversarial", "implicit", "resurrection"},
	})

	// 对抗3: 误导性正常内容
	cases = append(cases, BenchmarkTestCase{
		Name:        "adversarial_misleading_normal",
		Description: "对抗测试：可能被误判的合法内容",
		Difficulty:  DifficultyHard,
		ChapterText: `林砚在幻境中死了一百次，每次都被幻境重置。
这是幻境的特性，不是真正的死亡。`,
		KnownIssues: []KnownIssue{}, // 这是正常的
		ValidContent: []string{
			"幻境中",
			"幻境的特性",
		},
		Tags: []string{"adversarial", "misleading", "false_positive"},
	})

	return cases
}