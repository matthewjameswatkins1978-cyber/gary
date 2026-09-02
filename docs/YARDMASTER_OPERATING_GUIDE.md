# GARY Yardmaster Operating Guide

## Purpose

This guide describes a strong default way to turn a finished project plan into completed engineering work through GARY's workers.

It explains how to establish project identity, prepare the filesystem and toolchain, create isolated workspaces, define verification, divide implementation into coherent jobs, construct worker packets, run work locally or in the cloud, preserve evidence, repair failures, integrate results, and verify completion.

This is an operating method, not a rigid procedure.

Use it as accumulated engineering tradecraft. Understand why each practice exists, apply the parts that help, omit or change the parts that do not, and add steps when the project clearly needs them. Current project evidence outranks habit.

The objective is not compliance with this guide. The objective is to get the real project finished correctly, efficiently, and with trustworthy evidence.

The system should make the sensible path easy while leaving enough freedom for an intelligent agent to discover a better one.

At every stage ask:

> What arrangement of project state, tools, information, workers, ownership, and evidence gives this project the best chance of being completed correctly?

---

## 1. Establish the project identity

Before assigning implementation work, identify the project unambiguously.

Record and preserve:

- product objective;
- v1 or current-scope boundary;
- major architecture decisions that are intended to remain stable;
- canonical repository;
- exact base revision or Git SHA;
- master acceptance criteria;
- important permissions and authority boundaries;
- explicit exclusions and protected decisions;
- external systems involved, such as Google Cloud, GitHub, databases, APIs, or deployment targets.

The original master packet remains authoritative. Summaries, worker notes, and later plans may help execution but do not silently replace the requested outcome.

Do not tell a worker to use "the latest version" when an exact accepted base can be named.

Example:

```text
Project: Suture 0.1
Repository: <canonical repository>
Base SHA: abc123...
Objective: implement the frozen Suture 0.1 behaviour
Acceptance: cargo fmt --check; cargo test; cargo clippy ...
Protected: public semantics and frozen acceptance
```

A worker may discover a better implementation. It may not redefine the project merely because another outcome is easier.

---

## 2. Give every project an isolated home

Each active project should have durable state and a clearly owned mutable workspace.

A useful conceptual layout is:

```text
yard/
  projects/
    <project-id>/
      source/
      worktrees/
      control/
      evidence/
      artifacts/
      logs/
      cache/
```

The exact paths are not sacred. The ownership model is.

### `source/`

Canonical local checkout or integration checkout. Keep it clean where practical.

### `worktrees/`

One isolated Git worktree or equivalent checkout per active worker job when jobs can overlap in time.

Example:

```text
worktrees/
  suture-021/
  suture-022/
  suture-023/
```

Do not let unrelated workers casually mutate the same checkout.

### `control/`

Durable project metadata, such as:

```text
project.json
master-packet
state.json
amendments/
jobs/
```

### `evidence/`

Machine-generated evidence by job or project stage:

```text
evidence/
  suture-021/
    git.json
    tests.json
    build.log
    lint.log
    diff.patch
    result.json
```

### `artifacts/`

Binaries, packages, screenshots, generated reports, deployment manifests, checksums, and other final or intermediate artifacts worth preserving.

### `logs/`

Execution and orchestration logs.

### `cache/`

Disposable dependency or build caches. Nothing important should exist only here.

The key principle is:

> One project owns its mutable workspace, and each concurrent worker has unambiguous write ownership.

This is what allows many projects to be in the yard at once without mixing their state.

---

## 3. Inspect the repository before planning deeply

Do enough reconnaissance to understand how the project actually works.

Prefer the repository's own executable and documented truth over assumptions.

Look first for files such as:

```text
README.md
AGENTS.md
CONTRIBUTING.md
Makefile
justfile
package.json
pnpm-workspace.yaml
go.mod
Cargo.toml
rust-toolchain.toml
pyproject.toml
Dockerfile
.github/workflows/*
infra/*
tests/*
scripts/*
```

Answer practical questions:

- What language and toolchain does this use?
- Is a toolchain version pinned?
- How is it built?
- How is it tested?
- How is it formatted or linted?
- What already exists?
- Where are the natural architectural boundaries?
- Are files generated?
- Are there external services?
- Does CI reveal the real build commands?
- Is deployment part of acceptance?

Do not read the entire repository for ceremony. Inspect enough to make good orchestration decisions.

CI is often valuable because it exposes the repository's actual verification path when prose documentation is stale.

If documentation and current executable configuration disagree, investigate rather than blindly choosing either.

---

## 4. Establish a useful baseline

When it is cheap and meaningful, run the relevant verification before workers modify the project.

Examples:

### Go

```bash
git status --short
go test ./...
go vet ./...
```

### Rust

```bash
git status --short
cargo fmt --check
cargo test
cargo clippy --all-targets --all-features -- -D warnings
```

### Node/Web

```bash
npm ci
npm test
npm run build
```

or use the project's own package manager and scripts.

Record useful baseline evidence:

- exact base SHA;
- working-tree state;
- tool versions where relevant;
- build result;
- test result;
- known existing failures.

If the baseline is already broken, record that fact. Do not make a worker spend an hour repairing a failure it did not create.

Do not insist on a perfect baseline when obtaining one is expensive or impossible and does not help the next decision. Baseline checks are evidence, not ritual.

---

## 5. Prepare the environment before repeatedly dispatching implementation

Setup is engineering work.

Make sure ordinary tools required by the job and its acceptance checks are available before repeatedly sending workers into the same broken environment.

Prefer project-declared or pinned versions.

Typical useful tools include:

```text
git
curl
wget
jq
yq
ripgrep
fd
zip/unzip
tar
make
just
shellcheck
```

Install only language ecosystems current work needs.

### Go

```text
go
gofmt
go vet
```

### Rust

```text
rustup
cargo
rustfmt
clippy
```

### Node/Web

```text
node
corepack
pnpm
```

Prefer the project's declared package manager. Do not convert an npm project to pnpm merely because pnpm is generally preferred.

### Python

When genuinely needed:

```text
python
uv
```

Do not introduce Python as a runtime dependency merely because orchestration finds it convenient.

### Java/Android

When needed:

```text
JDK
Gradle
Android command-line tools
Android SDK
```

### Web testing

Use browser automation such as Playwright when actual browser behaviour matters.

If a required ordinary development tool such as `protoc` is missing, safely provisioning it is usually preferable to making several workers rediscover the same missing executable.

Distinguish environment preparation from redesign.

Installing a missing compiler is setup.

Replacing PostgreSQL with SQLite because PostgreSQL is inconvenient is an architecture change.

If the obvious tool route fails, investigate a safe alternative before declaring the environment blocked.

---

## 6. Keep secrets and authority outside ordinary worker prompts

Secrets do not belong in:

- Git;
- task packets;
- README files;
- agent prompts;
- logs;
- screenshots.

Use environment injection, workload identity, short-lived credentials, or a proper secret manager.

Where possible, prefer identity over long-lived secret files.

Cloud secrets should use the platform's intended secret mechanism, such as Google Secret Manager.

A trusted orchestration layer may hold narrowly scoped authority that an implementation worker does not need.

For example, a trusted wrapper may:

- obtain short-lived GitHub App credentials;
- upload evidence;
- communicate with GARY state storage;
- launch or terminate workers;
- measure runtime and cost;
- run trusted verification;
- push authorised checkpoints.

An unprivileged worker generally needs:

- source code;
- its JobPacket;
- local development tools;
- an isolated Git workspace.

It normally does not need personal credentials, production secrets, merge authority, or unrestricted cloud identity.

---

## 7. Prefer one canonical project verification entry point

When practical, make the project's important verification obvious through a single entry point such as:

```bash
just verify
```

or:

```bash
./scripts/verify
```

That entry point may run:

- format checks;
- lint;
- type checking;
- unit tests;
- integration tests;
- build;
- protected acceptance checks;
- smoke verification.

Focused child jobs should still run smaller relevant checks during implementation.

The canonical verifier exists so that final project acceptance does not depend on every worker inventing a slightly different definition of "tested".

Do not create a wrapper merely for cosmetic uniformity when the repository already has a clear canonical command.

---

## 8. Find the natural work graph

Do not immediately slice a project into equal durations or equal numbers of files.

Identify coherent implementation outcomes.

A good child job should be:

- useful;
- understandable as one outcome;
- objectively testable where possible;
- large enough to justify a worker;
- small enough for one worker to own coherently;
- independent of concurrent work where practical;
- bounded by the project's architecture and acceptance.

Bad:

```text
Work on Suture for 15 minutes.
```

Better:

```text
Implement the strict JSON provider, including malformed-input behaviour
and focused tests, while preserving the existing provider interface.
```

The second packet gives the worker a real engineering result to own.

Think in terms of dependencies:

```text
core types
   |
   +-- text provider
   +-- JSON provider
   +-- workspace layer
          |
          +-- CLI integration
```

Independent middle jobs may run concurrently after the shared interface is stable enough.

Parallelism is useful when work is genuinely separable.

It is harmful when several workers all need uncontrolled writes to the same central files.

Consider both logical dependency and mutable ownership.

---

## 9. Use writable ownership intelligently

For parallel work, define expected writable areas when doing so genuinely prevents collisions.

Example:

```yaml
allowed_paths:
  - src/calle/firestore/**
  - tests/calle/firestore/**
```

Protected areas may include:

```yaml
protected_paths:
  - tests/protected/**
  - acceptance/**
  - infra/production/**
```

These boundaries can also inform scheduling: jobs with overlapping mutable ownership may need sequencing or separate integration handling.

Do not turn path lists into a filesystem religion.

Some coherent changes legitimately cross layers. If the project architecture makes strict path ownership artificial, describe semantic scope instead and isolate the worker through Git/worktree ownership.

The purpose is collision reduction and clear responsibility, not arbitrary rejection.

---

## 10. Construct a proper JobPacket

A worker packet should let a capable worker answer five questions immediately:

1. What outcome am I responsible for?
2. What exact project state am I starting from?
3. What context changes how I should perform this job?
4. What must I preserve or avoid changing?
5. What evidence will demonstrate success?

A strong JobPacket normally includes the following.

### Identity

```text
Run ID
Project ID
Job ID
Plan/manual version if relevant
Repository
Base SHA
Worker branch/worktree
```

### Objective

One precise description of what must exist when the job is finished.

### Relevant context

Only facts the worker needs and should not have to rediscover unnecessarily.

### Existing behaviour

What currently exists that matters to this job.

### Required behaviour

What must change.

### Scope and ownership

Expected writable files, modules, components, or semantic territory when useful.

### Protected or forbidden scope

Things that must not be casually altered.

### Dependencies

Accepted work this job requires.

### Inputs

Schemas, interfaces, fixtures, APIs, existing components, or evidence needed by the worker.

### Expected outputs

Expected files, behaviours, artifacts, or interfaces.

### Acceptance commands

Focused, executable checks.

### Behavioural acceptance

Important properties not fully represented by one command.

### Resources and budget

When operationally relevant: model tier, machine profile, cost ceiling, or runtime safety ceiling.

These are resource controls, not substitutes for engineering judgement.

### Genuine stop conditions

Use stop conditions for real authority or architecture conflicts.

Example:

```text
If completing this job requires changing the frozen CallResult schema,
return the conflict rather than silently redesigning the schema.
```

Do not use arbitrary elapsed-time targets as semantic failure conditions.

---

## 11. Example JobPacket

```text
JOB: Strict JSON provider

OUTCOME
Implement the JSON provider required by Stage C.

WORKSPACE
Repository: <project repo>
Base SHA: abc123...
Branch/worktree: worker-owned isolated workspace

RELEVANT CONTEXT
The provider interface already exists in src/provider.rs.
TextProvider is the closest reference implementation.
JSON must reject malformed input rather than silently recover.

EXISTING BEHAVIOUR
TextProvider passes its focused tests. No JSON provider currently exists.

REQUIRED BEHAVIOUR
Implement strict JSON parsing and provider behaviour required by the
frozen Stage C contract.

SCOPE
Implement the JSON provider and focused tests. Small supporting changes
to shared provider code are allowed if implementation genuinely requires them.

PRESERVE
Do not weaken existing tests.
Do not alter public execution semantics.
Do not redesign the provider interface merely for convenience.

ACCEPTANCE
cargo fmt --check
cargo test json_provider
cargo test

USE YOUR JUDGEMENT
Choose the implementation approach yourself. Inspect additional repository
files when useful. Provision ordinary development dependencies if needed and permitted.
```

Notice what is deliberately absent: a thirty-step implementation recipe.

The worker is an intelligent implementation agent, not a script interpreter.

---

## 12. Give the worker the right amount of context

Too little context causes rediscovery and avoidable mistakes.

Too much context causes distraction and consumes reasoning capacity.

Pass information that changes how the worker should act.

Useful:

```text
Previous attempt established that the parser logic is correct. The remaining
failure is Windows newline handling in decodeFixture. Preserve the parser changes.
```

Usually not useful:

```text
Here is the full history of every project discussion since conception.
```

Workers normally need:

- their outcome;
- relevant architecture;
- current accepted state;
- constraints;
- dependencies;
- important previous evidence;
- acceptance commands.

Let them inspect ordinary implementation details themselves.

Do not make workers spend half their context rediscovering facts the Yardmaster already knows, and do not drown them in irrelevant history.

---

## 13. Choose local or cloud execution according to the work

Not every job belongs in the cloud.

Local execution is often best for:

- one or two jobs;
- quick investigations;
- architectural probes;
- tiny fixes;
- interactive work;
- early project setup;
- debugging the environment itself.

Cloud parallelism becomes valuable when several useful jobs are genuinely independent.

For example:

```text
A: API implementation
B: fixtures
C: UI shell
D: persistence adapter
```

If their dependencies and mutable ownership do not collide, they may run simultaneously.

Cloud execution does not make one worker magically faster. It lets multiple workers make progress at once.

Choose the smallest machine profile that gives the worker a healthy environment. Do not give every worker a huge machine, but do not save pennies by creating predictable out-of-memory or painfully slow build failures.

Disposable workers should be reproducible copies of a known environment, not mysterious machines that happen to compile today.

---

## 14. Let workers work

Once a capable worker has a coherent job, an isolated workspace, adequate context, and the necessary tools, give it room to solve the problem.

Do not interrupt healthy builds, tests, installs, downloads, or implementation merely because an arbitrary slice length elapsed.

Observe engineering signals such as:

- meaningful source changes;
- build progress;
- narrowing test failures;
- resolved dependencies;
- new diagnostic evidence;
- repeated identical failures;
- absence of substantive progress.

A twelve-minute compiler build may be healthy.

Repeating the same failed command six times in three minutes without learning anything is not.

Time is evidence about cost and progress. It is not proof that engineering work has failed.

---

## 15. Supervise from evidence

At a supervision point ask:

- What changed?
- What now passes?
- What still fails?
- Is uncertainty decreasing?
- Is useful partial work accumulating?
- Is the worker intelligently changing approach when evidence changes?
- Does it need information or authority it cannot obtain?

Then choose an appropriate action such as:

```text
CONTINUE
GUIDE
STOP_AND_REPLAN
READY_FOR_INSPECTION
```

Guidance should add useful information.

Weak:

```text
Keep going.
```

Better:

```text
The parser tests now pass. The remaining failure is workspace_roundtrip.
Preserve the parser changes and focus on why reconstruction changes ordering.
```

---

## 16. Treat failed attempts as handovers, not resets

A subsequent worker should inherit what has already been learned.

A good repair packet records:

```text
WHAT WORKS
Parser and validation tests pass.

WHAT FAILED
workspace_roundtrip fails.

EVIDENCE
Expected X, received Y.

WHAT WAS TRIED
Changing serializer ordering did not alter the failure.

PRESERVE
Do not discard accepted parser work from commit abc123.

NEXT OBJECTIVE
Determine why workspace reconstruction changes ordering and repair it.
```

Bad repair instruction:

```text
Try again.
```

Better:

```text
Everything passes except duplicate empty-call responses. Repair this exact
failing case without altering the adapter interface. Preserve the accepted
transaction handling from the previous attempt.
```

The next worker should begin on the current lap, not restart the race.

---

## 17. Preserve useful partial work

A failed final check does not mean an attempt produced nothing useful.

Distinguish:

```text
attempt failed final acceptance
```

from:

```text
attempt produced no useful progress
```

If a worker implemented substantial correct work, preserve it in the project's controlled history and use it as the basis for repair.

Do not throw away ninety percent correct work merely because the last edge case still fails.

Similarly, do not preserve a large change merely because effort was spent on it. Evidence determines usefulness.

---

## 18. Separate machine evidence from worker commentary

Return two different things.

### A. Trusted machine evidence

Where practical, GARY or its trusted wrapper should measure authoritative facts itself.

Useful evidence includes:

#### Git

- base SHA;
- final SHA;
- branch/worktree identity;
- commits;
- changed paths;
- diff;
- working-tree state.

#### Scope

- expected/allowed path observations when applicable;
- protected-path changes;
- unexpected generated files.

#### Verification

For each important check:

- command;
- exit code;
- output or log reference;
- duration.

#### Runtime

- worker/model identity;
- machine profile;
- elapsed time;
- retries;
- measured model/token/cost data when available;
- infrastructure interruptions.

Do not ask the AI to report facts the machinery can measure more reliably.

### B. Worker commentary

The worker also returns a concise human-readable note explaining what it did and any important design observation.

Example:

```text
Implemented the Firestore CallResultStore adapter.
Added idempotent event writes, transaction-safe status updates, and malformed
document handling. Existing fixtures lack provider timestamps, so current
received_at behaviour is preserved. No architecture changes required.
```

This is useful context.

It is not proof.

---

## 19. Classify results for humans without reducing judgement to labels

Useful human-facing categories are:

### Good Job

The work is useful and satisfies its important acceptance criteria.

Typical evidence:

- correct scope;
- clean understandable Git state;
- relevant tests pass;
- build passes;
- required acceptance passes;
- no unresolved architecture surprise.

Good Job means candidate for integration, not automatic merge authority.

### Fix

The work is substantially useful but has a narrow repairable problem.

Examples:

- one edge case fails;
- lint failure;
- small integration mismatch;
- one required behaviour is missing;
- minor contract mismatch.

Produce a focused repair packet using current evidence.

Do not mechanically limit useful repair to an arbitrary single attempt. If evidence shows convergence, continue within cost and safety boundaries. If repeated attempts produce no new useful evidence, replan rather than endlessly retrying.

### Broken

The current approach should stop and be reconsidered.

Examples:

- wrong architecture;
- major scope violation;
- protected acceptance was rewritten;
- requirement fundamentally misunderstood;
- unresolved product decision is genuinely required;
- dependency or environment mismatch invalidates the approach;
- repeated attempts show no meaningful progress.

Broken is useful when it accurately identifies why current work cannot safely continue.

It should not become a euphemism for "the worker needed another ordinary tool installed".

---

## 20. Inspect before integration

A worker claiming completion means ready for inspection.

Inspect:

- exact diff;
- relevant tests;
- working-tree cleanliness;
- unexpected files;
- accidental secrets;
- scope creep;
- protected acceptance machinery;
- relationship to other accepted work.

Integrate only accepted useful work into the project's controlled staging/integration state.

No worker earns merge or production authority merely by reporting success.

---

## 21. Replan from the new reality

After accepted work is integrated, reconsider the remaining project.

Do not blindly execute a task list created before the project taught you anything.

Suppose the original plan was:

```text
A -> B -> C -> D
```

If B proves C is unnecessary, remove C.

If B reveals missing foundation E, add E.

If two previously coupled jobs become safely independent, run them in parallel.

If an assumption was wrong, update the plan while preserving the master objective and frozen constraints.

The plan serves the project. The project does not serve the plan.

---

## 22. Maintain clean ownership throughout

At all times know who owns mutable state.

Conceptually:

```text
Project
  integration branch/workspace

Worker A
  isolated branch/worktree A

Worker B
  isolated branch/worktree B
```

Shared read access is normal.

Uncontrolled shared write access is not.

When several projects run concurrently, each project also keeps its own durable package and integration state.

Concurrency should be limited by real shared resources, cost, mutable ownership, and infrastructure capacity, not by an unnecessary global project mutex.

---

## 23. Use tools and external systems sensibly

Workers should be able to use ordinary tools needed to finish authorised work.

Examples:

- install a compatible development package;
- use repository-pinned compilers;
- inspect official documentation when behaviour is unclear;
- use Google Cloud tooling when the project requires Google Cloud;
- use a safe equivalent CLI tool if the preferred convenience tool is unavailable.

Always distinguish implementation freedom from changing the requested outcome.

Using `rg` instead of `grep` is an implementation choice.

Replacing a requested Cloud Run deployment with a local-only Docker demo is a product/deployment change.

---

## 24. Ask for human intervention only when judgement or authority genuinely belongs outside the yard

Ordinary engineering problems should normally be solved by the Yardmaster and workers.

Human intervention is appropriate for things such as:

- irreversible destructive action;
- missing account authority;
- significant unapproved expenditure;
- genuinely ambiguous product requirement;
- two materially different valid interpretations that require product choice;
- secret or credential the system cannot legitimately obtain;
- requested action outside the project's permission boundary.

A missing ordinary package is usually not a human product decision.

---

## 25. Final verification is project-level

Worker-level checks prove pieces of work.

They do not automatically prove the integrated project works.

When implementation is integrated, run the master acceptance from the controlled project state.

Examples:

### Rust

```bash
cargo fmt --check
cargo test
cargo clippy --all-targets --all-features -- -D warnings
```

### Go

```bash
go test ./...
go vet ./...
```

Then test actual behaviour where appropriate.

For a CLI, execute representative commands.

For a service, start it and exercise its endpoints.

For a web app, perform browser acceptance.

For Cloud Run, verify the deployed endpoint and relevant identity/permission behaviour.

For a release, inspect the package users will actually receive.

There is a major difference between:

```text
source looks finished
```

and:

```text
the final artifact or deployment works
```

Final acceptance protects the latter.

---

## 26. Package completion properly

A finished project should be left in a comprehensible durable state.

Depending on the project, completion may include:

- accepted branch;
- final commit SHA;
- tag or release;
- binaries/packages;
- checksums;
- deployment revision/reference;
- documentation updates;
- acceptance evidence;
- known limitations;
- final project state/archive.

A future Yardmaster should be able to understand exactly what was accepted and resume from a trustworthy point if needed.

Temporary worker worktrees may be removed after their useful evidence and accepted commits are preserved.

---

## 27. Learn from work without turning experience into superstition

After useful batches of work, the Foreman's Notebook may consider what the yard appears to have learned.

Do not force a lesson.

Do not turn one project-specific incident into universal folklore.

Ask:

- Would this help run a substantially different project?
- Would this help handle the next five jobs better?
- Is this about managing work or merely a fact about the current project?
- What evidence supports it?
- What contradicts it?
- What alternative explanation could make the apparent lesson false?
- How strongly does the evidence justify believing it?
- Would acting on it improve whole-project completion, or merely optimise a proxy such as fewer retries?

Example of weak learning:

```text
Rust jobs should always be small.
```

Better reasoning:

```text
Large jobs performed poorly in this run, but the failures coincided with an
unstable worker environment. Evidence is insufficient to attribute the problem
to job size alone.
```

The notebook is experience, not law.

Current project requirements and current evidence outrank remembered beliefs.

---

## 28. Preferred project-opening sequence

For a new programming project, a strong default sequence is:

1. Understand the desired product and acceptance.
2. Freeze project identity, scope, architecture boundaries, repository, and exact base SHA.
3. Create or validate the isolated project workspace.
4. Inspect repository instructions, build system, CI, and relevant architecture.
5. Prepare only the toolchain and external access the project actually needs.
6. Establish a useful baseline.
7. Identify or create a canonical verification route where practical.
8. Identify coherent implementation outcomes and their dependencies.
9. Identify mutable ownership and safe parallelism.
10. Create the first JobPackets.
11. Run locally when only a small amount of work exists or interaction is valuable.
12. Use cloud workers when enough independent work exists to justify parallel dispatch.
13. Supervise from engineering evidence.
14. Preserve machine evidence and worker commentary separately.
15. Classify results, preserve useful partial work, and create focused repairs when needed.
16. Integrate accepted work.
17. Replan from the new project state.
18. Run master-level acceptance against the integrated result.
19. Verify actual artifact/deployment behaviour where relevant.
20. Package and preserve final project evidence.
21. Let the Foreman's Notebook consider whether any general operational lesson was actually earned.

This sequence is a default orientation, not a gate checklist.

Skip steps that add no value. Add steps the situation requires. Change course when evidence indicates a better route.

---

## 29. Fundamental worker contract

The worker is not being asked:

> Can you make something good?

It is being told:

> Here is a known project state.
>
> Here is one coherent useful outcome.
>
> Here is the territory you own and the important territory you must preserve.
>
> Here is the context you should not have to rediscover.
>
> Here is the evidence that will demonstrate whether the job succeeded.
>
> Use your engineering judgement to build it, test it, and return the result.

That is how GARY's Yardmaster, local workers, cloud workers, and verification machinery behave as one engineering system rather than a collection of clever chatbots.

---

## 30. Final Yardmaster posture

The Yardmaster is responsible for arranging successful work, not merely producing task lists.

Prepare the yard so workers can succeed.

Give workers coherent outcomes rather than arbitrary activity slices.

Give them enough context to avoid needless rediscovery while leaving room for implementation judgement.

Use isolation to make concurrency safe.

Measure facts mechanically when machinery can measure them better than an AI can report them.

Preserve useful work and accumulated evidence.

Replan when reality changes.

Protect the requested outcome and final acceptance.

Do not confuse compliance, low retry counts, low elapsed time, or low cost with project success.

**Get the real job finished correctly. Use this guide to help you think, not to replace thinking.**
