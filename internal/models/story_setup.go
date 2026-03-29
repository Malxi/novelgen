package models

import (
	"encoding/json"
	"os"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string      `json:"project_name" prompt:"小说名称" desc:"2-6个字，有吸引力，不超过60个字符"`
	Genres         []string    `json:"genres" prompt:"类型" desc:"2-4个具体小说类型"`
	Premise        string      `json:"premise" prompt:"核心设定" desc:"核心设定，2-4句话，不要列表"`
	Theme          string      `json:"theme" prompt:"主题" desc:"主题，清晰的陈述，不要单个词"`
	Rules          []string    `json:"rules" prompt:"规则" desc:"小说设定的规则"`
	TargetAudience string      `json:"target_audience" prompt:"目标读者" desc:"包含年龄段和读者类型"`
	Tone           string      `json:"tone" prompt:"基调" desc:"小说基调，一句话，2-4个形容词，逗号分隔"`
	Tense          string      `json:"tense" prompt:"时态" desc:"过去时或现在时"`
	POVStyle       string      `json:"pov_style" prompt:"视角" desc:"第一人称、第三人称有限视角或第三人称全知视角"`
	Storylines     []Storyline `json:"storylines,omitempty" prompt:"故事线" desc:"故事线"`
	Premises       []Premise   `json:"premises,omitempty" prompt:"设定体系" desc:"设定升级体系"`
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name        string `json:"name" prompt:"Name" desc:"故事线名称"`
	Description string `json:"description" prompt:"Description" desc:"故事线描述，2-4句话"`
	Type        string `json:"type" prompt:"Type" desc:"故事线类型"`                   // main, subplot, character_arc, etc.
	Importance  int    `json:"importance" prompt:"Importance" desc:"故事线重要性，1-10"` // 1-10, 10 being most important
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name        string             `json:"name" prompt:"Name" desc:"设定体系名称"`
	Description string             `json:"description" prompt:"Description" desc:"设定体系描述，2-4句话"`
	Category    string             `json:"category" prompt:"Category" desc:"设定体系类型"`         // 机甲, 基因, 飞船, 魔法, etc.
	Progression []ProgressionStage `json:"progression" prompt:"Progression" desc:"设定体系升级体系"` // 升级体系
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level" prompt:"Level" desc:"设定体系升级体系等级"`
	Name         string `json:"name" prompt:"Name" desc:"设定体系升级体系名称"`
	Description  string `json:"description" prompt:"Description" desc:"设定体系升级体系描述"`
	Requirements string `json:"requirements,omitempty" prompt:"Requirements" desc:"设定体系升级体系要求"`
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
