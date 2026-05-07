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
}

var (
	reBalancedNotOnly = regexp.MustCompile(`不仅[^。！？\n]{0,30}(而且|还|更|也)`)
	reAsA             = regexp.MustCompile(`作为[^，。！？\n]{1,18}[，,]`)
	reLongSentence    = regexp.MustCompile(`[。！？!?]`)
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
