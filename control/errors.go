// Package control provides structured error types for GARY operations.
package control

import "strings"

// ErrorClass classifies the type of failure.
type ErrorClass string

const (
	ErrClassEnvironment ErrorClass = "environment"
	ErrClassAuth        ErrorClass = "auth"
	ErrClassWorker      ErrorClass = "worker"
	ErrClassTimeout     ErrorClass = "timeout"
	ErrClassBudget      ErrorClass = "budget"
	ErrClassUnknown     ErrorClass = "unknown"
)

// StructuredError provides concise, machine-readable error information.
type StructuredError struct {
	Class      ErrorClass `json:"class"`
	BatchJobID string     `json:"batch_job_id,omitempty"`
	Attempt    int        `json:"attempt,omitempty"`
	WorkerState string    `json:"worker_state,omitempty"`
	Acceptance string     `json:"acceptance_state,omitempty"`
	Branch     string     `json:"branch,omitempty"`
	Commit     string     `json:"commit,omitempty"`
	Summary    string     `json:"summary"`
	NextAction string     `json:"next_action"`
}

// ClassifyError determines the error class from error characteristics.
func ClassifyError(err error, batchState string) ErrorClass {
	if err == nil {
		return ErrClassUnknown
	}

	msg := strings.ToLower(err.Error())

	// Environment failures
	if containsAnyLower(msg, []string{
		"not found on path",
		"executable file not found",
		"no such file or directory",
		"permission denied",
	}) {
		return ErrClassEnvironment
	}

	// Auth failures
	if containsAnyLower(msg, []string{
		"api_key_invalid",
		"api key not valid",
		"401",
		"403",
		"not authenticated",
	}) {
		return ErrClassAuth
	}

	// Timeout
	if containsAnyLower(msg, []string{
		"timeout",
		"deadline exceeded",
		"context canceled",
	}) {
		return ErrClassTimeout
	}

	// Budget
	if containsAnyLower(msg, []string{
		"budget_stopped",
		"exceeds limit",
	}) {
		return ErrClassBudget
	}

	// Worker failures based on Batch state
	lower := strings.ToLower(batchState)
	if lower == "failed" || lower == "cancelled" {
		return ErrClassWorker
	}

	return ErrClassUnknown
}

func containsAnyLower(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// SuggestNextAction returns a human-readable next step based on error class.
func SuggestNextAction(class ErrorClass) string {
	switch class {
	case ErrClassEnvironment:
		return "Check environment: run 'gary doctor'"
	case ErrClassAuth:
		return "Verify GEMINI_API_KEY and GCP credentials"
	case ErrClassTimeout:
		return "Check Batch job logs, consider increasing timeout"
	case ErrClassBudget:
		return "Increase GARY_MAX_COST_USD or wait for budget reset"
	case ErrClassWorker:
		return "Check worker logs: 'gcloud batch jobs describe <job>'"
	default:
		return "Inspect logs for details"
	}
}
