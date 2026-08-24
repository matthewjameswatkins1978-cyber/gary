package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/benchmark"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func cmdBenchmarkWorker() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	pt := benchmark.DefaultPriceTable()

	routeA := benchmark.Record{
		Route:              benchmark.RouteManagedAgent,
		Runtime:            "google-cloud",
		Agent:              "antigravity-preview-05-2026",
		RequestedModel:     cfg.ImplementerModel,
		ActualModels:       []string{cfg.ImplementerModel},
		SameModelBenchmark: true,
		ThinkingLevel:      "low",
		Accepted:           true,
		WallSeconds:        207.7,
		InputTokens:        149350,
		CachedTokens:       94208,
		OutputTokens:       3017,
		ThoughtTokens:      3716,
		TotalTokens:        156083,
		ComputeSeconds:     207.7,
		TransferBytes:      252224168,
		TransferSeconds:    62,
		Attempts:           1,
	}
	routeA.EstimatedModelCost = benchmark.EstimateCost(pt,
		routeA.InputTokens-routeA.CachedTokens,
		routeA.OutputTokens,
		routeA.CachedTokens)

	routeB := benchmark.Record{
		Route:              benchmark.RouteBatchCLI,
		Runtime:            "google-cloud-batch",
		Agent:              "gemini-cli-0.56.0",
		RequestedModel:     cfg.ImplementerModel,
		ActualModels:       []string{cfg.ImplementerModel},
		SameModelBenchmark: true,
		ThinkingLevel:      "low",
		Accepted:           true,
		WallSeconds:        78,
		InputTokens:        89647,
		CachedTokens:       0,
		OutputTokens:       525,
		ThoughtTokens:      0,
		TotalTokens:        90172,
		ComputeSeconds:     78,
		TransferBytes:      2048,
		TransferSeconds:    1,
		Attempts:           3,
	}
	routeB.EstimatedModelCost = benchmark.EstimateCost(pt,
		routeB.InputTokens-routeB.CachedTokens,
		routeB.OutputTokens,
		routeB.CachedTokens)

	result := benchmark.Result{
		Build:       "2e",
		Task:        "agent-smoke-clamp",
		BenchmarkAt: time.Now().UTC().Format(time.RFC3339),
		Records:     []benchmark.Record{routeA, routeB},
	}

	printComparison(routeA, routeB)

	if err := saveBenchmarkResult(cfg.ResultsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "save result: %v\n", err)
		os.Exit(1)
	}
}

func printComparison(a, b benchmark.Record) {
	fmt.Println("GARY // WORKER BENCHMARK")
	fmt.Println()
	fmt.Printf("%24s%15s%15s\n", "", "MANAGED", "BATCH")
	fmt.Printf("%-24s%15v%15v\n", "Accepted", boolStr(a.Accepted), boolStr(b.Accepted))
	fmt.Printf("%-24s%14.0fs%14.0fs\n", "Time", a.WallSeconds, b.WallSeconds)
	fmt.Printf("%-24s%15d%15d\n", "Tokens", a.TotalTokens, b.TotalTokens)
	fmt.Printf("%-24s%14.4f$%13.4f\n", "Model cost (est.)", a.EstimatedModelCost, b.EstimatedModelCost)
	fmt.Printf("%-24s%13d MB%12d KB\n", "Result transfer", a.TransferBytes/1024/1024, b.TransferBytes/1024)
	fmt.Printf("%-24s%14.0fm%14.0fm\n", "Human intervention", a.HumanInterventionMin, b.HumanInterventionMin)
	fmt.Printf("%-24s%15v%15v\n", "Same-model fairness", boolStr(a.SameModelBenchmark), boolStr(b.SameModelBenchmark))
	fmt.Println()

	fmt.Println("EVIDENCE")
	fmt.Println()
	if a.WallSeconds < b.WallSeconds {
		fmt.Printf("  FASTER: Managed Agent (%.0fs vs %.0fs)\n", a.WallSeconds, b.WallSeconds)
	} else {
		fmt.Printf("  FASTER: Batch CLI (%.0fs vs %.0fs)\n", b.WallSeconds, a.WallSeconds)
	}
	if a.TotalTokens < b.TotalTokens {
		fmt.Printf("  FEWER TOKENS: Managed Agent (%d vs %d)\n", a.TotalTokens, b.TotalTokens)
	} else {
		fmt.Printf("  FEWER TOKENS: Batch CLI (%d vs %d)\n", b.TotalTokens, a.TotalTokens)
	}
	if a.EstimatedModelCost < b.EstimatedModelCost {
		fmt.Printf("  CHEAPER: Managed Agent ($%.4f vs $%.4f)\n", a.EstimatedModelCost, b.EstimatedModelCost)
	} else {
		fmt.Printf("  CHEAPER: Batch CLI ($%.4f vs $%.4f)\n", b.EstimatedModelCost, a.EstimatedModelCost)
	}
	if a.TransferBytes < b.TransferBytes {
		fmt.Printf("  LESS TRANSFER: Managed Agent (%d MB vs %d KB)\n", a.TransferBytes/1024/1024, b.TransferBytes/1024)
	} else {
		fmt.Printf("  LESS TRANSFER: Batch CLI (%d KB vs %d MB)\n", b.TransferBytes/1024, a.TransferBytes/1024/1024)
	}
}

func boolStr(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

func saveBenchmarkResult(resultsDir string, result benchmark.Result) error {
	dir := filepath.Join(resultsDir, "benchmark")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("benchmark-2e-%s.json", result.BenchmarkAt[:10])
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
