package control

import (
	"fmt"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		batchState string
		expected   ErrorClass
	}{
		{"nil error", nil, "", ErrClassUnknown},
		{"environment - not found", fmt.Errorf("not found on PATH"), "", ErrClassEnvironment},
		{"environment - permission uppercase", fmt.Errorf("Permission denied"), "", ErrClassEnvironment},
		{"environment - permission lowercase", fmt.Errorf("permission denied"), "", ErrClassEnvironment},
		{"environment - permission mixed", fmt.Errorf("Permission Denied"), "", ErrClassEnvironment},
		{"auth - invalid key", fmt.Errorf("API key not valid"), "", ErrClassAuth},
		{"auth - 401", fmt.Errorf("401 Unauthorized"), "", ErrClassAuth},
		{"auth - 403", fmt.Errorf("403 Forbidden"), "", ErrClassAuth},
		{"timeout", fmt.Errorf("timeout waiting for job"), "", ErrClassTimeout},
		{"timeout uppercase", fmt.Errorf("TIMEOUT waiting"), "", ErrClassTimeout},
		{"budget", fmt.Errorf("BUDGET_STOPPED: exceeds limit"), "", ErrClassBudget},
		{"budget lowercase", fmt.Errorf("budget_stopped: exceeds limit"), "", ErrClassBudget},
		{"worker - FAILED", fmt.Errorf("some error"), "FAILED", ErrClassWorker},
		{"worker - CANCELLED", fmt.Errorf("some error"), "CANCELLED", ErrClassWorker},
		{"worker - failed lowercase", fmt.Errorf("some error"), "failed", ErrClassWorker},
		{"worker - cancelled mixed", fmt.Errorf("some error"), "Cancelled", ErrClassWorker},
		{"unknown", fmt.Errorf("something weird"), "", ErrClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err, tt.batchState)
			if result != tt.expected {
				t.Errorf("ClassifyError = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSuggestNextAction(t *testing.T) {
	tests := []struct {
		class    ErrorClass
		expected string
	}{
		{ErrClassEnvironment, "Check environment: run 'gary doctor'"},
		{ErrClassAuth, "Verify GEMINI_API_KEY and GCP credentials"},
		{ErrClassTimeout, "Check Batch job logs, consider increasing timeout"},
		{ErrClassBudget, "Increase GARY_MAX_COST_USD or wait for budget reset"},
		{ErrClassWorker, "Check worker logs: 'gcloud batch jobs describe <job>'"},
		{ErrClassUnknown, "Inspect logs for details"},
	}

	for _, tt := range tests {
		t.Run(string(tt.class), func(t *testing.T) {
			result := SuggestNextAction(tt.class)
			if result != tt.expected {
				t.Errorf("SuggestNextAction = %q, want %q", result, tt.expected)
			}
		})
	}
}
