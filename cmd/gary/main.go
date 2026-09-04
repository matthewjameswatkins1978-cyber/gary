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
	case "work":
		cmdWork()
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
	fmt.Fprintln(os.Stderr, "  work <job.json>  Give one strong agent one whole job")
	fmt.Fprintln(os.Stderr, "  status           Show recorded work status")
	fmt.Fprintln(os.Stderr, "  cost             Show recorded job cost")
	fmt.Fprintln(os.Stderr, "  doctor           Check environment readiness")
	fmt.Fprintln(os.Stderr, "  smoke            Test the Google Cloud Batch boundary")
	fmt.Fprintln(os.Stderr, "  route            Decision-only Telltail adapter; launches nothing")
	fmt.Fprintln(os.Stderr, "  version          Print version information")
	fmt.Fprintln(os.Stderr, "  help             Show this help message")
}
