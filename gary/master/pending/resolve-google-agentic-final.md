GARY MASTER JOB: FINISH AND PACKAGE RESOLVE AI FOR THE GOOGLE ALL THINGS AGENTIC HACKATHON

## 0. YOUR ROLE

You are GARY, the build and completion agent.

Your job is **not to redesign this project**.

Your job is to take the existing Resolve AI implementation, verify what is genuinely working, fix only material defects, prepare the strongest possible hackathon demonstration and submission package, and leave behind a reproducible, clean, judge-friendly final build.

You know nothing about this project before reading this document.

This document is the source of truth.

---

# 1. DEADLINE

Hackathon:

**Google All Things Agentic Hackathon**

Final submission deadline:

**1 September 2026 at 00:00 UTC**

UK time:

**1 September 2026 at 1:00 AM BST**

Do not plan work right up to the deadline.

By Monday 31 August the goal is:

- code frozen
- Cloud Run verified
- README final
- architecture diagram final
- screenshots final
- demo rehearsed
- video recorded
- video uploaded
- Devpost fields ready
- repository accessible to judges

The final day is for fixes and submission packaging, not feature development.

---

# 2. HACKATHON CATEGORY

Submit under:

**Taskmaster**

Do not move this into Fortified Enterprise Fleet.

Resolve is strongest as a focused autonomous-work system rather than an enterprise platform.

---

# 3. JUDGING CRITERIA

The official judging weighting is:

## Innovation & Operational Utility: 40%

Judges want autonomous, high-value action with little human hand-holding.

## Architectural Discipline & Tech Stack: 30%

Judges care about:

- state
- memory
- failure handling
- separation of concerns
- credential security
- robust architecture
- production-minded engineering

## Demo & Production Readiness: 30%

Judges want:

- working demo
- clean architecture diagram
- reproducible setup
- visible Google Cloud proof
- clear repository
- convincing short video

Every decision should improve one of these three scores.

Do not add features merely because they are interesting.

---

# 4. CORE PRODUCT THESIS

Resolve AI is a decision control plane for autonomous agents.

The central insight is that three decisions which are often collapsed into one agent loop should remain independent.

## ATTENTION

**Should the human hear about this?**

Attention controls interruption and visibility.

It does not change permissions.

It does not determine whether work passed.

---

## AUTHORITY

**May this action happen?**

Authority controls consequences.

Its deterministic states are:

- `ALLOW`
- `ASK`
- `DENY`

The model must never be the security boundary.

Missing authority fails closed.

---

## ACCEPTANCE

**Is the work actually good enough?**

Acceptance is based on a frozen contract and trusted evidence.

The worker cannot:

- weaken the acceptance criteria
- edit the trusted verifier
- declare itself successful
- move the finish line

---

## THE ONE-LINE PRODUCT MODEL

> Attention decides whether you hear about it. Authority decides whether it may happen. Acceptance decides whether the work is actually good enough.

That sentence is the centre of the project.

Do not dilute it.

---

# 5. PRIMARY PRODUCT RULE

> The agent may improve itself. It may not move the goalposts.

Autonomous workers should be free to:

- retry
- change strategy
- repair mistakes
- revise implementation
- recover from failure

They should not be free to:

- weaken success criteria
- increase their own permissions
- edit trusted verification
- reinterpret failure as success

---

# 6. EXISTING REPOSITORY

Repository:

`matthewjameswatkins1978-cyber/resolve-ai`

Current repository URL:

`https://github.com/matthewjameswatkins1978-cyber/resolve-ai`

The repository is currently private.

Do not delete it.

Do not rewrite Git history.

Do not destroy previous work.

---

# 7. PRESERVATION RULE

Existing projects and implementations may be used as foundations.

The originals must remain preserved.

Before making submission changes:

1. Record the current `main` HEAD SHA.
2. Create a permanent tag such as:

`pre-hackathon-final-2026-08-29`

3. Create a working branch:

`hackathon/google-agentic-final`

4. Do all final changes on that branch.
5. Never force-push.
6. Never rewrite existing commits.
7. Do not delete historical branches.
8. Only merge the final branch after all acceptance checks pass.

If other projects supply code, preserve their original repositories untouched.

---

# 8. HACKATHON ELIGIBILITY / PRE-EXISTING WORK

The Google hackathon allows frameworks, libraries, starter material, development tools and pre-existing work where permitted, but incorporated pre-existing work must be disclosed appropriately.

Resolve itself was actively developed during the hackathon submission period.

Do not unnecessarily copy large pieces of:

- GARY
- Lantern Keeper
- Tethers
- other projects

Those projects may inform Resolve architecturally.

If actual source code from another pre-existing project is incorporated:

1. record exactly what was copied
2. record the source repository
3. record the original creation date if available
4. add a clear disclosure section to the README

Never try to hide provenance.

---

# 9. IMPORTANT: DO NOT REBUILD THIS PROJECT

A newer planning discussion suggested implementing Resolve from scratch using TypeScript and Genkit.

IGNORE THAT PART.

The existing implementation is considerably further along.

Do not rewrite working Python into TypeScript.

Do not migrate Google ADK to Genkit.

Do not replace working architecture for aesthetic reasons.

Do not create a second competing implementation.

The correct strategy is:

**audit → repair → verify → demonstrate → package**

not:

**rewrite → rediscover bugs → run out of time**

---

# 10. CURRENT IMPLEMENTATION

The existing Resolve implementation includes substantial working functionality.

The repository has already progressed through approximately Builds 1–8.

Major systems include:

- Google ADK planner
- Gemini planner
- genuine Gemini coding worker
- frozen Acceptance Contracts
- bounded worker execution harness
- protected verifier
- deterministic Authority
- Attention projection
- convergence supervision
- best-known checkpoint tracking
- regression detection
- reassignment logic
- Firestore persistence
- causal event ledger
- operator UI
- approval semantics
- Cloud Run deployment
- live demo path
- deterministic smoke path
- remote deployment verification
- Devpost submission draft

Treat this as an existing product that needs final QA and presentation.

---

# 11. CURRENT GOOGLE STACK

Current intended Google stack:

## Agent framework

**Google Agent Development Kit / ADK**

Do not replace this unless it is broken beyond repair.

## Planner

Gemini via Google ADK.

Current planner model:

**Gemini 3.6 Flash**

## Coding worker

Gemini via Google ADK.

Current worker model:

**Gemini 3.7 Flash**

## Model access

Vertex AI / Google Gemini stack.

## Compute

**Google Cloud Run**

## State

**Google Cloud Firestore**

These satisfy the hackathon requirement to use:

- Gemini 3.5 or newer
- a Google agent framework
- Google Cloud infrastructure

---

# 12. CURRENT GOOGLE CLOUD DEPLOYMENT

Project:

`resolve-ai-agentic-2608151746`

Cloud Run service:

`resolve-ai-control-plane`

Region:

`europe-west2`

Public health endpoint:

`https://resolve-ai-control-plane-116252318019.europe-west2.run.app/health`

Public operator UI:

`https://resolve-ai-control-plane-116252318019.europe-west2.run.app/operator`

Runtime provenance:

`https://resolve-ai-control-plane-116252318019.europe-west2.run.app/operator/api/runtime`

The deployment uses zero minimum instances.

Cold starts are therefore expected.

Do not change this purely to eliminate a few seconds of cold-start delay unless there is a genuine demo blocker.

---

# 13. CURRENT LIVE DEMO ENDPOINT

The real deployed demo trigger is:

`POST /demo/build7`

It requires:

`X-Resolve-Demo-Key`

The secret comes from:

`RESOLVE_DEMO_KEY`

This endpoint must remain protected.

The key must NEVER appear in:

- repository
- README
- screenshot
- demo video
- public Devpost description
- logs exposed publicly

Rejected requests must fail before:

- Gemini calls
- demo workspace creation
- expensive operations
- consequential side effects

If judges genuinely need the key, it can later be provided through private Devpost testing instructions.

---

# 14. CURRENT REPOSITORY ARCHITECTURE

Important files include:
```
control_plane/app.py
control_plane/planner.py
control_plane/supervisor.py
control_plane/attention.py
control_plane/coordinator.py
control_plane/approval.py
control_plane/authority_snapshot.py
control_plane/executor.py
control_plane/causal_recorder.py
control_plane/ledger_store.py
control_plane/ledger_reducer.py
control_plane/operator_views.py
control_plane/firestore_repo.py
control_plane/branding.py

worker/gemini_worker.py
worker/harness.py
worker/workspace.py
worker/verifier.py
worker/git_ops.py
worker/proposal_utils.py
worker/models.py

shared/models.py

scripts/demo_fixture/
scripts/run_build7_live_demo.py
scripts/run_build8_smoke.py
scripts/verify_build8_deployment.py

README.md
devpost-submission.md
pyproject.toml
```

Do not reorganise the whole repository unless there is a serious reason.

---

# 15. EXISTING TRUST ARCHITECTURE

The effective trust chain is:
```
UNTRUSTED GEMINI WORKER
        |
        v
STRUCTURED PROPOSAL
        |
        v
BOUNDED HARNESS
        |
        +--> Authority checks
        |
        +--> Workspace constraints
        |
        +--> Git state
        |
        +--> Protected verifier
        |
        v
TRUSTED EVIDENCE BUNDLE
        |
        v
ACCEPTANCE + CONVERGENCE
        |
        v
AUTHORITY FOR CONSEQUENTIAL ACTION
```

This separation is important.

Do not collapse it into a conventional "agent calls tools directly" architecture.

---

# 16. HOW THE GEMINI WORKER CURRENTLY WORKS

The Gemini coding worker should not get arbitrary shell execution.

Its intended output is structured intent resembling:
```
summary
patches:
  path
  new_content
```

The model should NOT provide the authoritative stale file digest.

Resolve derives `expected_sha256` from the real workspace bytes.

Therefore:

Gemini proposes intent.

Resolve determines what state actually exists.

That boundary is intentional.

Preserve it.

---

# 17. AUTHORITY GATE

Authority is deterministic.

Current relevant capabilities include:
```
workspace.read
workspace.apply_patch
test.run
git.status
git.diff
git.commit
git.push
git.merge
deploy.production
```

Example demo policy:
```
workspace.read        ALLOW
workspace.apply_patch ALLOW
test.run              ALLOW
git.status            ALLOW
git.diff               ALLOW
git.commit             ALLOW
git.push               ASK
git.merge              ASK
deploy.production     DENY
```

Rules:

- Missing capability means DENY.
- Malformed capability means failure.
- ASK must not execute automatically.
- DENY must never be overridable by Gemini.
- Attention must not change Authority.
- Acceptance must not change Authority.

The harness should preflight required capabilities before effects and re-check authority at important action boundaries.

---

# 18. THE KEY DEMO MOMENT

A critical property already exists:

**The code can be ACCEPTED while `git.merge` remains ASK.**

That is the strongest compact demonstration of Resolve.

It proves:

Acceptance is not Authority.

The work can be objectively good enough while Resolve still refuses to cause a consequential action without permission.

Protect this behaviour.

Do not accidentally turn acceptance into automatic merge permission.

---

# 19. ACCEPTANCE GATE

The Acceptance Contract must become immutable at supervision start.

The worker may receive the criteria.

It may not alter them.

The verifier must remain outside the worker-writable workspace.

Important acceptance principles:

- worker prose is not evidence
- model confidence is not evidence
- test/verifier results are evidence
- exact Git state is evidence
- passing one attempt does not require fake additional attempts
- genuine first-attempt success should be accepted
- unresolved criteria must remain unresolved
- impossible/contradictory acceptance must not be magically passed

The worker must never be allowed to edit:

- acceptance tests
- protected verifier
- Acceptance Contract
- verifier configuration used as trusted evidence

---

# 20. CONVERGENCE

Resolve should distinguish:

## PROGRESS

Strict improvement in verified criteria.

## REGRESSION

Previously verified criteria cease to pass.

## NO PROGRESS

No meaningful improvement.

## ACCEPTED

Frozen criteria pass against trusted evidence.

The system should preserve the best-known verified checkpoint.

Do not combine criterion success from unrelated attempts into fictional cumulative success.

A later regression must not erase the existence of the genuine earlier best checkpoint.

---

# 21. ATTENTION GATE

Attention is already implemented as deterministic event projection.

Current modes:

- `HANDS_OFF`
- `BALANCED`
- `CLOSE`

Intended behaviour:

## HANDS_OFF

Suppress routine progress and ordinary strategy changes.

Still surface things such as:

- authority required
- action denied
- control failure
- work accepted
- work stopped

## BALANCED

Suppress routine progress.

Show more strategy/recovery information.

## CLOSE

Surface effectively everything.

Important invariant:

**Attention changes visibility, not permission.**

Examples:

`HANDS_OFF + git.merge ASK`

must still require approval.

`CLOSE + git.merge ALLOW`

must not invent an approval requirement.

---

# 22. FIRESTORE / CAUSAL STATE

Resolve records durable state in Firestore.

The causal ledger is intended to be:

- deterministic
- replay-aware
- hash chained
- tamper-evident
- idempotent for identical semantic events

Do not describe it as "tamper-proof."

Use accurate language:

**tamper-evident**

not:

**tamper-proof**

The operator UI should be able to show useful causal state without exposing secrets.

---

# 23. CLOUD RUN WORKSPACE

Cloud Run container filesystems should not be treated as durable project storage.

The demo already uses a temporary workspace.

Preserve this model.

A live demo job should:

1. create a fresh temp workspace
2. seed the sample coding project
3. initialise Git
4. run the worker
5. run protected verification
6. store durable state in Firestore
7. remove the temporary workspace afterwards

Do not build persistence around local Cloud Run filesystem state.

---

# 24. DEMO FIXTURE

The demonstration should use a fixed, easily understood software defect.

The current demo uses:

`process_numbers.py`

The worker is constrained to the allowed writable file.

The protected verifier lives outside the writable workspace.

This is good.

Do not make the coding challenge complicated.

Judges are evaluating Resolve, not Gemini's ability to solve a hard programming puzzle.

The task should be simple enough that:

- the worker usually succeeds
- the acceptance evidence is clear
- the authority distinction is obvious
- the live demo is repeatable

---

# 25. PRODUCT NON-GOALS

DO NOT add these before submission:

- GARY itself inside Resolve
- Lantern Keeper
- Tethers policy language
- new DSL
- arbitrary workflow builder
- multi-user accounts
- enterprise fleet registry
- arbitrary GitHub automation
- IDE extension
- multi-model router
- model marketplace
- cost optimisation engine
- complex auth platform
- billing system
- generic plugin ecosystem
- Kubernetes
- GKE
- new database
- new frontend framework
- new agent framework
- model training
- mobile app

These are scope traps.

---

# 26. CURRENT README

There is already a strong hackathon README.

It includes:

- three-gate thesis
- architecture
- Google stack
- deployment URLs
- demo explanation
- local setup
- live demo commands
- smoke commands
- known limitations
- repository map
- security boundary explanation

Do not rewrite it from zero.

Review it for:

- accuracy
- broken commands
- stale model names
- stale URLs
- contradictions
- unsupported claims
- spelling
- judge readability

Preserve its strongest language.

---

# 27. CURRENT DEVPOST DRAFT

The repository contains:

`devpost-submission.md`

This already contains:

- project title
- one-line summary
- problem
- solution
- why it matters
- AI usage
- Google stack
- architecture
- testing instructions
- links
- demo outline
- screenshot shot list
- judging positioning
- known limitations
- suggested Devpost form answers

Treat this as a draft to verify and finalise.

Do not discard it.

---

# 28. INITIAL GARY ACTION: FULL AUDIT

Before changing code, audit the current state.

Produce a concise report with four categories:

## WORKING

Features proven by code/tests/live deployment.

## BROKEN

Things that definitely fail.

## SUSPECT

Things that may be wrong but need verification.

## MISSING

Required submission elements that do not exist.

Do not start feature work until this report exists.

---

# 29. VERIFY GIT STATE

Record:

- current branch
- current HEAD
- recent commit history
- uncommitted files
- untracked files
- existing tags
- existing submission branches

Create preservation tag and working branch before changes.

---

# 30. VERIFY TEST SUITE

Run:
```
pytest
```

Run:
```
ruff check .
```

If the full suite depends on the Firestore emulator, clearly separate:

- unit tests
- integration tests
- emulator tests
- genuine cloud tests

Never claim "all tests pass" unless you actually ran the relevant suite.

Record exact counts.

If a previously recorded commit says 80 tests passed but the current repository contains more tests, use the current test count rather than repeating 80.

---

# 31. VERIFY DETERMINISTIC SMOKE PATH

Run:
```
python scripts/run_build8_smoke.py
```

This should exercise important control-plane wiring without a billable Gemini call.

Record:

- exit status
- significant checks
- any warnings
- final result

Fix material failures.

---

# 32. VERIFY DEPLOYED SERVICE

Run:
```
python scripts/verify_build8_deployment.py \
  https://resolve-ai-control-plane-116252318019.europe-west2.run.app
```

Also manually verify:
```
/health
/operator
/operator/api/runtime
```

Verify:

- health response
- operator renders
- runtime says Cloud Run
- runtime identifies Google ADK
- runtime identifies Firestore
- model information is correct
- no secrets visible
- wrong/missing demo key gets rejected

---

# 33. VERIFY GENUINE GEMINI PATH

If Google credentials and budget permit, run the genuine demo:
```
python scripts/run_build7_live_demo.py
```

Then verify the resulting state in:

- Firestore
- operator UI
- ledger
- evidence view

A genuine Google model call should be demonstrated before final submission.

Do not fake "LIVE" provenance.

Fixture runs must remain clearly marked `FIXTURE`.

Real runs must remain clearly marked `LIVE`.

---

# 34. VERIFY PROTECTED DEMO ENDPOINT

Test the deployed endpoint with:

## No key

Expected:

`403`

## Wrong key

Expected:

`403`

## Correct key

Expected:

genuine demo job, only when intentionally invoked.

Verify rejected calls do not create demo jobs or trigger Gemini work.

Do not print the real key to logs or terminal transcripts that will later be screenshotted.

---

# 35. SECURITY AUDIT

Search the repository for:

- API keys
- service-account JSON
- private keys
- demo secret
- Google credentials
- bearer tokens
- accidental `.env`
- local machine paths
- email credentials
- GitHub tokens

Check Git history where practical.

Do not expose secrets in:

- README
- screenshots
- architecture diagram
- logs
- video

Runtime authentication should use Google Cloud identity where possible.

---

# 36. AUTHORITY TESTS

Explicitly verify:

1. ALLOW permits intended bounded operation.
2. DENY blocks operation.
3. ASK blocks operation pending human approval.
4. missing capability fails closed.
5. malformed capability fails closed.
6. worker cannot rewrite policy.
7. Attention cannot weaken Authority.
8. Acceptance cannot grant Authority.
9. approval binds to exact action state.
10. reused approval cannot cause duplicate execution.

Do not add tests already adequately covered unless there is a real gap.

---

# 37. ACCEPTANCE TESTS

Explicitly verify:

1. contract is frozen before work starts
2. worker receives a copy, not mutable authority over source contract
3. worker cannot modify verifier
4. worker cannot modify protected acceptance files
5. failed verifier cannot result in accepted status
6. passing trusted verifier can result in accepted status
7. accepted status is tied to exact evidence/Git state
8. worker prose cannot override failing evidence
9. first-attempt success is allowed
10. regression cannot be merged into fictional success

---

# 38. ATTENTION TESTS

Verify:

- HANDS_OFF suppresses routine chatter
- BALANCED surfaces appropriate strategic events
- CLOSE surfaces routine activity
- response-required events always surface
- attention level never changes Authority
- attention level never changes Acceptance

This is important because Attention is part of the headline three-gate thesis.

---

# 39. OPERATOR UI

The operator UI is a demonstration surface, not a product dashboard.

Do not build a dashboard empire.

One strong page is enough.

The judge should be able to understand:

## Acceptance

- frozen contract
- contract digest
- passed/unresolved criteria
- accepted state

## Authority

- current policy
- pending ASK
- allowed/denied capability
- consequential boundary

## Attention

- mode
- surfaced events

## Evidence

- trusted verifier evidence
- Git state
- causal ledger
- run provenance

The hierarchy must be visually obvious.

---

# 40. UI POLISH RULE

Only modify the UI if the existing presentation makes the three gates difficult to understand.

Preferred changes are small:

- clearer headings
- stronger labels
- less clutter
- visually separate gates
- obvious LIVE badge
- readable evidence trail
- readable contract digest
- obvious `git.merge = ASK`

Do not introduce React if the current UI does not require it.

Do not spend submission time on animation.

---

# 41. REQUIRED ARCHITECTURE DIAGRAM

Create a polished architecture diagram suitable for Devpost.

Devpost accepts appropriate image/document formats.

Produce at minimum:
```
docs/resolve-architecture.svg
docs/resolve-architecture.png
```

Prefer also a PDF if convenient.

Diagram should show:
```
Human Goal
   |
   v
Gemini Planner / Google ADK
   |
   v
Frozen Acceptance Contract
   |
   +------------------------+
   |                        |
   v                        v
Firestore                Attention Gate
Control State
   |
   v
Gemini Coding Worker / Google ADK
   |
   v
Structured Proposal
   |
   v
Bounded Harness
   |
   +--> Authority Gate
   |
   +--> Writable Git Workspace
   |
   +--> Protected Verifier
   |
   v
Trusted EvidenceBundle
   |
   v
Acceptance Evaluation
   |
   v
Convergence Supervisor
   |
   +--> Retry
   +--> Restore best checkpoint
   +--> Reassign
   `--> Accepted
          |
          v
   Consequential Authority
      ALLOW | ASK | DENY
          |
          v
    External consequence

Firestore / causal ledger should receive trusted state throughout.
Operator UI reads the resulting trusted state.
Cloud Run surrounds the deployed control plane.
```

The diagram must make the three gates visually separate.

Do not draw Attention as if it grants permission.

Do not draw Acceptance as if it executes consequences.

---

# 42. ARCHITECTURE DIAGRAM LABELS

Include visible Google technology labels:

- Gemini 3.6 Flash
- Gemini 3.7 Flash
- Google ADK
- Vertex AI
- Cloud Run
- Firestore

Judges should not need to hunt for proof of the required stack.

---

# 43. SCREENSHOTS

Prepare 4 or 5 final screenshots.

## Screenshot 1: Operator overview

Show:

- genuine LIVE job
- three concerns clearly visible
- professional overall state

## Screenshot 2: Acceptance

Show:

- frozen contract
- digest
- criteria
- accepted result

## Screenshot 3: Authority

Show:

- accepted work
- `git.merge = ASK`

This is one of the strongest screenshots.

## Screenshot 4: Evidence / causal trail

Show:

- trusted verifier evidence
- ledger
- provenance

## Screenshot 5: Google Cloud proof

Show:

- Cloud Run service
- deployed endpoint
- optionally Firestore
- real Google Cloud project

Avoid screenshots full of browser chrome or irrelevant UI.

---

# 44. DEMO VIDEO

Target:

**under 4 minutes**

Aim around 3:15–3:45.

Do not exceed four minutes.

Judges may only consider the allowed duration.

The video is one of the highest-leverage submission assets.

---

# 45. VIDEO STRUCTURE

## 0:00–0:20

Start with the actual product.

Do not spend 30 seconds on a logo.

Say approximately:

"Autonomous agents increasingly decide how to do work. Resolve keeps three decisions outside that loop: Attention decides whether you hear about it, Authority decides whether it may happen, and Acceptance decides whether the work is actually good enough."

Show the operator UI immediately.

---

## 0:20–0:50

Show the architecture diagram.

Point to:

- Gemini / ADK
- frozen Acceptance
- bounded worker harness
- Authority
- Firestore
- Cloud Run

Explain:

"The worker can change its method. It cannot move its finish line or promote its permissions."

---

## 0:50–2:20

Run/show the real coding demo.

Show:

1. coding task
2. frozen criteria
3. genuine Gemini worker
4. structured proposal
5. bounded execution
6. verifier evidence
7. acceptance result

Do not drown judges in logs.

Show only evidence needed to establish the control loop is real.

---

## 2:20–2:50

Show the killer distinction.

The work is:

`ACCEPTED`

but:

`git.merge = ASK`

Say:

"Resolve considers the work good enough, but that does not mean the agent has permission to merge it. Acceptance and Authority are independent."

This is the memorable moment.

---

## 2:50–3:20

Show Attention.

Explain that HANDS_OFF can suppress routine progress while still surfacing:

- approval requests
- failures
- completion

Say:

"Hands-off does not mean unbounded."

---

## 3:20–3:40

Show Google Cloud proof:

- Cloud Run
- Firestore
- runtime provenance
- `.run.app` URL

Explicitly mention:

- Google ADK
- Gemini
- Cloud Run
- Firestore

---

## Final seconds

End with:

"The agent may improve itself. It may not move the goalposts."

Do not add a long outro.

---

# 46. VIDEO PRODUCTION RULES

The final demo should be:

- clear
- legible
- direct
- unedited enough to feel real
- free of long waiting periods
- free of secrets
- free of irrelevant terminal clutter

If the live Gemini call has variable latency, prepare the flow beforehand.

Do not fake model output.

If necessary, show the genuine completed LIVE job after demonstrating that the real trigger exists.

---

# 47. README FINAL PASS

README must make the following obvious within the first screen:

## What Resolve is

Decision control plane for autonomous AI work.

## Three gates

Attention / Authority / Acceptance.

## Main principle

The agent may improve itself. It may not move the goalposts.

## Google stack

Google ADK + Gemini + Cloud Run + Firestore.

## Live URL

Operator surface.

Do not bury the central idea beneath implementation detail.

---

# 48. README REPRODUCTION SECTION

Verify every command.

At minimum include:
```
python -m venv .venv
```

activation instructions for Linux/macOS and Windows if already present.

Then:
```
python -m pip install -e ".[dev]"
```

Run:
```
uvicorn control_plane.app:app --host 127.0.0.1 --port 8080
```

Tests:
```
pytest
ruff check .
```

Smoke:
```
python scripts/run_build8_smoke.py
```

Genuine demo:
```
python scripts/run_build7_live_demo.py
```

Remote verify:
```
python scripts/verify_build8_deployment.py \
  https://resolve-ai-control-plane-116252318019.europe-west2.run.app
```

Never publish the private demo key.

---

# 49. KNOWN LIMITATIONS

Keep limitations explicit.

Resolve should NOT claim:

- hardened OS sandbox
- production GitHub merge executor
- enterprise fleet management
- universal worker support
- universal exactly-once external effects
- production-grade arbitrary hostile-code containment
- model marketplace
- full cost-aware routing

Accurate limitations make the architecture more credible.

---

# 50. DEVPOST TITLE

Use:

**Resolve AI**

---

# 51. DEVPOST TAGLINE

Primary:

**Three independent gates for safe autonomous work: Attention, Authority and Acceptance.**

Alternative if character limits require shorter text:

**Autonomous agents that cannot move their own goalposts.**

---

# 52. DEVPOST CATEGORY

**Taskmaster**

---

# 53. SUBMITTER

Individual.

Country:

**United Kingdom**

If the form requires an organisation field for an individual, use an accurate neutral value such as `N/A` only if the field accepts it.

Do not invent a company.

---

# 54. PROJECT START DATE

Determine the truthful project start date from Git history during the eligible submission period.

Do not guess.

Use the earliest actual Resolve development commit that constitutes this hackathon project.

The existing repository contains significant work from mid-August 2026.

Record the exact date in `MM-DD-YY` form for Devpost.

---

# 55. GOOGLE SDK FIELD

Select:

**Agent Development Kit / ADK**

If Devpost permits multiple selections and it is truthful, also select Google GenAI SDK because the repository uses `google-genai`.

Do not select frameworks not actually used.

---

# 56. GOOGLE CLOUD SERVICES

Select:

- Cloud Run
- Firestore

Only add others if they are genuinely used by the final implementation.

---

# 57. GOOGLE AI MODELS

List truthfully:

- Gemini 3.7 Flash coding worker
- Gemini 3.6 Flash planner

Verify these names against the deployed runtime before final submission.

Do not submit stale model information.

---

# 58. HOSTED PROJECT URL

Primary:

`https://resolve-ai-control-plane-116252318019.europe-west2.run.app/operator`

---

# 59. CODE REPOSITORY

Current:

`https://github.com/matthewjameswatkins1978-cyber/resolve-ai`

Before submission choose one:

## Preferred

Make repository public after security audit.

OR

## Private

Keep it private but ensure official hackathon judge accounts have access.

The currently known official accounts are:

`testing@devpost.com`

`cloudhackathons@google.com`

Verify the required judge-sharing mechanism when packaging.

---

# 60. DEVPOST PROJECT DESCRIPTION CORE

Use the existing `devpost-submission.md` as the starting draft.

Its central argument should remain:

More capable autonomous agents should be allowed to adapt, retry and recover without controlling:

1. when the human gets interrupted
2. what consequences they are authorised to cause
3. whether their own work passes

Resolve externalises those three decisions.

---

# 61. REQUIRED SUBMISSION ASSETS

GARY must leave the repository with:
```
README.md
devpost-submission.md
docs/resolve-architecture.svg
docs/resolve-architecture.png
docs/screenshots/
docs/demo-script.md
docs/submission-checklist.md
```

Optional but useful:
```
docs/resolve-architecture.pdf
docs/demo-shot-list.md
docs/disclosures.md
```

---

# 62. SUBMISSION CHECKLIST FILE

Create:

`docs/submission-checklist.md`

It must contain checkboxes for:

## Code

- tests pass
- lint passes
- smoke passes
- live Google path proven
- Cloud Run proven
- Firestore proven
- no secrets

## Documentation

- README
- architecture diagram
- setup instructions
- limitations
- disclosures

## Demo

- script
- screenshots
- video
- public video URL
- Google Cloud proof

## Devpost

- category
- country
- start date
- repo URL
- hosted URL
- SDK
- cloud services
- models
- architecture upload
- video URL

## Final

- all links tested logged out/incognito
- repository access tested
- video playable publicly
- submitted before deadline
- no post-deadline modification

---

# 63. P0 PRIORITIES

P0 means submission cannot proceed safely without it.

P0 work:

1. preserve repository state
2. run full audit
3. verify tests
4. verify lint
5. verify smoke path
6. verify deployed service
7. verify three gates
8. verify genuine Google execution
9. fix real blockers
10. security scan
11. verify README commands

Do not start cosmetic work while P0 is red.

---

# 64. P1 PRIORITIES

P1 work:

1. architecture diagram
2. operator UI clarity fixes if genuinely needed
3. screenshots
4. final README pass
5. final Devpost copy
6. demo script
7. submission checklist
8. judge access preparation

---

# 65. P2 PRIORITIES

Only after P0 and P1 are green:

- small visual polish
- minor wording improvements
- optional bonus social copy
- optional public technical article

Do not risk the core submission for bonus points.

---

# 66. BUG-FIX RULE

Every proposed code change must answer:

**What actual submission risk does this remove?**

Good reasons:

- failing test
- security issue
- broken demo
- inaccurate evidence
- broken Cloud Run behaviour
- confusing three-gate distinction
- stale Google model/framework reference
- reproducibility failure

Bad reasons:

- "would be cleaner"
- "could use a newer framework"
- "TypeScript would be nicer"
- "we could add another database"
- "more generic"
- "more enterprise"
- "more configurable"

---

# 67. DO NOT OVER-POLISH

The judges need to believe:

1. it works
2. the architecture is deliberate
3. Google technology is genuinely in the loop
4. it solves a real problem
5. the demo proves the thesis

They do not need 30 settings screens.

---

# 68. GARY REPORTING FORMAT

After every meaningful build job, report:

## CHANGED

Exact files and behaviour.

## VERIFIED

Tests/commands actually executed.

## FAILED

Anything still broken.

## RISKS

Anything uncertain.

## NEXT

The next smallest useful job.

Do not say "done" based solely on code generation.

---

# 69. ABSOLUTE INVARIANTS

The final implementation must preserve all of these:

### Invariant 1

Attention cannot grant Authority.

### Invariant 2

Attention cannot determine Acceptance.

### Invariant 3

Authority cannot redefine Acceptance.

### Invariant 4

Acceptance cannot grant Authority.

### Invariant 5

The worker cannot change the frozen Acceptance Contract.

### Invariant 6

The worker cannot edit trusted verification.

### Invariant 7

The worker cannot promote its own permissions.

### Invariant 8

Worker prose is not trusted evidence.

### Invariant 9

External consequences occur only after Authority permits them.

### Invariant 10

`HANDS_OFF` does not mean `UNBOUNDED`.

These are more important than any feature.

---

# 70. FINAL ACCEPTANCE CRITERIA FOR GARY

GARY's work is accepted only when all applicable items below are proven.

## BUILD

- existing Resolve implementation preserved
- final work isolated on submission branch
- no history rewritten
- test suite passes or remaining failures are explicitly justified
- Ruff passes
- deterministic smoke path passes

## GOOGLE

- Google ADK genuinely used
- Gemini 3.5+ genuinely used
- real model execution demonstrated
- Cloud Run deployment reachable
- Firestore genuinely stores project state
- runtime provenance accurate

## ATTENTION

- deterministic Attention gate exists
- modes behave correctly
- response-required events surface
- Attention cannot modify permission

## AUTHORITY

- ALLOW works
- ASK pauses
- DENY blocks
- missing capability fails closed
- worker cannot rewrite policy
- exact consequence/approval binding preserved

## ACCEPTANCE

- contract frozen
- verifier protected
- failed verification cannot pass
- passing trusted evidence can accept
- worker cannot self-grade

## CONVERGENCE

- verified progress tracked
- regression detected
- best checkpoint preserved
- no fictional merging of unrelated criterion success

## DEMO

- genuine LIVE coding job exists
- acceptance clearly visible
- `git.merge = ASK` clearly visible
- Google Cloud proof visible
- demo repeatable

## SECURITY

- no committed secrets
- no demo key exposed
- wrong/missing demo key rejected
- credentials handled server-side/cloud-side

## REPOSITORY

- clean README
- correct URLs
- reproducible instructions
- known limitations
- disclosures if required

## SUBMISSION ASSETS

- architecture SVG
- architecture PNG
- screenshots
- demo script
- Devpost copy
- submission checklist

---

# 71. STOP CONDITION

Once all final acceptance criteria are green:

**STOP BUILDING FEATURES.**

Freeze the code.

Create final release tag:

`google-agentic-hackathon-final-2026`

Record:

- final commit SHA
- Cloud Run revision
- deployed URL
- test count
- demo job ID
- architecture asset checksum if useful
- video URL once supplied

From that point onward only fix genuine submission blockers.

---

# 72. FINAL PRODUCT POSITIONING

Resolve is not trying to prove that autonomous agents are dangerous.

It is trying to make autonomous agents more usable.

The value proposition is:

Humans should not need to supervise every microscopic step of autonomous work.

But removing constant supervision should not require surrendering:

- visibility
- permission boundaries
- objective verification

Resolve gives autonomous workers room to adapt while keeping those control surfaces independent.

That is the product.

---

# 73. THE DEMO STORY IN ONE PARAGRAPH

A human gives Resolve a coding task. Gemini plans and performs the work. The worker can revise its method, but it cannot rewrite the frozen Acceptance Contract or trusted verifier. Resolve independently checks whether requested operations are authorised, applies bounded changes, runs protected verification, tracks convergence and records trusted state in Firestore. When the work passes, Resolve marks it accepted. But because `git.merge` is configured as `ASK`, the system still pauses before the consequential action. Meanwhile Attention decides how much of the routine process the human needs to see. One autonomous workflow, three independent gates.

---

# 74. THE THREE SENTENCES JUDGES SHOULD REMEMBER

**Attention decides whether you hear about it.**

**Authority decides whether it may happen.**

**Acceptance decides whether the work is actually good enough.**

And:

**The agent may improve itself. It may not move the goalposts.**

---

# 75. STARTING INSTRUCTION

Begin now with:

**JOB 1: FINAL-STATE AUDIT**

Do not make code changes yet.

Inspect the current repository and deployed system against this specification.

Return:

1. current HEAD and preservation plan
2. current implemented feature map
3. current tests and their status
4. live deployment status
5. three-gate implementation status
6. security findings
7. submission asset status
8. exact P0 blockers
9. proposed smallest sequence of repair jobs

Then execute those repair jobs one at a time using the acceptance criteria above.

Do not redesign Resolve.

Finish it.
