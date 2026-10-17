package agentruntime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// RunError classifies a runner failure at the process boundary. Callers should
// use IsTransient instead of matching runner stderr text, so a message change in
// the Python runner cannot silently disable a retry policy.
type RunError struct {
	Err       error
	Transient bool
}

func (e *RunError) Error() string {
	if e == nil || e.Err == nil {
		return "agent runtime failure"
	}
	return e.Err.Error()
}

func (e *RunError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsTransient reports whether err carries a transient runner classification.
func IsTransient(err error) bool {
	var runErr *RunError
	return errors.As(err, &runErr) && runErr.Transient
}

// NewRunError wraps a runner error with an explicit transient flag. Tests and
// alternative runtimes can use it to exercise retry policies without spawning a
// process.
func NewRunError(err error, transient bool) error {
	if err == nil {
		return nil
	}
	return &RunError{Err: err, Transient: transient}
}

func classifyRunnerFailure(ctx context.Context, err error, stderr string) bool {
	if err == nil {
		return false
	}
	if ctx != nil && errors.Is(ctx.Err(), context.Canceled) {
		return false
	}
	if ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	text := strings.ToLower(stderr)
	for _, marker := range []string{
		"error in hook callback",
		"stream closed",
		"connection reset",
		"broken pipe",
		"econnreset",
		"timeout",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
