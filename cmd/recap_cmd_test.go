package cmd

import (
	"strings"
	"testing"

	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
)

func TestEffectiveRecapConcurrency(t *testing.T) {
	if got := effectiveRecapConcurrency(0, 3, false); got != 1 {
		t.Fatalf("concurrency = %d, want 1", got)
	}
	if got := effectiveRecapConcurrency(10, 3, false); got != 3 {
		t.Fatalf("concurrency = %d, want 3", got)
	}
	if got := effectiveRecapConcurrency(4, 3, true); got != 1 {
		t.Fatalf("agent-sdk concurrency = %d, want 1", got)
	}
}

func TestValidateRecapAgentApplyOption(t *testing.T) {
	if err := validateRecapAgentApplyOption(true, true); err != nil {
		t.Fatalf("expected --agent-sdk --agent-apply to pass, got %v", err)
	}
	err := validateRecapAgentApplyOption(false, true)
	if err == nil || !strings.Contains(err.Error(), "--agent-apply requires --agent-sdk") {
		t.Fatalf("expected --agent-apply validation error, got %v", err)
	}
}

func TestResolveAgentAppliedRecapDataUsesSavedPatchResult(t *testing.T) {
	store := recap.NewStore(t.TempDir())
	before := testRecap("P1-V1-C1", "旧地点")
	after := testRecap("P1-V1-C1", "新地点")
	if err := store.Save(before); err != nil {
		t.Fatalf("save before recap: %v", err)
	}
	if err := store.Save(after); err != nil {
		t.Fatalf("save after recap: %v", err)
	}

	got, applied := resolveAgentAppliedRecapData(logger.GetLogger(), 0, store, "P1-V1-C1", before, before)
	if !applied {
		t.Fatalf("applied = false, want true")
	}
	if got == nil || got.Location != "新地点" {
		t.Fatalf("resolved recap = %#v, want saved patch result", got)
	}
}

func TestResolveAgentAppliedRecapDataDoesNotTreatReturnedJSONAsApply(t *testing.T) {
	store := recap.NewStore(t.TempDir())
	before := testRecap("P1-V1-C1", "旧地点")
	returned := testRecap("P1-V1-C1", "仅返回的新地点")
	if err := store.Save(before); err != nil {
		t.Fatalf("save before recap: %v", err)
	}

	got, applied := resolveAgentAppliedRecapData(logger.GetLogger(), 0, store, "P1-V1-C1", before, returned)
	if applied {
		t.Fatalf("applied = true, want false")
	}
	if got == nil || got.Location != "旧地点" {
		t.Fatalf("resolved recap = %#v, want unchanged saved recap", got)
	}
}

func testRecap(chapterID, location string) *models.ChapterRecap {
	return &models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "测试章节",
		Location:        location,
		Time:            "同夜",
		Present:         []string{"林野"},
		PlotBeats:       []string{"林野完成测试动作。"},
		LastLine:        "他抬头看向出口。",
		NextOpeningHint: "他抬头看向出口后，继续前进。",
	}
}
