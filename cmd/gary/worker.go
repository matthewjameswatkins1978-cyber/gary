package main

import (
	"fmt"
	"os"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func cmdWorker() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Default worker: %s\n", cfg.WorkerRuntime)
	fmt.Printf("Model: %s\n", cfg.ImplementerModel)
}
