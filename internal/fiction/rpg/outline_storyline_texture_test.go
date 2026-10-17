package rpg

import "testing"

func TestValidateStorylineTextureIsSuggestionOnly(t *testing.T) {
	outline := &StoryOutline{
		Parts: []StoryPart{{
			ID: "P1",
			Volumes: []StoryVolume{{
				ID: "P1-V1",
				Chapters: []StoryChapter{
					{
						ID: "P1-V1-C1",
						Events: []StoryEvent{{
							Type:    "storyline",
							Subject: "Signal War",
							Change:  "revealed artificial origin",
						}},
					},
					{
						ID: "P1-V1-C2",
						StorylineAdvances: []StorylineAdvance{{
							StorylineName: "Signal War",
							Stage:         "pressure",
							Change:        "The fleet hides the signal report.",
						}},
					},
				},
			}},
		}},
	}

	validator := NewOutlineValidator(outline)
	validator.validateStorylineTexture()

	if len(validator.Issues) != 0 {
		t.Fatalf("issues = %#v, want none", validator.Issues)
	}
	if len(validator.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", validator.Warnings)
	}
	if len(validator.Suggestions) != 2 {
		t.Fatalf("suggestions = %d, want 2: %#v", len(validator.Suggestions), validator.Suggestions)
	}
	if validator.Suggestions[0].Type != "storyline_texture" {
		t.Fatalf("first suggestion type = %q", validator.Suggestions[0].Type)
	}
}
