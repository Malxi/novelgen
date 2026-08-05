package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestWriteComposeImproveReport(t *testing.T) {
	dir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	if err := os.MkdirAll(filepath.Join("story", "compose"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := `{"iteration":1,"volume_id":"P1-V1","volume_title":"第一卷：外门修仙","score":76,"changed":true,"summary":"补齐 payoff_contract 与开场节拍","remaining_medium_plus":3}` + "\n"
	if err := os.WriteFile(filepath.Join("story", "compose", "outline_improve_report.jsonl"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeComposeImproveReport(5); err != nil {
		t.Fatalf("writeComposeImproveReport() error: %v", err)
	}
	reports, err := filepath.Glob(filepath.Join("logs", "compose_improve_report_*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("report files = %d, want 1", len(reports))
	}
	content, err := os.ReadFile(reports[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"P1-V1",
		"第一卷：外门修仙",
		"补齐 payoff_contract 与开场节拍",
		"剩余中高优问题: 3",
		"门禁修复后",
		"剩余中高优问题: 5",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestWriteComposeImproveReportEmpty(t *testing.T) {
	dir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldDir) }()

	if err := writeComposeImproveReport(-1); err != nil {
		t.Fatalf("empty report should not error: %v", err)
	}
	reports, _ := filepath.Glob(filepath.Join("logs", "compose_improve_report_*.md"))
	if len(reports) != 0 {
		t.Fatalf("empty report should not create a file, got %d", len(reports))
	}
}

func TestResumeProgressMatchesRun(t *testing.T) {
	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.json")
	outline := &models.Outline{
		Parts: []models.Part{
			{ID: "P1", Volumes: []models.Volume{{ID: "P1-V1"}, {ID: "P1-V2"}}},
		},
	}

	writeProgress := func(iteration int, targets []string) {
		data, _ := json.Marshal(map[string]interface{}{
			"iteration":      iteration,
			"target_volumes": targets,
		})
		if err := os.WriteFile(progressPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeProgress(1, []string{"P1-V1", "P1-V2"})
	if !resumeProgressMatchesRun(progressPath, outline, 1) {
		t.Fatalf("matching progress should be recognized as a resume")
	}

	writeProgress(1, []string{"P1-V2", "P1-V1"})
	if !resumeProgressMatchesRun(progressPath, outline, 1) {
		t.Fatalf("target order should not matter")
	}

	writeProgress(1, []string{"P1-V1"})
	if resumeProgressMatchesRun(progressPath, outline, 1) {
		t.Fatalf("missing target should not match")
	}

	writeProgress(0, []string{"P1-V1", "P1-V2"})
	if resumeProgressMatchesRun(progressPath, outline, 1) {
		t.Fatalf("gate-repair iteration 0 must not match the main run")
	}

	if resumeProgressMatchesRun(filepath.Join(dir, "missing.json"), outline, 1) {
		t.Fatalf("missing progress file must not match")
	}
}

func TestLoadComposeImproveReportEntriesDedupe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.jsonl")
	content := strings.Join([]string{
		`{"iteration":1,"volume_id":"P1-V1","volume_title":"第一版","score":70,"changed":true}`,
		`{"iteration":1,"volume_id":"P1-V2","volume_title":"第二卷","score":80,"changed":true}`,
		`{"iteration":1,"volume_id":"P1-V1","volume_title":"第一版修复","score":90,"changed":true}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := loadComposeImproveReportEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (deduped)", len(entries))
	}
	if entries[0].VolumeID != "P1-V1" || entries[0].Score != 90 {
		t.Fatalf("duplicate should keep the latest entry, got %#v", entries[0])
	}
	if entries[1].VolumeID != "P1-V2" {
		t.Fatalf("order should be preserved, got %#v", entries)
	}
}
