# GARY MASTER PROJECT PACKET — SUTURE v0.1

**Packet date:** 2026-09-02  
**Project:** Suture  
**Target repository:** `matthewjameswatkins1978-cyber/suture`  
**Implementation language:** Rust  
**Project class:** New standalone AI-first developer tool  
**Primary users:** coding agents and technically strong developers  
**Build objective:** Produce, verify, benchmark, document, and package a complete Suture v0.1 that is genuinely useful as a deterministic file-mutation boundary for AI agents.

---

## 0. Yardmaster authority and operating instructions

This packet is the canonical project brief.

GARY/Yardmaster owns implementation planning, decomposition, worker assignment, repair, verification, integration, and stopping when acceptance is met. The Yardmaster may choose the exact internal module boundaries and implementation details, but it MUST preserve the product semantics, safety invariants, scope boundaries, and acceptance criteria in this packet.

Do not redesign Suture into a general-purpose editing framework. Do not expand scope merely because a worker finds an interesting adjacent problem. Complexity must earn its keep.

Use small, cheap, deterministic tests first. Run expensive or broad verification only when it materially improves confidence or is required by acceptance.

If the target repository is empty, bootstrap it cleanly. If it already contains Suture work, audit and preserve useful work/history rather than rewriting blindly.

If the target repository does not exist or cannot be accessed, STOP before paid worker execution and report the exact repository/bootstrap blocker. Do not mutate an unrelated repository as a substitute.

All workers must rehydrate this project packet/current durable GARY manual before planning or implementation. Do not rely on a condensed chat-memory summary in place of the persisted packet.

---

# 1. Product definition

## Canonical one-sentence definition

**Suture is an AI-first deterministic mutation protocol/runtime that identifies an exact target, checks reality, applies only the authorised file mutation, preserves everything else it can guarantee, refuses ambiguity, and returns compact evidence so the agent does not have to guess or reread the file.**

## Design motto

> **Change exactly what I meant; treat everything else as protected state.**

## Core product contract

A Suture request combines:

```text
TARGET IDENTITY
+ EXPECTED REALITY
+ AUTHORISED MUTATION
+ PRESERVED STATE
+ VERIFIED RESULT
```

The runtime must behave as a deterministic mutation boundary, not as an improvising editor.

## Conceptual execution model

```text
OBSERVE
  ↓
IDENTIFY
  ↓
GUARD
  ↓
MUTATE
  ↓
VERIFY
  ↓
CERTIFY
```

Every meaningful implementation decision should fit this model.

---

# 2. Why Suture exists

Current coding agents disproportionately struggle with the mechanics of small file edits even when they understand the desired semantic change.

Observed pain includes:

- rereading large files merely to verify a tiny edit;
- fuzzy patch context that becomes stale;
- exact-string matching broken by tabs/spaces/trailing whitespace;
- duplicate and near-duplicate targets;
- full-file rewrites that silently drop unrelated content;
- regex over-match/under-match and escaping problems;
- editing JSON/TOML/YAML as text and leaving invalid documents;
- serializers that destroy comments, ordering, quote style, spacing, or line endings;
- sequential edits invalidating earlier line numbers/anchors;
- stale agent context after concurrent human/agent changes;
- CRLF/LF confusion;
- UTF-8 BOM and other encoding surprises;
- invisible Unicode such as NBSP and zero-width characters;
- PowerShell/Bash/WSL quoting layers;
- Windows/WSL/Linux path confusion;
- symlink/path traversal risks;
- uncertainty about whether a mutation really landed exactly as intended.

A major economic/product insight from direct AI-agent critique is:

> **Verification-by-rereading is a hidden token and reasoning tax.**

A successful Suture operation should normally provide enough deterministic evidence that the caller does not need to reread the entire mutated file.

This is both a safety product and a context-compression product.

---

# 3. AI user research that shaped this design

The concept was directly pressure-tested from the perspective of several AI systems rather than designed only from human preferences.

## Gemini

Strongest contributions:

- primary AI interface should be strict structured/schema tool calls, not a bespoke textual DSL;
- stable anchors, cardinality, and evidence certificates are high-value;
- token-budget/dry-run preview can be useful;
- no general-purpose control flow.

## DeepSeek

Strongest contributions:

- fail closed, but fail informatively;
- refusal diagnostics must reduce retry loops;
- symlink escape, TOCTOU, resource exhaustion and hostile inputs are first-class security issues;
- structural providers must be mandatory for structural edits;
- deterministic primitives should stay “dumber” than the planning agent.

Suggestions deliberately NOT accepted into v0.1:
- workspace-wide semantic find-references;
- dependent transaction branching/control flow;
- broad AST/LSP integration;
- pseudo-precise formatting drift scores.

## Claude

Strongest contributions:

- verification rereads are a major hidden token sink;
- certificates should contain exact minimal diffs rather than full-file echoes;
- content-addressed anchors are preferable to position-addressed anchors;
- search, formatting and language-server responsibilities should remain separate;
- strongest case for omitting a general regex mutation engine from v0.1;
- two structured providers are necessary to demonstrate the provider abstraction is real.

## Consensus

Strong convergence existed around:

- identity/hash guarding;
- exact cardinality;
- fail-closed ambiguity;
- actionable refusals;
- source-preserving structured providers;
- compact evidence certificates;
- stable/content-addressed targeting;
- strict structured tool calls;
- workspace/symlink containment;
- no general-purpose programming layer;
- deterministic handling of Windows/WSL/Linux details below the agent.

Treat this convergence as product evidence, not proof. The benchmark in this packet must still falsify or validate the idea.

---

# 4. Non-negotiable invariants

These are frozen unless Matthew explicitly amends the project packet.

## Invariant 1 — never guess a destructive target

If exactly one target is required and reality yields zero or multiple candidates, Suture refuses.

It must never silently choose “the first one”.

## Invariant 2 — every mutation is identity guarded

The runtime must protect against stale context and object substitution.

The caller need not manually supply a hash for every immediate operation if Suture can establish and preserve the identity guard internally, but the mutation itself must be guarded.

## Invariant 3 — providers never write files

Providers may parse, identify, plan, validate and declare capabilities.

Only Suture Core may commit bytes to the filesystem.

## Invariant 4 — no silent weakening

A structural request may not fall back to raw text, regex, or another weaker mutation method because parsing/provider support failed.

If required semantics or preservation cannot be guaranteed, refuse.

## Invariant 5 — unauthorised state is protected

Everything not explicitly authorised for mutation is protected state.

Where Suture claims byte preservation, unchanged byte regions must actually remain byte-for-byte unchanged.

## Invariant 6 — verification precedes success

A mutation is not successful merely because a write syscall completed.

The candidate result must pass the relevant structural/preservation/identity checks before Suture reports APPLIED.

## Invariant 7 — evidence accompanies success

A successful mutation returns compact deterministic evidence sufficient for normal verification without a full-file reread.

## Invariant 8 — workspace confinement is mandatory

No path traversal, symlink escape, or equivalent path trick may let a request mutate outside the declared workspace.

## Invariant 9 — malformed structure is not “helpfully repaired”

If a structural provider cannot parse the input according to its contract, the mutation refuses with precise diagnostics.

Do not silently repair unrelated malformed syntax as a side effect.

## Invariant 10 — outcome semantics are typed and exact

```text
APPLIED
NO_CHANGE
REFUSED
FAILED
```

These outcomes have distinct meanings and must not be conflated.

---

# 5. Outcome semantics

## APPLIED

The authorised mutation was committed and verified.

The certificate must identify what changed and what was validated/preserved.

## NO_CHANGE

The desired state was already satisfied.

Example:

```text
set /server/timeout = 30
```

when the existing value is already `30`.

NO_CHANGE must NOT mean “I could not find the target.”

## REFUSED

The Suture runtime operated correctly but reality did not satisfy the mutation contract safely.

Examples:

- zero matches when exactly one required;
- multiple matches when exactly one required;
- stale identity/snapshot;
- target outside workspace;
- symlink escape;
- malformed structured document;
- required preservation guarantee unavailable;
- unsupported encoding;
- unsupported provider operation.

## FAILED

The runtime/provider/environment malfunctioned.

Examples:

- internal invariant violation;
- I/O failure unrelated to a valid refusal rule;
- provider crash;
- unrecoverable platform error.

Make refusal reasons and failure reasons machine-readable enums/codes, with human-readable details as data, not as the only semantics.

---

# 6. Cardinality semantics

Cardinality is a first-class property, not an optional assertion bolted onto replacement.

Destructive operations default to exactly one target unless their structured semantics inherently identify a unique path/node.

Support explicit expectations sufficient for v0.1, such as:

- exactly one;
- exactly N;
- all, only when the caller explicitly requests bulk mutation.

Do not invent a large cardinality algebra unless tests demonstrate need.

### Required behaviour

Expected 1, observed 0:

```text
REFUSED
reason: TARGET_NOT_FOUND or CARDINALITY_MISMATCH
expected: 1
observed: 0
```

Expected 1, observed 2:

```text
REFUSED
reason: CARDINALITY_AMBIGUOUS
expected: 1
observed: 2
candidates: [...]
```

Bulk mutation must be explicit and auditable.

---

# 7. Fail closed, but fail informatively

A refusal must help the agent make one informed correction instead of triggering blind retry loops.

## Multiple-match refusal

Return each candidate with the minimum useful distinguishing data:

- stable/candidate anchor;
- line/byte range for human diagnostics;
- concise surrounding context;
- provider structural path/scope where available;
- candidate content/context hash.

Do not dump entire files.

## Zero-match refusal

Where cheap and safe, return useful near-miss diagnostics.

Especially diagnose invisible text differences such as:

- U+0020 SPACE vs U+00A0 NO-BREAK SPACE;
- TAB vs spaces;
- CRLF vs LF;
- U+200B ZERO WIDTH SPACE;
- BOM;
- smart quotes;
- Unicode normalization/composition differences.

A useful diagnostic is:

```text
closest candidate differs at byte 12
expected: U+0020 SPACE
observed: U+00A0 NO-BREAK SPACE
```

Do not build a heavyweight fuzzy-search engine merely to produce near misses. The feature must remain bounded and cheap.

---

# 8. Representation: protocol first, DSL later if ever

## Primary v0.1 interface

Strict structured JSON/schema tool calls.

The AI must not be required to generate a custom textual Suture DSL.

The runtime may expose a CLI wrapper that accepts request JSON from stdin/file and emits result JSON to stdout.

## Single source of truth

Prefer defining Rust protocol types once and deriving:

- serialization/deserialization;
- JSON Schema;
- validation/tool schema.

Candidate approach:

- Serde;
- `schemars`.

Avoid maintaining separate hand-written runtime models and JSON schemas that can drift.

Unknown/unrecognised fields should fail under the strict protocol where practical rather than be silently ignored.

## Future representations

Possible later serializations:

- human-readable `.sut`;
- richer IPC/binary framing;
- “Suture View” / pretty output.

They must compile to the same semantic operation model.

Presentation may change. Semantics may not.

---

# 9. Implementation language: full-strength idiomatic Rust

Rust is the chosen implementation language after comparing Rust, Go and OCaml.

## Why Rust

Suture needs both:

1. strong semantic/state modelling; and
2. careful systems/filesystem behaviour.

Rust provides:

- enums/algebraic-data-type style modelling;
- exhaustive `match`;
- `Option`/`Result`;
- traits;
- generics;
- ownership/borrowing;
- lifetimes;
- type-state;
- iterators;
- zero-cost abstractions;
- strong cross-platform systems support;
- memory safety;
- native single-binary distribution.

## Global implementation rule

Use Rust as Rust was designed to be used.

Do NOT dumb the program down for beginner readability or make Rust imitate Go/C.

The primary readers and maintainers are AIs and technically strong developers.

Use advanced features where they materially improve:

- correctness;
- impossible-state modelling;
- clarity;
- expressiveness;
- composability;
- performance;
- maintainability.

This explicitly permits:

- type-state;
- sophisticated enums;
- exhaustive pattern matching;
- appropriate traits/generics;
- lifetimes;
- iterator-based designs;
- ownership/borrowing patterns;
- zero-cost abstractions;
- const generics or macros if they genuinely earn their keep.

The constraint is against gratuitous cleverness, abstraction ornament, novelty for its own sake, and complexity that does not improve the product.

Do not “simplify” a strong Rust invariant into runtime booleans merely because the type-level version is advanced.

Use `#![forbid(unsafe_code)]` in Suture-owned code unless a concrete requirement demonstrates that a safe implementation is impossible or materially defective. If unsafe becomes necessary, isolate it, document the invariant, and add focused tests.

---

# 10. Initial repository shape

Start as one repository and preferably one primary crate.

Do NOT start with a Cargo workspace simply to look architectural.

A reasonable initial shape is:

```text
Cargo.toml
Cargo.lock
README.md
LICENSE
src/
  main.rs
  protocol.rs
  engine.rs
  document.rs
  workspace.rs
  path.rs
  certificate.rs
  diff.rs
  provider/
    mod.rs
    text.rs
    json.rs
    toml.rs
tests/
testdata/
docs/
benches/ or benchmark/
```

The Yardmaster may improve the exact module boundaries if the result remains smaller/clearer and preserves the core-provider safety boundary.

Split crates only when real pressure appears.

---

# 11. Dependency discipline

Choose mature, focused dependencies after checking current versions/APIs.

Candidate building blocks identified during research:

- `serde`
- `serde_json`
- `schemars`
- `cap-std`
- `jsonc-parser` configured for strict JSON/CST-style editing if it provides the required preservation
- `toml_edit`
- `sha2`
- `similar`
- `thiserror`
- a small CLI parser if useful
- an atomic-write approach/library only if its guarantees compose correctly with capability-safe workspace handling

These are candidates, not mandatory if a better current crate exists.

### Explicit v0.1 non-dependencies unless strictly justified

- Tokio/async runtime
- database
- web framework
- network client
- embedded scripting engine
- regex engine
- Tree-sitter
- LSP framework
- WASM runtime
- dynamic Rust plugin loader

Every dependency must earn its attack surface, build cost and semantic weight.

---

# 12. Core architecture

## Rule: providers never commit

The strongest architectural boundary is:

```text
AI / caller
    ↓
strict request schema
    ↓
SUTURE CORE
    ↓
provider identifies/plans
    ↓
explicit MutationPlan / ByteEdits
    ↓
SUTURE CORE validates plan
    ↓
candidate document in memory
    ↓
provider/core validation
    ↓
safe commit
    ↓
certificate
```

A provider may never independently open a path and persist changes outside Core’s commit machinery.

## MutationPlan direction

The exact Rust API is Yardmaster-owned, but conceptually a plan must capture enough data to prove admissibility:

```text
MutationPlan
  source identity
  provider identity/version
  target identity
  expected cardinality
  authorised byte/node changes
  preservation requirements
  validations
```

Ultimately the commit layer should receive explicit non-overlapping byte changes or an equivalently strict representation.

Core rejects:

- overlapping edits;
- edits outside authorised source object;
- stale target/source identity;
- provider plans inconsistent with the request;
- preservation requirement failures.

---

# 13. Internal type-state direction

Use Rust’s type system to prevent invalid execution orders where this improves the design.

Conceptual states:

```text
ObservedDocument
      ↓
IdentifiedTarget
      ↓
PreparedPlan
      ↓
VerifiedPlan
      ↓
CommittedMutation
```

Only a verified plan should be committable.

This does not require literal types with exactly these names, but the implementation should make it difficult or impossible to accidentally call commit on an unverified plan.

Avoid vague state such as:

```text
success = true
validated = maybe
message = "worked"
```

Prefer typed state.

---

# 14. v0.1 operation vocabulary

Keep the mutation algebra deliberately small.

## Required core operations

### identify

Establish a mutation-safe identity for a target or report candidates/refusal.

This is NOT a codebase search engine.

Exploratory discovery remains the job of `rg`, IDE/LSP tools, or the calling agent.

### replace

Replace one guarded target with new content.

Destructive default: exactly one.

### insert_before

Insert new content immediately before an identified target/anchor.

### insert_after

Insert new content immediately after an identified target/anchor.

### delete

Delete an identified target.

### move

Move an identified region/node to a new identified location if the implementation can keep semantics clean and fully guarded.

If move introduces disproportionate v0.1 complexity, the Yardmaster may implement it only after replace/insert/delete are accepted, but it remains part of the intended core direction.

## Structured-provider operations

Where meaningful and safely implementable:

### set

Set a value/node by a structured path.

### rename-key

Rename a mapping/property key where the provider can preserve/validate the surrounding representation.

Do NOT interpret “rename-key” as “semantic symbol rename across a workspace”.

---

# 15. What v0.1 explicitly does NOT contain

This section is a scope firewall.

Do not implement any of the following unless they are strictly required to satisfy another frozen acceptance criterion:

- general regex mutation;
- source-code AST semantic editing;
- Tree-sitter integration;
- language-server integration;
- workspace-wide `find_references`;
- semantic symbol rename across a codebase;
- YAML provider;
- HTML provider;
- XML provider;
- Markdown structural provider;
- arbitrary scripting;
- loops;
- general if/else workflow programming;
- variables/general expression language;
- network access;
- arbitrary subprocess execution;
- Git operations;
- test runner;
- formatter/linter;
- dynamic plugin system;
- signed/cryptographic correctness claims;
- remote provider marketplace;
- full multi-agent merge system;
- mandatory custom `.sut` syntax.

If a worker starts building one of these because it is “cool” or “future proof”, stop it.

---

# 16. Provider model

Suture Core owns mutation safety.

Providers own format knowledge.

## Common provider responsibilities

A provider may:

- parse;
- identify structured targets;
- expose supported operations;
- propose mutation plans;
- validate candidate output;
- declare preservation capabilities/limitations.

A provider may NOT:

- commit independently;
- weaken an operation silently;
- claim preservation it cannot guarantee.

## Universal vocabulary is an interface, not a universal storage tree

Do not flatten all formats into one generic JSON-like tree and regenerate documents from it.

The provider-native lossless/concrete representation remains authoritative.

This avoids destroying format-specific realities such as:

- comments;
- duplicate keys;
- quote style;
- key ordering;
- YAML anchors/aliases later;
- XML namespaces later;
- Markdown/code trivia later.

---

# 17. Built-in v0.1 providers

Ship these inside the main binary.

No plugin installation for core usefulness.

## 17.1 Raw text provider

Required capabilities:

- identify exact literal/content target;
- replace;
- insert before;
- insert after;
- delete;
- move if core move is accepted;
- exact cardinality;
- content/context anchors;
- invisible-text diagnostics;
- byte preservation outside changed ranges;
- line-ending and BOM preservation.

Do not include general regex mutation.

## 17.2 Strict JSON provider

Required goals:

- strict JSON parsing/validation;
- structured identify/path selection;
- set;
- insert/add mapping/array value where semantics are well-defined;
- delete;
- rename key if implemented;
- preserve unrelated bytes/formatting as strongly as the chosen concrete-syntax mechanism permits;
- validate final strict JSON before commit.

Avoid the naïve pipeline:

```text
parse to serde_json::Value
modify
serialize whole document
```

if that would rewrite unrelated representation.

Investigate/use concrete-syntax or targeted source-preserving mutation.

A candidate is `jsonc-parser` in strict configuration if its current CST editing API meets requirements.

## 17.3 TOML provider

Required goals:

- parse TOML;
- structured identify/path selection;
- set;
- insert mapping/value where supported;
- delete;
- rename key if safe;
- preserve comments, whitespace and relative ordering to the extent guaranteed;
- validate final TOML before commit.

`toml_edit` is an obvious candidate, but verify current behaviour and document limitations.

Provider certificates must not overclaim dotted-key ordering, metadata, or other properties that the underlying representation cannot preserve.

---

# 18. Preservation model

Default principle:

> **Everything not explicitly authorised is protected state.**

Where v0.1 can make a hard claim, use hard facts instead of subjective scores.

Possible certificate facts:

```text
unrelated_byte_regions_changed: 0
comments_removed: 0
line_endings_changed: false
bom_changed: false
final_newline_changed: false
formatter_invoked: false
```

Do NOT invent a universal “formatting drift score”.

## Preservation dimensions to model where relevant

- unrelated bytes;
- comments;
- whitespace;
- indentation;
- ordering;
- quote style;
- trailing commas/conventions where format permits;
- CRLF/LF;
- encoding/BOM;
- final newline;
- file permissions;
- timestamps/ACLs/extended metadata only if genuinely supported.

If a requested preservation guarantee cannot be met:

```text
REFUSED: PRESERVATION_UNAVAILABLE
```

unless the caller explicitly chose a lossy mode supported by that provider.

Lossy mode should be explicit and visible in the certificate.

---

# 19. Encoding and newline handling

v0.1 must deliberately support a small known set rather than guessing.

## Required

- UTF-8
- UTF-8 with BOM
- LF
- CRLF
- preservation of existing final-newline state

## Optional only if clean

- UTF-16LE with BOM
- UTF-16BE with BOM

Unknown/legacy encodings should refuse rather than guess unless a tested explicit decoder is added.

Keep unchanged source bytes unchanged where feasible rather than decoding and re-encoding the entire file.

This is particularly important for the raw text provider.

---

# 20. Stable targeting and anchors

Position-only anchors are not trustworthy.

Line numbers and byte offsets may be returned for diagnostics, but they must not be treated as durable identity by themselves.

## Required design direction

An anchor should be content/context addressed.

It may combine:

- source/document identity;
- provider;
- target content hash;
- stable surrounding context;
- structural parent/path where available.

The implementation does not need the final perfect future anchor system in v0.1, but it must avoid creating a false abstraction where “line 87” is treated as a stable target after mutation.

## Future guard modes to preserve in architecture

### strict snapshot

Any change to the observed file invalidates the request.

### region guard

Unrelated changes may be tolerated if the exact target/guarded region remains provably unchanged.

v0.1 may implement strict whole-file guarding first if that is materially safer and simpler, but structure the design so a later region guard does not require rewriting the entire protocol.

---

# 21. Idempotence and replay

A repeated call caused by timeout/retry must not casually double-apply a mutation.

Where the desired-state semantics permit it:

- second application should return NO_CHANGE if the requested desired state already holds;
- otherwise a stale/changed guard should refuse.

Do not achieve idempotence by weakening identity or cardinality checks.

Add explicit test cases for retry-after-success.

---

# 22. Workspace and filesystem security

This is non-optional.

## Workspace root

Every mutation occurs under an explicit declared workspace root.

Relative mutation paths resolve within that capability/root.

## Path traversal

Refuse `..` or equivalent traversal that escapes the workspace.

## Symlink escape

Resolve/handle symlinks so an in-workspace path cannot cause mutation of an external object.

## TOCTOU

Do not:

1. canonicalise/validate a path;
2. release that identity;
3. later reopen the same pathname and assume it still refers to the same object.

The guard and commit path must preserve object/directory confinement across verification and write.

A capability-oriented approach such as `cap-std` is worth using if it composes correctly with the commit implementation.

## Atomic replacement

Single-file v0.1 edits must be staged and committed safely.

Investigate cross-platform atomic-write primitives/libraries, but do not falsely claim metadata preservation or universal atomic semantics.

If the chosen Windows/Unix implementation differs, expose the real guarantee in tests/documentation.

## Metadata

Preserve file permissions where safely supported.

Do not claim ACL/timestamp/xattr preservation unless tested and guaranteed.

---

# 23. Windows / WSL / Linux path model

Suture should reduce, not amplify, platform confusion for AI agents.

Treat a path as typed/contextual data rather than merely a slash-filled string.

A conceptual model may include:

```rust
enum PathNamespace {
    Native,
    Windows,
    Wsl { distro: Option<String> },
    Posix,
}
```

The exact API is flexible.

## Required behaviours

- structured invocation, not shell-string mutation commands;
- no need for the agent to escape content through PowerShell/Bash layers;
- preserve file line endings independent of host OS;
- respect filesystem case sensitivity;
- safely resolve symlinks;
- distinguish a Windows path from a Linux-native path;
- do not blindly assume WSL drive mounts always use `/mnt/c`;
- where mapping is supported, detect/resolve WSL automount configuration rather than hard-code it;
- echo/display paths in the caller’s original namespace where practical;
- return explicit unmappable/context errors rather than inventing a path.

## Important separation

Path namespace and execution environment are distinct concepts.

Do not infer “this is running on Windows” solely because the request contained backslashes.

---

# 24. Commit semantics

## Required v0.1

Atomic/safe single-file commit with precommit verification and postcommit identity/evidence.

The runtime should:

1. observe/open under workspace confinement;
2. establish source identity;
3. build candidate in memory/staging;
4. validate candidate;
5. confirm guard/object identity immediately before commit;
6. commit safely;
7. establish post identity;
8. emit certificate.

## Multi-file transactions

The architecture may reserve a transaction envelope, but true multi-file atomicity is NOT a reason to delay v0.1.

Do not claim universal multi-file atomicity.

Future guarantees may be labelled:

- atomic;
- transactional-with-rollback;
- best-effort rollback.

If any multi-file feature is included early, it must state its real guarantee exactly.

---

# 25. Certificate and response contract

The default response is for an AI context window.

It must be compact.

## Required successful certificate fields

At minimum, machine-readable equivalents of:

- protocol/schema version;
- `status: APPLIED | NO_CHANGE | REFUSED | FAILED`;
- request/operation id if useful;
- file/path;
- provider name/version;
- expected/observed cardinality;
- changed ranges;
- exact minimal diff with bounded context;
- pre-mutation identity/hash;
- post-mutation identity/hash when applied;
- structured validation result where applicable;
- preservation facts/guarantees;
- refusal/failure code and diagnostics where applicable.

## Diff policy

Do not echo whole files by default.

For normal tiny edits, return the small exact changed region.

For huge diffs, bound output and return:

- changed region/line count;
- diff hash or evidence hash;
- retrieval mechanism/flag if needed.

Suture must not save 800 reread tokens only to emit a 2,000-line certificate.

## Hash language

Hashes prove identity/integrity evidence.

Do NOT call a hash a proof that the semantic edit was conceptually correct.

Use terms such as:

- evidence certificate;
- deterministic mutation evidence;
- identity hash.

Do not market v0.1 as cryptographic correctness proof.

---

# 26. Dry-run

Every supported mutation must have a cheap dry-run path.

Dry-run should perform the same:

- identification;
- cardinality;
- guard checks;
- candidate construction;
- validation;
- preservation checks;

but not persist the mutation.

Return the would-be compact certificate/diff.

Dry-run must not become a second separate mutation engine.

Reuse the same planning/verification pipeline.

---

# 27. Human CLI

AI-first does not mean hostile to humans.

Provide a small conventional CLI around the structured protocol.

Reasonable commands/surfaces:

```text
suture --version
suture doctor
suture schema
suture apply --request request.json
suture dry-run --request request.json
```

Exact spelling is Yardmaster-owned.

Important requirements:

- JSON/stdin-friendly;
- machine-readable output;
- concise human output when requested;
- no daemon;
- no interactive setup required.

`doctor` should be boring and useful, e.g.:

```text
Suture 0.1.0
Platform: Windows
Workspace access: OK
Text provider: ready
JSON provider: ready
TOML provider: ready
```

Do not turn doctor into dependency archaeology.

---

# 28. Installation and distribution

Installation friction is a product requirement.

## End-user expectation

One native executable:

```text
suture.exe
suture
```

No:

- Rust toolchain for end users;
- Cargo for end users;
- Python;
- Node;
- Java;
- opam;
- virtual environment;
- daemon;
- database;
- account;
- cloud login;
- mandatory configuration;
- telemetry requirement;
- network requirement.

Deleting the binary should substantially uninstall the program.

## Release targets

Required:

- Windows x86_64;
- Linux x86_64 suitable for normal Linux/WSL use.

Strongly desirable if cheap:

- Linux musl/static-ish artefact;
- macOS arm64/x86_64.

Use current Rust distribution tooling such as cargo-dist or a simpler reproducible release workflow if appropriate.

Produce checksums.

Do not make packaging machinery larger than Suture itself.

---

# 29. Performance requirement

Ordinary local edits should feel effectively instantaneous.

The common path is:

```text
open
→ identify/hash
→ plan
→ candidate
→ validate
→ safe write
→ post identity
→ compact certificate
```

No daemon is needed.

Do not build an abstraction palace that makes a one-line edit take seconds.

Add simple benchmark timing for:

- tiny files;
- ordinary config files;
- large text file with a tiny edit.

Do not chase microseconds at the cost of semantic safety.

---

# 30. Other-tool boundaries

Suture intentionally cooperates with existing tools rather than replacing them.

## Not a search engine

Use `rg`/grep/LSP/etc. to discover where something might be.

Use Suture to establish and mutate a safe target.

## Not Git

Do not implement branch, merge, commit, status or push.

## Not a formatter

Do not replace prettier, black, gofmt, ruff, rustfmt, etc.

## Not a test runner

Callers/GARY/CI own behavioural tests.

## Not a language server

No workspace semantic reference graph in Core.

## Not a scripting language

Complex planning/branching remains in the AI/controller.

---

# 31. Generated files

This is not required to be sophisticated in v0.1.

If practical, detect common generated/do-not-edit markers and expose them in observation/refusal diagnostics.

Do not attempt to infer the true generator/source relationship without evidence.

Do not build a generated-file framework before core acceptance.

---

# 32. Secrets and evidence privacy

Certificates/diffs can accidentally reproduce secrets present in edited content.

Design the certificate layer so later redaction policy is possible.

For v0.1:

- never log unrelated file content;
- bound context;
- do not include environment secrets/tokens;
- avoid full-file echoes;
- tests should include a fixture demonstrating that unrelated secret-looking text elsewhere in a file is not copied into the certificate.

If an authorised changed region itself contains sensitive data, document the current behaviour explicitly rather than pretending it is always safely redacted.

---

# 33. Error model

Use a typed, stable error/refusal taxonomy.

Examples of machine-readable refusal codes:

```text
TARGET_NOT_FOUND
CARDINALITY_MISMATCH
CARDINALITY_AMBIGUOUS
STALE_IDENTITY
OUTSIDE_WORKSPACE
SYMLINK_ESCAPE
UNSUPPORTED_ENCODING
MALFORMED_DOCUMENT
PROVIDER_CAPABILITY_MISSING
PRESERVATION_UNAVAILABLE
LOSSY_OPERATION_REQUIRES_OPT_IN
UNSUPPORTED_OPERATION
```

Examples of runtime failures:

```text
IO_FAILURE
PROVIDER_FAILURE
INTERNAL_INVARIANT
COMMIT_FAILURE
```

The exact names may improve, but categories must remain distinguishable.

---

# 34. Documentation deliverables

The finished repository must include concise, technically serious documentation.

## README

Must explain:

- what Suture is;
- what problem it solves;
- install;
- 60-second example;
- protocol/CLI examples;
- outcomes;
- providers;
- safety properties;
- limitations/non-goals;
- benchmark headline results;
- Windows/WSL/Linux support.

## Architecture document

Explain:

- Core/provider trust boundary;
- mutation pipeline;
- type-state/identity model;
- workspace confinement;
- commit model;
- certificate design.

## Protocol document

Document:

- request schema/versioning;
- operation semantics;
- cardinality;
- outcomes;
- refusal codes;
- certificate shape;
- provider capabilities.

## Provider authoring/internal contract document

Even though external plugins are deferred, document the in-process provider contract well enough that a future external protocol can be designed from evidence.

## Threat model

Cover:

- path traversal;
- symlink escape;
- TOCTOU;
- stale context;
- malformed structured input;
- resource/size limits;
- certificate leakage;
- provider overclaiming;
- concurrent mutation.

## Benchmark report

Must state methodology, models/tools used, corpus categories, metrics, raw/summary results and limitations.

Do not massage failures away.

---

# 35. Test philosophy

Suture is a correctness tool. Tests are part of the product.

Prefer:

- table-driven tests;
- property tests where they materially strengthen invariants;
- golden/snapshot tests for stable protocol/certificate shapes;
- adversarial filesystem fixtures;
- targeted integration tests;
- cross-platform CI.

Use idiomatic full-strength Rust testing libraries/features if useful.

Do not avoid property-based testing merely because it is advanced; use it if it earns its keep.

## Key properties worth testing

Examples:

- applying a ByteEdit plan cannot change bytes outside authorised ranges;
- non-overlapping edits compose deterministically;
- overlapping plans refuse;
- parseable JSON remains parseable after every accepted JSON operation;
- parseable TOML remains parseable after every accepted TOML operation;
- dry-run candidate equals applied candidate for the same unchanged source;
- APPLIED certificate hashes correspond to actual pre/post bytes;
- repeated desired-state operation becomes NO_CHANGE or valid stale refusal, never double-applies;
- outside-workspace/symlink escape cannot mutate external fixture;
- line-ending preservation holds for unchanged lines.

---

# 36. Adversarial v0.1 corpus

Build a corpus based directly on observed AI failure modes.

At minimum include:

1. unique exact literal target;
2. duplicate exact target;
3. zero target;
4. two near-identical blocks in different contexts;
5. stale whole-file identity;
6. multiple sequential operations;
7. retry after successful operation;
8. CRLF file;
9. LF file;
10. UTF-8 BOM;
11. final newline present;
12. final newline absent;
13. TAB vs spaces;
14. NBSP vs normal space;
15. zero-width character;
16. smart quote mismatch;
17. Unicode normalization edge;
18. ugly but valid JSON formatting;
19. minified JSON;
20. JSON scalar set;
21. JSON array insert/delete;
22. malformed JSON;
23. TOML comments;
24. TOML unusual spacing;
25. TOML ordering;
26. TOML set/delete;
27. malformed TOML;
28. path traversal;
29. symlink escape;
30. concurrent/unrelated file modification;
31. large file with one tiny edit;
32. changed target after identify;
33. unsupported encoding;
34. provider capability missing;
35. lossy preservation refusal;
36. overlapping mutation plan;
37. certificate bounded-output behaviour;
38. unrelated secret-looking text not echoed in certificate.

Add more if they test a frozen invariant rather than inflate counts.

---

# 37. Comparative benchmark / falsification experiment

Suture must prove usefulness, not merely compile.

Compare Suture against practical baseline agent-edit methods, using the same edit corpus where possible:

- conventional str_replace/search-replace;
- apply_patch/unified patch style;
- full-file rewrite where representative;
- jq for JSON where relevant as a specialised baseline, noting representation differences.

Do not force every baseline onto cases where it is nonsensical.

## Use multiple AI models if available

The purpose is to test whether Suture is robust across agent styles rather than tuned to one model.

At minimum use available representative models/agents from the existing toolbox if cost permits.

Keep the run bounded and cheap.

## Measure

- valid request/tool-call rate;
- first-attempt correct target;
- correct final file;
- unintended changed regions;
- refusal rate;
- refusal recovery calls;
- full-file verification rereads required;
- input/output tokens or approximate context volume where measurable;
- wall-clock/tool overhead;
- wrong mutation reported as success.

## Primary metric

> **WRONG MUTATION REPORTED AS SUCCESSFUL**

This is the most important metric.

Suture should drive this toward zero.

## Secondary product hypothesis

Suture should reduce verification rereads/context volume for successful tiny edits.

Do not claim a 20x/100x token reduction unless the actual experiment measures it.

## Falsification rule

If the benchmark shows that Suture does not materially improve mutation safety/observability and does not meaningfully reduce retry/reread burden compared with existing tools, report that plainly.

Do not keep expanding Suture out of sunk-cost loyalty.

---

# 38. Acceptance criteria

GARY may declare Suture v0.1 accepted only when ALL required criteria below pass.

## Build

- Rust project builds cleanly on supported targets.
- `cargo test` passes.
- `cargo fmt --check` passes.
- `cargo clippy` passes with an appropriately strict policy chosen by the Yardmaster.
- no unexplained unsafe code exists in Suture-owned source.
- release binary launches without Rust/Cargo installed on the target system.

## Protocol

- strict versioned JSON request/response schema exists;
- Rust protocol types and generated schema cannot silently drift;
- unknown invalid operation shapes refuse/parse-fail deterministically;
- APPLIED/NO_CHANGE/REFUSED/FAILED are typed and tested.

## Raw text

- exact single-target replace works;
- duplicate target refuses with actionable candidate diagnostics;
- missing target refuses;
- insert-before/insert-after/delete work;
- move works if included;
- unchanged bytes outside authorised edits are verified;
- CRLF/LF/BOM/final-newline preservation tests pass;
- invisible-character diagnostics have focused tests.

## JSON

- strict JSON provider is source-preserving enough to avoid unrelated whole-file rewrite;
- supported structured set/insert/delete operations work;
- malformed JSON refuses;
- output is parsed/validated before APPLIED;
- no text fallback occurs.

## TOML

- comments/spacing/order are preserved according to documented guarantees;
- supported structured set/insert/delete operations work;
- malformed TOML refuses;
- output is parsed/validated before APPLIED;
- provider limitations are surfaced rather than hidden.

## Identity/cardinality

- stale identity refuses;
- expected cardinality is enforced;
- no destructive first-match fallback exists;
- replay/idempotence tests pass.

## Filesystem security

- workspace traversal test passes;
- symlink escape test passes on supported platforms;
- TOCTOU-sensitive design is documented and implementation tested as far as feasible;
- commit path does not deliberately drop from safe capability verification to an unguarded pathname reopen;
- single-file safe/atomic replacement tests pass;
- real metadata-preservation limitations are documented.

## Certificates

- compact diff-only certificate exists;
- hashes/identities correspond to real pre/post content;
- provider validation result is included;
- preservation facts are included;
- refusal certificates contain actionable structured diagnostics;
- default response does not full-echo large files;
- unrelated secret-looking fixture content is not leaked into normal bounded certificate context.

## Dry-run

- dry-run uses the same planning/verification pipeline as apply;
- dry-run produces the same candidate/diff as apply when source identity is unchanged;
- dry-run never persists.

## Platform

- Windows x86_64 binary verified;
- Linux x86_64/WSL binary verified;
- path/line-ending behaviour covered by platform-specific tests;
- `suture doctor` reports usable built-in provider/platform state.

## Packaging

- release binaries generated;
- checksums generated;
- install/use instructions require no language runtime;
- packaged binary can be placed on PATH and used immediately.

## Benchmark

- adversarial corpus committed;
- comparative benchmark executed;
- results preserved in machine-readable/raw form plus human report;
- failures are not removed from report;
- primary “wrong mutation reported as successful” metric is clearly reported;
- reread/context-saving hypothesis is measured where practical.

## Documentation

- README complete;
- architecture doc complete;
- protocol doc complete;
- threat model complete;
- provider contract doc complete;
- benchmark report complete.

---

# 39. Recommended GARY execution strategy

Yardmaster may improve the decomposition, but a sensible bounded sequence is:

## Stage A — foundation and frozen protocol

- bootstrap repo;
- Rust project/tooling;
- protocol types/schema;
- outcome/refusal taxonomy;
- document identity model;
- MutationPlan/ByteEdit model;
- certificate skeleton;
- baseline tests.

No provider sophistication until the Core contract is coherent.

## Stage B — raw text and safe commit

- workspace confinement;
- identity guard;
- exact targeting/cardinality;
- text edits;
- preservation;
- safe single-file commit;
- dry-run;
- compact diff;
- adversarial text/fs tests.

This stage should prove the trusted heart.

## Stage C — JSON provider

- strict parsing;
- structural target/set/insert/delete;
- source-preserving plan generation;
- validation;
- preservation tests.

## Stage D — TOML provider

- source-preserving TOML mutation;
- capability declarations;
- validation/preservation tests.

## Stage E — path/platform hardening

- Windows/WSL/Linux path model;
- symlink/path tests;
- BOM/newline/invisible-text diagnostics;
- package Windows/Linux binaries.

Some of this can happen in parallel with B–D if integration remains controlled.

## Stage F — adversarial benchmark

- freeze v0.1 behaviour;
- run baseline comparison;
- record raw evidence;
- fix bugs that violate frozen invariants;
- do NOT add broad v-next features to improve benchmark optics.

## Stage G — release/package

- docs;
- examples;
- checksums;
- `doctor`;
- final acceptance;
- tag/release candidate.

---

# 40. Worker decomposition guidance

GARY workers should receive bounded jobs with objective acceptance.

Useful independent slices may include:

- protocol/type-state design;
- safe workspace/commit layer;
- text provider;
- JSON provider;
- TOML provider;
- certificate/diff layer;
- cross-platform/path hardening;
- adversarial corpus;
- packaging/CI;
- benchmark harness;
- docs/threat model.

Do not let multiple workers invent incompatible copies of the protocol.

The Yardmaster owns the canonical protocol/core integration.

Where a worker proposes an abstraction, require evidence that it simplifies or strengthens a frozen invariant.

---

# 41. Source-control expectations

Use normal clean Git practice.

- preserve history;
- small coherent commits;
- no secrets;
- no generated build junk;
- lockfile committed for application reproducibility;
- CI should cover supported platforms;
- release tags follow a clear version scheme.

Suggested first release:

```text
v0.1.0
```

Do not call it 1.0.

---

# 42. Quality bar

Suture should feel more like `rg` or `jq` than an IDE platform:

- download;
- put on PATH;
- call it;
- trust the boring result.

The implementation itself may use sophisticated Rust internally.

External complexity must remain low.

The product should be:

- small;
- fast;
- deterministic;
- explicit;
- auditable;
- difficult to misuse accidentally;
- helpful when it refuses.

---

# 43. Explicit product tensions to resolve correctly

## Safety vs convenience

Choose refusal over guessing.

Then make refusal informative enough that safety does not create pointless retry loops.

## Source preservation vs structural understanding

Do not solve structure by whole-file reserialization unless the caller explicitly accepts lossy representation and the provider clearly reports it.

## Strong Rust vs needless abstraction

Use full-strength idiomatic Rust.

Do not confuse “advanced feature” with “bad complexity”.

A complex type that mechanically prevents unsafe commit states may be simpler at the system level than five runtime flags.

## Cross-platform support vs false universality

Implement real Windows and Linux/WSL behaviour.

Document platform-specific differences honestly.

## Compact evidence vs missing evidence

Certificates should be small enough for an AI context but sufficient to avoid ordinary rereads.

Bound context; do not omit critical changed-region evidence merely to save bytes.

---

# 44. Deferred v-next backlog

Do not implement during v0.1 unless explicitly amended.

Potential future work:

- region-level concurrent guards;
- richer content-addressed anchors;
- external provider process protocol;
- YAML;
- Markdown/HTML/XML;
- Tree-sitter/ast-grep-backed code providers;
- semantic anchors;
- multi-file transaction guarantees;
- signed/attested certificates;
- human `.sut` textual representation;
- Suture View/fmt;
- richer encoding support;
- generated-file policies;
- optional bounded pattern provider if benchmarks prove a real need.

A future pattern provider must not reintroduce an unbounded regex-shaped security/ambiguity problem casually.

---

# 45. Definition of done

Suture v0.1 is done when:

1. a coding agent can issue a strict structured request to mutate raw text, JSON or TOML;
2. the runtime establishes identity and cardinality instead of guessing;
3. unrelated file state is preserved according to explicit guarantees;
4. invalid/ambiguous/stale operations refuse with enough structured information for a useful retry;
5. the mutation is safely committed only after verification;
6. a compact evidence certificate lets the caller confirm normal edits without rereading the entire file;
7. Windows and Linux/WSL builds install as single native executables;
8. the adversarial benchmark demonstrates exactly where Suture is safer/useful and where it is not;
9. all acceptance tests pass;
10. v-next features have NOT been allowed to bloat the trusted v0.1 core.

If these are true, release `v0.1.0`.

If the benchmark materially falsifies the core value proposition, stop and report rather than expanding the product until it becomes unfalsifiable.

---

# 46. Final instruction to the Yardmaster

Build the smallest complete Suture that deserves to exist.

Do not optimise for feature count.

Optimise for this:

> **When Suture says APPLIED, the caller should have unusually strong, compact evidence that exactly the authorised mutation occurred and that ambiguity, stale context, structural invalidity and workspace escape were not silently tolerated.**

And preserve the second half of the product:

> **When Suture cannot establish that safely, refuse clearly enough that an AI can recover without guessing.**
