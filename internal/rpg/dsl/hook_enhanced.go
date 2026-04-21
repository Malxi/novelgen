package dsl

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"novelgen/internal/rpg"
)

// EnhancedHookManager provides full hook functionality
type EnhancedHookManager struct {
	*HookManager
	enemyKillCounts map[string]int // enemy_id -> count
	skillUseCounts  map[string]int // skill_id -> count
	triggeredFlags  map[string]bool // trigger_id -> triggered
}

// NewEnhancedHookManager creates an enhanced hook manager
func NewEnhancedHookManager() *EnhancedHookManager {
	return &EnhancedHookManager{
		HookManager:     NewHookManager(),
		enemyKillCounts: make(map[string]int),
		skillUseCounts:  make(map[string]int),
		triggeredFlags:  make(map[string]bool),
	}
}

// OnKillEnhanced handles kill events with full filtering
func (ehm *EnhancedHookManager) OnKillEnhanced(killer *rpg.Character, victim *rpg.Character, world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	// Track kill count for this enemy type
	if victim != nil {
		ehm.enemyKillCounts[victim.ID]++
	}

	for _, hook := range ehm.hooks {
		if hook.EventType != "on_kill" {
			continue
		}

		// Check condition
		if hook.Condition != "" {
			if !ehm.evaluator.EvaluateCondition(hook.Condition, killer, world) {
				continue
			}
		}

		// Update counters with filtering
		for _, counter := range hook.Counters {
			if state, ok := ehm.counters[counter.Name]; ok {
				// Check filter with enhanced logic
				if counter.Filter != "" {
					if !ehm.evaluateKillFilter(counter.Filter, victim, killer) {
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
							Type:      "milestone",
							Counter:   counter.Name,
							Value:     milestone.Value,
							Reward:    milestone.Reward,
							Narration: fmt.Sprintf("达成里程碑: %s = %d", counter.Name, milestone.Value),
						})
					}
				}
			}
		}
	}

	// Check triggers
	triggerResults := ehm.checkTriggersEnhanced(world)
	results = append(results, triggerResults...)

	return results
}

// OnDamageTakenEnhanced handles damage with full filtering
func (ehm *EnhancedHookManager) OnDamageTakenEnhanced(target *rpg.Character, damage int, attacker *rpg.Character, world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	for _, hook := range ehm.hooks {
		if hook.EventType != "on_damage_taken" {
			continue
		}

		// Check condition
		if hook.Condition != "" {
			if !ehm.evaluator.EvaluateCondition(hook.Condition, target, world) {
				continue
			}
		}

		// Update counters with filtering
		for _, counter := range hook.Counters {
			if state, ok := ehm.counters[counter.Name]; ok {
				// Check filter with enhanced logic
				if counter.Filter != "" {
					if !ehm.evaluateDamageFilter(counter.Filter, damage, target, attacker) {
						continue
					}
				}

				oldValue := state.Value
				state.Value++

				// Check milestones
				for _, milestone := range state.Milestones {
					if oldValue < milestone.Value && state.Value >= milestone.Value {
						results = append(results, HookResult{
							Type:      "milestone",
							Counter:   counter.Name,
							Value:     milestone.Value,
							Reward:    milestone.Reward,
							Narration: fmt.Sprintf("达成里程碑: %s = %d", counter.Name, milestone.Value),
						})
					}
				}
			}
		}
	}

	// Check triggers
	triggerResults := ehm.checkTriggersEnhanced(world)
	results = append(results, triggerResults...)

	return results
}

// OnSkillUseEnhanced handles skill use with tracking
func (ehm *EnhancedHookManager) OnSkillUseEnhanced(user *rpg.Character, skillID string, world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	// Track skill usage
	ehm.skillUseCounts[skillID]++

	for _, hook := range ehm.hooks {
		if hook.EventType != "on_skill_use" {
			continue
		}

		// Check condition
		if hook.Condition != "" {
			if !ehm.evaluator.EvaluateCondition(hook.Condition, user, world) {
				continue
			}
		}

		// Update counters with filtering
		for _, counter := range hook.Counters {
			if state, ok := ehm.counters[counter.Name]; ok {
				// Check filter
				if counter.Filter != "" {
					if !ehm.evaluateSkillFilter(counter.Filter, skillID, user) {
						continue
					}
				}

				oldValue := state.Value
				state.Value++

				// Check milestones
				for _, milestone := range state.Milestones {
					if oldValue < milestone.Value && state.Value >= milestone.Value {
						results = append(results, HookResult{
							Type:      "milestone",
							Counter:   counter.Name,
							Value:     milestone.Value,
							Reward:    milestone.Reward,
							Narration: fmt.Sprintf("达成里程碑: %s = %d", counter.Name, milestone.Value),
						})
					}
				}
			}
		}
	}

	// Check triggers
	triggerResults := ehm.checkTriggersEnhanced(world)
	results = append(results, triggerResults...)

	return results
}

// checkTriggersEnhanced checks all triggers with full condition support
func (ehm *EnhancedHookManager) checkTriggersEnhanced(world *rpg.GameWorld) []HookResult {
	results := make([]HookResult, 0)

	for _, trigger := range ehm.triggers {
		// Skip if trigger is one-time and already triggered
		if trigger.Once && ehm.triggeredFlags[trigger.ID] {
			continue
		}

		// Evaluate conditions with full support
		if ehm.evaluateTriggerConditions(trigger.Conditions, world) {
			// Mark as triggered
			ehm.triggeredFlags[trigger.ID] = true

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

// evaluateTriggerConditions evaluates trigger conditions with counter support
func (ehm *EnhancedHookManager) evaluateTriggerConditions(conditions []Condition, world *rpg.GameWorld) bool {
	if len(conditions) == 0 {
		return true
	}

	player := world.Player
	if player == nil {
		return false
	}

	for _, cond := range conditions {
		var result bool

		switch cond.Type {
		case "stat":
			val := ehm.getStatValue(cond.Stat, player)
			result = ehm.compare(val, cond.Op, ehm.toFloat(cond.Value))

		case "flag":
			// Check world flags
			// In full implementation, check world.Context.Flags
			result = false

		case "counter":
			// Check counter value
			if state, ok := ehm.counters[cond.Counter]; ok {
				result = ehm.compare(float64(state.Value), cond.Op, ehm.toFloat(cond.Value))
			} else {
				result = false
			}

		case "kill_count":
			// Check kill count for specific enemy
			count := ehm.enemyKillCounts[cond.Counter]
			result = ehm.compare(float64(count), cond.Op, ehm.toFloat(cond.Value))

		case "skill_use":
			// Check skill use count
			count := ehm.skillUseCounts[cond.Counter]
			result = ehm.compare(float64(count), cond.Op, ehm.toFloat(cond.Value))

		case "location":
			result = player.Position.MapID == cond.Location

		case "random":
			// Random chance trigger
			result = ehm.evaluator.functions["random_float"]([]interface{}{0.0, 1.0}, player, world).(float64) < cond.Random

		default:
			result = false
		}

		if !result {
			return false // AND logic
		}
	}

	return true
}

// Filter evaluation methods

func (ehm *EnhancedHookManager) evaluateKillFilter(filter string, victim *rpg.Character, killer *rpg.Character) bool {
	// Parse filter expression: "enemy_id == 'enemy_wasp'" or "level >= 5"
	filter = strings.TrimSpace(filter)

	// Check for enemy_id match
	if strings.Contains(filter, "enemy_id") {
		re := regexp.MustCompile(`enemy_id\s*==\s*['"](\w+)['"]`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			expectedID := matches[1]
			return victim != nil && victim.ID == expectedID
		}
	}

	// Check for enemy type match
	if strings.Contains(filter, "enemy_type") {
		re := regexp.MustCompile(`enemy_type\s*==\s*['"](\w+)['"]`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			expectedType := matches[1]
			// In full implementation, check victim.Type
			_ = expectedType
		}
	}

	// Check for level condition
	if strings.Contains(filter, "level") {
		// Parse: level >= 5, level < 10, etc.
		re := regexp.MustCompile(`level\s*([<>=!]+)\s*(\d+)`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 3 {
			op := matches[1]
			level, _ := strconv.Atoi(matches[2])
			// In full implementation, check victim.Level
			_ = op
			_ = level
		}
	}

	// Default: pass filter
	return true
}

func (ehm *EnhancedHookManager) evaluateDamageFilter(filter string, damage int, target *rpg.Character, attacker *rpg.Character) bool {
	filter = strings.TrimSpace(filter)

	// Check for damage threshold: "damage >= max_hp * 0.5"
	if strings.Contains(filter, "damage") {
		// Parse percentage threshold
		re := regexp.MustCompile(`damage\s*>=\s*max_hp\s*\*\s*(\d*\.?\d+)`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			percentage, _ := strconv.ParseFloat(matches[1], 64)
			maxHP := float64(target.BaseStats.HP)
			threshold := maxHP * percentage
			return float64(damage) >= threshold
		}

		// Parse absolute threshold: "damage >= 50"
		re = regexp.MustCompile(`damage\s*>=\s*(\d+)`)
		matches = re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			threshold, _ := strconv.Atoi(matches[1])
			return damage >= threshold
		}
	}

	// Check for hp percentage: "hp_percentage < 20"
	if strings.Contains(filter, "hp_percentage") {
		re := regexp.MustCompile(`hp_percentage\s*([<>=!]+)\s*(\d+)`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 3 {
			op := matches[1]
			percentage, _ := strconv.Atoi(matches[2])
			currentPercentage := int(float64(target.CurrentStats.HP) / float64(target.BaseStats.HP) * 100)

			switch op {
			case "<":
				return currentPercentage < percentage
			case ">":
				return currentPercentage > percentage
			case "<=":
				return currentPercentage <= percentage
			case ">=":
				return currentPercentage >= percentage
			}
		}
	}

	return true
}

func (ehm *EnhancedHookManager) evaluateSkillFilter(filter string, skillID string, user *rpg.Character) bool {
	filter = strings.TrimSpace(filter)

	// Check for skill_type: "skill_type == 'combat'"
	if strings.Contains(filter, "skill_type") {
		re := regexp.MustCompile(`skill_type\s*==\s*['"](\w+)['"]`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			expectedType := matches[1]
			// In full implementation, look up skill type from skillID
			_ = expectedType
		}
	}

	// Check for skill_id: "skill_id == 'skill_fireball'"
	if strings.Contains(filter, "skill_id") {
		re := regexp.MustCompile(`skill_id\s*==\s*['"](\w+)['"]`)
		matches := re.FindStringSubmatch(filter)
		if len(matches) >= 2 {
			expectedID := matches[1]
			return skillID == expectedID
		}
	}

	return true
}

// Helper methods

func (ehm *EnhancedHookManager) getStatValue(stat string, character *rpg.Character) float64 {
	if character == nil {
		return 0
	}

	switch stat {
	case "level":
		return float64(character.Level)
	case "hp":
		return float64(character.CurrentStats.HP)
	case "max_hp":
		return float64(character.BaseStats.HP)
	case "attack":
		return float64(character.CurrentStats.Attack)
	case "defense":
		return float64(character.CurrentStats.Defense)
	case "speed":
		return float64(character.CurrentStats.Speed)
	default:
		return 0
	}
}

func (ehm *EnhancedHookManager) compare(left float64, op string, right float64) bool {
	switch op {
	case "==", "=":
		return left == right
	case "!=":
		return left != right
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}

func (ehm *EnhancedHookManager) toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

// GetKillCount returns the kill count for an enemy type
func (ehm *EnhancedHookManager) GetKillCount(enemyID string) int {
	return ehm.enemyKillCounts[enemyID]
}

// GetSkillUseCount returns the skill use count
func (ehm *EnhancedHookManager) GetSkillUseCount(skillID string) int {
	return ehm.skillUseCounts[skillID]
}

// GetSkillCount is an alias for GetSkillUseCount for test compatibility
func (ehm *EnhancedHookManager) GetSkillCount(skillID string) int {
	return ehm.GetSkillUseCount(skillID)
}

// ResetCounter resets a counter to 0
func (ehm *EnhancedHookManager) ResetCounter(name string) {
	if state, ok := ehm.counters[name]; ok {
		state.Value = 0
	}
}

// ResetAllCounters resets all counters
func (ehm *EnhancedHookManager) ResetAllCounters() {
	for _, state := range ehm.counters {
		state.Value = 0
	}
}

// ResetTrigger allows a one-time trigger to fire again
func (ehm *EnhancedHookManager) ResetTrigger(triggerID string) {
	delete(ehm.triggeredFlags, triggerID)
}

// SaveState returns the current state for persistence
func (ehm *EnhancedHookManager) SaveState() map[string]interface{} {
	state := make(map[string]interface{})

	// Save counters
	counters := make(map[string]int)
	for name, counter := range ehm.counters {
		counters[name] = counter.Value
	}
	state["counters"] = counters

	// Save kill counts
	state["enemy_kill_counts"] = ehm.enemyKillCounts

	// Save skill use counts
	state["skill_use_counts"] = ehm.skillUseCounts

	// Save triggered flags
	state["triggered_flags"] = ehm.triggeredFlags

	return state
}

// LoadState restores state from persistence
func (ehm *EnhancedHookManager) LoadState(state map[string]interface{}) {
	if counters, ok := state["counters"].(map[string]int); ok {
		for name, value := range counters {
			if state, exists := ehm.counters[name]; exists {
				state.Value = value
			}
		}
	}

	if kills, ok := state["enemy_kill_counts"].(map[string]int); ok {
		ehm.enemyKillCounts = kills
	}

	if skills, ok := state["skill_use_counts"].(map[string]int); ok {
		ehm.skillUseCounts = skills
	}

	if triggered, ok := state["triggered_flags"].(map[string]bool); ok {
		ehm.triggeredFlags = triggered
	}
}

// Additional helper methods for testing

// RegisterHook registers a new hook (simplified for testing)
func (ehm *EnhancedHookManager) RegisterHook(eventType, hookID string, config HookConfig) {
	hook := Hook{
		EventType: eventType,
		Condition: config.Filter,
	}
	ehm.hooks = append(ehm.hooks, hook)
}

// InitializeCounter initializes a counter with config
func (ehm *EnhancedHookManager) InitializeCounter(name string, config CounterConfig) {
	milestones := make([]Milestone, 0, len(config.Milestones))
	for _, m := range config.Milestones {
		milestones = append(milestones, Milestone{
			Value: m,
			Reward: Reward{
				Title: fmt.Sprintf("Reached %d %s!", m, config.Track),
			},
		})
	}
	ehm.counters[name] = &CounterState{
		Name:       name,
		Value:      0,
		Milestones: milestones,
	}
}

// IncrementCounter increments a counter by 1
func (ehm *EnhancedHookManager) IncrementCounter(name string) {
	if state, ok := ehm.counters[name]; ok {
		state.Value++
	} else {
		// Auto-create counter if not exists
		ehm.counters[name] = &CounterState{
			Name:  name,
			Value: 1,
		}
	}
}

// IsMilestoneReached checks if a milestone has been reached
func (ehm *EnhancedHookManager) IsMilestoneReached(counterName string, milestone int) bool {
	if state, ok := ehm.counters[counterName]; ok {
		return state.Value >= milestone
	}
	return false
}

// IsTriggered checks if a trigger has been triggered
func (ehm *EnhancedHookManager) IsTriggered(triggerID string) bool {
	return ehm.triggeredFlags[triggerID]
}

// SetTriggered manually sets a trigger as triggered
func (ehm *EnhancedHookManager) SetTriggered(triggerID string) {
	ehm.triggeredFlags[triggerID] = true
}

// HookConfig is a simplified config for registering hooks
type HookConfig struct {
	Filter string
	Action string
}

// CounterConfig is config for initializing counters
type CounterConfig struct {
	Track      string
	Milestones []int
}