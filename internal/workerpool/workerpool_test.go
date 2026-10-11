package workerpool

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapReturnsResultsInItemOrder(t *testing.T) {
	items := []int{5, 1, 4, 2, 3}
	got, errs := Map(context.Background(), 4, items, func(_ context.Context, _ int, value int) (int, error) {
		// Finish out of order on purpose; results must still be input-ordered.
		time.Sleep(time.Duration(value) * time.Millisecond)
		return value * 10, nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []int{50, 10, 40, 20, 30}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("result[%d] = %d, want %d (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestMapCollectsErrorsInItemOrderAndKeepsGoing(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	var processed int32
	_, errs := Map(context.Background(), 5, items, func(_ context.Context, _ int, value int) (int, error) {
		atomic.AddInt32(&processed, 1)
		if value%2 == 0 {
			return 0, fmt.Errorf("boom %d", value)
		}
		return value, nil
	})
	if got := atomic.LoadInt32(&processed); got != int32(len(items)) {
		t.Fatalf("processed = %d, want %d (Map must not stop early)", got, len(items))
	}
	want := "boom 0,boom 2,boom 4"
	var got []string
	for _, err := range errs {
		got = append(got, err.Error())
	}
	if strings.Join(got, ",") != want {
		t.Fatalf("errors = %v, want %q", got, want)
	}
}

func TestMapCapsConcurrency(t *testing.T) {
	const workers = 3
	const items = 30
	var active, maxActive int32
	inputs := make([]int, items)
	for i := range inputs {
		inputs[i] = i
	}
	_, errs := Map(context.Background(), workers, inputs, func(_ context.Context, _ int, _ int) (int, error) {
		now := atomic.AddInt32(&active, 1)
		for {
			prev := atomic.LoadInt32(&maxActive)
			if now <= prev || atomic.CompareAndSwapInt32(&maxActive, prev, now) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return 0, nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if maxActive > workers {
		t.Fatalf("max concurrency = %d, want <= %d", maxActive, workers)
	}
}

func TestMapEmptyInput(t *testing.T) {
	results, errs := Map(context.Background(), 4, []int{}, func(_ context.Context, _ int, _ int) (int, error) {
		t.Fatal("fn must not be called for empty input")
		return 0, nil
	})
	if results != nil || errs != nil {
		t.Fatalf("empty input should return (nil, nil), got (%v, %v)", results, errs)
	}
}

func TestMapNonPositiveWorkersRunsSequentially(t *testing.T) {
	items := []int{1, 2, 3}
	var active, maxActive int32
	got, errs := Map(context.Background(), 0, items, func(_ context.Context, _ int, value int) (int, error) {
		now := atomic.AddInt32(&active, 1)
		if now > maxActive {
			atomic.StoreInt32(&maxActive, now)
		}
		time.Sleep(time.Millisecond)
		atomic.AddInt32(&active, -1)
		return value, nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if maxActive != 1 {
		t.Fatalf("max concurrency = %d, want 1 for non-positive workers", maxActive)
	}
	if len(got) != len(items) {
		t.Fatalf("got %d results, want %d", len(got), len(items))
	}
}

func TestMapWorkersCappedAtItemCount(t *testing.T) {
	// More workers than items must not spawn idle goroutines or panic; every
	// item still runs exactly once.
	var processed int32
	got, errs := Map(context.Background(), 100, []int{7, 8}, func(_ context.Context, _ int, value int) (int, error) {
		atomic.AddInt32(&processed, 1)
		return value, nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if atomic.LoadInt32(&processed) != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}
	if len(got) != 2 || got[0] != 7 || got[1] != 8 {
		t.Fatalf("results = %v, want [7 8]", got)
	}
}
