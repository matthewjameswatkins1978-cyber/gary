# GARY ACCEPTANCE PROTOCOL

GARY has one implementation agent and one deterministic acceptance boundary.

The normal path is:

**AGENT WORKS -> CANDIDATE -> CLEAN REPRODUCTION -> ACCEPTANCE -> DELIVER**

## Candidate

The agent's result is a candidate, not success. Preserve the exact patch and supporting evidence before judging it.

Batch `SUCCEEDED` means the disposable container finished. Agent prose saying "done" means the agent thinks it finished. Neither awards `DONE`.

## Clean reproduction

The trusted controller:

1. clones the declared repository;
2. checks out the declared base branch/revision;
3. creates the result branch;
4. applies the exact returned patch;
5. prepares only the ordinary environment required by the declared checks;
6. runs the declared acceptance commands.

If the exact candidate cannot be recovered, it cannot be accepted.

## Acceptance result

If the declared checks pass, the controller commits and pushes that exact verified candidate. Successful delivery records `DONE`.

If the checks fail because the candidate is wrong, the run records `FAILED` with the concrete evidence. It does not automatically summon another agent.

If verification itself is prevented by a genuine authority or infrastructure boundary, record `BLOCKED` where appropriate rather than pretending the code failed.

## Semantic judgement

A permanent second AI quality judge is intentionally absent.

If a particular job needs semantic review that mechanical checks cannot establish, make that requirement explicit in the job's evaluation plan or Telltail test. Do not impose a second model call on every piece of work merely because one might sometimes be useful.

## Delivery

A candidate is not `DONE` until the exact independently verified commit/branch is successfully delivered to the target repository.

A writeback/authentication failure after successful verification is a delivery blocker. Do not reimplement the code because GitHub credentials failed.

## Existing work

If inspection shows the requested result already exists and the declared acceptance passes without repository changes, that should be treated as a valid no-change completion once the controller supports that case explicitly. Do not manufacture a meaningless patch simply to satisfy bookkeeping.

## Invariants

1. The implementation agent cannot award itself success.
2. Batch state is transport evidence, not product acceptance.
3. The controller verifies the exact returned candidate from the declared base.
4. Declared checks cannot be weakened by the implementation agent.
5. Model escalation is never an acceptance-repair mechanism.
6. Delivery failure is not implementation failure.
7. No permanent AI manager or quality judge sits between candidate and mechanical acceptance.
