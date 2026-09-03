# GARY FAILURE & RECOVERY MAP

## Diagnose the stage that actually failed

Failure handling follows one rule:

**Repair the layer that failed.**

Do not turn every failure into a Worker retry. Do not turn every Worker failure into project failure. Do not confuse infrastructure, authority, verification, integration, or telemetry defects with implementation defects.

A failed attempt is evidence.

## 1. Recovery principle

When something goes wrong:

1. identify the failed stage;
2. preserve useful work and evidence;
3. determine whether progress remains possible;
4. repair, retry, reroute, replan, wait, block, or cancel at the correct layer;
5. change something material before repeating an unsuccessful attempt.

Never retry merely because retry exists.

## 2. Failure classes

GARY recognises these major classes:

1. project/context integrity;
2. identity/authentication;
3. authority/permission;
4. job construction;
5. provisioning/environment;
6. dispatch/queue/infrastructure;
7. Worker execution;
8. telemetry/handoff;
9. candidate recovery;
10. trusted verification;
11. quality inspection;
12. integration/reconciliation;
13. project acceptance;
14. cost/resource boundary;
15. external dependency;
16. control-plane/state integrity;
17. teardown/cleanup.

## 3. Project or context integrity

Examples include missing Project Packet, digest mismatch, invalid manual provenance, contradictory durable state, or corrupted generation.

Stop new planning and dispatch. Recover from trusted durable source if possible. If correct project truth cannot be established, use a genuine integrity blocker.

Never invent missing project requirements or silently reconstruct authority from model memory.

## 4. Identity or authentication

Repair or refresh the trusted identity mechanism. Retry the affected operation after identity is restored.

Do not send the Worker back to rewrite code or weaken authentication controls.

## 5. Authority or permission

Determine whether the action should actually be permitted.

If yes, repair authority at the narrowest trusted layer. If new human authority is required, use WAITING or BLOCKED as appropriate.

Do not broaden permissions indiscriminately or mistake missing IAM for implementation failure.

## 6. Job construction

Examples include malformed JobPacket, wrong base revision, contradictory scope, missing required context, impossible acceptance, or missing dependency.

Return to Yardmaster planning. Correct the packet or dependency arrangement. Preserve useful work already produced.

Do not rerun the same broken packet.

## 7. Provisioning or environment

Provision the minimum compatible capability, prefer project-pinned versions, smoke-test the environment, then resume if assumptions remain valid.

A missing ordinary installable tool is normally not a human product decision.

## 8. Dispatch, queue, or infrastructure

Determine whether the Worker actually began meaningful work.

If no useful execution occurred, retry infrastructure with suitable backoff.

If useful candidate state exists, recover and inspect it before deciding whether another Worker run is necessary.

Transport failure is not implementation failure.

## 9. Worker execution

Inspect the failure delta.

Choose continue, guide, repair attempt, different or stronger Worker, changed job boundary, or replan.

Preserve useful partial work.

Every retry must use new evidence or a materially different approach.

Repeated equivalent failure is evidence the route should change.

## 10. Telemetry or handoff

Treat telemetry and implementation as separate facts.

Recover Worker artifacts from trustworthy sources and repair the handoff mechanism.

Prefer immutable or append-only evidence objects.

No telemetry does not necessarily mean no work.

## 11. Candidate recovery

If the Worker reports success but the exact branch, commit, patch, or identity cannot be recovered, treat it as a control-plane integrity fault.

An approximate reconstruction cannot be accepted.

## 12. Trusted verification

Classify whether failure belongs to candidate, baseline, verification environment, broken/stale verifier, or infrastructure.

Repair the correct layer.

A non-zero verifier is not automatically a Worker coding defect.

## 13. Quality inspection

Green mechanical checks may still leave semantic or engineering defects.

Choose SMALL REPAIR, MAJOR REPAIR, or REPLAN.

Use direct Yardmaster repair only when the fix is narrow and obvious.

## 14. Integration or reconciliation

Preserve accepted child work.

Treat merge conflict, target advancement, overlap, and integrated failures as reconciliation problems.

Use direct small repair, controlled merge/rebase/cherry-pick, reconciliation Worker, or replan as appropriate.

Reverify the integrated result.

## 15. Project acceptance

If master acceptance fails, diagnose the integrated project and create the smallest repair plan addressing that failure.

Do not reopen every child job by default.

## 16. Cost or resource boundary

If adaptable, replan around available capacity, cheaper capable Workers, more sequential execution, smaller jobs, or reuse of accepted work.

If genuinely hard, use BLOCKED or WAITING with the exact continuation condition.

Requested resource quantity is not sacred unless it is an invariant.

## 17. External dependency

Determine whether there is a safe alternative, independent work, a known WAITING condition, or a genuine blocker.

Do not burn repeated Worker attempts against an unchanged external condition.

## 18. Control-plane or state integrity

Examples include impossible lifecycle transition, terminal resurrection, candidate identity changes, stale projection overwrite, duplicate active Yardmaster ownership, or conflicting generations.

Stop unsafe lifecycle mutation. Preserve evidence. Repair trusted state before resuming ordinary work.

Do not reason around an impossible state.

## 19. Teardown or cleanup

Separate cleanup from product success.

If the result was safely recovered and accepted, cleanup failure does not revoke it.

Retry cleanup independently where worthwhile. Escalate only for meaningful cost, security, or integrity risk.

## 20. Preserve before retry

Before discarding an unsuccessful attempt, preserve useful commits, branches, patches, tests, diagnosis, logs, artifacts, reproductions, environment findings, and integration evidence.

The next attempt should inherit knowledge, not amnesia.

## 21. Retry rule

Retry when the failure appears recoverable, expected value is reasonable, and something material changes.

Material changes include repaired infrastructure, changed Worker, stronger Worker, better context, narrower job, corrected dependency, new diagnosis, or different implementation route.

Raw retry count is not sufficient reasoning.

## 22. Backoff and circuit breakers

Use backoff for likely transient failures such as scheduling, network, service availability, and rate limits.

Use circuit breakers where repeated automatic retries could waste money, duplicate work, overload infrastructure, overwrite evidence, or repeatedly wake a stalled project.

A circuit breaker stops automation. It does not declare the project failed.

## 23. Continue independent work

One failing branch of the project should not freeze unrelated progress when assumptions remain valid, mutable ownership is safe, outputs remain useful, and cost remains justified.

## 24. Escalation

Escalate to the human only when the missing action genuinely belongs to them, such as new permission, altered requirement, irreversible consequential decision, unavailable secret, contradictory frozen requirements, or hard budget increase.

Do not escalate first implementation failure, missing installable tools, ordinary merge conflicts, temporary cloud error, or Worker confusion GARY can resolve.

## 25. WAITING versus BLOCKED

Use `WAITING` when a named external condition can reasonably change and GARY knows what it is waiting for.

Use `BLOCKED` when a hard boundary prevents responsible continuation and no authorised automatic route exists.

Neither means FAILED.

## 26. Project failure

GARY does not use a routine project `FAILED` lifecycle state.

A project can contain many failures and still succeed.

If responsible continuation becomes impossible, record the exact blocker or explicit cancellation rather than inventing a vague terminal failure.

## Recovery invariants

1. Failure is attributed to the correct stage where evidence permits.
2. Infrastructure failure does not automatically become Worker failure.
3. Worker failure does not automatically become project failure.
4. Accepted work survives later integration or infrastructure problems.
5. Useful candidate state is preserved before destructive retry where practical.
6. Equivalent retries require new evidence or a changed route.
7. WAITING names an external condition.
8. BLOCKED names a genuine hard boundary.
9. Control-plane integrity faults stop unsafe state mutation.
10. Cleanup failure does not erase accepted product evidence.
11. Independent useful work may continue when safe.
12. Human escalation is reserved for decisions or authority GARY genuinely does not own.

## Short form

**Find the layer that failed.  
Preserve what is useful.  
Repair that layer.  
Change something before retrying.  
Keep independent work moving.  
Wait for real external conditions.  
Block only on real boundaries.  
Never turn one failed attempt into a false story about the whole project.**
