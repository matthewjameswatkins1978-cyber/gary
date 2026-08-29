# Resolve AI: compact finalisation brief

## Objective

Finish and package the EXISTING Resolve AI project for the Google All Things Agentic Hackathon.

Target repository:
https://github.com/matthewjameswatkins1978-cyber/resolve-ai.git

Target branch:
hackathon/google-agentic-final

Deadline:
2026-09-01 00:00 UTC (01:00 BST).

Category:
Taskmaster.

This is a finalisation job, not a rewrite.

## First rule

Inspect the current repository before planning work.

The repository already contains a substantial implementation, tests, Cloud Run deployment, README, and Devpost draft. Prefer proving, repairing, and packaging what exists.

Do not rebuild Resolve in TypeScript.
Do not migrate Google ADK to Genkit.
Do not add GARY, Lantern Keeper, Tethers, a new DSL, a new database, Kubernetes, enterprise fleet management, a model router, or other scope expansion.

Make code changes only for a demonstrated submission blocker.

## Product thesis

Resolve is a decision control plane for autonomous agents.

Keep these three decisions independent:

1. Attention: should the human hear about this?
2. Authority: may this consequential action happen?
3. Acceptance: is the work actually good enough?

The key sentence is:

Attention decides whether you hear about it. Authority decides whether it may happen. Acceptance decides whether the work is actually good enough.

Primary principle:

The agent may improve itself. It may not move the goalposts.

The strongest demo proof is that work may be ACCEPTED while git.merge remains ASK.

Do not accidentally couple those decisions.

## Existing implementation to preserve

The repository already includes, or is expected to include:

- Python control plane
- Google ADK
- Gemini planner and coding worker
- frozen Acceptance Contracts
- bounded worker harness
- deterministic ALLOW / ASK / DENY Authority
- deterministic Attention projection
- protected verifier
- convergence / regression handling
- Firestore durable state
- causal ledger
- operator UI
- Cloud Run deployment
- live Gemini demo path
- deterministic smoke path
- README.md
- devpost-submission.md

Inspect actual code and tests rather than trusting this list blindly.

## Google stack

Final submission must truthfully demonstrate the Google technology actually in use.

Expected current stack:

- Google ADK
- Gemini 3.6 Flash planner
- Gemini 3.7 Flash worker
- Cloud Run
- Firestore
- Vertex AI / Gemini access

Verify model names and runtime provenance before changing documentation.

Do not add technology merely to improve the list.

## Deployed service

Expected Cloud Run service:

resolve-ai-control-plane

Expected region:

europe-west2

Expected operator:
https://resolve-ai-control-plane-116252318019.europe-west2.run.app/operator

Expected health:
https://resolve-ai-control-plane-116252318019.europe-west2.run.app/health

Expected runtime provenance:
https://resolve-ai-control-plane-116252318019.europe-west2.run.app/operator/api/runtime

The real demo trigger is expected at POST /demo/build7 and is protected by X-Resolve-Demo-Key / RESOLVE_DEMO_KEY.

Never commit or expose the real demo key.

## Preservation

Preserve existing history and branches.
Never force-push.
Do not delete old implementations.
Do not rewrite unrelated files.

If incorporating code from another existing project, preserve the original and disclose material reused code appropriately.

## Work order

### P0: prove the current system

Inspect current state and run the strongest available checks.

Where practical verify:

- pytest
- ruff check .
- python scripts/run_build8_smoke.py
- deployed /health
- deployed /operator
- deployed /operator/api/runtime
- genuine Gemini path if credentials are available
- wrong or missing demo key is rejected
- no secrets are committed

Also inspect tests/code for the three-gate invariants.

Do not create new tests merely to inflate the test count. Add tests only for a real uncovered risk.

If a check cannot be run because credentials, browser access, or another external prerequisite is unavailable, record that limitation accurately rather than fabricating success.

### P0: fix only real blockers

Examples of valid fixes:

- failing tests
- broken demo
- security leak
- stale runtime/model claim
- broken reproducibility command
- accidental coupling between Attention, Authority, and Acceptance
- Cloud Run/runtime issue

Examples of invalid work:

- framework migrations
- architecture rewrites
- generic refactors
- new product features
- enterprise expansion
- visual polish that risks the working demo

### P1: submission assets

Required tracked deliverables:

- README.md
- devpost-submission.md
- docs/resolve-architecture.svg
- docs/resolve-architecture.png
- docs/demo-script.md
- docs/submission-checklist.md
- docs/screenshots/README.md

The screenshots README is a capture plan and evidence checklist. Do NOT fabricate screenshots. Real screenshots and the public video are human/browser capture tasks.

Architecture diagram must clearly show:

Human goal
-> Gemini planner / Google ADK
-> frozen Acceptance Contract
-> Gemini coding worker
-> structured proposal
-> bounded harness
-> Authority checks
-> writable Git workspace
-> protected verifier
-> trusted evidence
-> Acceptance evaluation
-> convergence supervisor
-> accepted result
-> consequential Authority ALLOW / ASK / DENY

Also show:

- Attention as a separate visibility projection
- Firestore / causal ledger
- operator UI
- Cloud Run boundary
- Google ADK
- Gemini
- Cloud Run
- Firestore

Do not draw Attention as permission.
Do not draw Acceptance as permission.

### README

Do not rewrite README from zero.

Check for:

- accurate three-gate explanation
- Google stack visible near the top
- correct URLs
- reproducible commands
- accurate model names
- accurate limitations
- no unsupported claims
- no secrets

### Devpost draft

Use the existing devpost-submission.md as the base.

Keep category Taskmaster.

Keep the pitch focused on autonomous utility plus architectural discipline.

Do not claim Resolve is a hardened OS sandbox, universal autonomous coding system, enterprise fleet, production GitHub merge executor, or universal exactly-once effect engine.

### Demo script

Target under four minutes.

The script should quickly show:

1. three-gate thesis
2. architecture
3. genuine Gemini work
4. trusted Acceptance evidence
5. ACCEPTED plus git.merge = ASK
6. Attention mode
7. Cloud Run / Firestore / Google ADK / Gemini proof

The memorable point is:

Good work and permission to cause a consequence are different decisions.

Do not manufacture a failure if the worker succeeds on the first attempt.

### Screenshot plan

docs/screenshots/README.md should specify captures for:

1. operator overview with genuine LIVE job
2. Acceptance contract/result
3. Authority showing git.merge = ASK
4. evidence / ledger / provenance
5. Google Cloud proof

Do not create fake screenshots.

### Submission checklist

Create a practical checklist covering:

- tests/lint/smoke
- genuine Google path
- Cloud Run
- Firestore
- secret scan
- README
- architecture diagram
- demo script
- screenshot capture
- public video URL
- repository judge access
- Devpost fields
- logged-out/incognito link check
- final freeze

Mark evidence as verified only when actually verified.

## Three-gate invariants

Preserve these:

- Attention cannot grant Authority.
- Attention cannot determine Acceptance.
- Authority cannot redefine Acceptance.
- Acceptance cannot grant Authority.
- The worker cannot change the frozen Acceptance Contract.
- The worker cannot edit trusted verification.
- The worker cannot promote its own permissions.
- Worker prose is not trusted evidence.
- External consequences happen only after Authority permits them.
- HANDS_OFF does not mean UNBOUNDED.

## Stop condition

Stop adding features when the tracked submission assets exist, final acceptance passes, and no genuine P0 blocker remains.

Prefer a finished, demonstrated Resolve over a more ambitious Resolve.

When uncertain, choose the smaller change.
