package creativity

import (
	"fmt"
	"math/rand"
	"strings"
)

// Engine is a deterministic "institution + artifact + inversion" seed. It is
// deliberately more constrained than a free-form provocation: the artifact,
// issuer, holder, hidden function, loophole, cost and protagonist role are
// sampled together so the generated world has a causal spine before the model
// writes any prose.
type Engine struct {
	ID                 string   `json:"id"`
	Seed               int64    `json:"seed"`
	Genre              string   `json:"genre"`
	Family             string   `json:"family"`
	Artifact           string   `json:"artifact"`
	Issuer             string   `json:"issuer"`
	Holder             string   `json:"holder"`
	WorldAnchor        string   `json:"world_anchor"`
	StatedPurpose      string   `json:"stated_purpose"`
	HiddenFunction     string   `json:"hidden_function"`
	Trigger            string   `json:"trigger"`
	Loophole           string   `json:"loophole"`
	Cost               string   `json:"cost"`
	ProtagonistRole    string   `json:"protagonist_role"`
	PublicSecret       string   `json:"public_secret"`
	DefaultExpectation string   `json:"default_expectation"`
	CounterExpectation string   `json:"counter_expectation"`
	ConflictTypes      []string `json:"conflict_types"`
	SignatureWin       string   `json:"signature_win"`
	OpponentMisread    string   `json:"opponent_misread"`
	PremiseScaffold    string   `json:"premise_scaffold"`
	RuleSeeds          []string `json:"rule_seeds"`
	StorylineSeeds     []string `json:"storyline_seeds"`
	CastSeeds          []string `json:"cast_seeds"`
	VolumeEscalation   []string `json:"volume_escalation"`
	RequiredQuestions  []string `json:"required_questions"`
	RejectIf           []string `json:"reject_if"`
	Tags               []string `json:"tags"`
}

// EngineResult is one deterministic engine-card draw.
type EngineResult struct {
	Seed     int64    `json:"seed"`
	Genre    string   `json:"genre"`
	Engines  []Engine `json:"engines"`
	Warnings []string `json:"warnings,omitempty"`
}

// EngineRequest controls one engine-card draw.
type EngineRequest struct {
	Genre  string
	Seed   int64
	Count  int
	Source string
}

const maxEngineCount = 20

type engineFamilyDef struct {
	ID              string
	IssuerSuffix    string
	RoleSuffix      string
	Assets          map[string]string
	StatedPurposes  []string
	HiddenFunctions []string
	Triggers        []string
	Loopholes       []string
	SignatureWins   []string
	Misreads        []string
	ConflictTypes   []string
	CounterMoves    []string
}

var engineFamilies = []engineFamilyDef{
	{
		ID:           "insurance",
		IssuerSuffix: "保险公司",
		RoleSuffix:   "理赔员",
		Assets: map[string]string{
			"科幻": "虫洞战舰", "仙侠": "灵脉洞府", "都市": "住宅", "悬疑": "证物仓", "通用": "资产",
		},
		StatedPurposes: []string{
			"承保损失并在事故后赔付",
			"把不可预测的灾难变成可计算的保费",
		},
		HiddenFunctions: []string{
			"用理赔数据预测下一次事故",
			"把救援延迟变成利润",
			"把死者变成可定价资产",
			"迫使幸存者放弃索赔换取救援",
			"把风险转嫁给没有投保的人",
		},
		Triggers: []string{"全损认定", "虫潮爆发", "引力剪切", "失踪满七日", "继承人申报"},
		Loopholes: []string{
			"只有保险人信标确认的残骸才算全损，而信标在船上",
			"理赔必须在事故前备案，事故后补办一律无效",
			"失踪者要等七年才能视同死亡，除非有人替他还清保费",
			"非战斗损失不在承保范围内，但虫族造成的损失很难被认定为战斗",
			"索赔人必须证明自己不是受益人的亲属",
		},
		SignatureWins: []string{
			"用保险公司自己的风险模型证明事故是计划内成本",
			"让敌人替自己支付理赔金",
			"把一艘已注销的船重新登记为事故船",
			"在公开听证会上让理赔官承认沉船数据被篡改",
		},
		Misreads: []string{
			"以为主角要逃理赔，没想到主角要理赔所有船",
			"以为主角会揭露骗局，没想到主角用骗局救船",
			"以为主角是受害者，没想到主角是唯一能签字的人",
		},
		ConflictTypes: []string{"资源", "制度", "认知", "道德", "关系"},
		CounterMoves: []string{
			"主角发现取消理赔会让舰队断粮，只能用骗局逼公司兑现",
			"主角选择让公司继续存在，但把理赔权交给所有船员",
			"主角把自己登记为事故资产，换取一个救援窗口",
		},
	},
	{
		ID:           "license",
		IssuerSuffix: "管理局",
		RoleSuffix:   "稽查员",
		Assets: map[string]string{
			"科幻": "虫洞航道", "仙侠": "秘境入口", "都市": "夜市摊位", "悬疑": "案发现场", "通用": "危险作业",
		},
		StatedPurposes: []string{"允许持有人从事高风险作业", "把稀缺的通行机会分配给合格者"},
		HiddenFunctions: []string{
			"筛选可以被征用的劳动力",
			"把通行权变成可以炒卖的金融产品",
			"用续期控制人口流动",
			"为特定势力预留航道",
			"合法没收违规者的全部资产",
		},
		Triggers: []string{"续期审计", "航道封锁", "事故调查", "虫潮预警", "身份复核"},
		Loopholes: []string{
			"许可证只认编号不认人，编号可以通过继承转移",
			"临时豁免可以由上级追认，但追认前的一切行为都算违规",
			"只在原辖区有效，跨区作业需要当地担保人",
			"许可证的副本可以单独抵押，正本仍归原持有人",
			"过期前二十四小时提交续期申请，旧证可以继续使用一年",
		},
		SignatureWins: []string{
			"用敌人的许可证合法进入敌人的禁区",
			"把一张过期许可证变成一场公开审判的证据",
			"让管理局自己承认临时豁免的追认记录",
		},
		Misreads: []string{
			"以为主角要偷许可证，没想到主角要申请一张合法许可证",
			"以为主角会违规，没想到主角用规则把审查员变成违规者",
			"以为许可证是护身符，没想到许可证是定位器",
		},
		ConflictTypes: []string{"资源", "制度", "身份", "道德", "关系"},
		CounterMoves: []string{
			"主角放弃自己的许可证，让整支舰队都变成无证者，迫使管理局重新发证",
			"主角把许可证公开拍卖，让敌人无法独占航道",
			"主角承认违规，换取一次公开听证",
		},
	},
	{
		ID:           "contract",
		IssuerSuffix: "公会",
		RoleSuffix:   "仲裁员",
		Assets: map[string]string{
			"科幻": "机甲驾驶员", "仙侠": "外门弟子", "都市": "外卖骑手", "悬疑": "线人", "通用": "劳工",
		},
		StatedPurposes: []string{"约定工作内容、报酬与违约责任", "把危险工作变成可执行的雇佣关系"},
		HiddenFunctions: []string{
			"把身体、记忆或未来收益抵押给雇主",
			"让工人承担设备损失",
			"用竞业条款锁死技能",
			"把罢工定义为违约",
			"把死亡算作自动离职",
		},
		Triggers: []string{"罢工", "工伤", "战损", "违约仲裁", "合同续签"},
		Loopholes: []string{
			"合同只保护签字人，替班的人不受保护",
			"雇主可以在任务开始前单方面修改风险条款",
			"伤残赔偿按最后一次出勤计算，而不是按实际伤情",
			"辞职需要提前三十天通知，但被解雇可以立即生效",
			"仲裁员由雇主指定，除非工人能证明存在利益冲突",
		},
		SignatureWins: []string{
			"让雇主自己违反合同，从而触发工人的集体索赔权",
			"把一份死亡合同变成继承合同",
			"用仲裁程序冻结敌人的战舰",
		},
		Misreads: []string{
			"以为主角要罢工，没想到主角要按合同索赔",
			"以为主角是雇主的人，没想到主角是替班工人",
			"以为合同能保护主角，没想到合同把主角变成了设备",
		},
		ConflictTypes: []string{"制度", "资源", "关系", "道德", "身份"},
		CounterMoves: []string{
			"主角承认自己违约，让所有替班工人获得同等索赔权",
			"主角把合同公开，让敌人无法继续雇佣任何人",
			"主角选择留在合同里，从内部改写风险条款",
		},
	},
	{
		ID:           "inheritance",
		IssuerSuffix: "公证处",
		RoleSuffix:   "公证员",
		Assets: map[string]string{
			"科幻": "舰籍与债务", "仙侠": "道统与道债", "都市": "房产与户口", "悬疑": "遗产与证物", "通用": "身份与债务",
		},
		StatedPurposes: []string{"把死者的身份、权利与债务转给合法继承人", "确认谁有资格继续使用死者的资源"},
		HiddenFunctions: []string{
			"把债务连同身份一起继承",
			"让机构成为每个家庭的隐形继承人",
			"用继承资格控制婚姻",
			"把死者变成永久债务人",
			"制造可以交易的未来继承权",
		},
		Triggers: []string{"死亡申报", "继承听证", "遗嘱开封", "债务结算", "身份注销"},
		Loopholes: []string{
			"继承权可以在死者死亡前转让，但转让记录不公开",
			"没有直系亲属时，公证处可以指定继承人",
			"遗嘱的副本有效，正本只在争议时出现",
			"继承人可以选择只继承财产、不继承债务，但必须证明自己不知情",
			"死亡时间以登记时间为准，而不是医学时间",
		},
		SignatureWins: []string{
			"让敌人继承一笔他无法拒绝的债务",
			"用一份伪造的死亡时间证明继承无效",
			"把整支舰队的债务集中到一个已死的人身上",
		},
		Misreads: []string{
			"以为主角要争遗产，没想到主角要继承债务",
			"以为死者是受害者，没想到死者是债权人",
			"以为公证处是中立的，没想到公证处是继承人",
		},
		ConflictTypes: []string{"身份", "制度", "关系", "道德", "资源"},
		CounterMoves: []string{
			"主角放弃继承权，让债务回到债权人身上",
			"主角承认自己伪造了死亡时间，迫使公证处公开全部继承记录",
			"主角把继承权拆成多份，让所有船员都成为债权人",
		},
	},
	{
		ID:           "death_record",
		IssuerSuffix: "殡葬署",
		RoleSuffix:   "法医",
		Assets: map[string]string{
			"科幻": "冷冻舱死者", "仙侠": "剑冢亡魂", "都市": "无名尸", "悬疑": "失踪者", "通用": "死者",
		},
		StatedPurposes: []string{"确认死亡并安排善后", "给死者一个可被系统识别的身份"},
		HiddenFunctions: []string{
			"决定谁有资格领取死者留下的资源",
			"掩盖失踪人口",
			"把活人登记为死者",
			"把死亡时间变成可以拍卖的数据",
			"用死亡登记清理不受欢迎的人口",
		},
		Triggers: []string{"无名尸出现", "死亡人数异常", "失踪申报", "集体葬礼", "身份复核"},
		Loopholes: []string{
			"没有家属认领的尸体可以由殡葬署代为火化，骨灰不再保留",
			"死亡时间可以由两名证人推翻",
			"在虫洞附近失踪的人，七年后自动视为死亡",
			"尸体在冷冻舱中保存超过一年，身份可以被重新分配",
			"非自然死亡必须由法医签字，但法医的签字可以被上级追认",
		},
		SignatureWins: []string{
			"用死亡登记证明一个活人不存在",
			"让敌人自己的死亡确认书成为伪证",
			"把一具无名尸变成整艘船的所有权凭证",
		},
		Misreads: []string{
			"以为主角在找凶手，没想到主角在找自己的死亡记录",
			"以为死者已经死了，没想到死者刚刚完成登记",
			"以为法医是中立证人，没想到法医是唯一能修改死亡时间的人",
		},
		ConflictTypes: []string{"认知", "身份", "制度", "道德", "关系"},
		CounterMoves: []string{
			"主角承认自己已经死了，让整艘船的登记失效",
			"主角拒绝签字，让所有无名尸都无法被清理",
			"主角把死亡时间公开，让敌人无法再伪造失踪",
		},
	},
	{
		ID:           "salvage",
		IssuerSuffix: "打捞公会",
		RoleSuffix:   "打捞员",
		Assets: map[string]string{
			"科幻": "战舰残骸", "仙侠": "陨落剑修洞府", "都市": "事故车", "悬疑": "沉船证物", "通用": "残骸",
		},
		StatedPurposes: []string{"确认残骸所有权并允许合法打捞", "把战场废弃物变成可回收资源"},
		HiddenFunctions: []string{
			"把救援变成掠夺",
			"用坐标垄断补给",
			"让沉船成为保险资产",
			"迫使幸存者签署放弃书",
			"用虫巢样本抵扣债务",
		},
		Triggers: []string{"残骸坐标泄露", "虫巢迁移", "航道封锁", "保险理赔", "幸存者求救"},
		Loopholes: []string{
			"先到达的人拥有打捞权，但必须在二十四小时内完成登记",
			"如果残骸中仍有幸存者，打捞权自动转为救援权",
			"军用残骸需要舰队许可，民用残骸只需要坐标备案",
			"打捞者可以放弃部分残骸换取优先补给",
			"虫巢样本可以抵扣打捞税，但必须由公会检验",
		},
		SignatureWins: []string{
			"把一艘敌舰登记为自己的残骸",
			"用救援权覆盖敌人的打捞权",
			"让虫巢替自己拆解战舰",
		},
		Misreads: []string{
			"以为主角要抢残骸，没想到主角要救幸存者",
			"以为打捞公会会保护主角，没想到公会才是最大的买家",
			"以为虫巢是敌人，没想到虫巢是免费的拆船工",
		},
		ConflictTypes: []string{"资源", "制度", "道德", "关系", "认知"},
		CounterMoves: []string{
			"主角把打捞权无偿公开，让所有船都能来抢",
			"主角故意留下幸存者，让打捞权自动失效",
			"主角把虫巢样本交给敌人，换取一次救援窗口",
		},
	},
	{
		ID:           "quota",
		IssuerSuffix: "调度中心",
		RoleSuffix:   "调度员",
		Assets: map[string]string{
			"科幻": "虫洞航道", "仙侠": "灵脉", "都市": "学区", "悬疑": "庭审", "通用": "通行",
		},
		StatedPurposes: []string{"把稀缺的通行机会分配给最需要的人", "控制流量以避免系统崩溃"},
		HiddenFunctions: []string{
			"拍卖生存机会",
			"把拥堵变成税收",
			"优先放行付费者",
			"用配额交换情报",
			"让难民成为配额的抵押品",
		},
		Triggers: []string{"虫洞闭合", "航道拥堵", "灵脉枯竭", "开学摇号", "紧急疏散"},
		Loopholes: []string{
			"配额可以转让，但转让后原持有人失去优先权",
			"紧急疏散时，配额按家庭而不是按个人计算",
			"调度员可以预留百分之五的机动配额，不需要说明用途",
			"连续三次放弃配额的人，会被降为最低优先级",
			"配额在过期前可以兑换成补给券，但兑换后不能再申请",
		},
		SignatureWins: []string{
			"让敌人拿走一个无法使用的配额",
			"用机动配额救下一整艘船",
			"把配额拍卖会变成一场公开审判",
		},
		Misreads: []string{
			"以为主角要抢配额，没想到主角要重新分配配额",
			"以为调度中心是中立机构，没想到调度中心靠拥堵赚钱",
			"以为配额是通行证，没想到配额是债务",
		},
		ConflictTypes: []string{"资源", "制度", "关系", "道德", "身份"},
		CounterMoves: []string{
			"主角把自己的配额让给敌人，让敌人成为所有难民的靶子",
			"主角公开机动配额的去向，让调度中心无法继续暗箱操作",
			"主角故意制造拥堵，让配额系统崩溃后重新分配",
		},
	},
	{
		ID:           "maintenance",
		IssuerSuffix: "维修厂",
		RoleSuffix:   "维修师",
		Assets: map[string]string{
			"科幻": "机甲与反应堆", "仙侠": "飞剑与护山大阵", "都市": "电梯与供暖系统", "悬疑": "监控与门锁", "通用": "关键设备",
		},
		StatedPurposes: []string{"记录设备状态并安排维修", "保证关键设备在任务中不会失效"},
		HiddenFunctions: []string{
			"把停机责任推给驾驶员",
			"用零件短缺控制舰队",
			"伪造战损骗取保险",
			"把虫族附着登记为正常磨损",
			"让维修工成为人质",
		},
		Triggers: []string{"战损检修", "反应堆过热", "虫族附着", "保修到期", "事故调查"},
		Loopholes: []string{
			"维修日志只记录更换过的零件，不记录拆下来的零件",
			"保修范围不包括虫族造成的腐蚀",
			"驾驶员签字确认设备完好后，维修厂不再承担责任",
			"紧急维修可以由值班员口头授权，但必须在七天内补交书面报告",
			"同一故障连续出现三次，才能申请更换整机",
		},
		SignatureWins: []string{
			"让敌人签下一份设备完好确认书",
			"用维修日志证明战舰在出击前就已经报废",
			"把虫族附着写进保修范围",
		},
		Misreads: []string{
			"以为主角在修机器，没想到主角在修一份记录",
			"以为维修厂会帮主角，没想到维修厂在等主角出事",
			"以为机甲是武器，没想到机甲是保险标的",
		},
		ConflictTypes: []string{"制度", "资源", "认知", "道德", "关系"},
		CounterMoves: []string{
			"主角拒绝签字，让整支舰队都无法出击",
			"主角把维修日志公开，让维修厂承认自己伪造战损",
			"主角故意让机甲故障，逼敌人接管维修责任",
		},
	},
	{
		ID:           "registry",
		IssuerSuffix: "登记处",
		RoleSuffix:   "登记员",
		Assets: map[string]string{
			"科幻": "舰籍与身份", "仙侠": "名讳与命盘", "都市": "户口与房产", "悬疑": "失踪人口", "通用": "身份",
		},
		StatedPurposes: []string{"确认身份并记录归属", "让每个人都能被系统识别"},
		HiddenFunctions: []string{
			"把身份变成需要续费的资格",
			"把无证者变成可雇佣资产",
			"用登记控制迁徙",
			"把死者的身份转卖",
			"让继承权成为抵押品",
		},
		Triggers: []string{"身份到期", "舰籍变更", "继承登记", "失踪复归", "大规模迁徙"},
		Loopholes: []string{
			"身份可以在不同辖区重复登记，只要编号不冲突",
			"没有身份的人可以由雇主担保，但担保期内不能离开雇主",
			"死者的身份在注销前可以继续使用",
			"登记员可以更正拼写错误，但不能更正出生日期",
			"临时身份在战争期间自动延长，不需要续费",
		},
		SignatureWins: []string{
			"让敌人成为一个不存在的人",
			"用死者的身份登上敌舰",
			"把整支舰队的身份绑定到一个人身上",
		},
		Misreads: []string{
			"以为主角在查身份，没想到主角在查谁没有身份",
			"以为登记处是保护者，没想到登记处是最大的雇主",
			"以为身份是权利，没想到身份是债务",
		},
		ConflictTypes: []string{"身份", "制度", "资源", "关系", "道德"},
		CounterMoves: []string{
			"主角注销自己的身份，让整艘船失去合法船员",
			"主角给所有无证者登记同一个编号，制造一场身份危机",
			"主角承认自己用了死者的身份，迫使登记处公开全部记录",
		},
	},
	{
		ID:           "evidence",
		IssuerSuffix: "法院",
		RoleSuffix:   "证物管理员",
		Assets: map[string]string{
			"科幻": "黑匣子与战斗记录", "仙侠": "剑意残影", "都市": "监控录像", "悬疑": "凶案证物", "通用": "证据",
		},
		StatedPurposes: []string{"保全证据并保证它在审判前不被污染", "让事实可以被法庭确认"},
		HiddenFunctions: []string{
			"决定什么能被看见",
			"把失踪定义为无证",
			"用封存拖延救援",
			"让证物成为谈判筹码",
			"把目击者变成证据",
		},
		Triggers: []string{"开庭", "证物失踪", "黑匣子回收", "目击者翻供", "封存到期"},
		Loopholes: []string{
			"证物封存后，原件只能由法官和证物管理员同时在场时开启",
			"副本可以作为证据，但副本的真实性由提交者负责",
			"证物在运输途中丢失，责任由承运人承担",
			"涉及国家安全的证物可以不公开审理",
			"目击者的记忆可以被认定为证物，但必须有两次独立陈述",
		},
		SignatureWins: []string{
			"让敌人的证据在法庭上证明敌人有罪",
			"用一份被封存的证物换取救援",
			"把整艘战舰变成一件证物",
		},
		Misreads: []string{
			"以为主角在找证据，没想到主角在藏证据",
			"以为法院会主持公道，没想到法院在等证据消失",
			"以为证物是死的，没想到证物会改变所有人的身份",
		},
		ConflictTypes: []string{"认知", "制度", "道德", "关系", "身份"},
		CounterMoves: []string{
			"主角销毁证物，让所有人只能依赖活人的证词",
			"主角把证物公开，让法庭无法继续拖延",
			"主角承认自己篡改过证物，迫使法院重新调查",
		},
	},
	{
		ID:           "sanctuary",
		IssuerSuffix: "收容署",
		RoleSuffix:   "收容官",
		Assets: map[string]string{
			"科幻": "难民船", "仙侠": "山门庇护", "都市": "临时安置点", "悬疑": "证人保护", "通用": "避难者",
		},
		StatedPurposes: []string{"为无家可归者提供临时保护", "把危险人口转移到安全区域"},
		HiddenFunctions: []string{
			"把庇护变成劳动力招募",
			"用床位分配控制人口",
			"用身份换取情报",
			"把儿童登记为可收养资产",
			"用遣返威胁维持秩序",
		},
		Triggers: []string{"难民船靠港", "虫潮逼近", "收容名额满员", "身份审查", "遣返令"},
		Loopholes: []string{
			"收容登记只记录进入时间，不记录离开时间",
			"没有身份的人可以由收容官指定一个临时编号",
			"收容期内的人不能被雇佣，但可以自愿劳动",
			"遣返令需要原籍地接收，而原籍地可能已经不存在",
			"儿童可以被登记为孤儿，但需要两名证人",
		},
		SignatureWins: []string{
			"让敌人的追兵成为收容对象",
			"用收容登记证明一支舰队已经不存在",
			"把整艘难民船登记为一个需要保护的人",
		},
		Misreads: []string{
			"以为主角在救人，没想到主角在登记人",
			"以为收容署是避风港，没想到收容署是劳动力市场",
			"以为庇护是免费的，没想到庇护要用未来偿还",
		},
		ConflictTypes: []string{"道德", "制度", "身份", "关系", "资源"},
		CounterMoves: []string{
			"主角给所有难民同一个临时编号，让收容署无法区分他们",
			"主角拒绝遣返，让收容署承担全部安置成本",
			"主角把收容名单公开，让敌人无法混入难民船",
		},
	},
	{
		ID:           "ration",
		IssuerSuffix: "后勤处",
		RoleSuffix:   "配给员",
		Assets: map[string]string{
			"科幻": "氧气与冷却剂", "仙侠": "丹药与灵石", "都市": "药品与供暖", "悬疑": "封口费", "通用": "生存物资",
		},
		StatedPurposes: []string{"按需分配稀缺物资", "保证关键人员优先获得补给"},
		HiddenFunctions: []string{
			"用热量控制行为",
			"把药品变成货币",
			"让工人以未来劳动预付",
			"把虫族威胁等级与配给挂钩",
			"用断供惩罚罢工",
		},
		Triggers: []string{"补给船迟到", "虫潮封锁", "配给站遇袭", "罢工", "寒冬"},
		Loopholes: []string{
			"配给券可以转让，但转让记录不进入主账本",
			"紧急配给可以由现场指挥官口头批准，事后补签",
			"儿童和老人有双份额度，但必须由户主统一领取",
			"连续三天未领取的配给券会自动作废",
			"参与危险任务的人可以预支下个月的配给",
		},
		SignatureWins: []string{
			"让敌人领取一份被做了标记的配给",
			"用配给券证明敌人囤积物资",
			"把整艘船的配给券集中到一个人手里，逼后勤处谈判",
		},
		Misreads: []string{
			"以为主角在抢物资，没想到主角在重新分配物资",
			"以为配给员是小人物，没想到配给员掌握全船的生死",
			"以为配给券是福利，没想到配给券是债务",
		},
		ConflictTypes: []string{"资源", "制度", "道德", "关系", "身份"},
		CounterMoves: []string{
			"主角把自己的配给券全部烧掉，让后勤处无法证明谁在挨饿",
			"主角公开配给账本，让所有人看到谁在囤积",
			"主角故意制造断供，逼全船一起罢工",
		},
	},
}

var engineIssuerPrefixes = map[string][]string{
	"科幻": {"深空", "舰队", "航道", "殖民署", "虫巢"},
	"仙侠": {"宗门", "长老会", "山门", "灵脉"},
	"都市": {"市", "社区", "城投", "医院"},
	"悬疑": {"市", "法院", "警局", "殡仪馆"},
	"通用": {""},
}

var engineRolePrefixes = map[string][]string{
	"科幻": {"机甲", "战舰", "航道", "虫巢"},
	"仙侠": {"外门", "灵脉", "剑冢", "丹房"},
	"都市": {"社区", "医院", "物业", "银行"},
	"悬疑": {"法医", "档案", "证物", "失踪人口"},
	"通用": {""},
}

var engineHolders = map[string][]string{
	"科幻": {"舰长", "机甲驾驶员", "打捞员", "领航员", "反应堆工程师", "幸存者", "虫族研究员"},
	"仙侠": {"宗主", "外门弟子", "守山人", "丹师", "剑奴", "债主"},
	"都市": {"业主", "租客", "保安", "护士", "快递员", "银行柜员"},
	"悬疑": {"法医", "证人", "失踪者家属", "保安", "档案员"},
	"通用": {"申请人", "持有人", "继承人", "见证人", "执行人"},
}

var engineWorldAnchors = map[string][]string{
	"科幻": {"虫洞边缘", "残骸带", "母舰机库", "轨道电梯", "潮汐阴影区"},
	"仙侠": {"山门", "灵脉深处", "剑冢", "藏经阁", "渡口"},
	"都市": {"第三层", "地下车库", "旧码头", "医院走廊", "婚姻登记处"},
	"悬疑": {"太平间", "档案室", "旧码头", "钟楼", "法院走廊"},
	"通用": {"城门", "渡口", "钟楼", "档案馆"},
}

var engineCosts = []string{
	"一段记忆", "一年寿命", "一个名字", "一份身份", "一次睡眠", "一段关系",
	"一只眼睛", "一颗牙", "一段声音", "一份继承权", "一场婚礼", "一次死亡",
	"未来十年的劳动", "全部信用记录", "对亲人的记忆", "离开辖区的权利",
}

var engineDefaultExpectationsByFamily = map[string][]string{
	"insurance": {
		"主角揭穿骗局，让所有被拒赔的人得到补偿",
		"主角迫使保险公司公开风险模型",
		"主角发现事故本身被设计成理赔业务",
	},
	"license": {
		"主角偷到许可证并绕过管理局",
		"主角靠一张临时许可证完成不可能的通行",
		"主角发现许可证本身就是定位工具",
	},
	"contract": {
		"主角靠武力撕毁合同并赢得自由",
		"主角发现工人正在被合同转换成设备",
		"主角利用替班条款让所有工人获得索赔权",
	},
	"inheritance": {
		"主角继承隐藏遗产并成为新继承人",
		"主角发现遗嘱把债务和身份一起传给了自己",
		"主角被迫替一个从未见过的人主持继承听证",
	},
	"death_record": {
		"主角找到凶手并让死者安息",
		"主角发现死者身份被机构用来清理活人",
		"主角追查一名仍活在档案里的死者",
	},
	"salvage": {
		"主角打捞到最值钱的宝物并成为英雄",
		"主角在残骸里找到能推翻机构的证据",
		"主角必须先证明打捞物不是自己偷走的",
	},
	"quota": {
		"主角击败排队者成为配额拥有者",
		"主角发现配额系统靠制造短缺维持利润",
		"主角把自己的名额让给陌生人，逼系统重新分配",
	},
	"maintenance": {
		"主角关闭旧系统并让所有人重获自由",
		"主角发现维修延误本身就是一门生意",
		"主角必须在系统停摆前决定谁先被修复",
	},
	"registry": {
		"主角恢复真实身份并推翻档案署",
		"主角发现有人正在批量制造合法身份",
		"主角必须证明自己不是系统里已经死亡的那个人",
	},
	"evidence": {
		"主角把证据交给官方，法庭主持正义",
		"主角发现证据封存制度正在帮助凶手拖延时间",
		"主角必须让一份无法公开的证据被所有人看见",
	},
	"sanctuary": {
		"主角抢到唯一庇护名额",
		"主角发现庇护名额只按身份和关系分配",
		"主角必须决定让谁进入只剩一个位置的安全区",
	},
	"ration": {
		"主角夺取最多配给并成为幸存者领袖",
		"主角发现配给点故意保持长期短缺",
		"主角必须让所有人共同签收一份配给账单",
	},
}

// GenerateEngines draws deterministic institution/artifact engine cards.
func GenerateEngines(req EngineRequest) (EngineResult, error) {
	genre := NormalizeGenre(req.Genre)
	count := req.Count
	warnings := make([]string, 0)
	if count <= 0 {
		count = 5
	}
	if count > maxEngineCount {
		warnings = append(warnings, fmt.Sprintf("count capped from %d to %d", count, maxEngineCount))
		count = maxEngineCount
	}
	seed := req.Seed
	if seed == 0 {
		seed = randomSeed()
	}
	rng := rand.New(rand.NewSource(seed))
	usedFamilies := make(map[string]int)
	usedPairs := make(map[string]bool)
	engines := make([]Engine, 0, count)
	attempts := 0
	maxAttempts := count*100 + 100

	for len(engines) < count && attempts < maxAttempts {
		attempts++
		def := chooseEngineFamily(rng, usedFamilies)
		engine := buildEngine(rng, seed, genre, def)
		pairKey := def.ID + "|" + engine.Artifact + "|" + engine.HiddenFunction
		if usedPairs[pairKey] {
			continue
		}
		usedPairs[pairKey] = true
		usedFamilies[def.ID]++
		engine.ID = fmt.Sprintf("engine-%d-%02d", seed, len(engines)+1)
		engines = append(engines, engine)
	}
	if len(engines) < count {
		warnings = append(warnings, fmt.Sprintf("generated %d unique engine(s), requested %d", len(engines), count))
	}
	return EngineResult{Seed: seed, Genre: genre, Engines: engines, Warnings: warnings}, nil
}

func chooseEngineFamily(rng *rand.Rand, used map[string]int) engineFamilyDef {
	minUses := -1
	for _, def := range engineFamilies {
		uses := used[def.ID]
		if minUses < 0 || uses < minUses {
			minUses = uses
		}
	}
	fresh := make([]engineFamilyDef, 0, len(engineFamilies))
	for _, def := range engineFamilies {
		if used[def.ID] == minUses {
			fresh = append(fresh, def)
		}
	}
	return fresh[rng.Intn(len(fresh))]
}

func buildEngine(rng *rand.Rand, seed int64, genre string, def engineFamilyDef) Engine {
	asset := def.Assets[genre]
	if asset == "" {
		asset = def.Assets["通用"]
	}
	issuerPrefix := pick(rng, engineIssuerPrefixes[genre])
	rolePrefix := pick(rng, engineRolePrefixes[genre])
	artifact := asset + def.IDToSuffix()
	issuer := issuerPrefix + def.IssuerSuffix
	holder := pick(rng, engineHolders[genre])
	world := pick(rng, engineWorldAnchors[genre])
	stated := pick(rng, def.StatedPurposes)
	hidden := pick(rng, def.HiddenFunctions)
	trigger := pick(rng, def.Triggers)
	loophole := pick(rng, def.Loopholes)
	cost := pick(rng, engineCosts)
	role := rolePrefix + def.RoleSuffix
	defaultExpectations := engineDefaultExpectationsByFamily[def.ID]
	if len(defaultExpectations) == 0 {
		defaultExpectations = []string{"主角沿用现有规则解决眼前问题"}
	}
	defaultExpectation := pick(rng, defaultExpectations)
	counterExpectation := pick(rng, def.CounterMoves)
	win := pick(rng, def.SignatureWins)
	misread := pick(rng, def.Misreads)
	conflicts := pickDistinct(rng, def.ConflictTypes, 4)

	engine := Engine{
		Seed:               seed,
		Genre:              genre,
		Family:             def.ID,
		Artifact:           artifact,
		Issuer:             issuer,
		Holder:             holder,
		WorldAnchor:        world,
		StatedPurpose:      stated,
		HiddenFunction:     hidden,
		Trigger:            trigger,
		Loophole:           loophole,
		Cost:               cost,
		ProtagonistRole:    role,
		PublicSecret:       fmt.Sprintf("读者知道%s，主角暂时不知道；持有%s的%s以为主角只是来办事的。", hidden, artifact, holder),
		DefaultExpectation: defaultExpectation,
		CounterExpectation: counterExpectation,
		ConflictTypes:      conflicts,
		SignatureWin:       win,
		OpponentMisread:    misread,
		Tags:               []string{genre, def.ID, artifact, hidden, role},
	}
	engine.PremiseScaffold = fmt.Sprintf(
		"在%s，%s由%s签发。它表面用于%s，实际用于%s。主角是%s，必须在%s之前处理一份%s。",
		world, artifact, issuer, stated, hidden, role, trigger, artifact,
	)
	engine.RuleSeeds = []string{
		fmt.Sprintf("%s只在%s之后生效；没有%s，任何主张都不被承认。", artifact, trigger, artifact),
		fmt.Sprintf("%s。绕过它需要支付%s，而且必须留下可追查的记录。", loophole, cost),
		fmt.Sprintf("%s靠“%s”获利，所以它不会主动废除%s；受害者之所以不推翻它，是因为他们也在靠它换取下一顿饭。", issuer, hidden, artifact),
	}
	engine.StorylineSeeds = []string{
		fmt.Sprintf("资源线：%s与%s争夺%s的解释权；冲突类型是%s。", holder, issuer, artifact, conflicts[0]),
		fmt.Sprintf("认知线：主角发现“%s”并不是事故，而是%s的日常业务；冲突类型是%s。", hidden, issuer, conflicts[1]),
		fmt.Sprintf("制度线：主角必须在维持%s和揭露“%s”之间选择；冲突类型是%s。", artifact, hidden, conflicts[2]),
	}
	engine.CastSeeds = []string{
		fmt.Sprintf("主角：%s，唯一能同时接触%s和%s的人。", role, artifact, issuer),
		fmt.Sprintf("对手：%s的代表，真心相信%s让更多人活下来。", issuer, hidden),
		fmt.Sprintf("盟友：%s，被%s伤害过，但仍在依赖它。", holder, artifact),
		fmt.Sprintf("变量：一个知道%s却不愿签字的人。", loophole),
	}
	engine.VolumeEscalation = []string{
		fmt.Sprintf("第一卷：一个人被%s卡住，主角用%s解决眼前问题。", artifact, loophole),
		fmt.Sprintf("第二卷：整支舰队/整个机构开始利用%s，主角发现%s本身在制造稀缺。", artifact, issuer),
		fmt.Sprintf("第三卷：主角必须决定废除%s，还是成为新的%s；两种选择都会让一部分人立刻死去。", artifact, issuer),
	}
	engine.RequiredQuestions = []string{
		"这份凭证在普通人一天中的哪个时刻出现？",
		"谁签发、谁执行、谁出预算、谁从中获利？",
		"没有这份凭证的人如何活下去？",
		"违反或绕过它的具体代价是什么？",
		"它的漏洞为什么没有被立刻堵上？",
		"这个引擎能产生哪三种不同性质的卷级冲突？",
		"哪个反转是“表面用途/真实功能”的必然后果？",
	}
	engine.RejectIf = []string{
		"需要第二条独立魔法规则才能解释。",
		"签发机构没有预算、执行者或受益者。",
		"普通人可以零成本无视这份凭证。",
		"卷级冲突需要新增规则才能成立。",
		"反转没有前置线索，只是临时追加。",
		"把制度解释成梦、幻觉、传说或纯隐喻。",
	}
	return engine
}

func (d engineFamilyDef) IDToSuffix() string {
	switch d.ID {
	case "insurance":
		return "保险单"
	case "license":
		return "许可证"
	case "contract":
		return "雇佣契"
	case "inheritance":
		return "继承状"
	case "death_record":
		return "死亡确认书"
	case "salvage":
		return "打捞权证"
	case "quota":
		return "通行配额"
	case "maintenance":
		return "维修日志"
	case "registry":
		return "登记证"
	case "evidence":
		return "证物封存单"
	case "sanctuary":
		return "收容登记"
	case "ration":
		return "配给券"
	default:
		return "凭证"
	}
}

func pickDistinct(rng *rand.Rand, values []string, count int) []string {
	if count <= 0 || len(values) == 0 {
		return nil
	}
	shuffled := append([]string(nil), values...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	if count > len(shuffled) {
		count = len(shuffled)
	}
	return append([]string(nil), shuffled[:count]...)
}

// ValidateEngine checks the deterministic part of the engine contract. It does
// not judge prose quality; it only rejects cards whose causal spine is missing.
func ValidateEngine(engine Engine) []string {
	var issues []string
	required := map[string]string{
		"artifact":            engine.Artifact,
		"issuer":              engine.Issuer,
		"holder":              engine.Holder,
		"world_anchor":        engine.WorldAnchor,
		"stated_purpose":      engine.StatedPurpose,
		"hidden_function":     engine.HiddenFunction,
		"trigger":             engine.Trigger,
		"loophole":            engine.Loophole,
		"cost":                engine.Cost,
		"protagonist_role":    engine.ProtagonistRole,
		"counter_expectation": engine.CounterExpectation,
		"signature_win":       engine.SignatureWin,
		"opponent_misread":    engine.OpponentMisread,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, "missing "+name)
		}
	}
	if strings.TrimSpace(engine.HiddenFunction) == strings.TrimSpace(engine.StatedPurpose) {
		issues = append(issues, "hidden_function must differ from stated_purpose")
	}
	if len(uniqueStrings(engine.ConflictTypes)) < 3 {
		issues = append(issues, "need at least three distinct conflict types")
	}
	if strings.TrimSpace(engine.CounterExpectation) == strings.TrimSpace(engine.DefaultExpectation) {
		issues = append(issues, "counter_expectation must differ from default_expectation")
	}
	for _, value := range []string{engine.Artifact, engine.Issuer, engine.Holder, engine.ProtagonistRole} {
		if strings.Contains(value, "某物") || strings.Contains(value, "%s") || strings.Contains(value, "{") {
			issues = append(issues, fmt.Sprintf("unfilled slot in %q", value))
		}
	}
	return issues
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
