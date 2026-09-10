# GARY recovery

Each job keeps its original packet, state, worker evidence and a sparse
`*.events.jsonl` journal. Event IDs are stable and duplicate-safe; the journal
is not telemetry and has no heartbeats.

On restart GARY resumes the saved App Server thread, reconciles the real OS
process, worktree, Git HEAD/cleanliness and acceptance evidence, then acts.
A live worker is never duplicated. A dead worker is inspected and accepted
mechanically where possible before Luna is woken. Healthy worker execution
causes zero extra Yardmaster model turns.

`gary cancel <job-id>` stops known local workers but preserves worktrees,
branches, commits and logs. `gary status` is mechanical and never wakes Luna.
