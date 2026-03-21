package models

// StateMatrix represents the current state of the story at a specific point
type StateMatrix struct {
	Characters    map[string]*Character // Character name -> Character (static attributes)
	Locations     map[string]*Location  // Location name -> Location
	Items         map[string]*Item      // Item name -> Item
	Relationships map[string]string     // "char1_char2" -> relationship state
	Goals         map[string][]string   // character name -> current goals
	Storylines    map[string]string     // storyline ID -> current state
	Premises      map[string]string     // premise ID -> current state for each character
}
