// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/fiction/mutation"
	"novelgen/internal/fiction/rpg"
	"novelgen/internal/platform/logger"
)

func runComposePipeline(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE PIPELINE")

	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, false); err != nil {
		return err
	}
	if err := validateComposeAgentApplyOption(composeAgentSDKFlag, composeAgentApplyFlag); err != nil {
		return err
	}

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	projectConfig := session.Config()
	setup, err := session.RequireSetup()
	if err != nil {
		return err
	}

	totalVolumes := projectConfig.Structure.TargetParts * projectConfig.Structure.TargetVolumes
	if totalVolumes <= 0 {
		return fmt.Errorf("project structure must define positive part and volume counts")
	}
	fromVolume := composePipelineFromVol
	toVolume := composePipelineToVol
	if toVolume == 0 {
		toVolume = totalVolumes
	}
	if fromVolume < 1 || toVolume < 1 || fromVolume > toVolume || toVolume > totalVolumes {
		return fmt.Errorf("invalid volume range %d..%d; valid range is 1..%d", fromVolume, toVolume, totalVolumes)
	}
	if composePipelineMaxRounds < 1 && !composePipelineSkipImprove {
		return fmt.Errorf("--max-rounds must be at least 1 unless --skip-improve is set")
	}

	agent, err := session.NewComposeAgent()
	if err != nil {
		return err
	}
	cfg, client, err := session.LoadLLM()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	outlinePath := book.Root().OutlineJSON()
	outline, err := ensureComposePipelineSkeleton(book, ctx, agent, setup, projectConfig, outlinePath, composeAgentSDKFlag)
	if err != nil {
		return err
	}
	if err := saveComposePipelineOutline(ctx, book, outline, outlinePath); err != nil {
		return err
	}
	fmt.Printf("Pipeline range: volumes %d..%d (%d total)\n", fromVolume, toVolume, totalVolumes)

	for globalVolume := fromVolume; globalVolume <= toVolume; globalVolume++ {
		partIdx, volIdx, err := outlineVolumePosition(outline, globalVolume)
		if err != nil {
			return err
		}
		volume := &outline.Parts[partIdx].Volumes[volIdx]
		fmt.Printf("\n=== Pipeline volume %d: %s ===\n", globalVolume, volume.Title)

		if !composePipelineSkipGen {
			if len(volume.Chapters) > 0 && !composePipelineForce {
				fmt.Printf("Generation skipped: volume already has %d chapter(s)\n", len(volume.Chapters))
			} else {
				if composeAgentSDKFlag {
					err = generateComposePipelineVolumeAgentSDK(book, ctx, agent, setup, projectConfig, outline, partIdx, volIdx, globalVolume, totalVolumes)
				} else {
					err = generateComposePipelineVolume(book, ctx, agent, setup, projectConfig, outline, partIdx, volIdx, globalVolume, totalVolumes)
				}
				if err != nil {
					return err
				}
				if err := saveComposePipelineOutline(ctx, book, outline, outlinePath); err != nil {
					return err
				}
				fmt.Printf("Saved after generation: %s\n", outlinePath)
			}
		}

		if !composePipelineSkipImprove {
			if len(volume.Chapters) == 0 {
				return fmt.Errorf("volume %d has no chapters to improve; generate it first or remove --skip-gen", globalVolume)
			}
			improveOutline, err := outlineWithImproveVolumeSelection(outline, globalVolume, 0, 0)
			if err != nil {
				return err
			}
			if composeAgentSDKFlag {
				err = iterateOutlineImprovementAgentSDK(ctx, book, improveOutline, setup, projectConfig, composePipelineMaxRounds, composePipelineForce, composePromptFlag, composeAgentApplyFlag, true, nil, "", outline)
			} else {
				err = iterateOutlineImprovement(ctx, book, improveOutline, setup, projectConfig, cfg, client, composePipelineMaxRounds, 1, true, composePipelineForce, composePromptFlag)
			}
			if err != nil {
				return fmt.Errorf("failed to improve volume %d: %w", globalVolume, err)
			}
			mergeGeneratedVolumes(outline, improveOutline)
			if err := saveComposePipelineOutline(ctx, book, outline, outlinePath); err != nil {
				return err
			}
			fmt.Printf("Saved after improve: %s\n", outlinePath)
		}

		if !composePipelineSkipCross {
			patched, issues, warnings, err := applySetupOutlineCrossPatch(ctx, book, setup, outline)
			if err != nil {
				return err
			}
			if patched > 0 {
				fmt.Printf("Cross setup patch: added %d missing resource(s)\n", patched)
			}
			if len(issues)+len(warnings) > 0 {
				fmt.Printf("Cross check: %d issue(s), %d warning(s)\n", len(issues), len(warnings))
			}
		}
	}

	quality.LogQualityGateResult("outline", quality.RunOutlineQualityGate(setup, outline))
	fmt.Printf("\nPipeline complete. Current outline: %s\n", outlinePath)
	fmt.Printf("Generated volumes: %d, empty future volumes: %d\n", countGeneratedVolumes(outline), countEmptyVolumes(outline))
	return nil
}

func saveComposePipelineOutline(ctx context.Context, book *project.Book, outline *models.Outline, outlinePath string) error {
	if book != nil && outlinePath == book.Root().OutlineJSON() {
		return saveOutlineMutation(ctx, book, outline)
	}
	if err := book.Root().SaveOutlineAt(outlinePath, outline); err != nil {
		return err
	}
	if _, err := book.SaveOutlineMarkdown(outline); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}
	return nil
}

// saveOutlineProgress stores an incremental canonical outline snapshot while
// hierarchical generation is still running. It deliberately skips
// normalization, validation and derived markdown; the final commit uses
// saveOutlineMutation/saveOutlineMutationRaw.
func saveOutlineProgress(ctx context.Context, book *project.Book, outline *models.Outline) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := mutation.NewMutationService(book.Root()).CommitOutlineRaw(ctx, outline, mutation.MutationOptions{
		Source: "compose-progress",
	})
	return err
}

func outlineVolumePosition(outline *models.Outline, globalVolume int) (int, int, error) {
	if outline == nil {
		return 0, 0, fmt.Errorf("outline is nil")
	}
	if globalVolume < 1 {
		return 0, 0, fmt.Errorf("volume index must be positive")
	}
	current := 0
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			current++
			if current == globalVolume {
				return partIdx, volIdx, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("volume %d not found in outline (%d volume(s) available)", globalVolume, current)
}

func applySetupOutlineCrossPatch(ctx context.Context, book *project.Book, setup *models.StorySetup, outline *models.Outline) (int, []string, []string, error) {
	if setup == nil || outline == nil {
		return 0, nil, nil, nil
	}
	checkOutline := outlineWithGeneratedVolumes(outline)
	if countGeneratedVolumes(checkOutline) == 0 {
		checkOutline = outline
	}
	checkData, err := json.Marshal(checkOutline)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to prepare outline for cross check: %w", err)
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(checkData, &storyOutline); err != nil {
		return 0, nil, nil, fmt.Errorf("failed to parse outline for cross check: %w", err)
	}

	issues, warnings := quality.ValidateSetupOutlineCross(setup, &storyOutline)
	missingResources := missingSetupResources(setup, &storyOutline)
	if len(missingResources) == 0 {
		return 0, issues, warnings, nil
	}
	for _, name := range missingResources {
		setup.WorldResources = append(setup.WorldResources, models.WorldResource{
			Name:        name,
			Category:    "general",
			Scarcity:    "rare",
			Description: "Auto-added by compose pipeline cross check; refine manually.",
		})
	}
	if err := saveSetupMutation(ctx, book, setup); err != nil {
		return 0, issues, warnings, fmt.Errorf("failed to save cross-patched setup: %w", err)
	}
	return len(missingResources), issues, warnings, nil
}

func missingSetupResources(setup *models.StorySetup, outline *rpg.StoryOutline) []string {
	if setup == nil || outline == nil {
		return nil
	}
	defined := map[string]bool{}
	for _, resource := range setup.WorldResources {
		name := strings.TrimSpace(resource.Name)
		if name != "" {
			defined[name] = true
		}
	}
	missing := map[string]bool{}
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, entry := range ch.ResourceLedger {
					name := strings.TrimSpace(entry.Item)
					if name != "" && !defined[name] {
						missing[name] = true
					}
				}
			}
		}
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
