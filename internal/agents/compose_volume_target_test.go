package agents

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestIdentifyVolumesToImproveResolvesActualVolumeIDs(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V2-C3"},
			{TargetID: "P1-V2"},
			{TargetID: "global"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected P1-V2 indices [0 1], got %v", got[0])
	}
}

func TestIdentifyVolumesToImproveToleratesSinglePartMalformedTarget(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P2-V2-C9"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected malformed P2-V2 to resolve to P1-V2 [0 1], got %v", got[0])
	}
}

func TestPreserveImprovedVolumeIdentityKeepsStableIDs(t *testing.T) {
	original := &models.Volume{
		ID: "P1-V10",
		Chapters: []models.Chapter{
			{ID: "P1-V10-C1"},
			{ID: "P1-V10-C2"},
		},
	}
	improved := &models.Volume{
		ID: "part1-volume10",
		Chapters: []models.Chapter{
			{ID: "p1-v10-c1", Title: "改写一"},
			{ID: "p1-v10-c2", Title: "改写二"},
			{ID: "p1-v10-c3", Title: "新增章"},
		},
	}

	preserveImprovedVolumeIdentity(original, improved)

	if improved.ID != "P1-V10" {
		t.Fatalf("volume id = %q, want P1-V10", improved.ID)
	}
	if improved.Chapters[0].ID != "P1-V10-C1" || improved.Chapters[1].ID != "P1-V10-C2" {
		t.Fatalf("chapter ids were not preserved: %#v", improved.Chapters)
	}
	if improved.Chapters[2].ID != "p1-v10-c3" {
		t.Fatalf("extra chapter id should be untouched, got %q", improved.Chapters[2].ID)
	}
}

func TestAppealAndPayoffBriefsSkipEmptyAndKeepUpgradePath(t *testing.T) {
	appeal := formatAppealEngineBrief("Appeal", &models.AppealEngine{
		Appeal:      "wins through timing",
		UpgradePath: "predicts longer chains",
	})
	if !strings.Contains(appeal, "upgrade_path=predicts longer chains") {
		t.Fatalf("appeal brief missing upgrade path: %q", appeal)
	}

	emptyVolume := formatVolumeBrief(models.Volume{
		ID:             "P1-V1",
		Title:          "Empty",
		Summary:        "No payoff yet",
		PayoffContract: &models.VolumePayoffContract{},
	})
	if strings.Contains(emptyVolume, "Payoff:") {
		t.Fatalf("empty payoff contract should not be formatted:\n%s", emptyVolume)
	}

	emptyChapter := formatChapterBrief(models.Chapter{
		ID:            "P1-V1-C1",
		Title:         "Empty",
		Summary:       "No chapter payoff yet",
		ChapterPayoff: &models.ChapterPayoff{},
	})
	if strings.Contains(emptyChapter, "payoff:") {
		t.Fatalf("empty chapter payoff should not be formatted:\n%s", emptyChapter)
	}

	partialChapter := formatChapterBrief(models.Chapter{
		ID:      "P1-V1-C2",
		Title:   "Partial",
		Summary: "Only a payoff moment is known",
		ChapterPayoff: &models.ChapterPayoff{
			PayoffMoment: "The guard salutes an empty corner.",
		},
	})
	if strings.Contains(partialChapter, "->") || !strings.Contains(partialChapter, "moment=The guard salutes an empty corner.") {
		t.Fatalf("partial chapter payoff should format only populated fields:\n%s", partialChapter)
	}
}
