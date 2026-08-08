package agents

// ReviewFocus 定义一组内置审查 focus，供 compose review --focus 使用。
// 每个 focus 是一段审查指令文本，会被拼进 user prompt 作为审查重点。
// 与 --prompt 的区别：--prompt 是用户自由输入，--focus 是命名内置视角，
// 可逗号分隔组合（如 --focus logic,deai）或 --focus all 全开。
type ReviewFocus struct {
	Name        string // 英文短名，如 "deai"
	DisplayName string // 中文显示名
	Prompt      string // 审查指令文本
}

var reviewFocuses = []ReviewFocus{
	{
		Name:        "reader",
		DisplayName: "毒舌读者(节奏/爽点/弃书)",
		Prompt: `你是一位毒舌的网文老读者，专看玄幻修仙文，对套路极其敏感，看到降智剧情会摔手机。本次审查请完全从读者体验出发，聚焦：
1. 节奏：哪里拖沓、哪里仓促、信息密度是否失衡
2. 爽点：爽点是否到位、是否利用本书独特设定、有没有泛泛打赢捡宝
3. 弃书点：哪个位置读者会弃书，为什么
忽略设定细节和逻辑严谨性，专注阅读体验。给出具体到章节的尖锐建议。`,
	},
	{
		Name:        "logic",
		DisplayName: "逻辑洁癖(因果/自洽/证据链)",
		Prompt: `你是一位逻辑洁癖的剧情编辑，最恨因果断裂和设定漏洞。本次审查请完全聚焦逻辑层面：
1. 因果链：每个关键事件是否有合理的因果支撑，还是"需要时天上掉"
2. 伏笔回收：埋了的线有没有收，收得合不合理
3. 设定自洽：角色行为是否违背已确立的规则（能力边界、时间线、力量体系）
4. 信息差博弈：各方的信息差是否成立、会不会被轻易戳破
忽略文笔和节奏，专注逻辑闭环。给出具体到章节的证据链问题。`,
	},
	{
		Name:        "character",
		DisplayName: "人物弧光(动机/成长/关系)",
		Prompt: `你是一位人物弧光教练，最擅长把工具人角色救活。本次审查请完全聚焦人物层面：
1. 动机可信度：每个核心角色为什么这么做，动机是否前后一致、令人信服
2. 成长轨迹：角色有没有清晰可见的变化弧光，还是原地踏步
3. 关系线：人物互动是否推动剧情、有没有情感温度，关系线是否在推进
4. 反派塑造：对位角色的动机和行动是否成立
忽略剧情结构，专注人物。给出具体到角色的弧光建议。`,
	},
	{
		Name:        "commercial",
		DisplayName: "商业编辑(结构/预期/追读)",
		Prompt: `你是一位商业网文编辑，操盘过多个连载项目，对读者预期和付费点极其敏感。本次审查请完全聚焦商业与结构层面：
1. 卷结构：三幕/卷内起承转合是否成立，高潮位置对不对
2. 读者预期：卷首的承诺（如卷名、部名）卷末有没有兑现
3. 升级节奏：主角实力/局面是否持续向上，适合不适合连载追读
4. payoff 兑现：卷级 payoff_contract、章节 chapter_payoff 是否落地，爽点是否密集分布
5. 钩子：卷末钩子能不能勾住读者点下一卷
忽略文笔细节，专注商业价值和结构。`,
	},
	{
		Name:        "storyline",
		DisplayName: "故事线追踪(推进/回收/信息差)",
		Prompt: `你是一位故事线追踪者，负责盯住每条 story 线的推进与回收。本次审查请完全聚焦故事线与信息差层面：
1. 故事线推进：setup 里每条 storyline 在本卷有没有实质推进，还是占位
2. 压力累积：repeatable_pressure、payoff_cadence、mutation、failure_mode 等设定是否被兑现
3. 信息差博弈：日志系统带来的信息差是否形成有效博弈，双向还是单向
4. 被遗忘的线：有没有埋了但卷内完全没动的线
忽略单章细节，专注线的推进。给出具体到卷/章的故事线状态建议。`,
	},
	{
		Name:        "deai",
		DisplayName: "去AI味(模板化/降智/金句/说明书)",
		Prompt: `你是一位"去AI味"专项审查官，本职工作是识别并清除 AI 生成内容特有的"机味"——那种一眼就能看出不是人写的、缺乏人类直觉与生命质感的东西。人类写手可能写得烂，但烂得有人的体温；AI 写手写得再工整，也工整得让人起鸡皮疙瘩。本次审查请完全聚焦 AI 味，逐项排查：
1. 模板化结构：章节是否每一章都完美走完"目标-冲突-解决-钩子"的循环，毫无意外、毫无粗糙感？节奏是否均匀得像节拍器？
2. 平均化与安全化：所有情节是否都"恰到好处"？有没有真正冒险、真正失控的设计？主角是否从不做出不理智但真实的选择？
3. 过度工整的伏笔：伏笔回收是否完美对称、每件事都有前因后果？现实故事有断线、有遗忘，大纲是否缺乏这种粗粝感？
4. 情绪正确性：角色的反应是否总是"合理"的？是否没有人会因为偏见、冲动、面子、误会而做蠢事？
5. 具体性缺失：描述是否抽象而空洞？缺乏可感知的细节锚点（具体物件、具体数字、具体感官信息）？
6. 设计感过重：每个角色是否都精准服务于剧情？每场戏是否都有明确功能，没有一次漫无目的的闲笔？
7. 金句与排比：summary/章节标题是否充满对仗、排比、工整的总结感？
8. 强行降智：是否为了让主角赢或制造冲突，把对手/配角写成傻子？
9. 世界观说明书：设定信息是否以说明文形式倾倒，而不是通过事件自然透出？
输出要求：每条建议必须指出具体章节、具体的"AI 味"现象，并给出让这段大纲更像人写的修改方向。不要泛泛而谈"增强真实感"这种空话。`,
	},
}

// findReviewFocus 按名称查找内置 focus。
func findReviewFocus(name string) (ReviewFocus, bool) {
	for _, f := range reviewFocuses {
		if f.Name == name {
			return f, true
		}
	}
	return ReviewFocus{}, false
}

// allReviewFocusPrompt 返回所有 focus 的审查指令合并文本（--focus all 用）。
func allReviewFocusPrompt() string {
	var sb string
	for i, f := range reviewFocuses {
		if i > 0 {
			sb += "\n\n"
		}
		sb += f.Prompt
	}
	return sb
}

// ResolveReviewFocusPrompt 解析 --focus 参数，返回拼好的审查指令文本。
// focus 可为逗号分隔的多个名字，或 "all"（全部）。未知名字会被跳过。
func ResolveReviewFocusPrompt(focusFlag string) string {
	names := splitFocusNames(focusFlag)
	if len(names) == 0 {
		return ""
	}
	if containsFocusName(names, "all") {
		return allReviewFocusPrompt()
	}
	var parts []string
	for _, n := range names {
		if f, ok := findReviewFocus(n); ok {
			parts = append(parts, f.Prompt)
		}
	}
	var sb string
	for i, p := range parts {
		if i > 0 {
			sb += "\n\n"
		}
		sb += p
	}
	return sb
}

func splitFocusNames(s string) []string {
	var out []string
	var cur string
	for _, r := range s {
		if r == ',' {
			if cur = trimSpace(cur); cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur = trimSpace(cur); cur != "" {
		out = append(out, cur)
	}
	return out
}

func containsFocusName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// ListReviewFocusNames 返回所有可用 focus 名称（--help 提示用）。
func ListReviewFocusNames() []string {
	names := make([]string, 0, len(reviewFocuses))
	for _, f := range reviewFocuses {
		names = append(names, f.Name)
	}
	return names
}
