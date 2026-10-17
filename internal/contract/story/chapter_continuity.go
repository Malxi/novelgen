package models

// ChapterContinuity is the writer-facing continuity snapshot before a target
// chapter begins. It replaces direct write-stage dependence on StateMatrix while
// preserving the same continuity facts during the migration.
type ChapterContinuity struct {
	RPG        *RPGState             `json:"rpg,omitempty"`
	Characters map[string]*Character `json:"characters,omitempty"`
	Locations  map[string]*Location  `json:"locations,omitempty"`
	Items      map[string]*Item      `json:"items,omitempty"`

	Premises map[string]string         `json:"premises,omitempty"`
	Gates    map[string]*GateState     `json:"gates,omitempty"`
	Status   map[string]*StatusState   `json:"status,omitempty"`
	Memories map[string][]*MemoryState `json:"memories,omitempty"`
}
