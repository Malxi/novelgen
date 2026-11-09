package agentmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/platform/store"
)

func TestMCPServeInitializeListAndStatus(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "novel.json"), []byte(`{"name":"MCP Test","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "story", "setup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "story", "setup", "story_setup.json"), []byte(`{"project_name":"MCP Test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "story", "compose"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "story", "compose", "outline.json"), []byte(`{"parts":[{"id":"P1","title":"Part","volumes":[{"id":"P1-V1","title":"Volume","chapters":[{"id":"P1-V1-C1","title":"Chapter 1"}]}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := store.OpenProject(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"novelgen_status","arguments":{}}}`,
	}
	var out bytes.Buffer
	server := New(root, "novelgen")
	if err := server.Serve(context.Background(), strings.NewReader(strings.Join(requests, "\n")), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses = %d: %s", len(lines), out.String())
	}
	var initResp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp["result"] == nil {
		t.Fatalf("initialize failed: %s", lines[0])
	}
	var listResp struct {
		Result struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Result.Tools) < 6 {
		t.Fatalf("tools = %d", len(listResp.Result.Tools))
	}
	var callResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &callResp); err != nil {
		t.Fatal(err)
	}
	if len(callResp.Result.Content) != 1 || !strings.Contains(callResp.Result.Content[0].Text, `"next"`) {
		t.Fatalf("status content = %s", lines[2])
	}
}

func TestMCPServeStatusEmptyVolumeIsOutlineStage(t *testing.T) {
	rootDir := t.TempDir()
	writeMCPTestFile(t, rootDir, "novel.json", `{"name":"MCP Test","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":2}}`)
	writeMCPTestFile(t, rootDir, "story/setup/story_setup.json", `{"project_name":"MCP Test"}`)
	writeMCPTestFile(t, rootDir, "story/compose/outline.json", `{"parts":[{"id":"P1","title":"Part","volumes":[]}]}`)
	root, err := store.OpenProject(rootDir)
	if err != nil {
		t.Fatal(err)
	}

	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"novelgen_status","arguments":{}}}`
	var out bytes.Buffer
	if err := New(root, "novelgen").Serve(context.Background(), strings.NewReader(requests), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("decode response %s: %v", out.String(), err)
	}
	if len(response.Result.Content) != 1 {
		t.Fatalf("status content = %s", out.String())
	}
	var status struct {
		Chapters []interface{}          `json:"chapters"`
		Next     map[string]interface{} `json:"next"`
	}
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Next["stage"] != "outline" {
		t.Fatalf("next stage = %v, want outline: %s", status.Next["stage"], response.Result.Content[0].Text)
	}
	if status.Chapters == nil {
		t.Fatalf("chapters should be an empty array, not null: %s", response.Result.Content[0].Text)
	}
	if len(status.Chapters) != 0 {
		t.Fatalf("chapters = %d, want 0", len(status.Chapters))
	}
}

func TestMCPServeUnknownMethodUsesMethodNotFound(t *testing.T) {
	rootDir := t.TempDir()
	writeMCPTestFile(t, rootDir, "novel.json", `{"name":"MCP Test"}`)
	root, err := store.OpenProject(rootDir)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := New(root, "novelgen").Serve(context.Background(),
		strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"bogus"}`), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != -32601 {
		t.Fatalf("error code = %d, want -32601", response.Error.Code)
	}
}

func TestMCPGlobalServerExposesInitAndBoundsProjects(t *testing.T) {
	workspace := t.TempDir()
	server := NewGlobal(workspace, "novelgen")
	if server.Workspace == "" || !server.AllowCreate {
		t.Fatalf("global server not configured: %+v", server)
	}
	if _, err := server.status(""); err == nil {
		t.Fatal("global status without projectDir should fail outside a project workspace")
	}
	if _, err := server.projectPath(filepath.Join(t.TempDir(), "escape"), false); err == nil {
		t.Fatal("projectDir outside the workspace should be rejected for creation")
	}
	if _, err := server.projectPath(filepath.Join(t.TempDir(), "escape"), true); err != nil {
		t.Fatalf("absolute projectDir should be usable for an existing project: %v", err)
	}
	if got := safeProjectDirName(`我的书:1`); got != "我的书-1" {
		t.Fatalf("safeProjectDirName = %q", got)
	}

	var out bytes.Buffer
	requests := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	if err := server.Serve(context.Background(), strings.NewReader(requests), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range response.Result.Tools {
		names[tool.Name] = true
	}
	if !names["book_init"] {
		t.Fatalf("global tool list has no book_init: %s", out.String())
	}
	if !names["book_status"] {
		t.Fatalf("global tool list has no book_status: %s", out.String())
	}
}

func TestMCPQueryArgsMirrorCLIQueryFlags(t *testing.T) {
	args := map[string]interface{}{
		"section":    "ideas",
		"type":       "namecard",
		"genre":      "科幻",
		"count":      float64(12),
		"seed":       float64(42),
		"mode":       "wild",
		"entityType": "character",
		"chapterId":  "P1-V1-C1",
		"maxChars":   float64(4000),
		"content":    true,
	}
	got := strings.Join(queryArgs(args), " ")
	for _, want := range []string{
		"tool query ideas",
		"--type namecard",
		"--genre 科幻",
		"--count 12",
		"--seed 42",
		"--mode wild",
		"--entity-type character",
		"--chapter-id P1-V1-C1",
		"--max-chars 4000",
		"--content",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("queryArgs missing %q: %s", want, got)
		}
	}
}

func TestMCPInitSchemaExposesVolumes(t *testing.T) {
	tools := bookToolDefinitions(true)
	for _, tool := range tools {
		if tool["name"] != "book_init" {
			continue
		}
		schema := tool["inputSchema"].(map[string]interface{})
		properties := schema["properties"].(map[string]interface{})
		for _, key := range []string{"parts", "volumes", "chapters"} {
			if _, ok := properties[key]; !ok {
				t.Fatalf("book_init schema missing %s: %#v", key, properties)
			}
		}
		return
	}
	t.Fatal("book_init not found")
}

func TestMCPBookWriteSchemaHint(t *testing.T) {
	result := &toolResult{
		Content: []toolContent{{Type: "text", Text: "invalid setup JSON: cannot unmarshal string into Go struct field StorySetup.premises"}},
		IsError: true,
	}
	hinted := withSchemaHint(result, "setup")
	if !strings.Contains(hinted.Content[0].Text, "setup-schema") {
		t.Fatalf("schema hint missing: %s", hinted.Content[0].Text)
	}
	ok := &toolResult{Content: []toolContent{{Type: "text", Text: "ok"}}}
	if withSchemaHint(ok, "setup") != ok {
		t.Fatal("successful result should not be changed")
	}
}

func TestMCPPatchAndReplaceArgsCapIssues(t *testing.T) {
	patch := strings.Join(patchArgs(map[string]interface{}{"section": "chapter", "id": "P1-V1-C1"}), " ")
	if !strings.Contains(patch, "--max-issues 12") {
		t.Fatalf("patchArgs missing default issue cap: %s", patch)
	}
	replace := strings.Join(replaceArgs(map[string]interface{}{"section": "outline"}), " ")
	if !strings.Contains(replace, "--max-issues 12") {
		t.Fatalf("replaceArgs missing default issue cap: %s", replace)
	}
	replace = strings.Join(replaceArgs(map[string]interface{}{"section": "outline", "maxIssues": float64(3)}), " ")
	if !strings.Contains(replace, "--max-issues 3") {
		t.Fatalf("replaceArgs did not honor maxIssues: %s", replace)
	}
}

func writeMCPTestFile(t *testing.T, rootDir, rel, body string) {
	t.Helper()
	target := filepath.Join(rootDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
