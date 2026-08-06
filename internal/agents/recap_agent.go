package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
)

// RecapExtractInput is the input for recap extraction
type RecapExtractInput struct {
	ChapterID       string   `json:"chapter_id" md:"chapter_id" desc:"Chapter ID"`
	Title           string   `json:"title" md:"title" desc:"Chapter title"`
	ChapterText     string   `json:"chapter_text" md:"chapter_text" desc:"Full chapter text to extract recap from"`
	Feedback        string   `json:"feedback,omitempty" md:"feedback,omitempty" desc:"Optional feedback for recap extraction"`
	ApplyPatches    bool     `json:"apply_patches,omitempty" md:"apply_patches,omitempty" desc:"Whether the workflow may apply a validated recap patch"`
	RequiredQueries []string `json:"required_queries,omitempty" md:"required_queries,omitempty" desc:"Tool queries/checks the SDK workflow should run before patching"`
	Instructions    []string `json:"instructions,omitempty" md:"instructions,omitempty" desc:"Additional Agent SDK workflow instructions"`
}

// RecapExtractOutput is the output for recap extraction
type RecapExtractOutput struct {
	Recap models.ChapterRecap `json:"recap" md:"recap" desc:"Extracted chapter recap with character states and events"`
}

// RecapAgent extracts a canonical recap JSON from chapter text
// It wraps BaseAgent to provide type-safe methods
type RecapAgent struct {
	base *BaseAgent
}

// NewRecapAgent creates a new RecapAgent
func NewRecapAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *RecapAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "RecapAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &RecapAgent{
		base: base,
	}
}

// SetLanguage sets the output language
func (a *RecapAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// Extract extracts a recap from chapter text
func (a *RecapAgent) Extract(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	return a.ExtractWithFeedback(ctx, chapterID, title, chapterText, "")
}

// ExtractWithFeedback extracts a recap and optionally provides structured feedback
// to force the model to fill missing fields (minimal gate).
func (a *RecapAgent) ExtractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	params := InvokeParams{
		Skills:  []string{"recap-extract"},
		Command: "extract chapter recap",
	}
	return a.extractWithFeedback(ctx, chapterID, title, chapterText, feedback, params)
}

// ExtractWithAgentSDK extracts a recap through the Claude Agent SDK workflow.
// The agent only returns structured JSON; Go still validates and saves it.
func (a *RecapAgent) ExtractWithAgentSDK(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	return a.ExtractWithFeedbackAgentSDK(ctx, chapterID, title, chapterText, "")
}

// ExtractWithAgentSDKApply extracts a recap through Agent SDK and optionally
// lets the agent write it via validated recap patch tools.
func (a *RecapAgent) ExtractWithAgentSDKApply(ctx context.Context, chapterID, title string, chapterText string, applyPatches bool) (*models.ChapterRecap, error) {
	return a.extractWithFeedbackAgentSDK(ctx, chapterID, title, chapterText, "", applyPatches)
}

// ExtractWithFeedbackAgentSDK extracts a recap through Agent SDK and optional
// deterministic feedback for retry passes.
func (a *RecapAgent) ExtractWithFeedbackAgentSDK(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	return a.extractWithFeedbackAgentSDK(ctx, chapterID, title, chapterText, feedback, false)
}

func (a *RecapAgent) extractWithFeedbackAgentSDK(ctx context.Context, chapterID, title string, chapterText string, feedback string, applyPatches bool) (*models.ChapterRecap, error) {
	params := InvokeParams{
		Skills:                 []string{"recap-extract"},
		SDKSkills:              []string{"recap-extract-workflow"},
		RequireSDK:             true,
		MaxTurns:               recapAgentSDKMaxTurns(chapterText),
		Timeout:                900,
		CompactOutputSchema:    true,
		DisableSDKOutputFormat: true,
		Command:                "extract chapter recap with Agent SDK workflow; return only the JSON object, with no prose summary, checklist, markdown, or explanation",
		Tools:                  nil,
		AllowedTools:           nil,
		PermissionMode:         "",
	}
	if applyPatches {
		params.SDKSkills = []string{"novel-tools-core", "recap-extract-workflow"}
		params.Tools = []string{"Bash"}
		params.AllowedTools = []string{"Bash"}
		params.PermissionMode = "dontAsk"
		params.ToolAllowlist = []string{
			fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", chapterID),
			fmt.Sprintf("novelgen tool patch recap --id %q --apply", chapterID),
		}
		params.ToolEvidence = ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
		}
		params.MaxTurns = 14
		params.Command = "extract and validate chapter recap using recap query/check/patch tools"
	}
	return a.extractWithFeedback(ctx, chapterID, title, chapterText, feedback, params, applyPatches)
}

func recapAgentSDKMaxTurns(chapterText string) int {
	n := narrativeUnitCount(strings.TrimSpace(chapterText))
	switch {
	case n > 1200:
		return 6
	case n > 800:
		return 5
	default:
		return 3
	}
}

func (a *RecapAgent) extractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string, params InvokeParams, applyPatches ...bool) (*models.ChapterRecap, error) {
	logger.Section("RECAP AGENT - Extract Recap")
	logger.Info("Chapter: %s - %s", chapterID, title)
	logger.Info("Language: %s", a.base.language)
	usePatchTools := len(applyPatches) > 0 && applyPatches[0]
	if usePatchTools {
		logger.Info("Agent apply enabled: SDK may write through validated recap patch tools")
	}

	input := RecapExtractInput{
		ChapterID:       chapterID,
		Title:           title,
		ChapterText:     chapterText,
		Feedback:        feedback,
		ApplyPatches:    usePatchTools,
		RequiredQueries: buildRecapAgentSDKRequiredQueries(chapterID, usePatchTools),
		Instructions:    buildRecapAgentSDKInstructions(usePatchTools),
	}

	var output RecapExtractOutput

	// Up to two passes: first extraction, then an auto-repair pass if the recap
	// fails the minimal continuity gate.
	attempts := 1
	if strings.TrimSpace(feedback) == "" {
		attempts = 2
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		curFeedback := strings.TrimSpace(feedback)
		if i == 1 {
			// Second pass: inject deterministic reasons as "must address" feedback.
			if ok, reasons := recap.ValidateMinimal(&output.Recap); !ok {
				curFeedback = strings.Join(reasons, "; ")
				input.Feedback = curFeedback
			} else if ok, reasons := recap.ValidateConsistency(&output.Recap); !ok {
				curFeedback = strings.Join(reasons, "; ")
				input.Feedback = curFeedback
			}
		}

		if err := a.base.Execute(ctx, params, input, &output); err != nil {
			lastErr = err
			continue
		}
		normalizeRecapIdentity(&output.Recap, chapterID, title)
		normalizeRecapSize(&output.Recap)
		repairRecapContinuationHint(&output.Recap)

		// If it passes the minimal continuity gate, we're done. Consistency
		// findings are fuzzy warnings and are handled by the caller's gate; do
		// not spend another full Agent SDK pass just to improve a warning.
		if ok, _ := recap.ValidateMinimal(&output.Recap); ok {
			if ok2, _ := recap.ValidateConsistency(&output.Recap); ok2 {
				logger.Info("✓ Recap extracted successfully")
				return &output.Recap, nil
			}
			logger.Info("✓ Recap extracted (with warnings)")
			return &output.Recap, nil
		}
	}

	// Return whatever we got from the last pass, or the last error. Minimal
	// failures must not become project state; consistency failures remain a
	// warning because they are intentionally fuzzy continuity heuristics.
	if lastErr != nil {
		return nil, lastErr
	}
	if ok, reasons := recap.ValidateMinimal(&output.Recap); !ok {
		return nil, fmt.Errorf("recap failed minimal validation: %s", strings.Join(reasons, "; "))
	}

	logger.Info("✓ Recap extracted (with warnings)")
	return &output.Recap, nil
}

func normalizeRecapIdentity(r *models.ChapterRecap, chapterID, title string) {
	if r == nil {
		return
	}
	r.ChapterID = strings.TrimSpace(chapterID)
	r.Title = strings.TrimSpace(title)
}

func normalizeRecapSize(r *models.ChapterRecap) {
	if r == nil {
		return
	}
	r.Location = clipRecapText(r.Location, 80)
	r.Time = clipRecapText(r.Time, 80)
	r.Present = compactRecapList(r.Present, 10, 60)
	r.PlotBeats = compactRecapList(r.PlotBeats, 8, 140)
	r.Decisions = compactRecapList(r.Decisions, 5, 140)
	r.Reveals = compactRecapList(r.Reveals, 6, 140)
	r.Unresolved = compactRecapList(r.Unresolved, 5, 140)
	r.Promises = compactRecapList(r.Promises, 4, 140)
	r.Items = compactRecapList(r.Items, 6, 120)
	r.Status = compactRecapList(r.Status, 6, 140)
	r.LastLine = clipRecapText(r.LastLine, 180)
	r.Cliffhanger = clipRecapText(r.Cliffhanger, 180)
	r.NextOpeningHint = clipRecapText(r.NextOpeningHint, 220)
}

func repairRecapContinuationHint(r *models.ChapterRecap) {
	if r == nil || strings.TrimSpace(r.LastLine) == "" || strings.TrimSpace(r.NextOpeningHint) == "" {
		return
	}
	if ok, _ := recap.ValidateConsistency(r); ok {
		return
	}
	anchor := recapContinuationAnchor(r.LastLine)
	if anchor == "" || strings.Contains(r.NextOpeningHint, anchor) {
		return
	}
	r.NextOpeningHint = clipRecapText(anchor+"之后，"+r.NextOpeningHint, 220)
}

func recapContinuationAnchor(lastLine string) string {
	text := strings.TrimSpace(lastLine)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	start := 0
	for start < len(runes) {
		switch runes[start] {
		case '，', '。', '！', '？', '“', '”', '（', '）', '：', ':', '—', '-', ' ', '\n', '\t', '而':
			start++
		default:
			goto foundStart
		}
	}
foundStart:
	if start >= len(runes) {
		return ""
	}
	end := start
	for end < len(runes) && end-start < 8 {
		switch runes[end] {
		case '，', '。', '！', '？', '“', '”', '（', '）', '：', ':', '—', '-', ' ', '\n', '\t':
			if end-start >= 2 {
				return string(runes[start:end])
			}
		}
		end++
	}
	if end-start < 2 {
		return ""
	}
	return string(runes[start:end])
}

func compactRecapList(values []string, maxItems int, maxRunes int) []string {
	if len(values) == 0 || maxItems <= 0 {
		return []string{}
	}
	capacity := len(values)
	if capacity > maxItems {
		capacity = maxItems
	}
	out := make([]string, 0, capacity)
	seen := map[string]bool{}
	for _, value := range values {
		value = clipRecapText(value, maxRunes)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func clipRecapText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func buildRecapAgentSDKRequiredQueries(chapterID string, applyPatches bool) []string {
	if !applyPatches {
		return nil
	}
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", chapterID),
		fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", chapterID),
	}
}

func buildRecapAgentSDKInstructions(applyPatches bool) []string {
	if !applyPatches {
		return []string{
			"Use only the provided chapter_text.",
			"Make next_opening_hint directly continue from last_line; reuse at least one concrete noun, character, location, or image from last_line.",
			"Keep recap compact: plot_beats <= 8, decisions <= 5, reveals <= 6, unresolved <= 5, promises <= 4, items/status <= 6; no prose summary or validation checklist.",
			"Return corrected recap JSON; Go will validate and save it.",
		}
	}
	return []string{
		"Execute required_queries before building the recap patch.",
		"Use the provided full chapter_text as the source of truth; query output is only for current recap/check/navigation context.",
		"Build a minimal recap patch with continuity-critical fields only.",
		"First dry-run with `printf '%s' '<compact-json>' | novelgen tool patch recap --id <chapter_id>`. For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. Use --patch-json only for small ASCII-only patches.",
		"If dry-run passes, repeat the same stdin-piped command with `--apply`, then run the focused recap check again.",
		"Do not write files by any other method.",
		"Return the final recap JSON even when you applied the patch; Go may compare it with the saved recap.",
	}
}

// ExtractFromJSON extracts a recap from a JSON string (for manual editing)
func (a *RecapAgent) ExtractFromJSON(jsonStr string) (*models.ChapterRecap, error) {
	var recap models.ChapterRecap
	if err := json.Unmarshal([]byte(jsonStr), &recap); err != nil {
		return nil, fmt.Errorf("failed to parse recap JSON: %w", err)
	}
	return &recap, nil
}
