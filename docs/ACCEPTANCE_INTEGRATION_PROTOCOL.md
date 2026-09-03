# GARY ACCEPTANCE & INTEGRATION PROTOCOL

## From Worker candidate to project progress

This protocol defines how returned Worker work becomes accepted project state.

The normal path is:

**RETURN -> RECOVER -> VERIFY -> INSPECT -> REPAIR IF NEEDED -> ACCEPT -> INTEGRATE -> REASSESS**

## 1. Returned work is a candidate

A Worker may return candidate-ready work, useful partial work, a blocker, or an unsuccessful result.

Any recoverable implementation work should be preserved before judging the outcome.

Transport or teardown failure must not destroy useful implementation evidence.

## 2. Recover the exact candidate

Before verification, establish exactly what is being judged:

- repository;
- candidate branch;
- candidate commit;
- base revision;
- JobPacket identity;
- project generation.

Verification and inspection must refer to the same candidate.

If the candidate cannot be recovered reliably, it cannot be accepted.

## 3. Trusted mechanical verification

Run required mechanical checks in a trusted controlled environment against the exact candidate.

Worker-reported results are useful evidence. They do not replace trusted reruns where trusted reruns are required.

## 4. Classify verification failures

A failed verification may belong to:

- candidate;
- baseline;
- verification environment;
- verifier;
- infrastructure.

Repair the layer that failed.

Do not send code back to a Worker because the verifier lost network access.

## 5. Semantic inspection

After mechanical checks pass, the Yardmaster inspects the actual candidate.

Judge whether it satisfies the assigned objective, preserves invariants, respects scope and authority, fits surrounding architecture, avoids obvious harmful side effects, contains appropriate tests, avoids suspicious unrelated changes, and meets a reasonable engineering standard.

## 6. Decisions

### PASS
The exact candidate is fit to integrate.

### SMALL REPAIR
The approach is sound and only narrow obvious defects remain.

Prefer direct Yardmaster repair when safely bounded. After repair, record the change, create a new exact candidate identity, rerun trusted verification, inspect the final diff, and make a fresh decision.

### MAJOR REPAIR
Useful work exists but substantial correction is required.

Preserve the candidate and dispatch a repair Worker with exact state, what is correct, what failed, trusted evidence, diagnosis, preservation requirements, and required changes.

### REPLAN
The problem is not merely defective code. Change route, job boundary, dependency assumption, Worker, or authority arrangement.

## 7. Yardmaster small-repair boundary

Direct Yardmaster repair is intended to remove pointless ceremony.

Use it when the defect is local, the intended correction is clear, no meaningful architecture choice is introduced, semantics remain obvious, the change is small enough to review immediately, and project-level attention is not being consumed by implementation work.

Do not use it for substantial new code, ambiguous behaviour, redesign, or meaningful product decisions.

## 8. Child acceptance record

A child result becomes accepted only when GARY records at least:

- child job ID;
- accepted candidate identity;
- base revision;
- JobPacket identity;
- trusted verification result;
- Yardmaster quality decision;
- acceptance timestamp;
- relevant evidence references.

## 9. Accepted work is immutable progress

Once accepted, the exact child result is durable project progress.

Do not casually discard, regenerate from memory, rerun from scratch, replace with an unverified equivalent, or silently modify it before integration.

Any modification creates a new candidate requiring verification.

## 10. Integration ownership

Once child work is accepted, integration becomes a Yardmaster/control-plane responsibility.

The Worker does not need to remain alive merely to merge its own work.

## 11. Clean integration

If the accepted result applies cleanly to controlled project state, integrate it.

Record source accepted commit, target state, resulting integrated commit, method, and timestamp.

## 12. Integration conflict

A conflict is a new reconciliation problem.

It does not revoke the fact that the child result was accepted against its original base.

For tiny obvious reconciliation, the Yardmaster may fix directly and reverify.

For significant reconciliation, dispatch a bounded reconciliation Worker.

If assumptions became invalid, replan.

Do not blindly rerun the original child job.

## 13. Target advancement

If controlled state advanced after the Worker began, choose the safest suitable merge, rebase, cherry-pick, manual reconciliation, reconciliation Worker, or replan route.

The resulting integrated state must be known, recoverable, and verified.

## 14. Dependent work

Dependent jobs should normally begin from integrated state containing their accepted dependencies.

Do not mutate an already-running Worker's checkout underneath it.

## 15. Post-integration verification

Integration can create failures neither child had independently.

Run verification appropriate to the risk. Do not mechanically rerun every expensive test after every tiny integration when narrower evidence is sufficient.

## 16. Integration failure is not child failure

These can both be true:

- the child implementation was accepted;
- current integration failed.

Repair the integration layer. Do not falsify child history.

## 17. Reassess after integration

Every successful integration changes project truth.

Recalculate unsatisfied requirements, dependencies, jobs still necessary, newly discovered work, and acceptance readiness.

## 18. Project-level acceptance

Child acceptance proves a bounded outcome. Project acceptance proves the whole requested result.

When sufficient work is integrated:

1. rehydrate project truth;
2. identify master acceptance conditions;
3. run required mechanical checks;
4. perform semantic/project inspection where needed;
5. verify the actual deliverable where relevant.

Only project-level acceptance may transition lifecycle to `ACCEPTED`.

## 19. Project acceptance failure

Diagnose the integrated project.

Do not automatically reopen every child job.

Preserve valid child acceptance history unless evidence proves the prior child acceptance itself was invalid.

## 20. Monotonic acceptance

Once project acceptance is durably recorded, late Worker failures, delayed cloud events, stale workflow writes, old retries, and watchdogs cannot resurrect it.

A later discovered defect is a new maintenance or explicit reopen decision, not permission for stale machinery to rewrite history.

## 21. Integrity faults

Treat inconsistent combinations as control-plane faults, including:

- accepted child with no recoverable candidate;
- PASS without required trusted checks;
- integrated result with unknown source candidate;
- project marked accepted without master acceptance evidence;
- terminal state overwritten by stale active state;
- candidate identity changing between verification and inspection.

Preserve evidence and repair control state.

## Invariants

1. Worker completion is not child acceptance.
2. Trusted verification examines the exact candidate.
3. Yardmaster inspection examines that same candidate.
4. Yardmaster repair creates a new candidate.
5. Accepted child work has a recoverable identity.
6. Accepted work is not silently changed before integration.
7. Integration is a control-plane responsibility.
8. Integration conflict does not erase child acceptance.
9. Dependent work uses appropriate integrated state.
10. Project acceptance is separate from child acceptance.
11. Only project acceptance means success.
12. Terminal project outcomes cannot be undone by stale execution events.

## Short form

**Worker returns a candidate.  
Trusted machinery checks it.  
The Yardmaster judges it.  
Tiny defects may be fixed directly.  
Substantial defects go through a Worker.  
Accepted work is preserved and integrated.  
Integration problems are solved as integration problems.  
Only master acceptance means finished.**
