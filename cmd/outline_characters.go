package cmd

import (
	"sort"
	"strings"

	"nolvegen/internal/models"
)

func collectKnownCharactersFromOutline(outline *models.Outline) []string {
	if outline == nil {
		return nil
	}
	set := map[string]bool{}
	for _, ch := range getAllChapters(outline) {
		if ch == nil {
			continue
		}
		for _, n := range ch.Characters {
			n = strings.TrimSpace(n)
			if n != "" {
				set[n] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
