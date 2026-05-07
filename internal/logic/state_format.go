package logic

import (
	"fmt"
	"sort"
	"strings"

	"novelgen/internal/models"
)

// FormatStateMatrix formats the state matrix into a human-readable string for prompts.
// This is a shared utility used by legacy DraftAgent paths.
func FormatStateMatrix(state *models.StateMatrix, chapter *models.Chapter) string {
	if state == nil {
		return FormatChapterContinuity(nil, chapter)
	}
	return FormatChapterContinuity(&models.ChapterContinuity{
		RPG:        state.RPG,
		Characters: state.Characters,
		Locations:  state.Locations,
		Items:      state.Items,
		Premises:   state.Premises,
		Gates:      state.Gates,
		Status:     state.Status,
		Memories:   state.Memories,
	}, chapter)
}

// FormatChapterContinuity formats the writer-facing continuity snapshot before
// a chapter begins.
func FormatChapterContinuity(state *models.ChapterContinuity, chapter *models.Chapter) string {
	var sb strings.Builder
	if state == nil {
		state = &models.ChapterContinuity{}
	}

	sb.WriteString("CURRENT CONTINUITY:\n")
	if state.RPG != nil {
		sb.WriteString("\n")
		sb.WriteString(FormatRPGState(state.RPG, chapter))
		sb.WriteString("\n")
	}

	// Build set of relevant characters for this chapter
	// Include both explicitly listed characters and those mentioned in events
	relevantChars := make(map[string]bool)
	for _, charName := range chapter.Characters {
		relevantChars[charName] = true
	}
	for _, event := range chapter.Events {
		for _, charName := range event.Characters {
			relevantChars[charName] = true
		}
	}

	// Characters present in this chapter
	sb.WriteString("\nCharacters:\n")
	for charName := range relevantChars {
		if char, exists := state.Characters[charName]; exists {
			sb.WriteString(fmt.Sprintf("  %s:\n", char.Name))
			if char.Age != "" {
				sb.WriteString(fmt.Sprintf("    - Age: %s\n", char.Age))
			}
			if char.Gender != "" {
				sb.WriteString(fmt.Sprintf("    - Gender: %s\n", char.Gender))
			}
			if char.Race != "" {
				sb.WriteString(fmt.Sprintf("    - Race: %s\n", char.Race))
			}
			if len(char.Aliases) > 0 {
				sb.WriteString(fmt.Sprintf("    - Aliases: %s\n", strings.Join(char.Aliases, ", ")))
			}
			if char.Appearance != "" {
				sb.WriteString(fmt.Sprintf("    - Appearance: %s\n", char.Appearance))
			}
			if len(char.Personality) > 0 {
				sb.WriteString(fmt.Sprintf("    - Personality: %s\n", strings.Join(char.Personality, ", ")))
			}
			if char.Background != "" {
				sb.WriteString(fmt.Sprintf("    - Background: %s\n", char.Background))
			}
			if len(char.Skills) > 0 {
				sb.WriteString(fmt.Sprintf("    - Skills: %s\n", strings.Join(char.Skills, ", ")))
			}
			if char.Voice != "" {
				sb.WriteString(fmt.Sprintf("    - Voice: %s\n", char.Voice))
			}
			if char.Notes != "" {
				sb.WriteString(fmt.Sprintf("    - Notes: %s\n", char.Notes))
			}
			// Goals from RPGState (canonical source)
			if state.RPG != nil && state.RPG.Characters[charName] != nil {
				rpgChar := state.RPG.Characters[charName]
				if len(rpgChar.Goals) > 0 {
					sb.WriteString(fmt.Sprintf("    - Current Goals: %s\n", strings.Join(rpgChar.Goals, ", ")))
				}
			}
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

	// Active storylines from RPGState (canonical source)
	if state.RPG != nil && len(state.RPG.Storylines) > 0 {
		activeStorylines := make(map[string]*models.RPGQuestState)
		for id, sl := range state.RPG.Storylines {
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

				if len(sl.ProgressHistory) > 0 {
					sb.WriteString("  Progress History:\n")
					for i, progress := range sl.ProgressHistory {
						sb.WriteString(fmt.Sprintf("    Step %d [%s]: %s - %s\n",
							i+1, progress.ChapterID, progress.Status, progress.Details))
					}
				}

				if id != sl.Name {
					sb.WriteString(fmt.Sprintf("  [ID: %s]\n", id))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Character relationships from RPGState (canonical source)
	if state.RPG != nil && len(state.RPG.Relationships) > 0 {
		relevantRelations := make(map[string]*models.RPGRelationState)
		for _, rel := range state.RPG.Relationships {
			for _, chapterChar := range chapter.Characters {
				if rel.From == chapterChar || rel.To == chapterChar {
					relevantRelations[rel.From+"_"+rel.To] = rel
					break
				}
			}
		}
		if len(relevantRelations) > 0 {
			sb.WriteString("Key Relationships:\n")
			for _, rel := range relevantRelations {
				sb.WriteString(fmt.Sprintf("- %s → %s: %s", rel.From, rel.To, rel.Status))
				if rel.Details != "" {
					sb.WriteString(fmt.Sprintf(" (%s)", rel.Details))
				}
				sb.WriteString("\n")
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

	// Active Gates/Obstacles affecting characters in this chapter
	if len(state.Gates) > 0 {
		relevantGates := make(map[string]*models.GateState)
		for gateName, gate := range state.Gates {
			// Check if gate affects any character in this chapter
			for _, chapterChar := range chapter.Characters {
				if gate.Characters == chapterChar {
					relevantGates[gateName] = gate
					break
				}
			}
		}
		if len(relevantGates) > 0 {
			sb.WriteString("Active Obstacles/Gates:\n")
			for _, gate := range relevantGates {
				sb.WriteString(fmt.Sprintf("- %s", gate.Name))
				if gate.Status != "" {
					sb.WriteString(fmt.Sprintf(" [%s]", gate.Status))
				}
				if gate.Characters != "" {
					sb.WriteString(fmt.Sprintf(" (affecting: %s)", gate.Characters))
				}
				sb.WriteString("\n")
				if gate.Details != "" {
					sb.WriteString(fmt.Sprintf("  Details: %s\n", gate.Details))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Character Status (physical/mental states)
	if len(state.Status) > 0 {
		relevantStatus := make(map[string]*models.StatusState)
		for statusKey, status := range state.Status {
			// Check if status affects any character in this chapter
			for _, chapterChar := range chapter.Characters {
				if strings.HasPrefix(statusKey, chapterChar+"_") {
					relevantStatus[statusKey] = status
					break
				}
			}
		}
		if len(relevantStatus) > 0 {
			sb.WriteString("Character Status:\n")
			for _, status := range relevantStatus {
				sb.WriteString(fmt.Sprintf("- %s: %s", status.Type, status.State))
				if status.Severity != "" {
					sb.WriteString(fmt.Sprintf(" (%s)", status.Severity))
				}
				sb.WriteString("\n")
				if status.Details != "" {
					sb.WriteString(fmt.Sprintf("  Details: %s\n", status.Details))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Character Memories/Information
	if len(state.Memories) > 0 {
		hasRelevantMemories := false
		for _, chapterChar := range chapter.Characters {
			if memories, exists := state.Memories[chapterChar]; exists && len(memories) > 0 {
				if !hasRelevantMemories {
					sb.WriteString("Character Knowledge/Memories:\n")
					hasRelevantMemories = true
				}
				sb.WriteString(fmt.Sprintf("- %s knows:\n", chapterChar))
				for _, memory := range memories {
					sb.WriteString(fmt.Sprintf("  • [%s] %s\n", memory.Category, memory.Info))
					if memory.Details != "" {
						sb.WriteString(fmt.Sprintf("    %s\n", memory.Details))
					}
				}
			}
		}
		if hasRelevantMemories {
			sb.WriteString("\n")
		}
	}

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

	sb.WriteString("=== END CONTINUITY ===\n")

	return sb.String()
}

// FormatRPGState formats the structured RPG state for writing prompts.
func FormatRPGState(state *models.RPGState, chapter *models.Chapter) string {
	if state == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("RPG STATE (structured, authoritative):\n")
	if state.CurrentChapter != "" || state.CurrentLocation != "" {
		sb.WriteString(fmt.Sprintf("- Current: chapter=%s location=%s\n", state.CurrentChapter, state.CurrentLocation))
	}

	relevantChars := map[string]bool{}
	if chapter != nil {
		for _, name := range chapter.Characters {
			relevantChars[name] = true
		}
		for _, event := range chapter.Events {
			if actor := event.GetActor(); actor != "" {
				relevantChars[actor] = true
			}
			if target := event.GetTarget(); target != "" && event.GetTargetType() == models.TargetTypeCharacter {
				relevantChars[target] = true
			}
			for _, name := range event.Characters {
				relevantChars[name] = true
			}
		}
	}

	characterNames := sortedRPGCharacterNames(state.Characters)
	if len(characterNames) > 0 {
		sb.WriteString("- Characters:\n")
		for _, name := range characterNames {
			if len(relevantChars) > 0 && !relevantChars[name] {
				continue
			}
			char := state.Characters[name]
			sb.WriteString(fmt.Sprintf("  - %s", char.Name))
			if char.Role != "" {
				sb.WriteString(fmt.Sprintf(" role=%s", char.Role))
			}
			if char.Location != "" {
				sb.WriteString(fmt.Sprintf(" location=%s", char.Location))
			}
			if char.Realm != "" {
				sb.WriteString(fmt.Sprintf(" realm=%s", char.Realm))
			}
			if char.Level > 0 {
				sb.WriteString(fmt.Sprintf(" level=%d", char.Level))
			}
			if !char.Alive {
				sb.WriteString(" alive=false")
			}
			sb.WriteString("\n")
			writeStringMap(&sb, "status", char.Status, "    ")
			writeIntMap(&sb, "inventory", char.Inventory, "    ")
			if len(char.Goals) > 0 {
				sb.WriteString(fmt.Sprintf("    goals=%s\n", strings.Join(char.Goals, " | ")))
			}
			if len(char.Knowledge) > 0 {
				sb.WriteString(fmt.Sprintf("    knowledge=%s\n", strings.Join(char.Knowledge, " | ")))
			}
		}
	}

	if len(state.Relationships) > 0 {
		sb.WriteString("- Relationships:\n")
		for _, key := range sortedRPGRelationKeys(state.Relationships) {
			rel := state.Relationships[key]
			if len(relevantChars) > 0 && !relevantChars[rel.From] && !relevantChars[rel.To] {
				continue
			}
			sb.WriteString(fmt.Sprintf("  - %s -> %s: %s", rel.From, rel.To, rel.Status))
			if rel.Details != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", rel.Details))
			}
			sb.WriteString("\n")
		}
	}

	if len(state.Resources) > 0 {
		var resourceLines strings.Builder
		for _, key := range sortedRPGResourceKeys(state.Resources) {
			res := state.Resources[key]
			if res.Owner == "" && res.Status == "available" {
				continue
			}
			resourceLines.WriteString(fmt.Sprintf("  - %s owner=%s qty=%d status=%s\n", res.Name, res.Owner, res.Quantity, res.Status))
		}
		if resourceLines.Len() > 0 {
			sb.WriteString("- Resources:\n")
			sb.WriteString(resourceLines.String())
		}
	}

	if len(state.Storylines) > 0 {
		sb.WriteString("- Storylines:\n")
		for _, key := range sortedRPGQuestKeys(state.Storylines) {
			quest := state.Storylines[key]
			sb.WriteString(fmt.Sprintf("  - %s status=%s progress=%s\n", quest.Name, quest.Status, quest.Progress))
		}
	}

	if len(state.Deltas) > 0 {
		sb.WriteString("- Recent State Deltas:\n")
		start := len(state.Deltas) - 8
		if start < 0 {
			start = 0
		}
		for _, delta := range state.Deltas[start:] {
			sb.WriteString(fmt.Sprintf("  - [%s] %s.%s %s -> %s", delta.ChapterID, delta.Target, delta.Field, delta.From, delta.To))
			if delta.Kind != "" {
				sb.WriteString(fmt.Sprintf(" kind=%s", delta.Kind))
			}
			if delta.Delta != 0 {
				sb.WriteString(fmt.Sprintf(" delta=%d", delta.Delta))
			}
			if delta.Note != "" {
				sb.WriteString(fmt.Sprintf(" note=%s", delta.Note))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("=== END RPG STATE ===\n")
	return sb.String()
}

func FormatRPGStateComparison(comparison RPGStateComparison) string {
	var sb strings.Builder
	sb.WriteString("RPG STATE COMPARISON (outline expected vs DSL observed):\n")
	if comparison.ChapterID != "" {
		sb.WriteString(fmt.Sprintf("- Chapter: %s\n", comparison.ChapterID))
	}
	if len(comparison.Drifts) == 0 {
		sb.WriteString("- No state drift detected.\n")
		sb.WriteString("=== END RPG STATE COMPARISON ===\n")
		return sb.String()
	}
	summary := map[string]int{}
	for _, drift := range comparison.Drifts {
		summary[drift.Severity]++
	}
	sb.WriteString(fmt.Sprintf("- Summary: critical=%d warning=%d info=%d\n", summary["critical"], summary["warning"], summary["info"]))
	for _, drift := range comparison.Drifts {
		sb.WriteString(fmt.Sprintf("  - [%s/%s] %s", drift.Severity, drift.Kind, drift.Target))
		if drift.Field != "" {
			sb.WriteString("." + drift.Field)
		}
		if drift.Expected != "" || drift.Observed != "" {
			sb.WriteString(fmt.Sprintf(" expected=%q observed=%q", drift.Expected, drift.Observed))
		}
		if drift.Note != "" {
			sb.WriteString(" note=" + drift.Note)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("=== END RPG STATE COMPARISON ===\n")
	return sb.String()
}

func sortedRPGCharacterNames(values map[string]*models.RPGCharacterState) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRPGResourceKeys(values map[string]*models.RPGResourceState) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRPGRelationKeys(values map[string]*models.RPGRelationState) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedRPGQuestKeys(values map[string]*models.RPGQuestState) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeStringMap(sb *strings.Builder, label string, values map[string]string, indent string) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	sb.WriteString(fmt.Sprintf("%s%s=%s\n", indent, label, strings.Join(parts, ", ")))
}

func writeIntMap(sb *strings.Builder, label string, values map[string]int, indent string) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	sb.WriteString(fmt.Sprintf("%s%s=%s\n", indent, label, strings.Join(parts, ", ")))
}

// formatItems formats the items section of the state matrix
func formatItems(sb *strings.Builder, state *models.ChapterContinuity, chapter *models.Chapter) {
	// Items relevant to this chapter
	relevantItems := make(map[string]string) // itemName -> description

	// 1. Items owned by characters in this chapter (from RPGState Resources)
	if state.RPG != nil {
		for itemName, res := range state.RPG.Resources {
			if res.Owner != "" {
				for _, charName := range chapter.Characters {
					if res.Owner == charName {
						relevantItems[itemName] = fmt.Sprintf("owned by %s (qty: %d)", charName, res.Quantity)
						break
					}
				}
			}
		}
	}

	// 1b. Fallback: also check state.Items for items not yet in RPGState
	for itemName, item := range state.Items {
		if _, already := relevantItems[itemName]; already {
			continue
		}
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
