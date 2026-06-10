---
name: spec-planner
description: >
  Transforms specifications, requirements documents, or natural language project descriptions
  into a fully registered spec-graph plan. Creates arch entities (REQ, DEC, ACT, RSK),
  execution entities (PLN, PHS), and mapping relations (covers). Validates the graph across
  all three layers before finalizing. Use when user asks to "plan this project", "create a
  spec-graph plan", "register requirements", "break this into phases", "turn this spec into
  a plan", or provides a requirements document and wants it converted into a spec-graph
  structure. Requires spec-graph CLI installed and a .spec-graph/ directory (will init if
  absent). Does NOT generate Markdown phase files — spec-graph is the sole source of truth.
---

# spec-planner

Transforms specifications into a validated spec-graph plan. The graph — not markdown — is
the source of truth. This skill enforces a strict procedure; follow each step in order.

## Required Skill Dependencies

**STOP. Before proceeding with this skill, you MUST load the `spec-graph` skill.**

The `spec-graph` skill provides the schema knowledge required to operate spec-graph correctly:
- Valid entity types and their ID prefixes (REQ, DEC, ACT, RSK, PLN, PHS, etc.)
- Valid relation types per layer (`mitigates`, `has_criterion`, `covers`, `belongs_to`, etc.)
- Edge matrix: which `(from_type, to_type, relation_type)` combinations are allowed
- Validation rules: what makes a graph valid across arch/exec/mapping layers

**Without `spec-graph` skill loaded, you will hit `INVALID_INPUT` and `INVALID_EDGE` errors
and waste tokens guessing at the schema.** This is non-negotiable.

To load it, invoke the `skill` tool with `name="spec-graph"` before continuing.

## Prerequisites

- `spec-graph` CLI installed and available in PATH
- Project directory where `.spec-graph/` will live (or already exists)

## Core Principles

1. **Zero interpretation**: Never invent requirements. Flag ambiguities instead of guessing.
2. **Full coverage**: Every extracted requirement must map to at least one phase via `covers`.
3. **Phase continuity**: Phase N+1 depends only on phases ≤ N. No circular dependencies.
4. **Phase ID ordering**: PHS IDs MUST match execution order numerically. PHS-001 executes first, PHS-002 second, etc. Never assign PHS IDs arbitrarily — the numeric suffix IS the execution sequence.
4. **Binary acceptance**: Every exit_criteria in phase metadata must be testable as pass/fail.
5. **Phase buildability**: Every phase, when completed, MUST leave the project in a state where it builds without errors OR the dev server runs successfully. No phase may end with broken compilation or runtime boot failures.
6. **Phase test integrity**: Every phase, when completed, MUST have all tests passing. No test failures are allowed. Tests may only be skipped with explicit user confirmation — the skip reason must be recorded in exit_criteria metadata.
6. **Query before create**: Always check existing graph state before creating entities or relations.
7. **Compute first**: Run `validate` after every batch of mutations.

---

## Procedure

### Step 0: Information Gathering

**Do not proceed until sufficient information is collected.**

Minimum requirements:
- Project purpose / scope
- Core features or functional requirements
- Technical stack / constraints (language, framework, infra)
- Priority signals (must-have vs nice-to-have) if available

If input is a spec document, read it fully before proceeding.

If input is natural language and lacks the above, ask targeted questions:

```
I need a few more details before creating the plan:

1. [specific missing info]
2. [specific missing info]

What's the scope boundary — what is explicitly OUT of scope?
```

Do not ask more than 3 questions at once. Iterate if needed.

### Step 1: Input Analysis

Extract from the input:
- **Requirements** (REQ): functional and non-functional
- **Decisions** (DEC): technology choices, architectural decisions, policies
- **Acceptance Criteria** (ACT): testable conditions for requirements
- **Risks** (RSK): known risks with likelihood/impact

Additionally, if the input **explicitly** mentions:
- API endpoints, interfaces, event contracts → register as API
- State machines, state transitions → register as STT
- Test scenarios → register as TST
- Open questions → register as QST
- Assumptions → register as ASM

**Rule**: Only register what is explicit in the input. Never infer entities that aren't stated.

### Step 2: spec-graph Initialization

```bash
# Check if .spec-graph/ exists
ls .spec-graph/ 2>/dev/null

# If not, initialize
spec-graph init
```

If `.spec-graph/` already exists, check for active plans:

```bash
spec-graph entity list --type plan --status active
```

If an active plan exists, inform the user:

```
An active plan already exists: PLN-XXX "[title]"
Creating a new plan requires archiving the existing one. Proceed?
```

Wait for user confirmation. On confirmation:

```bash
spec-graph entity update PLN-XXX --status deprecated
```

### Step 3: Register arch Entities

Register extracted entities. Query before creating to avoid duplicates.

> **Note**: `--id` is now optional for single creates (auto-generated, capture from `.entity.id`). However, this batch planning workflow uses explicit `--id` so that IDs can be cross-referenced in subsequent `relation add` commands within the same pass. Keep explicit IDs here.

```bash
# Check existing
spec-graph entity list --type requirement --layer arch

# Register requirements
spec-graph entity add --type requirement --id REQ-001 \
  --title "..." \
  --description "..." \
  --metadata '{"priority":"must","kind":"functional"}'

# Register decisions
spec-graph entity add --type decision --id DEC-001 \
  --title "..." \
  --metadata '{"rationale":"...","date":"YYYY-MM-DD"}'

# Register acceptance criteria and link to requirements
spec-graph entity add --type criterion --id ACT-001 \
  --title "..." \
  --metadata '{"given":"...","when":"...","then":"..."}'
spec-graph relation add --from REQ-001 --to ACT-001 --type has_criterion

# Register risks
spec-graph entity add --type risk --id RSK-001 \
  --title "..." \
  --metadata '{"likelihood":"medium","impact":"high"}'

# Register arch-internal relations
spec-graph relation add --from REQ-001 --to DEC-001 --type constrained_by
spec-graph relation add --from REQ-002 --to REQ-001 --type depends_on
```

### Step 4: Create PLN + PHS Entities

Determine phase count based on project scale:

| Scale | Signals | Phases |
|-------|---------|--------|
| Small | 1-2 modules, <10 endpoints | 2-4 |
| Medium | Multiple modules, auth/DB, 3-5 integrations | 4-7 |
| Large | Multi-service, CI/CD, monitoring | 6-12 |

```bash
# Create plan (--status active is required for phase next to work)
spec-graph entity add --type plan --id PLN-001 \
  --title "..." \
  --status active

# Create phases (all as draft)
spec-graph entity add --type phase --id PHS-001 \
  --title "Phase 1 - ..." \
  --metadata '{"goal":"...","order":1,"exit_criteria":["criterion 1","criterion 2"]}'

spec-graph entity add --type phase --id PHS-002 \
  --title "Phase 2 - ..." \
  --metadata '{"goal":"...","order":2,"exit_criteria":["criterion 1","criterion 2"]}'
```

**PHS ID = execution order**: The numeric suffix of PHS IDs determines execution sequence. PHS-001 is always first, PHS-002 always second. Do NOT assign IDs out of order (e.g., PHS-003 before PHS-001). If you reorder phases, renumber the IDs to match.

**Rules for exit_criteria**:
- Must be binary pass/fail
- **MANDATORY for every phase**: Must include "Project builds without errors OR dev server starts successfully". This is non-negotiable — a phase that leaves the project in a non-buildable/non-runnable state is invalid regardless of feature completeness.
- **MANDATORY for every phase**: Must include "All tests pass (no failures)". Tests may only be skipped if the user explicitly confirms the skip — record the skip reason in exit_criteria.
- No subjective language ("looks good", "works well")

**Phase buildability constraint**: When decomposing work into phases, ensure each phase's scope is self-contained enough that the project remains buildable/runnable after completion. If a feature requires multiple phases to become buildable, use techniques like:
- Feature flags or dead-code paths that compile but aren't reachable
- Interface stubs that satisfy type-checking
- Conditional compilation or build tags
- Never leave dangling imports, unresolved types, or missing implementations that break the build

### Step 5: exec Relations

```bash
# Assign phases to plan
spec-graph relation add --from PHS-001 --to PLN-001 --type belongs_to
spec-graph relation add --from PHS-002 --to PLN-001 --type belongs_to

# Set ordering
spec-graph relation add --from PHS-001 --to PHS-002 --type precedes
```

Parallel-eligible phases: if two phases have no dependency, do NOT add `precedes` between them.

### Step 6: mapping (covers)

Map every arch entity to at least one phase:

```bash
spec-graph relation add --from PHS-001 --to REQ-001 --type covers
spec-graph relation add --from PHS-001 --to REQ-002 --type covers
spec-graph relation add --from PHS-002 --to REQ-003 --type covers
```

**Coverage rule**: Every active arch entity (REQ, DEC, ACT, RSK, and any registered API/STT/TST)
must be covered by at least one phase. No orphaned arch entities.

### Step 7: Validate

Run full 3-layer validation:

```bash
spec-graph validate --layer arch
spec-graph validate --layer exec
spec-graph validate --layer mapping
```

If any check fails:
1. Parse the JSON error output
2. Identify the failing check and affected entities
3. Fix (add missing relations, correct ordering, resolve conflicts)
4. Re-validate

Do not proceed until all three layers pass.

### Step 8: Persistent Instructions

Check for persistent instructions files in project root:

```bash
ls AGENTS.md CLAUDE.md .cursorrules .github/copilot-instructions.md 2>/dev/null
```

**If found**: Check if spec-graph section already exists. If not, append:

```markdown
## spec-graph

This project uses spec-graph for requirements and phase management.

- `.spec-graph/` directory is the source of truth
- Before implementation: `spec-graph query scope <PHS-ID>` to check phase scope
- After implementation: `spec-graph validate` to verify
- Before changes: `spec-graph impact <ID>` for impact analysis
- Phase lifecycle: draft → active → resolved
```

**If not found**: Inform the user:

```
No persistent instructions file found (AGENTS.md, CLAUDE.md, etc.).
For other agents to recognize spec-graph in this project, creating one is recommended.
Create AGENTS.md with spec-graph instructions?
```

Act on user response.

---

## Error Handling

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Proceed |
| 1 | Runtime error | Check stderr, retry |
| 2 | Validation failure | Parse output, fix, re-validate |
| 3 | Invalid input | Check arguments/schema, fix, retry |

---

## Anti-Patterns

1. **Inventing requirements**: Never add entities not present in the input. Flag gaps instead.
2. **Skipping validation**: Never proceed past Step 7 without all checks passing.
3. **Over-registration**: Don't register API/STT/TST unless explicitly stated in input.
4. **Ignoring existing graph**: Always query before creating. Duplicates corrupt the graph.
5. **Vague exit_criteria**: "Works correctly" is not testable. Be specific.
