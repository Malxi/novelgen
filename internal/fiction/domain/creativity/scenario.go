package creativity

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

const maxScenarioCount = 30

// ScenarioRequest controls a deterministic Scenario Card draw. Scenario cards
// are for hard-SF / campaign ideation: the code fixes the causal roles and the
// model may only instantiate names, history, factions, and texture.
type ScenarioRequest struct {
	Genre string
	Seed  int64
	Count int
}

// ScenarioResult is a replayable set of causal scenario cards.
type ScenarioResult struct {
	Seed  int64          `json:"seed"`
	Genre string         `json:"genre"`
	Cards []ScenarioCard `json:"cards"`
}

// ScenarioCard is a causal skeleton, not a finished premise. Every field is a
// role in the same survival economy; changing a role means writing a different
// scenario instead of adding a new exception.
type ScenarioCard struct {
	ID                    string            `json:"id"`
	Seed                  int64             `json:"seed"`
	Genre                 string            `json:"genre"`
	Arena                 string            `json:"arena"`
	Scarcity              string            `json:"scarcity"`
	Hazard                string            `json:"hazard"`
	HazardCycle           string            `json:"hazard_cycle"`
	Objective             string            `json:"objective"`
	Enforcer              string            `json:"enforcer"`
	Mobility              string            `json:"mobility"`
	MechaRole             string            `json:"mecha_role"`
	Threat                string            `json:"threat"`
	ThreatSignal          string            `json:"threat_signal"`
	Countermeasure        string            `json:"countermeasure"`
	Cost                  string            `json:"cost"`
	Twist                 string            `json:"twist"`
	PremiseKernel         string            `json:"premise_kernel"`
	ElementRoles          map[string]string `json:"element_roles"`
	CausalChain           []string          `json:"causal_chain"`
	MustAnswer            []string          `json:"must_answer"`
	RejectIf              []string          `json:"reject_if"`
	ForbiddenExplanations []string          `json:"forbidden_explanations"`
}

// ScenarioPlanItem is one assessed card in a card-level best-of-N selection.
type ScenarioPlanItem struct {
	Card            ScenarioCard `json:"card"`
	Family          string       `json:"family"`
	Predictability  int          `json:"predictability"`
	Novelty         int          `json:"novelty"`
	SelectionReason string       `json:"selection_reason,omitempty"`
}

// ScenarioPlan is the cheap divergence layer: generate many cards, assess them,
// and select a few causally different cards before any full setup is written.
type ScenarioPlan struct {
	Seed     int64              `json:"seed"`
	Genre    string             `json:"genre"`
	Selected []ScenarioPlanItem `json:"selected"`
	Rejected []ScenarioPlanItem `json:"rejected,omitempty"`
}

var scenarioArenas = []string{
	"虫洞边缘的废弃航道",
	"双星轨道的残骸带",
	"殖民站的引力井",
	"冰巨星背面的补给线",
	"黑洞吸积盘外侧的锚地",
	"被虫族孢子云覆盖的旧战场",
}

var scenarioScarcities = []string{
	"反应堆冷却剂",
	"跃迁燃料",
	"氧气",
	"舰船备件",
	"护盾电容",
	"反物质",
	"淡水",
}

var scenarioHazards = []string{
	"引力剪切",
	"虫洞潮汐",
	"离子风暴",
	"微陨石雨",
	"孢子云",
	"恒星耀斑",
	"辐射带",
}

var scenarioHazardCycles = []string{
	"每49小时",
	"每13个周期",
	"每7天",
	"每24小时",
	"每12小时",
	"每30天",
	"每72小时",
	"每10个周期",
}

var scenarioObjectives = []string{
	"唯一的跃迁窗口",
	"虫洞坐标",
	"航道控制权",
	"母舰残骸里的反应堆核心",
	"殖民地撤离许可",
	"停战协议",
}

var scenarioEnforcers = []string{
	"深空保险公司",
	"虫洞航道局",
	"舰队联合体",
	"黑市打捞公会",
	"边境殖民署",
	"航运卡特尔",
}

var scenarioMobility = []string{
	"机甲",
	"打捞船",
	"登陆舱",
	"无人机",
	"破冰船",
	"护航舰",
}

var scenarioThreats = []string{
	"虫族工蜂",
	"虫族母巢",
	"回声虫",
	"孢子虫",
	"酸蚀虫",
	"寄生蜂",
}

var scenarioThreatSignals = []string{
	"反应堆热签名",
	"生命维持热",
	"电磁信号",
	"引擎尾焰",
	"声呐脉冲",
}

var scenarioCountermeasures = []string{
	"关闭反应堆",
	"释放诱饵",
	"切换静默航行",
	"进入潮汐阴影区",
	"切断通讯",
	"抛掉护盾电容",
}

var scenarioMechaRoles = []string{
	"机甲负责登舰、打捞和把战舰拖出引力剪切",
	"机甲是唯一能进入残骸内部回收黑匣子的单位",
	"机甲负责在静默航行时充当热诱饵",
	"机甲负责在虫巢外围建立临时信标",
}

var scenarioCosts = []string{
	"被注销身份",
	"失去战舰",
	"被剥离公民权",
	"成为虫族孵化场",
	"被永久通缉",
	"继承死者的债务",
}

var scenarioTwists = []string{
	"真正的收益是理赔数据，不是航道",
	"虫族是航道局放养的清理工具",
	"虫洞坐标其实是一个活着的领航员",
	"虫洞提前关闭是人为制造的",
	"目标资源是诱饵，真正的资源是残骸里的黑匣子",
	"赢家会被改造成下一批虫巢的母体",
}

// GenerateScenarios draws causal cards. Each field is sampled from a compatible
// role pool, then the card is assembled as one causal chain instead of a
// sentence that later needs ad-hoc exceptions.
func GenerateScenarios(req ScenarioRequest) (ScenarioResult, error) {
	genre := NormalizeGenre(req.Genre)
	if genre != "科幻" && genre != "通用" {
		return ScenarioResult{}, fmt.Errorf("scenario cards currently support 科幻 (or empty), got %q", req.Genre)
	}
	count := req.Count
	if count <= 0 {
		count = 3
	}
	if count > maxScenarioCount {
		count = maxScenarioCount
	}
	seed := req.Seed
	if seed == 0 {
		seed = randomSeed()
	}
	rng := rand.New(rand.NewSource(seed))
	cards := make([]ScenarioCard, 0, count)
	seen := map[string]bool{}
	for len(cards) < count {
		card := buildScenarioCard(rng, seed, len(cards)+1)
		key := card.PremiseKernel
		if seen[key] {
			continue
		}
		seen[key] = true
		cards = append(cards, card)
		if len(cards) >= 100 {
			break
		}
	}
	return ScenarioResult{Seed: seed, Genre: "科幻", Cards: cards}, nil
}

// PlanScenarios selects a diverse subset of cards by causal family. It is a
// deterministic first-pass predictor: repeated hazard/threat/countermeasure
// combinations are treated as more predictable, while cards from a new family
// are preferred. A model-based predictor can later replace or augment the score.
func PlanScenarios(result ScenarioResult, selectCount int) ScenarioPlan {
	if selectCount <= 0 {
		selectCount = 3
	}
	if selectCount > len(result.Cards) {
		selectCount = len(result.Cards)
	}
	assessed := make([]ScenarioPlanItem, 0, len(result.Cards))
	for _, card := range result.Cards {
		predictability, reasons := scenarioPredictability(card)
		assessed = append(assessed, ScenarioPlanItem{
			Card:            card,
			Family:          scenarioFamily(card),
			Predictability:  predictability,
			Novelty:         100 - predictability,
			SelectionReason: strings.Join(reasons, "; "),
		})
	}

	// Keep the strongest card per family first, then fill from the remaining
	// cards by novelty. This prevents five re-skins of the same scenario.
	sort.SliceStable(assessed, func(i, j int) bool {
		if assessed[i].Novelty == assessed[j].Novelty {
			return assessed[i].Card.ID < assessed[j].Card.ID
		}
		return assessed[i].Novelty > assessed[j].Novelty
	})
	selected := SelectScenarioItems(assessed, selectCount)
	selectedIDs := map[string]bool{}
	for _, item := range selected {
		selectedIDs[item.Card.ID] = true
	}

	rejected := make([]ScenarioPlanItem, 0, len(assessed)-len(selected))
	for _, item := range assessed {
		if !selectedIDs[item.Card.ID] {
			item.SelectionReason = "lower novelty or duplicate family"
			rejected = append(rejected, item)
		}
	}
	return ScenarioPlan{
		Seed:     result.Seed,
		Genre:    result.Genre,
		Selected: selected,
		Rejected: rejected,
	}
}

// SelectScenarioItems re-ranks assessed cards after a model predictor has
// replaced the initial heuristic scores. One card per causal family goes first,
// then the remaining slots are filled by novelty.
func SelectScenarioItems(items []ScenarioPlanItem, selectCount int) []ScenarioPlanItem {
	if selectCount <= 0 {
		selectCount = 3
	}
	if selectCount > len(items) {
		selectCount = len(items)
	}
	sorted := append([]ScenarioPlanItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Novelty == sorted[j].Novelty {
			return sorted[i].Card.ID < sorted[j].Card.ID
		}
		return sorted[i].Novelty > sorted[j].Novelty
	})
	selected := make([]ScenarioPlanItem, 0, selectCount)
	usedFamilies := map[string]bool{}
	selectedIDs := map[string]bool{}
	for _, item := range sorted {
		if len(selected) >= selectCount {
			break
		}
		if usedFamilies[item.Family] {
			continue
		}
		usedFamilies[item.Family] = true
		selectedIDs[item.Card.ID] = true
		item.SelectionReason = "highest novelty in family " + item.Family
		selected = append(selected, item)
	}
	for _, item := range sorted {
		if len(selected) >= selectCount {
			break
		}
		if selectedIDs[item.Card.ID] {
			continue
		}
		selectedIDs[item.Card.ID] = true
		item.SelectionReason = "novelty fill after family selection"
		selected = append(selected, item)
	}
	return selected
}

func scenarioFamily(card ScenarioCard) string {
	return strings.Join([]string{
		card.Enforcer,
		card.Hazard,
		card.Threat,
		card.Countermeasure,
	}, " / ")
}

func scenarioPredictability(card ScenarioCard) (int, []string) {
	score := 0
	reasons := make([]string, 0)
	add := func(points int, reason string) {
		score += points
		reasons = append(reasons, reason)
	}
	if card.Enforcer == "深空保险公司" {
		add(10, "corporate-insurance arena is a familiar pattern")
	}
	if card.Hazard == "引力剪切" && card.Scarcity == "舰船备件" {
		add(15, "gravity shear + spare parts is a familiar survival loop")
	}
	if card.Threat == "虫族工蜂" && card.Countermeasure == "关闭反应堆" {
		add(15, "heat-seeking swarm + shutdown is a familiar answer")
	}
	if card.Threat == "酸蚀虫" && card.Countermeasure == "抛掉护盾电容" {
		add(12, "acid swarm + discard capacitor is a familiar tradeoff")
	}
	if card.Arena == "虫洞边缘的废弃航道" && (card.Objective == "航道控制权" || card.Objective == "唯一的跃迁窗口") {
		add(10, "wormhole-edge transit race is a familiar arena")
	}
	if card.Enforcer == "虫洞航道局" && (card.Scarcity == "舰船备件" || card.Scarcity == "跃迁燃料") {
		add(8, "transit authority + fuel/spares is a familiar monopoly")
	}
	if card.Cost == "被注销身份" && card.Enforcer == "深空保险公司" {
		add(8, "insurance de-identification is a familiar cost")
	}
	if strings.Contains(card.Twist, "目标资源是诱饵") {
		add(8, "bait prize is a familiar twist")
	}
	if strings.Contains(card.Twist, "赢家会被改造成下一批虫巢的母体") {
		add(10, "winner-becomes-brood-mother is a familiar reveal")
	}
	if strings.Contains(card.Twist, "虫洞坐标其实是一个活着的领航员") {
		add(10, "living navigator is a familiar wormhole reveal")
	}
	if strings.Contains(card.Twist, "理赔数据") {
		add(8, "insurance data reveal is a familiar corporate twist")
	}
	if score > 100 {
		score = 100
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no high-frequency combination detected")
	}
	return score, reasons
}

func buildScenarioCard(rng *rand.Rand, seed int64, index int) ScenarioCard {
	arena := pick(rng, scenarioArenas)
	scarcity := pick(rng, scenarioScarcities)
	hazard := pick(rng, scenarioHazards)
	cycle := pick(rng, scenarioHazardCycles)
	objective := pick(rng, scenarioObjectives)
	enforcer := pick(rng, scenarioEnforcers)
	mobility := pick(rng, scenarioMobility)
	mechaRole := pick(rng, scenarioMechaRoles)
	threat := pick(rng, scenarioThreats)
	signal := pick(rng, scenarioThreatSignals)
	countermeasure := pick(rng, scenarioCountermeasures)
	cost := pick(rng, scenarioCosts)
	twist := pick(rng, scenarioTwists)
	premise := fmt.Sprintf(
		"在%s，%s是唯一硬通货；%s%s发生，只有%s能在灾害中行动；%s举办一场围绕%s的大逃杀，赢家获得%s，输家%s；真正的秘密是%s。",
		arena, scarcity, hazard, cycle, mobility, enforcer, scarcity, objective, cost, twist,
	)
	roles := map[string]string{
		"星战": "enforcer 与不同舰队争夺 objective",
		"战舰": "scarcity 的主要消耗单位",
		"机甲": mechaRole,
		"天灾": "hazard，周期性压缩行动窗口",
		"虫洞": "arena，同时是 objective 和倒计时",
		"虫族": "threat，被 threat_signal 吸引",
	}
	chain := []string{
		fmt.Sprintf("%s 周期性触发 %s，压缩 %s 的可用窗口。", hazard, cycle, arena),
		fmt.Sprintf("%s 控制 %s，并通过 %s 获利。", enforcer, scarcity, objective),
		fmt.Sprintf("%s 只能在灾害中移动，因此成为争夺焦点。", mobility),
		fmt.Sprintf("%s 会被 %s 吸引；%s 能暂时降低风险，但会牺牲 %s。", threat, signal, countermeasure, scarcity),
		fmt.Sprintf("输家承担 %s；真正的收益流向 %s。", cost, twist),
	}
	return ScenarioCard{
		ID:             fmt.Sprintf("scenario-%d-%02d", seed, index),
		Seed:           seed,
		Genre:          "科幻",
		Arena:          arena,
		Scarcity:       scarcity,
		Hazard:         hazard,
		HazardCycle:    cycle,
		Objective:      objective,
		Enforcer:       enforcer,
		Mobility:       mobility,
		MechaRole:      mechaRole,
		Threat:         threat,
		ThreatSignal:   signal,
		Countermeasure: countermeasure,
		Cost:           cost,
		Twist:          twist,
		PremiseKernel:  premise,
		ElementRoles:   roles,
		CausalChain:    chain,
		MustAnswer:     scenarioMustAnswer(),
		RejectIf:       scenarioRejectIf(),
		ForbiddenExplanations: []string{
			"第二条魔法规则", "梦", "幻觉", "传说", "隐喻", "巧合",
			"把稀缺资源写成免费", "把灾害写成纯背景", "把虫族写成无代价外挂",
		},
	}
}

func scenarioMustAnswer() []string {
	return []string{
		"稀缺资源具体怎么被消耗、补给和测量？",
		"灾害周期如何改变行动窗口？谁掌握周期预测？",
		"执行者靠什么获利？谁付钱、谁承担损失？",
		"机动单位为什么不可替代？它的限制是什么？",
		"虫族为什么会被这种信号吸引？反制措施为什么有代价？",
		"输家的代价如何执行？为什么他们不集体退出？",
		"真正的收益如何反过来解释这场大逃杀？",
		"六个元素分别在因果链的哪一环？",
	}
}

func scenarioRejectIf() []string {
	return []string{
		"稀缺资源可以免费获得或凭空产生。",
		"灾害只是背景，不改变行动窗口。",
		"机构没有受益者、预算或执行能力。",
		"机动单位可以无条件使用。",
		"虫族不依赖 threat_signal，或反制没有成本。",
		"输家的代价无法执行，或所有人都能零成本退出。",
		"需要第二条魔法规则才能让因果链成立。",
		"六个元素中有一个只是名词装饰。",
	}
}

// Validate checks the deterministic invariants of a scenario card.
func (c ScenarioCard) Validate() error {
	required := map[string]string{
		"arena": c.Arena, "scarcity": c.Scarcity, "hazard": c.Hazard,
		"hazard_cycle": c.HazardCycle, "objective": c.Objective,
		"enforcer": c.Enforcer, "mobility": c.Mobility, "mecha_role": c.MechaRole, "threat": c.Threat,
		"threat_signal": c.ThreatSignal, "countermeasure": c.Countermeasure,
		"cost": c.Cost, "twist": c.Twist, "premise_kernel": c.PremiseKernel,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("scenario %s: %s is empty", c.ID, name)
		}
	}
	for _, element := range []string{"星战", "战舰", "机甲", "天灾", "虫洞", "虫族"} {
		if strings.TrimSpace(c.ElementRoles[element]) == "" {
			return fmt.Errorf("scenario %s: element %s has no role", c.ID, element)
		}
	}
	if len(c.CausalChain) < 4 {
		return fmt.Errorf("scenario %s: causal chain is too short", c.ID)
	}
	return nil
}
