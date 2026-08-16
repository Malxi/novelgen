package style

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// AIFlavorResult reports deterministic "AI-flavor" signals in prose.
type AIFlavorResult struct {
	Score       int
	HasIssue    bool
	Issues      []string
	Suggestions []string
	Matches     []AIFlavorMatch
}

type AIFlavorMatch struct {
	Rule    string
	Example string
	Count   int
}

type phraseRule struct {
	Name       string
	Phrases    []string
	Suggestion string
	Weight     int
}

var phraseRules = []phraseRule{
	{
		Name:       "抽象总结/意义拔高",
		Phrases:    []string{"这不仅是", "不仅仅是", "更是", "这意味着", "这标志着", "充分体现", "深刻体现", "彰显了", "赋予了", "毋庸置疑", "不可否认"},
		Suggestion: "删掉抽象总结和意义拔高句，把含义落到角色动作、感官细节、具体选择和后果里。",
		Weight:     8,
	},
	{
		Name:       "论文/公文连接词",
		Phrases:    []string{"与此同时", "换言之", "总而言之", "综上所述", "从某种意义上说", "值得注意的是", "需要指出的是", "因此可以说"},
		Suggestion: "减少论文式连接词，用场景动作和对话自然转场，不要替读者总结逻辑。",
		Weight:     7,
	},
	{
		Name:       "模板化情绪词",
		Phrases:    []string{"复杂的情绪", "难以言喻", "无法形容", "前所未有", "内心深处", "某种力量", "命运的齿轮", "空气仿佛凝固"},
		Suggestion: "把模板化情绪词改成可见的身体反应、微动作、环境触发和人物独有的表达。",
		Weight:     6,
	},
	{
		Name:       "空泛强度副词",
		Phrases:    []string{"极其", "非常", "无比", "格外", "显得格外", "令人震撼", "令人窒息", "深深地", "彻底地"},
		Suggestion: "少用空泛强度副词，改用更准确的动词、名词和具体画面承担力度。",
		Weight:     4,
	},
	{
		Name:       "时间强调词滥用",
		Phrases:    []string{"那一刻", "此刻", "就在这时", "下一瞬", "下一秒", "忽然", "突然", "骤然"},
		Suggestion: "“那一刻/此刻/忽然/突然”每章超过 3 次即显机械，删掉大部分，让事件因果自然衔接，或用具体动作替代。",
		Weight:     3,
	},
	{
		Name:       "心理描写AI腔",
		Phrases:    []string{"他不禁想", "他忍不住想", "他暗自思忖", "某种说不清道不明的情绪", "一股莫名的不安", "一种不祥的预感", "内心翻涌", "五味杂陈", "百感交集", "深吸一口气，平复"},
		Suggestion: "把心理总结句改成可见的身体反应、微动作和具体画面，不要直接报情绪名词。",
		Weight:     5,
	},
	{
		Name:       "网文套路化形容",
		Phrases:    []string{"心头一凛", "心头一沉", "心头一震", "眸中闪过一道精光", "眼中闪过一丝", "嘴角勾起一抹", "眼神深邃", "一股寒意", "一股暖流", "冷汗直冒", "头皮发麻"},
		Suggestion: "“心头一凛/眸中精光/嘴角弧度/一股寒意”是网文 AI 高频套路，换成角色独有的反应方式。",
		Weight:     4,
	},
	{
		Name:       "转折强调模板",
		Phrases:    []string{"不是……而是", "这不是", "更重要的是", "真正重要的是", "关键在于", "说到底", "本质上"},
		Suggestion: "“不是…而是…/更重要的是/关键在于”式强调句 AI 最爱，一段内用两次必是机味；删掉或让情节自己说话。",
		Weight:     5,
	},
	{
		Name:       "金手指失灵措辞",
		Phrases:    []string{"乱码", "糊成碎片", "刷新不回", "提示失效", "系统坏了", "出bug", "出故障", "低能耗", "被屏蔽", "故障了", "宕机"},
		Suggestion: "“乱码/糊成碎片/提示失效”等金手指故障措辞违背“金手指从不失灵”铁律——信息获取不到只能是信息差（环境特性/对手反制/物理痕迹），不是系统坏。改成“该区域暂无日志/查无此条/信息天然残缺”。",
		Weight:     10,
	},
}

var (
	reBalancedNotOnly = regexp.MustCompile(`不仅[^。！？\n]{0,30}(而且|还|更|也)`)
	reAsA             = regexp.MustCompile(`作为[^，。！？\n]{1,18}[，,]`)
	reLongSentence    = regexp.MustCompile(`[。！？!?]`)
	// 不是X，是Y 短句转折 (AI 强调句母句式): 不是+2-8字+，是+2-10字
	reNotIsShort = regexp.MustCompile(`不是[^，。！？；\n]{2,8}，是[^，。！？；\n]{2,10}`)
	// 排除合理因果: 不是因为...而是因为 (正常归因)
	reNotBecause = regexp.MustCompile(`不是因为[^，。！？；\n]{1,15}，而是因为`)
)

// CheckAIFlavor detects formulaic prose patterns and returns an editing-oriented result.
func CheckAIFlavor(text string, threshold int) AIFlavorResult {
	if threshold <= 0 {
		threshold = 75
	}
	result := AIFlavorResult{Score: 100}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return result
	}

	penalty := 0
	for _, rule := range phraseRules {
		count, example := countAny(clean, rule.Phrases)
		if count == 0 {
			continue
		}
		penalty += count * rule.Weight
		result.Matches = append(result.Matches, AIFlavorMatch{Rule: rule.Name, Example: example, Count: count})
		result.Issues = append(result.Issues, fmt.Sprintf("%s命中 %d 次，例如「%s」", rule.Name, count, example))
		result.Suggestions = appendUnique(result.Suggestions, rule.Suggestion)
	}

	if count := len(reBalancedNotOnly.FindAllString(clean, -1)); count > 0 {
		penalty += count * 10
		result.Matches = append(result.Matches, AIFlavorMatch{Rule: "不仅/而且平衡句", Example: firstMatch(reBalancedNotOnly, clean), Count: count})
		result.Issues = append(result.Issues, fmt.Sprintf("检测到 %d 处“不仅/而且/更是”式平衡句，容易显得模板化", count))
		result.Suggestions = appendUnique(result.Suggestions, "拆掉“不仅/而且/更是”式平衡句，改成角色当下能感知到的一到两个具体变化。")
	}

	// 不是X，是Y 短句转折 (AI 强调句母句式), 排除"不是因为...而是因为"正常归因
	notIsMatches := reNotIsShort.FindAllString(clean, -1)
	notIsCount := 0
	example := ""
	for _, m := range notIsMatches {
		if reNotBecause.MatchString(m) {
			continue
		}
		notIsCount++
		if example == "" {
			example = m
		}
	}
	if notIsCount > 0 {
		penalty += notIsCount * 5
		result.Matches = append(result.Matches, AIFlavorMatch{Rule: "不是X，是Y短句转折", Example: firstMatch(reNotIsShort, clean), Count: notIsCount})
		result.Issues = append(result.Issues, fmt.Sprintf("检测到 %d 处“不是X，是Y”短句转折（如“不是他慢，是他快”），AI 强调句痕迹明显", notIsCount))
		result.Suggestions = appendUnique(result.Suggestions, "“不是X，是Y”短句转折（如“不是他慢，是他快”）改成直接陈述或动作呈现，一句最多保留一次。")
	}

	if count := len(reAsA.FindAllString(clean, -1)); count > 2 {
		penalty += (count - 2) * 4
		result.Matches = append(result.Matches, AIFlavorMatch{Rule: "作为句式过密", Example: firstMatch(reAsA, clean), Count: count})
		result.Issues = append(result.Issues, fmt.Sprintf("“作为……”句式出现 %d 次，叙述声音偏说明文", count))
		result.Suggestions = appendUnique(result.Suggestions, "把“作为……”身份说明改为人物行为、称呼、环境反应或对话中的自然信息。")
	}

	longCount := countLongSentences(clean, 90)
	if longCount > 2 {
		penalty += (longCount - 2) * 5
		result.Issues = append(result.Issues, fmt.Sprintf("超过 90 字的长句有 %d 句，节奏容易发硬", longCount))
		result.Suggestions = appendUnique(result.Suggestions, "拆分过长句，穿插短句、动作和对白，让段落节奏更像人工修过的小说正文。")
	}

	result.Score = 100 - penalty
	if result.Score < 0 {
		result.Score = 0
	}
	result.HasIssue = result.Score < threshold

	sort.SliceStable(result.Matches, func(i, j int) bool {
		return result.Matches[i].Count > result.Matches[j].Count
	})
	return result
}

func countAny(text string, phrases []string) (int, string) {
	total := 0
	example := ""
	for _, phrase := range phrases {
		count := strings.Count(text, phrase)
		if count == 0 {
			continue
		}
		total += count
		if example == "" {
			example = phrase
		}
	}
	return total, example
}

func firstMatch(re *regexp.Regexp, text string) string {
	match := re.FindString(text)
	if utf8.RuneCountInString(match) > 28 {
		runes := []rune(match)
		return string(runes[:28])
	}
	return match
}

func countLongSentences(text string, limit int) int {
	count := 0
	start := 0
	for _, loc := range reLongSentence.FindAllStringIndex(text, -1) {
		if utf8.RuneCountInString(strings.TrimSpace(text[start:loc[0]])) > limit {
			count++
		}
		start = loc[1]
	}
	if start < len(text) && utf8.RuneCountInString(strings.TrimSpace(text[start:])) > limit {
		count++
	}
	return count
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}
