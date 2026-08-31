// Command gary is the CLI controller for GARY.
package main

import (
	"fmt"
	"os"

	"github.com/matthewjameswatkins1978-cyber/gary/control"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		cmdVersion()
	case "route":
		cmdRoute()
	case "doctor":
		cmdDoctor()
	case "run":
		requireDirectRunOverride()
		cmdRun()
	case "run-many":
		requireDirectRunOverride()
		cmdRunMany()
	case "master":
		cmdMaster()
	case "project":
		cmdProject()
	case "inbox":
		cmdInbox()
	case "issue":
		cmdIssue()
	case "status":
		cmdStatus()
	case "cost":
		cmdCost()
	case "snapshot":
		cmdSnapshot()
	case "smoke":
		cmdSmoke()
	case "agent-smoke":
		cmdAgentSmoke()
	case "benchmark":
		cmdBenchmarkWorker()
	case "benchmark-verify":
		cmdBenchmarkVerify()
	case "worker":
		cmdWorker()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func requireDirectRunOverride() {
	if os.Getenv("GARY_ALLOW_UNSAFE_DIRECT_RUN") == "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "direct run is temporarily locked because the source-level retry loop can exceed the protected workflow budget")
	fmt.Fprintln(os.Stderr, "use GARY master/inbox/control for guarded work; set GARY_ALLOW_UNSAFE_DIRECT_RUN=1 only as an explicit break-glass override")
	os.Exit(2)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gary <command>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version      Print version information")
	fmt.Fprintln(os.Stderr, "  route        Decide a route without launching a worker")
	fmt.Fprintln(os.Stderr, "  doctor       Check environment readiness")
	fmt.Fprintln(os.Stderr, "  run          Run one coding job (guarded workflows preferred)")
	fmt.Fprintln(os.Stderr, "  run-many     Run coding jobs concurrently (guarded workflows preferred)")
	fmt.Fprintln(os.Stderr, "  master       Run a large objective through the Yardmaster")
	fmt.Fprintln(os.Stderr, "  project      Durable project packet and Yardmaster manual operations")
	fmt.Fprintln(os.Stderr, "  inbox        Validate or dispatch repository inbox jobs")
	fmt.Fprintln(os.Stderr, "  issue        Execute one authenticated GitHub Issue command")
	fmt.Fprintln(os.Stderr, "  status       Show yard status and cost summary")
	fmt.Fprintln(os.Stderr, "  cost         Show the terminal cost for one job or master")
	fmt.Fprintln(os.Stderr, "  snapshot     Show GitHub-readable status snapshot")
	fmt.Fprintln(os.Stderr, "  worker       Show default worker configuration")
	fmt.Fprintln(os.Stderr, "  smoke        Run Google Cloud Batch heartbeat smoke test")
	fmt.Fprintln(os.Stderr, "  agent-smoke  Run Gemini AI worker smoke test")
	fmt.Fprintln(os.Stderr, "  benchmark    Show worker benchmark comparison")
	fmt.Fprintln(os.Stderr, "  benchmark-verify  Download and verify Batch result")
	fmt.Fprintln(os.Stderr, "  help         Show this help message")
}

func cmdSnapshot() {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		fmt.Fprintln(os.Stderr, "not in a git repository")
		os.Exit(1)
	}

	snap, err := control.ReadSnapshot(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("GARY // SNAPSHOT\n")
	fmt.Printf("Updated:  %s\n", snap.UpdatedAt)
	fmt.Printf("Yard:     %s\n", snap.YardState)
	fmt.Printf("Active:   %d\n", len(snap.ActiveJobs))
	for _, j := range snap.ActiveJobs {
		fmt.Printf("  [%s] %s attempts=%d\n", j.State, j.ID, j.Attempts)
	}
	fmt.Printf("Recent:   %d\n", len(snap.RecentJobs))
	for _, j := range snap.RecentJobs {
		if j.Cost != nil {
			fmt.Printf("  [%s] %s cost=$%.4f (%s)\n", j.State, j.ID, j.Cost.TotalCostUSD, j.Cost.PricingStatus)
		} else {
			fmt.Printf("  [%s] %s cost=$%.4f (LEGACY_ESTIMATE)\n", j.State, j.ID, j.EstCost)
		}
	}
}
