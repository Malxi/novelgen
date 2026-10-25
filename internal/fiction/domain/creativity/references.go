package creativity

import (
	"fmt"
	"strings"
)

// ReferenceCard records a researched setting pattern. It deliberately stores
// short original summaries and structural mechanics, not copied setting text.
// Generated world packs may use the card as a style lens, but must not import
// protected proper nouns, characters, or plot events from the source work.
type ReferenceCard struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	OriginalWork        string   `json:"original_work"`
	Medium              string   `json:"medium"`
	SourceURL           string   `json:"source_url"`
	AccessedOn          string   `json:"accessed_on"`
	CorePattern         string   `json:"core_pattern"`
	StructuralMechanics []string `json:"structural_mechanics"`
	StoryEngines        []string `json:"story_engines"`
	ToneTags            []string `json:"tone_tags"`
	FusionRule          string   `json:"fusion_rule"`
	Avoid               []string `json:"avoid"`
}

var referenceCards = []ReferenceCard{
	{
		ID:           "stellaris",
		Name:         "群星",
		OriginalWork: "Stellaris",
		Medium:       "video game",
		SourceURL:    "https://www.gog.com/en/game/stellaris",
		AccessedOn:   "2026-10-25",
		CorePattern:  "把星际探索、资源治理、外交与战争放进同一套长期策略循环。",
		StructuralMechanics: []string{
			"探索揭示异常，异常又改变政策与外交关系",
			"不同伦理和利益集团会把外部资源转成内部政治冲突",
			"长期收益来自航线、领地和制度组合，而不是一次战斗",
		},
		StoryEngines: []string{
			"一颗新发现的世界迫使两个阵营重写互不侵犯条约",
			"一项看似无害的科研异常逐渐改变整个帝国的资源分配",
			"边境殖民地用自治换生存，中央却要求统一征税",
		},
		ToneTags:   []string{"银河尺度", "制度选择", "探索与外交", "长期经营"},
		FusionRule: "把参考作品当作宏观策略层：只吸收探索—异常—政策—派系冲突的循环，不复制其种族、帝国或专有历史。",
		Avoid:      []string{"把宏观外交写成单个救世主通关", "只借用外星种族名称而不改变制度"},
	},
	{
		ID:           "warhammer40k",
		Name:         "战锤",
		OriginalWork: "Warhammer 40,000",
		Medium:       "tabletop setting and fiction",
		SourceURL:    "https://www.warhammer-community.com/en-gb/universes/warhammer-40000/",
		AccessedOn:   "2026-10-25",
		CorePattern:  "庞大、军事化、以信仰和官僚维持的星际秩序，在无止境冲突中不断制造自己的敌人。",
		StructuralMechanics: []string{
			"战争不是例外，而是制度维持预算、身份和忠诚的常态",
			"官僚、教会与军事派系互相需要，也互相掩盖失败",
			"个人英雄很强，但无法单独改变宏大秩序的代价",
		},
		StoryEngines: []string{
			"前线胜利会暴露后方补给与信仰体系的矛盾",
			"一个被宣布为异端的证人掌握着帝国最需要的技术",
			"守住一座城市意味着向它征收足以毁掉它的战争税",
		},
		ToneTags:   []string{"宏大压迫", "阵营冲突", "官僚战争", "黑暗讽刺"},
		FusionRule: "只使用“战争常态、派系官僚、信仰与资源互相锁定”的结构；生成时使用原创阵营、神系与术语。",
		Avoid:      []string{"直接搬用受保护角色、军团与神祇名称", "把黑暗写成无差别血腥描写"},
	},
	{
		ID:           "alien",
		Name:         "异形",
		OriginalWork: "Alien",
		Medium:       "film",
		SourceURL:    "https://www.20thcenturystudios.com/movies/alien",
		AccessedOn:   "2026-10-25",
		CorePattern:  "封闭工作空间中的求生，被公司合同、未知生物和隐瞒任务目标共同放大。",
		StructuralMechanics: []string{
			"劳资合同要求船员调查危险信号，救人义务与公司利润冲突",
			"未知生物真正可怕的地方是它能改变船内空间和人员信任",
			"密闭环境让声音、热量、氧气或路线都成为可追踪的弱点",
		},
		StoryEngines: []string{
			"救援任务其实是把某个活体样本带回总部",
			"船员每救一个人就会暴露更多舱室位置",
			"公司规章要求保留资产，却要求船员承担事故责任",
		},
		ToneTags:   []string{"幽闭恐惧", "企业阴谋", "生物威胁", "生存倒计时"},
		FusionRule: "只吸收企业合同、封闭空间、未知生物和隐藏任务目标的因果链；不使用既有生物名称、船员或飞船情节。",
		Avoid:      []string{"把异形写成不可解释的万能怪物", "让公司阴谋与生存机制各自独立"},
	},
	{
		ID:           "starwars",
		Name:         "星战",
		OriginalWork: "Star Wars",
		Medium:       "film and multimedia setting",
		SourceURL:    "https://www.starwars.com/databank/the-force",
		AccessedOn:   "2026-10-25",
		CorePattern:  "银河政治、战争与一套有自身意志的超自然力量并存，个人选择因此具有宇宙尺度后果。",
		StructuralMechanics: []string{
			"超自然力量既赋予能力，也塑造使用者的判断与责任",
			"共和国、帝国或反抗组织的变化会改变普通人的法律身份",
			"师徒、组织和信念之间的忠诚，经常比舰队数量更重要",
		},
		StoryEngines: []string{
			"一种被国家垄断的力量训练体系开始制造内部异端",
			"主角必须在拯救亲人和阻止政权篡位之间选择",
			"古老组织的和平承诺掩盖了对力量使用者的长期控制",
		},
		ToneTags:   []string{"神话冒险", "能力与责任", "银河政治", "师徒冲突"},
		FusionRule: "只吸收“有意志的力量、秩序与反叛、个人选择的宇宙后果”；生成原创力量体系、阵营和角色。",
		Avoid:      []string{"直接使用既有绝地、西斯、帝国人物或台词", "把有意志的力量简化成无代价超能力"},
	},
	{
		ID:           "nomanssky",
		Name:         "无人深空",
		OriginalWork: "No Man's Sky",
		Medium:       "video game",
		SourceURL:    "https://www.nomanssky.com/",
		AccessedOn:   "2026-10-25",
		CorePattern:  "以程序化星系为舞台，把探索、采集、建造、贸易、战斗和多人协作编织成开放式生存循环。",
		StructuralMechanics: []string{
			"未知环境先迫使玩家采集资源，再把资源转换成可建造的根据地",
			"远距离旅行需要燃料、路线和维护，探索成本不断累积",
			"同一片星系可以同时产生贸易机会、危险环境、遗迹与竞争队伍",
		},
		StoryEngines: []string{
			"一张程序化生成的地图不断重复同一段坐标，却每次给出不同遗迹",
			"一个宜居星球的价值来自它能让多少艘船补给",
			"探索队发现前文明留下的导航网络正在改变贸易路线",
		},
		ToneTags:   []string{"开放探索", "资源循环", "孤独奇观", "共同建设"},
		FusionRule: "只吸收探索—采集—建造—交易—生存的循环，以及程序化遗迹带来的认知悬念；不复制具体星球和图鉴。",
		Avoid:      []string{"把无限宇宙写成没有成本的地图旅行", "只抽星球名字而不给资源循环"},
	},
}

var referenceAliases = map[string]string{
	"群星":           "stellaris",
	"stellaris":    "stellaris",
	"战锤":           "warhammer40k",
	"战锤40k":        "warhammer40k",
	"warhammer":    "warhammer40k",
	"warhammer40k": "warhammer40k",
	"wh40k":        "warhammer40k",
	"异形":           "alien",
	"异型":           "alien",
	"alien":        "alien",
	"星战":           "starwars",
	"星球大战":         "starwars",
	"starwars":     "starwars",
	"star wars":    "starwars",
	"无人深空":         "nomanssky",
	"nomanssky":    "nomanssky",
	"no man's sky": "nomanssky",
	"no mans sky":  "nomanssky",
}

var familyReferenceDefaults = map[string]string{
	"insurance":    "alien",
	"license":      "stellaris",
	"contract":     "warhammer40k",
	"inheritance":  "starwars",
	"death_record": "alien",
	"salvage":      "nomanssky",
	"quota":        "stellaris",
	"maintenance":  "warhammer40k",
	"registry":     "starwars",
	"evidence":     "alien",
	"sanctuary":    "nomanssky",
	"ration":       "warhammer40k",
}

// ReferenceCards returns the researched reference pool. An empty source returns
// all profiles; aliases such as "群星", "战锤" and "无人深空" are accepted.
func ReferenceCards(source string) ([]ReferenceCard, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		return append([]ReferenceCard(nil), referenceCards...), nil
	}
	id, ok := referenceAliases[source]
	if !ok {
		return nil, fmt.Errorf("unknown reference source %q; valid sources: stellaris, warhammer40k, alien, starwars, nomanssky", source)
	}
	for _, card := range referenceCards {
		if card.ID == id {
			return []ReferenceCard{card}, nil
		}
	}
	return nil, fmt.Errorf("reference source %q is not configured", source)
}

func referenceForEngine(engine Engine, requested string) (ReferenceCard, error) {
	if strings.TrimSpace(requested) != "" {
		cards, err := ReferenceCards(requested)
		if err != nil {
			return ReferenceCard{}, err
		}
		return cards[0], nil
	}
	id := familyReferenceDefaults[engine.Family]
	for _, card := range referenceCards {
		if card.ID == id {
			return card, nil
		}
	}
	return referenceCards[0], nil
}
