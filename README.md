# GARY

**Give the job once. See the work. Check the proof.**

GARY is a local foreman for delegated software work. It keeps a durable job moving through real Git worktrees and bounded coding workers, while making completion depend on verification, not on an agent declaring itself finished.

This is an engineering tool under active development, not a hosted service or an autonomous permission authority. The primary path is a local Luna Yardmaster; a former cloud Batch path remains available only as an explicit legacy command.

## The local Luna Yardmaster

The normal path is now local-first: `gary work job.json` starts one durable
Luna thread through the local Codex App Server and exposes a small yard boundary
for local Git worktrees, at most two bounded Codex workers, real status/diff inspection,
deterministic acceptance and a persisted Matthew blocker. GARY does not rebuild
an agent supervisor or cloud scheduler. See
[`docs/LUNA_YARDMASTER_PROTOTYPE.md`](docs/LUNA_YARDMASTER_PROTOTYPE.md).

The former Google Cloud Batch path is retained only as the explicit
`gary work-batch job.json` legacy command.

GARY is Matthew's local foreman for one complete software job. The canonical
path is a durable Luna Yardmaster thread, isolated Git worktrees, bounded Codex
workers, deterministic acceptance and recoverable human boundaries.

The local controller stays deliberately thin. Luna owns job judgement; Git,
the filesystem and trusted acceptance own evidence. A worker's prose never
means DONE. The current implementation supports the proven Luna route plus
explicit candidate Codex profiles `gpt-6-astra`, `gpt-5.6-sol`,
`gpt-5.6-terra`, `gpt-5.5`, OpenCode routes
`opencode-go/muse-spark-1.3-contributor`, `deepseek/deepseek-v4-flash`, and
`cheaper-inference/deepseek-v4-flash`, plus Antigravity CLI routes for the
Gemini 3.8 Flash high/medium/low tiers and `mimo-v2.5`. Candidate profiles are
rejected unless the job names them explicitly, and an unknown route is
rejected before dispatch.

There is no AI swarm, cloud scheduler, or second orchestration engine in the
normal execution path. After dispatch, Luna sleeps; a mechanical worker exit
or recovery event wakes the same durable thread. A second worker is allowed
only for a genuinely independent task and accepted commits are integrated
serially.

## Core idea

```text
Matthew -> gary do -> immutable packet -> Luna Yardmaster
                                      |
                         isolated worker worktree
                                      |
                         trusted acceptance -> DONE
```

The implementation agent chooses how to do the work. It may inspect, research, install ordinary dependencies and tools, change tactics, debug, and test without asking for permission every five minutes.

It stops for the human only when progress genuinely requires something outside
its authority or reach, such as an unavailable secret, inaccessible account or
hardware, permission increase, material unapproved spend, consequential
irreversible action, or contradictory requirement needing a product decision.

MiMo Token Plan is used only through the Codex CLI Responses route. GARY does
not store the API key; set `MIMO_API_KEY` in the invoking process and override
the regional endpoint with `GARY_MIMO_BASE_URL` when needed.

OpenCode profiles use the existing local OpenCode credential store; GARY stores
only the selected provider/model name and never copies the credential into job
state or Git. Antigravity profiles use the local `agy` session and quota. These
routes are candidate-only until a real filesystem/tool canary and independent
acceptance certify them.

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

The normal execution front door is:

```powershell
gary do "Review this repository" --accept "go test ./..."
gary do --accept "go test ./..." "Review this repository"
gary do --file .\JOB.md --repo C:\work\project --base master --accept "go test ./..."
gary do --worker-profile mimo-v2.5 "Run the explicit MiMo test job"
gary do -- "--job text may begin with a dash"
```

The lower-level `work <job.json>` command remains available for prepared
manifests and recovery. GARY stores new front-door jobs below
`%USERPROFILE%\.gary`; set `GARY_STATE_ROOT` to override that location.

Useful non-work commands:

```powershell
go run ./cmd/gary doctor
go run ./cmd/gary where
go run ./cmd/gary status
go run ./cmd/gary cancel <job-id>
go run ./cmd/gary cost <job-id>
go run ./cmd/gary smoke
go test ./...
go vet ./...
```

`route --dry-run --json` remains only as a decision-only Telltail adapter. It launches no worker and changes no target repository.

## Outcomes

The meaningful work outcomes are deliberately small:

- `DONE` — the exact candidate passed independent acceptance and was delivered;
- `NEEDS_MATTHEW` — responsible continuation requires something outside GARY's authority or reach and the job is resumable;
- `FAILED` — execution or implementation ended without completion and without a genuine external blocker.

`RUNNING` is observable while a local worker is active. Parking is an external administrative switch, not an agent lifecycle state machine.

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

The disposable local Git worktree is the main implementation sandbox. The agent may act broadly inside that worktree while consequential external authority remains constrained. Batch is legacy-only and explicit.

See `docs/github-app-setup.md` and `docs/github-oidc-setup.md` for identity setup.

## Acceptance

The agent is expected to run the relevant checks while it works. After the agent returns a candidate, GARY independently checks the exact committed candidate in its clean worker worktree, reruns the declared acceptance commands, then requires clean-base serial integration and final acceptance in the target checkout.

Only that independent pass can produce `DONE`.

Batch `SUCCEEDED` means the container finished. It is not software acceptance.

## GARY Black Box

GARY keeps a sparse append-only journal for each job and derives a human note
and searchable local history from it. This makes messy work inspectable across
projects without adding another supervisor or relying on model recollection.

```powershell
gary jobs
gary history search "worker"
gary show <job-id>
gary review <job-id>
gary doctor --job <job-id> --bundle
gary health
```

See [`docs/BLACK_BOX.md`](docs/BLACK_BOX.md) for the storage layout, explicit
incident/related-job/lesson commands, rebuild behavior, and Lucy context
export. The core rule is: **Job history records reality. Incidents record
failures. Lessons record what was learned. Tests prove fixes. Rules govern
future behavior.**

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
