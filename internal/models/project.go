package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// FindProjectRoot searches for novel.json in startDir and its parent
// directories, returning the first directory that contains it.
func FindProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "novel.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// Validation errors
var (
	ErrInvalidProjectName  = errors.New("project name cannot be empty")
	ErrInvalidStructure    = errors.New("story structure must have positive values for parts, volumes, and chapters")
	ErrInvalidChapterWords = errors.New("chapter word counts must be positive and min <= target <= max")
	ErrInvalidLLMConfig    = errors.New("LLM provider and model cannot be empty")
	ErrInvalidLanguage     = errors.New("language cannot be empty")
)

// Validate checks if the project configuration is valid
func (p *ProjectConfig) Validate() error {
	if p.Name == "" {
		return ErrInvalidProjectName
	}

	if p.Language == "" {
		return ErrInvalidLanguage
	}

	if err := p.Structure.Validate(); err != nil {
		return err
	}

	if err := p.ChapterConfig.Validate(); err != nil {
		return err
	}

	if err := p.LLM.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks if the story structure is valid
func (s *StoryStructure) Validate() error {
	if s.TargetParts <= 0 {
		return fmt.Errorf("%w: target_parts must be positive, got %d", ErrInvalidStructure, s.TargetParts)
	}
	if s.TargetVolumes <= 0 {
		return fmt.Errorf("%w: target_volumes must be positive, got %d", ErrInvalidStructure, s.TargetVolumes)
	}
	if s.TargetChapters <= 0 {
		return fmt.Errorf("%w: target_chapters must be positive, got %d", ErrInvalidStructure, s.TargetChapters)
	}
	return nil
}

// Validate checks if the chapter configuration is valid
func (c *ChapterConfig) Validate() error {
	if c.TargetWordsPerChapter <= 0 {
		return fmt.Errorf("%w: target_words_per_chapter must be positive", ErrInvalidChapterWords)
	}
	if c.MinWordsPerChapter <= 0 {
		return fmt.Errorf("%w: min_words_per_chapter must be positive", ErrInvalidChapterWords)
	}
	if c.MaxWordsPerChapter <= 0 {
		return fmt.Errorf("%w: max_words_per_chapter must be positive", ErrInvalidChapterWords)
	}
	if c.MinWordsPerChapter > c.TargetWordsPerChapter {
		return fmt.Errorf("%w: min_words_per_chapter (%d) cannot exceed target_words_per_chapter (%d)",
			ErrInvalidChapterWords, c.MinWordsPerChapter, c.TargetWordsPerChapter)
	}
	if c.TargetWordsPerChapter > c.MaxWordsPerChapter {
		return fmt.Errorf("%w: target_words_per_chapter (%d) cannot exceed max_words_per_chapter (%d)",
			ErrInvalidChapterWords, c.TargetWordsPerChapter, c.MaxWordsPerChapter)
	}
	return nil
}

// Validate checks if the LLM configuration is valid
func (l *ProjectLLM) Validate() error {
	if l.Provider == "" {
		return fmt.Errorf("%w: provider cannot be empty", ErrInvalidLLMConfig)
	}
	if l.Model == "" {
		return fmt.Errorf("%w: model cannot be empty", ErrInvalidLLMConfig)
	}
	return nil
}
