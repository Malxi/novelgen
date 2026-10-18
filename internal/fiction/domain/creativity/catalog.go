package creativity

// wordPool is the vocabulary used by the sentence grammar. The pools are
// intentionally broad and mostly mundane: the surprise comes from putting a
// normal institutional verb next to an impossible subject, not from replacing
// every word with fantasy jargon.
type wordPool struct {
	subjects          []string
	verbs             []string
	intransitiveVerbs []string
	eventVerbs        []string
	giveVerbs         []string
	objects           []string
	times             []string
	places            []string
	targets           []string
	things            []string
	timeUnits         []string
	units             []string
	consequences      []string
	goals             []string
	reasons           []string
	contracts         []string
	transferVerbs     []string
	recipients        []string
	enforcers         []string
	actions           []string
	conditions        []string
	costs             []string
	lawConsequences   []string
	lawThings         []string
	deadlines         []string
	deadlineVerbs     []string
	hazards           []string
	hazardVerbs       []string
	groundedResources []string
	decayVerbs        []string
	sources           []string
	vehicles          []string
	systems           []string
	creatures         []string
	signatures        []string
	countermeasures   []string
	suppressVerbs     []string
	organizations     []string
	prizes            []string
	penalties         []string
	devices           []string
	roles             []string
	outcomes          []string
	sideEffects       []string
	groundedActions   []string
	groundedTimes     []string
	groundedThings    []string
	groundedTargets   []string
	groundedPlaces    []string
	groundedPlaces2   []string
	fuelResources     []string
	lifeResources     []string
}

type sentenceTemplate struct {
	ID     string
	Format string
	Slots  []string
	Wild   bool
}

var axiomTemplates = []sentenceTemplate{
	{ID: "axiom_time_ritual", Format: "在%s，%s会替%s%s%s。", Slots: []string{"time", "subject", "target", "verb", "object"}},
	{ID: "axiom_place_audit", Format: "%s在%s对%s进行%s。", Slots: []string{"place", "time", "object", "verb"}, Wild: true},
	{ID: "axiom_all_must", Format: "所有%s必须在%s%s%s。", Slots: []string{"subject", "time", "verb", "object"}},
	{ID: "axiom_election", Format: "%s每%s%s一次%s。", Slots: []string{"subject", "time_unit", "verb", "object"}, Wild: true},
	{ID: "axiom_only", Format: "只有%s能在%s%s%s。", Slots: []string{"subject", "place", "verb", "object"}},
	{ID: "axiom_forbid", Format: "所有%s禁止在%s%s%s。", Slots: []string{"subject", "time", "verb", "object"}},
	{ID: "axiom_keeper", Format: "%s负责保管%s的%s。", Slots: []string{"subject", "place", "thing"}},
	{ID: "axiom_stored", Format: "%s里保存着%s的%s。", Slots: []string{"place", "target", "thing"}},
	{ID: "axiom_grows", Format: "%s的%s每%s长出%s。", Slots: []string{"place", "thing", "time_unit", "unit"}, Wild: true},
	{ID: "axiom_write", Format: "%s在%s替%s写%s。", Slots: []string{"subject", "time", "target", "thing"}, Wild: true},
	{ID: "axiom_clear", Format: "%s每%s会%s一次%s。", Slots: []string{"subject", "time_unit", "verb", "object"}},
	{ID: "axiom_anyone", Format: "任何人不得在%s%s%s。", Slots: []string{"place", "verb", "object"}},
	{ID: "axiom_only_by", Format: "%s只能由%s%s。", Slots: []string{"thing", "subject", "verb"}},
	{ID: "axiom_each", Format: "到了%s，%s都会%s%s。", Slots: []string{"time", "subject", "verb", "object"}},
	{ID: "axiom_if", Format: "如果%s在%s%s，%s就必须%s。", Slots: []string{"subject", "time", "intransitive_verb", "target", "intransitive_verb"}, Wild: true},
	{ID: "axiom_count", Format: "%s每年只能%s%s一次。", Slots: []string{"subject", "verb", "object"}},
	{ID: "axiom_name", Format: "在%s，%s的%s必须由%s命名。", Slots: []string{"place", "target", "thing", "subject"}},
	{ID: "axiom_witness", Format: "%s必须由%s见证，否则%s。", Slots: []string{"thing", "subject", "consequence"}, Wild: true},
	{ID: "axiom_transfer", Format: "%s可以把%s抵押给%s，但必须留下%s。", Slots: []string{"subject", "thing", "target", "object"}},
	{ID: "axiom_public_list", Format: "%s会把%s的%s公开写在%s上。", Slots: []string{"subject", "target", "thing", "place"}, Wild: true},
	{ID: "axiom_inherit_duty", Format: "%s死后，%s必须继承%s的%s，直到%s。", Slots: []string{"target", "subject", "target2", "thing", "time"}, Wild: true},
	{ID: "axiom_unaskable", Format: "任何人不得询问%s为什么会在%s留下%s。", Slots: []string{"target", "place", "thing"}, Wild: true},
	{ID: "axiom_liability", Format: "如果%s在%s%s，%s必须替它偿还%s。", Slots: []string{"subject", "place", "intransitive_verb", "target", "thing"}, Wild: true},
	{ID: "axiom_reincarnate", Format: "%s被%s注销后，会以%s的身份重新出现。", Slots: []string{"target", "subject", "thing"}, Wild: true},
	{ID: "axiom_responsible", Format: "%s必须为%s在%s留下的%s负责。", Slots: []string{"subject", "target", "place", "thing"}},
	{ID: "axiom_quota", Format: "%s每%s只能让%s带走%s。", Slots: []string{"subject", "time_unit", "target", "thing"}},
	{ID: "axiom_mirror_office", Format: "每个%s在%s必须向%s报告自己的%s。", Slots: []string{"target", "time", "subject", "thing"}, Wild: true},
}

var eventTemplates = []sentenceTemplate{
	{ID: "event_take", Format: "%s在%s%s了%s的%s。", Slots: []string{"subject", "place", "event_verb", "target", "thing"}, Wild: true},
	{ID: "event_give", Format: "%s把%s的%s%s%s。", Slots: []string{"subject", "target", "thing", "give_verb", "target2"}, Wild: true},
	{ID: "event_for", Format: "为了%s，%s在%s%s了%s。", Slots: []string{"goal", "subject", "place", "event_verb", "thing"}, Wild: true},
	{ID: "event_after_death", Format: "%s死后，%s才发现%s一直由%s保管。", Slots: []string{"target", "subject", "thing", "subject"}, Wild: true},
	{ID: "event_year", Format: "在%s，%s在%s%s了%s，从此%s。", Slots: []string{"time", "subject", "place", "event_verb", "thing", "consequence"}, Wild: true},
	{ID: "event_after", Format: "%s被%s%s之后，%s开始%s。", Slots: []string{"target", "subject", "event_verb", "subject", "intransitive_verb"}, Wild: true},
	{ID: "event_nobody", Format: "从%s开始，%s每月%s一次%s，官方档案只记录结果。", Slots: []string{"time", "subject", "verb", "object"}, Wild: true},
	{ID: "event_contract", Format: "%s和%s在%s签了一份关于%s的%s。", Slots: []string{"subject", "target", "place", "thing", "contract"}},
	{ID: "event_first", Format: "第一份%s不是在%s签的，而是在%s由%s口述。", Slots: []string{"thing", "place", "place2", "subject"}, Wild: true},
	{ID: "event_confiscate", Format: "%s在%s没收了%s的%s，理由是%s。", Slots: []string{"subject", "place", "target", "thing", "reason"}},
	{ID: "event_witness", Format: "%s在%s看见了%s，第二天%s就%s了。", Slots: []string{"subject", "place", "object", "target", "intransitive_verb"}, Wild: true},
	{ID: "event_law", Format: "%s通过了一条关于%s的法律，从%s起，%s必须%s%s。", Slots: []string{"subject", "thing", "time", "target", "verb", "object"}},
	{ID: "event_rewrite", Format: "%s在%s把%s的%s改写成了%s。", Slots: []string{"subject", "place", "target", "thing", "object"}, Wild: true},
	{ID: "event_public_read", Format: "%s在%s当众宣读了%s，随后%s必须%s。", Slots: []string{"subject", "place", "thing", "target", "intransitive_verb"}, Wild: true},
	{ID: "event_trial", Format: "%s因为%s在%s受审，但唯一的证人是%s。", Slots: []string{"target", "reason", "place", "thing"}, Wild: true},
	{ID: "event_forged", Format: "%s发现%s的%s是%s在%s伪造的。", Slots: []string{"subject", "target", "thing", "target2", "place"}, Wild: true},
	{ID: "event_confession", Format: "%s在%s承认%s其实一直属于%s。", Slots: []string{"target", "place", "thing", "subject"}, Wild: true},
	{ID: "event_quarantine", Format: "%s把%s在%s发现的%s封存起来，直到%s。", Slots: []string{"subject", "target", "place", "thing", "time"}, Wild: true},
}

var lawTemplates = []sentenceTemplate{
	{ID: "law_transfer", Format: "每一个%s都可以被%s，代价是%s。", Slots: []string{"law_thing", "transfer_verb", "cost"}, Wild: true},
	{ID: "law_conservation", Format: "%s不能被销毁，只能转移给%s；转移必须由%s登记。", Slots: []string{"law_thing", "recipient", "enforcer"}},
	{ID: "law_event_transfer", Format: "每一次%s都会把%s转给%s，直到%s。", Slots: []string{"action", "law_thing", "recipient", "deadline"}, Wild: true},
	{ID: "law_deadline", Format: "如果%s没有在%s前%s，%s就会自动归%s所有。", Slots: []string{"target", "deadline", "deadline_verb", "law_thing", "recipient"}},
	{ID: "law_collateral", Format: "%s可以被抵押，但抵押期间%s不能转让。", Slots: []string{"law_thing", "target"}},
	{ID: "law_witness", Format: "只有%s能见证%s；没有见证的%s不算数。", Slots: []string{"enforcer", "law_thing", "law_thing"}, Wild: true},
	{ID: "law_inheritance", Format: "%s死后，他的%s会自动转给%s，除非%s。", Slots: []string{"target", "law_thing", "recipient", "condition"}, Wild: true},
	{ID: "law_forbid", Format: "任何人不得%s自己的%s；否则%s。", Slots: []string{"transfer_verb", "law_thing", "law_consequence"}, Wild: true},
	{ID: "law_settlement", Format: "%s每%s结算一次，未结清的部分由%s承担。", Slots: []string{"law_thing", "time_unit", "recipient"}},
	{ID: "law_exception", Format: "任何%s都可以%s，除非%s。", Slots: []string{"law_thing", "transfer_verb", "condition"}, Wild: true},
	{ID: "law_price", Format: "%s必须用%s支付，不能直接用钱。", Slots: []string{"law_thing", "cost"}},
}

// groundedLawTemplates are for hard-SF / engineering-flavored campaigns. The
// randomness lives in which physical or economic constraint binds, not in a
// magical conservation law.
var groundedLawTemplates = []sentenceTemplate{
	{ID: "grounded_tide", Format: "在%s，%s会%s；只有进入%s的%s能留在航道。", Slots: []string{"tide_time", "hazard", "hazard_verb", "route_place", "vehicle"}},
	{ID: "grounded_fuel", Format: "%s在虫洞边缘会%s，%s必须在%s前从%s取得%s。", Slots: []string{"fuel_resource", "decay_verb", "vehicle", "grounded_time", "source", "fuel_resource"}},
	{ID: "grounded_storm", Format: "只有%s能在%s中行动，因为%s会瘫痪%s，而它保留了机械备份。", Slots: []string{"vehicle", "hazard", "hazard", "system"}},
	{ID: "grounded_swarm", Format: "%s会被%s吸引；%s能暂时%s它们。", Slots: []string{"creature", "signature", "countermeasure", "suppress_verb"}},
	{ID: "grounded_arena", Format: "%s举办这场大逃杀：胜者获得%s，败者%s。", Slots: []string{"organization", "prize", "penalty"}},
	{ID: "grounded_life_support", Format: "每艘战舰的%s只能支撑到%s；超过时限，%s必须%s，否则%s会%s。", Slots: []string{"life_resource", "grounded_time", "grounded_target", "grounded_action", "hazard", "hazard_verb"}},
	{ID: "grounded_wormhole", Format: "%s会把%s从%s送到%s，但会%s。", Slots: []string{"transport_device", "grounded_thing", "grounded_place", "grounded_place2", "side_effect"}},
	{ID: "grounded_lock", Format: "只有%s能启动%s；一旦启动，它会%s。", Slots: []string{"role", "lock_device", "outcome"}},
	{ID: "grounded_deadline", Format: "所有%s必须在%s前%s；否则%s会%s。", Slots: []string{"vehicle", "grounded_time", "deadline_action", "hazard", "hazard_verb"}},
	{ID: "grounded_salvage", Format: "%s的%s是%s的唯一来源；%s会为它%s。", Slots: []string{"grounded_place", "resource", "vehicle", "organization", "salvage_action"}},
	// Causal-chain templates. These are the main quality upgrade over
	// single-clause constraints: each new clause is forced by the previous
	// one, so the model cannot explain the sentence by adding an unrelated
	// institution or a second magic rule.
	{ID: "grounded_chain_navigation", Format: "%s会%s；%s只能靠%s导航，但%s会吸引%s，所以每次穿越%s前都必须%s。", Slots: []string{"hazard", "hazard_verb", "vehicle", "active_nav_device", "active_nav_device", "creature", "route_place", "nav_emcon_action"}},
	{ID: "grounded_chain_resupply", Format: "%s是%s的唯一来源；%s必须在%s前从那里取得%s，但每次采集都会%s，因此%s必须决定%s。", Slots: []string{"grounded_place", "resource", "vehicle", "grounded_time", "resource", "resupply_side_effect", "grounded_target", "dilemma"}},
	{ID: "grounded_chain_ecological", Format: "%s会被%s吸引；%s能暂时%s它们，但会%s，因此%s只能在%s和%s之间选择。", Slots: []string{"creature", "signature", "countermeasure", "suppress_verb", "ecological_side_effect", "grounded_target", "ecological_dilemma", "ecological_dilemma2"}},
	{ID: "grounded_chain_institution", Format: "%s只承认%s签发的%s；没有%s的%s会%s，因此%s成为%s的硬通货。", Slots: []string{"organization", "organization2", "institution_thing", "institution_thing", "vehicle", "penalty", "institution_thing", "organization"}},
	{ID: "grounded_chain_physics", Format: "%s在%s会%s；%s必须靠%s维持它，但%s会%s，所以%s必须%s。", Slots: []string{"resource", "physics_place", "decay_verb", "vehicle", "physics_counter_device", "physics_counter_device", "physics_side_effect", "grounded_target", "grounded_action"}},
}

func poolForGenre(genre string) wordPool {
	pool := clonePool(commonPool)
	switch genre {
	case "仙侠":
		appendPool(&pool, xianxiaPool)
	case "都市":
		appendPool(&pool, urbanPool)
	case "悬疑":
		appendPool(&pool, mysteryPool)
	case "科幻":
		appendPool(&pool, scifiPool)
	}
	return pool
}

func clonePool(source wordPool) wordPool {
	return wordPool{
		subjects:          append([]string(nil), source.subjects...),
		verbs:             append([]string(nil), source.verbs...),
		intransitiveVerbs: append([]string(nil), source.intransitiveVerbs...),
		eventVerbs:        append([]string(nil), source.eventVerbs...),
		giveVerbs:         append([]string(nil), source.giveVerbs...),
		objects:           append([]string(nil), source.objects...),
		times:             append([]string(nil), source.times...),
		places:            append([]string(nil), source.places...),
		targets:           append([]string(nil), source.targets...),
		things:            append([]string(nil), source.things...),
		timeUnits:         append([]string(nil), source.timeUnits...),
		units:             append([]string(nil), source.units...),
		consequences:      append([]string(nil), source.consequences...),
		goals:             append([]string(nil), source.goals...),
		reasons:           append([]string(nil), source.reasons...),
		contracts:         append([]string(nil), source.contracts...),
		transferVerbs:     append([]string(nil), source.transferVerbs...),
		recipients:        append([]string(nil), source.recipients...),
		enforcers:         append([]string(nil), source.enforcers...),
		actions:           append([]string(nil), source.actions...),
		conditions:        append([]string(nil), source.conditions...),
		costs:             append([]string(nil), source.costs...),
		lawConsequences:   append([]string(nil), source.lawConsequences...),
		lawThings:         append([]string(nil), source.lawThings...),
		deadlines:         append([]string(nil), source.deadlines...),
		deadlineVerbs:     append([]string(nil), source.deadlineVerbs...),
		hazards:           append([]string(nil), source.hazards...),
		hazardVerbs:       append([]string(nil), source.hazardVerbs...),
		groundedResources: append([]string(nil), source.groundedResources...),
		decayVerbs:        append([]string(nil), source.decayVerbs...),
		sources:           append([]string(nil), source.sources...),
		vehicles:          append([]string(nil), source.vehicles...),
		systems:           append([]string(nil), source.systems...),
		creatures:         append([]string(nil), source.creatures...),
		signatures:        append([]string(nil), source.signatures...),
		countermeasures:   append([]string(nil), source.countermeasures...),
		suppressVerbs:     append([]string(nil), source.suppressVerbs...),
		organizations:     append([]string(nil), source.organizations...),
		prizes:            append([]string(nil), source.prizes...),
		penalties:         append([]string(nil), source.penalties...),
		devices:           append([]string(nil), source.devices...),
		roles:             append([]string(nil), source.roles...),
		outcomes:          append([]string(nil), source.outcomes...),
		sideEffects:       append([]string(nil), source.sideEffects...),
		groundedActions:   append([]string(nil), source.groundedActions...),
		groundedTimes:     append([]string(nil), source.groundedTimes...),
		groundedThings:    append([]string(nil), source.groundedThings...),
		groundedTargets:   append([]string(nil), source.groundedTargets...),
		groundedPlaces:    append([]string(nil), source.groundedPlaces...),
		groundedPlaces2:   append([]string(nil), source.groundedPlaces2...),
		fuelResources:     append([]string(nil), source.fuelResources...),
		lifeResources:     append([]string(nil), source.lifeResources...),
	}
}

func appendPool(target *wordPool, extra wordPool) {
	target.subjects = append(target.subjects, extra.subjects...)
	target.verbs = append(target.verbs, extra.verbs...)
	target.intransitiveVerbs = append(target.intransitiveVerbs, extra.intransitiveVerbs...)
	target.eventVerbs = append(target.eventVerbs, extra.eventVerbs...)
	target.giveVerbs = append(target.giveVerbs, extra.giveVerbs...)
	target.objects = append(target.objects, extra.objects...)
	target.times = append(target.times, extra.times...)
	target.places = append(target.places, extra.places...)
	target.targets = append(target.targets, extra.targets...)
	target.things = append(target.things, extra.things...)
	target.timeUnits = append(target.timeUnits, extra.timeUnits...)
	target.units = append(target.units, extra.units...)
	target.consequences = append(target.consequences, extra.consequences...)
	target.goals = append(target.goals, extra.goals...)
	target.reasons = append(target.reasons, extra.reasons...)
	target.contracts = append(target.contracts, extra.contracts...)
	target.transferVerbs = append(target.transferVerbs, extra.transferVerbs...)
	target.recipients = append(target.recipients, extra.recipients...)
	target.enforcers = append(target.enforcers, extra.enforcers...)
	target.actions = append(target.actions, extra.actions...)
	target.conditions = append(target.conditions, extra.conditions...)
	target.costs = append(target.costs, extra.costs...)
	target.lawConsequences = append(target.lawConsequences, extra.lawConsequences...)
	target.lawThings = append(target.lawThings, extra.lawThings...)
	target.deadlines = append(target.deadlines, extra.deadlines...)
	target.deadlineVerbs = append(target.deadlineVerbs, extra.deadlineVerbs...)
	target.hazards = append(target.hazards, extra.hazards...)
	target.hazardVerbs = append(target.hazardVerbs, extra.hazardVerbs...)
	target.groundedResources = append(target.groundedResources, extra.groundedResources...)
	target.decayVerbs = append(target.decayVerbs, extra.decayVerbs...)
	target.sources = append(target.sources, extra.sources...)
	target.vehicles = append(target.vehicles, extra.vehicles...)
	target.systems = append(target.systems, extra.systems...)
	target.creatures = append(target.creatures, extra.creatures...)
	target.signatures = append(target.signatures, extra.signatures...)
	target.countermeasures = append(target.countermeasures, extra.countermeasures...)
	target.suppressVerbs = append(target.suppressVerbs, extra.suppressVerbs...)
	target.organizations = append(target.organizations, extra.organizations...)
	target.prizes = append(target.prizes, extra.prizes...)
	target.penalties = append(target.penalties, extra.penalties...)
	target.devices = append(target.devices, extra.devices...)
	target.roles = append(target.roles, extra.roles...)
	target.outcomes = append(target.outcomes, extra.outcomes...)
	target.sideEffects = append(target.sideEffects, extra.sideEffects...)
	target.groundedActions = append(target.groundedActions, extra.groundedActions...)
	target.groundedTimes = append(target.groundedTimes, extra.groundedTimes...)
	target.groundedThings = append(target.groundedThings, extra.groundedThings...)
	target.groundedTargets = append(target.groundedTargets, extra.groundedTargets...)
	target.groundedPlaces = append(target.groundedPlaces, extra.groundedPlaces...)
	target.groundedPlaces2 = append(target.groundedPlaces2, extra.groundedPlaces2...)
	target.fuelResources = append(target.fuelResources, extra.fuelResources...)
	target.lifeResources = append(target.lifeResources, extra.lifeResources...)
}

var commonPool = wordPool{
	subjects: []string{
		"殡仪馆", "税务局", "乌鸦", "台风", "镜子", "祠堂", "电梯", "海关", "医院", "图书馆",
		"当铺", "邮局", "法院", "剧院", "码头", "矿井", "气象台", "婚礼", "葬礼", "账本",
		"影子", "潮水", "钟楼", "药铺", "书院", "城门", "渡口", "灯塔", "档案馆", "银行",
		"保险公司", "物业", "仲裁庭", "剧团", "养蜂人", "守夜人", "抄经人", "摆渡人",
	},
	verbs: []string{
		"审计", "申报", "审批", "登记", "继承", "抵押", "清算", "仲裁", "征收", "抄写",
		"保管", "选举", "罢免", "公证", "验收", "召回", "封存", "拍卖", "租赁", "翻译",
		"校对", "收养", "驱逐", "追偿", "结算", "注销", "超度", "封印",
	},
	intransitiveVerbs: []string{
		"申报", "登记", "审计", "清算", "公证", "辞职", "闭馆", "停业", "失踪", "搬迁", "罢工", "开庭",
	},
	eventVerbs: []string{
		"拿走", "吞掉", "烧掉", "抵押", "拍卖", "封存", "偷走", "继承", "注销", "召回",
		"没收", "祭告", "超度",
	},
	giveVerbs: []string{
		"交给", "寄给", "送给", "转交给", "抵押给", "拍卖给", "典当给", "遗赠给",
	},
	objects: []string{
		"云层", "辞职信", "遗嘱", "账本", "影子", "雨声", "债务", "记忆", "名字", "梦境",
		"牙齿", "钥匙", "契约", "印章", "骨灰", "潮汐", "月光", "车牌", "门票", "婚书",
		"病历", "户口", "地图", "钟声", "香火", "债主", "继承人", "嫌疑犯", "证人", "尸体",
		"星星", "井水", "回声", "发票", "合同", "监控录像",
	},
	times: []string{
		"冬至", "每月初一", "每周三下午三点", "日落前", "午夜", "雨季", "第七年", "闰年",
		"每次月圆", "发薪日", "开庭前", "葬礼结束前", "第一场雪", "退潮时", "鸡叫第三遍",
		"清明", "除夕", "午夜十二点", "飞升前", "天劫后",
	},
	places: []string{
		"第三层", "地下车库", "旧码头", "西厢房", "祠堂", "海关大厅", "殡仪馆", "钟楼",
		"矿井", "渡口", "剧院后台", "法院走廊", "旧书店", "城隍庙", "医院太平间", "火车站",
		"灯塔", "图书馆地下室", "档案室", "天台", "菜市场", "婚礼现场", "婚姻登记处",
	},
	targets: []string{
		"宗主", "市长", "死者", "新郎", "债主", "继承人", "税务员", "守夜人", "乌鸦", "镜子",
		"外乡人", "保安", "法医", "房东", "租客", "邮差",
		"新娘", "长老", "掌门", "病人", "证人",
	},
	things: []string{
		"辞职信", "遗嘱", "婚书", "债务", "记忆", "名字", "骨灰", "牙齿", "钥匙", "印章",
		"门票", "病历", "户口", "地图", "账本", "契约", "发票", "合同", "车牌", "录音带",
		"旧照片", "尸检报告", "许可证", "协议",
	},
	timeUnits: []string{"年", "七年", "月", "周", "天", "季", "十年", "百年", "闰年", "雨季"},
	units:     []string{"一颗牙", "一只眼睛", "一枚印章", "一把钥匙", "一片鳞", "一根羽毛", "一枚铜钱", "一颗钉子", "一张嘴", "一只耳朵"},
	consequences: []string{
		"所有镜子开始申报收入", "祠堂的账本开始长牙", "台风不再选举市长", "没有人再敢在冬至辞职",
		"乌鸦成了正式的公证人", "殡仪馆开始收门票", "电梯拒绝停靠第三层", "海关开始征收名字",
		"医院开始保管遗嘱", "法院开始审理云层",
	},
	goals: []string{
		"还清一笔不存在的债", "让死者闭嘴", "保住一场婚礼", "换回一个名字", "掩盖一次审计",
		"通过继承审查", "让台风改道", "把遗嘱送上法庭", "让乌鸦闭嘴", "把影子抵押出去",
	},
	reasons: []string{
		"没有申报", "逾期未还", "名字写错", "缺少公证", "惊动了云层", "违反了冬至条例",
		"没有交门票", "账本对不上", "影子太重", "遗嘱过期",
	},
	contracts: []string{
		"秘密协议", "婚书", "遗嘱", "租约", "停战书", "审计报告", "收养协议",
		"债务重组协议", "器官捐献书", "离婚协议",
	},
	transferVerbs: []string{
		"转让", "抵押", "继承", "登记", "清算", "拍卖", "租赁",
		"征税", "追偿", "结算", "赎回",
	},
	recipients: []string{
		"债主", "继承人", "见证人", "法院", "医院", "祠堂", "银行", "保险公司",
		"下一个说出口的人", "最近听过它的人", "死者的家属", "登记处", "公证处",
	},
	enforcers: []string{
		"法院", "银行", "医院", "祠堂", "公证处", "税务局", "海关", "保险公司",
		"钟楼", "邮局", "档案馆", "太平间", "登记处",
	},
	actions: []string{
		"死亡", "出生", "结婚", "离婚", "继承", "借贷", "治病", "埋葬", "说谎",
		"承诺", "悔约", "改名", "搬迁", "失踪", "开庭", "审计",
	},
	conditions: []string{
		"有人替他偿还", "死者留下遗嘱", "见证人还活着", "名字被烧掉", "记忆被赎回",
		"契约被公证", "债主放弃追偿", "登记处关门", "时间到了", "有人承认",
	},
	costs: []string{
		"一段记忆", "一年寿命", "一个名字", "一份身份", "一次睡眠", "一段关系",
		"一只眼睛", "一颗牙", "一段声音", "一份继承权", "一场婚礼", "一次死亡",
	},
	lawConsequences: []string{
		"所有人开始申报记忆", "名字不能再被直呼", "法院开始审理梦", "继承人必须替死者还债",
		"镜子成为证人", "死亡需要预约", "婚姻变成债务重组", "没有人再敢说出承诺",
		"所有契约开始收利息", "失踪者会被自动注销",
	},
	lawThings: []string{
		"名字", "记忆", "债务", "承诺", "契约", "遗嘱", "身份", "寿命", "声音", "影子",
		"关系", "继承权", "信用", "协议",
	},
	deadlines: []string{
		"冬至", "日落", "午夜", "第七年", "闰年", "开庭", "葬礼结束",
		"第一场雪", "退潮", "清明", "除夕", "发薪日", "午夜十二点",
	},
	deadlineVerbs: []string{"申报", "登记", "公证", "结算", "封存", "赎回", "继承", "转让"},
	hazards: []string{
		"虫洞潮汐", "引力剪切", "离子风暴", "电磁脉冲", "微陨石雨", "孢子云", "酸蚀云", "辐射带",
	},
	hazardVerbs: []string{
		"撕开航道", "扫过战场", "切断通讯", "让护盾失效", "扭曲导航", "冻结引擎", "点燃大气",
		"跟随热签名", "登舰", "发现战舰", "撕开船体", "让电子系统瘫痪", "腐蚀船体",
	},
	groundedResources: []string{
		"跃迁燃料", "冷却剂", "氧气", "反应堆功率", "弹药", "备件", "引力锚", "虫洞坐标", "护盾电容",
	},
	decayVerbs:      []string{"衰减", "衰变", "冻结", "泄漏", "失效", "沸腾", "挥发", "结晶", "受潮"},
	sources:         []string{"残骸", "敌方战舰", "虫巢", "补给站", "冷冻舱", "黑市", "废弃矿站", "母舰"},
	vehicles:        []string{"机甲", "登陆舱", "破冰船", "无人机", "救生艇", "打捞船", "护航舰", "突袭艇"},
	systems:         []string{"战舰电子系统", "导航", "护盾", "反应堆控制", "通讯", "火控", "生命维持"},
	creatures:       []string{"虫族工蜂", "虫族母巢", "回声虫", "孢子虫", "酸蚀虫", "虫群", "寄生蜂", "掘地虫"},
	signatures:      []string{"反应堆热签名", "电磁信号", "生命维持热", "引擎尾焰", "声呐脉冲", "护盾辐射"},
	countermeasures: []string{"关闭反应堆", "静默航行", "释放诱饵", "涂覆隔热层", "进入阴影区", "切断通讯"},
	suppressVerbs:   []string{"压制", "驱散", "迷惑", "冻结", "引开", "避开", "烧退"},
	organizations:   []string{"虫洞航道局", "深空保险公司", "舰队联合体", "黑市打捞公会", "边境殖民署", "航道清算所"},
	prizes:          []string{"通行许可", "跃迁配额", "一艘新战舰", "虫洞坐标", "赦免令", "燃料优先权"},
	penalties:       []string{"留在潮汐区", "被注销身份", "成为虫族孵化场", "被永久通缉", "被剥离战舰", "被拍卖为劳工"},
	devices:         []string{"跃迁核心", "引力锚", "虫洞信标", "反应堆", "航道锁", "护盾发生器", "虫洞探针", "反应堆余热", "主动声呐", "机械罗盘", "废热回收器", "冷却回路"},
	roles:           []string{"领航员", "机甲驾驶员", "舰长", "打捞员", "虫族研究员", "通讯员", "反应堆工程师"},
	outcomes:        []string{"打开航道", "关闭虫洞", "过载", "熔毁", "召来虫群", "暴露舰队坐标", "烧毁航道"},
	sideEffects:     []string{"丢失导航", "引来虫族", "让燃料衰变", "造成通讯延迟", "让护盾过载", "暴露热签名"},
	groundedActions: []string{"进入虫洞", "抛弃战舰", "向敌方投降", "启动自毁", "请求补给", "关闭反应堆", "切换静默航行", "抢占航道"},
	groundedTimes:   []string{"49小时", "下一次潮汐", "日落", "午夜", "第7天", "第13个周期", "虫洞闭合", "护盾归零"},
	groundedThings:  []string{"跃迁核心", "虫洞坐标", "黑匣子", "信标", "燃料棒", "护盾电容", "反应堆", "导航数据", "虫巢样本", "军用许可证", "舰船识别码", "冷冻舱"},
	groundedTargets: []string{"舰长", "船员", "驾驶员", "领航员", "打捞员", "幸存者", "通讯员", "舰队"},
	groundedPlaces:  []string{"虫洞边缘", "航道入口", "废弃矿站", "母舰机库", "殖民局", "旧码头", "轨道电梯", "虫巢外围", "潮汐阴影区", "残骸带"},
	groundedPlaces2: []string{"虫巢外围", "残骸带", "母舰机库", "旧码头", "殖民局", "潮汐阴影区", "轨道电梯", "航道入口"},
	fuelResources:   []string{"跃迁燃料", "冷却剂", "反应堆功率", "氧气", "护盾电容", "弹药"},
	lifeResources:   []string{"冷却剂", "氧气", "反应堆功率", "备件"},
}

var xianxiaPool = wordPool{
	subjects: []string{
		"宗门", "长老会", "剑冢", "丹房", "灵脉", "山门", "藏经阁", "葬剑谷", "药圃",
		"外门", "内门", "掌门", "守山人", "炼丹师", "抄经人",
	},
	verbs: []string{"封存", "继承", "清算", "公证", "祭告", "抄录", "审计", "抵押", "追偿", "罢免", "收养", "驱逐", "封印", "超度"},
	objects: []string{
		"命盘", "道侣", "香火", "灵牌", "剑谱", "丹方", "寿元", "因果", "道债", "天劫",
		"山门", "名讳", "灵脉", "剑意", "元婴", "洞府",
	},
	times: []string{"冬至", "每月初一", "日落前", "午夜", "第七年", "每次月圆", "飞升前", "天劫后", "清明", "除夕"},
	places: []string{
		"山门", "丹房", "剑冢", "藏经阁", "祠堂", "渡口", "葬剑谷", "钟楼", "灵脉深处",
		"外门账房", "长老殿", "山门石阶",
	},
	targets: []string{"宗主", "长老", "掌门", "外门弟子", "守山人", "债主", "死人", "新娘", "新任掌门", "丹师", "剑奴"},
	things: []string{
		"辞职信", "遗嘱", "婚书", "剑谱", "丹方", "名讳", "寿元", "灵牌", "香火", "契约",
		"账本", "命盘", "道债", "天劫文书",
	},
	lawThings:  []string{"命盘", "道债", "寿元", "香火", "因果", "名讳", "灵牌", "剑谱", "丹方", "元婴", "契约"},
	deadlines:  []string{"飞升", "天劫结束", "月圆"},
	eventVerbs: []string{"拿走", "吞掉", "烧掉", "抵押", "拍卖", "封存", "偷走", "继承", "注销", "召回", "没收", "祭告", "超度"},
	consequences: []string{
		"所有弟子开始申报香火", "祠堂的账本开始长牙", "飞升不再需要渡劫", "没有人再敢在冬至辞职",
		"乌鸦成了正式的公证人", "山门开始收门票", "剑冢拒绝埋葬长老", "天劫开始按月收费",
	},
	goals: []string{
		"还清一笔道债", "让死者闭嘴", "保住一场双修典礼", "换回一个名字", "掩盖一次香火审计",
		"通过继承审查", "让天劫改道", "把遗嘱送上长老会", "让乌鸦闭嘴", "把影子抵押给山门",
	},
	contracts: []string{"双修契约", "婚书", "遗嘱", "租约", "停战书", "审计报告", "收徒协议", "债务重组协议", "器官捐献书", "离婚协议"},
}

var urbanPool = wordPool{
	subjects: []string{
		"物业", "税务局", "殡仪馆", "电梯", "地铁站", "保险公司", "法院", "医院", "图书馆",
		"当铺", "邮局", "银行", "居委会", "保安亭", "停车场", "外卖站", "直播间",
		"物业办公室", "工商局", "仲裁庭", "婚姻登记处", "档案馆",
	},
	verbs: []string{"审计", "申报", "审批", "登记", "继承", "抵押", "清算", "仲裁", "征收", "保管", "公证", "验收", "召回", "封存", "拍卖", "租赁", "追偿", "结算", "注销", "停业"},
	objects: []string{
		"云层", "辞职信", "遗嘱", "账本", "影子", "雨声", "债务", "记忆", "名字", "病历",
		"户口", "发票", "合同", "骨灰", "车牌", "外卖单", "监控录像", "物业费", "贷款",
		"门禁卡", "房产证", "保险单",
	},
	times: []string{"冬至", "每月初一", "每周三下午三点", "日落前", "午夜", "雨季", "发薪日", "开庭前", "清明", "除夕", "午夜十二点"},
	places: []string{
		"第三层", "地下车库", "旧码头", "法院走廊", "医院太平间", "地铁站", "物业办公室",
		"旧书店", "城隍庙", "火车站", "灯塔", "天台", "菜市场", "婚礼现场", "婚姻登记处", "档案室",
	},
	targets:    []string{"业主", "死者", "新郎", "债主", "继承人", "租客", "保安", "税务员", "外乡人", "房东", "病人", "证人", "邮差", "新娘"},
	things:     []string{"遗嘱", "婚书", "病历", "户口", "发票", "合同", "骨灰", "钥匙", "印章", "门票", "账本", "契约", "房产证", "保险单"},
	lawThings:  []string{"户口", "房产证", "保险单", "病历", "合同", "发票", "车牌", "信用", "学位", "工龄", "遗嘱", "婚书", "名字", "记忆", "债务"},
	eventVerbs: []string{"拿走", "吞掉", "烧掉", "抵押", "拍卖", "封存", "偷走", "继承", "注销", "召回", "没收"},
	consequences: []string{
		"所有镜子开始申报收入", "祠堂的账本开始长牙", "台风不再选举市长", "没有人再敢在冬至辞职",
		"乌鸦成了正式的公证人", "殡仪馆开始收门票", "电梯拒绝停靠第三层", "海关开始征收名字",
		"医院开始保管遗嘱", "法院开始审理云层",
	},
	goals: []string{
		"还清一笔不存在的债", "让死者闭嘴", "保住一场婚礼", "换回一个名字", "掩盖一次审计",
		"通过继承审查", "让台风改道", "把遗嘱送上法庭", "让乌鸦闭嘴", "把影子抵押出去",
	},
	reasons:   []string{"没有申报", "逾期未还", "名字写错", "缺少公证", "惊动了云层", "违反了冬至条例", "没有交门票", "账本对不上", "影子太重", "遗嘱过期"},
	contracts: []string{"秘密协议", "婚书", "遗嘱", "租约", "停战书", "审计报告", "收养协议", "债务重组协议", "器官捐献书", "离婚协议"},
}

var mysteryPool = wordPool{
	subjects: []string{
		"殡仪馆", "档案馆", "法医室", "旧码头", "钟楼", "灯塔", "邮局", "法院", "当铺",
		"剧院后台", "医院", "物业", "保安亭", "档案室",
	},
	verbs: []string{"封存", "登记", "公证", "审计", "召回", "验收", "保管", "抄录", "清算", "追偿", "注销", "追查"},
	objects: []string{
		"遗嘱", "病历", "证物", "录音带", "旧照片", "尸检报告", "失踪人口", "钥匙", "账本",
		"车牌", "监控录像", "名字", "骨灰",
	},
	times:      []string{"冬至", "每月初一", "午夜", "雨季", "第七年", "开庭前", "葬礼结束前", "第一场雪", "退潮时", "清明", "除夕"},
	places:     []string{"太平间", "档案室", "旧码头", "钟楼", "灯塔", "剧院后台", "法院走廊", "地下车库", "第三层", "殡仪馆"},
	targets:    []string{"死者", "证人", "继承人", "失踪者", "保安", "法医", "债主", "新郎", "新娘", "邮差"},
	things:     []string{"遗嘱", "证物", "录音带", "旧照片", "尸检报告", "钥匙", "账本", "婚书", "病历", "车牌"},
	lawThings:  []string{"证物", "录音带", "尸检报告", "遗嘱", "车牌", "证词", "记忆", "名字", "病历", "合同"},
	eventVerbs: []string{"拿走", "烧掉", "封存", "偷走", "注销", "召回", "没收"},
	consequences: []string{
		"所有镜子开始申报收入", "殡仪馆开始收门票", "电梯拒绝停靠第三层", "医院开始保管遗嘱",
		"法院开始审理云层", "没有人再敢在冬至辞职",
	},
	goals: []string{
		"让死者闭嘴", "保住一场婚礼", "换回一个名字", "掩盖一次审计", "通过继承审查",
		"把遗嘱送上法庭", "让乌鸦闭嘴", "把影子抵押出去",
	},
	reasons:   []string{"没有申报", "名字写错", "缺少公证", "账本对不上", "遗嘱过期", "证物失踪", "监控坏掉"},
	contracts: []string{"秘密协议", "遗嘱", "租约", "审计报告", "收养协议", "离婚协议", "封口协议"},
}

var scifiPool = wordPool{
	subjects: []string{
		"空间站", "殖民局", "记忆库", "气闸", "轨道电梯", "无人机", "保险局", "档案馆",
		"气候局", "调度中心", "身份局",
	},
	verbs: []string{"审计", "登记", "清算", "封存", "召回", "验收", "公证", "拍卖", "结算", "翻译", "注销", "校准"},
	objects: []string{
		"记忆", "身份", "轨道", "氧气", "基因", "许可证", "协议", "数据", "影子", "债务",
		"坐标", "许可证编号", "备份",
	},
	times:      []string{"冬至", "每月初一", "午夜", "第七年", "闰年", "每次月圆", "发薪日", "第一场雪", "轨道偏移时"},
	places:     []string{"气闸", "第三层", "记忆库", "轨道电梯", "殖民局", "旧码头", "调度中心", "天台"},
	targets:    []string{"殖民者", "克隆人", "债主", "继承人", "调度员", "外乡人", "乘客", "驾驶员"},
	things:     []string{"许可证", "协议", "记忆", "身份", "遗嘱", "合同", "票据", "坐标", "备份"},
	lawThings:  []string{"记忆", "身份", "许可证", "协议", "数据", "氧气", "基因", "备份", "坐标", "合同"},
	deadlines:  []string{"轨道偏移", "燃料耗尽", "气闸关闭"},
	eventVerbs: []string{"拿走", "吞掉", "封存", "偷走", "注销", "召回", "没收", "上传"},
	consequences: []string{
		"所有镜子开始申报收入", "海关开始征收名字", "法院开始审理云层", "电梯拒绝停靠第三层",
		"轨道开始按月收费", "记忆库开始收门票",
	},
	goals: []string{
		"还清一笔不存在的债", "换回一个名字", "掩盖一次审计", "通过继承审查", "把影子抵押出去",
		"让轨道改道", "取回一份备份",
	},
	reasons:   []string{"没有申报", "逾期未还", "名字写错", "缺少公证", "惊动了云层", "违反了轨道条例", "没有交门票", "备份对不上"},
	contracts: []string{"秘密协议", "租约", "停战书", "审计报告", "收养协议", "债务重组协议", "器官捐献书"},
}
