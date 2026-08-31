# GARY

**General Agent Routing Yard**

GARY routes bounded implementation jobs to disposable AI workers, verifies progress,
controls retries and cost, and keeps planning on a small observable control plane.

Reliability R1 splits coordination from labour: Yardmaster planning is one direct
Gemini API call on the control plane with a five-minute hard limit; Google Cloud
Batch is reserved for isolated implementation workers.

## Quick start

```powershell
go run ./cmd/gary version
```

The decision-only routing entrypoint consumes one `telltail.adapter.v1`
request from stdin and returns one `telltail.decision.v1` response without
launching a worker or changing a repository:

```powershell
Get-Content -Raw telltail-adapter-request.json | go run ./cmd/gary route --dry-run --json
```

```powershell
go run ./cmd/gary doctor
```

```powershell
go test ./...
```

## Configuration

Set environment variables in PowerShell:

```powershell
$env:GARY_GCP_PROJECT = "your-project-id"
$env:GARY_GCP_REGION = "us-central1"
$env:GARY_GCP_ZONE = "us-central1-a"
$env:GARY_GITHUB_REPO = "owner/repo"
$env:GARY_RESULTS_DIR = ".\results"
$env:GARY_PROJECTS_DIR = ".\gary\projects"
```

Or copy `.env.example` to `.env` and source it. See `.env.example` for all
supported variables.

| Variable | Required | Description |
|---|---|---|
| `GARY_GCP_PROJECT` | Yes (cloud ops) | Google Cloud project ID |
| `GARY_GCP_REGION` | Yes (cloud ops) | Google Cloud region |
| `GARY_GCP_ZONE` | No | Google Cloud zone |
| `GARY_GITHUB_REPO` | No | GitHub repository (owner/repo) |
| `GARY_RESULTS_DIR` | No | Local results directory (default: ./results) |
| `GARY_WORKER_RUNTIME` | No | Default worker: `batch` or `managed` (default: batch) |
| `GARY_PROJECTS_DIR` | No | Durable project context root (default: ./gary/projects) |

## Layout

```text
cmd/gary/             CLI entry point
internal/config/      configuration loading and validation
internal/jobs/        job dossier and result types
internal/batch/       Google Cloud Batch boundary
internal/overseer/    outcome-review policy
internal/report/      terminal-only reporting
internal/gitops/      worker repository operations
docker/worker/        disposable worker image
scripts/              local maintenance and smoke-test helpers
examples/smoke-test/  smallest eventual Batch run fixture
docs/                 decisions and operational notes
testdata/             stable fixtures for tests
```

## Status

Reliability R1: direct control-plane Yardmaster + Batch implementation workers.
No workflow rewrites GARY source at runtime. See `docs/reliability-r1.md`.

Terminal runs persist a structured cost record. Provider token usage is labelled
`PROVIDER_REPORTED`; Batch VM runtime is priced from the observed runtime and
labelled estimated because GARY does not query the final Cloud Billing invoice.
Historical records retain their legacy estimate semantics.

```powershell
go run ./cmd/gary cost <job-or-master-id>
```

Ordinary results are stored under `results/runs/<job-id>.json`; master results
are stored under `gary/master/status/<master-id>.json`. The GitHub-readable yard
snapshot is `control/status.json` and reports `idle` when no jobs remain active.

## Durable projects

Long master packets can be ingested into durable project context. The original
packet is stored before the Yardmaster call, identified by SHA-256, and never
rewritten. The Yardmaster produces versioned structured manuals; amendments
produce new manual versions while preserving the packet and older manuals.

```powershell
go run ./cmd/gary project start --id book-burner --repo https://github.com/matthewjameswatkins1978-cyber/book-burner.git --base-branch master MASTER_PACKET.md
go run ./cmd/gary project status book-burner
go run ./cmd/gary project resume book-burner
go run ./cmd/gary project amend book-burner AMENDMENT.md
go run ./cmd/gary project inspect book-burner --json
```

Project-backed master manifests use `project_id` instead of `brief_path`.
Every master wave and child worker rehydrates and verifies the project packet
and current manual before continuing. Missing or tampered context blocks the
operation; project acceptance is required before archival.

Do not put credentials in this repository; copy `.env.example` to `.env`
for local development.

## First Cloud Heartbeat

```powershell
go run ./cmd/gary smoke
```

Submits one tiny Google Cloud Batch job, waits for it to succeed, verifies
the heartbeat marker in Cloud Logging, and records a local result. Requires
`GARY_GCP_PROJECT` and `GARY_GCP_REGION` (or a configured `gcloud` project).

## Default Worker

Batch + Gemini CLI is the default worker (faster, lower-token, cheaper per
Build 2E benchmark). Managed Agent is available as an alternate route.

```powershell
go run ./cmd/gary worker
```

```
Default worker: batch
Model: gemini-3.5-flash-lite
```

## Go Development Tools

### Required (built into Go)

| Tool | Purpose | Notes |
|------|---------|-------|
| `go` | Go toolchain | v1.24.0+ required |
| `gofmt` | Code formatting | Built-in, runs automatically |
| `go vet` | Basic static analysis | Built-in, catches common mistakes |

### Recommended (for coding agents like opencode)

| Tool | Purpose | Install |
|------|---------|---------|
| `gopls` | Language Server Protocol (LSP) | `go install golang.org/x/tools/gopls@latest` |
| `goimports` | Import organization | `go install golang.org/x/tools/cmd/goimports@latest` |
| `staticcheck` | Advanced static analysis | `go install honnef.co/go/tools/cmd/staticcheck@latest` |

### Optional (not installed for GARY V1)

| Tool | Reason skipped |
|------|----------------|
| `delve` | Debugger not needed for V1 CLI |
| `golangci-lint` | Meta-linter overkill for small project |

### Verification

Run `gary doctor` to check all tools are available:

```powershell
go run ./cmd/gary doctor
```
