# CALL-E + Tethers Hackathon Entry — GARY Gate 0

## EXECUTION OVERRIDE — READ FIRST

This brief contains downstream product context, but **this run is Gate 0 only**.

Do not build the full hackathon application yet.

Authorised work for this run:

1. prove the real CALL-E boundary as far as the execution environment genuinely allows;
2. prove the real Tethers machine boundary;
3. freeze the smallest interfaces justified by those proofs;
4. create one concise evidence report;
5. STOP.

The full-product context exists only so Gate 0 choices remain compatible with the intended hackathon application.

A missing external prerequisite such as an unavailable secret or authorised phone number is a valid **PARTIAL** Gate 0 result. Do not burn retries or waves trying to manufacture credentials or phone numbers. Complete every non-blocked proof first, record the exact blocker, then stop.

---

## PRODUCT THESIS

Future product: a small judge-friendly CALL-E hackathon application for routine delegated phone calls such as appointment booking, rearranging, availability checks and mundane enquiries.

Core idea:

> The conversational AI may be capable of making a decision, but it may only make decisions the user has explicitly authorised.

CALL-E handles conversation.

**Tethers owns authority.**

Working line:

> The AI decides what to say. Tethers decides what it is allowed to do.

Do not redesign Tethers. Do not redesign CALL-E. Do not expand this into a general assistant.

---

## SUPPORTED CALL-E ARCHITECTURE

CALL-E confirmed on 14 August 2026 that active calls currently cannot accept external mid-call policy decisions.

Supported:

- create/start calls;
- retrieve calls;
- ordered observation-only lifecycle events through GET /v1/calls/{call_id}/events or current SDK equivalent;
- terminal/final result retrieval;
- structured results where supported;
- transcript/summary/task completion/evidence fields where actually returned;
- idempotency support;
- terminal webhooks where useful.

Not supported:

- mid-call policy callbacks;
- tool-call interception;
- external response injection;
- external proceed/refuse/defer control;
- pausing/resuming an active conversation;
- external live behaviour modification.

**Do not build or imply live Tethers control inside an active CALL-E call.**

Future architecture is atomic calls:

authority before call -> CALL-E call -> evaluate result -> defer if new authority is needed -> human decision outside the call -> new frozen authority -> follow-up CALL-E call.

Gate 0 proves only the two real boundaries.

---

# WORKSTREAM A — CALL-E CONNECTIVITY SPIKE

## Ownership

Worker A owns only:

- `spikes/calle-gate0/**`
- `docs/gate0-calle.md`

Do not modify Tethers implementation files.

This spike folder is a temporary proof location on the Gate 0 branch because a dedicated CALL-E application repository does not yet exist. It is **not** the final app location and must not cause Tethers to depend on CALL-E.

## Objective

Prove, using current official TypeScript SDK/API shapes:

TypeScript
-> CALL-E
-> one authorised real controlled phone call
-> call ID retained
-> observation-only events retrieved
-> terminal structured result retrieved
-> transcript/evidence fields inspected

If live prerequisites are unavailable inside the GARY worker environment, prove everything up to that external boundary and report PARTIAL exactly.

## Runtime/package

Use current official TypeScript package:

`@call-e/calle`

Inspect installed/current package types and official package examples before coding. Installed SDK/types are authoritative.

Record exact:

- Node version;
- npm/pnpm version;
- TypeScript version;
- `@call-e/calle` version;
- exact methods actually used.

Do not blindly rely on remembered field names.

## Secret handling

Required variable:

`CALLE_API_KEY`

Safe existence check only. Never print its value.

Never:

- echo or log the key;
- commit it;
- put it in README;
- put it in `.env.example`;
- put it in screenshots;
- put it in evidence output;
- write it to source-controlled files.

If a local `.env` is used, ensure it is ignored before writing any secret.

If `CALLE_API_KEY` is absent in the GARY worker environment:

- do NOT invent or reconstruct it;
- do NOT ask for another key;
- do NOT make repeated authentication attempts;
- complete SDK setup, compilation, deterministic local tests, request construction, idempotency logic and result/event parsing;
- record in `docs/gate0-calle.md` that the key was not securely injected into the GARY Batch environment;
- treat live CALL-E proof as PARTIAL.

## Authorised phone

Use only:

- Matthew's own explicitly authorised phone number; or
- another number explicitly authorised for this test.

Preferred env variable:

`CALLE_TEST_PHONE`

Never commit or print the number in the final report.

If no authorised number is available, complete all setup first and record that as the only phone-specific human prerequisite.

## Controlled call

If both `CALLE_API_KEY` and `CALLE_TEST_PHONE` are available, make normally **one** controlled call.

Task semantics:

"Tell the recipient this is a CALL-E development test. Ask whether they can hear the caller clearly. Do not make purchases, bookings, promises or other commitments."

Use a strict structured result equivalent to:

```json
{
  "type": "object",
  "required": ["can_hear_clearly"],
  "properties": {
    "can_hear_clearly": {
      "type": "string",
      "enum": ["yes", "no", "unknown"]
    }
  },
  "additionalProperties": false
}
```

Adapt field placement to the actual current SDK/API.

## Idempotency

Use a stable idempotency key for the logical test call.

Required lifecycle:

construct request
-> assign stable idempotency key
-> create once
-> immediately retain/persist call ID locally
-> observe/wait using that call ID
-> retrieve final result using that call ID

A wait/poll timeout must never blindly create another call.

Do not intentionally place duplicate calls to test idempotency.

A second real call is acceptable only if the first exposes a concrete technical problem and one rerun is necessary to prove the fix.

## Metadata

If supported, include harmless correlation metadata:

```json
{
  "project": "calle-tethers-hackathon",
  "stage": "gate-0",
  "spike": "connectivity"
}
```

Confirm whether metadata round-trips.

Do not build workflow/policy ID machinery yet.

## Events

Inspect current events API/SDK from the SAME call.

Capture sanitized:

- event type;
- event ID;
- timestamp;
- status/message;
- details structure;
- cursor;
- next cursor.

Events are observation-only. Never imply policy control.

## Final result evidence

Record documented vs actually observed fields separately.

Inspect availability of:

- call ID;
- terminal status;
- structured result;
- recipient result if applicable;
- summary;
- transcript;
- task completion;
- confidence;
- evidence;
- failure code/message;
- metadata;
- timestamps.

## Spike A tests

Create small deterministic local tests for at least:

- request/schema construction;
- stable idempotency key behaviour;
- no retry path can create a second call after a wait timeout;
- parsing of a representative terminal result;
- safe failure when required config is absent.

Do not fake a successful real call.

## Spike A output

Write `docs/gate0-calle.md` containing:

- PASS/PARTIAL/BLOCKED;
- runtime/SDK versions;
- exact SDK methods;
- sanitized call ID if a real call occurred;
- terminal result;
- structured result;
- events;
- transcript/evidence fields;
- metadata behaviour;
- idempotency strategy;
- real call count;
- errors/blockers;
- explicit secret-handling confirmation.

---

# WORKSTREAM B — TETHERS MACHINE INTERFACE

## Target

This master job is running against the real Tethers repository.

Expected base branch for this Gate 0 run:

`portable-release-linux`

Expected starting commit at dispatch time was approximately:

`7bedf535390268cec3e9cd360cf12507e4726cb3`

Record the actual base SHA seen by the worker and do not assume this expected value is still current.

## Ownership

Worker B owns:

- existing Tethers implementation paths genuinely required for this narrow interface;
- Tethers fixture/test files;
- `docs/gate0-tethers.md`.

Worker B must not modify `spikes/calle-gate0/**`.

## Objective

Prove:

JSON facts
-> real existing Tethers
-> deterministic ALLOW | ASK | DENY
-> matched rule
-> human-readable reason

Tethers remains the deterministic authority.

No LLM may reinterpret or override Tethers' final decision.

## Audit before modification

Before changing code, record:

- repo URL;
- base branch;
- SHA;
- dirty/clean state;
- relevant workspace structure.

Inspect for:

- parser;
- evaluator;
- policy result type;
- existing CLI;
- serde/JSON support;
- error handling;
- existing authority semantics;
- Portable 0.1 interfaces already present.

Run relevant existing baseline tests before modification.

Historic evidence is only a reference, not acceptance:

- parser normalization had previously passed J13A 25/25 twice;
- broad Rust suite had previously been around 695 passing tests.

Report today's actual result.

## Change limit

Reuse existing functionality whenever possible.

Allowed:

- tiny CLI/machine adapter;
- JSON serialization;
- machine response type;
- CALL-E-specific fixture tests.

Not allowed:

- parser redesign;
- semantic redesign;
- HQ/editor work;
- Rocket work;
- canonicalisation optimisation;
- network service;
- database;
- new policy language;
- speculative Tethers V2 features.

If the required CALL-E behaviour genuinely needs a Tethers semantic/language change:

**STOP THAT CHANGE AND REPORT IT.**

## Target contract

Expose approximately:

request JSON
-> Tethers
-> response JSON

Response shape should be equivalent to:

```json
{
  "schema_version": 1,
  "decision": "ALLOW",
  "matched_rule": "appointment.standard",
  "reason": "within delegated authority"
}
```

Decisions:

`ALLOW | ASK | DENY`

If native terminology differs, implement the thinnest faithful mapping.

Malformed/unsupported input must never become ALLOW.

Acceptable fail-closed output:

```json
{
  "schema_version": 1,
  "decision": "DENY",
  "error": "invalid_policy_input"
}
```

or deterministic non-zero process failure that a future adapter would map to DENY.

Document which strategy is used.

## Minimum authority semantics

Represent authority equivalent to:

ALLOW:
- appointment booking;
- consultation;
- weekday;
- after 1 PM;
- price <= £60.

ASK:
- price > £60 but potentially approvable;
- additional service;
- materially new commitment;
- unknown but potentially authorisable choice.

DENY:
- explicitly prohibited treatment;
- explicitly forbidden action;
- malformed/unsafe evaluation state.

Use existing Tethers syntax/semantics.

Do not invent unnecessary constructs.

## Required fixtures

Mechanical tests for all five:

1. Tuesday 3:30 PM, £45 consultation -> ALLOW
2. Tuesday 3:30 PM, £95 consultation -> ASK
3. extra hygienist service -> ASK
4. explicitly forbidden treatment -> DENY
5. malformed input such as `{"banana":"ferret"}` -> DENY or safe deterministic error

## Determinism

Run the fixture set repeatedly.

Same policy + facts + Tethers version must produce the same:

- decision;
- matched rule;
- structured response.

Capture proof.

## Spike B output

Write `docs/gate0-tethers.md` containing:

- PASS/PARTIAL/BLOCKED;
- repo/base SHA;
- relevant baseline tests;
- exact machine invocation;
- exact request schema;
- exact response schema;
- ALLOW/ASK/DENY mapping;
- five fixture results;
- malformed-input fail-closed proof;
- repeated determinism proof;
- regression tests;
- any change made, with clear statement if existing functionality was simply reused.

---

# YARDMASTER COORDINATION

Wave 1 should prefer two independent jobs:

- Worker A: CALL-E connectivity spike, only `spikes/calle-gate0/**` and `docs/gate0-calle.md`
- Worker B: Tethers audit/machine interface, no CALL-E spike files, plus `docs/gate0-tethers.md`

They may run in parallel because their owned paths do not overlap.

Do not create a setup job whose ephemeral state another worker needs.

After both evidence files exist, a later bounded job may synthesize the final report.

Do not let workers redefine shared interfaces independently. Freeze interfaces only after actual evidence exists.

Use the current GARY Yardmaster as architectural coordinator. Keep cheap workers bounded and mechanical.

---

# FINAL GATE 0 REPORT

Create:

`docs/calle-tethers-gate0-report.md`

It must contain:

## Executive

Exactly one:

- GREEN
- PARTIAL
- RED

GREEN only if both real boundaries are proven.

PARTIAL if one boundary works and the other has an exact fixable external/technical blocker.

RED if a foundational assumption is false.

## CALL-E

- SDK/runtime versions;
- exact methods used;
- branch and SHA;
- sanitized real-call evidence if available;
- terminal/structured result;
- events;
- transcript/evidence fields;
- metadata result;
- idempotency strategy;
- call count;
- errors/blockers.

## Tethers

- repo/path;
- base SHA;
- GARY staging/final SHA;
- baseline tests;
- machine invocation;
- request schema;
- response schema;
- decision mapping;
- five fixtures;
- fail-closed proof;
- repeated determinism proof;
- regressions.

## Recommended frozen interfaces

State only interfaces actually proven.

Conceptually expected, but do not force if reality differs:

CALL-E:
create(request, idempotencyKey) -> callId -> events(callId) -> terminalResult

Tethers:
evaluate(policy, facts) -> ALLOW | ASK | DENY + rule + reason

## Human decisions

Only genuine remaining decisions/input for Matthew/Lucy.

## Security confirmation

Explicitly state:

- CALL-E API key not committed;
- CALL-E API key absent from reports/log output produced by this work;
- ignored env handling used if applicable;
- Git diff/history created by this work contains no CALL-E secret;
- private phone number absent from Git.

---

# GATE 0 STOP CONDITION

After the report exists and the two spikes have been pushed as accepted GARY commits:

**STOP.**

Do not build:

- UI;
- dashboard;
- follow-up call orchestration;
- deployment;
- calendar;
- accounts;
- policy editor;
- generic workflow engine;
- mobile app;
- CRM;
- payment flow;
- live voice interception;
- browser automation;
- general assistant features.

A PARTIAL result caused solely by missing secure key injection or missing authorised test phone is still a completed Gate 0 investigation. Record it and stop rather than spending extra worker attempts.

The eventual full product will be authorised separately after Matthew/Lucy review Gate 0.
