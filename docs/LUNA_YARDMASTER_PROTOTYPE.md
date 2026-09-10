# GARY Luna Yardmaster prototype

This branch changes the normal local path to one durable Luna conversation in
Codex App Server plus a small local yard. The yard does not run a supervisor,
planner, cloud queue, heartbeat protocol, telemetry reader, retry loop or model
router.

```text
Matthew -> gary work -> local Codex App Server -> durable Luna thread
                                      |
                                      +-- dynamic yard tools
                                            +-- one git worktree
                                            +-- one codex exec worker
                                            +-- real git status/diff
                                            +-- declared acceptance
                                            +-- persisted human blocker
```

## Run

The canonical front door is `gary do`; use a prepared manifest only when
recovering or testing the lower-level boundary. Use a disposable local Git
repository and copy `examples/luna-yardmaster/job.example.json` for a direct
prototype run.
Set `repo_path` to its absolute path, then run:

```powershell
go run ./cmd/gary work .\job.json
```

The state directory contains the small job record, App Server JSONL evidence,
worker output, last message, acceptance result and blocker record. Set
`GARY_STATE_DIR` only if a shared state location is preferred; otherwise the
manifest's `state_dir`, or `.gary-state` beside the manifest, is used.

The baseline worker profile is `gpt-5.6-luna`. The explicit `mimo-v2.5`
profile is launched through the same installed `codex exec --json` boundary
with the MiMo Token Plan Responses provider override. It requires a
process-injected `MIMO_API_KEY` and never persists that credential. Workers run
in fresh local worktrees, receive no GitHub token and do not push.

## Restart and Matthew boundary

If the client or App Server is stopped, rerun the same command. The job record
supplies the existing thread id, and App Server resumes the stored thread.
The yard status tool reconciles the saved PID, worktree, branch, commit, dirty
state and evidence files. An existing worker is never silently duplicated.

For a persisted human boundary, the command exits with status 3 and prints the
exact required action. Supply the answer to the same job:

```powershell
go run ./cmd/gary work .\job.json --answer "the required action is now available"
```

This resumes the same logical job and thread. It does not create a new job or
worker.

## Evidence gates

1. **Luna brain:** the App Server handshake, durable `thread/start`, whole-job
   prompt and streamed turn are recorded in the App Server JSONL log.
2. **Worker control:** the dynamic `gary_spawn_worker` call creates one
   worktree and one supported Codex worker; status and diff calls read reality.
3. **Kill/resume:** stop the client while the worker is active, rerun the same
   job, and capture the resumed thread id plus reconciled worker/worktree state.
4. **Matthew interruption:** use a disposable canary whose packet deliberately
   calls `gary_request_matthew`; capture the blocker JSON, exit status 3, then
   rerun with `--answer` and verify the same thread id.

The trusted terminal outcome is awarded only after `gary_run_acceptance` passes
against the actual worker worktree. A worker's final message is evidence to
inspect, never acceptance.

## Legacy path

The former Google Cloud Batch implementation remains available only as the
explicit `gary work-batch` command and `gary smoke` boundary. It is not used by
the local Luna path and is retained for historical comparison and controlled
legacy experiments.
