package setupcmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"novelgen/internal/app"
	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/creativity"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

const maxSetupExploreCandidates = 8

var (
	setupExploreCount        int
	setupExploreSeed         int64
	setupExploreLevel        string
	setupExploreMode         string
	setupExploreGenre        string
	setupExploreApplyBest    bool
	setupExploreStrict       bool
	setupExploreAgentSDK     bool
	setupExploreScenario     bool
	setupExploreSelect       int
	setupExplorePlanOnly     bool
	setupExplorePredict      bool
	setupExplorePredictLimit int
)

var setupExploreCmd = &cobra.Command{
	Use:   "explore [prompt]",
	Short: "Generate and rank several provocation-driven story setups",
	Long: `Generate several different story setups from one idea and rank them.

Each candidate starts from a deterministic "structured nonsense" provocation.
Go writes the full candidate JSON under story/setup/explore/<seed>/, runs the
deterministic setup quality gate, and asks the setup reviewer to score the
candidate. A second strict causal reviewer rejects candidates with multiple
axioms, orphan rules, institutions without a beneficiary/budget, zero-cost
bypasses, or unseeded twists. Generic-trope hits are reported and used as a
small ranking penalty.

Use --agent-sdk to run candidate generation, general review, and causal review
through the Agent SDK runtime; the deterministic quality gate stays in Go.

Use --scenario to generate Scenario Cards first, select a diverse subset at the
card level, and expand only those into full setups. In this mode --count is the
number of cards to draw (default 10) and --select is the number to expand
(default 3). Use --plan-only to stop after writing the selected-card plan.
Use --predict to rerank the candidate cards with a model-based "most obvious
developments" predictor before writing the plan.

Nothing replaces story/setup/story_setup.json unless --apply-best is set.

Examples:
  novelgen setup explore "一个关于宗门继承的故事" --random-genre 仙侠 --mode wild
  novelgen setup explore "一个星际大逃杀故事" --scenario --random-genre 科幻 --count 10 --select 3
  novelgen setup explore "一个星际大逃杀故事" --scenario --plan-only --count 20 --select 5
  novelgen setup explore "一个星际大逃杀故事" --scenario --plan-only --predict --count 20 --select 5 --predict-limit 8
  novelgen setup explore "一座会吞掉遗嘱的城市" --count 5 --seed 4217
  novelgen setup explore "一个关于宗门继承的故事" --agent-sdk --strict
  novelgen setup explore "一个关于宗门继承的故事" --strict=false
  novelgen setup explore "一个关于宗门继承的故事" --apply-best`,
	Args: cobra.ExactArgs(1),
	RunE: runSetupExplore,
}

type setupExploreCandidate struct {
	Candidate              int                             `json:"candidate"`
	Provocation            *creativity.Provocation         `json:"provocation,omitempty"`
	Scenario               *creativity.ScenarioCard        `json:"scenario,omitempty"`
	ScenarioFamily         string                          `json:"scenario_family,omitempty"`
	ScenarioNovelty        int                             `json:"scenario_novelty,omitempty"`
	ScenarioPredictability int                             `json:"scenario_predictability,omitempty"`
	Setup                  models.StorySetup               `json:"setup"`
	QualityScore           float64                         `json:"quality_score"`
	ReviewScore            float64                         `json:"review_score"`
	RankingScore           float64                         `json:"ranking_score"`
	GenericTropes          []string                        `json:"generic_tropes,omitempty"`
	Review                 models.ReviewResult             `json:"review"`
	CausalReview           *agents.SetupCausalReviewOutput `json:"causal_review,omitempty"`
	CausalRejected         bool                            `json:"causal_rejected"`
	CausalIssues           []string                        `json:"causal_issues,omitempty"`
	Gate                   quality.QualityGateResult       `json:"-"`
	Path                   string                          `json:"-"`
}

type setupExploreReport struct {
	Idea       string                `json:"idea"`
	Seed       int64                 `json:"seed"`
	Level      string                `json:"level"`
	Mode       string                `json:"mode"`
	Genre      string                `json:"genre"`
	Scenario   bool                  `json:"scenario"`
	Strict     bool                  `json:"strict"`
	Considered int                   `json:"considered,omitempty"`
	Selected   int                   `json:"selected,omitempty"`
	Warnings   []string              `json:"warnings,omitempty"`
	Ranked     []setupExploreSummary `json:"ranked"`
}

type setupExploreSummary struct {
	Rank                   int      `json:"rank"`
	Candidate              int      `json:"candidate"`
	ProvocationID          string   `json:"provocation_id"`
	Sentence               string   `json:"sentence"`
	ScenarioID             string   `json:"scenario_id,omitempty"`
	ScenarioPremise        string   `json:"scenario_premise,omitempty"`
	ScenarioFamily         string   `json:"scenario_family,omitempty"`
	ScenarioNovelty        int      `json:"scenario_novelty,omitempty"`
	ScenarioPredictability int      `json:"scenario_predictability,omitempty"`
	ProjectName            string   `json:"project_name"`
	Premise                string   `json:"premise"`
	QualityScore           float64  `json:"quality_score"`
	ReviewScore            float64  `json:"review_score"`
	RankingScore           float64  `json:"ranking_score"`
	CausalRejected         bool     `json:"causal_rejected"`
	CausalIssues           []string `json:"causal_issues,omitempty"`
	CausalSummary          string   `json:"causal_summary,omitempty"`
	Path                   string   `json:"path"`
}

func init() {
	setupCmd.AddCommand(setupExploreCmd)
	setupExploreCmd.Flags().IntVar(&setupExploreCount, "count", 5, "Number of candidates/cards to generate (max 8, or 20 in --scenario mode)")
	setupExploreCmd.Flags().Int64Var(&setupExploreSeed, "seed", 0, "Random seed; 0 picks and reports a replayable seed")
	setupExploreCmd.Flags().StringVar(&setupExploreLevel, "level", creativity.LevelLaw, "Constraint level: axiom, event, law, or name")
	setupExploreCmd.Flags().StringVar(&setupExploreMode, "mode", creativity.ModeWild, "Exploration mode: safe, balanced, wild, or grounded")
	setupExploreCmd.Flags().StringVar(&setupExploreGenre, "random-genre", "", "Genre hint for provocation/scenario draws: 仙侠, 都市, 悬疑, 科幻, or 通用")
	setupExploreCmd.Flags().BoolVar(&setupExploreApplyBest, "apply-best", false, "Replace the canonical story setup with the highest-ranked candidate")
	setupExploreCmd.Flags().BoolVar(&setupExploreStrict, "strict", true, "Reject candidates that fail the causal review")
	setupExploreCmd.Flags().BoolVar(&setupExploreAgentSDK, "agent-sdk", false, "Generate candidates and causal reviews through the Agent SDK runtime")
	setupExploreCmd.Flags().BoolVar(&setupExploreScenario, "scenario", false, "Draw Scenario Cards and expand only a diverse card-level selection")
	setupExploreCmd.Flags().IntVar(&setupExploreSelect, "select", 3, "Scenario mode: number of diverse cards to expand (maximum 5)")
	setupExploreCmd.Flags().BoolVar(&setupExplorePlanOnly, "plan-only", false, "Scenario mode: write the selected card plan and skip full setup generation")
	setupExploreCmd.Flags().BoolVar(&setupExplorePredict, "predict", false, "Scenario mode: use a model predictor to rerank the selected cards")
	setupExploreCmd.Flags().IntVar(&setupExplorePredictLimit, "predict-limit", 5, "Maximum cards to send to the model predictor")
}

func runSetupExplore(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN SETUP EXPLORE")

	session, err := app.Open(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	projectConfig := session.Config
	cfg, client, err := session.LoadLLM()
	if err != nil {
		return err
	}
	reviewAgent, err := session.NewSetupAgent()
	if err != nil {
		return err
	}

	count := setupExploreCount
	var warnings []string
	if count <= 0 {
		count = 5
	}
	scenarioMode := setupExploreScenario
	if setupExplorePlanOnly && !scenarioMode {
		return fmt.Errorf("--plan-only requires --scenario")
	}
	if scenarioMode && (cmd.Flags().Changed("level") || cmd.Flags().Changed("mode")) {
		warnings = append(warnings, "--level/--mode are ignored in --scenario mode")
	}
	if scenarioMode && !cmd.Flags().Changed("count") {
		count = 10
	}
	maxCount := maxSetupExploreCandidates
	if scenarioMode {
		maxCount = 20
	}
	if count > maxCount {
		warnings = append(warnings, fmt.Sprintf("count capped from %d to %d", count, maxCount))
		count = maxCount
	}

	generateCandidate := func(index, total int, provocation *creativity.Provocation, scenario *creativity.ScenarioCard, family string, novelty, predictability int) (setupExploreCandidate, error) {
		label := ""
		if scenario != nil {
			label = scenario.PremiseKernel
		} else if provocation != nil {
			label = provocation.Sentence
		}
		fmt.Printf("\n[%d/%d] %s\n", index, total, label)
		setup, err := generateStorySetupWithAI(
			cmd.Context(),
			args[0],
			projectConfig.Language,
			&projectConfig.LLM,
			cfg,
			client,
			provocation,
			scenario,
			setupExploreAgentSDK,
		)
		if err != nil {
			return setupExploreCandidate{}, fmt.Errorf("generate setup candidate %d: %w", index, err)
		}
		if scenario != nil {
			repaired, repairErr := repairSetupWithScenarioCoverage(cmd.Context(), setup, scenario, projectConfig, cfg, client, "exploration")
			if repairErr != nil {
				warnings = append(warnings, fmt.Sprintf("candidate %d scenario coverage repair: %v", index, repairErr))
			} else {
				setup = repaired
			}
		}
		gate := quality.RunSetupQualityGate(setup)
		var review agents.SetupReviewOutput
		if setupExploreAgentSDK {
			review, err = reviewAgent.ReviewWithAgentSDK(cmd.Context(), agents.SetupReviewInput{ExistingSetup: *setup})
		} else {
			review, err = reviewAgent.Review(cmd.Context(), agents.SetupReviewInput{ExistingSetup: *setup})
		}
		if err != nil {
			return setupExploreCandidate{}, fmt.Errorf("review setup candidate %d: %w", index, err)
		}
		var causalReview *agents.SetupCausalReviewOutput
		var causalIssues []string
		causalRejected := false
		if setupExploreStrict {
			causalInput := agents.SetupCausalReviewInput{
				Idea:          args[0],
				ExistingSetup: *setup,
				Scenario:      scenario,
			}
			if provocation != nil {
				causalInput.Provocation = *provocation
			}
			var audit agents.SetupCausalReviewOutput
			if setupExploreAgentSDK {
				audit, err = reviewAgent.ReviewCausalityWithAgentSDK(cmd.Context(), causalInput)
			} else {
				audit, err = reviewAgent.ReviewCausality(cmd.Context(), causalInput)
			}
			if err != nil {
				return setupExploreCandidate{}, fmt.Errorf("causal review setup candidate %d: %w", index, err)
			}
			causalReview = &audit
			causalRejected = audit.Rejected()
			causalIssues = audit.Issues()
		}
		if scenario != nil {
			if missing := scenarioCoverageMissing(setup, scenario); len(missing) > 0 {
				causalRejected = true
				causalIssues = append(causalIssues, "scenario_coverage: "+strings.Join(missing, ", "))
			}
		}
		tropes := genericSetupTropeHits(setup, args[0])
		return setupExploreCandidate{
			Candidate:              index,
			Provocation:            provocation,
			Scenario:               scenario,
			ScenarioFamily:         family,
			ScenarioNovelty:        novelty,
			ScenarioPredictability: predictability,
			Setup:                  *setup,
			QualityScore:           quality.ScoreFromGate(gate),
			ReviewScore:            review.Result.OverallScore,
			RankingScore:           review.Result.OverallScore - float64(2*len(tropes)),
			GenericTropes:          tropes,
			Review:                 review.Result,
			CausalReview:           causalReview,
			CausalRejected:         causalRejected,
			CausalIssues:           causalIssues,
			Gate:                   gate,
		}, nil
	}

	considered := 0
	selected := 0
	reportDrawn := creativity.Result{Seed: setupExploreSeed, Level: setupExploreLevel, Mode: setupExploreMode, Genre: setupExploreGenre}
	candidates := make([]setupExploreCandidate, 0, count)
	if scenarioMode {
		scenarioResult, err := creativity.GenerateScenarios(creativity.ScenarioRequest{
			Genre: setupExploreGenre,
			Seed:  setupExploreSeed,
			Count: count,
		})
		if err != nil {
			return fmt.Errorf("draw setup exploration scenarios: %w", err)
		}
		selectCount := setupExploreSelect
		if selectCount <= 0 {
			selectCount = 3
		}
		if selectCount > 5 {
			warnings = append(warnings, fmt.Sprintf("select capped from %d to 5", selectCount))
			selectCount = 5
		}
		plan := creativity.PlanScenarios(scenarioResult, selectCount)
		if setupExplorePredict {
			predicted, err := predictScenarioPlan(cmd.Context(), reviewAgent, args[0], plan, setupExplorePredictLimit, setupExploreAgentSDK)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("scenario predictor failed: %v", err))
			} else {
				plan = predicted
			}
		}
		considered = len(scenarioResult.Cards)
		selected = len(plan.Selected)
		fmt.Printf("Scenario plan: considered=%d selected=%d seed=%d\n", considered, selected, scenarioResult.Seed)
		if setupExplorePlanOnly {
			planPath, err := writeScenarioPlan(book.Root, scenarioResult.Seed, plan)
			if err != nil {
				return err
			}
			for i, item := range plan.Selected {
				fmt.Printf("%d. %s\n   id=%s family=%s\n   novelty=%d predictability=%d\n   reason=%s\n",
					i+1, item.Card.PremiseKernel, item.Card.ID, item.Family, item.Novelty, item.Predictability, item.SelectionReason)
			}
			fmt.Printf("\nPlan written: %s\n", planPath)
			fmt.Println("No full setup was generated. Pick a card and run setup gen --scenario-index <n>.")
			return nil
		}
		for i, item := range plan.Selected {
			candidate, err := generateCandidate(i+1, len(plan.Selected), nil, &item.Card, item.Family, item.Novelty, item.Predictability)
			if err != nil {
				return err
			}
			candidates = append(candidates, candidate)
		}
		reportDrawn = creativity.Result{Seed: scenarioResult.Seed, Level: "scenario", Mode: "scenario", Genre: scenarioResult.Genre}
	} else {
		drawn, err := creativity.Generate(creativity.Request{
			Level: setupExploreLevel,
			Genre: setupExploreGenre,
			Mode:  setupExploreMode,
			Seed:  setupExploreSeed,
			Count: count,
		})
		if err != nil {
			return fmt.Errorf("draw setup exploration provocations: %w", err)
		}
		warnings = append(warnings, drawn.Warnings...)
		if len(drawn.Provocations) == 0 {
			return fmt.Errorf("setup exploration produced no provocations for seed %d", drawn.Seed)
		}
		considered = len(drawn.Provocations)
		selected = considered
		fmt.Printf("Exploring %d setup candidates with seed %d\n", considered, drawn.Seed)
		for i := range drawn.Provocations {
			provocation := &drawn.Provocations[i]
			candidate, err := generateCandidate(i+1, considered, provocation, nil, "", 0, 0)
			if err != nil {
				return err
			}
			candidates = append(candidates, candidate)
		}
		reportDrawn = drawn
	}

	sortSetupExploreCandidates(candidates)

	for i := range candidates {
		path, err := writeSetupExploreCandidate(book.Root, reportDrawn.Seed, candidates[i])
		if err != nil {
			return err
		}
		candidates[i].Path = path
	}
	report := buildSetupExploreReport(args[0], reportDrawn, candidates, warnings, setupExploreStrict, scenarioMode, considered, selected)
	reportPath := book.Root.SetupExploreFile(reportDrawn.Seed, "report.json")
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal setup exploration report: %w", err)
	}
	if err := book.Root.Write(reportPath, reportData, store.FileMode); err != nil {
		return fmt.Errorf("write setup exploration report: %w", err)
	}

	fmt.Printf("\nRanking (seed %d):\n", reportDrawn.Seed)
	for i, candidate := range candidates {
		causal := "skip"
		if candidate.CausalReview != nil {
			if candidate.CausalRejected {
				causal = "REJECT"
			} else {
				causal = "keep"
			}
		}
		fmt.Printf("%d. candidate-%02d  review=%.1f  quality=%.1f  tropes=%d  causal=%s\n   %s\n   %s\n",
			i+1,
			candidate.Candidate,
			candidate.ReviewScore,
			candidate.QualityScore,
			len(candidate.GenericTropes),
			causal,
			candidate.Setup.ProjectName,
			candidate.Setup.Premise,
		)
		if len(candidate.CausalIssues) > 0 {
			fmt.Printf("   causal issues: %s\n", summarizeCausalIssues(candidate.CausalIssues, 3))
		}
	}
	fmt.Printf("\nCandidates: %s\nReport: %s\n", book.Root.SetupExploreDir(reportDrawn.Seed), reportPath)

	if setupExploreApplyBest {
		var best *setupExploreCandidate
		for i := range candidates {
			if !candidates[i].CausalRejected {
				best = &candidates[i]
				break
			}
		}
		if best == nil {
			return fmt.Errorf("all candidates were rejected by causal review; use --strict=false or a different seed")
		}
		if book.Root.HasSetup() {
			existing, err := book.LoadSetup()
			if err != nil {
				return fmt.Errorf("load existing setup before apply-best: %w", err)
			}
			if _, err := saveSetupCheckpoint(book, existing); err != nil {
				return fmt.Errorf("checkpoint existing setup before apply-best: %w", err)
			}
		}
		if err := saveStorySetup(cmd.Context(), book, &best.Setup); err != nil {
			return fmt.Errorf("save best setup candidate: %w", err)
		}
		fmt.Printf("\nApplied candidate-%02d as the canonical story setup.\n", best.Candidate)
	} else {
		fmt.Println("No canonical setup was changed. Re-run with --apply-best to install the top candidate.")
	}
	return nil
}

func writeSetupExploreCandidate(root *store.Root, seed int64, candidate setupExploreCandidate) (string, error) {
	path := root.SetupExploreFile(seed, fmt.Sprintf("candidate-%02d.json", candidate.Candidate))
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal setup candidate %d: %w", candidate.Candidate, err)
	}
	if err := root.Write(path, data, store.FileMode); err != nil {
		return "", fmt.Errorf("write setup candidate %d: %w", candidate.Candidate, err)
	}
	return path, nil
}

func sortSetupExploreCandidates(candidates []setupExploreCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].CausalRejected != candidates[j].CausalRejected {
			return !candidates[i].CausalRejected
		}
		if candidates[i].RankingScore == candidates[j].RankingScore {
			return candidates[i].QualityScore > candidates[j].QualityScore
		}
		return candidates[i].RankingScore > candidates[j].RankingScore
	})
}

func summarizeCausalIssues(issues []string, max int) string {
	if len(issues) == 0 {
		return ""
	}
	if max <= 0 || len(issues) <= max {
		return strings.Join(issues, "; ")
	}
	return strings.Join(issues[:max], "; ") + fmt.Sprintf("; ... %d more", len(issues)-max)
}

func buildSetupExploreReport(idea string, drawn creativity.Result, candidates []setupExploreCandidate, warnings []string, strict, scenario bool, considered, selected int) setupExploreReport {
	report := setupExploreReport{
		Idea:       strings.TrimSpace(idea),
		Seed:       drawn.Seed,
		Level:      drawn.Level,
		Mode:       drawn.Mode,
		Genre:      drawn.Genre,
		Scenario:   scenario,
		Strict:     strict,
		Considered: considered,
		Selected:   selected,
		Warnings:   append([]string(nil), warnings...),
		Ranked:     make([]setupExploreSummary, 0, len(candidates)),
	}
	for i, candidate := range candidates {
		provocationID := ""
		sentence := ""
		if candidate.Provocation != nil {
			provocationID = candidate.Provocation.ID
			sentence = candidate.Provocation.Sentence
		}
		scenarioID := ""
		scenarioPremise := ""
		if candidate.Scenario != nil {
			scenarioID = candidate.Scenario.ID
			scenarioPremise = candidate.Scenario.PremiseKernel
		}
		report.Ranked = append(report.Ranked, setupExploreSummary{
			Rank:                   i + 1,
			Candidate:              candidate.Candidate,
			ProvocationID:          provocationID,
			Sentence:               sentence,
			ScenarioID:             scenarioID,
			ScenarioPremise:        scenarioPremise,
			ScenarioFamily:         candidate.ScenarioFamily,
			ScenarioNovelty:        candidate.ScenarioNovelty,
			ScenarioPredictability: candidate.ScenarioPredictability,
			ProjectName:            candidate.Setup.ProjectName,
			Premise:                candidate.Setup.Premise,
			QualityScore:           candidate.QualityScore,
			ReviewScore:            candidate.ReviewScore,
			RankingScore:           candidate.RankingScore,
			CausalRejected:         candidate.CausalRejected,
			CausalIssues:           append([]string(nil), candidate.CausalIssues...),
			Path:                   candidate.Path,
		})
		if candidate.CausalReview != nil {
			report.Ranked[len(report.Ranked)-1].CausalSummary = candidate.CausalReview.Summary
		}
	}
	return report
}

var genericSetupTropes = []string{
	"废柴",
	"逆袭",
	"退婚",
	"老爷爷",
	"戒指里的灵魂",
	"天降金手指",
	"系统面板",
	"宗门大比",
	"秘境夺宝",
	"拍卖会捡漏",
	"扮猪吃虎",
	"神秘血脉",
	"灭门惨案",
}

func genericSetupTropeHits(setup *models.StorySetup, idea string) []string {
	if setup == nil {
		return nil
	}
	text := setup.Premise + "\n" + setup.Theme + "\n" + strings.Join(setup.Rules, "\n")
	for _, storyline := range setup.Storylines {
		text += "\n" + storyline.Name + "\n" + storyline.Description + "\n" + storyline.Turn
	}
	for _, premise := range setup.Premises {
		text += "\n" + premise.Name + "\n" + premise.Description
	}
	for _, seed := range setup.CoreCast {
		text += "\n" + seed.StoryFunction + "\n" + seed.Payoff
	}

	var hits []string
	for _, trope := range genericSetupTropes {
		// An explicit user request outranks the generic-trope penalty. If the
		// author asked for a named trope, using it is intent, not sameness.
		if strings.Contains(idea, trope) {
			continue
		}
		if strings.Contains(text, trope) {
			hits = append(hits, trope)
		}
	}
	return hits
}
