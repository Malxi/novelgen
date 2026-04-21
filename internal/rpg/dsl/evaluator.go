package dsl

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"novelgen/internal/rpg"
)

// ExpressionEvaluator evaluates DSL expressions and conditions
type ExpressionEvaluator struct {
	functions map[string]Function
}

// Function represents a built-in function
type Function func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator() *ExpressionEvaluator {
	ee := &ExpressionEvaluator{
		functions: make(map[string]Function),
	}
	
	// Register built-in functions
	ee.registerMathFunctions()
	ee.registerLogicFunctions()
	ee.registerQueryFunctions()
	ee.registerRandomFunctions()
	
	return ee
}

// registerMathFunctions registers mathematical functions
func (ee *ExpressionEvaluator) registerMathFunctions() {
	// random(min, max) - random integer
	ee.functions["random"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 2 {
			return 0
		}
		min := ee.toInt(args[0])
		max := ee.toInt(args[1])
		if min >= max {
			return min
		}
		return rand.Intn(max-min+1) + min
	}
	
	// random_float(min, max) - random float
	ee.functions["random_float"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 2 {
			return 0.0
		}
		min := ee.toFloat(args[0])
		max := ee.toFloat(args[1])
		return min + rand.Float64()*(max-min)
	}
	
	// min(a, b) - minimum
	ee.functions["min"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return 0
		}
		result := ee.toFloat(args[0])
		for i := 1; i < len(args); i++ {
			val := ee.toFloat(args[i])
			if val < result {
				result = val
			}
		}
		return result
	}
	
	// max(a, b) - maximum
	ee.functions["max"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return 0
		}
		result := ee.toFloat(args[0])
		for i := 1; i < len(args); i++ {
			val := ee.toFloat(args[i])
			if val > result {
				result = val
			}
		}
		return result
	}
	
	// clamp(val, min, max) - clamp value
	ee.functions["clamp"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 3 {
			return 0
		}
		val := ee.toFloat(args[0])
		min := ee.toFloat(args[1])
		max := ee.toFloat(args[2])
		if val < min {
			return min
		}
		if val > max {
			return max
		}
		return val
	}
	
	// round(val) - round to nearest integer
	ee.functions["round"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 {
			return 0
		}
		return int(math.Round(ee.toFloat(args[0])))
	}
	
	// floor(val) - floor
	ee.functions["floor"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 {
			return 0
		}
		return int(math.Floor(ee.toFloat(args[0])))
	}
	
	// ceil(val) - ceiling
	ee.functions["ceil"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 {
			return 0
		}
		return int(math.Ceil(ee.toFloat(args[0])))
	}
}

// registerLogicFunctions registers logical functions
func (ee *ExpressionEvaluator) registerLogicFunctions() {
	// and(...) - logical AND
	ee.functions["and"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		for _, arg := range args {
			if !ee.toBool(arg) {
				return false
			}
		}
		return true
	}
	
	// or(...) - logical OR
	ee.functions["or"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		for _, arg := range args {
			if ee.toBool(arg) {
				return true
			}
		}
		return false
	}
	
	// not(x) - logical NOT
	ee.functions["not"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 {
			return true
		}
		return !ee.toBool(args[0])
	}
}

// registerQueryFunctions registers query functions
func (ee *ExpressionEvaluator) registerQueryFunctions() {
	// has_flag(name) - check if flag is set
	ee.functions["has_flag"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 || world == nil {
			return false
		}
		// For MVP, simplified flag checking
		return false
	}
	
	// has_item(id) - check if character has item
	ee.functions["has_item"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 || character == nil {
			return false
		}
		// Simplified for MVP
		return false
	}
	
	// item_quantity(id) - get item quantity
	ee.functions["item_quantity"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 || character == nil {
			return 0
		}
		// Simplified for MVP
		return 0
	}
	
	// kill_count(enemy_id) - get kill count
	ee.functions["kill_count"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 || world == nil {
			return 0
		}
		// Simplified for MVP
		return 0
	}
	
	// skill_use_count(skill_id) - get skill use count
	ee.functions["skill_use_count"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 1 || character == nil {
			return 0
		}
		// Simplified for MVP
		return 0
	}
	
	// current_location() - get current location
	ee.functions["current_location"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if character == nil || world == nil {
			return ""
		}
		// Simplified for MVP
		return ""
	}
	
	// get_stat(target, stat) - get character stat
	ee.functions["get_stat"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) != 2 || character == nil {
			return 0
		}
		target := ee.toString(args[0])
		stat := ee.toString(args[1])
		
		if target != character.ID {
			return 0
		}
		
		switch stat {
		case "hp":
			return character.CurrentStats.HP
		case "max_hp":
			return character.BaseStats.HP
		case "level":
			return character.Level
		default:
			return 0
		}
	}
	
	// player_level() - get player level
	ee.functions["player_level"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if world != nil && world.Player != nil {
			return world.Player.Level
		}
		return 0
	}
}

// registerRandomFunctions registers random/choice functions
func (ee *ExpressionEvaluator) registerRandomFunctions() {
	// random_choice(list) - random selection from list
	ee.functions["random_choice"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return nil
		}
		
		// Handle single array argument
		var list []interface{}
		if arr, ok := args[0].([]interface{}); ok {
			list = arr
		} else {
			list = args
		}
		
		if len(list) == 0 {
			return nil
		}
		return list[rand.Intn(len(list))]
	}
}

// EvaluateCondition evaluates a single condition string
func (ee *ExpressionEvaluator) EvaluateCondition(condition string, character *rpg.Character, world *rpg.GameWorld) bool {
	// Simple condition parsing for MVP
	// Examples: "level >= 10", "hp < 50", "has_flag('awakened')"
	
	// Check for function calls
	if strings.Contains(condition, "(") {
		return ee.evaluateFunctionCall(condition, character, world)
	}
	
	// Parse comparison: stat op value
	re := regexp.MustCompile(`(\w+)\s*([<>=!]+)\s*(.+)`)
	matches := re.FindStringSubmatch(condition)
	if len(matches) != 4 {
		return false
	}
	
	left := matches[1]
	op := matches[2]
	right := matches[3]
	
	// Get left value
	var leftVal float64
	switch left {
	case "level":
		if character != nil {
			leftVal = float64(character.Level)
		}
	case "hp":
		if character != nil {
			leftVal = float64(character.CurrentStats.HP)
		}
	case "max_hp":
		if character != nil {
			leftVal = float64(character.BaseStats.HP)
		}
	default:
		leftVal = 0
	}
	
	// Parse right value
	rightVal := ee.parseValue(right, character, world)
	
	// Compare
	return ee.compare(leftVal, op, rightVal)
}

// EvaluateConditions evaluates multiple conditions
func (ee *ExpressionEvaluator) EvaluateConditions(conditions []Condition, character *rpg.Character, world *rpg.GameWorld) bool {
	if len(conditions) == 0 {
		return true
	}
	
	for _, cond := range conditions {
		var result bool
		
		switch cond.Type {
		case "stat":
			val := ee.getStatValue(cond.Stat, character)
			result = ee.compare(val, cond.Op, ee.toFloat(cond.Value))
		case "flag":
			// Simplified for MVP
			result = false
		case "counter":
			// Simplified for MVP
			result = false
		case "location":
			// Simplified for MVP
			result = false
		case "random":
			result = rand.Float64() < cond.Random
		default:
			result = false
		}
		
		if !result {
			return false // AND logic by default
		}
	}
	
	return true
}

// evaluateFunctionCall evaluates a function call in a condition
func (ee *ExpressionEvaluator) evaluateFunctionCall(expr string, character *rpg.Character, world *rpg.GameWorld) bool {
	// Extract function name and arguments
	re := regexp.MustCompile(`(\w+)\s*\(([^)]*)\)`)
	matches := re.FindStringSubmatch(expr)
	if len(matches) != 3 {
		return false
	}
	
	funcName := matches[1]
	argsStr := matches[2]
	
	// Parse arguments
	args := ee.parseArguments(argsStr)
	
	// Call function
	if fn, ok := ee.functions[funcName]; ok {
		result := fn(args, character, world)
		return ee.toBool(result)
	}
	
	return false
}

// parseArguments parses function arguments
func (ee *ExpressionEvaluator) parseArguments(argsStr string) []interface{} {
	args := make([]interface{}, 0)
	if strings.TrimSpace(argsStr) == "" {
		return args
	}
	
	// Split by comma (simplified - doesn't handle nested parens)
	parts := strings.Split(argsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Try to parse as number
		if val, err := strconv.ParseFloat(part, 64); err == nil {
			args = append(args, val)
		} else if part == "true" {
			args = append(args, true)
		} else if part == "false" {
			args = append(args, false)
		} else if strings.HasPrefix(part, "'") && strings.HasSuffix(part, "'") {
			// String literal
			args = append(args, part[1:len(part)-1])
		} else if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
			// String literal
			args = append(args, part[1:len(part)-1])
		} else {
			// Variable reference
			args = append(args, part)
		}
	}
	
	return args
}

// parseValue parses a value string
func (ee *ExpressionEvaluator) parseValue(val string, character *rpg.Character, world *rpg.GameWorld) float64 {
	val = strings.TrimSpace(val)
	
	// Try direct number
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	
	// Try percentage (e.g., "max_hp * 0.5")
	if strings.Contains(val, "*") {
		parts := strings.Split(val, "*")
		if len(parts) == 2 {
			base := ee.parseValue(strings.TrimSpace(parts[0]), character, world)
			multiplier := ee.parseValue(strings.TrimSpace(parts[1]), character, world)
			return base * multiplier
		}
	}
	
	// Try stat reference
	switch val {
	case "level":
		if character != nil {
			return float64(character.Level)
		}
	case "hp":
		if character != nil {
			return float64(character.CurrentStats.HP)
		}
	case "max_hp":
		if character != nil {
			return float64(character.BaseStats.HP)
		}
	}
	
	return 0
}

// getStatValue gets a stat value
func (ee *ExpressionEvaluator) getStatValue(stat string, character *rpg.Character) float64 {
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

// compare compares two values with an operator
func (ee *ExpressionEvaluator) compare(left float64, op string, right float64) bool {
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

// Helper conversion functions

func (ee *ExpressionEvaluator) toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

func (ee *ExpressionEvaluator) toFloat(v interface{}) float64 {
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

func (ee *ExpressionEvaluator) toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case float64:
		return val != 0
	case string:
		return val != "" && val != "false" && val != "0"
	default:
		return false
	}
}

func (ee *ExpressionEvaluator) toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// EvaluateExpression evaluates a complex expression with functions
func (ee *ExpressionEvaluator) EvaluateExpression(expr string, character *rpg.Character, world *rpg.GameWorld) interface{} {
	expr = strings.TrimSpace(expr)
	
	// Check if it's a function call
	if strings.Contains(expr, "(") {
		re := regexp.MustCompile(`(\w+)\s*\(([^)]*)\)`)
		matches := re.FindStringSubmatch(expr)
		if len(matches) == 3 {
			funcName := matches[1]
			argsStr := matches[2]
			args := ee.parseArguments(argsStr)
			
			if fn, ok := ee.functions[funcName]; ok {
				return fn(args, character, world)
			}
		}
	}
	
	// Try to parse as number
	if val, err := strconv.ParseFloat(expr, 64); err == nil {
		return val
	}
	
	// Try to parse as boolean
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}
	
	// Return as string
	return expr
}