package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"

	"github.com/spf13/cobra"
)

var (
	draftChapterFlag     string
	draftVolumeFlag      string
	draftPartFlag        string
	draftWordsFlag       int
	draftAllFlag         bool
	draftConcurrencyFlag int
)

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Generate draft chapters",
	Long: `Generate draft chapters based on outline and current story state.

The draft command calculates the story state matrix by applying events from
previous chapters, then generates a ~500 word draft for the specified chapter.`,
}

var draftGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate draft chapter(s)",
	Long: `Generate draft chapter(s) based on outline and story state.

Examples:
  # Generate draft for chapter 1
  novel draft gen --chapter 1

  # Generate draft for chapters 1 to 5
  novel draft gen --chapter 1-5

  # Generate draft for all chapters
  novel draft gen --all

  # Generate draft for specific chapter by ID
  novel draft gen --chapter chap_1_1_1

  # Generate with custom word count
  novel draft gen --chapter 1 --words 800

  # Generate for first chapter of volume 2
  novel draft gen --volume 2 --chapter 1`,
	RunE: runDraftGen,
}

func init() {
	draftCmd.AddCommand(draftGenCmd)

	draftGenCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'chap_1_1_1')")
	draftGenCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume number for context (e.g., '1')")
	draftGenCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part number for context (e.g., '1')")
	draftGenCmd.Flags().IntVar(&draftWordsFlag, "words", 500, "Target word count for the draft")
	draftGenCmd.Flags().BoolVar(&draftAllFlag, "all", false, "Generate drafts for all chapters")
	draftGenCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")

	rootCmd.AddCommand(draftCmd)
}

func runDraftGen(cmd *cobra.Command, args []string) error {
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

	// Create draft agent
	agent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get list of chapters to generate
	chapters, err := getChaptersToGenerate(outline, draftChapterFlag, draftVolumeFlag, draftPartFlag, draftAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating drafts for %d chapter(s) with concurrency %d", len(chapters), draftConcurrencyFlag)

	// Use worker pool for concurrent generation
	concurrency := draftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	// Create work channel and wait group
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Generating draft for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Calculate story state matrix
				stateMatrix := calculateStateMatrix(outline, chapter)

				// Generate draft
				draft, err := agent.GenerateDraft(chapter, &agents.StateMatrix{
					Characters:    stateMatrix.Characters,
					Locations:     stateMatrix.Locations,
					Items:         stateMatrix.Items,
					Relationships: stateMatrix.Relationships,
					Storylines:    stateMatrix.Storylines,
					Premises:      stateMatrix.Premises,
				}, draftWordsFlag)
				if err != nil {
					log.Error("Failed to generate draft for chapter %s: %v", chapter.ID, err)
					continue
				}

				// Save draft
				if err := saveDraft(chapter, draft); err != nil {
					log.Error("Failed to save draft for chapter %s: %v", chapter.ID, err)
					continue
				}

				log.Info("[Worker %d] Draft saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(draft)))
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	log.Info("Draft generation complete")
	return nil
}

// findTargetChapter finds the target chapter based on flags
// getChaptersToGenerate returns list of chapters to generate based on flags
func getChaptersToGenerate(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) ([]*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	// If --all flag, return all chapters
	if allFlag {
		return getAllChapters(outline), nil
	}

	// If no chapter flag, error
	if chapterFlag == "" {
		return nil, fmt.Errorf("please specify --chapter or --all")
	}

	// Check if it's a range (e.g., "1-5")
	if strings.Contains(chapterFlag, "-") {
		parts := strings.Split(chapterFlag, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return getChapterRange(outline, start, end)
			}
		}
	}

	// Single chapter
	chapter, err := findTargetChapter(outline, chapterFlag, volumeFlag, partFlag)
	if err != nil {
		return nil, err
	}
	return []*models.Chapter{chapter}, nil
}

// getAllChapters returns all chapters in order
func getAllChapters(outline *models.Outline) []*models.Chapter {
	var chapters []*models.Chapter
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i := range vol.Chapters {
				chapters = append(chapters, &vol.Chapters[i])
			}
		}
	}
	return chapters
}

// getChapterRange returns chapters from start to end (inclusive)
func getChapterRange(outline *models.Outline, start, end int) ([]*models.Chapter, error) {
	allChapters := getAllChapters(outline)

	if start < 1 || start > len(allChapters) {
		return nil, fmt.Errorf("start chapter %d out of range (1-%d)", start, len(allChapters))
	}
	if end < 1 || end > len(allChapters) {
		return nil, fmt.Errorf("end chapter %d out of range (1-%d)", end, len(allChapters))
	}
	if start > end {
		return nil, fmt.Errorf("start chapter %d must be <= end chapter %d", start, end)
	}

	return allChapters[start-1 : end], nil
}

// findTargetChapter finds the target chapter based on flags
func findTargetChapter(outline *models.Outline, chapterFlag, volumeFlag, partFlag string) (*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	// If chapter flag contains "chap_", treat it as ID
	if strings.HasPrefix(chapterFlag, "chap_") {
		return findChapterByID(outline, chapterFlag)
	}

	// Parse chapter number
	chapterNum, err := strconv.Atoi(chapterFlag)
	if err != nil {
		return nil, fmt.Errorf("invalid chapter number: %s", chapterFlag)
	}

	// If volume specified, find chapter in that volume
	if volumeFlag != "" {
		volNum, err := strconv.Atoi(volumeFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid volume number: %s", volumeFlag)
		}
		return findChapterInVolume(outline, volNum, chapterNum)
	}

	// Otherwise find by global chapter number
	return findChapterByNumber(outline, chapterNum)
}

// findChapterByID finds a chapter by its ID
func findChapterByID(outline *models.Outline, chapterID string) (*models.Chapter, error) {
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if ch.ID == chapterID {
					return &ch, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("chapter not found: %s", chapterID)
}

// findChapterInVolume finds a chapter by volume and chapter number
func findChapterInVolume(outline *models.Outline, volNum, chapterNum int) (*models.Chapter, error) {
	volIdx := volNum - 1
	chapterIdx := chapterNum - 1

	for _, part := range outline.Parts {
		if volIdx >= 0 && volIdx < len(part.Volumes) {
			vol := part.Volumes[volIdx]
			if chapterIdx >= 0 && chapterIdx < len(vol.Chapters) {
				return &vol.Chapters[chapterIdx], nil
			}
		}
	}
	return nil, fmt.Errorf("chapter %d in volume %d not found", chapterNum, volNum)
}

// findChapterByNumber finds a chapter by global chapter number
func findChapterByNumber(outline *models.Outline, chapterNum int) (*models.Chapter, error) {
	currentNum := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i := range vol.Chapters {
				currentNum++
				if currentNum == chapterNum {
					return &vol.Chapters[i], nil
				}
			}
		}
	}
	return nil, fmt.Errorf("chapter %d not found", chapterNum)
}

// StateMatrix represents the current state of the story
type StateMatrix struct {
	Characters    map[string]*models.Character
	Locations     map[string]*models.Location
	Items         map[string]*models.Item
	Relationships map[string]string // "char1_char2" -> "relationship state"
	Storylines    map[string]string // storyline ID -> current state
	Premises      map[string]string // premise ID -> current state for each character
}

// calculateStateMatrix calculates the story state up to the target chapter
func calculateStateMatrix(outline *models.Outline, targetChapter *models.Chapter) *StateMatrix {
	state := &StateMatrix{
		Characters:    make(map[string]*models.Character),
		Locations:     make(map[string]*models.Location),
		Items:         make(map[string]*models.Item),
		Relationships: make(map[string]string),
		Storylines:    make(map[string]string),
		Premises:      make(map[string]string),
	}

	// Load all generated elements
	loadElementsIntoState(state)

	// Apply events from all chapters up to target
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Stop when we reach target chapter
				if ch.ID == targetChapter.ID {
					return state
				}

				// Apply events from this chapter
				for _, event := range ch.Events {
					applyEvent(state, event)
				}
			}
		}
	}

	return state
}

// applyEvent applies a single event to the state matrix
func applyEvent(state *StateMatrix, event models.Event) {
	switch event.Type {
	case "relationship":
		// Format: relationship between char1 and char2 changes
		if len(event.Characters) >= 2 {
			key := event.Characters[0] + "_" + event.Characters[1]
			state.Relationships[key] = event.Change
		}
	case "goal":
		// Character goal update
		if len(event.Characters) > 0 {
			charName := event.Characters[0]
			if char, exists := state.Characters[charName]; exists {
				// Update character's goals based on event
				if event.Change != "" {
					char.Goals = append(char.Goals, event.Change)
				}
			}
		}
	case "item":
		// Character gets or loses item
		if len(event.Characters) > 0 && event.Subject != "" {
			charName := event.Characters[0]
			itemName := event.Subject
			if event.Change == "get" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = charName
				}
			} else if event.Change == "lost" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = ""
				}
			}
		}
	case "premise":
		// Character premise/progression update
		if len(event.Characters) > 0 {
			key := event.Characters[0] + "_" + event.Subject
			state.Premises[key] = event.Change
		}
	case "storyline":
		// Storyline progression
		if event.Subject != "" {
			state.Storylines[event.Subject] = event.Change
		}
	}
}

// loadElementsIntoState loads generated elements into state matrix
func loadElementsIntoState(state *StateMatrix) {
	root, err := findProjectRoot()
	if err != nil {
		return
	}

	// Load characters
	charPath := filepath.Join(root, "config", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		var chars map[string]*models.Character
		if err := json.Unmarshal(data, &chars); err == nil {
			for name, char := range chars {
				state.Characters[name] = char
			}
		}
	}

	// Load locations
	locPath := filepath.Join(root, "config", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		var locs map[string]*models.Location
		if err := json.Unmarshal(data, &locs); err == nil {
			for name, loc := range locs {
				state.Locations[name] = loc
			}
		}
	}

	// Load items
	itemPath := filepath.Join(root, "config", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		var items map[string]*models.Item
		if err := json.Unmarshal(data, &items); err == nil {
			for name, item := range items {
				state.Items[name] = item
			}
		}
	}
}

// saveDraft saves the generated draft to file
func saveDraft(chapter *models.Chapter, draft string) error {
	draftsDir := filepath.Join("drafts")
	if err := os.MkdirAll(draftsDir, 0755); err != nil {
		return fmt.Errorf("failed to create drafts directory: %w", err)
	}

	filename := filepath.Join(draftsDir, chapter.ID+".md")
	content := fmt.Sprintf("# %s\n\n%s\n\n%s\n", chapter.Title, chapter.Summary, draft)

	return os.WriteFile(filename, []byte(content), 0644)
}
