package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging
type Logger struct {
	mu         sync.RWMutex
	level      Level
	output     io.Writer
	file       *os.File
	projectDir string
}

// LoggerInterface defines the logger interface
type LoggerInterface interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

var defaultLogger *Logger

func init() {
	defaultLogger = New(InfoLevel)
}

// New creates a new logger
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		output: os.Stdout,
	}
}

// SetProjectDir sets the project directory for file logging
func (l *Logger) SetProjectDir(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.projectDir = dir
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// EnableFileLogging enables logging to file
func (l *Logger) EnableFileLogging() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.projectDir == "" {
		return fmt.Errorf("project directory not set")
	}

	logDir := filepath.Join(l.projectDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, fmt.Sprintf("nolvegen_%s.log", time.Now().Format("20060102_150405")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	if l.file != nil {
		l.file.Close()
	}

	l.file = file
	l.output = io.MultiWriter(os.Stdout, file)
	return nil
}

// Close closes the logger
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// log writes a log entry
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), msg)

	fmt.Fprint(l.output, logLine)
}

// Debug logs debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Info logs info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Warn logs warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Error logs error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// Section logs a section header
func (l *Logger) Section(name string) {
	l.Info("========== %s ==========", name)
}

// Prompt logs prompt information
func (l *Logger) Prompt(skill, name string, systemPrompt, userPrompt string) {
	l.Section("PROMPT")
	l.Info("Skill: %s", skill)
	l.Info("Template: %s", name)
	l.Debug("System Prompt:\n%s", systemPrompt)
	l.Debug("User Prompt:\n%s", userPrompt)
}

// LLMRequest logs LLM request
func (l *Logger) LLMRequest(model string, messages int, maxTokens int) {
	l.Section("LLM REQUEST")
	l.Info("Model: %s", model)
	l.Info("Messages: %d", messages)
	l.Info("Max Tokens: %d", maxTokens)
}

// LLMResponse logs LLM response
func (l *Logger) LLMResponse(model string, tokensUsed int, content string) {
	l.Section("LLM RESPONSE")
	l.Info("Model: %s", model)
	l.Info("Tokens Used: %d", tokensUsed)
	l.Debug("Content:\n%s", content)
}

// Error logs error with details
func (l *Logger) ErrorWithDetails(err error, details string) {
	l.Section("ERROR")
	l.Error("%s: %v", details, err)
}

// Default returns the default logger
func Default() *Logger {
	return defaultLogger
}

// GetLogger returns the default logger as interface (alias for Default)
func GetLogger() LoggerInterface {
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Package-level functions
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func Section(name string) {
	defaultLogger.Section(name)
}

func Prompt(skill, name string, systemPrompt, userPrompt string) {
	defaultLogger.Prompt(skill, name, systemPrompt, userPrompt)
}

func LLMRequest(model string, messages int, maxTokens int) {
	defaultLogger.LLMRequest(model, messages, maxTokens)
}

func LLMResponse(model string, tokensUsed int, content string) {
	defaultLogger.LLMResponse(model, tokensUsed, content)
}
