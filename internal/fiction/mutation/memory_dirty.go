package mutation

import (
	"fmt"

	"novelgen/internal/fiction/domain/memory"
	"novelgen/internal/platform/store"
)

// markMemoryDirtyAfterWrite is a best-effort derived-cache invalidation. A
// failure here must never turn a successful canonical write into a command
// failure; source fingerprints still detect staleness on the next query.
func markMemoryDirtyAfterWrite(root *store.Root, reason string, warnings []string) []string {
	marked, err := memory.MarkNarrativeMemoryDirty(root, reason)
	if err != nil {
		return append(warnings, fmt.Sprintf("saved canonical state but could not mark narrative memory stale: %v", err))
	}
	if marked {
		return append(warnings, "narrative memory marked stale by canonical write; the next query rebuilds it in memory and `tool refresh narrative-memory` persists it")
	}
	return warnings
}
