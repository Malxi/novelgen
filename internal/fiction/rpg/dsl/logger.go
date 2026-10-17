package dsl

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogCategory represents the category of log entry
type LogCategory string

const (
	LogCategoryParse      LogCategory = "PARSE"
	LogCategoryValidate   LogCategory = "VALIDATE"
	LogCategoryConvert    LogCategory = "CONVERT"
	LogCategoryExecute    LogCategory = "EXECUTE"
	LogCategoryHook       LogCategory = "HOOK"
	LogCategoryTrigger    LogCategory = "TRIGGER"
	LogCategoryExpression LogCategory = "EXPR"
	LogCategorySystem     LogCategory = "SYSTEM"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Category  LogCategory            `json:"category"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
}

// Logger provides DSL execution logging capabilities
type Logger struct {
	writer      io.Writer
	minLevel    LogLevel
	categories  map[LogCategory]bool
	entries     []LogEntry
	maxEntries  int
	mu          sync.RWMutex
	includeTime bool
	jsonFormat  bool
}

// LoggerOption configures the logger
type LoggerOption func(*Logger)

// WithMinLevel sets the minimum log level
func WithMinLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.minLevel = level
	}
}

// WithCategories sets the categories to log
func WithCategories(categories ...LogCategory) LoggerOption {
	return func(l *Logger) {
		l.categories = make(map[LogCategory]bool)
		for _, cat := range categories {
			l.categories[cat] = true
		}
	}
}

// WithMaxEntries sets the maximum number of entries to keep in memory
func WithMaxEntries(max int) LoggerOption {
	return func(l *Logger) {
		l.maxEntries = max
	}
}

// WithJSONFormat enables JSON output format
func WithJSONFormat() LoggerOption {
	return func(l *Logger) {
		l.jsonFormat = true
	}
}

// NewLogger creates a new logger
func NewLogger(writer io.Writer, opts ...LoggerOption) *Logger {
	l := &Logger{
		writer:      writer,
		minLevel:    LogLevelInfo,
		categories:  nil, // nil means all categories
		entries:     make([]LogEntry, 0),
		maxEntries:  1000,
		includeTime: true,
		jsonFormat:  false,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// NewConsoleLogger creates a logger that writes to stdout
func NewConsoleLogger(opts ...LoggerOption) *Logger {
	return NewLogger(os.Stdout, opts...)
}

// log writes a log entry
func (l *Logger) log(level LogLevel, category LogCategory, message string, fields map[string]interface{}) {
	// Check level
	if level < l.minLevel {
		return
	}

	// Check category
	if l.categories != nil && !l.categories[category] {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
		Fields:    fields,
	}

	// Store in memory
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[1:]
	}
	l.mu.Unlock()

	// Write to output
	l.writeEntry(entry)
}

// writeEntry writes a single entry to the output
func (l *Logger) writeEntry(entry LogEntry) {
	var output string
	if l.jsonFormat {
		data, _ := json.Marshal(entry)
		output = string(data)
	} else {
		output = l.formatEntry(entry)
	}
	fmt.Fprintln(l.writer, output)
}

// formatEntry formats an entry as text
func (l *Logger) formatEntry(entry LogEntry) string {
	var sb strings.Builder

	// Timestamp
	if l.includeTime {
		sb.WriteString(entry.Timestamp.Format("15:04:05.000"))
		sb.WriteString(" ")
	}

	// Level
	sb.WriteString(fmt.Sprintf("[%-5s]", entry.Level.String()))

	// Category
	sb.WriteString(fmt.Sprintf("[%-9s]", entry.Category))

	// Message
	sb.WriteString(" ")
	sb.WriteString(entry.Message)

	// Fields
	if len(entry.Fields) > 0 {
		sb.WriteString(" ")
		first := true
		for k, v := range entry.Fields {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
	}

	// Duration
	if entry.Duration > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", entry.Duration))
	}

	return sb.String()
}

// Debug logs a debug message
func (l *Logger) Debug(category LogCategory, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelDebug, category, message, f)
}

// Info logs an info message
func (l *Logger) Info(category LogCategory, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelInfo, category, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(category LogCategory, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelWarn, category, message, f)
}

// Error logs an error message
func (l *Logger) Error(category LogCategory, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelError, category, message, f)
}

// GetEntries returns all stored entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// GetEntriesByCategory returns entries filtered by category
func (l *Logger) GetEntriesByCategory(category LogCategory) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []LogEntry
	for _, entry := range l.entries {
		if entry.Category == category {
			result = append(result, entry)
		}
	}
	return result
}

// DSLExecutionLogger provides high-level logging for DSL operations
type DSLExecutionLogger struct {
	logger     *Logger
	startTime  time.Time
	operations map[string]time.Time
}

// NewDSLExecutionLogger creates a new DSL execution logger
func NewDSLExecutionLogger(logger *Logger) *DSLExecutionLogger {
	return &DSLExecutionLogger{
		logger:     logger,
		startTime:  time.Now(),
		operations: make(map[string]time.Time),
	}
}

// LogParseStart logs the start of parsing
func (dl *DSLExecutionLogger) LogParseStart(source string) {
	dl.logger.Info(LogCategoryParse, "Starting DSL parse", map[string]interface{}{
		"source_length": len(source),
	})
	dl.operations["parse"] = time.Now()
}

// LogParseEnd logs the end of parsing
func (dl *DSLExecutionLogger) LogParseEnd(success bool, err error) {
	duration := time.Since(dl.operations["parse"])
	fields := map[string]interface{}{
		"success":  success,
		"duration": duration.Milliseconds(),
	}
	if err != nil {
		fields["error"] = err.Error()
		dl.logger.Error(LogCategoryParse, "DSL parse failed", fields)
	} else {
		dl.logger.Info(LogCategoryParse, "DSL parse completed", fields)
	}
}

// LogValidationStart logs the start of validation
func (dl *DSLExecutionLogger) LogValidationStart() {
	dl.logger.Info(LogCategoryValidate, "Starting DSL validation")
	dl.operations["validate"] = time.Now()
}

// LogValidationEnd logs the end of validation
func (dl *DSLExecutionLogger) LogValidationEnd(errors []ValidationError, warnings []ValidationWarning) {
	duration := time.Since(dl.operations["validate"])
	fields := map[string]interface{}{
		"error_count":   len(errors),
		"warning_count": len(warnings),
		"duration_ms":   duration.Milliseconds(),
	}
	if len(errors) > 0 {
		dl.logger.Error(LogCategoryValidate, "DSL validation failed", fields)
		for _, err := range errors {
			dl.logger.Error(LogCategoryValidate, "Validation error", map[string]interface{}{
				"field":   err.Field,
				"message": err.Message,
				"line":    err.Line,
			})
		}
	} else {
		dl.logger.Info(LogCategoryValidate, "DSL validation completed", fields)
	}
}

// LogConversionStart logs the start of conversion
func (dl *DSLExecutionLogger) LogConversionStart() {
	dl.logger.Info(LogCategoryConvert, "Starting DSL to World conversion")
	dl.operations["convert"] = time.Now()
}

// LogConversionEnd logs the end of conversion
func (dl *DSLExecutionLogger) LogConversionEnd(worldStats map[string]int, err error) {
	duration := time.Since(dl.operations["convert"])
	fields := map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}
	for k, v := range worldStats {
		fields[k] = v
	}
	if err != nil {
		fields["error"] = err.Error()
		dl.logger.Error(LogCategoryConvert, "DSL conversion failed", fields)
	} else {
		dl.logger.Info(LogCategoryConvert, "DSL conversion completed", fields)
	}
}

// LogHookExecution logs hook execution
func (dl *DSLExecutionLogger) LogHookExecution(hookType string, actor string, resultCount int) {
	dl.logger.Info(LogCategoryHook, "Hook executed", map[string]interface{}{
		"hook_type":    hookType,
		"actor":        actor,
		"result_count": resultCount,
	})
}

// LogTriggerEvaluation logs trigger evaluation
func (dl *DSLExecutionLogger) LogTriggerEvaluation(triggerID string, condition string, result bool) {
	level := LogLevelInfo
	if !result {
		level = LogLevelDebug
	}
	dl.logger.LogTrigger(level, LogCategoryTrigger, "Trigger evaluated", map[string]interface{}{
		"trigger_id": triggerID,
		"condition":  condition,
		"result":     result,
	})
}

// LogExpression logs expression evaluation
func (dl *DSLExecutionLogger) LogExpression(expression string, result interface{}, err error) {
	fields := map[string]interface{}{
		"expression": expression,
		"result":     result,
	}
	if err != nil {
		fields["error"] = err.Error()
		dl.logger.Error(LogCategoryExpression, "Expression evaluation failed", fields)
	} else {
		dl.logger.Debug(LogCategoryExpression, "Expression evaluated", fields)
	}
}

// LogSystem logs a system-level message
func (dl *DSLExecutionLogger) LogSystem(message string, fields map[string]interface{}) {
	dl.logger.Info(LogCategorySystem, message, fields)
}

// GetTotalDuration returns the total execution duration
func (dl *DSLExecutionLogger) GetTotalDuration() time.Duration {
	return time.Since(dl.startTime)
}

// GenerateReport generates a summary report
func (dl *DSLExecutionLogger) GenerateReport() map[string]interface{} {
	return map[string]interface{}{
		"total_duration_ms": dl.GetTotalDuration().Milliseconds(),
		"entries_count":     len(dl.logger.GetEntries()),
	}
}

// Helper method for trigger logging with dynamic level
func (l *Logger) LogTrigger(level LogLevel, category LogCategory, message string, fields map[string]interface{}) {
	l.log(level, category, message, fields)
}
