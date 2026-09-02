# GARY Gate Audit — adversarial simplification

## Objective

Audit GARY's current mechanical/hard gates and remove or soften any gate whose likely downside outweighs the failure it prevents. Preserve GARY's core character: intelligent completion, few hard rules, hard evidence, cheap recovery, and Yardmaster judgement.

This is not a general redesign. Make the smallest coherent changes that reduce self-inflicted blocking while preserving safety, truth and recoverability.

## Governing test for every hard gate

For each current mechanical gate, reason from both sides before changing it:

1. Best case: what concrete bad outcome does the gate prevent, and how costly is that outcome?
2. Worst case: how can the gate falsely block healthy work, deadlock the yard, discard useful work, force unnecessary replanning, or turn a recoverable condition into failure?
3. Blast radius: if the gate is wrong, does it waste one attempt, one job, one wave, or stop the project?
4. Escape route: can the Yardmaster recover or override safely from evidence?
5. Cheaper alternative: could warning, observation, guidance, recovery, bounded retry, or post-check provide the benefit without blocking progress?
6. Gate-bug test: if the gate itself is buggy or misclassifies the situation, what happens?

Default posture: OBSERVE -> WARN/GUIDE -> REASON -> RECOVER. Use BLOCK only when allowing progress risks materially worse harm.

## Strong presumption to KEEP

Do not weaken gates that protect truth, authority, irreversible state, or externally bounded spend. In particular, preserve unless strong code evidence shows a defect:

- project/repository identity and provenance integrity;
- permission/security/destructive-action boundaries;
- explicit cost ceilings;
- final mechanical acceptance and independent final semantic inspection;
- prevention of fake/empty writeback or unverified completion;
- isolation/concurrency protections needed to prevent conflicting mutable ownership;
- genuine emergency platform/runtime ceilings needed to prevent orphaned or runaway infrastructure.

These should fail closed when their protected condition is genuinely violated.

## High-priority gates to attack adversarially

### 1. Economy gate that forces exactly one child job per wave

Current code/prompt includes maxEconomicWaveJobs=1 and enforceEconomicPlan truncation, despite max_jobs_per_wave, max_workers and dependency reasoning already existing.

Best case: lower spend and simpler integration.
Worst case: throws away valid independent Yardmaster planning, serialises work unnecessarily, adds paid planning waves, delays completion, and substitutes a fixed number for Yardmaster judgement.

Unless there is stronger evidence than the current rationale, REMOVE the hard one-job truncation. Keep economy as guidance. Let Yardmaster schedule up to configured max_jobs_per_wave while respecting max_workers, dependencies and non-overlapping mutable scopes.

### 2. estimated_minutes as a dispatch-validity gate

Timing is observational/pacing metadata, not a deadline. A missing or >60 estimate should not make an otherwise valid engineering job impossible to dispatch.

Remove hard rejection based solely on estimated_minutes. Keep estimates useful for telemetry/cost learning. Normalize or tolerate absent/unusual estimates rather than failing the plan.

### 3. task-length gate at 1600 characters

Concise child tasks are good guidance. A fixed 1600-character rejection can turn a perfectly valid task into planner failure and may encourage omission of important constraints.

Remove the semantic hard stop. Keep concise-task guidance. If a transport/resource ceiling is genuinely required, make it a generous emergency limit derived from the actual transport constraint, not an arbitrary planning rule.

### 4. raw ordinary attempt ceiling

Current runtime still contains a fixed maxAttempts safety count. The doctrine says unsuccessful attempts are evidence and improving work should continue; low-progress evidence, cost boundaries and supervision already exist.

Do not let a raw attempt count declare a productive job failed. Prefer evidence-based stall detection, cost boundaries and Yardmaster/Closer decisions. A very high emergency implementation ceiling may remain only if needed to prevent an actual runaway loop, and it must be documented as an emergency infrastructure safety boundary rather than a job verdict.

### 5. timing/planning ceilings

Audit yardmasterPlannerTimeout, worker runtime and scheduling grace specifically because prior timing gates caused false blocking.

Do not remove genuine network/process emergency timeouts required to avoid hanging infrastructure. But elapsed time alone must not become evidence that the engineering work is wrong. Where a timeout can safely be treated as a recoverable transport/control-plane event rather than project failure, do so. Do not add new timing gates.

### 6. wave budget

Current master wave budget should remain a resumable checkpoint, not a project-failure condition. Verify that behaviour and tests match it. Do not reintroduce max-waves-as-failure semantics. Question arbitrary validation ceilings if they can prevent a valid bounded resume without protecting infrastructure.

### 7. acceptance-command validation

Audit the mechanical acceptance syntax gate because it has previously produced false rejections around command parsing/escaping. Do NOT simply allow arbitrary dangerous shell execution. Preserve safe deterministic execution, but remove needless syntax restrictions if the runtime can represent/execute the command safely another way. Prefer capability/argv-safe execution over brittle character policing where a small change is possible. Avoid a broad command-runner redesign in this job.

## Rulebook addition

Add a short durable doctrine stating:

"Hard gates are exceptional. Before adding or retaining one, compare the harm it prevents with the harm a false positive or gate bug can cause. Prefer observation, guidance and recovery. Make only dangerous or truth-corrupting actions impossible."

Keep this concise and ensure runtime doctrine/documentation stay aligned.

## Required outcome

- Enumerate the hard/mechanical gates actually present in current code before editing.
- Remove/soften only gates that fail the adversarial test above.
- Do not weaken acceptance truth, provenance, permissions, destructive-action safety or explicit cost ceilings.
- Do not add replacement bureaucracy.
- Prefer deleting machinery over adding abstractions.
- Update focused tests so they prove the new behaviour rather than merely changing constants.
- Preserve current Suture/master state files and unrelated pending jobs.

## Acceptance

The repository must pass:

- `go test ./...`
- `git diff --check`

Tests should specifically demonstrate at minimum that:

- multiple independent jobs are no longer mechanically truncated to one;
- unusual/missing timing estimates do not by themselves invalidate a plan;
- task concision is guidance rather than the 1600-character semantic gate;
- wave-budget exhaustion remains a resumable pause/checkpoint, not failure;
- final acceptance/provenance/cost safety boundaries remain intact.
