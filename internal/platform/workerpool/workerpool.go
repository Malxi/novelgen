// Package workerpool provides a small bounded-concurrency helper for the agent
// stages that fan work out in parallel.
//
// The stages previously hand-rolled the same "WaitGroup + mutex + channel"
// pattern a dozen times. Centralizing it keeps the concurrency policy in one
// place and removes per-site locking from the callers.
package workerpool

import (
	"context"
	"sync"
)

// Map applies fn to every item using at most workers concurrent goroutines and
// returns the results in the same order as items. Errors are collected in item
// order (nil entries are skipped).
//
// Map never stops early: every item is attempted so callers can report a
// complete picture. If a fast abort is needed, fn should observe ctx.
//
// A non-positive workers value is treated as 1, and workers is capped at
// len(items) so a small job never spawns idle goroutines.
func Map[T any, R any](
	ctx context.Context,
	workers int,
	items []T,
	fn func(ctx context.Context, index int, item T) (R, error),
) ([]R, []error) {
	if len(items) == 0 {
		return nil, nil
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(items) {
		workers = len(items)
	}

	results := make([]R, len(items))
	errsByIndex := make([]error, len(items))

	queue := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range queue {
				result, err := fn(ctx, index, items[index])
				if err != nil {
					errsByIndex[index] = err
					continue
				}
				// Each goroutine writes a distinct index, so no lock is needed.
				results[index] = result
			}
		}()
	}
	for index := range items {
		queue <- index
	}
	close(queue)
	wg.Wait()

	errs := make([]error, 0, len(items))
	for _, err := range errsByIndex {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return results, errs
}
