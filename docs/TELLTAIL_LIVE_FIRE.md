# Telltail live fire for simplified GARY

This packet is the single authorised Telltail live-fire for the simplified
GARY. It runs only from `master` after the simplification has passed its cheap
CI gates.

## Real job

Use:

```text
gary/work/telltail-v03-single-agent.json
```

It gives one Gemini 3.8 Flash implementation agent the complete existing Telltail 0.3 job packet and the canonical sacrificial seed branch. There is no Yardmaster decomposition and no automatic model escalation.

## What this test is meant to learn

The primary result is not whether GARY followed a prescribed workflow. It is whether one capable agent can actually finish a substantial product job when given sufficient freedom.

Record at least:

- independent acceptance result;
- elapsed wall time;
- provider input/output/thinking usage where available;
- total model and Batch cost;
- number of human interventions;
- whether the agent requested help for something it could have obtained itself;
- genuine blocker quality;
- model-escalation requests and their evidence;
- repeated equivalent failures/circling visible in tool events;
- unavailable/phantom tool calls;
- useful recovery after failure;
- scope drift or unnecessary redesign;
- exact delivered branch/commit.

## Success

A strong result is:

1. the agent works without human intervention inside ordinary engineering authority;
2. it acquires ordinary tools/dependencies itself;
3. it does not cross permission, spend, secret or model boundaries;
4. it either completes the Telltail acceptance or returns a precise genuine blocker;
5. if it completes, the trusted controller reproduces the exact candidate and acceptance passes;
6. the resulting Telltail is usable enough to inspect the observable run evidence afterwards.

## Failure is useful evidence

If the agent fails the implementation, do not automatically rerun it or invoke a stronger model. Preserve the evidence and classify why.

If it requests escalation, Matthew/Lucy decide whether the request is justified before any stronger model is used.

If a failure is infrastructure-only, repair that deterministic layer without pretending the model failed.

## Telltail evaluation boundary

GARY should not grow a permanent Telltail-specific event protocol merely to satisfy this experiment. GARY already preserves Gemini CLI stream JSONL, Batch identity, patch, acceptance output, timing and cost. Telltail or a thin external adapter should consume that observable evidence after the run.

Keep measurement outside the worker wherever possible so the worker cannot grade itself.

## Launch

After the simplification is merged to `master`, the protected workflow launches
this packet once:

```powershell
go run ./cmd/gary work gary/work/telltail-v03-single-agent.json
```

The packet must not be rewritten, decomposed, retried as a paid implementation,
or handed to a second agent.
