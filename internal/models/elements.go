package models

// Character represents a detailed character profile
// Note: Dynamic fields like relationships, goals, character_arc are managed by StateMatrix
type Character struct {
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases,omitempty"`
	Age          string   `json:"age,omitempty"`
	Gender       string   `json:"gender,omitempty"`
	Race         string   `json:"race,omitempty"`
	Appearance   string   `json:"appearance"`
	Personality  []string `json:"personality"`
	Background   string   `json:"background"`
	Motivation   string   `json:"motivation"`
	Skills       []string `json:"skills,omitempty"`
	Abilities    []string `json:"abilities,omitempty"`
	Affiliations []string `json:"affiliations,omitempty"`
	RoleInStory  string   `json:"role_in_story"`
	Voice        string   `json:"voice,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

// Location represents a detailed location description
type Location struct {
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Description        string          `json:"description"`
	Appearance         string          `json:"appearance"`
	Atmosphere         string          `json:"atmosphere"`
	SensoryDetails     *SensoryDetails `json:"sensory_details,omitempty"`
	Significance       string          `json:"significance"`
	History            string          `json:"history,omitempty"`
	Inhabitants        []string        `json:"inhabitants,omitempty"`
	ConnectedLocations []string        `json:"connected_locations,omitempty"`
	Events             []string        `json:"events,omitempty"`
	Secrets            string          `json:"secrets,omitempty"`
	Notes              string          `json:"notes,omitempty"`
}

// SensoryDetails contains sensory information about a location
type SensoryDetails struct {
	Sights   []string `json:"sights,omitempty"`
	Sounds   []string `json:"sounds,omitempty"`
	Smells   []string `json:"smells,omitempty"`
	Textures []string `json:"textures,omitempty"`
}

// Item represents a detailed item description
type Item struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Description  string     `json:"description"`
	Appearance   string     `json:"appearance"`
	Function     string     `json:"function"`
	Origin       string     `json:"origin,omitempty"`
	History      string     `json:"history,omitempty"`
	Powers       []string   `json:"powers,omitempty"`
	Limitations  []string   `json:"limitations,omitempty"`
	Owner        string     `json:"owner,omitempty"`
	Significance string     `json:"significance"`
	RelatedItems []string   `json:"related_items,omitempty"`
	Secrets      StringList `json:"secrets,omitempty"`
	Notes        string     `json:"notes,omitempty"`
}

// Organization represents a faction, guild, or organization in the story
type Organization struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Founding     string   `json:"founding,omitempty"`
	Headquarters string   `json:"headquarters,omitempty"`
	Leadership   string   `json:"leadership,omitempty"`
	Members      []string `json:"members,omitempty"`
	Goals        []string `json:"goals"`
	Ideology     string   `json:"ideology,omitempty"`
	Resources    []string `json:"resources,omitempty"`
	Allies       []string `json:"allies,omitempty"`
	Enemies      []string `json:"enemies,omitempty"`
	Reputation   string   `json:"reputation,omitempty"`
	Structure    string   `json:"structure,omitempty"`
	Significance string   `json:"significance"`
	Secrets      string   `json:"secrets,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

// Race represents a species or race in the story world
type Race struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Appearance   string   `json:"appearance"`
	Traits       []string `json:"traits"`
	Abilities    []string `json:"abilities,omitempty"`
	Weaknesses   []string `json:"weaknesses,omitempty"`
	Lifespan     string   `json:"lifespan,omitempty"`
	Culture      string   `json:"culture,omitempty"`
	Society      string   `json:"society,omitempty"`
	Habitat      string   `json:"habitat,omitempty"`
	Diet         string   `json:"diet,omitempty"`
	Reproduction string   `json:"reproduction,omitempty"`
	Language     string   `json:"language,omitempty"`
	Relations    []string `json:"relations,omitempty"`
	History      string   `json:"history,omitempty"`
	Significance string   `json:"significance"`
	Notes        string   `json:"notes,omitempty"`
}

// AbilitySystem represents a magic system, cultivation system, or skill system
type AbilitySystem struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Source        string   `json:"source,omitempty"`
	Mechanics     string   `json:"mechanics"`
	Levels        []string `json:"levels,omitempty"`
	Requirements  []string `json:"requirements,omitempty"`
	Limitations   []string `json:"limitations,omitempty"`
	Costs         []string `json:"costs,omitempty"`
	Applications  []string `json:"applications,omitempty"`
	Practitioners []string `json:"practitioners,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
	RelatedItems  []string `json:"related_items,omitempty"`
	Significance  string   `json:"significance"`
	Notes         string   `json:"notes,omitempty"`
}

// WorldLore represents world-building elements like history, culture, or rules
type WorldLore struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Description     string   `json:"description"`
	Content         string   `json:"content"`
	Origin          string   `json:"origin,omitempty"`
	Significance    string   `json:"significance"`
	RelatedElements []string `json:"related_elements,omitempty"`
	Notes           string   `json:"notes,omitempty"`
}
