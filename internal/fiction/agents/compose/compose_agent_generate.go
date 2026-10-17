// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package compose

import (
	"context"
	"fmt"
	agentbase "novelgen/internal/fiction/agents/base"
	"novelgen/internal/fiction/agents/prompt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/platform/logger"
)

// ComposeSkeletonInput is the input for generating outline skeleton (parts and volumes only)
type ComposeSkeletonInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeSkeletonOutput is the output for outline skeleton generation
type ComposeSkeletonOutput struct {
	Parts []models.Part `json:"parts" md:"parts" desc:"Generated story parts with volumes"`
}

// ComposeSkeletonReviewInput is the input for reviewing an outline skeleton
// before chapter-level generation. It intentionally focuses on parts, volumes,
// summaries, payoff contracts, and long-form escalation rather than chapter
// completeness.
type ComposeSkeletonReviewInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to review"`
	Setup           models.StorySetup     `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

// ComposeSkeletonReviewOutput is the output for outline skeleton review.
type ComposeSkeletonReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeSkeletonImproveInput is the input for improving an outline skeleton.
type ComposeSkeletonImproveInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to improve"`
	ReviewResult    models.ReviewResult   `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup           models.StorySetup     `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	RevisionContext string                `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier skeleton review and improve rounds"`
}

// ComposeSkeletonImproveOutput is the output for outline skeleton improvement.
type ComposeSkeletonImproveOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Improved outline skeleton"`
}

// ComposeChaptersInput is the input for generating chapters for a volume
type ComposeChaptersInput struct {
	Setup          models.StorySetup `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Part           models.Part       `json:"part" md:"part" desc:"Current part information"`
	Volume         models.Volume     `json:"volume" md:"volume" desc:"Current volume information"`
	VolumeIndex    int               `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes   int               `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
	ChaptersPerVol int               `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Number of chapters per volume"`
	PreviousVolume *models.Volume    `json:"previous_volume,omitempty" md:"previous_volume,omitempty" desc:"Previous volume for continuity"`
	OutlineContext string            `json:"outline_context" md:"outline_context" desc:"Context from previously generated outline"`
}

// ComposeChaptersOutput is the output for chapter generation
type ComposeChaptersOutput struct {
	Chapters []models.Chapter `json:"chapters" md:"chapters" desc:"Generated chapters for the volume"`
}

type composeSkeletonPromptInput struct {
	SetupBrief string                `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, long-form plan, core cast seeds, storylines, progression systems"`
	Structure  models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target parts and volumes"`
}

type composeSkeletonReviewPromptInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to review"`
	SetupBrief      string                `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for checking skeleton alignment"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

type composeSkeletonImprovePromptInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to improve"`
	ReviewResult    models.ReviewResult   `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	SetupBrief      string                `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for preserving setup promises"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	RevisionContext string                `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier skeleton review and improve rounds"`
}

type composeChaptersPromptInput struct {
	SetupBrief     string         `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: rules, resources, core cast seeds, long-form plan, storylines"`
	Part           models.Part    `json:"part" md:"part" desc:"Current part information"`
	Volume         models.Volume  `json:"volume" md:"volume" desc:"Current volume information"`
	VolumeIndex    int            `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes   int            `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
	ChaptersPerVol int            `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Number of chapters to generate in this call"`
	PreviousVolume *models.Volume `json:"previous_volume,omitempty" md:"previous_volume,omitempty" desc:"Previous volume for continuity"`
	OutlineContext string         `json:"outline_context" md:"outline_context" desc:"Context from previously generated outline"`
}

func composeImproveDetailChapterIDs(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput) (map[string]bool, bool) {
	if !composeImproveHasFocusedTasks(promptInput) {
		return nil, false
	}
	volumeChapterIDs := map[string]bool{}
	for _, chapter := range volume.Chapters {
		if id := strings.TrimSpace(chapter.ID); id != "" {
			volumeChapterIDs[id] = true
		}
	}
	ids := map[string]bool{}
	for _, suggestion := range promptInput.ReviewResult.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID == strings.TrimSpace(volume.ID) {
			return nil, false
		}
		if volumeChapterIDs[targetID] {
			ids[targetID] = true
			composeAddAdjacentChapterIDs(ids, volume, targetID)
		}
	}
	for _, id := range composeImproveBoundaryChapterIDs(volume, promptInput.UserPrompt) {
		if volumeChapterIDs[id] {
			ids[id] = true
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

func composeAddAdjacentChapterIDs(ids map[string]bool, volume models.Volume, targetID string) {
	for i, chapter := range volume.Chapters {
		if strings.TrimSpace(chapter.ID) != targetID {
			continue
		}
		if i > 0 {
			if id := strings.TrimSpace(volume.Chapters[i-1].ID); id != "" {
				ids[id] = true
			}
		}
		if i+1 < len(volume.Chapters) {
			if id := strings.TrimSpace(volume.Chapters[i+1].ID); id != "" {
				ids[id] = true
			}
		}
		return
	}
}

func composeImproveBoundaryChapterIDs(volume models.Volume, prompt string) []string {
	rawPrompt := strings.TrimSpace(prompt)
	prompt = strings.ToLower(rawPrompt)
	if prompt == "" || composeImprovePromptRequestsAllChapters(prompt) {
		return nil
	}
	ids := []string{}
	for i, chapter := range volume.Chapters {
		id := strings.TrimSpace(chapter.ID)
		if id == "" {
			continue
		}
		if strings.Contains(prompt, strings.ToLower(id)) || composeImprovePromptMentionsChapterOrdinal(prompt, i+1) {
			ids = append(ids, id)
		}
	}
	if composeImprovePromptMentionsFirstChapter(prompt) && len(volume.Chapters) > 0 {
		if id := strings.TrimSpace(volume.Chapters[0].ID); id != "" {
			if !composeStringSliceContains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	if composeImprovePromptMentionsLastChapter(prompt) && len(volume.Chapters) > 0 {
		if id := strings.TrimSpace(volume.Chapters[len(volume.Chapters)-1].ID); id != "" && !composeStringSliceContains(ids, id) {
			ids = append(ids, id)
		}
	}
	// A prompt that enumerates every chapter in a multi-chapter volume is a
	// whole-volume revision, not a boundary-scoped request: there is no
	// narrower subset to restrict queries to, and demanding a focused check
	// per chapter would be redundant with the count-based evidence minimums
	// (and would fail runs where the agent reasonably skips checks for
	// unchanged chapters). Single-chapter volumes keep the boundary so a
	// specific chapter target still filters global suggestions.
	if len(volume.Chapters) > 1 && len(ids) == len(volume.Chapters) {
		return nil
	}
	return ids
}

func composeImprovePromptRequestsAllChapters(prompt string) bool {
	for _, marker := range []string{"每章", "每一章", "所有章节", "全部章节", "逐章", "全卷", "整卷", "all chapters", "every chapter", "whole volume"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeImprovePromptMentionsChapterOrdinal(prompt string, ordinal int) bool {
	if ordinal <= 0 {
		return false
	}
	markers := []string{
		fmt.Sprintf("第%d章", ordinal),
		fmt.Sprintf("第 %d 章", ordinal),
		fmt.Sprintf("chapter %d", ordinal),
		fmt.Sprintf("ch%d", ordinal),
	}
	if cn := smallChineseOrdinal(ordinal); cn != "" {
		markers = append(markers, "第"+cn+"章")
	}
	for _, marker := range markers {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeImprovePromptMentionsFirstChapter(prompt string) bool {
	for _, marker := range []string{"首章", "第一章", "开头", "开篇", "卷首", "first chapter", "opening chapter"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeImprovePromptMentionsLastChapter(prompt string) bool {
	for _, marker := range []string{"卷尾", "尾章", "最后一章", "末章", "结尾", "收束", "卷末", "final chapter", "last chapter", "ending"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

// Generate creates a story outline from setup and structure
// This is the type-safe wrapper around BaseAgent.Execute
func (a *ComposeAgent) Generate(ctx context.Context, input ComposeGenInput) (ComposeGenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Generation")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes × %d chapters",
		input.Structure.TargetParts, input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Language: %s", a.base.Language())

	var output ComposeGenOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-gen"},
		Command: "generate a story outline with the specified structure",
	}

	promptInput := composeGenPromptInput{
		SetupBrief: a.buildSetupBrief(&input.Setup),
		Structure:  input.Structure,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate the outline structure
	if err := a.validateOutlineStructure(&output.Outline, input.Structure); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate chapter anchors and state change mapping
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Assign IDs to all elements using IDManager
	idManager := logic.NewIDManager(&output.Outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := input.Structure.TotalChapters()
	logger.Info("Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume",
		len(output.Outline.Parts), input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Total: %d chapters", totalChapters)

	return output, nil
}

// Regenerate regenerates a story outline element (part, volume, or chapter)
func (a *ComposeAgent) Regenerate(ctx context.Context, input ComposeRegenInput) (ComposeRegenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Regeneration")
	logger.Info("Element Type: %s", input.ElementType)
	logger.Info("Element ID: %s", input.ElementID)
	logger.Info("Language: %s", a.base.Language())

	params := agentbase.InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: fmt.Sprintf("regenerate a %s while maintaining continuity", input.ElementType),
	}

	var output ComposeRegenOutput

	switch input.ElementType {
	case "part":
		var part models.Part
		if err := a.base.Execute(ctx, params, input, &part); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Part = &part
		logger.Info("✓ Part regenerated: %s", part.Title)

	case "volume":
		var volume models.Volume
		if err := a.base.Execute(ctx, params, input, &volume); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Volume = &volume
		logger.Info("✓ Volume regenerated: %s (%d chapters)", volume.Title, len(volume.Chapters))

	case "chapter":
		var chapter models.Chapter
		if err := a.base.Execute(ctx, params, input, &chapter); err != nil {
			return ComposeRegenOutput{}, err
		}
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeRegenOutput{}, fmt.Errorf("validation failed: %w", err)
		}
		output.Chapter = &chapter
		logger.Info("✓ Chapter regenerated: %s", chapter.Title)

	default:
		return ComposeRegenOutput{}, fmt.Errorf("invalid element type: %s", input.ElementType)
	}

	return output, nil
}

// ReviewSkeleton reviews an outline skeleton without requiring generated chapters.
func (a *ComposeAgent) ReviewSkeleton(ctx context.Context, input ComposeSkeletonReviewInput) (ComposeSkeletonReviewOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Skeleton Review")
	logger.Info("Language: %s", a.base.Language())

	var output ComposeSkeletonReviewOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-skeleton-review"},
		Command: "review the story outline skeleton and provide improvement suggestions",
	}

	promptInput := composeSkeletonReviewPromptInput{
		ExistingOutline: input.ExistingOutline,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		Structure:       input.Structure,
		UserPrompt:      input.UserPrompt,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Result); err != nil {
		return ComposeSkeletonReviewOutput{}, err
	}
	output.Result.NormalizeScoreScale()

	logger.Section("Outline Skeleton Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// ImproveSkeleton improves only the part/volume skeleton. Existing chapters are
// preserved, so this is safe to run before or after some volumes have chapters.
func (a *ComposeAgent) ImproveSkeleton(ctx context.Context, input ComposeSkeletonImproveInput) (ComposeSkeletonImproveOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Skeleton Improvement")
	logger.Info("Language: %s", a.base.Language())

	var output ComposeSkeletonImproveOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-skeleton-improve"},
		Command: "improve the story outline skeleton",
	}

	promptInput := composeSkeletonImprovePromptInput{
		ExistingOutline: input.ExistingOutline,
		ReviewResult:    prompt.CompactReviewForPrompt(input.ReviewResult),
		UserPrompt:      input.UserPrompt,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		Structure:       input.Structure,
		RevisionContext: input.RevisionContext,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}

	if err := preserveSkeletonIdentityAndChapters(&input.ExistingOutline, &output.Outline); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}
	models.EnsureVolumeTitleOrdinals(&output.Outline)
	if err := validateOutlineSkeleton(&output.Outline, input.Structure); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}

	return output, nil
}

func formatChapterBrief(ch models.Chapter) string {
	beats := ch.GetBeats()
	firstBeat := ""
	lastBeat := ""
	if len(beats) > 0 {
		firstBeat = beats[0]
		lastBeat = beats[len(beats)-1]
	}
	var states []string
	if ch.StateChange != "" {
		states = append(states, "state_change="+prompt.ClipForPrompt(ch.StateChange, 100))
	}
	if ch.StateAnchor.Cultivation != "" {
		states = append(states, "cultivation="+prompt.ClipForPrompt(ch.StateAnchor.Cultivation, 80))
	}
	if len(ch.StateAnchor.Allies) > 0 {
		states = append(states, "allies="+strings.Join(ch.StateAnchor.Allies, ", "))
	}
	stateLine := strings.Join(states, "; ")
	if stateLine == "" {
		stateLine = "no explicit state anchor"
	}

	payoffLine := ""
	if payoff := formatChapterPayoffBrief(ch.ChapterPayoff); payoff != "" {
		payoffLine = " | payoff: " + payoff
	}

	return fmt.Sprintf("- %s %s: %s | %s%s | first beat: %s | last beat: %s\n",
		ch.ID, ch.Title, prompt.ClipForPrompt(ch.Summary, 240), stateLine, payoffLine, prompt.ClipForPrompt(firstBeat, 120), prompt.ClipForPrompt(lastBeat, 120))
}

func formatChapterPayoffBrief(payoff *models.ChapterPayoff) string {
	if payoff.IsZero() {
		return ""
	}
	var parts []string
	if strings.TrimSpace(payoff.Desire) != "" {
		parts = append(parts, "desire="+prompt.ClipForPrompt(payoff.Desire, 90))
	}
	if strings.TrimSpace(payoff.Pressure) != "" {
		parts = append(parts, "pressure="+prompt.ClipForPrompt(payoff.Pressure, 90))
	}
	if strings.TrimSpace(payoff.CleverMove) != "" {
		parts = append(parts, "move="+prompt.ClipForPrompt(payoff.CleverMove, 100))
	}
	if strings.TrimSpace(payoff.PayoffMoment) != "" {
		parts = append(parts, "moment="+prompt.ClipForPrompt(payoff.PayoffMoment, 120))
	}
	if strings.TrimSpace(payoff.Reward) != "" {
		parts = append(parts, "reward="+prompt.ClipForPrompt(payoff.Reward, 90))
	}
	if strings.TrimSpace(payoff.SocialProof) != "" {
		parts = append(parts, "proof="+prompt.ClipForPrompt(payoff.SocialProof, 90))
	}
	if strings.TrimSpace(payoff.Hook) != "" {
		parts = append(parts, "hook="+prompt.ClipForPrompt(payoff.Hook, 100))
	}
	return strings.Join(parts, " | ")
}

// BuildChapterContext builds context for chapter regeneration
func (a *ComposeAgent) BuildChapterContext(chapter *models.Chapter, outline *models.Outline) string {
	var context strings.Builder

	// Find chapter in outline
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i, chap := range vol.Chapters {
				if chap.ID == chapter.ID {
					// Add part and volume context
					context.WriteString("=== CURRENT LOCATION IN STORY ===\n")
					context.WriteString(fmt.Sprintf("Part: %s\nPart Summary: %s\n\n", part.Title, part.Summary))
					context.WriteString(fmt.Sprintf("Volume: %s\nVolume Summary: %s\n\n", vol.Title, vol.Summary))

					// Add previous chapters context (up to 2 chapters back for better continuity)
					context.WriteString("=== PREVIOUS CHAPTERS (For Continuity) ===\n")
					if i > 0 {
						prevChap := vol.Chapters[i-1]
						context.WriteString(fmt.Sprintf("Previous Chapter (%s): %s\n", prevChap.ID, prevChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prevChap.Summary))
						context.WriteString(fmt.Sprintf("Events: %s\n", a.formatEvents(prevChap.Events)))
						prevBeats := "None"
						if len(prevChap.GetBeats()) > 0 {
							prevBeats = strings.Join(prevChap.GetBeats(), "; ")
						}
						lastBeat := "None"
						if len(prevChap.GetBeats()) > 0 {
							lastBeat = prevChap.GetBeats()[len(prevChap.GetBeats())-1]
						}
						prevClosing := getClosingBeat(prevChap)
						if prevClosing == "" {
							prevClosing = lastBeat
						}
						context.WriteString(fmt.Sprintf("Beats: %s\n", prevBeats))
						context.WriteString(fmt.Sprintf("Final Beat: %s\n", lastBeat))
						context.WriteString(fmt.Sprintf("Closing Beat: %s\n", prevClosing))
						context.WriteString("\n")
					}
					if i > 1 {
						prev2Chap := vol.Chapters[i-2]
						context.WriteString(fmt.Sprintf("Two Chapters Back (%s): %s\n", prev2Chap.ID, prev2Chap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prev2Chap.Summary))
						context.WriteString(fmt.Sprintf("Key Events: %s\n\n", a.formatEvents(prev2Chap.Events)))
					}

					// Add next chapter context
					if i < len(vol.Chapters)-1 {
						nextChap := vol.Chapters[i+1]
						context.WriteString("=== NEXT CHAPTER (What This Chapter Must Lead To) ===\n")
						context.WriteString(fmt.Sprintf("Next Chapter (%s): %s\n", nextChap.ID, nextChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", nextChap.Summary))
						nextFirstBeat := getOpeningBeat(nextChap)
						context.WriteString(fmt.Sprintf("Opening Beat: %s\n", nextFirstBeat))
						context.WriteString(fmt.Sprintf("This chapter MUST set up: %s\n\n", nextChap.Summary))
					}

					// Add current chapter to regenerate
					context.WriteString("=== CURRENT CHAPTER TO REGENERATE ===\n")
					context.WriteString(fmt.Sprintf("Chapter Title: %s\n", chapter.Title))
					context.WriteString(fmt.Sprintf("Current Summary: %s\n", chapter.Summary))
					context.WriteString(fmt.Sprintf("Current Events: %s\n", a.formatEvents(chapter.Events)))

					return context.String()
				}
			}
		}
	}

	return context.String()
}

// validateOutlineChapters validates all chapters in the outline
func (a *ComposeAgent) validateOutlineChapters(outline *models.Outline) error {
	if outline == nil {
		return fmt.Errorf("outline is nil")
	}
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			for chapIdx := range outline.Parts[partIdx].Volumes[volIdx].Chapters {
				chapter := &outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx]
				if err := a.validateChapterOutput(chapter); err != nil {
					return fmt.Errorf("chapter %d.%d.%d invalid: %w", partIdx+1, volIdx+1, chapIdx+1, err)
				}
			}
		}
	}
	return nil
}

// validateChapterOutput validates a chapter's output
func (a *ComposeAgent) validateChapterOutput(chapter *models.Chapter) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	if len(chapter.GetBeats()) == 0 {
		return fmt.Errorf("scene beats are required")
	}
	if len(chapter.Events) == 0 {
		return fmt.Errorf("events are required")
	}
	return nil
}

func validateOutlineSkeleton(outline *models.Outline, structure models.StoryStructure) error {
	if outline == nil {
		return fmt.Errorf("outline is nil")
	}
	if len(outline.Parts) == 0 {
		return fmt.Errorf("outline skeleton has no parts")
	}
	if structure.TargetParts > 0 && len(outline.Parts) != structure.TargetParts {
		return fmt.Errorf("outline skeleton has %d parts, expected %d", len(outline.Parts), structure.TargetParts)
	}
	for partIdx, part := range outline.Parts {
		if strings.TrimSpace(part.Title) == "" {
			return fmt.Errorf("part %d has empty title", partIdx+1)
		}
		if strings.TrimSpace(part.Summary) == "" {
			return fmt.Errorf("part %d has empty summary", partIdx+1)
		}
		if len(part.Volumes) == 0 {
			return fmt.Errorf("part %d has no volumes", partIdx+1)
		}
		if structure.TargetVolumes > 0 && len(part.Volumes) != structure.TargetVolumes {
			return fmt.Errorf("part %d has %d volumes, expected %d", partIdx+1, len(part.Volumes), structure.TargetVolumes)
		}
		for volIdx, volume := range part.Volumes {
			if strings.TrimSpace(volume.Title) == "" {
				return fmt.Errorf("volume %d.%d has empty title", partIdx+1, volIdx+1)
			}
			if strings.TrimSpace(volume.Summary) == "" {
				return fmt.Errorf("volume %d.%d has empty summary", partIdx+1, volIdx+1)
			}
		}
	}
	return nil
}

func preserveSkeletonIdentityAndChapters(original *models.Outline, improved *models.Outline) error {
	if original == nil || improved == nil {
		return fmt.Errorf("outline is nil")
	}
	if len(original.Parts) != len(improved.Parts) {
		return fmt.Errorf("improved skeleton changed part count from %d to %d", len(original.Parts), len(improved.Parts))
	}
	for partIdx := range original.Parts {
		if len(original.Parts[partIdx].Volumes) != len(improved.Parts[partIdx].Volumes) {
			return fmt.Errorf("improved skeleton changed volume count in part %d from %d to %d",
				partIdx+1, len(original.Parts[partIdx].Volumes), len(improved.Parts[partIdx].Volumes))
		}
		improved.Parts[partIdx].ID = original.Parts[partIdx].ID
		for volIdx := range original.Parts[partIdx].Volumes {
			improved.Parts[partIdx].Volumes[volIdx].ID = original.Parts[partIdx].Volumes[volIdx].ID
			improved.Parts[partIdx].Volumes[volIdx].Chapters = original.Parts[partIdx].Volumes[volIdx].Chapters
		}
	}
	return nil
}

// GenerateSkeleton generates the outline skeleton (parts and volumes without chapters)
func (a *ComposeAgent) GenerateSkeleton(ctx context.Context, input ComposeSkeletonInput) (ComposeSkeletonOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Outline Skeleton")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes",
		input.Structure.TargetParts, input.Structure.TargetVolumes)
	logger.Info("Language: %s", a.base.Language())

	var output ComposeSkeletonOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-skeleton"},
		Command: "generate the story outline skeleton with parts and volumes only",
	}

	promptInput := composeSkeletonPromptInput{
		SetupBrief: a.buildSetupBrief(&input.Setup),
		Structure:  input.Structure,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeSkeletonOutput{}, err
	}

	// Validate the skeleton structure
	if len(output.Parts) != input.Structure.TargetParts {
		return ComposeSkeletonOutput{}, fmt.Errorf("AI generated %d parts, but %d were requested",
			len(output.Parts), input.Structure.TargetParts)
	}

	for i, part := range output.Parts {
		if len(part.Volumes) != input.Structure.TargetVolumes {
			return ComposeSkeletonOutput{}, fmt.Errorf("part %d has %d volumes, but %d were requested",
				i+1, len(part.Volumes), input.Structure.TargetVolumes)
		}
	}

	logger.Info("Generated skeleton with %d part(s), %d volume(s) per part",
		len(output.Parts), input.Structure.TargetVolumes)

	return output, nil
}

// GenerateChaptersForVolume generates chapters for a specific volume
func (a *ComposeAgent) GenerateChaptersForVolume(ctx context.Context, input ComposeChaptersInput) (ComposeChaptersOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Chapters for Volume")
	logger.Info("Volume: %s (%d/%d)", input.Volume.Title, input.VolumeIndex, input.TotalVolumes)
	logger.Info("Chapters to generate: %d", input.ChaptersPerVol)
	logger.Info("Language: %s", a.base.Language())

	var output ComposeChaptersOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-chapters"},
		Command: fmt.Sprintf("generate %d chapters for this volume with proper continuity", input.ChaptersPerVol),
	}

	promptInput := a.buildChaptersPromptInput(input)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeChaptersOutput{}, err
	}

	chapters := append([]models.Chapter(nil), output.Chapters...)
	if len(chapters) > input.ChaptersPerVol {
		logger.Warn("AI generated %d chapters, but %d were requested; truncating extras", len(chapters), input.ChaptersPerVol)
		chapters = chapters[:input.ChaptersPerVol]
	}

	const maxAttempts = 3
	for attempt := 2; len(chapters) < input.ChaptersPerVol && attempt <= maxAttempts; attempt++ {
		missing := input.ChaptersPerVol - len(chapters)
		logger.Warn("AI generated %d/%d chapters; requesting %d missing chapter(s) (attempt %d/%d)",
			len(chapters), input.ChaptersPerVol, missing, attempt, maxAttempts)

		continueInput := input
		continueInput.ChaptersPerVol = missing
		continueInput.Volume.Chapters = append([]models.Chapter(nil), chapters...)
		continueInput.OutlineContext = buildChapterContinuationContext(input.OutlineContext, chapters, input.ChaptersPerVol)

		continueParams := agentbase.InvokeParams{
			Skills:  []string{"compose-chapters"},
			Command: fmt.Sprintf("continue this volume by generating only the remaining %d chapters; do not repeat existing chapters", missing),
		}

		var continued ComposeChaptersOutput
		continuePromptInput := a.buildChaptersPromptInput(continueInput)
		if err := a.base.Execute(ctx, continueParams, continuePromptInput, &continued); err != nil {
			return ComposeChaptersOutput{}, err
		}
		before := len(chapters)
		chapters = appendNewChapters(chapters, continued.Chapters)
		if len(chapters) > input.ChaptersPerVol {
			logger.Warn("Continuation generated too many chapters; truncating to requested %d", input.ChaptersPerVol)
			chapters = chapters[:input.ChaptersPerVol]
		}
		if len(chapters) == before {
			return ComposeChaptersOutput{}, fmt.Errorf("AI generated %d chapters, but %d were requested; continuation returned no new chapters",
				len(chapters), input.ChaptersPerVol)
		}
	}

	if len(chapters) != input.ChaptersPerVol {
		return ComposeChaptersOutput{}, fmt.Errorf("AI generated %d chapters after retries, but %d were requested",
			len(chapters), input.ChaptersPerVol)
	}

	// Validate each chapter
	for i, chapter := range chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeChaptersOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	output.Chapters = chapters
	logger.Info("Generated %d chapters for volume", len(output.Chapters))

	return output, nil
}

func (a *ComposeAgent) buildChaptersPromptInput(input ComposeChaptersInput) composeChaptersPromptInput {
	return composeChaptersPromptInput{
		SetupBrief:     a.buildSetupBrief(&input.Setup),
		Part:           input.Part,
		Volume:         input.Volume,
		VolumeIndex:    input.VolumeIndex,
		TotalVolumes:   input.TotalVolumes,
		ChaptersPerVol: input.ChaptersPerVol,
		PreviousVolume: input.PreviousVolume,
		OutlineContext: input.OutlineContext,
	}
}

func buildChapterContinuationContext(baseContext string, existing []models.Chapter, totalRequested int) string {
	var b strings.Builder
	b.WriteString(baseContext)
	if baseContext != "" && !strings.HasSuffix(baseContext, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n=== EXISTING GENERATED CHAPTERS IN THIS VOLUME ===\n")
	b.WriteString(fmt.Sprintf("The volume must contain %d chapters total. The following %d chapter(s) already exist; continue after them and return only missing chapters.\n",
		totalRequested, len(existing)))
	for _, chapter := range existing {
		b.WriteString(formatChapterBrief(chapter))
	}
	return b.String()
}

func appendNewChapters(existing []models.Chapter, candidates []models.Chapter) []models.Chapter {
	seen := make(map[string]bool, len(existing))
	for _, chapter := range existing {
		seen[chapterIdentity(chapter)] = true
	}
	for _, chapter := range candidates {
		key := chapterIdentity(chapter)
		if key != "" && seen[key] {
			continue
		}
		existing = append(existing, chapter)
		seen[key] = true
	}
	return existing
}

func chapterIdentity(chapter models.Chapter) string {
	key := strings.ToLower(strings.TrimSpace(chapter.Title)) + "|" + strings.ToLower(strings.TrimSpace(chapter.Summary))
	if key == "|" {
		return ""
	}
	return key
}

// GenerateChaptersHierarchical generates chapters for each volume in hierarchical mode
// Supports incremental generation with save callback
func (a *ComposeAgent) GenerateChaptersHierarchical(ctx context.Context, setup models.StorySetup, structure models.StoryStructure, outline *models.Outline, onVolumeComplete func(*models.Outline, int, int, int)) (*models.Outline, error) {
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	volumeCount := 0

	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			volumeCount++
			volume := &outline.Parts[partIdx].Volumes[volIdx]

			// Skip if already has chapters (resumed generation)
			if len(volume.Chapters) > 0 {
				logger.Info("✓ Volume %d.%d: %s - already has %d chapters (skipping)",
					partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))
				continue
			}

			logger.Info("Generating chapters for Volume %d.%d: %s",
				partIdx+1, volIdx+1, volume.Title)

			// Build context from previous volume (for continuity)
			var outlineContext string
			if partIdx > 0 || volIdx > 0 {
				outlineContext = a.buildHierarchicalContext(outline, partIdx, volIdx)
			}

			// Get previous volume for continuity
			var previousVolume *models.Volume
			if volIdx > 0 {
				previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
			} else if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				if len(prevPart.Volumes) > 0 {
					previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
				}
			}

			chaptersInput := ComposeChaptersInput{
				Setup:          setup,
				Part:           outline.Parts[partIdx],
				Volume:         *volume,
				VolumeIndex:    volumeCount,
				TotalVolumes:   totalVolumes,
				ChaptersPerVol: structure.TargetChapters,
				PreviousVolume: previousVolume,
				OutlineContext: outlineContext,
			}

			chaptersOutput, err := a.GenerateChaptersForVolume(ctx, chaptersInput)
			if err != nil {
				return outline, fmt.Errorf("failed to generate chapters for volume %d.%d: %w",
					partIdx+1, volIdx+1, err)
			}

			// Assign chapters to volume
			volume.Chapters = chaptersOutput.Chapters

			logger.Info("✓ Volume %d.%d: %s - %d chapters generated",
				partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))

			// Call save callback if provided
			if onVolumeComplete != nil {
				onVolumeComplete(outline, partIdx, volIdx, volumeCount)
			}
		}
	}

	// Assign IDs to all elements
	idManager := logic.NewIDManager(outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			totalChapters += len(vol.Chapters)
		}
	}
	logger.Info("Generated complete outline with %d part(s), %d volume(s), %d chapter(s)",
		len(outline.Parts), structure.TargetVolumes*structure.TargetParts, totalChapters)

	return outline, nil
}

// BuildHierarchicalContext exposes the same continuity context used by full
// hierarchical generation for callers that generate one volume at a time.
func (a *ComposeAgent) BuildHierarchicalContext(outline *models.Outline, partIdx, volIdx int) string {
	return a.buildHierarchicalContext(outline, partIdx, volIdx)
}

// buildHierarchicalContext builds context for hierarchical generation
func (a *ComposeAgent) buildHierarchicalContext(outline *models.Outline, partIdx, volIdx int) string {
	var context strings.Builder

	context.WriteString("=== STORY CONTEXT ===\n\n")

	// Add previous volumes summary
	context.WriteString("Previous Volumes Summary:\n")
	for p := 0; p <= partIdx; p++ {
		for v := 0; v < len(outline.Parts[p].Volumes); v++ {
			if p < partIdx || v < volIdx {
				vol := outline.Parts[p].Volumes[v]
				context.WriteString(fmt.Sprintf("- %s: %s\n", vol.Title, vol.Summary))
				if len(vol.Chapters) > 0 {
					lastChap := vol.Chapters[len(vol.Chapters)-1]
					context.WriteString(fmt.Sprintf("  Last chapter: %s - %s\n", lastChap.Title, lastChap.Summary))
					context.WriteString(fmt.Sprintf("  Closing beat: %s\n", getClosingBeat(lastChap)))
				}
				context.WriteString("\n")
			}
		}
	}
	context.WriteString(a.buildContinuitySnapshot(outline, partIdx, volIdx))

	// Add current part context
	currentPart := outline.Parts[partIdx]
	context.WriteString("=== CURRENT PART ===\n")
	context.WriteString(fmt.Sprintf("Part: %s\n", currentPart.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n\n", currentPart.Summary))

	// Add current volume context
	currentVolume := currentPart.Volumes[volIdx]
	context.WriteString("=== CURRENT VOLUME ===\n")
	context.WriteString(fmt.Sprintf("Volume: %s\n", currentVolume.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n", currentVolume.Summary))
	context.WriteString("This volume needs chapters that:\n")
	context.WriteString("1. Follow from the previous volume's ending\n")
	context.WriteString("2. Build toward this volume's summary\n")
	context.WriteString("3. Set up for the next volume (if any)\n")

	return context.String()
}
