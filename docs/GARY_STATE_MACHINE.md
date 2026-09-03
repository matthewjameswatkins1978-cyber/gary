# GARY STATE MACHINE

## Canonical project lifecycle

This document defines the authoritative lifecycle model for GARY.

It is normative. Code, schemas, workflows, watchdogs, board projections, and tests must conform to it.

## 1. Two different things: lifecycle and activity

A project has exactly one **lifecycle state**:

- `ACTIVE`
- `PARKED`
- `WAITING`
- `BLOCKED`
- `ACCEPTED`
- `CANCELLED`

An `ACTIVE` project may simultaneously contain several **activities**:

- `PLANNING`
- `EXECUTING`
- `VERIFYING`
- `INTEGRATING`

Activities are not mutually exclusive project states. Worker A may still be executing while Worker B is being verified and Worker C is being integrated.

`ARCHIVED` is not a lifecycle state. Archiving is storage/visibility metadata.

## 2. Lifecycle meanings

### ACTIVE
The Yardmaster may plan, execute, verify, and integrate work. One or more activities may be present.

### PARKED
The project is unfinished and resumable, but Yardmaster execution is not currently active. Executable work may remain.

### WAITING
Progress requires a named external condition or authorised human input. The condition must be recorded explicitly.

### BLOCKED
A genuine hard boundary prevents responsible continuation under current authority or resources. The blocker and unblock condition must be recorded.

### ACCEPTED
Project-level acceptance passed. This is successful terminal completion.

### CANCELLED
The project was explicitly closed without acceptance. This is terminal but not successful completion.

## 3. Board projection

The project board is a projection, not a second lifecycle:

- `ACTIVE | PARKED | WAITING | BLOCKED -> IN_PROCESS`
- `ACCEPTED -> COMPLETED`
- `CANCELLED -> CANCELLED`

Archiving does not alter the outcome.

## 4. Activities

Activities may be added or removed while lifecycle is `ACTIVE`.

Typical examples:

- planning wave: `PLANNING`
- workers live: `EXECUTING`
- one or more candidates under trusted review: `VERIFYING`
- accepted work being reconciled into staging: `INTEGRATING`

The lifecycle remains `ACTIVE` throughout.

## 5. Transition ownership

Only trusted Yardmaster/control-plane logic may mutate lifecycle state.

Workers report evidence. Infrastructure events trigger reassessment. Human input may authorise a change. None of these directly owns lifecycle mutation.

## 6. Legal lifecycle transitions

### ACTIVE
May transition to `PARKED`, `WAITING`, `BLOCKED`, `ACCEPTED`, or `CANCELLED`.

### PARKED
May transition to `ACTIVE`, `WAITING`, `BLOCKED`, or `CANCELLED`.

A watchdog wake must transition `PARKED -> ACTIVE` and then enter normal Yardmaster planning. It never dispatches a Worker directly.

### WAITING
May transition to `ACTIVE` when its recorded condition is satisfied, to `BLOCKED` if the condition becomes a hard barrier, or to `CANCELLED`.

### BLOCKED
May transition to `ACTIVE` when the blocker is resolved, or to `CANCELLED`.

### ACCEPTED
No normal lifecycle transition is allowed.

### CANCELLED
No normal lifecycle transition is allowed.

A future explicit reopen feature, if ever added, must be a separately authorised operation and must create a new lifecycle generation rather than allowing stale execution to revive a terminal project.

## 7. Failure is not a project state

Worker failure, timeout, Batch failure, verification failure, integration conflict, budget interruption, telemetry failure, and retry exhaustion are evidence about stages of work.

There is no routine lifecycle `FAILED` state.

The Yardmaster responds by repairing, replanning, parking, waiting, blocking, continuing independent work, or cancelling explicitly.

## 8. Lifecycle is an append-only event stream

Lifecycle changes are durable append-only events.

A canonical transition records at least:

```text
schema
event_id
event_digest
previous_event_digest
project_id
generation
previous_state
new_state
actor
reason
evidence_refs
created_at
```

The current `project.json`, `state.json`, project board, and dashboards are projections of this history.

They are not independent competing sources of lifecycle truth.

## 9. Generation and stale-write protection

Each accepted lifecycle transition increments the project generation.

A writer must compare against the expected generation before committing a transition.

A stale writer cannot overwrite newer state merely because its timestamp is later.

Terminal truth always defeats stale active truth.

## 10. Safe planning boundary

Execution-changing feedback may apply only while lifecycle is `ACTIVE` and the Yardmaster is at a safe `PLANNING` activity boundary.

A safe boundary means current evidence has been recorded and no running Worker packet is being rewritten underneath the Worker.

## 11. Worker dispatch

New Workers may be dispatched only for `ACTIVE` projects.

Do not dispatch new Workers while `PARKED`, `WAITING`, `BLOCKED`, `ACCEPTED`, or `CANCELLED`.

Late Worker output after a terminal outcome may be preserved for audit but cannot alter project lifecycle.

## 12. Verification and integration

A Worker completion creates a candidate.

Trusted verification and Yardmaster inspection are activities inside `ACTIVE`, not project outcomes.

Accepted child work enters controlled integration before dependent work relies on it.

An integration conflict remains an integration/reconciliation problem, not a revocation of the child's prior acceptance against its original base.

## 13. Parking and waking

A project may enter `PARKED` only when:

- it is unfinished;
- current Yardmaster execution has safely stopped;
- durable state is sufficient for resumption;
- any live ownership is known and recorded.

A watchdog may claim a wake only when:

- lifecycle is `PARKED`;
- explicit executable work exists;
- no Yardmaster is active;
- no equivalent wake generation is active;
- the project is not actually waiting or blocked.

Wake acquisition must be atomic.

Repeated wakes without meaningful progress use bounded backoff and a circuit breaker.

## 14. WAITING

`WAITING` requires a named condition, for example:

```text
waiting_for:
  type: human_authority
  description: approval to deploy production
```

or:

```text
waiting_for:
  type: external_event
  description: upstream service becomes available
```

Do not pollute WAITING with repeated unchanged retries.

## 15. BLOCKED

`BLOCKED` requires a real hard boundary and evidence.

Record:

- what is blocked;
- which boundary prevents continuation;
- evidence;
- what would unblock it;
- whether human authority is required.

Do not use BLOCKED as a synonym for difficult, inconvenient, missing an installable tool, first implementation failure, Worker confusion, or slow CI.

## 16. Archiving

Archiving is metadata.

An accepted or cancelled project may be archived for storage/visibility purposes without changing its lifecycle outcome.

The archive record should contain `archived_at` and optional archive metadata.

## 17. Minimum projected state

A current state projection should contain at least:

```text
project_id
lifecycle_state
generation
activities
updated_at
updated_by
reason
last_event_id
```

Where relevant also record:

```text
active_master_run
active_workers
waiting_for
blocker
wake_generation
last_progress_at
accepted_at
cancelled_at
archived_at
```

## 18. Invariants

The implementation must enforce:

1. One canonical lifecycle exists.
2. Lifecycle and activity are separate concepts.
3. Only trusted control logic changes lifecycle.
4. Workers never complete projects.
5. ACCEPTED and CANCELLED are terminal.
6. Archiving is metadata, not an outcome.
7. PARKED wake always returns through ACTIVE planning.
8. WAITING requires an explicit external condition.
9. BLOCKED requires a genuine hard boundary.
10. Candidate verification is separate from Worker completion.
11. Integration is separate from Worker execution.
12. Child acceptance is separate from project acceptance.
13. Lifecycle transitions are append-only durable events.
14. Generation/CAS protection rejects stale writers.
15. Board and state files are projections of lifecycle history.

## 19. Required tests

At minimum test:

- every legal transition;
- every illegal transition;
- concurrent activities under ACTIVE;
- terminal resurrection attempts;
- stale generation writes;
- simultaneous wake claims;
- wake while Yardmaster is active;
- wake from WAITING and BLOCKED;
- Worker completion after ACCEPTED or CANCELLED;
- Yardmaster small repair followed by re-verification;
- integration conflict;
- project acceptance after child integration;
- cancellation board projection;
- archiving without lifecycle change;
- board projection consistency.

## Short form

**Lifecycle says whether the project can continue.  
Activities say what GARY is doing inside an active project.  
Every lifecycle change is a durable event.  
Only ACCEPTED means success.  
CANCELLED means deliberately closed without success.  
Archiving is just storage.**
