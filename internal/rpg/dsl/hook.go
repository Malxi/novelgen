package dsl

import (
	"novelgen/internal/rpg"
	"strings"
)

// HookManager manages all hooks and counters
type HookManager struct {
	hooks     []Hook
	counters  map[string]*CounterState
	triggers  []TriggerDef
	evaluator *ExpressionEvaluator
}

// CounterState tracks the current state of a counter
type CounterState struct {
	Name       string
	Value      int
	Max        int
	Filter     string
	Milestones []Milestone
}

// NewHookManager creates a new hook manager
func NewHookManager() *HookManager {
	return &HookManager{
		hooks:     make([]Hook, 0),
		counters:  make(map[string]*CounterState),
		triggers:  make([]TriggerDef, 0),
		evaluator: NewExpressionEvaluator(),
	}
}

// RegisterHooks registers hooks from DSL
func (hm *HookManager) RegisterHooks(systems *Systems) {
	if systems == nil {
		return
	}

	// Register hooks
	for _, hook := range systems.Hooks {
		hm.hooks = append(hm.hooks, hook)

		// Initialize counters for this hook
		for _, counter := range hook.Counters {
			hm.counters[counter.Name] = &CounterState{
				Name:       counter.Name,
				Value:      0,
				Max:        counter.Max,
				Filter:     counter.Filter,
				Milestones: counter.Milestones,
			}
		}
	}

	// Register triggers
	for _, trigger := range systems.Triggers {
		hm.triggers = append(hm.triggers, trigger)
	}
}

// OnKill is called when a character kills an enemy
func (hm *HookManager) OnKill(killer *rpg.Character, victim *rpg.Character, world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	for _, hook := range hm.hooks {
		if hook.EventType != "on_kill" {
			continue
		}

		// Check condition
		if hook.Condition != "" {
			if !hm.evaluator.EvaluateCondition(hook.Condition, killer, world) {
				continue
			}
		}

		// Update counters
		for _, counter := range hook.Counters {
			if state, ok := hm.counters[counter.Name]; ok {
				// Check filter
				if counter.Filter != "" {
					if !hm.matchesKillFilter(counter.Filter, victim) {
						continue
					}
				}

				// Increment counter
				oldValue := state.Value
				state.Value++
				if state.Max > 0 && state.Value > state.Max {
					state.Value = state.Max
				}

				// Check milestones
				for _, milestone := range state.Milestones {
					if oldValue < milestone.Value && state.Value >= milestone.Value {
						results = append(results, HookResult{
							Type:    "milestone",
							Counter: counter.Name,
							Value:   milestone.Value,
							Reward:  milestone.Reward,
						})
					}
				}
			}
		}
	}

	// Check triggers
	triggerResults := hm.checkTriggers(world)
	results = append(results, triggerResults...)

	return results
}

// OnDamageTaken is called when a character takes damage
func (hm *HookManager) OnDamageTaken(target *rpg.Character, damage int, world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	for _, hook := range hm.hooks {
		if hook.EventType != "on_damage_taken" {
			continue
		}

		// Check condition
		if hook.Condition != "" {
			if !hm.evaluator.EvaluateCondition(hook.Condition, target, world) {
				continue
			}
		}

		// Update counters
		for _, counter := range hook.Counters {
			if state, ok := hm.counters[counter.Name]; ok {
				// Check filter (e.g., damage >= max_hp * 0.5)
				if counter.Filter != "" {
					if !hm.matchesDamageFilter(counter.Filter, damage, target) {
						continue
					}
				}

				oldValue := state.Value
				state.Value++

				// Check milestones
				for _, milestone := range state.Milestones {
					if oldValue < milestone.Value && state.Value >= milestone.Value {
						results = append(results, HookResult{
							Type:    "milestone",
							Counter: counter.Name,
							Value:   milestone.Value,
							Reward:  milestone.Reward,
						})
					}
				}
			}
		}
	}

	// Check triggers
	triggerResults := hm.checkTriggers(world)
	results = append(results, triggerResults...)

	return results
}

// checkTriggers checks all triggers
func (hm *HookManager) checkTriggers(world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	for _, trigger := range hm.triggers {
		// Skip if trigger is one-time and already triggered
		if trigger.Once {
			// In a real implementation, we'd track triggered state
			continue
		}

		// Evaluate conditions
		if hm.evaluator.EvaluateConditions(trigger.Conditions, world.Player, world) {
			results = append(results, HookResult{
				Type:      "trigger",
				TriggerID: trigger.ID,
				Narration: trigger.OnTrigger.Narration,
				Reward: Reward{
					Exp:   trigger.OnTrigger.Exp,
					Items: trigger.OnTrigger.Items,
					Flags: []string{trigger.OnTrigger.SetFlag},
				},
			})
		}
	}

	return results
}

// Helper methods

func (hm *HookManager) matchesKillFilter(filter string, victim *rpg.Character) bool {
	if victim != nil && strings.Contains(filter, victim.ID) {
		return true
	}
	return true
}

func (hm *HookManager) matchesDamageFilter(filter string, damage int, target *rpg.Character) bool {
	if target.BaseStats.HP > 0 {
		percentage := float64(damage) / float64(target.BaseStats.HP)
		return percentage >= 0.5
	}
	return false
}

// GetCounter returns the current value of a counter
func (hm *HookManager) GetCounter(name string) (int, bool) {
	if state, ok := hm.counters[name]; ok {
		return state.Value, true
	}
	return 0, false
}

// HookResult represents the result of a hook execution
type HookResult struct {
	Type      string
	Counter   string
	Value     int
	TriggerID string
	Narration string
	Reward    Reward
}
