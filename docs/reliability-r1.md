# GARY Reliability R1 — Historical Record

> **Historical document.** This describes an earlier reliability design and is retained for provenance. It is **not** the current GARY runtime contract. In particular, retry counts, planner timing, worker-attempt policy, model selection, and escalation behaviour have changed since R1. For current behaviour, use `GARY_RULEBOOK.md`, `docs/JOBPACKET_CONTRACT.md`, the current workflows/code, and the tests.

## Original goal

Make GARY boringly dependable before adding more autonomy.

## R1 architecture at the time

```text
GitHub / human / Lucy
        |
        v
GARY control plane
  - read master brief
  - clone current staging branch
  - build a bounded repository snapshot
  - make one direct Vertex AI planning request
        |
        v
validated JSON plan
        |
        v
Google Cloud Batch workers
  - isolated clone/branch
  - bounded worker attempts
  - failure evidence carried into repair
        |
        v
mechanical acceptance
        |
        v
staging branch for review/integration
```

The durable ideas that survived R1 are separation of coordination from disposable labour, isolated worker state, observable evidence, explicit cost control, preservation of useful failure evidence, and no automatic production/merge authority for ordinary workers.

## Superseded R1 mechanics

R1 also contained concrete limits and policies that were useful experiments at the time but must not be treated as current doctrine, including:

- a fixed five-minute Yardmaster planning ceiling as part of the design description;
- fixed ordinary worker-attempt counts;
- specific model names and thinking levels;
- fixed nested retry behaviour;
- assumptions about when a Closer would or would not be invoked.

Those values evolved as GARY gained live supervision, durable project context, stronger evidence handling, repair behaviour, and completion-oriented orchestration.

The current principle is that elapsed time and raw retry counts are observations, not semantic verdicts. Hard ceilings remain only where they protect real resource, infrastructure, security, authority, or integrity boundaries.

## Historical debugging surfaces

R1 established an important observability direction that remains useful conceptually:

- GitHub Actions run/step status;
- Yardmaster and worker lifecycle logs;
- master status records;
- worker/Batch identifiers;
- captured failure evidence;
- worker branches and accepted commits.

Exact file locations and fields should be taken from the current implementation rather than this historical note.

## Historical failure philosophy

R1 deliberately removed hidden retries, runtime source rewriting, and other machinery that made failures harder to understand. That direction remains valuable: failures should preserve concrete evidence and return control cleanly rather than disappearing inside opaque nested automation.

The modern implementation is less rigid about attempt counts and timing because later experience showed that productive work can be harmed by mechanical limits that mistake time or attempt number for lack of progress.

## Why this document remains

This file is kept so the evolution of GARY's reliability design remains inspectable. It should explain where ideas came from, not constrain current Yardmaster behaviour.

If anything here conflicts with current executable behaviour or current operating documentation, this document loses.
