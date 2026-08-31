# Durable project context

GARY stores project context under `gary/projects/<project-id>/` by default:

```text
project.json
state.json
source/MASTER_PACKET.md
manuals/project-manual-v001.json
amendments/amendment-001.md
jobs/
evidence/
archive/
```

The source packet is written before the Yardmaster ingestion call. Its SHA-256
is checked on every context load. Manuals are immutable versioned snapshots and
must carry source packet provenance for their requirements and conflicts.

`gary project resume <project-id>` performs a fresh validated rehydration and
returns the project to planning without launching a worker. A project-backed
`gary master` run rehydrates before every Yardmaster wave and rebuilds the
narrow child execution context from the current manual before each worker
submission.

Project context is deliberately separate from execution history:

- the master packet and manual describe what the project is and what must be
  true;
- `state.json` describes current phase and progress;
- master status and worker results describe what GARY tried and observed.

Project context is treated as untrusted input. It cannot override GARY's
authority, sandbox, provider, acceptance-command, or deterministic routing
controls.
