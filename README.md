# GARY

## Current prototype: local Luna Yardmaster

The normal path is now local-first: `gary work job.json` starts one durable
Luna thread through the local Codex App Server and exposes a small yard boundary
for one local Git worktree, one Codex worker, real status/diff inspection,
deterministic acceptance and a persisted Matthew blocker. GARY does not rebuild
an agent supervisor or cloud scheduler. See
[`docs/LUNA_YARDMASTER_PROTOTYPE.md`](docs/LUNA_YARDMASTER_PROTOTYPE.md).

The former Google Cloud Batch path is retained only as the explicit
`gary work-batch job.json` legacy command.

GARY is a small harness for giving one capable AI agent one complete software job in an isolated cloud workspace.

The agent gets the whole job, the repository, ordinary development freedom, and a fixed model. GARY provides the boring machinery around it: short-lived repository access, Google Cloud Batch isolation, cost/runtime ceilings, artifact capture, independent acceptance checks, and delivery of the exact verified branch.

There is no AI manager, worker hierarchy, live supervisor, wave planner, automatic retry loop, or automatic model escalation in the normal execution path.

## Core idea

```text
whole job packet
      |
      v
  gary work
      |
      v
one Gemini 3.8 Flash agent
in one disposable Batch workspace
      |
      v
candidate patch + evidence
      |
      v
independent acceptance
      |
   +--+--+
   |     |
 DONE  BLOCKED / FAILED
```

The implementation agent chooses how to do the work. It may inspect, research, install ordinary dependencies and tools, change tactics, debug, and test without asking for permission every five minutes.

It stops for the human only when progress genuinely requires something outside its authority or reach, such as an unavailable secret, inaccessible account or hardware, permission increase, material unapproved spend, consequential irreversible action, contradictory requirement needing a product decision, or a stronger model.

A stronger model is always a human decision. GARY never routes or escalates models automatically.

## Job format

A job points to the complete packet rather than decomposing it into child jobs:

```json
{
  "id": "example-job",
  "repo": "https://github.com/example/project.git",
  "base_branch": "master",
  "packet_path": "JOB.md",
  "acceptance": [
    "go test ./...",
    "go vet ./..."
  ],
  "max_run_seconds": 3600
}
```

`packet_path` is resolved relative to the job JSON. For small jobs, `task` may contain the complete instruction directly instead. Do not supply both.

The packet itself is the source of truth. GARY does not ask another model to summarise or repartition it before implementation.

## Run

GARY work is enabled for the explicitly authorised live-fire run. Outside an
authorised run, parking is an external human-controlled switch; it is not an
agent lifecycle state.

When enabled, the execution front door is:

```powershell
go run ./cmd/gary work job.json
```

Useful non-work commands:

```powershell
go run ./cmd/gary doctor
go run ./cmd/gary status
go run ./cmd/gary cost <job-id>
go run ./cmd/gary smoke
go test ./...
go vet ./...
```

`route --dry-run --json` remains only as a decision-only Telltail adapter. It launches no worker and changes no target repository.

## Outcomes

The meaningful work outcomes are deliberately small:

- `DONE` — the exact candidate passed independent acceptance and was delivered;
- `BLOCKED` — responsible continuation requires something outside GARY's authority or reach;
- `FAILED` — execution or implementation ended without completion and without a genuine external blocker.

`RUNNING` is observable while the Batch job is active. Parking is an external administrative switch, not an agent lifecycle state machine.

## Blockers

If the agent is genuinely blocked, it writes an out-of-band `.gary-blocked.json` containing:

```json
{
  "kind": "credential",
  "summary": "deployment requires an unavailable signing credential",
  "needed": "the authorised signing credential",
  "evidence": "the required release command rejects the unsigned artifact",
  "unblock_action": "provide the credential or change the release requirement"
}
```

The wrapper removes this file before committing the target repository and returns it to the controller as evidence.

For model escalation, `kind` is `model_escalation`. The request is reported to the human and nothing automatically changes model.

## What GARY keeps

GARY preserves evidence that is expensive or dangerous to reconstruct:

- the supplied job/packet at intake;
- base repository state and resulting branch/commit;
- declared acceptance checks and results;
- Batch identity and result artifacts;
- model usage and cost records;
- genuine blocker evidence;
- enough infrastructure logs to diagnose failures.

It does not need permanent planner memory, child-job histories, supervision telemetry, AI-to-AI guidance, or a second model narrating the first model's work.

## Security and authority

Workers receive a short-lived GitHub App installation token for the target repository, never the App private key. Google Cloud identity and secrets remain at the appropriate trusted boundary.

The disposable Batch workspace is the main implementation sandbox. The agent may act broadly inside that workspace while consequential external authority remains constrained.

See `docs/github-app-setup.md` and `docs/github-oidc-setup.md` for identity setup.

## Acceptance

The agent is expected to run the relevant checks while it works. After the agent returns a candidate, GARY independently clones the base repository, applies that exact patch, and reruns the declared acceptance commands in a clean workspace.

Only that independent pass can produce `DONE`.

Batch `SUCCEEDED` means the container finished. It is not software acceptance.

## Development

GARY is written in Go.

```powershell
go test ./...
go vet ./...
```

CI is deliberately execution-free: it never invokes `gary work`, Google Batch, or Gemini.

## Doctrine

Read `GARY_RULEBOOK.md`. It is intentionally short.

**Whole job. One capable agent. Useful tools. Real boundaries. Hard evidence. Human-controlled escalation. Finish the work.**
