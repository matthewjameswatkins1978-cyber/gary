package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matthewjameswatkins1978-cyber/gary/internal/batch"
	"github.com/matthewjameswatkins1978-cyber/gary/internal/config"
)

func cmdSmoke() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	if cfg.GCPProject == "" {
		cfg.GCPProject = discoverProject()
	}
	if cfg.GCPProject == "" {
		fmt.Fprintln(os.Stderr, "no GCP project: set GARY_GCP_PROJECT or run gcloud config set project ...")
		os.Exit(1)
	}

	if cfg.GCPRegion == "" {
		cfg.GCPRegion = "europe-west2"
	}

	heartbeat := fmt.Sprintf("GARY_HEARTBEAT_%d", time.Now().UnixNano())
	jobName := fmt.Sprintf("gary-smoke-%s", time.Now().Format("20060102-150405"))

	fmt.Println("GARY // SMOKE")
	fmt.Println()
	fmt.Printf("Project   %s\n", cfg.GCPProject)
	fmt.Printf("Region    %s\n", cfg.GCPRegion)
	fmt.Printf("Job       %s\n", jobName)
	fmt.Printf("Machine   e2-small\n")
	fmt.Printf("Heartbeat %s\n", heartbeat)
	fmt.Println()

	batchCfg := batch.JobConfig{
		Project:      cfg.GCPProject,
		Region:       cfg.GCPRegion,
		JobName:      jobName,
		Heartbeat:    heartbeat,
		MachineType:  "e2-small",
		MaxRunDurSec: 300,
		MaxRetries:   0,
	}

	submittedAt := time.Now()
	fmt.Println("SUBMITTED")

	jobID, err := batch.SubmitJob(batchCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "submission failed: %v\n", err)
		os.Exit(1)
	}
	_ = jobID

	fmt.Println()
	fmt.Println("POLLING")

	status, finishedAt, err := batch.WaitForTerminal(
		cfg.GCPProject, cfg.GCPRegion, jobName,
		5*time.Second, 10*time.Minute,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "polling failed: %v\n", err)
		os.Exit(1)
	}

	if status.State != "SUCCEEDED" {
		fmt.Fprintf(os.Stderr, "job ended with state: %s\n", status.State)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("VERIFYING HEARTBEAT")

	var heartbeatVerified bool
	for attempt := 0; attempt < 12; attempt++ {
		_, err := batch.QueryLogs(cfg.GCPProject, status.UID, heartbeat)
		if err == nil {
			heartbeatVerified = true
			fmt.Println("HEARTBEAT VERIFIED")
			break
		}
		if attempt < 11 {
			time.Sleep(5 * time.Second)
		}
	}

	if !heartbeatVerified {
		fmt.Fprintln(os.Stderr, "heartbeat could not be verified")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("RECORDING RESULT")

	result := batch.SmokeResult{
		Kind:              "gary-smoke",
		JobID:             jobName,
		JobUID:            status.UID,
		Project:           cfg.GCPProject,
		Region:            cfg.GCPRegion,
		MachineType:       "e2-small",
		ProvisioningModel: "STANDARD",
		SubmittedAt:       submittedAt.Format(time.RFC3339),
		FinishedAt:        finishedAt.Format(time.RFC3339),
		DurationSeconds:   finishedAt.Sub(submittedAt).Seconds(),
		FinalState:        status.State,
		Heartbeat:         heartbeat,
		HeartbeatVerified: heartbeatVerified,
	}

	if err := saveSmokeResult(cfg.ResultsDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "save result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("DONE")
	fmt.Printf("Duration %.0fs\n", result.DurationSeconds)
}

func discoverProject() string {
	out, err := runCmd("gcloud", "config", "get-value", "project")
	if err != nil {
		return ""
	}
	return out
}

func saveSmokeResult(resultsDir string, result batch.SmokeResult) error {
	dir := filepath.Join(resultsDir, "smoke")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("smoke-%s.json", result.JobID)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
