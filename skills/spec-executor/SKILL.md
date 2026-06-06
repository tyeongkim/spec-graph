---
name: spec-executor
description: >
  Manages spec-graph updates during implementation. Tracks phase progress by registering
  new entities discovered during development (API, STT, TST, QST, ASM, CHG), adding delivers
  relations for completed work, and running impact analysis before changes. Does NOT
  control how implementation is done — only keeps the graph in sync with reality.
  Use when implementing a phase and needing to update spec-graph, when starting work on
  a phase, when checking what remains to implement, or when registering implementation
  artifacts in the graph. Requires spec-graph CLI and an existing .spec-graph/ with a
  plan created by spec-planner.
---

# spec-executor

Keeps spec-graph synchronized with implementation progress. This skill manages graph
updates only — implementation approach is the agent's discretion.

## Required Skill Dependencies

**STOP. Before proceeding with this skill, you MUST load the `spec-graph` skill.**

The `spec-graph` skill provides the schema knowledge required to operate spec-graph correctly:
- Valid entity types and their ID prefixes (API, STT, TST, QST, ASM, etc.)
- Valid relation types per layer (`delivers`, `covers`, `belongs_to`, `precedes`, etc.)
- Edge matrix: which `(from_type, to_type, relation_type)` combinations are allowed
- Impact analysis semantics: what `spec-graph impact` returns and how to act on it

**Without `spec-graph` skill loaded, you will hit `INVALID_INPUT` and `INVALID_EDGE` errors
and waste tokens guessing at the schema.** This is non-negotiable.

To load it, invoke the `skill` tool with `name="spec-graph"` before continuing.

## Prerequisites

- `spec-graph` CLI installed and available in PATH
- Existing `.spec-graph/` with at least one PLN and PHS entities (created by spec-planner)

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
| PHS: draft → active | **spec-executor (you)** | Predecessor phases resolved (soft — warn if not) |
| PHS: active → resolved | spec-verifier | All deliverables verified, gate passes |
| Any → deprecated | User (manual) | `--force` required |

### Rules

1. **Only one active PLN** at a time — `single_active_plan` check enforces this.
2. **PHS activation order**: phases with `precedes` predecessors should be activated in order.
3. **PHS MUST be active before work begins**: If status is `draft`, transition to `active` first. Never work on a draft phase.
4. **No skipping states**: `draft → resolved` is invalid. Must pass through `active`.
5. **deprecated is terminal**: no transitions out of `deprecated`.
6. **Verify activation**: After `phase next --activate`, confirm the response shows `"activated": true`. If `false`, manually run `entity update PHS-XXX --status active`.

---

## Core Principles

1. **Graph reflects reality**: Only add `delivers` when implementation is actually complete.
2. **Impact first**: Always run `impact` before modifying entities affected by your changes.
3. **Scope discipline**: Write only to the current PHS. Other phases are read-only.
4. **Query before create**: Check existing entities before registering new ones.
5. **Delegate when possible**: If `task()` is available, delegate code work and keep graph updates to yourself.

---

## Procedure

### Step 0: Phase Selection

#### Case A: User specifies a PHS

Validate the specified phase:

```bash
# Check phase exists and get its metadata
spec-graph entity get PHS-XXX

# Check predecessor phases are resolved
spec-graph query neighbors PHS-XXX --depth 1
```

Verify all `precedes` predecessors have status `resolved`. If not:

```
PHS-XXX cannot proceed optimally.
Reason: PHS-YYY (predecessor) is not yet resolved.
Currently recommended phase: PHS-YYY

Proceed with PHS-XXX anyway?
```

If user confirms, proceed regardless. Do not block.

#### Case B: User does not specify a PHS

Use the `phase next` shortcut:

```bash
spec-graph phase next --activate
```

This command finds the next eligible phase (predecessors resolved, lowest order) within
the active plan, activates it, and returns scope information in one call.

If you need to inspect without activating:

```bash
spec-graph phase next
```

#### Activate the Phase (MANDATORY)

**A phase MUST be `active` before any work begins.** Working on a `draft` phase is forbidden.

Once phase is selected, transition to active:

```bash
# Only if current status is draft
spec-graph entity update PHS-XXX --status active
```

**Verification**: After activation, confirm status:
```bash
spec-graph entity get PHS-XXX
# Verify: "status": "active"
```

If using `phase next --activate`, verify the response contains `"activated": true`.

### Step 1: Scope Review

Query what this phase covers:

```bash
spec-graph query scope PHS-XXX
```

Parse the output to identify:
- Arch entities covered by this phase (via `covers` relations)
- Which of those already have `delivers` relations (already done)
- Remaining work = covered entities without `delivers`

Present the work summary:

```
Phase PHS-XXX scope:
- Total covered: N entities
- Already delivered: M entities
- Remaining: K entities

Remaining work:
- REQ-001: "..."
- REQ-003: "..."
- DEC-002: "..."
```

### Step 2: Pre-Implementation (Impact Analysis)

Before implementing each entity, run impact:

```bash
spec-graph impact REQ-001
```

Inform the agent/user of affected entities:

```
Implementing REQ-001 affects:
- API-005 (high, structural) — direct implementation
- TST-003 (medium, behavioral) — verifies this requirement

Consider these when implementing.
```

This step is informational. It does not block implementation.

### Step 3: During Implementation (Entity Registration)

As implementation reveals new artifacts, register them:

```bash
# Query first — avoid duplicates
spec-graph entity list --type interface --layer arch

# Register discovered API
spec-graph entity add --type interface --id API-001 \
  --title "POST /api/auth/login" \
  --metadata '{"kind":"http"}'

# Register test
spec-graph entity add --type test --id TST-001 \
  --title "Auth login returns JWT on valid credentials" \
  --metadata '{"kind":"integration"}'

# Register state transition
spec-graph entity add --type state --id STT-001 \
  --title "User: unauthenticated → authenticated" \
  --metadata '{"entity":"User","from":"unauthenticated","to":"authenticated"}'

# Register open question (if discovered)
spec-graph entity add --type question --id QST-001 \
  --title "Should refresh tokens be stored in Redis or DB?" \
  --metadata '{"owner":"backend-team"}'

# Register a change (lightweight work unit — independent of phase)
spec-graph entity add --type change --id CHG-001 \
  --title "Fix authentication token expiry bug" \
  --metadata '{"changed_entities":["internal/auth/token.go","internal/auth/token_test.go"]}'

# Connect change to arch scope via covers
spec-graph relation add --from CHG-001 --to REQ-001 --type covers
```

Note: CHG entities are independent — they do NOT belong to any phase or plan.
Do NOT add belongs_to, precedes, or blocks relations for CHG.
CHG uses `covers` only (not `delivers`) — it is a scope marker, not a delivery unit.

Add arch-internal relations:

```bash
# API implements requirement
spec-graph relation add --from API-001 --to REQ-001 --type implements

# Test verifies requirement
spec-graph relation add --from TST-001 --to REQ-001 --type verifies

# Interface triggers state
spec-graph relation add --from API-001 --to STT-001 --type triggers
```

Validate after each batch of mutations:

```bash
spec-graph validate --layer arch
```

### Step 4: Post-Implementation (delivers)

When an arch entity's implementation is confirmed complete, add `delivers`:

```bash
spec-graph relation add --from PHS-XXX --to REQ-001 --type delivers
spec-graph relation add --from PHS-XXX --to API-001 --type delivers
```

**Rules for delivers**:
- Only add when implementation is actually done (not planned, not in-progress)
- Use the minimal proxy set — not every related entity, only those necessary and sufficient
- Only add delivers for the current PHS (scope discipline)
- The CLI auto-transitions target entities from `draft` to `active` on delivers — do NOT manually set status to active after adding delivers

Validate after adding delivers:

```bash
spec-graph validate --layer mapping --phase PHS-XXX
```

### Step 5: Progress Report

After each work session, summarize:

```
Phase PHS-XXX progress:
- Delivered: M / N entities
- New entities registered: [list]
- Open questions: [list if any]
- Remaining: [list]
```

**Git**: Commit `.spec-graph/` changes after completing the progress report:
```bash
git add .spec-graph/ && git commit -m "spec-graph: PHS-XXX progress - delivered M/N entities"
```

---

## Delegation Policy

When `task()` is available (orchestration environment):

| Action | Owner | Delegate? |
|--------|-------|-----------|
| Code implementation | Delegated agent | Yes — via task() |
| spec-graph entity/relation CRUD | This skill (you) | Never delegate |
| Impact analysis | This skill (you) | Never delegate |
| Validation | This skill (you) | Never delegate |

**Workflow with delegation**:
1. Run impact analysis (yourself)
2. Compose implementation prompt with context from impact + scope
3. Delegate code work via `task()`
4. Receive result
5. Verify implementation (yourself)
6. Register entities and delivers (yourself)
7. Validate (yourself)

**Without delegation**: Do all steps yourself.

---

## Error Handling

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Proceed |
| 1 | Runtime error | Check stderr, retry |
| 2 | Validation failure | Parse output, fix relations/entities, re-validate |
| 3 | Invalid input | Check arguments/schema, fix, retry |

---

## Anti-Patterns

1. **Premature delivers**: Adding `delivers` before implementation is complete.
2. **Scope violation**: Adding `delivers` for a phase other than the current one.
3. **Skipping impact**: Modifying entities without checking what else is affected.
4. **Bulk delivers**: Adding delivers for every related entity instead of the minimal set.
5. **Delegating graph ops**: Letting delegated agents run spec-graph commands directly.
6. **Phantom entities**: Registering entities for code that doesn't exist yet.
