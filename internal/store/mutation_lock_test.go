package store_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"novelgen/internal/store"
)

func TestMutationLockFollowsProjectDirectory(t *testing.T) {
	dir := t.TempDir()
	first, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	second, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open second: %v", err)
	}

	var active int32
	var maxActive int32
	var wg sync.WaitGroup
	for _, root := range []*store.Root{first, second} {
		root := root
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := root.WithMutationLock(func() error {
				current := atomic.AddInt32(&active, 1)
				for {
					seen := atomic.LoadInt32(&maxActive)
					if current <= seen || atomic.CompareAndSwapInt32(&maxActive, seen, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			})
			if err != nil {
				t.Errorf("mutation lock: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Fatalf("two Roots for the same project entered the mutation lock concurrently: max active=%d", maxActive)
	}
}
