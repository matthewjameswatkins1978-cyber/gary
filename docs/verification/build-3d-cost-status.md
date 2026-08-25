# Build 3D — Cost Guard + Status

## What Changed

- `GARY_MAX_COST_USD` optional per-run budget limit
- JSONL cost ledger at `results/cost-ledger.jsonl`
- `gary status` command with cost totals + PARKED detection
- Cumulative tracking: run, month-to-date, lifetime
- Budget check prevents new work when limit exceeded
- Running spend printed during run-many
- All costs labelled ESTIMATED

## Cost Estimation

- Model: ~15K input + 7.5K output tokens per attempt (Gemini flash-lite pricing)
- Compute: e2-small at $0.0207/hr
- Conservative, labelled ESTIMATED

## Ledger Format

Each line: timestamp, job_id, status, attempts, model, model/compute/total cost, duration, branch, commit

## Live Check

- **Job:** fix-clamp
- **Branch:** gary/fix-clamp-a91bfef5
- **Commit:** bb24e02
- **Duration:** 303s
- **Estimated cost:** $0.0251
- **Ledger:** results/cost-ledger.jsonl written
- **Status:** PARKED
- **Active jobs:** 0

## Unit Tests

- Ledger append/read/empty
- Budget check: unset, within limit, exceeds limit
- Running work not killed by budget
- BUDGET_STOPPED status
- Cost summarization (run/month/lifetime)
- Cost estimation
- Ledger entry creation
