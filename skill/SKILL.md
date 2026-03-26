---
name: spec-graph
description: >
  Use this skill whenever the project uses spec-graph for managing requirements,
  decisions, phases, interfaces, states, tests, and other semantic entities in a
  typed graph. Trigger when the user mentions spec-graph, or when the task involves
  any of: tracking requirements or decisions, planning development phases, analyzing
  change impact, validating workflow gates, managing entity relationships in a
  specification graph, or coordinating agent work through structured impact analysis.
  Also trigger when you see a .spec-graph/ directory in the project, or when the user
  asks about impact analysis, gap detection, coverage checks, or phase exit criteria.
  This skill is essential for any phase-based development workflow that uses spec-graph
  as its semantic operator layer.
---

# spec-graph: Agent Operator Skill

spec-graph is a CLI tool that layers a typed semantic graph on top of phase-based development.
The structured graph — not markdown — is the source of truth. Agents receive computed impact sets
and patch-target lists instead of reasoning over free-text documents.

Four core capabilities:
- **Impact Analysis** — compute what must change together when an entity changes
- **Gap Detection** — find missing implementations, verifications, plans, or open questions
- **Consistency Validation** — check graph integrity and workflow gates
- **Agent Coordination** — work only on computed affected targets, not entire documents

## Three-Layer Architecture

v1 organizes the graph into three distinct layers. Each layer has its own entity types,
relation types, edge matrix, and validation checks.

### arch (architecture layer)
Contains the "what" and "why" of the system: requirements, decisions, interfaces, states,
tests, and supporting entities. This is where semantic meaning lives.

Entity types: `requirement`, `decision`, `interface`, `state`, `test`, `crosscut`,
`criterion`, `assumption`, `risk`, `question`

Relation types: `implements`, `verifies`, `depends_on`, `constrained_by`, `triggers`,
`answers`, `assumes`, `has_criterion`, `mitigates`, `supersedes`, `conflicts_with`, `references`

### exec (execution layer)
Contains the "when" and "how" of delivery: plans and phases. A plan groups phases into
a single active delivery sequence. Only one plan may be active at a time.

Entity types: `plan`, `phase`

Relation types: `belongs_to` (phase→plan), `precedes` (phase→phase), `blocks` (phase→phase)

### mapping (cross-layer)
Connects arch entities to exec entities. This is where intent meets delivery.

Relation types: `covers` (phase→arch entity), `delivers` (phase→arch entity)

`covers` replaced the removed `planned_in` — direction is inverted (phase→arch, not arch→phase).
`delivers` replaced the removed `delivered_in` — same inversion.

## Core Principles

1. **Compute first**: never modify by guesswork. Always run `impact` and `validate` to identify targets before making changes.
2. **JSON contract**: all CLI output goes to JSON stdout. Parse it to decide the next action.
3. **Layer discipline**: arch entities belong in arch, exec entities in exec. Do not mix concerns.
4. **Phase gates**: always run `validate --layer mapping --phase` before completing a phase.
5. **Changeset grouping**: bundle related changes into a single changeset.
6. **covers/delivers**: use these mapping relations. `planned_in` and `delivered_in` were removed in v1.

---

## Quick Reference: CLI Commands

See `references/cli-reference.md` for full options.

### Project Init
```bash
spec-graph init
spec-graph init --path /custom/path
```

### Entity CRUD
```bash
spec-graph entity add --type <TYPE> --id <ID> --title "..." [--description "..."] [--metadata '{}']
spec-graph entity get <ID>
spec-graph entity list --type <TYPE> [--status <STATUS>] [--layer arch|exec|mapping|all]
spec-graph entity update <ID> --title "..." --reason "..."
spec-graph entity deprecate <ID> --reason "..."
spec-graph entity delete <ID>
```

### Relation CRUD
```bash
spec-graph relation add --from <ID> --to <ID> --type <RELATION_TYPE>
spec-graph relation list --from <ID> [--layer arch|exec|mapping|all]
spec-graph relation delete --from <ID> --to <ID> --type <RELATION_TYPE>
```

### Impact Analysis
```bash
spec-graph impact <ID> [<ID>...]
spec-graph impact <ID> --follow implements,verifies,covers
spec-graph impact <ID> --min-severity medium
spec-graph impact <ID> --dimension structural|behavioral|planning
spec-graph impact <ID> --layer arch
```

### Validation
```bash
spec-graph validate
spec-graph validate --layer arch
spec-graph validate --layer exec
spec-graph validate --layer mapping
spec-graph validate --check orphans|coverage|cycles|conflicts|invalid_edges|superseded_refs|unresolved
spec-graph validate --check phase_order|single_active_plan|orphan_phases|exec_cycles|invalid_exec_edges
spec-graph validate --check plan_coverage|delivery_completeness|mapping_consistency|invalid_mapping_edges
spec-graph validate --phase <PHS-ID>
spec-graph validate --entity <ID>
```

### Query
```bash
spec-graph query neighbors <ID> --depth 2
spec-graph query path <FROM-ID> <TO-ID>
spec-graph query scope <PHS-ID>
spec-graph query unresolved --type question|assumption|risk
spec-graph query sql "SELECT ..."
```

### History
```bash
spec-graph history changeset <CHG-ID>
spec-graph history entity <ID>
spec-graph history relation <FROM>:<TO>:<TYPE>
```

### Export
```bash
spec-graph export --format json|dot|mermaid
spec-graph export --center <ID> --depth 3 --format json
spec-graph export --layer arch --format dot
```

### Bootstrap
```bash
spec-graph bootstrap scan --input ./docs/ [--format json]
spec-graph bootstrap import --input extracted.json --mode review
```

---

## Entity & Relation Quick Reference

See `references/data-model.md` for full type catalog, metadata schemas, and edge matrices.

### Entity Types (12)

| Prefix | Type | Layer | Purpose |
|--------|------|-------|---------|
| REQ | requirement | arch | functional / non-functional requirement |
| DEC | decision | arch | policy / architecture decision |
| API | interface | arch | API contract, module interface, event contract |
| STT | state | arch | state or state-transition rule |
| TST | test | arch | test case / scenario |
| XCT | crosscut | arch | cross-cutting concern (auth, audit, etc.) |
| QST | question | arch | unresolved question |
| ASM | assumption | arch | unverified assumption |
| ACT | criterion | arch | acceptance criterion |
| RSK | risk | arch | explicit risk item |
| PLN | plan | exec | delivery plan grouping phases |
| PHS | phase | exec | development phase or milestone |

### Entity Status: `draft` → `active` → `deprecated` / `resolved` / `deleted`

### Relation Types (17)

**Architecture layer (12):**
`implements`, `verifies`, `depends_on`, `constrained_by`, `triggers`, `answers`,
`assumes`, `has_criterion`, `mitigates`, `supersedes`, `conflicts_with`, `references`

**Execution layer (3):**
`belongs_to`, `precedes`, `blocks`

**Mapping layer (2):**
`covers`, `delivers`

`planned_in` and `delivered_in` were removed in v1. Use `covers` and `delivers` instead.

---

## Agent Workflow Patterns

This section is the heart of this skill. Agents follow these patterns.

### Pattern 1: Plan and Phase Setup

Create a plan, add phases to it, and wire up the mapping:

```bash
# 1. Create the plan (only one active plan allowed)
spec-graph entity add --type plan --id PLN-001 \
  --title "v1 Delivery Plan" \
  --metadata '{"status":"active"}'

# 2. Create phases
spec-graph entity add --type phase --id PHS-001 \
  --title "Phase 1 - Auth" \
  --metadata '{"goal":"Build authentication","order":1,"exit_criteria":["Auth API complete","E2E tests pass"]}'

# 3. Assign phases to plan
spec-graph relation add --from PHS-001 --to PLN-001 --type belongs_to

# 4. Set phase ordering
spec-graph relation add --from PHS-001 --to PHS-002 --type precedes

# 5. Map arch entities to phases using covers (not planned_in)
spec-graph relation add --from PHS-001 --to REQ-001 --type covers
spec-graph relation add --from PHS-001 --to REQ-002 --type covers

# 6. Gate check before starting
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
```

### Pattern 2: Change Handling

When an existing entity changes, always run impact first:

```bash
# 1. Compute impact — what else must change
spec-graph impact DEC-031 --dimension behavioral

# 2. Inspect affected targets (parse JSON)
spec-graph impact DEC-031 | jq '.affected[] | {id, type, impact, reason}'

# 3. Check unresolved items
spec-graph query unresolved --type question

# 4. Modify only affected targets (do not touch unrelated entities)
spec-graph entity update DEC-031 --title "New decision" --reason "Policy change"

# 5. Full validation
spec-graph validate
```

Never modify related entities by guesswork without running impact first.

### Pattern 3: Phase Exit

Before completing a phase, always run these checks:

```bash
# 1. Review phase scope — what arch entities are covered
spec-graph query scope PHS-002

# 2. Arch coverage check — missing implementations / tests
spec-graph validate --layer arch --check coverage

# 3. Mapping completeness — covered items that have no delivery
spec-graph validate --layer mapping --phase PHS-002 --check delivery_completeness

# 4. Mapping consistency — cross-layer integrity
spec-graph validate --layer mapping --phase PHS-002 --check mapping_consistency

# 5. Exec gate check — phase ordering, plan validity
spec-graph validate --layer exec --check phase_order
```

If validate reports issues, do not complete the phase. Resolve issues first.

#### Handling "covered but not delivered" mapping failures

When `delivery_completeness` reports a covered arch entity has no `delivers` relation:

```bash
# 1. Identify what the phase covers
spec-graph query scope PHS-002

# 2. Find implementing entities for the requirement
spec-graph relation list --to REQ-001   # find what implements/verifies REQ-001

# 3. Determine the MINIMAL proxy set — only entities whose delivery in this
#    phase is necessary and sufficient to consider REQ-001 delivered

# 4. Add delivers ONLY for that minimal set
spec-graph relation add --from PHS-002 --to API-005 --type delivers
spec-graph relation add --from PHS-002 --to TST-001 --type delivers

# 5. Re-validate
spec-graph validate --layer mapping --phase PHS-002 --check delivery_completeness
```

Critical rules for delivery proxy resolution:
- Compute the minimum set of implementing entities per requirement. Do not add all related entities.
- If the check still fails after adding the minimal proxy set, investigate the validator semantics
  or the graph model before expanding further.
- Apply the same precision level consistently across all phases.
- After adding proxy relations, verify semantic correctness: does each `delivers` accurately
  represent work completed in this phase, or is it just silencing the check?

### Pattern 4: Full Patch Orchestration (recommended)

The safest change-handling flow:

```
1. Identify change target
2. spec-graph impact → compute affected set
3. spec-graph validate → check currently broken rules
4. Modify only affected targets (entity update, relation add/delete, etc.)
5. Semantic review → does each added relation accurately represent the intended meaning?
6. spec-graph validate → re-verify after modifications
7. spec-graph history → review changeset records
```

The agent modifies only entities in the `affected` list from step 2.
If an entity outside the list needs modification, first run `query neighbors` to verify the relationship.

Step 5 (semantic review) is critical. Before re-validating, review every relation you added
and ask: "Does this relation reflect a real semantic relationship, or am I adding it to pass a check?"
Check passage alone does not prove graph correctness.

### Pattern 5: Adding a Requirement

Typical flow for adding a new requirement and wiring it into the graph:

```bash
# 1. Create requirement
spec-graph entity add --type requirement --id REQ-015 \
  --title "All payments must be idempotent" \
  --metadata '{"priority":"must","kind":"non_functional","owner":"payment-team"}'

# 2. Attach acceptance criterion
spec-graph entity add --type criterion --id ACT-020 \
  --title "Duplicate request within window processed only once" \
  --metadata '{"given":"Payment request already sent","when":"Same request resent","then":"No duplicate processing; return existing result"}'
spec-graph relation add --from REQ-015 --to ACT-020 --type has_criterion

# 3. Map to phase using covers (not planned_in)
spec-graph relation add --from PHS-003 --to REQ-015 --type covers

# 4. Link crosscut constraint (if applicable)
spec-graph relation add --from REQ-015 --to XCT-002 --type constrained_by

# 5. Validate arch layer
spec-graph validate --layer arch --entity REQ-015
```

### Pattern 6: Bootstrap (graph from existing docs)

When existing markdown documents are available:

```bash
# 1. Extract candidates — generates review candidates, not auto-committed
spec-graph bootstrap scan --input ./docs/ --format json

# 2. Review — filter low-confidence items
cat extracted.json | jq '.entities[] | select(.confidence >= 0.7)'

# 3. Import in review mode
spec-graph bootstrap import --input extracted.json --mode review
```

Low-confidence relations are never auto-imported. A human must confirm, or the agent must
cross-reference against the source document before deciding.

---

## Validation Checks Guide

When to use each check. See `references/validation-rules.md` for detailed rules.

### Architecture Layer (`--layer arch`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `orphans` | isolated arch entities with no relations | periodic cleanup, before phase start |
| `coverage` | missing implementations / tests | required before phase exit |
| `cycles` | circular references in depends_on chains | after adding relations |
| `conflicts` | semantic conflicts between entities | after changes |
| `invalid_edges` | arch edge matrix violations | after adding relations |
| `superseded_refs` | active refs to deprecated entities | after deprecation |
| `unresolved` | open questions, unverified assumptions, unmitigated risks | before phase start |

### Execution Layer (`--layer exec`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `phase_order` | valid phase sequence | after adding exec relations |
| `single_active_plan` | only one active plan | after plan creation or status change |
| `orphan_phases` | phases not in any plan | after adding phases |
| `exec_cycles` | circular precedes/blocks chains | after adding exec relations |
| `invalid_exec_edges` | exec edge matrix violations | after adding exec relations |

### Mapping Layer (`--layer mapping`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `plan_coverage` | all active requirements covered | before phase start |
| `delivery_completeness` | covered entities have delivery evidence | required before phase exit |
| `mapping_consistency` | covers/delivers targets are valid arch entities | after adding mapping relations |
| `invalid_mapping_edges` | mapping edge matrix violations | after adding mapping relations |

### Common Combinations

```bash
# Before phase start
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
spec-graph validate --layer arch --check unresolved
spec-graph validate --layer mapping --check plan_coverage

# Before phase completion (required)
spec-graph validate --layer arch --check coverage
spec-graph validate --layer mapping --phase PHS-003 --check delivery_completeness
spec-graph validate --layer mapping --phase PHS-003 --check mapping_consistency

# After any change
spec-graph validate
```

---

## Interpreting Impact Results

Key fields in `impact` JSON output:

```json
{
  "affected": [
    {
      "id": "API-005",
      "type": "interface",
      "layer": "arch",
      "depth": 1,
      "impact": {
        "overall": "high",
        "structural": "high",
        "behavioral": "medium",
        "planning": "low"
      },
      "reason": "direct implementation"
    }
  ],
  "summary": {
    "total": 5,
    "by_type": {"interface": 2, "test": 3},
    "by_impact": {"high": 1, "medium": 2, "low": 2}
  }
}
```

**Agent behavior rules**:
- `overall: high` → must review and modify if needed
- `overall: medium` → inspect content, modify if actually affected
- `overall: low` → scan list only, modification rarely needed

**Dimension filtering**: use `--dimension` to focus on specific concerns
- interface change → `--dimension structural`
- policy/behavior change → `--dimension behavioral`
- schedule/scope change → `--dimension planning`

---

## Exit Codes

| Code | Meaning | Agent Action |
|------|---------|--------------|
| 0 | success | proceed to next step |
| 1 | runtime error | check error message, retry or report |
| 2 | validation failure | resolve issues from validate output |
| 3 | invalid input | check arguments / schema, retry |

---

## Caveats

- `bootstrap import` defaults to `--mode review`. Never use `--mode auto`.
- `supersedes` requires both entities to be the same type.
- `conflicts_with` does not allow self-loops.
- Adding a relation that violates the allowed edge matrix fails with exit code 3.
  On failure, consult the edge matrix in `references/data-model.md`.
- `metadata` is a JSON string. Each type has required fields — see `references/data-model.md`.
- `--phase` is only valid with `--layer mapping` or `--layer all`. Using `--phase` with
  `--layer arch` or `--layer exec` returns an error (exit 3).
- Only one plan may have `active` status at a time. The `single_active_plan` exec check
  enforces this.

## Anti-Patterns

These are known failure modes. If you catch yourself doing any of these, stop and reconsider.

### 1. Mixing arch and exec concerns
**Symptom**: attempting to add a requirement directly to a phase using `planned_in` (removed in v1),
or treating a phase as an arch entity by linking it with arch-only relations.
**Why it's wrong**: arch and exec are separate layers with separate edge matrices. Cross-layer
connections belong in the mapping layer using `covers` and `delivers`.
**Correct approach**: use `covers` (phase→arch) to express intent, `delivers` (phase→arch)
to express completion.

### 2. Using planned_in / delivered_in
**Symptom**: attempting to add `planned_in` or `delivered_in` relations.
**Why it's wrong**: these relations were removed in v1. They no longer exist in the schema.
**Correct approach**: use `covers` instead of `planned_in`, `delivers` instead of `delivered_in`.
Note the direction is inverted: `phase --covers--> arch_entity` (not `arch_entity --planned_in--> phase`).

### 3. Check-driven patching
**Symptom**: check fails → add relations broadly until check passes → commit.
**Why it's wrong**: passing a check does not mean the graph is correct. Over-broad relations
pollute the graph and produce inaccurate impact analysis downstream.
**Correct approach**: diagnose why the check fails, compute the minimal fix, verify semantic
accuracy, then re-validate.

### 4. Bulk delivers expansion
**Symptom**: a requirement is "covered but not delivered" → add `delivers` for every
related interface, state, and test to the phase.
**Why it's wrong**: not all implementing entities belong to every phase. Each `delivers`
must represent actual delivery in that specific phase.
**Correct approach**: identify the minimal proxy set per requirement. Only entities whose
delivery in this phase is necessary and sufficient to consider the requirement fulfilled.

### 5. Semantic ambiguity bypass
**Symptom**: discover a model-level conflict (e.g. edge matrix prevents a relation type
the check seems to require) → work around it by expanding other relations instead of
investigating the conflict.
**Why it's wrong**: the conflict is a signal that either (a) the graph model needs revision,
(b) the validator semantics need clarification, or (c) the agent's understanding is incomplete.
**Correct approach**: when you encounter a semantic conflict between edge matrix constraints
and validator expectations, stop and investigate. Check `references/data-model.md` for the
intended semantics. If the conflict is genuine, report it to the user rather than working around it.

### 6. Inconsistent precision across phases
**Symptom**: Phase N uses broad relation additions, Phase N+1 uses precise minimal additions.
**Why it's wrong**: the same rules must apply uniformly. If Phase 3 adds only 3 delivery
proxies, Phase 2 should not have added 15 for a similar scope.
**Correct approach**: establish the precision standard on the first phase, then apply it
consistently to all subsequent phases.
