package toolplane_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

func TestToolPatchChapterDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	outline := validToolPatchTestOutline()
	if err := writeToolTestOutline(root, outline); err != nil {
		t.Fatal(err)
	}
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	chapterPath := filepath.Join(chapterDir, "chapter-P1-V1-C1.md")
	if err := os.WriteFile(chapterPath, []byte("# Opening\n\nLin starts here."), 0644); err != nil {
		t.Fatal(err)
	}

	storeRoot := toolRoot(t, root)

	content := "# Opening\n\n" + strings.Repeat("Lin repairs the star core inside the mine and keeps the signal stable. ", 140)
	patch := fmt.Sprintf(`{"content":%q}`, content)

	originalFlags := toolplane.ToolPatchFlags
	defer func() { toolplane.ToolPatchFlags = originalFlags }()
	toolplane.ToolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{ID: "P1-V1-C1", PatchJSON: patch}
	var dryOut bytes.Buffer
	dryCmd := bindProject(&cobra.Command{}, storeRoot)
	dryCmd.SetOut(&dryOut)
	if err := toolplane.RunToolPatch(dryCmd, []string{"chapter"}); err != nil {
		t.Fatalf("dry-run chapter patch: %v", err)
	}
	var dryResult toolplane.ToolPatchResult
	if err := json.Unmarshal(dryOut.Bytes(), &dryResult); err != nil {
		t.Fatalf("decode dry-run result: %v\n%s", err, dryOut.String())
	}
	if dryResult.Applied || !dryResult.DryRun || dryResult.Target != "chapter" || dryResult.Check == nil || dryResult.Check.Kind != "quality" {
		t.Fatalf("unexpected dry-run result: %#v", dryResult)
	}
	if len(dryResult.NextActions) == 0 || dryResult.NextActions[0].Action != "apply_validated_patch" {
		t.Fatalf("dry-run should return apply next action: %#v", dryResult.NextActions)
	}
	if data, err := os.ReadFile(chapterPath); err != nil || strings.Contains(string(data), "repairs the star core") {
		t.Fatalf("dry-run should not modify chapter, err=%v content=%s", err, string(data))
	}

	toolplane.ToolPatchFlags.Apply = true
	var applyOut bytes.Buffer
	applyCmd := bindProject(&cobra.Command{}, storeRoot)
	applyCmd.SetOut(&applyOut)
	if err := toolplane.RunToolPatch(applyCmd, []string{"chapter"}); err != nil {
		t.Fatalf("apply chapter patch: %v", err)
	}
	var applyResult toolplane.ToolPatchResult
	if err := json.Unmarshal(applyOut.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode apply result: %v\n%s", err, applyOut.String())
	}
	if !applyResult.Applied || applyResult.DryRun || applyResult.Files["chapter"] != chapterPath || applyResult.Files["checkpoint"] == "" {
		t.Fatalf("unexpected apply result: %#v", applyResult)
	}
	if applyResult.Check == nil || applyResult.Check.Kind != "quality" || applyResult.Check.Meta["mode"] != "post_apply_quality" ||
		applyResult.Check.Meta["simulation_requires_derived_dsl_refresh"] != true {
		t.Fatalf("apply should return post-apply quality check with simulation refresh marker: %#v", applyResult.Check)
	}
	coverage, ok := applyResult.Check.Meta["coverage"].(map[string]interface{})
	if !ok || coverage["target"] != "chapter" || coverage["kind"] != "quality" {
		t.Fatalf("chapter apply check should preserve coverage meta: %#v", applyResult.Check.Meta)
	}
	if len(applyResult.NextActions) != 2 ||
		applyResult.NextActions[0].Action != "refresh_derived_dsl" ||
		!strings.Contains(applyResult.NextActions[0].Command, `tool refresh chapter-dsl --id 'P1-V1-C1'`) ||
		applyResult.NextActions[1].Action != "post_refresh_check" ||
		!strings.Contains(applyResult.NextActions[1].Command, `tool check all --target chapter --scope chapter --id 'P1-V1-C1'`) {
		t.Fatalf("apply should return refresh/check next actions: %#v", applyResult.NextActions)
	}
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repairs the star core") {
		t.Fatalf("apply did not write patched chapter: %s", string(data))
	}
	if _, err := os.Stat(applyResult.Files["checkpoint"]); err != nil {
		t.Fatalf("checkpoint missing: %v", err)
	}
}

func TestToolPatchChapterParagraphEditOnlyChangesTargetBlock(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Paragraph Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeToolTestOutline(root, validToolPatchTestOutline()); err != nil {
		t.Fatal(err)
	}
	chapterPath := filepath.Join(root, "chapters", "chapter-P1-V1-C1.md")
	if err := os.MkdirAll(filepath.Dir(chapterPath), 0755); err != nil {
		t.Fatal(err)
	}
	first := strings.Repeat("矿道里的风从裂缝里挤出来，Lin听见远处的铁轨轻轻震动。", 18)
	target := strings.Repeat("Lin停在封死的门前，蓝色灯光从门缝下面漏出来。", 18)
	last := strings.Repeat("Lin把手按在冰冷的铁门上，决定先确认门后是否有人呼吸。", 18)
	content := "# Opening\n\n" + first + "\n\n" + target + "\n\n" + last
	if err := os.WriteFile(chapterPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	replacement := strings.Repeat("Lin没有推门，只把耳朵贴上去，先数清门后究竟有几道呼吸。", 18)
	patch := map[string]interface{}{
		"expected_content_sha256": project.ChapterContentSHA256(content),
		"paragraph_edits": []map[string]interface{}{{
			"op":              "replace",
			"index":           3,
			"expected_sha256": project.ChapterParagraphSHA256(target),
			"text":            replacement,
		}},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}

	originalFlags := toolplane.ToolPatchFlags
	defer func() { toolplane.ToolPatchFlags = originalFlags }()
	toolplane.ToolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{
		ID:          "P1-V1-C1",
		PatchJSON:   string(patchBytes),
		TargetWords: 2000,
	}

	dryCmd := bindProject(&cobra.Command{}, toolRoot(t, root))
	var dryOut bytes.Buffer
	dryCmd.SetOut(&dryOut)
	if err := toolplane.RunToolPatch(dryCmd, []string{"chapter"}); err != nil {
		t.Fatalf("dry-run paragraph patch: %v", err)
	}
	dryResult := decodeToolPatchResult(t, dryOut.String())
	if dryResult.Applied || dryResult.Meta["patch_mode"] != "paragraphs" || dryResult.Meta["paragraph_edit_count"] != float64(1) {
		t.Fatalf("unexpected paragraph dry-run result: %#v", dryResult)
	}
	if len(dryResult.Changed) != 1 || !strings.Contains(dryResult.Changed[0].Path, "paragraph[3].replace") {
		t.Fatalf("unexpected paragraph changes: %#v", dryResult.Changed)
	}
	if saved, err := os.ReadFile(chapterPath); err != nil || string(saved) != content {
		t.Fatalf("paragraph dry-run modified chapter, err=%v", err)
	}

	toolplane.ToolPatchFlags.Apply = true
	applyCmd := bindProject(&cobra.Command{}, toolRoot(t, root))
	var applyOut bytes.Buffer
	applyCmd.SetOut(&applyOut)
	if err := toolplane.RunToolPatch(applyCmd, []string{"chapter"}); err != nil {
		t.Fatalf("apply paragraph patch: %v", err)
	}
	applyResult := decodeToolPatchResult(t, applyOut.String())
	if !applyResult.Applied || applyResult.Meta["patch_mode"] != "paragraphs" {
		t.Fatalf("unexpected paragraph apply result: %#v", applyResult)
	}
	saved, err := os.ReadFile(chapterPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(saved)
	if !strings.Contains(got, first) || !strings.Contains(got, last) {
		t.Fatalf("paragraph patch changed untouched blocks:\n%s", got)
	}
	if strings.Contains(got, target) || !strings.Contains(got, replacement) {
		t.Fatalf("paragraph patch did not replace only the target block:\n%s", got)
	}
}

func TestToolPatchChapterParagraphEditRejectsStaleContent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Paragraph Stale Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeToolTestOutline(root, validToolPatchTestOutline()); err != nil {
		t.Fatal(err)
	}
	chapterPath := filepath.Join(root, "chapters", "chapter-P1-V1-C1.md")
	if err := os.MkdirAll(filepath.Dir(chapterPath), 0755); err != nil {
		t.Fatal(err)
	}
	target := "Lin stops before the sealed door."
	content := "# Opening\n\nFirst block.\n\n" + target + "\n"
	if err := os.WriteFile(chapterPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	patch := map[string]interface{}{
		"expected_content_sha256": project.ChapterContentSHA256(content),
		"paragraph_edits": []map[string]interface{}{{
			"op":              "replace",
			"index":           3,
			"expected_sha256": project.ChapterParagraphSHA256(target),
			"text":            "Lin listens at the sealed door.",
		}},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chapterPath, []byte(content+"\nConcurrent edit.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	originalFlags := toolplane.ToolPatchFlags
	defer func() { toolplane.ToolPatchFlags = originalFlags }()
	toolplane.ToolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{ID: "P1-V1-C1", PatchJSON: string(patchBytes), TargetWords: 2000}

	cmd := bindProject(&cobra.Command{}, toolRoot(t, root))
	cmd.SetOut(&bytes.Buffer{})
	err = toolplane.RunToolPatch(cmd, []string{"chapter"})
	if err == nil || !strings.Contains(err.Error(), "changed since the patch was prepared") {
		t.Fatalf("expected stale content error, got %v", err)
	}
}

func TestToolPatchBufferAppendShowClear(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Buffer Test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	storeRoot := toolRoot(t, root)

	originalFlags := toolplane.ToolPatchBufferFlags
	defer func() { toolplane.ToolPatchBufferFlags = originalFlags }()

	toolplane.ToolPatchBufferFlags.ID = "P1-V1-C1-draft"
	toolplane.ToolPatchBufferFlags.Text = "# Opening\n\nLin repairs "
	appendCmd := bindProject(&cobra.Command{}, storeRoot)
	var appendOut bytes.Buffer
	appendCmd.SetOut(&appendOut)
	if err := toolplane.RunToolPatchBuffer(appendCmd, []string{"append"}); err != nil {
		t.Fatalf("append buffer: %v", err)
	}

	toolplane.ToolPatchBufferFlags.Text = "the star core."
	var appendOut2 bytes.Buffer
	appendCmd2 := bindProject(&cobra.Command{}, storeRoot)
	appendCmd2.SetOut(&appendOut2)
	if err := toolplane.RunToolPatchBuffer(appendCmd2, []string{"append"}); err != nil {
		t.Fatalf("append buffer second chunk: %v", err)
	}

	toolplane.ToolPatchBufferFlags.Text = ""
	toolplane.ToolPatchBufferFlags.Stdin = true
	var appendOut3 bytes.Buffer
	appendCmd3 := bindProject(&cobra.Command{}, storeRoot)
	appendCmd3.SetIn(strings.NewReader("\n林野用 stdin 继续追加。"))
	appendCmd3.SetOut(&appendOut3)
	if err := toolplane.RunToolPatchBuffer(appendCmd3, []string{"append"}); err != nil {
		t.Fatalf("append buffer stdin chunk: %v", err)
	}

	toolplane.ToolPatchBufferFlags.Stdin = false
	toolplane.ToolPatchBufferFlags.MaxChars = 18
	var showOut bytes.Buffer
	showCmd := bindProject(&cobra.Command{}, storeRoot)
	showCmd.SetOut(&showOut)
	if err := toolplane.RunToolPatchBuffer(showCmd, []string{"show"}); err != nil {
		t.Fatalf("show buffer: %v", err)
	}
	var shown map[string]interface{}
	if err := json.Unmarshal(showOut.Bytes(), &shown); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, showOut.String())
	}
	if shown["truncated"] != true || !strings.Contains(shown["preview"].(string), "Opening") {
		t.Fatalf("unexpected show output: %#v", shown)
	}

	path, err := toolplane.ToolPatchBufferPath(toolRoot(t, root), "P1-V1-C1-draft")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Opening\n\nLin repairs the star core.\n林野用 stdin 继续追加。" {
		t.Fatalf("unexpected buffer content: %q", string(data))
	}

	var clearOut bytes.Buffer
	clearCmd := bindProject(&cobra.Command{}, storeRoot)
	clearCmd.SetOut(&clearOut)
	if err := toolplane.RunToolPatchBuffer(clearCmd, []string{"clear"}); err != nil {
		t.Fatalf("clear buffer: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("buffer should be removed, stat err=%v", err)
	}
}

func TestBuildChapterPatchNextActionsAfterRefreshDerived(t *testing.T) {
	clean := &toolplane.ToolCheckResult{Blocking: false}
	actions := toolplane.BuildChapterPatchNextActions("P1-V1-C1", true, true, clean)
	if len(actions) != 1 || actions[0].Action != "return_final_json" ||
		strings.Contains(actions[0].Command, "refresh chapter-dsl") {
		t.Fatalf("refreshed clean patch should stop without another refresh: %#v", actions)
	}

	blocking := &toolplane.ToolCheckResult{Blocking: true}
	actions = toolplane.BuildChapterPatchNextActions("P1-V1-C1", true, true, blocking)
	if len(actions) != 1 || actions[0].Action != "repair_remaining_issues" ||
		!strings.Contains(actions[0].Command, `tool check all --target chapter --scope chapter --id 'P1-V1-C1'`) {
		t.Fatalf("refreshed blocking patch should route to repair check: %#v", actions)
	}
}

func TestLoadToolPatchBufferWrapsChapterContent(t *testing.T) {
	root := t.TempDir()
	path, err := toolplane.ToolPatchBufferPath(toolRoot(t, root), "chapter-draft")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Opening\n\n林野完成一次受控修复。"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	payload, err := toolplane.LoadToolPatchBuffer(toolRoot(t, root), "chapter-draft", "chapter")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode wrapped payload: %v\n%s", err, string(payload))
	}
	if decoded["content"] != content {
		t.Fatalf("content = %q, want %q", decoded["content"], content)
	}
	if _, err := toolplane.ToolPatchBufferPath(toolRoot(t, root), "../bad"); err == nil {
		t.Fatalf("expected invalid patch buffer id to be rejected")
	}
}

func TestLoadToolPatchBufferPreservesCompleteChapterPatchJSON(t *testing.T) {
	root := t.TempDir()
	path, err := toolplane.ToolPatchBufferPath(toolRoot(t, root), "chapter-json-patch")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	patchJSON := `{"expected_content_sha256":"abc","paragraph_edits":[{"op":"replace","index":2,"expected_sha256":"def","text":"Lin opens the door."}]}`
	if err := os.WriteFile(path, []byte(patchJSON), 0644); err != nil {
		t.Fatal(err)
	}
	payload, err := toolplane.LoadToolPatchBuffer(toolRoot(t, root), "chapter-json-patch", "chapter")
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != patchJSON {
		t.Fatalf("raw chapter patch buffer changed:\n got %s\nwant %s", payload, patchJSON)
	}
}

func TestToolPatchBufferShowCapsPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Buffer Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := toolplane.ToolPatchBufferPath(toolRoot(t, root), "long-draft")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("字", 3000)), 0644); err != nil {
		t.Fatal(err)
	}

	storeRoot := toolRoot(t, root)

	originalFlags := toolplane.ToolPatchBufferFlags
	defer func() { toolplane.ToolPatchBufferFlags = originalFlags }()
	toolplane.ToolPatchBufferFlags.ID = "long-draft"
	toolplane.ToolPatchBufferFlags.MaxChars = 999999
	var out bytes.Buffer
	cmd := bindProject(&cobra.Command{}, storeRoot)
	cmd.SetOut(&out)
	if err := toolplane.RunToolPatchBuffer(cmd, []string{"show"}); err != nil {
		t.Fatal(err)
	}
	var shown map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, out.String())
	}
	if len([]rune(shown["preview"].(string))) != 2000 || shown["truncated"] != true {
		t.Fatalf("preview should be capped to 2000 chars: %#v", shown)
	}
}

func TestToolPatchValidationUsesQualityAndSimulation(t *testing.T) {
	outline := validToolPatchTestOutline()
	result, err := toolplane.RunToolPatchValidationCheck(nil, outline, "volume", "P1-V1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "all" {
		t.Fatalf("patch validation kind = %q, want all", result.Kind)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected validation to include quality/simulation feedback")
	}
	coverage, ok := result.Meta["coverage"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected coverage meta: %#v", result.Meta)
	}
	if coverage["target"] != "outline" || coverage["scope"] != "volume" || coverage["simulation_phase"] != "outline" ||
		coverage["simulation_backend"] != "in_memory_model_adapter" ||
		coverage["invokes_llm"] != false ||
		coverage["uses_derived_rpg_files"] != false ||
		coverage["refresh_required_before_simulation"] != false {
		t.Fatalf("unexpected outline validation coverage: %#v", coverage)
	}
}

func TestToolOutlinePatchResultExposesValidationCoverage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	if err := toolRoot(t, root).SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	if err := writeToolTestOutline(root, validToolPatchTestOutline()); err != nil {
		t.Fatal(err)
	}

	storeRoot := toolRoot(t, root)

	originalFlags := toolplane.ToolPatchFlags
	defer func() { toolplane.ToolPatchFlags = originalFlags }()
	toolplane.ToolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{
		Target:    "volume",
		ID:        "P1-V1",
		PatchJSON: `{"summary":"Volume arc with a clearer causal spine."}`,
	}

	var out bytes.Buffer
	cmd := bindProject(&cobra.Command{}, storeRoot)
	cmd.SetOut(&out)
	if err := toolplane.RunToolPatch(cmd, []string{"outline"}); err != nil {
		t.Fatalf("outline patch dry-run: %v", err)
	}
	result := decodeToolPatchResult(t, out.String())
	validation, ok := result.Meta["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing validation meta: %#v", result.Meta)
	}
	if validation["kind"] != "all" || validation["target"] != "outline" || validation["scope"] != "volume" ||
		validation["id"] != "P1-V1" {
		t.Fatalf("unexpected validation identity: %#v", validation)
	}
	coverage, ok := validation["coverage"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing validation coverage: %#v", validation)
	}
	if coverage["simulation_backend"] != "in_memory_model_adapter" ||
		coverage["simulation_phase"] != "outline" ||
		coverage["invokes_llm"] != false ||
		coverage["uses_derived_rpg_files"] != false ||
		coverage["refresh_required_before_simulation"] != false {
		t.Fatalf("unexpected validation coverage: %#v", coverage)
	}
}

func TestToolOutlinePatchRejectsNewGlobalBlockingIssueOnDryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	if err := toolRoot(t, root).SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	outline := validToolPatchTestOutline()
	volume := &outline.Parts[0].Volumes[0]
	volume.Chapters[0].Mysteries.Planted = []models.MysteryPlanted{{
		ID:      "myst_old_barrier",
		Clue:    "The old barrier still glows.",
		Horizon: "next_volume",
	}}
	second := volume.Chapters[0]
	second.ID = "P1-V1-C2"
	second.Title = "Second"
	second.Mysteries = models.ChapterMysteries{}
	volume.Chapters = append(volume.Chapters, second)
	if err := writeToolTestOutline(root, outline); err != nil {
		t.Fatal(err)
	}

	storeRoot := toolRoot(t, root)

	originalFlags := toolplane.ToolPatchFlags
	defer func() { toolplane.ToolPatchFlags = originalFlags }()
	toolplane.ToolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{
		Target:    "volume",
		ID:        "P1-V1",
		PatchJSON: `{"changed_chapters":[{"id":"P1-V1-C2","mysteries":{"planted":[{"id":"myst_old_barrier","clue":"Repeated clue"}]}}]}`,
	}

	var out bytes.Buffer
	cmd := bindProject(&cobra.Command{}, storeRoot)
	cmd.SetOut(&out)
	err := toolplane.RunToolPatch(cmd, []string{"outline"})
	if err == nil || !strings.Contains(err.Error(), "global outline check introduced") || !strings.Contains(err.Error(), "myst_old_barrier") {
		t.Fatalf("expected global blocking rejection, got %v output=%s", err, out.String())
	}
}

func TestRunToolOutlineCheckUsesGlobalSimulationContextForScopedMysteries(t *testing.T) {
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Plant",
				Summary: "Plant a cross-volume mystery.",
				Mysteries: models.ChapterMysteries{Planted: []models.MysteryPlanted{{
					ID:   "myst_cross_volume",
					Clue: "A signal appears.",
				}}},
			}},
		}, {
			ID: "P1-V2",
			Chapters: []models.Chapter{{
				ID:      "P1-V2-C1",
				Title:   "Resolve",
				Summary: "Resolve the cross-volume mystery.",
				Mysteries: models.ChapterMysteries{Resolved: []models.MysteryResolved{{
					ID:         "myst_cross_volume",
					Resolution: "The signal came from the old relay.",
				}}},
			}},
		}},
	}}}

	check, err := toolplane.RunToolOutlineCheck("all", setup, outline, "volume", "P1-V2")
	if err != nil {
		t.Fatalf("run scoped check: %v", err)
	}
	for _, issue := range check.Issues {
		if toolplane.NormalizeKey(issue.Category) == "mysteries" && strings.Contains(issue.Issue, "从未被planted") {
			t.Fatalf("scoped check should use global mystery context, got issue: %#v", issue)
		}
	}
}

func TestOutlinePatchPostCheckCommandScopesTarget(t *testing.T) {
	if got := toolplane.OutlinePatchPostCheckCommand("volume", "P1-V1"); got != `novelgen tool check all --target outline --scope volume --id 'P1-V1' --min-priority medium --max-issues 12` {
		t.Fatalf("volume post check = %q", got)
	}
	if got := toolplane.OutlinePatchPostCheckCommand("chapter", "P1-V1-C1"); got != `novelgen tool check all --target outline --scope chapter --id 'P1-V1-C1' --min-priority low --max-issues 8` {
		t.Fatalf("chapter post check = %q", got)
	}
	if got := toolplane.OutlinePatchPostCheckCommand("all", ""); got != "" {
		t.Fatalf("unexpected post check = %q", got)
	}
}
