package dsl

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/rpg"
)

// EnhancedExpressionEvaluator adds more built-in functions
type EnhancedExpressionEvaluator struct {
	*ExpressionEvaluator
}

// NewEnhancedExpressionEvaluator creates an enhanced evaluator
func NewEnhancedExpressionEvaluator() *EnhancedExpressionEvaluator {
	ee := &EnhancedExpressionEvaluator{
		ExpressionEvaluator: NewExpressionEvaluator(),
	}
	ee.registerEnhancedFunctions()
	return ee
}

// registerEnhancedFunctions registers additional functions
func (ee *EnhancedExpressionEvaluator) registerEnhancedFunctions() {
	ee.registerStringFunctions()
	ee.registerArrayFunctions()
	ee.registerTimeFunctions()
	ee.registerAdvancedMathFunctions()
	ee.registerUtilityFunctions()
}

// registerStringFunctions registers string manipulation functions
func (ee *EnhancedExpressionEvaluator) registerStringFunctions() {
	// concat(str1, str2, ...) - concatenate strings
	ee.functions["concat"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		var result strings.Builder
		for _, arg := range args {
			result.WriteString(ee.toString(arg))
		}
		return result.String()
	}

	// format(template, ...) - format string
	ee.functions["format"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return ""
		}
		template := ee.toString(args[0])
		formattedArgs := make([]interface{}, len(args)-1)
		for i := 1; i < len(args); i++ {
			formattedArgs[i-1] = args[i]
		}
		return fmt.Sprintf(template, formattedArgs...)
	}

	// upper(str) - uppercase
	ee.functions["upper"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return ""
		}
		return strings.ToUpper(ee.toString(args[0]))
	}

	// lower(str) - lowercase
	ee.functions["lower"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return ""
		}
		return strings.ToLower(ee.toString(args[0]))
	}

	// contains(str, substr) - check if string contains substring
	ee.functions["contains"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return false
		}
		return strings.Contains(ee.toString(args[0]), ee.toString(args[1]))
	}

	// starts_with(str, prefix) - check prefix
	ee.functions["starts_with"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return false
		}
		return strings.HasPrefix(ee.toString(args[0]), ee.toString(args[1]))
	}

	// ends_with(str, suffix) - check suffix
	ee.functions["ends_with"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return false
		}
		return strings.HasSuffix(ee.toString(args[0]), ee.toString(args[1]))
	}

	// replace(str, old, new) - replace substring
	ee.functions["replace"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 3 {
			return ee.toString(args[0])
		}
		return strings.ReplaceAll(ee.toString(args[0]), ee.toString(args[1]), ee.toString(args[2]))
	}

	// split(str, delimiter) - split string
	ee.functions["split"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return []interface{}{}
		}
		parts := strings.Split(ee.toString(args[0]), ee.toString(args[1]))
		result := make([]interface{}, len(parts))
		for i, part := range parts {
			result[i] = part
		}
		return result
	}

	// trim(str) - trim whitespace
	ee.functions["trim"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return ""
		}
		return strings.TrimSpace(ee.toString(args[0]))
	}

	// len(str) - string length
	ee.functions["len"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		switch v := args[0].(type) {
		case string:
			return len(v)
		case []interface{}:
			return len(v)
		default:
			return 0
		}
	}

	// matches(str, pattern) - regex match
	ee.functions["matches"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return false
		}
		str := ee.toString(args[0])
		pattern := ee.toString(args[1])
		matched, _ := regexp.MatchString(pattern, str)
		return matched
	}
}

// registerArrayFunctions registers array/list manipulation functions
func (ee *EnhancedExpressionEvaluator) registerArrayFunctions() {
	// array(item1, item2, ...) - create array
	ee.functions["array"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		return args
	}

	// first(arr) - get first element
	ee.functions["first"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return nil
		}
		if arr, ok := args[0].([]interface{}); ok && len(arr) > 0 {
			return arr[0]
		}
		return nil
	}

	// last(arr) - get last element
	ee.functions["last"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return nil
		}
		if arr, ok := args[0].([]interface{}); ok && len(arr) > 0 {
			return arr[len(arr)-1]
		}
		return nil
	}

	// nth(arr, n) - get nth element (0-indexed)
	ee.functions["nth"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return nil
		}
		if arr, ok := args[0].([]interface{}); ok {
			n := ee.toInt(args[1])
			if n >= 0 && n < len(arr) {
				return arr[n]
			}
		}
		return nil
	}

	// shuffle(arr) - shuffle array
	ee.functions["shuffle"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return []interface{}{}
		}
		if arr, ok := args[0].([]interface{}); ok {
			rand.Shuffle(len(arr), func(i, j int) {
				arr[i], arr[j] = arr[j], arr[i]
			})
			return arr
		}
		return args[0]
	}

	// sort(arr) - sort array
	ee.functions["sort"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return []interface{}{}
		}
		if arr, ok := args[0].([]interface{}); ok {
			sort.Slice(arr, func(i, j int) bool {
				return ee.toFloat(arr[i]) < ee.toFloat(arr[j])
			})
			return arr
		}
		return args[0]
	}

	// reverse(arr) - reverse array
	ee.functions["reverse"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return []interface{}{}
		}
		if arr, ok := args[0].([]interface{}); ok {
			for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
				arr[i], arr[j] = arr[j], arr[i]
			}
			return arr
		}
		return args[0]
	}

	// unique(arr) - remove duplicates
	ee.functions["unique"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return []interface{}{}
		}
		if arr, ok := args[0].([]interface{}); ok {
			seen := make(map[string]bool)
			result := make([]interface{}, 0)
			for _, item := range arr {
				key := ee.toString(item)
				if !seen[key] {
					seen[key] = true
					result = append(result, item)
				}
			}
			return result
		}
		return args[0]
	}

	// join(arr, delimiter) - join array into string
	ee.functions["join"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return ""
		}
		if arr, ok := args[0].([]interface{}); ok {
			delimiter := ee.toString(args[1])
			parts := make([]string, len(arr))
			for i, item := range arr {
				parts[i] = ee.toString(item)
			}
			return strings.Join(parts, delimiter)
		}
		return ""
	}

	// contains_item(arr, item) - check if array contains item
	ee.functions["contains_item"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return false
		}
		if arr, ok := args[0].([]interface{}); ok {
			target := ee.toString(args[1])
			for _, item := range arr {
				if ee.toString(item) == target {
					return true
				}
			}
		}
		return false
	}
}

// registerTimeFunctions registers time-related functions
func (ee *EnhancedExpressionEvaluator) registerTimeFunctions() {
	// now() - current timestamp
	ee.functions["now"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		return time.Now().Unix()
	}

	// day_of_week() - current day (0=Sunday, 6=Saturday)
	ee.functions["day_of_week"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		return int(time.Now().Weekday())
	}

	// hour() - current hour
	ee.functions["hour"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		return time.Now().Hour()
	}
}

// registerAdvancedMathFunctions registers advanced mathematical functions
func (ee *EnhancedExpressionEvaluator) registerAdvancedMathFunctions() {
	// abs(x) - absolute value
	ee.functions["abs"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Abs(ee.toFloat(args[0]))
	}

	// pow(x, y) - power
	ee.functions["pow"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 2 {
			return 0
		}
		return math.Pow(ee.toFloat(args[0]), ee.toFloat(args[1]))
	}

	// sqrt(x) - square root
	ee.functions["sqrt"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Sqrt(ee.toFloat(args[0]))
	}

	// log(x) - natural logarithm
	ee.functions["log"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Log(ee.toFloat(args[0]))
	}

	// log10(x) - base-10 logarithm
	ee.functions["log10"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Log10(ee.toFloat(args[0]))
	}

	// sin(x), cos(x), tan(x) - trigonometric functions
	ee.functions["sin"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Sin(ee.toFloat(args[0]))
	}

	ee.functions["cos"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Cos(ee.toFloat(args[0]))
	}

	ee.functions["tan"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return math.Tan(ee.toFloat(args[0]))
	}
}

// registerUtilityFunctions registers utility functions
func (ee *EnhancedExpressionEvaluator) registerUtilityFunctions() {
	// if(condition, true_value, false_value) - ternary operator
	ee.functions["if"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 3 {
			return nil
		}
		if ee.toBool(args[0]) {
			return args[1]
		}
		return args[2]
	}

	// coalesce(val1, val2, ...) - return first non-nil/non-zero value
	ee.functions["coalesce"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		for _, arg := range args {
			if arg != nil {
				switch v := arg.(type) {
				case string:
					if v != "" {
						return arg
					}
				case int:
					if v != 0 {
						return arg
					}
				case float64:
					if v != 0 {
						return arg
					}
				case bool:
					if v {
						return arg
					}
				default:
					return arg
				}
			}
		}
		return nil
	}

	// type_of(val) - return type of value
	ee.functions["type_of"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return "nil"
		}
		switch args[0].(type) {
		case string:
			return "string"
		case int, float64:
			return "number"
		case bool:
			return "boolean"
		case []interface{}:
			return "array"
		default:
			return "unknown"
		}
	}

	// to_string(val) - convert to string
	ee.functions["to_string"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return ""
		}
		return ee.toString(args[0])
	}

	// to_number(val) - convert to number
	ee.functions["to_number"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return ee.toFloat(args[0])
	}

	// to_int(val) - convert to integer
	ee.functions["to_int"] = func(args []interface{}, character *rpg.Character, world *rpg.GameWorld) interface{} {
		if len(args) < 1 {
			return 0
		}
		return ee.toInt(args[0])
	}
}

// Helper methods for EnhancedExpressionEvaluator

func (ee *EnhancedExpressionEvaluator) toInt(v interface{}) int {
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

func (ee *EnhancedExpressionEvaluator) toFloat(v interface{}) float64 {
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

func (ee *EnhancedExpressionEvaluator) toString(v interface{}) string {
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

func (ee *EnhancedExpressionEvaluator) toBool(v interface{}) bool {
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

// GetAllFunctions returns list of all available functions
func (ee *EnhancedExpressionEvaluator) GetAllFunctions() []string {
	funcs := make([]string, 0, len(ee.functions))
	for name := range ee.functions {
		funcs = append(funcs, name)
	}
	sort.Strings(funcs)
	return funcs
}

// Evaluate evaluates an expression and returns the result
// This is a convenience wrapper around EvaluateExpression
func (ee *EnhancedExpressionEvaluator) Evaluate(expression string, context map[string]interface{}) (interface{}, error) {
	// For testing purposes, we'll use EvaluateExpression with nil character/world
	result := ee.EvaluateExpression(expression, nil, nil)
	return result, nil
}