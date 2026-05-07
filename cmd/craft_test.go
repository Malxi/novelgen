package cmd

import (
	"testing"

	"novelgen/internal/models"
)

func TestElementExtractorUsesRPGRelevantOutlineFields(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		Volumes: []models.Volume{{
			Chapters: []models.Chapter{{
				ID:         "P1-V1-C1",
				Characters: []string{"Chen Dong"},
				Location:   "Mine Gate",
				StateAnchor: models.StateAnchor{
					Allies:   []string{"San Ge"},
					Location: "Deep Shaft: wet and dark",
					KeyItems: []string{"Rusty Pickaxe"},
				},
				Enemies:        []models.OutlineEnemy{{Name: "Drone Guard", Faction: "Iron Hive"}},
				ResourceLedger: []models.ResourceLedgerEntry{{Item: "Spirit Stone", Start: 0, Delta: 3, End: 3}},
				Scenes: []models.OutlineScene{{
					POV:        "San Ge",
					Location:   "Repair Bay",
					Characters: []string{"Mechanic Luo"},
				}},
				Events: []models.Event{
					{Actor: "Chen Dong", Action: models.ActionAcquire, Target: "Blood Token", TargetType: models.TargetTypeItem},
					{Actor: "Drone Guard", Action: models.ActionMove, Target: "Mine Gate", TargetType: models.TargetTypeLocation},
					{Actor: "San Ge", Action: models.ActionMeet, Target: "Hidden Trader", TargetType: models.TargetTypeCharacter},
				},
			}},
		}},
	}}}

	elements := NewElementExtractor(outline, nil).Extract()

	for _, name := range []string{"Chen Dong", "San Ge", "Drone Guard", "Mechanic Luo", "Hidden Trader"} {
		if !containsString(elements.Characters, name) {
			t.Fatalf("missing character %q in %+v", name, elements.Characters)
		}
	}
	for _, name := range []string{"Mine Gate", "Deep Shaft", "Repair Bay"} {
		if !containsString(elements.Locations, name) {
			t.Fatalf("missing location %q in %+v", name, elements.Locations)
		}
	}
	for _, name := range []string{"Rusty Pickaxe", "Spirit Stone", "Blood Token"} {
		if !containsString(elements.Items, name) {
			t.Fatalf("missing item %q in %+v", name, elements.Items)
		}
	}
	if !containsString(elements.Organizations, "Iron Hive") {
		t.Fatalf("missing organization %q in %+v", "Iron Hive", elements.Organizations)
	}
}

func TestElementExtractorDoesNotTurnSetupPremisesIntoCraftItems(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{}}}
	setup := &models.StorySetup{
		Premises: []models.Premise{{
			Name:     "星核同步体系",
			Category: "ability",
			Progression: []models.ProgressionStage{{
				Name: "一阶同步者",
			}},
		}},
	}

	elements := NewElementExtractor(outline, setup).Extract()

	if len(elements.Characters) != 0 || len(elements.Locations) != 0 || len(elements.Items) != 0 {
		t.Fatalf("setup premises should not become craft elements: %+v", elements)
	}
}

func TestFindUnknownAbilitySystemRefsUsesSetupPremisesAsAuthority(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		Volumes: []models.Volume{{
			Chapters: []models.Chapter{{
				Events: []models.Event{
					{Actor: "Chen Dong", Action: models.ActionAwaken, Target: "星核同步体系", TargetType: models.TargetTypePremise},
					{Actor: "Chen Dong", Action: models.ActionUpgrade, Target: "一阶同步者", TargetType: models.TargetTypePremise},
					{Actor: "Chen Dong", Action: models.ActionAwaken, Target: "血脉燃烧体系", TargetType: models.TargetTypePremise},
				},
			}},
		}},
	}}}
	setup := &models.StorySetup{
		Premises: []models.Premise{{
			Name: "星核同步体系",
			Progression: []models.ProgressionStage{{
				Name: "一阶同步者",
			}},
		}},
	}

	unknown := findUnknownAbilitySystemRefs(outline, setup)

	if len(unknown) != 1 || unknown[0] != "血脉燃烧体系" {
		t.Fatalf("unexpected unknown ability refs: %+v", unknown)
	}
}

func TestElementExtractorKeepsStorylinesOutOfOrganizationsUnlessFactionLike(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{}}}
	setup := &models.StorySetup{
		Storylines: []models.Storyline{
			{Name: "成长主线", Type: "main"},
			{Name: "赤旗军团", Type: "faction", SetupRole: "势力压力"},
		},
	}

	elements := NewElementExtractor(outline, setup).Extract()

	if containsString(elements.Organizations, "成长主线") {
		t.Fatalf("ordinary storyline should not be extracted as organization: %+v", elements.Organizations)
	}
	if !containsString(elements.Organizations, "赤旗军团") {
		t.Fatalf("faction-like storyline should be extracted as organization: %+v", elements.Organizations)
	}
}

func TestMissingGeneratedNames(t *testing.T) {
	missing := missingGeneratedNames([]string{"A", "B"}, map[string]models.Character{"A": {Name: "A"}})
	if len(missing) != 1 || missing[0] != "B" {
		t.Fatalf("unexpected missing names: %+v", missing)
	}
}

func TestCraftFiltersResolveNumericIDs(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:         "P1-V1-C1",
				Characters: []string{"A"},
				Location:   "L",
				Enemies:    []models.OutlineEnemy{{Faction: "F"}},
			}},
		}},
	}}}
	elements := &ExtractedElements{
		Characters:    []string{"A", "B"},
		Locations:     []string{"L", "Elsewhere"},
		Organizations: []string{"F", "Other"},
	}

	byChapter, err := filterElementsByChapter(elements, "1", outline)
	if err != nil {
		t.Fatalf("filter chapter: %v", err)
	}
	if !containsString(byChapter.Characters, "A") || containsString(byChapter.Characters, "B") {
		t.Fatalf("unexpected chapter filter result: %+v", byChapter.Characters)
	}

	byVolume, err := filterElementsByVolume(elements, "1", outline)
	if err != nil {
		t.Fatalf("filter volume: %v", err)
	}
	if !containsString(byVolume.Organizations, "F") || containsString(byVolume.Organizations, "Other") {
		t.Fatalf("unexpected volume filter result: %+v", byVolume.Organizations)
	}

	byPart, err := filterElementsByPart(elements, "1", outline)
	if err != nil {
		t.Fatalf("filter part: %v", err)
	}
	if !containsString(byPart.Locations, "L") || containsString(byPart.Locations, "Elsewhere") {
		t.Fatalf("unexpected part filter result: %+v", byPart.Locations)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
