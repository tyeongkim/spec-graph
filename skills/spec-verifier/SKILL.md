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

## Core Principles

1. **Graph is source of truth**: The spec-graph defines "correct". Not markdown, not memory.
2. **Failures only**: Never list passing items. Only report what failed or is missing.
3. **Evidence-based**: A `delivers` relation means verified delivery, not claimed delivery.
4. **No regression**: A fix must not break previously passing checks.
5. **Only verifier resolves**: Phase status → `resolved` is exclusively this skill's authority.
6. **Delegate fixes, own verification**: Code corrections can be delegated; judgment stays here.

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
- User says "verify all" → verify all active/draft phases in order
- No specification → find phases in `active` status and verify those

```bash
spec-graph entity list --type phase --status active
```

### Step 1: Phase Scope

Query the phase's coverage:

```bash
spec-graph query scope PHS-XXX
```

Parse to get:
- All arch entities this phase `covers`
- Which have `delivers` relations (claimed complete by spec-executor)
- Which lack `delivers` (potentially incomplete)

### Step 2: Graph-Level Verification

Run spec-graph validation checks:

```bash
# Mapping layer — phase-specific
spec-graph validate --layer mapping --phase PHS-XXX

# Arch layer — coverage and unresolved
spec-graph validate --layer arch --check coverage,unresolved

# Check for unresolved items blocking this phase
spec-graph query unresolved
```

Collect all failures. Each failure becomes a verification finding.

### Step 3: Code-Level Verification

For each arch entity covered by this phase, verify implementation exists.

#### Structural Verification (mandatory)

| Entity Type | What to Check |
|-------------|---------------|
| REQ (requirement) | Feature code exists, relevant module/file present |
| DEC (decision) | Decision reflected in code (architecture, config, patterns) |
| ACT (criterion) | Acceptance condition satisfiable by current code |
| RSK (risk) | Mitigation implemented (if mitigation was planned) |
| API (interface) | Endpoint/handler/contract exists |
| STT (state) | State transition logic implemented |
| TST (test) | Test file exists and covers the target |

Use grep, AST search, file reads to confirm existence.

#### Behavioral Verification (when possible)

```bash
# If test runner available
[run project tests]

# If build command available
[run build]
```

If neither is available, structural verification alone determines the verdict.

### Step 4: delivers Confirmation

For entities where implementation is verified:

```bash
# Check if delivers already exists
spec-graph relation list --from PHS-XXX --type delivers

# Add delivers for verified entities (if not already present)
spec-graph relation add --from PHS-XXX --to REQ-001 --type delivers
```

**Rules**:
- Only add `delivers` when YOU have verified the implementation
- Use minimal proxy set — only necessary and sufficient entities
- If spec-executor already added `delivers`, verify it's accurate. Remove if not.

### Step 5: Verdict

Classify findings by severity:

| Severity | Criteria |
|----------|----------|
| Critical (red) | Core requirement unimplemented, blocking dependency missing |
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
spec-graph entity update PHS-XXX --status resolved --reason "Verification passed: all deliverables confirmed"
```

If the gate blocks (exit 2):

```bash
# Check what's blocking
spec-graph validate --layer mapping --phase PHS-XXX --check delivery_completeness
spec-graph validate --layer mapping --phase PHS-XXX --check gates

# Fix blocking issues, then retry
spec-graph entity update PHS-XXX --status resolved --reason "Verification passed"
```

On success:

```
Phase PHS-XXX: PASSED
All deliverables verified. Phase resolved.
Next phase: PHS-YYY "[title]" (if exists)
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
