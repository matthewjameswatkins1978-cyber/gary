# GARY

**General Agent Routing Yard**

GARY turns project plans into bounded implementation work for disposable AI workers, preserves durable project context, supervises progress from observable evidence, controls cost and authority, integrates accepted work, and verifies completion at project level.

The control plane owns orchestration. Implementation workers operate in isolated repository state and return evidence-backed results. Rules protect authority, provenance, security, cost, and acceptance; the Yardmaster is otherwise expected to use judgement to get the real job finished.

## Documentation

Current documentation is organised deliberately:

- `GARY_RULEBOOK.md` — completion principle, universal boundaries, Yardmaster and Worker roles.
- `docs/YARDMASTER_OPERATING_GUIDE.md` — advisory engineering tradecraft for project setup, JobPackets, supervision, repair, integration, and completion.
- `docs/project-context.md` — durable project packet/manual/state model.
- `docs/github-app-setup.md` — short-lived GitHub result-push credentials.
- `docs/github-oidc-setup.md` — GitHub Actions to Google Cloud identity setup.
- `docs/reliability-r1.md` and `docs/verification/` — historical design/verification evidence, not current runtime doctrine.

For exact mechanics, current code, schemas, tests, and workflows are authoritative.

## Quick start

```powershell
go run ./cmd/gary version
go run ./cmd/gary doctor
go test ./...
```

The decision-only Telltail adapter consumes one `telltail.adapter.v1` request from stdin and returns one `telltail.decision.v1` response without launching a worker or changing a repository:

```powershell
Get-Content -Raw telltail-adapter-request.json | go run ./cmd/gary route --dry-run --json
```

## Configuration

Typical local/cloud configuration:

```powershell
$env:GARY_GCP_PROJECT = "your-project-id"
$env:GARY_GCP_REGION = "europe-west2"
$env:GARY_GCP_ZONE = "europe-west2-a"
$env:GARY_GITHUB_REPO = "owner/repo"
$env:GARY_RESULTS_DIR = ".\results"
$env:GARY_PROJECTS_DIR = ".\gary\projects"
```

Or copy `.env.example` to `.env` and use the values appropriate to the current environment. Never commit credentials.

| Variable | Required | Description |
|---|---|---|
| `GARY_GCP_PROJECT` | Yes for cloud operations | Google Cloud project ID |
| `GARY_GCP_REGION` | Yes for cloud operations | Google Cloud region |
| `GARY_GCP_ZONE` | No | Google Cloud zone when a path requires one |
| `GARY_GITHUB_REPO` | No | GARY control repository (`owner/repo`) |
| `GARY_RESULTS_DIR` | No | Local results directory |
| `GARY_WORKER_RUNTIME` | No | Worker runtime selection where supported |
| `GARY_PROJECTS_DIR` | No | Durable project-context root |

See `.env.example` and current configuration code for the complete set and defaults.

## Repository layout

```text
cmd/gary/             CLI and master/Yardmaster entry points
internal/config/      configuration
internal/jobs/        jobs, dossiers, and results
internal/batch/       Google Cloud Batch boundary
internal/supervisor/  supervision state/evidence
internal/project/     durable project context
internal/gitops/      repository operations
internal/githubapp/   short-lived GitHub App credentials
docker/worker/        disposable worker image
scripts/              maintenance and smoke helpers
docs/                 current guidance plus historical verification notes
gary/                 durable/control-plane project and master state
```

## Durable projects

Long master packets can be ingested into durable project context. The original packet is stored before Yardmaster execution, identified by SHA-256, and preserved. Versioned project manuals and amendments retain provenance rather than replacing the source packet.

```powershell
go run ./cmd/gary project start --id book-burner --repo https://github.com/matthewjameswatkins1978-cyber/book-burner.git --base-branch master MASTER_PACKET.md
go run ./cmd/gary project status book-burner
go run ./cmd/gary project resume book-burner
go run ./cmd/gary project amend book-burner AMENDMENT.md
go run ./cmd/gary project inspect book-burner --json
go run ./cmd/gary project explain book-burner --master-id book-burner-launch
go run ./cmd/gary project feedback book-burner --scope project_preference --text "Prefer smaller worker packets."
```

Project-backed master work rehydrates validated project context before planning and child execution. Missing or tampered provenance is an integrity problem, not something an agent is allowed to reason around.

Conduct Review is a low-friction side channel for supervision. `project explain`
is read-only and reconstructs recorded state, packet/manual provenance, master
wave plans, worker results, branches/commits, and cost telemetry; it does not
ask the model to reveal hidden reasoning. `project feedback` records explicit
append-only review events. Notes and doctrine proposals remain observational;
project preferences and corrections become visible to the Yardmaster only at
the next safe `PLANNING` boundary. No feedback changes the packet, manual,
in-flight worker packet, acceptance gate, or global doctrine automatically.

Do not put credentials in this repository; copy `.env.example` to `.env`
for local development.

## Worker model

Workers are implementation agents. They receive a bounded outcome, relevant context, repository state, constraints, and executable evidence of success. They may choose implementation tactics and ordinary development setup within their authority.

The Yardmaster should divide work into coherent outcomes, run independent work concurrently when isolation makes that safe, supervise from engineering evidence, preserve useful partial work, and replan when reality changes.

Elapsed time and raw attempt count are observations, not project-success criteria. Genuine cost, infrastructure, security, authority, and acceptance boundaries remain enforceable limits.

## Evidence and acceptance

A worker saying "done" is not acceptance, and neither is a Batch terminal label. A recoverable child candidate is checkpointed, then GARY's trusted control layer reruns the required mechanical checks on the exact candidate in a clean workspace. If those checks pass, Yardmaster performs a bounded merge-quality inspection of the actual change. Only a quality `PASS` plus trusted mechanical verification makes the child result accepted and eligible for staging integration. `SMALL_FIX`, `MAJOR_FIX`, and `REPLAN` preserve useful work while routing the appropriate correction.

Master completion is a separate gate: project-level acceptance runs against the integrated staging state, followed by independent semantic inspection where configured. Ordinary workers do not gain merge or production authority merely by completing a child job.

Structured result and master status locations are implementation details; use the current CLI and `docs/README.md` rather than historical verification notes when exact locations matter.

## Cloud heartbeat

```powershell
go run ./cmd/gary smoke
```

This exercises the configured Google Cloud Batch path and records the result. It requires the current Google Cloud project/region identity setup.

## Development

GARY is a Go project. At minimum use the repository-required Go toolchain and run:

```powershell
go test ./...
go vet ./...
```

Useful agent-development tools may include `gopls`, `goimports`, and `staticcheck`, but the repository's current tests and declared toolchain are the source of truth.

## Doctrine

**Small instructions. Clear roles. Small context. Hard evidence. Finish the work.**
