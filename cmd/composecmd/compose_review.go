// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"novelgen/internal/agents"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	"novelgen/internal/rpg"
	"novelgen/internal/store"
	"novelgen/internal/workerpool"
)

func runComposeSkeletonReview(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE SKELETON REVIEW")

	book, projectConfig, setup, outline, err := loadComposeProjectState(cmd)
	if err != nil {
		return err
	}

	agent, err := newComposeAgentForProject(book.Root, projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)

	review, err := agent.ReviewSkeleton(cmd.Context(), agents.ComposeSkeletonReviewInput{
		ExistingOutline: *outline,
		Setup:           *setup,
		Structure:       projectConfig.Structure,
		UserPrompt:      composePromptFlag,
	})
	if err != nil {
		return fmt.Errorf("skeleton review failed: %w", err)
	}

	reportPath := book.ComposeFile("skeleton_review.json")
	if err := saveReviewResult(book, reportPath, review.Result); err != nil {
		return err
	}
	printReviewResult("Outline skeleton review", review.Result)
	fmt.Printf("[ok] Skeleton review saved to %s\n", reportPath)
	return nil
}

func runComposeReview(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE REVIEW")

	if composeReviewMatrixFlag {
		return runComposeReviewMatrix(cmd, args)
	}

	if composeCrossVolumeFlag {
		return runComposeCrossVolumeReview(cmd, args)
	}

	book, projectConfig, setup, outline, err := loadComposeProjectState(cmd)
	if err != nil {
		return err
	}
	agent, err := newComposeAgentForProject(book.Root, projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(composeModelFlag) != "" {
		agent.SetModelOverride(composeModelFlag)
	}

	var review models.ReviewResult
	// --focus 提供内置审查视角，与 --prompt 自由文本可组合使用。
	userPrompt := composePromptFlag
	if focusPrompt := agents.ResolveReviewFocusPrompt(composeReviewFocusFlag); focusPrompt != "" {
		if userPrompt != "" {
			userPrompt = focusPrompt + "\n\n" + userPrompt
		} else {
			userPrompt = focusPrompt
		}
	}
	if composeAgentSDKFlag {
		input := agents.ComposeOutlineReviewInput{
			Outline:    *outline,
			Setup:      *setup,
			UserPrompt: userPrompt,
		}
		if composeImproveVolume > 0 {
			volumeID, resolveErr := resolveGlobalVolumeID(outline, composeImproveVolume)
			if resolveErr != nil {
				return resolveErr
			}
			input.VolumeID = volumeID
		}
		review, err = agent.ReviewOutlineWithAgentSDK(cmd.Context(), input)
		if err != nil {
			return fmt.Errorf("outline review failed: %w", err)
		}
	} else {
		result, reviewErr := agent.Review(cmd.Context(), agents.ComposeReviewInput{
			ExistingOutline: *outline,
			Setup:           *setup,
			UserPrompt:      userPrompt,
		})
		if reviewErr != nil {
			return fmt.Errorf("outline review failed: %w", reviewErr)
		}
		review = result.Result
	}

	reportPath := strings.TrimSpace(composeReviewOutFlag)
	if reportPath == "" {
		reportPath = book.ComposeFile("outline_review.json")
	} else {
		reportPath = book.ResolvePath(reportPath)
	}
	if err := saveReviewResult(book, reportPath, review); err != nil {
		return err
	}
	printReviewResult("Outline review", review)
	fmt.Printf("[ok] Outline review saved to %s\n", reportPath)
	return nil
}

// runComposeReviewMatrix 运行多模型 × 多 focus 的 review 矩阵:
// 每个 (model, focus) 组合跑一轮只读 review, 合并所有建议, 按主题聚类,
// 分层抽样后输出 sampled JSON (可直接喂 --suggestions) 和合理化 prompt。
func runComposeReviewMatrix(cmd *cobra.Command, args []string) error {
	book, projectConfig, setup, outline, err := loadComposeProjectState(cmd)
	if err != nil {
		return err
	}

	// 解析模型列表
	var modelList []string
	if strings.TrimSpace(composeReviewModelsFlag) != "" {
		for _, m := range strings.Split(composeReviewModelsFlag, ",") {
			if m = strings.TrimSpace(m); m != "" {
				modelList = append(modelList, m)
			}
		}
	}
	if len(modelList) == 0 && strings.TrimSpace(projectConfig.LLM.Model) != "" {
		modelList = []string{strings.TrimSpace(projectConfig.LLM.Model)}
	}
	if len(modelList) == 0 {
		return fmt.Errorf("no models to run matrix with; pass --models or set project model")
	}

	// 解析 focus 列表
	var focusNames []string
	rawFocus := strings.TrimSpace(composeReviewFocusFlag)
	if rawFocus == "" || rawFocus == "all" {
		focusNames = agents.ListReviewFocusNames()
	} else {
		for _, f := range strings.Split(rawFocus, ",") {
			if f = strings.TrimSpace(f); f != "" {
				focusNames = append(focusNames, f)
			}
		}
	}
	if len(focusNames) == 0 {
		return fmt.Errorf("no focus selected for matrix")
	}

	totalRuns := len(modelList) * len(focusNames)
	logger.Info("Review matrix: %d models × %d focuses = %d runs", len(modelList), len(focusNames), totalRuns)
	fmt.Printf("=== Review matrix: %d models × %d focuses = %d runs ===\n", len(modelList), len(focusNames), totalRuns)

	// 并发跑 review
	parallel := composeReviewParallelFlag
	if parallel <= 0 {
		parallel = 4
	}
	if parallel > totalRuns {
		parallel = totalRuns
	}
	type runSpec struct {
		model string
		focus string
	}
	type runResult struct {
		model  string
		focus  string
		score  float64
		review models.ReviewResult
		err    error
	}
	specs := make([]runSpec, 0, totalRuns)
	for _, model := range modelList {
		for _, focus := range focusNames {
			specs = append(specs, runSpec{model: model, focus: focus})
		}
	}

	results, _ := workerpool.Map(cmd.Context(), parallel, specs, func(ctx context.Context, _ int, spec runSpec) (runResult, error) {
		model, focus := spec.model, spec.focus
		agent, aerr := newComposeAgentForProject(book.Root, projectConfig)
		if aerr != nil {
			return runResult{model: model, focus: focus, err: aerr}, nil
		}
		agent.SetLanguage(projectConfig.Language)
		if model != "" {
			agent.SetModelOverride(model)
		}
		focusPrompt := agents.ResolveReviewFocusPrompt(focus)
		userPrompt := focusPrompt
		if strings.TrimSpace(composePromptFlag) != "" {
			if userPrompt != "" {
				userPrompt = userPrompt + "\n\n" + composePromptFlag
			} else {
				userPrompt = composePromptFlag
			}
		}
		input := agents.ComposeOutlineReviewInput{
			Outline:    *outline,
			Setup:      *setup,
			UserPrompt: userPrompt,
		}
		if composeImproveVolume > 0 {
			volumeID, resolveErr := resolveGlobalVolumeID(outline, composeImproveVolume)
			if resolveErr != nil {
				return runResult{model: model, focus: focus, err: resolveErr}, nil
			}
			input.VolumeID = volumeID
		}
		review, rerr := agent.ReviewOutlineWithAgentSDK(ctx, input)
		return runResult{model: model, focus: focus, score: review.OverallScore, review: review, err: rerr}, nil
	})

	// 汇总: 收集所有建议
	var allSugs []agents.ReviewMatrixSuggestion
	okCount := 0
	for _, r := range results {
		status := "ok"
		if r.err != nil {
			status = fmt.Sprintf("ERR: %v", r.err)
			logger.Error("Matrix run %s/%s failed: %v", r.model, r.focus, r.err)
		} else {
			okCount++
			for _, s := range r.review.Suggestions {
				allSugs = append(allSugs, agents.ReviewMatrixSuggestion{
					ReviewSuggestion: s,
					Src:              r.model + "/" + r.focus,
					ReviewCore:       int(r.score),
				})
			}
		}
		fmt.Printf("  [%s/%s] score=%v %s\n", r.model, r.focus, r.score, status)
	}
	fmt.Printf("成功 %d/%d 轮\n", okCount, totalRuns)
	if len(allSugs) == 0 {
		return fmt.Errorf("matrix collected no suggestions (all runs failed)")
	}

	agents.AnnotateMatrixSuggestions(allSugs)

	// 主题分布
	themeCount := map[string]int{}
	for _, s := range allSugs {
		if len(s.Themes) > 0 {
			themeCount[s.Themes[0]]++
		}
	}
	fmt.Printf("合并建议总数: %d\n", len(allSugs))
	fmt.Printf("主题分布: %v\n", themeCount)

	// 输出目录: story/reviews/matrix_<timestamp>/
	outDir := book.ReviewFile(fmt.Sprintf("matrix_%s", time.Now().Format("20060102_150405")))

	// 保存全部建议
	allPath := filepath.Join(outDir, "all_suggestions.json")
	if data, err := json.MarshalIndent(allSugs, "", "  "); err == nil {
		if err := book.Write(allPath, data, store.FileMode); err != nil {
			return err
		}
	}

	// 分层抽样
	sampleN := composeReviewSampleFlag
	if sampleN <= 0 {
		sampleN = 10
	}
	picked := agents.StratifiedSample(allSugs, sampleN, composeReviewSeedFlag)
	var sampled []agents.ReviewMatrixSuggestion
	for _, idx := range picked {
		sampled = append(sampled, allSugs[idx])
	}

	samplePath := filepath.Join(outDir, fmt.Sprintf("sampled_%d.json", len(sampled)))
	if data, err := json.MarshalIndent(sampled, "", "  "); err == nil {
		if err := book.Write(samplePath, data, store.FileMode); err != nil {
			return err
		}
	}

	// 合理化 prompt
	rp := agents.BuildRationalizePrompt(sampled)
	rpPath := filepath.Join(outDir, "rationalize_prompt.txt")
	if err := book.WriteText(rpPath, rp); err != nil {
		return err
	}

	// 汇总报告
	reportPath := filepath.Join(outDir, "matrix_report.txt")
	var rb strings.Builder
	fmt.Fprintf(&rb, "review 矩阵: %d 模型 × %d focus = %d 轮\n", len(modelList), len(focusNames), totalRuns)
	fmt.Fprintf(&rb, "成功: %d/%d\n", okCount, totalRuns)
	fmt.Fprintf(&rb, "合并建议: %d\n", len(allSugs))
	fmt.Fprintf(&rb, "主题分布: %v\n", themeCount)
	fmt.Fprintf(&rb, "抽样(分层): %d 条, seed=%d\n", len(sampled), composeReviewSeedFlag)
	sampleThemes := map[string]int{}
	for _, s := range sampled {
		primary := "其他"
		if len(s.Themes) > 0 {
			primary = s.Themes[0]
		}
		sampleThemes[primary]++
	}
	fmt.Fprintf(&rb, "抽样主题覆盖: %v\n", sampleThemes)
	if err := book.WriteText(reportPath, rb.String()); err != nil {
		return err
	}

	fmt.Printf("\n[ok] Matrix review complete. Outputs in %s\n", outDir)
	fmt.Printf("  - all suggestions:   %s\n", allPath)
	fmt.Printf("  - stratified sample: %s\n", samplePath)
	fmt.Printf("  - rationalize:       %s\n", rpPath)
	fmt.Printf("  - report:            %s\n", reportPath)
	fmt.Printf("\n下一步: novelgen compose improve --agent-sdk --suggestions %s\n", samplePath)
	return nil
}

// runComposeCrossVolumeReview 跨卷审查: 单 agent session 内审查卷范围的连续性。
func runComposeCrossVolumeReview(cmd *cobra.Command, args []string) error {
	book, projectConfig, setup, outline, err := loadComposeProjectState(cmd)
	if err != nil {
		return err
	}
	fromIdx := composeReviewFromVolumeFlag
	if fromIdx <= 0 {
		return fmt.Errorf("--from-volume is required for --cross-volume review")
	}
	toIdx := composeReviewToVolumeFlag
	if toIdx > 0 && toIdx < fromIdx {
		return fmt.Errorf("--to-volume %d must be >= --from-volume %d", toIdx, fromIdx)
	}

	agent, err := newComposeAgentForProject(book.Root, projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(composeModelFlag) != "" {
		agent.SetModelOverride(composeModelFlag)
	}

	input := agents.ComposeOutlineReviewInput{
		Outline:         *outline,
		Setup:           *setup,
		UserPrompt:      composePromptFlag,
		CrossVolume:     true,
		FromVolumeIndex: fromIdx,
		ToVolumeIndex:   toIdx,
		SurfaceOnly:     composeReviewSurfaceOnlyFlag,
	}
	// --focus 拼进 prompt
	if focusPrompt := agents.ResolveReviewFocusPrompt(composeReviewFocusFlag); focusPrompt != "" {
		if input.UserPrompt != "" {
			input.UserPrompt = focusPrompt + "\n\n" + input.UserPrompt
		} else {
			input.UserPrompt = focusPrompt
		}
	}

	review, err := agent.ReviewOutlineWithAgentSDK(cmd.Context(), input)
	if err != nil {
		return fmt.Errorf("cross-volume review failed: %w", err)
	}

	reportPath := strings.TrimSpace(composeReviewOutFlag)
	if reportPath == "" {
		reportPath = book.ReviewFile("cross_volume_review.json")
	} else {
		reportPath = book.ResolvePath(reportPath)
	}
	if err := saveReviewResult(book, reportPath, review); err != nil {
		return err
	}
	printReviewResult("Cross-volume review", review)
	fmt.Printf("[ok] Cross-volume review saved to %s\n", reportPath)
	return nil
}

func resolveGlobalVolumeID(outline *models.Outline, index int) (string, error) {
	if outline == nil || index <= 0 {
		return "", fmt.Errorf("invalid volume index %d", index)
	}
	seen := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			seen++
			if seen == index {
				vid := strings.TrimSpace(volume.ID)
				if vid == "" {
					return "", fmt.Errorf("volume %d has no ID", index)
				}
				return vid, nil
			}
		}
	}
	return "", fmt.Errorf("volume index %d out of range (total %d)", index, seen)
}

func runComposeCheck(cmd *cobra.Command, args []string) error {
	logger.Section("NOVELGEN COMPOSE CHECK")

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	outlinePath := book.OutlineJSON()
	if !book.HasOutline() {
		return fmt.Errorf("outline not found at %s. Run 'novelgen compose gen' first", outlinePath)
	}

	data, err := book.Read(outlinePath)
	if err != nil {
		return fmt.Errorf("failed to read outline: %w", err)
	}

	var modelOutline models.Outline
	if err := json.Unmarshal(data, &modelOutline); err != nil {
		return fmt.Errorf("failed to parse outline: %w", err)
	}

	checkOutline := &modelOutline
	partialCheck := countGeneratedVolumes(&modelOutline) > 0 && countEmptyVolumes(&modelOutline) > 0
	if partialCheck {
		checkOutline = outlineWithGeneratedVolumes(&modelOutline)
	}

	checkData, err := json.Marshal(checkOutline)
	if err != nil {
		return fmt.Errorf("failed to prepare outline for validation: %w", err)
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(checkData, &storyOutline); err != nil {
		return fmt.Errorf("failed to parse outline: %w", err)
	}

	if checkOutline != nil {
		var setupForGate *models.StorySetup
		if loadedSetup, loadErr := book.LoadSetup(); loadErr == nil {
			setupForGate = loadedSetup
		}
		quality.LogQualityGateResult("outline", quality.RunOutlineQualityGate(setupForGate, checkOutline))
		if strings.TrimSpace(composeCheckSuggestionsOut) != "" {
			gate := quality.RunOutlineCombinedGateForScope(book.Root, setupForGate, checkOutline, false)
			report := quality.QualityGateReviewResult("Deterministic outline check suggestions.", gate)
			if err := saveReviewResult(book, composeCheckSuggestionsOut, *report); err != nil {
				return err
			}
			fmt.Printf("[ok] Check suggestions report saved to %s\n", composeCheckSuggestionsOut)
		}
	}

	validator := rpg.NewOutlineValidator(&storyOutline)
	result := validator.Validate()

	if composeCheckJSONFlag {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("\n===== OUTLINE VALIDATION =====\n")
	if partialCheck {
		fmt.Printf("Partial outline: checking %d generated volume(s), skipping %d empty future volume(s).\n",
			countGeneratedVolumes(&modelOutline), countEmptyVolumes(&modelOutline))
	}
	if result.IsValid {
		fmt.Println("[ok] Outline passed validation")
	} else {
		fmt.Printf("[fail] Issues: %d | Warnings: %d | Suggestions: %d\n\n",
			result.IssueCount, result.WarningCount, len(result.Suggestions))
	}

	for _, issue := range result.Issues {
		icon := "[fail]"
		if issue.Severity == "critical" {
			icon = "[critical]"
		}
		fmt.Printf("  %s [%s/%s] %s: %s\n", icon, issue.Type, issue.Severity, issue.Location, issue.Description)
		if issue.Fix != "" {
			fmt.Printf("       Fix: %s\n", issue.Fix)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n  --- Warnings (%d) ---\n", result.WarningCount)
	}
	for _, w := range result.Warnings {
		fmt.Printf("  ⚠ [%s] %s: %s\n", w.Type, w.Location, w.Description)
		if w.Suggestion != "" {
			fmt.Printf("     -> %s\n", w.Suggestion)
		}
	}

	if len(result.Suggestions) > 0 {
		fmt.Printf("\n  --- Suggestions (%d) ---\n", len(result.Suggestions))
	}
	for _, s := range result.Suggestions {
		fmt.Printf("  💡 [%s] %s\n", s.Type, s.Location)
		fmt.Printf("     Current: %s\n", s.Current)
		fmt.Printf("     -> %s\n", s.Suggested)
	}

	fmt.Printf("\n%s\n", result.Summary)

	// Cross-module: verify outline consistency with story setup
	setupPath := book.SetupJSON()
	if setupData, err := book.Read(setupPath); err == nil {
		var setup models.StorySetup
		if json.Unmarshal(setupData, &setup) == nil {
			crossIssues, crossWarnings := quality.ValidateSetupOutlineCross(&setup, &storyOutline)
			if len(crossIssues)+len(crossWarnings) > 0 {
				// Collect missing resources and auto-patch setup
				missingResources := make(map[string]bool)
				for _, part := range storyOutline.Parts {
					for _, vol := range part.Volumes {
						for _, ch := range vol.Chapters {
							for _, entry := range ch.ResourceLedger {
								found := false
								for _, r := range setup.WorldResources {
									if r.Name == entry.Item {
										found = true
										break
									}
								}
								if !found && entry.Item != "" {
									missingResources[entry.Item] = true
								}
							}
						}
					}
				}
				if len(missingResources) > 0 {
					for name := range missingResources {
						setup.WorldResources = append(setup.WorldResources, models.WorldResource{
							Name: name, Category: "通用", Scarcity: "稀有",
							Description: "(自动添加 — 请手动补充描述)",
						})
					}
					if saveErr := saveSetupMutation(cmd.Context(), book, &setup); saveErr == nil {
						fmt.Printf("\n  [ok] Auto-patched setup: added %d missing resources\n", len(missingResources))
					}
				}

				fmt.Printf("\n  --- Cross-Module (Setup↔Outline) ---\n")
				for _, w := range crossWarnings {
					fmt.Printf("  ⚠ [cross] %s\n", w)
				}
				for _, i := range crossIssues {
					fmt.Printf("  [fail] [cross] %s\n", i)
				}
			}
		}
	}

	return nil
}

func runComposeNormalize(cmd *cobra.Command, args []string) error {
	logger.Section("NOVELGEN COMPOSE NORMALIZE")

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	outlinePath := book.OutlineJSON()
	outline, err := session.RequireOutline()
	if err != nil {
		return err
	}

	report := models.NormalizeOutline(outline)
	if !report.Changed() && !composeNormalizeForceFlag {
		fmt.Println("[ok] Outline already normalized")
		return nil
	}

	if err := backupOutlineFiles(book); err != nil {
		return err
	}

	if err := saveOutlineMutation(cmd.Context(), book, outline); err != nil {
		return fmt.Errorf("failed to write normalized outline: %w", err)
	}

	mdPath := book.OutlineMarkdown()

	reportPath := book.ComposeFile("outline_normalization_manual.json")
	if err := book.WriteJSON(reportPath, report); err != nil {
		return fmt.Errorf("failed to marshal normalization report: %w", err)
	}

	if report.Changed() {
		fmt.Printf("[ok] Applied %d deterministic outline cleanup(s)\n", len(report.Changes))
	} else {
		fmt.Println("[ok] Outline already normalized; rewrote derived files (--force)")
	}
	fmt.Printf("[ok] Updated %s and %s\n", outlinePath, mdPath)
	fmt.Printf("[ok] Report saved to %s\n", reportPath)
	return nil
}

// saveReviewResult stores a review report through the project store, so the
// report is written atomically and its parent directory is created for us.
func saveReviewResult(book *project.Book, path string, result models.ReviewResult) error {
	if err := book.WriteJSON(path, result); err != nil {
		return fmt.Errorf("failed to write review result to %s: %w", path, err)
	}
	return nil
}

func printReviewResult(label string, result models.ReviewResult) {
	fmt.Printf("\n%s: %.1f/100\n", label, result.OverallScore)
	if strings.TrimSpace(result.Summary) != "" {
		fmt.Printf("Summary: %s\n", result.Summary)
	}
	if len(result.Suggestions) > 0 {
		fmt.Println("Top suggestions:")
		limit := len(result.Suggestions)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			s := result.Suggestions[i]
			fmt.Printf("- [%s] %s: %s\n", s.Priority, s.TargetName, s.Suggestion)
		}
	}
}
