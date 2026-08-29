# GARY Reliability R1

## Goal

Make GARY boringly dependable before adding more autonomy.

## Architecture

```text
GitHub / human / Lucy
        |
        v
GARY control plane
  - reads master brief
  - clones current staging branch
  - builds a bounded repository snapshot
  - one direct Gemini API planning request
  - gemini-3.7-flash
  - thinking level: medium
  - hard wall-clock limit: 5 minutes
  - no client retry loop
        |
        v
validated JSON plan
        |
        v
Google Cloud Batch workers
  - cheap implementation model
  - isolated clone/branch
  - Gemini CLI internal request attempts: 1
  - Batch automatic retries: 0
  - GARY worker attempts: max 2
  - previous failure evidence supplied to attempt 2
        |
        v
local mechanical acceptance
        |
        v
staging branch for human review
```

## Rules

1. Planning never runs in Batch.
2. Planning never gets more than five minutes.
3. Network failure is reported as transport failure, not mistaken for thinking.
4. GARY owns retries. Nested tools do not run hidden retry loops.
5. Normal worker paths do not automatically invoke a Closer.
6. Per-job cost is checked before every paid worker attempt.
7. `GARY_MAX_WORKERS` is a hard ceiling.
8. Workflows run source as committed. They do not rewrite Go files at runtime.
9. Workers may use Batch. Coordination stays on the control plane.
10. A result is not merged automatically.

## Debug surfaces

The control plane is deliberately observable from GitHub:

- GitHub Actions run and step status
- Yardmaster start, finish, duration, model and wave in workflow logs
- `gary/master/status/<id>.json` for terminal master state and per-wave planner duration
- `control/status.json` for active/recent worker state
- Batch job ID on worker entries
- Batch failure log tail copied into GARY failure evidence
- worker branch and commit for accepted work

This is the supported intervention path for Lucy or a human. Debugging should not require entering a disposable worker VM.

## Failure policy

Planner transport error:
- fail the wave immediately
- preserve status
- no worker dispatch

Planner timeout:
- kill at five minutes
- preserve status
- no worker dispatch

Worker transport or Batch failure:
- capture failure evidence
- allow at most one GARY-authorised second attempt if the cost fuse permits it

Acceptance failure:
- attempt 2 receives the previous failure evidence
- if it still fails, stop and return control

Integration failure:
- preserve accepted worker branches
- fail the master job without destroying staging state

## What R1 intentionally removes

- Batch-based Yardmaster planning
- hidden Gemini CLI fetch retry loops
- three ordinary worker attempts
- paid repair-model call between ordinary attempts
- normal-path automatic Closer escalation
- runtime Python rewriting of GARY source

These features can only return if evidence shows they earn their complexity.
