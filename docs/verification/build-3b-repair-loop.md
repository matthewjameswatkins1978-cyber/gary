# Build 3B — Repair Loop

## What Changed

- `gary run` now retries up to 3 attempts with repair briefs
- After failed acceptance, GARY downloads patch + acceptance output
- Gemini generates a compact repair brief (cause/keep/change/do_not_change/next)
- Progress detection prevents circling
- After 3 low-progress attempts, escalates to Closer (configurable `GARY_CLOSER_MODEL`)
- Worker captures acceptance output to `acceptance_output.txt` alongside `result.patch`
- Default machine type: `e2-small` (was `e2-standard-4`)
- `RunResult` extended: `attempts`, `repair_count`, `escalated`, `closer_used`, `final_status`

## Live Test

- **Job:** fix-clamp
- **Repository:** gary-test-clamp
- **Attempts:** 1 (worker succeeded on first try)
- **Result:** DONE
- **Branch:** gary/fix-clamp-85dcb40a
- **Commit:** f4b92cb
- **Duration:** 263s
- **Closer:** not invoked
- **Master unchanged:** YES

## Unit Tests

- `TestSummarizeAcceptance` — 4 cases (empty, all pass, some fail, no ok lines)
- `TestCheckProgress` — 6 cases (first attempt, fewer/same/more failures, pass after fail, different output)
- `TestFormatRepairBrief` — output formatting

## Repair Loop Logic

1. Attempt 1: normal task
2. If Batch fails or local acceptance fails:
   - Download patch and acceptance output from GCS
   - Generate repair brief via Gemini
   - Append failure evidence to task for next attempt
3. Progress check:
   - Fewer failures = progress
   - Different failure messages = progress
   - Same failures = no progress
4. After 3 low-progress attempts: escalate to Closer
5. Closer gets all accumulated context and strongest model

## Parked State

- No active Batch jobs
- No compute instances running
- GCS retains attempt artifacts (7-day lifecycle)
