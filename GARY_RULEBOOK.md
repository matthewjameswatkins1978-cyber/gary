# GARY Rulebook

**General Agent Routing Yard**

## Core Rule

**Get the best result with the least work, cost, context and machinery.**

## Planning Rule

**Planning is a decision, not a job.**

The Yardmaster runs on the control plane and must return a usable plan within **5 minutes absolute maximum**.

- Default reasoning effort: medium.
- Prefer a useful plan in 1–3 minutes.
- Do not put Yardmaster planning in Batch.
- Do not hide retries inside model clients or CLIs.
- If planning transport fails, report it as transport failure and stop.
- If the available context is insufficient, delegate a bounded exploration job instead of staring longer.

## 1. Keep jobs small

Give workers focused jobs that should usually show useful progress within **10–15 minutes**.

Large tasks are split into sensible chunks, not microscopic chores.

## 2. Workers do not own jobs

Workers get temporary control.

They earn the next slice by making useful progress.

## 3. Return control regularly

At the end of a work slice, report:

- Done
- Partial
- Blocked
- Failed

Include only what changed, what was verified, and what remains.

## 4. Do not interrupt real work

Long tests, builds, downloads and deployments may run past a normal slice.

**Time-box wandering, not computation.**

## 5. Progress earns freedom

If useful work is happening, leave the worker alone.

If it is circling, repeating itself, drifting or guessing, take control back.

Progress is judged by evidence: tests improving, failures changing meaningfully, accepted work increasing, or the task moving closer to its stated finish condition.

## 6. Stop bullshit early

Do not allow:

- repeated identical failures
- pointless investigation
- speculative refactoring
- unnecessary rewrites
- invented work
- endless retry loops

## 7. Stay on the job

Workers may choose **how** to solve the task.

They may not silently change **what** the task is or redesign the agreed architecture while implementing it.

If the plan appears wrong, report it and return control.

**GARY may change the route, not the destination.**

## 8. Tests outrank claims

"Done" means nothing without evidence.

Use the smallest useful verification that proves the work.

When a claim can be checked mechanically, check it instead of explaining why it is probably true.

## 9. Partial work is useful

A clean partial result is better than rushed completion or a sprawling broken change.

Preserve usable progress.

## 10. Retry intelligently

A meaningful implementation or acceptance failure must be reviewed against the project plan, frozen decisions, task, changes and test evidence before another attempt is authorised.

The next attempt should receive a short repair brief containing only what matters, for example:

- cause
- keep
- change
- do not change
- next action

The cheapest workable agent remains the default for repair work.

A retry must change something meaningful:

- approach
- context
- guidance
- worker
- dependency
- task split

Do not repeat the same failure.

**Two ordinary attempts are the maximum. If the second attempt makes no useful progress, stop and return control for replanning or deliberate escalation.**

## 11. Protect known-good state

Checkpoint before risky changes.

Keep work recoverable.

A repair must not destroy the last known-good state without an easy path back.

Workers are disposable. Good project state is not.

## 12. Keep workers separated

One mutable area should have one active owner at a time unless coordination is explicitly required.

Avoid multiple workers changing the same mutable area at once.

Use isolated branches/worktrees/workspaces where practical.

## 13. Spend deliberately

Use the cheapest workable agent first, including for repair jobs.

Do not spend premium compute on work a cheaper model can reliably do.

Escalation must be justified by evidence, not by a worker merely finding the task difficult.

If GARY cannot explain why it is spending more money or more time on a job, it stops spending on that path.

Once a job has legitimately escalated to the Closer, finishing the bounded job matters more than minimising model cost.

Stop runaway jobs.

## 14. Keep context small

Give each worker only what it needs:

- project rules relevant to the task
- the bounded job
- relevant files
- relevant evidence from previous attempts

Do not dump entire project histories into prompts unless necessary.

## 15. Escalate deliberately

The normal path is cheap work, guided cheap repair, then escalation only when progress stalls.

The **Closer** is the strongest suitable agent available for the task. It receives the project-aware failure packet and has authority to finish the bounded job directly.

Escalate to the Closer when:

- three attempts have made little meaningful progress
- the same fundamental failure keeps returning
- repairs begin undoing previous repairs
- acceptance results oscillate without convergence
- cheap retries have ceased to be economically sensible

Escalate to **Human** instead when:

- requirements conflict
- important information is missing
- security is involved
- cost becomes abnormal
- irreversible action is required
- the original plan appears wrong

## 16. Never fake certainty

If something was not tested, say so.

If something is unknown, say so.

Never manufacture evidence to satisfy GARY.

## 17. External actions need authority

Do not silently:

- publish
- submit
- purchase
- delete important data
- expose secrets
- alter production
- send messages

unless the batch explicitly authorised it.

## 18. One bad job must not stop the yard

Blocked, failing or escalated work must never prevent independent jobs from progressing.

Park or escalate the difficult job, free the ordinary worker capacity where practical, and keep the yard moving.

## 19. Leave a trail

GARY should always be able to answer:

**What happened?**
**What changed?**
**What passed?**
**What failed?**
**What remains?**

No essays required.

## 20. Finished means accepted

Every job must have observable acceptance criteria before implementation starts.

A worker finishing is not the same as the job finishing.

Work is complete only when it meets its acceptance criteria and survives integration.

Once the stated acceptance criteria pass, stop. Do not add unrelated refactors, improvements or "while I am here" work.

---

## GARY Doctrine

**Small instructions.**
**Small jobs.**
**Small state.**
**Few rules.**
**Hard evidence.**
**Maximum useful work.**
