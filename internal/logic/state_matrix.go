package logic

import (
	"encoding/json"
	"os"
	"path/filepath"

	"novelgen/internal/models"
)

// StateMatrixManager handles StateMatrix calculations and operations
type StateMatrixManager struct {
	projectRoot string
}

// NewStateMatrixManager creates a new StateMatrixManager
func NewStateMatrixManager(projectRoot string) *StateMatrixManager {
	return &StateMatrixManager{projectRoot: projectRoot}
}

// CalculateStateMatrix calculates the story state up to the target chapter
func (m *StateMatrixManager) CalculateStateMatrix(outline *models.Outline, targetChapter *models.Chapter) *models.StateMatrix {
	state := &models.StateMatrix{
		Characters:    make(map[string]*models.Character),
		Locations:     make(map[string]*models.Location),
		Items:         make(map[string]*models.Item),
		Relationships: make(map[string]string),
		Goals:         make(map[string][]string),
		Storylines:    make(map[string]string),
		Premises:      make(map[string]string),
	}

	// Load all generated elements
	m.loadElementsIntoState(state)

	// Apply events from all chapters up to target
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Stop when we reach target chapter
				if ch.ID == targetChapter.ID {
					return state
				}

				// Apply events from this chapter
				for _, event := range ch.Events {
					m.applyEvent(state, event)
				}
			}
		}
	}

	return state
}

// applyEvent applies a single event to the state matrix
func (m *StateMatrixManager) applyEvent(state *models.StateMatrix, event models.Event) {
	switch event.Type {
	case "relationship":
		// Format: relationship between char1 and char2 changes
		if len(event.Characters) >= 2 {
			key := event.Characters[0] + "_" + event.Characters[1]
			state.Relationships[key] = event.Change
		}
	case "goal":
		// Character goal update
		if len(event.Characters) > 0 {
			charName := event.Characters[0]
			// Update character's goals in state matrix
			if event.Change != "" {
				state.Goals[charName] = append(state.Goals[charName], event.Change)
			}
		}
	case "item":
		// Character gets or loses item
		if len(event.Characters) > 0 && event.Subject != "" {
			charName := event.Characters[0]
			itemName := event.Subject
			if event.Change == "get" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = charName
				}
			} else if event.Change == "lost" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = ""
				}
			}
		}
	case "premise":
		// Character premise/progression update
		if len(event.Characters) > 0 {
			key := event.Characters[0] + "_" + event.Subject
			state.Premises[key] = event.Change
		}
	case "storyline":
		// Storyline progression
		if event.Subject != "" {
			state.Storylines[event.Subject] = event.Change
		}
	}
}

// loadElementsIntoState loads generated elements into state matrix
func (m *StateMatrixManager) loadElementsIntoState(state *models.StateMatrix) {
	if m.projectRoot == "" {
		return
	}

	// Load characters
	charPath := filepath.Join(m.projectRoot, "story", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		var chars map[string]*models.Character
		if err := json.Unmarshal(data, &chars); err == nil {
			for name, char := range chars {
				state.Characters[name] = char
			}
		}
	}

	// Load locations
	locPath := filepath.Join(m.projectRoot, "story", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		var locs map[string]*models.Location
		if err := json.Unmarshal(data, &locs); err == nil {
			for name, loc := range locs {
				state.Locations[name] = loc
			}
		}
	}

	// Load items
	itemPath := filepath.Join(m.projectRoot, "story", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		var items map[string]*models.Item
		if err := json.Unmarshal(data, &items); err == nil {
			for name, item := range items {
				state.Items[name] = item
			}
		}
	}
}
