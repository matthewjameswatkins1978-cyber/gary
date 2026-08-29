// Package config loads and validates GARY configuration from
// environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the GARY environment configuration.
type Config struct {
	GCPProject       string
	GCPRegion        string
	GCPZone          string
	GitHubRepo       string
	ResultsDir       string
	GeminiAPIKey     string
	ImplementerModel string
	CloserModel      string
	MasterModel      string
	WorkerRuntime    string
	MaxCostUSD       float64
}

// Load reads configuration from environment variables and applies defaults.
func Load() (*Config, error) {
	c := &Config{
		GCPProject:       os.Getenv("GARY_GCP_PROJECT"),
		GCPRegion:        os.Getenv("GARY_GCP_REGION"),
		GCPZone:          os.Getenv("GARY_GCP_ZONE"),
		GitHubRepo:       os.Getenv("GARY_GITHUB_REPO"),
		ResultsDir:       os.Getenv("GARY_RESULTS_DIR"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		ImplementerModel: os.Getenv("GARY_IMPLEMENTER_MODEL"),
		CloserModel:      os.Getenv("GARY_CLOSER_MODEL"),
		MasterModel:      os.Getenv("GARY_MASTER_MODEL"),
		WorkerRuntime:    os.Getenv("GARY_WORKER_RUNTIME"),
	}

	if c.ResultsDir == "" {
		c.ResultsDir = filepath.Join(".", "results")
	}

	if c.ImplementerModel == "" {
		c.ImplementerModel = "gemini-3.5-flash-lite"
	}

	if c.CloserModel == "" {
		c.CloserModel = "gemini-2.5-flash"
	}

	if c.MasterModel == "" {
		c.MasterModel = "gemini-3.7-flash"
	}

	if c.WorkerRuntime == "" {
		c.WorkerRuntime = "batch"
	}

	if v := os.Getenv("GARY_MAX_COST_USD"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid GARY_MAX_COST_USD: %s", v)
		}
		c.MaxCostUSD = n
	}

	return c, nil
}

// Validate checks that the configuration contains the values required for
// cloud operations. Local-only usage (version, doctor) does not require
// validation.
func (c *Config) Validate() error {
	var missing []string

	if c.GCPProject == "" {
		missing = append(missing, "GARY_GCP_PROJECT")
	}
	if c.GCPRegion == "" {
		missing = append(missing, "GARY_GCP_REGION")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %v", missing)
	}

	return nil
}

// HasGCPConfig reports whether the GCP project and region are set.
func (c *Config) HasGCPConfig() bool {
	return c.GCPProject != "" && c.GCPRegion != ""
}
