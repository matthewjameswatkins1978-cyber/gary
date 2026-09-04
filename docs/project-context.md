# Durable work evidence

GARY does not maintain a second AI-authored project manual or a multi-agent lifecycle as active context.

For each real job, preserve only durable information that is valuable after context or infrastructure disappears:

```text
original job manifest
original human job packet
repository and base identity
assigned model
configured cost/runtime boundary
Batch job identity
candidate patch/artifacts
acceptance output
usage/cost record
result branch and commit
blocker evidence, when present
```

## Original packet

The human packet is source truth and should be stored byte-for-byte at intake, with a digest where useful for integrity checking.

Do not replace it with a model summary. Summaries may be created temporarily by the implementation agent for its own working convenience, but they are not project authority.

## Active context

The implementation agent receives the complete job plus the small GARY constitution. Additional tool or project documentation should be retrieved only when relevant.

Old conversations, planner memories, child-job histories, lifecycle event streams, project boards, and supervision telemetry are not default prompt material.

## Results

The result record should make it possible to answer:

- what was requested;
- what repository/base was used;
- which model ran;
- what candidate was returned;
- what independent acceptance showed;
- what was delivered;
- what it cost;
- whether a genuine blocker stopped the job.

That is enough durable truth for the simple architecture.

## Historical project machinery

Older `gary/projects/` packets, manuals, lifecycle streams, board projections, feedback records and wake state remain historical evidence in Git history and existing repositories. They are not part of the new `gary work` execution contract.
