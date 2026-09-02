# GARY Self-Job: Integrate the Yardmaster Operating Guide

## Objective

Make `docs/YARDMASTER_OPERATING_GUIDE.md` a real part of GARY's Yardmaster behaviour, not merely documentation.

The full guide is already committed on `master`. It deliberately contains practical engineering tradecraft rather than another rigid rule layer.

Implement the smallest idiomatic Go integration that gives the Yardmaster the useful operating approach during planning and, where clearly useful, supervision, while preserving model judgement and keeping prompt size sensible.

## Why this exists

GARY already has a completion principle, universal rules, role rules, project context, and mechanical acceptance. What is missing is a strong default operating method describing how an experienced technical lead normally sets a project up for successful agent work.

This guide covers project identity, exact base revisions, filesystem/workspace organisation, isolated worktrees, environment/tool preparation, baseline verification, coherent job decomposition, dependency and mutable-ownership reasoning, JobPacket construction, context sizing, local-vs-cloud dispatch, evidence-based supervision, repair handovers, machine evidence, result classification, integration, replanning, final acceptance, packaging, and learning.

The operating guide is advice to support intelligence. It is not a checklist gate.

## Runtime semantics

The Yardmaster must receive a concise runtime form of the guide with framing equivalent to:

> This is accumulated engineering tradecraft and a strong default operating method, not a compulsory procedure. Understand why each practice exists. Apply, adapt, combine, omit, or extend it according to current project evidence. Current project requirements and evidence outrank habit. The objective is not compliance with this guide. The objective is to arrange the project, tools, workers, ownership and evidence so the requested result is completed correctly.

Preserve the core question:

> What arrangement of project state, tools, information, workers, ownership, and evidence gives this project the best chance of being completed correctly?

The runtime form should cover, compactly but materially:

1. Establish exact project/repository/base-SHA identity and preserve the master packet.
2. Give projects and concurrent workers isolated mutable workspaces/worktrees.
3. Inspect repository instructions, CI, toolchain and relevant architecture before deep planning.
4. Establish a useful baseline when cheap and informative.
5. Prepare ordinary required tools before repeatedly dispatching workers into a broken environment; distinguish setup from redesign.
6. Prefer a canonical verification route where useful.
7. Split work into coherent outcomes, not arbitrary time slices.
8. Reason about dependencies and mutable ownership before parallelising.
9. Build JobPackets containing identity, outcome, relevant context, existing/required behaviour, scope/ownership, protected constraints, dependencies, inputs, expected outputs, acceptance and genuine stop conditions.
10. Give enough context to prevent needless rediscovery without flooding workers with project history.
11. Choose local/cloud execution and machine resources according to actual work.
12. Supervise from engineering evidence, not elapsed time alone.
13. Treat failed attempts as handovers: preserve useful partial work and pass evidence to repair workers.
14. Prefer trusted machine measurement for Git/test/scope/runtime facts; worker commentary is context, not proof.
15. Use Good Job / Fix / Broken as human-facing judgements without turning retry counts into mechanical law.
16. Inspect before integration, then replan from the new accepted reality.
17. Keep mutable ownership unambiguous across concurrent projects/workers.
18. Ask humans only for genuine authority/product decisions, not ordinary missing tools.
19. Run project-level acceptance and actual artifact/deployment verification before completion.
20. Package durable final state and let the Foreman's Notebook learn cautiously rather than turning incidents into folklore.

## Important non-goals

- Do not create a new mechanical gate for every section of the guide.
- Do not require every project to use the example filesystem paths literally.
- Do not make writable-path declarations mandatory when semantic ownership or isolation is more appropriate.
- Do not create a generic workflow engine, policy language, knowledge graph, vector DB, or large new subsystem.
- Do not make the Yardmaster execute tools during its no-tools planning model call. The guide should influence what setup/work it delegates or what the trusted runtime prepares.
- Do not duplicate the entire long Markdown guide verbatim into every prompt if a compact faithful runtime form is better.
- Do not weaken project provenance, final acceptance, permission/security boundaries, cost ceilings, or worker isolation.
- Do not reintroduce arbitrary timing gates.

## Relationship to other doctrine

Use this conceptual order:

1. Completion purpose / authority boundaries.
2. Yardmaster operating approach.
3. Foreman's Notebook evidence-weighted experience when available.
4. Current project packet and current evidence.
5. Yardmaster judgement and action.

The operating guide is neither above the project brief nor a source of new project requirements.

## JobPacket quality

The planner should be nudged toward packets like:

```text
JOB: Strict JSON provider

OUTCOME
Implement strict JSON provider behaviour required by Stage C.

STARTING STATE
Repository: <repo>
Base: accepted staging SHA/branch

RELEVANT CONTEXT
Provider interface already exists. TextProvider is the closest reference.
Malformed JSON must be rejected.

SCOPE / OWNERSHIP
Provider implementation and focused tests. Small shared supporting changes are allowed if genuinely necessary.

PRESERVE
Do not weaken tests or casually redesign the public provider interface.

ACCEPTANCE
cargo fmt --check
cargo test json_provider
cargo test

Use your judgement on implementation tactics and ordinary permitted setup.
```

Do not force literal headings if the existing Job schema remains a compact task string plus acceptance commands. Preserve existing schemas unless a small backwards-compatible improvement is clearly justified. The semantic content matters more than formatting.

## Tests

Add focused deterministic tests with no paid model calls. At minimum prove:

- the Yardmaster planning prompt contains the operating-guide framing;
- the prompt communicates coherent outcomes rather than arbitrary time slicing;
- project/current evidence explicitly outranks the guide;
- ordinary environment/tool setup is described as something to solve rather than an automatic blocker;
- isolation/mutable ownership and evidence handover are represented;
- the guide is advisory and does not create a new hard acceptance gate;
- existing prompt/doctrine tests continue to pass.

Keep the runtime prompt reasonably sized. If you introduce a compact runtime constant, keep the full human-readable guide in `docs/YARDMASTER_OPERATING_GUIDE.md` as the explanatory source.

## Documentation

Add a short cross-reference from the appropriate existing GARY documentation or rulebook so future maintainers understand:

- Rulebook/doctrine defines boundaries and roles.
- Yardmaster Operating Guide provides practical default tradecraft.
- Foreman's Notebook records GARY's own evidence-weighted experience.

Do not merge these into one giant undifferentiated prompt.

## Acceptance

Run:

```text
go test ./...
git diff --check
```

The implementation is complete only when the operating method genuinely reaches Yardmaster planning behaviour and tests demonstrate that it remains advisory rather than becoming another hard gate.
