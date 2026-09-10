// Command gary is the deliberately small GARY controller.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		cmdVersion()
	case "doctor":
		cmdDoctor()
	case "where":
		cmdWhere()
	case "profiles":
		cmdProfiles()
	case "work":
		cmdYardmaster()
	case "do":
		cmdDo()
	case "resume":
		cmdResume()
	case "cancel":
		cmdCancel()
	case "needs-me":
		cmdNeedsMatthew()
	case "park":
		cmdPark()
	case "unpark":
		cmdUnpark()
	case "work-batch":
		cmdWorkBatch()
	case "yardmaster":
		cmdYardmaster()
	case "route":
		// Kept for Telltail's decision-only adapter. It never launches work.
		cmdRoute()
	case "status":
		cmdStatus()
	case "cost":
		cmdCost()
	case "smoke":
		cmdSmoke()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gary <command>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  do <job>         Run one ordinary-language job through canonical GARY")
	fmt.Fprintln(os.Stderr, "  work <job.json>  Give Luna one whole local-first job")
	fmt.Fprintln(os.Stderr, "  yardmaster <job.json>  Alias for work")
	fmt.Fprintln(os.Stderr, "  resume <job-id>  Resume a persisted job after Matthew's answer")
	fmt.Fprintln(os.Stderr, "  cancel <job-id>  Cancel one job while preserving work and evidence")
	fmt.Fprintln(os.Stderr, "  needs-me         Show persisted human-only blockers")
	fmt.Fprintln(os.Stderr, "  park             Prevent new jobs")
	fmt.Fprintln(os.Stderr, "  unpark           Allow new jobs")
	fmt.Fprintln(os.Stderr, "  work-batch <job.json>  Explicit legacy Google Cloud Batch path")
	fmt.Fprintln(os.Stderr, "  status           Show recorded work status")
	fmt.Fprintln(os.Stderr, "  cost             Show recorded job cost")
	fmt.Fprintln(os.Stderr, "  doctor           Check environment readiness")
	fmt.Fprintln(os.Stderr, "  where            Show the canonical GARY executable and state root")
	fmt.Fprintln(os.Stderr, "  profiles         Show non-secret worker route inventory")
	fmt.Fprintln(os.Stderr, "  smoke            Test the optional legacy Google Cloud Batch boundary")
	fmt.Fprintln(os.Stderr, "  route            Decision-only Telltail adapter; launches nothing")
	fmt.Fprintln(os.Stderr, "  version          Print version information")
	fmt.Fprintln(os.Stderr, "  help             Show this help message")
}
