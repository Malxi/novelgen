package models

import (
	"encoding/json"
	"os"
	"time"
)

// ProjectConfig represents the novel.json configuration file
type ProjectConfig struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Story language (e.g., "zh" for Chinese, "en" for English)
	Language string `json:"language"`

	// Story structure configuration (user-defined)
	Structure StoryStructure `json:"structure"`

	// Chapter writing configuration
	ChapterConfig ChapterConfig `json:"chapter_config"`

	// LLM configuration for this project (only provider and model name)
	LLM ProjectLLM `json:"llm"`
}

// StoryStructure defines the target story structure
type StoryStructure struct {
	TargetParts    int `json:"target_parts"`    // Number of parts (部)
	TargetVolumes  int `json:"target_volumes"`  // Number of volumes per part (卷)
	TargetChapters int `json:"target_chapters"` // Number of chapters per volume (章)
}

// DefaultStoryStructure returns a default story structure
func DefaultStoryStructure() StoryStructure {
	return StoryStructure{
		TargetParts:    1,
		TargetVolumes:  1,
		TargetChapters: 20,
	}
}

// TotalChapters returns the total number of chapters
func (s *StoryStructure) TotalChapters() int {
	return s.TargetParts * s.TargetVolumes * s.TargetChapters
}

// ChapterConfig contains chapter writing settings
type ChapterConfig struct {
	TargetWordsPerChapter int `json:"target_words_per_chapter"`
	MinWordsPerChapter    int `json:"min_words_per_chapter"`
	MaxWordsPerChapter    int `json:"max_words_per_chapter"`
}

// DefaultChapterConfig returns default chapter configuration
func DefaultChapterConfig() ChapterConfig {
	return ChapterConfig{
		TargetWordsPerChapter: 3000,
		MinWordsPerChapter:    2000,
		MaxWordsPerChapter:    5000,
	}
}

// ProjectLLM contains only provider and model selection for the project
type ProjectLLM struct {
	Provider string `json:"provider"` // e.g., "ollama", "openai"
	Model    string `json:"model"`    // e.g., "qwen3.5:4b", "gpt-4"
}

// DefaultProjectLLM returns default project LLM selection
func DefaultProjectLLM() ProjectLLM {
	return ProjectLLM{
		Provider: "ollama",
		Model:    "qwen3.5:4b",
	}
}

// Save writes the project config to novel.json
func (p *ProjectConfig) Save(path string) error {
	p.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadProjectConfig reads the project config from novel.json
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// FindProjectRoot searches for novel.json in the current directory and parent directories
func FindProjectRoot(startDir string) (string, error) {
	// For now, just check if novel.json exists in the current directory
	// This can be enhanced to search parent directories
	if _, err := os.Stat(startDir + "/novel.json"); err == nil {
		return startDir, nil
	}
	return "", os.ErrNotExist
}
