// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package compose

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/fiction/agents/prompt"
	"sort"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

// Compact compose prompt payloads keep large story setup files from crowding
// out the outline task. Public inputs stay rich for CLI/internal contracts.
type composeGenPromptInput struct {
	SetupBrief string                `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, rules, progression limits, resources, core cast seeds, storylines"`
	Structure  models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

type composeReviewPromptInput struct {
	ExistingOutline models.Outline `json:"existing_outline" md:"existing_outline" desc:"Existing outline to review"`
	SetupBrief      string         `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for checking outline alignment"`
	UserPrompt      string         `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

type composeImprovePromptInput struct {
	ExistingOutline models.Outline      `json:"existing_outline" md:"existing_outline" desc:"Existing outline to improve"`
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	SetupBrief      string              `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for preserving setup promises"`
	RevisionContext string              `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier outline review and improve rounds"`
}

// composeImproveVolumePromptInput is the compact prompt payload sent to the LLM.
// The public input above can stay rich for internal routing/checkpoint logic;
// this shape avoids repeating the full outline, part, volume, and setup in one prompt.
type composeImproveVolumePromptInput struct {
	SetupBrief     string              `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, rules, progression limits, resources, storylines"`
	OutlineContext string              `json:"outline_context" md:"outline_context" desc:"Compact continuity context around the target volume"`
	TargetVolume   models.Volume       `json:"target_volume" md:"target_volume" desc:"Only this volume should be improved and returned"`
	ReviewResult   models.ReviewResult `json:"review_result" md:"review_result" desc:"Filtered review/DSL feedback relevant to this volume"`
	UserPrompt     string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
}

func composeImprovePromptCheckCategories(prompt string) []string {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	categories := []string{}
	if strings.Contains(prompt, "logic") || strings.Contains(prompt, "逻辑") || strings.Contains(prompt, "连贯") || strings.Contains(prompt, "承接") {
		categories = append(categories, "logic")
	}
	if strings.Contains(prompt, "transition") || strings.Contains(prompt, "过渡") || strings.Contains(prompt, "地点") || strings.Contains(prompt, "移动") {
		categories = append(categories, "transition")
	}
	if strings.Contains(prompt, "state_anchor") || strings.Contains(prompt, "状态锚点") || strings.Contains(prompt, "修炼境界") {
		categories = append(categories, "state_anchor")
	}
	if len(categories) == 0 {
		categories = append(categories, "all")
	}
	return categories
}

func smallChineseOrdinal(value int) string {
	if value <= 0 || value > 99 {
		return ""
	}
	digits := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if value < 10 {
		return digits[value]
	}
	if value == 10 {
		return "十"
	}
	tens := value / 10
	ones := value % 10
	if tens == 1 {
		if ones == 0 {
			return "十"
		}
		return "十" + digits[ones]
	}
	if ones == 0 {
		return digits[tens] + "十"
	}
	return digits[tens] + "十" + digits[ones]
}

func clipOutlineReviewResult(result *models.ReviewResult) {
	if result == nil {
		return
	}
	result.Summary = prompt.ClipForPrompt(result.Summary, 300)
	result.Strengths = prompt.ClipReviewStringList(result.Strengths, prompt.MaxReviewStrengths, prompt.MaxReviewStrengthRunes)
	result.Weaknesses = prompt.ClipReviewStringList(result.Weaknesses, prompt.MaxReviewWeaknesses, prompt.MaxReviewWeaknessRunes)
	suggestions := make([]models.ReviewSuggestion, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		suggestions = append(suggestions, prompt.ClipReviewSuggestion(s))
		if len(suggestions) >= prompt.MaxReviewSuggestions {
			break
		}
	}
	result.Suggestions = suggestions
}

func promptInputRequiredFocusedChecks(promptInput composeAgentSDKImproveVolumePromptInput) []string {
	const prefix = "Required focused checks before final JSON: "
	for _, instruction := range promptInput.Instructions {
		if !strings.HasPrefix(instruction, prefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(instruction, prefix))
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ";")
		checks := make([]string, 0, len(parts))
		for _, part := range parts {
			if check := strings.TrimSpace(part); check != "" {
				checks = append(checks, check)
			}
		}
		return checks
	}
	return nil
}

func (a *ComposeAgent) buildImproveVolumePromptInput(input ComposeImproveVolumeInput) composeImproveVolumePromptInput {
	return composeImproveVolumePromptInput{
		SetupBrief:     a.buildSetupBrief(&input.Setup),
		OutlineContext: a.buildVolumeImproveContext(&input.Outline, input.Part.ID, input.Volume.ID),
		TargetVolume:   input.Volume,
		ReviewResult:   prompt.CompactReviewForPrompt(input.ReviewResult),
		UserPrompt:     input.UserPrompt,
	}
}

func (a *ComposeAgent) buildSetupBrief(setup *models.StorySetup) string {
	if setup == nil || setup.ProjectName == "" {
		return "No setup provided."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Project: %s\n", setup.ProjectName))
	if len(setup.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(setup.Genres, ", ")))
	}
	b.WriteString(fmt.Sprintf("Premise: %s\n", prompt.ClipForPrompt(setup.Premise, 700)))
	b.WriteString(fmt.Sprintf("Theme: %s\n", prompt.ClipForPrompt(setup.Theme, 350)))
	if setup.Tone != "" {
		b.WriteString(fmt.Sprintf("Tone: %s\n", prompt.ClipForPrompt(setup.Tone, 160)))
	}

	if setup.LongFormPlan != nil && !setup.LongFormPlan.IsZero() {
		plan := setup.LongFormPlan
		b.WriteString("\nLong Form Plan:\n")
		if plan.TargetChapters > 0 || plan.TargetVolumes > 0 {
			b.WriteString(fmt.Sprintf("- Target scale: %d chapter(s), %d volume(s)\n", plan.TargetChapters, plan.TargetVolumes))
		}
		if plan.MainLoop != "" {
			b.WriteString(fmt.Sprintf("- Main loop: %s\n", prompt.ClipForPrompt(plan.MainLoop, 260)))
		}
		if len(plan.EscalationLadder) > 0 {
			b.WriteString(fmt.Sprintf("- Escalation ladder: %s\n", joinLimited(plan.EscalationLadder, 8)))
		}
		if len(plan.ReaderPromises) > 0 {
			b.WriteString(fmt.Sprintf("- Reader promises: %s\n", joinLimited(plan.ReaderPromises, 8)))
		}
		if plan.PayoffCadence != "" {
			b.WriteString(fmt.Sprintf("- Payoff cadence: %s\n", prompt.ClipForPrompt(plan.PayoffCadence, 220)))
		}
		if len(plan.VolumePattern) > 0 {
			b.WriteString(fmt.Sprintf("- Volume pattern: %s\n", joinLimited(plan.VolumePattern, 8)))
		}
		if plan.MidpointMutation != "" || plan.EndgamePromise != "" {
			b.WriteString(fmt.Sprintf("- Mutation/endgame: %s | %s\n", prompt.ClipForPrompt(plan.MidpointMutation, 180), prompt.ClipForPrompt(plan.EndgamePromise, 180)))
		}
	}

	if len(setup.Rules) > 0 {
		b.WriteString("\nCore Rules:\n")
		for i, rule := range setup.Rules {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("- ... %d more rule(s)\n", len(setup.Rules)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s\n", prompt.ClipForPrompt(rule, 360)))
		}
	}

	if len(setup.CoreCast) > 0 {
		b.WriteString("\nCore Cast Seeds:\n")
		for i, seed := range setup.CoreCast {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more cast seed(s)\n", len(setup.CoreCast)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s, importance %d, entry %s): %s\n",
				seed.Name, seed.Role, seed.Importance, seed.EntryPhase, prompt.ClipForPrompt(seed.StoryFunction, 220)))
			if seed.RelationshipArc != "" || seed.Payoff != "" {
				b.WriteString(fmt.Sprintf("  Arc: %s | Payoff: %s\n",
					prompt.ClipForPrompt(seed.RelationshipArc, 160), prompt.ClipForPrompt(seed.Payoff, 180)))
			}
			if len(seed.StorylineRefs) > 0 {
				b.WriteString(fmt.Sprintf("  Storylines: %s\n", strings.Join(seed.StorylineRefs, ", ")))
			}
		}
	}

	if len(setup.Storylines) > 0 {
		b.WriteString("\nStorylines:\n")
		for i, sl := range setup.Storylines {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more storyline(s)\n", len(setup.Storylines)-i))
				break
			}
			scopeNote := compactStorylineScope(sl)
			b.WriteString(fmt.Sprintf("- %s (%s%s, importance %d): %s\n",
				sl.Name, sl.Type, scopeNote, sl.Importance, prompt.ClipForPrompt(sl.Description, 260)))
			if sl.SetupRole != "" {
				b.WriteString(fmt.Sprintf("  Role: %s\n", prompt.ClipForPrompt(sl.SetupRole, 160)))
			}
			if sl.RepeatablePressure != "" || sl.PayoffCadence != "" || sl.Mutation != "" {
				b.WriteString(fmt.Sprintf("  Serial engine: pressure=%s | cadence=%s | mutation=%s\n",
					prompt.ClipForPrompt(sl.RepeatablePressure, 150), prompt.ClipForPrompt(sl.PayoffCadence, 130), prompt.ClipForPrompt(sl.Mutation, 150)))
			}
			if sl.FailureMode != "" {
				b.WriteString(fmt.Sprintf("  Avoid failure mode: %s\n", prompt.ClipForPrompt(sl.FailureMode, 140)))
			}
			if sl.Desire != "" || sl.Opposition != "" || sl.Payoff != "" {
				b.WriteString(fmt.Sprintf("  Desire: %s | Opposition: %s | Payoff%s: %s\n",
					prompt.ClipForPrompt(sl.Desire, 160), prompt.ClipForPrompt(sl.Opposition, 160), compactPayoffStyle(sl), prompt.ClipForPrompt(sl.Payoff, 160)))
			}
			if sl.AppealEngine != nil {
				b.WriteString(formatAppealEngineBrief("  Appeal", sl.AppealEngine))
			}
		}
	}

	if len(setup.Premises) > 0 {
		b.WriteString("\nProgression Systems:\n")
		for i, premise := range setup.Premises {
			if i >= 8 {
				b.WriteString(fmt.Sprintf("- ... %d more progression system(s)\n", len(setup.Premises)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", premise.Name, premise.Category, prompt.ClipForPrompt(premise.Description, 280)))
			if premise.AppealEngine != nil {
				b.WriteString(formatAppealEngineBrief("  Appeal", premise.AppealEngine))
			}
			for j, stage := range premise.Progression {
				if j >= 8 {
					b.WriteString(fmt.Sprintf("  ... %d more progression stage(s)\n", len(premise.Progression)-j))
					break
				}
				b.WriteString(fmt.Sprintf("  L%d %s: %s Requirement: %s\n",
					stage.Level, stage.Name, prompt.ClipForPrompt(stage.Description, 130), prompt.ClipForPrompt(stage.Requirements, 130)))
			}
		}
	}

	if len(setup.WorldResources) > 0 {
		b.WriteString("\nResources:\n")
		for i, res := range setup.WorldResources {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more resource(s)\n", len(setup.WorldResources)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s, scarcity: %s): %s\n",
				res.Name, res.Category, res.Scarcity, prompt.ClipForPrompt(res.Description, 180)))
		}
	}

	return b.String()
}

func formatAppealEngineBrief(prefix string, engine *models.AppealEngine) string {
	if engine == nil {
		return ""
	}
	var parts []string
	if strings.TrimSpace(engine.Appeal) != "" {
		parts = append(parts, "appeal="+prompt.ClipForPrompt(engine.Appeal, 120))
	}
	if strings.TrimSpace(engine.SurfaceLimit) != "" {
		parts = append(parts, "surface_limit="+prompt.ClipForPrompt(engine.SurfaceLimit, 120))
	}
	if strings.TrimSpace(engine.Exploit) != "" {
		parts = append(parts, "exploit="+prompt.ClipForPrompt(engine.Exploit, 120))
	}
	if strings.TrimSpace(engine.SignatureWin) != "" {
		parts = append(parts, "signature_win="+prompt.ClipForPrompt(engine.SignatureWin, 120))
	}
	if strings.TrimSpace(engine.UpgradePath) != "" {
		parts = append(parts, "upgrade_path="+prompt.ClipForPrompt(engine.UpgradePath, 120))
	}
	if strings.TrimSpace(engine.OpponentMisread) != "" {
		parts = append(parts, "opponent_misread="+prompt.ClipForPrompt(engine.OpponentMisread, 120))
	}
	if strings.TrimSpace(engine.RewardType) != "" {
		parts = append(parts, "reward_type="+prompt.ClipForPrompt(engine.RewardType, 80))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s\n", prefix, strings.Join(parts, " | "))
}

func compactStorylineScope(sl models.Storyline) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(sl.Scope) != "" {
		parts = append(parts, "scope "+strings.TrimSpace(sl.Scope))
	}
	if strings.TrimSpace(sl.PayoffStyle) != "" {
		parts = append(parts, "payoff "+strings.TrimSpace(sl.PayoffStyle))
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

func compactPayoffStyle(sl models.Storyline) string {
	if strings.TrimSpace(sl.PayoffStyle) == "" {
		return ""
	}
	return " [" + strings.TrimSpace(sl.PayoffStyle) + "]"
}

func formatVolumeBrief(volume models.Volume) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("- %s (%s): %s\n", volume.Title, volume.ID, prompt.ClipForPrompt(volume.Summary, 500)))
	if payoff := formatVolumePayoffBrief(volume.PayoffContract); payoff != "" {
		b.WriteString("  Payoff: " + payoff + "\n")
	}
	if len(volume.Chapters) > 0 {
		first := volume.Chapters[0]
		last := volume.Chapters[len(volume.Chapters)-1]
		b.WriteString(fmt.Sprintf("  Opens with: %s - %s\n", first.Title, prompt.ClipForPrompt(first.Summary, 180)))
		b.WriteString(fmt.Sprintf("  Ends with: %s - %s\n", last.Title, prompt.ClipForPrompt(last.Summary, 180)))
	}
	return b.String()
}

func formatVolumePayoffBrief(payoff *models.VolumePayoffContract) string {
	if payoff.IsZero() {
		return ""
	}
	var parts []string
	if strings.TrimSpace(payoff.VolumeQuestion) != "" {
		parts = append(parts, "question="+prompt.ClipForPrompt(payoff.VolumeQuestion, 120))
	}
	if strings.TrimSpace(payoff.PowerPromise) != "" {
		parts = append(parts, "promise="+prompt.ClipForPrompt(payoff.PowerPromise, 120))
	}
	if strings.TrimSpace(payoff.MainOpponentMisread) != "" {
		parts = append(parts, "misread="+prompt.ClipForPrompt(payoff.MainOpponentMisread, 120))
	}
	if strings.TrimSpace(payoff.BigWin) != "" {
		parts = append(parts, "big_win="+prompt.ClipForPrompt(payoff.BigWin, 140))
	}
	if strings.TrimSpace(payoff.VisibleReward) != "" {
		parts = append(parts, "reward="+prompt.ClipForPrompt(payoff.VisibleReward, 100))
	}
	if strings.TrimSpace(payoff.ReputationShift) != "" {
		parts = append(parts, "reputation="+prompt.ClipForPrompt(payoff.ReputationShift, 100))
	}
	if strings.TrimSpace(payoff.NextBiggerGame) != "" {
		parts = append(parts, "next="+prompt.ClipForPrompt(payoff.NextBiggerGame, 120))
	}
	return strings.Join(parts, " | ")
}

func (a *ComposeAgent) appendComposeImproveReport(entry ComposeImproveReportEntry) error {
	root, err := a.composeImproveRoot()
	if err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return root.Append(root.ComposeFile("outline_improve_report.jsonl"), append(data, '\n'), store.FileMode)
}

func composePromptRequestsCheckFirstNoop(prompt string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(prompt)), " "))
	if normalized == "" {
		return false
	}
	hasCheckIntent := strings.Contains(normalized, "check") ||
		strings.Contains(normalized, "检查") ||
		strings.Contains(normalized, "校验") ||
		strings.Contains(normalized, "验证")
	hasCleanCondition := strings.Contains(normalized, "0 issue") ||
		strings.Contains(normalized, "0issue") ||
		strings.Contains(normalized, "无 issue") ||
		strings.Contains(normalized, "无问题") ||
		strings.Contains(normalized, "没有问题") ||
		strings.Contains(normalized, "返回 0") ||
		strings.Contains(normalized, "返回0")
	hasNoopIntent := strings.Contains(normalized, "不要 patch") ||
		strings.Contains(normalized, "不要patch") ||
		strings.Contains(normalized, "不要改") ||
		strings.Contains(normalized, "不要修改") ||
		(strings.Contains(normalized, "不要") && strings.Contains(normalized, "改写")) ||
		(strings.Contains(normalized, "不要") && strings.Contains(normalized, "patch")) ||
		strings.Contains(normalized, "不改写") ||
		strings.Contains(normalized, "no-op") ||
		strings.Contains(normalized, "no patch")
	return hasCheckIntent && hasCleanCondition && hasNoopIntent
}

// formatEvents formats events for context display
func (a *ComposeAgent) formatEvents(events []models.Event) string {
	if len(events) == 0 {
		return "None"
	}
	var parts []string
	for _, e := range events {
		part := fmt.Sprintf("[%s: %s - %s]", e.Type, e.Subject, e.Change)
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func compactLabel(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("; %s=%s", label, prompt.ClipForPrompt(value, 100))
}

func joinLimited(values []string, limit int) string {
	values = compactStringList(values)
	if len(values) == 0 {
		return "none"
	}
	if limit > 0 && len(values) > limit {
		values = append(values[:limit], fmt.Sprintf("...%d more", len(values)-limit))
	}
	return strings.Join(values, ", ")
}

func compactStringList(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func formatIntMap(values map[string]int, limit int) string {
	keys := sortedKeys(values)
	var parts []string
	for i, key := range keys {
		if limit > 0 && i >= limit {
			parts = append(parts, fmt.Sprintf("...%d more", len(keys)-limit))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, "; ")
}

func formatStringMap(values map[string]string, limit int) string {
	keys := sortedKeys(values)
	var parts []string
	for i, key := range keys {
		if limit > 0 && i >= limit {
			parts = append(parts, fmt.Sprintf("...%d more", len(keys)-limit))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// getOpeningBeat returns beats[0] or empty string.
func getOpeningBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[0]
	}
	return ""
}

// getClosingBeat returns beats[last] or empty string.
func getClosingBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[len(chapter.GetBeats())-1]
	}
	return ""
}
