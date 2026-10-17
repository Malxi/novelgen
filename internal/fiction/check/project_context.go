// Project loading for the tool plane: ProjectContext loads exactly the artifacts a query section
// needs, and the load* helpers cache the typed setup/outline/craft models for context bundles.

package check

import (
	"fmt"
	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

type ProjectContext struct {
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
func (c ProjectContext) Dir() string { return c.Root.Dir() }

func (c *ProjectContext) Load(kind string) error {
	switch kind {
	case "craft":
		return c.LoadElements()
	case "story-setup", "setup":
		if err := c.LoadSetup(); err != nil {
			return err
		}
		_ = c.LoadOutline()
		return nil
	case "outline":
		if err := c.LoadOutline(); err != nil {
			return err
		}
		return nil
	case "chapter", "chapters":
		if err := c.LoadOutline(); err != nil {
			return err
		}
		_ = c.LoadElements()
		return nil
	case "context":
		if err := c.LoadSetup(); err != nil {
			return err
		}
		if err := c.LoadOutline(); err != nil {
			return err
		}
		return c.LoadElements()
	case "logs", "log":
		return nil
	default:
		return fmt.Errorf("unsupported query section %q", kind)
	}
}

func (c *ProjectContext) LoadSetup() error {
	setup, err := c.Root.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	c.Setup = setup
	return nil
}

func (c *ProjectContext) LoadOutline() error {
	outline, err := c.Root.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	c.Outline = outline
	return nil
}

func (c *ProjectContext) LoadElements() error {
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
