package creativity

import (
	"math/rand"
	"strings"
)

type storylineArchetype struct {
	ID                   string
	Slot                 string
	Kind                 string
	Title                string
	Promise              string
	Pressure             string
	Opposition           string
	Turn                 string
	Payoff               string
	LinkedRoles          []string
	LinkedItemCategories []string
}

var storylineArcSlots = []string{"claim", "revelation", "choice"}

var storylineArchetypes = map[string][]storylineArchetype{
	"claim": {
		{
			ID: "access_claim", Slot: "claim", Kind: "resource",
			Title:                "谁有权使用{resource}",
			Promise:              "{protagonist}用{resource}解决眼前损失，读者先看到规则怎样给行动定价。",
			Pressure:             "{issuer}掌握签发权，{holder}必须付出{cost}才能继续。",
			Opposition:           "{antagonist}坚持只有官方签发的{artifact}有效。",
			Turn:                 "{protagonist}发现{hidden}并不是意外，而是{issuer}的日常收益。",
			Payoff:               "{signature_win}",
			LinkedRoles:          []string{"protagonist", "antagonist"},
			LinkedItemCategories: []string{"resource", "credential"},
		},
		{
			ID: "black_market", Slot: "claim", Kind: "resource",
			Title:                "{resource}的黑市",
			Promise:              "{ally}找到一条不经过官方渠道的交换路线，但每个副本都会留下追踪痕迹。",
			Pressure:             "{trigger}迫近，{issuer}随时可以把所有参与者登记为违约者。",
			Opposition:           "{antagonist}利用吊销权逼迫买家交出名单。",
			Turn:                 "黑市副本比正本更早记录了{hidden}。",
			Payoff:               "{protagonist}让黑市渠道成为公开审计入口，使{issuer}无法继续独占解释权。",
			LinkedRoles:          []string{"ally", "antagonist"},
			LinkedItemCategories: []string{"commodity", "credential"},
		},
		{
			ID: "quota_revolt", Slot: "claim", Kind: "resource",
			Title:                "配额上涨之前",
			Promise:              "{protagonist}必须在{trigger}前为{resource}找到替代供给。",
			Pressure:             "官方配额被冻结，{holder}和家属先承担{cost}。",
			Opposition:           "{issuer}把短缺解释成无法避免的自然现象。",
			Turn:                 "真正稀缺的是被认可的凭证，而不是可用物资本身。",
			Payoff:               "幸存者用公共库存迫使{issuer}重新结算。",
			LinkedRoles:          []string{"protagonist", "ally"},
			LinkedItemCategories: []string{"resource", "commodity"},
		},
		{
			ID: "debt_transfer", Slot: "claim", Kind: "resource",
			Title:                "债务随凭证转移",
			Promise:              "{ally}拒绝接收死者的债务，却发现{artifact}会自动改变责任人。",
			Pressure:             "每次拒签都会让另一个家庭承担{cost}。",
			Opposition:           "{antagonist}以程序合法为由完成债务转移。",
			Turn:                 "缺失的联合签名可以让债务回到签发者身上。",
			Payoff:               "{protagonist}公开转移链，让{issuer}自己成为最后债务人。",
			LinkedRoles:          []string{"ally", "antagonist"},
			LinkedItemCategories: []string{"credential", "prop"},
		},
	},
	"revelation": {
		{
			ID: "hidden_function", Slot: "revelation", Kind: "revelation",
			Title:                "被隐藏的{artifact}",
			Promise:              "{protagonist}追查{artifact}为什么失效，逐步取得可验证的签名和记录。",
			Pressure:             "{issuer}会封存{artifact}，并把追查者登记为违约者。",
			Opposition:           "{antagonist}代表制度执法，{holder}却仍依赖这套制度生活。",
			Turn:                 "关键漏洞是：{loophole}",
			Payoff:               "{hidden}的真实功能被公开，{issuer}失去独占解释权。",
			LinkedRoles:          []string{"protagonist", "antagonist", "ally"},
			LinkedItemCategories: []string{"prop", "equipment"},
		},
		{
			ID: "missing_signature", Slot: "revelation", Kind: "revelation",
			Title:                "缺失的一条签名",
			Promise:              "{protagonist}拿到一份看似完整的{artifact}，却发现最后一个签名被人切走。",
			Pressure:             "没有完整签名，{issuer}可以把证据反过来当成伪造品。",
			Opposition:           "{antagonist}抢先提交版本更完整、但时间错误的记录。",
			Turn:                 "真正缺失的签名保存在{holder}一直随身携带的旧记录里。",
			Payoff:               "{protagonist}用两条不完整记录完成相互验证，证明{hidden}。",
			LinkedRoles:          []string{"protagonist", "antagonist", "ally"},
			LinkedItemCategories: []string{"prop", "credential"},
		},
		{
			ID: "false_witness", Slot: "revelation", Kind: "revelation",
			Title:                "证人说得太完整",
			Promise:              "{ally}的证词能证明{hidden}，但每个细节都精确得像提前背过。",
			Pressure:             "{issuer}公开质疑证词，要求用{artifact}原件复核。",
			Opposition:           "{antagonist}利用一处真实但缺少上下文的记录制造伪证。",
			Turn:                 "证词确实被人改写，改写者却是在保护真正的目击者。",
			Payoff:               "{protagonist}把证词与{equipment}记录的原始噪声对齐，恢复完整时间线。",
			LinkedRoles:          []string{"ally", "antagonist", "protagonist"},
			LinkedItemCategories: []string{"prop", "equipment"},
		},
		{
			ID: "decoy_prize", Slot: "revelation", Kind: "revelation",
			Title:                "{resource}只是诱饵",
			Promise:              "{protagonist}和{antagonist}争夺{resource}，却逐渐发现它只是用来定位所有参与者的诱饵。",
			Pressure:             "每次接近资源都会触发{trigger}，让{issuer}获得更多坐标。",
			Opposition:           "{antagonist}明知风险仍拒绝放弃，因为放弃等于承认自己已被当作资产。",
			Turn:                 "真正的目标藏在{prop}里，而{resource}只负责把所有人聚到同一条航线。",
			Payoff:               "{protagonist}故意让诱饵继续广播，反向定位{issuer}的控制中心。",
			LinkedRoles:          []string{"protagonist", "antagonist"},
			LinkedItemCategories: []string{"resource", "prop"},
		},
	},
	"choice": {
		{
			ID: "reform_dilemma", Slot: "choice", Kind: "institution",
			Title:                "规则废除后的第一个早晨",
			Promise:              "{protagonist}准备废除{artifact}，却发现很多普通人正靠它换取生活所需。",
			Pressure:             "一旦{artifact}停摆，{issuer}会立刻让{holder}承担{cost}。",
			Opposition:           "{antagonist}并非单纯作恶，他也在维护一个依赖旧规则的群体。",
			Turn:                 "{counter_expectation}",
			Payoff:               "{protagonist}改写{artifact}的签发条件，而不是简单摧毁它。",
			LinkedRoles:          []string{"protagonist", "antagonist", "ally"},
			LinkedItemCategories: []string{"credential", "resource"},
		},
		{
			ID: "rival_conversion", Slot: "choice", Kind: "relationship",
			Title:                "{antagonist}为什么开始迟疑",
			Promise:              "{protagonist}必须争取{antagonist}的暂时合作，而对方仍掌握完成任务所需的关键权限。",
			Pressure:             "{issuer}用{cost}逼迫{antagonist}继续执行旧命令。",
			Opposition:           "{antagonist}相信服从制度至少还能保护一部分人。",
			Turn:                 "{antagonist}发现自己的权限也会在{trigger}后被同样回收。",
			Payoff:               "{antagonist}公开倒向幸存者，并交出{credential}。",
			LinkedRoles:          []string{"protagonist", "antagonist"},
			LinkedItemCategories: []string{"credential", "prop"},
		},
		{
			ID: "loyalty_test", Slot: "choice", Kind: "relationship",
			Title:                "把{resource}交给谁",
			Promise:              "{protagonist}必须在{ally}和更大的幸存者群体之间分配唯一的{resource}。",
			Pressure:             "{trigger}只剩一个窗口，任何选择都会让另一部分人失去补给。",
			Opposition:           "{issuer}故意公开分配公式，让幸存者互相争夺。",
			Turn:                 "{ally}拒绝接受独占份额，提出把{resource}用于建立公共中继。",
			Payoff:               "{protagonist}放弃个人优先权，用共享配额完成一次更大的救援。",
			LinkedRoles:          []string{"protagonist", "ally"},
			LinkedItemCategories: []string{"resource", "vehicle"},
		},
		{
			ID: "public_audit", Slot: "choice", Kind: "trial",
			Title:                "公开听证会",
			Promise:              "{protagonist}把{artifact}带进公开听证，希望{issuer}无法继续私下结算。",
			Pressure:             "{issuer}提前删改了支持文件，只留下对{holder}不利的版本。",
			Opposition:           "{antagonist}以程序问题拖延听证，等待{trigger}结束。",
			Turn:                 "{prop}保留了被删记录的另一份时间戳。",
			Payoff:               "听证结果让所有幸存者成为共同见证人和共同债权人。",
			LinkedRoles:          []string{"protagonist", "antagonist", "ally"},
			LinkedItemCategories: []string{"prop", "credential"},
		},
	},
}

func buildStorylineArchetypeDeck(engine Engine, reference ReferenceCard, packID string, cast []CastCard, resources []ResourceCard, items []ItemCard) []StorylineCard {
	rng := rand.New(rand.NewSource(stableNameSeed(packID + "|storylines")))
	tokens := storylineTokens(engine, cast, resources, items)
	premiseID := "premise-" + packID
	result := make([]StorylineCard, 0, len(storylineArcSlots))
	for slotIndex, slot := range storylineArcSlots {
		candidates := storylineArchetypes[slot]
		if preferred := preferredStorylineArchetypes(candidates, reference.StoryBias); len(preferred) > 0 {
			candidates = preferred
		}
		archetype := candidates[rng.Intn(len(candidates))]
		result = append(result, StorylineCard{
			ID:               "storyline-" + packID + "-" + archetype.ID,
			PackID:           packID,
			ReferenceID:      reference.ID,
			ArchetypeID:      archetype.ID,
			ArcSlot:          slot,
			Kind:             archetype.Kind,
			Title:            renderStorylineTemplate(archetype.Title, tokens),
			Promise:          renderStorylineTemplate(archetype.Promise, tokens),
			Pressure:         renderStorylineTemplate(archetype.Pressure, tokens),
			Opposition:       renderStorylineTemplate(archetype.Opposition, tokens),
			Turn:             renderStorylineTemplate(archetype.Turn, tokens),
			Payoff:           renderStorylineTemplate(archetype.Payoff, tokens),
			LinkedPremise:    premiseID,
			LinkedCharacters: linkedStorylineCharacters(cast, archetype.LinkedRoles),
			LinkedItems:      linkedStorylineItems(items, archetype.LinkedItemCategories),
			ReferenceHook:    reference.StoryEngines[slotIndex%len(reference.StoryEngines)],
		})
	}
	return result
}

func preferredStorylineArchetypes(candidates []storylineArchetype, preferred []string) []storylineArchetype {
	wanted := make(map[string]bool, len(preferred))
	for _, id := range preferred {
		wanted[id] = true
	}
	result := make([]storylineArchetype, 0, len(candidates))
	for _, candidate := range candidates {
		if wanted[candidate.ID] {
			result = append(result, candidate)
		}
	}
	return result
}

func storylineTokens(engine Engine, cast []CastCard, resources []ResourceCard, items []ItemCard) map[string]string {
	resourceName := engine.Artifact
	if len(resources) > 0 {
		resourceName = resources[0].Name
	}
	roles := make(map[string]string, len(cast))
	for _, card := range cast {
		roles[card.Role] = card.Name
	}
	itemNames := make(map[string]string, len(items))
	for _, item := range items {
		if _, exists := itemNames[item.Category]; !exists {
			itemNames[item.Category] = item.Name
		}
	}
	return map[string]string{
		"{artifact}":            engine.Artifact,
		"{issuer}":              engine.Issuer,
		"{holder}":              engine.Holder,
		"{cost}":                engine.Cost,
		"{hidden}":              engine.HiddenFunction,
		"{loophole}":            engine.Loophole,
		"{trigger}":             engine.Trigger,
		"{signature_win}":       engine.SignatureWin,
		"{counter_expectation}": engine.CounterExpectation,
		"{protagonist}":         roles["protagonist"],
		"{antagonist}":          roles["antagonist"],
		"{ally}":                roles["ally"],
		"{wildcard}":            roles["wildcard"],
		"{resource}":            resourceName,
		"{equipment}":           itemNames["equipment"],
		"{consumable}":          itemNames["consumable"],
		"{prop}":                itemNames["prop"],
		"{vehicle}":             itemNames["vehicle"],
		"{credential}":          itemNames["credential"],
		"{commodity}":           itemNames["commodity"],
	}
}

func renderStorylineTemplate(template string, tokens map[string]string) string {
	replacements := make([]string, 0, len(tokens)*2)
	for token, value := range tokens {
		replacements = append(replacements, token, value)
	}
	return strings.NewReplacer(replacements...).Replace(template)
}

func linkedStorylineCharacters(cast []CastCard, roles []string) []string {
	byRole := make(map[string]string, len(cast))
	for _, card := range cast {
		byRole[card.Role] = card.ID
	}
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		if id := byRole[role]; id != "" {
			result = append(result, id)
		}
	}
	return result
}

func linkedStorylineItems(items []ItemCard, categories []string) []string {
	wanted := make(map[string]bool, len(categories))
	for _, category := range categories {
		wanted[category] = true
	}
	result := make([]string, 0, len(categories))
	for _, item := range items {
		if wanted[item.Category] {
			result = append(result, item.ID)
		}
	}
	return result
}
