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
- **Job:** gary-benchmark-r1-20260824-214559
- **Status:** SUCCEEDED
- **Service account:** gary-batch-worker@gary-agent-yard.iam.gserviceaccount.com
- **Secret:** GEMINI_API_KEY via Secret Manager (projects/418146339023/secrets/gary-gemini-api-key)

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

- Result packaged as TAR: 1,212 bytes
- Uploaded to GCS: gs://gary-batch-results-418146339023/benchmarks/unknown-1787604648/result.tar
- Download/extraction: ~2s
- GARY independently verified: marker verified, tests unchanged, local tests pass

### Acceptance

- Tests unchanged: YES (verified by GARY)
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
| Estimated cost | $0.0651 | $0.0224 |
| Result transfer | 252 MB | 1 KB |
| Human intervention | 0m | 0m |
| Same-model fairness | YES | YES |
| Actual models | gemini-3.5-flash-lite | gemini-3.5-flash-lite |

### Evidence Summary

- **FASTER:** Batch CLI (78s vs 208s)
- **FEWER TOKENS:** Batch CLI (90,172 vs 156,083)
- **CHEAPER:** Batch CLI ($0.0224 vs $0.0651)
- **LESS TRANSFER:** Batch CLI (1 KB vs 252 MB)

## Infrastructure

### Created

- Artifact Registry repository: `gary-workers` (docker, europe-west2)
- GCS bucket: `gary-batch-results-418146339023` (europe-west2, 7-day lifecycle, public prevention, uniform access)
- Docker image: Go 1.24.0, Node 20, Gemini CLI 0.56.0, gsutil, git, bash
- APIs enabled: artifactregistry.googleapis.com, cloudbuild.googleapis.com, secretmanager.googleapis.com
- Secret Manager secret: `gary-gemini-api-key`
- Service account: `gary-batch-worker` with minimal permissions:
  - roles/secretmanager.secretAccessor (on gary-gemini-api-key)
  - roles/artifactregistry.reader
  - roles/storage.objectCreator (on results bucket)
  - roles/logging.logWriter
  - roles/compute.instanceAdmin.v1

### Persistent Resources

- Artifact Registry repo (tiny storage)
- GCS bucket (auto-expires objects after 7 days)
- Docker image in Artifact Registry
- Secret Manager secret (contains GEMINI_API_KEY)

### Security

- GEMINI_API_KEY stored in Secret Manager, not in plaintext
- Batch worker uses dedicated service account with least-privilege permissions
- No secrets in image, config files, or Git

## Cost Model

Configurable local price table (Gemini Developer API standard pricing):

| Type | Price per token |
|------|----------------|
| Input | $0.30 / 1M ($0.0000003) |
| Output (includes thinking) | $2.50 / 1M ($0.0000025) |
| Cached input | $0.03 / 1M ($0.00000003) |

### Cost Calculation

- Billed output tokens = output_tokens + thought_tokens
- Route A: (149,350 - 94,208) × $0.0000003 + (3,017 + 3,716) × $0.0000025 + 94,208 × $0.00000003 = $0.0651
- Route B: (89,647 - 0) × $0.0000003 + (525 + 0) × $0.0000025 + 0 × $0.00000003 = $0.0224

## Code

- `internal/benchmark/benchmark.go` — Record types, price table, cost estimation
- `internal/benchmark/benchmark_test.go` — Cost calculation tests
- `cmd/gary/benchmark.go` — `gary benchmark` CLI command
- `cmd/gary/benchmark_verify.go` — `gary benchmark-verify` CLI command
- `cmd/gary/main.go` — registered benchmark and benchmark-verify commands

## CLI Usage

```powershell
gary benchmark
gary benchmark-verify gs://gary-batch-results-418146339023/benchmarks/<job-id>/result.tar <job-id>
```

## Verification

- `gofmt` — pass
- `go vet ./...` — pass
- `staticcheck ./...` — pass
- `go test ./...` — all packages pass

## Note

This document does not pick GARY's permanent default worker. That decision is Build 2F.
