package store_test

import (
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

func TestProjectKeysRejectPathTraversal(t *testing.T) {
	root := store.MemRoot()
	if err := root.SaveChapterContent("../escape", "text"); err == nil {
		t.Fatal("SaveChapterContent accepted a path traversal key")
	}
	if _, _, err := root.LoadChapterContent("../escape"); err == nil {
		t.Fatal("LoadChapterContent accepted a path traversal key")
	}
	if err := root.SaveDraftContent("a/b", "text"); err == nil {
		t.Fatal("SaveDraftContent accepted a separator")
	}
	if err := root.SaveRecap(&models.ChapterRecap{ChapterID: ".."}); err == nil {
		t.Fatal("SaveRecap accepted a traversal key")
	}
}
