package models

// StorylineState represents the current state of a storyline
type StorylineState struct {
	Name        string // Storyline name/title
	Description string // Storyline description
	Status      string // Current status (started, progressed, completed, etc.)
	Progress    string // Current progress description (e.g., "主角刚发现线索", "追逐战即将开始")
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
}
