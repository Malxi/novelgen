package models

// Character represents a detailed character profile
type Character struct {
	Name          string            `json:"name"`
	Aliases       []string          `json:"aliases,omitempty"`
	Age           string            `json:"age,omitempty"`
	Gender        string            `json:"gender,omitempty"`
	Appearance    string            `json:"appearance"`
	Personality   []string          `json:"personality"`
	Background    string            `json:"background"`
	Motivation    string            `json:"motivation"`
	Goals         []string          `json:"goals"`
	Fears         []string          `json:"fears,omitempty"`
	Skills        []string          `json:"skills,omitempty"`
	Relationships map[string]string `json:"relationships,omitempty"`
	RoleInStory   string            `json:"role_in_story"`
	CharacterArc  string            `json:"character_arc,omitempty"`
	Voice         string            `json:"voice,omitempty"`
	Notes         string            `json:"notes,omitempty"`
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
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Appearance   string   `json:"appearance"`
	Function     string   `json:"function"`
	Origin       string   `json:"origin,omitempty"`
	History      string   `json:"history,omitempty"`
	Powers       []string `json:"powers,omitempty"`
	Limitations  []string `json:"limitations,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	Significance string   `json:"significance"`
	RelatedItems []string `json:"related_items,omitempty"`
	Secrets      string   `json:"secrets,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}
