// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package compose

import (
	"context"
	"fmt"
	agentbase "novelgen/internal/agents/base"
	"novelgen/internal/agents/prompt"
	"novelgen/internal/toolcmd"
	"regexp"
	"strings"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/store"
)

type ComposeGenInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeGenOutput is the output for outline generation
type ComposeGenOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Generated complete story outline with parts, volumes, chapters"`
}

// ComposeRegenInput is the input for outline regeneration
type ComposeRegenInput struct {
	Outline     models.Outline `json:"outline" md:"outline" desc:"Existing outline to regenerate from"`
	ElementType string         `json:"element_type" md:"element_type" desc:"Type of element to regenerate: part, volume, or chapter"`
	ElementID   string         `json:"element_id" md:"element_id" desc:"ID of the element to regenerate"`
	Suggestions string         `json:"suggestions" md:"suggestions" desc:"User suggestions for regeneration"`
	Context     string         `json:"context,omitempty" md:"context,omitempty" desc:"Surrounding context for continuity"`
}

// ComposeRegenOutput is the output for outline regeneration
type ComposeRegenOutput struct {
	Part    *models.Part    `json:"part,omitempty" md:"part,omitempty" desc:"Regenerated part (if element_type is part)"`
	Volume  *models.Volume  `json:"volume,omitempty" md:"volume,omitempty" desc:"Regenerated volume (if element_type is volume)"`
	Chapter *models.Chapter `json:"chapter,omitempty" md:"chapter,omitempty" desc:"Regenerated chapter (if element_type is chapter)"`
}

// ComposeReviewInput is the input for outline review
type ComposeReviewInput struct {
	ExistingOutline models.Outline    `json:"existing_outline" md:"existing_outline" desc:"Existing outline to review"`
	Setup           models.StorySetup `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	UserPrompt      string            `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

// ComposeReviewOutput is the output for outline review
type ComposeReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveInput is the input for outline improvement
type ComposeImproveInput struct {
	ExistingOutline models.Outline      `json:"existing_outline" md:"existing_outline" desc:"Existing outline to improve"`
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup           models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	RevisionContext string              `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier outline review and improve rounds"`
}

// ComposeImproveOutput is the output for outline improvement
type ComposeImproveOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Improved story outline"`
}

// ComposeReviewVolumeInput is the input for reviewing a specific volume
type ComposeReviewVolumeInput struct {
	Outline      models.Outline `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part    `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume  `json:"volume" md:"volume" desc:"Volume to review"`
	VolumeIndex  int            `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes int            `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
}

// ComposeReviewVolumeOutput is the output for volume review
type ComposeReviewVolumeOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveVolumeInput is the input for improving a specific volume
type ComposeImproveVolumeInput struct {
	Outline      models.Outline      `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part         `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume       `json:"volume" md:"volume" desc:"Volume to improve"`
	ReviewResult models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result for improvement guidance"`
	UserPrompt   string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup        models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	// CrossVolumeIDs, when non-empty, switches this session into cross-volume mode:
	// the agent may query/check/patch ALL of the listed volumes in one session,
	// so cross-volume issues (e.g. volume-end hook + next-volume payoff) can be
	// fixed consistently instead of per-volume in isolation.
	CrossVolumeIDs []string `json:"cross_volume_ids,omitempty" md:"cross_volume_ids" desc:"Optional list of additional volume IDs this session may patch, enabling cross-volume improvement in one session"`
}

// ComposeImproveVolumeOutput is the output for volume improvement
type ComposeImproveVolumeOutput struct {
	Volume models.Volume `json:"volume" md:"volume" desc:"Improved volume with chapters"`
}

// ComposeOutlineReviewInput is the input for an Agent SDK outline review.
// VolumeID is optional; empty means the whole outline.
type ComposeOutlineReviewInput struct {
	Outline    models.Outline
	Setup      models.StorySetup
	UserPrompt string
	VolumeID   string
	// CrossVolume 为 true 时按跨卷模式审查 (范围: FromVolumeIndex..ToVolumeIndex, 1-based 全局索引)。
	CrossVolume     bool
	FromVolumeIndex int
	ToVolumeIndex   int
	// SurfaceOnly 为 true 时只暴露读者可见字段 (title/summary/opening_beat/scenes),
	// 不暴露 events/payoff_contract/state_anchor 等内部结构字段——用于审"读者读到的",
	// 而非"作者的面板"(后者常被误判为 AI 味)。
	SurfaceOnly bool
}

// ComposeAgent handles AI generation for story outline
// It wraps BaseAgent to provide type-safe methods
type ComposeAgent struct {
	base    *agentbase.BaseAgent
	project *store.Root
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *ComposeAgent {
	base := agentbase.NewBaseAgent(agentbase.BaseAgentConfig{
		Name:       "ComposeAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &ComposeAgent{base: base}
}

// SetLanguage sets the output language
func (a *ComposeAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// SetProjectRoot binds the agent to an explicit project storage root. Agent SDK
// patch recovery must reload the same project it patched, not the ambient one.
func (a *ComposeAgent) SetProjectRoot(root *store.Root) {
	if a != nil {
		a.project = root
		if a.base != nil {
			a.base.SetProjectRoot(root)
		}
	}
}

// SetModelOverride overrides the project model for this agent's chat options.
func (a *ComposeAgent) SetModelOverride(model string) {
	if a != nil && a.base != nil {
		a.base.SetModelOverride(model)
	}
}

func composeRequiredQueryAllowlist(requiredQueries []string) []string {
	return agentbase.DedupeToolAllowlist(requiredQueries)
}

func composeImproveToolAllowlist(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) []string {
	allowlist := composeRequiredQueryAllowlist(promptInput.RequiredQueries)
	allowlist = append(allowlist, composeImproveVolumeToolAllowlist(volume, promptInput, applyPatches)...)
	for _, suggestion := range promptInput.ReviewResult.Suggestions {
		if suggestion.Navigation == nil {
			continue
		}
		allowlist = append(allowlist, suggestion.Navigation.DetailQueries...)
		allowlist = append(allowlist, suggestion.Navigation.FocusedCheckQuery)
		allowlist = append(allowlist, suggestion.Navigation.RepairRouteQuery)
		allowlist = append(allowlist, suggestion.Navigation.RepairContextQuery)
	}
	return agentbase.DedupeToolAllowlist(allowlist)
}

// composeImproveVolumeToolAllowlist returns the query/check/patch allowlist for
// ONE volume (used by single-volume improve and reused per volume in
// cross-volume mode so the agent can patch several volumes in one session).
func composeImproveVolumeToolAllowlist(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) []string {
	allowlist := composeRequiredQueryAllowlist(promptInput.RequiredQueries)
	volumeID := strings.TrimSpace(volume.ID)
	detailChapterIDs, restrictDetailChapters := composeImproveDetailChapterIDs(volume, promptInput)
	allowExploratoryDetail := composeImproveHasFocusedTasks(promptInput) && !composeImproveUsesCleanProbe(promptInput)
	if volumeID != "" {
		allowlist = append(allowlist,
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID, View: "brief"}),
			fmt.Sprintf("novelgen tool query context --type outline-repair --id %s --name", volumeID),
			toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: volumeID}),
		)
		if allowExploratoryDetail {
			allowlist = append(allowlist,
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "volume", ID: volumeID}),
				"novelgen tool query outline --type refs --entity-type character --name",
				"novelgen tool query outline --type refs --entity-type item --name",
				"novelgen tool query outline --type refs --entity-type location --name",
				"novelgen tool query story-setup --type search",
				"novelgen tool query story-setup --type core-cast --name",
				"novelgen tool query story-setup --type storyline --name",
				"novelgen tool query story-setup --type premise --name",
				"novelgen tool query story-setup --type resource --name",
				"novelgen tool query story-setup --type timeline --name",
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", VolumeID: volumeID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", VolumeID: volumeID, Fields: "action,actor,target,target_type,type,details,result", View: "brief"}),
			)
		}
	}
	for _, chapter := range volume.Chapters {
		chapterID := strings.TrimSpace(chapter.ID)
		if chapterID == "" {
			continue
		}
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query context --type outline-repair --id %s --name", chapterID),
			toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: chapterID}),
		)
		if allowExploratoryDetail && (!restrictDetailChapters || detailChapterIDs[chapterID]) {
			allowlist = append(allowlist,
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: chapterID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: chapterID, Fields: "scenes", View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: chapterID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: chapterID, Fields: "result,details,target,target_type,actor,action", View: "brief"}),
			)
		}
	}
	patchTool := "novelgen tool patch outline --target volume"
	if volumeID != "" {
		patchTool += " --id " + toolcmd.SingleQuote(volumeID)
	}
	if applyPatches {
		patchTool += " --apply"
	}
	allowlist = append(allowlist, patchTool)
	return allowlist
}

// composeAdjacentVolumeIDs returns the IDs of the volumes immediately before
// and after the given volume in the flattened outline order. The current
// volume itself is never included, and volumes at part boundaries still
// consider the neighboring part's first/last volume as adjacent.
func composeAdjacentVolumeIDs(outline models.Outline, volumeID string) []string {
	volumeIDs := make([]string, 0)
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			volumeIDs = append(volumeIDs, volume.ID)
		}
	}
	for index, id := range volumeIDs {
		if id != volumeID {
			continue
		}
		adjacent := make([]string, 0, 2)
		if index > 0 {
			adjacent = append(adjacent, volumeIDs[index-1])
		}
		if index+1 < len(volumeIDs) {
			adjacent = append(adjacent, volumeIDs[index+1])
		}
		return adjacent
	}
	return nil
}

// composeAdjacentVolumeQueryAllowlist adds read-only payoff_contract/summary
// queries for the immediately adjacent volumes, so the Agent SDK workflow can
// verify cross-volume continuity facts without being denied by the allowlist.
// Adjacent volumes are never patchable in this workflow.
func composeAdjacentVolumeQueryAllowlist(outline models.Outline, volumeID string) []string {
	allowlist := make([]string, 0, 2)
	for _, adjacentID := range composeAdjacentVolumeIDs(outline, volumeID) {
		if strings.TrimSpace(adjacentID) == "" {
			continue
		}
		allowlist = append(allowlist,
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "volume", ID: adjacentID, Fields: "payoff_contract,summary", View: "brief"}),
		)
	}
	return allowlist
}

func composeImproveHasFocusedTasks(promptInput composeAgentSDKImproveVolumePromptInput) bool {
	return promptInput.ForceIssueRepair ||
		strings.TrimSpace(promptInput.UserPrompt) != "" ||
		len(promptInput.ReviewResult.Suggestions) > 0
}

func composeImproveUsesCleanProbe(promptInput composeAgentSDKImproveVolumePromptInput) bool {
	return len(promptInput.ReviewResult.Suggestions) == 0 && composePromptRequestsCheckFirstNoop(promptInput.UserPrompt)
}

func composeImproveRequiredFocusedChecks(volume models.Volume, prompt string) []string {
	ids := composeImproveBoundaryChapterIDs(volume, prompt)
	if len(ids) == 0 {
		return nil
	}
	categories := composeImprovePromptCheckCategories(prompt)
	checks := make([]string, 0, len(ids)*len(categories))
	for _, id := range ids {
		for _, category := range categories {
			if strings.EqualFold(strings.TrimSpace(category), "all") {
				// "all" 意味着任意 category 均可：用裸命令(不带 --category)作为要求，
				// 这样 agent 实际跑的 --category pacing/logic/... 变体都能被 contains 匹配。
				checks = append(checks, toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: id}))
				continue
			}
			checks = append(checks, toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: id, Category: category, MinPriority: "low", MaxIssues: 8}))
		}
	}
	return checks
}

func composeStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func globalVolumeIndex(outline models.Outline, volumeID string) int {
	idx := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			idx++
			if strings.EqualFold(strings.TrimSpace(volume.ID), strings.TrimSpace(volumeID)) {
				return idx
			}
		}
	}
	return 0
}

func countOutlineScope(outline models.Outline, volumeID string) (int, int) {
	volumes, chapters := 0, 0
	volumeID = strings.TrimSpace(volumeID)
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if volumeID != "" && !strings.EqualFold(strings.TrimSpace(volume.ID), volumeID) {
				continue
			}
			volumes++
			chapters += len(volume.Chapters)
		}
	}
	return volumes, chapters
}

func composeOutlineReviewRequiredQueries(outline models.Outline, volumeID string) []string {
	volumeID = strings.TrimSpace(volumeID)
	if volumeID != "" {
		return []string{toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID, View: "index"})}
	}
	return []string{"novelgen tool query outline --type all --view index"}
}

// composeOutlineReviewToolAllowlist 列出单卷/全书 review 允许的只读查询。
// 说明：review 要核对 setup 铁律，必须放行 story-setup --type rules（internal/toolplane
// 的 story-setup 合法类型表里含 rules），否则 agent 会撞 deny 白耗一个回合。
func composeOutlineReviewToolAllowlist(outline models.Outline, volumeID string) []string {
	allowlist := []string{
		"novelgen tool query story-setup --type rules",
		"novelgen tool query story-setup --type search",
		"novelgen tool query story-setup --type index",
		"novelgen tool query story-setup --type all",
		"novelgen tool query story-setup --type core-cast",
		"novelgen tool query story-setup --type storyline",
		"novelgen tool query story-setup --type premise",
		"novelgen tool query story-setup --type resource",
		"novelgen tool query story-setup --type timeline",
		"novelgen tool query context --type outline-volume",
		"novelgen tool query outline --type refs --entity-type storyline --name",
		"novelgen tool query outline --type refs --entity-type character --name",
		"novelgen tool query outline --type refs --entity-type item --name",
		"novelgen tool query outline --type refs --entity-type location --name",
	}
	addVolume := func(volume models.Volume) {
		vid := strings.TrimSpace(volume.ID)
		if vid == "" {
			return
		}
		allowlist = append(allowlist,
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "volume", ID: vid, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", VolumeID: vid, View: "brief"}),
		)
		for _, chapter := range volume.Chapters {
			cid := strings.TrimSpace(chapter.ID)
			if cid == "" {
				continue
			}
			allowlist = append(allowlist,
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: cid, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: cid, View: "brief"}),
			)
		}
	}
	volumeID = strings.TrimSpace(volumeID)
	if volumeID != "" {
		allowlist = append(allowlist, toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID, View: "index"}))
		for _, part := range outline.Parts {
			for _, volume := range part.Volumes {
				if strings.EqualFold(strings.TrimSpace(volume.ID), volumeID) {
					addVolume(volume)
					// Cross-volume by default: also allow read-only queries of the
					// adjacent volumes (previous + next) so the review can check
					// volume-end hooks, next-volume payoffs, and continuity drift
					// without a separate --cross-volume run.
					for _, adjID := range composeAdjacentVolumeIDs(outline, volume.ID) {
						for _, part2 := range outline.Parts {
							for _, adj := range part2.Volumes {
								if strings.EqualFold(strings.TrimSpace(adj.ID), adjID) {
									addVolume(adj)
								}
							}
						}
					}
				}
			}
		}
		return agentbase.DedupeToolAllowlist(allowlist)
	}
	allowlist = append(allowlist, "novelgen tool query outline --type all --view index")
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			addVolume(volume)
		}
	}
	return agentbase.DedupeToolAllowlist(allowlist)
}

func composeImproveToolEvidence(promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) agentbase.ToolEvidenceRequirement {
	evidence := agentbase.ToolEvidenceRequirement{
		MinContextQueryCalls: 1,
		MinCheckCalls:        1,
		RequireNoDeniedTools: true,
	}
	evidence.RequiredToolCommands = append(evidence.RequiredToolCommands, promptInputRequiredFocusedChecks(promptInput)...)
	if !composeImproveHasFocusedTasks(promptInput) || composeImproveUsesCleanProbe(promptInput) {
		evidence.MaxQueryCalls = 2
		evidence.MaxContextQueryCalls = 2
		evidence.DisallowQueryFullCalls = true
	}
	if applyPatches {
		evidence.RequirePatchApplyFollowupCheck = true
	}
	return evidence
}

func liveSummaryHasAnyToolCommand(summary *agentruntime.LiveSummary, commands []string) bool {
	for _, command := range commands {
		if agentbase.LiveSummaryHasToolCommand(summary, command) {
			return true
		}
	}
	return false
}

func isEmptyPayoffContract(payoff *models.VolumePayoffContract) bool {
	return payoff == nil || payoff.IsZero()
}

// Review reviews an existing outline and provides improvement suggestions
func (a *ComposeAgent) Review(ctx context.Context, input ComposeReviewInput) (ComposeReviewOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Review")
	logger.Info("Language: %s", a.base.Language())

	var output ComposeReviewOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-review"},
		Command: "review the story outline and provide improvement suggestions",
	}

	promptInput := composeReviewPromptInput{
		ExistingOutline: input.ExistingOutline,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		UserPrompt:      input.UserPrompt,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Result); err != nil {
		return ComposeReviewOutput{}, err
	}
	output.Result.NormalizeScoreScale()

	// Log result
	logger.Section("Outline Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// Improve improves an existing outline
func (a *ComposeAgent) Improve(ctx context.Context, input ComposeImproveInput) (ComposeImproveOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Improvement")
	logger.Info("Language: %s", a.base.Language())

	var output ComposeImproveOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-improve"},
		Command: "improve the story outline",
	}

	promptInput := composeImprovePromptInput{
		ExistingOutline: input.ExistingOutline,
		ReviewResult:    prompt.CompactReviewForPrompt(input.ReviewResult),
		UserPrompt:      input.UserPrompt,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		RevisionContext: input.RevisionContext,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	// Validate the improved outline
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	return output, nil
}

// BuildPartContext builds context for part regeneration
func (a *ComposeAgent) BuildPartContext(part *models.Part, outline *models.Outline) string {
	var context strings.Builder

	// Find part index
	partIdx := -1
	for i, p := range outline.Parts {
		if p.ID == part.ID {
			partIdx = i
			break
		}
	}

	if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		context.WriteString(fmt.Sprintf("Previous Part (%s): %s\nSummary: %s\n\n",
			prevPart.ID, prevPart.Title, prevPart.Summary))
	}

	if partIdx < len(outline.Parts)-1 {
		nextPart := outline.Parts[partIdx+1]
		context.WriteString(fmt.Sprintf("Next Part (%s): %s\nSummary: %s\n\n",
			nextPart.ID, nextPart.Title, nextPart.Summary))
	}

	return context.String()
}

// ImproveVolume improves a specific volume based on review feedback
func (a *ComposeAgent) ImproveVolume(ctx context.Context, input ComposeImproveVolumeInput) (ComposeImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT - Volume Improvement")
	logger.Info("Volume: %s", input.Volume.Title)
	logger.Info("Language: %s", a.base.Language())

	var output ComposeImproveVolumeOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"compose-improve-volume"},
		Command: "improve the chapters in this volume based on review feedback",
	}

	promptInput := a.buildImproveVolumePromptInput(input)
	if err := a.base.Execute(ctx, params, promptInput, &output.Volume); err != nil {
		return ComposeImproveVolumeOutput{}, err
	}

	// Validate the improved volume
	if len(output.Volume.Chapters) == 0 {
		return ComposeImproveVolumeOutput{}, fmt.Errorf("improved volume has no chapters")
	}
	if expected := len(input.Volume.Chapters); expected > 0 && len(output.Volume.Chapters) != expected {
		logger.Warn("Improved volume returned %d chapters, expected %d; preserving original chapter slots", len(output.Volume.Chapters), expected)
		if len(output.Volume.Chapters) > expected {
			output.Volume.Chapters = output.Volume.Chapters[:expected]
		}
		for len(output.Volume.Chapters) < expected {
			idx := len(output.Volume.Chapters)
			output.Volume.Chapters = append(output.Volume.Chapters, input.Volume.Chapters[idx])
		}
	}

	// Validate each chapter
	for i, chapter := range output.Volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	// Preserve original volume ID and chapter IDs to maintain consistency
	originalVolumeID := input.Volume.ID
	output.Volume.ID = originalVolumeID

	// Preserve original chapter IDs
	for i := range output.Volume.Chapters {
		if i < len(input.Volume.Chapters) {
			output.Volume.Chapters[i].ID = input.Volume.Chapters[i].ID
		}
	}

	logger.Info("✓ Volume improved: %s (%d chapters)", output.Volume.Title, len(output.Volume.Chapters))

	return output, nil
}

func (a *ComposeAgent) buildVolumeImproveContext(outline *models.Outline, partID, volumeID string) string {
	if outline == nil {
		return "No outline context provided."
	}

	var b strings.Builder
	for partIdx, part := range outline.Parts {
		for volIdx, volume := range part.Volumes {
			if volume.ID != volumeID {
				continue
			}

			b.WriteString(fmt.Sprintf("Current Part: %s (%s)\nSummary: %s\n", part.Title, prompt.NonEmpty(part.ID, partID), prompt.ClipForPrompt(part.Summary, 650)))
			b.WriteString(fmt.Sprintf("Current Volume: %s (%s), volume %d/%d\nSummary: %s\n",
				volume.Title, volume.ID, volIdx+1, len(part.Volumes), prompt.ClipForPrompt(volume.Summary, 650)))

			if volIdx > 0 {
				prevVol := part.Volumes[volIdx-1]
				b.WriteString("\nPrevious Volume:\n")
				b.WriteString(formatVolumeBrief(prevVol))
			}
			if volIdx < len(part.Volumes)-1 {
				nextVol := part.Volumes[volIdx+1]
				b.WriteString("\nNext Volume:\n")
				b.WriteString(formatVolumeBrief(nextVol))
			}

			if len(volume.Chapters) > 0 {
				b.WriteString("\nTarget Volume Chapter Map:\n")
				for _, ch := range volume.Chapters {
					b.WriteString(formatChapterBrief(ch))
				}
			}

			if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				b.WriteString("\nPrevious Part Summary:\n")
				b.WriteString(fmt.Sprintf("- %s: %s\n", prevPart.Title, prompt.ClipForPrompt(prevPart.Summary, 350)))
			}
			if partIdx < len(outline.Parts)-1 {
				nextPart := outline.Parts[partIdx+1]
				b.WriteString("\nNext Part Summary:\n")
				b.WriteString(fmt.Sprintf("- %s: %s\n", nextPart.Title, prompt.ClipForPrompt(nextPart.Summary, 350)))
			}

			return b.String()
		}
	}

	return "Target volume was not found in the outline context."
}

// ComposeImproveReportEntry is a per-volume improvement summary recorded during
// Agent SDK outline improvement. The CLI aggregates these into an end-of-run
// Markdown report.
type ComposeImproveReportEntry struct {
	Iteration           int     `json:"iteration"`
	VolumeID            string  `json:"volume_id"`
	VolumeTitle         string  `json:"volume_title"`
	Score               float64 `json:"score"`
	Changed             bool    `json:"changed"`
	Skipped             bool    `json:"skipped"`
	Summary             string  `json:"summary"`
	RemainingMediumPlus int     `json:"remaining_medium_plus"`
}

// composeImproveRoot returns the project the improve loop checkpoints into.
// Compose agents are constructed with an explicit root so checkpointing can
// never drift to the process's ambient project.
func (a *ComposeAgent) composeImproveRoot() (*store.Root, error) {
	if a != nil && a.project != nil && a.project.Dir() != "" {
		return a.project, nil
	}
	return nil, fmt.Errorf("resolve project root for compose improve progress: no project root bound")
}

var (
	composeChineseVolumeListPattern = regexp.MustCompile(`第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷(?:\s*(?:和|及|与|、|,|，)\s*第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷)+`)
	composeChineseVolumePattern     = regexp.MustCompile(`第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷`)
	composeVolumeIDListPattern      = regexp.MustCompile(`P\d+-V\d+(?:\s*(?:和|及|与|、|,|，)\s*P\d+-V\d+)+`)
)

func preserveImprovedVolumeIdentity(original *models.Volume, improved *models.Volume) {
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

// BuildVolumeContext builds context for volume regeneration
func (a *ComposeAgent) BuildVolumeContext(volume *models.Volume, outline *models.Outline) string {
	var context strings.Builder

	// Find volume in outline
	for _, part := range outline.Parts {
		for i, vol := range part.Volumes {
			if vol.ID == volume.ID {
				// Add part context
				context.WriteString(fmt.Sprintf("Part: %s\nSummary: %s\n\n", part.Title, part.Summary))

				// Add sibling volumes
				if i > 0 {
					prevVol := part.Volumes[i-1]
					context.WriteString(fmt.Sprintf("Previous Volume (%s): %s\nSummary: %s\n\n",
						prevVol.ID, prevVol.Title, prevVol.Summary))
				}
				if i < len(part.Volumes)-1 {
					nextVol := part.Volumes[i+1]
					context.WriteString(fmt.Sprintf("Next Volume (%s): %s\nSummary: %s\n\n",
						nextVol.ID, nextVol.Title, nextVol.Summary))
				}
				return context.String()
			}
		}
	}

	return context.String()
}

// validateOutlineStructure validates the outline matches the expected structure
func (a *ComposeAgent) validateOutlineStructure(outline *models.Outline, structure models.StoryStructure) error {
	if len(outline.Parts) != structure.TargetParts {
		logger.Error("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
		return fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	return nil
}

func (a *ComposeAgent) buildContinuitySnapshot(outline *models.Outline, partIdx, volIdx int) string {
	if outline == nil {
		return ""
	}

	var chapters []models.Chapter
	for p := 0; p <= partIdx && p < len(outline.Parts); p++ {
		for v := 0; v < len(outline.Parts[p].Volumes); v++ {
			if p == partIdx && v >= volIdx {
				break
			}
			chapters = append(chapters, outline.Parts[p].Volumes[v].Chapters...)
		}
	}
	if len(chapters) == 0 {
		return ""
	}

	lastChapter := chapters[len(chapters)-1]
	resources := map[string]int{}
	openMysteries := map[string]string{}
	activeBosses := map[string]string{}
	recentStorylines := map[string]string{}

	for _, chapter := range chapters {
		for _, entry := range chapter.ResourceLedger {
			item := strings.TrimSpace(entry.Item)
			if item != "" {
				resources[item] = entry.End
			}
		}
		for _, planted := range chapter.Mysteries.Planted {
			id := strings.TrimSpace(planted.ID)
			if id != "" {
				openMysteries[id] = prompt.ClipForPrompt(planted.Clue, 120)
			}
		}
		for _, resolved := range chapter.Mysteries.Resolved {
			delete(openMysteries, strings.TrimSpace(resolved.ID))
		}
		for _, enemy := range chapter.Enemies {
			if !enemy.IsBoss && strings.TrimSpace(enemy.BossID) == "" {
				continue
			}
			id := strings.TrimSpace(enemy.BossID)
			if id == "" {
				id = strings.TrimSpace(enemy.Name)
			}
			if id == "" {
				continue
			}
			status := strings.TrimSpace(enemy.Status)
			if status == "" {
				status = "engaged"
			}
			if status == "defeated" {
				delete(activeBosses, id)
			} else {
				activeBosses[id] = fmt.Sprintf("%s in %s", status, chapter.ID)
			}
		}
		for _, advance := range chapter.StorylineAdvances {
			name := strings.TrimSpace(advance.StorylineName)
			if name == "" {
				continue
			}
			recentStorylines[name] = fmt.Sprintf("%s: %s%s%s",
				prompt.NonEmpty(advance.Stage, "progress"),
				prompt.ClipForPrompt(advance.Change, 120),
				compactLabel("pressure", advance.Pressure),
				compactLabel("consequence", advance.Consequence))
		}
	}

	var b strings.Builder
	b.WriteString("\n=== CONTINUITY SNAPSHOT ===\n")
	b.WriteString(fmt.Sprintf("Last generated chapter: %s %s - %s\n",
		lastChapter.ID, lastChapter.Title, prompt.ClipForPrompt(lastChapter.Summary, 220)))
	if beats := lastChapter.GetBeats(); len(beats) > 0 {
		b.WriteString(fmt.Sprintf("Last closing beat: %s\n", prompt.ClipForPrompt(beats[len(beats)-1], 180)))
	}
	if lastChapter.StateAnchor.Cultivation != "" || lastChapter.StateAnchor.Location != "" || len(lastChapter.StateAnchor.Allies) > 0 || len(lastChapter.StateAnchor.Injuries) > 0 || len(lastChapter.StateAnchor.KeyItems) > 0 {
		b.WriteString(fmt.Sprintf("Carry-forward state: cultivation=%s; location=%s; allies=%s; injuries=%s; key_items=%s; notes=%s\n",
			prompt.ClipForPrompt(lastChapter.StateAnchor.Cultivation, 120),
			prompt.ClipForPrompt(lastChapter.StateAnchor.Location, 120),
			joinLimited(lastChapter.StateAnchor.Allies, 6),
			joinLimited(lastChapter.StateAnchor.Injuries, 6),
			joinLimited(lastChapter.StateAnchor.KeyItems, 8),
			prompt.ClipForPrompt(lastChapter.StateAnchor.Notes, 180)))
	}
	if len(resources) > 0 {
		b.WriteString("Resource ledger endings: ")
		b.WriteString(formatIntMap(resources, 8))
		b.WriteString("\n")
	}
	if len(openMysteries) > 0 {
		b.WriteString("Open mysteries to preserve: ")
		b.WriteString(formatStringMap(openMysteries, 6))
		b.WriteString("\n")
	}
	if len(activeBosses) > 0 {
		b.WriteString("Active boss continuity: ")
		b.WriteString(formatStringMap(activeBosses, 6))
		b.WriteString("\n")
	}
	if len(recentStorylines) > 0 {
		b.WriteString("Recent storyline pressure/payoff traces: ")
		b.WriteString(formatStringMap(recentStorylines, 8))
		b.WriteString("\n")
	}
	b.WriteString("Use this snapshot as the starting state for the next generated volume; do not reset resources, injuries, mysteries, bosses, or storyline pressure without an explicit event.\n\n")
	return b.String()
}
