package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
	"novelgen/internal/rpg/dsl"
)

const batchCacheVersion = 3

type batchManifest struct {
	Version    int                 `json:"version"`
	BookName   string              `json:"book_name"`
	BatchIndex int                 `json:"batch_index"`
	BatchSize  int                 `json:"batch_size"`
	BatchHash  string              `json:"batch_hash"`
	DSLFile    string              `json:"dsl_file"`
	UpdatedAt  string              `json:"updated_at"`
	Chapters   []batchChapterState `json:"chapters"`
}

type batchChapterState struct {
	ChapterID     string `json:"chapter_id"`
	ChapterPath   string `json:"chapter_path"`
	ChapterSHA256 string `json:"chapter_sha256"`
	RecapPath     string `json:"recap_path,omitempty"`
	RecapSHA256   string `json:"recap_sha256,omitempty"`
}

type chapterDSLInputFile struct {
	ChapterID   string
	ChapterPath string
	RecapPath   string
}

func convertChaptersWithBatchCache(
	ctx context.Context,
	agent *agents.ChapterToDSLAgent,
	bookName, bookPath string,
	characters map[string]models.Character,
	locations map[string]models.Location,
	chapterFiles []chapterDSLInputFile,
	batchSize int,
) (string, *dsl.DSL, error) {
	if batchSize <= 0 {
		batchSize = 10
	}

	sortChapterJSONFiles(chapterFiles)

	cacheDir := filepath.Join(bookPath, "story", "rpg", "cache", "chapter_batches")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", nil, fmt.Errorf("创建 batch cache 目录失败: %w", err)
	}

	combined := &dsl.DSL{
		Metadata:   &dsl.Metadata{},
		World:      &dsl.World{},
		Characters: &dsl.Characters{},
		Storyline:  &dsl.Storyline{},
		Systems:    &dsl.Systems{},
	}

	for batchIndex, batch := range splitChapterBatches(chapterFiles, batchSize) {
		batchDSLs, err := convertChapterBatchAdaptive(
			ctx,
			agent,
			bookName,
			characters,
			locations,
			batch,
			batchIndex*batchSize,
			batchSize,
			cacheDir,
		)
		if err != nil {
			return "", nil, err
		}
		for _, batchDSL := range batchDSLs {
			mergeBatchDSL(combined, batchDSL)
		}
	}

	content := combined.String()
	return content, combined, nil
}

func convertChapterBatchAdaptive(
	ctx context.Context,
	agent *agents.ChapterToDSLAgent,
	bookName string,
	characters map[string]models.Character,
	locations map[string]models.Location,
	chapterFiles []chapterDSLInputFile,
	batchIndex int,
	configuredBatchSize int,
	cacheDir string,
) ([]*dsl.DSL, error) {
	batchDSL, err := convertChapterBatch(ctx, agent, bookName, characters, locations, chapterFiles, batchIndex, configuredBatchSize, cacheDir)
	if err == nil {
		return []*dsl.DSL{batchDSL}, nil
	}
	if len(chapterFiles) <= 1 {
		return nil, err
	}

	mid := len(chapterFiles) / 2
	fmt.Printf("batch %d 失败，自动拆分为 %d + %d 章: %v\n", batchIndex+1, mid, len(chapterFiles)-mid, err)
	left, leftErr := convertChapterBatchAdaptive(ctx, agent, bookName, characters, locations, chapterFiles[:mid], batchIndex, configuredBatchSize, cacheDir)
	if leftErr != nil {
		return nil, leftErr
	}
	right, rightErr := convertChapterBatchAdaptive(ctx, agent, bookName, characters, locations, chapterFiles[mid:], batchIndex+mid, configuredBatchSize, cacheDir)
	if rightErr != nil {
		return nil, rightErr
	}
	return append(left, right...), nil
}

func convertChapterBatch(
	ctx context.Context,
	agent *agents.ChapterToDSLAgent,
	bookName string,
	characters map[string]models.Character,
	locations map[string]models.Location,
	chapterFiles []chapterDSLInputFile,
	batchIndex int,
	configuredBatchSize int,
	cacheDir string,
) (*dsl.DSL, error) {
	manifest, err := buildBatchManifest(bookName, batchIndex, configuredBatchSize, chapterFiles, cacheDir)
	if err != nil {
		return nil, err
	}

	dslContent, reused, err := loadBatchFromCache(manifest)
	if err != nil {
		return nil, err
	}
	if !reused {
		fmt.Printf("AI 转换 batch %d: %s -> %s (%d 章)\n",
			batchIndex+1,
			manifest.Chapters[0].ChapterID,
			manifest.Chapters[len(manifest.Chapters)-1].ChapterID,
			len(manifest.Chapters),
		)
		chapterData, err := loadChapterDSLInputData(chapterFiles)
		if err != nil {
			return nil, err
		}
		dslContent, err = agent.ConvertChapterData(ctx, bookName, characters, locations, chapterData)
		if err != nil {
			return nil, fmt.Errorf("AI 转换 batch %d 失败: %w", batchIndex+1, err)
		}
		dslContent, err = ensureParseableDSL(ctx, agent, bookName, dslContent)
		if err != nil {
			return nil, fmt.Errorf("batch %d DSL 解析/修复失败: %w", batchIndex+1, err)
		}
		if err := saveBatchToCache(manifest, dslContent); err != nil {
			return nil, err
		}
	} else {
		fmt.Printf("复用 batch cache %d: %s -> %s (%d 章)\n",
			batchIndex+1,
			manifest.Chapters[0].ChapterID,
			manifest.Chapters[len(manifest.Chapters)-1].ChapterID,
			len(manifest.Chapters),
		)
	}

	parser := dsl.NewParser(dslContent)
	batchDSL, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("解析 batch %d 缓存 DSL 失败: %w", batchIndex+1, err)
	}
	normalizeBatchChapterIDs(batchDSL, manifest)
	return batchDSL, nil
}

func normalizeBatchChapterIDs(batchDSL *dsl.DSL, manifest *batchManifest) {
	if batchDSL == nil || batchDSL.Storyline == nil || manifest == nil {
		return
	}
	for i := range batchDSL.Storyline.Chapters {
		if i >= len(manifest.Chapters) {
			break
		}
		expectedID := manifest.Chapters[i].ChapterID
		if strings.TrimSpace(expectedID) == "" {
			continue
		}
		batchDSL.Storyline.Chapters[i].ID = expectedID
	}
}

func findCheckNovelChapterInputs(bookPath string, outline *rpg.StoryOutline) ([]chapterDSLInputFile, error) {
	chapterFiles := map[string]string{}
	if outline != nil {
		for _, part := range outline.Parts {
			for _, volume := range part.Volumes {
				for _, chapter := range volume.Chapters {
					for _, path := range candidateCheckNovelChapterPaths(bookPath, chapter.ID) {
						if _, err := os.Stat(path); err == nil {
							chapterFiles[chapter.ID] = path
							break
						}
					}
				}
			}
		}
	}
	if len(chapterFiles) == 0 {
		pattern := filepath.Join(bookPath, "chapters", "chapter-*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			chapterID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "chapter-"), filepath.Ext(file))
			chapterFiles[chapterID] = file
		}
	}

	inputs := make([]chapterDSLInputFile, 0, len(chapterFiles))
	for chapterID, chapterPath := range chapterFiles {
		input := chapterDSLInputFile{
			ChapterID:   chapterID,
			ChapterPath: chapterPath,
		}
		recapPath := filepath.Join(bookPath, "story", "recaps", chapterID+".json")
		if _, err := os.Stat(recapPath); err == nil {
			input.RecapPath = recapPath
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func candidateCheckNovelChapterPaths(bookPath, chapterID string) []string {
	return []string{
		filepath.Join(bookPath, "chapters", "chapter-"+chapterID+".md"),
		filepath.Join(bookPath, "chapters", "chapter-"+extractCheckNovelChapterNumber(chapterID)+".md"),
	}
}

func extractCheckNovelChapterNumber(chapterID string) string {
	parts := strings.Split(chapterID, "-")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		return strings.TrimLeft(last, "Cc")
	}
	parts = strings.Split(chapterID, "_")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return chapterID
}

func ensureParseableDSL(ctx context.Context, agent *agents.ChapterToDSLAgent, bookName, dslContent string) (string, error) {
	parser := dsl.NewParser(dslContent)
	if _, parseErr := parser.Parse(); parseErr != nil {
		fmt.Printf("初次 DSL 解析失败，进入自动修复: %v\n", parseErr)
		lastErr := parseErr
		for attempt := 0; attempt < 2; attempt++ {
			repairedDSL, repairErr := agent.RepairDSL(ctx, bookName, lastErr.Error(), dslContent)
			if repairErr != nil {
				return "", fmt.Errorf("DSL 自动修复失败: %w", repairErr)
			}
			reparse := dsl.NewParser(repairedDSL)
			if _, err := reparse.Parse(); err == nil {
				fmt.Printf("DSL 自动修复成功 (attempt %d/2)\n", attempt+1)
				return repairedDSL, nil
			} else {
				lastErr = err
				dslContent = repairedDSL
				fmt.Printf("修复后仍解析失败 (attempt %d/2): %v\n", attempt+1, err)
			}
		}
		dumpFailedDSL(bookName, dslContent, lastErr)
		return "", fmt.Errorf("DSL 解析失败且修复未成功: %w", lastErr)
	}
	return dslContent, nil
}

func loadChapterDSLInputData(files []chapterDSLInputFile) ([]agents.ChapterData, error) {
	chapters := make([]agents.ChapterData, 0, len(files))
	for _, file := range files {
		content, err := os.ReadFile(file.ChapterPath)
		if err != nil {
			return nil, err
		}
		chapter := agents.ChapterData{
			ChapterID: file.ChapterID,
			Title:     firstMarkdownHeadingForCheckNovel(string(content), file.ChapterID),
			Content:   string(content),
		}
		if file.RecapPath != "" {
			recapData, err := os.ReadFile(file.RecapPath)
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

func firstMarkdownHeadingForCheckNovel(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return fallback
}

func dumpFailedDSL(bookName, dslContent string, parseErr error) {
	dir := filepath.Join("books", bookName, "story", "rpg", "cache", "failed_dsl")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	stamp := time.Now().Format("20060102_150405")
	base := filepath.Join(dir, fmt.Sprintf("failed_%s", stamp))
	_ = os.WriteFile(base+".rpg", []byte(dslContent), 0644)
	_ = os.WriteFile(base+".err.txt", []byte(parseErr.Error()), 0644)
	fmt.Printf("失败 DSL 已保存用于排查: %s.rpg\n", base)
}

func splitChapterBatches(files []chapterDSLInputFile, batchSize int) [][]chapterDSLInputFile {
	var batches [][]chapterDSLInputFile
	for start := 0; start < len(files); start += batchSize {
		end := start + batchSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[start:end])
	}
	return batches
}

func buildBatchManifest(bookName string, batchIndex, batchSize int, chapterFiles []chapterDSLInputFile, cacheDir string) (*batchManifest, error) {
	chapters := make([]batchChapterState, 0, len(chapterFiles))
	var hashInput strings.Builder
	hashInput.WriteString(fmt.Sprintf("version=%d\nbook=%s\nbatch_index=%d\nbatch_size=%d\n", batchCacheVersion, bookName, batchIndex, batchSize))

	for _, file := range chapterFiles {
		chapterSum, err := fileSHA256(file.ChapterPath)
		if err != nil {
			return nil, err
		}
		recapSum := ""
		if file.RecapPath != "" {
			recapSum, err = fileSHA256(file.RecapPath)
			if err != nil {
				return nil, err
			}
		}
		chapters = append(chapters, batchChapterState{
			ChapterID:     file.ChapterID,
			ChapterPath:   filepath.ToSlash(file.ChapterPath),
			ChapterSHA256: chapterSum,
			RecapPath:     filepath.ToSlash(file.RecapPath),
			RecapSHA256:   recapSum,
		})
		hashInput.WriteString(file.ChapterID)
		hashInput.WriteString(".chapter=")
		hashInput.WriteString(chapterSum)
		hashInput.WriteString("\n")
		hashInput.WriteString(file.ChapterID)
		hashInput.WriteString(".recap=")
		hashInput.WriteString(recapSum)
		hashInput.WriteString("\n")
	}

	batchHashBytes := sha256.Sum256([]byte(hashInput.String()))
	batchHash := hex.EncodeToString(batchHashBytes[:])
	first := chapters[0].ChapterID
	last := chapters[len(chapters)-1].ChapterID
	stem := fmt.Sprintf("batch_%03d_%s_%s_%s", batchIndex+1, first, last, batchHash[:12])

	return &batchManifest{
		Version:    batchCacheVersion,
		BookName:   bookName,
		BatchIndex: batchIndex,
		BatchSize:  batchSize,
		BatchHash:  batchHash,
		DSLFile:    filepath.Join(cacheDir, stem+".rpg"),
		Chapters:   chapters,
	}, nil
}

func loadBatchFromCache(expected *batchManifest) (string, bool, error) {
	manifestPath := strings.TrimSuffix(expected.DSLFile, filepath.Ext(expected.DSLFile)) + ".json"
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("读取 batch manifest 失败: %w", err)
	}

	var cached batchManifest
	if err := json.Unmarshal(b, &cached); err != nil {
		return "", false, nil
	}
	if cached.Version != expected.Version ||
		cached.BookName != expected.BookName ||
		cached.BatchIndex != expected.BatchIndex ||
		cached.BatchSize != expected.BatchSize ||
		cached.BatchHash != expected.BatchHash ||
		cached.DSLFile != expected.DSLFile {
		return "", false, nil
	}

	content, err := os.ReadFile(expected.DSLFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("读取 batch DSL 失败: %w", err)
	}
	if _, err := dsl.NewParser(string(content)).Parse(); err != nil {
		return "", false, nil
	}
	return string(content), true, nil
}

func saveBatchToCache(manifest *batchManifest, dslContent string) error {
	if err := os.WriteFile(manifest.DSLFile, []byte(dslContent), 0644); err != nil {
		return fmt.Errorf("保存 batch DSL 失败: %w", err)
	}
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 batch manifest 失败: %w", err)
	}
	manifestPath := strings.TrimSuffix(manifest.DSLFile, filepath.Ext(manifest.DSLFile)) + ".json"
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("保存 batch manifest 失败: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取章节文件失败 %s: %w", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func sortChapterJSONFiles(files []chapterDSLInputFile) {
	sort.Slice(files, func(i, j int) bool {
		a1, b1, c1 := parsePVC(files[i].ChapterID)
		a2, b2, c2 := parsePVC(files[j].ChapterID)
		if a1 != a2 {
			return a1 < a2
		}
		if b1 != b2 {
			return b1 < b2
		}
		if c1 != c2 {
			return c1 < c2
		}
		return files[i].ChapterID < files[j].ChapterID
	})
}

func mergeBatchDSL(dst, src *dsl.DSL) {
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
		dst.Metadata.Genre = appendMissingStrings(dst.Metadata.Genre, src.Metadata.Genre)
	}
	if src.World != nil {
		dst.World.Locations = appendMissingLocations(dst.World.Locations, src.World.Locations)
		dst.World.Items = appendMissingItems(dst.World.Items, src.World.Items)
		dst.World.Rules = append(dst.World.Rules, src.World.Rules...)
	}
	if src.Characters != nil {
		if dst.Characters.Player == nil {
			dst.Characters.Player = src.Characters.Player
		}
		dst.Characters.Enemies = appendMissingEnemies(dst.Characters.Enemies, src.Characters.Enemies)
		dst.Characters.NPCs = appendMissingNPCs(dst.Characters.NPCs, src.Characters.NPCs)
	}
	if src.Storyline != nil {
		dst.Storyline.Arcs = appendMissingArcs(dst.Storyline.Arcs, src.Storyline.Arcs)
		dst.Storyline.Chapters = append(dst.Storyline.Chapters, src.Storyline.Chapters...)
	}
	if src.Systems != nil && dst.Systems == nil {
		dst.Systems = src.Systems
	}
}

func appendMissingStrings(dst, src []string) []string {
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

func appendMissingLocations(dst, src []dsl.Location) []dsl.Location {
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

func appendMissingItems(dst, src []dsl.Item) []dsl.Item {
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

func appendMissingEnemies(dst, src []dsl.Enemy) []dsl.Enemy {
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

func appendMissingNPCs(dst, src []dsl.NPC) []dsl.NPC {
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

func appendMissingArcs(dst, src []dsl.Arc) []dsl.Arc {
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
