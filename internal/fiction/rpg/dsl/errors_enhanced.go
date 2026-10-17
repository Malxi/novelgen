package dsl

import (
	"fmt"
	"strings"
)

// Position represents a location in the source code
type Position struct {
	Line   int
	Column int
	Offset int
}

// String returns a string representation of the position
func (p Position) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("line %d, column %d", p.Line, p.Column)
	}
	return "unknown position"
}

// DSLParseError represents a parsing error with position information
type DSLParseError struct {
	Pos     Position
	Message string
	Context string
}

// Error implements the error interface
func (e *DSLParseError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("parse error at %s: %s", e.Pos.String(), e.Message))
	if e.Context != "" {
		sb.WriteString(fmt.Sprintf("\n  context: %s", e.Context))
	}
	return sb.String()
}

// DSLValidationError represents a validation error with position information
type DSLValidationError struct {
	Pos     Position
	Field   string
	Message string
	Hint    string
}

// Error implements the error interface
func (e *DSLValidationError) Error() string {
	var sb strings.Builder
	if e.Pos.Line > 0 {
		sb.WriteString(fmt.Sprintf("validation error at %s", e.Pos.String()))
	} else {
		sb.WriteString("validation error")
	}
	if e.Field != "" {
		sb.WriteString(fmt.Sprintf(" in '%s'", e.Field))
	}
	sb.WriteString(fmt.Sprintf(": %s", e.Message))
	if e.Hint != "" {
		sb.WriteString(fmt.Sprintf("\n  hint: %s", e.Hint))
	}
	return sb.String()
}

// DSLExecutionError represents an execution/runtime error
type DSLExecutionError struct {
	Pos       Position
	Operation string
	Message   string
	Cause     error
}

// Error implements the error interface
func (e *DSLExecutionError) Error() string {
	var sb strings.Builder
	if e.Pos.Line > 0 {
		sb.WriteString(fmt.Sprintf("execution error at %s", e.Pos.String()))
	} else {
		sb.WriteString("execution error")
	}
	if e.Operation != "" {
		sb.WriteString(fmt.Sprintf(" during %s", e.Operation))
	}
	sb.WriteString(fmt.Sprintf(": %s", e.Message))
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf("\n  caused by: %v", e.Cause))
	}
	return sb.String()
}

// Unwrap returns the underlying cause
func (e *DSLExecutionError) Unwrap() error {
	return e.Cause
}

// ErrorCollection collects multiple errors
type ErrorCollection struct {
	Errors []error
}

// Error implements the error interface
func (ec *ErrorCollection) Error() string {
	if len(ec.Errors) == 0 {
		return "no errors"
	}
	if len(ec.Errors) == 1 {
		return ec.Errors[0].Error()
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d errors occurred:\n", len(ec.Errors)))
	for i, err := range ec.Errors {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// Add adds an error to the collection
func (ec *ErrorCollection) Add(err error) {
	if err != nil {
		ec.Errors = append(ec.Errors, err)
	}
}

// HasErrors returns true if there are errors
func (ec *ErrorCollection) HasErrors() bool {
	return len(ec.Errors) > 0
}

// ErrorReporter provides enhanced error reporting capabilities
type ErrorReporter struct {
	source      string
	lines       []string
	showContext bool
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter(source string) *ErrorReporter {
	return &ErrorReporter{
		source:      source,
		lines:       strings.Split(source, "\n"),
		showContext: true,
	}
}

// Report generates a detailed error report
func (er *ErrorReporter) Report(err error) string {
	var sb strings.Builder
	sb.WriteString(err.Error())
	sb.WriteString("\n")

	// Try to extract position information
	if pos := er.extractPosition(err); pos.Line > 0 && er.showContext {
		sb.WriteString(er.getContext(pos))
	}

	return sb.String()
}

// extractPosition tries to extract position from error
func (er *ErrorReporter) extractPosition(err error) Position {
	switch e := err.(type) {
	case *DSLParseError:
		return e.Pos
	case *DSLValidationError:
		return e.Pos
	case *DSLExecutionError:
		return e.Pos
	default:
		return Position{}
	}
}

// getContext returns the source context around a position
func (er *ErrorReporter) getContext(pos Position) string {
	if pos.Line <= 0 || pos.Line > len(er.lines) {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nSource context:\n")

	// Show 2 lines before and after
	startLine := pos.Line - 2
	if startLine < 1 {
		startLine = 1
	}
	endLine := pos.Line + 2
	if endLine > len(er.lines) {
		endLine = len(er.lines)
	}

	for i := startLine; i <= endLine; i++ {
		line := er.lines[i-1]
		marker := "  "
		if i == pos.Line {
			marker = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%3d | %s\n", marker, i, line))

		// Show column pointer for error line
		if i == pos.Line && pos.Column > 0 {
			pointer := strings.Repeat(" ", pos.Column+6) + "^"
			sb.WriteString(pointer + "\n")
		}
	}

	return sb.String()
}

// Parser helper methods for enhanced error reporting

// newParseError creates a parse error at current position
func (p *Parser) newParseError(message string) *DSLParseError {
	return &DSLParseError{
		Pos: Position{
			Line:   p.line,
			Column: p.col,
			Offset: p.pos,
		},
		Message: message,
		Context: p.getCurrentContext(),
	}
}

// newParseErrorf creates a formatted parse error at current position
func (p *Parser) newParseErrorf(format string, args ...interface{}) *DSLParseError {
	return p.newParseError(fmt.Sprintf(format, args...))
}

// getCurrentContext returns the current parsing context
func (p *Parser) getCurrentContext() string {
	// Get surrounding text
	start := p.pos - 20
	if start < 0 {
		start = 0
	}
	end := p.pos + 20
	if end > len(p.content) {
		end = len(p.content)
	}

	context := p.content[start:end]
	context = strings.ReplaceAll(context, "\n", "\\n")
	return fmt.Sprintf("...%s...", context)
}

// expectCharEnhanced expects a specific character with enhanced error reporting
func (p *Parser) expectCharEnhanced(c byte) error {
	p.skipWhitespace()
	if p.peek() != c {
		return p.newParseErrorf("expected '%c', got '%c'", c, p.peek())
	}
	p.advance()
	return nil
}

// Common error messages with hints
var CommonErrorHints = map[string]string{
	"expected identifier":    "Identifiers must start with a letter and can contain letters, numbers, and underscores",
	"expected string":        "Strings must be enclosed in double quotes",
	"expected number":        "Numbers can be integers or decimals",
	"unterminated string":    "Check for missing closing quote",
	"unterminated block":     "Check for missing closing brace '}'",
	"unknown block type":     "Valid block types are: metadata, world, characters, storyline, systems",
	"duplicate id":           "All IDs must be unique within their scope",
	"undefined reference":    "Make sure the referenced ID is defined before use",
	"invalid expression":     "Check expression syntax and variable names",
	"type mismatch":          "Ensure values match the expected type for this field",
	"missing required field": "This field is required and cannot be empty",
}
