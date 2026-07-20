---
name: spec-verifier
description: >
  Verifies implementation against spec-graph, confirms delivery completeness, and signs off
  phases. Performs graph-level validation (mapping, arch coverage, unresolved items) and
  code-level verification (structural + behavioral). Auto-fixes failures when possible by
  delegating code corrections. Only this skill can transition a phase to resolved status.
  Use when user asks to "verify the implementation", "check phase completion", "sign off
  this phase", "validate against the spec", "run spec verification", or when a phase
  implementation is complete and needs acceptance. Requires spec-graph CLI and an existing
  .spec-graph/ with phases in active status.
---

# spec-verifier

Verifies implementation fidelity against spec-graph and signs off completed phases.
This skill is the sole authority for transitioning phases to `resolved`.

## Required Skill Dependencies

**STOP. Before proceeding with this skill, you MUST load the `spec-graph` skill.**

The `spec-graph` skill provides the schema knowledge required to operate spec-graph correctly:
- Valid entity types and their ID prefixes (REQ, DEC, ACT, RSK, API, STT, TST, etc.)
- Valid relation types per layer (`delivers`, `covers`, `mitigates`, `verifies`, etc.)
- Validation rules: arch coverage, mapping completeness, unresolved checks (e.g., RSK needs `mitigates`)
- Phase lifecycle semantics: draft → active → resolved transitions

**Without `spec-graph` skill loaded, you will misinterpret validation output and sign off
incomplete phases.** This is non-negotiable.

To load it, invoke the `skill` tool with `name="spec-graph"` before continuing.

## Prerequisites

- `spec-graph` CLI installed and available in PATH
- Existing `.spec-graph/` with active plan and phases
- At least one PHS in `active` status (implementation done, ready for verification)

## PLN / PHS Lifecycle

### Status State Machine

```
PLN:  draft → active → resolved (gated: plan_coverage)
                     → deprecated (--force required)

PHS:  draft → active → resolved (gated: delivery_completeness + gates)
                     → deprecated (--force required)
```

### Transition Ownership

| Transition | Owner | Precondition |
|------------|-------|--------------|
| PLN: draft → active | spec-planner | Only one active plan allowed |
| PHS: draft → active | spec-executor | Predecessor phases resolved |
| PHS: active → resolved | **spec-verifier (you)** | All deliverables verified, gate passes |
| Any → deprecated | User (manual) | `--force` required |

### Rules

1. **Only spec-verifier resolves phases.** No other skill may transition PHS to `resolved`.
2. **Only `active` phases can be verified.** If a phase is `draft`, reject verification and instruct the user to activate it first via spec-executor.
3. **No skipping states**: `draft → resolved` is invalid. Must pass through `active`.
4. **deprecated is terminal**: no transitions out of `deprecated`.

## Core Principles

1. **Graph is source of truth**: The spec-graph defines "correct". Not markdown, not memory.
2. **Failures only**: Never list passing items. Only report what failed or is missing.
3. **Evidence-based**: A `delivers` relation means verified delivery, not claimed delivery.
4. **No regression**: A fix must not break previously passing checks.
5. **Only verifier resolves**: Phase status → `resolved` is exclusively this skill's authority.
6. **Delegate fixes, own verification**: Code corrections can be delegated; judgment stays here.
7. **Context first**: For task-managed phases, `phase context` is the verification contract. Use
   direct scope only for an isolated pre-existing taskless phase or explicitly supplied Markdown.

---

## Procedure

### Step 0: Verification Target

Determine scope:

```bash
# Check .spec-graph/ exists
spec-graph entity list --type plan --status active
```

If no `.spec-graph/` or no active plan:
```
No spec-graph found (or no active plan). Cannot verify.
Run spec-planner first to create a plan.
```

Determine what to verify:
- User specifies phase → verify that phase
- User says "verify all" → verify all `active` phases in dependency order
- No specification → find phases in `active` status and apply selection heuristic

```bash
spec-graph entity list --type phase --status active
```

#### Phase Selection (when multiple active phases exist)

If multiple phases are in `active` status:

1. **Check dependency order**: Verify phases whose predecessors are already `resolved` first.
   ```bash
   spec-graph query neighbors PHS-XXX --depth 1
   ```
2. **Present options to user** if no clear ordering exists:
   ```
   Multiple active phases found:
   - PHS-001 "..." (no unresolved dependencies)
   - PHS-002 "..." (depends on PHS-001)

   Recommend verifying PHS-001 first (unblocks PHS-002).
   Which phase should I verify?
   ```
3. **Never verify a phase whose dependencies aren't resolved** — it will fail the gate anyway.
4. **Reject `draft` phases** — instruct user to activate via spec-executor first.

### Step 1: Phase Context

Query the phase execution context:

```bash
spec-graph phase context PHS-XXX
```

When `tasks` is non-empty, verify the returned task contracts, prerequisites, covers/delivers,
effective scope/delivery, blockers, and ready/blocked IDs. Task-managed scope is the union of child
task mappings. Every task must resolve with QA evidence and required delivery before phase sign-off.

When `tasks` is empty, use `spec-graph query scope PHS-XXX` as an isolated legacy path only for a
pre-existing taskless phase or explicitly supplied existing Markdown. Preserve Markdown byte-for-byte;
never auto-import, delete, or reinterpret it.

### Step 2: Graph-Level Verification

Run spec-graph validation checks:

```bash
# Mapping layer — phase-specific
spec-graph validate --layer mapping --phase PHS-XXX
spec-graph validate --layer exec --check task_graph
spec-graph validate --layer mapping --phase PHS-XXX --check task_scope

# Arch layer — coverage and unresolved
spec-graph validate --layer arch --check coverage,unresolved

# Check for unresolved items blocking this phase
spec-graph query unresolved
```

Collect all failures. Each failure becomes a verification finding.

### Step 3: Code-Level Verification (Delegated Per-Entity)

For each arch entity covered by this phase, delegate verification to a sub-agent.
**Do NOT verify entities yourself sequentially.** Spawn one sub-agent per entity, in parallel.

#### Role Separation

| Responsibility | Owner |
|----------------|-------|
| Graph validation (Steps 0-2) | Verifier (you) |
| Code-level evidence gathering + logic analysis | Sub-agent |
| Interpreting sub-agent reports → PASS/FAIL/NEEDS_REVIEW | Verifier (you) |
| Spot-check code on ambiguous/conflicting reports | Verifier (you) |
| `delivers` relation management | Verifier (you) |
| Verdict and sign-off | Verifier (you) |
| Cross-entity reconciliation | Verifier (you) |

The verifier does not perform routine code-level verification itself. It **may** inspect
implementation code to adjudicate low-confidence, conflicting, incomplete, or sign-off-critical
sub-agent findings.

Sub-agents NEVER run spec-graph commands or make graph decisions.

#### Sub-Agent Mission

Each sub-agent receives exactly ONE entity and must answer TWO questions:

1. **Structural**: Does implementation code exist for this entity?
2. **Logical**: Does the implementation correctly satisfy the entity's intent?

The sub-agent must provide **evidence** for its conclusions — not bare verdicts.

#### What the Sub-Agent Must Do

For the assigned entity, the sub-agent:

1. **Locate** — Find all files/functions/modules relevant to the entity using grep, AST search, file reads. Do not stop at the first match — search comprehensively.
2. **Confirm existence** — Report whether the implementation physically exists (files, exports, handlers, configs, migrations, scripts, etc.).
3. **Analyze correctness** — Read the implementation and assess whether it logically satisfies the entity's title and description. Check:
   - Does the code do what the entity says it should?
   - Are edge cases handled that the entity implies?
   - Are there obvious logic errors, missing branches, or incomplete flows?
   - For negative requirements (e.g., "do not persist secrets"), verify absence of the forbidden pattern.
4. **Report** — Return a structured verdict with full evidence (see template below).

Do not mark `LOGIC=CORRECT` merely because a matching name exists. Only mark correct if the
implementation behavior can be traced to the entity intent.

#### What the Sub-Agent Must NOT Do

- Run spec-graph commands
- Add/remove relations or change entity status
- Make architectural decisions beyond the specific entity
- Modify any code (read-only investigation)
- Verify entities other than the one assigned
- Infer requirements beyond what the entity title/description states

#### Verification Depth (per entity type)

| Entity Type | Structural Check | Logic Check |
|-------------|-----------------|-------------|
| REQ (requirement) | Module/file exists for the feature | Feature logic matches requirement description |
| DEC (decision) | Architecture/config reflects the decision | No contradictions to the decided approach |
| ACT (criterion) | Code path exists for the acceptance condition | Given/When/Then satisfiable by current logic |
| RSK (risk) | Mitigation code present | Mitigation actually addresses the risk scenario |
| API (interface) | Endpoint/handler/contract defined | Request/response shape matches, error handling present |
| STT (state) | State transitions implemented | All declared transitions reachable, no dead states |
| TST (test) | Test file exists targeting the entity | Test assertions actually verify the claimed behavior |

For entity types not listed above: verify structural evidence matching the entity description
and assess logical satisfaction from title, description, and relations. If insufficient context,
return `CANNOT_ASSESS`.

#### Sub-Agent Prompt Template

For each entity, spawn a sub-agent with this prompt structure:

```
ROLE: You are a code verification agent. Your job is to find and analyze implementation
code for a single spec-graph entity. Report your findings — do not modify anything.

PHASE CONTEXT:
  Phase ID: <phase-id>
  Phase title: "<phase-title>"
  Source directories: <list of source dirs, e.g., src/, internal/, cmd/>
  Test directories: <list of test dirs, e.g., tests/, *_test.go>
  Known relevant files (if any): <files identified in prior steps>

ENTITY:
  ID: <ID>
  Type: <type>
  Title: "<title>"
  Description: "<description>" (if available)
  Metadata: <relevant metadata, e.g., priority, kind, acceptance criteria>

RELATED ENTITIES (for context only — do NOT verify these):
  - <ID> <type> "<title>" (relationship: <relation type>)
  - ...

TASK:
  1. LOCATE: Find all source files implementing this entity.
     - Search by: entity ID, title keywords, domain concepts, API routes, exported symbols.
     - Check source dirs, config files, migrations, scripts — not just code files.
     - Do NOT stop at the first match. Search comprehensively.
  2. STRUCTURAL CHECK: Does the implementation exist?
     - List file paths and relevant symbols (functions, types, handlers).
     - If nothing found, report MISSING and list all directories/queries searched.
  3. LOGIC CHECK: Does the implementation correctly satisfy the entity?
     - Read the code and assess whether it fulfills the title/description.
     - Flag: missing edge cases, incomplete flows, logic errors, contradictions.
     - For tests (TST): inspect assertions, not just file/test names.
     - For negative requirements: verify absence of the forbidden pattern.
     - For partial implementations: report as ISSUES_FOUND with specifics.
  4. REPORT using this exact format:

     ENTITY: <ID> "<title>"
     STRUCTURAL: FOUND | MISSING
     LOGIC: CORRECT | ISSUES_FOUND | CANNOT_ASSESS
     CONFIDENCE: HIGH | MEDIUM | LOW

     SEARCHED:
       - Queries/keywords used: [...]
       - Directories inspected: [...]
       - Files read but rejected: [...]

     EVIDENCE:
       - <file>:<symbol or line range> — <why this is relevant>
       - ...

     REASONING:
       - Structural rationale: <why FOUND or MISSING>
       - Logic rationale: <why CORRECT, ISSUES_FOUND, or CANNOT_ASSESS>

     ISSUES (if any):
       - [description of logic gap or error]

     CROSS-ENTITY NOTES (if any):
       - [mention dependencies or contradictions involving related entities — do NOT verify them]

CONSTRAINTS:
  - Do NOT run spec-graph commands.
  - Do NOT modify any files.
  - Do NOT assess entities other than the one above.
  - If you cannot determine correctness (e.g., requires runtime state, integration test,
    or external service), report CANNOT_ASSESS with explanation of what would be needed.
  - Do NOT mark LOGIC=CORRECT merely because a matching name/file exists.
    Trace behavior to entity intent.
```

#### Concurrency Rules

- **One sub-agent per entity.** Do not batch multiple entities into one agent.
- **Bounded parallelism:**
  - Entity count ≤ 8: run all in parallel.
  - Entity count > 8: run in batches of 5–8 concurrent sub-agents.
  - Still produce one report per entity.
- **Wait for all results** before proceeding to Step 4.

#### Result Interpretation (Verifier's job)

After all sub-agents return, classify each entity:

| Sub-Agent Report | Verifier Decision |
|------------------|-------------------|
| FOUND + CORRECT + HIGH/MEDIUM confidence | **PASS** |
| FOUND + CORRECT + LOW confidence | **NEEDS_REVIEW** — verifier spot-checks code |
| FOUND + ISSUES_FOUND (any confidence) | **FAIL** (with specific findings) |
| FOUND + CANNOT_ASSESS | **NEEDS_REVIEW** — blocked until verifier inspects or user accepts risk |
| MISSING (any) | **FAIL** (Critical — unimplemented) |
| Malformed/incomplete report | Rerun sub-agent or **NEEDS_REVIEW** |
| Conflicting findings across sub-agents | **NEEDS_REVIEW** — verifier reconciles |

**NEEDS_REVIEW resolution:**
- Verifier reads the relevant code directly to adjudicate.
- If verifier confirms correctness → PASS.
- If verifier finds issues → FAIL.
- If verifier cannot determine (runtime-only) → record as accepted risk with user confirmation.

**Phase sign-off requires:**
- No FAIL entities remaining
- No unresolved NEEDS_REVIEW entities
- All NEEDS_REVIEW resolved to PASS or explicitly accepted by user

#### Cross-Entity Reconciliation (Verifier's job)

After collecting all reports:

1. Check for contradictions between sub-agent findings (e.g., one reports a function exists, another says it's missing).
2. Check for dependency gaps (e.g., API entity PASS but its state transitions FAIL).
3. If sub-agents flagged cross-entity concerns in their reports, reconcile them here.
4. Contradictions or dependency gaps → NEEDS_REVIEW on affected entities.

#### Behavioral Verification (after structural + logic)

```bash
# If test runner available
[run project tests]
```

If test runner is not available, structural + logic verification alone determines the verdict.

**Important**: If behavioral tests fail but sub-agents reported PASS on related entities,
the test failure overrides the sub-agent verdict. Mark affected entities as FAIL.

#### Test Verification (MANDATORY)

**This check is non-negotiable. A phase CANNOT pass with failing tests.**

Every phase must have all tests passing upon completion. Execute the project's test suite:

```bash
# Run full test suite
[run test command — e.g., make test, go test ./..., npm test, cargo test, pytest]
```

**Rules**:
- All tests must pass (exit code 0). Any test failure = Critical severity finding.
- Skipped tests are allowed ONLY with explicit user confirmation. If tests are skipped:
  1. Ask the user: "Tests X, Y are being skipped. Confirm skip?"
  2. User must explicitly confirm.
  3. Record skip reason in the verification report.
- Pre-existing test failures unrelated to the current phase: flag to user and request confirmation to proceed. Do NOT silently ignore them.
- If no test runner exists, skip this check (not a failure).

#### Build/Run Verification (MANDATORY)

**This check is non-negotiable. A phase CANNOT pass without it.**

Every phase must leave the project in a buildable or runnable state. Execute one of the following:

```bash
# Option 1: Build command (compiled languages, bundled projects)
[run build command — e.g., make build, go build ./..., npm run build, cargo build]

# Option 2: Dev server start (web apps, interpreted languages)
[start dev server and confirm it boots without errors — e.g., npm run dev, go run .]
```

**Rules**:
- If the project has a build command → run it. Exit code 0 required.
- If the project has a dev server → start it, confirm no boot errors, then stop it.
- If both exist → build command is sufficient (it's stricter).
- If neither exists (e.g., pure library with no build step) → confirm that the language toolchain reports no errors (e.g., `go vet ./...`, `tsc --noEmit`, `python -m py_compile`).
- **Build/run failure = Critical severity finding.** It blocks phase sign-off unconditionally.

Determine the correct command by checking project config files (Makefile, package.json, Cargo.toml, go.mod, etc.).

### Step 4: delivers Confirmation

For entities where implementation is verified:

```bash
# Check task delivery in PhaseContext
spec-graph phase context PHS-XXX

# Add delivers for verified entities (if not already present)
spec-graph relation add --from TSK-XXX --to REQ-001 --type delivers
```

Use direct phase `delivers` only on the isolated taskless legacy path.

**Rules**:
- Only add `delivers` when YOU have verified the implementation
- Use minimal proxy set — only necessary and sufficient entities
- If spec-executor already added `delivers`, verify it's accurate. Remove if not.

### Step 5: Verdict

Classify findings by severity:

| Severity | Criteria |
|----------|----------|
| Critical (red) | Core requirement unimplemented, blocking dependency missing, **build/run fails**, **tests fail** |
| Major (yellow) | Significant feature gap, test coverage missing |
| Minor (green) | Non-blocking issue, cosmetic, documentation gap |

Output format (failures only):

```
# Verification Report

**Phase**: PHS-XXX "[title]"
**Verdict**: PASS / FAIL / PARTIAL

## Failures (if any)

### 1. [short title]
- **Severity**: Critical / Major / Minor
- **Entity**: REQ-001 "..."
- **Expected**: [what spec-graph says]
- **Actual**: [what code shows]
- **Fix**: [specific action]

### 2. ...
```

If PASS: skip to Step 7.

### Step 6: Auto-Fix (on FAIL)

#### With delegation (task() available):

1. Compose fix prompt from failure findings:
   - Entity ID and title
   - Expected behavior
   - Current state
   - Specific fix required
2. Delegate via `task()`
3. Receive result
4. Re-verify (yourself) — go back to Step 3 for fixed items only
5. Check for regressions on previously passing items

#### Without delegation:

1. Fix code directly
2. Re-verify
3. Check for regressions

**Max 3 retry loops.** If still failing after 3 attempts:

```
# Verification Report

**Verdict**: PARTIAL
**Resolved**: X / Y failures fixed
**Remaining** (cannot auto-fix):
- [failure description + why it can't be auto-fixed]

Manual intervention required for remaining items.
```

#### Guardrails (never violate during auto-fix):

- Never delete tests to pass verification
- Never suppress errors (`as any`, `@ts-ignore`, empty catch)
- Never introduce dependencies not in the plan
- Never change public API surface without flagging
- Preserve existing passing tests

### Step 7: Phase Sign-Off (on PASS)

When all checks pass:

```bash
# Attempt resolution — gate runs automatically
spec-graph entity update PHS-XXX --status resolved
```

If the gate blocks (exit 2):

```bash
# Check what's blocking
spec-graph validate --layer mapping --phase PHS-XXX --check delivery_completeness
spec-graph validate --layer mapping --phase PHS-XXX --check gates
spec-graph validate --layer exec --check task_graph
spec-graph validate --layer mapping --phase PHS-XXX --check task_scope

# Fix blocking issues, then retry
spec-graph entity update PHS-XXX --status resolved
```

On success:

```
Phase PHS-XXX: PASSED
All deliverables verified. Phase resolved.
Next phase: PHS-YYY "[title]" (if exists)
```

**Git**: Commit the phase resolution:
```bash
git add .spec-graph/ && git commit -m "spec-graph: PHS-XXX resolved - verification passed"
```

---

## Delegation Policy

| Action | Owner | Delegate? |
|--------|-------|-----------|
| Code fixes for failed findings | Delegated agent | Yes — via task() |
| spec-graph validation | This skill (you) | Never |
| delivers relation management | This skill (you) | Never |
| Phase sign-off decision | This skill (you) | Never |
| Verdict determination | This skill (you) | Never |

**Delegated agents must NOT**:
- Run spec-graph commands
- Add/remove relations
- Change entity status
- Make architectural decisions beyond the specific fix

---

## Error Handling

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Proceed |
| 1 | Runtime error | Check stderr, retry |
| 2 | Validation failure / gate blocked | Parse output, resolve issues, retry |
| 3 | Invalid input | Check arguments, fix, retry |

---

## Anti-Patterns

1. **Listing passing items**: Only report failures. Passing items waste tokens.
2. **Premature sign-off**: Never resolve a phase without full verification.
3. **Trusting claimed delivers**: spec-executor's delivers are claims. Verify before confirming.
4. **Deleting tests**: Never remove tests to make verification pass.
5. **Infinite retry**: Stop at 3 attempts. Report PARTIAL and ask for help.
6. **Scope creep in fixes**: Auto-fix only the specific failure. Don't refactor surrounding code.
7. **Delegating judgment**: Never let a delegated agent decide if verification passes.
8. **Legacy reinterpretation**: Never derive tasks from or delete existing Markdown.
9. **Mixed mappings**: Never combine direct phase mappings with child-task mappings.
