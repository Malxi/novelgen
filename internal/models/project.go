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
}

// Save writes the project config to novel.json
func (p *ProjectConfig) Save(path string) error {
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
