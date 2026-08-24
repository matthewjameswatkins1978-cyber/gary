# GARY Rulebook

**General Agent Routing Yard**

## Core Rule

**Get the best result with the least work, cost, context and machinery.**

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

They may not silently change **what** the task is.

**GARY may change the route, not the destination.**

## 8. Tests outrank claims

"Done" means nothing without evidence.

Use the smallest useful verification that proves the work.

## 9. Partial work is useful

A clean partial result is better than rushed completion or a sprawling broken change.

Preserve usable progress.

## 10. Retry intelligently

A retry must change something meaningful:

- approach
- context
- model
- worker
- dependency
- task split

Do not repeat the same failure.

## 11. Protect known-good state

Checkpoint before risky changes.

Keep work recoverable.

Workers are disposable. Good project state is not.

## 12. Keep workers separated

Avoid multiple workers changing the same mutable area at once.

Use isolated branches/worktrees/workspaces where practical.

## 13. Spend deliberately

Use the cheapest capable worker.

Do not spend premium compute on work a cheaper model can reliably do.

Stop runaway jobs.

## 14. Keep context small

Give each worker only what it needs.

Do not dump entire project histories into prompts unless necessary.

## 15. Escalate real uncertainty

Workers should solve ordinary problems themselves.

Escalate when:

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

Blocked or repeatedly failing work gets parked for review.

Independent work continues.

## 19. Leave a trail

GARY should always be able to answer:

**What happened?**
**What changed?**
**What passed?**
**What failed?**
**What remains?**

No essays required.

## 20. Finished means accepted

A worker finishing is not the same as the job finishing.

Work is complete only when it meets its acceptance criteria and survives integration.

---

## GARY Doctrine

**Small instructions.**
**Small jobs.**
**Small state.**
**Few rules.**
**Hard evidence.**
**Maximum useful work.**
