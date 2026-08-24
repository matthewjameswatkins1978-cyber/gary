package benchmark

import (
	"testing"
)

func TestEstimateCost(t *testing.T) {
	pt := DefaultPriceTable()

	cost := EstimateCost(pt, 100000, 5000, 50000)

	inputExpected := float64(100000) * 0.000000075
	outputExpected := float64(5000) * 0.0000003
	cachedExpected := float64(50000) * 0.00000001875
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
}
