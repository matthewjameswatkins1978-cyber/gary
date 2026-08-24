# GARY

**General Agent Routing Yard**

GARY routes small implementation jobs to AI workers, verifies progress,
controls retries and escalation, and collects cost/performance statistics.

## Quick start

```powershell
go run ./cmd/gary version
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

Build 2A complete. Do not put credentials in this repository; copy
`.env.example` to `.env` when configuration is later introduced.
