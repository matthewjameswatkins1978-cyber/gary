# GARY JOBPACKET CONTRACT

## Canonical Worker handoff

A JobPacket is the complete bounded instruction set given to a Worker.

Its purpose is simple:

**Give the Worker enough truth, authority, and evidence to complete one useful outcome without making it rediscover the project.**

A JobPacket is not a miniature Project Packet.

## 1. Required information

Every JobPacket identifies:

### Identity
- project ID;
- child job ID;
- project generation;
- packet version or fingerprint.

### Objective
One coherent useful outcome. State what must become true.

### Starting state
Where relevant:
- repository;
- exact base commit;
- branch or workspace identity;
- accepted dependencies already integrated.

### Relevant context
Only the project facts needed for this job.

### Invariants
Facts the Worker must preserve.

### Scope
The territory the Worker owns and important areas it must protect.

### Authority
What the Worker may and may not do.

### Acceptance
Mechanical checks and semantic expectations that genuinely demonstrate the child outcome.

## 2. Optional information

Include only when useful:

- known evidence;
- repair guidance;
- dependency notes;
- required or pinned toolchain information;
- external interfaces.

Never include secrets.

## 3. Do not put these in every packet

Do not include:

- the whole Project Packet unless genuinely required;
- unrelated project history;
- sibling Worker instructions;
- Yardmaster internal planning;
- speculative future work;
- model-routing tables;
- broad cloud credentials;
- reusable personal credentials;
- production secrets;
- long logs when an excerpt is enough;
- obsolete contradictory instructions;
- hidden acceptance criteria.

## 4. Outcome, not method

Normally specify **what must be achieved**, not a rigid implementation recipe.

Prescribe method only when the method itself is an invariant or authorised project decision.

## 5. Small context, complete context

Small context does not mean incomplete context.

Include every fact whose absence would reasonably cause the Worker to violate an invariant, repeat known failed work, misunderstand the objective, modify the wrong state, or fail acceptance.

Everything else should earn its place.

## 6. Worker freedom

Within the packet boundaries, the Worker may use normal engineering judgement.

It may inspect nearby relevant code, choose implementation details, add narrowly useful tests, use existing project tools, install ordinary compatible development dependencies where authorised, investigate unexpected behaviour, and take a safer equivalent route when the obvious one fails.

It does not need permission for every ordinary development decision.

## 7. Scope is not a trap

Scope prevents unrelated work and conflicting ownership.

If a tiny supporting change just outside a named file list is clearly required and does not violate ownership or authority, the Worker may make it and report it.

If completion requires substantial work outside the intended boundary, return the dependency to the Yardmaster rather than silently expanding the job.

## 8. Acceptance must match the job

Child acceptance should be focused enough to give fast useful feedback and still support master acceptance.

Mechanical acceptance and semantic requirements should be distinguishable.

## 9. Repair packets preserve progress

A repair packet must not pretend the previous attempt never happened.

Include the exact useful candidate where possible and state:

- what passed;
- what failed;
- relevant trusted verification output;
- Yardmaster diagnosis;
- what should be kept;
- what should change.

## 10. Packet immutability

Once dispatched, a JobPacket is immutable.

Do not rewrite it underneath a running Worker.

If circumstances materially change, either let the Worker continue if assumptions remain valid, stop it if continuing would waste effort or create unsafe output, or issue a new versioned packet.

## 11. Packet identity and candidate traceability

Record enough to prove which project generation created the packet, which exact base revision it used, which packet version was dispatched, which Worker received it, and which returned candidate belongs to it.

## 12. Result contract

The Worker returns:

- concise status;
- candidate branch/commit/patch or equivalent artifact where work exists;
- actual checks run and results;
- material changes;
- unresolved concrete dependencies or blockers.

The label is evidence, not acceptance.

## 13. Worker self-report is evidence

The Worker must not award itself acceptance, claim tests it did not run, conceal failures, weaken acceptance to make itself pass, or declare the wider project complete.

## 14. Construction test

Before dispatch, the Yardmaster should be able to answer:

**Could a capable Worker complete this outcome correctly from this packet and repository state without reconstructing the whole project?**

If no, improve the packet.

Then stop adding context.

## Canonical conceptual shape

```text
job_id
project_id
project_generation
packet_version
base_revision

objective

context
invariants
scope
authority
dependencies

acceptance:
  mechanical
  semantic

known_evidence
repair_guidance
toolchain

result_contract
```

Exact schema belongs in code.

## Short form

**One useful outcome.  
One known starting point.  
Enough context.  
Clear boundaries.  
Visible acceptance.  
No unnecessary history.  
Let the Worker work.**
