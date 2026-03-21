package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	craftChapterFlag      string
	craftVolumeFlag       string
	craftPartFlag         string
	craftPromptFlag       string
	craftBatchFlag        int
	craftConcurrencyFlag  int
	craftMaxRoundsFlag    int
	craftElementTypeFlag  string
	craftStartChaptersFlg int
)

var craftCmd = &cobra.Command{
	Use:   "craft",
	Short: "Generate story world elements",
	Long: `Generate detailed world elements from the outline and story setup.

This command scans the outline and setup to identify all story elements and generates
detailed profiles for each:
  - Characters: appearance, personality, background, motivation, goals, relationships, affiliations
  - Locations: description, atmosphere, sensory details, history, significance
  - Items: appearance, function, origin, powers, limitations, significance
  - Organizations: factions, guilds, empires with goals, structure, relationships
  - Races: species with biology, culture, society, abilities
  - Ability Systems: magic, cultivation, technology systems with mechanics
  - World Lore: history, culture, myths, rules that shape the world

Generated elements are saved to story/craft/ directory.
Already generated elements are skipped by default (incremental generation).

Subcommands:
  gen     - Generate story elements
  improve - Improve existing elements through AI review`,
}

var craftGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate story elements",
	Long: `Generate story elements (characters, locations, items) based on outline.

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

This command loads the current elements (characters, locations, items) and runs 
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

  # Run 3 improvement rounds
  novelgen craft improve --max-rounds 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("craft improve command is not yet implemented")
	},
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
	craftGenCmd.Flags().IntVar(&craftStartChaptersFlg, "start-chapters", 3, "Chapters window to treat as start-of-story for relationship hardening")

	craftImproveCmd.Flags().StringVar(&craftElementTypeFlag, "type", "all", "Element type to improve (all/characters/locations/items)")
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

	log.Info("Extracted elements from outline: characters=%d, locations=%d, items=%d",
		len(elements.Characters),
		len(elements.Locations),
		len(elements.Items))

	// Filter elements based on flags
	if craftChapterFlag != "" {
		log.Info("Filtering by chapter: %s", craftChapterFlag)
		elements = filterElementsByChapter(elements, craftChapterFlag, outline)
		log.Info("After chapter filter: characters=%d, locations=%d, items=%d",
			len(elements.Characters), len(elements.Locations), len(elements.Items))
	} else if craftVolumeFlag != "" {
		log.Info("Filtering by volume: %s", craftVolumeFlag)
		elements = filterElementsByVolume(elements, craftVolumeFlag, outline)
	} else if craftPartFlag != "" {
		log.Info("Filtering by part: %s", craftPartFlag)
		elements = filterElementsByPart(elements, craftPartFlag, outline)
	}

	// Load already generated elements to skip
	generated := loadGeneratedElements()

	// Filter out already generated elements
	elementsToGenerate := filterUnGenerated(elements, generated)

	log.Info("Elements to generate: characters=%d, locations=%d, items=%d",
		len(elementsToGenerate.Characters),
		len(elementsToGenerate.Locations),
		len(elementsToGenerate.Items))

	if len(elementsToGenerate.Characters) == 0 &&
		len(elementsToGenerate.Locations) == 0 &&
		len(elementsToGenerate.Items) == 0 {
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
	agent := agents.NewCraftAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Generate elements in batches
	batchSize := craftBatchFlag
	if batchSize <= 0 {
		batchSize = 1
	}

	// Generate characters
	if err := generateCharacters(agent, elementsToGenerate.Characters, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate characters: %w", err)
	}

	// Generate locations
	if err := generateLocations(agent, elementsToGenerate.Locations, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate locations: %w", err)
	}

	// Generate items
	if err := generateItems(agent, elementsToGenerate.Items, generated, batchSize); err != nil {
		return fmt.Errorf("failed to generate items: %w", err)
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
	Characters     []string
	Locations      []string
	Items          []string
	Organizations  []string
	Races          []string
	AbilitySystems []string
	WorldLore      []string
}

// GeneratedElements tracks already generated elements
type GeneratedElements struct {
	Characters     map[string]bool
	Locations      map[string]bool
	Items          map[string]bool
	Organizations  map[string]bool
	Races          map[string]bool
	AbilitySystems map[string]bool
	WorldLore      map[string]bool
}

func NewElementExtractor(outline *models.Outline, setup *models.StorySetup) *ElementExtractor {
	return &ElementExtractor{outline: outline, setup: setup}
}

func (e *ElementExtractor) Extract() *ExtractedElements {
	result := &ExtractedElements{
		Characters:     make([]string, 0),
		Locations:      make([]string, 0),
		Items:          make([]string, 0),
		Organizations:  make([]string, 0),
		Races:          make([]string, 0),
		AbilitySystems: make([]string, 0),
		WorldLore:      make([]string, 0),
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)
	orgMap := make(map[string]bool)
	raceMap := make(map[string]bool)
	systemMap := make(map[string]bool)
	loreMap := make(map[string]bool)

	// Extract from outline chapters
	for _, part := range e.outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				// Extract characters
				for _, char := range chapter.Characters {
					if !charMap[char] {
						charMap[char] = true
						result.Characters = append(result.Characters, char)
					}
				}

				// Extract location
				if chapter.Location != "" && !locMap[chapter.Location] {
					locMap[chapter.Location] = true
					result.Locations = append(result.Locations, chapter.Location)
				}

				// Extract items from events
				for _, event := range chapter.Events {
					if event.Type == models.EventTypeItem && event.Subject != "" {
						if !itemMap[event.Subject] {
							itemMap[event.Subject] = true
							result.Items = append(result.Items, event.Subject)
						}
					}
				}
			}
		}
	}

	// Extract from story setup premises (ability systems)
	if e.setup != nil {
		for _, premise := range e.setup.Premises {
			if premise.Name != "" && !systemMap[premise.Name] {
				systemMap[premise.Name] = true
				result.AbilitySystems = append(result.AbilitySystems, premise.Name)
			}
		}

		// Extract from storylines (potential organizations or lore)
		for _, storyline := range e.setup.Storylines {
			if storyline.Name != "" {
				// Storylines could represent factions/organizations
				if !orgMap[storyline.Name] && len(result.Organizations) < 10 {
					orgMap[storyline.Name] = true
					result.Organizations = append(result.Organizations, storyline.Name)
				}
			}
		}
	}

	// Suppress unused variable warnings
	_ = raceMap
	_ = loreMap

	return result
}

func filterElementsByChapter(elements *ExtractedElements, chapterID string, outline *models.Outline) *ExtractedElements {
	result := &ExtractedElements{
		Characters: make([]string, 0),
		Locations:  make([]string, 0),
		Items:      make([]string, 0),
	}

	chapter := outline.GetChapterByID(chapterID)
	if chapter == nil {
		return result
	}

	// Get characters from this chapter
	charMap := make(map[string]bool)
	for _, char := range chapter.Characters {
		charMap[char] = true
	}
	for _, char := range elements.Characters {
		if charMap[char] {
			result.Characters = append(result.Characters, char)
		}
	}

	// Get location from this chapter
	if chapter.Location != "" {
		result.Locations = append(result.Locations, chapter.Location)
	}

	// Get items from this chapter's events
	itemMap := make(map[string]bool)
	for _, event := range chapter.Events {
		if event.Type == models.EventTypeItem && event.Subject != "" {
			itemMap[event.Subject] = true
		}
	}
	for _, item := range elements.Items {
		if itemMap[item] {
			result.Items = append(result.Items, item)
		}
	}

	return result
}

func filterElementsByVolume(elements *ExtractedElements, volumeID string, outline *models.Outline) *ExtractedElements {
	result := &ExtractedElements{
		Characters: make([]string, 0),
		Locations:  make([]string, 0),
		Items:      make([]string, 0),
	}

	volume := outline.GetVolumeByID(volumeID)
	if volume == nil {
		return result
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)

	for _, chapter := range volume.Chapters {
		for _, char := range chapter.Characters {
			charMap[char] = true
		}
		if chapter.Location != "" {
			locMap[chapter.Location] = true
		}
		for _, event := range chapter.Events {
			if event.Type == models.EventTypeItem && event.Subject != "" {
				itemMap[event.Subject] = true
			}
		}
	}

	for _, char := range elements.Characters {
		if charMap[char] {
			result.Characters = append(result.Characters, char)
		}
	}
	for _, loc := range elements.Locations {
		if locMap[loc] {
			result.Locations = append(result.Locations, loc)
		}
	}
	for _, item := range elements.Items {
		if itemMap[item] {
			result.Items = append(result.Items, item)
		}
	}

	return result
}

func filterElementsByPart(elements *ExtractedElements, partID string, outline *models.Outline) *ExtractedElements {
	result := &ExtractedElements{
		Characters: make([]string, 0),
		Locations:  make([]string, 0),
		Items:      make([]string, 0),
	}

	part := outline.GetPartByID(partID)
	if part == nil {
		return result
	}

	charMap := make(map[string]bool)
	locMap := make(map[string]bool)
	itemMap := make(map[string]bool)

	for _, volume := range part.Volumes {
		for _, chapter := range volume.Chapters {
			for _, char := range chapter.Characters {
				charMap[char] = true
			}
			if chapter.Location != "" {
				locMap[chapter.Location] = true
			}
			for _, event := range chapter.Events {
				if event.Type == models.EventTypeItem && event.Subject != "" {
					itemMap[event.Subject] = true
				}
			}
		}
	}

	for _, char := range elements.Characters {
		if charMap[char] {
			result.Characters = append(result.Characters, char)
		}
	}
	for _, loc := range elements.Locations {
		if locMap[loc] {
			result.Locations = append(result.Locations, loc)
		}
	}
	for _, item := range elements.Items {
		if itemMap[item] {
			result.Items = append(result.Items, item)
		}
	}

	return result
}

func loadGeneratedElements() *GeneratedElements {
	result := &GeneratedElements{
		Characters: make(map[string]bool),
		Locations:  make(map[string]bool),
		Items:      make(map[string]bool),
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

	return result
}

func filterUnGenerated(elements *ExtractedElements, generated *GeneratedElements) *ExtractedElements {
	result := &ExtractedElements{
		Characters:     make([]string, 0),
		Locations:      make([]string, 0),
		Items:          make([]string, 0),
		Organizations:  make([]string, 0),
		Races:          make([]string, 0),
		AbilitySystems: make([]string, 0),
		WorldLore:      make([]string, 0),
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

	for _, race := range elements.Races {
		if !generated.Races[race] {
			result.Races = append(result.Races, race)
		}
	}

	for _, system := range elements.AbilitySystems {
		if !generated.AbilitySystems[system] {
			result.AbilitySystems = append(result.AbilitySystems, system)
		}
	}

	for _, lore := range elements.WorldLore {
		if !generated.WorldLore[lore] {
			result.WorldLore = append(result.WorldLore, lore)
		}
	}

	return result
}

func generateCharacters(agent *agents.CraftAgent, characters []string, generated *GeneratedElements, batchSize int) error {
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

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating characters batch: count=%d", workerID, len(batch))

				results, err := agent.GenerateCharacters(batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate characters batch: %v", workerID, err)
					continue
				}

				// Save results
				if err := saveCharacters(results); err != nil {
					log.Error("[Worker %d] Failed to save characters: %v", workerID, err)
					continue
				}

				// Update generated tracking
				mu.Lock()
				for name := range results {
					generated.Characters[name] = true
				}
				mu.Unlock()

				log.Info("[Worker %d] Saved %d characters", workerID, len(results))
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

	return nil
}

func generateLocations(agent *agents.CraftAgent, locations []string, generated *GeneratedElements, batchSize int) error {
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

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating locations batch: count=%d", workerID, len(batch))

				results, err := agent.GenerateLocations(batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate locations batch: %v", workerID, err)
					continue
				}

				// Save results
				if err := saveLocations(results); err != nil {
					log.Error("[Worker %d] Failed to save locations: %v", workerID, err)
					continue
				}

				// Update generated tracking
				mu.Lock()
				for name := range results {
					generated.Locations[name] = true
				}
				mu.Unlock()

				log.Info("[Worker %d] Saved %d locations", workerID, len(results))
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

	return nil
}

func generateItems(agent *agents.CraftAgent, items []string, generated *GeneratedElements, batchSize int) error {
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

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for batch := range batchChan {
				log.Info("[Worker %d] Generating items batch: count=%d", workerID, len(batch))

				results, err := agent.GenerateItems(batch, craftPromptFlag)
				if err != nil {
					log.Error("[Worker %d] Failed to generate items batch: %v", workerID, err)
					continue
				}

				// Save results
				if err := saveItems(results); err != nil {
					log.Error("[Worker %d] Failed to save items: %v", workerID, err)
					continue
				}

				// Update generated tracking
				mu.Lock()
				for name := range results {
					generated.Items[name] = true
				}
				mu.Unlock()

				log.Info("[Worker %d] Saved %d items", workerID, len(results))
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

	return nil
}

func saveCharacters(characters map[string]*models.Character) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "characters.json")
	return saveJSON(path, characters)
}

func saveLocations(locations map[string]*models.Location) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "locations.json")
	return saveJSON(path, locations)
}

func saveItems(items map[string]*models.Item) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "story", "craft", "items.json")
	return saveJSON(path, items)
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
		json.Unmarshal(fileData, &existing)
	}

	// Merge new data
	newData, _ := json.Marshal(data)
	var newMap map[string]interface{}
	json.Unmarshal(newData, &newMap)

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

func loadAllElements() (map[string]*models.Character, map[string]*models.Location, map[string]*models.Item, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, nil, nil, err
	}

	characters := make(map[string]*models.Character)
	locations := make(map[string]*models.Location)
	items := make(map[string]*models.Item)

	// Load characters
	charPath := filepath.Join(root, "story", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		if err := json.Unmarshal(data, &characters); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse characters: %w", err)
		}
	}

	// Load locations
	locPath := filepath.Join(root, "story", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		if err := json.Unmarshal(data, &locations); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse locations: %w", err)
		}
	}

	// Load items
	itemPath := filepath.Join(root, "story", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse items: %w", err)
		}
	}

	return characters, locations, items, nil
}
