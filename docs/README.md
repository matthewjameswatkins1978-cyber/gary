# GARY Documentation

This directory contains current canonical operating documents and historical evidence.

## Canonical doctrine

Use this authority order for GARY behaviour:

1. The accepted Project Packet and authorised amendments define the requested project outcome.
2. `GARY_RULEBOOK.md` is the GARY Bible: constitutional doctrine and role boundaries.
3. `docs/GARY_STATE_MACHINE.md` defines canonical lifecycle, activities, transition ownership, generation, and board projection.
4. `docs/YARDMASTER_OPERATING_GUIDE.md` describes practical Yardmaster operation.
5. `docs/JOBPACKET_CONTRACT.md` defines Worker handoff semantics.
6. `docs/ACCEPTANCE_INTEGRATION_PROTOCOL.md` defines candidate verification, bounded Yardmaster repair, acceptance, reconciliation, and project acceptance.
7. `docs/FAILURE_RECOVERY_MAP.md` defines stage-accurate failure diagnosis and recovery.
8. `docs/project-context.md` documents durable storage and migration mechanics.
9. GitHub/OIDC setup documents describe credential and cloud setup where they match the implementation.
10. Executable schemas, code, tests, and workflows are authoritative for exact mechanics, but must conform to the doctrine above rather than invent competing policy.

The canonical documents deliberately separate principles from mechanics. Do not copy lifecycle enums or acceptance rules into new prose unless the document genuinely owns that definition.

## Historical material

`docs/reliability-r1.md`, files under `docs/verification/`, incident notes, and other dated design/build records are historical evidence.

They may explain why a current rule exists. They are not standing runtime instructions.

When historical material conflicts with the canonical set, treat it as superseded. Do not attempt to satisfy both.
