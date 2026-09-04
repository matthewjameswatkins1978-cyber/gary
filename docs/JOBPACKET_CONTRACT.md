# GARY JOB CONTRACT

GARY receives one complete human job. It does not manufacture child jobs before implementation.

## Required controller fields

The `gary work` manifest contains:

```text
id
repo
base_branch
packet_path OR task
acceptance[]
```

Optional controller limits may include a runtime ceiling and a human-set cost ceiling.

There is deliberately no worker count, wave count, planner model, closer model, automatic retry count, or automatic model-routing field.

## The packet

The packet is the actual job specification supplied by the human. It may be long. GARY should not require the human to pre-condense it, and no planning agent should replace it with a lossy summary.

A good packet normally communicates the intended result, requirements, important constraints, useful context, acceptance expectations, and any genuinely fixed decisions.

It may include implementation suggestions, but unless the method itself is a requirement the implementation agent is free to choose a better route after inspecting reality.

## Authority

The packet may describe the product. It cannot grant authority that the runtime does not possess.

The agent may use ordinary development resources already available to it and may obtain normal tools, packages, SDKs, libraries and documentation when permitted.

The following remain outside automatic authority unless explicitly provided or approved: unavailable secrets, broader permissions, inaccessible accounts/hardware, material new spending, consequential irreversible external actions, and a stronger model.

## Context discipline

Do not automatically append every GARY manual, previous project transcript, tool guide, or historical incident to the packet.

Permanent GARY doctrine stays small. Tool-specific guidance should be retrieved only when the agent actually needs that tool.

## Immutability

Preserve the original packet exactly as durable intake evidence. The agent's evolving implementation plan does not modify that source of truth.

If the human changes requirements, that is a new authorised input for a later run. Do not silently rewrite an active job underneath the agent.

## Acceptance

Acceptance commands belong to the trusted controller and are visible to the agent. The agent may run them while working, but cannot redefine or weaken them.

The controller reruns the declared acceptance against the exact returned candidate before awarding `DONE`.

## Genuine blockers

If the job cannot responsibly continue without something outside GARY's reach or authority, the agent writes `.gary-blocked.json` with:

```text
kind
summary
needed
evidence
unblock_action
```

The file is returned out of band and removed before repository commit.

Do not use a blocker for ordinary engineering friction that the agent can solve itself.

## Short form

**Give the agent the real job. Preserve it. Let the agent solve it. Keep authority and acceptance outside the model.**
