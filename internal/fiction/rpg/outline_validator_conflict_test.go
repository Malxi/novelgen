package rpg

import "testing"

func TestValidatePlotLogicAcceptsEventResultAsConflictOutcome(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{{
				ID:          "P1-V1-C1",
				Title:       "Ambush",
				Summary:     "The hero escapes the ambush.",
				Conflict:    "The road is blocked by raiders.",
				Beats:       []string{"Raiders block the road.", "Hero leaves the ridge."},
				StateChange: "Hero escaped with the map.",
				Events: []StoryEvent{{
					Type:   "combat",
					Action: "combat",
					Target: "raiders",
					Result: "Hero defeats the raiders and escapes with the map.",
				}},
			}},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validatePlotLogic()

	assertNoSuggestionTypeAt(t, validator.Suggestions, "logic", "P1-V1-C1")
}

func TestValidatePlotLogicStillSuggestsWhenConflictHasNoOutcome(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{{
				ID:       "P1-V1-C1",
				Title:    "Ambush",
				Conflict: "The road is blocked by raiders.",
				Beats:    []string{"Raiders block the road.", "Hero sees smoke."},
			}},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validatePlotLogic()

	assertHasSuggestionTypeAt(t, validator.Suggestions, "logic", "P1-V1-C1")
}

func assertNoSuggestionTypeAt(t *testing.T, suggestions []OutlineSuggestion, typ, location string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Type == typ && suggestion.Location == location {
			t.Fatalf("unexpected suggestion at %s: %#v", location, suggestion)
		}
	}
}

func assertHasSuggestionTypeAt(t *testing.T, suggestions []OutlineSuggestion, typ, location string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Type == typ && suggestion.Location == location {
			return
		}
	}
	t.Fatalf("missing suggestion type %q at %s: %#v", typ, location, suggestions)
}
