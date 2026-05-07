package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/rpg/dsl"

	"github.com/spf13/cobra"
)

var (
	craftChapterFlag     string
	craftVolumeFlag      string
	craftPartFlag        string
	craftPromptFlag      string
	craftBatchFlag       int
	craftConcurrencyFlag int
	craftMaxRoundsFlag   int
	craftElementTypeFlag string
)

var craftCmd = &cobra.Command{
	Use:   "craft",
	Short: "Generate story world elements",
	Long: `Generate detailed world elements from the outline and story setup.

This command scans the outline and setup to identify all story elements and generates
detailed profiles for each:
  - Characters: appearance, personality, background, motivation, affiliations, RPG/DSL roles and stats
  - Locations: description, atmosphere, sensory details, history, significance, RPG/DSL map metadata
  - Items: appearance, function, origin, powers, limitations, significance, RPG/DSL item metadata
  - Organizations: factions, guilds, empires, companies, sects, and other power groups

Ability systems and progression rules are owned by story/setup/story_setup.json
(premises). Craft may reference them while generating characters, locations,
and items, but it does not generate a separate ability-system catalog.

Generated elements are saved to story/craft/ directory.
Already generated elements are skipped by default (incremental generation).

Subcommands:
  gen     - Generate story elements
  improve - Improve existing elements through AI review`,
}

var craftGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate story elements",
	Long: `Generate story elements (characters, locations, items, organizations) based on outline.

Examples:
  # Generate all elements from outline
  novelgen craft gen

  # Generate elements for specific chapter
  novelgen craft gen --chapter 1

  # Generate elements for specific volume
  novelgen craft gen --volume 1

  # Generate elements for specific part
  novelgen craft gen --part 1

  # Generate with custom prompt adjustment
  novelgen craft gen --chapter 1 --prompt "focus on combat abilities"

  # Generate in small batches
  novelgen craft gen --batch 5

  # Generate with concurrency
  novelgen craft gen --concurrency 3`,
	RunE: runCraftGen,
}

var craftImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve existing elements through AI review",
	Long: `Improve existing story elements by running AI review and enhancement cycles.

This command loads the current elements (characters, locations, items, organizations) and runs
multiple rounds of AI self-review to identify weaknesses and improve the quality,
consistency, and depth of the world building.

Examples:
  # Improve all elements with 1 round
  novelgen craft improve

  # Improve only characters
  novelgen craft improve --type characters

  # Improve only locations
  novelgen craft improve --type locations

  # Improve only items
  novelgen craft improve --type items

  # Improve only organizations
  novelgen craft improve --type organizations

  # Run 3 improvement rounds
  novelgen craft improve --max-rounds 3`,
	RunE: runCraftImprove,
}

func init() {
	craftCmd.AddCommand(craftGenCmd)
	craftCmd.AddCommand(craftImproveCmd)

	craftGenCmd.Flags().StringVar(&craftChapterFlag, "chapter", "", "Generate elements for specific chapter (e.g., '1', 'P1-V1-C1')")
	craftGenCmd.Flags().StringVar(&craftVolumeFlag, "volume", "", "Generate elements for specific volume (e.g., '1', 'P1-V1')")
	craftGenCmd.Flags().StringVar(&craftPartFlag, "part", "", "Generate elements for specific part (e.g., '1', 'P1')")
	craftGenCmd.Flags().StringVar(&craftPromptFlag, "prompt", "", "Additional prompt to guide generation")
	craftGenCmd.Flags().IntVar(&craftBatchFlag, "batch", 1, "Number of elements to generate in one batch")
	craftGenCmd.Flags().IntVar(&craftConcurrencyFlag, "concurrency", 1, "Number of concurrent element generations")

	craftImproveCmd.Flags().StringVar(&craftElementTypeFlag, "type", "all", "Element type to improve (all/characters/locations/items/organizations)")
	craftImproveCmd.Flags().IntVar(&craftMaxRoundsFlag, "max-rounds", 1, "Maximum number of improvement rounds")
	craftImproveCmd.Flags().StringVar(&craftPromptFlag, "prompt", "", "Additional prompt to guide improvement")

	// Register craft command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return craftCmd
	})
}

func runCraftGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Extract elements from outline
	extractor := NewElementExtractor(outline, setup)
	elements := extractor.Extract()
	if unknownAbilityRefs := findUnknownAbilitySystemRefs(outline, setup); len(unknownAbilityRefs) > 0 {
		log.Warn("Outline references ability systems or stages missing from setup.premises: %s. Craft will not generate ability systems; update setup or run setup improve.",
			strings.Join(unknownAbilityRefs, ", "))
	}

	log.Info("Extracted elements from outline: characters=%d, locations=%d, items=%d, organizations=%d",
		len(elements.Characters),
		len(elements.Locations),
		len(elements.Items),
		len(elements.Organizations))

	// Filter elements based on flags
	if craftChapterFlag != "" {
		log.Info("Filtering by chapter: %s", craftChapterFlag)
		elements, err = filterElementsByChapter(elements, craftChapterFlag, outline)
		if err != nil {
			return err
		}
		log.Info("After chapter filter: characters=%d, locations=%d, items=%d, organizations=%d",
			len(elements.Characters), len(elements.Locations), len(elements.Items), len(elements.Organizations))
	} else if craftVolumeFlag != "" {
		log.Info("Filtering by volume: %s", craftVolumeFlag)
		elements, err = filterElementsByVolume(elements, craftVolumeFlag, outline)
		if err != nil {
			return err
		}
	} else if craftPartFlag != "" {
		log.Info("Filtering by part: %s", craftPartFlag)
		elements, err = filterElementsByPart(elements, craftPartFlag, outline)
		if err != nil {
			return err
		}
	}

	// Load already generated elements to skip
	generated := loadGeneratedElements()

	// Filter out already generated elements
	elementsToGenerate := filterUnGenerated(elements, generated)

	log.Info("Elements to generate: characters=%d, locations=%d, items=%d, organizations=%d",
		len(elementsToGenerate.Characters),
		len(elementsToGenerate.Locations),
		len(elementsToGenerate.Items),
		len(elementsToGenerate.Organizations))

	if len(elementsToGenerate.Characters) == 0 &&
		len(elementsToGenerate.Locations) == 0 &&
		len(elementsToGenerate.Items) == 0 &&
		len(elementsToGenerate.Organizations) == 0 {
		log.Info("All elements already generated")
		return nil
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create craft agent
	agent := agents.NewCraftAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	// Generate elements in batches
	batchSize := craftBatchFlag
	if batchSize <= 0 {
		batchSize = 1
	}

	ctx := cmd.Context()

	// Generate characters
	if err := generateCharacters(ctx, agent, elementsToGenerate.Characters, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate characters: %w", err)
	}

	// Generate locations
	if err := generateLocations(ctx, agent, elementsToGenerate.Locations, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate locations: %w", err)
	}

	// Generate items
	if err := generateItems(ctx, agent, elementsToGenerate.Items, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate items: %w", err)
	}

	// Generate organizations
	if err := generateOrganizations(ctx, agent, elementsToGenerate.Organizations, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate organizations: %w", err)
	}

	log.Info("Craft generation completed")
	return nil
}

// ElementExtractor extracts story elements from outline
type ElementExtractor struct {
	outline *models.Outline
	setup   *models.StorySetup
}

// ExtractedElements holds all extracted elements
type ExtractedElements struct {
	Characters    []string
	Locations     []string
	Items         []string
	Organizations []string
}

// GeneratedElements tracks already generated elements
type GeneratedElements struct {
	Characters    map[string]bool
	Locations     map[string]bool
	Items         map[string]bool
	Organizations map[string]bool
}

func NewElementExtractor(outline *models.Outline, setup *models.StorySetup) *ElementExtractor {
	return &ElementExtractor{outline: outline, setup: setup}
}

func (e *ElementExtractor) Extract() *ExtractedElements {
	result := &ExtractedElements{
		Characters:    make([]string, 0),
		Locations:     make([]string, 0),
		Items:         make([]string, 0),
		Organizations: make([]string, 0),
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)
	orgMap := make(map[string]bool)

	// Extract from outline chapters
	for _, part := range e.outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, char := range chapter.Characters {
					addExtractedName(char, charMap, &result.Characters)
				}
				for _, ally := range chapter.StateAnchor.Allies {
					addExtractedName(ally, charMap, &result.Characters)
				}
				for _, enemy := range chapter.Enemies {
					addExtractedName(enemy.Name, charMap, &result.Characters)
					addExtractedName(enemy.Faction, orgMap, &result.Organizations)
				}

				addExtractedName(chapter.Location, locMap, &result.Locations)
				addExtractedName(chapter.StateAnchor.Location, locMap, &result.Locations)

				for _, item := range chapter.StateAnchor.KeyItems {
					addExtractedName(item, itemMap, &result.Items)
				}
				for _, entry := range chapter.ResourceLedger {
					addExtractedName(entry.Item, itemMap, &result.Items)
				}
				for _, scene := range chapter.Scenes {
					addExtractedName(scene.POV, charMap, &result.Characters)
					for _, char := range scene.Characters {
						addExtractedName(char, charMap, &result.Characters)
					}
					addExtractedName(scene.Location, locMap, &result.Locations)
				}
				for _, event := range chapter.Events {
					addExtractedName(event.GetActor(), charMap, &result.Characters)
					switch event.GetTargetType() {
					case models.TargetTypeItem:
						addExtractedName(event.GetTarget(), itemMap, &result.Items)
					case models.TargetTypeLocation:
						addExtractedName(event.GetTarget(), locMap, &result.Locations)
					case models.TargetTypeCharacter:
						addExtractedName(event.GetTarget(), charMap, &result.Characters)
					}
					if event.Type == models.EventTypeItem {
						addExtractedName(event.GetTarget(), itemMap, &result.Items)
					}
					if isItemAction(event.GetAction()) && event.GetTarget() != "" && event.GetTargetType() == "" {
						addExtractedName(event.GetTarget(), itemMap, &result.Items)
					}
					if event.Context != "" {
						addExtractedName(event.Context, locMap, &result.Locations)
					}
				}
			}
		}
	}

	if e.setup != nil {
		// Extract from storylines (potential organizations or lore)
		for _, storyline := range e.setup.Storylines {
			if storyline.Name != "" && storylineLooksLikeOrganization(storyline) {
				if !orgMap[storyline.Name] && len(result.Organizations) < 10 {
					orgMap[storyline.Name] = true
					result.Organizations = append(result.Organizations, storyline.Name)
				}
			}
		}
	}

	return result
}

func storylineLooksLikeOrganization(storyline models.Storyline) bool {
	text := strings.ToLower(strings.Join([]string{
		storyline.Name,
		storyline.Type,
		storyline.SetupRole,
		storyline.Desire,
		storyline.Opposition,
		storyline.Stakes,
	}, " "))
	for _, marker := range []string{
		"faction", "organization", "guild", "sect", "empire", "kingdom", "company", "corporation", "army", "fleet", "alliance",
		"势力", "阵营", "组织", "宗门", "门派", "公会", "帝国", "王国", "公司", "集团", "军团", "舰队", "联盟",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func findUnknownAbilitySystemRefs(outline *models.Outline, setup *models.StorySetup) []string {
	if outline == nil {
		return nil
	}

	known := make(map[string]bool)
	if setup != nil {
		for _, premise := range setup.Premises {
			addKnownAbilityRef(premise.Name, known)
			for _, stage := range premise.Progression {
				addKnownAbilityRef(stage.Name, known)
			}
		}
	}

	seen := make(map[string]bool)
	var unknown []string
	addUnknown := func(ref string) {
		ref = cleanExtractedName(ref)
		if ref == "" || known[strings.ToLower(ref)] || seen[ref] {
			return
		}
		seen[ref] = true
		unknown = append(unknown, ref)
	}

	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, event := range chapter.Events {
					if event.GetTargetType() == models.TargetTypePremise || event.Type == models.EventTypePremise {
						addUnknown(event.GetTarget())
					}
				}
			}
		}
	}

	return unknown
}

func addKnownAbilityRef(ref string, known map[string]bool) {
	ref = cleanExtractedName(ref)
	if ref != "" {
		known[strings.ToLower(ref)] = true
	}
}

func addExtractedName(name string, seen map[string]bool, out *[]string) {
	name = cleanExtractedName(name)
	if name == "" || seen[name] {
		return
	}
	seen[name] = true
	*out = append(*out, name)
}

func cleanExtractedName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, sep := range []string{"：", ":", "\n", " - ", " -- "} {
		if idx := strings.Index(name, sep); idx > 0 {
			name = strings.TrimSpace(name[:idx])
			break
		}
	}
	return name
}

func isItemAction(action string) bool {
	switch action {
	case models.ActionAcquire, models.ActionUse, models.ActionLose, models.ActionCraft:
		return true
	default:
		return false
	}
}

func filterElementsByChapter(elements *ExtractedElements, chapterID string, outline *models.Outline) (*ExtractedElements, error) {
	result := &ExtractedElements{
		Characters:    make([]string, 0),
		Locations:     make([]string, 0),
		Items:         make([]string, 0),
		Organizations: make([]string, 0),
	}

	resolvedChapterID, err := resolveCraftChapterID(outline, chapterID)
	if err != nil {
		return result, err
	}

	chapter := outline.GetChapterByID(resolvedChapterID)
	if chapter == nil {
		return result, fmt.Errorf("chapter %s not found", resolvedChapterID)
	}

	charMap, locMap, itemMap, orgMap := collectChapterElementSets(*chapter)
	filterElementSet(elements, result, charMap, locMap, itemMap, orgMap)

	return result, nil
}

func filterElementsByVolume(elements *ExtractedElements, volumeID string, outline *models.Outline) (*ExtractedElements, error) {
	result := &ExtractedElements{
		Characters:    make([]string, 0),
		Locations:     make([]string, 0),
		Items:         make([]string, 0),
		Organizations: make([]string, 0),
	}

	resolvedVolumeID, err := resolveCraftVolumeID(outline, volumeID)
	if err != nil {
		return result, err
	}

	volume := outline.GetVolumeByID(resolvedVolumeID)
	if volume == nil {
		return result, fmt.Errorf("volume %s not found", resolvedVolumeID)
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)
	orgMap := make(map[string]bool)

	for _, chapter := range volume.Chapters {
		chChars, chLocs, chItems, chOrgs := collectChapterElementSets(chapter)
		mergeBoolMap(charMap, chChars)
		mergeBoolMap(locMap, chLocs)
		mergeBoolMap(itemMap, chItems)
		mergeBoolMap(orgMap, chOrgs)
	}

	filterElementSet(elements, result, charMap, locMap, itemMap, orgMap)

	return result, nil
}

func filterElementsByPart(elements *ExtractedElements, partID string, outline *models.Outline) (*ExtractedElements, error) {
	result := &ExtractedElements{
		Characters:    make([]string, 0),
		Locations:     make([]string, 0),
		Items:         make([]string, 0),
		Organizations: make([]string, 0),
	}

	resolvedPartID, err := resolveCraftPartID(outline, partID)
	if err != nil {
		return result, err
	}

	part := outline.GetPartByID(resolvedPartID)
	if part == nil {
		return result, fmt.Errorf("part %s not found", resolvedPartID)
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)
	orgMap := make(map[string]bool)

	for _, volume := range part.Volumes {
		for _, chapter := range volume.Chapters {
			chChars, chLocs, chItems, chOrgs := collectChapterElementSets(chapter)
			mergeBoolMap(charMap, chChars)
			mergeBoolMap(locMap, chLocs)
			mergeBoolMap(itemMap, chItems)
			mergeBoolMap(orgMap, chOrgs)
		}
	}

	filterElementSet(elements, result, charMap, locMap, itemMap, orgMap)

	return result, nil
}

func collectChapterElementSets(chapter models.Chapter) (map[string]bool, map[string]bool, map[string]bool, map[string]bool) {
	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)
	orgMap := make(map[string]bool)
	addToSet := func(value string, target map[string]bool) {
		value = cleanExtractedName(value)
		if value != "" {
			target[value] = true
		}
	}
	for _, char := range chapter.Characters {
		addToSet(char, charMap)
	}
	for _, ally := range chapter.StateAnchor.Allies {
		addToSet(ally, charMap)
	}
	for _, enemy := range chapter.Enemies {
		addToSet(enemy.Name, charMap)
		addToSet(enemy.Faction, orgMap)
	}
	addToSet(chapter.Location, locMap)
	addToSet(chapter.StateAnchor.Location, locMap)
	for _, item := range chapter.StateAnchor.KeyItems {
		addToSet(item, itemMap)
	}
	for _, entry := range chapter.ResourceLedger {
		addToSet(entry.Item, itemMap)
	}
	for _, scene := range chapter.Scenes {
		addToSet(scene.POV, charMap)
		for _, char := range scene.Characters {
			addToSet(char, charMap)
		}
		addToSet(scene.Location, locMap)
	}
	for _, event := range chapter.Events {
		addToSet(event.GetActor(), charMap)
		switch event.GetTargetType() {
		case models.TargetTypeItem:
			addToSet(event.GetTarget(), itemMap)
		case models.TargetTypeLocation:
			addToSet(event.GetTarget(), locMap)
		case models.TargetTypeCharacter:
			addToSet(event.GetTarget(), charMap)
		}
		if event.Type == models.EventTypeItem || isItemAction(event.GetAction()) {
			addToSet(event.GetTarget(), itemMap)
		}
		addToSet(event.Context, locMap)
	}
	return charMap, locMap, itemMap, orgMap
}

func resolveCraftChapterID(outline *models.Outline, chapterInput string) (string, error) {
	if outline == nil {
		return "", fmt.Errorf("outline is nil")
	}
	manager := logic.NewIDManager(outline)
	return manager.ResolveChapterID(chapterInput, craftPartFlag, craftVolumeFlag)
}

func resolveCraftVolumeID(outline *models.Outline, volumeInput string) (string, error) {
	if outline == nil {
		return "", fmt.Errorf("outline is nil")
	}
	manager := logic.NewIDManager(outline)
	return manager.ResolveVolumeID(volumeInput, craftPartFlag)
}

func resolveCraftPartID(outline *models.Outline, partInput string) (string, error) {
	if outline == nil {
		return "", fmt.Errorf("outline is nil")
	}
	manager := logic.NewIDManager(outline)
	return manager.ResolvePartID(partInput)
}

func filterElementSet(elements, result *ExtractedElements, charMap, locMap, itemMap, orgMap map[string]bool) {
	for _, char := range elements.Characters {
		if charMap[cleanExtractedName(char)] {
			result.Characters = append(result.Characters, char)
		}
	}
	for _, loc := range elements.Locations {
		if locMap[cleanExtractedName(loc)] {
			result.Locations = append(result.Locations, loc)
		}
	}
	for _, item := range elements.Items {
		if itemMap[cleanExtractedName(item)] {
			result.Items = append(result.Items, item)
		}
	}
	for _, org := range elements.Organizations {
		if orgMap[cleanExtractedName(org)] {
			result.Organizations = append(result.Organizations, org)
		}
	}
}

func mergeBoolMap(dst, src map[string]bool) {
	for key := range src {
		dst[key] = true
	}
}

func loadGeneratedElements() *GeneratedElements {
	result := &GeneratedElements{
		Characters:    make(map[string]bool),
		Locations:     make(map[string]bool),
		Items:         make(map[string]bool),
		Organizations: make(map[string]bool),
	}

	root, err := findProjectRoot()
	if err != nil {
		return result
	}

	// Load characters
	charPath := filepath.Join(root, "story", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		var chars map[string]interface{}
		if err := json.Unmarshal(data, &chars); err == nil {
			for name := range chars {
				result.Characters[name] = true
			}
		}
	}

	// Load locations
	locPath := filepath.Join(root, "story", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		var locs map[string]interface{}
		if err := json.Unmarshal(data, &locs); err == nil {
			for name := range locs {
				result.Locations[name] = true
			}
		}
	}

	// Load items
	itemPath := filepath.Join(root, "story", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		var items map[string]interface{}
		if err := json.Unmarshal(data, &items); err == nil {
			for name := range items {
				result.Items[name] = true
			}
		}
	}

	// Load organizations
	orgPath := filepath.Join(root, "story", "craft", "organizations.json")
	if data, err := os.ReadFile(orgPath); err == nil {
		var orgs map[string]interface{}
		if err := json.Unmarshal(data, &orgs); err == nil {
			for name := range orgs {
				result.Organizations[name] = true
			}
		}
	}

	return result
}

func filterUnGenerated(elements *ExtractedElements, generated *GeneratedElements) *ExtractedElements {
	result := &ExtractedElements{
		Characters:    make([]string, 0),
		Locations:     make([]string, 0),
		Items:         make([]string, 0),
		Organizations: make([]string, 0),
	}

	for _, char := range elements.Characters {
		if !generated.Characters[char] {
			result.Characters = append(result.Characters, char)
		}
	}

	for _, loc := range elements.Locations {
		if !generated.Locations[loc] {
			result.Locations = append(result.Locations, loc)
		}
	}

	for _, item := range elements.Items {
		if !generated.Items[item] {
			result.Items = append(result.Items, item)
		}
	}

	for _, org := range elements.Organizations {
		if !generated.Organizations[org] {
			result.Organizations = append(result.Organizations, org)
		}
	}

	return result
}

func generateCharacters(ctx context.Context, agent *agents.CraftAgent, characters []string, generated *GeneratedElements, batchSize int) error {
	if len(characters) == 0 {
		return nil
	}

	log := logger.GetLogger()
	log.Info("Generating %d characters with concurrency %d, batch size %d", len(characters), craftConcurrencyFlag, batchSize)

	// Use worker pool for concurrent generation
	concurrency := craftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}

	// Create batches
	batches := make([][]string, 0)
	for i := 0; i < len(characters); i += batchSize {
		end := i + batchSize
		if end > len(characters) {
			end = len(characters)
		}
		batches = append(batches, characters[i:end])
	}

	if concurrency > len(batches) {
		concurrency = len(batches)
	}

	// Create work channel and wait group
	batchChan := make(chan []string, len(batches))
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]models.Character)
	var errs []error

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating characters batch: count=%d", workerID, len(batch))

				batchResults, err := agent.GenerateCharacters(ctx, batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate characters batch: %v", workerID, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("characters batch %v: %w", batch, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				for name, char := range batchResults {
					results[name] = char
				}
				mu.Unlock()

				log.Info("[Worker %d] Generated %d characters", workerID, len(batchResults))
			}
		}(i)
	}

	// Send batches to workers
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)

	// Wait for all workers to complete
	wg.Wait()

	for _, name := range missingGeneratedNames(characters, results) {
		errs = append(errs, fmt.Errorf("character %q was not returned by the LLM", name))
	}
	if len(results) > 0 {
		if err := saveCharacters(results); err != nil {
			errs = append(errs, fmt.Errorf("save characters: %w", err))
		} else {
			for name := range results {
				generated.Characters[name] = true
			}
			log.Info("Saved %d characters", len(results))
		}
	}
	return errors.Join(errs...)
}

func generateLocations(ctx context.Context, agent *agents.CraftAgent, locations []string, generated *GeneratedElements, batchSize int) error {
	if len(locations) == 0 {
		return nil
	}

	log := logger.GetLogger()
	log.Info("Generating %d locations with concurrency %d, batch size %d", len(locations), craftConcurrencyFlag, batchSize)

	// Use worker pool for concurrent generation
	concurrency := craftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}

	// Create batches
	batches := make([][]string, 0)
	for i := 0; i < len(locations); i += batchSize {
		end := i + batchSize
		if end > len(locations) {
			end = len(locations)
		}
		batches = append(batches, locations[i:end])
	}

	if concurrency > len(batches) {
		concurrency = len(batches)
	}

	// Create work channel and wait group
	batchChan := make(chan []string, len(batches))
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]models.Location)
	var errs []error

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating locations batch: count=%d", workerID, len(batch))

				batchResults, err := agent.GenerateLocations(ctx, batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate locations batch: %v", workerID, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("locations batch %v: %w", batch, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				for name, loc := range batchResults {
					results[name] = loc
				}
				mu.Unlock()

				log.Info("[Worker %d] Generated %d locations", workerID, len(batchResults))
			}
		}(i)
	}

	// Send batches to workers
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)

	// Wait for all workers to complete
	wg.Wait()

	for _, name := range missingGeneratedNames(locations, results) {
		errs = append(errs, fmt.Errorf("location %q was not returned by the LLM", name))
	}
	if len(results) > 0 {
		if err := saveLocations(results); err != nil {
			errs = append(errs, fmt.Errorf("save locations: %w", err))
		} else {
			for name := range results {
				generated.Locations[name] = true
			}
			log.Info("Saved %d locations", len(results))
		}
	}
	return errors.Join(errs...)
}

func generateItems(ctx context.Context, agent *agents.CraftAgent, items []string, generated *GeneratedElements, batchSize int) error {
	if len(items) == 0 {
		return nil
	}

	log := logger.GetLogger()
	log.Info("Generating %d items with concurrency %d, batch size %d", len(items), craftConcurrencyFlag, batchSize)

	// Use worker pool for concurrent generation
	concurrency := craftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}

	// Create batches
	batches := make([][]string, 0)
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	if concurrency > len(batches) {
		concurrency = len(batches)
	}

	// Create work channel and wait group
	batchChan := make(chan []string, len(batches))
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]models.Item)
	var errs []error

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating items batch: count=%d", workerID, len(batch))

				batchResults, err := agent.GenerateItems(ctx, batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate items batch: %v", workerID, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("items batch %v: %w", batch, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				for name, item := range batchResults {
					results[name] = item
				}
				mu.Unlock()

				log.Info("[Worker %d] Generated %d items", workerID, len(batchResults))
			}
		}(i)
	}

	// Send batches to workers
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)

	// Wait for all workers to complete
	wg.Wait()

	for _, name := range missingGeneratedNames(items, results) {
		errs = append(errs, fmt.Errorf("item %q was not returned by the LLM", name))
	}
	if len(results) > 0 {
		if err := saveItems(results); err != nil {
			errs = append(errs, fmt.Errorf("save items: %w", err))
		} else {
			for name := range results {
				generated.Items[name] = true
			}
			log.Info("Saved %d items", len(results))
		}
	}
	return errors.Join(errs...)
}

func generateOrganizations(ctx context.Context, agent *agents.CraftAgent, organizations []string, generated *GeneratedElements, batchSize int) error {
	if len(organizations) == 0 {
		return nil
	}

	log := logger.GetLogger()
	log.Info("Generating %d organizations with concurrency %d, batch size %d", len(organizations), craftConcurrencyFlag, batchSize)

	concurrency := craftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}

	batches := make([][]string, 0)
	for i := 0; i < len(organizations); i += batchSize {
		end := i + batchSize
		if end > len(organizations) {
			end = len(organizations)
		}
		batches = append(batches, organizations[i:end])
	}
	if concurrency > len(batches) {
		concurrency = len(batches)
	}

	batchChan := make(chan []string, len(batches))
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]models.Organization)
	var errs []error

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating organizations batch: count=%d", workerID, len(batch))

				batchResults, err := agent.GenerateOrganizations(ctx, batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate organizations batch: %v", workerID, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("organizations batch %v: %w", batch, err))
					mu.Unlock()
					continue
				}

				mu.Lock()
				for name, org := range batchResults {
					results[name] = org
				}
				mu.Unlock()

				log.Info("[Worker %d] Generated %d organizations", workerID, len(batchResults))
			}
		}(i)
	}

	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)
	wg.Wait()

	for _, name := range missingGeneratedNames(organizations, results) {
		errs = append(errs, fmt.Errorf("organization %q was not returned by the LLM", name))
	}
	if len(results) > 0 {
		if err := saveOrganizations(results); err != nil {
			errs = append(errs, fmt.Errorf("save organizations: %w", err))
		} else {
			for name := range results {
				generated.Organizations[name] = true
			}
			log.Info("Saved %d organizations", len(results))
		}
	}
	return errors.Join(errs...)
}

func missingGeneratedNames[T any](requested []string, generated map[string]T) []string {
	var missing []string
	for _, name := range requested {
		if _, ok := generated[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func saveCharacters(characters map[string]models.Character) error {
	for name, char := range characters {
		char.NormalizeForCraft(name)
		characters[name] = char
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "characters.json")
	return saveJSON(path, characters)
}

func saveLocations(locations map[string]models.Location) error {
	for name, loc := range locations {
		loc.NormalizeForCraft(name)
		locations[name] = loc
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "locations.json")
	return saveJSON(path, locations)
}

func saveItems(items map[string]models.Item) error {
	for name, item := range items {
		item.NormalizeForCraft(name)
		items[name] = item
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "items.json")
	return saveJSON(path, items)
}

func saveOrganizations(organizations map[string]models.Organization) error {
	for name, org := range organizations {
		org.NormalizeForCraft(name)
		organizations[name] = org
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "organizations.json")
	return saveJSON(path, organizations)
}

func saveJSON(path string, data interface{}) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Read existing data if file exists
	existing := make(map[string]interface{})
	if fileData, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(fileData, &existing); err != nil {
			return fmt.Errorf("failed to parse existing data: %w", err)
		}
	}

	// Merge new data
	newData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal new data: %w", err)
	}
	var newMap map[string]interface{}
	if err := json.Unmarshal(newData, &newMap); err != nil {
		return fmt.Errorf("failed to parse new data: %w", err)
	}

	for k, v := range newMap {
		existing[k] = v
	}

	// Save merged data
	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return os.WriteFile(path, output, 0644)
}

func runCraftImprove(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load existing elements
	charModels, locModels, itemModels, orgModels, err := loadAllElements()
	if err != nil {
		return fmt.Errorf("failed to load elements: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create iteration agent
	agent := agents.NewCraftIterationAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	threshold := 80.0
	maxRounds := craftMaxRoundsFlag
	if maxRounds <= 0 {
		maxRounds = 1
	}

	// Determine type to improve
	elemType := craftElementTypeFlag
	if elemType == "" {
		elemType = "all"
	}
	if elemType != "all" && elemType != "characters" && elemType != "locations" && elemType != "items" && elemType != "organizations" {
		return fmt.Errorf("unknown craft element type %q (expected all/characters/locations/items/organizations)", elemType)
	}

	// DSL simulation bridge for enrichment
	bridge := dsl.NewSimulationBridge()

	// Improve characters
	if elemType == "all" || elemType == "characters" {
		if len(charModels) == 0 {
			log.Info("No characters to improve")
		} else {
			chars := make(map[string]models.Character)
			for k, v := range charModels {
				chars[k] = *v
			}

			log.Info("Improving %d characters (max %d rounds)...", len(chars), maxRounds)
			improved, review, iterErr := agent.IterateCharacters(ctx, chars, maxRounds, threshold, craftPromptFlag)
			if iterErr != nil {
				log.Error("Character improvement failed: %v", iterErr)
			} else {
				// Enrich with DSL simulation
				if review != nil {
					enrichReviewWithDSL(log, bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft, review)
					// Extra improve if critical DSL issues found
					if hasCriticalDSLIssues(bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft) {
						improved, _, _ = agent.IterateCharacters(ctx, improved, 1, threshold, craftPromptFlag)
						log.Info("Extra character improve pass for DSL-critical issues")
					}
				}
				if saveErr := saveCharacters(improved); saveErr != nil {
					log.Error("Failed to save characters: %v", saveErr)
				} else {
					log.Info("✓ Improved characters saved")
				}
			}
		}
	}

	// Improve locations
	if elemType == "all" || elemType == "locations" {
		if len(locModels) == 0 {
			log.Info("No locations to improve")
		} else {
			locs := make(map[string]models.Location)
			for k, v := range locModels {
				locs[k] = *v
			}

			log.Info("Improving %d locations (max %d rounds)...", len(locs), maxRounds)
			improved, review, iterErr := agent.IterateLocations(ctx, locs, maxRounds, threshold, craftPromptFlag)
			if iterErr != nil {
				log.Error("Location improvement failed: %v", iterErr)
			} else {
				if review != nil {
					enrichReviewWithDSL(log, bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft, review)
					if hasCriticalDSLIssues(bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft) {
						improved, _, _ = agent.IterateLocations(ctx, improved, 1, threshold, craftPromptFlag)
						log.Info("Extra location improve pass for DSL-critical issues")
					}
				}
				if saveErr := saveLocations(improved); saveErr != nil {
					log.Error("Failed to save locations: %v", saveErr)
				} else {
					log.Info("✓ Improved locations saved")
				}
			}
		}
	}

	// Improve items
	if elemType == "all" || elemType == "items" {
		if len(itemModels) == 0 {
			log.Info("No items to improve")
		} else {
			items := make(map[string]models.Item)
			for k, v := range itemModels {
				items[k] = *v
			}

			log.Info("Improving %d items (max %d rounds)...", len(items), maxRounds)
			improved, review, iterErr := agent.IterateItems(ctx, items, maxRounds, threshold, craftPromptFlag)
			if iterErr != nil {
				log.Error("Item improvement failed: %v", iterErr)
			} else {
				if review != nil {
					enrichReviewWithDSL(log, bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft, review)
					if hasCriticalDSLIssues(bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft) {
						improved, _, _ = agent.IterateItems(ctx, improved, 1, threshold, craftPromptFlag)
						log.Info("Extra item improve pass for DSL-critical issues")
					}
				}
				if saveErr := saveItems(improved); saveErr != nil {
					log.Error("Failed to save items: %v", saveErr)
				} else {
					log.Info("✓ Improved items saved")
				}
			}
		}
	}

	// Improve organizations
	if elemType == "all" || elemType == "organizations" {
		if len(orgModels) == 0 {
			log.Info("No organizations to improve")
		} else {
			orgs := make(map[string]models.Organization)
			for k, v := range orgModels {
				orgs[k] = *v
			}

			log.Info("Improving %d organizations (max %d rounds)...", len(orgs), maxRounds)
			improved, review, iterErr := agent.IterateOrganizations(ctx, orgs, maxRounds, threshold, craftPromptFlag)
			if iterErr != nil {
				log.Error("Organization improvement failed: %v", iterErr)
			} else {
				if review != nil {
					enrichReviewWithDSL(log, bridge, setup, outline, charModels, locModels, itemModels, orgModels, dsl.PhaseCraft, review)
				}
				if saveErr := saveOrganizations(improved); saveErr != nil {
					log.Error("Failed to save organizations: %v", saveErr)
				} else {
					log.Info("鉁?Improved organizations saved")
				}
			}
		}
	}

	return nil
}

// enrichReviewWithDSL runs DSL simulation and merges issues into the review result.
func enrichReviewWithDSL(log logger.LoggerInterface, bridge *dsl.SimulationBridge, setup *models.StorySetup, outline *models.Outline, characters map[string]*models.Character, locations map[string]*models.Location, items map[string]*models.Item, organizations map[string]*models.Organization, phase dsl.MergePhase, review *models.ReviewResult) {
	adapter := dsl.NewModelAdapterWithOrganizations(setup, outline, characters, locations, items, organizations)
	dslIssues, err := adapter.Simulate(phase)
	if err != nil {
		log.Debug("DSL simulation skipped: %v", err)
		return
	}
	if len(dslIssues) == 0 {
		return
	}
	bridge.MergeIntoReview(dslIssues, review)
	log.Info("DSL simulation enriched review with %d additional issues", len(dslIssues))
}

// hasCriticalDSLIssues returns true if DSL simulation finds critical issues.
func hasCriticalDSLIssues(bridge *dsl.SimulationBridge, setup *models.StorySetup, outline *models.Outline, characters map[string]*models.Character, locations map[string]*models.Location, items map[string]*models.Item, organizations map[string]*models.Organization, phase dsl.MergePhase) bool {
	adapter := dsl.NewModelAdapterWithOrganizations(setup, outline, characters, locations, items, organizations)
	dslIssues, err := adapter.Simulate(phase)
	if err != nil {
		return false
	}
	for _, iss := range dslIssues {
		if iss.Severity == dsl.SeverityCritical {
			return true
		}
	}
	return false
}

// findProjectRoot finds the project root directory by looking for novel.json
func findProjectRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		// Check if novel.json exists in this directory
		configPath := filepath.Join(dir, "novel.json")
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found (novel.json not found in current or parent directories)")
}

func loadProjectConfig() (*models.ProjectConfig, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(root, "novel.json")
	return models.LoadProjectConfig(configPath)
}

func loadStorySetup() (*models.StorySetup, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "story", "setup", "story_setup.json")
	return models.LoadStorySetup(path)
}

func loadOutline() (*models.Outline, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "story", "compose", "outline.json")
	return models.LoadOutline(path)
}

func loadAllElements() (map[string]*models.Character, map[string]*models.Location, map[string]*models.Item, map[string]*models.Organization, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	characters := make(map[string]*models.Character)
	locations := make(map[string]*models.Location)
	items := make(map[string]*models.Item)
	organizations := make(map[string]*models.Organization)

	// Load characters
	charPath := filepath.Join(root, "story", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		if err := json.Unmarshal(data, &characters); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to parse characters: %w", err)
		}
	}

	// Load locations
	locPath := filepath.Join(root, "story", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		if err := json.Unmarshal(data, &locations); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to parse locations: %w", err)
		}
	}

	// Load items
	itemPath := filepath.Join(root, "story", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to parse items: %w", err)
		}
	}

	orgPath := filepath.Join(root, "story", "craft", "organizations.json")
	if data, err := os.ReadFile(orgPath); err == nil {
		if err := json.Unmarshal(data, &organizations); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to parse organizations: %w", err)
		}
	}

	return characters, locations, items, organizations, nil
}
