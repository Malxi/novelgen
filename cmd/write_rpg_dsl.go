package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
)

const writeRPGDSLCacheVersion = 3

type writeRPGDSLResult struct {
	DSLPath          string
	CacheDir         string
	ChapterCount     int
	BatchCount       int
	ReusedBatches    int
	GeneratedBatches int
	Issues           []rpgdsl.SimulationIssue // DSL-RPG simulate 检测出的 issues
}

type writeRPGDSLBatchManifest struct {
	Version    int                       `json:"version"`
	BookName   string                    `json:"book_name"`
	BatchIndex int                       `json:"batch_index"`
	BatchSize  int                       `json:"batch_size"`
	BatchHash  string                    `json:"batch_hash"`
	DSLFile    string                    `json:"dsl_file"`
	UpdatedAt  string                    `json:"updated_at"`
	Chapters   []writeRPGDSLChapterState `json:"chapters"`
}

type writeRPGDSLChapterState struct {
	ChapterID     string `json:"chapter_id"`
	ChapterPath   string `json:"chapter_path"`
	ChapterSHA256 string `json:"chapter_sha256"`
	RecapPath     string `json:"recap_path,omitempty"`
	RecapSHA256   string `json:"recap_sha256,omitempty"`
}

type writeRPGDSLChapterInput struct {
	ChapterID   string
	ChapterPath string
	RecapPath   string
}

func emitChapterRPGDSLForProject(
	ctx context.Context,
	projectRoot string,
	bookName string,
	projectConfig *models.ProjectConfig,
	llmConfig *llm.Config,
	setup *models.StorySetup,
	client llm.Client,
	batchSize int,
) (writeRPGDSLResult, error) {
	result := writeRPGDSLResult{}
	if batchSize <= 0 {
		batchSize = 10
	}

	outline, err := loadOutline()
	if err != nil {
		return result, err
	}
	chapterInputs, err := findProjectChapterInputs(projectRoot, outline)
	if err != nil {
		return result, err
	}
	if len(chapterInputs) == 0 {
		return result, fmt.Errorf("no chapter markdown files found under %s", filepath.Join(projectRoot, "chapters"))
	}
	sortRPGDSLChapterInputs(chapterInputs)

	characters, locations, _, _, err := loadAllElements()
	if err != nil {
		return result, err
	}
	characterValues := derefCharacterMap(characters)
	locationValues := derefLocationMap(locations)

	agent := agents.NewChapterToDSLAgent(client, llmConfig, &projectConfig.LLM, setup)
	agent.SetLanguage(projectConfig.Language)
	agent.SetRequireAI(true)

	rpgDir := filepath.Join(projectRoot, "story", "rpg")
	cacheDir := filepath.Join(rpgDir, "cache", "chapter_batches")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return result, err
	}

	combined := &rpgdsl.DSL{
		Metadata:   &rpgdsl.Metadata{},
		World:      &rpgdsl.World{},
		Characters: &rpgdsl.Characters{},
		Storyline:  &rpgdsl.Storyline{},
		Systems:    &rpgdsl.Systems{},
	}

	for batchIndex, batch := range splitRPGDSLBatchInputs(chapterInputs, batchSize) {
		manifest, err := buildWriteRPGDSLBatchManifest(bookName, batchIndex*batchSize, batchSize, batch, cacheDir)
		if err != nil {
			return result, err
		}
		result.BatchCount++

		dslContent, reused, err := loadWriteRPGDSLBatchFromCache(manifest)
		if err != nil {
			return result, err
		}
		if reused {
			result.ReusedBatches++
			logger.Info("RPG DSL batch cache reused: %s -> %s (%d chapters)", manifest.Chapters[0].ChapterID, manifest.Chapters[len(manifest.Chapters)-1].ChapterID, len(manifest.Chapters))
		} else {
			result.GeneratedBatches++
			logger.Info("RPG DSL AI conversion batch: %s -> %s (%d chapters)", manifest.Chapters[0].ChapterID, manifest.Chapters[len(manifest.Chapters)-1].ChapterID, len(manifest.Chapters))
			chapterData, err := loadWriteRPGDSLChapterData(batch)
			if err != nil {
				return result, err
			}
			dslContent, err = agent.ConvertChapterData(ctx, bookName, characterValues, locationValues, chapterData)
			if err != nil {
				return result, fmt.Errorf("AI chapter -> RPG DSL failed for batch %d: %w", batchIndex+1, err)
			}
			dslContent, err = ensureWriteRPGDSLParseable(ctx, agent, bookName, dslContent)
			if err != nil {
				return result, fmt.Errorf("RPG DSL parse/repair failed for batch %d: %w", batchIndex+1, err)
			}
			if err := saveWriteRPGDSLBatchToCache(manifest, dslContent); err != nil {
				return result, err
			}
			logger.Info("RPG DSL batch output: %s", manifest.DSLFile)
		}

		parsed, err := rpgdsl.NewParser(dslContent).Parse()
		if err != nil {
			return result, fmt.Errorf("failed to parse RPG DSL batch cache %d: %w", batchIndex+1, err)
		}
		normalizeWriteRPGDSLChapterIDs(parsed, manifest)
		mergeWriteRPGDSL(combined, parsed)
	}

	outputPath := filepath.Join(rpgDir, "04_chapters.rpg")
	if err := os.WriteFile(outputPath, []byte(combined.String()), 0644); err != nil {
		return result, err
	}

	// Run DSL-RPG Simulator to detect issues
	simulator := rpgdsl.NewSimulator(combined)
	result.Issues = simulator.SimulateAll()

	result.DSLPath = outputPath
	result.CacheDir = cacheDir
	result.ChapterCount = len(chapterInputs)
	return result, nil
}

func findProjectChapterInputs(projectRoot string, outline *models.Outline) ([]writeRPGDSLChapterInput, error) {
	chapterFiles := map[string]string{}
	for _, chapter := range allOutlineChapters(outline) {
		for _, path := range candidateChapterMarkdownPaths(projectRoot, chapter.ID) {
			if _, err := os.Stat(path); err == nil {
				chapterFiles[chapter.ID] = path
				break
			}
		}
	}

	if len(chapterFiles) == 0 {
		pattern := filepath.Join(projectRoot, "chapters", "chapter-*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			chapterID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "chapter-"), filepath.Ext(file))
			chapterFiles[chapterID] = file
		}
	}

	inputs := make([]writeRPGDSLChapterInput, 0, len(chapterFiles))
	for chapterID, chapterPath := range chapterFiles {
		input := writeRPGDSLChapterInput{
			ChapterID:   chapterID,
			ChapterPath: chapterPath,
		}
		recapPath := filepath.Join(projectRoot, "story", "recaps", chapterID+".json")
		if _, err := os.Stat(recapPath); err == nil {
			input.RecapPath = recapPath
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func allOutlineChapters(outline *models.Outline) []*models.Chapter {
	if outline == nil {
		return nil
	}
	var chapters []*models.Chapter
	for pi := range outline.Parts {
		for vi := range outline.Parts[pi].Volumes {
			for ci := range outline.Parts[pi].Volumes[vi].Chapters {
				chapters = append(chapters, &outline.Parts[pi].Volumes[vi].Chapters[ci])
			}
		}
	}
	return chapters
}

func candidateChapterMarkdownPaths(projectRoot, chapterID string) []string {
	return []string{
		filepath.Join(projectRoot, "chapters", "chapter-"+chapterID+".md"),
		filepath.Join(projectRoot, "chapters", "chapter-"+extractChapterNumber(chapterID)+".md"),
	}
}

func ensureWriteRPGDSLParseable(ctx context.Context, agent *agents.ChapterToDSLAgent, bookName, content string) (string, error) {
	if _, err := rpgdsl.NewParser(content).Parse(); err == nil {
		return content, nil
	} else {
		lastErr := err
		for attempt := 0; attempt < 2; attempt++ {
			repaired, repairErr := agent.RepairDSL(ctx, bookName, lastErr.Error(), content)
			if repairErr != nil {
				return "", repairErr
			}
			if _, parseErr := rpgdsl.NewParser(repaired).Parse(); parseErr == nil {
				logger.Info("RPG DSL auto-repair succeeded on attempt %d/2", attempt+1)
				return repaired, nil
			} else {
				lastErr = parseErr
				content = repaired
			}
		}
		dumpWriteRPGDSLFailure(bookName, content, lastErr)
		return "", lastErr
	}
}

func loadWriteRPGDSLChapterData(inputs []writeRPGDSLChapterInput) ([]agents.ChapterData, error) {
	chapters := make([]agents.ChapterData, 0, len(inputs))
	for _, input := range inputs {
		content, err := os.ReadFile(input.ChapterPath)
		if err != nil {
			return nil, err
		}
		chapter := agents.ChapterData{
			ChapterID: input.ChapterID,
			Title:     firstMarkdownHeadingForWriteRPGDSL(string(content), input.ChapterID),
			Content:   string(content),
		}
		if input.RecapPath != "" {
			recapData, err := os.ReadFile(input.RecapPath)
			if err != nil {
				return nil, err
			}
			var recap agents.ChapterRecap
			if err := json.Unmarshal(recapData, &recap); err != nil {
				return nil, err
			}
			chapter.Recap = &recap
			chapter.Location = recap.Location
			chapter.Time = recap.Time
			chapter.Present = recap.Present
			chapter.PlotBeats = recap.PlotBeats
			if strings.TrimSpace(recap.Title) != "" {
				chapter.Title = recap.Title
			}
		}
		chapters = append(chapters, chapter)
	}
	return chapters, nil
}

func firstMarkdownHeadingForWriteRPGDSL(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return fallback
}

func dumpWriteRPGDSLFailure(bookName, content string, parseErr error) {
	dir := filepath.Join("books", bookName, "story", "rpg", "cache", "failed_dsl")
	if projectDir := strings.TrimSpace(logger.Default().ProjectDir()); projectDir != "" {
		dir = filepath.Join(projectDir, "story", "rpg", "cache", "failed_dsl")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	stem := filepath.Join(dir, "failed_"+time.Now().Format("20060102_150405"))
	_ = os.WriteFile(stem+".rpg", []byte(content), 0644)
	_ = os.WriteFile(stem+".err.txt", []byte(parseErr.Error()), 0644)
	logger.Error("Failed RPG DSL dump: %s.rpg", stem)
}

func buildWriteRPGDSLBatchManifest(bookName string, batchIndex, batchSize int, chapterInputs []writeRPGDSLChapterInput, cacheDir string) (*writeRPGDSLBatchManifest, error) {
	chapters := make([]writeRPGDSLChapterState, 0, len(chapterInputs))
	var hashInput strings.Builder
	hashInput.WriteString(fmt.Sprintf("version=%d\nbook=%s\nbatch_index=%d\nbatch_size=%d\n", writeRPGDSLCacheVersion, bookName, batchIndex, batchSize))
	for _, input := range chapterInputs {
		chapterSum, err := fileSHA256ForWriteRPGDSL(input.ChapterPath)
		if err != nil {
			return nil, err
		}
		recapSum := ""
		if input.RecapPath != "" {
			recapSum, err = fileSHA256ForWriteRPGDSL(input.RecapPath)
			if err != nil {
				return nil, err
			}
		}
		chapters = append(chapters, writeRPGDSLChapterState{
			ChapterID:     input.ChapterID,
			ChapterPath:   filepath.ToSlash(input.ChapterPath),
			ChapterSHA256: chapterSum,
			RecapPath:     filepath.ToSlash(input.RecapPath),
			RecapSHA256:   recapSum,
		})
		hashInput.WriteString(input.ChapterID + ".chapter=" + chapterSum + "\n")
		hashInput.WriteString(input.ChapterID + ".recap=" + recapSum + "\n")
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("empty RPG DSL batch")
	}

	batchHashBytes := sha256.Sum256([]byte(hashInput.String()))
	batchHash := hex.EncodeToString(batchHashBytes[:])
	stem := fmt.Sprintf("batch_%03d_%s_%s_%s", batchIndex+1, chapters[0].ChapterID, chapters[len(chapters)-1].ChapterID, batchHash[:12])
	return &writeRPGDSLBatchManifest{
		Version:    writeRPGDSLCacheVersion,
		BookName:   bookName,
		BatchIndex: batchIndex,
		BatchSize:  batchSize,
		BatchHash:  batchHash,
		DSLFile:    filepath.Join(cacheDir, stem+".rpg"),
		Chapters:   chapters,
	}, nil
}

func loadWriteRPGDSLBatchFromCache(expected *writeRPGDSLBatchManifest) (string, bool, error) {
	manifestPath := strings.TrimSuffix(expected.DSLFile, filepath.Ext(expected.DSLFile)) + ".json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	var cached writeRPGDSLBatchManifest
	if err := json.Unmarshal(data, &cached); err != nil {
		return "", false, nil
	}
	if cached.Version != expected.Version || cached.BookName != expected.BookName || cached.BatchIndex != expected.BatchIndex || cached.BatchSize != expected.BatchSize || cached.BatchHash != expected.BatchHash || cached.DSLFile != expected.DSLFile {
		return "", false, nil
	}
	content, err := os.ReadFile(expected.DSLFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if _, err := rpgdsl.NewParser(string(content)).Parse(); err != nil {
		return "", false, nil
	}
	return string(content), true, nil
}

func saveWriteRPGDSLBatchToCache(manifest *writeRPGDSLBatchManifest, content string) error {
	if err := os.WriteFile(manifest.DSLFile, []byte(content), 0644); err != nil {
		return err
	}
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestPath := strings.TrimSuffix(manifest.DSLFile, filepath.Ext(manifest.DSLFile)) + ".json"
	return os.WriteFile(manifestPath, data, 0644)
}

func normalizeWriteRPGDSLChapterIDs(parsed *rpgdsl.DSL, manifest *writeRPGDSLBatchManifest) {
	if parsed == nil || parsed.Storyline == nil || manifest == nil {
		return
	}
	for i := range parsed.Storyline.Chapters {
		if i >= len(manifest.Chapters) {
			break
		}
		parsed.Storyline.Chapters[i].ID = manifest.Chapters[i].ChapterID
	}
}

func splitRPGDSLBatchInputs(inputs []writeRPGDSLChapterInput, batchSize int) [][]writeRPGDSLChapterInput {
	var batches [][]writeRPGDSLChapterInput
	for start := 0; start < len(inputs); start += batchSize {
		end := start + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batches = append(batches, inputs[start:end])
	}
	return batches
}

func sortRPGDSLChapterInputs(inputs []writeRPGDSLChapterInput) {
	sort.Slice(inputs, func(i, j int) bool {
		a1, b1, c1 := parseRPGDSLChapterID(inputs[i].ChapterID)
		a2, b2, c2 := parseRPGDSLChapterID(inputs[j].ChapterID)
		if a1 != a2 {
			return a1 < a2
		}
		if b1 != b2 {
			return b1 < b2
		}
		if c1 != c2 {
			return c1 < c2
		}
		return inputs[i].ChapterID < inputs[j].ChapterID
	})
}

func parseRPGDSLChapterID(chapterID string) (int, int, int) {
	match := regexp.MustCompile(`P(\d+)-V(\d+)-C(\d+)`).FindStringSubmatch(chapterID)
	if len(match) != 4 {
		return 9999, 9999, 9999
	}
	p, _ := strconv.Atoi(match[1])
	v, _ := strconv.Atoi(match[2])
	c, _ := strconv.Atoi(match[3])
	return p, v, c
}

func fileSHA256ForWriteRPGDSL(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func derefCharacterMap(values map[string]*models.Character) map[string]models.Character {
	out := make(map[string]models.Character, len(values))
	for key, value := range values {
		if value != nil {
			out[key] = *value
		}
	}
	return out
}

func derefLocationMap(values map[string]*models.Location) map[string]models.Location {
	out := make(map[string]models.Location, len(values))
	for key, value := range values {
		if value != nil {
			out[key] = *value
		}
	}
	return out
}

func mergeWriteRPGDSL(dst, src *rpgdsl.DSL) {
	if src == nil {
		return
	}
	if src.Metadata != nil {
		if dst.Metadata.Title == "" {
			dst.Metadata.Title = src.Metadata.Title
		}
		if dst.Metadata.Subtitle == "" {
			dst.Metadata.Subtitle = src.Metadata.Subtitle
		}
		if dst.Metadata.PowerSystem == "" {
			dst.Metadata.PowerSystem = src.Metadata.PowerSystem
		}
		if dst.Metadata.Tone == "" {
			dst.Metadata.Tone = src.Metadata.Tone
		}
		if dst.Metadata.DSLVersion == "" {
			dst.Metadata.DSLVersion = src.Metadata.DSLVersion
		}
		dst.Metadata.Genre = appendMissingWriteRPGDSLStrings(dst.Metadata.Genre, src.Metadata.Genre)
	}
	if src.World != nil {
		dst.World.Locations = appendMissingWriteRPGDSLLocations(dst.World.Locations, src.World.Locations)
		dst.World.Items = appendMissingWriteRPGDSLItems(dst.World.Items, src.World.Items)
		dst.World.Rules = append(dst.World.Rules, src.World.Rules...)
	}
	if src.Characters != nil {
		if dst.Characters.Player == nil {
			dst.Characters.Player = src.Characters.Player
		}
		dst.Characters.Enemies = appendMissingWriteRPGDSLEnemies(dst.Characters.Enemies, src.Characters.Enemies)
		dst.Characters.NPCs = appendMissingWriteRPGDSLNPCs(dst.Characters.NPCs, src.Characters.NPCs)
	}
	if src.Storyline != nil {
		dst.Storyline.Arcs = appendMissingWriteRPGDSLArcs(dst.Storyline.Arcs, src.Storyline.Arcs)
		dst.Storyline.Chapters = append(dst.Storyline.Chapters, src.Storyline.Chapters...)
	}
	if src.Systems != nil && dst.Systems == nil {
		dst.Systems = src.Systems
	}
}

func appendMissingWriteRPGDSLStrings(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, item := range dst {
		seen[item] = true
	}
	for _, item := range src {
		if !seen[item] {
			dst = append(dst, item)
			seen[item] = true
		}
	}
	return dst
}

func appendMissingWriteRPGDSLLocations(dst, src []rpgdsl.Location) []rpgdsl.Location {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item.ID] = true
	}
	for _, item := range src {
		if !seen[item.ID] {
			dst = append(dst, item)
			seen[item.ID] = true
		}
	}
	return dst
}

func appendMissingWriteRPGDSLItems(dst, src []rpgdsl.Item) []rpgdsl.Item {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item.ID] = true
	}
	for _, item := range src {
		if !seen[item.ID] {
			dst = append(dst, item)
			seen[item.ID] = true
		}
	}
	return dst
}

func appendMissingWriteRPGDSLEnemies(dst, src []rpgdsl.Enemy) []rpgdsl.Enemy {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item.ID] = true
	}
	for _, item := range src {
		if !seen[item.ID] {
			dst = append(dst, item)
			seen[item.ID] = true
		}
	}
	return dst
}

func appendMissingWriteRPGDSLNPCs(dst, src []rpgdsl.NPC) []rpgdsl.NPC {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item.ID] = true
	}
	for _, item := range src {
		if !seen[item.ID] {
			dst = append(dst, item)
			seen[item.ID] = true
		}
	}
	return dst
}

func appendMissingWriteRPGDSLArcs(dst, src []rpgdsl.Arc) []rpgdsl.Arc {
	seen := map[string]bool{}
	for _, item := range dst {
		seen[item.ID] = true
	}
	for _, item := range src {
		if !seen[item.ID] {
			dst = append(dst, item)
			seen[item.ID] = true
		}
	}
	return dst
}

// FilterImportantIssues 过滤出重要的 issues (critical 和 warning)
func FilterImportantIssues(issues []rpgdsl.SimulationIssue) []rpgdsl.SimulationIssue {
	var important []rpgdsl.SimulationIssue
	for _, issue := range issues {
		if issue.Severity == rpgdsl.SeverityCritical || issue.Severity == rpgdsl.SeverityWarning {
			important = append(important, issue)
		}
	}
	return important
}

// GroupIssuesByChapter 将 issues 按章节分组
func GroupIssuesByChapter(issues []rpgdsl.SimulationIssue) map[string][]rpgdsl.SimulationIssue {
	grouped := make(map[string][]rpgdsl.SimulationIssue)
	for _, issue := range issues {
		chapterID := issue.Chapter
		if chapterID == "" {
			chapterID = "_global_" // 全局问题
		}
		grouped[chapterID] = append(grouped[chapterID], issue)
	}
	return grouped
}

// FormatIssuesAsSuggestions 将 issues 格式化为 improvement suggestions
func FormatIssuesAsSuggestions(issues []rpgdsl.SimulationIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## DSL-RPG 模拟检测发现的问题\n\n")

	for i, issue := range issues {
		severity := "警告"
		if issue.Severity == rpgdsl.SeverityCritical {
			severity = "严重"
		}

		sb.WriteString(fmt.Sprintf("### 问题 %d [%s] %s\n", i+1, severity, issue.Type))
		sb.WriteString(fmt.Sprintf("- **描述**: %s\n", issue.Description))
		sb.WriteString(fmt.Sprintf("- **建议**: %s\n", issue.Suggestion))
		if issue.Chapter != "" {
			sb.WriteString(fmt.Sprintf("- **位置**: 章节 %s", issue.Chapter))
			if issue.Step > 0 {
				sb.WriteString(fmt.Sprintf(" (步骤 %d)", issue.Step))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetChapterSpecificSuggestions 获取特定章节的 suggestions
func GetChapterSpecificSuggestions(issues []rpgdsl.SimulationIssue, chapterID string) string {
	var chapterIssues []rpgdsl.SimulationIssue
	for _, issue := range issues {
		if issue.Chapter == chapterID {
			chapterIssues = append(chapterIssues, issue)
		}
	}
	return FormatIssuesAsSuggestions(chapterIssues)
}
