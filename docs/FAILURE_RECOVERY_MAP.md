# GARY FAILURE AND RECOVERY

One rule does most of the work:

**Repair the layer that actually failed.**

Do not automatically turn failure into another model call.

## 1. Authority or access blocker

Examples: unavailable credential, inaccessible private resource, missing permission, physical hardware unavailable to the worker, or a consequential action outside the run's authority.

Return `BLOCKED` with exact evidence and the smallest outside action that would unblock the job.

Do not broaden permissions or invent credentials.

## 2. Ordinary environment problem

Examples: missing compiler, SDK, package, CLI, development library, documentation, or compatible open-source tool.

The agent should normally solve this itself when the resource is legitimately obtainable inside its workspace.

Do not bother the human merely because the environment was not pre-baked with every possible tool.

## 3. Implementation failure

The assigned agent should debug, inspect evidence, preserve working changes, and change tactics when useful during its run.

If the run ends without satisfying acceptance, record `FAILED` with the useful evidence. Do not automatically launch a retry, manager, repair agent, or stronger model.

A later retry is a human decision until Telltail demonstrates a specific automatic retry rule is reliably beneficial.

## 4. Model limitation

If the agent concludes that model capability itself is the blocker, it must produce a `model_escalation` blocker with concrete evidence explaining why a stronger model is likely to change the outcome.

The run stops. The model never changes automatically.

## 5. Batch or cloud failure

Separate infrastructure from implementation.

Preserve any candidate patch, acceptance output, usage telemetry, and logs that already exist. A provider failure with no meaningful implementation work may be retried only by deterministic infrastructure policy where duplication and spend remain controlled.

Do not describe a queue outage as bad code.

## 6. Acceptance failure

The trusted controller runs the declared checks against the exact recovered candidate.

If the candidate fails them, record the failure evidence. Do not ask a second AI to reinterpret a mechanical non-zero exit code before recording the fact.

If the verifier itself lacks a required legitimate environment capability, repair the verifier/environment rather than blaming the candidate.

## 7. Delivery failure

If the candidate passed independent acceptance but cannot be pushed because authentication or the remote service is unavailable, preserve the exact verified candidate and record `BLOCKED`.

Do not reimplement accepted code to fix a GitHub write problem.

## 8. Cost or runtime boundary

Configured cost and runtime ceilings are hard controller boundaries.

If completion needs more authorised budget or a longer ceiling, return the evidence to the human. The implementation agent cannot increase its own allowance.

## 9. Contradictory or impossible job

The agent should first establish that the contradiction is real rather than merely inconvenient.

If two required outcomes cannot both be true, return `BLOCKED` with the conflicting requirements and evidence. Do not silently weaken one requirement.

## 10. Preserve useful evidence

Where practical retain the original packet, exact base identity, candidate patch, declared acceptance output, provider identity, usage/cost record, blocker record, and concise infrastructure logs.

Do not keep giant chains of model reasoning or supervisor chatter merely because they existed.

## Recovery principle

**Solve what GARY can solve. Stop only at a real boundary. Preserve evidence. Never spend another intelligent agent automatically just because the first attempt was difficult.**
