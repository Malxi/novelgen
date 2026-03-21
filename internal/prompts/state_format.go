package prompts

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// FormatStateMatrix formats the state matrix into a human-readable string for prompts.
// This is a shared utility used by both DraftAgent and WriteAgent.
func FormatStateMatrix(state *models.StateMatrix, chapter *models.Chapter) string {
	var sb strings.Builder

	sb.WriteString("=== CURRENT STORY STATE ===\n\n")

	// Characters present in this chapter
	sb.WriteString("Characters in this chapter:\n")
	for _, charName := range chapter.Characters {
		if char, exists := state.Characters[charName]; exists {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", char.Name, char.RoleInStory))
			if char.Age != "" {
				sb.WriteString(fmt.Sprintf("  Age: %s\n", char.Age))
			}
			if len(char.Personality) > 0 {
				sb.WriteString(fmt.Sprintf("  Personality: %s\n", strings.Join(char.Personality, ", ")))
			}
			if char.Motivation != "" {
				sb.WriteString(fmt.Sprintf("  Motivation: %s\n", char.Motivation))
			}
			// Goals are stored in state matrix dynamically
			if goals, exists := state.Goals[charName]; exists && len(goals) > 0 {
				sb.WriteString(fmt.Sprintf("  Current Goals: %s\n", strings.Join(goals, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	// Location
	if chapter.Location != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", chapter.Location))

		// Try to find location info - chapter.Location might be a compound string like "矿井口与下井滑道"
		// So we try to match any known location that appears in the string
		foundLocation := false
		for locName, loc := range state.Locations {
			if strings.Contains(chapter.Location, locName) {
				sb.WriteString(fmt.Sprintf("  [%s] Description: %s\n", locName, loc.Description))
				sb.WriteString(fmt.Sprintf("  [%s] Atmosphere: %s\n", locName, loc.Atmosphere))
				foundLocation = true
			}
		}

		// Fallback: try exact match
		if !foundLocation {
			if loc, exists := state.Locations[chapter.Location]; exists {
				sb.WriteString(fmt.Sprintf("  Description: %s\n", loc.Description))
				sb.WriteString(fmt.Sprintf("  Atmosphere: %s\n", loc.Atmosphere))
			}
		}

		sb.WriteString("\n")
	}

	// Active storylines (filter out completed ones)
	activeStorylines := make(map[string]*models.StorylineState)
	for id, sl := range state.Storylines {
		// Skip completed storylines
		if sl.Status == "completed" || strings.Contains(sl.Status, "completed") {
			continue
		}
		activeStorylines[id] = sl
	}

	if len(activeStorylines) > 0 {
		sb.WriteString("Active Storylines:\n")
		for id, sl := range activeStorylines {
			sb.WriteString(fmt.Sprintf("- %s", sl.Name))
			if sl.Description != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", sl.Description))
			}
			sb.WriteString(fmt.Sprintf(": %s", sl.Status))
			sb.WriteString("\n")

			// Show full progress history
			if len(sl.ProgressHistory) > 0 {
				sb.WriteString("  Progress History:\n")
				for i, progress := range sl.ProgressHistory {
					sb.WriteString(fmt.Sprintf("    Step %d [%s]: %s - %s\n",
						i+1, progress.ChapterID, progress.Status, progress.Details))
				}
			}

			// Also show the ID for reference
			if id != sl.Name {
				sb.WriteString(fmt.Sprintf("  [ID: %s]\n", id))
			}
		}
		sb.WriteString("\n")
	}

	// Character relationships (only show relationships involving characters in this chapter)
	if len(state.Relationships) > 0 {
		relevantRelations := make(map[string]string)
		for pair, relation := range state.Relationships {
			// Check if either character in the pair is in this chapter
			chars := strings.Split(pair, "_")
			if len(chars) >= 2 {
				for _, chapterChar := range chapter.Characters {
					if chars[0] == chapterChar || chars[1] == chapterChar {
						relevantRelations[pair] = relation
						break
					}
				}
			}
		}
		if len(relevantRelations) > 0 {
			sb.WriteString("Key Relationships:\n")
			for pair, relation := range relevantRelations {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", pair, relation))
			}
			sb.WriteString("\n")
		}
	}

	// Character premise states (only show for characters in this chapter)
	if len(state.Premises) > 0 {
		relevantPremises := make(map[string]string)
		for key, progress := range state.Premises {
			// Key format: "characterName_premiseName"
			parts := strings.Split(key, "_")
			if len(parts) >= 1 {
				charName := parts[0]
				for _, chapterChar := range chapter.Characters {
					if charName == chapterChar {
						relevantPremises[key] = progress
						break
					}
				}
			}
		}
		if len(relevantPremises) > 0 {
			sb.WriteString("Character Progression:\n")
			for key, progress := range relevantPremises {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", key, progress))
			}
			sb.WriteString("\n")
		}
	}

	// Items relevant to this chapter
	formatItems(&sb, state, chapter)

	// Chapter events to cover
	if len(chapter.Events) > 0 {
		sb.WriteString("Events to cover in this chapter:\n")
		for _, event := range chapter.Events {
			sb.WriteString(fmt.Sprintf("- [%s] ", event.Type))
			if len(event.Characters) > 0 {
				sb.WriteString(fmt.Sprintf("Characters: %s, ", strings.Join(event.Characters, ", ")))
			}
			if event.Subject != "" {
				sb.WriteString(fmt.Sprintf("Subject: %s, ", event.Subject))
			}
			sb.WriteString(fmt.Sprintf("Change: %s\n", event.Change))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== END STATE ===\n")

	return sb.String()
}

// formatItems formats the items section of the state matrix
func formatItems(sb *strings.Builder, state *models.StateMatrix, chapter *models.Chapter) {
	// Items relevant to this chapter
	relevantItems := make(map[string]string) // itemName -> description

	// 1. Items owned by characters in this chapter
	for itemName, item := range state.Items {
		if item.Owner != "" {
			for _, charName := range chapter.Characters {
				if item.Owner == charName {
					relevantItems[itemName] = fmt.Sprintf("owned by %s", charName)
					break
				}
			}
		}
	}

	// 2. Items mentioned in chapter events (get/lost/subject)
	for _, event := range chapter.Events {
		// Item type events
		if event.Type == "item" && event.Subject != "" {
			itemName := event.Subject
			charName := ""
			if len(event.Characters) > 0 {
				charName = event.Characters[0]
			}
			switch event.Change {
			case "get":
				relevantItems[itemName] = fmt.Sprintf("will be acquired by %s", charName)
			case "lost":
				relevantItems[itemName] = fmt.Sprintf("will be lost by %s", charName)
			default:
				if _, exists := relevantItems[itemName]; !exists {
					relevantItems[itemName] = fmt.Sprintf("involved in event with %s", charName)
				}
			}
		}
		// Other event types where subject is an item
		if event.Subject != "" && event.Type != "item" {
			if _, exists := state.Items[event.Subject]; exists {
				if _, alreadyListed := relevantItems[event.Subject]; !alreadyListed {
					relevantItems[event.Subject] = "mentioned in event"
				}
			}
		}
	}

	if len(relevantItems) > 0 {
		sb.WriteString("Relevant Items:\n")
		for itemName, desc := range relevantItems {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", itemName, desc))
		}
		sb.WriteString("\n")
	}
}

// FormatChapterContext formats the surrounding chapter context for prompts.
// previous: previous chapters with content snippets
// next: next chapters (summaries only, for foreshadowing)
// maxSnippetLen: maximum length of content snippet for previous chapters
func FormatChapterContext(previous []*ContextChapter, next []*ContextChapter, maxSnippetLen int) string {
	var sb strings.Builder

	sb.WriteString("=== CHAPTER CONTEXT ===\n\n")

	// Previous chapters
	if len(previous) > 0 {
		sb.WriteString("PREVIOUS CHAPTERS:\n")
		for _, prev := range previous {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", prev.Chapter.ID, prev.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", prev.Chapter.Summary))
			// Include a snippet of the content (or full content if maxSnippetLen is 0)
			content := prev.Content
			if maxSnippetLen > 0 && len(content) > maxSnippetLen {
				content = content[:maxSnippetLen] + "..."
			}
			sb.WriteString(fmt.Sprintf("Content:\n%s\n", content))
		}
		sb.WriteString("\n")
	}

	// Next chapters (for foreshadowing)
	if len(next) > 0 {
		sb.WriteString("UPCOMING CHAPTERS (for foreshadowing):\n")
		for _, n := range next {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", n.Chapter.ID, n.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", n.Chapter.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== END CONTEXT ===\n")

	return sb.String()
}

// ContextChapter represents a chapter with its content for context formatting
type ContextChapter struct {
	Chapter *models.Chapter
	Content string
}
