package continuity

import (
	"strings"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

func TestExtractLastNonEmptyLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "first\nsecond", "second"},
		{"trailing blanks", "first\nsecond\n\n   \n", "second"},
		{"crlf", "a\r\nb\r\n", "b"},
		{"single", "only", "only"},
		{"empty", "", ""},
		{"only whitespace", "  \n\t\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExtractLastNonEmptyLine(c.in); got != c.want {
				t.Fatalf("ExtractLastNonEmptyLine(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestBuildOfflineRecapAnchorsNextOpening(t *testing.T) {
	ch := &models.Chapter{
		ID:         "P1-V1-C1",
		Title:      "Opening",
		Location:   "Arena",
		Characters: []string{"Hero", "Mentor"},
		Summary:    "Hero wins the public test",
	}

	r := BuildOfflineRecap(ch, "line one\nHero steps forward.\n")

	if r.ChapterID != ch.ID || r.Title != ch.Title || r.Location != "Arena" {
		t.Fatalf("recap identity = %#v", r)
	}
	if r.LastLine != "Hero steps forward." || r.Cliffhanger != r.LastLine {
		t.Fatalf("last line/cliffhanger = %q/%q", r.LastLine, r.Cliffhanger)
	}
	if len(r.Unresolved) != 1 || r.Unresolved[0] != r.LastLine {
		t.Fatalf("unresolved = %#v", r.Unresolved)
	}
	if !strings.Contains(r.NextOpeningHint, r.LastLine) || !strings.Contains(r.NextOpeningHint, "Arena") {
		t.Fatalf("next opening hint should anchor on the last line and location: %q", r.NextOpeningHint)
	}
	if len(r.Present) != 2 || len(r.PlotBeats) != 1 || r.PlotBeats[0] != ch.Summary {
		t.Fatalf("present/beats = %#v/%#v", r.Present, r.PlotBeats)
	}
}

func TestBuildOfflineRecapWithoutTextOrLocation(t *testing.T) {
	blank := BuildOfflineRecap(&models.Chapter{ID: "P1-V1-C1"}, "   \n")
	if blank.LastLine != "" || blank.Cliffhanger != "" || blank.NextOpeningHint != "" || blank.Unresolved != nil {
		t.Fatalf("blank text should produce no anchor: %#v", blank)
	}

	noLocation := BuildOfflineRecap(&models.Chapter{ID: "P1-V1-C1"}, "tail line")
	if !strings.Contains(noLocation.NextOpeningHint, "tail line") {
		t.Fatalf("hint should still carry the last line: %q", noLocation.NextOpeningHint)
	}
	if strings.Contains(noLocation.NextOpeningHint, "仍在") {
		t.Fatalf("hint must not name a location when the chapter has none: %q", noLocation.NextOpeningHint)
	}
}

func TestCollectKnownCharactersDedupesTrimsAndSorts(t *testing.T) {
	volume := &models.Volume{Chapters: []models.Chapter{
		{Characters: []string{" Hero ", "Mentor", ""}},
		{Characters: []string{"Hero", "Rival"}},
	}}

	got := CollectKnownCharacters(volume)
	want := []string{"Hero", "Mentor", "Rival"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("CollectKnownCharacters = %#v, want %#v", got, want)
	}
	if CollectKnownCharacters(nil) != nil {
		t.Fatal("nil volume should return nil")
	}
}

func TestContainsStr(t *testing.T) {
	if !ContainsStr([]string{"a", "b"}, "b") {
		t.Fatal("ContainsStr should find an existing value")
	}
	if ContainsStr([]string{"a"}, "b") || ContainsStr(nil, "a") {
		t.Fatal("ContainsStr should report missing values as false")
	}
}

func TestRecapGateFeedbackListsReasonsAndRequirements(t *testing.T) {
	got := RecapGateFeedback([]string{"missing location", "   "}, &models.ChapterRecap{})

	for _, want := range []string{
		"Recap minimal gate failed",
		"- missing location",
		"Hard requirements:",
		"- location:",
		"- next_opening_hint:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("feedback missing %q:\n%s", want, got)
		}
	}
}

func TestTransitionFixBuildersHandleNilAndCarryMarkers(t *testing.T) {
	if BuildTeleportFixInstruction(nil) != "" || BuildTransitionBridgePatchRequest(nil) != "" {
		t.Fatal("nil recap should produce empty instructions")
	}

	recap := &models.ChapterRecap{LastLine: "The gate closes."}
	if got := BuildTeleportFixInstruction(recap); !strings.Contains(got, "补转场桥段") {
		t.Fatalf("teleport fix instruction = %q", got)
	}
	if got := BuildTransitionBridgePatchRequest(recap); !strings.Contains(got, "TRANSITION_BRIDGE") {
		t.Fatalf("bridge patch request = %q", got)
	}
}

func TestApplyHeuristicTransitionChecksAugmentsReview(t *testing.T) {
	volume := &models.Volume{ID: "P1-V1", Chapters: []models.Chapter{
		{ID: "P1-V1-C1", Characters: []string{"Hero", "Rival"}},
		{ID: "P1-V1-C2", Characters: []string{"Hero", "Mentor"}},
	}}
	drafts := map[string]string{
		"P1-V1-C1": "Hero enters the arena.",
		"P1-V1-C2": "Rival blocks the path. Rival draws a blade. Hero steps back.",
	}
	review := &models.VolumeReview{Reviews: []models.DraftReview{
		{ChapterID: "P1-V1-C1"},
		{ChapterID: "P1-V1-C2"},
	}}

	ApplyHeuristicTransitionChecks(nil, volume, drafts, review)

	if got := review.Reviews[1].CharacterPresence.Issues; !sliceContainsSubstring(got, "Mentor") {
		t.Fatalf("expected a missing-character issue for Mentor, got %#v", got)
	}
	if got := review.Reviews[1].Suggestions; !sliceContainsSubstring(got, "CHARACTER_PRESENCE_PATCH") {
		t.Fatalf("expected a character presence patch request, got %#v", got)
	}
	if got := review.Reviews[1].CharacterPresence.Issues; !sliceContainsSubstring(got, "Rival") {
		t.Fatalf("expected an unexpected-opening-character issue for Rival, got %#v", got)
	}
}

func TestApplyHeuristicTransitionChecksIgnoresNilInputs(t *testing.T) {
	ApplyHeuristicTransitionChecks(nil, nil, nil, nil)
	ApplyHeuristicTransitionChecks(nil, &models.Volume{}, nil, &models.VolumeReview{})
}

func TestLoadPreviousRecapJSONUsesInjectedRoot(t *testing.T) {
	root := store.MemRoot()
	if err := root.SaveRecap(&models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		LastLine:        "Lin opens the door.",
		NextOpeningHint: "紧接上一章最后一刻。",
	}); err != nil {
		t.Fatalf("SaveRecap: %v", err)
	}

	ambient := store.MemRoot()
	store.SetDefault(ambient)
	defer store.ResetDefault()

	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{
				{ID: "P1-V1-C1", Title: "One"},
				{ID: "P1-V1-C2", Title: "Two"},
			},
		}},
	}}}
	book := project.Open(root)
	got := LoadPreviousRecapJSON(book, outline, &models.Chapter{ID: "P1-V1-C2"})
	if !strings.Contains(got, "P1-V1-C1") || !strings.Contains(got, "Lin opens the door") {
		t.Fatalf("recap = %q, want the injected root's recap", got)
	}
	if names := ambient.FS().(*store.MemFS).Names(); len(names) != 0 {
		t.Fatalf("ambient project was touched: %v", names)
	}
}

func sliceContainsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
