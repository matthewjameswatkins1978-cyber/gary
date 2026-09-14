# GARY Black Box

GARY Black Box is local operational memory for real GARY work. It is not a
second planner, supervisor, telemetry service, cloud scheduler, vector store,
or self-modifying rule engine.

The governing separation is:

> Job history records reality. Incidents record failures. Lessons record what
> was learned. Tests prove fixes. Rules govern future behavior.

## Evidence and storage

The existing per-job state and append-only `*.events.jsonl` file remain the
source evidence. GARY records meaningful lifecycle changes, worker dispatch and
exit, Git/worktree and acceptance facts, human boundaries, recovery, and the
terminal outcome. Duplicate event IDs are ignored and a torn final JSONL line
does not make earlier evidence unreadable.

Under the configured state root (normally `%USERPROFILE%\.gary`):

```text
jobs/<safe-job-id>.json                 durable current state
jobs/<safe-job-id>.events.jsonl         append-only forensic journal
jobs/<safe-job-id>.JOB_NOTE.md          derived human summary
jobs/<safe-job-id>/JOB_NOTE.md          derived summary for front-door jobs
blackbox/history.json                   versioned, rebuildable derived index
blackbox/incidents.jsonl                append-only incident records
blackbox/relations.jsonl                explicit related-job links
blackbox/lessons.jsonl                  evidence-backed lesson candidates
blackbox/bundles/<job-id>.json          redactable diagnostic output
```

`history.json` is disposable. `gary history rebuild` reconstructs it from job
state and raw journals; it is also rebuilt automatically by history reads. A
bad index cannot destroy the raw job evidence.

## Inspecting a job

```powershell
gary jobs
gary recent
gary history --repo C:\work\tethers --status FAILED --since 7d
gary history search "acceptance"
gary history show <job-id>
gary show <job-id>
gary review <job-id>
gary doctor --job <job-id> --bundle
```

`show` is the one-screen operational summary. `review` is analysis-oriented
and keeps genuine Matthew-level questions separate from ordinary debugging.
`doctor --job ... --bundle` writes a compact JSON bundle and prints it; the
bundle includes redacted packet/configuration/event data and the safe next
action. Very large packets are truncated with a SHA-256 reference so the
bundle remains compact. Check the bundle before sharing it outside the machine.

`gary history context --repo <repo> --limit 10` emits compact JSON for Lucy.
It is derived from durable state and journals, not from model recollection.

`gary note <job-id> <text>` appends an `OPERATOR_NOTE` event and regenerates the
summary. Notes are evidence of an operator observation, not a replacement for
state or acceptance.

## Relationships, incidents, and lessons

Relationships are explicit and require evidence:

```powershell
gary relate predecessor repair-job repair --evidence INC-0042 --reason "explicit retry"
```

Incident opening records symptom and expected behavior separately. A suspected
cause remains suspected until a closeout supplies a proven cause, fix, and
regression/live proof:

```powershell
gary incident open --job job-id --class worker-stall `
  --symptom "worker disappeared" `
  --expected "worker exits or requests Matthew" `
  --suspected "control path may have died"
gary incidents --open
gary incident close INC-XXXX `
  --proven "worker was dead while state remained RUNNING" `
  --fix "reconcile before deciding whether to retry" `
  --proof "go test ./internal/yard -run TestRecovery"
```

The append-only incident log is projected by ID, so a resolved incident does
not appear twice. Recurrence is counted only for matching recorded class and
symptom; GARY does not infer a relationship from vague similarity.

Lesson candidates keep their destination explicit:

```powershell
gary lesson job-id "Reconcile a dead worker before retrying" `
  --confidence HIGH --evidence INC-XXXX `
  --destination regression_test
```

Destinations are `code_fix`, `regression_test`, or `operational_rule`. A lesson
candidate never rewrites the rulebook or agent hints. Promotion is an explicit
repository change backed by the cited evidence.

The intended improvement loop is:

```text
failure -> evidence -> diagnosis -> smallest fix -> regression test ->
live proof -> close incident -> lesson
```

## Health and honest uncertainty

`gary health` reports counts derived from recorded jobs and incidents. It says
`INSUFFICIENT EVIDENCE` when there are too few jobs for a trend and `UNKNOWN`
when comparable time-series data is absent. It does not manufacture a success
rate or deterioration claim.

## Recovery boundary

Black Box hooks only record existing GARY actions and regenerate a view after a
durable state write. It never starts workers. Restart continues to use the
existing state, process/worktree reconciliation, duplicate-safe worker rules,
and serial integration/rollback behavior. A healthy worker is not duplicated
because the note or history index was rebuilt.
