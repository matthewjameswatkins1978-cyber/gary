package benchmark

import "github.com/matthewjameswatkins1978-cyber/gary/internal/cost"

// Route identifies which execution route was used.
type Route string

const (
	RouteManagedAgent Route = "managed-agent"
	RouteBatchCLI     Route = "batch-cli"
)

// Record holds the benchmark result for one route.
type Record struct {
	Route                Route    `json:"route"`
	Runtime              string   `json:"runtime"`
	Agent                string   `json:"agent"`
	RequestedModel       string   `json:"requested_model"`
	ActualModels         []string `json:"actual_models"`
	SameModelBenchmark   bool     `json:"same_model_benchmark"`
	ThinkingLevel        string   `json:"thinking_level"`
	Accepted             bool     `json:"accepted"`
	WallSeconds          float64  `json:"wall_seconds"`
	HumanInterventionMin float64  `json:"human_intervention_minutes"`
	InputTokens          int      `json:"input_tokens"`
	CachedTokens         int      `json:"cached_tokens"`
	OutputTokens         int      `json:"output_tokens"`
	ThoughtTokens        int      `json:"thought_tokens"`
	TotalTokens          int      `json:"total_tokens"`
	ComputeSeconds       float64  `json:"compute_seconds"`
	TransferBytes        int64    `json:"transfer_bytes"`
	TransferSeconds      float64  `json:"transfer_seconds"`
	Attempts             int      `json:"attempts"`
	EstimatedModelCost   float64  `json:"estimated_model_cost_usd"`
}

// Result holds both route records and derived comparison.
type Result struct {
	Build       string   `json:"build"`
	Task        string   `json:"task"`
	BenchmarkAt string   `json:"benchmark_at"`
	Records     []Record `json:"records"`
}

// PriceTable is retained as an alias for compatibility. Pricing now lives in
// the shared cost package so every GARY path uses the same versioned table.
type PriceTable = cost.PriceTable

// DefaultPriceTable returns the current Gemini Developer API standard pricing
// for gemini-3.5-flash-lite.
func DefaultPriceTable() PriceTable {
	return cost.PriceForModel("gemini-3.5-flash-lite")
}

// PriceTableForModel returns the current standard text-token pricing used by
// GARY's pre-dispatch cost estimator. Unknown models are explicitly unknown;
// callers must not silently substitute another model's price.
func PriceTableForModel(model string) PriceTable {
	return cost.PriceForModel(model)
}

// EstimateCost calculates estimated model cost in USD. The thoughtTokens
// argument remains for source compatibility; provider output counts already
// include reasoning tokens and must not be double-counted.
func EstimateCost(pt PriceTable, inputTokens, outputTokens, thoughtTokens, cachedTokens int) float64 {
	// thoughtTokens is intentionally ignored: provider output counts already
	// include reasoning tokens. Keep the argument for source compatibility.
	return cost.ModelCost(pt, inputTokens-cachedTokens, cachedTokens, outputTokens)
}
