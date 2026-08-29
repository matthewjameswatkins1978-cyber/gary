# GARY MASTER JOB: MAKE GARY SAFER IN SOURCE, THEN REVIEW GARY AS A SYSTEM

You are GARY working on the GARY repository itself.

This is a bounded self-improvement job. You are not being asked to redesign yourself, replace your architecture, or autonomously decide what GARY should become.

## What GARY is for

The intended shape of GARY is:

- A strong Yardmaster keeps long-term task context and makes higher-level decisions.
- Cheap workers do bounded implementation work.
- Workers are mechanically verified.
- Failed workers should not blindly repeat the same approach.
- Expensive intelligence should sit mainly at decision points, not routine labour.
- Useful partial work should be preserved.
- Repeated low-progress work should be stopped or replanned.
- Cost must remain visible and bounded.
- Human review remains outside the final acceptance and merge boundary.
- GARY should stay small, understandable, maintainable and economical.

The design philosophy is: **least machinery that reliably gets the job done.** Prefer a few strong rules over layers of policy, prompts, state and orchestration.

Do not turn this task into an architecture exercise. Finish the concrete safety work first.

# PART A: IMPLEMENT ONE CONCRETE IMPROVEMENT

Permanently move the current runtime safety guards into source code so workflows no longer need to rewrite Go source before execution.

Required outcomes:

1. Ordinary implementation jobs default to no more than 2 paid attempts.
2. Retry 2 receives previous failure and acceptance evidence, including when attempt 1 produced no patch.
3. No paid repair-analysis model call occurs between ordinary worker attempts.
4. Normal worker paths do not automatically invoke the Closer model after ordinary failure.
5. The configured per-job cost limit is checked before every paid attempt, not only before attempt 1.
6. `GARY_MAX_WORKERS` is a hard ceiling, including when a master manifest requests more workers.
7. An interrupted or existing non-final master run is not automatically duplicated.
8. Paid cross-repository work requires the dedicated GARY GitHub App credential before cloud work starts.
9. Direct `gary run` and `gary run-many` remain fail-closed unless the explicit break-glass override is supplied.
10. Once equivalent source protections are proven, remove workflow code whose only purpose is rewriting Go source at runtime.

Keep the implementation small and deterministic. Reuse existing structures and tests. Do not add a new orchestration layer, policy engine, database, service, framework or model router.

Do not change unrelated product behaviour.

## Part A acceptance

Provide focused regression tests proving the safeguards above, then run:

- `go test ./...`

Also verify mechanically that the automatic workflow YAML no longer rewrites Go source at runtime.

Do not run a paid cloud smoke test unless local/static verification genuinely cannot establish correctness. If you believe a paid test is necessary, stop and record why instead of silently launching it.

All implementation output must remain on a separate GARY staging/worker branch. Do not merge to `master`.

# PART B: THEORETICAL GARY IMPROVEMENT REVIEW

After Part A is complete, inspect GARY as a whole and create:

`docs/gary-theoretical-improvements.md`

**PART B IS ADVISORY ONLY. DO NOT IMPLEMENT ANY IDEA FROM THIS SECTION.**

Think carefully about what could make GARY better at:

- preserving context across long jobs;
- choosing good worker boundaries;
- detecting when a retry is actually worthwhile;
- distinguishing transient infrastructure failure from reasoning or coding failure;
- preserving useful partial work;
- knowing when to replan rather than repair;
- controlling cost without becoming timid;
- choosing cheap versus strong models at the correct decision points;
- improving verification and acceptance quality;
- avoiding duplicate work;
- making failures easier to diagnose;
- making long master jobs more reliable;
- remaining simple rather than becoming an orchestration framework.

For every proposed improvement include:

1. Problem it solves.
2. Proposed idea.
3. Expected benefit.
4. Complexity or cost introduced.
5. Likely failure modes.
6. Recommendation: `KEEP SIMPLE`, `CONSIDER`, or `AVOID`.
7. Timing: now, later, or probably never.

Include a section titled:

## Things GARY Should Probably Never Become

Explicitly challenge ideas that would turn GARY into:

- an autonomous product manager;
- an architecture-debate machine;
- a giant multi-agent bureaucracy;
- a self-merging or self-deploying system;
- a system that uses strong expensive models everywhere;
- a controller with so much state or policy that maintaining GARY becomes harder than doing the work.

End the document with a ranked list of at most **5 theoretical improvements** offering the highest expected gain for the least added machinery.

Again: do not implement Part B ideas.

# FINAL OUTPUT

Return evidence that clearly states:

- staging/worker branch or branches used;
- exact files changed;
- focused test results and `go test ./...` result;
- whether any paid cloud action was launched;
- unresolved risks or uncertainty;
- location of `docs/gary-theoretical-improvements.md`;
- confirmation that nothing was merged to `master` by GARY.

Prioritise correctness, bounded cost and simplicity over ambition.
