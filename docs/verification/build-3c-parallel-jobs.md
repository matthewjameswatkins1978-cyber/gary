# Build 3C — Parallel Job Execution

## What Changed

- `gary run-many <job-file> ...` runs multiple jobs concurrently
- `GARY_MAX_WORKERS` controls max concurrency (default 4)
- Refactored `RunJob()` as reusable function (no `os.Exit`)
- Mutex-protected output with `[job-id]` prefix per line
- Each job gets unique Batch names, GCS paths, temp dirs, branches
- Independent failure: one job's failure does not stop others

## Live Test

- **Concurrency:** 3
- **Start:** 01:57:45
- **End:** 02:03:15
- **Wall clock:** 330s
- **Sequential estimate:** ~810s (3 x 270s)
- **Overlap:** Yes — 3 Batch jobs ran simultaneously

### Results

| Job | Status | Branch | Commit |
|-----|--------|--------|--------|
| fix-clamp-a | DONE | gary/fix-clamp-a-07ae278c | a21edc4 |
| fix-clamp-b | DONE | gary/fix-clamp-b-4b8d993f | e273ffc |
| fix-clamp-c | DONE | gary/fix-clamp-c-c6c9e939 | 7a85e8d |

- All branches differ from base (main: f4b92cb)
- All acceptance passed
- Base unchanged

## Parked State

- No active Batch jobs
- No compute instances running
