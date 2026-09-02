# GARY Foreman's Notebook

## Purpose

Implement a small, persistent, human-readable long-term memory for GARY's Foreman/Yardmaster. The notebook is not a rulebook and must not become a new mechanical gate. It exists to help GARY improve how it plans, delegates, supervises, recovers and verifies work across unrelated projects.

Permanent north star:

> Get the real job finished correctly. Learn only what helps you become better at doing that again.

The notebook records GARY's own evidence-weighted beliefs about how to run the yard well. Matthew wants to observe what GARY learns. Humans must be able to read it easily, but normal operation must not require or encourage human curation of beliefs.

## Core doctrine

The notebook exists to improve judgement, not replace it.

Current project requirements and current evidence outrank remembered experience. Notebook beliefs are advisory. They may be wrong, stale or irrelevant. The Yardmaster must be free to contradict them.

A belief must never become a hard gate merely because it appears in the notebook.

"Better" means increasing the probability that meaningful work is completed correctly with trustworthy evidence, while reducing wasted attempts, avoidable cost, repeated mistakes, worker confusion, unnecessary intervention and poor recovery. Fewer worker failures alone is not the objective. Cheaper, faster, smaller or safer is not automatically better if it reduces whole-project completion quality.

## Capacity

The live notebook has at most 20 beliefs.

Do not try to fill 20 slots. Start empty. A reflection may legitimately conclude that nothing worth remembering was learned.

When full, new beliefs compete with existing beliefs for usefulness. Prefer refining or merging an existing belief over creating a near-duplicate.

## What belongs in the notebook

A belief must be about running workers or completing projects in a way plausibly useful across unrelated future projects, for example:

- task sizing and decomposition
- worker/model selection when evidence supports it
- briefing quality
- acceptance design
- supervision and intervention
- distinguishing worker failure from infrastructure failure
- retry, diagnosis and recovery strategy
- concurrency/integration patterns
- tool/capability discovery
- verification quality
- recurring GARY self-failures that should be fixed rather than worked around

Reject project-specific facts such as filenames, APIs, package details, architecture decisions, repository trivia or one-off implementation knowledge.

Admission questions for every candidate belief:

1. Would this plausibly help me run a substantially different project?
2. Would this plausibly help me handle the next five jobs better?
3. Is this about managing work rather than merely a fact about the current project?
4. What evidence supports it?
5. What evidence contradicts it?
6. What alternative explanation could make the apparent lesson false?
7. How strongly does the evidence justify believing it?
8. Would acting on it improve whole-project completion, or merely optimise a proxy such as smaller jobs, fewer failures, lower cost or shorter elapsed time?
9. Does it refine, merge with or replace an existing belief?

## Belief model

Each live belief must be compact and human-readable and carry at least:

- stable ID
- claim/belief text
- rationale or practical implication
- supporting evidence count or concise evidence summary
- contradictory evidence count or concise counterevidence summary
- truth confidence
- generality
- utility
- creation timestamp
- last-updated timestamp
- optional last-challenged timestamp

Use bounded, explicit numeric scores for truth confidence, generality and utility, preferably 0.00 through 1.00. The values must be evidence-informed, not treated as mathematical certainty.

Truth confidence means: how strongly does the evidence support the observation?

Generality means: how likely is the belief to transfer across materially different projects/environments/workers?

Utility means: when GARY acts on this belief, is it likely to improve correct project completion rather than merely a proxy metric?

A belief may be highly true but low utility. Preserve that distinction.

Confidence must move both directions as evidence accumulates. Counterexamples matter. Model/tool/environment changes may make old evidence less relevant.

## Reflection cadence

Implement a cheap bounded reflection mechanism. Prefer reflecting after a small batch of completed child jobs rather than an expensive model call after every trivial event. A default cadence around 3 to 5 accepted child jobs is appropriate, but make the exact cadence easy to configure without making it a hard workflow dependency.

Also allow an explicit manual reflection command for testing/inspection.

Reflection receives recent completed-job evidence plus the current notebook and may KEEP, REFINE, MERGE, REPLACE, DISCARD or DO NOTHING.

It must not manufacture a lesson just because reflection was requested.

A deeper consolidation may occur after a larger amount of evidence (for example 20 to 30 completed jobs), but do not build a complicated scheduler merely to achieve this. Keep the mechanism small.

## Evidence discipline

The reflection prompt must explicitly ask:

> What did this job or batch appear to teach me, and what alternative explanation could make that lesson false?

Avoid confirmation bias. Existing notebook beliefs are hypotheses, not privileged truths. Supporting and contradictory observations must both be considered.

Do not allow a GARY defect to be silently converted into a worker-management belief. When evidence plausibly indicates a supervisor, gate, environment or infrastructure defect, the reflection should record that diagnosis distinctly and prefer exposing/fixing the defect over teaching the Yardmaster to work around it.

## Injection into Yardmaster judgement

Provide the notebook to Yardmaster planning/supervision in a compact clearly delimited section such as FOREMAN'S NOTEBOOK.

Include a permanent header with meaning equivalent to:

> These are evidence-weighted observations from previous work, not requirements. They may be wrong, stale or irrelevant. Current project requirements and current evidence outrank them. Contradict them freely when present evidence justifies it. Never turn a notebook belief into a hard gate merely because it is remembered.

The notebook is capped at 20 and entries are compact, so vector retrieval is not required for v1. Prefer simple deterministic loading over embeddings or a database unless the existing architecture already provides a clearly simpler storage mechanism.

## Human readability and observability

Matthew wants to read what GARY itself learns, not curate it.

Add a simple CLI surface, preferably:

- `gary notebook` or `gary notebook show` to print the current notebook cleanly
- `gary notebook history` to inspect belief evolution/change records
- `gary notebook reflect` to explicitly run a reflection when appropriate

Do not add a normal edit command for humans. Do not require approval of individual beliefs. An emergency corruption/reset mechanism may exist only if consistent with existing GARY operational patterns, but it must not become the normal workflow.

The persisted representation must also be readable directly in the repository/state directory where practical. Preserve a history of changes so belief evolution can be audited. Git-backed history or append-only change records are both acceptable; choose the smallest idiomatic fit for this codebase.

## Persistence and scope

The Foreman's Notebook is global GARY operational experience, not project memory. It must survive project boundaries and process/context-window boundaries.

Do not place the only copy inside one project's state.

Writes must be safe/atomic enough for the existing concurrency model. Avoid creating a new service if a small file-backed or existing-state solution is sufficient.

Do not store secrets, raw prompts containing credentials, giant logs, source-code dumps or private project content in the notebook.

## Telltail readiness

Keep the mechanism observable enough that Telltail can later compare cold GARY versus GARY with notebook experience. Useful signals include notebook revision, belief IDs supplied to a plan, reflection count, belief changes, confidence/generalisation/utility changes, and whether a planner explicitly used or contradicted a belief.

Do not implement a large Telltail integration in this job. Merely expose stable structured evidence/events where this can be done simply.

## Implementation constraints

- Keep this small and idiomatic Go.
- Prefer deleting complexity over adding frameworks.
- Do not introduce embeddings, a vector DB, a separate daemon or a generic knowledge graph for v1.
- Do not introduce new hard gates.
- Preserve current project/master state and unrelated pending jobs.
- Do not weaken final acceptance, provenance, security/authority or explicit cost boundaries.
- Existing GARY rulebook doctrine remains authoritative where compatible with this brief.
- Use GARY's existing planner/model abstraction rather than adding a separate AI provider stack.

## Tests

Add focused tests proving at minimum:

1. notebook starts empty and supports at most 20 live beliefs;
2. project-specific candidate material is rejectable by the reflection contract/prompt;
3. beliefs persist across separate project contexts/process loads;
4. belief history is preserved when an entry is refined/replaced/removed;
5. confidence/generality/utility and supporting/counter evidence are represented and round-trip safely;
6. Yardmaster context includes the notebook plus the advisory/current-evidence-outranks-memory warning;
7. no notebook content is interpreted as a mechanical acceptance/gate rule by storage/injection code;
8. CLI output is human-readable and does not require a human edit path;
9. empty/no-learning reflection is valid;
10. concurrent/atomic persistence follows existing codebase expectations.

Run the repository's normal Go tests and formatting/static checks. Keep tests deterministic; model-backed reflection tests should use fakes/fixtures rather than paid live calls.

## Acceptance

The implementation is complete only when:

- the notebook is persistent and global across projects;
- it holds no more than 20 evidence-weighted beliefs;
- reflection can leave it unchanged;
- beliefs carry truth confidence, generality and utility distinctly;
- reflection explicitly challenges alternative explanations/counterevidence;
- Yardmaster sees the notebook as advisory experience with current evidence taking precedence;
- Matthew can inspect current beliefs and their history from the CLI;
- humans are not expected to curate/edit normal beliefs;
- focused tests pass;
- `go test ./...` passes;
- `git diff --check` passes;
- documentation briefly explains purpose and non-rule semantics.

Do not pre-populate the notebook with invented wisdom. Let GARY earn its own first beliefs from actual work.