# GARY recovery

Each job keeps its original packet, state, worker evidence and a sparse
`*.events.jsonl` journal. Event IDs are stable and duplicate-safe; the journal
is not telemetry and has no heartbeats.

On restart GARY resumes the saved App Server thread, reconciles the real OS
processes, worktrees, Git HEAD/cleanliness and acceptance evidence, then acts.
A live worker is never duplicated. A dead worker is inspected and accepted
mechanically where possible before Luna is woken. While any healthy worker is
running, execution causes zero extra Yardmaster model turns.

Serial integration also uses the event journal as a durable target-head cursor.
That lets a second independent worker, originally built from the same base, land
safely after the first worker advances the target. If cherry-pick composition or
final acceptance fails, GARY restores the previously proven clean target HEAD
instead of leaving a partial integration behind. A crash after the integration
event but before the state save is recovered from that same durable evidence.

`gary cancel <job-id>` stops known local workers but preserves worktrees,
branches, commits and logs. `gary status` is mechanical and never wakes Luna.
