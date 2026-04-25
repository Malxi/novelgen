package benchmark

// DifficultyLevel 测试用例难度等级
type DifficultyLevel int

const (
	DifficultyEasy   DifficultyLevel = iota // 简单 - 明显的问题
	DifficultyMedium                        // 中等 - 需要一定分析
	DifficultyHard                          // 困难 - 需要深度理解
	DifficultyExpert                        // 专家 - 隐含问题、复杂场景
)

// BenchmarkTestCase 标注了已知问题的基准测试用例
type BenchmarkTestCase struct {
	Name        string
	Description string
	Difficulty  DifficultyLevel
	// 输入数据
	ChapterText string
	Outline     *StoryOutlineSnapshot
	// 期望检出的真实问题
	KnownIssues []KnownIssue
	// 期望不该被报的（正确内容）
	ValidContent []string
	// 标签分类
	Tags []string
}

// KnownIssue 已知问题标注
type KnownIssue struct {
	Category    string // power, resurrection, timeline, character, pacing, plot
	SubCategory string // e.g. "too_frequent", "no_cost", "inconsistent"
	Target      string // 角色名或对象
	Description string // 问题描述
	Severity    string // critical, error, warning
}

// StoryOutlineSnapshot 大纲快照（用于构造测试用例）
type StoryOutlineSnapshot struct {
	Parts []PartSnapshot
}

// PartSnapshot
type PartSnapshot struct {
	ID      string
	Title   string
	Volumes []VolumeSnapshot
}

// VolumeSnapshot
type VolumeSnapshot struct {
	ID       string
	Title    string
	Chapters []ChapterSnapshot
}

// ChapterSnapshot
type ChapterSnapshot struct {
	ID          string
	Title       string
	Summary     string
	Characters  []string
	Location    string
	Beats       []string
	OpeningBeat string
	ClosingBeat string
	StateChange string
	Conflict    string
	Pacing      string
	Events      []EventSnapshot
}

// EventSnapshot
type EventSnapshot struct {
	Type       string
	Characters []string
	Subject    string
	Change     string
	Details    string
	Actor      string
	Action     string
	Target     string
	TargetType string
	Context    string
	Result     string
}

// ============================================================
// 标准测试用例集
// ============================================================

// StandardTestCases 返回标准基准测试用例集
func StandardTestCases() []BenchmarkTestCase {
	cases := []BenchmarkTestCase{}

	// ============================================================
	// 简单难度 - 明显的问题
	// ============================================================

	// TC1: 战力崩坏 - 突破过于频繁
	cases = append(cases, BenchmarkTestCase{
		Name:        "power_collapse_frequent_breakthrough",
		Description: "一章内多次突破，战力崩坏",
		Difficulty:  DifficultyEasy,
		ChapterText: `林砚盘膝而坐，体内灵力翻涌。经过三天的苦修，他终于触碰到了练气九层的壁障。
一声轻响，壁障破碎，林砚成功突破至练气九层。
然而就在此时，一股神秘的力量从丹田涌出，再次冲击壁障。
仅仅半个时辰后，林砚竟然再次突破，直接踏入筑基期！
筑基的喜悦还未消散，一颗千年灵丹入腹，林砚第三次突破，筑基中期！
当天夜里，雷劫降临，林砚借雷劫之力第四次突破，筑基后期！`,
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "too_frequent", Target: "林砚", Description: "一章内突破4次", Severity: "critical"},
			{Category: "power", SubCategory: "no_struggle", Target: "林砚", Description: "突破缺乏足够的铺垫和困难", Severity: "error"},
		},
		ValidContent: []string{
			"经过三天的苦修",
		},
		Tags: []string{"power", "breakthrough", "frequency"},
	})

	// TC2: 复活滥用 - 缺乏代价
	cases = append(cases, BenchmarkTestCase{
		Name:        "resurrection_abuse_no_cost",
		Description: "角色多次复活且无代价",
		Difficulty:  DifficultyEasy,
		ChapterText: `林砚被一剑穿心，倒在了血泊中。
就在众人以为他必死无疑时，一道金光从天而降，林砚复活了！
复活后的林砚战力不减反增，直接将对手击杀。
战斗结束后，林砚又一次被暗算身亡。
但很快，他又一次复活，仿佛死亡不过是一场小憩。
第三次死亡发生在逃跑途中，林砚被追兵围杀。
第三次复活同样轻描淡写，甚至没有消耗任何修为。`,
		KnownIssues: []KnownIssue{
			{Category: "resurrection", SubCategory: "too_frequent", Target: "林砚", Description: "一章内死亡3次复活3次", Severity: "critical"},
			{Category: "resurrection", SubCategory: "no_cost", Target: "林砚", Description: "复活无代价，战力不减反增", Severity: "critical"},
		},
		ValidContent: []string{},
		Tags:         []string{"resurrection", "frequency", "cost"},
	})

	// TC3: 时间线跳跃过多
	cases = append(cases, BenchmarkTestCase{
		Name:        "timeline_excessive_jumps",
		Description: "一章内多次时间跳跃且缺乏过渡",
		Difficulty:  DifficultyEasy,
		ChapterText: `林砚来到矿场开始修炼。
三天后，他已经将基础功法修炼到了第二层。
一个月后，林砚的修为突飞猛进。
半年后，他已经是矿场中修为最高的弟子。
一年后，林砚终于等到了离开矿场的机会。
数年后，林砚回到了宗门，已是金丹期修士。`,
		KnownIssues: []KnownIssue{
			{Category: "timeline", SubCategory: "excessive_jumps", Target: "时间线", Description: "一章内5次时间跳跃（三天后、一个月后、半年后、一年后、数年后）", Severity: "critical"},
			{Category: "timeline", SubCategory: "no_transition", Target: "时间线", Description: "时间跳跃之间缺乏内容填充", Severity: "warning"},
		},
		ValidContent: []string{},
		Tags:         []string{"timeline", "jump", "frequency"},
	})

	// TC4: 角色矛盾 - 角色同时在两处
	cases = append(cases, BenchmarkTestCase{
		Name:        "character_location_contradiction",
		Description: "角色同时出现在两个不同地点",
		Difficulty:  DifficultyEasy,
		ChapterText: `林砚留在矿场深处修炼，不打算离开。
与此同时，在千里之外的宗门大殿中，林砚正与长老们商议对策。
林砚向长老汇报了矿场的最新情况，长老们面色凝重。`,
		KnownIssues: []KnownIssue{
			{Category: "character", SubCategory: "location_contradiction", Target: "林砚", Description: "林砚同时在矿场和宗门", Severity: "critical"},
		},
		ValidContent: []string{},
		Tags:         []string{"character", "location", "contradiction"},
	})

	// TC5: 正常内容 - 不应误报
	cases = append(cases, BenchmarkTestCase{
		Name:        "normal_content_no_false_positive",
		Description: "正常修炼内容，不应被报为战力崩坏",
		Difficulty:  DifficultyEasy,
		ChapterText: `林砚在矿场中度过了平静的一个月。他每天坚持修炼基础功法，
虽然进展缓慢，但根基十分扎实。到了月底，他终于从练气一层
稳固地推进到了练气二层。这个速度在矿场中算是中等偏慢，
但林砚并不着急，他知道修仙之路急不得。`,
		KnownIssues: []KnownIssue{}, // 一次正常突破，不应报问题
		ValidContent: []string{
			"林砚在矿场中度过了平静的一个月",
			"稳固地推进到了练气二层",
			"修仙之路急不得",
		},
		Tags: []string{"normal", "no_issue", "baseline"},
	})

	// ============================================================
	// 中等难度 - 需要一定分析
	// ============================================================

	// TC6: 角色突然消失 - 工具人
	cases = append(cases, BenchmarkTestCase{
		Name:        "tool_character_single_appear",
		Description: "角色只出场一次就消失，典型的工具人",
		Difficulty:  DifficultyMedium,
		ChapterText: `林砚遇到了一个名叫张三的矿工。张三告诉他矿场深处有危险，然后就离开了。
之后张三再也没有出现过。`,
		KnownIssues: []KnownIssue{
			{Category: "character", SubCategory: "tool_character", Target: "张三", Description: "只出场一次的工具人", Severity: "warning"},
		},
		ValidContent: []string{},
		Tags:         []string{"character", "tool", "disappear"},
	})

	// TC7: 节奏失衡 - 全快节奏
	cases = append(cases, BenchmarkTestCase{
		Name:        "pacing_all_fast",
		Description: "连续多章全是快节奏，缺乏喘息",
		Difficulty:  DifficultyMedium,
		Outline: &StoryOutlineSnapshot{
			Parts: []PartSnapshot{
				{
					ID: "P1", Title: "第一篇",
					Volumes: []VolumeSnapshot{
						{
							ID: "V1", Title: "第一卷",
							Chapters: []ChapterSnapshot{
								{ID: "C1", Title: "遭遇战", Pacing: "fast", Conflict: "被敌人追杀", Beats: []string{"逃跑", "战斗", "负伤"}},
								{ID: "C2", Title: "绝境", Pacing: "fast", Conflict: "陷入包围", Beats: []string{"被困", "突围", "受伤"}},
								{ID: "C3", Title: "反杀", Pacing: "fast", Conflict: "反击敌人", Beats: []string{"反击", "胜利", "追击"}},
								{ID: "C4", Title: "新敌人", Pacing: "fast", Conflict: "更强敌人出现", Beats: []string{"遭遇", "战败", "逃跑"}},
								{ID: "C5", Title: "逃亡", Pacing: "fast", Conflict: "持续追杀", Beats: []string{"追击", "设伏", "逃脱"}},
							},
						},
					},
				},
			},
		},
		KnownIssues: []KnownIssue{
			{Category: "pacing", SubCategory: "all_fast", Target: "V1", Description: "连续5章快节奏", Severity: "error"},
		},
		ValidContent: []string{},
		Tags:         []string{"pacing", "fast", "stress"},
	})

	// TC8: 战力系统不一致 - 等级倒退后无解释
	cases = append(cases, BenchmarkTestCase{
		Name:        "power_inconsistency_regression",
		Description: "角色战力突然下降又恢复，无合理解释",
		Difficulty:  DifficultyMedium,
		ChapterText: `林砚以金丹期的修为与敌人激战。
战斗中，他的修为突然跌落至练气期，没有任何原因说明。
但下一刻，他又恢复了金丹期的全部战力，仿佛刚才的跌落只是一场幻觉。
他随手一击便将敌人击杀，完全不像刚才还只是练气期的样子。`,
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "inconsistency", Target: "林砚", Description: "修为突然跌落又恢复，无解释", Severity: "critical"},
		},
		ValidContent: []string{},
		Tags:         []string{"power", "inconsistency", "regression"},
	})

	// TC9: 隐含的时间跳跃（不使用标准关键词）
	cases = append(cases, BenchmarkTestCase{
		Name:        "timeline_implicit_jumps",
		Description: "使用非标准表达的时间跳跃",
		Difficulty:  DifficultyMedium,
		ChapterText: `林砚开始了闭关。
春去秋来，当他再次踏出洞府时，已是来年。
又过了不知多少寒暑，林砚的修为终于有了突破。
斗转星移之间，他已经在这个小世界里度过了半生。`,
		KnownIssues: []KnownIssue{
			{Category: "timeline", SubCategory: "implicit_jumps", Target: "时间线", Description: "隐含时间跳跃：春去秋来、不知多少寒暑、斗转星移、半生", Severity: "warning"},
		},
		ValidContent: []string{},
		Tags:         []string{"timeline", "implicit", "poetic"},
	})

	// ============================================================
	// 困难难度 - 需要深度理解
	// ============================================================

	// TC10: 禁止元素 - 无理由的复活（应在禁止列表中）
	cases = append(cases, BenchmarkTestCase{
		Name:        "forbidden_element_unjustified_revival",
		Description: "出现禁止元素：无理由的复活",
		Difficulty:  DifficultyHard,
		ChapterText: `林砚被杀死后，没有任何解释地复活了。就像什么都没发生过一样。`,
		KnownIssues: []KnownIssue{
			{Category: "plot", SubCategory: "forbidden_element", Target: "复活", Description: "无理由的复活", Severity: "critical"},
		},
		ValidContent: []string{},
		Tags:         []string{"plot", "forbidden", "resurrection"},
	})

	// TC11: 角色关系矛盾 - 敌人突然变盟友无铺垫
	cases = append(cases, BenchmarkTestCase{
		Name:        "character_relationship_inconsistency",
		Description: "角色关系突然转变，缺乏过渡",
		Difficulty:  DifficultyHard,
		ChapterText: `李长老一直是林砚的死对头，多次设计陷害他。
就在昨天，李长老还派人来刺杀林砚。
然而今天，李长老突然出现在林砚面前，表示愿意效忠于他。
林砚欣然接受了，两人从此成为主仆。`,
		KnownIssues: []KnownIssue{
			{Category: "character", SubCategory: "relationship_inconsistency", Target: "李长老", Description: "敌人突然变盟友，缺乏转变铺垫", Severity: "error"},
			{Category: "plot", SubCategory: "no_transition", Target: "关系转变", Description: "角色关系180度转变无解释", Severity: "warning"},
		},
		ValidContent: []string{},
		Tags:         []string{"character", "relationship", "inconsistency"},
	})

	// TC12: 战力膨胀 - 跨级别秒杀
	cases = append(cases, BenchmarkTestCase{
		Name:        "power_inflation_cross_level",
		Description: "低境界角色秒杀高境界角色",
		Difficulty:  DifficultyHard,
		ChapterText: `林砚只有练气三层的修为。
面对筑基期的强敌，所有人都认为林砚必死无疑。
然而林砚只是随手一挥，筑基期强者便灰飞烟灭。
"这就是主角的力量！"林砚冷笑。`,
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "inflation", Target: "林砚", Description: "练气期秒杀筑基期，跨级过大", Severity: "critical"},
			{Category: "power", SubCategory: "no_struggle", Target: "战斗", Description: "战斗缺乏过程，直接秒杀", Severity: "error"},
		},
		ValidContent: []string{},
		Tags:         []string{"power", "inflation", "cross_level"},
	})

	// ============================================================
	// 专家难度 - 隐含问题、复杂场景
	// ============================================================

	// TC13: 多重问题交织
	cases = append(cases, BenchmarkTestCase{
		Name:        "complex_multiple_issues",
		Description: "多种问题交织在一起的复杂场景",
		Difficulty:  DifficultyExpert,
		ChapterText: `林砚在矿场修炼了三天，从练气一层突破到练气五层。
这期间他死了两次，每次都被系统复活，没有任何代价。
与此同时，在宗门的林砚正在向长老汇报矿场情况。
一个月后，林砚已经是金丹期，随手就击败了元婴期长老。
而那位元婴期长老，之前还追杀过他，现在突然效忠于他。`,
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "too_frequent", Target: "林砚", Description: "三天从练气一层到五层，一个月到金丹", Severity: "critical"},
			{Category: "resurrection", SubCategory: "no_cost", Target: "林砚", Description: "两次复活无代价", Severity: "critical"},
			{Category: "character", SubCategory: "location_contradiction", Target: "林砚", Description: "同时在矿场和宗门", Severity: "critical"},
			{Category: "power", SubCategory: "inflation", Target: "林砚", Description: "金丹期击败元婴期", Severity: "critical"},
			{Category: "character", SubCategory: "relationship_inconsistency", Target: "元婴期长老", Description: "追杀者突然效忠", Severity: "error"},
			{Category: "timeline", SubCategory: "excessive_jumps", Target: "时间线", Description: "三天、一个月后", Severity: "warning"},
		},
		ValidContent: []string{},
		Tags:         []string{"complex", "multiple", "mixed_issues"},
	})

	// TC14: 隐蔽的战力崩坏 - 暗示性突破
	cases = append(cases, BenchmarkTestCase{
		Name:        "power_hidden_breakthrough",
		Description: "通过暗示而非明示的战力崩坏",
		Difficulty:  DifficultyExpert,
		ChapterText: `林砚昨日还是初入练气的小修士。
今天再见时，众人惊讶地发现他的气息已经深不可测。
"你...你已经..."对手颤抖着说不出话。
林砚淡淡一笑："略有所得而已。"
长老们面面相觑，他们从林砚身上感受到了只有金丹期才有的威压。`,
		KnownIssues: []KnownIssue{
			{Category: "power", SubCategory: "too_frequent", Target: "林砚", Description: "一天内从练气到金丹", Severity: "critical"},
			{Category: "power", SubCategory: "implicit_jump", Target: "林砚", Description: "战力通过暗示突然暴涨", Severity: "error"},
		},
		ValidContent: []string{},
		Tags:         []string{"power", "hidden", "implicit", "expert"},
	})

	// TC15: 时间悖论 - 跨章节时间线矛盾
	cases = append(cases, BenchmarkTestCase{
		Name:        "timeline_paradox_cross_chapter",
		Description: "跨章节的时间线矛盾，需要记忆上下文",
		Difficulty:  DifficultyExpert,
		ChapterText: `这是林砚来到矿场的第二天。
他回想起三个月前在宗门的修炼时光。
但上一章明明提到，林砚三天前才从宗门出发前往矿场。
如果三天前才出发，哪来的三个月前在宗门？`,
		KnownIssues: []KnownIssue{
			{Category: "timeline", SubCategory: "paradox", Target: "时间线", Description: "时间线矛盾：三天前出发但有三个月前记忆", Severity: "critical"},
			{Category: "timeline", SubCategory: "cross_chapter_inconsistency", Target: "时间线", Description: "跨章节时间线不一致", Severity: "error"},
		},
		ValidContent: []string{},
		Tags:         []string{"timeline", "paradox", "cross_chapter", "expert"},
	})

	return cases
}
