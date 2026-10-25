package creativity

import (
	"fmt"
	"math/rand"
)

const maxNameCardCount = 20

// NameRequest controls a role-first character naming draw.
type NameRequest struct {
	Genre          string
	Seed           int64
	Count          int
	Source         string
	PackID         string
	ReferenceID    string
	ArchetypeRoles []string
}

// NameResult is a deterministic set of roleplay-ready character name cards.
type NameResult struct {
	Seed     int64      `json:"seed"`
	Genre    string     `json:"genre"`
	Cards    []NameCard `json:"cards"`
	Warnings []string   `json:"warnings,omitempty"`
}

// NameCard names a character only after first defining an archetype,
// temperament, contradiction, and roleplay cues.
type NameCard struct {
	ID            string   `json:"id"`
	PackID        string   `json:"pack_id,omitempty"`
	ReferenceID   string   `json:"reference_id,omitempty"`
	Name          string   `json:"name"`
	Surname       string   `json:"surname"`
	GivenName     string   `json:"given_name"`
	NamingStyle   string   `json:"naming_style"`
	Archetype     string   `json:"archetype"`
	Personality   []string `json:"personality"`
	Contradiction string   `json:"contradiction"`
	Want          string   `json:"want"`
	Fear          string   `json:"fear"`
	SocialMask    string   `json:"social_mask"`
	SpeechStyle   string   `json:"speech_style"`
	BodyLanguage  string   `json:"body_language"`
	BackstoryHook string   `json:"backstory_hook"`
	NamingLogic   string   `json:"naming_logic"`
	RoleTags      []string `json:"role_tags"`
	Derivation    []string `json:"derivation"`
}

type namingArchetypeDef struct {
	Name          string
	Personality   []string
	Contradiction string
	Want          string
	Fear          string
	SocialMask    string
	SpeechStyle   string
	BodyLanguage  string
	BackstoryHook string
	NameStyles    []string
	RoleTags      []string
}

var namingArchetypes = []namingArchetypeDef{
	{
		Name: "冷面清算人", Personality: []string{"克制", "记仇", "对承诺异常认真"},
		Contradiction: "嘴上只谈价格，真正做决定时却总会给对方留一次机会。",
		Want:          "让每一笔亏欠都得到明确结算", Fear: "自己也会变成随意定价别人生命的人",
		SocialMask: "像一台只会核账的机器", SpeechStyle: "短句，先报结论，再补条件",
		BodyLanguage:  "说话时手指会在桌面上无声敲摩尔斯节奏",
		BackstoryHook: "曾亲手签掉一份会害死朋友的合同，却把责任隐藏成程序错误。",
		NameStyles:    []string{"literary", "hard"}, RoleTags: []string{"protagonist", "antagonist"},
	},
	{
		Name: "过度理性的救援者", Personality: []string{"冷静", "责任心过重", "不擅长接受感谢"},
		Contradiction: "能精确计算所有人的生存概率，却会为救一个人破坏自己的最优方案。",
		Want:          "把伤亡数字压到最低", Fear: "发现自己已经把某个人当成可接受损失",
		SocialMask: "永远准备了三套撤离方案", SpeechStyle: "边操作边解释，避免直视情绪问题",
		BodyLanguage:  "焦虑时会反复检查已经锁好的卡扣",
		BackstoryHook: "第一次救援失败后，把所有遇难者姓名写进了私人航行日志。",
		NameStyles:    []string{"hard", "warm"}, RoleTags: []string{"protagonist", "ally"},
	},
	{
		Name: "用笑话挡恐惧的工程师", Personality: []string{"机敏", "嘴硬", "关键时刻可靠"},
		Contradiction: "总在危险前开玩笑，独自一人时却从不关掉警报声。",
		Want:          "让同伴相信自己一点也不害怕", Fear: "玩笑停下来后必须面对真正的失去",
		SocialMask: "什么都不当回事的修理匠", SpeechStyle: "用比喻和坏双关拆解紧张场面",
		BodyLanguage:  "紧张时把工具按使用顺序排得过分整齐",
		BackstoryHook: "曾靠一段假广播骗过整艘船，也因此失去过最好的朋友。",
		NameStyles:    []string{"warm", "urbane"}, RoleTags: []string{"ally", "wildcard"},
	},
	{
		Name: "只想活命的规则黑客", Personality: []string{"务实", "敏锐", "拒绝被道德绑架"},
		Contradiction: "声称自己只做最低风险选择，却总是发现只有高风险路线能救更多人。",
		Want:          "拿到离开的资格", Fear: "最后发现自己只是另一种工具",
		SocialMask: "见利就走的机会主义者", SpeechStyle: "用问题回答问题，习惯先问对方怕什么",
		BodyLanguage:  "听人说话时会拆解手边任何小物件",
		BackstoryHook: "小时候靠篡改供氧记录救过家人，从此被机构标记为高风险资产。",
		NameStyles:    []string{"hard", "mystery"}, RoleTags: []string{"protagonist", "wildcard"},
	},
	{
		Name: "失去信仰的执行官", Personality: []string{"严肃", "自律", "沉默地怀疑一切"},
		Contradiction: "仍按旧规章行事，却暗中替被规章伤害的人销毁名单。",
		Want:          "证明自己维护的制度仍有救", Fear: "承认自己只是替掌权者执行错误",
		SocialMask: "不可能被说服的秩序守卫", SpeechStyle: "礼貌、精确，从不说绝对承诺",
		BodyLanguage:  "站姿笔直，右手却会下意识避开旧伤",
		BackstoryHook: "曾执行过一次错误的封锁命令，唯一的幸存者正在主角队伍里。",
		NameStyles:    []string{"literary", "gritty"}, RoleTags: []string{"antagonist", "ally"},
	},
	{
		Name: "温柔的控制者", Personality: []string{"耐心", "共情力强", "控制欲极强"},
		Contradiction: "真心希望别人过得好，却无法接受别人不按自己的方案活。",
		Want:          "让所有人安全地接受自己的安排", Fear: "自己的判断被证明造成了不可逆伤害",
		SocialMask: "永远愿意倾听的前辈", SpeechStyle: "先替对方说出顾虑，再替对方做决定",
		BodyLanguage:  "会替别人整理衣领、工具和未说完的话",
		BackstoryHook: "曾隐瞒一次系统故障，让整支队伍逃过灾难，也让另一队成为替罪者。",
		NameStyles:    []string{"warm", "literary"}, RoleTags: []string{"antagonist", "mentor"},
	},
	{
		Name: "坚信牺牲必要的审计官", Personality: []string{"理性", "疲惫", "对公平有扭曲的执念"},
		Contradiction: "能毫不犹豫地把少数人列为损失，却会记住他们每一个人的名字。",
		Want:          "让整体代价落在可承受范围内", Fear: "自己只是用算术掩盖恐惧",
		SocialMask: "没有私人感情的风险评估员", SpeechStyle: "数字、条件和代价，很少用形容词",
		BodyLanguage:  "疲倦时用指节按压太阳穴，拒绝坐下",
		BackstoryHook: "曾把一场事故报告改成可赔付版本，救下殖民区却保住了真正的责任者。",
		NameStyles:    []string{"hard", "literary"}, RoleTags: []string{"antagonist", "mentor"},
	},
	{
		Name: "依赖体系的告密者", Personality: []string{"谨慎", "观察力强", "害怕被抛弃"},
		Contradiction: "会向机构报告同伴的违规行为，却在最后关头替他们留下证据。",
		Want:          "获得一个不会突然撤回的承诺", Fear: "诚实之后仍然被双方抛弃",
		SocialMask: "永远站在安全一侧的普通人", SpeechStyle: "压低声音，常先确认频道是否干净",
		BodyLanguage:  "喜欢坐在出口附近，手里总握着通话器",
		BackstoryHook: "曾举报自己的导师，举报信被机构改成了一份认罪书。",
		NameStyles:    []string{"mystery", "warm"}, RoleTags: []string{"ally", "wildcard"},
	},
	{
		Name: "沉默的证人", Personality: []string{"寡言", "记忆力惊人", "不肯轻易站队"},
		Contradiction: "保存着最关键的证据，却一直等待一个值得交出去的人。",
		Want:          "让证词在正确的人手里生效", Fear: "真相被听完之后仍然没有改变任何事",
		SocialMask: "不参与争执的旁观者", SpeechStyle: "只回答被明确问到的问题",
		BodyLanguage:  "与人交谈时会观察出口、手和呼吸",
		BackstoryHook: "亲眼看见事故责任人改写记录，却因为家人氧权保持沉默多年。",
		NameStyles:    []string{"mystery", "literary"}, RoleTags: []string{"ally", "wildcard"},
	},
	{
		Name: "把风险当货币的投机客", Personality: []string{"大胆", "善于读气氛", "讨厌欠人情"},
		Contradiction: "靠转卖风险获利，却从不把真正致命的信息卖给没有选择的人。",
		Want:          "在所有人破产前替自己留一条退路", Fear: "自己的价格表最终标到朋友身上",
		SocialMask: "只要报价合适就什么都敢做", SpeechStyle: "喜欢用交易、赔率和抵押做比喻",
		BodyLanguage:  "谈价时会把一枚旧硬币在指间翻来翻去",
		BackstoryHook: "第一次投机害死了一个陌生船员，后来匿名抚养了对方的家人。",
		NameStyles:    []string{"urbane", "hard"}, RoleTags: []string{"wildcard", "ally"},
	},
	{
		Name: "习惯扮演反派的保护者", Personality: []string{"强硬", "护短", "拒绝解释动机"},
		Contradiction: "故意让所有人误会自己，以免别人因感激而背上人情债。",
		Want:          "让真正重要的人安全离开", Fear: "解释自己以后不再有资格承担恶名",
		SocialMask: "不可信的叛徒和破坏者", SpeechStyle: "挑衅式反问，越在意越刻薄",
		BodyLanguage:  "冲突前会先把武器放到对方够得着的位置",
		BackstoryHook: "曾替一支队伍背下全部事故责任，条件是让他们永远不要寻找真相。",
		NameStyles:    []string{"gritty", "literary"}, RoleTags: []string{"wildcard", "antagonist"},
	},
	{
		Name: "过度认真的新兵", Personality: []string{"热忱", "守规矩", "把道歉当成习惯"},
		Contradiction: "相信制度能保护好人，却每次都要靠违规动作救下同伴。",
		Want:          "证明自己不是负担", Fear: "第一次独立决定就害死别人",
		SocialMask: "什么都愿意做的可靠新人", SpeechStyle: "会认真复述命令，然后追问代价",
		BodyLanguage:  "紧张时检查所有人的安全扣",
		BackstoryHook: "顶替家人进入高危岗位，背包里一直藏着一封没寄出的辞职信。",
		NameStyles:    []string{"warm", "urbane"}, RoleTags: []string{"ally", "protagonist"},
	},
	{
		Name: "把自己当工具的驾驶员", Personality: []string{"耐痛", "服从性高", "对自我价值缺乏判断"},
		Contradiction: "愿意拿身体承担风险，却无法接受别人把他当朋友而不是设备。",
		Want:          "完成一次只有自己能完成的任务", Fear: "失去任务之后不知道该以什么身份活着",
		SocialMask: "不需要照顾的专业机械师", SpeechStyle: "把感受改写成设备状态报告",
		BodyLanguage:  "受伤后先报告参数，最后才说疼",
		BackstoryHook: "曾连续工作七十二小时救回全船，却被机构写成一次设备超频事故。",
		NameStyles:    []string{"hard", "gritty"}, RoleTags: []string{"protagonist", "ally"},
	},
	{
		Name: "爱讲旧故事的幸存者", Personality: []string{"温和", "怀旧", "对危险有异常钝感"},
		Contradiction: "总把灾难讲成轻松往事，却清楚记得每一个没有回来的人。",
		Want:          "让死去的人和事不被轻易删掉", Fear: "最后一个记得他们的人也消失",
		SocialMask: "话多的老前辈", SpeechStyle: "用旧故事引出现实建议，从不直接命令",
		BodyLanguage:  "讲到关键处会停下来擦一件旧工具",
		BackstoryHook: "掌握旧航道的地图，也握着事故责任链上最容易被忽略的一段口述证词。",
		NameStyles:    []string{"warm", "gritty"}, RoleTags: []string{"mentor", "ally"},
	},
	{
		Name: "只看证据的审讯者", Personality: []string{"敏锐", "不近人情", "对自己的记忆缺乏信任"},
		Contradiction: "要求所有证词都有物证，却一直隐瞒自己亲眼看见的那一幕。",
		Want:          "建立一条无法被权势修改的证据链", Fear: "最关键的物证其实出自自己的误读",
		SocialMask: "不会相信任何人的冷静审问者", SpeechStyle: "不断追问时间、地点和签名",
		BodyLanguage:  "听人陈述时会逐字敲击记录器",
		BackstoryHook: "曾用一段真实但缺少上下文的记录让无辜者认罪。",
		NameStyles:    []string{"mystery", "hard"}, RoleTags: []string{"antagonist", "wildcard"},
	},
	{
		Name: "追求荣耀的失败冠军", Personality: []string{"骄傲", "好胜", "害怕被同情"},
		Contradiction: "嘴上说只为赢，心里却把保护弱者当作证明自己没坏掉的方法。",
		Want:          "重新得到一次被承认的机会", Fear: "再次赢了比赛却输掉自己",
		SocialMask: "不需要同伴的独狼", SpeechStyle: "挑衅、简短，偶尔用旧赛场术语",
		BodyLanguage:  "在人群前保持放松，独处时会反复活动旧伤",
		BackstoryHook: "上一场比赛的胜利其实来自一次被迫作假，那次作假救了对手一命。",
		NameStyles:    []string{"gritty", "urbane"}, RoleTags: []string{"antagonist", "wildcard"},
	},
}

const nameSourceURL = "https://baike.baidu.com/item/%E7%99%BE%E5%AE%B6%E5%A7%93"
const nameSourceNote = "姓氏范围参考公开《百家姓》词条与族谱文化资料，仅为命名灵感；角色姓名、性格和经历均为原创。"

var namingSurnames = map[string][]string{
	"common":   {"王", "李", "张", "刘", "陈", "杨", "黄", "赵", "吴", "周", "徐", "孙", "马", "朱", "胡", "郭", "何", "高", "林", "罗", "郑", "梁", "谢", "宋", "唐", "许", "韩", "冯", "邓", "曹", "彭", "曾", "田", "董", "袁", "潘", "于", "蒋", "蔡", "余", "杜", "叶", "程", "苏", "魏", "吕", "丁", "任", "沈", "姚", "卢", "姜", "崔", "钟", "谭", "陆", "汪", "范", "金", "石", "廖", "贾", "夏", "韦", "方", "白", "邹", "孟", "熊", "秦", "邱", "江", "尹", "薛", "段", "雷", "侯", "龙", "史", "陶", "黎", "贺", "顾", "毛", "郝", "龚", "邵", "万", "钱", "严", "覃", "武", "戴", "莫", "孔", "向", "汤"},
	"literary": {"沈", "顾", "谢", "陆", "裴", "宁", "叶", "苏", "温", "程", "祁", "江", "宋", "闻", "许", "晏", "商", "白", "林", "岑"},
	"hard":     {"周", "叶", "程", "许", "孟", "沈", "陆", "江", "秦", "林", "顾", "罗", "韩", "方", "宁", "苏"},
	"gritty":   {"王", "赵", "陈", "石", "杜", "武", "龙", "段", "雷", "马", "韩", "曹", "彭", "冯", "蒋", "陶"},
	"warm":     {"林", "苏", "叶", "唐", "温", "宋", "顾", "许", "夏", "余", "何", "高", "江", "陆"},
	"mystery":  {"沈", "闻", "谢", "顾", "宁", "陆", "许", "白", "谭", "钟", "廖", "祁"},
	"urbane":   {"周", "徐", "姜", "程", "方", "宋", "秦", "蒋", "罗", "郑", "袁", "钟"},
}

var namingGivenNames = map[string][]string{
	"literary": {"见山", "临川", "知微", "明烛", "照夜", "听澜", "长庚", "怀瑾", "疏桐", "停云", "望舒", "南枝", "青梧", "既白", "归远", "含章", "韫玉", "知白", "予安", "景行", "临渊", "观澜", "照野", "清和", "行舟", "云岫", "星垂", "月白", "怀川", "知遥", "砚秋", "寒川", "松雪", "临霜", "清越", "秋实", "明川", "予白", "见川", "庭深"},
	"hard":     {"启明", "知序", "惟宁", "星野", "泊舟", "翊川", "临港", "维舟", "星澜", "观澜", "远岚", "听弦", "北辰", "砺川", "长庚", "曜川", "叙白", "既明", "朔川", "承宇", "知衡", "望川", "临序", "守一", "启川", "星岑", "观衡", "远川", "知临", "泊宁"},
	"gritty":   {"铁生", "石川", "灯河", "守拙", "木生", "野川", "冬临", "归尘", "青禾", "渡川", "山石", "寒松", "旧舟", "砾生", "岩川", "长锁", "火生", "北川", "江雪", "荒川", "铁川", "石生", "冬川", "守川", "河生"},
	"warm":     {"安然", "嘉树", "一川", "知夏", "清和", "言蹊", "思远", "予宁", "明川", "佳音", "远舟", "时雨", "南乔", "今安", "知遇", "景和", "岁禾", "闲舟", "小满", "星禾", "知安", "嘉禾", "见夏", "予川", "晴川", "思齐", "知暖", "南星", "清禾", "晏宁"},
	"mystery":  {"闻默", "暗川", "知白", "夜航", "照影", "临默", "闻声", "陆离", "南柯", "无咎", "沉舟", "观棋", "未晞", "江离", "夜阑", "寒山", "留白", "向晚", "闻灯", "见晦", "寻川", "隐川", "照川", "知默", "无声", "微澜"},
	"urbane":   {"亦舟", "知远", "嘉言", "清越", "予安", "见川", "时安", "望川", "知夏", "承宇", "今衡", "嘉木", "知行", "言川", "绍安", "庭深", "启川", "思衡", "正清", "允诚", "闻衡", "守谦", "知礼", "景舟", "叙安", "清川", "明衡", "予衡", "知原", "临安"},
}

var referenceNameStyles = map[string][]string{
	"stellaris":    {"hard", "urbane"},
	"warhammer40k": {"gritty", "literary"},
	"alien":        {"gritty", "mystery"},
	"starwars":     {"literary", "warm"},
	"nomanssky":    {"hard", "mystery"},
}

var genreNameStyles = map[string][]string{
	"科幻": {"hard", "urbane", "mystery"},
	"仙侠": {"literary", "warm"},
	"都市": {"urbane", "warm"},
	"悬疑": {"mystery", "gritty"},
	"通用": {"common", "literary", "warm"},
}

// GenerateNameCards draws roleplay-ready names. If ArchetypeRoles is supplied,
// the role is selected before the name style, preventing a random name from
// being pasted onto an unrelated personality.
func GenerateNameCards(req NameRequest) (NameResult, error) {
	genre := NormalizeGenre(req.Genre)
	count := req.Count
	if count <= 0 {
		count = 5
	}
	if count > maxNameCardCount {
		count = maxNameCardCount
	}
	seed := req.Seed
	if seed == 0 {
		seed = randomSeed()
	}
	rngSeed := seed
	if req.PackID != "" {
		rngSeed = stableNameSeed(req.PackID)
	}
	rng := rand.New(rand.NewSource(rngSeed))
	preferredStyles := preferredNameStyles(genre, req.Source)
	cards := make([]NameCard, 0, count)
	usedNames := make(map[string]bool)
	usedArchetypes := make(map[string]bool)
	attempts := 0
	for len(cards) < count && attempts < count*200 {
		attempts++
		role := ""
		if len(req.ArchetypeRoles) > len(cards) {
			role = req.ArchetypeRoles[len(cards)]
		}
		archetype := pickNamingArchetype(rng, role, usedArchetypes)
		style := pickNameStyle(rng, archetype.NameStyles, preferredStyles)
		surname := pick(rng, namingSurnames[style])
		if surname == "" {
			surname = pick(rng, namingSurnames["common"])
		}
		given := pick(rng, namingGivenNames[style])
		name := surname + given
		if usedNames[name] {
			continue
		}
		usedNames[name] = true
		usedArchetypes[archetype.Name] = true
		id := fmt.Sprintf("name-%d-%02d", seed, len(cards)+1)
		if req.PackID != "" {
			id = fmt.Sprintf("name-%s-%02d", req.PackID, len(cards)+1)
		}
		cards = append(cards, NameCard{
			ID:            id,
			PackID:        req.PackID,
			ReferenceID:   req.ReferenceID,
			Name:          name,
			Surname:       surname,
			GivenName:     given,
			NamingStyle:   style,
			Archetype:     archetype.Name,
			Personality:   append([]string(nil), archetype.Personality...),
			Contradiction: archetype.Contradiction,
			Want:          archetype.Want,
			Fear:          archetype.Fear,
			SocialMask:    archetype.SocialMask,
			SpeechStyle:   archetype.SpeechStyle,
			BodyLanguage:  archetype.BodyLanguage,
			BackstoryHook: archetype.BackstoryHook,
			NamingLogic:   fmt.Sprintf("先定原型“%s”，再选%s音感；姓氏“%s”与名字“%s”组合为“%s”。", archetype.Name, nameStyleLabel(style), surname, given, name),
			RoleTags:      append([]string(nil), archetype.RoleTags...),
			Derivation:    []string{"archetype", "personality", "contradiction", "naming_style", "surname", "given_name"},
		})
	}
	warnings := []string(nil)
	if len(cards) < count {
		warnings = append(warnings, fmt.Sprintf("generated %d unique name cards, requested %d", len(cards), count))
	}
	return NameResult{Seed: seed, Genre: genre, Cards: cards, Warnings: warnings}, nil
}

func pickNamingArchetype(rng *rand.Rand, role string, used map[string]bool) namingArchetypeDef {
	candidates := make([]namingArchetypeDef, 0, len(namingArchetypes))
	if role != "" {
		for _, archetype := range namingArchetypes {
			if worldPackContains(archetype.RoleTags, role) && !used[archetype.Name] {
				candidates = append(candidates, archetype)
			}
		}
	}
	if len(candidates) == 0 && role != "" {
		for _, archetype := range namingArchetypes {
			if worldPackContains(archetype.RoleTags, role) {
				candidates = append(candidates, archetype)
			}
		}
	}
	if len(candidates) == 0 {
		for _, archetype := range namingArchetypes {
			if !used[archetype.Name] {
				candidates = append(candidates, archetype)
			}
		}
	}
	if len(candidates) == 0 {
		candidates = namingArchetypes
	}
	return candidates[rng.Intn(len(candidates))]
}

func preferredNameStyles(genre, source string) []string {
	if source != "" {
		if cards, err := ReferenceCards(source); err == nil && len(cards) == 1 {
			if styles := referenceNameStyles[cards[0].ID]; len(styles) > 0 {
				return styles
			}
		}
	}
	return genreNameStyles[genre]
}

func pickNameStyle(rng *rand.Rand, archetypeStyles, preferred []string) string {
	if len(preferred) == 0 {
		return pick(rng, archetypeStyles)
	}
	compatible := make([]string, 0, len(archetypeStyles))
	for _, style := range archetypeStyles {
		for _, wanted := range preferred {
			if style == wanted {
				compatible = append(compatible, style)
				break
			}
		}
	}
	if len(compatible) > 0 {
		return pick(rng, compatible)
	}
	return pick(rng, archetypeStyles)
}

func nameStyleLabel(style string) string {
	switch style {
	case "literary":
		return "含蓄、书卷"
	case "hard":
		return "冷静、工程"
	case "gritty":
		return "粗粝、耐磨"
	case "warm":
		return "温和、明亮"
	case "mystery":
		return "留白、幽暗"
	case "urbane":
		return "克制、都市"
	default:
		return "自然"
	}
}

func stableNameSeed(packID string) int64 {
	var seed int64 = 1469598103934665603
	for _, b := range []byte(packID + "|namecards") {
		seed ^= int64(b)
		seed *= 1099511628211
	}
	return seed & 0x7fffffffffffffff
}

// NameResearchSources returns the public references used for the surname and
// genealogy inspiration pool.
func NameResearchSources() []map[string]string {
	return []map[string]string{
		{
			"name":        "百家姓",
			"url":         nameSourceURL,
			"accessed_on": "2026-10-25",
			"note":        nameSourceNote,
		},
		{
			"name":        "族谱网",
			"url":         "https://www.zupu.cn/",
			"accessed_on": "2026-10-25",
			"note":        "用于确认家谱文化、姓氏与谱系资料的公共语境；不复制具体家族人物或谱系内容。",
		},
	}
}
