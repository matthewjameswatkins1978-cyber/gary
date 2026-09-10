package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/yard"
)

func cmdProfiles() {
	jsonOutput := len(os.Args) == 3 && os.Args[2] == "--json"
	if len(os.Args) > 2 && !jsonOutput {
		fmt.Fprintln(os.Stderr, "Usage: gary profiles [--json]")
		os.Exit(2)
	}
	profiles := yard.Profiles()
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(profiles)
		return
	}
	fmt.Println("GARY WORKER PROFILES")
	for _, profile := range profiles {
		fmt.Printf("%-48s %-18s %-10s identity=%s web=%s\n", profile.ID, profile.Certification, profile.BillingClass, profile.IdentityConfidence, profile.WebSearch)
	}
}
