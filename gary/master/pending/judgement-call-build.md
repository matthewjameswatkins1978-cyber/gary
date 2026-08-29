# JUDGEMENT CALL — GARY MASTER BUILD DOSSIER

Frozen: 25 August 2026
Target repository: https://github.com/matthewjameswatkins1978-cyber/judgement-call.git
Starting branch: gary/recovered-jc-01-8762c907
Verified JC-01 commit: 184a71587d8bb7db0e0154bd80a13a1b0054a026

## Master objective

Finish all GARY-authorized implementation work for JUDGEMENT CALL from the verified JC-01 scaffold through the final GARY handover definition.

This is one whole project job. The Yardmaster owns decomposition, dependency ordering, parallel scheduling, retry/replan decisions, integration and final verification. Do not require Matthew/Lucy to submit JC-02, JC-03, etc. one by one.

Treat JC-01 as already complete. Do not rerun or rebuild it unless a real baseline defect is discovered.

Do not perform human-authorized AWS deployment, create AWS resources, use AWS credentials, run live Bedrock tests requiring credentials, publish the repository, record video, create screenshots, or submit to Devpost. Those remain after GARY.

## Identity and goal

PROJECT: JUDGEMENT CALL
COMPONENT: Attention Governor
TAGLINE: Only ask me when I'm useful.
HACKATHON: AWS / Amazon Agents for Humans Hackathon
TRACK: Professional Agents
DEADLINE: 15 September 2026, 1:00am BST

Build a Strands-based professional coding agent that completes a bounded software task autonomously while suppressing unnecessary human interruptions and pausing only when human judgement would materially change the outcome.

Core demo task:
Make process_items() execute work concurrently while preserving its existing public call signature and successful-result ordering. Do not invent externally visible error semantics if the existing code and tests do not determine them.

The demo must show:
1. Worker inspects repository.
2. Worker surfaces non-trivial choices through request_decision().
3. Low-value choices are automatically resolved by Attention Governor.
4. Worker edits code.
5. Worker runs tests.
6. One genuinely unresolved behavioural choice reaches a human.
7. UI presents one concise Decision Card.
8. Human chooses an option.
9. Same run/session resumes.
10. Independent mechanical verifier passes.
11. Result contains unified diff and real Attention Receipt.

The concurrency patch itself is not the product. The product is the boundary between machine work and scarce human judgement.

## Frozen architecture

Two AI roles only:
1. Worker Agent, a Strands coding agent.
2. Attention Governor plus one no-tools Gate Agent.

The Worker deliberately routes non-trivial choices through request_decision().
AttentionGovernor is a Strands intervention attached to request_decision().
Deterministic rules run before Gate Agent.
Gate Agent may return only AUTO_RESOLVE or ASK_HUMAN.
Human pause/resume uses raw Strands interrupt.
Authorization uses Strands CedarAuthorization with file policy and default deny.
Frontend is FastAPI plus vanilla HTML/CSS/JS.
Session storage uses AgentCore managed storage mounted at /mnt/workspace.
Strands FileSessionManager stores state under /mnt/workspace/.sessions.
Project workspace is /mnt/workspace/project.
Database: none.
Queue: none.
Redis: none.
React: none.
Swarm: none.
Graph: none.
Provider: Amazon Bedrock.
Model: eu.anthropic.claude-sonnet-4-6.
Region: eu-west-2.
Deployment target: Amazon Bedrock AgentCore Runtime.
Observability: built-in AgentCore/CloudWatch only.

Frozen stack:
Python 3.12, pip, Strands Agents SDK, strands-agents[cedar], Pydantic 2, FastAPI, vanilla HTML/CSS/JS, strands-agents-evals, bedrock-agentcore, bedrock-agentcore-starter-toolkit, pytest, Ruff, Python build, MIT licence.

The repository already contains docs/build-spec.md. Treat it as supporting frozen context. This master dossier is authoritative where it is more specific.

## Out of scope

Do not build arbitrary repository ingestion, GitHub OAuth/push/PR inside the product, unrestricted shell, multiple Worker Agents, a planner or supervisor inside the product, vector DB, long-term memory, AgentCore Gateway or Browser, Cognito, public accounts, billing, admin UI, model selector, token pricing UI, fancy analytics, notifications, mobile app, React, websockets unless forced by a concrete blocker, public proxy, speculative scaling, generic policy language, GARY integration, Resolve integration, Tethers integration, large visual redesign or unnecessary abstractions.

Do not reuse GARY, Resolve or Warren source code.

## Frozen service contract

JudgementCallService.start(request: StartRequest, session_id: str) -> RunResponse
JudgementCallService.resume(request: ResumeRequest, session_id: str) -> RunResponse

session_id must remain stable across start/resume.

StartRequest:
op literal start
scenario literal concurrency-demo
task non-empty max 4000 chars
No user-supplied filesystem path, repo URL, shell command or test command.

ResumeRequest:
op literal resume
interrupt_id non-empty
response.choice_id must be one of Decision Card option IDs
response.note optional max 1000 chars

Server-created TaskContract:
scenario concurrency-demo
allowed_paths:
- src/demoqueue/processor.py
- tests/test_processor.py
protected_paths:
- pyproject.toml
acceptance_command:
python -m pytest -q
frozen constraints:
public_signature = process_items(items, worker)
success_result_order = input-order
dependency_policy = no-new-runtime-dependencies
interface_style = synchronous

DecisionProposal fields:
question
options, exactly 2 to 4, each with id, label, consequence
recommendation
dimensions
impact
reversible
constraint_key nullable
evidence

Allowed dimensions:
implementation
public_behavior
security
cost
data
external_side_effect

Allowed impact:
low
medium
high

GateDecision:
action AUTO_RESOLVE with choice_id, or ASK_HUMAN with null choice_id.
No numeric confidence.

DecisionCard:
interrupt_id, question, why_human, options, recommendation, evidence.
Must not expose chain-of-thought.

AttentionReceipt:
decision_proposals
auto_resolved
human_interrupts
policy_denials
tool_calls
test_runs
final_verifier_passed

Do not estimate time saved.
Count an interruption as saved only when Governor actually processes and auto-resolves a DecisionProposal.

Response types:
completed: status, run_id, summary, diff, verification, receipt
needs_human: status, run_id, decision, receipt
failed: status, run_id, code, message, receipt

Allowed failure codes:
INVALID_REQUEST
INVALID_SESSION
WORKSPACE_ERROR
POLICY_DENIED
MODEL_ERROR
TOOL_LIMIT
VERIFICATION_FAILED
INTERNAL_ERROR

## Product Worker tools, exact set

list_tree
read_text
search_text
write_text
run_tests
request_decision

No shell.
No network.
No GitHub.
No package-install tool.

read_text rejects traversal outside workspace.
search_text is bounded and read-only.
write_text may write only TaskContract allowed_paths and rejects symlink escape and traversal.
run_tests executes exactly python -m pytest -q from demo workspace.
request_decision is the only tool allowed to request human judgement and uses raw Strands interrupt after Governor permits it.

Default maximum 40 successful product tool calls per run.
Maximum 8 DecisionProposals per run.
Exceeding either returns TOOL_LIMIT.

## Attention Governor rules

Before Gate Agent:
1. If constraint_key exactly matches a frozen constraint and that constraint determines the option, AUTO_RESOLVE.
2. If every dimension is implementation, reversible is true, and impact is low or medium, AUTO_RESOLVE using Worker recommendation.
3. If proposal lacks two concrete options or adequate evidence, return Guide instructing Worker to investigate further. Do not ask human.
4. If dimensions include external_side_effect, security, data or public_behavior and evidence does not determine the answer, send compact proposal to Gate Agent.
5. Gate Agent may return only AUTO_RESOLVE or ASK_HUMAN.

Auto-resolution returns Strands Guide with selected choice, short reason and instruction to continue.
Human-needed returns Proceed so request_decision executes raw interrupt.

Baseline evaluation mode:
governor_enabled=False.
Every valid DecisionProposal proceeds to interrupt.

Gate Agent has no tools.
Its rule is: human attention is scarce. Ask only when contract, repository evidence and reversible machine action cannot resolve a choice and the choice materially changes public behaviour, security, data handling, cost, external side effect or user intent. Internal reversible implementation choices should not reach the human.

## Cedar

File: policies/agent.cedar
Default deny.
Permit only:
list_tree
read_text
search_text
write_text
run_tests
request_decision

Filesystem path policy remains deterministic application truth.

## Happy path

start -> validate -> copy fixture -> pristine snapshot -> TaskContract -> FileSessionManager -> Worker -> Cedar tools -> request_decision -> Governor -> Guide or Proceed -> raw interrupt -> Decision Card -> resume same session with interruptResponse -> Worker continues -> tests -> independent verifier -> unified diff + Attention Receipt.

Failure rules:
prohibited path: deterministic reject and policy_denials increment
premature question: Guide investigate and no interrupt
reversible implementation choice: decision_proposals+1 and auto_resolved+1
genuine decision: decision_proposals+1 and human_interrupts+1
test failure: Worker may repair; do not ask human solely because a test failed
verifier fail: VERIFICATION_FAILED
tool/proposal limit: TOOL_LIMIT

## Security and authority

GARY workers receive no AWS credentials.
Never commit AWS access key IDs, secret access keys, session tokens, credential .env files, generated deployment credentials, Devpost credentials or public-demo access codes.

Product may invoke configured Bedrock model, write within its session workspace and emit built-in observability.
Product may not push Git, merge code, send email, call arbitrary APIs, deploy applications, modify AWS infrastructure or execute user-supplied shell commands.

GARY may create repository code, docs and templates only.
GARY must not create AgentCore runtime, IAM roles/users, access keys, secrets, public endpoints or paid cloud infrastructure.
Actual agentcore configure and agentcore deploy require later human authorization.

## Frozen decisions

Do not silently change:
1. JUDGEMENT CALL name.
2. Attention Governor differentiator.
3. Professional Agents track.
4. Python 3.12.
5. Strands Agents SDK.
6. Amazon Bedrock.
7. Claude Sonnet 4.6.
8. eu-west-2.
9. AgentCore Runtime.
10. AgentCore managed session storage at /mnt/workspace.
11. Strands FileSessionManager.
12. Cedar default deny.
13. raw Strands interrupts.
14. one Worker Agent and one no-tools Gate Agent.
15. no swarm.
16. no graph.
17. no arbitrary shell product tool.
18. no arbitrary hosted repo execution.
19. fixed Python fixture.
20. user cannot supply test commands.
21. deterministic final verifier outside worker-editable workspace.
22. no database.
23. no queue.
24. no Redis.
25. no React.
26. FastAPI plus vanilla HTML/CSS/JS.
27. no GitHub write access in product.
28. no AWS credentials to GARY workers.
29. evaluation baseline surfaces every valid DecisionProposal.
30. attention saved measured as suppressed or auto-resolved proposed interruptions, not guessed time.
31. no GARY/Resolve/Warren source reuse.
32. public API schemas are frozen.

If a frozen decision appears technically impossible, stop and return control.
Do not silently substitute another language, model, region, cloud service, frontend framework or persistence system.

## Remaining work landmarks after JC-01

These are planning landmarks, not separate operator submissions. Yardmaster may decompose differently if useful while preserving contracts, ownership and dependencies.

### JC-02 Contracts and attention ledger

Owned:
src/judgement_call/contracts.py
src/judgement_call/ledger.py
tests/test_contracts.py

Implement Pydantic models/enums:
StartRequest, ResumeRequest, TaskContract, DecisionOption, DecisionProposal, GateDecision, DecisionCard, AttentionReceipt, CompletedResponse, NeedsHumanResponse, FailureResponse.

Implement session-scoped RunLedger:
record_tool_call
record_decision_proposal
record_auto_resolve
record_human_interrupt
record_policy_denial
record_test_run
set_final_verifier
receipt

No global mutable singleton.

Tests prove invalid op rejected, invalid impact rejected, 2-4 DecisionProposal options, unknown response choice rejected, deterministic counters.

### JC-03 Workspace, tools and verifier

Owned:
src/judgement_call/workspace.py
src/judgement_call/tools.py
src/judgement_call/verifier.py
fixtures/concurrency_demo/**
tests/test_workspace.py
tests/test_tools.py

Fixture starts sequential:
process_items(items, worker) returns worker results in input order.
Fixture tests cover successful values, ordering, public signature.
Deliberately no failing-worker semantics test.

Workspace:
copy fixture
record pristine
canonical paths
reject ..
reject outside-project writes
reject symlink escape
exact allowed paths
unified diff vs pristine

Implement exact six tools.
Independent verifier checks only allowed paths changed, pytest passes, signature unchanged, success order preserved, and final behaviour matches recorded human failure-semantics choice.
Verifier lives outside Worker-editable workspace.

Traversal, protected-path and symlink escape tests required.

### JC-04 Attention Governor and Gate Agent

Owned:
src/judgement_call/gate.py
src/judgement_call/governor.py
policies/agent.cedar
tests/test_governor.py

GateDecider Protocol:
decide(proposal, contract) -> GateDecision

StrandsGateDecider:
no-tools Strands Agent
Sonnet 4.6
structured GateDecision

AttentionGovernor:
custom Strands intervention for request_decision
deterministic rules first
auto -> Guide
human-needed -> Proceed
baseline governor_enabled=False -> all valid proposals proceed

Unit tests use fake GateDecider and no AWS credentials.
Required cases:
internal+reversible+low auto
frozen constraint auto
inadequate evidence Guide
unresolved public behaviour calls Gate
Gate ASK proceeds
baseline proceeds
invalid Cedar tool denied

If current Strands intervention API materially differs from documented Guide/Proceed semantics, return control.

### JC-05 Worker Agent and service lifecycle

Owned:
src/judgement_call/agent.py
src/judgement_call/service.py
tests/test_service.py

Worker:
Sonnet 4.6
Cedar authorization
AttentionGovernor
exact tools
FileSessionManager
deterministic TaskContract prompt

Worker rules:
temporary control of one bounded coding task
inspect before editing
no writes outside allowed paths
no runtime dependencies
run tests
for every non-trivial choice with more than one plausible option call request_decision rather than guess
Governor may resolve without human
test failure alone is not a reason to ask user
do not ask for information already in repository or TaskContract
stop only when tests pass or genuinely blocked

request_decision uses ToolContext.interrupt.

Service start:
validate
initialize fixture
create contract
initialize ledger/session manager
invoke Worker
map AgentResult

Service resume:
validate interrupt
map choice to Strands interruptResponse
invoke same session
map result

On Worker completion:
run independent verifier
return completed only when verifier passes.

Tests prove Decision Card mapping, resume choice mapping, verifier failure cannot return completed, same session across resume, receipt survives pause/resume.

If FileSessionManager cannot restore Strands interrupts as required, return control.

### JC-06 Attention evaluation harness

Owned:
evals/**
tests/test_evals.py

At least 10 fixed DecisionProposal cases:
4 internal/reversible auto
2 frozen-constraint auto
4 material unresolved ask

Cover implementation, public_behavior, security, cost, data, external_side_effect.

Baseline: every valid request_decision reaches human.
Attention mode: Governor enabled.

Metrics:
total_proposals
human_interrupts
auto_resolved
expected_ask_recall
false_suppression_count

Primary safety condition:
false_suppression_count == 0

Do not alter production policy to make metrics look better.
Do not invent percentages. Generate numbers.

### JC-07 Human-facing demo UI

Owned:
src/judgement_call/web.py
static/**
tests/test_web.py

One page.
Initial:
JUDGEMENT CALL
Only ask me when I'm useful.
prefilled task
Give it the job button

Running:
quiet status
short event timeline
no chain-of-thought

Decision:
prominent Decision Card with question, why human, options, consequences, recommendation, evidence and buttons

Completed:
verification PASS/FAIL
concise diff
Attention Receipt with real counts

No chat UI.
No raw reasoning.
No CDN.

Routes:
GET /
POST /api/start
POST /api/resume

Tests mock service.

### JC-08 AgentCore runtime adapter and deployment pack

Owned:
src/judgement_call/runtime.py
scripts/invoke_agentcore.py
deploy/**
tests/test_runtime.py

Use BedrockAgentCoreApp.
One AgentCore entrypoint.
context.session_id is service session ID.
Pass start/resume payload unchanged.
Invoke script supports start and resume.
Docs describe direct-code deployment, eu-west-2 and /mnt/workspace managed session storage.

Do not deploy.
Runtime must not duplicate business logic.

If required AgentCore direct deployment/session storage is unavailable in eu-west-2, return control.

### JC-09 End-to-end local integration

Owned:
tests/live/test_demo_flow.py
scripts/demo_local.py
scripts/replay_demo.py
docs/demo.md

Offline path uses injected deterministic/fake model components and must exercise:
start
at least one auto-resolved DecisionProposal
one surfaced Decision Card
resume
successful verifier
non-empty diff
real Attention Receipt

Live path is marked pytest.mark.live, uses real Bedrock Worker/Gate and must not run in ordinary pytest.

replay_demo.py may replay stored events only with visible REPLAY MODE label and must never masquerade as live AI.

Offline demo required result:
status completed
final_verifier_passed true
auto_resolved at least 1
human_interrupts exactly 1
diff non-empty
all paths allowed
receipt internally consistent

### JC-10 Judge-facing docs and architecture

Owned:
README.md
docs/architecture.md
docs/architecture.mmd

README covers:
what product is
problem
human attention scarcity
core demo
architecture
actual Strands technology
actual AgentCore technology
local setup
offline tests
live AWS test
evaluation
security and authority
known limitations
new hackathon project and no GARY/Resolve/Warren code reuse
MIT licence

Mermaid source matches architecture.

Do not claim universal coding support, production security platform, eliminated oversight, unmeasured time savings, zero-risk autonomy, production GitHub integration or public live demo unless actually proven.

## Wave guidance

Because JC-01 is already verified:

Wave A, parallel where safe:
JC-02
JC-03
JC-04

Wave B after prerequisites, parallel where safe:
JC-05
JC-06
JC-07
JC-08

Wave C:
JC-09

Wave D:
JC-10

Yardmaster owns actual decomposition and scheduling. This is guidance, not a reason to ask the operator for separate submissions.

## Development and acceptance

Fresh clone:
python -m venv .venv
python -m pip install --upgrade pip
python -m pip install -e .[dev,deploy]

Offline checks:
python -m pytest -q
ruff check .
python -m build

Local web:
python -m judgement_call.web
default 127.0.0.1:8000

Local AgentCore entrypoint:
python -m judgement_call.runtime

Workers may install only minimum missing development dependencies for their GARY child job. Product Worker itself has no install tool.

Mechanical acceptance only.
At integration checkpoints run:
ruff check .
python -m pytest -q

Package:
python -m build

Workspace tests must prove traversal denial, disallowed write denial, symlink escape denial and no user-supplied arbitrary command.

Service tests must prove pause, Decision Card, resume and verifier failure gating.

Evaluation offline must report false_suppression_count == 0.

Model self-report is irrelevant to completion.

Offline project integration:
python scripts/demo_local.py --offline

It must start, auto-resolve at least one internal/reversible proposal, surface one unresolved failure-semantics Decision Card, resume choice A, complete, run verifier and return completed with final_verifier_passed true, auto_resolved at least 1, human_interrupts 1, non-empty diff, allowed paths only and consistent receipt.

Normal tests require no AWS credentials.

## Escalation and stop conditions

Return control if:
dossier requirements conflict
current hackathon rules materially conflict with frozen dossier
Strands APIs materially differ from assumed intervention/interrupt behaviour
AgentCore selected architecture/session storage unavailable
Sonnet 4.6 model/inference unavailable
frozen schemas conflict
secure workspace confinement requires broad shell/system authority
architecture change required
AWS credentials required
cloud resource creation required
secret required
unusual AWS cost appears
required package incompatible with Python 3.12
child work requires violating another active owned area
hackathon rule ambiguity risks invalidation

Do not silently substitute.

## Final GARY handover definition

GARY may declare this whole master job complete only when on the integrated staging branch:

repository installs from fresh clone
MIT licence exists
python -m build passes
ruff check . passes
normal pytest suite passes without AWS credentials
offline integration demo passes
Attention Governor deterministic tests pass
evaluation harness runs with real generated metrics
false_suppression_count == 0
workspace escape and path tests pass
independent verifier rejects incorrect work and gates completion
local web UI runs
Decision Card start/resume works
Attention Receipt uses real counters
AgentCore entrypoint runs locally without deployment
deployment instructions exist
no secrets or credentials committed
README accurately describes evidence and limitations
architecture Mermaid source exists
known limitations explicit
successful implementation work exists on accepted/integrated GARY branches
only human-authorized live AWS deployment, cloud smoke, screenshots, video and Devpost packaging remain

GARY must not claim complete merely because unit tests pass, a Worker says it succeeded, replay mode works, or live AgentCore/Bedrock functionality is described but untested.

## Building rule

When uncertain about adding machinery ask:
Does this help prove that JUDGEMENT CALL completes real professional work while consuming less unnecessary human judgement?

If not, do not build it.

Judge-facing product is the boundary:
machine work on one side, scarce human judgement on the other.
