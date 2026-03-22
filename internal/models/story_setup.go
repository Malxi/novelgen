package models

import (
	"encoding/json"
	"os"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string      `json:"project_name" prompt:"Project Name" desc:"2-6 words, evocative, <= 60 characters"`
	Genres         []string    `json:"genres" prompt:"Genres" desc:"2-4 specific genres"`
	Premise        string      `json:"premise" prompt:"Premise" desc:"2-4 sentences, no lists"`
	Theme          string      `json:"theme" prompt:"Theme" desc:"Clear statement, not a single word"`
	Rules          []string    `json:"rules" prompt:"Rules" desc:"3-7 enforceable rules"`
	TargetAudience string      `json:"target_audience" prompt:"Target Audience" desc:"Include age range and readership type"`
	Tone           string      `json:"tone" prompt:"Tone" desc:"2-4 adjectives, comma-separated"`
	Tense          string      `json:"tense" prompt:"Tense" desc:"past or present"`
	POVStyle       string      `json:"pov_style" prompt:"POV Style" desc:"first person, third person limited, or third person omniscient"`
	Storylines     []Storyline `json:"storylines,omitempty" prompt:"Storylines" desc:"3-5 items; include at least one main and one subplot or character_arc"`
	Premises       []Premise   `json:"premises,omitempty" prompt:"Premises" desc:"1-3 items tied to setting or power system"`
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name        string `json:"name" prompt:"Name" desc:"Short, specific"`
	Description string `json:"description" prompt:"Description" desc:"2-4 sentences"`
	Type        string `json:"type" prompt:"Type" desc:"main, subplot, or character_arc"` // main, subplot, character_arc, etc.
	Importance  int    `json:"importance" prompt:"Importance" desc:"1-10, 10 most important"` // 1-10, 10 being most important
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name        string             `json:"name" prompt:"Name" desc:"Specific and setting-tied"`
	Description string             `json:"description" prompt:"Description" desc:"2-4 sentences"`
	Category    string             `json:"category" prompt:"Category" desc:"e.g., mecha, gene, ship, magic"` // 机甲, 基因, 飞船, 魔法, etc.
	Progression []ProgressionStage `json:"progression" prompt:"Progression" desc:"3-5 stages minimum"` // 升级体系
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level" prompt:"Level" desc:"Start at 1, increment by 1"`
	Name         string `json:"name" prompt:"Name" desc:"Distinct stage title"`
	Description  string `json:"description" prompt:"Description" desc:"What changes at this stage"`
	Requirements string `json:"requirements,omitempty" prompt:"Requirements" desc:"Conditions to advance"`
}

// Save writes the story setup to a file
func (s *StorySetup) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStorySetup reads the story setup from a file
func LoadStorySetup(path string) (*StorySetup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var setup StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil, err
	}
	return &setup, nil
}
