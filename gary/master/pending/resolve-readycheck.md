# Resolve private-repo readiness probe

Purpose: verify that GARY Yardmaster can access the exact private Resolve AI submission branch before any proper master job is sent.

Target branch is expected to already contain:
- README.md
- pyproject.toml

Instructions:
- Inspect the current target repository state.
- If README.md and pyproject.toml already exist, return done=true and jobs=[].
- Do not create any child jobs.
- Do not modify product/source files.
- This is an access and stop-condition probe only.
