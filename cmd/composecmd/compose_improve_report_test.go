package composecmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

func TestWriteComposeImproveReport(t *testing.T) {
	book := newComposeTestBook(t)

	entry := `{"iteration":1,"volume_id":"P1-V1","volume_title":"第一卷：外门修仙","score":76,"changed":true,"summary":"补齐 payoff_contract 与开场节拍","remaining_medium_plus":3}` + "\n"
	if err := book.Root().WriteText(book.Root().ComposeFile("outline_improve_report.jsonl"), entry); err != nil {
		t.Fatal(err)
	}

	if err := writeComposeImproveReport(book, 5); err != nil {
		t.Fatalf("writeComposeImproveReport() error: %v", err)
	}
	reports, err := filepath.Glob(book.Root().LogFile("compose_improve_report_*.md"))
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
	book := newComposeTestBook(t)

	if err := writeComposeImproveReport(book, -1); err != nil {
		t.Fatalf("empty report should not error: %v", err)
	}
	reports, _ := filepath.Glob(book.Root().LogFile("compose_improve_report_*.md"))
	if len(reports) != 0 {
		t.Fatalf("empty report should not create a file, got %d", len(reports))
	}
}

// newComposeTestBook returns a project rooted in a temporary directory, so tests
// exercise compose against an explicit root instead of chdir-ing the process
// into a fixture.
func newComposeTestBook(t *testing.T) *project.Book {
	t.Helper()
	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open temp project: %v", err)
	}
	if err := root.WriteText(root.NovelJSON(), `{"name":"compose-test"}`); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}
	return project.Open(root)
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
	if !resumeProgressMatchesRun(store.At(dir), progressPath, outline, 1) {
		t.Fatalf("matching progress should be recognized as a resume")
	}

	writeProgress(1, []string{"P1-V2", "P1-V1"})
	if !resumeProgressMatchesRun(store.At(dir), progressPath, outline, 1) {
		t.Fatalf("target order should not matter")
	}

	writeProgress(1, []string{"P1-V1"})
	if resumeProgressMatchesRun(store.At(dir), progressPath, outline, 1) {
		t.Fatalf("missing target should not match")
	}

	writeProgress(0, []string{"P1-V1", "P1-V2"})
	if resumeProgressMatchesRun(store.At(dir), progressPath, outline, 1) {
		t.Fatalf("gate-repair iteration 0 must not match the main run")
	}

	if resumeProgressMatchesRun(store.At(dir), filepath.Join(dir, "missing.json"), outline, 1) {
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
	entries, err := loadComposeImproveReportEntries(store.At(dir), path)
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
