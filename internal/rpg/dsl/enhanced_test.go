package dsl

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"novelgen/internal/rpg"
)

// Helper function to create a test character
func createTestCharacter(id, name string, charType rpg.CharacterType) *rpg.Character {
	template := &rpg.CharacterTemplate{
		ID:   id,
		Name: name,
		Type: charType,
		BaseStats: rpg.BaseStats{
			HP:     100,
			MP:     50,
			Attack: 10,
		},
	}
	return rpg.NewCharacter(template, name)
}

// ==================== Enhanced Hook Tests ====================

func TestEnhancedHookManager_OnKillEnhanced(t *testing.T) {
	world := rpg.NewGameWorld()
	killer := createTestCharacter("player", "Player", rpg.CharacterTypePlayer)
	victim := createTestCharacter("goblin_1", "Goblin", rpg.CharacterTypeEnemy)
	killer.Level = 10
	victim.Level = 5

	// Set up world
	world.SetPlayer(killer)
	world.Characters.AddCharacterInstance(victim)

	// Create hook manager
	hm := NewEnhancedHookManager()

	// Register a hook for goblin kills
	hm.RegisterHook("on_kill", "goblin_slayer", HookConfig{
		Filter: "enemy_id == 'goblin_1'",
		Action: "log_kill",
	})

	// Test kill
	results := hm.OnKillEnhanced(killer, victim, world)

	if len(results) == 0 {
		t.Error("Expected hook results, got none")
	}

	// Check counter
	count := hm.GetKillCount("goblin_1")
	if count != 1 {
		t.Errorf("Expected kill count 1, got %d", count)
	}
}

func TestEnhancedHookManager_FilterEvaluation(t *testing.T) {
	world := rpg.NewGameWorld()
	killer := createTestCharacter("player", "Player", rpg.CharacterTypePlayer)
	victim := createTestCharacter("dragon_1", "Dragon", rpg.CharacterTypeBoss)
	killer.Level = 50
	victim.Level = 50

	world.SetPlayer(killer)
	world.Characters.AddCharacterInstance(victim)

	hm := NewEnhancedHookManager()

	tests := []struct {
		name     string
		filter   string
		expected bool
	}{
		{"exact match", "enemy_id == 'dragon_1'", true},
		{"prefix match", "enemy_id starts_with 'dragon'", true},
		{"level check", "level >= 50", true},
		{"level check false", "level > 100", false},
		{"name contains", "enemy_name contains 'rag'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm.RegisterHook("on_kill", "test_hook", HookConfig{
				Filter: tt.filter,
				Action: "test",
			})

			result := hm.evaluateKillFilter(tt.filter, victim, killer)
			if result != tt.expected {
				t.Errorf("Filter '%s': expected %v, got %v", tt.filter, tt.expected, result)
			}
		})
	}
}

func TestEnhancedHookManager_CounterMilestones(t *testing.T) {
	hm := NewEnhancedHookManager()

	// Initialize counter with milestones
	hm.InitializeCounter("test_counter", CounterConfig{
		Track:      "kills",
		Milestones: []int{5, 10, 25, 50},
	})

	// Increment to milestone
	for i := 0; i < 5; i++ {
		hm.IncrementCounter("test_counter")
	}

	// Check milestone reached
	if !hm.IsMilestoneReached("test_counter", 5) {
		t.Error("Expected milestone 5 to be reached")
	}

	// Check next milestone not reached
	if hm.IsMilestoneReached("test_counter", 10) {
		t.Error("Expected milestone 10 to NOT be reached")
	}
}

func TestEnhancedHookManager_SaveLoadState(t *testing.T) {
	hm := NewEnhancedHookManager()

	// Set up some state
	hm.IncrementCounter("kills")
	hm.IncrementCounter("kills")
	hm.IncrementCounter("skills")
	hm.SetTriggered("test_trigger")

	// Save state
	state := hm.SaveState()

	// Create new manager and load state
	hm2 := NewEnhancedHookManager()
	hm2.LoadState(state)

	// Verify state
	if hm2.GetKillCount("kills") != 2 {
		t.Errorf("Expected kill count 2, got %d", hm2.GetKillCount("kills"))
	}

	if hm2.GetSkillCount("skills") != 1 {
		t.Errorf("Expected skill count 1, got %d", hm2.GetSkillCount("skills"))
	}

	if !hm2.IsTriggered("test_trigger") {
		t.Error("Expected trigger to be set")
	}
}

// ==================== Enhanced Function Tests ====================

func TestEnhancedExpressionEvaluator_StringFunctions(t *testing.T) {
	eval := NewEnhancedExpressionEvaluator()

	tests := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{"concat", `concat("Hello", " ", "World")`, "Hello World"},
		{"upper", `upper("hello")`, "HELLO"},
		{"lower", `lower("HELLO")`, "hello"},
		{"contains true", `contains("hello world", "world")`, true},
		{"contains false", `contains("hello world", "foo")`, false},
		{"starts_with true", `starts_with("hello world", "hello")`, true},
		{"ends_with true", `ends_with("hello world", "world")`, true},
		{"trim", `trim("  hello  ")`, "hello"},
		{"len", `len("hello")`, float64(5)},
		{"replace", `replace("hello world", "world", "go")`, "hello go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(tt.expression, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEnhancedExpressionEvaluator_ArrayFunctions(t *testing.T) {
	eval := NewEnhancedExpressionEvaluator()

	tests := []struct {
		name       string
		expression string
		checkFunc  func(result interface{}) bool
	}{
		{"first", `first(array(1, 2, 3))`, func(r interface{}) bool { return r == float64(1) }},
		{"last", `last(array(1, 2, 3))`, func(r interface{}) bool { return r == float64(3) }},
		{"nth", `nth(array(1, 2, 3), 1)`, func(r interface{}) bool { return r == float64(2) }},
		{"join", `join(array("a", "b", "c"), ",")`, func(r interface{}) bool { return r == "a,b,c" }},
		{"contains_item true", `contains_item(array(1, 2, 3), 2)`, func(r interface{}) bool { return r == true }},
		{"contains_item false", `contains_item(array(1, 2, 3), 5)`, func(r interface{}) bool { return r == false }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(tt.expression, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tt.checkFunc(result) {
				t.Errorf("Unexpected result: %v", result)
			}
		})
	}
}

func TestEnhancedExpressionEvaluator_MathFunctions(t *testing.T) {
	eval := NewEnhancedExpressionEvaluator()

	tests := []struct {
		name       string
		expression string
		expected   float64
		delta      float64
	}{
		{"abs positive", `abs(-5)`, 5, 0.001},
		{"abs negative", `abs(5)`, 5, 0.001},
		{"pow", `pow(2, 3)`, 8, 0.001},
		{"sqrt", `sqrt(16)`, 4, 0.001},
		{"sqrt fraction", `sqrt(2)`, 1.414, 0.001},
		{"log", `log(100)`, 4.605, 0.001},
		{"sin", `sin(0)`, 0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(tt.expression, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			resultFloat, ok := result.(float64)
			if !ok {
				t.Fatalf("Expected float64, got %T", result)
			}

			if diff := resultFloat - tt.expected; diff < -tt.delta || diff > tt.delta {
				t.Errorf("Expected %v, got %v (delta %v)", tt.expected, resultFloat, tt.delta)
			}
		})
	}
}

func TestEnhancedExpressionEvaluator_UtilityFunctions(t *testing.T) {
	eval := NewEnhancedExpressionEvaluator()

	tests := []struct {
		name       string
		expression string
		expected   interface{}
	}{
		{"if true", `if(true, "yes", "no")`, "yes"},
		{"if false", `if(false, "yes", "no")`, "no"},
		{"coalesce first", `coalesce("first", "second")`, "first"},
		{"type_of string", `type_of("hello")`, "string"},
		{"type_of number", `type_of(42)`, "number"},
		{"to_string", `to_string(42)`, "42"},
		{"to_number", `to_number("42")`, float64(42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.Evaluate(tt.expression, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ==================== Enhanced Error Tests ====================

func TestDSLParseError_Error(t *testing.T) {
	err := &DSLParseError{
		Pos: Position{
			Line:   10,
			Column: 5,
		},
		Message: "expected identifier",
		Context: "...metadata { title...",
	}

	expected := "parse error at line 10, column 5: expected identifier\n  context: ...metadata { title..."
	if err.Error() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, err.Error())
	}
}

func TestDSLValidationError_Error(t *testing.T) {
	err := &DSLValidationError{
		Pos: Position{
			Line:   15,
			Column: 10,
		},
		Field:   "metadata.title",
		Message: "title is required",
		Hint:    "Add a title field to the metadata block",
	}

	expected := "validation error at line 15, column 10 in 'metadata.title': title is required\n  hint: Add a title field to the metadata block"
	if err.Error() != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, err.Error())
	}
}

func TestEnhancedParser_ParseErrorReporting(t *testing.T) {
	source := `metadata {
    title = "Test"
    invalid_syntax_here!!!
}`

	parser := NewEnhancedParser(source)
	_, err := parser.ParseEnhanced()

	if err == nil {
		t.Fatal("Expected parse error")
	}

	// Check that error contains position information
	errStr := err.Error()
	if !strings.Contains(errStr, "line") {
		t.Error("Expected error to contain line information")
	}
}

func TestErrorReporter_Report(t *testing.T) {
	source := `metadata {
    title = "Test"
}`

	reporter := NewErrorReporter(source)
	err := &DSLParseError{
		Pos:     Position{Line: 2, Column: 5},
		Message: "test error",
	}

	report := reporter.Report(err)

	if !strings.Contains(report, "Source context:") {
		t.Error("Expected report to contain source context")
	}

	if !strings.Contains(report, "2 |") {
		t.Error("Expected report to contain line 2")
	}
}

// ==================== Logger Tests ====================

func TestLogger_BasicLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithMinLevel(LogLevelDebug))

	logger.Debug(LogCategoryParse, "debug message", map[string]interface{}{"key": "value"})
	logger.Info(LogCategoryValidate, "info message")
	logger.Warn(LogCategoryExecute, "warn message")
	logger.Error(LogCategorySystem, "error message")

	output := buf.String()

	if !strings.Contains(output, "[DEBUG]") {
		t.Error("Expected DEBUG level in output")
	}
	if !strings.Contains(output, "[PARSE]") {
		t.Error("Expected PARSE category in output")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("Expected debug message in output")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithMinLevel(LogLevelWarn))

	logger.Debug(LogCategoryParse, "debug")
	logger.Info(LogCategoryParse, "info")
	logger.Warn(LogCategoryParse, "warn")
	logger.Error(LogCategoryParse, "error")

	output := buf.String()

	if strings.Contains(output, "debug") {
		t.Error("Debug message should be filtered")
	}
	if strings.Contains(output, "info") {
		t.Error("Info message should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("Warn message should be present")
	}
}

func TestLogger_CategoryFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithCategories(LogCategoryParse, LogCategoryValidate))

	logger.Info(LogCategoryParse, "parse message")
	logger.Info(LogCategoryValidate, "validate message")
	logger.Info(LogCategoryExecute, "execute message")

	output := buf.String()

	if !strings.Contains(output, "parse message") {
		t.Error("Parse message should be present")
	}
	if !strings.Contains(output, "validate message") {
		t.Error("Validate message should be present")
	}
	if strings.Contains(output, "execute message") {
		t.Error("Execute message should be filtered")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithJSONFormat())

	logger.Info(LogCategoryParse, "test message", map[string]interface{}{"key": "value"})

	output := buf.String()

	if !strings.Contains(output, `"timestamp"`) {
		t.Error("Expected JSON timestamp field")
	}
	if !strings.Contains(output, `"level"`) {
		t.Error("Expected JSON level field")
	}
	if !strings.Contains(output, `"message"`) {
		t.Error("Expected JSON message field")
	}
}

func TestLogger_GetEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithMaxEntries(10))

	logger.Info(LogCategoryParse, "message 1")
	logger.Info(LogCategoryParse, "message 2")
	logger.Info(LogCategoryValidate, "message 3")

	entries := logger.GetEntries()

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Test category filter
	parseEntries := logger.GetEntriesByCategory(LogCategoryParse)
	if len(parseEntries) != 2 {
		t.Errorf("Expected 2 parse entries, got %d", len(parseEntries))
	}
}

func TestLogger_MaxEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithMaxEntries(3))

	logger.Info(LogCategoryParse, "message 1")
	logger.Info(LogCategoryParse, "message 2")
	logger.Info(LogCategoryParse, "message 3")
	logger.Info(LogCategoryParse, "message 4")

	entries := logger.GetEntries()

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries (max), got %d", len(entries))
	}

	// First entry should be message 2 (oldest kept)
	if entries[0].Message != "message 2" {
		t.Errorf("Expected oldest entry to be 'message 2', got '%s'", entries[0].Message)
	}
}

func TestDSLExecutionLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)
	execLogger := NewDSLExecutionLogger(logger)

	// Test parse logging
	execLogger.LogParseStart(`metadata { title = "Test" }`)
	time.Sleep(10 * time.Millisecond)
	execLogger.LogParseEnd(true, nil)

	// Test validation logging
	execLogger.LogValidationStart()
	execLogger.LogValidationEnd(nil, nil)

	// Test conversion logging
	execLogger.LogConversionStart()
	execLogger.LogConversionEnd(map[string]int{"locations": 5}, nil)

	// Test hook logging
	execLogger.LogHookExecution("on_kill", "player", 2)

	// Test trigger logging
	execLogger.LogTriggerEvaluation("trigger_1", "level > 10", true)

	// Test expression logging
	execLogger.LogExpression("1 + 1", 2, nil)

	// Test system logging
	execLogger.LogSystem("System initialized", map[string]interface{}{"version": "1.0"})

	// Generate report
	report := execLogger.GenerateReport()
	if report["total_duration_ms"] == nil {
		t.Error("Expected total_duration_ms in report")
	}

	// Verify log output
	output := buf.String()
	if !strings.Contains(output, "Starting DSL parse") {
		t.Error("Expected parse start message")
	}
	if !strings.Contains(output, "DSL parse completed") {
		t.Error("Expected parse end message")
	}
}

func TestDSLExecutionLogger_ParseFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)
	execLogger := NewDSLExecutionLogger(logger)

	execLogger.LogParseStart(`invalid dsl`)
	execLogger.LogParseEnd(false, &DSLParseError{
		Pos:     Position{Line: 1, Column: 1},
		Message: "unexpected token",
	})

	output := buf.String()
	if !strings.Contains(output, "DSL parse failed") {
		t.Error("Expected parse failure message")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("Expected ERROR level for failure")
	}
}

func TestDSLExecutionLogger_ValidationErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)
	execLogger := NewDSLExecutionLogger(logger)

	execLogger.LogValidationStart()
	execLogger.LogValidationEnd([]ValidationError{
		{Field: "metadata.title", Message: "required", Line: 2},
		{Field: "world.locations", Message: "empty", Line: 10},
	}, nil)

	output := buf.String()
	if !strings.Contains(output, "DSL validation failed") {
		t.Error("Expected validation failure message")
	}
	if !strings.Contains(output, "error_count=2") {
		t.Error("Expected error count in output")
	}
}

// ==================== Integration Tests ====================

func TestFullWorkflow_WithLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, WithMinLevel(LogLevelDebug))
	execLogger := NewDSLExecutionLogger(logger)

	// Parse DSL
	source := `metadata {
    title = "Test Adventure"
    dsl_version = "0.1.0"
}

world {
    location "Starting Village" {
        id = "village"
        type = "city"
    }
}

characters {
    player "Hero" {
        id = "player"
        class = "warrior"
        hp = 100
    }
}

storyline {
    chapter "Chapter 1" {
        id = "ch1"
        objective "Start Quest" {
            step 1 {
                description = "Talk to elder"
                event {
                    type = "dialogue"
                }
            }
        }
    }
}`

	execLogger.LogParseStart(source)
	parser := NewEnhancedParser(source)
	dsl, err := parser.ParseEnhanced()
	execLogger.LogParseEnd(err == nil, err)

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Validate
	execLogger.LogValidationStart()
	validator := NewValidator()
	valErr := validator.Validate(dsl)
	execLogger.LogValidationEnd(validator.GetErrors(), validator.GetWarnings())

	if valErr != nil {
		t.Fatalf("Validation failed: %v", valErr)
	}

	// Log world stats
	worldStats := map[string]interface{}{
		"locations": len(dsl.World.Locations),
		"items":     len(dsl.World.Items),
		"enemies":   len(dsl.Characters.Enemies),
		"npcs":      len(dsl.Characters.NPCs),
		"chapters":  len(dsl.Storyline.Chapters),
	}
	execLogger.LogSystem("DSL parsed successfully", worldStats)

	// Verify log output
	output := buf.String()
	if !strings.Contains(output, "Starting DSL parse") {
		t.Error("Expected parse start in logs")
	}
	if !strings.Contains(output, "DSL parse completed") {
		t.Error("Expected parse completion in logs")
	}
	if !strings.Contains(output, "Starting DSL validation") {
		t.Error("Expected validation start in logs")
	}
	if !strings.Contains(output, "locations=1") {
		t.Error("Expected locations count in logs")
	}
}

func TestEnhancedParser_ErrorWithContext(t *testing.T) {
	source := `metadata {
    title = "Test"
    # This is a comment
    invalid!!! = "syntax"
}`

	parser := NewEnhancedParser(source)
	_, err := parser.ParseEnhanced()

	if err == nil {
		t.Fatal("Expected parse error")
	}

	// Format the error
	formatted := parser.FormatError(err)

	// Should contain source context
	if !strings.Contains(formatted, "Source context:") {
		t.Error("Expected formatted error to contain source context")
	}

	// Should show the error line
	if !strings.Contains(formatted, "invalid!!!") {
		t.Error("Expected formatted error to show the problematic line")
	}
}
