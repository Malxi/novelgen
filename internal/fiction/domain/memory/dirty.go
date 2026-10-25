package memory

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"novelgen/internal/platform/store"
)

type DirtyMarker struct {
	MarkedAt time.Time `json:"marked_at"`
	Changes  []string  `json:"changes"`
}

// MarkNarrativeMemoryDirty records a cheap invalidation hint after canonical
// story state changes. It is a no-op when the project has never persisted a
// narrative-memory index, so projects that do not use the cache pay only one
// file-existence check.
func MarkNarrativeMemoryDirty(root *store.Root, reason string) (bool, error) {
	if root == nil || root.Dir() == "" {
		return false, fmt.Errorf("project root is required")
	}
	if !root.Exists(root.NarrativeMemoryIndexFile()) {
		return false, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "canonical project state changed"
	}
	marker := DirtyMarker{MarkedAt: time.Now().UTC()}
	if existing, err := LoadNarrativeMemoryDirty(root); err != nil {
		return false, err
	} else if existing != nil {
		marker.Changes = append(marker.Changes, existing.Changes...)
	}
	marker.Changes = uniqueStrings(append(marker.Changes, reason)...)
	if err := root.WriteJSON(root.NarrativeMemoryDirtyFile(), marker); err != nil {
		return false, err
	}
	return true, nil
}

// LoadNarrativeMemoryDirty returns nil when no dirty marker exists.
func LoadNarrativeMemoryDirty(root *store.Root) (*DirtyMarker, error) {
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	var marker DirtyMarker
	if err := root.ReadJSON(root.NarrativeMemoryDirtyFile(), &marker); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &marker, nil
}

func clearNarrativeMemoryDirty(root *store.Root) error {
	if root == nil || root.Dir() == "" {
		return fmt.Errorf("project root is required")
	}
	if err := root.Remove(root.NarrativeMemoryDirtyFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
