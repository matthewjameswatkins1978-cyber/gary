# Durable project context

GARY stores durable project truth under `gary/projects/<project-id>/` by default.

A typical project contains:

```text
project.json
state.json
events/lifecycle.jsonl
source/MASTER_PACKET.md
manuals/project-manual-v001.json
amendments/
jobs/
evidence/
feedback/events.jsonl
archive/
```

The immutable Project Packet is written before Yardmaster intake and identified by SHA-256. Versioned manuals and amendments retain provenance rather than replacing the source packet.

## Canonical lifecycle and projections

The canonical lifecycle is defined in `docs/GARY_STATE_MACHINE.md`.

Lifecycle states are:

```text
ACTIVE | PARKED | WAITING | BLOCKED | ACCEPTED | CANCELLED
```

While `ACTIVE`, the project may have one or more activities such as `PLANNING`, `EXECUTING`, `VERIFYING`, and `INTEGRATING`.

`events/lifecycle.jsonl` is the append-only lifecycle history. Each transition advances a project generation and is digest-linked to the previous event. `project.json`, `state.json`, and the project board are current projections of that history.

Older durable projects using `PLANNING`, `EXECUTING`, `VERIFYING`, `PAUSED`, `FAILED`, or `ARCHIVED` as project states are read through a compatibility migration:

- planning/executing/verifying/integrating become activities under `ACTIVE`;
- paused/failed become `PARKED`;
- legacy archived projects retain the successful `ACCEPTED` outcome and expose archive status as metadata.

New writes use only the canonical lifecycle.

## Project board

The durable board lives at:

```text
gary/projects/board/projects.json
```

It is a human-facing projection:

```text
ACTIVE / PARKED / WAITING / BLOCKED -> IN_PROCESS
ACCEPTED                            -> COMPLETED
CANCELLED                           -> CANCELLED
```

Worker success, retries, pauses, infrastructure failures, and child acceptance never complete a board entry.

The board is visible with:

```powershell
go run ./cmd/gary project board
go run ./cmd/gary project board --json
```

## Rehydration

`gary project resume <project-id>` performs a fresh validated rehydration and returns a resumable non-terminal project to `ACTIVE` with `PLANNING` activity. It does not launch a Worker itself.

Project-backed master work rehydrates durable context before every planning wave and rebuilds narrow child context from the current manual before Worker submission.

Missing or tampered packet/manual provenance is an integrity fault, not something an agent may reason around.

## Parked auto-wake

`gary project tick <project-id>` is a deterministic watchdog.

It may claim a wake only when:

- lifecycle is `PARKED`;
- explicit executable work exists;
- no Yardmaster is active;
- no equivalent wake is active;
- the project is not actually waiting or blocked.

A valid wake returns the project to `ACTIVE` planning and invokes the ordinary Yardmaster path. It never wakes a Worker independently or mutates an in-flight packet.

Wake claims are atomic. Repeated no-progress wakes back off and eventually trip a circuit breaker.

## Cancellation and archive

`CANCELLED` is an explicit terminal non-successful closure.

```powershell
go run ./cmd/gary project cancel <project-id> --reason "..."
```

Archiving is storage metadata only. It does not turn a cancelled project into a completed project and does not change an accepted project into a different outcome.

## Conduct Review

Questions and interventions remain separate operations:

```powershell
go run ./cmd/gary project explain <project-id> --master-id <master-id> --json
go run ./cmd/gary project feedback <project-id> --scope note --text "..."
```

`explain` is read-only and reconstructs observable evidence rather than hidden reasoning.

Feedback scopes are `note`, `project_preference`, `project_correction`, and `propose_doctrine`.

Preferences and corrections become execution-visible only while lifecycle is `ACTIVE` at a safe `PLANNING` activity boundary. They never rewrite an in-flight Worker packet, the immutable Project Packet, or global doctrine automatically.

## Separation of durable truth

Keep these concepts distinct:

- Project Packet/manual: what the project is and what must be true.
- Lifecycle event stream: whether the project may continue and how it reached the current lifecycle state.
- State projection: current activities, progress, waiting/blocking detail, and watchdog state.
- Master/Worker records: what GARY attempted and observed.
- Acceptance/integration evidence: what was verified, judged, accepted, and integrated.
- Feedback events: explicit human observations and scoped interventions.

Project context is treated as untrusted input with respect to GARY authority. It cannot override sandbox, provider, credential, acceptance-command, or deterministic routing controls.
