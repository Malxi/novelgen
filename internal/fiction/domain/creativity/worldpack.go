package creativity

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
)

// WorldPack is one coherent setting draw. Every premise, storyline, cast card,
// and resource card is derived from the same Engine, so independently drawing
// decks with the same seed/count produces components with matching PackID.
type WorldPack struct {
	ID            string              `json:"id"`
	Seed          int64               `json:"seed"`
	Genre         string              `json:"genre"`
	Family        string              `json:"family"`
	Reference     ReferenceCard       `json:"reference"`
	Engine        Engine              `json:"engine"`
	Premise       PremiseCard         `json:"premise"`
	Storylines    []StorylineCard     `json:"storylines"`
	Cast          []CastCard          `json:"cast"`
	Resources     []ResourceCard      `json:"resources"`
	Items         []ItemCard          `json:"items"`
	Compatibility CompatibilityReport `json:"compatibility"`
}

// WorldPackResult is a deterministic set of compatible setting packs.
type WorldPackResult struct {
	Seed     int64       `json:"seed"`
	Genre    string      `json:"genre"`
	Packs    []WorldPack `json:"packs"`
	Warnings []string    `json:"warnings,omitempty"`
}

// PremiseCard is the compact world contract derived from an Engine.
type PremiseCard struct {
	ID                  string   `json:"id"`
	PackID              string   `json:"pack_id"`
	ReferenceID         string   `json:"reference_id"`
	Family              string   `json:"family"`
	Title               string   `json:"title"`
	Statement           string   `json:"statement"`
	SurfaceRule         string   `json:"surface_rule"`
	HiddenTruth         string   `json:"hidden_truth"`
	Scarcity            string   `json:"scarcity"`
	CentralConflict     string   `json:"central_conflict"`
	ProtagonistLeverage string   `json:"protagonist_leverage"`
	DefaultExpectation  string   `json:"default_expectation"`
	CounterExpectation  string   `json:"counter_expectation"`
	ReferenceMechanic   string   `json:"reference_mechanic"`
	MustAnswer          []string `json:"must_answer"`
	RejectIf            []string `json:"reject_if"`
}

// StorylineCard is one storyline promise tied to a WorldPack.
type StorylineCard struct {
	ID            string `json:"id"`
	PackID        string `json:"pack_id"`
	ReferenceID   string `json:"reference_id"`
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	Promise       string `json:"promise"`
	Pressure      string `json:"pressure"`
	Opposition    string `json:"opposition"`
	Turn          string `json:"turn"`
	Payoff        string `json:"payoff"`
	LinkedPremise string `json:"linked_premise"`
	ReferenceHook string `json:"reference_hook"`
}

// CastCard is a named role promise for one WorldPack. Names are presentation;
// role, motive, secret, and function are derived from the shared Engine.
type CastCard struct {
	ID            string   `json:"id"`
	PackID        string   `json:"pack_id"`
	ReferenceID   string   `json:"reference_id"`
	RunNamecardID string   `json:"namecard_id"`
	Name          string   `json:"name"`
	Role          string   `json:"role"`
	Archetype     string   `json:"archetype"`
	Personality   []string `json:"personality"`
	Contradiction string   `json:"contradiction"`
	Want          string   `json:"want"`
	Secret        string   `json:"secret"`
	Function      string   `json:"function"`
	Relationship  string   `json:"relationship"`
	SpeechStyle   string   `json:"speech_style"`
	BodyLanguage  string   `json:"body_language"`
	NamingLogic   string   `json:"naming_logic"`
}

// ResourceCard is an economy unit tied to a WorldPack.
type ResourceCard struct {
	ID                  string `json:"id"`
	PackID              string `json:"pack_id"`
	ReferenceID         string `json:"reference_id"`
	Name                string `json:"name"`
	Category            string `json:"category"`
	Meaning             string `json:"meaning"`
	Issuer              string `json:"issuer"`
	SpendRule           string `json:"spend_rule"`
	ScarcityMechanism   string `json:"scarcity_mechanism"`
	FailureMode         string `json:"failure_mode"`
	ReferenceConstraint string `json:"reference_constraint"`
}

// CompatibilityReport records the deterministic checks that kept one pack's
// cards tied to the same Engine.
type CompatibilityReport struct {
	Compatible bool     `json:"compatible"`
	PackID     string   `json:"pack_id"`
	Checks     []string `json:"checks"`
	Warnings   []string `json:"warnings,omitempty"`
}

type familyResourceSpec struct {
	Name     string
	Category string
	Meaning  string
}

var worldPackFamilyLabels = map[string]string{
	"insurance":    "风险与理赔",
	"license":      "许可与通行",
	"contract":     "劳动与违约",
	"inheritance":  "身份与继承",
	"death_record": "死亡与登记",
	"salvage":      "打捞与回收",
	"quota":        "配额与排队",
	"maintenance":  "维护与停摆",
	"registry":     "身份与档案",
	"evidence":     "证据与封存",
	"sanctuary":    "庇护与准入",
	"ration":       "配给与生存",
}

var familyResourceSpecs = map[string]familyResourceSpec{
	"insurance":    {"理赔额度", "制度信用", "决定事故能否被承认为可赔付损失"},
	"license":      {"通行额度", "通行权", "决定谁能在限制区内工作或移动"},
	"contract":     {"可执行工时", "劳动权", "把身体时间转换成可转让的履约能力"},
	"inheritance":  {"继承顺位", "身份信用", "决定死者权利和债务由谁承接"},
	"death_record": {"死亡登记号", "行政凭证", "决定一个人在法律上是否仍然存在"},
	"salvage":      {"回收配额", "作业权", "决定打捞物能否被合法带回并结算"},
	"quota":        {"配额点数", "制度货币", "决定稀缺服务在队伍中的分配顺序"},
	"maintenance":  {"维护工时", "基础设施权", "决定关键设备何时停机以及谁先被牺牲"},
	"registry":     {"身份档案号", "身份凭证", "决定一个人能否被系统承认为原主"},
	"evidence":     {"封存凭证", "证据权", "决定记录能否被法庭和公众接收"},
	"sanctuary":    {"庇护名额", "准入权", "决定危险区内谁有权进入受保护空间"},
	"ration":       {"生活配给点", "生存资源", "决定日常消耗品能否被兑换和续期"},
}

var characterNamePools = map[string][]string{
	"科幻": {"周泊", "叶弦", "苏禾", "程砚", "闻舟", "孟铎", "陆遥", "宁澈", "沈岚", "祁舟", "陶砾", "顾原"},
	"仙侠": {"沈砚", "陆清", "谢岚", "宁照", "顾雪", "江蘅", "苏问", "祁夜", "白岑", "闻溪", "叶棠", "程烛"},
	"都市": {"周宁", "林澄", "许言", "陈禾", "唐舟", "苏桥", "顾安", "叶岑", "陆遥", "沈青", "程诺", "夏然"},
	"悬疑": {"沈闻", "林默", "许宁", "程砚", "顾岚", "叶舟", "陆清", "苏禾", "唐照", "周棠", "陈雾", "祁安"},
	"通用": {"周泊", "叶弦", "苏禾", "程砚", "闻舟", "孟铎", "陆遥", "宁澈", "沈岚", "祁舟", "陶砾", "顾原"},
}

// GenerateWorldPacks draws coherent world packs. Count means the number of
// Engine families/packs, not the number of derived cards.
func GenerateWorldPacks(req EngineRequest) (WorldPackResult, error) {
	engines, err := GenerateEngines(req)
	if err != nil {
		return WorldPackResult{}, err
	}
	packs := make([]WorldPack, 0, len(engines.Engines))
	for i, engine := range engines.Engines {
		reference, err := referenceForEngine(engine, req.Source)
		if err != nil {
			return WorldPackResult{}, err
		}
		packs = append(packs, buildWorldPack(engine, reference, i))
	}
	return WorldPackResult{
		Seed:     engines.Seed,
		Genre:    engines.Genre,
		Packs:    packs,
		Warnings: engines.Warnings,
	}, nil
}

func buildWorldPack(engine Engine, reference ReferenceCard, index int) WorldPack {
	packID := engine.ID
	premise := buildPremiseCard(engine, reference)
	cast := buildCastDeck(engine, reference, packID)
	resources := buildResourceDeck(engine, reference, packID)
	items := buildItemDeck(engine, reference, packID)
	storylines := buildStorylineDeck(engine, reference, packID, cast, resources)
	pack := WorldPack{
		ID:         packID,
		Seed:       engine.Seed,
		Genre:      engine.Genre,
		Family:     engine.Family,
		Reference:  reference,
		Engine:     engine,
		Premise:    premise,
		Storylines: storylines,
		Cast:       cast,
		Resources:  resources,
		Items:      items,
	}
	pack.Compatibility = CompatibilityReport{
		PackID: packID,
		Checks: []string{
			"all_cards_share_pack_id",
			"premise_references_engine_artifact",
			"storylines_reference_engine_domain",
			"cast_has_closed_role_set",
			"resources_include_engine_artifact",
			"items_cover_core_categories",
			"default_expectation_matches_family",
			"one_reference_lens_per_pack",
		},
	}
	issues := ValidateWorldPack(pack)
	pack.Compatibility.Compatible = len(issues) == 0
	pack.Compatibility.Warnings = issues
	return pack
}

func buildPremiseCard(engine Engine, reference ReferenceCard) PremiseCard {
	label := worldPackFamilyLabels[engine.Family]
	if label == "" {
		label = engine.Family
	}
	return PremiseCard{
		ID:                  "premise-" + engine.ID,
		PackID:              engine.ID,
		ReferenceID:         reference.ID,
		Family:              engine.Family,
		Title:               label + "：" + engine.Artifact,
		Statement:           engine.PremiseScaffold,
		SurfaceRule:         engine.RuleSeeds[0],
		HiddenTruth:         fmt.Sprintf("%s表面用于%s，实际用于%s。", engine.Artifact, engine.StatedPurpose, engine.HiddenFunction),
		Scarcity:            fmt.Sprintf("只有%s签发的%s被承认；绕过它通常要支付%s并留下可追查记录。", engine.Issuer, engine.Artifact, engine.Cost),
		CentralConflict:     fmt.Sprintf("%s与%s争夺%s的解释权；冲突类型包括%s。", engine.Holder, engine.Issuer, engine.Artifact, strings.Join(engine.ConflictTypes, "、")),
		ProtagonistLeverage: engine.SignatureWin,
		DefaultExpectation:  engine.DefaultExpectation,
		CounterExpectation:  engine.CounterExpectation,
		ReferenceMechanic:   reference.StructuralMechanics[0],
		MustAnswer:          append([]string(nil), engine.RequiredQuestions...),
		RejectIf:            append([]string(nil), engine.RejectIf...),
	}
}

func buildStorylineDeck(engine Engine, reference ReferenceCard, packID string, cast []CastCard, resources []ResourceCard) []StorylineCard {
	resourceName := engine.Artifact
	if len(resources) > 0 {
		resourceName = resources[0].Name
	}
	premiseID := "premise-" + packID
	return []StorylineCard{
		{
			ID:            "storyline-" + packID + "-resource",
			PackID:        packID,
			ReferenceID:   reference.ID,
			Kind:          "resource",
			Title:         "谁有权使用" + resourceName,
			Promise:       fmt.Sprintf("%s用%s解决眼前损失，读者看到规则如何定价。", cast[0].Name, resourceName),
			Pressure:      fmt.Sprintf("%s掌握签发权，%s必须付出%s才能继续。", engine.Issuer, engine.Holder, engine.Cost),
			Opposition:    fmt.Sprintf("%s不能让所有人都获得同一种%s解释权。", engine.Issuer, resourceName),
			Turn:          fmt.Sprintf("主角发现%s并非事故，而是%s的日常收益。", engine.HiddenFunction, engine.Issuer),
			Payoff:        engine.SignatureWin,
			LinkedPremise: premiseID,
			ReferenceHook: reference.StoryEngines[0],
		},
		{
			ID:            "storyline-" + packID + "-revelation",
			PackID:        packID,
			ReferenceID:   reference.ID,
			Kind:          "revelation",
			Title:         "被隐藏的" + engine.Artifact,
			Promise:       fmt.Sprintf("主角追查%s为什么失效，逐步得到可验证的签名和记录。", engine.Artifact),
			Pressure:      fmt.Sprintf("%s会封存%s，并把追查者登记为违约者。", engine.Issuer, engine.Artifact),
			Opposition:    fmt.Sprintf("%s代表制度执行，%s则同时依赖这套制度生活。", engine.Issuer, engine.Holder),
			Turn:          fmt.Sprintf("关键漏洞是：%s", engine.Loophole),
			Payoff:        fmt.Sprintf("%s的真实功能被公开，%s失去独占解释权。", engine.HiddenFunction, engine.Issuer),
			LinkedPremise: premiseID,
			ReferenceHook: reference.StoryEngines[1],
		},
		{
			ID:            "storyline-" + packID + "-institution",
			PackID:        packID,
			ReferenceID:   reference.ID,
			Kind:          "institution",
			Title:         "制度的第一个早晨：" + engine.Artifact,
			Promise:       fmt.Sprintf("主角准备废除%s，却发现很多普通人正靠它换取生活所需。", engine.Artifact),
			Pressure:      fmt.Sprintf("一旦%s停摆，%s会立刻承担%s。", engine.Artifact, engine.Issuer, engine.Cost),
			Opposition:    fmt.Sprintf("%s内部的执行者并非都想作恶，他们也在维护自己的配额。", engine.Issuer),
			Turn:          engine.CounterExpectation,
			Payoff:        fmt.Sprintf("主角改写%s的签发条件，而不是简单摧毁它。", engine.Artifact),
			LinkedPremise: premiseID,
			ReferenceHook: reference.StoryEngines[2],
		},
	}
}

func buildCastDeck(engine Engine, reference ReferenceCard, packID string) []CastCard {
	names := deterministicNames(engine.Genre, packID, 4)
	nameResult, _ := GenerateNameCards(NameRequest{
		Genre:          engine.Genre,
		Seed:           engine.Seed,
		Count:          4,
		Source:         reference.ID,
		PackID:         packID,
		ReferenceID:    reference.ID,
		ArchetypeRoles: []string{"protagonist", "antagonist", "ally", "wildcard"},
	})
	namecards := nameResult.Cards
	if len(namecards) == 4 {
		names = []string{namecards[0].Name, namecards[1].Name, namecards[2].Name, namecards[3].Name}
	} else {
		fallback := []string{"主角", "对手", "盟友", "变量"}
		namecards = make([]NameCard, 4)
		for i := range namecards {
			namecards[i] = NameCard{
				ID:            fmt.Sprintf("name-%s-fallback-%02d", packID, i+1),
				Name:          names[i],
				Archetype:     fallback[i],
				Personality:   []string{"待定"},
				Contradiction: "待定",
				SpeechStyle:   "待定",
				BodyLanguage:  "待定",
				NamingLogic:   "使用兼容回退姓名卡",
			}
		}
	}
	functions := engine.CastSeeds
	if len(functions) < 4 {
		functions = []string{"主角", "对手", "盟友", "变量"}
	}
	return []CastCard{
		{
			ID:            "cast-" + packID + "-protagonist",
			PackID:        packID,
			ReferenceID:   reference.ID,
			RunNamecardID: namecards[0].ID,
			Name:          names[0],
			Role:          "protagonist",
			Archetype:     namecards[0].Archetype,
			Personality:   namecards[0].Personality,
			Contradiction: namecards[0].Contradiction,
			Want:          fmt.Sprintf("在不摧毁%s的前提下保住自己负责的人。", engine.Artifact),
			Secret:        fmt.Sprintf("已经接触过%s，但还没有证据。", engine.HiddenFunction),
			Function:      functions[0],
			Relationship:  fmt.Sprintf("唯一能同时接触%s和%s的人", engine.Artifact, engine.Issuer),
			SpeechStyle:   namecards[0].SpeechStyle,
			BodyLanguage:  namecards[0].BodyLanguage,
			NamingLogic:   namecards[0].NamingLogic,
		},
		{
			ID:            "cast-" + packID + "-counterpart",
			PackID:        packID,
			ReferenceID:   reference.ID,
			RunNamecardID: namecards[1].ID,
			Name:          names[1],
			Role:          "antagonist",
			Archetype:     namecards[1].Archetype,
			Personality:   namecards[1].Personality,
			Contradiction: namecards[1].Contradiction,
			Want:          fmt.Sprintf("维持%s的正常运转，避免系统性停摆。", engine.Artifact),
			Secret:        fmt.Sprintf("知道%s会造成伤害，但相信替代方案更糟。", engine.HiddenFunction),
			Function:      functions[1],
			Relationship:  fmt.Sprintf("%s的代表与执行者", engine.Issuer),
			SpeechStyle:   namecards[1].SpeechStyle,
			BodyLanguage:  namecards[1].BodyLanguage,
			NamingLogic:   namecards[1].NamingLogic,
		},
		{
			ID:            "cast-" + packID + "-ally",
			PackID:        packID,
			ReferenceID:   reference.ID,
			RunNamecardID: namecards[2].ID,
			Name:          names[2],
			Role:          "ally",
			Archetype:     namecards[2].Archetype,
			Personality:   namecards[2].Personality,
			Contradiction: namecards[2].Contradiction,
			Want:          fmt.Sprintf("拿回被%s扣走的东西。", engine.Artifact),
			Secret:        fmt.Sprintf("知道%s的一个漏洞，但害怕失去现有生活。", engine.Loophole),
			Function:      functions[2],
			Relationship:  fmt.Sprintf("被%s伤害过，仍依赖它生活的%s", engine.Artifact, engine.Holder),
			SpeechStyle:   namecards[2].SpeechStyle,
			BodyLanguage:  namecards[2].BodyLanguage,
			NamingLogic:   namecards[2].NamingLogic,
		},
		{
			ID:            "cast-" + packID + "-variable",
			PackID:        packID,
			ReferenceID:   reference.ID,
			RunNamecardID: namecards[3].ID,
			Name:          names[3],
			Role:          "wildcard",
			Archetype:     namecards[3].Archetype,
			Personality:   namecards[3].Personality,
			Contradiction: namecards[3].Contradiction,
			Want:          "让别人先签字，自己保留选择。",
			Secret:        fmt.Sprintf("掌握%s为何没有被堵上的证据。", engine.Loophole),
			Function:      functions[3],
			Relationship:  "知道漏洞却不愿主动作证的变量",
			SpeechStyle:   namecards[3].SpeechStyle,
			BodyLanguage:  namecards[3].BodyLanguage,
			NamingLogic:   namecards[3].NamingLogic,
		},
	}
}

func buildResourceDeck(engine Engine, reference ReferenceCard, packID string) []ResourceCard {
	spec := familyResourceSpecs[engine.Family]
	if spec.Name == "" {
		spec = familyResourceSpec{Name: "结算凭证", Category: "制度信用", Meaning: "决定资源与身份能否被机构承认"}
	}
	return []ResourceCard{
		{
			ID:                  "resource-" + packID + "-artifact",
			PackID:              packID,
			ReferenceID:         reference.ID,
			Name:                engine.Artifact,
			Category:            "凭证",
			Meaning:             engine.StatedPurpose,
			Issuer:              engine.Issuer,
			SpendRule:           fmt.Sprintf("只有%s签发的%s能触发%s。", engine.Issuer, engine.Artifact, engine.Trigger),
			ScarcityMechanism:   fmt.Sprintf("%s通过控制签发权维持稀缺，绕过需要支付%s。", engine.Issuer, engine.Cost),
			FailureMode:         fmt.Sprintf("若%s可以被任意复制，整个%s冲突会失效。", engine.Artifact, engine.Family),
			ReferenceConstraint: reference.StructuralMechanics[1],
		},
		{
			ID:                  "resource-" + packID + "-settlement",
			PackID:              packID,
			ReferenceID:         reference.ID,
			Name:                spec.Name,
			Category:            spec.Category,
			Meaning:             spec.Meaning,
			Issuer:              engine.Issuer,
			SpendRule:           fmt.Sprintf("%s按%s结算，未结算部分由持有人承担。", spec.Name, engine.Trigger),
			ScarcityMechanism:   fmt.Sprintf("%s控制%s，并把%s转嫁给%s。", engine.Issuer, spec.Name, engine.HiddenFunction, engine.Holder),
			FailureMode:         fmt.Sprintf("如果%s可以免费获得，%s就不再具有约束力。", spec.Name, engine.Artifact),
			ReferenceConstraint: reference.StructuralMechanics[2],
		},
	}
}

func buildItemDeck(engine Engine, reference ReferenceCard, packID string) []ItemCard {
	result, _ := GenerateItemCards(ItemRequest{
		Genre:       engine.Genre,
		Seed:        engine.Seed,
		Count:       5,
		Source:      reference.ID,
		PackID:      packID,
		ReferenceID: reference.ID,
		Engine:      &engine,
	})
	return result.Items
}

func deterministicNames(genre, packID string, count int) []string {
	pool := characterNamePools[genre]
	if len(pool) == 0 {
		pool = characterNamePools["通用"]
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(packID + "|cast-names"))
	rng := rand.New(rand.NewSource(int64(h.Sum64() & 0x7fffffffffffffff)))
	order := rng.Perm(len(pool))
	names := make([]string, 0, count)
	seen := make(map[string]bool)
	for _, idx := range order {
		name := pool[idx]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
		if len(names) == count {
			break
		}
	}
	return names
}

// ValidateWorldPack checks that every derived card belongs to the same Engine
// and still references the Engine's structural facts.
func ValidateWorldPack(pack WorldPack) []string {
	issues := make([]string, 0)
	if strings.TrimSpace(pack.ID) == "" || pack.ID != pack.Engine.ID {
		issues = append(issues, "pack id must match engine id")
	}
	if strings.TrimSpace(pack.Reference.ID) == "" || strings.TrimSpace(pack.Reference.SourceURL) == "" {
		issues = append(issues, "world pack needs a researched reference card with source_url")
	}
	if len(pack.Reference.StructuralMechanics) < 3 || len(pack.Reference.StoryEngines) < 3 {
		issues = append(issues, "world pack reference needs at least three mechanics and story engines")
	}
	if pack.Premise.PackID != pack.ID {
		issues = append(issues, "premise pack_id does not match pack")
	}
	if pack.Premise.ReferenceID != pack.Reference.ID {
		issues = append(issues, "premise reference_id does not match pack")
	}
	if strings.TrimSpace(pack.Premise.ReferenceMechanic) == "" {
		issues = append(issues, "premise does not apply its reference mechanic")
	}
	if len(pack.Storylines) < 3 {
		issues = append(issues, "world pack needs at least three storylines")
	}
	if len(pack.Cast) < 4 {
		issues = append(issues, "world pack needs at least four cast cards")
	}
	if len(pack.Resources) < 2 {
		issues = append(issues, "world pack needs at least two resource cards")
	}
	if len(pack.Items) < 5 {
		issues = append(issues, "world pack needs at least five item cards")
	}
	premiseText := strings.Join([]string{
		pack.Premise.Statement,
		pack.Premise.SurfaceRule,
		pack.Premise.HiddenTruth,
		pack.Premise.Scarcity,
	}, "\n")
	if pack.Engine.Artifact != "" && !strings.Contains(premiseText, pack.Engine.Artifact) {
		issues = append(issues, "premise does not reference engine artifact")
	}
	for _, storyline := range pack.Storylines {
		if storyline.PackID != pack.ID {
			issues = append(issues, "storyline "+storyline.ID+" has a foreign pack_id")
		}
		if storyline.ReferenceID != pack.Reference.ID {
			issues = append(issues, "storyline "+storyline.ID+" has a foreign reference_id")
		}
		if strings.TrimSpace(storyline.ReferenceHook) == "" {
			issues = append(issues, "storyline "+storyline.ID+" has no reference hook")
		}
		text := storyline.Promise + storyline.Pressure + storyline.Opposition + storyline.Turn + storyline.Payoff
		if !containsAnyWorldPackText(text, pack.Engine.Artifact, pack.Engine.Issuer, pack.Engine.Holder, pack.Engine.HiddenFunction) {
			issues = append(issues, "storyline "+storyline.ID+" does not reference its engine")
		}
	}
	seenNames := make(map[string]bool)
	for _, cast := range pack.Cast {
		if cast.PackID != pack.ID {
			issues = append(issues, "cast "+cast.ID+" has a foreign pack_id")
		}
		if cast.ReferenceID != pack.Reference.ID {
			issues = append(issues, "cast "+cast.ID+" has a foreign reference_id")
		}
		if strings.TrimSpace(cast.Name) == "" || seenNames[cast.Name] {
			issues = append(issues, "cast names must be non-empty and unique")
		}
		if strings.TrimSpace(cast.RunNamecardID) == "" || strings.TrimSpace(cast.Archetype) == "" ||
			len(cast.Personality) == 0 || strings.TrimSpace(cast.SpeechStyle) == "" ||
			strings.TrimSpace(cast.NamingLogic) == "" {
			issues = append(issues, "cast "+cast.ID+" is missing roleplay name-card fields")
		}
		seenNames[cast.Name] = true
	}
	hasArtifact := false
	for _, resource := range pack.Resources {
		if resource.PackID != pack.ID {
			issues = append(issues, "resource "+resource.ID+" has a foreign pack_id")
		}
		if resource.ReferenceID != pack.Reference.ID {
			issues = append(issues, "resource "+resource.ID+" has a foreign reference_id")
		}
		if strings.TrimSpace(resource.ReferenceConstraint) == "" {
			issues = append(issues, "resource "+resource.ID+" has no reference constraint")
		}
		if resource.Name == pack.Engine.Artifact {
			hasArtifact = true
		}
	}
	if !hasArtifact {
		issues = append(issues, "resources do not include engine artifact")
	}
	itemCategoriesSeen := make(map[string]bool)
	for _, item := range pack.Items {
		if item.PackID != pack.ID {
			issues = append(issues, "item "+item.ID+" has a foreign pack_id")
		}
		if item.ReferenceID != pack.Reference.ID {
			issues = append(issues, "item "+item.ID+" has a foreign reference_id")
		}
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Function) == "" ||
			strings.TrimSpace(item.Limit) == "" || strings.TrimSpace(item.Counterplay) == "" {
			issues = append(issues, "item "+item.ID+" is incomplete")
		}
		itemCategoriesSeen[item.Category] = true
	}
	for _, category := range []string{"resource", "equipment", "consumable", "prop", "credential"} {
		if !itemCategoriesSeen[category] {
			issues = append(issues, "world pack is missing item category "+category)
		}
	}
	if !worldPackContains(engineDefaultExpectationsByFamily[pack.Engine.Family], pack.Engine.DefaultExpectation) {
		issues = append(issues, "engine default expectation does not match its family")
	}
	if strings.TrimSpace(pack.Engine.DefaultExpectation) == strings.TrimSpace(pack.Engine.CounterExpectation) {
		issues = append(issues, "default and counter expectations must differ")
	}
	return issues
}

func containsAnyWorldPackText(text string, values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func worldPackContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
