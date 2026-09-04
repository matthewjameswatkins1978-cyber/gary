# GARY WORK STATES

GARY does not need a project-management state machine to run one agent.

## Administrative control

`PARKED` is a human-controlled enable/disable condition for GARY itself. While GARY is parked, no `gary work` execution should be launched. Parking is not an agent reasoning state and has no automatic wake transition.

Only Matthew's explicit instruction to unpark GARY authorises work to resume.

## Work state

A launched job has one useful live state:

- `RUNNING` — its single Batch worker is active or being observed.

It ends in exactly one recorded outcome:

- `DONE` — the exact candidate passed trusted independent acceptance and was delivered;
- `BLOCKED` — responsible continuation requires something outside GARY's current authority or reach;
- `FAILED` — execution or implementation ended without satisfying acceptance and without a genuine external blocker.

These are records of what happened, not prompts telling the model how to think.

## BLOCKED

A blocker must identify a real boundary, such as unavailable credentials, inaccessible account/hardware, permission increase, material unapproved spend, consequential irreversible action, contradictory requirements needing a human decision, or a requested stronger model.

A blocker records:

```text
kind
summary
needed
evidence
unblock_action
```

Missing installable tools, ordinary dependency problems, first implementation failures, merge conflicts, unfamiliar code, and routes that merely need debugging are not blockers.

Resolving a blocker does not silently resume the old run. The human may choose to launch a new run with the changed authority or input.

## FAILED

`FAILED` is intentionally ordinary. It means the attempt did not finish the job and no outside authority boundary explains why.

GARY does not automatically retry a failed implementation with another model or another agent. The failure evidence is returned for a human decision until Telltail evidence justifies a narrower automatic recovery rule.

Infrastructure retries may happen only inside deterministic provider/tool behavior where they cannot duplicate meaningful implementation work or cross the configured cost/runtime boundary.

## DONE

`DONE` requires all of the following:

1. a recoverable exact candidate;
2. the trusted controller reproduces that candidate from the declared base;
3. the declared acceptance commands pass there;
4. the exact verified commit/branch is delivered successfully.

Batch `SUCCEEDED` and agent self-report are not `DONE`.

## What is deliberately absent

There are no normal project states for planning, executing, verifying, integrating, waiting, waking, child acceptance, waves, supervisor turns, or Yardmaster ownership.

Those concepts either describe ordinary work inside the agent or belonged to the retired multi-agent architecture.

## Invariants

1. Parked GARY launches no work.
2. One `gary work` invocation launches one implementation agent.
3. The assigned model cannot change during the run.
4. Model escalation requires human approval.
5. A genuine blocker is explicit and evidenced.
6. Independent acceptance, not agent prose or Batch status, awards `DONE`.
7. Failure does not automatically trigger another intelligent agent.
