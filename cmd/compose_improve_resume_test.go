package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestRetryAgentSDKImproveWithResume(t *testing.T) {
	calls := 0
	err := retryAgentSDKImproveWithResume(3, func() error {
		calls++
		if calls < 3 {
			return errors.New("agent runner failed: exit status 1")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry() returned error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3 (resumed twice)", calls)
	}
}

func TestRetryAgentSDKImproveWithResumeGivesUpAfterAttempts(t *testing.T) {
	calls := 0
	err := retryAgentSDKImproveWithResume(2, func() error {
		calls++
		return errors.New("agent runner failed: exit status 1")
	})
	if err == nil {
		t.Fatalf("retry() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRetryAgentSDKImproveWithResumeSingleAttempt(t *testing.T) {
	calls := 0
	err := retryAgentSDKImproveWithResume(1, func() error {
		calls++
		return errors.New("boom")
	})
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d, want err and 1 call", err, calls)
	}
}

func TestIsTransientAgentSDKError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "hook callback stream closed",
			err:  errors.New("agent runner failed: exit status 1: Error in hook callback hook_0: 2485 | `)}"),
			want: true,
		},
		{
			name: "lowercase stream closed",
			err:  errors.New("exit status 1: error: Stream closed at sendRequest"),
			want: true,
		},
		{
			name: "wrapped transient error",
			err:  fmt.Errorf("iteration failed: %w", errors.New("agent runner failed: exit status 1: Error in hook callback hook_2")),
			want: true,
		},
		{
			name: "connection reset",
			err:  errors.New("agent runner failed: exit status 1: Connection reset by peer"),
			want: true,
		},
		{
			name: "deterministic parse failure",
			err:  errors.New("agent runner failed: exit status 1: failed to parse structured output"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientAgentSDKError(tt.err); got != tt.want {
				t.Fatalf("isTransientAgentSDKError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRepairAgentSDKWithTransientRetryRetriesThenSucceeds(t *testing.T) {
	calls := 0
	outline, err := repairAgentSDKWithTransientRetry(func() (*models.Outline, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("agent runner failed: exit status 1: Error in hook callback hook_0")
		}
		return &models.Outline{}, nil
	})
	if err != nil {
		t.Fatalf("repair() returned error: %v", err)
	}
	if outline == nil {
		t.Fatalf("repair() returned nil outline")
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3 (transient failures resumed twice)", calls)
	}
}

func TestRepairAgentSDKWithTransientRetryGivesUpAfterAttempts(t *testing.T) {
	calls := 0
	outline, err := repairAgentSDKWithTransientRetry(func() (*models.Outline, error) {
		calls++
		return nil, errors.New("agent runner failed: exit status 1: Error in hook callback hook_2")
	})
	if err == nil {
		t.Fatalf("repair() returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "Error in hook callback") {
		t.Fatalf("error = %v", err)
	}
	if outline != nil {
		t.Fatalf("repair() returned non-nil outline on failure")
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestRepairAgentSDKWithTransientRetryPermanentFailureSingleAttempt(t *testing.T) {
	calls := 0
	_, err := repairAgentSDKWithTransientRetry(func() (*models.Outline, error) {
		calls++
		return nil, errors.New("agent runner failed: exit status 1: failed to parse structured output")
	})
	if err == nil {
		t.Fatalf("repair() returned nil error, want error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (permanent failure must not retry)", calls)
	}
}
