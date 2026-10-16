// Project loading for the tool plane: ToolProjectContext loads exactly the artifacts a query section
// needs, and the load* helpers cache the typed setup/outline/craft models for context bundles.

package toolplane

import (
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/store"
)

type ToolProjectContext struct {
	Root          *store.Root
	Setup         *models.StorySetup
	Outline       *models.Outline
	Characters    map[string]*models.Character
	Locations     map[string]*models.Location
	Items         map[string]*models.Item
	Organizations map[string]*models.Organization
	RawCharacters map[string]map[string]interface{}
	RawLocations  map[string]map[string]interface{}
	RawItems      map[string]map[string]interface{}
	RawOrgs       map[string]map[string]interface{}
}

// dir returns the project directory this context is scoped to, or "" when the
// context carries no root. Tests also build a ToolProjectContext with artifacts
// pre-loaded and no root, so every use of Root goes through here or through a
// nil-safe *store.Root path method.
func (c ToolProjectContext) dir() string { return c.Root.Dir() }

func (c *ToolProjectContext) load(kind string) error {
	switch kind {
	case "craft":
		return c.loadElements()
	case "story-setup", "setup":
		if err := c.loadSetup(); err != nil {
			return err
		}
		_ = c.loadOutline()
		return nil
	case "outline":
		if err := c.loadOutline(); err != nil {
			return err
		}
		return nil
	case "chapter", "chapters":
		if err := c.loadOutline(); err != nil {
			return err
		}
		_ = c.loadElements()
		return nil
	case "context":
		if err := c.loadSetup(); err != nil {
			return err
		}
		if err := c.loadOutline(); err != nil {
			return err
		}
		return c.loadElements()
	case "logs", "log":
		return nil
	default:
		return fmt.Errorf("unsupported query section %q", kind)
	}
}

func (c *ToolProjectContext) loadSetup() error {
	setup, err := c.Root.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	c.Setup = setup
	return nil
}

func (c *ToolProjectContext) loadOutline() error {
	outline, err := c.Root.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	c.Outline = outline
	return nil
}

func (c *ToolProjectContext) loadElements() error {
	characters, locations, items, organizations, err := c.Root.LoadElements()
	if err != nil {
		return err
	}
	c.Characters = characters
	c.Locations = locations
	c.Items = items
	c.Organizations = organizations
	raw := map[store.CraftKind]*map[string]map[string]interface{}{
		store.CraftCharacters:    &c.RawCharacters,
		store.CraftLocations:     &c.RawLocations,
		store.CraftItems:         &c.RawItems,
		store.CraftOrganizations: &c.RawOrgs,
	}
	for kind, target := range raw {
		values, err := c.Root.LoadCraftRaw(kind)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", kind.FileName(), err)
		}
		*target = values
	}
	return nil
}
