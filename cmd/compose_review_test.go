package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/models"
)

func TestLoadReviewSuggestionReports(t *testing.T) {
	dir := t.TempDir()
	reviewPath := filepath.Join(dir, "review.json")
	checkPath := filepath.Join(dir, "check.json")
	writeReport := func(path string, report models.ReviewResult) {
		t.Helper()
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write report: %v", err)
		}
	}
	writeReport(reviewPath, models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{{TargetID: "P1-V1", Priority: "high", Issue: "a"}},
	})
	writeReport(checkPath, models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{{TargetID: "P1-V2", Priority: "medium", Issue: "b"}},
	})

	got, err := loadReviewSuggestionReports(reviewPath + "," + checkPath)
	if err != nil {
		t.Fatalf("loadReviewSuggestionReports() error = %v", err)
	}
	if len(got) != 2 || got[0].TargetID != "P1-V1" || got[1].TargetID != "P1-V2" {
		t.Fatalf("suggestions = %#v", got)
	}
	if _, err := loadReviewSuggestionReports(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatalf("loadReviewSuggestionReports with missing file should fail")
	}
}

func TestResolveGlobalVolumeID(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID:      "P1",
		Volumes: []models.Volume{{ID: "P1-V1"}, {ID: "P1-V2"}},
	}}}
	got, err := resolveGlobalVolumeID(outline, 2)
	if err != nil || got != "P1-V2" {
		t.Fatalf("resolveGlobalVolumeID(2) = %q, %v", got, err)
	}
	if _, err := resolveGlobalVolumeID(outline, 3); err == nil {
		t.Fatalf("out-of-range volume index should fail")
	}
}
