// Package control provides structured error types for GARY operations.
package control

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
	Class          ErrorClass `json:"class"`
	BatchJobID     string     `json:"batch_job_id,omitempty"`
	Attempt        int        `json:"attempt,omitempty"`
	WorkerState    string     `json:"worker_state,omitempty"`
	Acceptance     string     `json:"acceptance_state,omitempty"`
	Branch         string     `json:"branch,omitempty"`
	Commit         string     `json:"commit,omitempty"`
	Summary        string     `json:"summary"`
	NextAction     string     `json:"next_action"`
}

// ClassifyError determines the error class from error characteristics.
func ClassifyError(err error, batchState string) ErrorClass {
	if err == nil {
		return ErrClassUnknown
	}

	msg := err.Error()

	// Environment failures
	if containsAny(msg, []string{
		"not found on PATH",
		"executable file not found",
		"no such file or directory",
		"Permission denied",
	}) {
		return ErrClassEnvironment
	}

	// Auth failures
	if containsAny(msg, []string{
		"API_KEY_INVALID",
		"API key not valid",
		"permission denied",
		"401",
		"403",
		"not authenticated",
	}) {
		return ErrClassAuth
	}

	// Timeout
	if containsAny(msg, []string{
		"timeout",
		"deadline exceeded",
		"context canceled",
	}) {
		return ErrClassTimeout
	}

	// Budget
	if containsAny(msg, []string{
		"BUDGET_STOPPED",
		"exceeds limit",
	}) {
		return ErrClassBudget
	}

	// Worker failures based on Batch state
	if batchState == "FAILED" || batchState == "CANCELLED" {
		return ErrClassWorker
	}

	return ErrClassUnknown
}

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if len(s) >= len(p) {
			for i := 0; i <= len(s)-len(p); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
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
