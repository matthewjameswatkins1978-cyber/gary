package benchmark

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

// PriceTable is a configurable local price table for model cost estimation.
// Prices are in USD per token.
type PriceTable struct {
	ModelName   string  `json:"model_name"`
	InputPrice  float64 `json:"input_per_token"`
	OutputPrice float64 `json:"output_per_token"`
	CachedPrice float64 `json:"cached_per_token"`
}

// DefaultPriceTable returns the current Gemini Developer API standard pricing
// for gemini-3.5-flash-lite.
func DefaultPriceTable() PriceTable {
	return PriceTable{
		ModelName:   "gemini-3.5-flash-lite",
		InputPrice:  0.0000003,  // $0.30 / 1M
		OutputPrice: 0.0000025,  // $2.50 / 1M (includes thinking tokens)
		CachedPrice: 0.00000003, // $0.03 / 1M
	}
}

// PriceTableForModel returns the current standard text-token pricing used by
// GARY's cost estimator. Gemini 3.7/3.6 Flash promotional pricing is valid
// through 2026-12-31. Unknown models fall back to the existing Lite table
// rather than silently returning zero cost.
func PriceTableForModel(model string) PriceTable {
	switch model {
	case "gemini-3.7-flash", "gemini-3.6-flash":
		return PriceTable{
			ModelName:   model,
			InputPrice:  0.00000075,  // $0.75 / 1M
			OutputPrice: 0.00000375,  // $3.75 / 1M (includes thinking tokens)
			CachedPrice: 0.000000075, // $0.075 / 1M
		}
	case "gemini-2.5-flash", "gemini-3.5-flash-lite":
		pt := DefaultPriceTable()
		pt.ModelName = model
		return pt
	default:
		return DefaultPriceTable()
	}
}

// EstimateCost calculates estimated model cost in USD.
// Billed output tokens = outputTokens + thoughtTokens.
func EstimateCost(pt PriceTable, inputTokens, outputTokens, thoughtTokens, cachedTokens int) float64 {
	input := float64(inputTokens) * pt.InputPrice
	output := float64(outputTokens+thoughtTokens) * pt.OutputPrice
	cached := float64(cachedTokens) * pt.CachedPrice
	return input + output + cached
}
