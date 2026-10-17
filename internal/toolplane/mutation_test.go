package toolplane_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/contract/testfixtures"
	"novelgen/internal/platform/store"
	"novelgen/internal/toolplane"
)

func seedMutationProject(t *testing.T) *store.Root {
	t.Helper()
	root := store.MemRoot()
	if err := root.SaveConfig(&models.ProjectConfig{
		Name:          "Mutation Project",
		ChapterConfig: models.ChapterConfig{TargetWordsPerChapter: 800},
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := root.SaveSetup(testfixtures.ValidLongFormSetup()); err != nil {
		t.Fatalf("SaveSetup: %v", err)
	}
	if err := root.SaveOutline(validToolPatchTestOutline()); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	return root
}

func TestMutationServiceReplaceSetupWritesCanonicalAndDerived(t *testing.T) {
	root := seedMutationProject(t)
	setup, err := root.LoadSetup()
	if err != nil {
		t.Fatalf("LoadSetup: %v", err)
	}
	setup.Theme = "mutation service theme"

	service := toolplane.NewMutationService(root)
	result, err := service.ReplaceSetup(context.Background(), setup, toolplane.MutationOptions{Validate: false, Source: "test"})
	if err != nil {
		t.Fatalf("ReplaceSetup: %v", err)
	}
	if !result.Applied {
		t.Fatal("ReplaceSetup did not report an applied mutation")
	}
	saved, err := root.LoadSetup()
	if err != nil {
		t.Fatalf("LoadSetup after mutation: %v", err)
	}
	if saved.Theme != "mutation service theme" {
		t.Fatalf("theme = %q, want mutation service theme", saved.Theme)
	}
	if !root.Exists(root.SetupMarkdown()) {
		t.Fatal("setup markdown was not rendered by the mutation service")
	}
	if result.Files["setup"] != root.SetupJSON() {
		t.Fatalf("setup file = %q, want %q", result.Files["setup"], root.SetupJSON())
	}
}

func TestMutationServiceUpsertCraftElementsPreservesUnrelatedEntries(t *testing.T) {
	root := seedMutationProject(t)
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist", "notes": "old"},
		"Bo":  {"name": "Bo", "role_in_story": "ally", "notes": "unchanged"},
	}); err != nil {
		t.Fatalf("seed craft: %v", err)
	}

	service := toolplane.NewMutationService(root)
	result, err := service.UpsertCraftElements(context.Background(), store.CraftCharacters, map[string]models.Character{
		"Lin": {
			Name:        "Lin",
			RoleInStory: "protagonist",
			Notes:       "updated by mutation service",
		},
	}, toolplane.MutationOptions{Validate: false, Source: "test"})
	if err != nil {
		t.Fatalf("UpsertCraftElements: %v", err)
	}
	values, err := root.LoadCraftRaw(store.CraftCharacters)
	if err != nil {
		t.Fatalf("LoadCraftRaw: %v", err)
	}
	if values["Lin"]["notes"] != "updated by mutation service" {
		t.Fatalf("Lin was not updated: %#v", values["Lin"])
	}
	if values["Bo"]["notes"] != "unchanged" {
		t.Fatalf("Bo was lost: %#v", values["Bo"])
	}
	if len(result.Changed) == 0 {
		t.Fatal("craft mutation reported no changes")
	}
}

func TestMutationServiceEmptyCraftBatchDoesNotEraseExistingElements(t *testing.T) {
	root := seedMutationProject(t)
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}); err != nil {
		t.Fatalf("seed craft: %v", err)
	}
	service := toolplane.NewMutationService(root)
	result, err := service.UpsertCraftElements(context.Background(), store.CraftCharacters, map[string]models.Character{}, toolplane.MutationOptions{
		Validate: false,
		Source:   "test",
	})
	if err != nil {
		t.Fatalf("UpsertCraftElements(empty): %v", err)
	}
	if result.Applied {
		t.Fatal("empty craft batch should not report an applied write")
	}
	values, err := root.LoadCraftRaw(store.CraftCharacters)
	if err != nil {
		t.Fatalf("LoadCraftRaw: %v", err)
	}
	if _, ok := values["Lin"]; !ok {
		t.Fatalf("empty craft batch erased existing elements: %#v", values)
	}
}

func TestMutationServiceReplaceChapterWritesFinalChapter(t *testing.T) {
	root := seedMutationProject(t)
	service := toolplane.NewMutationService(root)
	result, err := service.ReplaceChapter(context.Background(), "P1-V1-C1", "Lin enters the mine.", toolplane.MutationOptions{Validate: false, Source: "test"})
	if err != nil {
		t.Fatalf("ReplaceChapter: %v", err)
	}
	content, path, err := root.LoadChapterContent("P1-V1-C1")
	if err != nil {
		t.Fatalf("LoadChapterContent: %v", err)
	}
	if !strings.Contains(content, "Lin enters the mine.") {
		t.Fatalf("chapter content = %q", content)
	}
	if result.Files["chapter"] != path {
		t.Fatalf("chapter file = %q, want %q", result.Files["chapter"], path)
	}
	if !root.Exists(path) {
		t.Fatal("chapter path does not exist after mutation")
	}
}

func TestMutationServiceReplaceRecapWritesValidatedShape(t *testing.T) {
	root := seedMutationProject(t)
	recap := &models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Mine",
		Time:            "night",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin enters the mine."},
		Decisions:       []string{"Lin keeps moving."},
		LastLine:        "The tunnel closed.",
		NextOpeningHint: "Lin looks for another exit.",
	}
	service := toolplane.NewMutationService(root)
	if _, err := service.ReplaceRecap(context.Background(), recap, toolplane.MutationOptions{Validate: false, Source: "test"}); err != nil {
		t.Fatalf("ReplaceRecap: %v", err)
	}
	saved, err := root.LoadRecap("P1-V1-C1")
	if err != nil {
		t.Fatalf("LoadRecap: %v", err)
	}
	if saved.LastLine != "The tunnel closed." {
		t.Fatalf("recap last line = %q", saved.LastLine)
	}
}

func TestMutationServiceReplaceOutlineWritesCanonicalAndDerived(t *testing.T) {
	root := seedMutationProject(t)
	outline, err := root.LoadOutline()
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	outline.Parts[0].Volumes[0].Chapters[0].Title = "Mutation Opening"
	service := toolplane.NewMutationService(root)
	result, err := service.ReplaceOutline(context.Background(), outline, toolplane.MutationOptions{Validate: false, Source: "test"})
	if err != nil {
		t.Fatalf("ReplaceOutline: %v", err)
	}
	saved, err := root.LoadOutline()
	if err != nil {
		t.Fatalf("LoadOutline after mutation: %v", err)
	}
	if saved.Parts[0].Volumes[0].Chapters[0].Title != "Mutation Opening" {
		t.Fatalf("chapter title = %q", saved.Parts[0].Volumes[0].Chapters[0].Title)
	}
	if !root.Exists(root.OutlineMarkdown()) {
		t.Fatal("outline markdown was not rendered by the mutation service")
	}
	if result.Files["outline"] != root.OutlineJSON() {
		t.Fatalf("outline file = %q, want %q", result.Files["outline"], root.OutlineJSON())
	}
}

func TestMutationServiceConcurrentCraftUpsertsDoNotLoseEntries(t *testing.T) {
	root := seedMutationProject(t)
	service := toolplane.NewMutationService(root)

	const writers = 16
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("Character %02d", i)
			_, err := service.UpsertCraftElements(context.Background(), store.CraftCharacters, map[string]models.Character{
				name: {
					Name:        name,
					RoleInStory: "supporting",
					Notes:       "concurrent mutation",
				},
			}, toolplane.MutationOptions{Validate: false, Source: "test"})
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent upsert: %v", err)
	}

	values, err := root.LoadCraftRaw(store.CraftCharacters)
	if err != nil {
		t.Fatalf("LoadCraftRaw: %v", err)
	}
	if len(values) != writers {
		t.Fatalf("craft entries = %d, want %d; a concurrent upsert was lost", len(values), writers)
	}
}
