package writersdl

import (
	"context"
	"fmt"
	"os"

	"novelgen/internal/agents"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
)

// WriteRPGDSLEmitInput describes one chapter→RPG-DSL emission run with all
// project facts already resolved by the caller. It exists so callers that do not
// run with the novel project as the process working directory — the standalone
// tools — reuse exactly the same batching, per-batch cache and merge pipeline as
// `novelgen write`, instead of carrying their own copy.
type WriteRPGDSLEmitInput struct {
	BookName  string
	CacheDir  string
	Chapters  []WriteRPGDSLChapterInput
	BatchSize int
	Agent     *agents.ChapterToDSLAgent
	// Characters and Locations are the craft element values the conversion
	// agent receives as project facts.
	Characters map[string]models.Character
	Locations  map[string]models.Location
	// ExistingDSLPath is optional. When TargetChapterIDs is set and the file
	// exists, its chapters are kept and only the target chapters are refreshed.
	ExistingDSLPath  string
	TargetChapterIDs []string
}

// EmitWriteRPGDSL converts chapter markdown to DSL in batches, reusing the
// per-batch cache, and returns the merged DSL. It does not write the merged file
// and does not run the simulator: callers own those steps because they may need
// to post-process the DSL first (for example to apply simulation defaults).
func EmitWriteRPGDSL(ctx context.Context, input WriteRPGDSLEmitInput) (WriteRPGDSLResult, *rpgdsl.DSL, error) {
	result := WriteRPGDSLResult{}
	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	chapters := input.Chapters
	targetSet := WriteRPGDSLTargetChapterSet(input.TargetChapterIDs)
	if len(targetSet) > 0 {
		chapters = FilterWriteRPGDSLChapterInputs(chapters, targetSet)
	}
	if len(chapters) == 0 {
		return result, nil, fmt.Errorf("no chapter markdown files to convert for RPG DSL")
	}
	SortRPGDSLChapterInputs(chapters)

	if err := os.MkdirAll(input.CacheDir, 0755); err != nil {
		return result, nil, err
	}

	combined := &rpgdsl.DSL{
		Metadata:   &rpgdsl.Metadata{},
		World:      &rpgdsl.World{},
		Characters: &rpgdsl.Characters{},
		Storyline:  &rpgdsl.Storyline{},
		Systems:    &rpgdsl.Systems{},
	}
	if len(targetSet) > 0 && input.ExistingDSLPath != "" {
		if existing, err := LoadWriteRPGDSLExisting(input.ExistingDSLPath); err == nil {
			combined = existing
			RemoveWriteRPGDSLChapters(combined, targetSet)
		} else if !os.IsNotExist(err) {
			logger.Warn("Ignoring existing chapter RPG DSL during targeted refresh: %v", err)
		}
	}

	for batchIndex, batch := range SplitRPGDSLBatchInputs(chapters, batchSize) {
		manifest, err := BuildWriteRPGDSLBatchManifest(input.BookName, batchIndex*batchSize, batchSize, batch, input.CacheDir)
		if err != nil {
			return result, nil, err
		}
		result.BatchCount++

		dslContent, reused, err := LoadWriteRPGDSLBatchFromCache(manifest)
		if err != nil {
			return result, nil, err
		}
		if reused {
			result.ReusedBatches++
			logger.Info("RPG DSL batch cache reused: %s -> %s (%d chapters)", manifest.Chapters[0].ChapterID, manifest.Chapters[len(manifest.Chapters)-1].ChapterID, len(manifest.Chapters))
		} else {
			result.GeneratedBatches++
			logger.Info("RPG DSL AI conversion batch: %s -> %s (%d chapters)", manifest.Chapters[0].ChapterID, manifest.Chapters[len(manifest.Chapters)-1].ChapterID, len(manifest.Chapters))
			chapterData, err := LoadWriteRPGDSLChapterData(batch)
			if err != nil {
				return result, nil, err
			}
			dslContent, err = input.Agent.ConvertChapterData(ctx, input.BookName, input.Characters, input.Locations, chapterData)
			if err != nil {
				return result, nil, fmt.Errorf("AI chapter -> RPG DSL failed for batch %d: %w", batchIndex+1, err)
			}
			dslContent, err = EnsureWriteRPGDSLParseable(ctx, input.Agent, input.BookName, dslContent)
			if err != nil {
				return result, nil, fmt.Errorf("RPG DSL parse/repair failed for batch %d: %w", batchIndex+1, err)
			}
		}

		parsed, err := rpgdsl.NewParser(dslContent).Parse()
		if err != nil {
			return result, nil, fmt.Errorf("failed to parse RPG DSL batch cache %d: %w", batchIndex+1, err)
		}
		dslContent, parsed, err = EnsureWriteRPGDSLManifestChapters(dslContent, parsed, manifest)
		if err != nil {
			return result, nil, fmt.Errorf("RPG DSL batch %d missing manifest chapters: %w", batchIndex+1, err)
		}
		NormalizeWriteRPGDSLChapterIDs(parsed, manifest)
		EnrichWriteRPGDSLCombatSignals(parsed)
		normalizedContent := parsed.String()
		if !reused || normalizedContent != dslContent {
			dslContent = normalizedContent
			if err := SaveWriteRPGDSLBatchToCache(manifest, dslContent); err != nil {
				return result, nil, err
			}
			logger.Info("RPG DSL batch output: %s", manifest.DSLFile)
		}
		MergeWriteRPGDSL(combined, parsed)
	}

	result.ChapterCount = len(chapters)
	return result, combined, nil
}
