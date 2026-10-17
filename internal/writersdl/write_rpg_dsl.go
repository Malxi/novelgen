package writersdl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
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
	"novelgen/internal/store"
)

const writeRPGDSLCacheVersion = 4

type WriteRPGDSLResult struct {
	DSLPath          string
	CacheDir         string
	ChapterCount     int
	BatchCount       int
	ReusedBatches    int
	GeneratedBatches int
	Issues           []rpgdsl.SimulationIssue // DSL-RPG simulate 检测出的 issues
}

type WriteRPGDSLBatchManifest struct {
	Version    int                       `json:"version"`
	BookName   string                    `json:"book_name"`
	BatchIndex int                       `json:"batch_index"`
	BatchSize  int                       `json:"batch_size"`
	BatchHash  string                    `json:"batch_hash"`
	DSLFile    string                    `json:"dsl_file"`
	UpdatedAt  string                    `json:"updated_at"`
	Chapters   []WriteRPGDSLChapterState `json:"chapters"`
}

type WriteRPGDSLChapterState struct {
	ChapterID     string `json:"chapter_id"`
	ChapterPath   string `json:"chapter_path"`
	ChapterSHA256 string `json:"chapter_sha256"`
	RecapPath     string `json:"recap_path,omitempty"`
	RecapSHA256   string `json:"recap_sha256,omitempty"`
}

type WriteRPGDSLChapterInput struct {
	ChapterID   string
	ChapterPath string
	RecapPath   string
}

func EmitChapterRPGDSLForProject(
	ctx context.Context,
	root *store.Root,
	bookName string,
	projectConfig *models.ProjectConfig,
	llmConfig *llm.Config,
	setup *models.StorySetup,
	client llm.Client,
	batchSize int,
	targetChapterIDs ...string,
) (WriteRPGDSLResult, error) {
	if root == nil {
		return WriteRPGDSLResult{}, fmt.Errorf("project root is required")
	}
	outline, err := root.LoadOutline()
	if err != nil {
		return WriteRPGDSLResult{}, err
	}
	chapterInputs, err := FindProjectChapterInputsAt(root, outline)
	if err != nil {
		return WriteRPGDSLResult{}, err
	}
	targetSet := WriteRPGDSLTargetChapterSet(targetChapterIDs)
	if len(targetSet) > 0 {
		chapterInputs = FilterWriteRPGDSLChapterInputs(chapterInputs, targetSet)
	}
	if len(chapterInputs) == 0 {
		return WriteRPGDSLResult{}, fmt.Errorf("no chapter markdown files found under %s", root.ChaptersDir())
	}

	characters, locations, _, _, err := root.LoadElements()
	if err != nil {
		return WriteRPGDSLResult{}, err
	}

	agent := agents.NewChapterToDSLAgent(client, llmConfig, &projectConfig.LLM, setup)
	agent.SetLanguage(projectConfig.Language)
	agent.SetRequireAI(true)

	cacheDir := root.RPGChapterBatchCacheDir()
	outputPath := root.RPGChaptersDSL()

	result, combined, err := EmitWriteRPGDSL(ctx, WriteRPGDSLEmitInput{
		BookName:         bookName,
		FS:               root.FS(),
		CacheDir:         cacheDir,
		Chapters:         chapterInputs,
		BatchSize:        batchSize,
		Agent:            agent,
		Characters:       DerefCharacterMap(characters),
		Locations:        DerefLocationMap(locations),
		ExistingDSLPath:  outputPath,
		TargetChapterIDs: targetChapterIDs,
	})
	if err != nil {
		return result, err
	}

	if err := root.WriteText(outputPath, combined.String()); err != nil {
		return result, err
	}

	// Run DSL-RPG Simulator to detect issues
	simulator := rpgdsl.NewSimulator(combined)
	result.Issues = simulator.SimulateAll()

	result.DSLPath = outputPath
	result.CacheDir = cacheDir
	return result, nil
}

func FindProjectChapterInputs(projectRoot string, outline *models.Outline) ([]WriteRPGDSLChapterInput, error) {
	return FindProjectChapterInputsAt(store.At(projectRoot), outline)
}

// FindProjectChapterInputsAt is FindProjectChapterInputs for a caller that
// already holds a project root.
func FindProjectChapterInputsAt(book *store.Root, outline *models.Outline) ([]WriteRPGDSLChapterInput, error) {
	chapterFiles := map[string]string{}
	for _, chapter := range AllOutlineChapters(outline) {
		for _, path := range book.ChapterCandidates(chapter.ID) {
			exists, err := book.ExistsErr(path)
			if err != nil {
				return nil, fmt.Errorf("check chapter %s: %w", path, err)
			}
			if exists {
				chapterFiles[chapter.ID] = path
				break
			}
		}
	}

	if len(chapterFiles) == 0 {
		entries, err := book.FS().ReadDir(book.ChaptersDir())
		if err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return nil, err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(name, "chapter-") || !strings.HasSuffix(name, ".md") {
				continue
			}
			chapterID := strings.TrimSuffix(strings.TrimPrefix(name, "chapter-"), ".md")
			chapterFiles[chapterID] = filepath.Join(book.ChaptersDir(), name)
		}
	}

	inputs := make([]WriteRPGDSLChapterInput, 0, len(chapterFiles))
	for chapterID, chapterPath := range chapterFiles {
		input := WriteRPGDSLChapterInput{
			ChapterID:   chapterID,
			ChapterPath: chapterPath,
		}
		recapPath := book.RecapFile(chapterID)
		recapExists, err := book.ExistsErr(recapPath)
		if err != nil {
			return nil, fmt.Errorf("check recap %s: %w", recapPath, err)
		}
		if recapExists {
			if recapIsStale(book.FS(), chapterPath, recapPath) {
				logger.Warn("Skipping stale recap for RPG DSL conversion: %s is older than %s", recapPath, chapterPath)
			} else {
				input.RecapPath = recapPath
			}
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

// recapIsStale reports whether the stored recap is older than the chapter it
// describes. Both files are read through the project's filesystem.
func recapIsStale(fs store.FS, chapterPath, recapPath string) bool {
	chapterInfo, chapterErr := fs.Stat(chapterPath)
	recapInfo, recapErr := fs.Stat(recapPath)
	if chapterErr != nil || recapErr != nil {
		return false
	}
	return recapInfo.ModTime().Before(chapterInfo.ModTime())
}

func WriteRPGDSLTargetChapterSet(chapterIDs []string) map[string]bool {
	out := map[string]bool{}
	for _, chapterID := range chapterIDs {
		chapterID = strings.TrimSpace(chapterID)
		if chapterID != "" {
			out[chapterID] = true
		}
	}
	return out
}

func FilterWriteRPGDSLChapterInputs(inputs []WriteRPGDSLChapterInput, targetSet map[string]bool) []WriteRPGDSLChapterInput {
	if len(targetSet) == 0 {
		return inputs
	}
	out := make([]WriteRPGDSLChapterInput, 0, len(targetSet))
	for _, input := range inputs {
		if targetSet[input.ChapterID] {
			out = append(out, input)
		}
	}
	return out
}

func LoadWriteRPGDSLExisting(fs store.FS, path string) (*rpgdsl.DSL, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed, err := rpgdsl.NewParser(string(data)).Parse()
	if err != nil {
		return nil, err
	}
	if parsed.Metadata == nil {
		parsed.Metadata = &rpgdsl.Metadata{}
	}
	if parsed.World == nil {
		parsed.World = &rpgdsl.World{}
	}
	if parsed.Characters == nil {
		parsed.Characters = &rpgdsl.Characters{}
	}
	if parsed.Storyline == nil {
		parsed.Storyline = &rpgdsl.Storyline{}
	}
	if parsed.Systems == nil {
		parsed.Systems = &rpgdsl.Systems{}
	}
	return parsed, nil
}

func RemoveWriteRPGDSLChapters(dsl *rpgdsl.DSL, targetSet map[string]bool) {
	if dsl == nil || dsl.Storyline == nil || len(targetSet) == 0 {
		return
	}
	chapters := dsl.Storyline.Chapters[:0]
	for _, chapter := range dsl.Storyline.Chapters {
		if !targetSet[chapter.ID] {
			chapters = append(chapters, chapter)
		}
	}
	dsl.Storyline.Chapters = chapters
}

func AllOutlineChapters(outline *models.Outline) []*models.Chapter {
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

// CandidateChapterMarkdownPaths returns the file names a saved chapter may live
// under, in the order callers should try them. The list comes from internal/store
// so the DSL pipeline, the write pipeline and the agent tools all read the same
// file for a chapter.
func CandidateChapterMarkdownPaths(projectRoot, chapterID string) []string {
	return store.At(projectRoot).ChapterCandidates(chapterID)
}

func EnsureWriteRPGDSLParseable(ctx context.Context, agent *agents.ChapterToDSLAgent, bookName, content string) (string, error) {
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
		DumpWriteRPGDSLFailure(bookName, content, lastErr)
		return "", lastErr
	}
}

func LoadWriteRPGDSLChapterData(fs store.FS, inputs []WriteRPGDSLChapterInput) ([]agents.ChapterData, error) {
	chapters := make([]agents.ChapterData, 0, len(inputs))
	for _, input := range inputs {
		content, err := fs.ReadFile(input.ChapterPath)
		if err != nil {
			return nil, err
		}
		chapter := agents.ChapterData{
			ChapterID: input.ChapterID,
			Title:     FirstMarkdownHeadingForWriteRPGDSL(string(content), input.ChapterID),
			Content:   string(content),
		}
		if input.RecapPath != "" {
			recapData, err := fs.ReadFile(input.RecapPath)
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

func FirstMarkdownHeadingForWriteRPGDSL(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return fallback
}

func DumpWriteRPGDSLFailure(bookName, content string, parseErr error) {
	// Prefer the project the pipeline is actually working in; the books/<name>
	// workspace layout is only the standalone-tool fallback.
	root, err := store.Default()
	if err != nil {
		root, err = store.Open(store.BookDir("", bookName))
		if err != nil {
			return
		}
	}
	if projectDir := strings.TrimSpace(logger.Default().ProjectDir()); projectDir != "" {
		if projectRoot, openErr := store.Open(projectDir); openErr == nil {
			root = projectRoot
		}
	}
	dir := root.RPGFile(filepath.Join("cache", "failed_dsl"))
	if err := root.MkdirAll(dir, store.DirMode); err != nil {
		return
	}
	stem := filepath.Join(dir, "failed_"+time.Now().Format("20060102_150405"))
	_ = root.WriteText(stem+".rpg", content)
	_ = root.WriteText(stem+".err.txt", parseErr.Error())
	logger.Error("Failed RPG DSL dump: %s.rpg", stem)
}

func BuildWriteRPGDSLBatchManifest(fs store.FS, bookName string, batchIndex, batchSize int, chapterInputs []WriteRPGDSLChapterInput, cacheDir string) (*WriteRPGDSLBatchManifest, error) {
	chapters := make([]WriteRPGDSLChapterState, 0, len(chapterInputs))
	var hashInput strings.Builder
	hashInput.WriteString(fmt.Sprintf("version=%d\nbook=%s\nbatch_index=%d\nbatch_size=%d\n", writeRPGDSLCacheVersion, bookName, batchIndex, batchSize))
	for _, input := range chapterInputs {
		chapterSum, err := FileSHA256ForWriteRPGDSL(fs, input.ChapterPath)
		if err != nil {
			return nil, err
		}
		recapSum := ""
		if input.RecapPath != "" {
			recapSum, err = FileSHA256ForWriteRPGDSL(fs, input.RecapPath)
			if err != nil {
				return nil, err
			}
		}
		chapters = append(chapters, WriteRPGDSLChapterState{
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
	return &WriteRPGDSLBatchManifest{
		Version:    writeRPGDSLCacheVersion,
		BookName:   bookName,
		BatchIndex: batchIndex,
		BatchSize:  batchSize,
		BatchHash:  batchHash,
		DSLFile:    filepath.Join(cacheDir, stem+".rpg"),
		Chapters:   chapters,
	}, nil
}

func LoadWriteRPGDSLBatchFromCache(fs store.FS, expected *WriteRPGDSLBatchManifest) (string, bool, error) {
	manifestPath := strings.TrimSuffix(expected.DSLFile, filepath.Ext(expected.DSLFile)) + ".json"
	data, err := fs.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	var cached WriteRPGDSLBatchManifest
	if err := json.Unmarshal(data, &cached); err != nil {
		return "", false, nil
	}
	if cached.Version != expected.Version || cached.BookName != expected.BookName || cached.BatchIndex != expected.BatchIndex || cached.BatchSize != expected.BatchSize || cached.BatchHash != expected.BatchHash || cached.DSLFile != expected.DSLFile {
		return "", false, nil
	}
	content, err := fs.ReadFile(expected.DSLFile)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if _, err := rpgdsl.NewParser(string(content)).Parse(); err != nil {
		return "", false, nil
	}
	return string(content), true, nil
}

// SaveWriteRPGDSLBatchToCache stores one converted batch plus its manifest. It
// writes through the caller's filesystem (the cache and the chapter files must
// live on the same one), which the FS-less version of this function quietly did
// not: it always went to the host filesystem.
func SaveWriteRPGDSLBatchToCache(fs store.FS, manifest *WriteRPGDSLBatchManifest, content string) error {
	if fs == nil {
		fs = store.OSFS{}
	}
	cache := store.OpenFS(filepath.Dir(manifest.DSLFile), fs)
	if err := cache.WriteText(manifest.DSLFile, content); err != nil {
		return err
	}
	manifest.UpdatedAt = time.Now().Format(time.RFC3339)
	manifestPath := strings.TrimSuffix(manifest.DSLFile, filepath.Ext(manifest.DSLFile)) + ".json"
	return cache.WriteJSON(manifestPath, manifest)
}

func NormalizeWriteRPGDSLChapterIDs(parsed *rpgdsl.DSL, manifest *WriteRPGDSLBatchManifest) {
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

func EnsureWriteRPGDSLManifestChapters(content string, parsed *rpgdsl.DSL, manifest *WriteRPGDSLBatchManifest) (string, *rpgdsl.DSL, error) {
	if manifest == nil || len(manifest.Chapters) == 0 {
		return content, parsed, nil
	}
	if parsed == nil || parsed.Storyline == nil {
		return content, parsed, fmt.Errorf("storyline block is missing")
	}
	if len(parsed.Storyline.Chapters) >= len(manifest.Chapters) {
		return content, parsed, nil
	}
	if len(manifest.Chapters) != 1 || len(parsed.Storyline.Chapters) != 0 {
		return content, parsed, fmt.Errorf("got %d chapter blocks, want %d", len(parsed.Storyline.Chapters), len(manifest.Chapters))
	}
	wrapped, ok := WrapBareWriteRPGDSLStorylineObjectives(content, manifest.Chapters[0].ChapterID, WriteRPGDSLManifestChapterTitle(parsed, manifest))
	if !ok {
		return content, parsed, fmt.Errorf("got 0 chapter blocks for %s and could not wrap bare storyline objectives", manifest.Chapters[0].ChapterID)
	}
	reparsed, err := rpgdsl.NewParser(wrapped).Parse()
	if err != nil {
		return content, parsed, fmt.Errorf("wrapped bare storyline objectives but parse failed: %w", err)
	}
	if reparsed.Storyline == nil || len(reparsed.Storyline.Chapters) == 0 {
		return content, parsed, fmt.Errorf("wrapped bare storyline objectives still produced no chapter blocks")
	}
	return wrapped, reparsed, nil
}

func WriteRPGDSLManifestChapterTitle(parsed *rpgdsl.DSL, manifest *WriteRPGDSLBatchManifest) string {
	if parsed != nil && parsed.Metadata != nil && strings.TrimSpace(parsed.Metadata.Title) != "" {
		return strings.TrimSpace(parsed.Metadata.Title)
	}
	if manifest != nil && len(manifest.Chapters) > 0 {
		return manifest.Chapters[0].ChapterID
	}
	return "chapter"
}

func WrapBareWriteRPGDSLStorylineObjectives(content, chapterID, title string) (string, bool) {
	start, openBrace, closeBrace, ok := FindWriteRPGDSLStorylineBlock(content)
	if !ok {
		return content, false
	}
	inner := strings.TrimSpace(content[openBrace+1 : closeBrace])
	if inner == "" || strings.Contains(strings.ToLower(inner), "chapter ") {
		return content, false
	}
	if !strings.Contains(strings.ToLower(inner), "objective ") {
		return content, false
	}
	var replacement strings.Builder
	replacement.WriteString(content[start : openBrace+1])
	replacement.WriteString("\n  chapter ")
	replacement.WriteString(strconv.Quote(title))
	replacement.WriteString(" {\n    id = ")
	replacement.WriteString(strconv.Quote(chapterID))
	replacement.WriteString("\n")
	replacement.WriteString(IndentWriteRPGDSLLines(inner, "    "))
	replacement.WriteString("\n  }\n")
	replacement.WriteString(content[closeBrace : closeBrace+1])
	return content[:start] + replacement.String() + content[closeBrace+1:], true
}

func FindWriteRPGDSLStorylineBlock(content string) (start int, openBrace int, closeBrace int, ok bool) {
	lower := strings.ToLower(content)
	start = strings.Index(lower, "storyline")
	if start < 0 {
		return 0, 0, 0, false
	}
	openBrace = strings.Index(content[start:], "{")
	if openBrace < 0 {
		return 0, 0, 0, false
	}
	openBrace += start
	depth := 0
	inString := false
	escaped := false
	for i := openBrace; i < len(content); i++ {
		ch := content[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return start, openBrace, i, true
			}
		}
	}
	return 0, 0, 0, false
}

func IndentWriteRPGDSLLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func EnrichWriteRPGDSLCombatSignals(parsed *rpgdsl.DSL) {
	if parsed == nil || parsed.Storyline == nil {
		return
	}
	protagonistTarget := "protagonist"
	if parsed.Characters != nil && parsed.Characters.Player != nil && strings.TrimSpace(parsed.Characters.Player.ID) != "" {
		protagonistTarget = strings.TrimSpace(parsed.Characters.Player.ID)
	}
	for ci := range parsed.Storyline.Chapters {
		chapter := &parsed.Storyline.Chapters[ci]
		for oi := range chapter.Objectives {
			objective := &chapter.Objectives[oi]
			for si := range objective.Steps {
				step := &objective.Steps[si]
				EnrichWriteRPGDSLStepCombatSignals(step, protagonistTarget)
			}
		}
	}
}

func EnrichWriteRPGDSLStepCombatSignals(step *rpgdsl.Step, protagonistTarget string) {
	if step == nil {
		return
	}
	original := append([]rpgdsl.StateDelta(nil), step.Event.StateDeltas...)
	for _, delta := range original {
		if WriteRPGDSLIsMechEquipmentSignal(delta, step.Description) {
			target := WriteRPGDSLProtagonistSignalTarget(delta, protagonistTarget)
			form := WriteRPGDSLSignalValue(delta, step.Description)
			if !WriteRPGDSLHasDelta(step.Event.StateDeltas, target, "mech", "form") {
				step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
					Target: target,
					Kind:   "mech",
					Field:  "form",
					To:     form,
					Note:   "normalized from equipment signal",
				})
			}
			if !WriteRPGDSLHasDelta(step.Event.StateDeltas, target, "mech", "energy") {
				step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
					Target: target,
					Kind:   "mech",
					Field:  "energy",
					To:     "80",
					Note:   "normalized initial equipment energy",
				})
			}
		}
		if WriteRPGDSLIsGeneLevelSignal(delta, step.Description) {
			target := WriteRPGDSLProtagonistSignalTarget(delta, protagonistTarget)
			if !WriteRPGDSLHasDelta(step.Event.StateDeltas, target, "gene", "level") {
				step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
					Target: target,
					Kind:   "gene",
					Field:  "level",
					From:   delta.From,
					To:     WriteRPGDSLFirstNonEmpty(delta.To, WriteRPGDSLLastInt(step.Description), strconv.Itoa(delta.Delta), "1"),
					Note:   "normalized from progression signal",
				})
			}
		}
		if WriteRPGDSLIsGeneStabilitySignal(delta, step.Description) {
			target := WriteRPGDSLProtagonistSignalTarget(delta, protagonistTarget)
			if !WriteRPGDSLHasDelta(step.Event.StateDeltas, target, "gene", "stability") {
				step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
					Target: target,
					Kind:   "gene",
					Field:  "stability",
					To:     WriteRPGDSLStabilityValue(delta, step.Description),
					Note:   "normalized from stability signal",
				})
			}
		}
	}
	if WriteRPGDSLTextHasGeneLevelSignal(step.Description) && !WriteRPGDSLHasAnyDelta(step.Event.StateDeltas, "gene", "level") {
		step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
			Target: protagonistTarget,
			Kind:   "gene",
			Field:  "level",
			To:     WriteRPGDSLFirstNonEmpty(WriteRPGDSLLastInt(step.Description), "1"),
			Note:   "normalized from step description",
		})
	}
	if WriteRPGDSLTextHasGeneStabilitySignal(step.Description) && !WriteRPGDSLHasAnyDelta(step.Event.StateDeltas, "gene", "stability") {
		step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
			Target: protagonistTarget,
			Kind:   "gene",
			Field:  "stability",
			To:     WriteRPGDSLFirstNonEmpty(WriteRPGDSLLastInt(step.Description), "60"),
			Note:   "normalized from step description",
		})
	}
	if WriteRPGDSLTextHasMechSignal(step.Description) && !WriteRPGDSLHasAnyDelta(step.Event.StateDeltas, "mech", "form") {
		step.Event.StateDeltas = append(step.Event.StateDeltas, rpgdsl.StateDelta{
			Target: protagonistTarget,
			Kind:   "mech",
			Field:  "form",
			To:     step.Description,
			Note:   "normalized from step description",
		})
	}
}

func WriteRPGDSLIsMechEquipmentSignal(delta rpgdsl.StateDelta, stepDescription string) bool {
	kind := strings.ToLower(strings.TrimSpace(delta.Kind))
	field := strings.ToLower(strings.TrimSpace(delta.Field))
	if kind != "item" && kind != "equipment" && kind != "resource" {
		return false
	}
	if field == "equipment" {
		return true
	}
	return WriteRPGDSLTextHasMechSignal(strings.Join([]string{delta.Field, delta.To, delta.Note, stepDescription}, " "))
}

func WriteRPGDSLIsGeneLevelSignal(delta rpgdsl.StateDelta, stepDescription string) bool {
	kind := strings.ToLower(strings.TrimSpace(delta.Kind))
	field := strings.ToLower(strings.TrimSpace(delta.Field))
	if kind != "breakthrough" && kind != "evolution" && kind != "cultivation" && kind != "power_change" {
		return false
	}
	return field == "gene_level" ||
		field == "gene" ||
		strings.Contains(field, "gene") ||
		WriteRPGDSLTextHasGeneLevelSignal(strings.Join([]string{delta.Field, delta.To, delta.Note, stepDescription}, " "))
}

func WriteRPGDSLIsGeneStabilitySignal(delta rpgdsl.StateDelta, stepDescription string) bool {
	field := strings.ToLower(strings.TrimSpace(delta.Field))
	return field == "stability" ||
		field == "gene_stability" ||
		WriteRPGDSLTextHasGeneStabilitySignal(strings.Join([]string{delta.Field, delta.To, delta.Note, stepDescription}, " "))
}

func WriteRPGDSLTextHasMechSignal(text string) bool {
	text = strings.ToLower(text)
	for _, term := range []string{
		"mech", "armor", "exoskeleton",
		"\u673a\u7532",
		"\u88c5\u7532",
		"\u5916\u9aa8\u9abc",
		"\u706b\u79cd",
	} {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func WriteRPGDSLTextHasGeneLevelSignal(text string) bool {
	text = strings.ToLower(text)
	for _, term := range []string{
		"gene",
		"\u57fa\u56e0",
		"\u9002\u914d",
		"\u89c9\u9192",
	} {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func WriteRPGDSLTextHasGeneStabilitySignal(text string) bool {
	text = strings.ToLower(text)
	for _, term := range []string{
		"gene stability", "stability",
		"\u57fa\u56e0\u7a33\u5b9a",
		"\u7a33\u5b9a\u6027",
		"\u7a33\u5b9a",
	} {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func WriteRPGDSLProtagonistSignalTarget(delta rpgdsl.StateDelta, protagonistTarget string) string {
	target := strings.TrimSpace(delta.Target)
	if target == "" || strings.HasPrefix(strings.ToLower(target), "resource_") {
		return protagonistTarget
	}
	return target
}

func WriteRPGDSLHasAnyDelta(deltas []rpgdsl.StateDelta, kind, field string) bool {
	for _, delta := range deltas {
		if strings.EqualFold(strings.TrimSpace(delta.Kind), kind) &&
			strings.EqualFold(strings.TrimSpace(delta.Field), field) {
			return true
		}
	}
	return false
}

func WriteRPGDSLHasDelta(deltas []rpgdsl.StateDelta, target, kind, field string) bool {
	for _, delta := range deltas {
		if strings.EqualFold(strings.TrimSpace(delta.Target), strings.TrimSpace(target)) &&
			strings.EqualFold(strings.TrimSpace(delta.Kind), kind) &&
			strings.EqualFold(strings.TrimSpace(delta.Field), field) {
			return true
		}
	}
	return false
}

func WriteRPGDSLSignalValue(delta rpgdsl.StateDelta, fallback string) string {
	value := strings.TrimSpace(delta.To)
	if value == "" || strings.EqualFold(value, "true") || strings.EqualFold(value, "acquired") {
		value = strings.TrimSpace(delta.Note)
	}
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		value = strings.TrimSpace(delta.Field)
	}
	if value == "" {
		value = "equipment"
	}
	return value
}

func WriteRPGDSLStabilityValue(delta rpgdsl.StateDelta, fallback string) string {
	if value := WriteRPGDSLLastInt(delta.To + " " + delta.Note); value != "" {
		return value
	}
	if value := WriteRPGDSLLastInt(fallback); value != "" {
		return value
	}
	if delta.Delta != 0 {
		return strconv.Itoa(delta.Delta)
	}
	return WriteRPGDSLSignalValue(delta, fallback)
}

func WriteRPGDSLLastInt(text string) string {
	matches := regexp.MustCompile(`\d+`).FindAllString(text, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func WriteRPGDSLFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "0" {
			return value
		}
	}
	return ""
}

func SplitRPGDSLBatchInputs(inputs []WriteRPGDSLChapterInput, batchSize int) [][]WriteRPGDSLChapterInput {
	var batches [][]WriteRPGDSLChapterInput
	for start := 0; start < len(inputs); start += batchSize {
		end := start + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batches = append(batches, inputs[start:end])
	}
	return batches
}

func SortRPGDSLChapterInputs(inputs []WriteRPGDSLChapterInput) {
	sort.Slice(inputs, func(i, j int) bool {
		a1, b1, c1 := ParseRPGDSLChapterID(inputs[i].ChapterID)
		a2, b2, c2 := ParseRPGDSLChapterID(inputs[j].ChapterID)
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

func ParseRPGDSLChapterID(chapterID string) (int, int, int) {
	match := regexp.MustCompile(`P(\d+)-V(\d+)-C(\d+)`).FindStringSubmatch(chapterID)
	if len(match) != 4 {
		return 9999, 9999, 9999
	}
	p, _ := strconv.Atoi(match[1])
	v, _ := strconv.Atoi(match[2])
	c, _ := strconv.Atoi(match[3])
	return p, v, c
}

func FileSHA256ForWriteRPGDSL(fs store.FS, path string) (string, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func DerefCharacterMap(values map[string]*models.Character) map[string]models.Character {
	out := make(map[string]models.Character, len(values))
	for key, value := range values {
		if value != nil {
			out[key] = *value
		}
	}
	return out
}

func DerefLocationMap(values map[string]*models.Location) map[string]models.Location {
	out := make(map[string]models.Location, len(values))
	for key, value := range values {
		if value != nil {
			out[key] = *value
		}
	}
	return out
}

func MergeWriteRPGDSL(dst, src *rpgdsl.DSL) {
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
		dst.Metadata.Genre = AppendMissingWriteRPGDSLStrings(dst.Metadata.Genre, src.Metadata.Genre)
	}
	if src.World != nil {
		dst.World.Locations = AppendMissingWriteRPGDSLLocations(dst.World.Locations, src.World.Locations)
		dst.World.Items = AppendMissingWriteRPGDSLItems(dst.World.Items, src.World.Items)
		dst.World.Rules = append(dst.World.Rules, src.World.Rules...)
	}
	if src.Characters != nil {
		if dst.Characters.Player == nil {
			dst.Characters.Player = src.Characters.Player
		}
		dst.Characters.Enemies = AppendMissingWriteRPGDSLEnemies(dst.Characters.Enemies, src.Characters.Enemies)
		dst.Characters.NPCs = AppendMissingWriteRPGDSLNPCs(dst.Characters.NPCs, src.Characters.NPCs)
	}
	if src.Storyline != nil {
		dst.Storyline.Arcs = AppendMissingWriteRPGDSLArcs(dst.Storyline.Arcs, src.Storyline.Arcs)
		dst.Storyline.Chapters = append(dst.Storyline.Chapters, src.Storyline.Chapters...)
	}
	if src.Systems != nil && dst.Systems == nil {
		dst.Systems = src.Systems
	}
}

func AppendMissingWriteRPGDSLStrings(dst, src []string) []string {
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

func AppendMissingWriteRPGDSLLocations(dst, src []rpgdsl.Location) []rpgdsl.Location {
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

func AppendMissingWriteRPGDSLItems(dst, src []rpgdsl.Item) []rpgdsl.Item {
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

func AppendMissingWriteRPGDSLEnemies(dst, src []rpgdsl.Enemy) []rpgdsl.Enemy {
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

func AppendMissingWriteRPGDSLNPCs(dst, src []rpgdsl.NPC) []rpgdsl.NPC {
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

func AppendMissingWriteRPGDSLArcs(dst, src []rpgdsl.Arc) []rpgdsl.Arc {
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
