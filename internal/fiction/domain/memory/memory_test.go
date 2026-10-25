package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

type fakeEmbedder struct {
	model     string
	batches   [][]string
	dimension int
}

func (f *fakeEmbedder) Model() string { return f.model }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.batches = append(f.batches, append([]string(nil), texts...))
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		lower := strings.ToLower(text)
		vector := make([]float32, 4)
		if strings.Contains(lower, "blue") || strings.Contains(lower, "蓝") {
			vector[0] = 1
		}
		if strings.Contains(lower, "red") || strings.Contains(lower, "红") {
			vector[1] = 1
		}
		if strings.Contains(lower, "door") || strings.Contains(lower, "门") {
			vector[2] = 1
		}
		vector[3] = 0.1
		vectors[i] = vector
	}
	return vectors, nil
}

func (f *fakeEmbedder) embeddedCount() int {
	total := 0
	for _, batch := range f.batches {
		total += len(batch)
	}
	return total
}

type failingEmbedder struct{}

func (failingEmbedder) Model() string { return "failing-embed" }
func (failingEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedding service unavailable")
}

func memoryFixture(t *testing.T) *store.Root {
	t.Helper()
	root := store.At(t.TempDir())
	if err := root.SaveConfig(&models.ProjectConfig{Name: "Memory Fixture", Language: "en"}); err != nil {
		t.Fatal(err)
	}
	setup := &models.StorySetup{
		ProjectName: "Memory Fixture",
		Premise:     "A sealed door remembers every visitor.",
		Theme:       "Memory is leverage.",
		Storylines: []models.Storyline{{
			Name: "Door mystery", Importance: 9, Payoff: "Reveal who spoke behind the door.",
		}},
	}
	if err := root.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	outline := memoryOutline()
	if err := root.SaveOutline(outline); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]models.Character{
		"Lin": {Name: "Lin", Aliases: []string{"Captain Lin"}, Motivation: "Find the sealed door", Voice: "clipped and suspicious"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftItems), map[string]models.Item{
		"Blue Key": {Name: "Blue Key", Owner: "Lin", Significance: "opens the sealed door"},
	}); err != nil {
		t.Fatal(err)
	}

	chapters := map[string]string{
		"P1-V1-C1": "# Opening\n\nLin finds a blue key inside the mine.\n\nA voice whispers behind the sealed door.\n\nLin marks the door with chalk and hides the key.\n\nOutside, rain erases every footprint.\n\nLin decides the voice must belong to someone still alive.",
		"P1-V1-C2": "# Pressure\n\nLin studies the blue key under a red lamp.\n\nThe door remains sealed while miners pass through the tunnel.\n\nLin compares the chalk mark with the mine map.\n\nThe map shows a chamber that should not exist.\n\nLin promises to open the door before the next shift.",
		"P1-V1-C3": "# Return\n\nLin returns to the sealed door with the blue key.",
		"P1-V1-C4": "# Revelation\n\nA future-only nebula reveals the answer: the red queen spoke behind the door. Lin opens it with the blue key.",
	}
	for chapterID, content := range chapters {
		if err := root.SaveChapterContent(chapterID, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := root.SaveRecap(&models.ChapterRecap{
		ChapterID: "P1-V1-C1", Title: "Opening", Location: "Mine",
		Present: []string{"Lin"}, Unresolved: []string{"Find who spoke behind the door"},
		Promises: []string{"Open the sealed door"}, Status: []string{"Lin has the blue key"},
		Items: []string{"Blue Key owned by Lin"}, LastLine: "Lin hears the voice again.",
		NextOpeningHint: "Lin checks the key under better light.",
	}); err != nil {
		t.Fatal(err)
	}
	if err := root.SaveRecap(&models.ChapterRecap{
		ChapterID: "P1-V1-C2", Title: "Pressure", Location: "Mine",
		Present: []string{"Lin"}, Unresolved: []string{"Find who spoke behind the door"},
		Promises: []string{"Open the sealed door"}, Status: []string{"Lin still has the blue key"},
		LastLine: "The map shows an impossible chamber.", NextOpeningHint: "Lin returns to the door.",
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func memoryOutline() *models.Outline {
	newChapter := func(id, title, summary string) models.Chapter {
		return models.Chapter{
			ID: id, Title: title, Summary: summary,
			Characters: []string{"Lin"}, Location: "Mine",
			Scenes: []models.OutlineScene{{
				Order: 1, POV: "Lin", Location: "Mine", Characters: []string{"Lin"},
			}},
			StorylineAdvances: []models.StorylineAdvance{{
				StorylineName: "Door mystery", Change: summary,
			}},
		}
	}
	c1 := newChapter("P1-V1-C1", "Opening", "Lin finds a blue key and hears a voice behind the door.")
	c1.Mysteries.Planted = []models.MysteryPlanted{{
		ID: "myst_door", Clue: "Someone whispered behind the sealed door.", Horizon: "this_volume",
	}}
	c2 := newChapter("P1-V1-C2", "Pressure", "Lin studies the key and the impossible mine map.")
	c3 := newChapter("P1-V1-C3", "Return", "Lin returns to the door.")
	c4 := newChapter("P1-V1-C4", "Revelation", "The red queen is revealed behind the door.")
	c4.Mysteries.Resolved = []models.MysteryResolved{{ID: "myst_door", Resolution: "The red queen spoke."}}
	return &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{c1, c2, c3, c4},
		}},
	}}}
}

func TestBuildConstructsGraphScenesAndPromises(t *testing.T) {
	root := memoryFixture(t)
	result, err := Build(context.Background(), root, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Index.Stats.Chapters != 4 || result.Index.Stats.Manuscripts != 4 || result.Index.Stats.Scenes == 0 {
		t.Fatalf("unexpected stats: %#v", result.Index.Stats)
	}
	if result.Index.Stats.Nodes == 0 || result.Index.Stats.Edges == 0 {
		t.Fatalf("graph was not built: %#v", result.Index.Stats)
	}
	if !hasNode(result.Index, "character:lin") || !hasNode(result.Index, "item:blue key") || !hasNode(result.Index, "storyline:door mystery") {
		t.Fatalf("missing canonical graph nodes")
	}
	if !hasPromiseText(result.Index.Promises, "Someone whispered behind the sealed door.") {
		t.Fatalf("mystery promise missing: %#v", result.Index.Promises)
	}
	if result.Index.Vector.Enabled {
		t.Fatalf("vectors should be disabled without an embedder")
	}
}

func TestQueryRespectsChapterOrderRecapAndPromise(t *testing.T) {
	root := memoryFixture(t)
	result, err := Query(context.Background(), root, QueryOptions{
		ChapterID: "P1-V1-C3", Intent: "chapter-write", Focus: "blue key sealed door",
		MaxChars: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta.Status != "ephemeral" {
		t.Fatalf("status = %q, want ephemeral", result.Meta.Status)
	}
	if len(result.Retrieved) == 0 {
		t.Fatalf("expected retrieved scenes")
	}
	for _, scene := range result.Retrieved {
		if scene.ChapterID == "P1-V1-C3" || scene.ChapterID == "P1-V1-C4" {
			t.Fatalf("future/current chapter leaked into write context: %#v", scene)
		}
	}
	if !strings.Contains(result.MustInclude.PreviousRecap, "returns to the door") {
		t.Fatalf("previous recap missing: %#v", result.MustInclude)
	}
	if !containsString(result.MustInclude.CurrentState, "Lin still has the blue key") {
		t.Fatalf("current state missing: %#v", result.MustInclude.CurrentState)
	}
	if len(result.Promises) == 0 || !hasPromiseKind(result.Promises, "mystery", "upcoming_due") {
		t.Fatalf("expected due mystery promise: %#v", result.Promises)
	}
}

func TestRefreshPersistsIndexAndReusesUnchangedVectors(t *testing.T) {
	root := memoryFixture(t)
	embedder := &fakeEmbedder{model: "fake-embed-v1"}
	first, err := Refresh(context.Background(), root, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "rebuilt" || !first.Vector.Enabled || first.Vector.Count == 0 {
		t.Fatalf("unexpected first refresh: %#v", first)
	}
	firstCalls := embedder.embeddedCount()
	if firstCalls == 0 {
		t.Fatalf("expected embeddings to be generated")
	}

	second, err := Refresh(context.Background(), root, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "up_to_date" {
		t.Fatalf("second refresh status = %q", second.Status)
	}
	if embedder.embeddedCount() != firstCalls {
		t.Fatalf("unchanged refresh embedded again: %d -> %d", firstCalls, embedder.embeddedCount())
	}

	if err := root.SaveChapterContent("P1-V1-C2", "# Pressure\n\nLin studies the blue key under a red lamp.\n\nThe door remains sealed.\n\nA new clue appears in the dust."); err != nil {
		t.Fatal(err)
	}
	third, err := Refresh(context.Background(), root, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if third.Status != "rebuilt" {
		t.Fatalf("third refresh status = %q", third.Status)
	}
	if embedder.embeddedCount() <= firstCalls {
		t.Fatalf("changed chapter should generate at least one new vector")
	}
	if third.Vector.Reused == 0 {
		t.Fatalf("expected unchanged scene vectors to be reused: %#v", third.Vector)
	}
}

func TestQueryUsesPersistedVectorsWithoutLeakingFutureScenes(t *testing.T) {
	root := memoryFixture(t)
	embedder := &fakeEmbedder{model: "fake-embed-v1"}
	if _, err := Refresh(context.Background(), root, embedder); err != nil {
		t.Fatal(err)
	}
	result, err := Query(context.Background(), root, QueryOptions{
		ChapterID: "P1-V1-C3", Intent: "voice-echo", Focus: "future red nebula",
		MaxChars: 1800, Embedder: embedder,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Meta.VectorUsed || result.Meta.VectorCandidates == 0 {
		t.Fatalf("expected vector retrieval: %#v", result.Meta)
	}
	for _, scene := range result.Retrieved {
		if scene.ChapterOrder > result.ChapterOrder {
			t.Fatalf("future vector scene leaked: %#v", scene)
		}
	}
}

func TestStalePersistedIndexFallsBackToEphemeralQuery(t *testing.T) {
	root := memoryFixture(t)
	if _, err := Refresh(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if err := root.SaveChapterContent("P1-V1-C2", "# Pressure\n\nLin updates the map after a new tunnel collapse."); err != nil {
		t.Fatal(err)
	}
	result, err := Query(context.Background(), root, QueryOptions{ChapterID: "P1-V1-C3", Intent: "continuity"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Meta.Status != "ephemeral" {
		t.Fatalf("stale index status = %q, want ephemeral", result.Meta.Status)
	}
	if !containsWarning(result.Warnings, "stale") {
		t.Fatalf("missing stale warning: %#v", result.Warnings)
	}
}

func TestEmbeddingFailureDegradesToGraphAndLexical(t *testing.T) {
	root := memoryFixture(t)
	result, err := Refresh(context.Background(), root, failingEmbedder{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Vector.Enabled || result.Vector.Count != 0 {
		t.Fatalf("failed embeddings should disable vectors: %#v", result.Vector)
	}
	if !containsWarning(result.Warnings, "embedding failed") {
		t.Fatalf("missing embedding failure warning: %#v", result.Warnings)
	}
	query, err := Query(context.Background(), root, QueryOptions{ChapterID: "P1-V1-C3", Focus: "blue key"})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Retrieved) == 0 {
		t.Fatalf("graph/lexical retrieval should still work")
	}
}

func TestRefreshRecreatesMissingVectorSidecar(t *testing.T) {
	root := memoryFixture(t)
	embedder := &fakeEmbedder{model: "fake-embed-v1"}
	if _, err := Refresh(context.Background(), root, embedder); err != nil {
		t.Fatal(err)
	}
	if err := root.Remove(root.NarrativeMemoryVectorsFile()); err != nil {
		t.Fatal(err)
	}
	result, err := Refresh(context.Background(), root, embedder)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "rebuilt" || result.Vector.Generated == 0 {
		t.Fatalf("missing vector sidecar should rebuild: %#v", result)
	}
	if !containsWarning(result.Warnings, "incomplete") {
		t.Fatalf("missing sidecar warning: %#v", result.Warnings)
	}
}

func TestDirtyMarkerInvalidatesCacheAndRefreshClearsIt(t *testing.T) {
	root := memoryFixture(t)
	if marked, err := MarkNarrativeMemoryDirty(root, "chapter:P1-V1-C2"); err != nil || marked {
		t.Fatalf("dirty marker should no-op without a cache: marked=%t err=%v", marked, err)
	}
	if _, err := Refresh(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	marked, err := MarkNarrativeMemoryDirty(root, "chapter:P1-V1-C2")
	if err != nil || !marked {
		t.Fatalf("mark dirty = %t, %v", marked, err)
	}
	if got := Inspect(root, ""); got.Status != "stale" || !containsString(got.ChangedSources, "dirty:chapter:P1-V1-C2") {
		t.Fatalf("inspect dirty marker = %#v", got)
	}
	query, err := Query(context.Background(), root, QueryOptions{ChapterID: "P1-V1-C3"})
	if err != nil {
		t.Fatal(err)
	}
	if query.Meta.Status != "ephemeral" || !containsWarning(query.Warnings, "marked stale") {
		t.Fatalf("dirty query = %#v / %#v", query.Meta, query.Warnings)
	}
	if _, err := Refresh(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(root, ""); got.Status != "fresh" {
		t.Fatalf("inspect after refresh = %#v", got)
	}
	if root.Exists(root.NarrativeMemoryDirtyFile()) {
		t.Fatalf("dirty marker was not cleared")
	}
}

func TestInspectReportsFreshAndStaleIndex(t *testing.T) {
	root := memoryFixture(t)
	if got := Inspect(root, ""); got.Status != "missing" {
		t.Fatalf("initial inspect = %q, want missing", got.Status)
	}
	if _, err := Refresh(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(root, ""); got.Status != "fresh" {
		t.Fatalf("inspect = %q, want fresh: %#v", got.Status, got)
	}
	if err := root.SaveChapterContent("P1-V1-C1", "# Opening\n\nA changed manuscript."); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(root, ""); got.Status != "stale" || !containsString(got.ChangedSources, "chapter:P1-V1-C1") {
		t.Fatalf("inspect = %#v, want stale chapter source", got)
	}
}

func TestInspectTreatsEmbeddingModelChangeAsStale(t *testing.T) {
	root := memoryFixture(t)
	if _, err := Refresh(context.Background(), root, &fakeEmbedder{model: "embed-v1"}); err != nil {
		t.Fatal(err)
	}
	if got := Inspect(root, "embed-v1"); got.Status != "fresh" {
		t.Fatalf("same model inspect = %#v", got)
	}
	if got := Inspect(root, "embed-v2"); got.Status != "stale" || !containsString(got.ChangedSources, "embedding_model") {
		t.Fatalf("model change inspect = %#v", got)
	}
}

func TestItemOwnershipSpansFollowAcquireAndLossEvents(t *testing.T) {
	chapters := []flatMemoryChapter{
		{Order: 1, Chapter: &models.Chapter{ID: "C1", Events: []models.Event{{
			Actor: "Lin", Action: models.ActionAcquire, Target: "Blue Key", TargetType: models.TargetTypeItem,
		}}}},
		{Order: 2, Chapter: &models.Chapter{ID: "C2"}},
		{Order: 3, Chapter: &models.Chapter{ID: "C3", Events: []models.Event{{
			Actor: "Lin", Action: models.ActionLose, Target: "Blue Key", TargetType: models.TargetTypeItem,
		}}}},
	}
	spans, timed := itemOwnershipSpans(chapters)
	if !timed["blue key"] || len(spans) != 1 {
		t.Fatalf("unexpected ownership spans: %#v timed=%#v", spans, timed)
	}
	if spans[0].Actor != "Lin" || spans[0].FromChapter != 1 || spans[0].ToChapter != 3 {
		t.Fatalf("unexpected ownership validity: %#v", spans[0])
	}
}

func hasNode(index *Index, id string) bool {
	for _, node := range index.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func hasPromiseText(promises []PromiseRecord, text string) bool {
	for _, promise := range promises {
		if promise.Text == text {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsWarning(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(fragment)) {
			return true
		}
	}
	return false
}

func hasPromiseKind(values []PromiseHit, kind, reason string) bool {
	for _, value := range values {
		if value.Kind == kind && value.Reason == reason {
			return true
		}
	}
	return false
}
