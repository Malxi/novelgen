package agents

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestBuildContinuitySnapshotCarriesDurableState(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{
					ID: "P1-V1",
					Chapters: []models.Chapter{{
						ID:       "P1-V1-C1",
						Title:    "Signal",
						Summary:  "Lin discovers the signal",
						Location: "Mine",
						Scenes: []models.OutlineScene{{
							Order: 1,
							Beats: []string{"Lin enters the mine", "The signal wakes"},
						}},
						StateAnchor: models.StateAnchor{
							Cultivation: "Lv2",
							Location:    "Mine",
							Allies:      []string{"Zhao"},
							Injuries:    []string{"burned arm"},
							KeyItems:    []string{"signal shard"},
						},
						ResourceLedger: []models.ResourceLedgerEntry{{
							Item: "ore", Start: 1, Delta: 2, End: 3,
						}},
						Mysteries: models.ChapterMysteries{
							Planted: []models.MysteryPlanted{{ID: "myst_signal", Clue: "unknown sender"}},
						},
						Enemies: []models.OutlineEnemy{{
							Name: "Queen", BossID: "boss_queen", IsBoss: true, Status: "engaged",
						}},
						StorylineAdvances: []models.StorylineAdvance{{
							StorylineName: "Signal Arc",
							Stage:         "pressure",
							Change:        "Signal points to the hive",
							Pressure:      "Hive can trace it back",
						}},
					}},
				},
				{ID: "P1-V2", Title: "Next"},
			},
		}},
	}

	got := (&ComposeAgent{}).buildContinuitySnapshot(outline, 0, 1)
	for _, want := range []string{
		"Last generated chapter: P1-V1-C1 Signal",
		"cultivation=Lv2",
		"ore=3",
		"myst_signal=unknown sender",
		"boss_queen=engaged in P1-V1-C1",
		"Signal Arc=pressure: Signal points to the hive",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot missing %q:\n%s", want, got)
		}
	}
}
