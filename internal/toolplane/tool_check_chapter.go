// The chapter target checks: deterministic prose/format gates over the saved markdown and the
// RPG-DSL simulation gate with its signal diagnostics and stale-DSL detection.

package toolplane

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"novelgen/internal/logic/style"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	rpgdsl "novelgen/internal/rpg/dsl"
	"novelgen/internal/store"
	"novelgen/internal/toolcmd"
	"novelgen/internal/utils"
	"novelgen/internal/writersdl"
)

// RunToolChapterCheckWithTargetWords is the string-root entry point used by the
// write pipeline, which holds a resolved project directory.
func RunToolChapterCheckWithTargetWords(rootDir, kind, scope, id string, targetWordsOverride int) (*ToolCheckResult, error) {
	return runToolChapterCheckAt(store.At(rootDir), kind, scope, id, targetWordsOverride)
}

func runToolChapterCheckAt(root *store.Root, kind, scope, id string, targetWordsOverride int) (*ToolCheckResult, error) {
	kind = NormalizeKey(kind)
	if kind == "" {
		kind = "quality"
	}
	switch kind {
	case "quality", "simulation", "all":
	default:
		return nil, fmt.Errorf("unsupported chapter check kind %q", kind)
	}
	scope = normalizeToolScope(scope)
	if scope == "all" || scope == "" {
		scope = "chapter"
	}
	id = strings.TrimSpace(id)
	if scope != "chapter" {
		return nil, fmt.Errorf("chapter checks currently support only --scope chapter")
	}
	if id == "" {
		return nil, fmt.Errorf("--id is required for chapter check")
	}

	outline, err := root.LoadOutline()
	if err != nil {
		return nil, fmt.Errorf("load outline: %w", err)
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found in outline", id)
	}

	// Read the manuscript through the same candidate list the write pipeline and
	// export use, and from the project this check was scoped to rather than from
	// the ambient default root.
	content, _ := root.LoadChapterContent(chapter.ID)
	targetWords := targetWordsOverride
	if targetWords <= 0 {
		targetWords = toolChapterTargetWords()
	}
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		gate = RunToolChapterQualityGate(chapter, content, targetWords)
	case "simulation":
		gate = runToolChapterSimulationGateAt(root, id)
	case "all":
		gate = RunToolChapterQualityGate(chapter, content, targetWords)
		sim := runToolChapterSimulationGateAt(root, id)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	result := MakeToolCheckResult(kind, "chapter", scope, id, gate)
	if targetWords > 0 {
		addToolCheckMeta(result, "target_words", targetWords)
	}
	return result, nil
}

func RunToolChapterQualityGate(chapter *models.Chapter, content string, targetWords int) (gate quality.QualityGateResult) {
	defer func() {
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}()
	chapterID := ""
	chapterTitle := ""
	if chapter != nil {
		chapterID = strings.TrimSpace(chapter.ID)
		chapterTitle = strings.TrimSpace(chapter.Title)
	}
	targetName := quality.CoalesceString(chapterTitle, chapterID)
	clean := strings.TrimSpace(content)
	if clean == "" {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter markdown is missing or empty", "Run write gen or write improve for this chapter, then rerun tool check quality --target chapter.", models.PriorityHigh))
		return gate
	}

	if chapterTitle != "" && !strings.HasPrefix(clean, "# "+chapterTitle) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter markdown does not start with the outline title", "Keep the saved chapter headed by the outline title so downstream export and recap tools read the correct chapter.", models.PriorityMedium))
	}

	body := strings.TrimSpace(stripToolChapterMarkdownTitle(clean, chapterTitle))
	if body == "" {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body is empty after the title", "Regenerate the chapter body before running recap or RPG DSL extraction.", models.PriorityHigh))
		return gate
	}
	if chapterTitle != "" && repeatedChapterTitlePrefix(body, chapterTitle) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body repeats the outline title immediately after the heading", "Remove the duplicate title from the first body paragraph; keep one markdown heading and start the body with scene prose.", models.PriorityMedium))
	}
	if strings.HasPrefix(body, "{") && strings.Contains(body, `"content"`) {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body looks like raw JSON output", "Regenerate or improve the chapter so the saved markdown contains prose, not the agent JSON envelope.", models.PriorityHigh))
	}
	if strings.Contains(body, "```") {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body contains markdown code fences", "Remove code fences and save only narrative prose in the final chapter markdown.", models.PriorityHigh))
	}
	if strings.Contains(strings.ToLower(body), "<br") {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body contains HTML line break tags", "Replace HTML <br> tags with normal Markdown paragraph breaks or prose punctuation.", models.PriorityMedium))
	}
	if regexp.MustCompile(`\*\*[^*\n][^*\n]*\*\*`).MatchString(body) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body contains inline bold markdown", "Remove inline bold formatting from final prose; keep only the chapter heading as Markdown structure.", models.PriorityMedium))
	}
	for _, artifact := range chapterTypoArtifacts(body) {
		gate.Add(quality.QualitySuggestion("prose", chapterID, targetName, fmt.Sprintf("final chapter body contains suspicious typo artifact: %s", artifact), "Fix the suspicious typo while preserving the surrounding prose and chapter events.", models.PriorityMedium))
	}

	count := ToolNarrativeUnitCount(body)
	paragraphs := toolChapterParagraphCount(body)
	if count >= 600 && paragraphs < 3 {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, fmt.Sprintf("final chapter has too few prose paragraphs: got %d paragraphs for %d narrative units", paragraphs, count), "Split the chapter into mobile-readable prose and dialogue paragraphs while preserving the same events.", models.PriorityMedium))
	}
	if targetWords > 0 {
		min := targetWords / 2
		if min < 120 {
			min = 120
		}
		if count < min {
			gate.Add(quality.QualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is too short: got %d narrative units, target %d", count, targetWords), "Regenerate or improve the chapter with enough scene-level prose to satisfy the target length.", models.PriorityHigh))
		}
		hardMax := toolChapterLengthHardMax(targetWords)
		if count > hardMax {
			gate.Add(quality.QualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is much longer than target: got %d narrative units, target %d", count, targetWords), "Trim the chapter or split non-target material so this saved file contains only the selected chapter.", models.PriorityMedium))
		}
	}

	if chapter != nil && len(chapter.Characters) > 0 {
		missing := []string{}
		for _, name := range firstNStrings(chapter.Characters, 5) {
			name = strings.TrimSpace(name)
			if name != "" && !chapterTextMentionsCharacter(body, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			gate.Add(quality.QualitySuggestion("character", chapterID, targetName, fmt.Sprintf("final chapter text does not mention outline character(s): %s", strings.Join(missing, ", ")), "Revise the saved chapter so the named outline characters are present under their canonical names, or update the outline only through the validated outline patch workflow if the cast changed.", models.PriorityHigh))
		}
	}

	flavor := style.CheckAIFlavor(body, 75)
	if flavor.HasIssue {
		gate.Add(quality.QualitySuggestion("style", chapterID, targetName, fmt.Sprintf("deterministic AI-flavor check score is %d", flavor.Score), strings.Join(flavor.Suggestions, " "), models.PriorityMedium))
	}
	return gate
}

func chapterTypoArtifacts(body string) []string {
	checks := []string{
		"十好几个",
	}
	found := []string{}
	for _, check := range checks {
		if strings.Contains(body, check) {
			found = append(found, check)
		}
	}
	return found
}

func chapterTextMentionsCharacter(body, name string) bool {
	body = strings.TrimSpace(body)
	for _, candidate := range chapterCharacterMentionCandidates(name) {
		if candidate != "" && strings.Contains(body, candidate) {
			return true
		}
	}
	return false
}

func chapterCharacterMentionCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	candidates := []string{name}
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("()（）[]【】,，:：/、|", r)
	})
	if len(fields) > 0 {
		candidates = append(candidates, fields[len(fields)-1])
	}
	runes := []rune(name)
	if containsCJKRune(name) && len(runes) >= 4 {
		candidates = append(candidates, string(runes[len(runes)-2:]))
	}
	if containsCJKRune(name) && len(runes) >= 5 {
		candidates = append(candidates, string(runes[len(runes)-3:]))
	}
	return dedupeNonEmptyStrings(candidates)
}

func containsCJKRune(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func stripToolChapterMarkdownTitle(content, title string) string {
	return project.StripChapterTitle(content, title)
}

func repeatedChapterTitlePrefix(content, title string) bool {
	return project.HasRepeatedChapterTitlePrefix(content, title)
}

func toolChapterTargetWords() int {
	cfg, err := project.LoadConfig()
	if err == nil && cfg.ChapterConfig.TargetWordsPerChapter > 0 {
		return cfg.ChapterConfig.TargetWordsPerChapter
	}
	return 2000
}

func ToolNarrativeUnitCount(content string) int {
	return project.NarrativeUnitCount(content)
}

func toolChapterParagraphCount(content string) int {
	count := 0
	for _, part := range regexp.MustCompile(`\r?\n\s*\r?\n`).Split(strings.TrimSpace(content), -1) {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "#") || strings.HasPrefix(part, "<!--") {
			continue
		}
		count++
	}
	return count
}

// RunToolChapterSimulationGate is the string-root entry point for callers that
// hold a resolved project directory.
func RunToolChapterSimulationGate(rootDir, chapterID string) quality.QualityGateResult {
	return runToolChapterSimulationGateAt(store.At(rootDir), chapterID)
}

func runToolChapterSimulationGateAt(root *store.Root, chapterID string) quality.QualityGateResult {
	var gate quality.QualityGateResult
	if stale, info := toolChapterRPGDSLStale(root, chapterID); stale {
		gate.Add(quality.QualitySuggestion(
			"simulation",
			chapterID,
			chapterID,
			"chapter RPG DSL is stale relative to the saved final chapter markdown",
			fmt.Sprintf("Refresh chapter RPG DSL before trusting simulation results: %s. Chapter markdown %s is newer than %s. Simulation checks do not invoke LLM conversion by themselves.", toolcmd.ChapterDSLRefresh(chapterID), info.ChapterPath, info.DSLPath),
			models.PriorityHigh,
		))
		gate.Blocking = true
		return gate
	}
	dslData, err := loadToolMergedRPGDSL(root)
	if err != nil {
		gate.Add(quality.QualitySuggestion(
			"simulation",
			chapterID,
			chapterID,
			fmt.Sprintf("chapter RPG DSL simulation is unavailable: %v", err),
			"Refresh chapter RPG DSL first with `"+toolcmd.ChapterDSLRefresh(chapterID)+"`, then rerun this simulation check. Simulation checks do not invoke LLM conversion by themselves.",
			models.PriorityHigh,
		))
		gate.Blocking = true
		return gate
	}
	simulator := rpgdsl.NewSimulator(dslData)
	issues := simulator.SimulateChapter(chapterID)
	gate.Add(rpgdsl.NewSimulationBridge().ConvertIssuesToSuggestions(issues)...)
	if diagnostic := BuildChapterSimulationSignalDiagnostic(chapterID, dslData, issues); diagnostic != nil {
		gate.Add(*diagnostic)
	}
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return gate
}

type chapterSimulationSignalDiagnostics struct {
	CombatSteps          int
	EnemyRefs            []string
	HasCombatResult      bool
	HasPowerChange       bool
	HasBreakthrough      bool
	HasGene              bool
	HasMech              bool
	HasEquipmentOrItem   bool
	HasAlly              bool
	HasTacticalText      bool
	MissingRepairSignals []string
}

func BuildChapterSimulationSignalDiagnostic(chapterID string, dslData *rpgdsl.DSL, issues []rpgdsl.SimulationIssue) *models.ReviewSuggestion {
	if !chapterSimulationNeedsSignalDiagnostic(chapterID, issues) {
		return nil
	}
	diagnostic, ok := collectChapterSimulationSignalDiagnostics(dslData, chapterID)
	if !ok || diagnostic.CombatSteps == 0 {
		return nil
	}
	issue := fmt.Sprintf("simulation signal diagnostics: combat_steps=%d; enemies=%s; combat_result=%t; power_change=%t; breakthrough=%t; gene=%t; mech=%t; equipment_or_item=%t; ally=%t; tactical_text=%t",
		diagnostic.CombatSteps,
		strings.Join(firstNStrings(diagnostic.EnemyRefs, 8), ", "),
		diagnostic.HasCombatResult,
		diagnostic.HasPowerChange,
		diagnostic.HasBreakthrough,
		diagnostic.HasGene,
		diagnostic.HasMech,
		diagnostic.HasEquipmentOrItem,
		diagnostic.HasAlly,
		diagnostic.HasTacticalText,
	)
	if len(diagnostic.MissingRepairSignals) > 0 {
		issue += "; missing_repair_signals=" + strings.Join(diagnostic.MissingRepairSignals, ", ")
	}
	suggestion := "Make the next prose patch produce explicit DSL-readable repair signals. For combat balance, add supported mech/gene/equipment/item/ally/power_change/breakthrough signals or reduce enemy count/level. For missing combat result, add an on_complete narration/result and durable state_delta consequence. Then run `novelgen tool refresh chapter-dsl --id \"" + chapterID + "\"` and rerun chapter simulation check."
	return &models.ReviewSuggestion{
		Category:   models.CategoryLogic,
		TargetID:   chapterID,
		TargetName: chapterID,
		Issue:      issue,
		Suggestion: suggestion,
		Priority:   models.PriorityLow,
	}
}

func chapterSimulationNeedsSignalDiagnostic(chapterID string, issues []rpgdsl.SimulationIssue) bool {
	for _, issue := range issues {
		if strings.TrimSpace(issue.Chapter) != "" && issue.Chapter != chapterID {
			continue
		}
		if issue.Type == rpgdsl.IssueBalance || issue.Type == rpgdsl.IssueDescription {
			return true
		}
		text := issue.Description + " " + issue.Suggestion
		if strings.Contains(text, "战斗难度") || strings.Contains(text, "叙事性结果") || strings.Contains(strings.ToLower(text), "combat") {
			return true
		}
	}
	return false
}

func collectChapterSimulationSignalDiagnostics(dslData *rpgdsl.DSL, chapterID string) (chapterSimulationSignalDiagnostics, bool) {
	var out chapterSimulationSignalDiagnostics
	chapter := findDSLChapterByID(dslData, chapterID)
	if chapter == nil {
		return out, false
	}
	enemyRefs := map[string]bool{}
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			event := step.Event
			if strings.EqualFold(event.Type, "combat") || event.Combat != nil {
				out.CombatSteps++
				if event.Combat != nil {
					for _, enemy := range event.Combat.Setup.Enemies {
						ref := strings.TrimSpace(enemy.ID)
						if ref == "" {
							ref = "enemy_unknown"
						}
						ref = fmt.Sprintf("%s(count=%d,level=%d,elite=%t,boss=%t)", ref, enemy.Count, enemy.Level, enemy.Elite, enemy.Boss)
						if !enemyRefs[ref] {
							enemyRefs[ref] = true
							out.EnemyRefs = append(out.EnemyRefs, ref)
						}
					}
				}
			}
			if eventResultHasNarration(event.OnComplete) ||
				(event.Combat != nil && (eventResultHasNarration(event.Combat.OnVictory) || eventResultHasNarration(event.Combat.OnDefeat))) {
				out.HasCombatResult = true
			}
			if chapterStepHasTacticalText(step) {
				out.HasTacticalText = true
			}
			for _, delta := range event.StateDeltas {
				switch strings.ToLower(strings.TrimSpace(delta.Kind)) {
				case "power_change":
					out.HasPowerChange = true
				case "breakthrough", "evolution", "cultivation":
					out.HasBreakthrough = true
				case "gene":
					out.HasGene = true
				case "mech":
					out.HasMech = true
				case "item", "equipment", "resource":
					if strings.Contains(strings.ToLower(delta.Field+" "+delta.Target+" "+delta.To), "item") ||
						strings.TrimSpace(delta.Target) != "" ||
						strings.TrimSpace(delta.To) != "" {
						out.HasEquipmentOrItem = true
					}
				case "ally":
					out.HasAlly = true
				}
			}
		}
	}
	sort.Strings(out.EnemyRefs)
	out.MissingRepairSignals = missingChapterSimulationRepairSignals(out)
	return out, true
}

func findDSLChapterByID(dslData *rpgdsl.DSL, chapterID string) *rpgdsl.Chapter {
	if dslData == nil || dslData.Storyline == nil {
		return nil
	}
	for i := range dslData.Storyline.Chapters {
		if dslData.Storyline.Chapters[i].ID == chapterID {
			return &dslData.Storyline.Chapters[i]
		}
	}
	return nil
}

func eventResultHasNarration(result *rpgdsl.EventResult) bool {
	return result != nil && (strings.TrimSpace(result.Narration) != "" || strings.TrimSpace(result.Result) != "")
}

func chapterStepHasTacticalText(step rpgdsl.Step) bool {
	text := strings.ToLower(step.Description)
	event := step.Event
	if event.Combat != nil {
		text += " " + strings.ToLower(event.Combat.Setup.Location)
		for key, value := range event.Combat.Setup.Environment {
			text += " " + strings.ToLower(key) + " " + strings.ToLower(fmt.Sprint(value))
		}
		for _, phase := range event.Combat.Phases {
			text += " " + strings.ToLower(phase.Name+" "+phase.Trigger+" "+phase.Duration+" "+phase.Narration)
			for key, value := range phase.Modifiers {
				text += " " + strings.ToLower(key) + " " + strings.ToLower(fmt.Sprint(value))
			}
		}
	}
	for _, delta := range event.StateDeltas {
		text += " " + strings.ToLower(delta.Target+" "+delta.Kind+" "+delta.Field+" "+delta.From+" "+delta.To+" "+delta.Cost+" "+delta.Note)
	}
	for _, term := range []string{
		"机甲", "火种", "装甲", "伏击", "偷袭", "陷阱", "地形", "高地", "狭道", "三方", "混战", "互相消耗", "两败俱伤", "第三方",
		"mech", "armor", "ambush", "trap", "terrain", "high ground", "narrow", "chokepoint", "third party", "three-way", "attrition",
	} {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func missingChapterSimulationRepairSignals(d chapterSimulationSignalDiagnostics) []string {
	missing := make([]string, 0, 6)
	if !d.HasCombatResult {
		missing = append(missing, "combat_result_on_complete")
	}
	if !d.HasPowerChange {
		missing = append(missing, "power_change")
	}
	if !d.HasMech {
		missing = append(missing, "mech")
	}
	if !d.HasGene {
		missing = append(missing, "gene")
	}
	if !d.HasEquipmentOrItem {
		missing = append(missing, "equipment_or_item")
	}
	if !d.HasAlly {
		missing = append(missing, "ally")
	}
	if !d.HasTacticalText {
		missing = append(missing, "tactical_text")
	}
	return missing
}

type toolChapterRPGDSLStaleInfo struct {
	ChapterPath string
	DSLPath     string
}

func toolChapterRPGDSLStale(root *store.Root, chapterID string) (bool, toolChapterRPGDSLStaleInfo) {
	info := toolChapterRPGDSLStaleInfo{
		ChapterPath: toolFinalChapterPathForID(root, chapterID),
		DSLPath:     root.RPGChaptersDSL(),
	}
	chapterStat, err := os.Stat(info.ChapterPath)
	if err != nil {
		fallback := numericChapterFallbackPath(root, chapterID)
		chapterStat, err = os.Stat(fallback)
		if err != nil {
			return false, info
		}
		info.ChapterPath = fallback
	}
	dslStat, err := os.Stat(info.DSLPath)
	if err != nil {
		return false, info
	}
	return chapterStat.ModTime().After(dslStat.ModTime()), info
}

// numericChapterFallbackPath is the legacy numeric chapter filename
// (chapters/chapter-3.md for "P1-V1-C3"), which older projects still use.
func numericChapterFallbackPath(root *store.Root, chapterID string) string {
	return root.ChapterFile(utils.ExtractChapterNumber(chapterID))
}

func toolFinalChapterPathForID(root *store.Root, chapterID string) string {
	return root.ChapterFile(chapterID)
}

func loadToolMergedRPGDSL(root *store.Root) (*rpgdsl.DSL, error) {
	fragments, err := loadToolRPGDSLFragments(root)
	if err != nil {
		return nil, err
	}
	if len(fragments) == 0 {
		return nil, fmt.Errorf("no RPG DSL files found under %s", root.RPGDir())
	}
	if len(fragments) == 1 {
		return fragments[0].DSL, nil
	}
	merger := rpgdsl.NewDSLMerger(rpgdsl.NewConsoleLogger(rpgdsl.WithMinLevel(rpgdsl.LogLevelFatal)))
	for _, fragment := range fragments {
		merger.AddFragment(fragment.DSL, fragment.Phase, fragment.FilePath)
	}
	result, err := merger.Merge()
	if err != nil {
		return nil, err
	}
	return result.DSL, nil
}

func loadToolRPGDSLFragments(root *store.Root) ([]*rpgdsl.DSLFragment, error) {
	rpgDir := root.RPGDir()
	patterns := append(store.RPGPhaseFiles(), "*.rpg")
	var fragments []*rpgdsl.DSLFragment
	seen := map[string]bool{}
	var parseErrors []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rpgDir, pattern))
		if err != nil {
			continue
		}
		for _, file := range matches {
			if seen[file] {
				continue
			}
			seen[file] = true
			content, err := os.ReadFile(file)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("read %s failed: %v", file, err))
				continue
			}
			parsed, err := rpgdsl.NewParser(string(content)).Parse()
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("parse %s failed: %v", file, err))
				continue
			}
			fragments = append(fragments, &rpgdsl.DSLFragment{
				DSL:      parsed,
				Phase:    writersdl.InferPhaseFromFilename(filepath.Base(file)),
				FilePath: file,
			})
		}
	}
	if len(fragments) == 0 && len(parseErrors) > 0 {
		return nil, errors.New(strings.Join(parseErrors, "; "))
	}
	return fragments, nil
}
