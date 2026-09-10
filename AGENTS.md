# AGENTS.md

## Environment

- Windows local machine
- PowerShell 7.6.5
- Modern CLI-first environment
- Prefer existing CLI tools before adding dependencies

## Project

- GARY is CLI-only
- Read `GARY_RULEBOOK.md` before substantial work
- Preserve known-good Git state
- Run the smallest useful verification before claiming completion

## Startup

1. Read this file
2. Read `GARY_RULEBOOK.md` (relevant parts)
3. Inspect Git status
4. Inspect only files relevant to the current job
5. Check available CLI tooling before assuming something is missing
6. Begin work

## Canonical GARY

When Matthew or another controlling agent says “Gary, do X”, “use GARY”, or
equivalent, invoke the existing canonical GARY front door. Do not build a
substitute GARY, dispatcher or orchestration layer.
