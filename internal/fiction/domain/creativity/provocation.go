// Package creativity generates deterministic "structured nonsense" prompts for
// setup and outline ideation. The generated sentence is deliberately strange,
// but every sentence carries a required structural role and a rationalization
// contract, so an agent must turn it into a world rule, historical event, or
// proper name instead of decorating it as lore.
package creativity

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	LevelAxiom = "axiom"
	LevelEvent = "event"
	LevelLaw   = "law"
	LevelName  = "name"

	ModeSafe     = "safe"
	ModeBalanced = "balanced"
	ModeWild     = "wild"
	ModeGrounded = "grounded"

	maxProvocationCount = 20
)

// Request controls one deterministic provocation draw.
type Request struct {
	Level string
	Genre string
	Mode  string
	Seed  int64
	Count int
}

// Result is the full draw. Seed is always non-zero so the caller can replay it.
type Result struct {
	Seed         int64         `json:"seed"`
	Level        string        `json:"level"`
	Genre        string        `json:"genre"`
	Mode         string        `json:"mode"`
	Provocations []Provocation `json:"provocations"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// Provocation is a question the model must solve, not a finished premise.
type Provocation struct {
	ID                    string            `json:"id"`
	Seed                  int64             `json:"seed"`
	Level                 string            `json:"level"`
	Genre                 string            `json:"genre"`
	Mode                  string            `json:"mode"`
	Sentence              string            `json:"sentence"`
	RequiredRole          string            `json:"required_role"`
	AxiomBudget           int               `json:"axiom_budget"`
	MustAnswer            []string          `json:"must_answer"`
	DerivationSteps       []string          `json:"derivation_steps"`
	RejectIf              []string          `json:"reject_if"`
	ForbiddenExplanations []string          `json:"forbidden_explanations"`
	OutputContract        []string          `json:"output_contract"`
	TemplateID            string            `json:"template_id,omitempty"`
	Slots                 map[string]string `json:"slots,omitempty"`
}

// NormalizeLevel accepts the English names and a few Chinese aliases.
func NormalizeLevel(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", LevelAxiom, "rule", "公理", "规则":
		return LevelAxiom, nil
	case LevelEvent, "history", "事件", "历史":
		return LevelEvent, nil
	case LevelLaw, "rule-system", "法则", "世界法则", "systemic":
		return LevelLaw, nil
	case LevelName, "proper-name", "专名", "名字":
		return LevelName, nil
	default:
		return "", fmt.Errorf("unsupported idea level %q; valid levels: axiom, event, law, name", value)
	}
}

// NormalizeMode accepts the three exploration modes.
func NormalizeMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ModeBalanced:
		return ModeBalanced, nil
	case ModeSafe:
		return ModeSafe, nil
	case ModeWild:
		return ModeWild, nil
	case ModeGrounded, "hard", "hard-scifi", "grounded-scifi":
		return ModeGrounded, nil
	default:
		return "", fmt.Errorf("unsupported idea mode %q; valid modes: safe, balanced, wild, grounded", value)
	}
}

// NormalizeGenre maps common genre aliases to the small set of weighted word
// pools. Unknown genres deliberately fall back to the neutral pool.
func NormalizeGenre(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "仙侠", "修仙", "修真", "玄幻", "xianxia", "xuanhuan":
		return "仙侠"
	case "都市", "现代", "现实", "urban":
		return "都市"
	case "悬疑", "推理", "惊悚", "mystery", "thriller":
		return "悬疑"
	case "科幻", "星际", "sci-fi", "scifi", "science-fiction":
		return "科幻"
	default:
		return "通用"
	}
}

// Generate draws count unique provocations from the requested level. A zero
// seed means "pick a fresh seed"; the chosen seed is returned in the result.
func Generate(req Request) (Result, error) {
	level, err := NormalizeLevel(req.Level)
	if err != nil {
		return Result{}, err
	}
	mode, err := NormalizeMode(req.Mode)
	if err != nil {
		return Result{}, err
	}
	genre := NormalizeGenre(req.Genre)
	count := req.Count
	warnings := make([]string, 0)
	if count <= 0 {
		count = 5
	}
	if count > maxProvocationCount {
		warnings = append(warnings, fmt.Sprintf("count capped from %d to %d", count, maxProvocationCount))
		count = maxProvocationCount
	}
	seed := req.Seed
	if seed == 0 {
		seed = randomSeed()
	}

	rng := rand.New(rand.NewSource(seed))
	pool := poolForGenre(genre)
	seen := make(map[string]bool, count)
	templateUses := make(map[string]int)
	items := make([]Provocation, 0, count)
	attempts := 0
	maxAttempts := count*100 + 100

	for len(items) < count && attempts < maxAttempts {
		attempts++
		var sentence string
		var templateID string
		var slots map[string]string
		if level == LevelName {
			sentence, templateID, slots = generateName(rng, pool)
		} else {
			template := chooseTemplate(rng, level, mode, templateUses)
			slots = make(map[string]string, len(template.Slots))
			values := make(map[string]string, len(template.Slots))
			args := make([]interface{}, 0, len(template.Slots))
			for _, slot := range template.Slots {
				value, ok := values[slot]
				if !ok {
					value = slotValue(rng, slot, pool)
					values[slot] = value
				}
				slots[slot] = value
				args = append(args, value)
			}
			sentence = fmt.Sprintf(template.Format, args...)
			if !validSlotCombination(level, template, slots) {
				continue
			}
			templateID = template.ID
		}
		if strings.TrimSpace(sentence) == "" || seen[sentence] {
			continue
		}
		seen[sentence] = true
		if templateID != "" {
			templateUses[templateID]++
		}
		items = append(items, Provocation{
			ID:                    fmt.Sprintf("%s-%d-%02d", level, seed, len(items)+1),
			Seed:                  seed,
			Level:                 level,
			Genre:                 genre,
			Mode:                  mode,
			Sentence:              sentence,
			RequiredRole:          requiredRole(level),
			AxiomBudget:           1,
			MustAnswer:            mustAnswer(level),
			DerivationSteps:       derivationSteps(),
			RejectIf:              rejectConditions(),
			ForbiddenExplanations: forbiddenExplanations(),
			OutputContract:        outputContract(),
			TemplateID:            templateID,
			Slots:                 slots,
		})
	}
	if len(items) < count {
		warnings = append(warnings, fmt.Sprintf("generated %d unique provocation(s), requested %d", len(items), count))
	}

	return Result{
		Seed:         seed,
		Level:        level,
		Genre:        genre,
		Mode:         mode,
		Provocations: items,
		Warnings:     warnings,
	}, nil
}

// chooseTemplate samples from the least-used eligible grammar templates. A
// single draw should explore different sentence shapes before it reuses one;
// otherwise five "random" ideas can all be the same institutional sentence with
// different nouns, which is exactly the sameness this tool exists to break.
func chooseTemplate(rng *rand.Rand, level, mode string, used map[string]int) sentenceTemplate {
	all := templatesFor(level, mode)
	candidates := make([]sentenceTemplate, 0, len(all)*2)
	for _, template := range all {
		if mode == ModeSafe && template.Wild {
			continue
		}
		candidates = append(candidates, template)
		if mode == ModeWild && template.Wild {
			candidates = append(candidates, template)
		}
	}
	if len(candidates) == 0 {
		candidates = all
	}
	minUses := -1
	for _, template := range candidates {
		uses := used[template.ID]
		if minUses < 0 || uses < minUses {
			minUses = uses
		}
	}
	fresh := make([]sentenceTemplate, 0, len(candidates))
	for _, template := range candidates {
		if used[template.ID] == minUses {
			fresh = append(fresh, template)
		}
	}
	return fresh[rng.Intn(len(fresh))]
}

func templatesFor(level, mode string) []sentenceTemplate {
	switch level {
	case LevelEvent:
		return eventTemplates
	case LevelLaw:
		if mode == ModeGrounded {
			return groundedLawTemplates
		}
		return lawTemplates
	case LevelName:
		return nil
	default:
		return axiomTemplates
	}
}

func slotValue(rng *rand.Rand, slot string, pool wordPool) string {
	switch slot {
	case "subject":
		return pick(rng, pool.subjects)
	case "verb":
		return pick(rng, pool.verbs)
	case "intransitive_verb", "verb2":
		return pick(rng, pool.intransitiveVerbs)
	case "event_verb":
		return pick(rng, pool.eventVerbs)
	case "give_verb":
		return pick(rng, pool.giveVerbs)
	case "object":
		return pick(rng, pool.objects)
	case "time":
		return pick(rng, pool.times)
	case "time_unit":
		return pick(rng, pool.timeUnits)
	case "place", "place2":
		return pick(rng, pool.places)
	case "target", "target2":
		return pick(rng, pool.targets)
	case "thing":
		return pick(rng, pool.things)
	case "law_thing":
		return pick(rng, pool.lawThings)
	case "deadline":
		return pick(rng, pool.deadlines)
	case "deadline_verb":
		return pick(rng, pool.deadlineVerbs)
	case "unit":
		return pick(rng, pool.units)
	case "consequence":
		return pick(rng, pool.consequences)
	case "goal":
		return pick(rng, pool.goals)
	case "reason":
		return pick(rng, pool.reasons)
	case "contract":
		return pick(rng, pool.contracts)
	case "transfer_verb":
		return pick(rng, pool.transferVerbs)
	case "recipient":
		return pick(rng, pool.recipients)
	case "enforcer":
		return pick(rng, pool.enforcers)
	case "action":
		return pick(rng, pool.actions)
	case "condition":
		return pick(rng, pool.conditions)
	case "cost":
		return pick(rng, pool.costs)
	case "law_consequence":
		return pick(rng, pool.lawConsequences)
	case "hazard":
		return pick(rng, pool.hazards)
	case "hazard_verb":
		return pick(rng, pool.hazardVerbs)
	case "resource":
		return pick(rng, pool.groundedResources)
	case "decay_verb":
		return pick(rng, pool.decayVerbs)
	case "source":
		return pick(rng, pool.sources)
	case "vehicle":
		return pick(rng, pool.vehicles)
	case "system":
		return pick(rng, pool.systems)
	case "creature":
		return pick(rng, pool.creatures)
	case "signature":
		return pick(rng, pool.signatures)
	case "countermeasure":
		return pick(rng, pool.countermeasures)
	case "suppress_verb":
		return pick(rng, pool.suppressVerbs)
	case "organization":
		return pick(rng, pool.organizations)
	case "organization2":
		return pick(rng, pool.organizations)
	case "prize":
		return pick(rng, pool.prizes)
	case "penalty":
		return pick(rng, pool.penalties)
	case "device":
		return pick(rng, pool.devices)
	case "role":
		return pick(rng, pool.roles)
	case "outcome":
		return pick(rng, pool.outcomes)
	case "side_effect":
		return pick(rng, pool.sideEffects)
	case "grounded_action":
		return pick(rng, pool.groundedActions)
	case "grounded_action2":
		return pick(rng, pool.groundedActions)
	case "deadline_action":
		return pick(rng, []string{"进入航道", "关闭反应堆", "切换静默航行", "进入阴影区", "撤离", "锁定航道"})
	case "route_place":
		return pick(rng, []string{"潮汐阴影区", "航道入口", "虫洞边缘", "残骸带"})
	case "nav_device":
		return pick(rng, []string{"虫洞信标", "机械罗盘", "主动声呐", "虫洞探针", "导航数据", "引力锚"})
	case "counter_device":
		return pick(rng, []string{"反应堆余热", "废热回收器", "冷却回路", "补给舱"})
	case "transport_device":
		return pick(rng, []string{"虫洞信标", "跃迁核心", "引力锚", "虫洞探针", "航道锁"})
	case "lock_device":
		return pick(rng, []string{"航道锁", "反应堆", "跃迁核心", "护盾发生器", "虫洞探针"})
	case "dilemma", "dilemma2":
		return pick(rng, []string{"是否放弃伤员", "是否关闭生命维持", "是否冒险进入虫巢", "是否抛弃战舰", "是否向敌方投降", "是否启动自毁", "是否交出虫洞坐标", "是否切断通讯"})
	case "active_nav_device":
		return pick(rng, []string{"虫洞信标", "主动声呐", "虫洞探针"})
	case "emcon_action":
		return pick(rng, []string{"静默航行", "关闭反应堆", "进入阴影区", "涂覆隔热层"})
	case "resupply_side_effect":
		return pick(rng, []string{"暴露热签名", "引来虫族", "造成通讯延迟"})
	case "ecological_side_effect":
		return pick(rng, []string{"暴露热签名", "引来更多虫群", "造成通讯延迟", "让护盾过载"})
	case "physics_side_effect":
		return pick(rng, []string{"暴露热签名", "引来虫族", "让护盾过载"})
	case "salvage_action":
		return pick(rng, []string{"抢夺航道", "封锁矿站", "袭击补给船", "征收过路费", "黑市收购", "派兵驻守"})
	case "tide_time":
		return pick(rng, []string{"下一次潮汐到来时", "虫洞闭合前", "护盾归零前", "第7天", "第13个周期", "午夜", "日落"})
	case "nav_emcon_action":
		return pick(rng, []string{"静默航行", "进入阴影区", "涂覆隔热层"})
	case "physics_place":
		return pick(rng, []string{"虫洞边缘", "航道入口", "潮汐阴影区", "残骸带", "轨道电梯", "虫巢外围"})
	case "physics_counter_device":
		return pick(rng, []string{"补给舱", "密封舱", "冷却回路", "废热回收器"})
	case "ecological_dilemma", "ecological_dilemma2":
		return pick(rng, []string{"是否关闭生命维持", "是否放弃伤员", "是否抛弃战舰", "是否继续采集", "是否切断通讯", "是否暴露坐标"})
	case "institution_thing":
		return pick(rng, []string{"通行许可", "跃迁配额", "军用许可证", "舰船识别码", "保险单", "虫洞坐标"})
	case "grounded_time":
		return pick(rng, pool.groundedTimes)
	case "grounded_thing":
		return pick(rng, pool.groundedThings)
	case "grounded_target":
		return pick(rng, pool.groundedTargets)
	case "grounded_place":
		return pick(rng, pool.groundedPlaces)
	case "grounded_place2":
		return pick(rng, pool.groundedPlaces2)
	case "fuel_resource":
		return pick(rng, pool.fuelResources)
	case "life_resource":
		return pick(rng, pool.lifeResources)
	default:
		return "某物"
	}
}

func generateName(rng *rand.Rand, pool wordPool) (string, string, map[string]string) {
	syllables := []string{
		"阿", "尔", "癸", "卯", "昙", "烬", "澜", "柩", "砾", "鸢",
		"槐", "汐", "珀", "魇", "阙", "陵", "烛", "苔", "鸦", "霜",
		"玄", "弥", "朔", "砚", "黎", "桓", "漪", "檀", "默", "照",
	}
	suffixes := []string{"台", "司", "阁", "局", "渡", "井", "门", "院", "所", "仓", "塔", "祠", "驿", "坊", "监", "庭"}
	name := pick(rng, syllables) + pick(rng, syllables) + pick(rng, suffixes)
	if rng.Intn(2) == 0 {
		name = pick(rng, syllables) + name
	}
	sentence := fmt.Sprintf("在%s，人们把%s叫作“%s”，但没有人敢在%s提起这个名字。",
		pick(rng, pool.places), pick(rng, pool.things), name, pick(rng, pool.times))
	return sentence, "name_syllable", map[string]string{"name": name}
}

func requiredRole(level string) string {
	switch level {
	case LevelEvent:
		return "必须成为一条真实发生过的历史事件，并留下可查证的后果、受益者和受害者。"
	case LevelLaw:
		return "必须成为一条普适的世界法则：说明什么被守恒或转移、谁执行、代价是什么、普通人如何利用或绕过它。"
	case LevelName:
		return "必须成为一个专有名词（地点、机构、功法或禁忌），并说明它的来源、用途和禁忌。"
	default:
		return "必须成为世界的一条可执行规则或制度；不能只是传说、梦、隐喻、幻觉或巧合。"
	}
}

func mustAnswer(level string) []string {
	base := []string{
		"这条规则或事件由谁执行、记录或掩盖？",
		"谁从中受益？谁承担代价？",
		"违反它会发生什么可观察的后果？",
		"一个普通人的一天会怎样受它影响？",
		"它能产生至少三种不同性质的卷级冲突吗？",
		"哪些细节可以提前埋下，让读者事后觉得合理？",
		"这个设定最容易被合理化成的三种俗套解释是什么？你采用的解释为什么不是其中任何一种？",
	}
	switch level {
	case LevelEvent:
		return append(base,
			"它留下了什么可查证的痕迹？",
			"它如何改变资源、权力或亲属关系？",
			"它和主线人物有什么直接因果联系？",
		)
	case LevelLaw:
		return append(base,
			"这条法则改变了哪个基本单位：身份、记忆、名字、债务、时间，还是关系？",
			"它守恒什么？转移时什么东西不会消失？",
			"普通人如何利用它获利？谁最容易成为它的受害者？",
			"这条法则的漏洞是什么？为什么漏洞没有被立刻堵上？",
		)
	case LevelName:
		return append(base,
			"这个名字最早由谁使用，为什么不能被直呼？",
			"它现在的用途和最初的用途发生了什么偏移？",
		)
	default:
		return base
	}
}

func forbiddenExplanations() []string {
	return []string{
		"梦", "幻觉", "传说", "隐喻", "精神分裂", "其实没有发生",
		"只是巧合", "远古神秘力量", "作者也不知道为什么",
	}
}

func outputContract() []string {
	return []string{
		"为这句话写 3-5 条世界内 canon 事实；每条必须能被后续章节引用。",
		"说明谁执行、谁受益、谁承担代价、违反后的具体后果。",
		"给出至少 3 种不同性质的卷级冲突，不能都是同一种冲突换皮。",
		"给出 2-3 个可以提前埋下的线索，让反转事后合理。",
		"先写出三种最俗、最容易被预测的解释并明确排除，再给出你采用的解释。",
		"先写世界内因果，再起专有名词；不要用名词堆砌代替逻辑。",
		"只允许一条不可解释的公理；其余规则必须能从它和正常人的激励推导出来。",
	}
}

func derivationSteps() []string {
	return []string{
		"公理：用一句话写出唯一不可解释的规则。",
		"机制：这条规则在物理、经济或制度上如何运作？需要什么载体、记录或执行者？",
		"第一层后果：普通人的一天会因此改变什么？",
		"制度：谁负责执行？他们靠什么预算、权力或恐惧维持它？",
		"激励：谁受益、谁付代价？为什么受益者不废除它，受损者不推翻它？",
		"失败模式：人们会怎样绕过它？绕过为什么有成本？",
		"三类冲突：同一机制自然产生的三种不同失败模式，不是三条新规则。",
		"反转：哪个意外结果是前面 canon 事实的必然后果？它埋了哪些线索？",
	}
}

func rejectConditions() []string {
	return []string{
		"需要第二条魔法规则才能解释第一条。",
		"某个机构没有受益者、预算或执行能力。",
		"普通人可以零成本绕过这条规则。",
		"卷级冲突需要新增规则才能成立。",
		"反转没有前置线索，只是临时追加的事实。",
		"把规则解释成梦、幻觉、传说、隐喻或巧合。",
	}
}

func pick(rng *rand.Rand, values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[rng.Intn(len(values))]
}

// validSlotCombination rejects the common "technically random but semantically
// broken" outputs that make a provocation impossible to rationalize. The
// checks are deliberately narrow: they only reject combinations whose causal
// relation is already incoherent at the sentence level.
func validSlotCombination(level string, template sentenceTemplate, slots map[string]string) bool {
	if level != LevelLaw {
		return true
	}
	// Reusing the same slot is intentional (for example, "device" appears
	// twice in a chain). Two different slots with identical values, however,
	// usually create tautologies such as "身份必须用一份身份支付".
	owners := make(map[string]string, len(slots))
	for _, slot := range template.Slots {
		value := strings.TrimSpace(slots[slot])
		if value == "" {
			return false
		}
		if owner, ok := owners[value]; ok && owner != slot {
			return false
		}
		owners[value] = slot
	}

	switch template.ID {
	case "law_price":
		thing := strings.TrimSpace(slots["law_thing"])
		cost := strings.TrimSpace(slots["cost"])
		if thing == "" || cost == "" || strings.Contains(cost, thing) || strings.Contains(thing, cost) {
			return false
		}
	case "grounded_fuel":
		if !validResourceDecay(slots["fuel_resource"], slots["decay_verb"]) {
			return false
		}
		if !validResourceSource(slots["source"], slots["fuel_resource"]) {
			return false
		}
	case "grounded_tide":
		if !validHazardEffect(slots["hazard"], slots["hazard_verb"]) {
			return false
		}
	case "grounded_chain_physics":
		if !validResourceDecay(slots["resource"], slots["decay_verb"]) {
			return false
		}
	case "grounded_swarm":
		if !validSwarmPair(slots["countermeasure"], slots["suppress_verb"]) {
			return false
		}
	case "grounded_deadline":
		if !validHazardResponse(slots["deadline_action"], slots["hazard"]) {
			return false
		}
		if !validHazardEffect(slots["hazard"], slots["hazard_verb"]) {
			return false
		}
	case "grounded_life_support":
		if !validHazardResponse(slots["grounded_action"], slots["hazard"]) || !validLifeSupportHazard(slots["hazard"]) {
			return false
		}
		if !validHazardEffect(slots["hazard"], slots["hazard_verb"]) {
			return false
		}
	case "grounded_chain_navigation":
		if !validHazardEffect(slots["hazard"], slots["hazard_verb"]) {
			return false
		}
	case "grounded_salvage":
		if !validResourceSource(slots["grounded_place"], slots["resource"]) {
			return false
		}
	case "grounded_chain_resupply":
		if !validResourceSource(slots["grounded_place"], slots["resource"]) {
			return false
		}
		if strings.TrimSpace(slots["dilemma"]) == "" {
			return false
		}
	case "grounded_chain_ecological":
		if strings.TrimSpace(slots["dilemma"]) == strings.TrimSpace(slots["dilemma2"]) {
			return false
		}
		if !validSwarmPair(slots["countermeasure"], slots["suppress_verb"]) {
			return false
		}
	}
	return true
}

func validResourceDecay(resource, verb string) bool {
	allowed := map[string]map[string]bool{
		"氧气":    {"冻结": true, "泄漏": true, "挥发": true, "沸腾": true},
		"跃迁燃料":  {"衰变": true, "泄漏": true, "挥发": true, "失效": true},
		"冷却剂":   {"冻结": true, "泄漏": true, "沸腾": true, "挥发": true, "结晶": true},
		"反应堆功率": {"衰减": true, "失效": true},
		"护盾电容":  {"泄漏": true, "失效": true},
		"弹药":    {"受潮": true, "失效": true},
		"备件":    {"失效": true},
		"引力锚":   {"失效": true},
		"虫洞坐标":  {"失效": true},
	}
	if verbs, ok := allowed[resource]; ok {
		return verbs[verb]
	}
	return true
}

func validHazardResponse(action, hazard string) bool {
	allowed := map[string]map[string]bool{
		"虫洞潮汐": {"进入虫洞": true, "抢占航道": true, "抛弃战舰": true, "进入航道": true, "撤离": true, "锁定航道": true},
		"引力剪切": {"进入虫洞": true, "抛弃战舰": true, "切换静默航行": true, "进入航道": true, "撤离": true, "锁定航道": true},
		"电磁脉冲": {"关闭反应堆": true, "切换静默航行": true, "进入阴影区": true},
		"离子风暴": {"关闭反应堆": true, "切换静默航行": true, "进入阴影区": true},
		"虫族工蜂": {"关闭反应堆": true, "释放诱饵": true, "进入阴影区": true},
		"虫族母巢": {"释放诱饵": true, "进入阴影区": true, "抛弃战舰": true},
		"虫群":   {"关闭反应堆": true, "释放诱饵": true, "进入阴影区": true},
		"辐射带":  {"切换静默航行": true, "进入阴影区": true},
		"微陨石雨": {"进入阴影区": true, "抛弃战舰": true, "撤离": true},
		"孢子云":  {"切换静默航行": true, "进入阴影区": true},
		"酸蚀云":  {"切换静默航行": true, "进入阴影区": true},
	}
	if actions, ok := allowed[hazard]; ok {
		return actions[action]
	}
	return true
}

func validLifeSupportHazard(hazard string) bool {
	switch hazard {
	case "虫族工蜂", "虫群", "虫族母巢":
		return true
	default:
		return false
	}
}

func validHazardEffect(hazard, verb string) bool {
	allowed := map[string]map[string]bool{
		"虫洞潮汐": {"撕开航道": true, "扭曲导航": true, "冻结引擎": true, "切断通讯": true},
		"引力剪切": {"撕开航道": true, "扭曲导航": true, "冻结引擎": true, "撕开船体": true},
		"离子风暴": {"切断通讯": true, "让护盾失效": true, "扭曲导航": true, "冻结引擎": true},
		"电磁脉冲": {"切断通讯": true, "让护盾失效": true, "冻结引擎": true, "让电子系统瘫痪": true},
		"微陨石雨": {"撕开船体": true, "让护盾失效": true},
		"孢子云":  {"切断通讯": true, "让护盾失效": true, "腐蚀船体": true},
		"酸蚀云":  {"腐蚀船体": true, "让护盾失效": true},
		"辐射带":  {"切断通讯": true, "让护盾失效": true, "让电子系统瘫痪": true},
		"虫族工蜂": {"跟随热签名": true, "登舰": true, "撕开船体": true},
		"虫族母巢": {"跟随热签名": true, "登舰": true, "撕开船体": true},
		"虫群":   {"跟随热签名": true, "登舰": true, "撕开船体": true},
	}
	if verbs, ok := allowed[hazard]; ok {
		return verbs[verb]
	}
	return true
}

func validResourceSource(place, resource string) bool {
	allowed := map[string]map[string]bool{
		"废弃矿站":  {"冷却剂": true, "反应堆功率": true, "备件": true, "护盾电容": true},
		"补给站":   {"冷却剂": true, "反应堆功率": true, "备件": true, "护盾电容": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"母舰":    {"冷却剂": true, "反应堆功率": true, "备件": true, "护盾电容": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"残骸":    {"冷却剂": true, "备件": true, "护盾电容": true, "弹药": true, "跃迁燃料": true},
		"敌方战舰":  {"冷却剂": true, "备件": true, "护盾电容": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"虫巢":    {"冷却剂": true, "备件": true, "护盾电容": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"冷冻舱":   {"氧气": true, "冷却剂": true},
		"黑市":    {"弹药": true, "备件": true, "护盾电容": true, "氧气": true, "跃迁燃料": true},
		"殖民局":   {"弹药": true, "备件": true, "护盾电容": true, "氧气": true, "跃迁燃料": true, "反应堆功率": true},
		"旧码头":   {"冷却剂": true, "备件": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"母舰机库":  {"冷却剂": true, "反应堆功率": true, "备件": true, "护盾电容": true, "弹药": true, "氧气": true, "跃迁燃料": true, "引力锚": true},
		"轨道电梯":  {"冷却剂": true, "反应堆功率": true, "备件": true, "护盾电容": true, "氧气": true},
		"虫巢外围":  {"冷却剂": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"潮汐阴影区": {"冷却剂": true, "备件": true, "护盾电容": true, "氧气": true},
		"航道入口":  {"冷却剂": true, "备件": true, "弹药": true, "氧气": true, "跃迁燃料": true},
		"虫洞边缘":  {"冷却剂": true, "备件": true, "弹药": true, "氧气": true, "跃迁燃料": true},
	}
	if resources, ok := allowed[place]; ok {
		return resources[resource]
	}
	return true
}

func validSwarmPair(countermeasure, verb string) bool {
	allowed := map[string]map[string]bool{
		"关闭反应堆": {"迷惑": true, "避开": true, "驱散": true},
		"静默航行":  {"迷惑": true, "避开": true},
		"释放诱饵":  {"引开": true, "迷惑": true, "驱散": true},
		"进入阴影区": {"避开": true, "迷惑": true},
		"切断通讯":  {"避开": true, "迷惑": true},
		"涂覆隔热层": {"避开": true, "迷惑": true},
	}
	if verbs, ok := allowed[countermeasure]; ok {
		return verbs[verb]
	}
	return true
}

func randomSeed() int64 {
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err == nil {
		seed := int64(binary.LittleEndian.Uint64(buf[:]) & 0x7fffffffffffffff)
		if seed != 0 {
			return seed
		}
	}
	seed := time.Now().UnixNano() & 0x7fffffffffffffff
	if seed == 0 {
		return 1
	}
	return seed
}
