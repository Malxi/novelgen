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
	setup       *models.StorySetup // cached setup for storyline descriptions
}

// NewStateMatrixManager creates a new StateMatrixManager
func NewStateMatrixManager(projectRoot string) *StateMatrixManager {
	return &StateMatrixManager{projectRoot: projectRoot}
}

// loadSetup loads the story setup from file
func (m *StateMatrixManager) loadSetup() *models.StorySetup {
	if m.setup != nil {
		return m.setup
	}
	if m.projectRoot == "" {
		return nil
	}

	setupPath := filepath.Join(m.projectRoot, "story", "setup", "story_setup.json")
	data, err := os.ReadFile(setupPath)
	if err != nil {
		return nil
	}

	var setup models.StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil
	}

	m.setup = &setup
	return m.setup
}

// CalculateStateMatrix calculates the story state up to the target chapter
func (m *StateMatrixManager) CalculateStateMatrix(outline *models.Outline, targetChapter *models.Chapter) *models.StateMatrix {
	state := &models.StateMatrix{
		Characters:    make(map[string]*models.Character),
		Locations:     make(map[string]*models.Location),
		Items:         make(map[string]*models.Item),
		Relationships: make(map[string]string),
		Goals:         make(map[string][]string),
		Storylines:    make(map[string]*models.StorylineState),
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
					// Process storyline events from target chapter to get descriptions
					// but mark them as "starting" rather than "started"
					for _, event := range ch.Events {
						if event.Type == "storyline" && event.Subject != "" {
							m.applyStorylineEventWithDescription(state, event, ch.ID)
						}
					}
					return state
				}

				// Apply events from this chapter
				for _, event := range ch.Events {
					m.applyEvent(state, event, ch.ID)
				}
			}
		}
	}

	return state
}

// applyEvent applies a single event to the state matrix
func (m *StateMatrixManager) applyEvent(state *models.StateMatrix, event models.Event, chapterID string) {
	switch event.Type {
	case "relationship":
		// Format: relationship between char1 and char2 changes
		if len(event.Characters) >= 2 {
			key := event.Characters[0] + "_" + event.Characters[1]
			state.Relationships[key] = event.Change
		}
	case "goal":
		// Character goal update
		if len(event.Characters) > 0 && event.Change != "" {
			charName := event.Characters[0]
			change := event.Change

			// Skip if goal already exists (deduplication)
			for _, existing := range state.Goals[charName] {
				if existing == change {
					return
				}
			}

			// Remove completed/abandoned goals to prevent accumulation
			if change == "achieved" || change == "abandoned" {
				// Remove the goal that was achieved/abandoned
				if event.Subject != "" {
					newGoals := []string{}
					for _, g := range state.Goals[charName] {
						if g != event.Subject {
							newGoals = append(newGoals, g)
						}
					}
					state.Goals[charName] = newGoals
				}
				return
			}

			// Add new goal
			state.Goals[charName] = append(state.Goals[charName], change)

			// Limit goals per character to prevent explosion (keep most recent 5)
			const maxGoals = 5
			if len(state.Goals[charName]) > maxGoals {
				state.Goals[charName] = state.Goals[charName][len(state.Goals[charName])-maxGoals:]
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
		// Storyline progression - accumulate history
		if event.Subject != "" {
			// Find storyline description from setup
			storylineName := event.Subject
			storylineDesc := ""
			if setup := m.loadSetup(); setup != nil {
				for _, sl := range setup.Storylines {
					if sl.Name == event.Subject {
						storylineName = sl.Name
						storylineDesc = sl.Description
						break
					}
				}
			}

			// Get or create storyline state
			slState, exists := state.Storylines[event.Subject]
			if !exists {
				slState = &models.StorylineState{
					Name:            storylineName,
					Description:     storylineDesc,
					ProgressHistory: []models.StorylineProgress{},
				}
				state.Storylines[event.Subject] = slState
			}

			// Update current status and progress
			slState.Status = event.Change
			slState.Progress = event.Details

			// Append to history
			slState.ProgressHistory = append(slState.ProgressHistory, models.StorylineProgress{
				ChapterID: chapterID,
				Status:    event.Change,
				Details:   event.Details,
			})
		}
	}
}

// applyStorylineEventWithDescription applies a storyline event to get its description
// This is used for storyline events in the current chapter so AI knows what the storyline is about
func (m *StateMatrixManager) applyStorylineEventWithDescription(state *models.StateMatrix, event models.Event, chapterID string) {
	if event.Subject == "" {
		return
	}

	// Find storyline description from setup
	storylineName := event.Subject
	storylineDesc := ""
	if setup := m.loadSetup(); setup != nil {
		for _, sl := range setup.Storylines {
			if sl.Name == event.Subject {
				storylineName = sl.Name
				storylineDesc = sl.Description
				break
			}
		}
	}

	// Get or create storyline state
	slState, exists := state.Storylines[event.Subject]
	if !exists {
		slState = &models.StorylineState{
			Name:            storylineName,
			Description:     storylineDesc,
			ProgressHistory: []models.StorylineProgress{},
		}
		state.Storylines[event.Subject] = slState
	}

	// Update with "(starting this chapter)" marker
	slState.Status = event.Change + " (starting this chapter)"
	slState.Progress = event.Details

	// Append to history
	slState.ProgressHistory = append(slState.ProgressHistory, models.StorylineProgress{
		ChapterID: chapterID,
		Status:    event.Change,
		Details:   event.Details,
	})
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
