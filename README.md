# City Computer

CLI-only Google Cloud Batch AI build farm.

## Build 1: Local Foundation

This build provides the local CLI, configuration, environment checks, and tests.
Cloud job submission is not yet implemented.

### Quick start

Print version information:

```powershell
go run ./cmd/city version
```

Check environment readiness:

```powershell
go run ./cmd/city doctor
```

Run tests:

```powershell
go test ./...
```

### Configuration

Set environment variables in PowerShell:

```powershell
$env:CITY_GCP_PROJECT = "your-project-id"
$env:CITY_GCP_REGION = "us-central1"
$env:CITY_GCP_ZONE = "us-central1-a"
$env:CITY_GITHUB_REPO = "owner/repo"
$env:CITY_RESULTS_DIR = ".\results"
```

Or copy `.env.example` to `.env` and source it. See `.env.example` for all
supported variables.

| Variable | Required | Description |
|---|---|---|
| `CITY_GCP_PROJECT` | Yes (cloud ops) | Google Cloud project ID |
| `CITY_GCP_REGION` | Yes (cloud ops) | Google Cloud region |
| `CITY_GCP_ZONE` | No | Google Cloud zone |
| `CITY_GITHUB_REPO` | No | GitHub repository (owner/repo) |
| `CITY_RESULTS_DIR` | No | Local results directory (default: ./results) |

### Layout

```text
cmd/city/             CLI entry point
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

### Planned commands

- `city prepare` -- validate a project dossier and job definitions.
- `city launch` -- submit a bounded batch run.
- `city oversee` -- review worker outcomes and choose a next action.
- `city report` -- print run and cost summaries.

### Status

Build 1 complete. Do not put credentials in this repository; copy `.env.example`
to `.env` when configuration is later introduced.
