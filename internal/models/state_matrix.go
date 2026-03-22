package models

// StorylineProgress represents a single step in storyline progression
type StorylineProgress struct {
	ChapterID string `json:"chapter_id"` // Which chapter this progress happened
	Status    string `json:"status"`     // Status at this step (started, progressed, etc.)
	Details   string `json:"details"`    // Progress description
}

// StorylineState represents the current state of a storyline
type StorylineState struct {
	Name            string              // Storyline name/title
	Description     string              // Storyline description
	Status          string              // Current status (started, progressed, completed, etc.)
	Progress        string              // Current progress description
	ProgressHistory []StorylineProgress // All progress steps (accumulated)
}

// GateState represents a gate/obstacle in the story
type GateState struct {
	Name       string `json:"name"`       // Gate name (e.g., "搜魂阵旗")
	Status     string `json:"status"`     // introduced/escalated/overcome
	Characters string `json:"characters"` // Characters affected
	ChapterID  string `json:"chapter_id"` // Chapter where this gate exists
	Details    string `json:"details"`    // Description of the gate
}

// StatusState represents a character's physical/mental status
type StatusState struct {
	Type      string `json:"type"`       // Status type (injury, poison, emotion, etc.)
	State     string `json:"state"`      // Current state (e.g., "bleeding", "angry", "fatigued")
	Severity  string `json:"severity"`   // light/moderate/severe
	ChapterID string `json:"chapter_id"` // Chapter where status was introduced/changed
	Details   string `json:"details"`    // Description
}

// MemoryState represents information/memory a character has learned
type MemoryState struct {
	Info      string `json:"info"`       // The information/memory content
	Category  string `json:"category"`   // info/secret/knowledge/event
	ChapterID string `json:"chapter_id"` // Chapter where memory was acquired
	Details   string `json:"details"`    // Additional context
}

// StateMatrix represents the current state of the story at a specific point
type StateMatrix struct {
	Characters    map[string]*Character      // Character name -> Character (static attributes)
	Locations     map[string]*Location       // Location name -> Location
	Items         map[string]*Item           // Item name -> Item
	Relationships map[string]string          // "char1_char2" -> relationship state
	Goals         map[string][]string        // character name -> current goals
	Storylines    map[string]*StorylineState // storyline ID -> storyline state with description
	Premises      map[string]string          // premise ID -> current state for each character
	Gates         map[string]*GateState      // gate name -> gate state
	Status        map[string]*StatusState    // character_name_status_type -> status state
	Memories      map[string][]*MemoryState  // character name -> list of memories
}
