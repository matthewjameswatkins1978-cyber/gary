# Build 2E — Worker Benchmark: Managed Agent vs Batch CLI

**Date:** 2026-08-24

## Goal

Run the same frozen coding task through two Google-native worker routes and measure them objectively.

## Task

Fix `clamp.go` so 7 existing tests pass. Do not modify `clamp_test.go`. Create `GARY_AGENT_RESULT.txt` containing `GARY_AGENT_SMOKE_OK`.

## Route A — Managed Agent

- **Agent:** antigravity-preview-05-2026
- **Model:** gemini-3.5-flash-lite
- **Thinking:** low
- **Token ceiling:** 250,000
- **Interaction ID:** v1_Chd3RS1NYXJ1V005RzBrZFVQbWVDVXFRdxIXd0UtTWFydVdNOUcwa2RVUG1lQ1VxUXc
- **Environment ID:** 05905071a947fcde059b2027a44e645a
- **Status:** completed
- **Duration:** 208s

### Token Usage

| Metric | Value |
|--------|-------|
| Input tokens | 149,350 |
| Cached tokens | 94,208 |
| Output tokens | 3,017 |
| Thought tokens | 3,716 |
| Total tokens | 156,083 |

### Result Transfer

- Environment snapshot: 252,224,168 bytes (~252 MB TAR)
- Download/extraction: ~62s
- Format: plain TAR (streaming extraction)

### Acceptance

- Tests unchanged: YES
- Marker verified: YES
- Local tests pass: YES

## Route B — Batch + Gemini CLI

- **Runtime:** Google Cloud Batch (e2-small, europe-west2)
- **Gemini CLI version:** 0.56.0 (pinned in Docker image)
- **Model:** gemini-3.5-flash-lite
- **Approval mode:** YOLO (auto-approve all tools)
- **Image:** europe-west2-docker.pkg.dev/gary-agent-yard/gary-workers/gary-batch-worker:latest
- **Job:** gary-benchmark-20260824-201342
- **Status:** SUCCEEDED

### Tool Usage

| Tool | Calls | Success | Fail |
|------|-------|---------|------|
| read_file | 3 | 3 | 0 |
| run_shell_command | 2 | 2 | 0 |
| write_file | 2 | 2 | 0 |
| update_topic | 3 | 3 | 0 |
| **Total** | **9** | **9** | **0** |

### Token Usage

| Metric | Value |
|--------|-------|
| Input tokens | 89,647 |
| Cached tokens | 0 |
| Output tokens | 525 |
| Total tokens | 90,172 |

### Result Transfer

- No large snapshot
- Results available via Cloud Logging only
- Transfer size: ~2 KB (log output)
- GARY independently verified by running local tests

### Acceptance

- Tests unchanged: YES (verified by GARY locally)
- Marker verified: YES
- Local tests pass: YES

### Finding

Gemini CLI headless mode requires `--yolo` flag to enable file editing tools. Without it, only read-only tools (`read_file`, `grep_search`, `glob`) are available. With `--yolo`, all tools work: `read_file`, `run_shell_command`, `write_file`, `update_topic`.

## Comparison

| Metric | Managed Agent | Batch CLI |
|--------|--------------|-----------|
| Accepted | YES | YES |
| Wall time | 208s | 78s |
| Total tokens | 156,083 | 90,172 |
| Estimated cost | $0.0068 | $0.0069 |
| Result transfer | 252 MB | 2 KB |
| Human intervention | 0m | 0m |
| Same-model fairness | YES | YES |
| Actual models | gemini-3.5-flash-lite | gemini-3.5-flash-lite |

### Evidence Summary

- **FASTER:** Batch CLI (78s vs 208s)
- **FEWER TOKENS:** Batch CLI (90,172 vs 156,083)
- **CHEAPER:** Managed Agent ($0.0068 vs $0.0069) — negligible difference
- **LESS TRANSFER:** Batch CLI (2 KB vs 252 MB)

## Infrastructure

### Created

- Artifact Registry repository: `gary-workers` (docker, europe-west2)
- GCS bucket: `gary-batch-results-418146339023` (europe-west2, 7-day lifecycle, public prevention, uniform access)
- Docker image: Go 1.24.0, Node 20, Gemini CLI 0.56.0, git, bash
- APIs enabled: artifactregistry.googleapis.com, cloudbuild.googleapis.com

### Persistent Resources

- Artifact Registry repo (tiny storage)
- GCS bucket (auto-expires objects after 7 days)
- Docker image in Artifact Registry

### No Secrets Persisted

- GEMINI_API_KEY passed at job submission time only
- Not stored in image, config files, or Git

## Cost Model

Configurable local price table (Gemini 3.5 Flash-Lite standard pricing):

| Type | Price per token |
|------|----------------|
| Input | $0.000000075 |
| Output | $0.0000003 |
| Cached | $0.00000001875 |

## Code

- `internal/benchmark/benchmark.go` — Record types, price table, cost estimation
- `internal/benchmark/benchmark_test.go` — Cost calculation tests
- `cmd/gary/benchmark.go` — `gary benchmark` CLI command
- `cmd/gary/main.go` — registered benchmark command

## CLI Usage

```powershell
gary benchmark
```

Outputs formatted comparison table with evidence-based summary.

## Verification

- `gofmt` — pass
- `go vet ./...` — pass
- `staticcheck ./...` — pass
- `go test ./...` — all packages pass

## Note

This document does not pick GARY's permanent default worker. That decision is Build 2F.
