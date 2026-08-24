// Command gary is the CLI controller for GARY.
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
	case "smoke":
		cmdSmoke()
	case "agent-smoke":
		cmdAgentSmoke()
	case "benchmark":
		cmdBenchmarkWorker()
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
	fmt.Fprintln(os.Stderr, "  version      Print version information")
	fmt.Fprintln(os.Stderr, "  doctor       Check environment readiness")
	fmt.Fprintln(os.Stderr, "  smoke        Run Google Cloud Batch heartbeat smoke test")
	fmt.Fprintln(os.Stderr, "  agent-smoke  Run Gemini AI worker smoke test")
	fmt.Fprintln(os.Stderr, "  benchmark    Show worker benchmark comparison")
	fmt.Fprintln(os.Stderr, "  help         Show this help message")
}
