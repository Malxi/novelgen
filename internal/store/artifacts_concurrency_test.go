package store_test

import (
	"fmt"
	"sync"
	"testing"

	"novelgen/internal/store"
)

func TestWriteJSONMergingSerializesInProcess(t *testing.T) {
	root := store.MemRoot()
	path := root.ComposeFile("merge.json")

	const writers = 24
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := root.WriteJSONMerging(path, map[string]int{fmt.Sprintf("k%d", i): i}); err != nil {
				t.Errorf("WriteJSONMerging(%d): %v", i, err)
			}
		}()
	}
	wg.Wait()

	var got map[string]int
	if err := root.ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if len(got) != writers {
		t.Fatalf("merged keys = %d, want %d: %#v", len(got), writers, got)
	}
}
