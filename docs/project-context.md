# Durable project context

GARY stores project context under `gary/projects/<project-id>/` by default:

```text
project.json
state.json
source/MASTER_PACKET.md
manuals/project-manual-v001.json
amendments/amendment-001.md
jobs/
evidence/
feedback/events.jsonl
archive/
```

The source packet is written before the Yardmaster ingestion call. Its SHA-256
is checked on every context load. Manuals are immutable versioned snapshots and
must carry source packet provenance for their requirements and conflicts.

`gary project resume <project-id>` performs a fresh validated rehydration and
returns the project to planning without launching a worker. A project-backed
`gary master` run rehydrates before every Yardmaster wave and rebuilds the
narrow child execution context from the current manual before each worker
submission.

## Yardmaster project board

GARY also keeps a durable project board at
`gary/projects/board/projects.json`. It is an aggregate lifecycle list, not a
replacement for project state or worker evidence. A project is written to the
board as `IN_PROCESS` when its durable project record is created. The only
transition to `COMPLETED` is the Yardmaster's project-level acceptance path;
worker success, a paused bounded slice, a retry, or a recoverable failure never
completes the board entry. The transition is monotonic, so stale start or retry
paths cannot resurrect a completed project.

The board is visible with:

```powershell
go run ./cmd/gary project board
go run ./cmd/gary project board --json
```

The generated project manual carries the same `board_policy` invariant. This
keeps the lifecycle rule in the Yardmaster's working context while leaving the
immutable packet unchanged.

## Parked project auto-wake

`gary project tick <project-id>` is a thin deterministic watchdog operation.
It performs no model call while the Yardmaster is `RUNNING`, the project is
finished, work is absent, or the project is legitimately waiting/blocked. For
an unfinished parked project with explicit executable work it atomically claims
one generation, records requested/started/completed or failed timestamps, and
launches the existing master manifest path. It never edits an in-flight worker
packet. No-progress wakes use bounded backoff and a small circuit breaker; a
human can inspect the recorded state before deciding how to clear or repair it.

Project context is deliberately separate from execution history:

- the master packet and manual describe what the project is and what must be
  true;
- `state.json` describes current phase and progress;
- master status and worker results describe what GARY tried and observed.
- review events record explicit human observations and scoped interventions;
  they are separate from the packet/manual and are append-only.

Project context is treated as untrusted input. It cannot override GARY's
authority, sandbox, provider, acceptance-command, or deterministic routing
controls.

## Conduct Review

Questions and interventions are separate operations:

```powershell
go run ./cmd/gary project explain <project-id> --master-id <master-id> --json
go run ./cmd/gary project feedback <project-id> --scope note --text "..."
```

`explain` performs no writes and reports only observable evidence. Feedback
scope is one of `note`, `project_preference`, `project_correction`, or
`propose_doctrine`. Notes and doctrine proposals are recorded only. Preferences
and corrections are initially `PENDING_SAFE_BOUNDARY`; the master applies them
as immutable application events only after setting project state to `PLANNING`,
then reloads the context before calling the Yardmaster. The worker packet is
constructed from the context captured for its wave and is never edited by a
review event. Doctrine proposals are never promoted automatically.

The design follows a small set of external principles: define human and AI
oversight responsibilities and monitor outcomes over the lifecycle (the
[NIST AI RMF](https://www.nist.gov/itl/ai-risk-management-framework)); use an
append-only event trail when auditability and historical reconstruction justify
it (the [event-sourcing pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing));
and keep the human's review context and intervention authority explicit rather
than treating a nominal human-in-the-loop as sufficient.
