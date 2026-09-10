package main

import (
	"fmt"
	"os"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/yard"
)

func cmdCancel() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: gary cancel <job-id>")
		os.Exit(2)
	}
	root := config.StateRoot()
	if override := os.Getenv("GARY_STATE_DIR"); override != "" {
		root = override
	}
	manager, err := yard.LoadManager(root, os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cancel: %v\n", err)
		os.Exit(1)
	}
	if err := manager.Cancel(); err != nil {
		fmt.Fprintf(os.Stderr, "cancel: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("GARY_STATUS: CANCELLED\nJob: %s\n", os.Args[2])
}
