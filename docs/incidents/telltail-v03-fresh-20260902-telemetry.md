# Telltail 0.3.0 fresh-run supervision handover

## Finding

The restarted `telltail-v03-fresh` workers passed their mechanical acceptance
checks, checkpointed their candidate patches, and then waited for a Yardmaster
control generation that never arrived. Both Batch jobs eventually reached the
3600-second emergency ceiling.

The worker service account has `roles/storage.objectCreator` and
`roles/storage.objectViewer` on the results bucket, but not
`roles/storage.objectUser`. The dispatcher has object-read access but was not
granted object-write access. The worker uploaded its first `telemetry.json`
object successfully, then silently failed when later heartbeats and the
`candidate_ready` event attempted to overwrite that object. The worker script
suppressed those upload errors. The control plane therefore saw only the first
`running` snapshot and never received the event that triggers Yardmaster
supervision; no `control.json` was produced under either fresh-run attempt.

The durable project status incorrectly recorded both candidates as accepted:

- Wave 1: `142ac72`, Batch job `gary-run-telltail-v03-fre-w1-j1-1-20260902-225843`
- Wave 2: `899242c`, Batch job `gary-run-telltail-v03-fre-w2-j1-1-20260903-000236`

Those candidates were later independently inspected from the checkpoint, but
the live supervision handover itself did not complete. This distinction is
important when interpreting the project as a successful end-to-end run.

## Repair

Worker telemetry and Yardmaster control are now written as immutable snapshots
under `telemetry/` and `control/`. Both sides list and read the newest
snapshot, while retaining fallbacks for older runs that used single objects.
The dispatcher receives only object creation permission for new control
snapshots; the worker retains object creation and read permission. This keeps
the least-privilege boundary and makes the handover append-only.

The default Yardmaster model is now `gemini-3.8-flash`. The model remains
overrideable with `GARY_MASTER_MODEL`.
