// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/logger"
	"novelgen/internal/toolplane"
)

// isTransientAgentSDKError reports whether an Agent SDK runner failure is a
// transient CLI/SDK transport issue that a retry can recover from (the CLI
// control-channel "Stream closed" crash family and network resets), as opposed
// to a deterministic prompt/parse failure that will fail the same way again.
func isTransientAgentSDKError(err error) bool {
	if err == nil {
		return false
	}
	if agentruntime.IsTransient(err) {
		return true
	}
	// Fallback for runtimes and tests that return an unwrapped error. The real
	// process runtime classifies failures at the boundary (agentruntime.RunError).
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"error in hook callback",
		"stream closed",
		"connection reset",
		"broken pipe",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func runComposeSkeletonImprove(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE SKELETON IMPROVE")

	book, projectConfig, setup, outline, err := loadComposeProjectState(cmd)
	if err != nil {
		return err
	}
	if composeMaxRoundsFlag < 1 {
		return fmt.Errorf("--max-rounds must be at least 1")
	}

	agent, err := newComposeAgentForProject(book.Root(), projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)

	improved, review, err := agent.IterateSkeleton(cmd.Context(), outline, setup, projectConfig.Structure, composeMaxRoundsFlag, 80.0, composeForceImproveFlag, composePromptFlag)
	if err != nil {
		return fmt.Errorf("skeleton improvement failed: %w", err)
	}

	applyOutlineNormalization(book, improved, "skeleton_improve")
	if err := backupOutlineFiles(book); err != nil {
		return err
	}

	outlinePath := book.Root().OutlineJSON()
	if err := saveOutlineMutation(cmd.Context(), book, improved); err != nil {
		return fmt.Errorf("failed to save improved skeleton outline: %w", err)
	}
	mdPath := book.Root().OutlineMarkdown()

	if review != nil {
		reportPath := book.Root().ComposeFile("skeleton_improve_review.json")
		if err := saveReviewResult(book, reportPath, *review); err != nil {
			return err
		}
		printReviewResult("Outline skeleton improve review", *review)
		fmt.Printf("[ok] Skeleton improve review saved to %s\n", reportPath)
	}

	fmt.Printf("[ok] Skeleton outline improved and saved to %s\n", outlinePath)
	fmt.Printf("[ok] Markdown updated at %s\n", mdPath)
	return nil
}

func runOutlineQualityGateForScope(setup *models.StorySetup, outline *models.Outline, scoped bool) quality.QualityGateResult {
	if scoped {
		return quality.RunScopedOutlineQualityGate(setup, outline)
	}
	return quality.RunOutlineQualityGate(setup, outline)
}

func outlineGateHasPatchableGlobalIssues(gate quality.QualityGateResult, outline *models.Outline) bool {
	mysteryThreads := toolplane.CollectOutlineMysteryThreads(outline)
	for _, suggestion := range gate.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID != "" && !strings.EqualFold(targetID, "global") && toolplane.NormalizeKey(suggestion.Category) != "faction_tier" {
			continue
		}
		nav := toolplane.ToolIssueNavigation("all", "outline", "all", "", suggestion, 0)
		if toolplane.StringMapValue(nav, "patch_query") != "" && nav["patch_shape"] != nil {
			return true
		}
		if toolplane.NormalizeKey(suggestion.Category) == "mysteries" && toolplane.FirstPatchableOutlineMysteryThread(mysteryThreads) != nil {
			return true
		}
	}
	return false
}

func getRegenPrompt() (string, error) {
	fmt.Println("\n💡 Regeneration Prompt")
	fmt.Println("======================")
	fmt.Println("Enter your suggestions for regeneration (e.g., 'make it more intense', 'add a plot twist')")
	fmt.Println("Press Enter to skip and use default regeneration:")

	promptPrompt := &survey.Multiline{
		Message: "Your suggestions:",
	}

	var prompt string
	if err := survey.AskOne(promptPrompt, &prompt); err != nil {
		return "", err
	}

	return strings.TrimSpace(prompt), nil
}
