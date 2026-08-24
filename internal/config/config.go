// Package config loads and validates City Computer configuration from
// environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the City Computer environment configuration.
type Config struct {
	GCPProject  string
	GCPRegion   string
	GCPZone     string
	GitHubRepo  string
	ResultsDir  string
}

// Load reads configuration from environment variables and applies defaults.
func Load() (*Config, error) {
	c := &Config{
		GCPProject: os.Getenv("CITY_GCP_PROJECT"),
		GCPRegion:  os.Getenv("CITY_GCP_REGION"),
		GCPZone:    os.Getenv("CITY_GCP_ZONE"),
		GitHubRepo: os.Getenv("CITY_GITHUB_REPO"),
		ResultsDir: os.Getenv("CITY_RESULTS_DIR"),
	}

	if c.ResultsDir == "" {
		c.ResultsDir = filepath.Join(".", "results")
	}

	return c, nil
}

// Validate checks that the configuration contains the values required for
// cloud operations. Local-only usage (version, doctor) does not require
// validation.
func (c *Config) Validate() error {
	var missing []string

	if c.GCPProject == "" {
		missing = append(missing, "CITY_GCP_PROJECT")
	}
	if c.GCPRegion == "" {
		missing = append(missing, "CITY_GCP_REGION")
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
