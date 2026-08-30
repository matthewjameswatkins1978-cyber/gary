# Build 3D — Cost Guard + Status

## What Changed

- `GARY_MAX_COST_USD` optional per-run budget limit
- JSONL cost ledger at `results/cost-ledger.jsonl`
- `gary status` command with cost totals + IDLE detection
- Cumulative tracking: run, month-to-date, lifetime
- Budget check prevents new work when limit exceeded
- Running spend printed during run-many
- New records distinguish PROVIDER_REPORTED, MIXED, ESTIMATED, and legacy estimates

## Cost Accounting

- Workers preserve the pinned Gemini CLI 0.56.0 `stream-json` JSONL result as
  `usage.jsonl` and calculate model cost from provider-reported token counts.
- Yardmaster and repair responses record Vertex/Gemini `usageMetadata`, model
  version, response ID, and `x-goog-request-id` when present.
- Batch compute uses observed job runtime and the configured machine type, but
  remains estimated because GARY does not query Cloud Billing.
- Missing telemetry is explicit `ESTIMATED`; it is never silently recorded as
  zero or presented as actual provider usage.

## Ledger Format

Each line: timestamp, job_id, status, attempts, model, legacy-compatible model/compute/total fields, structured cost, duration, branch, commit

## Live Check

- **Job:** fix-clamp
- **Branch:** gary/fix-clamp-a91bfef5
- **Commit:** bb24e02
- **Duration:** 303s
- **Legacy estimate:** $0.0251
- **Ledger:** results/cost-ledger.jsonl written
- **Status:** IDLE
- **Active jobs:** 0

## Unit Tests

- Ledger append/read/empty
- Budget check: unset, within limit, exceeds limit
- Running work not killed by budget
- BUDGET_STOPPED status
- Cost summarization (run/month/lifetime)
- Structured usage parsing, pricing, aggregation, and legacy cost compatibility
- Ledger entry creation
