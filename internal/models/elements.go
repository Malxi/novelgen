package models

// Character represents a detailed character profile
// Note: Dynamic fields like relationships, goals, character_arc are managed by StateMatrix
type Character struct {
	Name         string   `json:"name" desc:"Character name"`
	Aliases      []string `json:"aliases,omitempty" desc:"Alternative names or titles (array of strings)"`
	Age          string   `json:"age,omitempty" desc:"Character age or age range"`
	Gender       string   `json:"gender,omitempty" desc:"Character gender"`
	Race         string   `json:"race,omitempty" desc:"Character race/species"`
	Appearance   string   `json:"appearance" desc:"Physical appearance description"`
	Personality  []string `json:"personality" desc:"Personality traits (array of strings)"`
	Background   string   `json:"background" desc:"Character backstory"`
	Motivation   string   `json:"motivation" desc:"Character's core motivation"`
	Skills       []string `json:"skills,omitempty" desc:"Skills and abilities (array of strings)"`
	Abilities    []string `json:"abilities,omitempty" desc:"Special powers or abilities (array of strings)"`
	Affiliations []string `json:"affiliations,omitempty" desc:"Organizations the character belongs to (array of strings)"`
	RoleInStory  string   `json:"role_in_story" desc:"Character's role in the story (protagonist/antagonist/supporting/etc)"`
	Voice        string   `json:"voice,omitempty" desc:"Speaking style and mannerisms"`
	Notes        string   `json:"notes,omitempty" desc:"Additional notes for writers (string, NOT array)"`
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
	Name         string     `json:"name" desc:"Item name"`
	Type         string     `json:"type" desc:"Item type/category"`
	Description  string     `json:"description" desc:"Item description"`
	Appearance   string     `json:"appearance" desc:"Physical appearance"`
	Function     string     `json:"function" desc:"What the item does"`
	Origin       string     `json:"origin,omitempty" desc:"Where the item comes from"`
	History      string     `json:"history,omitempty" desc:"Item's history/background"`
	Powers       []string   `json:"powers,omitempty" desc:"Special powers (array of strings)"`
	Limitations  []string   `json:"limitations,omitempty" desc:"Limitations or drawbacks (array of strings)"`
	Owner        string     `json:"owner,omitempty" desc:"Current owner"`
	Significance string     `json:"significance" desc:"Story significance"`
	RelatedItems []string   `json:"related_items,omitempty" desc:"Related items (array of strings)"`
	Secrets      StringList `json:"secrets,omitempty" desc:"Secrets about the item (array of strings)"`
	Notes        string     `json:"notes,omitempty" desc:"Additional notes (string, NOT array)"`
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
