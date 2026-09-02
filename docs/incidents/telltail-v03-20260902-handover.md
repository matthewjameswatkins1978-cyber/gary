# Telltail 0.3.0 first live-fire handover incident — 2026-09-02

## Purpose

Preserve the evidence from the first Telltail 0.3.0 GARY live-fire attempt before restarting the project from the canonical seed.

This record is forensic history, not active project state and not a source of product requirements. The canonical Telltail master brief remains:

`gary/master/pending/telltail-v0-3-mirror-gary-os-20260902.md`

## What happened

Two related master identities failed during the live-fire sequence:

- `telltail-v03-livefire`: planning failed because the Yardmaster emitted an ordinary pipe-based verifier and GARY's acceptance-command validator rejected the shell syntax.
- `telltail-v03-gary`: the first child worker was attempted twice. The durable status records a final `BUDGET_STOPPED` child result after about 7,585 seconds total worker duration, followed by a wave-2 planner failure on the same over-restrictive shell validator.

The worker-side evidence captured during the incident reported green Telltail acceptance, but that worker statement is not itself a GARY pass. The important failure is that the candidate did not survive the handover into a pushed child commit that GARY could independently verify and integrate.

## Durable Git evidence

Failed-run archive branch in this repository:

`archive/telltail-v03-failed-20260902`

Telltail target repository:

`matthewjameswatkins1978-cyber/gary-test-clamp`

Canonical seed:

`telltail-canonical-seed` at `f4b92cb12c5827ec699f1b140a940d1afdc80061`

Master staging branches left by the failed runs:

- `gary/master-telltail-v03-gar-677853`
- `gary/master-telltail-v03-liv-aef6f2`

At incident review time both staging branches were identical to `telltail-canonical-seed`. They therefore do **not** contain the worker's Telltail implementation and must not be presented as saved product work.

The recorded last child branch for `telltail-v03-gary` was:

`gary/telltail-v03-gar-w1-j1-18982f3b`

The durable status contains no commit SHA for it.

## Candidate artifact recovery reference

From GARY's run-ID construction, the candidate artifact prefix for that child run is expected to be:

`gs://gary-batch-results-418146339023/runs/telltail-v03-gar-w1-j1-18982f3b/`

with attempt checkpoints conventionally under:

- `attempt-1/`
- `attempt-2/`

The recorded final Batch job ID was:

`gary-run-telltail-v03-gar-w1-j1-2-20260902-193136`

This note does **not** claim the GCS objects still exist or are complete. If forensic recovery is ever useful, inspect those prefixes for `result.patch`, `acceptance_output.txt`, `usage.jsonl`, and telemetry before concluding that the candidate is recoverable.

## Lessons retained in GARY

The repair triggered by this incident makes these distinctions explicit:

1. Worker claims are evidence, not acceptance.
2. Batch terminal state is transport/execution evidence, not product acceptance.
3. A recoverable candidate must be independently mechanically verified even when Batch reports failure.
4. A mechanically green candidate must be durably checkpointed before the Yardmaster handover.
5. Mechanical green is necessary but not sufficient. Yardmaster must inspect the exact candidate for the child objective and reasonable professional engineering quality.
6. Child quality dispositions are `PASS`, `SMALL_FIX`, `MAJOR_FIX`, and `REPLAN`.
7. Only trusted mechanical verification + Yardmaster `PASS` + exact recoverable branch/commit may set `accepted=true` and permit staging integration.
8. Small and major repair findings preserve useful work rather than causing blind restarts.
9. Ordinary shell verifier composition is supported; dangerous control-plane actions remain rejected.

## Restart rule

The replacement Telltail run must use a fresh master/project identity and start from the unchanged canonical seed. It must not reuse either failed master staging branch as its starting state.

The historical candidate evidence is preserved for diagnosis and possible forensic recovery only.
