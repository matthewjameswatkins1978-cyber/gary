package cost

import (
	"encoding/json"
	"testing"
)

func TestParseGeminiCLIStreamJSONLUsesPinnedResultStats(t *testing.T) {
	data := []byte(`informational line
{"type":"tool_use","tool_name":"git"}
{"type":"result","status":"success","stats":{"total_tokens":1670,"input_tokens":1250,"output_tokens":420,"cached":200,"input":1050,"duration_ms":1840,"models":{"gemini-3.5-flash-lite":{"total_tokens":1670,"input_tokens":1250,"output_tokens":420,"cached":200,"input":1050}}}}
`)
	u, err := ParseGeminiCLIStreamJSONL(data, "gemini-3.5-flash-lite")
	if err != nil {
		t.Fatal(err)
	}
	if u.UsageSource != ProviderReported || u.InputTokens != 1250 || u.CachedInputTokens != 200 || u.FreshInputTokens != 1050 || u.OutputTokens != 420 || u.TotalTokens != 1670 {
		t.Fatalf("unexpected usage: %#v", u)
	}
}

func TestParseGeminiCLIStreamJSONLMissingTelemetryFails(t *testing.T) {
	if _, err := ParseGeminiCLIStreamJSONL([]byte(`{"type":"result","status":"error"}`), "gemini-3.5-flash-lite"); err == nil {
		t.Fatal("missing result stats must not become zero provider usage")
	}
}

func TestParseVertexUsageDoesNotDoubleCountThoughts(t *testing.T) {
	raw := json.RawMessage(`{"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":250,"candidatesTokenCount":400,"thoughtsTokenCount":150,"totalTokenCount":1400},"modelVersion":"gemini-3.7-flash-001","responseId":"resp-1"}`)
	u, err := ParseVertexUsage(raw, "gemini-3.7-flash", "", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if u.FreshInputTokens != 750 || u.OutputTokens != 400 || u.TotalTokens != 1400 || u.ThinkingTokens == nil || *u.ThinkingTokens != 150 {
		t.Fatalf("unexpected Vertex usage: %#v", u)
	}
	pt := PriceForModel("gemini-3.7-flash")
	if got := ModelCost(pt, u.FreshInputTokens, u.CachedInputTokens, u.OutputTokens); got != 0.00208125 {
		t.Fatalf("cost = %.7f, want %.7f", got, 0.00208125)
	}
}

func TestSummaryMarksProviderUsagePlusBatchComputeMixed(t *testing.T) {
	s := NewSummary()
	s.Add(Component{Kind: "worker", UsageSource: ProviderReported, ModelCostUSD: 0.01, ComputeCostUSD: 0.02, TotalCostUSD: 0.03, UnmeteredReason: "Batch invoice unavailable"})
	if s.PricingStatus != Mixed || s.TotalCostUSD != 0.03 {
		t.Fatalf("summary = %#v", s)
	}
}

func TestUnknownModelHasNoSilentPrice(t *testing.T) {
	pt := PriceForModel("unknown-model")
	if pt.Known || pt.InputPrice != 0 || pt.OutputPrice != 0 || pt.CachedPrice != 0 {
		t.Fatalf("unknown model received an implicit price: %#v", pt)
	}
}

func TestComputeCostUsesObservedRuntimeAndMachine(t *testing.T) {
	got, known := ComputeCost("e2-small", "STANDARD", 3600)
	if !known || got != 0.0207 {
		t.Fatalf("compute cost = %.7f known=%t", got, known)
	}
	if _, known := ComputeCost("e2-small", "SPOT", 3600); known {
		t.Fatal("unsupported provisioning model must not be treated as known")
	}
}
