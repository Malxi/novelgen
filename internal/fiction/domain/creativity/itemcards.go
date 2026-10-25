package creativity

import (
	"fmt"
	"math/rand"
	"strings"
)

const maxItemCardCount = 20

var itemCategories = []string{"resource", "equipment", "consumable", "prop", "credential", "commodity", "vehicle"}

// ItemCategories returns the canonical item categories in display order.
func ItemCategories() []string {
	return append([]string(nil), itemCategories...)
}

var itemCategoryAliases = map[string]string{
	"":            "",
	"all":         "",
	"any":         "",
	"resource":    "resource",
	"resources":   "resource",
	"资源":          "resource",
	"commodity":   "commodity",
	"goods":       "commodity",
	"trade_good":  "commodity",
	"商品":          "commodity",
	"贸易品":         "commodity",
	"equipment":   "equipment",
	"gear":        "equipment",
	"装备":          "equipment",
	"武器":          "equipment",
	"consumable":  "consumable",
	"consumables": "consumable",
	"消耗品":         "consumable",
	"道具":          "prop",
	"prop":        "prop",
	"story_item":  "prop",
	"vehicle":     "vehicle",
	"载具":          "vehicle",
	"credential":  "credential",
	"permit":      "credential",
	"凭证":          "credential",
}

// ItemRequest controls a role-aware item draw. When Engine is supplied, every
// item is structurally tied to that engine rather than being decorative loot.
type ItemRequest struct {
	Genre       string
	Seed        int64
	Count       int
	Source      string
	Category    string
	PackID      string
	ReferenceID string
	Engine      *Engine
}

// ItemResult is a deterministic set of items.
type ItemResult struct {
	Seed     int64      `json:"seed"`
	Genre    string     `json:"genre"`
	Items    []ItemCard `json:"items"`
	Warnings []string   `json:"warnings,omitempty"`
}

// ItemCard covers both material and institutional objects. Category controls
// how the object enters and changes a scene.
type ItemCard struct {
	ID                  string   `json:"id"`
	PackID              string   `json:"pack_id,omitempty"`
	ReferenceID         string   `json:"reference_id,omitempty"`
	Name                string   `json:"name"`
	BaseObject          string   `json:"base_object"`
	Category            string   `json:"category"`
	PhysicalForm        string   `json:"physical_form"`
	Function            string   `json:"function"`
	Acquisition         string   `json:"acquisition"`
	UseCost             string   `json:"use_cost"`
	Limit               string   `json:"limit"`
	Counterplay         string   `json:"counterplay"`
	FailureMode         string   `json:"failure_mode"`
	EngineLink          string   `json:"engine_link,omitempty"`
	ReferenceConstraint string   `json:"reference_constraint"`
	NarrativeUses       []string `json:"narrative_uses"`
	CompatibilityTags   []string `json:"compatibility_tags"`
}

type itemPatternSet struct {
	Nouns        []string
	Forms        []string
	Functions    []string
	UseCosts     []string
	Limits       []string
	Counterplays []string
	Failures     []string
	Narrative    []string
}

var itemPatterns = map[string]itemPatternSet{
	"resource": {
		Nouns:        []string{"认证配额", "净化滤芯", "冷却剂", "维修工时", "导航胶片", "备用能源单元"},
		Forms:        []string{"带编号的密封罐", "可转让的配额芯片", "带监控封条的工时券", "装在防静电匣里的胶片"},
		Functions:    []string{"维持系统运转并让一次高风险行动成为可能", "把稀缺服务转成可以交换的份额", "为设备恢复一个有限工作时间窗"},
		UseCosts:     []string{"每次提取都会记录使用者和剩余额度", "补货需要到指定节点重新认证", "跨越辖区会让配额折价"},
		Limits:       []string{"总量有限，补给需要回到指定节点", "按批次计量，损坏或转手会留下记录"},
		Counterplays: []string{"伪造提取记录", "拆下监控封条", "在结算前把额度转给第三方"},
		Failures:     []string{"配额被冻结后，所有依赖它的战术会同时失效", "监控封条损坏或批次过期时，整批资源会被视为赃物"},
		Narrative:    []string{"逼迫角色在眼前收益和家庭配额之间选择", "制造黑市交易和结算争议", "让一次救援需要先支付可见成本"},
	},
	"commodity": {
		Nouns:        []string{"合金坯", "维修组件", "药用藻", "燃料芯", "航海图册", "低温电池"},
		Forms:        []string{"压成标准块的复合材料", "装在透明管里的活体样本", "带旧编号的机械组件"},
		Functions:    []string{"修复设备或换取其他资源", "证明某条交易路线仍然存在", "让一艘失去动力的船重新启动"},
		UseCosts:     []string{"成交会暴露买卖双方的位置", "运输需要消耗更多认证氧气", "价格随灾害周期剧烈波动"},
		Limits:       []string{"储存条件、数量或运输空间至少有一项受限", "离开合适环境太久会降低价值"},
		Counterplays: []string{"掺假", "重新包装", "用官方订单掩盖黑市交易"},
		Failures:     []string{"冷链断裂、包装破损或市场突然拒收会让货物价值归零", "数量太多会拖慢撤离，数量太少又不值得冒险"},
		Narrative:    []string{"制造一场物资交换中的信任问题", "让不同阵营对同一件商品给出不同估值", "作为改变航线的诱饵"},
	},
	"equipment": {
		Nouns:        []string{"频谱探针", "切割臂", "牵引索", "折叠护盾", "维修外骨骼", "静默扫描器"},
		Forms:        []string{"可折叠的机械臂", "带旧式接口的便携设备", "需要两人校准的穿戴装备"},
		Functions:    []string{"让使用者绕过一种环境限制，但会暴露新的弱点", "把声音、热量或电磁信号转换成可读信息"},
		UseCosts:     []string{"运行会消耗能源并产生可探测信号", "每次使用都要更换一个易损部件"},
		Limits:       []string{"只能在特定介质、距离或角度内工作", "能源和易损件限制了连续使用次数"},
		Counterplays: []string{"切断电源", "制造更大的假信号", "用低技术手段绕过传感器"},
		Failures:     []string{"过载后设备会进入冷却盲区，短时间内无法重新启动", "易损件断裂时会把操作者固定在危险位置"},
		Narrative:    []string{"用同一件装备制造一次漂亮反杀", "让敌人误判其使用次数或工作范围", "迫使角色冒着暴露风险完成救援"},
	},
	"consumable": {
		Nouns:        []string{"密封泡沫", "应急氧片", "抗辐射片", "神经阻断剂", "冷却凝胶", "一次性信标"},
		Forms:        []string{"单次封装的透明管", "带破封记号的胶囊", "一次性投放的小型信标"},
		Functions:    []string{"在短时间内抵消一种伤害或暴露状态", "为角色争取一段可计算的安全窗口"},
		UseCosts:     []string{"使用后立刻消耗，无法重复利用", "会留下代谢、热量或登录记录"},
		Limits:       []string{"作用时间有限，不能替代长期恢复", "同一目标重复使用收益递减"},
		Counterplays: []string{"延迟使用", "伪造消耗记录", "让对手误以为已经用完"},
		Failures:     []string{"重复使用会产生抗性、过载或不可逆的组织损伤", "破封后必须立即使用，否则整件物品失效"},
		Narrative:    []string{"在最后一刻制造一次有限救援", "把资源交给谁成为道德选择", "用完后的空容器仍可作为证据"},
	},
	"prop": {
		Nouns:        []string{"维护印", "授权卡", "旧信标", "黑匣钥匙", "航行日志", "停靠牌"},
		Forms:        []string{"带磨损编号的金属牌", "无法复制的旧式芯片", "写着两个不同日期的纸质记录"},
		Functions:    []string{"证明某件事发生过，并改变谁有资格发言", "让一个被封锁的地点或人物重新进入故事"},
		UseCosts:     []string{"拿出来就会暴露持有人或来源", "只能使用一次，使用后证据可能被没收"},
		Limits:       []string{"必须配合见证人、渠道或现场语境", "原件不能长期暴露或反复搬运"},
		Counterplays: []string{"抢走并销毁", "质疑签名来源", "抢先给出另一份看似完整的记录"},
		Failures:     []string{"缺少见证人或合法渠道时，证据会被判定为无效", "出示后可能被当场没收，原件也容易在争夺中损坏"},
		Narrative:    []string{"把私人秘密转成公共证据", "迫使角色在名誉和安全之间选择", "让敌人自己承认道具的真实来源"},
	},
	"vehicle": {
		Nouns:        []string{"拖曳无人机", "登陆舱", "地表艇", "轨道滑橇", "小型穿梭器"},
		Forms:        []string{"带可拆卸装甲的短程载具", "需要母舰供能的滑橇", "只有两个座位的救援舱"},
		Functions:    []string{"把角色或物资送进普通载具无法进入的区域", "承担一次高风险撤离或追逐"},
		UseCosts:     []string{"每次启动都消耗大量燃料或氧气", "载具信号会被救援网络和敌人同时看到"},
		Limits:       []string{"载重、续航或入口尺寸至少有一项严格限制", "必须依赖母舰、燃料或导航信号"},
		Counterplays: []string{"锁死挂载点", "切断导航信号", "在狭窄空间用更小的单位追击"},
		Failures:     []string{"燃料、载重或入口尺寸不符时会直接失去撤离能力", "导航信号被切断后只能在本地盲行"},
		Narrative:    []string{"制造一次必须减重才能脱困的选择", "让两个人争夺同一个座位", "把载具残骸变成下一场证据"},
	},
	"credential": {
		Nouns:        []string{"打捞权证", "停靠许可", "维护签", "身份副本", "紧急通行令", "结算凭证"},
		Forms:        []string{"带防伪纹路的硬质卡", "只在指定节点有效的临时许可", "需要两个签名才能生效的电子凭证"},
		Functions:    []string{"改变谁有权进入、继承、打捞或发言", "把一件行为从违规变成可结算的合法行动"},
		UseCosts:     []string{"每次核验都会记录持有人和位置", "过期、借用或转手都有不同法律后果"},
		Limits:       []string{"权限只覆盖特定辖区或特定事件", "不能替代物证或现场见证"},
		Counterplays: []string{"吊销", "冒用", "利用跨区规则延迟核验"},
		Failures:     []string{"跨辖区、过期或签名缺失时，凭证会从保护变成罪证", "核验失败会立即锁定持有人并暴露位置"},
		Narrative:    []string{"让敌人不得不为对手开门", "制造一场关于继承顺位的公开争夺", "把合规行为变成更大的道德问题"},
	},
}

var referenceItemPrefixes = map[string][]string{
	"stellaris":    {"观测级", "自动化", "外交配给", "标准化", "边疆制式"},
	"warhammer40k": {"战地制式", "旧制", "审批版", "重装", "封存"},
	"alien":        {"工业级", "静默", "应急", "船用", "故障冗余"},
	"starwars":     {"旧盟约", "能量", "导航", "远征", "手工业"},
	"nomanssky":    {"便携", "程序化", "勘探", "折叠", "异星适配"},
	"":             {"旧制", "便携", "改装", "封存", "应急"},
}

var itemPhysicalForms = map[string][]string{
	"认证配额":   {"带编号的配额芯片", "封在透明匣里的额度卡"},
	"净化滤芯":   {"带一次性锁扣的圆柱滤芯", "嵌着状态灯的标准滤罐"},
	"冷却剂":    {"带压力表的密封冷却罐", "可插入反应堆回路的标准瓶"},
	"维修工时":   {"带工号和余量的数字工时券", "可在维护节点核验的纸质工票"},
	"导航胶片":   {"卷在防磁匣中的航线胶片", "边缘带校验孔的导航带"},
	"备用能源单元": {"可热插拔的电池方块", "带机械锁的应急电源匣"},
	"合金坯":    {"压成标准尺寸的合金锭", "贴在托盘上的复合金属块"},
	"维修组件":   {"成套封装的机械零件", "装在耐压盒里的替换模块"},
	"药用藻":    {"封在冷链管里的活体样本", "带生长记录的水培罐"},
	"燃料芯":    {"带防误插凸榫的燃料棒", "外层有磨损刻度的燃料匣"},
	"航海图册":   {"由多张航线片装订成的手册", "带磁扣的折叠航线册"},
	"低温电池":   {"带低温绝缘层的电池包", "可嵌入设备侧槽的冷电池"},
	"频谱探针":   {"可伸出的细长探头", "装在三脚支架上的频谱天线"},
	"切割臂":    {"带铰接关节的机械切割臂", "可换刀头的短程切割组件"},
	"牵引索":    {"带磁锚的盘绕钢索", "装在肩部绞盘里的牵引线"},
	"折叠护盾":   {"可展开的层压护盾板", "固定在前臂上的折叠防御片"},
	"维修外骨骼":  {"带承重支架的可穿戴外骨骼", "需要绑带固定的半身维修架"},
	"静默扫描器":  {"贴在头盔侧面的被动传感器", "无主动发射器的黑盒监听器"},
	"密封泡沫":   {"带破封拉环的压力罐", "单手可按的一次性喷嘴"},
	"应急氧片":   {"口腔溶解的密封片剂", "带使用刻度的微型氧包"},
	"抗辐射片":   {"铅色包装里的片剂", "可贴在皮肤上的缓释贴片"},
	"神经阻断剂":  {"带一次性针头的注射管", "自动推注的无针注射器"},
	"冷却凝胶":   {"可挤压的隔温软管", "贴在装甲内侧的凝胶袋"},
	"一次性信标":  {"只有单次启动键的微型信标", "可粘贴在船壳上的脉冲贴片"},
	"维护印":    {"带磨损编号的金属维护牌", "需要机械压印的授权徽片"},
	"授权卡":    {"带旧式触点的权限卡", "只在特定端口插入的硬质钥匙卡"},
	"旧信标":    {"外壳锈蚀的短程信标", "仍在发送循环编码的旧式发射器"},
	"黑匣钥匙":   {"带非标准接口的解密钥匙", "只能插入一次数据的读取针"},
	"航行日志":   {"被水汽浸过的纸质航行册", "封在损坏壳体中的航行记录"},
	"停靠牌":    {"刻有泊位和日期的金属牌", "带一次性封条的停靠凭证"},
	"拖曳无人机":  {"带可收放牵引钩的短翼无人机", "只有单侧推进器的小型拖船"},
	"登陆舱":    {"只能容纳两人的登陆舱", "外壳可抛弃的短程落体舱"},
	"地表艇":    {"带履带和密封舱的地表载具", "有四组独立悬挂的勘探艇"},
	"轨道滑橇":   {"带磁轨夹的滑橇", "靠母舰供能的轨道穿梭架"},
	"小型穿梭器":  {"带折叠机翼的小型穿梭器", "只有狭窄货槽的短程艇"},
	"打捞权证":   {"带防伪纹路的硬质卡", "需要现场核验的电子权证"},
	"停靠许可":   {"只在指定泊位有效的许可卡", "贴着兼容港编号的通行纸"},
	"维护签":    {"带双签名栏的维护签", "由维护员压印的纸质签条"},
	"身份副本":   {"封在透明片里的身份副本", "带有旧照片和编号的数据片"},
	"紧急通行令":  {"带红边的单次通行令", "只在灾害状态下生效的临时令牌"},
	"结算凭证":   {"带分段签名栏的结算单", "可拆成两半核验的结算卡"},
}

var itemSpecificFunctions = map[string][]string{
	"频谱探针":  {"探测不可见信号源，但会暴露自身扫描频率", "在复杂噪声中分离一个特定信号"},
	"切割臂":   {"在有限角度内切开舱壁或锁具，但会制造热量和噪声", "切断卡死的机械结构而不引爆外部压力"},
	"牵引索":   {"固定或拖曳目标，但受力过大时会成为对手的导火索", "在失重环境里建立一条可控的回收线"},
	"折叠护盾":  {"分散一次定向冲击，但展开后无法快速转向", "为正面目标争取一次短时防护"},
	"维修外骨骼": {"让穿戴者搬动超重部件，但穿脱需要时间", "在狭小空间里维持超负荷维修"},
	"静默扫描器": {"被动记录环境信号，但无法主动定位远处目标", "在不触发追踪器的情况下监听舱室"},
	"拖曳无人机": {"拖走小型载具或引出敌方传感器", "在母舰无法进入的区域执行回收"},
	"登陆舱":   {"把人员和物资投入高重力或厚大气区域", "在母舰无法靠近时完成一次性登陆"},
	"地表艇":   {"长时间穿越不稳定地表并携带小型货物", "在风暴中维持移动和通信"},
	"轨道滑橇":  {"沿固定轨道高速转移物资", "在母舰供能范围内完成短程追逐"},
	"小型穿梭器": {"在两艘船或站点之间完成短程接驳", "在狭窄泊位中装载少量关键人员"},
	"维护印":   {"证明一项维护指令曾被签发", "让非授权改动暂时进入合法流程"},
	"授权卡":   {"打开一扇与自身权限匹配的门", "让持有人被系统误认成某个旧职员"},
	"旧信标":   {"证明一条旧航线仍然存在", "把一段历史求救信号重新投送到公共频道"},
	"黑匣钥匙":  {"读取一段被加密的舰桥记录", "在不破坏原件的情况下复制证据"},
	"航行日志":  {"证明船只曾到过某地，或证明记录被人改过", "把私人记忆变成可公开核验的时间线"},
	"停靠牌":   {"让一艘船获得一次指定泊位", "把一艘敌对船标记为待检修资产"},
}

// GenerateItemCards draws coherent resources, tools, consumables, props, and
// vehicles. Category filters accept English canonical values and Chinese aliases.
func GenerateItemCards(req ItemRequest) (ItemResult, error) {
	genre := NormalizeGenre(req.Genre)
	category, err := NormalizeItemCategory(req.Category)
	if err != nil {
		return ItemResult{}, err
	}
	reference, err := itemReference(req, genre)
	if err != nil {
		return ItemResult{}, err
	}
	count := req.Count
	if count <= 0 {
		count = 6
	}
	if count > maxItemCardCount {
		count = maxItemCardCount
	}
	seed := req.Seed
	if seed == 0 {
		seed = randomSeed()
	}
	rngSeed := seed
	if req.PackID != "" {
		rngSeed = stableNameSeed(req.PackID + "|items")
	}
	rng := rand.New(rand.NewSource(rngSeed))
	categories := itemCategories
	if category != "" {
		categories = []string{category}
	}
	items := make([]ItemCard, 0, count)
	usedNames := make(map[string]bool)
	usedBaseObjects := make(map[string]map[string]bool)
	for _, itemCategory := range itemCategories {
		usedBaseObjects[itemCategory] = make(map[string]bool)
	}
	attempts := 0
	for len(items) < count && attempts < count*100 {
		attempts++
		categoryForCard := categories[len(items)%len(categories)]
		card := buildItemCard(rng, genre, reference, req, categoryForCard, len(items))
		usedInCategory := usedBaseObjects[categoryForCard]
		if usedInCategory[card.BaseObject] && len(usedInCategory) < len(itemPatterns[categoryForCard].Nouns) {
			continue
		}
		if usedNames[card.Name] {
			card.Name += "·" + fmt.Sprintf("%02d", len(items)+1)
		}
		usedNames[card.Name] = true
		usedInCategory[card.BaseObject] = true
		items = append(items, card)
	}
	return ItemResult{Seed: seed, Genre: genre, Items: items}, nil
}

// NormalizeItemCategory normalizes category names and common Chinese aliases.
func NormalizeItemCategory(value string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(value))
	category, ok := itemCategoryAliases[key]
	if !ok {
		return "", fmt.Errorf("unsupported item category %q; valid categories: resource, commodity, equipment, consumable, prop, vehicle, credential", value)
	}
	return category, nil
}

func itemReference(req ItemRequest, genre string) (ReferenceCard, error) {
	if strings.TrimSpace(req.Source) != "" {
		cards, err := ReferenceCards(req.Source)
		if err != nil {
			return ReferenceCard{}, err
		}
		return cards[0], nil
	}
	if req.Engine != nil {
		return referenceForEngine(*req.Engine, "")
	}
	seed := req.Seed
	if seed < 0 {
		seed = -seed
	}
	return referenceCards[int(seed%int64(len(referenceCards)))], nil
}

func buildItemCard(rng *rand.Rand, genre string, reference ReferenceCard, req ItemRequest, category string, index int) ItemCard {
	pattern := itemPatterns[category]
	prefix := pick(rng, referenceItemPrefixes[reference.ID])
	if prefix == "" {
		prefix = pick(rng, referenceItemPrefixes[""])
	}
	noun := pick(rng, pattern.Nouns)
	name := prefix + noun
	id := fmt.Sprintf("item-%d-%02d", req.Seed, index+1)
	if req.PackID != "" {
		id = fmt.Sprintf("item-%s-%02d", req.PackID, index+1)
	}
	engineLink := ""
	if req.Engine != nil {
		engineLink = fmt.Sprintf("与%s的签发、核验或流通直接相关", req.Engine.Artifact)
	}
	form := pick(rng, itemPhysicalForms[noun])
	if form == "" {
		form = pick(rng, pattern.Forms)
	}
	functions := itemSpecificFunctions[noun]
	if len(functions) == 0 {
		functions = pattern.Functions
	}
	return ItemCard{
		ID:                  id,
		PackID:              req.PackID,
		ReferenceID:         reference.ID,
		Name:                name,
		BaseObject:          noun,
		Category:            category,
		PhysicalForm:        form,
		Function:            pick(rng, functions),
		Acquisition:         itemAcquisition(reference, req.Engine),
		UseCost:             pick(rng, pattern.UseCosts),
		Limit:               pick(rng, pattern.Limits),
		Counterplay:         pick(rng, pattern.Counterplays),
		FailureMode:         pick(rng, pattern.Failures),
		EngineLink:          engineLink,
		ReferenceConstraint: reference.StructuralMechanics[index%len(reference.StructuralMechanics)],
		NarrativeUses:       pickDistinct(rng, pattern.Narrative, 2),
		CompatibilityTags:   []string{genre, reference.ID, category},
	}
}

func itemAcquisition(reference ReferenceCard, engine *Engine) string {
	if engine != nil {
		return fmt.Sprintf("通过%s的合法渠道、维护残骸或黑市副本获得", engine.Issuer)
	}
	return "通过本地机构、交易网络、任务现场或黑市副本获得"
}
