package benchmark

import (
	"testing"
)

func TestEstimateCost(t *testing.T) {
	pt := DefaultPriceTable()

	// 100K input, 5K output, 2K thought, 50K cached
	cost := EstimateCost(pt, 100000, 5000, 2000, 50000)

	// Billed output = output + thought = 7000
	inputExpected := float64(100000) * 0.0000003
	outputExpected := float64(7000) * 0.0000025
	cachedExpected := float64(50000) * 0.00000003
	expected := inputExpected + outputExpected + cachedExpected

	if cost != expected {
		t.Errorf("EstimateCost = %f, want %f", cost, expected)
	}
}

func TestEstimateCostZeroThought(t *testing.T) {
	pt := DefaultPriceTable()

	// Zero thought tokens
	cost := EstimateCost(pt, 100000, 5000, 0, 50000)

	inputExpected := float64(100000) * 0.0000003
	outputExpected := float64(5000) * 0.0000025
	cachedExpected := float64(50000) * 0.00000003
	expected := inputExpected + outputExpected + cachedExpected

	if cost != expected {
		t.Errorf("EstimateCost = %f, want %f", cost, expected)
	}
}

func TestDefaultPriceTable(t *testing.T) {
	pt := DefaultPriceTable()
	if pt.ModelName != "gemini-3.5-flash-lite" {
		t.Errorf("ModelName = %q, want %q", pt.ModelName, "gemini-3.5-flash-lite")
	}
	if pt.InputPrice <= 0 {
		t.Errorf("InputPrice = %f, want > 0", pt.InputPrice)
	}
	if pt.OutputPrice <= 0 {
		t.Errorf("OutputPrice = %f, want > 0", pt.OutputPrice)
	}
	if pt.CachedPrice <= 0 {
		t.Errorf("CachedPrice = %f, want > 0", pt.CachedPrice)
	}
}

func TestPriceTableForGemini37Flash(t *testing.T) {
	pt := PriceTableForModel("gemini-3.7-flash")
	if pt.ModelName != "gemini-3.7-flash" {
		t.Fatalf("ModelName = %q, want gemini-3.7-flash", pt.ModelName)
	}
	if pt.InputPrice != 0.00000075 {
		t.Errorf("InputPrice = %f, want 0.00000075", pt.InputPrice)
	}
	if pt.OutputPrice != 0.00000375 {
		t.Errorf("OutputPrice = %f, want 0.00000375", pt.OutputPrice)
	}
	if pt.CachedPrice != 0.000000075 {
		t.Errorf("CachedPrice = %f, want 0.000000075", pt.CachedPrice)
	}
}
