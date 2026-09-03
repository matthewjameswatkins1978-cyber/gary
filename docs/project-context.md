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
