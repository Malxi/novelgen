package agents

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"novelgen/internal/models"
)

// ReviewMatrixSuggestion 是矩阵模式下被聚类的单条建议, 带来源标注。
type ReviewMatrixSuggestion struct {
	models.ReviewSuggestion
	Src        string   `json:"_src"`
	Themes     []string `json:"_themes,omitempty"`
	ReviewCore int      `json:"_review_score,omitempty"`
}

// matrixThemeKeywords 主题聚类关键词表。顺序即优先级(先命中的为主主题)。
var matrixThemeKeywords = []struct {
	Name string
	Kws  []string
}{
	{"角色动机/弧光", []string{"动机", "弧光", "成长", "双标", "反思", "执念", "行为准则", "内心", "人物", "私心", "降智", "工具人", "弧线", "恩怨", "人情", "情绪", "失控"}},
	{"中段循环/节拍器", []string{"每日", "循环", "干预", "停摆", "引擎", "中段", "静默", "消失", "新办法", "方案", "变奏", "mutation", "压力", "节拍", "闭环", "同构", "重复", "节奏", "单调"}},
	{"推演/能力边界", []string{"推演", "模拟器", "能力边界", "日志系统", "数据挖掘", "单向", "边界", "报错", "信息差"}},
	{"感情线", []string{"感情线", "情感", "温度", "升温", "试探", "互动", "名不副实", "微光", "关系线", "CP"}},
	{"伏笔/铺垫", []string{"伏笔", "回收", "埋线", "断线", "暗示", "解密", "魔门", "内门", "末日", "故事线"}},
	{"逻辑/自洽", []string{"矛盾", "自洽", "证据", "时间", "时序", "苔藓", "语义", "因果", "坐标", "地图", "巧合", "恰好", "逻辑", "漏洞", "冲突", "对称"}},
	{"爽点/弃书", []string{"弃书", "爽点", "捡宝", "声望", "高光", "兑现", "追读", "金句", "排比", "模板", "AI味", "设计感", "平均化", "安全", "机味"}},
	{"结构/商业", []string{"结构", "商业", "预期", "承诺", "卷结构", "高潮", "钩子", "付费", "连载", "读者"}},
	{"倒计时/时序", []string{"倒计时", "30天", "天数", "剩余", "时钟"}},
	{"具体性", []string{"具体", "细节", "感官", "数字", "抽象", "说明书", "旁白", "空泛", "物证"}},
}

// ClassifyReviewThemes 对一条建议的 issue+suggestion 文本做主题分类。
func ClassifyReviewThemes(text string) []string {
	var hits []string
	for _, t := range matrixThemeKeywords {
		if containsAnyKeyword(text, t.Kws) {
			hits = append(hits, t.Name)
		}
	}
	if len(hits) == 0 {
		hits = []string{"其他"}
	}
	return hits
}

func containsAnyKeyword(text string, kws []string) bool {
	for _, kw := range kws {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// AnnotateMatrixSuggestions 给合并后的建议打上主题标签。
func AnnotateMatrixSuggestions(sugs []ReviewMatrixSuggestion) {
	for i := range sugs {
		text := sugs[i].Issue + " " + sugs[i].Suggestion
		sugs[i].Themes = ClassifyReviewThemes(text)
	}
}

// StratifiedSample 按主主题分层抽样: 每主题保底 1 条, 随机补足到 n 条。
// seed 控制随机性以便复现。返回抽样后的下标列表。
func StratifiedSample(sugs []ReviewMatrixSuggestion, n int, seed int64) []int {
	rng := rand.New(rand.NewSource(seed))
	byTheme := map[string][]int{}
	for idx, s := range sugs {
		primary := "其他"
		if len(s.Themes) > 0 {
			primary = s.Themes[0]
		}
		byTheme[primary] = append(byTheme[primary], idx)
	}
	var picked []int
	var remaining []int
	// 主题名排序保证确定性
	themeNames := make([]string, 0, len(byTheme))
	for name := range byTheme {
		themeNames = append(themeNames, name)
	}
	sort.Strings(themeNames)
	for _, theme := range themeNames {
		idxs := byTheme[theme]
		if len(idxs) > 0 {
			picked = append(picked, idxs[0])
			remaining = append(remaining, idxs[1:]...)
		}
	}
	for len(picked) < n && len(remaining) > 0 {
		pos := rng.Intn(len(remaining))
		picked = append(picked, remaining[pos])
		remaining = append(remaining[:pos], remaining[pos+1:]...)
	}
	sort.Ints(picked)
	return picked
}

// BuildRationalizePrompt 从抽样建议生成合理化裁决 prompt 文本。
func BuildRationalizePrompt(items []ReviewMatrixSuggestion) string {
	var sb strings.Builder
	sb.WriteString("以下是从多次独立审查中抽取的大纲修改建议。请对它们进行'合理化'裁决，输出整合后的终审建议：\n")
	sb.WriteString("1) 合并内容重叠或指向同一问题的建议（保留实质，去掉重复措辞）；\n")
	sb.WriteString("2) 对互相冲突的建议必须做出取舍裁决，并说明取舍理由，禁止和稀泥式融合；\n")
	sb.WriteString("3) 拒绝明显低价值、不可操作或空泛的建议；\n")
	sb.WriteString("4) 每条终审建议必须保留 target_id/目标位置、具体问题和可操作的修改方向；\n")
	sb.WriteString("5) 终审建议总数不超过 8 条，按优先级排序。\n\n")
	sb.WriteString("原始建议列表：\n")
	for i, it := range items {
		tgt := it.TargetName
		if tgt == "" {
			tgt = it.TargetID
		}
		if tgt == "" {
			tgt = "?"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n", i+1, it.Priority, tgt, it.Issue))
		if it.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("   建议: %s\n", it.Suggestion))
		}
	}
	return sb.String()
}
