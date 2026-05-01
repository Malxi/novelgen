package dsl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// NovelgenAdapter converts novelgen project data to DSL
type NovelgenAdapter struct {
	project *rpg.NovelgenProject
	logger  *Logger
}

// NewNovelgenAdapter creates a new adapter
func NewNovelgenAdapter(project *rpg.NovelgenProject, logger *Logger) *NovelgenAdapter {
	if logger == nil {
		logger = NewConsoleLogger(WithMinLevel(LogLevelInfo))
	}
	return &NovelgenAdapter{
		project: project,
		logger:  logger,
	}
}

// ToDSL converts the novelgen project to DSL
func (na *NovelgenAdapter) ToDSL(phase MergePhase) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Converting novelgen project to DSL",
		map[string]interface{}{
			"book":  na.project.BookName,
			"phase": phase,
		})

	dsl := &DSL{
		Metadata:   &Metadata{},
		World:      &World{},
		Characters: &Characters{},
		Storyline:  &Storyline{},
		Systems:    &Systems{},
	}

	// Set metadata
	dsl.Metadata.Title = na.project.BookName
	dsl.Metadata.DSLVersion = "0.2.0"
	dsl.Metadata.Phase = string(phase)

	switch phase {
	case PhaseSetup:
		return na.toSetupDSL(dsl)
	case PhaseOutline:
		return na.toOutlineDSL(dsl)
	case PhaseCraft:
		return na.toCraftDSL(dsl)
	default:
		return nil, fmt.Errorf("unsupported phase: %s", phase)
	}
}

// toSetupDSL creates setup phase DSL (metadata + numeric systems baseline)
func (na *NovelgenAdapter) toSetupDSL(dsl *DSL) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Generating setup DSL")

	setup := na.loadStorySetup()

	// Metadata baseline.
	if setup != nil {
		if strings.TrimSpace(setup.ProjectName) != "" {
			dsl.Metadata.Title = setup.ProjectName
		}
		dsl.Metadata.Genre = append([]string(nil), setup.Genres...)
		dsl.Metadata.Tone = setup.Tone
		dsl.Metadata.PowerSystem = na.inferPowerSystem(setup)
	} else {
		dsl.Metadata.PowerSystem = "default_progression_system"
	}

	// Baseline player shell for simulators.
	dsl.Characters.Player = na.inferBasePlayer(setup)

	// Setup-level world rules and resources are the canonical numerical
	// constraints that chapter DSL can later spend, violate, or reference.
	dsl.World.Rules = buildRulesFromSetup(setup)
	dsl.World.Items = buildWorldResourceItems(setup)

	// Numeric systems.
	dsl.Systems.AttributeSystem = na.buildAttributeSystem(setup)
	dsl.Systems.PowerFormula = na.buildPowerFormula(dsl.Systems.AttributeSystem)
	dsl.Systems.ProgressionSystems = na.buildProgressionSystems(setup)
	dsl.Systems.Counters = na.buildCounterSystems(setup)
	(&ModelAdapter{setup: setup}).buildStorylineContracts(dsl)
	(&ModelAdapter{setup: setup}).buildSetupContractChapters(dsl)

	return dsl, nil
}

// toOutlineDSL creates outline phase DSL (basic framework)
func (na *NovelgenAdapter) toOutlineDSL(dsl *DSL) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Generating outline DSL")

	setup := na.loadStorySetup()
	if setup != nil {
		if strings.TrimSpace(setup.ProjectName) != "" {
			dsl.Metadata.Title = setup.ProjectName
		}
		dsl.Metadata.Genre = append([]string(nil), setup.Genres...)
		dsl.Metadata.Tone = setup.Tone
		dsl.Metadata.PowerSystem = na.inferPowerSystem(setup)
		dsl.World.Rules = buildRulesFromSetup(setup)
		dsl.World.Items = buildWorldResourceItems(setup)
		dsl.Systems.AttributeSystem = na.buildAttributeSystem(setup)
		dsl.Systems.PowerFormula = na.buildPowerFormula(dsl.Systems.AttributeSystem)
		dsl.Systems.ProgressionSystems = na.buildProgressionSystems(setup)
		dsl.Systems.Counters = na.buildCounterSystems(setup)
		(&ModelAdapter{setup: setup}).buildStorylineContracts(dsl)
	}

	// Outline DSL should still carry enough setup-derived protagonist data for
	// simulation before craft has produced rich character sheets.
	dsl.Characters.Player = na.inferBasePlayer(setup)

	// Convert characters (placeholders only)
	for name, char := range na.project.Characters {
		if isProtagonistRole(char.RoleInStory) {
			continue
		}
		npc := NPC{
			ID:                sanitizeID(name),
			Name:              name,
			Role:              char.RoleInStory,
			IsPlaceholder:     true,
			PlaceholderSource: "outline",
		}
		dsl.Characters.NPCs = append(dsl.Characters.NPCs, npc)
	}
	na.addOutlineCharacterPlaceholders(dsl)

	// Convert locations (placeholders only)
	for name, loc := range na.project.Locations {
		location := Location{
			ID:                sanitizeID(name),
			Name:              name,
			Type:              inferMapTypeFromName(name),
			IsPlaceholder:     true,
			PlaceholderSource: "outline",
		}

		// Add connections
		for _, connected := range loc.ConnectedLocs {
			location.Connections = append(location.Connections, Connection{
				To: sanitizeID(connected),
			})
		}

		dsl.World.Locations = append(dsl.World.Locations, location)
	}

	// Convert outline to storyline
	if err := na.convertOutlineToStoryline(dsl); err != nil {
		return nil, err
	}

	return dsl, nil
}

func (na *NovelgenAdapter) addOutlineCharacterPlaceholders(dsl *DSL) {
	if dsl == nil || dsl.Characters == nil {
		return
	}
	seen := map[string]bool{}
	if dsl.Characters.Player != nil {
		seen[strings.TrimSpace(dsl.Characters.Player.Name)] = true
		seen[strings.TrimSpace(dsl.Characters.Player.ID)] = true
	}
	for _, npc := range dsl.Characters.NPCs {
		seen[strings.TrimSpace(npc.Name)] = true
		seen[strings.TrimSpace(npc.ID)] = true
	}

	for _, part := range na.project.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, rawName := range chapter.Characters {
					name := canonicalCharacterName(rawName)
					if name == "" || seen[name] || isGenericProtagonistName(name) {
						continue
					}
					id := coalesceID(sanitizeID(name), fmt.Sprintf("npc_%02d", len(dsl.Characters.NPCs)+1))
					if seen[id] {
						continue
					}
					dsl.Characters.NPCs = append(dsl.Characters.NPCs, NPC{
						ID:                id,
						Name:              name,
						Role:              "supporting",
						IsPlaceholder:     true,
						PlaceholderSource: "outline",
					})
					seen[name] = true
					seen[id] = true
				}
			}
		}
	}
}

func (na *NovelgenAdapter) loadStorySetup() *models.StorySetup {
	candidates := []string{
		filepath.Join(na.project.ProjectPath, "books", na.project.BookName, "story", "setup", "story_setup.json"),
		filepath.Join(na.project.ProjectPath, "books", na.project.BookName, "story", "setup.json"),
	}
	var lastErr error
	for _, setupPath := range candidates {
		setup, err := models.LoadStorySetup(setupPath)
		if err == nil {
			return setup
		}
		lastErr = err
	}

	na.logger.Warn(LogCategorySystem, "Failed to load story setup, using fallback",
		map[string]interface{}{"paths": candidates, "error": lastErr.Error()})
	return nil
}

func buildRulesFromSetup(setup *models.StorySetup) []Rule {
	if setup == nil {
		return nil
	}
	rules := make([]Rule, 0, len(setup.Rules))
	for i, text := range setup.Rules {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		rules = append(rules, Rule{
			Name:    fmt.Sprintf("setup_rule_%02d", i+1),
			Trigger: "always",
			Effect:  text,
		})
	}
	rules = append(rules, buildStructuredSetupRules(setup)...)
	return rules
}

func buildStructuredSetupRules(setup *models.StorySetup) []Rule {
	if setup == nil {
		return nil
	}

	rules := make([]Rule, 0, len(setup.WorldResources)+len(setup.Premises)*2+len(setup.Storylines))
	for i, res := range setup.WorldResources {
		name := strings.TrimSpace(res.Name)
		if name == "" {
			name = fmt.Sprintf("resource_%02d", i+1)
		}
		id := setupResourceID(name, i)
		effect := fmt.Sprintf("resource=%s; scarcity=%s; category=%s", id, strings.TrimSpace(res.Scarcity), strings.TrimSpace(res.Category))
		rules = append(rules, Rule{
			Name:    "resource_scarcity_" + id,
			Trigger: "resource.scarcity",
			Effect:  effect,
		})
	}

	for i, premise := range setup.Premises {
		systemID := coalesceID(sanitizeID(strings.TrimSpace(premise.Name)), fmt.Sprintf("premise_%02d", i+1))
		maxLevel := 0
		for _, stage := range premise.Progression {
			if stage.Level > maxLevel {
				maxLevel = stage.Level
			}
			req := strings.TrimSpace(stage.Requirements)
			if req == "" {
				continue
			}
			rules = append(rules, Rule{
				Name:    fmt.Sprintf("progression_requirement_%s_L%d", systemID, stage.Level),
				Trigger: "progression.requirement",
				Effect:  fmt.Sprintf("system=%s; level=%d; requires=%s", systemID, stage.Level, req),
			})
		}
		if maxLevel > 0 {
			rules = append(rules, Rule{
				Name:    "progression_max_" + systemID,
				Trigger: "progression.max_level",
				Effect:  fmt.Sprintf("system=%s; max_level=%d", systemID, maxLevel),
			})
		}
	}

	for i, storyline := range setup.Storylines {
		name := strings.TrimSpace(storyline.Name)
		if name == "" {
			name = fmt.Sprintf("storyline_%02d", i+1)
		}
		id := coalesceID(sanitizeID(name), fmt.Sprintf("storyline_%02d", i+1))
		effectParts := []string{
			"id=" + id,
			"name=" + sanitizeRuleValue(name),
			"type=" + sanitizeRuleValue(storyline.Type),
			fmt.Sprintf("importance=%d", storyline.Importance),
		}
		effectParts = appendStorylineRuleFields(effectParts, storyline)
		if len(storyline.PressurePoints) > 0 {
			effectParts = append(effectParts, "pressure_points="+sanitizeRuleValue(strings.Join(storyline.PressurePoints, " | ")))
		}
		rules = append(rules, Rule{
			Name:    "storyline_contract_" + id,
			Trigger: "storyline.contract",
			Effect:  strings.Join(effectParts, "; "),
		})
	}
	return rules
}

func appendStorylineRuleFields(effectParts []string, storyline models.Storyline) []string {
	ordered := []struct {
		key   string
		value string
	}{
		{"scope", storyline.Scope},
		{"payoff_style", storyline.PayoffStyle},
		{"setup_role", storyline.SetupRole},
		{"desire", storyline.Desire},
		{"opposition", storyline.Opposition},
		{"stakes", storyline.Stakes},
		{"turn", storyline.Turn},
		{"payoff", storyline.Payoff},
		{"open_question", storyline.OpenQuestion},
	}
	for _, field := range ordered {
		if strings.TrimSpace(field.value) != "" {
			effectParts = append(effectParts, field.key+"="+sanitizeRuleValue(field.value))
		}
	}
	return effectParts
}

func sanitizeRuleValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ";", "，")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func buildWorldResourceItems(setup *models.StorySetup) []Item {
	if setup == nil {
		return nil
	}
	items := make([]Item, 0, len(setup.WorldResources))
	for i, res := range setup.WorldResources {
		name := strings.TrimSpace(res.Name)
		if name == "" {
			name = fmt.Sprintf("resource_%02d", i+1)
		}
		items = append(items, Item{
			ID:          setupResourceID(name, i),
			Name:        name,
			Type:        coalesceString(strings.TrimSpace(res.Category), "resource"),
			Rarity:      inferRarityFromScarcity(res.Scarcity),
			Description: strings.TrimSpace(res.Description),
		})
	}
	return items
}

func appendResourceAttributes(attrs []AttributeDef, setup *models.StorySetup) []AttributeDef {
	if setup == nil {
		return attrs
	}
	seen := make(map[string]bool, len(attrs))
	for _, attr := range attrs {
		seen[attr.ID] = true
	}
	for i, res := range setup.WorldResources {
		name := strings.TrimSpace(res.Name)
		if name == "" {
			name = fmt.Sprintf("resource_%02d", i+1)
		}
		id := setupResourceID(name, i)
		if seen[id] {
			continue
		}
		seen[id] = true
		attrs = append(attrs, AttributeDef{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(res.Description),
			Type:        "resource",
			BaseValue:   0,
			MinValue:    0,
			MaxValue:    999999,
			IsResource:  true,
		})
	}
	return attrs
}

func setupResourceID(name string, index int) string {
	return fmt.Sprintf("resource_%02d", index+1)
}

func inferRarityFromScarcity(scarcity string) string {
	text := strings.ToLower(strings.TrimSpace(scarcity))
	switch {
	case text == "":
		return ""
	case strings.Contains(text, "unique"), strings.Contains(text, "唯一"), strings.Contains(text, "独一"):
		return "legendary"
	case strings.Contains(text, "极"), strings.Contains(text, "稀"), strings.Contains(text, "rare"):
		return "rare"
	case strings.Contains(text, "common"), strings.Contains(text, "常见"):
		return "common"
	default:
		return text
	}
}

func coalesceID(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func coalesceString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func (na *NovelgenAdapter) inferPowerSystem(setup *models.StorySetup) string {
	if setup == nil {
		return "default_progression_system"
	}
	parts := make([]string, 0, len(setup.Premises))
	for _, p := range setup.Premises {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		for _, r := range setup.Rules {
			r = strings.TrimSpace(r)
			if r != "" {
				parts = append(parts, r)
			}
			if len(parts) >= 3 {
				break
			}
		}
	}
	if len(parts) == 0 {
		return "default_progression_system"
	}
	return strings.Join(parts, " + ")
}

func (na *NovelgenAdapter) inferBasePlayer(setup *models.StorySetup) *Player {
	player := inferPlayerFromSetup(setup)

	// Prefer explicitly tagged protagonist in craft.
	for name, c := range na.project.Characters {
		if isProtagonistRole(c.RoleInStory) {
			craftPlayer := playerFromNovelgenCharacter(name, c)
			if player == nil {
				return craftPlayer
			}
			if !isGenericProtagonistName(craftPlayer.Name) {
				player.ID = craftPlayer.ID
				player.Name = craftPlayer.Name
			}
			player.Stats = craftPlayer.Stats
			if len(craftPlayer.Skills) > 0 {
				player.Skills = craftPlayer.Skills
			}
			if len(craftPlayer.Abilities) > 0 && len(player.Abilities) == 0 {
				player.Abilities = craftPlayer.Abilities
			}
			if len(craftPlayer.Affiliations) > 0 {
				player.Affiliations = craftPlayer.Affiliations
			}
			return player
		}
	}

	// Fallback: first character by sorted key for deterministic behavior.
	if player == nil && len(na.project.Characters) > 0 {
		keys := make([]string, 0, len(na.project.Characters))
		for k := range na.project.Characters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		k := keys[0]
		c := na.project.Characters[k]
		return playerFromNovelgenCharacter(k, c)
	}
	if player != nil {
		return player
	}

	return &Player{
		ID:    "char_player",
		Name:  "主角",
		Class: "adventurer",
		Stats: Stats{STR: 10, AGI: 10, INT: 10, VIT: 10, HP: 100, MP: 50},
	}
}

func playerFromNovelgenCharacter(id string, char rpg.NovelgenCharacter) *Player {
	name := strings.TrimSpace(char.Name)
	if name == "" {
		name = id
	}
	return &Player{
		ID:           coalesceID(sanitizeID(id), "protagonist"),
		Name:         name,
		Description:  char.Background,
		Age:          parseAge(char.Age),
		Gender:       char.Gender,
		Race:         char.Race,
		Background:   char.Background,
		Personality:  char.Personality,
		Motivation:   char.Motivation,
		Abilities:    char.Abilities,
		Affiliations: char.Affiliations,
		RoleInStory:  coalesceString(char.RoleInStory, "protagonist"),
		Voice:        char.Voice,
		Class:        inferClassFromCharacter(char),
		Skills:       char.Skills,
		Stats:        inferStatsFromCharacter(char),
	}
}

func inferPlayerFromSetup(setup *models.StorySetup) *Player {
	if setup == nil {
		return nil
	}
	name := inferProtagonistNameFromSetup(setup)
	if name == "" {
		name = "主角"
	}
	background := firstSentences(setup.Premise, 2)
	motivation := inferPrimaryMotivation(setup)
	personality := inferPersonalityFromSetup(setup)
	class := "adventurer"
	if setupMentions(setup, "机甲") {
		class = "mecha_pilot"
	}

	return &Player{
		ID:          coalesceID(sanitizeID(name), "protagonist"),
		Name:        name,
		Description: background,
		Background:  background,
		Personality: personality,
		Motivation:  motivation,
		RoleInStory: "protagonist",
		Class:       class,
		Skills:      inferPlayerSkillsFromSetup(setup),
		Abilities:   inferPlayerAbilitiesFromSetup(setup),
		Stats: Stats{
			STR: 12,
			AGI: 11,
			INT: 14,
			VIT: 12,
			HP:  120,
			MP:  60,
		},
	}
}

func inferProtagonistNameFromSetup(setup *models.StorySetup) string {
	text := setup.Premise + "\n"
	for _, storyline := range setup.Storylines {
		text += storyline.Description + "\n" + storyline.Desire + "\n"
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:主角|研发工程师|工程师|研发人员|适配者)([\p{Han}]{2,4})`),
		regexp.MustCompile(`([\p{Han}]{2,4})要`),
		regexp.MustCompile(`([\p{Han}]{2,4})在`),
	}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 && isLikelyCharacterName(match[1]) {
				return match[1]
			}
		}
	}
	return ""
}

func isLikelyCharacterName(value string) bool {
	value = strings.TrimSpace(value)
	if len([]rune(value)) < 2 || len([]rune(value)) > 4 {
		return false
	}
	noise := []string{"公元", "人类", "虫族", "沈氏", "主角", "火种", "机甲", "基因", "文明", "地球", "据点", "故事"}
	for _, word := range noise {
		if value == word || strings.Contains(value, word) {
			return false
		}
	}
	return true
}

func inferPrimaryMotivation(setup *models.StorySetup) string {
	best := ""
	bestImportance := -1
	for _, storyline := range setup.Storylines {
		desire := strings.TrimSpace(storyline.Desire)
		if desire == "" {
			continue
		}
		if storyline.Importance > bestImportance {
			bestImportance = storyline.Importance
			best = desire
		}
	}
	if best != "" {
		return best
	}
	return firstSentences(setup.Theme, 1)
}

func inferPersonalityFromSetup(setup *models.StorySetup) []string {
	re := regexp.MustCompile(`性格([^，。；;]+)`)
	if match := re.FindStringSubmatch(setup.Premise); len(match) > 1 {
		traits := splitChineseList(match[1])
		if len(traits) > 0 {
			return traits
		}
	}
	return []string{"沉稳务实", "重情重义", "求生意志强"}
}

func inferPlayerSkillsFromSetup(setup *models.StorySetup) []string {
	skills := []string{}
	if setupMentions(setup, "机甲") {
		skills = append(skills, "火种机甲同步")
	}
	if setupMentions(setup, "基因") {
		skills = append(skills, "基因适配控制")
	}
	if setupMentions(setup, "数据库") || setupMentions(setup, "蓝图") {
		skills = append(skills, "旧文明技术解码")
	}
	if len(skills) == 0 {
		skills = append(skills, "快速学习")
	}
	return skills
}

func inferPlayerAbilitiesFromSetup(setup *models.StorySetup) []string {
	abilities := []string{}
	for _, premise := range setup.Premises {
		text := premise.Name + premise.Category + premise.Description
		if strings.Contains(text, "主角") || strings.Contains(text, "林野") || strings.Contains(text, "机甲") || strings.Contains(text, "基因") {
			abilities = append(abilities, strings.TrimSpace(premise.Name))
		}
		if len(abilities) >= 4 {
			break
		}
	}
	return abilities
}

func setupMentions(setup *models.StorySetup, needle string) bool {
	if strings.Contains(setup.Premise, needle) || strings.Contains(setup.Theme, needle) {
		return true
	}
	for _, rule := range setup.Rules {
		if strings.Contains(rule, needle) {
			return true
		}
	}
	for _, premise := range setup.Premises {
		if strings.Contains(premise.Name, needle) || strings.Contains(premise.Category, needle) || strings.Contains(premise.Description, needle) {
			return true
		}
	}
	return false
}

func firstSentences(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" || max <= 0 {
		return text
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '。' || r == '.' || r == '！' || r == '!'
	})
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
		if len(out) >= max {
			break
		}
	}
	if len(out) == 0 {
		return text
	}
	return strings.Join(out, "。") + "。"
}

func splitChineseList(text string) []string {
	raw := strings.FieldsFunc(text, func(r rune) bool {
		return r == '、' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	var out []string
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func canonicalCharacterName(raw string) string {
	name := strings.TrimSpace(raw)
	for _, marker := range []string{"（", "(", "【", "["} {
		if idx := strings.Index(name, marker); idx > 0 {
			name = strings.TrimSpace(name[:idx])
		}
	}
	return name
}

func isProtagonistRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "protagonist" || role == "主角" || strings.Contains(role, "主角")
}

func isGenericProtagonistName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "" || name == "主角" || name == "protagonist" || name == "player"
}

func (na *NovelgenAdapter) buildAttributeSystem(setup *models.StorySetup) *AttributeSystem {
	sys := &AttributeSystem{
		ID:          "attr_core",
		Name:        "Core Attributes",
		Description: "Numeric baseline attributes for simulation and continuity checks.",
		Attributes: []AttributeDef{
			{ID: "str", Name: "Strength", Type: "stat", BaseValue: 10, MinValue: 0, MaxValue: 999, IsResource: false},
			{ID: "agi", Name: "Agility", Type: "stat", BaseValue: 10, MinValue: 0, MaxValue: 999, IsResource: false},
			{ID: "int", Name: "Intelligence", Type: "stat", BaseValue: 10, MinValue: 0, MaxValue: 999, IsResource: false},
			{ID: "vit", Name: "Vitality", Type: "stat", BaseValue: 10, MinValue: 0, MaxValue: 999, IsResource: false},
			{ID: "hp", Name: "HP", Type: "resource", BaseValue: 100, MinValue: 0, MaxValue: 999999, IsResource: true},
			{ID: "mp", Name: "MP", Type: "resource", BaseValue: 50, MinValue: 0, MaxValue: 999999, IsResource: true},
		},
	}

	// Add an extra custom resource from setup premise keywords when available.
	if setup != nil {
		resourceID := ""
		resourceName := ""
		for _, p := range setup.Premises {
			text := strings.ToLower(strings.TrimSpace(p.Name + " " + p.Description + " " + p.Category))
			switch {
			case strings.Contains(text, "修"), strings.Contains(text, "cultivation"):
				resourceID, resourceName = "qi", "Qi"
			case strings.Contains(text, "mana"), strings.Contains(text, "magic"), strings.Contains(text, "魔"):
				resourceID, resourceName = "mana", "Mana"
			case strings.Contains(text, "gene"), strings.Contains(text, "基因"):
				resourceID, resourceName = "gene", "Gene Potential"
			case strings.Contains(text, "mech"), strings.Contains(text, "机甲"):
				resourceID, resourceName = "sync", "Sync"
			}
			if resourceID != "" {
				break
			}
		}
		if resourceID != "" {
			sys.Attributes = append(sys.Attributes, AttributeDef{
				ID: resourceID, Name: resourceName, Type: "resource",
				BaseValue: 0, MinValue: 0, MaxValue: 999999, IsResource: true,
			})
		}
	}

	sys.Attributes = appendResourceAttributes(sys.Attributes, setup)
	return sys
}

func (na *NovelgenAdapter) buildPowerFormula(attrSys *AttributeSystem) *PowerFormula {
	factors := []Factor{
		{Attribute: "str", Name: "Strength", Weight: 2.0},
		{Attribute: "agi", Name: "Agility", Weight: 1.2},
		{Attribute: "int", Name: "Intelligence", Weight: 1.8},
		{Attribute: "vit", Name: "Vitality", Weight: 1.5},
		{Attribute: "hp", Name: "HP", Weight: 0.1},
		{Attribute: "mp", Name: "MP", Weight: 0.2},
	}

	// Include one extra custom factor if present.
	if attrSys != nil {
		for _, a := range attrSys.Attributes {
			if a.ID != "str" && a.ID != "agi" && a.ID != "int" && a.ID != "vit" && a.ID != "hp" && a.ID != "mp" {
				factors = append(factors, Factor{Attribute: a.ID, Name: a.Name, Weight: 1.0})
				break
			}
		}
	}

	return &PowerFormula{
		ID:          "power_core",
		Name:        "Core Power Formula",
		Description: "Deterministic combat power formula for benchmark simulation.",
		Formula:     "base + sum(attr_i * weight_i)",
		BasePower:   10,
		Factors:     factors,
	}
}

func (na *NovelgenAdapter) buildProgressionSystems(setup *models.StorySetup) []ProgressionSystem {
	if setup == nil || len(setup.Premises) == 0 {
		return []ProgressionSystem{
			{
				ID:          "default_progression",
				Name:        "Default Progression",
				Description: "Fallback progression system.",
				Levels: []ProgressionLevel{
					{Level: 1, Name: "Novice", Requirements: "start", Bonuses: []string{"+1 str", "+1 vit"}},
					{Level: 2, Name: "Adept", Requirements: "exp>=100", Bonuses: []string{"+2 str", "+2 hp"}},
					{Level: 3, Name: "Expert", Requirements: "exp>=300", Bonuses: []string{"+3 int", "+3 hp"}},
				},
			},
		}
	}

	out := make([]ProgressionSystem, 0, len(setup.Premises))
	for i, p := range setup.Premises {
		id := sanitizeID(strings.TrimSpace(p.Name))
		if id == "" {
			id = fmt.Sprintf("premise_%d", i+1)
		}
		ps := ProgressionSystem{
			ID:          id,
			Name:        p.Name,
			Description: p.Description,
			Levels:      make([]ProgressionLevel, 0, len(p.Progression)),
		}
		for _, st := range p.Progression {
			levelName := strings.TrimSpace(st.Name)
			if levelName == "" {
				levelName = fmt.Sprintf("L%d", st.Level)
			}
			req := strings.TrimSpace(st.Requirements)
			if req == "" {
				req = fmt.Sprintf("reach level %d", st.Level)
			}
			bonuses := []string{}
			if strings.TrimSpace(st.Description) != "" {
				bonuses = append(bonuses, st.Description)
			}
			ps.Levels = append(ps.Levels, ProgressionLevel{
				Level:        st.Level,
				Name:         levelName,
				Requirements: req,
				Bonuses:      bonuses,
			})
		}
		// Keep at least one level for parser/simulator stability.
		if len(ps.Levels) == 0 {
			ps.Levels = append(ps.Levels, ProgressionLevel{
				Level:        1,
				Name:         "Initial",
				Requirements: "start",
				Bonuses:      []string{"baseline"},
			})
		}
		out = append(out, ps)
	}
	return out
}

func (na *NovelgenAdapter) buildCounterSystems(setup *models.StorySetup) []CounterSystem {
	counters := []CounterSystem{
		{
			Name:        "chapter_progress",
			Track:       "story.chapter",
			Description: "Story progression counter",
			Milestones: []CounterMilestone{
				{Value: 5, Reward: Reward{Title: "Arc Warmup", Description: "Reached chapter 5"}},
				{Value: 10, Reward: Reward{Title: "Arc Core", Description: "Reached chapter 10"}},
			},
		},
	}

	// Add premise-driven counter for numeric pacing.
	if setup != nil && len(setup.Premises) > 0 {
		p := setup.Premises[0]
		counters = append(counters, CounterSystem{
			Name:        sanitizeID(p.Name) + "_progress",
			Track:       "player.progression",
			Description: "Primary progression pace tracker",
			Milestones: []CounterMilestone{
				{Value: 1, Reward: Reward{Title: "System Activated", Description: p.Name}},
				{Value: 3, Reward: Reward{Title: "First Breakthrough", Description: "Core growth checkpoint"}},
				{Value: 5, Reward: Reward{Title: "Mid-Arc Growth", Description: "Stable advancement"}},
			},
		})
	}
	return counters
}

// toCraftDSL creates craft phase DSL (detailed information)
func (na *NovelgenAdapter) toCraftDSL(dsl *DSL) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Generating craft DSL")

	// Convert characters with full details
	for name, char := range na.project.Characters {
		player := na.convertNovelgenCharacterToPlayer(name, char)

		// Only set player if it's the protagonist
		if char.RoleInStory == "主角" || char.RoleInStory == "protagonist" {
			dsl.Characters.Player = player
			// Don't add protagonist as NPC
			continue
		}

		// Otherwise add as NPC
		npc := na.convertPlayerToNPC(player)
		npc.IsPlaceholder = false
		dsl.Characters.NPCs = append(dsl.Characters.NPCs, npc)
	}

	// Convert locations with full details
	for name, loc := range na.project.Locations {
		location := na.convertNovelgenLocationToLocation(name, loc)
		dsl.World.Locations = append(dsl.World.Locations, location)
	}

	// Convert items
	for name, item := range na.project.Items {
		rpgItem := na.convertNovelgenItemToItem(name, item)
		dsl.World.Items = append(dsl.World.Items, rpgItem)
	}

	return dsl, nil
}

// convertOutlineToStoryline converts the story outline to DSL storyline
func (na *NovelgenAdapter) convertOutlineToStoryline(dsl *DSL) error {
	outline := na.project.Outline

	// Convert parts/volumes/chapters
	partNum := 1
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if strings.TrimSpace(volume.ID) != "" {
				dsl.Storyline.Arcs = append(dsl.Storyline.Arcs, Arc{
					ID:       volume.ID,
					Name:     volume.Title,
					Position: len(dsl.Storyline.Arcs) + 1,
				})
			}
			for _, chapter := range volume.Chapters {
				dslChapter := Chapter{
					ID:       chapter.ID,
					Title:    chapter.Title,
					Arc:      volume.ID,
					Position: partNum,
				}

				// Convert objectives from chapter content
				objective := Objective{
					ID:   fmt.Sprintf("obj_%s", chapter.ID),
					Name: chapter.Title,
					Type: "sequence",
				}

				// Convert events to steps
				stepNum := 1
				for _, advance := range chapter.StorylineAdvances {
					objective.Steps = append(objective.Steps, Step{
						Order:       stepNum,
						Description: storyStorylineAdvanceDescription(advance),
						Event: Event{
							Type:        "storyline",
							StateDeltas: buildStoryStorylineAdvanceDeltas(chapter.ID, advance),
						},
					})
					stepNum++
				}

				for _, step := range buildStoryMysterySteps(chapter.ID, chapter.Mysteries) {
					step.Order = stepNum
					objective.Steps = append(objective.Steps, step)
					stepNum++
				}

				for _, delta := range buildStoryStateAnchorDeltas(chapter) {
					objective.Steps = append(objective.Steps, Step{
						Order:       stepNum,
						Description: delta.Note,
						Event: Event{
							Type:        "status",
							StateDeltas: []StateDelta{delta},
						},
					})
					stepNum++
				}

				na.addStoryOutlineEnemies(dsl, chapter.Enemies)
				for _, event := range chapter.Events {
					step := na.convertEventToStep(event, chapter.Enemies, stepNum)
					if step != nil {
						objective.Steps = append(objective.Steps, *step)
						stepNum++
					}
				}

				for _, entry := range chapter.ResourceLedger {
					objective.Steps = append(objective.Steps, Step{
						Order:       stepNum,
						Description: fmt.Sprintf("%s resource change: %d %+d = %d. %s", entry.Item, entry.Start, entry.Delta, entry.End, entry.Reason),
						Event: Event{
							Type: "resource",
							StateDeltas: []StateDelta{{
								Target: entry.Item,
								Kind:   "resource",
								Field:  "quantity",
								From:   fmt.Sprintf("%d", entry.Start),
								To:     fmt.Sprintf("%d", entry.End),
								Delta:  entry.Delta,
								Note:   entry.Reason,
							}},
						},
					})
					stepNum++
				}

				if chapter.Timeline.TimeJump || strings.TrimSpace(chapter.Timeline.Transition) != "" {
					objective.Steps = append(objective.Steps, Step{
						Order:       stepNum,
						Description: strings.TrimSpace(chapter.Timeline.Transition),
						Event: Event{
							Type: "transition",
							StateDeltas: []StateDelta{{
								Kind:  "transition",
								Field: "time_jump",
								To:    "true",
								Note:  strings.TrimSpace(chapter.Timeline.PreviousGap + " " + chapter.Timeline.Transition),
							}},
						},
					})
					stepNum++
				}

				if len(objective.Steps) > 0 {
					dslChapter.Objectives = append(dslChapter.Objectives, objective)
				}

				dsl.Storyline.Chapters = append(dsl.Storyline.Chapters, dslChapter)
				partNum++
			}
		}
	}

	return nil
}

// convertEventToStep converts an outline event to a storyline step
func (na *NovelgenAdapter) convertEventToStep(event rpg.StoryEvent, enemies []rpg.StoryOutlineEnemy, order int) *Step {
	step := &Step{
		Order:       order,
		Description: describeStoryEvent(event),
	}

	// Determine event type and create appropriate event
	switch event.Type {
	case "combat", "battle":
		step.Event = Event{
			Type: "combat",
			Combat: &CombatEvent{
				Setup: CombatSetup{
					Location: event.Context,
					Enemies:  storyOutlineEnemySpawns(enemies),
				},
			},
		}

	case "dialogue", "talk":
		step.Event = Event{
			Type: "dialogue",
			Dialogue: &DialogueEvent{
				Speaker: event.Actor,
				Text:    event.Result,
			},
		}

	case "acquire", "item":
		step.Event = Event{
			Type: "acquire",
			Acquire: &AcquireEvent{
				Actor:  event.Actor,
				Item:   event.Target,
				Source: event.Context,
			},
		}

	case "move", "reach":
		step.Event = Event{
			Type: "move",
			Move: &MoveEvent{
				Actor: event.Actor,
				To:    event.Target,
			},
		}

	case "status":
		step.Event = Event{
			Type: "status",
		}

	default:
		// Generic event
		step.Event = Event{
			Type: event.Type,
		}
	}
	if strings.TrimSpace(step.Event.Type) == "" {
		step.Event.Type = eventTypeFromAction(event.Action)
	}
	if step.Event.Type == "combat" && step.Event.Combat == nil {
		step.Event.Combat = &CombatEvent{
			Setup: CombatSetup{
				Location: event.Context,
				Enemies:  storyOutlineEnemySpawns(enemies),
			},
		}
	}
	step.Event.StateDeltas = append(step.Event.StateDeltas, buildStoryEventStateDeltas(event)...)

	return step
}

func (na *NovelgenAdapter) addStoryOutlineEnemies(dsl *DSL, enemies []rpg.StoryOutlineEnemy) {
	if dsl == nil || dsl.Characters == nil {
		return
	}
	seen := make(map[string]bool, len(dsl.Characters.Enemies))
	for _, enemy := range dsl.Characters.Enemies {
		seen[enemy.ID] = true
	}
	for i, enemy := range enemies {
		name := strings.TrimSpace(enemy.Name)
		if name == "" {
			continue
		}
		id := storyOutlineEnemyID(enemy, i)
		if seen[id] {
			continue
		}
		seen[id] = true
		level := enemy.Level
		if level <= 0 {
			level = 1
		}
		dsl.Characters.Enemies = append(dsl.Characters.Enemies, Enemy{
			ID:          id,
			Name:        name,
			Type:        coalesceString(enemy.Faction, "enemy"),
			Description: strings.TrimSpace(enemy.Context),
			Level:       level,
			Template: EnemyTemplate{
				BaseLevel: level,
				HPFormula: fmt.Sprintf("%d", 40+level*20),
				StatsPerLevel: map[string]int{
					"str": 6 + level,
					"agi": 4 + level,
					"int": 3 + level,
					"vit": 5 + level,
				},
			},
		})
	}
}

func storyOutlineEnemySpawns(enemies []rpg.StoryOutlineEnemy) []EnemySpawn {
	spawns := make([]EnemySpawn, 0, len(enemies))
	for i, enemy := range enemies {
		if strings.TrimSpace(enemy.Name) == "" {
			continue
		}
		count := enemy.Count
		if count <= 0 {
			count = 1
		}
		spawns = append(spawns, EnemySpawn{
			ID:    storyOutlineEnemyID(enemy, i),
			Count: count,
			Level: enemy.Level,
			Boss:  enemy.IsBoss,
		})
	}
	return spawns
}

func storyOutlineEnemyID(enemy rpg.StoryOutlineEnemy, index int) string {
	if strings.TrimSpace(enemy.BossID) != "" {
		return sanitizeID(enemy.BossID)
	}
	base := strings.TrimSpace(enemy.Faction + "_" + enemy.Tier + "_" + enemy.Name)
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("enemy_%02d", index+1)
	}
	return coalesceID(sanitizeID(base), fmt.Sprintf("enemy_%02d", index+1))
}

func describeStoryEvent(event rpg.StoryEvent) string {
	for _, candidate := range []string{event.Details, event.Result, event.Context} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	parts := []string{}
	for _, value := range []string{event.Actor, event.Action, event.Target, event.Change, event.Subject} {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	if len(parts) == 0 {
		return "outline event"
	}
	return strings.Join(parts, " -> ")
}

func buildStoryEventStateDeltas(event rpg.StoryEvent) []StateDelta {
	var deltas []StateDelta
	target := strings.TrimSpace(event.Target)
	if target == "" {
		target = strings.TrimSpace(event.Subject)
	}
	kind := strings.TrimSpace(event.TargetType)
	if kind == "" {
		kind = strings.TrimSpace(event.Type)
	}
	if kind == "" {
		kind = eventTypeFromAction(event.Action)
	}
	if target != "" || event.Change != "" || event.Result != "" {
		deltas = append(deltas, StateDelta{
			Target: target,
			Kind:   kind,
			Field:  event.Action,
			To:     coalesceString(event.Change, event.Result),
			Note:   describeStoryEvent(event),
		})
	}
	if event.Type == "storyline" || event.TargetType == "storyline" {
		deltas = append(deltas, StateDelta{
			Target: target,
			Kind:   "storyline",
			Field:  "event",
			To:     coalesceString(event.Change, event.Action),
			Note:   describeStoryEvent(event),
		})
	}
	return deltas
}

func storyStorylineAdvanceDescription(advance rpg.StorylineAdvance) string {
	parts := []string{advance.StorylineName, advance.Stage, advance.Change, advance.Consequence, advance.Pressure}
	var out []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, " | ")
}

func buildStoryStorylineAdvanceDeltas(chapterID string, advance rpg.StorylineAdvance) []StateDelta {
	stage := strings.ToLower(strings.TrimSpace(advance.Stage))
	if stage == "" {
		stage = "progress"
	}
	note := fmt.Sprintf("change=%s; consequence=%s; pressure=%s", advance.Change, advance.Consequence, advance.Pressure)
	deltas := []StateDelta{{
		Target: strings.TrimSpace(advance.StorylineName),
		Kind:   "storyline",
		Field:  "stage",
		To:     stage,
		Cost:   strings.TrimSpace(advance.Consequence),
		Unit:   strings.TrimSpace(advance.Pressure),
		Note:   note,
	}}
	switch stage {
	case "payoff", "resolve", "resolved", "completion", "completed":
		deltas = append(deltas, StateDelta{Target: advance.StorylineName, Kind: "plot_thread", Field: "storyline", To: "resolved", Note: "payoff in " + chapterID})
	case "hook", "pressure", "reveal", "reversal", "twist", "progress":
		deltas = append(deltas, StateDelta{Target: advance.StorylineName, Kind: "plot_thread", Field: "storyline", To: "raised", Note: "active pressure in " + chapterID})
	}
	return deltas
}

func buildStoryMysterySteps(chapterID string, mysteries rpg.StoryChapterMysteries) []Step {
	var steps []Step
	for _, planted := range mysteries.Planted {
		id := strings.TrimSpace(planted.ID)
		if id == "" {
			continue
		}
		clue := strings.TrimSpace(planted.Clue)
		steps = append(steps, Step{
			Description: fmt.Sprintf("mystery %s planted: %s", id, clue),
			Event: Event{
				Type: "mystery",
				StateDeltas: []StateDelta{{
					Target: id,
					Kind:   "plot_thread",
					Field:  "mystery",
					To:     "raised",
					Unit:   strings.TrimSpace(planted.Horizon),
					Cost:   strings.TrimSpace(planted.Status),
					Note:   clue,
				}},
			},
		})
	}
	for _, resolved := range mysteries.Resolved {
		id := strings.TrimSpace(resolved.ID)
		if id == "" {
			continue
		}
		resolution := strings.TrimSpace(resolved.Resolution)
		steps = append(steps, Step{
			Description: fmt.Sprintf("mystery %s resolved: %s", id, resolution),
			Event: Event{
				Type: "mystery",
				StateDeltas: []StateDelta{{
					Target: id,
					Kind:   "plot_thread",
					Field:  "mystery",
					To:     "resolved",
					Note:   resolution,
				}},
			},
		})
	}
	return steps
}

func buildStoryStateAnchorDeltas(chapter rpg.StoryChapter) []StateDelta {
	var deltas []StateDelta
	if strings.TrimSpace(chapter.StateAnchor.Cultivation) != "" {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "cultivation",
			Field:  "realm",
			To:     chapter.StateAnchor.Cultivation,
			Note:   "chapter start cultivation: " + chapter.StateAnchor.Cultivation,
		})
	}
	if chapter.StateAnchor.SpiritStones > 0 {
		deltas = append(deltas, StateDelta{
			Target: "spirit_stones",
			Kind:   "resource",
			Field:  "quantity",
			To:     fmt.Sprintf("%d", chapter.StateAnchor.SpiritStones),
			Note:   "chapter start spirit stones",
		})
	}
	if len(chapter.StateAnchor.Injuries) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "injury",
			Field:  "active",
			To:     "injured",
			Note:   "chapter start injuries: " + strings.Join(chapter.StateAnchor.Injuries, ", "),
		})
	}
	if len(chapter.StateAnchor.Allies) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "ally",
			Field:  "active",
			To:     strings.Join(chapter.StateAnchor.Allies, ", "),
			Note:   "chapter start allies: " + strings.Join(chapter.StateAnchor.Allies, ", "),
		})
	}
	if len(chapter.StateAnchor.KeyItems) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "item",
			Field:  "key_items",
			To:     strings.Join(chapter.StateAnchor.KeyItems, ", "),
			Note:   "chapter start key items: " + strings.Join(chapter.StateAnchor.KeyItems, ", "),
		})
	}
	deltas = append(deltas, buildStructuredProgressionDeltas(
		chapter.StateAnchor.Cultivation,
		chapter.StateAnchor.KeyItems,
		chapter.StateAnchor.Injuries,
	)...)
	return deltas
}

// convertNovelgenCharacterToPlayer converts novelgen character to DSL player
func (na *NovelgenAdapter) convertNovelgenCharacterToPlayer(name string, char rpg.NovelgenCharacter) *Player {
	player := &Player{
		ID:                sanitizeID(name),
		Name:              name,
		Description:       char.Background,
		Age:               parseAge(char.Age),
		Gender:            char.Gender,
		Race:              char.Race,
		Background:        char.Background,
		Personality:       char.Personality,
		Motivation:        char.Motivation,
		Abilities:         char.Abilities,
		Affiliations:      char.Affiliations,
		RoleInStory:       char.RoleInStory,
		Voice:             char.Voice,
		IsPlaceholder:     false,
		PlaceholderSource: "",
		Class:             inferClassFromCharacter(char),
		Skills:            char.Skills,
		Stats:             inferStatsFromCharacter(char),
		Traits:            make(map[string]Trait),
	}

	// Convert skills to traits
	for _, skill := range char.Skills {
		player.Traits[sanitizeID(skill)] = Trait{
			Unlocked: true,
			Trigger:  "passive",
		}
	}

	return player
}

// convertPlayerToNPC converts a player struct to NPC
func (na *NovelgenAdapter) convertPlayerToNPC(player *Player) NPC {
	return NPC{
		ID:            player.ID,
		Name:          player.Name,
		Role:          player.RoleInStory,
		Description:   player.Description,
		Age:           player.Age,
		Gender:        player.Gender,
		Appearance:    "", // Would need separate field in novelgen
		Background:    player.Background,
		Personality:   player.Personality,
		Affiliations:  player.Affiliations,
		IsPlaceholder: player.IsPlaceholder,
	}
}

// convertNovelgenLocationToLocation converts novelgen location to DSL location
func (na *NovelgenAdapter) convertNovelgenLocationToLocation(name string, loc rpg.NovelgenLocation) Location {
	location := Location{
		ID:             sanitizeID(name),
		Name:           name,
		Type:           inferMapTypeFromLocation(loc),
		Description:    loc.Description,
		Appearance:     loc.Appearance,
		Atmosphere:     loc.Atmosphere,
		History:        loc.History,
		Secrets:        loc.Secrets,
		SensoryDetails: loc.SensoryDetails,
		Inhabitants:    loc.Inhabitants,
		Events:         loc.Events,
		IsPlaceholder:  false,
	}

	// Add connections
	for _, connected := range loc.ConnectedLocs {
		location.Connections = append(location.Connections, Connection{
			To: sanitizeID(connected),
		})
	}

	return location
}

// convertNovelgenItemToItem converts novelgen item to DSL item
func (na *NovelgenAdapter) convertNovelgenItemToItem(name string, item rpg.NovelgenItem) Item {
	return Item{
		ID:          sanitizeID(name),
		Name:        name,
		Description: item.Description,
		Type:        inferItemTypeFromNovelgen(item),
		Rarity:      inferRarityFromSignificance(item.Significance),
		Effects:     convertPowersToEffects(item.Powers),
	}
}

// Helper functions

func sanitizeID(name string) string {
	// Convert name to valid ID
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, "'", "")
	id = strings.ReplaceAll(id, `"`, "")
	return id
}

func parseAge(age string) int {
	// Parse age string to int
	// Simplified version
	var result int
	fmt.Sscanf(age, "%d", &result)
	return result
}

func inferClassFromCharacter(char rpg.NovelgenCharacter) string {
	// Infer RPG class from character info
	for _, skill := range char.Skills {
		skillLower := strings.ToLower(skill)
		if strings.Contains(skillLower, "剑") || strings.Contains(skillLower, "格斗") {
			return "warrior"
		}
		if strings.Contains(skillLower, "法") || strings.Contains(skillLower, "术") {
			return "mage"
		}
		if strings.Contains(skillLower, "弓") || strings.Contains(skillLower, "射") {
			return "archer"
		}
	}
	return "adventurer"
}

func inferStatsFromCharacter(char rpg.NovelgenCharacter) Stats {
	// Infer stats from character skills and abilities
	stats := Stats{
		HP:  100,
		MP:  50,
		STR: 10,
		AGI: 10,
		INT: 10,
		VIT: 10,
	}

	// Adjust based on skills
	for _, skill := range char.Skills {
		skillLower := strings.ToLower(skill)
		if strings.Contains(skillLower, "力") || strings.Contains(skillLower, "剑") {
			stats.STR += 2
		}
		if strings.Contains(skillLower, "速") || strings.Contains(skillLower, "轻") {
			stats.AGI += 2
		}
		if strings.Contains(skillLower, "智") || strings.Contains(skillLower, "法") {
			stats.INT += 2
		}
	}

	// Adjust based on role
	if char.RoleInStory == "主角" {
		stats.HP += 20
		stats.MP += 10
	}

	return stats
}

func inferMapTypeFromName(name string) string {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "城") || strings.Contains(nameLower, "镇") || strings.Contains(nameLower, "据点") {
		return "city"
	}
	if strings.Contains(nameLower, "洞") || strings.Contains(nameLower, "穴") || strings.Contains(nameLower, "地下") {
		return "dungeon"
	}
	if strings.Contains(nameLower, "林") || strings.Contains(nameLower, "森") {
		return "forest"
	}
	return "field"
}

func inferMapTypeFromLocation(loc rpg.NovelgenLocation) string {
	nameLower := strings.ToLower(loc.Name)
	if strings.Contains(nameLower, "城") || strings.Contains(nameLower, "镇") || strings.Contains(nameLower, "据点") {
		return "city"
	}
	if strings.Contains(nameLower, "洞") || strings.Contains(nameLower, "穴") || strings.Contains(nameLower, "地下") {
		return "dungeon"
	}
	if strings.Contains(nameLower, "林") || strings.Contains(nameLower, "森") {
		return "forest"
	}
	return "field"
}

func inferItemTypeFromNovelgen(item rpg.NovelgenItem) string {
	typeLower := strings.ToLower(item.Type)
	if strings.Contains(typeLower, "消耗") || strings.Contains(typeLower, "药") {
		return "consumable"
	}
	if strings.Contains(typeLower, "材料") {
		return "material"
	}
	if strings.Contains(typeLower, "装备") || strings.Contains(typeLower, "武器") {
		return "equipment"
	}
	return "misc"
}

func inferRarityFromSignificance(significance string) string {
	sigLower := strings.ToLower(significance)
	if strings.Contains(sigLower, "核心") || strings.Contains(sigLower, "关键") {
		return "legendary"
	}
	if strings.Contains(sigLower, "重要") {
		return "epic"
	}
	return "common"
}

func convertPowersToEffects(powers []string) map[string]interface{} {
	effects := make(map[string]interface{})
	for _, power := range powers {
		effects[sanitizeID(power)] = power
	}
	return effects
}
