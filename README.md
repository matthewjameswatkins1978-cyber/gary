# City Computer

CLI-only Google Cloud Batch AI build farm.

V1 is intentionally just a repository skeleton. The eventual controller will
submit disposable worker jobs, collect their result records, and present a
small terminal report. It will not include a web UI, a daemon, or cloud setup
automation until those are separately designed.

## Planned commands

- `city prepare` — validate a project dossier and job definitions.
- `city launch` — submit a bounded batch run.
- `city oversee` — review worker outcomes and choose a next action.
- `city report` — print run and cost summaries.

Those commands are names only at this stage; none is implemented.

## Layout

```text
cmd/city/             CLI entry point
internal/config/      configuration loading and validation
internal/jobs/        job dossier and result types
internal/batch/       Google Cloud Batch boundary
internal/overseer/    outcome-review policy
internal/report/      terminal-only reporting
internal/gitops/      worker repository operations
docker/worker/        disposable worker image
scripts/              local maintenance and smoke-test helpers
examples/smoke-test/  smallest eventual Batch run fixture
docs/                 decisions and operational notes
testdata/             stable fixtures for tests
```

## Status

Scaffold only. Do not put credentials in this repository; copy `.env.example`
to `.env` when configuration is later introduced.
