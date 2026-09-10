# GARY local Luna prototype rulebook

GARY exists to finish one complete software job with the least machinery that
reliably works. The canonical implementation intelligence is a durable Luna
Yardmaster thread hosted by Codex App Server. GARY is only the local yard:
worktrees, bounded Codex workers, reality checks, acceptance evidence and
genuine human-boundary records.

## 1. Give GARY the whole job

### Let the agent be an agent

The supplied job packet is the source of truth. Preserve it and give it to one
Luna thread. Luna owns the objective, chooses when a worker is useful, observes
the result and decides the next sensible action from evidence. Do not create a
second planner, swarm, wave scheduler, telemetry protocol, project board or
agent lifecycle engine.

## 2. Thin local boundary

The normal `gary work` path is local-first. Codex App Server owns thread
history, persistence/resume, approvals, input and streamed events. Dynamic
GARY tools own only:

- one fresh local Git worktree;
- at most one supported `codex exec` worker;
- worker status, real Git diff and process reconciliation;
- declared deterministic acceptance;
- a persisted NEEDS_MATTHEW boundary.

The former Google Cloud Batch implementation is legacy-only behind the explicit
`gary work-batch` and `gary smoke` commands. It is not a normal runtime
dependency of the Luna path.

## 3. Worker discipline

The baseline profile is `gpt-5.6-luna`. Explicit candidate profiles include
the other native Codex models, the OpenCode routes
`opencode-go/muse-spark-1.3-contributor`, `deepseek/deepseek-v4-flash`, and
`cheaper-inference/deepseek-v4-flash`, the Antigravity Gemini 3.8 Flash
high/medium/low routes, and `mimo-v2.5`. OpenCode uses its existing local auth
store; Antigravity uses its local session; neither credential is copied into
GARY state. MiMo uses the Codex CLI Responses route and a process-injected
Token Plan credential; it is rejected when the credential is absent. Candidate
profiles must be named by the job and are never silently selected. Never silently
switch models or spawn a replacement after interruption. A worker saying
“done” is not acceptance. The worker worktree, commit, diff, process state and
declared checks are authoritative.

## 4. Human boundaries

Model escalation is human-controlled. Context is pull-based: keep permanent
instructions small and load detail only when the job needs it.

Stop as `NEEDS_MATTHEW` only for an actual outside boundary: unavailable
credential or account, permission increase, material new spend, consequential
irreversible action, inaccessible hardware, contradictory product decision, or
evidenced need for a stronger model. Record the exact needed action, evidence
and unblock instruction. Ordinary tools, packages, documentation, debugging,
first failures and merge conflicts remain the agent's job.

## 5. Evidence outcomes

- `RUNNING` — the durable job is still in progress;
- `NEEDS_MATTHEW` — the exact human action is persisted and required;
- `DONE` — the actual worker candidate passed independent acceptance;
- `FAILED` — execution ended without acceptance and without a genuine outside blocker.

Only deterministic acceptance against the actual worker worktree can award
`DONE`. The model's prose, a process exit code, an App Server turn status or a
legacy cloud status cannot do so alone.

## 6. Restart

Persist only the small job record linking job id, thread id, repo/base, worker,
worktree and evidence paths. On restart, resume the same thread and reconcile
the saved process/worktree/Git state. Never duplicate a worker merely because a
client or App Server connection disappeared.

**Whole job. One capable agent. Useful local tools. Real boundaries. Hard
evidence. Human-controlled escalation. Finish the work.**
