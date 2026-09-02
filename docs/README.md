# GARY Documentation

This directory contains two kinds of material: current operating documentation and historical verification records.

For current behaviour, use this order of authority:

1. The accepted project packet and its amendments define the requested project outcome.
2. `GARY_RULEBOOK.md` defines GARY's completion principle, universal boundaries, and role responsibilities.
3. `docs/YARDMASTER_OPERATING_GUIDE.md` provides advisory engineering tradecraft for setting up projects, constructing jobs, supervising workers, integrating results, and verifying completion.
4. `docs/project-context.md` documents the durable project-context model.
5. The GitHub/OIDC setup documents describe the current credential and deployment setup where they match the implementation.
6. Executable code, schemas, tests, and workflows are the source of truth for exact runtime mechanics.

The operating guide is intentionally not a gate checklist. It explains a strong default method while leaving the Yardmaster free to use current evidence and judgement.

## Historical material

`docs/reliability-r1.md` records an earlier reliability design and is retained for provenance. It is not the current runtime contract.

Files under `docs/verification/` are build/verification records from particular development stages. They are evidence of what was tested at that time, not standing instructions for current GARY behaviour.

When historical documentation conflicts with the current rulebook, operating guide, implementation, or tests, treat the historical document as superseded rather than trying to satisfy both.