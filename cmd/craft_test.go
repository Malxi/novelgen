package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"

	"github.com/spf13/cobra"
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

func TestSaveJSONMergesExistingUTF8BOMFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "characters.json")
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"Lin":{"name":"Lin"}}`)...), 0644); err != nil {
		t.Fatal(err)
	}

	if err := saveJSON(path, map[string]models.Character{
		"Mira": {Name: "Mira", RoleInStory: "ally"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["Lin"] == nil || got["Mira"] == nil {
		t.Fatalf("merged data missing existing or new key: %#v", got)
	}
}

func TestCollectRequestedCharactersMatchesByName(t *testing.T) {
	loaded := map[string]*models.Character{
		"other-key": {Name: "林野", RoleInStory: "lead"},
	}

	got := collectRequestedCharacters([]string{"林野"}, loaded)
	if len(got) != 1 || got["林野"].Name != "林野" {
		t.Fatalf("expected requested character from loaded craft, got %#v", got)
	}
}

func TestCraftGenAgentApplyRequiresAgentSDK(t *testing.T) {
	oldAgentApply := craftAgentApplyFlag
	oldAgentSDK := craftAgentSDKFlag
	craftAgentApplyFlag = true
	craftAgentSDKFlag = false
	defer func() {
		craftAgentApplyFlag = oldAgentApply
		craftAgentSDKFlag = oldAgentSDK
	}()

	err := runCraftGen(&cobra.Command{}, nil)
	if err == nil || !strings.Contains(err.Error(), "--agent-apply requires --agent-sdk") {
		t.Fatalf("expected --agent-apply validation error, got %v", err)
	}
}

func TestCraftImproveAgentApplyRequiresAgentSDK(t *testing.T) {
	oldAgentApply := craftAgentApplyFlag
	oldAgentSDK := craftAgentSDKFlag
	craftAgentApplyFlag = true
	craftAgentSDKFlag = false
	defer func() {
		craftAgentApplyFlag = oldAgentApply
		craftAgentSDKFlag = oldAgentSDK
	}()

	err := runCraftImprove(&cobra.Command{}, nil)
	if err == nil || !strings.Contains(err.Error(), "--agent-apply requires --agent-sdk") {
		t.Fatalf("expected --agent-apply validation error, got %v", err)
	}
}

func TestCraftNameBatchesAndSortedKeys(t *testing.T) {
	got := craftNameBatches([]string{"a", "b", "c"}, 2)
	if len(got) != 2 || strings.Join(got[0], ",") != "a,b" || strings.Join(got[1], ",") != "c" {
		t.Fatalf("batches = %#v", got)
	}

	values := map[string]*models.Item{
		"zeta":  {Name: "zeta"},
		"alpha": {Name: "alpha"},
		"nil":   nil,
		"":      {Name: "blank"},
	}
	keys := sortedCraftMapKeys(values)
	if strings.Join(keys, ",") != "alpha,zeta" {
		t.Fatalf("sorted keys = %#v", keys)
	}
}

func TestFilterCraftModelsByNameNarrowsSelectedType(t *testing.T) {
	chars := map[string]*models.Character{
		"李侑":  {Name: "李侑"},
		"陆青禾": {Name: "陆青禾"},
	}
	locs := map[string]*models.Location{
		"玄云宗": {Name: "玄云宗"},
	}

	gotChars, gotLocs, gotItems, gotOrgs, err := filterCraftModelsByName("characters", "李侑", chars, locs, nil, nil)
	if err != nil {
		t.Fatalf("filterCraftModelsByName() error = %v", err)
	}
	if len(gotChars) != 1 || gotChars["李侑"] == nil {
		t.Fatalf("characters not narrowed to 李侑: %#v", gotChars)
	}
	if len(gotLocs) != 0 || len(gotItems) != 0 || len(gotOrgs) != 0 {
		t.Fatalf("non-selected types should be empty: locs=%#v items=%#v orgs=%#v", gotLocs, gotItems, gotOrgs)
	}
}

func TestFilterCraftModelsByNameMatchesObjectNameForOldKeys(t *testing.T) {
	chars := map[string]*models.Character{
		"legacy-key": {Name: "李侑"},
	}

	gotChars, _, _, _, err := filterCraftModelsByName("all", "李侑", chars, nil, nil, nil)
	if err != nil {
		t.Fatalf("filterCraftModelsByName() error = %v", err)
	}
	if len(gotChars) != 1 || gotChars["legacy-key"] == nil {
		t.Fatalf("expected legacy-key character match: %#v", gotChars)
	}
}

func TestFilterCraftModelsByNameErrorsWhenMissing(t *testing.T) {
	_, _, _, _, err := filterCraftModelsByName("characters", "李侑", map[string]*models.Character{
		"陆青禾": {Name: "陆青禾"},
	}, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `李侑`) || !strings.Contains(err.Error(), "characters") {
		t.Fatalf("expected missing target error, got %v", err)
	}
}

func TestCraftImproveRegistersNameFlag(t *testing.T) {
	if craftImproveCmd.Flags().Lookup("name") == nil {
		t.Fatalf("craft improve should register --name")
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

func TestElementExtractorIncludesSetupCoreCast(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{}}}
	setup := &models.StorySetup{
		CoreCast: []models.CoreCastSeed{
			{Name: "Hero", Role: "protagonist", Importance: 10},
			{Name: "Late Rival", Role: "rival", Importance: 8, EntryPhase: "mid"},
		},
	}

	elements := NewElementExtractor(outline, setup).Extract()

	for _, name := range []string{"Hero", "Late Rival"} {
		if !containsString(elements.Characters, name) {
			t.Fatalf("missing core cast character %q in %+v", name, elements.Characters)
		}
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

func TestOrganizationWriteContextOnlyIncludesRelevantOrganizations(t *testing.T) {
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Mine Ambush",
		Summary: "Chen Dong faces an Iron Hive patrol.",
		Enemies: []models.OutlineEnemy{{
			Name:    "Drone Guard",
			Faction: "Iron Hive",
		}},
	}
	organizations := map[string]*models.Organization{
		"iron_hive": {
			Name:        "Iron Hive",
			Type:        "faction",
			Description: "A machine cult controlling the lower mine shafts.",
			Goals:       []string{"Hold the mine"},
		},
		"moon_court": {
			Name:        "Moon Court",
			Type:        "faction",
			Description: "A distant aristocratic house.",
			Goals:       []string{"Win court influence"},
		},
	}

	got := buildOrganizationWriteContext(chapter, organizations)
	if !strings.Contains(got, "Iron Hive") {
		t.Fatalf("expected relevant organization in context, got %q", got)
	}
	if strings.Contains(got, "Moon Court") {
		t.Fatalf("unmatched organization should not be injected into write context: %q", got)
	}
}

func TestOrganizationWriteContextReturnsEmptyWithoutMatches(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Quiet Camp"}
	organizations := map[string]*models.Organization{
		"moon_court": {Name: "Moon Court", Type: "faction"},
	}

	if got := buildOrganizationWriteContext(chapter, organizations); got != "" {
		t.Fatalf("expected no organization context for unrelated chapter, got %q", got)
	}
}

func TestNormalizeLoadedElementsCompactsAndDropsNilEntries(t *testing.T) {
	characters := map[string]*models.Character{
		"hero": {
			DSLTags:    []string{"pilot", "pilot", ""},
			PowerLevel: -1,
		},
		"nil": nil,
	}
	organizations := map[string]*models.Organization{
		"guild": {
			Goals:   []string{"survive", "survive", ""},
			DSLTags: []string{"mine", "mine"},
		},
		"nil": nil,
	}

	normalizeLoadedElements(characters, nil, nil, organizations)

	if _, ok := characters["nil"]; ok {
		t.Fatalf("nil character entry should be dropped")
	}
	if characters["hero"].Name != "hero" || characters["hero"].PowerLevel != 0 || len(characters["hero"].DSLTags) != 1 {
		t.Fatalf("character was not normalized: %+v", characters["hero"])
	}
	if _, ok := organizations["nil"]; ok {
		t.Fatalf("nil organization entry should be dropped")
	}
	if organizations["guild"].Name != "guild" || len(organizations["guild"].Goals) != 1 || len(organizations["guild"].DSLTags) != 1 {
		t.Fatalf("organization was not normalized: %+v", organizations["guild"])
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
