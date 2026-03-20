package cmd

import (
	"sort"
	"strings"

	"nolvegen/internal/models"
)

// ==================== Format Utils ====================

func formatReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, r := range reasons {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		sb.WriteString("- " + r + "\n")
	}
	return sb.String()
}

// ==================== Character Utils ====================

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

// getStartOfStoryCharacters returns a set of characters that appear at the start of the story.
// We define this as the union of characters present in the first N chapters in traversal order.
func getStartOfStoryCharacters(outline *models.Outline, firstNChapters int) map[string]bool {
	set := make(map[string]bool)
	if outline == nil || len(outline.Parts) == 0 {
		return set
	}

	if firstNChapters <= 0 {
		firstNChapters = 1
	}

	count := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, name := range ch.Characters {
					name = strings.TrimSpace(name)
					if name != "" {
						set[name] = true
					}
				}
				count++
				if count >= firstNChapters {
					return set
				}
			}
		}
	}
	return set
}

// hardenCharacterRelationships strips relationships that reference characters not present at story start.
// This prevents LLMs from pre-filling future relationships when crafting full-cast character sheets.
func hardenCharacterRelationships(chars map[string]*models.Character, startChars map[string]bool) {
	if len(chars) == 0 || len(startChars) == 0 {
		return
	}

	for _, ch := range chars {
		if ch == nil || len(ch.Relationships) == 0 {
			continue
		}
		filtered := make(map[string]string)
		for other, rel := range ch.Relationships {
			otherName := strings.TrimSpace(other)
			if otherName == "" {
				continue
			}
			if startChars[otherName] {
				filtered[otherName] = rel
			}
		}
		// If this character itself is not a start-of-story character, we drop all its relationships
		// to avoid implying pre-existing links for late entrants.
		if !startChars[strings.TrimSpace(ch.Name)] {
			filtered = map[string]string{}
		}
		if len(filtered) == 0 {
			ch.Relationships = nil
		} else {
			ch.Relationships = filtered
		}
	}
}
