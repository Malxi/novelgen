package quality

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/creativity"
)

// FourAxisScore is the deterministic pre-ranking used before an LLM judge.
// The four axes are intentionally separate: a setup that is creative but
// illogical, or logical but boring, should not win by averaging a high score
// with a near-zero one.
type FourAxisScore struct {
	Creativity   float64  `json:"creativity"`
	Plausibility float64  `json:"plausibility"`
	Logic        float64  `json:"logic"`
	Fun          float64  `json:"fun"`
	Total        float64  `json:"total"`
	Summary      string   `json:"summary"`
	Evidence     []string `json:"evidence,omitempty"`
	Issues       []string `json:"issues,omitempty"`
}

// ScoreSetupFourAxis scores one setup against the four user-facing axes. It is
// a ranking prior, not a replacement for the LLM reviewer: it rewards concrete
// engine fidelity, costs, conflict diversity and win images, and penalizes
// generic tropes, repetition and unsupported magic.
func ScoreSetupFourAxis(setup *models.StorySetup, engine *creativity.Engine, genericTropes []string) FourAxisScore {
	if setup == nil {
		return FourAxisScore{Summary: "empty setup"}
	}
	corpus := setupCorpus(setup)
	genre := ""
	if len(setup.Genres) > 0 {
		genre = setup.Genres[0]
	}
	if engine != nil && engine.Genre != "" {
		genre = engine.Genre
	}

	creativityScore, creativityEvidence, creativityIssues := scoreCreativity(setup, corpus, engine, genericTropes)
	plausibilityScore, plausibilityEvidence, plausibilityIssues := scorePlausibility(setup, corpus, engine, genre)
	logicScore, logicEvidence, logicIssues := scoreLogic(setup)
	funScore, funEvidence, funIssues := scoreFun(setup)

	axisScores := []float64{creativityScore, plausibilityScore, logicScore, funScore}
	minAxis := axisScores[0]
	for _, value := range axisScores[1:] {
		if value < minAxis {
			minAxis = value
		}
	}
	total := math.Pow(creativityScore*plausibilityScore*logicScore*funScore, 0.25)
	total = clampScore(total*0.85 + minAxis*0.15)

	score := FourAxisScore{
		Creativity:   clampScore(creativityScore),
		Plausibility: clampScore(plausibilityScore),
		Logic:        clampScore(logicScore),
		Fun:          clampScore(funScore),
		Total:        total,
		Evidence:     append(append(append(creativityEvidence, plausibilityEvidence...), logicEvidence...), funEvidence...),
		Issues:       append(append(append(creativityIssues, plausibilityIssues...), logicIssues...), funIssues...),
	}
	score.Summary = fmt.Sprintf("creative %.0f / plausible %.0f / logic %.0f / fun %.0f",
		score.Creativity, score.Plausibility, score.Logic, score.Fun)
	return score
}

func scoreCreativity(setup *models.StorySetup, corpus string, engine *creativity.Engine, genericTropes []string) (float64, []string, []string) {
	score := 45.0
	var evidence, issues []string

	if engine != nil {
		if containsText(corpus, engine.Artifact) {
			score += 10
			evidence = append(evidence, "artifact appears in setup: "+engine.Artifact)
		} else {
			score -= 8
			issues = append(issues, "random engine artifact is missing from premise/rules")
		}
		if containsText(corpus, engine.Issuer) {
			score += 5
			evidence = append(evidence, "issuer is part of the world: "+engine.Issuer)
		}
		if containsText(corpus, engine.ProtagonistRole) || roleKeywordsPresent(corpus, engine.ProtagonistRole) {
			score += 5
			evidence = append(evidence, "protagonist role is tied to the engine: "+engine.ProtagonistRole)
		}
		if containsText(corpus, engine.HiddenFunction) {
			score += 5
			evidence = append(evidence, "hidden function is stated in canon")
		}
	}

	distinctStorylineTypes := make(map[string]bool)
	for _, storyline := range setup.Storylines {
		key := strings.TrimSpace(storyline.Type)
		if key == "" {
			key = strings.TrimSpace(storyline.Name)
		}
		if key != "" {
			distinctStorylineTypes[key] = true
		}
	}
	if len(distinctStorylineTypes) >= 3 {
		score += 8
		evidence = append(evidence, "three or more distinct storyline engines")
	} else if len(setup.Storylines) > 0 {
		score -= 6
		issues = append(issues, "storylines repeat the same engine instead of using different conflict types")
	}

	appealFields := countAppealFields(setup)
	if appealFields >= 6 {
		score += 7
		evidence = append(evidence, "multiple concrete signature-win fields")
	}
	diversity, repetition := bigramDiversity(corpus)
	if diversity < 0.28 {
		score -= 8
		issues = append(issues, fmt.Sprintf("low lexical diversity (%.2f)", diversity))
	} else if diversity > 0.48 {
		score += 4
	}
	if repetition > 0.16 {
		score -= 10
		issues = append(issues, fmt.Sprintf("one phrase dominates the setup (%.2f)", repetition))
	}
	if engine != nil && engine.CounterExpectation != "" && containsText(corpus, engine.CounterExpectation) {
		score += 4
		evidence = append(evidence, "counter-default expectation is present")
	}
	for _, trope := range genericTropes {
		score -= 8
		issues = append(issues, "generic trope: "+trope)
	}
	return score, evidence, issues
}

func scorePlausibility(setup *models.StorySetup, corpus string, engine *creativity.Engine, genre string) (float64, []string, []string) {
	score := 48.0
	var evidence, issues []string

	groundingMarkers := []string{
		"预算", "税", "合同", "登记", "审计", "保险", "许可", "配额", "补给",
		"维修", "账本", "法医", "公证", "仲裁", "舰队", "反应堆", "冷却剂",
		"氧气", "燃料", "引力", "虫洞", "虫族", "医院", "法院", "银行",
	}
	grounded := 0
	for _, marker := range groundingMarkers {
		if containsText(corpus, marker) {
			grounded++
		}
	}
	if grounded >= 4 {
		score += 14
		evidence = append(evidence, "setup is anchored in institutions and physical constraints")
	} else if grounded >= 2 {
		score += 7
	} else {
		score -= 10
		issues = append(issues, "few concrete institutions, budgets or physical constraints")
	}
	if len(setup.WorldResources) > 0 && len(setup.WorldTimeline) > 0 {
		score += 8
		evidence = append(evidence, "resources and timeline give the world a history")
	}
	if engine != nil {
		if containsText(corpus, engine.Artifact) && containsText(corpus, engine.Issuer) {
			score += 8
			evidence = append(evidence, "engine artifact has an issuer and a social role")
		}
		if containsText(corpus, engine.Loophole) || containsText(corpus, engine.Cost) {
			score += 4
			evidence = append(evidence, "engine loophole or cost is acknowledged")
		}
	}
	if hasRuleWithCost(setup) {
		score += 6
		evidence = append(evidence, "rules carry explicit costs or conditions")
	}

	for _, marker := range unsupportedMagicMarkers(genre) {
		if containsText(corpus, marker) {
			score -= 8
			issues = append(issues, "unsupported magic marker: "+marker)
		}
	}
	return score, evidence, issues
}

func scoreLogic(setup *models.StorySetup) (float64, []string, []string) {
	score := 48.0
	var evidence, issues []string

	if len(setup.Premises) >= 2 && len(setup.Premises) <= 6 {
		score += 7
		evidence = append(evidence, "derived systems are split into bounded premise tracks")
	} else if len(setup.Premises) == 0 {
		score -= 8
		issues = append(issues, "no derived premise systems")
	} else if len(setup.Premises) > 6 {
		score -= 5
		issues = append(issues, "too many parallel premise systems; root rule is diluted")
	}

	rulesWithConditions := 0
	for _, rule := range setup.Rules {
		if containsAny(rule, []string{"只有", "除非", "否则", "必须", "代价", "不能", "才能", "如果"}) {
			rulesWithConditions++
		}
	}
	if rulesWithConditions >= 3 {
		score += 8
		evidence = append(evidence, "rules state conditions and boundaries")
	} else if len(setup.Rules) > 0 {
		score -= 5
		issues = append(issues, "rules read as description, not enforceable boundaries")
	}

	texturedStorylines := 0
	for _, storyline := range setup.Storylines {
		if strings.TrimSpace(storyline.Desire) != "" &&
			strings.TrimSpace(storyline.Opposition) != "" &&
			strings.TrimSpace(storyline.Stakes) != "" &&
			strings.TrimSpace(storyline.Turn) != "" {
			texturedStorylines++
		}
	}
	if texturedStorylines >= 2 {
		score += 9
		evidence = append(evidence, "storylines have desire, opposition, stakes and turns")
	} else {
		score -= 6
		issues = append(issues, "storylines lack desire/opposition/stakes/turn contracts")
	}

	timelineWithImpact := 0
	for _, entry := range setup.WorldTimeline {
		if strings.TrimSpace(entry.Impact) != "" {
			timelineWithImpact++
		}
	}
	if timelineWithImpact >= 2 {
		score += 5
		evidence = append(evidence, "timeline explains why the present exists")
	} else if len(setup.WorldTimeline) > 0 {
		score -= 4
		issues = append(issues, "timeline entries do not explain their impact")
	}

	if len(setup.CoreCast) > 0 {
		castWithFunction := 0
		for _, cast := range setup.CoreCast {
			if strings.TrimSpace(cast.StoryFunction) != "" {
				castWithFunction++
			}
		}
		if castWithFunction >= 2 {
			score += 4
			evidence = append(evidence, "core cast has story functions, not just names")
		}
	}

	duplicatePremiseNames := duplicateCount(premiseNames(setup))
	if duplicatePremiseNames > 0 {
		score -= float64(duplicatePremiseNames) * 6
		issues = append(issues, "duplicate premise names")
	}
	duplicateTypes := duplicateCount(storylineTypes(setup))
	if duplicateTypes > 1 {
		score -= float64(duplicateTypes) * 3
		issues = append(issues, "repeated storyline type")
	}
	if contradiction := findSimpleContradiction(setup.Rules); contradiction != "" {
		score -= 12
		issues = append(issues, "possible rule contradiction: "+contradiction)
	}
	return score, evidence, issues
}

func scoreFun(setup *models.StorySetup) (float64, []string, []string) {
	score := 44.0
	var evidence, issues []string

	appealFields := countAppealFields(setup)
	if appealFields >= 8 {
		score += 16
		evidence = append(evidence, "multiple appeal engines")
	} else if appealFields >= 3 {
		score += 8
	} else {
		score -= 8
		issues = append(issues, "no concrete signature win or opponent misread")
	}
	signatureWins, misreads := 0, 0
	for _, storyline := range setup.Storylines {
		if storyline.AppealEngine != nil {
			if strings.TrimSpace(storyline.AppealEngine.SignatureWin) != "" {
				signatureWins++
			}
			if strings.TrimSpace(storyline.AppealEngine.OpponentMisread) != "" {
				misreads++
			}
		}
	}
	for _, premise := range setup.Premises {
		if premise.AppealEngine != nil {
			if strings.TrimSpace(premise.AppealEngine.SignatureWin) != "" {
				signatureWins++
			}
			if strings.TrimSpace(premise.AppealEngine.OpponentMisread) != "" {
				misreads++
			}
		}
	}
	if signatureWins > 0 && misreads > 0 {
		score += 8
		evidence = append(evidence, "signature wins are paired with opponent misreads")
	}
	openQuestions, turns := 0, 0
	for _, storyline := range setup.Storylines {
		if strings.TrimSpace(storyline.OpenQuestion) != "" {
			openQuestions++
		}
		if strings.TrimSpace(storyline.Turn) != "" {
			turns++
		}
	}
	if openQuestions >= 2 && turns >= 2 {
		score += 7
		evidence = append(evidence, "storylines create curiosity and reversals")
	}
	if setup.LongFormPlan != nil && len(setup.LongFormPlan.VolumePattern) > 0 {
		score += 3
	}
	if len(setup.CoreCast) >= 3 {
		roles := make(map[string]bool)
		for _, cast := range setup.CoreCast {
			if role := strings.TrimSpace(cast.Role); role != "" {
				roles[role] = true
			}
		}
		if len(roles) >= 3 {
			score += 5
			evidence = append(evidence, "cast covers distinct dramatic roles")
		}
	}
	return score, evidence, issues
}

func setupCorpus(setup *models.StorySetup) string {
	if setup == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(setup.ProjectName)
	b.WriteString("\n")
	b.WriteString(setup.Premise)
	b.WriteString("\n")
	b.WriteString(setup.Theme)
	b.WriteString("\n")
	b.WriteString(strings.Join(setup.Rules, "\n"))
	for _, storyline := range setup.Storylines {
		b.WriteString("\n")
		b.WriteString(storyline.Name)
		b.WriteString("\n")
		b.WriteString(storyline.Description)
		b.WriteString("\n")
		b.WriteString(storyline.Desire)
		b.WriteString("\n")
		b.WriteString(storyline.Opposition)
		b.WriteString("\n")
		b.WriteString(storyline.Stakes)
		b.WriteString("\n")
		b.WriteString(storyline.Turn)
		b.WriteString("\n")
		b.WriteString(storyline.Payoff)
		b.WriteString("\n")
		b.WriteString(storyline.OpenQuestion)
	}
	for _, premise := range setup.Premises {
		b.WriteString("\n")
		b.WriteString(premise.Name)
		b.WriteString("\n")
		b.WriteString(premise.Description)
	}
	for _, cast := range setup.CoreCast {
		b.WriteString("\n")
		b.WriteString(cast.Name)
		b.WriteString("\n")
		b.WriteString(cast.Role)
		b.WriteString("\n")
		b.WriteString(cast.StoryFunction)
		b.WriteString("\n")
		b.WriteString(cast.Payoff)
	}
	for _, entry := range setup.WorldTimeline {
		b.WriteString("\n")
		b.WriteString(entry.Event)
		b.WriteString("\n")
		b.WriteString(entry.Impact)
	}
	for _, resource := range setup.WorldResources {
		b.WriteString("\n")
		b.WriteString(resource.Name)
		b.WriteString("\n")
		b.WriteString(resource.Description)
	}
	return b.String()
}

func roleKeywordsPresent(corpus, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	for _, suffix := range []string{"员", "师", "官", "手", "者", "人", "长", "工", "医", "吏"} {
		if strings.HasSuffix(role, suffix) {
			stem := strings.TrimSuffix(role, suffix)
			if len([]rune(stem)) >= 2 && containsText(corpus, stem) {
				return true
			}
		}
	}
	return false
}

func countAppealFields(setup *models.StorySetup) int {
	count := 0
	countEngine := func(engine *models.AppealEngine) {
		if engine == nil {
			return
		}
		for _, value := range []string{
			engine.Appeal, engine.SurfaceLimit, engine.Exploit, engine.SignatureWin,
			engine.UpgradePath, engine.OpponentMisread, engine.RewardType,
		} {
			if strings.TrimSpace(value) != "" {
				count++
			}
		}
	}
	for _, storyline := range setup.Storylines {
		countEngine(storyline.AppealEngine)
	}
	for _, premise := range setup.Premises {
		countEngine(premise.AppealEngine)
	}
	return count
}

func hasRuleWithCost(setup *models.StorySetup) bool {
	for _, rule := range setup.Rules {
		if containsAny(rule, []string{"代价", "必须", "否则", "只有", "不能", "除非", "税", "预算", "成本"}) {
			return true
		}
	}
	return false
}

func unsupportedMagicMarkers(genre string) []string {
	switch genre {
	case "仙侠":
		return []string{"命运", "天选", "远古神秘", "作者也不知道", "纯隐喻"}
	case "科幻":
		return []string{"魔法", "神力", "诅咒", "灵魂", "远古神秘", "命运"}
	case "都市", "悬疑":
		return []string{"魔法", "神力", "诅咒", "远古神秘", "天选"}
	default:
		return []string{"魔法", "神力", "诅咒", "远古神秘", "作者也不知道"}
	}
}

func bigramDiversity(text string) (float64, float64) {
	runes := make([]rune, 0, len(text))
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			runes = append(runes, unicode.ToLower(r))
		}
	}
	if len(runes) < 4 {
		return 0, 0
	}
	total := 0
	counts := make(map[string]int)
	for i := 0; i+1 < len(runes); i++ {
		key := string(runes[i : i+2])
		counts[key]++
		total++
	}
	if total == 0 {
		return 0, 0
	}
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}
	return float64(len(counts)) / float64(total), float64(maxCount) / float64(total)
}

func premiseNames(setup *models.StorySetup) []string {
	names := make([]string, 0, len(setup.Premises))
	for _, premise := range setup.Premises {
		names = append(names, strings.TrimSpace(premise.Name))
	}
	return names
}

func storylineTypes(setup *models.StorySetup) []string {
	types := make([]string, 0, len(setup.Storylines))
	for _, storyline := range setup.Storylines {
		types = append(types, strings.TrimSpace(storyline.Type))
	}
	return types
}

func duplicateCount(values []string) int {
	counts := make(map[string]int)
	for _, value := range values {
		if value != "" {
			counts[value]++
		}
	}
	duplicates := 0
	for _, count := range counts {
		if count > 1 {
			duplicates += count - 1
		}
	}
	return duplicates
}

func findSimpleContradiction(rules []string) string {
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			a, b := rules[i], rules[j]
			for _, noun := range []string{"氧气", "记忆", "名字", "身份", "债务", "许可证", "继承权"} {
				if containsText(a, noun) && containsText(b, noun) {
					if (containsText(a, "不能") && containsText(b, "可以")) ||
						(containsText(a, "禁止") && containsText(b, "允许")) {
						return noun
					}
				}
			}
		}
	}
	return ""
}

func containsText(text, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	return strings.Contains(text, needle)
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func clampScore(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return math.Round(value*10) / 10
}
