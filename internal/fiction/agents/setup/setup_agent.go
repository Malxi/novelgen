package setup

import (
	"context"
	"fmt"
	agentbase "novelgen/internal/fiction/agents/base"
	"novelgen/internal/fiction/agents/prompt"
	"novelgen/internal/fiction/agents/review"
	"novelgen/internal/fiction/domain/creativity"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

// SetupGenInput is the input for setup generation
type SetupGenInput struct {
	Idea                string                   `json:"idea" md:"idea" desc:"User's story idea or concept"`
	CreativeProvocation *creativity.Provocation  `json:"creative_provocation,omitempty" md:"creative_provocation,omitempty" desc:"Optional deterministic structured-nonsense constraint that must be rationalized into world canon"`
	CreativeScenario    *creativity.ScenarioCard `json:"creative_scenario,omitempty" md:"creative_scenario,omitempty" desc:"Optional fixed causal scenario card. Treat every field as a binding role and expand it into setup without changing the causal chain"`
}

// SetupGenOutput is the output for setup generation
type SetupGenOutput struct {
	Setup models.StorySetup `json:"setup" md:"setup" desc:"Generated story setup with premise, genres, themes, rules"`
}

// SetupImproveInput is the input for setup improvement
type SetupImproveInput struct {
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	ExistingSetup   models.StorySetup   `json:"existing_setup" md:"existing_setup" desc:"Current story setup to improve"`
	RevisionContext string              `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier generation, review, and improve rounds"`
}

// SetupImproveOutput is the output for setup improvement
type SetupImproveOutput struct {
	Setup models.StorySetup `json:"setup" md:"setup" desc:"Improved story setup"`
}

// SetupReviewInput is the input for setup review
type SetupReviewInput struct {
	ExistingSetup models.StorySetup `json:"existing_setup" md:"existing_setup" desc:"Story setup to review"`
}

// SetupReviewOutput is the output for setup review
type SetupReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// SetupCausalReviewInput asks a strict reviewer whether the generated setup is
// one axiom plus derived consequences, rather than a pile of unrelated cool
// rules added to explain the provocation.
type SetupCausalReviewInput struct {
	Idea          string                   `json:"idea" md:"idea" desc:"Original user idea or constraints"`
	Provocation   creativity.Provocation   `json:"provocation" md:"provocation" desc:"Original structured-nonsense constraint"`
	Scenario      *creativity.ScenarioCard `json:"creative_scenario,omitempty" md:"creative_scenario,omitempty" desc:"Optional fixed causal Scenario Card to audit"`
	ExistingSetup models.StorySetup        `json:"existing_setup" md:"existing_setup" desc:"Generated setup to audit"`
}

// SetupCausalReviewOutput is diagnosis-only; it does not rewrite the setup.
type SetupCausalReviewOutput struct {
	AxiomCount         int      `json:"axiom_count" md:"axiom_count" desc:"Number of independent unexplained magic rules; must be 1"`
	OrphanRules        []string `json:"orphan_rules,omitempty" md:"orphan_rules" desc:"Rules that cannot be derived from the axiom plus normal incentives"`
	MissingBeneficiary []string `json:"missing_beneficiary,omitempty" md:"missing_beneficiary" desc:"Institutions with no beneficiary, budget, or enforcement capacity"`
	ZeroCostBypasses   []string `json:"zero_cost_bypasses,omitempty" md:"zero_cost_bypasses" desc:"Rules an ordinary person can ignore at zero cost"`
	UnseededTwists     []string `json:"unseeded_twists,omitempty" md:"unseeded_twists" desc:"Reveals with no earlier canon fact or clue"`
	Verdict            string   `json:"verdict" md:"verdict" desc:"keep or reject"`
	Summary            string   `json:"summary" md:"summary" desc:"Short diagnosis under 300 characters"`
}

// ScenarioPredictionInput asks a cheap predictor to imagine the most obvious
// developments of a Scenario Card. The output is used to rerank cards before
// full setup generation.
type ScenarioPredictionInput struct {
	Idea     string                  `json:"idea" md:"idea" desc:"Original user idea or constraints"`
	Scenario creativity.ScenarioCard `json:"creative_scenario" md:"creative_scenario" desc:"Scenario Card to predict"`
}

// ScenarioPredictionOutput is a card-level novelty estimate.
type ScenarioPredictionOutput struct {
	Predictability      int      `json:"predictability" md:"predictability" desc:"0-100; higher means the card strongly implies familiar developments"`
	Novelty             int      `json:"novelty" md:"novelty" desc:"0-100; higher means less predictable"`
	ObviousDevelopments []string `json:"obvious_developments,omitempty" md:"obvious_developments" desc:"3-5 developments a generic model would predict"`
	Summary             string   `json:"summary" md:"summary" desc:"Short explanation under 200 characters"`
}

// NormalizedNovelty returns the novelty implied by predictability when the
// model omits the explicit novelty field.
func (o ScenarioPredictionOutput) NormalizedNovelty() int {
	if o.Novelty > 0 {
		return clampScenarioScore(o.Novelty)
	}
	return 100 - clampScenarioScore(o.Predictability)
}

func clampScenarioScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// Rejected reports whether the candidate fails the causal contract.
func (r SetupCausalReviewOutput) Rejected() bool {
	verdict := strings.ToLower(strings.TrimSpace(r.Verdict))
	if verdict == "reject" || verdict == "fail" || verdict == "invalid" {
		return true
	}
	return r.AxiomCount > 1 ||
		len(r.OrphanRules) > 0 ||
		len(r.MissingBeneficiary) > 0 ||
		len(r.ZeroCostBypasses) > 0 ||
		len(r.UnseededTwists) > 0
}

// Issues flattens the diagnosis into short strings for candidate reports.
func (r SetupCausalReviewOutput) Issues() []string {
	issues := make([]string, 0,
		len(r.OrphanRules)+len(r.MissingBeneficiary)+len(r.ZeroCostBypasses)+len(r.UnseededTwists))
	for _, item := range r.OrphanRules {
		issues = append(issues, "orphan_rule: "+item)
	}
	for _, item := range r.MissingBeneficiary {
		issues = append(issues, "missing_beneficiary: "+item)
	}
	for _, item := range r.ZeroCostBypasses {
		issues = append(issues, "zero_cost_bypass: "+item)
	}
	for _, item := range r.UnseededTwists {
		issues = append(issues, "unseeded_twist: "+item)
	}
	return issues
}

// SetupAgentSDKImproveInput is the compact prompt payload for the SDK workflow.
// The agent must query project facts through tools instead of receiving the
// entire story setup in prompt context.
type SetupAgentSDKImproveInput struct {
	ReviewResult     models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Quality/simulation findings or user guidance relevant to setup"`
	UserPrompt       string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user guidance"`
	ApplyPatches     bool                `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch setup --apply after a successful dry-run"`
	ForceIssueRepair bool                `json:"force_issue_repair" md:"force_issue_repair" desc:"When true, focused review suggestions are an explicit repair task list, including directly fixable low-priority issues"`
	RequiredQueries  []string            `json:"required_queries" md:"required_queries" desc:"novelgen tool commands that must be used before answering"`
	Instructions     []string            `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type setupAgentSDKImprovePatchOutput struct {
	ReviewResult review.SDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for the setup after the returned patch"`
	SetupPatch   map[string]interface{} `json:"setup_patch" md:"setup_patch" desc:"Minimal JSON object patch for story setup. Include only changed top-level setup fields."`
}

// SetupAgentSDKImproveOutput is returned to the CLI, which remains responsible
// for merge, validation, normalization, and saving unless the patch tool already
// applied the change.
type SetupAgentSDKImproveOutput struct {
	ReviewResult models.ReviewResult
	SetupPatch   map[string]interface{}
}

// SetupAgent handles AI generation for story setup
// It wraps BaseAgent to provide type-safe methods
type SetupAgent struct {
	base *agentbase.BaseAgent
}

// NewSetupAgent creates a new SetupAgent
func NewSetupAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *SetupAgent {
	base := agentbase.NewBaseAgent(agentbase.BaseAgentConfig{
		Name:       "SetupAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &SetupAgent{base: base}
}

// SetLanguage sets the output language
func (a *SetupAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// SetProjectRoot binds the agent to an explicit project root.
func (a *SetupAgent) SetProjectRoot(root *store.Root) {
	if a != nil && a.base != nil {
		a.base.SetProjectRoot(root)
	}
}

// Generate creates a story setup from an idea
// This is the type-safe wrapper around BaseAgent.Execute
func (a *SetupAgent) Generate(ctx context.Context, input SetupGenInput) (SetupGenOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Generation")
	logger.Info("Idea: %s", input.Idea)
	if input.CreativeProvocation != nil {
		logger.Info("Creative provocation [%s]: %s", input.CreativeProvocation.ID, input.CreativeProvocation.Sentence)
	}
	logger.Info("Language: %s", a.base.Language())

	var output SetupGenOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"setup-gen"},
		Command: "generate a story setup",
	}
	if err := a.base.Execute(ctx, params, input, &output.Setup); err != nil {
		return SetupGenOutput{}, err
	}

	// Normalize and validate
	a.normalizeSetup(&output.Setup)

	// Log result
	logger.Section("Story Setup Result")
	logger.Info("Project Name: %s", output.Setup.ProjectName)
	logger.Info("Genres: %v", output.Setup.Genres)
	logger.Info("Theme: %s", output.Setup.Theme)
	logger.Info("Tone: %s", output.Setup.Tone)

	return output, nil
}

// GenerateWithAgentSDK generates a setup through the Agent SDK runtime. The
// SDK workflow has no project tools: its only job is to turn the idea and the
// optional provocation into a typed setup JSON, which Go normalizes and
// validates before the caller writes anything.
func (a *SetupAgent) GenerateWithAgentSDK(ctx context.Context, input SetupGenInput) (SetupGenOutput, error) {
	logger.Section("SETUP AGENT SDK - Story Setup Generation")
	logger.Info("Idea: %s", input.Idea)
	if input.CreativeProvocation != nil {
		logger.Info("Creative provocation [%s]: %s", input.CreativeProvocation.ID, input.CreativeProvocation.Sentence)
	}
	logger.Info("Language: %s", a.base.Language())

	var output SetupGenOutput
	params := agentbase.InvokeParams{
		SDKSkills:      []string{"setup-generate-workflow"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		MaxTurns:       8,
		Timeout:        600,
		Command:        "generate a story setup from the idea and creative provocation",
	}
	if err := a.base.Execute(ctx, params, input, &output.Setup); err != nil {
		return SetupGenOutput{}, err
	}
	a.normalizeSetup(&output.Setup)
	return output, nil
}

// Improve improves an existing story setup
func (a *SetupAgent) Improve(ctx context.Context, input SetupImproveInput) (SetupImproveOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Improvement")
	logger.Info("Project: %s", input.ExistingSetup.ProjectName)
	logger.Info("Language: %s", a.base.Language())

	var output SetupImproveOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"setup-improve"},
		Command: "improve the story setup",
	}
	if err := a.base.Execute(ctx, params, input, &output.Setup); err != nil {
		return SetupImproveOutput{}, err
	}

	// Normalize and validate
	a.normalizeSetup(&output.Setup)

	return output, nil
}

// ImproveWithAgentSDK lets the SDK workflow query setup facts, run setup checks,
// validate a patch through tool patch setup, and return the minimal patch.
func (a *SetupAgent) ImproveWithAgentSDK(ctx context.Context, input SetupAgentSDKImproveInput) (SetupAgentSDKImproveOutput, error) {
	logger.Section("SETUP AGENT SDK - Story Setup Improvement")
	if input.ApplyPatches {
		logger.Info("Agent apply enabled: SDK may write through validated setup patch tools")
	}
	if input.ForceIssueRepair {
		logger.Info("Focused setup review issues are treated as repair tasks")
	}

	var output setupAgentSDKImprovePatchOutput
	params := setupAgentSDKParams(input.ApplyPatches)
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return SetupAgentSDKImproveOutput{}, err
	}
	if strings.TrimSpace(output.ReviewResult.Summary) == "" && output.ReviewResult.OverallScore == 0 && len(output.ReviewResult.Suggestions) == 0 {
		return SetupAgentSDKImproveOutput{}, fmt.Errorf("agent SDK setup improve output has empty review_result")
	}
	if output.SetupPatch == nil {
		output.SetupPatch = map[string]interface{}{}
	}
	if err := utils.ValidateNoSuspiciousPatchText(output.SetupPatch); err != nil {
		return SetupAgentSDKImproveOutput{}, fmt.Errorf("agent SDK setup patch rejected: %w", err)
	}
	review := output.ReviewResult.ToModelReviewResult()
	review.NormalizeScoreScale()
	return SetupAgentSDKImproveOutput{
		ReviewResult: review,
		SetupPatch:   output.SetupPatch,
	}, nil
}

// Review reviews an existing story setup and provides improvement suggestions
func (a *SetupAgent) Review(ctx context.Context, input SetupReviewInput) (SetupReviewOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Review")
	logger.Info("Project: %s", input.ExistingSetup.ProjectName)
	logger.Info("Language: %s", a.base.Language())

	var output SetupReviewOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"setup-review"},
		Command: "review the story setup and provide improvement suggestions",
	}
	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return SetupReviewOutput{}, err
	}

	// Log result
	logger.Section("Setup Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// ReviewWithAgentSDK runs the general setup attractiveness/compactness review
// through the Agent SDK skill tree.
func (a *SetupAgent) ReviewWithAgentSDK(ctx context.Context, input SetupReviewInput) (SetupReviewOutput, error) {
	logger.Section("SETUP AGENT SDK - Story Setup Review")
	logger.Info("Project: %s", input.ExistingSetup.ProjectName)

	var output SetupReviewOutput
	params := agentbase.InvokeParams{
		SDKSkills:      []string{"setup-review-workflow"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		MaxTurns:       6,
		Timeout:        300,
		Command:        "review the story setup and provide improvement suggestions",
	}
	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return SetupReviewOutput{}, err
	}
	return output, nil
}

// ReviewCausality runs the strict one-axiom / derivation audit used by
// `setup explore`. It is a separate pass from the general setup review because
// a setup can score well on attractiveness and compactness while still being a
// pile of unrelated exceptions.
func (a *SetupAgent) ReviewCausality(ctx context.Context, input SetupCausalReviewInput) (SetupCausalReviewOutput, error) {
	logger.Section("SETUP AGENT - Causal Review")
	logger.Info("Provocation: %s", input.Provocation.Sentence)
	if input.Scenario != nil {
		logger.Info("Scenario: %s", input.Scenario.PremiseKernel)
	}
	logger.Info("Idea: %s", input.Idea)

	var output SetupCausalReviewOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"setup-causal-review"},
		Command: "audit whether the setup derives from one axiom without orphan rules",
	}
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return SetupCausalReviewOutput{}, err
	}
	output.Verdict = strings.TrimSpace(output.Verdict)
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Verdict == "" && output.Summary == "" && output.AxiomCount == 0 && len(output.Issues()) == 0 {
		return SetupCausalReviewOutput{}, fmt.Errorf("causal review returned no diagnosis")
	}
	return output, nil
}

// ReviewCausalityWithAgentSDK runs the same strict audit through the Agent SDK
// skill tree, so `setup explore --agent-sdk` does not silently fall back to the
// chat backend for its causal reviewer.
func (a *SetupAgent) ReviewCausalityWithAgentSDK(ctx context.Context, input SetupCausalReviewInput) (SetupCausalReviewOutput, error) {
	logger.Section("SETUP AGENT SDK - Causal Review")
	logger.Info("Provocation: %s", input.Provocation.Sentence)
	if input.Scenario != nil {
		logger.Info("Scenario: %s", input.Scenario.PremiseKernel)
	}
	logger.Info("Idea: %s", input.Idea)

	var output SetupCausalReviewOutput
	params := agentbase.InvokeParams{
		SDKSkills:      []string{"setup-causal-review"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		MaxTurns:       4,
		Timeout:        300,
		Command:        "audit whether the setup derives from one axiom without orphan rules",
	}
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return SetupCausalReviewOutput{}, err
	}
	output.Verdict = strings.TrimSpace(output.Verdict)
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Verdict == "" && output.Summary == "" && output.AxiomCount == 0 && len(output.Issues()) == 0 {
		return SetupCausalReviewOutput{}, fmt.Errorf("causal review returned no diagnosis")
	}
	return output, nil
}

// PredictScenario asks the chat backend for the obvious developments of a card.
func (a *SetupAgent) PredictScenario(ctx context.Context, input ScenarioPredictionInput) (ScenarioPredictionOutput, error) {
	var output ScenarioPredictionOutput
	params := agentbase.InvokeParams{
		Skills:  []string{"scenario-predict"},
		Command: "predict the most obvious developments of this Scenario Card",
	}
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return ScenarioPredictionOutput{}, err
	}
	if output.Summary == "" && len(output.ObviousDevelopments) == 0 {
		return ScenarioPredictionOutput{}, fmt.Errorf("scenario predictor returned no diagnosis")
	}
	return output, nil
}

// PredictScenarioWithAgentSDK is the SDK equivalent used by exploration with
// --agent-sdk.
func (a *SetupAgent) PredictScenarioWithAgentSDK(ctx context.Context, input ScenarioPredictionInput) (ScenarioPredictionOutput, error) {
	var output ScenarioPredictionOutput
	params := agentbase.InvokeParams{
		SDKSkills:      []string{"scenario-predict"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		MaxTurns:       4,
		Timeout:        300,
		Command:        "predict the most obvious developments of this Scenario Card",
	}
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return ScenarioPredictionOutput{}, err
	}
	if output.Summary == "" && len(output.ObviousDevelopments) == 0 {
		return ScenarioPredictionOutput{}, fmt.Errorf("scenario predictor returned no diagnosis")
	}
	return output, nil
}

func setupAgentSDKParams(applyPatches bool) agentbase.InvokeParams {
	patchTool := "novelgen tool patch setup"
	if applyPatches {
		patchTool += " --apply"
	}
	evidence := agentbase.ToolEvidenceRequirement{MinQueryCalls: 1, MinCheckCalls: 1}
	if applyPatches {
		evidence.MinPatchApplyCalls = 1
		evidence.RequirePatchApplyFollowupCheck = true
	}
	toolAllowlist := []string{
		"novelgen tool query story-setup --view brief",
		"novelgen tool query story-setup --type search",
		"novelgen tool query story-setup --type core-cast",
		"novelgen tool query story-setup --type storyline",
		"novelgen tool query story-setup --type premise",
		"novelgen tool query story-setup --type resource",
		"novelgen tool query story-setup --type timeline",
		"novelgen tool query ideas --type axiom",
		"novelgen tool query ideas --type event",
		"novelgen tool query ideas --type law",
		"novelgen tool check all --target setup --min-priority medium --max-issues 12",
		"novelgen tool check all --target setup --category",
		patchTool,
	}
	toolAllowlist = append(toolAllowlist, agentbase.AgentSDKLogToolAllowlist()...)
	return agentbase.AgentSDKToolParams(
		[]string{"novel-tools", "setup-improve-workflow"},
		"review and improve story setup using project query/check/patch tools",
		toolAllowlist,
		evidence,
		22,
		900,
	)
}

func BuildSetupAgentSDKImprovePromptInput(review models.ReviewResult, userPrompt string, applyPatches bool, forceIssueRepair bool) SetupAgentSDKImproveInput {
	instructions := []string{
		"Use the required story-setup query and setup check before reviewing.",
		"Only use story-setup queries and setup checks; do not request full project files, outline, craft, chapters, RPG files, or inspect source code.",
		"Treat review_result.suggestions as the primary task list. If force_issue_repair=true, directly fixable low-priority issues should still be patched or explicitly justified as false positives with queried facts.",
		"If you build a non-empty setup_patch, first dry-run it with `printf '%s' '<compact-json>' | novelgen tool patch setup`. " + agentbase.AgentSDKNonASCIIPatchRule("novelgen tool patch setup", false),
	}
	if applyPatches {
		instructions = append(instructions,
			"If dry-run passes, repeat the same stdin-piped setup patch command with `--apply`, then run `novelgen tool check all --target setup --min-priority medium --max-issues 12`.",
			"Do not write files by any other method; "+agentbase.AgentSDKOnlyAllowedWriteClause("novelgen tool patch setup ..."),
		)
	} else {
		instructions = append(instructions,
			"Do not use --apply. Return the final setup_patch and Go will merge, validate, normalize, checkpoint, and save it.",
		)
	}
	instructions = append(instructions,
		"Return only JSON with review_result and setup_patch. setup_patch must contain only changed top-level setup fields.",
		"If no changes are needed, return an empty setup_patch object and explain the remaining false positives or non-blocking issues in review_result.",
	)
	return SetupAgentSDKImproveInput{
		ReviewResult:     prompt.CompactReviewForPrompt(review),
		UserPrompt:       userPrompt,
		ApplyPatches:     applyPatches,
		ForceIssueRepair: forceIssueRepair,
		RequiredQueries: []string{
			"novelgen tool query story-setup --view brief",
			"novelgen tool check all --target setup --min-priority medium --max-issues 12",
		},
		Instructions: instructions,
	}
}

// Iterate runs the review-improvement loop for story setup
// It directly uses Review and Improve methods
func (a *SetupAgent) Iterate(ctx context.Context, setup *models.StorySetup, maxIterations int, qualityThreshold float64, forceImprove bool) (*models.StorySetup, *models.ReviewResult, error) {
	logger.Section("SETUP AGENT - Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	currentSetup := *setup
	var finalReview *models.ReviewResult
	session := review.NewRevisionSession("setup", "Improve story setup until review feedback is resolved without losing the core concept.")

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review current setup
		reviewInput := SetupReviewInput{ExistingSetup: currentSetup}
		reviewOutput, err := a.Review(ctx, reviewInput)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}

		reviewOutput.Result.Iteration = i
		session.AddReview(i, reviewOutput.Result)
		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}

		// Determine if we should improve
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Improve the setup with review feedback
		improveInput := SetupImproveInput{
			ExistingSetup:   currentSetup,
			ReviewResult:    prompt.CompactReviewForPrompt(reviewOutput.Result),
			RevisionContext: session.Prompt(),
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentSetup = improveOutput.Setup
		session.AddImprove(i, "Applied the previous review feedback to the setup; next review should verify resolved issues and preserve existing strengths.")
		logger.Info("Setup improved based on review suggestions")
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
		logger.Info("✓ Setup improved based on review suggestions")
	}

	return &currentSetup, finalReview, nil
}

// normalizeSetup normalizes and validates the setup fields
func (a *SetupAgent) normalizeSetup(setup *models.StorySetup) {
	setup.ProjectName = strings.TrimSpace(setup.ProjectName)
	setup.Premise = strings.TrimSpace(setup.Premise)
	setup.Theme = strings.TrimSpace(setup.Theme)
	setup.TargetAudience = strings.TrimSpace(setup.TargetAudience)
	setup.Tone = strings.TrimSpace(setup.Tone)
	setup.Tense = strings.TrimSpace(setup.Tense)
	setup.POVStyle = strings.TrimSpace(setup.POVStyle)
	setup.WritingStyle = setup.WritingStyle.CompactReference(len([]rune(setup.WritingStyle.ReferenceExcerpt)))
	models.NormalizeStorySetup(setup)
	for i := range setup.Storylines {
		setup.Storylines[i].Name = strings.TrimSpace(setup.Storylines[i].Name)
		setup.Storylines[i].Description = strings.TrimSpace(setup.Storylines[i].Description)
		setup.Storylines[i].Type = strings.TrimSpace(setup.Storylines[i].Type)
		setup.Storylines[i].Scope = strings.TrimSpace(setup.Storylines[i].Scope)
		setup.Storylines[i].PayoffStyle = strings.TrimSpace(setup.Storylines[i].PayoffStyle)
		setup.Storylines[i].SetupRole = strings.TrimSpace(setup.Storylines[i].SetupRole)
		setup.Storylines[i].Desire = strings.TrimSpace(setup.Storylines[i].Desire)
		setup.Storylines[i].Opposition = strings.TrimSpace(setup.Storylines[i].Opposition)
		setup.Storylines[i].Stakes = strings.TrimSpace(setup.Storylines[i].Stakes)
		setup.Storylines[i].Turn = strings.TrimSpace(setup.Storylines[i].Turn)
		setup.Storylines[i].Payoff = strings.TrimSpace(setup.Storylines[i].Payoff)
		setup.Storylines[i].OpenQuestion = strings.TrimSpace(setup.Storylines[i].OpenQuestion)
		var pressurePoints []string
		for _, point := range setup.Storylines[i].PressurePoints {
			point = strings.TrimSpace(point)
			if point != "" {
				pressurePoints = append(pressurePoints, point)
			}
		}
		setup.Storylines[i].PressurePoints = pressurePoints
	}

	if setup.ProjectName == "" {
		logger.Warn("AI did not generate project name, using default")
		setup.ProjectName = "Untitled Novel"
	}

	// Validate tense (support both English and Chinese)
	validTenses := []string{
		"past", "过去", "过去时",
		"present", "现在", "现在时",
	}
	tenseValid := false
	for _, valid := range validTenses {
		if setup.Tense == valid {
			tenseValid = true
			break
		}
	}
	if setup.Tense != "" && !tenseValid {
		logger.Warn("Invalid tense value '%s', clearing", setup.Tense)
		setup.Tense = ""
	}

	// Validate POV style (support both English and Chinese)
	validPOVStyles := []string{
		"first person", "第一人称",
		"third person limited", "第三人称有限", "第三人称限制", "第三人称有限视角",
		"third person omniscient", "第三人称全知", "第三人称上帝视角", "第三人称全知视角",
	}
	povValid := false
	for _, valid := range validPOVStyles {
		if setup.POVStyle == valid {
			povValid = true
			break
		}
	}
	if setup.POVStyle != "" && !povValid {
		logger.Warn("Invalid POV style value '%s', clearing", setup.POVStyle)
		setup.POVStyle = ""
	}
}
