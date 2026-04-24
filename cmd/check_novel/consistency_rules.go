package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type consistencyRules struct {
	Modules    ruleModules    `yaml:"modules"`
	Thresholds ruleThresholds `yaml:"thresholds"`
	Keywords   ruleKeywords   `yaml:"keywords"`
}

type ruleModules struct {
	TimeGapTransition         *bool `yaml:"time_gap_transition"`
	ResurrectionConsistency   *bool `yaml:"resurrection_rule_consistency"`
	CultivationLifespan       *bool `yaml:"cultivation_lifespan_coupling"`
	NPCIdentityConflict       *bool `yaml:"npc_identity_conflict"`
	InjuryRecoveryContinuity  *bool `yaml:"injury_recovery_continuity"`
	ResourceBudgetConsistency *bool `yaml:"resource_budget_consistency"`
}

type ruleThresholds struct {
	TimeGapDays              int `yaml:"time_gap_days"`
	ResourceMinReuseCount    int `yaml:"resource_min_reuse_count"`
	ResurrectionDiversityMin int `yaml:"resurrection_diversity_min"`
}

type ruleKeywords struct {
	TransitionHints []string `yaml:"transition_hints"`
	FloorRuleHints  []string `yaml:"floor_rule_hints"`
	ResourceNames   []string `yaml:"resource_names"`
	ResourceConsume []string `yaml:"resource_consume"`
}

func defaultConsistencyRules() *consistencyRules {
	return &consistencyRules{
		Modules: ruleModules{
			TimeGapTransition:         boolPtr(true),
			ResurrectionConsistency:   boolPtr(true),
			CultivationLifespan:       boolPtr(true),
			NPCIdentityConflict:       boolPtr(true),
			InjuryRecoveryContinuity:  boolPtr(true),
			ResourceBudgetConsistency: boolPtr(true),
		},
		Thresholds: ruleThresholds{
			TimeGapDays:              10,
			ResourceMinReuseCount:    2,
			ResurrectionDiversityMin: 3,
		},
		Keywords: ruleKeywords{
			TransitionHints: []string{"这段时间", "这半个月", "期间", "过去", "经过", "连续", "连日", "逐渐"},
			FloorRuleHints:  []string{"练气一层是底线", "跌破练气一层", "代价转移到寿元", "扣寿元", "折损寿元"},
			ResourceNames:   []string{"半块下品灵石", "半块灵石"},
			ResourceConsume: []string{"吐纳", "修炼", "吸纳灵气", "炼化"},
		},
	}
}

func loadConsistencyRules(bookPath string) *consistencyRules {
	rules := defaultConsistencyRules()
	path := filepath.Join(bookPath, "story", "setup", "world_rules.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return rules
	}

	var user consistencyRules
	if err := yaml.Unmarshal(b, &user); err != nil {
		return rules
	}
	mergeRules(rules, &user)
	return rules
}

func mergeRules(dst, src *consistencyRules) {
	// modules
	if src.Modules.TimeGapTransition != nil {
		dst.Modules.TimeGapTransition = src.Modules.TimeGapTransition
	}
	if src.Modules.ResurrectionConsistency != nil {
		dst.Modules.ResurrectionConsistency = src.Modules.ResurrectionConsistency
	}
	if src.Modules.CultivationLifespan != nil {
		dst.Modules.CultivationLifespan = src.Modules.CultivationLifespan
	}
	if src.Modules.NPCIdentityConflict != nil {
		dst.Modules.NPCIdentityConflict = src.Modules.NPCIdentityConflict
	}
	if src.Modules.InjuryRecoveryContinuity != nil {
		dst.Modules.InjuryRecoveryContinuity = src.Modules.InjuryRecoveryContinuity
	}
	if src.Modules.ResourceBudgetConsistency != nil {
		dst.Modules.ResourceBudgetConsistency = src.Modules.ResourceBudgetConsistency
	}

	// thresholds
	if src.Thresholds.TimeGapDays > 0 {
		dst.Thresholds.TimeGapDays = src.Thresholds.TimeGapDays
	}
	if src.Thresholds.ResourceMinReuseCount > 0 {
		dst.Thresholds.ResourceMinReuseCount = src.Thresholds.ResourceMinReuseCount
	}
	if src.Thresholds.ResurrectionDiversityMin > 0 {
		dst.Thresholds.ResurrectionDiversityMin = src.Thresholds.ResurrectionDiversityMin
	}

	// keywords
	if len(src.Keywords.TransitionHints) > 0 {
		dst.Keywords.TransitionHints = src.Keywords.TransitionHints
	}
	if len(src.Keywords.FloorRuleHints) > 0 {
		dst.Keywords.FloorRuleHints = src.Keywords.FloorRuleHints
	}
	if len(src.Keywords.ResourceNames) > 0 {
		dst.Keywords.ResourceNames = src.Keywords.ResourceNames
	}
	if len(src.Keywords.ResourceConsume) > 0 {
		dst.Keywords.ResourceConsume = src.Keywords.ResourceConsume
	}
}

func boolPtr(v bool) *bool { return &v }
