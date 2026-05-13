package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDerivedFileAfterSaveWritesOutlineMarkdown(t *testing.T) {
	root := t.TempDir()
	outlineDir := filepath.Join(root, "story", "compose")
	if err := os.MkdirAll(outlineDir, 0755); err != nil {
		t.Fatalf("mkdir outline dir: %v", err)
	}

	outlinePath := filepath.Join(outlineDir, "outline.json")
	outlineJSON := `{
  "parts": [
    {
      "id": "P1",
      "title": "第一部",
      "summary": "测试部",
      "volumes": [
        {
          "id": "P1-V1",
          "title": "第一卷",
          "summary": "测试卷",
          "chapters": [
            {
              "id": "P1-V1-C1",
              "title": "第一章",
              "summary": "测试章",
              "characters": ["白烬"],
              "location": "青岚宗",
              "events": [],
              "conflict": "测试冲突",
              "pacing": "fast"
            }
          ]
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(outlinePath, []byte(outlineJSON), 0644); err != nil {
		t.Fatalf("write outline json: %v", err)
	}

	if err := syncDerivedFileAfterSave("story/compose/outline.json", outlinePath); err != nil {
		t.Fatalf("sync derived file: %v", err)
	}

	markdown, err := os.ReadFile(filepath.Join(outlineDir, "outline.md"))
	if err != nil {
		t.Fatalf("read outline markdown: %v", err)
	}

	text := string(markdown)
	for _, want := range []string{"# Story Outline", "## 第一部", "### 第一卷", "#### 第一章"} {
		if !strings.Contains(text, want) {
			t.Fatalf("outline markdown missing %q:\n%s", want, text)
		}
	}
}
