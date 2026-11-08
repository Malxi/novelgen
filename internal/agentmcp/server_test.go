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
