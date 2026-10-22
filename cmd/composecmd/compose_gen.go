// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/cli"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
)

func runComposeGen(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE GEN")
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	projectConfig := session.Config()
	cfg, client, err := session.LoadLLM()
	if err != nil {
		return err
	}

	// Check if story_setup.json exists
	setupPath := book.Root().SetupJSON()
	if !book.Root().HasSetup() {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := session.RequireSetup()
	if err != nil {
		return err
	}

	// Check if outline already exists. Hierarchical generation can resume from
	// an incomplete outline.json whose later volumes still have empty chapters.
	outlinePath := book.Root().OutlineJSON()
	if book.Root().HasOutline() {
		if !composeForceGenFlag && !composeOneShotFlag && canResumeOutlineGeneration(book.Root(), outlinePath, projectConfig.Structure) {
			logger.Info("Found incomplete outline at %s; resuming chapter generation", outlinePath)
		} else if !composeForceGenFlag {
			return fmt.Errorf("outline already exists at %s. Use 'novelgen compose regen' to regenerate, 'novelgen compose improve' to improve, or add --force to regenerate with backup", outlinePath)
		} else {

			// Backup existing outline
			logger.Info("Outline exists, creating backup...")
			backupPath, err := book.Root().BackupFile(store.ScopeCompose, outlinePath, "outline")
			if err != nil {
				return fmt.Errorf("failed to backup outline: %w", err)
			}

			// Also backup markdown if exists
			_, _ = book.Root().BackupFile(store.ScopeCompose, book.Root().OutlineMarkdown(), "outline")

			logger.Info("Outline backed up to: %s", backupPath)
			fmt.Printf("\n📦 Existing outline backed up to: %s\n\n", backupPath)
		}
	}

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

	// AI generation mode. Default to hierarchical generation because it saves
	// progress per volume and avoids asking the model to emit one huge outline.
	var outline *models.Outline
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	if composeAgentSDKFlag {
		logger.Info("Using Claude Agent SDK hierarchical generation mode")
		outline, err = generateOutlineHierarchicalAgentSDK(cmd.Context(), book, setup, projectConfig, cfg, client)
	} else if useHierarchical {
		logger.Info("Using hierarchical generation mode (better quality)")
		outline, err = generateOutlineHierarchical(cmd.Context(), book, setup, projectConfig, cfg, client)
	} else {
		logger.Info("Using one-shot generation mode (faster, less stable for long outlines)")
		outline, err = generateOutlineWithAI(setup, projectConfig, cfg, client)
		if err != nil {
			logger.Warn("One-shot outline generation failed; falling back to hierarchical generation: %v", err)
			outline, err = generateOutlineHierarchical(cmd.Context(), book, setup, projectConfig, cfg, client)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to generate outline with AI: %w", err)
	}

	var gate quality.QualityGateResult
	if composeAgentSDKFlag {
		outline, gate, err = repairOutlineWithQualityGateAgentSDK(book, cmd.Context(), outline, setup, projectConfig, cfg, client, "generation", false, false, "", nil)
	} else {
		outline, gate, err = repairOutlineWithQualityGate(book, cmd.Context(), outline, setup, projectConfig, cfg, client, "generation", false)
	}
	if err != nil {
		return err
	}
	quality.LogQualityGateResult("outline", gate)

	// Save outline
	if err := saveOutlineMutation(cmd.Context(), book, outline); err != nil {
		return fmt.Errorf("failed to save outline: %w", err)
	}

	// Print summary
	fmt.Printf("\n[ok] Story outline saved to %s\n", outlinePath)
	fmt.Printf("\n📊 Story Structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen compose improve' to improve the outline with AI review")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func runComposeRegen(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE REGEN")

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	projectConfig := session.Config()
	cfg, client, err := session.LoadLLM()
	if err != nil {
		return err
	}

	// Check if story_setup.json exists
	setupPath := book.Root().SetupJSON()
	if !book.Root().HasSetup() {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := session.RequireSetup()
	if err != nil {
		return err
	}

	// Load existing outline
	outlinePath := book.Root().OutlineJSON()
	outline, err := session.RequireOutline()
	if err != nil {
		return err
	}

	// Regenerate specific element
	if err := regenerateElement(outline, id, setup, projectConfig, cfg, client); err != nil {
		return fmt.Errorf("failed to regenerate element: %w", err)
	}

	// Save outline
	if err := saveOutlineMutation(cmd.Context(), book, outline); err != nil {
		return fmt.Errorf("failed to save outline: %w", err)
	}

	fmt.Printf("\n[ok] Outline regenerated and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func ensureComposePipelineSkeleton(book *project.Book, ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outlinePath string, useAgentSDK bool) (*models.Outline, error) {
	if book.Root().Exists(outlinePath) {
		outline, err := book.Root().LoadOutlineAt(outlinePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing outline: %w", err)
		}
		applyOutlineNormalization(book, outline, "pipeline_skeleton")
		return outline, nil
	}

	legacyProgressPath := book.Root().ComposeFile("outline_progress.json")
	if book.Root().Exists(legacyProgressPath) {
		outline, err := loadPartialOutline(book.Root(), legacyProgressPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load legacy outline progress: %w", err)
		}
		applyOutlineNormalization(book, outline, "pipeline_skeleton")
		return outline, nil
	}

	fmt.Println("No outline.json found; generating compose skeleton")
	skeletonInput := agents.ComposeSkeletonInput{
		Setup:     *setup,
		Structure: projectConfig.Structure,
	}
	var skeletonOutput agents.ComposeSkeletonOutput
	var err error
	if useAgentSDK {
		skeletonOutput, err = agent.GenerateSkeletonWithAgentSDK(ctx, skeletonInput)
	} else {
		skeletonOutput, err = agent.GenerateSkeleton(ctx, skeletonInput)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate skeleton: %w", err)
	}
	outline := &models.Outline{Parts: skeletonOutput.Parts}
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(book, outline, "pipeline_skeleton")
	return outline, nil
}

func generateComposePipelineVolume(book *project.Book, ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outline *models.Outline, partIdx, volIdx, globalVolume, totalVolumes int) error {
	if outline == nil || setup == nil || projectConfig == nil {
		return fmt.Errorf("pipeline generation requires setup, project config, and outline")
	}
	volume := &outline.Parts[partIdx].Volumes[volIdx]
	var outlineContext string
	if partIdx > 0 || volIdx > 0 {
		outlineContext = agent.BuildHierarchicalContext(outline, partIdx, volIdx)
	}

	var previousVolume *models.Volume
	if volIdx > 0 {
		previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
	} else if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		if len(prevPart.Volumes) > 0 {
			previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
		}
	}

	output, err := agent.GenerateChaptersForVolume(ctx, agents.ComposeChaptersInput{
		Setup:          *setup,
		Part:           outline.Parts[partIdx],
		Volume:         *volume,
		VolumeIndex:    globalVolume,
		TotalVolumes:   totalVolumes,
		ChaptersPerVol: projectConfig.Structure.TargetChapters,
		PreviousVolume: previousVolume,
		OutlineContext: outlineContext,
	})
	if err != nil {
		return fmt.Errorf("failed to generate chapters for volume %d: %w", globalVolume, err)
	}

	volume.Chapters = output.Chapters
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(book, outline, fmt.Sprintf("pipeline_gen_v%02d", globalVolume))
	return nil
}

func generateComposePipelineVolumeAgentSDK(book *project.Book, ctx context.Context, agent *agents.ComposeAgent, setup *models.StorySetup, projectConfig *models.ProjectConfig, outline *models.Outline, partIdx, volIdx, globalVolume, totalVolumes int) error {
	if outline == nil || setup == nil || projectConfig == nil {
		return fmt.Errorf("pipeline generation requires setup, project config, and outline")
	}
	volume := &outline.Parts[partIdx].Volumes[volIdx]
	var outlineContext string
	if partIdx > 0 || volIdx > 0 {
		outlineContext = agent.BuildHierarchicalContext(outline, partIdx, volIdx)
	}

	var previousVolume *models.Volume
	if volIdx > 0 {
		previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
	} else if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		if len(prevPart.Volumes) > 0 {
			previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
		}
	}

	output, err := agent.GenerateChaptersForVolumeWithAgentSDK(ctx, agents.ComposeChaptersInput{
		Setup:          *setup,
		Part:           outline.Parts[partIdx],
		Volume:         *volume,
		VolumeIndex:    globalVolume,
		TotalVolumes:   totalVolumes,
		ChaptersPerVol: projectConfig.Structure.TargetChapters,
		PreviousVolume: previousVolume,
		OutlineContext: outlineContext,
	})
	if err != nil {
		return fmt.Errorf("failed to generate chapters for volume %d: %w", globalVolume, err)
	}

	outline.Parts[partIdx].Volumes[volIdx] = output.Volume
	logic.NewIDManager(outline).AssignIDsToOutline()
	applyOutlineNormalization(book, outline, fmt.Sprintf("pipeline_agent_sdk_gen_v%02d", globalVolume))
	return nil
}

func generateOutlineWithAI(setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client) (*models.Outline, error) {
	// Get active provider and model
	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Story structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println()

	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	ctx := cli.CommandContext()
	input := agents.ComposeGenInput{
		Setup:     *setup,
		Structure: projectConfig.Structure,
	}
	output, err := agent.Generate(ctx, input)
	if err != nil {
		return nil, err
	}
	return &output.Outline, nil
}

// generateOutlineHierarchical generates outline using hierarchical approach
// Supports incremental save and resume
func generateOutlineHierarchical(ctx context.Context, book *project.Book, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client) (*models.Outline, error) {
	// Get active provider and model
	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Story structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println()

	outlinePath := book.Root().OutlineJSON()
	legacyProgressPath := book.Root().ComposeFile("outline_progress.json")

	var outline *models.Outline
	var resumeMode bool
	var err error

	if book.Root().Exists(outlinePath) && !composeForceGenFlag {
		fmt.Println("📂 Found existing outline.json. Resuming empty-volume chapter generation...")
		outline, err = loadPartialOutline(book.Root(), outlinePath)
		if err != nil {
			fmt.Printf("⚠️  Failed to load outline: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			fmt.Println("[ok] Resumed from outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	} else if book.Root().Exists(legacyProgressPath) && !composeForceGenFlag {
		fmt.Println("📂 Found legacy outline_progress.json. Migrating to outline.json and resuming...")
		outline, err = loadPartialOutline(book.Root(), legacyProgressPath)
		if err != nil {
			fmt.Printf("⚠️  Failed to load legacy progress: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			if err := saveOutlineProgress(ctx, book, outline); err != nil {
				return nil, fmt.Errorf("failed to migrate legacy progress to outline.json: %w", err)
			}
			fmt.Println("[ok] Migrated legacy progress to outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	}

	// Create agent
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())

	// Save callback for incremental saves (used in both fresh and resume modes)
	onVolumeComplete := func(o *models.Outline, partIdx, volIdx, volumeCount int) {
		if err := saveOutlineProgress(ctx, book, o); err != nil {
			logger.GetLogger().Warn("Failed to save outline progress: %v", err)
		} else {
			fmt.Printf("💾 outline.json saved (%d volumes completed)\n", volumeCount)
		}
	}

	if !resumeMode {
		// Fresh generation
		fmt.Println("Using hierarchical generation:")
		fmt.Println("  Phase 1: Generate skeleton (parts and volumes)")
		fmt.Printf("  Phase 2: Generate chapters for each of %d volumes\n",
			projectConfig.Structure.TargetParts*projectConfig.Structure.TargetVolumes)
		fmt.Println()

		// Generate skeleton first
		skeletonInput := agents.ComposeSkeletonInput{
			Setup:     *setup,
			Structure: projectConfig.Structure,
		}
		skeletonOutput, err := agent.GenerateSkeleton(ctx, skeletonInput)
		if err != nil {
			return nil, fmt.Errorf("failed to generate skeleton: %w", err)
		}

		outline = &models.Outline{
			Parts: skeletonOutput.Parts,
		}
		logic.NewIDManager(outline).AssignIDsToOutline()
		applyOutlineNormalization(book, outline, "fresh_skeleton")

		// Save skeleton as initial outline with empty chapter arrays.
		if err := saveOutlineProgress(ctx, book, outline); err != nil {
			logger.GetLogger().Warn("Failed to save initial outline: %v", err)
		}
		fmt.Println("💾 Skeleton saved to outline.json")

		// Generate chapters with incremental outline.json saves.
		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save outline even on error
			if saveErr := saveOutlineProgress(ctx, book, outline); saveErr != nil {
				logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
			}
			return nil, err
		}

		_ = book.Root().Remove(legacyProgressPath)
		fmt.Println("\n[ok] Generation complete! outline.json is the canonical state.")
	} else {
		// Resume generation - continue from where we left off
		fmt.Println("Continuing chapter generation...")
		fmt.Println()

		outline, err = agent.GenerateChaptersHierarchical(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
		if err != nil {
			// Save outline even on error
			if saveErr := saveOutlineProgress(ctx, book, outline); saveErr != nil {
				logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
			}
			return nil, err
		}

		_ = book.Root().Remove(legacyProgressPath)
		fmt.Println("\n[ok] Generation complete! outline.json is the canonical state.")
	}

	return outline, nil
}

func generateOutlineHierarchicalAgentSDK(ctx context.Context, book *project.Book, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client) (*models.Outline, error) {
	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Story structure: %d parts x %d volumes x %d chapters = %d total chapters\n",
		projectConfig.Structure.TargetParts,
		projectConfig.Structure.TargetVolumes,
		projectConfig.Structure.TargetChapters,
		projectConfig.Structure.TotalChapters())
	fmt.Println("Mode: Claude Agent SDK with read-only novelgen tool queries")
	fmt.Println()

	outlinePath := book.Root().OutlineJSON()
	legacyProgressPath := book.Root().ComposeFile("outline_progress.json")

	var outline *models.Outline
	var resumeMode bool
	var err error
	if book.Root().Exists(outlinePath) && !composeForceGenFlag {
		fmt.Println("[resume] Found existing outline.json. Resuming empty-volume Agent SDK chapter generation...")
		outline, err = loadPartialOutline(book.Root(), outlinePath)
		if err != nil {
			fmt.Printf("[warn] Failed to load outline: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			fmt.Println("[ok] Resumed from outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	} else if book.Root().Exists(legacyProgressPath) && !composeForceGenFlag {
		fmt.Println("[resume] Found legacy outline_progress.json. Migrating to outline.json and resuming...")
		outline, err = loadPartialOutline(book.Root(), legacyProgressPath)
		if err != nil {
			fmt.Printf("[warn] Failed to load legacy progress: %v\n", err)
			fmt.Println("   Starting fresh generation...")
		} else {
			resumeMode = true
			if err := saveOutlineProgress(ctx, book, outline); err != nil {
				return nil, fmt.Errorf("failed to migrate legacy progress to outline.json: %w", err)
			}
			fmt.Println("[ok] Migrated legacy progress to outline.json")
			printProgressStatus(outline, projectConfig.Structure)
		}
	}

	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())
	onVolumeComplete := func(o *models.Outline, partIdx, volIdx, volumeCount int) {
		if err := saveOutlineProgress(ctx, book, o); err != nil {
			logger.GetLogger().Warn("Failed to save outline progress: %v", err)
		} else {
			fmt.Printf("[save] outline.json saved (%d volumes completed)\n", volumeCount)
		}
		if _, err := book.SaveOutlineMarkdown(o); err != nil {
			logger.GetLogger().Warn("Failed to save outline markdown progress: %v", err)
		}
	}

	if !resumeMode {
		fmt.Println("Using Agent SDK hierarchical generation:")
		fmt.Println("  Phase 1: Generate skeleton (parts and volumes)")
		fmt.Printf("  Phase 2: Generate chapters for each of %d volumes\n",
			projectConfig.Structure.TargetParts*projectConfig.Structure.TargetVolumes)
		fmt.Println()

		skeletonOutput, err := agent.GenerateSkeletonWithAgentSDK(ctx, agents.ComposeSkeletonInput{
			Setup:     *setup,
			Structure: projectConfig.Structure,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate skeleton: %w", err)
		}
		outline = &models.Outline{Parts: skeletonOutput.Parts}
		logic.NewIDManager(outline).AssignIDsToOutline()
		applyOutlineNormalization(book, outline, "agent_sdk_fresh_skeleton")
		if err := saveOutlineProgress(ctx, book, outline); err != nil {
			logger.GetLogger().Warn("Failed to save initial outline: %v", err)
		}
		fmt.Println("[save] Skeleton saved to outline.json")
	} else {
		fmt.Println("Continuing Agent SDK chapter generation...")
		fmt.Println()
	}

	outline, err = agent.GenerateChaptersHierarchicalAgentSDK(ctx, *setup, projectConfig.Structure, outline, onVolumeComplete)
	if err != nil {
		if saveErr := saveOutlineProgress(ctx, book, outline); saveErr != nil {
			logger.GetLogger().Warn("Failed to save outline on error: %v", saveErr)
		}
		return nil, err
	}

	_ = book.Root().Remove(legacyProgressPath)
	fmt.Println("\n[ok] Agent SDK generation complete. outline.json is the canonical state.")
	return outline, nil
}

// loadPartialOutline loads a partially generated outline from progress file
func loadPartialOutline(root *store.Root, path string) (*models.Outline, error) {
	var outline models.Outline
	if err := root.ReadJSON(path, &outline); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}
	return &outline, nil
}

func canResumeOutlineGeneration(root *store.Root, path string, structure models.StoryStructure) bool {
	outline, err := loadPartialOutline(root, path)
	if err != nil {
		logger.GetLogger().Warn("Failed to inspect existing outline for resume: %v", err)
		return false
	}
	if len(outline.Parts) == 0 {
		return false
	}
	expectedVolumes := structure.TargetParts * structure.TargetVolumes
	actualVolumes := 0
	for _, part := range outline.Parts {
		actualVolumes += len(part.Volumes)
	}
	if expectedVolumes > 0 && actualVolumes != expectedVolumes {
		return false
	}
	return countEmptyVolumes(outline) > 0
}

func countGeneratedVolumes(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	count := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if len(volume.Chapters) > 0 {
				count++
			}
		}
	}
	return count
}

func countEmptyVolumes(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	count := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if len(volume.Chapters) == 0 {
				count++
			}
		}
	}
	return count
}

func outlineWithGeneratedVolumes(outline *models.Outline) *models.Outline {
	if outline == nil {
		return nil
	}
	filtered := &models.Outline{}
	for _, part := range outline.Parts {
		nextPart := part
		nextPart.Volumes = nil
		for _, volume := range part.Volumes {
			if len(volume.Chapters) > 0 {
				nextPart.Volumes = append(nextPart.Volumes, volume)
			}
		}
		if len(nextPart.Volumes) > 0 {
			filtered.Parts = append(filtered.Parts, nextPart)
		}
	}
	return filtered
}

func outlineWithImproveVolumeSelection(outline *models.Outline, volume, fromVolume, toVolume int) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if volume > 0 && (fromVolume > 0 || toVolume > 0) {
		return nil, fmt.Errorf("--volume cannot be combined with --from-volume or --to-volume")
	}
	if volume < 0 || fromVolume < 0 || toVolume < 0 {
		return nil, fmt.Errorf("volume indexes must be positive")
	}

	start, end := fromVolume, toVolume
	if volume > 0 {
		start, end = volume, volume
	}
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = start
	}
	if start > end {
		return nil, fmt.Errorf("--from-volume must be less than or equal to --to-volume")
	}

	filtered := &models.Outline{}
	globalVolume := 0
	selected := 0
	for _, part := range outline.Parts {
		nextPart := part
		nextPart.Volumes = nil
		for _, vol := range part.Volumes {
			globalVolume++
			if globalVolume < start || globalVolume > end || len(vol.Chapters) == 0 {
				continue
			}
			nextPart.Volumes = append(nextPart.Volumes, vol)
			selected++
		}
		if len(nextPart.Volumes) > 0 {
			filtered.Parts = append(filtered.Parts, nextPart)
		}
	}
	if selected == 0 {
		return nil, fmt.Errorf("selected range has no generated volumes to improve")
	}
	return filtered, nil
}

func mergeGeneratedVolumes(target *models.Outline, improved *models.Outline) {
	if target == nil || improved == nil {
		return
	}
	byID := map[string]models.Volume{}
	for _, part := range improved.Parts {
		for _, volume := range part.Volumes {
			if strings.TrimSpace(volume.ID) != "" {
				byID[volume.ID] = volume
			}
		}
	}
	for partIdx := range target.Parts {
		for volIdx := range target.Parts[partIdx].Volumes {
			volume := &target.Parts[partIdx].Volumes[volIdx]
			if improvedVolume, ok := byID[volume.ID]; ok {
				preserveImprovedVolumeIdentityCmd(volume, &improvedVolume)
				target.Parts[partIdx].Volumes[volIdx] = improvedVolume
			}
		}
	}
}

func preserveImprovedVolumeIdentityCmd(original *models.Volume, improved *models.Volume) {
	if original == nil || improved == nil {
		return
	}
	improved.ID = original.ID
	for i := range improved.Chapters {
		if i >= len(original.Chapters) {
			break
		}
		improved.Chapters[i].ID = original.Chapters[i].ID
	}
}

// printProgressStatus prints the current generation progress
func printProgressStatus(outline *models.Outline, structure models.StoryStructure) {
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	completedVolumes := 0
	totalChapters := 0

	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			if len(vol.Chapters) > 0 {
				completedVolumes++
				totalChapters += len(vol.Chapters)
			}
		}
	}

	fmt.Printf("\n📊 Progress: %d/%d volumes completed (%d chapters generated)\n",
		completedVolumes, totalVolumes, totalChapters)
	fmt.Println()
}

func regenerateElement(outline *models.Outline, id string, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client) error {
	parts := strings.Split(id, "_")

	// Get user prompt for regeneration
	userPrompt := composePromptFlag
	if userPrompt == "" {
		var err error
		userPrompt, err = getRegenPrompt()
		if err != nil {
			return fmt.Errorf("failed to get regeneration prompt: %w", err)
		}
	}

	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	// Create IDManager for ID resolution
	idManager := logic.NewIDManager(outline)

	ctx := cli.CommandContext()

	switch len(parts) {
	case 1:
		// Regenerate a part
		partNum, _ := strconv.Atoi(parts[0])
		partID := idManager.GeneratePartID(partNum)
		part := idManager.GetPartByID(partID)
		if part == nil {
			return fmt.Errorf("part %s not found", partID)
		}
		fmt.Printf("Regenerating part: %s\n", partID)

		context := agent.BuildPartContext(part, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "part",
			ElementID:   part.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Part != nil {
			part.Title = output.Part.Title
			part.Summary = output.Part.Summary
		}
		return nil

	case 2:
		// Regenerate a volume
		partNum, _ := strconv.Atoi(parts[0])
		volNum, _ := strconv.Atoi(parts[1])
		volumeID := idManager.GenerateVolumeID(partNum, volNum)
		volume, _ := idManager.GetVolumeByID(volumeID)
		if volume == nil {
			return fmt.Errorf("volume %s not found", volumeID)
		}
		fmt.Printf("Regenerating volume: %s\n", volumeID)

		context := agent.BuildVolumeContext(volume, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "volume",
			ElementID:   volume.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Volume != nil {
			volume.Title = output.Volume.Title
			volume.Summary = output.Volume.Summary
			if len(output.Volume.Chapters) > 0 {
				volume.Chapters = output.Volume.Chapters
			}
		}
		return nil

	case 3:
		// Regenerate a chapter
		partNum, _ := strconv.Atoi(parts[0])
		volNum, _ := strconv.Atoi(parts[1])
		chapNum, _ := strconv.Atoi(parts[2])
		chapterID := idManager.GenerateChapterID(partNum, volNum, chapNum)
		chapter, _, _ := idManager.GetChapterByID(chapterID)
		if chapter == nil {
			return fmt.Errorf("chapter %s not found", chapterID)
		}
		fmt.Printf("Regenerating chapter: %s\n", chapterID)

		context := agent.BuildChapterContext(chapter, outline)
		input := agents.ComposeRegenInput{
			Outline:     *outline,
			ElementType: "chapter",
			ElementID:   chapter.ID,
			Suggestions: userPrompt,
			Context:     context,
		}
		output, err := agent.Regenerate(ctx, input)
		if err != nil {
			return err
		}
		if output.Chapter != nil {
			existingID := chapter.ID
			*chapter = *output.Chapter
			chapter.ID = existingID
		}
		return nil

	default:
		return fmt.Errorf("invalid ID format: %s (expected format: \"1\", \"1_1\", or \"1_1_1\")", id)
	}
}
