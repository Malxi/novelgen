package cmd

import (
	"novelgen/internal/models"
	"novelgen/internal/project"
)

// Thin wrappers over internal/project. They exist so command files that have
// not been moved into a subpackage yet can keep using the historical short
// names. Extracted subpackages call internal/project directly.

func findProjectRoot() (string, error) {
	return project.FindRoot()
}

func loadProjectConfig() (*models.ProjectConfig, error) {
	return project.LoadConfig()
}

func loadStorySetup() (*models.StorySetup, error) {
	return project.LoadStorySetup()
}

func loadOutline() (*models.Outline, error) {
	return project.LoadOutline()
}

func loadAllElements() (map[string]*models.Character, map[string]*models.Location, map[string]*models.Item, map[string]*models.Organization, error) {
	return project.LoadAllElements()
}
