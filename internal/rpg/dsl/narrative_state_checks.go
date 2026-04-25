package dsl

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type narrativeChapterState struct {
	ID            string
	Title         string
	Text          string
	Deltas        []StateDelta
	ElapsedDays   int
	HasTransition bool
	HasRecovery   bool
	HasCultivate  bool
	Deaths        []narrativeDeathState
	Levels        []int
	ItemMentions  map[string]int
}

type narrativeDeathState struct {
	ChapterID    string
	HasDeath     bool
	HasRevive    bool
	HasLevelDrop bool
	HasLifespan  bool
	NoLifespan   bool
	FromLevel    int
	ToLevel      int
	Evidence     []IssueEvidence
}

type narrativeInjuryState struct {
	Actor   string
	Chapter string
}

func (s *Simulator) checkNarrativeStateConsistency() {
	if s == nil || s.DSL == nil || s.DSL.Storyline == nil {
		return
	}

	chapters := s.collectNarrativeChapterStates()
	if len(chapters) == 0 {
		return
	}

	s.checkNarrativeTimeGaps(chapters)
	s.checkNarrativeResurrectionRules(chapters)
	s.checkNarrativeCultivationFlow(chapters)
	s.checkNarrativeNPCIdentity()
	s.checkNarrativeInjuryRecovery(chapters)
	s.checkNarrativeResourceBudget(chapters)
}

func (s *Simulator) collectNarrativeChapterStates() []narrativeChapterState {
	out := make([]narrativeChapterState, 0, len(s.DSL.Storyline.Chapters))
	playerName := ""
	if s.DSL.Characters != nil && s.DSL.Characters.Player != nil {
		playerName = s.DSL.Characters.Player.Name
	}

	for _, chapter := range s.DSL.Storyline.Chapters {
		text := collectChapterText(chapter)
		deltas := collectChapterStateDeltas(chapter)
		out = append(out, narrativeChapterState{
			ID:            chapter.ID,
			Title:         chapter.Title,
			Text:          text,
			Deltas:        deltas,
			ElapsedDays:   inferChapterElapsedDays(text, deltas),
			HasTransition: hasTransitionSignal(text, deltas),
			HasRecovery:   hasRecoverySignal(text, deltas),
			HasCultivate:  containsAnyNarrative(text, narrativeCultivateTerms...),
			Deaths:        extractChapterDeaths(chapter.ID, text, deltas),
			Levels:        extractChapterCultivationLevels(text, playerName, deltas),
			ItemMentions:  countChapterResourceMentions(text, deltas),
		})
	}
	return out
}

func collectChapterStateDeltas(chapter Chapter) []StateDelta {
	deltas := make([]StateDelta, 0)
	for _, obj := range chapter.Objectives {
		for _, step := range obj.Steps {
			deltas = append(deltas, step.Event.StateDeltas...)
		}
	}
	return deltas
}

func collectChapterText(chapter Chapter) string {
	var b strings.Builder
	b.WriteString(chapter.ID)
	b.WriteByte('\n')
	b.WriteString(chapter.Title)
	for _, obj := range chapter.Objectives {
		b.WriteByte('\n')
		b.WriteString(obj.Name)
		for _, step := range obj.Steps {
			b.WriteByte('\n')
			b.WriteString(step.Description)
			b.WriteByte('\n')
			b.WriteString(step.Event.Type)
			if step.Event.Trigger != nil {
				b.WriteByte('\n')
				b.WriteString(step.Event.Trigger.Condition)
			}
			if step.Event.Require != nil {
				b.WriteByte('\n')
				b.WriteString(strings.Join(step.Event.Require.Flags, " "))
				b.WriteByte('\n')
				b.WriteString(strings.Join(step.Event.Require.Items, " "))
			}
			if step.Event.OnComplete != nil {
				b.WriteByte('\n')
				b.WriteString(step.Event.OnComplete.Narration)
				b.WriteByte('\n')
				b.WriteString(step.Event.OnComplete.Result)
				b.WriteByte('\n')
				b.WriteString(strings.Join(step.Event.OnComplete.Items, " "))
				b.WriteByte('\n')
				b.WriteString(step.Event.OnComplete.SetFlag)
			}
			if step.Event.OnFail != nil {
				b.WriteByte('\n')
				b.WriteString(step.Event.OnFail.Narration)
				b.WriteByte('\n')
				b.WriteString(step.Event.OnFail.Result)
			}
			if step.Event.Dialogue != nil {
				b.WriteByte('\n')
				b.WriteString(step.Event.Dialogue.Speaker)
				b.WriteByte('\n')
				b.WriteString(step.Event.Dialogue.Text)
			}
			if step.Event.Acquire != nil {
				b.WriteByte('\n')
				b.WriteString(step.Event.Acquire.Actor)
				b.WriteByte('\n')
				b.WriteString(step.Event.Acquire.Item)
			}
			for _, delta := range step.Event.StateDeltas {
				b.WriteByte('\n')
				b.WriteString(delta.Target)
				b.WriteByte('\n')
				b.WriteString(delta.Kind)
				b.WriteByte('\n')
				b.WriteString(delta.Field)
				b.WriteByte('\n')
				b.WriteString(delta.From)
				b.WriteByte('\n')
				b.WriteString(delta.To)
				b.WriteByte('\n')
				b.WriteString(delta.Cost)
				b.WriteByte('\n')
				b.WriteString(delta.Note)
			}
		}
	}
	return b.String()
}

func chapterEvidence(chapterID, source, text string) IssueEvidence {
	return IssueEvidence{
		Chapter: chapterID,
		Source:  source,
		Text:    text,
	}
}

func stateDeltaEvidence(chapterID string, delta StateDelta) IssueEvidence {
	parts := []string{}
	if delta.Target != "" {
		parts = append(parts, "target="+delta.Target)
	}
	if delta.Kind != "" {
		parts = append(parts, "kind="+delta.Kind)
	}
	if delta.Field != "" {
		parts = append(parts, "field="+delta.Field)
	}
	if delta.From != "" {
		parts = append(parts, "from="+delta.From)
	}
	if delta.To != "" {
		parts = append(parts, "to="+delta.To)
	}
	if delta.Delta != 0 {
		parts = append(parts, fmt.Sprintf("delta=%d", delta.Delta))
	}
	if delta.Unit != "" {
		parts = append(parts, "unit="+delta.Unit)
	}
	if delta.Cost != "" {
		parts = append(parts, "cost="+delta.Cost)
	}
	if delta.Note != "" {
		parts = append(parts, "note="+delta.Note)
	}
	return chapterEvidence(chapterID, "state_delta", strings.Join(parts, " "))
}

func chapterTextEvidence(chapterID, text string) IssueEvidence {
	return chapterEvidence(chapterID, "chapter_text", firstNarrativeEvidenceLine(text))
}

func firstNarrativeEvidenceLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len([]rune(line)) >= 6 {
			return line
		}
	}
	return strings.TrimSpace(text)
}

func inferChapterElapsedDays(text string, deltas []StateDelta) int {
	maxDays := 0
	for _, delta := range deltas {
		if delta.Kind != "time" || delta.Field != "elapsed_days" {
			continue
		}
		if delta.Delta > maxDays {
			maxDays = delta.Delta
		}
	}
	if maxDays > 0 {
		return maxDays
	}
	return inferNarrativeDays(text)
}

func hasTransitionSignal(text string, deltas []StateDelta) bool {
	for _, delta := range deltas {
		if delta.Kind == "transition" && (delta.To == "true" || delta.Delta > 0) {
			return true
		}
	}
	return containsAnyNarrative(text, narrativeTransitionTerms...)
}

func hasRecoverySignal(text string, deltas []StateDelta) bool {
	for _, delta := range deltas {
		if delta.Kind == "injury" && (delta.To == "recovered" || delta.To == "acting_injured") {
			return true
		}
	}
	return containsAnyNarrative(text, narrativeRecoveryTerms...)
}

func extractChapterDeaths(chapterID, text string, deltas []StateDelta) []narrativeDeathState {
	structured := extractStructuredDeaths(chapterID, deltas)
	if len(structured) > 0 {
		return structured
	}
	return extractNarrativeDeaths(chapterID, text)
}

func extractStructuredDeaths(chapterID string, deltas []StateDelta) []narrativeDeathState {
	death := narrativeDeathState{ChapterID: chapterID}
	seen := false
	for _, delta := range deltas {
		switch delta.Kind {
		case "death":
			death.HasDeath = true
			seen = true
			death.Evidence = append(death.Evidence, stateDeltaEvidence(chapterID, delta))
		case "revive":
			death.HasRevive = true
			seen = true
			death.Evidence = append(death.Evidence, stateDeltaEvidence(chapterID, delta))
			if strings.EqualFold(delta.Cost, "none") || strings.EqualFold(delta.Cost, "no_lifespan") {
				death.NoLifespan = true
			}
		case "cultivation":
			if isCultivationLevelField(delta.Field) {
				from := parseStateDeltaLevel(delta.From)
				to := parseStateDeltaLevel(delta.To)
				if from > 0 {
					death.FromLevel = from
				}
				if to > 0 {
					death.ToLevel = to
				}
				if death.FromLevel == 0 && death.ToLevel > 0 && delta.Delta < 0 {
					death.FromLevel = death.ToLevel - delta.Delta
				}
				if death.ToLevel == 0 && death.FromLevel > 0 && delta.Delta < 0 {
					death.ToLevel = death.FromLevel + delta.Delta
				}
				if (from > 0 && to > 0 && from > to) || delta.Delta < 0 {
					death.HasLevelDrop = true
					seen = true
					death.Evidence = append(death.Evidence, stateDeltaEvidence(chapterID, delta))
				}
			}
		case "lifespan":
			seen = true
			death.Evidence = append(death.Evidence, stateDeltaEvidence(chapterID, delta))
			if delta.Delta < 0 || strings.Contains(delta.Cost, "lifespan") || strings.Contains(delta.Cost, "寿") {
				death.HasLifespan = true
			}
			if delta.Delta == 0 || strings.EqualFold(delta.Cost, "none") {
				death.NoLifespan = true
			}
		}
	}
	if !seen {
		return nil
	}
	if death.HasRevive && (death.HasLevelDrop || death.HasLifespan || death.NoLifespan) {
		death.HasDeath = true
	}
	if !death.HasLifespan && death.NoLifespan {
		death.NoLifespan = true
	} else {
		death.NoLifespan = !death.HasLifespan
	}
	return []narrativeDeathState{death}
}

func extractChapterCultivationLevels(text, playerName string, deltas []StateDelta) []int {
	levels := make([]int, 0)
	for _, delta := range deltas {
		if delta.Kind != "cultivation" || !isCultivationLevelField(delta.Field) {
			continue
		}
		if from := parseStateDeltaLevel(delta.From); from > 0 {
			levels = append(levels, from)
		}
		if to := parseStateDeltaLevel(delta.To); to > 0 {
			levels = append(levels, to)
		}
	}
	if len(levels) > 0 {
		return levels
	}
	return extractProtagonistCultivationLevels(text, playerName)
}

func isCultivationLevelField(field string) bool {
	field = strings.TrimSpace(strings.ToLower(field))
	return field == "" || field == "level" || field == "realm" || field == "cultivation" || field == "修为" || field == "境界"
}

func countChapterResourceMentions(text string, deltas []StateDelta) map[string]int {
	items := countNarrativeItemMentions(text)
	for _, delta := range deltas {
		if delta.Kind != "resource" {
			continue
		}
		key := delta.Target
		if key == "" {
			key = delta.Field
		}
		if key == "" {
			continue
		}
		items[key]++
	}
	return items
}

func (s *Simulator) checkNarrativeTimeGaps(chapters []narrativeChapterState) {
	for i := 1; i < len(chapters); i++ {
		cur := chapters[i]
		if cur.ElapsedDays < 7 || cur.HasTransition {
			continue
		}
		evidence := []IssueEvidence{chapterEvidence(cur.ID, "time", fmt.Sprintf("elapsed_days=%d has_transition=%t", cur.ElapsedDays, cur.HasTransition))}
		for _, delta := range cur.Deltas {
			if delta.Kind == "time" || delta.Kind == "transition" {
				evidence = append(evidence, stateDeltaEvidence(cur.ID, delta))
			}
		}
		s.addIssueWithEvidence(IssueContinuity, SeverityWarning, cur.ID, 0,
			fmt.Sprintf("章节时间跨度约 %d 天，但 DSL 未捕捉到明确过渡说明", cur.ElapsedDays),
			"在章节开头或上一章结尾补充过渡段，说明这段时间的修炼、生存、关系和资源变化",
			evidence)
	}
}

func (s *Simulator) checkNarrativeResurrectionRules(chapters []narrativeChapterState) {
	floorRuleIndex := s.narrativeFloorRuleIndex(chapters)
	modes := map[string]bool{}

	for i, chapter := range chapters {
		for _, death := range chapter.Deaths {
			if !death.HasDeath {
				continue
			}
			mode := "none"
			switch {
			case death.HasLevelDrop && death.HasLifespan:
				mode = "level_and_lifespan"
			case death.HasLevelDrop:
				mode = "level_only"
			case death.HasLifespan:
				mode = "lifespan_only"
			}
			modes[mode] = true

			if floorRuleIndex >= 0 && i >= floorRuleIndex && death.NoLifespan && death.FromLevel >= 2 && death.ToLevel == 1 {
				evidence := append([]IssueEvidence{}, death.Evidence...)
				evidence = append(evidence,
					chapterEvidence(death.ChapterID, "rule_observation", fmt.Sprintf("floor_rule_seen_at_index=%d from_level=%d to_level=%d has_lifespan=%t no_lifespan=%t", floorRuleIndex, death.FromLevel, death.ToLevel, death.HasLifespan, death.NoLifespan)),
				)
				s.addIssueWithEvidence(IssueLogic, SeverityCritical, death.ChapterID, 0,
					"死亡后从练气二层或更高境界跌到练气一层，但未出现寿元代价，和既有底线规则冲突",
					"补充本次免寿元代价的触发条件，或修正为扣寿元版本",
					evidence)
			}
		}
	}

	if len(modes) >= 3 {
		modeNames := mapKeys(modes)
		sort.Strings(modeNames)
		s.addIssueWithEvidence(IssueLogic, SeverityWarning, "", 0,
			"多次死亡代价模式不一致，DSL 状态机观察到修为掉级、寿元折损和无明确代价并存",
			"把复活规则写入 story setup -> RPG DSL，并在每次死亡事件中显式标出触发分支",
			[]IssueEvidence{chapterEvidence("", "death_modes", strings.Join(modeNames, ", "))})
	}
}

func (s *Simulator) narrativeFloorRuleIndex(chapters []narrativeChapterState) int {
	var worldText strings.Builder
	if s.DSL.World != nil {
		for _, rule := range s.DSL.World.Rules {
			worldText.WriteString(rule.Name)
			worldText.WriteByte('\n')
			worldText.WriteString(rule.Trigger)
			worldText.WriteByte('\n')
			worldText.WriteString(rule.Effect)
			worldText.WriteByte('\n')
		}
	}
	if containsAnyNarrative(worldText.String(), narrativeFloorRuleTerms...) {
		return 0
	}
	for i, chapter := range chapters {
		if containsAnyNarrative(chapter.Text, narrativeFloorRuleTerms...) {
			return i
		}
		for _, death := range chapter.Deaths {
			if death.HasLifespan {
				return i
			}
		}
	}
	return -1
}

func (s *Simulator) checkNarrativeCultivationFlow(chapters []narrativeChapterState) {
	prevLevel := 0
	prevChapter := ""
	for _, chapter := range chapters {
		curLevel := maxInt(chapter.Levels)
		if prevLevel > 0 && curLevel > prevLevel && !chapter.HasCultivate && !chapter.HasRecovery {
			evidence := []IssueEvidence{
				chapterEvidence(prevChapter, "previous_level", fmt.Sprintf("level=%d", prevLevel)),
				chapterEvidence(chapter.ID, "current_level", fmt.Sprintf("level=%d has_cultivate=%t has_recovery=%t", curLevel, chapter.HasCultivate, chapter.HasRecovery)),
			}
			for _, delta := range chapter.Deltas {
				if delta.Kind == "cultivation" || delta.Kind == "injury" {
					evidence = append(evidence, stateDeltaEvidence(chapter.ID, delta))
				}
			}
			s.addIssueWithEvidence(IssueGrowth, SeverityWarning, chapter.ID, 0,
				fmt.Sprintf("修为从 %s 的练气%d层提升到本章练气%d层，但 DSL 未捕捉到明确修炼或恢复过程", prevChapter, prevLevel, curLevel),
				"补充吐纳、闭关、资源消耗或突破描写，或让 chapter -> DSL 显式输出 cultivation_change",
				evidence)
		}
		if curLevel > 0 {
			prevLevel = curLevel
			prevChapter = chapter.ID
		}
	}
}

func (s *Simulator) checkNarrativeNPCIdentity() {
	if s.DSL.Characters == nil {
		return
	}

	owners := map[string]map[string]bool{}
	for _, npc := range s.DSL.Characters.NPCs {
		text := strings.Join([]string{npc.Name, npc.Role, npc.Description, npc.Appearance, npc.Background}, "\n")
		for _, key := range narrativeDescriptorKeys(text) {
			if owners[key] == nil {
				owners[key] = map[string]bool{}
			}
			owners[key][npc.Name] = true
		}
	}

	for _, chapter := range s.DSL.Storyline.Chapters {
		for _, sentence := range splitNarrativeSentences(collectChapterText(chapter)) {
			names := s.narrativeNPCNamesInText(sentence)
			if len(names) == 0 {
				continue
			}
			for _, key := range narrativeDescriptorKeys(sentence) {
				if owners[key] == nil {
					owners[key] = map[string]bool{}
				}
				for _, name := range names {
					owners[key][name] = true
				}
			}
		}
	}

	keys := make([]string, 0, len(owners))
	for key := range owners {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		names := mapKeys(owners[key])
		if len(names) < 2 {
			continue
		}
		s.addIssueWithEvidence(IssueCharacter, SeverityWarning, "", 0,
			fmt.Sprintf("NPC 身份特征 '%s' 同时关联到多个名字：%s", key, strings.Join(names, "、")),
			"如果是两个人，请在早期明确区分；如果是同一人，请统一姓名或增加 alias 规则",
			[]IssueEvidence{chapterEvidence("", "npc_identity", fmt.Sprintf("descriptor=%s names=%s", key, strings.Join(names, ", ")))})
	}
}

func (s *Simulator) checkNarrativeInjuryRecovery(chapters []narrativeChapterState) {
	openInjuries := make([]narrativeInjuryState, 0)
	for _, chapter := range chapters {
		text := chapter.Text
		nextOpen := make([]narrativeInjuryState, 0, len(openInjuries))
		for _, injury := range openInjuries {
			if structuredActingNormally(chapter.Deltas, injury.Actor) || actorActsWithoutRecovery(text, injury.Actor, chapter.HasRecovery) {
				evidence := []IssueEvidence{
					chapterEvidence(injury.Chapter, "injury_opened", injury.Actor),
					chapterEvidence(chapter.ID, "injury_followup", fmt.Sprintf("actor=%s has_recovery=%t", injury.Actor, chapter.HasRecovery)),
				}
				for _, delta := range chapter.Deltas {
					if delta.Kind == "injury" {
						evidence = append(evidence, stateDeltaEvidence(chapter.ID, delta))
					}
				}
				s.addIssueWithEvidence(IssueContinuity, SeverityWarning, chapter.ID, 0,
					fmt.Sprintf("%s 在 %s 受伤后，本章又正常行动，但 DSL 未捕捉到恢复或带伤说明", injury.Actor, injury.Chapter),
					"补充伤势恢复、带伤行动限制，或说明伤势并不严重",
					evidence)
				continue
			}
			if !chapter.HasRecovery && !structuredRecovered(chapter.Deltas, injury.Actor) {
				nextOpen = append(nextOpen, injury)
			}
		}

		openInjuries = nextOpen
		for _, actor := range structuredInjuredActors(chapter.Deltas) {
			openInjuries = append(openInjuries, narrativeInjuryState{Actor: actor, Chapter: chapter.ID})
		}
		for _, actor := range s.extractNarrativeInjuredActors(text) {
			openInjuries = append(openInjuries, narrativeInjuryState{Actor: actor, Chapter: chapter.ID})
		}
	}
}

func (s *Simulator) checkNarrativeResourceBudget(chapters []narrativeChapterState) {
	totalMentions := map[string]int{}
	consumeCount := map[string]int{}
	replenished := map[string]bool{}

	for _, chapter := range chapters {
		for _, delta := range chapter.Deltas {
			if delta.Kind != "resource" {
				continue
			}
			item := delta.Target
			if item == "" {
				item = delta.Field
			}
			if item == "" {
				continue
			}
			totalMentions[item]++
			if delta.Delta < 0 {
				consumeCount[item]++
			}
			if delta.Delta > 0 {
				replenished[item] = true
			}
		}
		for item, count := range chapter.ItemMentions {
			totalMentions[item] += count
			if containsAnyNarrative(chapter.Text, narrativeConsumeTerms...) || chapter.HasCultivate {
				consumeCount[item]++
			}
			if containsAnyNarrative(chapter.Text, narrativeReplenishTerms...) {
				replenished[item] = true
			}
		}
	}

	for item, uses := range consumeCount {
		if uses >= 3 && totalMentions[item] >= 4 && !replenished[item] {
			s.addIssue(IssueEquipment, SeverityInfo, "", 0,
				fmt.Sprintf("%s 多次作为修炼/消耗资源出现，但 DSL 未捕捉到可支撑次数或补充来源", item),
				"给资源设置数量、单次消耗和耗尽状态，避免半块资源无限使用")
		}
	}
}

func extractNarrativeDeaths(chapterID, text string) []narrativeDeathState {
	hasDeath := containsAnyNarrative(text, "死亡", "死了", "死去", "被砸死", "断气", "毙命", "殒命")
	hasRevive := containsAnyNarrative(text, "复活", "重生", "醒来", "回到")
	if !hasDeath && !hasRevive {
		return nil
	}

	death := narrativeDeathState{
		ChapterID:   chapterID,
		HasDeath:    hasDeath,
		HasRevive:   hasRevive,
		HasLifespan: hasPositiveLifespanCost(text),
		NoLifespan:  !hasPositiveLifespanCost(text),
		Evidence:    []IssueEvidence{chapterTextEvidence(chapterID, text)},
	}

	if from, to, ok := extractLevelDrop(text); ok {
		death.FromLevel = from
		death.ToLevel = to
		death.HasLevelDrop = to > 0 && from > to
	}
	if !death.HasLevelDrop && containsAnyNarrative(text, "跌到练气一层", "掉到练气一层", "降到练气一层", "跌回练气一层", "掉回练气一层") {
		death.ToLevel = 1
		death.HasLevelDrop = true
	}
	if death.HasRevive && (death.HasLevelDrop || death.HasLifespan) {
		death.HasDeath = true
	}

	return []narrativeDeathState{death}
}

func hasPositiveLifespanCost(text string) bool {
	if containsAnyNarrative(text, "未折损", "未触到", "无需折损", "没有折损", "无寿元", "不折寿", "未扣寿") {
		return false
	}
	return containsAnyNarrative(text, "寿元", "寿命", "阳寿", "折寿", "十年寿", "十年阳寿")
}

func extractLevelDrop(text string) (int, int, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`练气\s*([一二三四五六七八九十\d]+)\s*层[^。\n]{0,40}(?:跌|掉|降|退)[^。\n]{0,20}(?:练气)?\s*([一二三四五六七八九十\d]+)\s*层`),
		regexp.MustCompile(`从\s*练气\s*([一二三四五六七八九十\d]+)\s*层[^。\n]{0,40}(?:到|回到|跌到|掉到|降到)\s*(?:练气)?\s*([一二三四五六七八九十\d]+)\s*层`),
	}
	for _, re := range patterns {
		m := re.FindStringSubmatch(text)
		if len(m) == 3 {
			from := parseNarrativeLevel(m[1])
			to := parseNarrativeLevel(m[2])
			if from > 0 && to > 0 {
				return from, to, true
			}
		}
	}
	levels := extractCultivationLevels(text)
	if len(levels) >= 2 {
		first := levels[0]
		last := levels[len(levels)-1]
		if first > last && containsAnyNarrative(text, "跌", "掉", "降", "退") {
			return first, last, true
		}
	}
	return 0, 0, false
}

func extractCultivationLevels(text string) []int {
	re := regexp.MustCompile(`练气\s*([一二三四五六七八九十\d]+)\s*层`)
	matches := re.FindAllStringSubmatch(text, -1)
	levels := make([]int, 0, len(matches))
	for _, m := range matches {
		if len(m) == 2 {
			if level := parseNarrativeLevel(m[1]); level > 0 {
				levels = append(levels, level)
			}
		}
	}
	return levels
}

func extractProtagonistCultivationLevels(text, playerName string) []int {
	levels := make([]int, 0)
	for _, sentence := range splitNarrativeSentences(text) {
		levels = append(levels, extractProtagonistCultivationLevelsInSentence(sentence, playerName)...)
	}
	return levels
}

func extractProtagonistCultivationLevelsInSentence(sentence, playerName string) []int {
	re := regexp.MustCompile(`练气\s*([一二三四五六七八九十\d]+)\s*层`)
	matches := re.FindAllStringSubmatchIndex(sentence, -1)
	levels := make([]int, 0, len(matches))
	for _, m := range matches {
		if len(m) != 4 {
			continue
		}
		context := surroundingText(sentence, m[0], m[1], 18)
		if !containsAnyNarrative(context, "修为", "境界", "主角", "他", "自身", "稳在", "稳固", "跌", "掉", "降", "恢复", "突破", playerName) {
			continue
		}
		if containsAnyNarrative(context, "暗卫", "周虎", "敌", "妖兽", "修士", "对手", "灰衣人") &&
			!containsAnyNarrative(context, "林砚修为", "自身修为", "他的修为", "修为从", "掉回", "跌至", "稳固") {
			continue
		}
		level := parseNarrativeLevel(sentence[m[2]:m[3]])
		if level > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

func parseStateDeltaLevel(raw string) int {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw, "qi_")
	raw = strings.TrimPrefix(raw, "level_")
	raw = strings.TrimPrefix(raw, "cultivation_")
	raw = strings.TrimSuffix(raw, "_layer")
	raw = strings.TrimSuffix(raw, "_level")
	raw = strings.ReplaceAll(raw, "练气", "")
	raw = strings.ReplaceAll(raw, "炼气", "")
	raw = strings.ReplaceAll(raw, "层", "")
	re := regexp.MustCompile(`[一二三四五六七八九十\d]+`)
	if match := re.FindString(raw); match != "" {
		raw = match
	}
	return parseNarrativeLevel(raw)
}

func parseNarrativeLevel(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	if v, ok := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
	}[raw]; ok {
		return v
	}
	if strings.HasPrefix(raw, "十") {
		return 10 + parseNarrativeLevel(strings.TrimPrefix(raw, "十"))
	}
	if strings.HasSuffix(raw, "十") {
		return parseNarrativeLevel(strings.TrimSuffix(raw, "十")) * 10
	}
	if parts := strings.Split(raw, "十"); len(parts) == 2 {
		return parseNarrativeLevel(parts[0])*10 + parseNarrativeLevel(parts[1])
	}
	return 0
}

func inferNarrativeDays(text string) int {
	rules := []struct {
		term string
		days int
	}{
		{"半个月", 15},
		{"半月", 15},
		{"一个月", 30},
		{"数月", 60},
		{"几个月", 60},
		{"数日", 3},
		{"几日", 3},
		{"数天", 3},
		{"几天", 3},
		{"一夜", 1},
	}
	maxDays := 0
	for _, rule := range rules {
		if strings.Contains(text, rule.term) && rule.days > maxDays {
			maxDays = rule.days
		}
	}
	re := regexp.MustCompile(`([一二三四五六七八九十\d]+)\s*(天|日|个月|月|年)`)
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if len(m) != 6 {
			continue
		}
		rawNum := text[m[2]:m[3]]
		unit := text[m[4]:m[5]]
		context := surroundingText(text, m[0], m[1], 12)
		if !looksLikeElapsedTime(context) {
			continue
		}
		n := parseNarrativeLevel(rawNum)
		switch unit {
		case "天", "日":
			maxDays = maxInt([]int{maxDays, n})
		case "个月", "月":
			maxDays = maxInt([]int{maxDays, n * 30})
		case "年":
			maxDays = maxInt([]int{maxDays, n * 365})
		}
	}
	return maxDays
}

func looksLikeElapsedTime(context string) bool {
	if containsAnyNarrative(context, "寿元", "寿命", "年龄", "岁", "修为", "练气", "炼气", "编号", "代价") {
		return false
	}
	return containsAnyNarrative(context, "过去", "之后", "以前", "以来", "来", "期间", "积累", "一晃", "转眼", "已经", "连续", "相隔", "间隔")
}

func countNarrativeItemMentions(text string) map[string]int {
	items := map[string]int{}
	for _, item := range []string{"半块下品灵石", "半块灵石", "下品灵石", "灵石碎渣", "碎灵石"} {
		if count := strings.Count(text, item); count > 0 {
			items[item] += count
		}
	}
	return items
}

func (s *Simulator) narrativeNPCNamesInText(text string) []string {
	if s.DSL == nil || s.DSL.Characters == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, npc := range s.DSL.Characters.NPCs {
		for _, name := range []string{npc.Name, npc.ID} {
			name = strings.TrimSpace(name)
			if name != "" && strings.Contains(text, name) {
				seen[name] = true
			}
		}
	}
	return mapKeys(seen)
}

func (s *Simulator) extractNarrativeInjuredActors(text string) []string {
	names := s.narrativeNPCNamesInText(text)
	if s.DSL.Characters != nil && s.DSL.Characters.Player != nil && strings.Contains(text, s.DSL.Characters.Player.Name) {
		names = append(names, s.DSL.Characters.Player.Name)
	}
	seen := map[string]bool{}
	for _, sentence := range splitNarrativeSentences(text) {
		if !containsAnyNarrative(sentence, narrativeInjuryTerms...) {
			continue
		}
		for _, name := range names {
			if strings.Contains(sentence, name) {
				seen[name] = true
			}
		}
	}
	return mapKeys(seen)
}

func actorActsWithoutRecovery(text, actor string, chapterHasRecovery bool) bool {
	if chapterHasRecovery || actor == "" {
		return false
	}
	for _, sentence := range splitNarrativeSentences(text) {
		if strings.Contains(sentence, actor) && containsAnyNarrative(sentence, narrativeActiveTerms...) && !containsAnyNarrative(sentence, narrativeRecoveryTerms...) {
			return true
		}
	}
	return false
}

func structuredActingNormally(deltas []StateDelta, actor string) bool {
	for _, delta := range deltas {
		if delta.Kind == "injury" && delta.Target == actor && delta.To == "acting_normally" {
			return true
		}
	}
	return false
}

func structuredRecovered(deltas []StateDelta, actor string) bool {
	for _, delta := range deltas {
		if delta.Kind == "injury" && delta.Target == actor && (delta.To == "recovered" || delta.To == "acting_injured") {
			return true
		}
	}
	return false
}

func structuredInjuredActors(deltas []StateDelta) []string {
	seen := map[string]bool{}
	for _, delta := range deltas {
		if delta.Kind != "injury" || delta.Target == "" {
			continue
		}
		if delta.To == "injured" || delta.To == "severe" {
			seen[delta.Target] = true
		}
	}
	return mapKeys(seen)
}

func narrativeDescriptorKeys(text string) []string {
	candidates := []string{
		"缺半块左耳", "缺了半块左耳", "缺耳",
		"左脸铜钱大青色胎记", "铜钱大青色胎记", "铜钱胎记", "青色胎记",
		"左脸黑痣", "边缘黑痣", "书形标记",
	}
	out := make([]string, 0)
	for _, candidate := range candidates {
		if strings.Contains(text, candidate) {
			out = append(out, candidate)
		}
	}
	return out
}

func splitNarrativeSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '\n', '。', '！', '？', '；', ';':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func surroundingText(text string, start, end, radius int) string {
	if start < radius {
		start = 0
	} else {
		start -= radius
	}
	if end+radius > len(text) {
		end = len(text)
	} else {
		end += radius
	}
	return text[start:end]
}

func containsAnyNarrative(text string, terms ...string) bool {
	for _, term := range terms {
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func maxInt(values []int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

var narrativeFloorRuleTerms = []string{
	"练气一层是底线", "练气一层为底线", "一层是底线", "跌破后代价转移到寿元", "代价转移到寿元",
}

var narrativeTransitionTerms = []string{
	"这段时间", "这些天", "半个月来", "半月来", "数日来", "几日来", "转眼", "一晃", "过去", "期间", "之后",
}

var narrativeRecoveryTerms = []string{
	"恢复", "养伤", "疗伤", "伤势好转", "伤口愈合", "带伤", "服药", "丹药", "调息", "稳固",
}

var narrativeCultivateTerms = []string{
	"修炼", "吐纳", "打坐", "闭关", "突破", "稳在", "稳固", "炼化", "吸收灵气", "运转功法",
}

var narrativeInjuryTerms = []string{
	"受伤", "重伤", "砸伤", "断骨", "吐血", "哀嚎", "伤口", "血肉模糊",
}

var narrativeActiveTerms = []string{
	"干活", "挖矿", "正常", "照常", "继续", "行走", "带路", "奔跑", "站起",
}

var narrativeConsumeTerms = []string{
	"消耗", "耗尽", "用掉", "炼化", "吐纳", "修炼", "吸收",
}

var narrativeReplenishTerms = []string{
	"获得", "补充", "买到", "分到", "捡到", "又得", "新增",
}
