// Package cost contains the small, versioned cost records shared by GARY's
// single-agent worker, ledger, and terminal status paths.
package cost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProviderReported = "PROVIDER_REPORTED"
	Mixed            = "MIXED"
	Estimated        = "ESTIMATED"
	LegacyEstimate   = "LEGACY_ESTIMATE"
	Unknown          = "UNKNOWN"

	PricingVersion       = "2026-08-30"
	PricingEffectiveDate = "2026-08-30"
	PricingSource        = "https://ai.google.dev/gemini-api/docs/pricing"
	ComputePricingSource = "https://cloud.google.com/compute/vm-instance-pricing"
)

// PriceTable stores USD prices per token. Prices are versioned with the
// accounting record so later price changes do not rewrite history.
type PriceTable struct {
	ModelName     string  `json:"model_name"`
	InputPrice    float64 `json:"input_per_token"`
	OutputPrice   float64 `json:"output_per_token"`
	CachedPrice   float64 `json:"cached_per_token"`
	Version       string  `json:"pricing_version"`
	EffectiveDate string  `json:"pricing_effective_date"`
	Source        string  `json:"pricing_source"`
	Known         bool    `json:"known"`
}

// Usage is provider-reported token telemetry. OutputTokens includes thinking
// tokens where the provider's output field is documented as inclusive.
type Usage struct {
	ModelRequested     string `json:"model_requested,omitempty"`
	ModelVersion       string `json:"model_version,omitempty"`
	UsageSource        string `json:"usage_source"`
	InputTokens        int    `json:"input_tokens"`
	CachedInputTokens  int    `json:"cached_input_tokens"`
	FreshInputTokens   int    `json:"fresh_input_tokens"`
	OutputTokens       int    `json:"output_tokens"`
	ThinkingTokens     *int   `json:"thinking_tokens,omitempty"`
	TotalTokens        int    `json:"total_tokens"`
	ProviderResponseID string `json:"provider_response_id,omitempty"`
	ProviderRequestID  string `json:"provider_request_id,omitempty"`
}

// Component records one billable model call and/or its Batch runtime.
type Component struct {
	Kind                 string  `json:"kind"`
	ModelRequested       string  `json:"model_requested,omitempty"`
	ModelVersion         string  `json:"model_version,omitempty"`
	UsageSource          string  `json:"usage_source"`
	InputTokens          int     `json:"input_tokens,omitempty"`
	CachedInputTokens    int     `json:"cached_input_tokens,omitempty"`
	FreshInputTokens     int     `json:"fresh_input_tokens,omitempty"`
	OutputTokens         int     `json:"output_tokens,omitempty"`
	ThinkingTokens       *int    `json:"thinking_tokens,omitempty"`
	TotalTokens          int     `json:"total_tokens,omitempty"`
	ModelCostUSD         float64 `json:"model_cost_usd"`
	ComputeCostUSD       float64 `json:"compute_cost_usd"`
	TotalCostUSD         float64 `json:"total_cost_usd"`
	RuntimeSeconds       float64 `json:"runtime_seconds,omitempty"`
	MachineType          string  `json:"machine_type,omitempty"`
	ProvisioningModel    string  `json:"provisioning_model,omitempty"`
	PricingVersion       string  `json:"pricing_version"`
	PricingEffectiveDate string  `json:"pricing_effective_date"`
	PricingSource        string  `json:"pricing_source"`
	ProviderResponseID   string  `json:"provider_response_id,omitempty"`
	ProviderRequestID    string  `json:"provider_request_id,omitempty"`
	UnmeteredReason      string  `json:"unmetered_reason,omitempty"`
}

// Summary is the terminal cost record for one GARY work invocation.
type Summary struct {
	PricingStatus       string      `json:"pricing_status"`
	Currency            string      `json:"currency"`
	PricingVersion      string      `json:"pricing_version"`
	WorkerModelCostUSD  float64     `json:"worker_model_cost_usd,omitempty"`
	ModelCostUSD        float64     `json:"model_cost_usd"`
	ComputeCostUSD      float64     `json:"compute_cost_usd"`
	TotalCostUSD        float64     `json:"total_cost_usd"`
	Components          []Component `json:"components,omitempty"`
	UnmeteredComponents []string    `json:"unmetered_components,omitempty"`
}

// NewSummary creates a summary with stable accounting metadata.
func NewSummary() *Summary {
	return &Summary{PricingStatus: Estimated, Currency: "USD", PricingVersion: PricingVersion}
}

// Add includes a component exactly once at the caller's chosen aggregation
// boundary. Resumed masters retain their existing summary and add only new
// components from newly executed waves.
func (s *Summary) Add(c Component) {
	if s == nil {
		return
	}
	if s.Currency == "" {
		s.Currency = "USD"
	}
	if s.PricingVersion == "" {
		s.PricingVersion = PricingVersion
	}
	s.Components = append(s.Components, c)
	s.ModelCostUSD += c.ModelCostUSD
	s.ComputeCostUSD += c.ComputeCostUSD
	s.TotalCostUSD += c.TotalCostUSD
	if c.Kind == "worker" {
		s.WorkerModelCostUSD += c.ModelCostUSD
	}
	if c.UnmeteredReason != "" {
		s.UnmeteredComponents = append(s.UnmeteredComponents, c.UnmeteredReason)
	}
	s.recomputeStatus()
}

func (s *Summary) recomputeStatus() {
	hasProvider, hasEstimate, hasLegacy := false, false, false
	for _, component := range s.Components {
		switch component.UsageSource {
		case ProviderReported:
			hasProvider = true
		default:
			if component.UsageSource == LegacyEstimate {
				hasLegacy = true
			} else {
				hasEstimate = true
			}
		}
		if component.UnmeteredReason != "" {
			hasEstimate = true
		}
	}
	switch {
	case hasLegacy && !hasProvider && !hasEstimate:
		s.PricingStatus = LegacyEstimate
	case hasProvider && !hasLegacy && !hasEstimate:
		s.PricingStatus = ProviderReported
	case hasProvider || hasLegacy:
		s.PricingStatus = Mixed
	default:
		s.PricingStatus = Estimated
	}
}

// PriceForModel returns the current versioned standard token prices used by
// GARY's Gemini API and Vertex calls. Unknown models are explicitly marked
// unknown; callers must not present their costs as provider-reported.
func PriceForModel(model string) PriceTable {
	pt := PriceTable{ModelName: model, Version: PricingVersion, EffectiveDate: PricingEffectiveDate, Source: PricingSource, Known: true}
	switch model {
	case "gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.6-flash":
		pt.InputPrice, pt.OutputPrice, pt.CachedPrice = 0.00000075, 0.00000375, 0.000000075
	case "gemini-3.5-flash-lite", "gemini-2.5-flash":
		pt.InputPrice, pt.OutputPrice, pt.CachedPrice = 0.00000030, 0.00000250, 0.00000003
	default:
		pt.Known = false
		pt.InputPrice, pt.OutputPrice, pt.CachedPrice = 0, 0, 0
	}
	return pt
}

// ModelCost applies fresh-input, cached-input, and output prices. Thinking
// tokens are deliberately not added: the provider output count is inclusive.
func ModelCost(pt PriceTable, freshInput, cachedInput, output int) float64 {
	return float64(freshInput)*pt.InputPrice + float64(cachedInput)*pt.CachedPrice + float64(output)*pt.OutputPrice
}

// ComputeCost estimates Batch VM cost from observed runtime. Google Batch
// billing is not queried here, so this component is always estimated.
func ComputeCost(machineType, provisioningModel string, runtimeSeconds float64) (float64, bool) {
	ratePerHour, ok := map[string]float64{
		"e2-small":      0.0207,
		"e2-standard-4": 0.1340,
	}[machineType]
	if provisioningModel != "STANDARD" {
		ok = false
	}
	if runtimeSeconds < 0 {
		runtimeSeconds = 0
	}
	return runtimeSeconds * ratePerHour / 3600, ok
}

// ParseGeminiCLIStreamJSONL parses the pinned Gemini CLI 0.56.0 stream-json
// terminal result. Non-JSON informational lines are ignored, but a missing or
// malformed terminal stats record is an explicit error.
func ParseGeminiCLIStreamJSONL(data []byte, requestedModel string) (Usage, error) {
	type modelStats struct {
		TotalTokens  *int `json:"total_tokens"`
		InputTokens  *int `json:"input_tokens"`
		OutputTokens *int `json:"output_tokens"`
		Cached       *int `json:"cached"`
		Input        *int `json:"input"`
	}
	type stats struct {
		TotalTokens  *int                  `json:"total_tokens"`
		InputTokens  *int                  `json:"input_tokens"`
		OutputTokens *int                  `json:"output_tokens"`
		Cached       *int                  `json:"cached"`
		Input        *int                  `json:"input"`
		Models       map[string]modelStats `json:"models"`
	}
	type event struct {
		Type  string `json:"type"`
		Stats *stats `json:"stats"`
	}

	var found *stats
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), 4<<20)
	for scanner.Scan() {
		var e event
		if json.Unmarshal(scanner.Bytes(), &e) != nil || e.Type != "result" || e.Stats == nil {
			continue
		}
		copy := *e.Stats
		found = &copy
	}
	if err := scanner.Err(); err != nil {
		return Usage{}, fmt.Errorf("read Gemini CLI JSONL: %w", err)
	}
	if found == nil {
		return Usage{}, fmt.Errorf("Gemini CLI JSONL has no result stats")
	}

	var ms modelStats
	if requestedModel != "" {
		ms = found.Models[requestedModel]
	}
	if ms.InputTokens == nil {
		for _, candidate := range found.Models {
			ms.InputTokens = addPtr(ms.InputTokens, candidate.InputTokens)
			ms.OutputTokens = addPtr(ms.OutputTokens, candidate.OutputTokens)
			ms.Cached = addPtr(ms.Cached, candidate.Cached)
			ms.Input = addPtr(ms.Input, candidate.Input)
			ms.TotalTokens = addPtr(ms.TotalTokens, candidate.TotalTokens)
		}
	}
	input := firstInt(ms.InputTokens, found.InputTokens)
	cached := firstInt(ms.Cached, found.Cached)
	fresh := firstInt(ms.Input, nil)
	if fresh == 0 && input >= cached {
		fresh = input - cached
	}
	output := firstInt(ms.OutputTokens, found.OutputTokens)
	total := firstInt(ms.TotalTokens, found.TotalTokens)
	if total == 0 {
		total = input + output
	}
	if input == 0 && output == 0 && total == 0 {
		return Usage{}, fmt.Errorf("Gemini CLI result stats contain no token usage")
	}
	return Usage{
		ModelRequested:    requestedModel,
		UsageSource:       ProviderReported,
		InputTokens:       input,
		CachedInputTokens: cached,
		FreshInputTokens:  fresh,
		OutputTokens:      output,
		TotalTokens:       total,
	}, nil
}

// ParseVertexUsage parses the usageMetadata object returned by Vertex or the
// Gemini Developer API. thoughtsTokenCount is retained as metadata only and
// is not added to candidatesTokenCount.
func ParseVertexUsage(raw json.RawMessage, requestedModel, responseID, requestID string) (Usage, error) {
	var response struct {
		UsageMetadata struct {
			PromptTokenCount        *int `json:"promptTokenCount"`
			CandidatesTokenCount    *int `json:"candidatesTokenCount"`
			TotalTokenCount         *int `json:"totalTokenCount"`
			CachedContentTokenCount *int `json:"cachedContentTokenCount"`
			ThoughtsTokenCount      *int `json:"thoughtsTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion"`
		ResponseID   string `json:"responseId"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return Usage{}, err
	}
	u := response.UsageMetadata
	if u.PromptTokenCount == nil || u.CandidatesTokenCount == nil || u.TotalTokenCount == nil {
		return Usage{}, fmt.Errorf("response has incomplete usageMetadata")
	}
	cached := firstInt(u.CachedContentTokenCount, nil)
	return Usage{
		ModelRequested:     requestedModel,
		ModelVersion:       response.ModelVersion,
		UsageSource:        ProviderReported,
		InputTokens:        *u.PromptTokenCount,
		CachedInputTokens:  cached,
		FreshInputTokens:   *u.PromptTokenCount - cached,
		OutputTokens:       *u.CandidatesTokenCount,
		ThinkingTokens:     u.ThoughtsTokenCount,
		TotalTokens:        *u.TotalTokenCount,
		ProviderResponseID: coalesce(response.ResponseID, responseID),
		ProviderRequestID:  requestID,
	}, nil
}

func firstInt(primary, fallback *int) int {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return 0
}

func addPtr(a, b *int) *int {
	if a == nil && b == nil {
		return nil
	}
	n := firstInt(a, nil) + firstInt(b, nil)
	return &n
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
