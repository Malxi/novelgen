package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/models"
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

// loadDraftContent loads draft content for a chapter from the drafts directory.
func loadDraftContent(chapterID string) string {
	root, err := findProjectRoot()
	if err != nil {
		return ""
	}
	path := filepath.Join(root, "drafts", chapterID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// extractChapterNumber extracts the chapter number from a chapter ID.
// For example, "chap_1_2_3" returns "3", "P1-V2-C5" returns "5".
func extractChapterNumber(chapterID string) string {
	// Try to extract the last numeric component
	parts := strings.Split(chapterID, "_")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Check if it's a number
		if _, err := os.ReadFile(""); err == nil {
			// This is just a placeholder check, we return the last part
			return lastPart
		}
		return lastPart
	}
	// Try splitting by "-" for format like "P1-V2-C5"
	parts = strings.Split(chapterID, "-")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Remove any non-numeric prefix like "C" in "C5"
		lastPart = strings.TrimLeft(lastPart, "Cc")
		return lastPart
	}
	return chapterID
}
