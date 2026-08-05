package dsl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

func TestSplitOutlineLocation(t *testing.T) {
	name, description := splitOutlineLocation("地下机甲库：空气里有机油味，灯光断续闪烁")

	if name != "地下机甲库" {
		t.Fatalf("unexpected name: %q", name)
	}
	if description != "空气里有机油味，灯光断续闪烁" {
		t.Fatalf("unexpected description: %q", description)
	}
}

func TestInferProtagonistNameFromSetupPrefersNamedAuthor(t *testing.T) {
	setup := &models.StorySetup{
		Premise: "专写矿场开局修仙文的网络作者陈东穿越为灵石矿奴，他因常年深耕同类型创作觉醒诡异金手指。",
		Theme:   "略带黑色幽默的主线危机。",
		Storylines: []models.Storyline{{
			Name:        "矿场死循环",
			Description: "陈东在矿难中反复死亡复活。",
			Desire:      "逃离矿场死循环",
		}},
	}

	if got := inferProtagonistNameFromSetup(setup); got != "陈东" {
		t.Fatalf("unexpected protagonist name: %q", got)
	}
}

func TestModelAdapterOutlinePlayerUsesCraftProtagonistDetails(t *testing.T) {
	characters := map[string]*models.Character{
		"Lin": {
			Name:         "Lin",
			RoleInStory:  "protagonist",
			Background:   "old-world engineer",
			Motivation:   "restore humanity",
			Personality:  []string{"calm", "stubborn"},
			CombatRole:   "mech pilot",
			Skills:       []string{"engineering"},
			Abilities:    []string{"fire core"},
			Affiliations: []string{"Qingteng"},
			DSLTags:      []string{"pilot"},
		},
		"bug": {
			Name:        "Bug",
			RoleInStory: "enemy scout",
			Background:  "hostile swarm unit",
			PowerLevel:  2,
		},
	}

	got, err := NewModelAdapter(&models.StorySetup{ProjectName: "Fire"}, &models.Outline{}, characters, nil, nil).BuildDSL(PhaseOutline)
	if err != nil {
		t.Fatalf("build outline DSL: %v", err)
	}
	player := got.Characters.Player
	if player == nil {
		t.Fatalf("player missing")
	}
	if player.Name != "Lin" ||
		player.Background != "old-world engineer" ||
		player.Motivation != "restore humanity" ||
		player.Class != "mech pilot" ||
		len(player.Personality) != 2 ||
		len(player.Skills) != 1 ||
		len(player.Abilities) != 1 ||
		len(player.Affiliations) != 1 {
		t.Fatalf("craft protagonist details were not copied to player: %+v", player)
	}
	if len(got.Characters.Enemies) != 1 || got.Characters.Enemies[0].Name != "Bug" {
		t.Fatalf("craft enemies should be available to outline simulation: %+v", got.Characters.Enemies)
	}
}

func TestNovelgenAdapterOutlineAddsUsablePlayerLocationsAndEventTypes(t *testing.T) {
	tmp := t.TempDir()
	bookName := "mine-v2"
	setupDir := filepath.Join(tmp, "books", bookName, "story", "setup")
	if err := os.MkdirAll(setupDir, 0o755); err != nil {
		t.Fatalf("mkdir setup: %v", err)
	}
	setup := models.StorySetup{
		ProjectName: "穿越者的天道弈局",
		Genres:      []string{"仙侠"},
		Premise:     "专写矿场开局修仙文的网络作者陈东穿越为灵石矿奴，他因常年深耕同类型创作觉醒诡异金手指。",
		Theme:       "黑暗修仙求生。",
		Tone:        "冷峻",
		Storylines: []models.Storyline{{
			Name:       "矿场死循环",
			Importance: 10,
			Desire:     "逃离矿场死循环",
		}},
	}
	writeJSON(t, filepath.Join(setupDir, "story_setup.json"), setup)

	project := &rpg.NovelgenProject{
		ProjectPath: tmp,
		BookName:    bookName,
		Characters:  map[string]rpg.NovelgenCharacter{},
		Items:       map[string]rpg.NovelgenItem{},
		Locations:   map[string]rpg.NovelgenLocation{},
		Outline: rpg.StoryOutline{Parts: []rpg.StoryPart{{
			ID: "P1",
			Volumes: []rpg.StoryVolume{{
				ID: "V1",
				Chapters: []rpg.StoryChapter{{
					ID:          "P1-V1-C1",
					Title:       "矿道惊醒",
					Characters:  []string{"陈东", "三哥"},
					Location:    "黑风矿·丙字三号矿道",
					StateAnchor: rpg.StoryStateAnchor{Location: "废弃矿道深处：潮湿、低温"},
					Events: []rpg.StoryEvent{
						{Type: "premise", Subject: "附矿场复活", Change: "awakened"},
						{Type: "gate", Subject: "矿场封锁", Change: "started"},
						{Type: "discover", Subject: "禁术封印", Change: "discovered"},
						{Type: "use", Target: "血符", TargetType: "item", Change: "used"},
					},
				}},
			}},
		}}},
	}

	adapter := NewNovelgenAdapter(project, NewConsoleLogger(WithMinLevel(LogLevelError)))
	outlineDSL, err := adapter.ToDSL(PhaseOutline)
	if err != nil {
		t.Fatalf("outline DSL: %v", err)
	}

	if outlineDSL.Characters.Player == nil || outlineDSL.Characters.Player.Name != "陈东" {
		t.Fatalf("unexpected player: %+v", outlineDSL.Characters.Player)
	}
	if outlineDSL.Characters.Player.Background == "" || outlineDSL.Characters.Player.Motivation == "" {
		t.Fatalf("player profile was not populated: %+v", outlineDSL.Characters.Player)
	}
	for _, npc := range outlineDSL.Characters.NPCs {
		if npc.Name == "陈东" {
			t.Fatalf("protagonist should not be emitted as NPC placeholder: %+v", npc)
		}
	}
	if !hasNPC(outlineDSL, "三哥") {
		t.Fatalf("supporting outline character was not emitted as NPC placeholder: %+v", outlineDSL.Characters.NPCs)
	}
	if !hasLocation(outlineDSL, "黑风矿·丙字三号矿道") || !hasLocation(outlineDSL, "废弃矿道深处") {
		t.Fatalf("outline locations were not emitted: %+v", outlineDSL.World.Locations)
	}

	var eventTypes []string
	for _, step := range outlineDSL.Storyline.Chapters[0].Objectives[0].Steps {
		eventTypes = append(eventTypes, step.Event.Type)
	}
	joined := strings.Join(eventTypes, ",")
	for _, invalid := range []string{"premise", "gate", "discover", "use"} {
		if strings.Contains(joined, invalid) {
			t.Fatalf("event type %q should have been normalized: %v", invalid, eventTypes)
		}
	}
}

func TestModelAdapterMapsEventResultToCompletion(t *testing.T) {
	adapter := &ModelAdapter{}
	event := adapter.buildEventFromModel(models.Event{
		Type:       "combat",
		Action:     models.ActionCombat,
		Target:     "raiders",
		TargetType: "character",
		Context:    "ridge",
		Result:     "Hero defeats the raiders and learns a better ambush tactic.",
	}, nil)

	if event.OnComplete == nil {
		t.Fatalf("OnComplete was not populated")
	}
	if event.OnComplete.Narration != "Hero defeats the raiders and learns a better ambush tactic." {
		t.Fatalf("Narration = %q", event.OnComplete.Narration)
	}
	if !stepHasNarrativeResult(&Step{Event: event}) {
		t.Fatalf("mapped event result should count as narrative result")
	}
	if !stepHasCombatGrowthReward(&Step{Event: event}) {
		t.Fatalf("combat result with learns should count as growth/reward")
	}
}

func TestModelAdapterMapsAcquireResultItems(t *testing.T) {
	adapter := &ModelAdapter{}
	event := adapter.buildEventFromModel(models.Event{
		Action: models.ActionAcquire,
		Target: "Signal Key",
		Result: "Hero obtains the Signal Key and opens the next route.",
	}, nil)

	if event.OnComplete == nil || len(event.OnComplete.Items) != 1 || event.OnComplete.Items[0] != "Signal Key" {
		t.Fatalf("acquire result/items not mapped: %#v", event.OnComplete)
	}
}

func TestMergerPreservesSetupPlayerWhenMergingOutline(t *testing.T) {
	merger := NewDSLMerger(NewConsoleLogger(WithMinLevel(LogLevelError)))
	merger.AddFragment(&DSL{
		Metadata: &Metadata{Title: "setup"},
		World:    &World{},
		Characters: &Characters{Player: &Player{
			ID:          "陈东",
			Name:        "陈东",
			Background:  "网络作者穿越为矿奴。",
			Motivation:  "逃离矿场",
			Personality: []string{"沉稳"},
			Stats:       Stats{STR: 12, AGI: 11, INT: 14, VIT: 12, HP: 120, MP: 60},
		}},
		Storyline: &Storyline{},
		Systems:   &Systems{},
	}, PhaseSetup, "setup")
	merger.AddFragment(&DSL{
		Metadata: &Metadata{Title: "outline"},
		World:    &World{},
		Characters: &Characters{Player: &Player{
			ID:   "陈东",
			Name: "陈东",
		}},
		Storyline: &Storyline{},
		Systems:   &Systems{},
	}, PhaseOutline, "outline")

	result, err := merger.Merge()
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	player := result.DSL.Characters.Player
	if player == nil || player.IsPlaceholder || player.Background == "" || player.Motivation == "" || player.Stats.HP != 120 {
		t.Fatalf("setup player details were not preserved: %+v", player)
	}
}

func TestMergerCombinesFullAndVolumeOutlineFragments(t *testing.T) {
	merger := NewDSLMerger(NewConsoleLogger(WithMinLevel(LogLevelError)))
	merger.AddFragment(&DSL{
		Metadata:   &Metadata{Title: "full"},
		World:      &World{},
		Characters: &Characters{},
		Storyline: &Storyline{
			Arcs: []Arc{{ID: "V1", Name: "第一卷"}},
			Chapters: []Chapter{
				{ID: "P1-V1-C1", Title: "旧一", Position: 1},
				{ID: "P1-V1-C2", Title: "旧二", Position: 2},
				{ID: "P1-V2-C1", Title: "第三章", Position: 3},
			},
		},
		Systems: &Systems{},
	}, PhaseOutline, "01_outline.rpg")
	merger.AddFragment(&DSL{
		Metadata:   &Metadata{Title: "volume"},
		World:      &World{},
		Characters: &Characters{},
		Storyline: &Storyline{
			Arcs: []Arc{{ID: "V1", Name: "第一卷新版"}},
			Chapters: []Chapter{
				{ID: "P1-V1-C1", Title: "新一", Position: 1},
				{ID: "P1-V1-C2", Title: "新二", Position: 2},
			},
		},
		Systems: &Systems{},
	}, PhaseOutline, "01_outline_v01.rpg")

	result, err := merger.Merge()
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	chapters := result.DSL.Storyline.Chapters
	if len(chapters) != 3 {
		t.Fatalf("volume fragment should update matching chapters without dropping others: %+v", chapters)
	}
	if chapters[0].Title != "新一" || chapters[1].Title != "新二" || chapters[2].Title != "第三章" {
		t.Fatalf("unexpected merged chapters: %+v", chapters)
	}
	if result.DSL.Storyline.Arcs[0].Name != "第一卷新版" {
		t.Fatalf("arc was not updated: %+v", result.DSL.Storyline.Arcs)
	}
}

func TestModelAdapterCraftUsesExplicitRPGMetadata(t *testing.T) {
	adapter := NewModelAdapterWithOrganizations(nil, nil,
		map[string]*models.Character{
			"chen_dong": {
				Name:        "Chen Dong",
				RoleInStory: "protagonist",
				CombatRole:  "mecha_pilot",
				RPGStats:    &models.CraftRPGStats{STR: 13, AGI: 12, INT: 15, VIT: 11, HP: 130, MP: 70, Level: 4},
				DSLTags:     []string{"pilot"},
			},
			"queen": {
				Name:       "Hive Queen",
				RPGRole:    "boss",
				PowerLevel: 7,
				RPGStats:   &models.CraftRPGStats{STR: 20, AGI: 8, INT: 16, VIT: 18, HP: 400, MP: 120, Level: 7},
			},
		},
		map[string]*models.Location{
			"mine": {
				Name:          "Black Wind Mine",
				Type:          "mine",
				RPGMapType:    "dungeon",
				DangerLevel:   6,
				EncounterTags: []string{"zerg"},
			},
		},
		map[string]*models.Item{
			"token": {
				Name:             "Blood Token",
				Type:             "token",
				RPGItemType:      "key",
				Rarity:           "rare",
				PowerLevel:       3,
				QuantityTracking: true,
				StateEffects: []models.CraftStateEffect{{
					Target: "protagonist",
					Kind:   "item",
					Field:  "key",
					To:     "Blood Token",
				}},
			},
		},
		map[string]*models.Organization{
			"ember_guild": {
				Name:      "Ember Guild",
				Type:      "guild",
				Goals:     []string{"Control the mine"},
				Resources: []string{"forges"},
				StateEffects: []models.CraftStateEffect{{
					Target: "ember_guild",
					Kind:   "faction",
					Field:  "influence",
					To:     "rising",
				}},
			},
		},
	)

	dslData, err := adapter.BuildDSL(PhaseCraft)
	if err != nil {
		t.Fatalf("build DSL: %v", err)
	}
	if dslData.Characters.Player == nil || dslData.Characters.Player.Stats.HP != 130 || dslData.Characters.Player.Class != "mecha_pilot" {
		t.Fatalf("player did not use explicit metadata: %+v", dslData.Characters.Player)
	}
	if len(dslData.Characters.Enemies) != 1 || dslData.Characters.Enemies[0].Level != 7 || dslData.Characters.Enemies[0].Template.HPFormula != "400" {
		t.Fatalf("enemy did not use explicit metadata: %+v", dslData.Characters.Enemies)
	}
	if len(dslData.World.Locations) != 1 || dslData.World.Locations[0].Type != "dungeon" || dslData.World.Locations[0].Properties["danger_level"] != 6 {
		t.Fatalf("location did not use explicit metadata: %+v", dslData.World.Locations)
	}
	if len(dslData.World.Items) != 1 || dslData.World.Items[0].Type != "key" || dslData.World.Items[0].Rarity != "rare" || dslData.World.Items[0].Effects["quantity_tracking"] != true {
		t.Fatalf("item did not use explicit metadata: %+v", dslData.World.Items)
	}
	if !hasRuleWithTrigger(dslData, "organization.profile") || !hasRuleWithTrigger(dslData, "organization.state_effect") {
		t.Fatalf("organization rules were not emitted: %+v", dslData.World.Rules)
	}
}

func TestModelAdapterCraftOrdersMapBackedElements(t *testing.T) {
	adapter := NewModelAdapterWithOrganizations(nil, nil,
		map[string]*models.Character{
			"zeta":  {Name: "Zeta", RoleInStory: "supporting"},
			"alpha": {Name: "Alpha", RoleInStory: "supporting"},
		},
		map[string]*models.Location{
			"zeta_base":  {Name: "Zeta Base"},
			"alpha_mine": {Name: "Alpha Mine"},
		},
		map[string]*models.Item{
			"zeta_key":  {Name: "Zeta Key"},
			"alpha_key": {Name: "Alpha Key"},
		},
		map[string]*models.Organization{
			"zeta_order":  {Name: "Zeta Order"},
			"alpha_order": {Name: "Alpha Order"},
		},
	)

	dslData, err := adapter.BuildDSL(PhaseCraft)
	if err != nil {
		t.Fatalf("build DSL: %v", err)
	}
	if len(dslData.Characters.NPCs) != 2 || dslData.Characters.NPCs[0].ID != "alpha" || dslData.Characters.NPCs[1].ID != "zeta" {
		t.Fatalf("characters were not deterministic: %+v", dslData.Characters.NPCs)
	}
	if len(dslData.World.Locations) != 2 || dslData.World.Locations[0].ID != "alpha_mine" || dslData.World.Locations[1].ID != "zeta_base" {
		t.Fatalf("locations were not deterministic: %+v", dslData.World.Locations)
	}
	if len(dslData.World.Items) != 2 || dslData.World.Items[0].ID != "alpha_key" || dslData.World.Items[1].ID != "zeta_key" {
		t.Fatalf("items were not deterministic: %+v", dslData.World.Items)
	}
	if !strings.Contains(dslData.World.Rules[0].Name, "alpha_order") || !strings.Contains(dslData.World.Rules[1].Name, "zeta_order") {
		t.Fatalf("organizations were not deterministic: %+v", dslData.World.Rules)
	}
}

func TestModelAdapterCraftResolvesLocationConnectionsByNameOrKey(t *testing.T) {
	adapter := NewModelAdapter(nil, nil, nil,
		map[string]*models.Location{
			"loc_start": {
				Name:               "Crystal Market",
				ConnectedLocations: []string{"Other Gate"},
			},
			"other_gate": {
				Name: "Other Gate",
			},
		},
		nil,
	)

	dslData, err := adapter.BuildDSL(PhaseCraft)
	if err != nil {
		t.Fatalf("build DSL: %v", err)
	}
	if len(dslData.World.Locations) != 2 {
		t.Fatalf("expected two locations, got %+v", dslData.World.Locations)
	}
	if len(dslData.World.Locations[0].Connections) != 1 || dslData.World.Locations[0].Connections[0].To != "other_gate" {
		t.Fatalf("expected location connection to resolve to craft key, got %+v", dslData.World.Locations[0].Connections)
	}
}

func TestDSLMergerPreservesCraftWorldRules(t *testing.T) {
	merger := NewDSLMerger(NewConsoleLogger(WithMinLevel(LogLevelError)))
	merger.AddFragment(&DSL{
		Metadata:   &Metadata{Title: "outline"},
		World:      &World{},
		Characters: &Characters{},
		Storyline:  &Storyline{},
		Systems:    &Systems{},
	}, PhaseOutline, "01_outline.rpg")
	merger.AddFragment(&DSL{
		Metadata: &Metadata{Title: "craft"},
		World: &World{
			Rules: []Rule{{
				Name:    "organization_ember_guild",
				Trigger: "organization.profile",
				Effect:  "id=ember_guild; name=Ember Guild",
			}},
		},
		Characters: &Characters{},
		Storyline:  &Storyline{},
		Systems:    &Systems{},
	}, PhaseCraft, "02_craft.rpg")

	result, err := merger.Merge()
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	found := false
	for _, rule := range result.DSL.World.Rules {
		if rule.Name == "organization_ember_guild" && rule.Trigger == "organization.profile" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected craft world rule to survive merge, got %+v", result.DSL.World.Rules)
	}
}

func writeJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func hasNPC(dsl *DSL, name string) bool {
	for _, npc := range dsl.Characters.NPCs {
		if npc.Name == name {
			return true
		}
	}
	return false
}

func hasLocation(dsl *DSL, name string) bool {
	for _, loc := range dsl.World.Locations {
		if loc.Name == name {
			return true
		}
	}
	return false
}

func hasRuleWithTrigger(dsl *DSL, trigger string) bool {
	for _, rule := range dsl.World.Rules {
		if rule.Trigger == trigger {
			return true
		}
	}
	return false
}
