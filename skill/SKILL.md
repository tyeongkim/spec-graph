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

## Core Principles

1. **Compute first**: never modify by guesswork. Always run `impact` and `validate` to identify targets before making changes.
2. **JSON contract**: all CLI output goes to JSON stdout. Parse it to decide the next action.
3. **Phase gates**: always run `validate --phase` before starting or completing a phase.
4. **Changeset grouping**: bundle related changes into a single changeset.

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
spec-graph entity list --type <TYPE> [--status <STATUS>]
spec-graph entity update <ID> --title "..." --reason "..."
spec-graph entity deprecate <ID> --reason "..."
spec-graph entity delete <ID>
```

### Relation CRUD
```bash
spec-graph relation add --from <ID> --to <ID> --type <RELATION_TYPE>
spec-graph relation list --from <ID>
spec-graph relation delete --from <ID> --to <ID> --type <RELATION_TYPE>
```

### Impact Analysis
```bash
spec-graph impact <ID> [<ID>...]
spec-graph impact <ID> --follow implements,verifies,planned_in
spec-graph impact <ID> --min-severity medium
spec-graph impact <ID> --dimension structural|behavioral|planning
```

### Validation
```bash
spec-graph validate
spec-graph validate --check orphans|coverage|cycles|conflicts|gates|invalid_edges|superseded_refs
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
```

### Bootstrap
```bash
spec-graph bootstrap scan --input ./docs/ [--format json]
spec-graph bootstrap import --input extracted.json --mode review
```

---

## Entity & Relation Quick Reference

See `references/data-model.md` for full type catalog, metadata schemas, and allowed edge matrix.

### Entity Types (11)

| Prefix | Type | Purpose |
|--------|------|---------|
| REQ | requirement | functional / non-functional requirement |
| DEC | decision | policy / architecture decision |
| PHS | phase | development phase or milestone |
| API | interface | API contract, module interface, event contract |
| STT | state | state or state-transition rule |
| TST | test | test case / scenario |
| XCT | crosscut | cross-cutting concern (auth, audit, etc.) |
| QST | question | unresolved question |
| ASM | assumption | unverified assumption |
| ACT | criterion | acceptance criterion |
| RSK | risk | explicit risk item |

### Entity Status: `draft` → `active` → `deprecated` / `resolved` / `deleted`

### Relation Types (14)

`implements`, `verifies`, `depends_on`, `constrained_by`, `planned_in`, `delivered_in`,
`triggers`, `answers`, `assumes`, `has_criterion`, `mitigates`, `supersedes`,
`conflicts_with`, `references`

---

## Agent Workflow Patterns

This section is the heart of this skill. Agents follow these patterns.

### Pattern 1: Phase Planning

Create a new phase and assign requirements:

```bash
# 1. Create phase
spec-graph entity add --type phase --id PHS-003 \
  --title "Phase 3 - Payment" \
  --metadata '{"goal":"Build payment system","order":3,"exit_criteria":["Payment API complete","E2E tests pass"]}'

# 2. Assign requirements to phase
spec-graph relation add --from REQ-010 --to PHS-003 --type planned_in
spec-graph relation add --from REQ-011 --to PHS-003 --type planned_in

# 3. Gate check — verify prerequisites before starting
spec-graph validate --phase PHS-003 --check gates
```

If gate check reports issues (unresolved questions, unmitigated risks, etc.), resolve them first.

### Pattern 2: Change Handling

When an existing entity changes, **always run impact first**:

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

**Never do this**: modify related entities by guesswork without running impact first.

### Pattern 3: Phase Exit

Before completing a phase, always run these:

```bash
# 1. Review phase scope — what is included
spec-graph query scope PHS-002

# 2. Coverage check — find missing implementations / tests
spec-graph validate --phase PHS-002 --check coverage

# 3. Gate check — unresolved questions, unmitigated risks, etc.
spec-graph validate --phase PHS-002 --check gates
```

If validate reports issues, do not complete the phase. Resolve issues first.

#### Handling "planned but not delivered" gate failures

Some entity types (e.g. `requirement`) cannot hold `delivered_in` directly per the edge matrix.
When the gate reports a requirement as "planned but not delivered," this is a **model-level signal**,
not a cue to bulk-add relations. Follow this procedure:

```bash
# 1. Identify the requirement the gate is complaining about
spec-graph query neighbors REQ-001 --depth 1

# 2. Find its implementing entities (interface, test, state)
spec-graph relation list --from REQ-001   # or --to REQ-001 for verifies/implements

# 3. Determine the MINIMAL proxy set — only the entities that directly
#    satisfy this requirement's delivery in this phase
#    Ask: "Which implementing entities are necessary and sufficient
#    to consider REQ-001 delivered in PHS-002?"

# 4. Add delivered_in ONLY for that minimal set
spec-graph relation add --from API-005 --to PHS-002 --type delivered_in
spec-graph relation add --from TST-001 --to PHS-002 --type delivered_in

# 5. Re-validate
spec-graph validate --phase PHS-002 --check gates
```

**Critical rules for delivery proxy resolution:**
- Compute the minimum set of implementing entities per requirement. Do not add all related entities.
- If the gate still fails after adding the minimal proxy set, investigate the validator semantics
  or the graph model before expanding further. Do not blindly widen the delivered set.
- Apply the same precision level consistently across all phases. If Phase 3 uses minimal proxies,
  Phase 2 must use the same standard.
- After adding proxy relations, verify semantic correctness: does each `delivered_in` accurately
  represent work completed in this phase, or is it just silencing the gate?

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

**Step 5 (semantic review) is critical.** Before re-validating, review every relation you added
and ask: "Does this relation reflect a real semantic relationship, or am I adding it to pass a gate?"
Gate passage alone does not prove graph correctness. A graph that passes all gates but contains
over-broad relations is worse than one that fails a gate with an honest gap.

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

# 3. Assign to phase
spec-graph relation add --from REQ-015 --to PHS-003 --type planned_in

# 4. Link crosscut constraint (if applicable)
spec-graph relation add --from REQ-015 --to XCT-002 --type constrained_by

# 5. Validate
spec-graph validate --entity REQ-015
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

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `orphans` | isolated entities with no relations | periodic cleanup, before phase start |
| `coverage` | missing implementations / tests | required before phase exit |
| `cycles` | disallowed circular references | after adding relations |
| `conflicts` | semantic conflicts between entities | after changes |
| `gates` | phase entry/exit prerequisites | required at phase start and completion |
| `invalid_edges` | edge matrix violations | after adding relations |
| `superseded_refs` | active refs to deprecated entities | after deprecation |

### Common Combinations

```bash
# Before phase start
spec-graph validate --phase PHS-003 --check gates
spec-graph validate --phase PHS-003 --check orphans

# Before phase completion (required)
spec-graph validate --phase PHS-003 --check coverage
spec-graph validate --phase PHS-003 --check gates

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

## Anti-Patterns

These are known failure modes. If you catch yourself doing any of these, stop and reconsider.

### 1. Gate-driven patching
**Symptom**: gate fails → add relations broadly until gate passes → commit.
**Why it's wrong**: passing a gate does not mean the graph is correct. Over-broad relations
pollute the graph and produce inaccurate impact analysis downstream.
**Correct approach**: diagnose *why* the gate fails, compute the minimal fix, verify semantic
accuracy, then re-validate.

### 2. Bulk delivered_in expansion
**Symptom**: a requirement is "planned but not delivered" → add `delivered_in` for every
related interface, state, and test to the phase.
**Why it's wrong**: not all implementing entities belong to every phase. Each `delivered_in`
must represent actual delivery in that specific phase.
**Correct approach**: identify the minimal proxy set per requirement. Only entities whose
delivery in this phase is necessary and sufficient to consider the requirement fulfilled.

### 3. Semantic ambiguity bypass
**Symptom**: discover a model-level conflict (e.g. edge matrix prevents a relation type
the gate seems to require) → work around it by expanding other relations instead of
investigating the conflict.
**Why it's wrong**: the conflict is a signal that either (a) the graph model needs revision,
(b) the validator semantics need clarification, or (c) the agent's understanding is incomplete.
Expanding relations without resolving the ambiguity compounds the error.
**Correct approach**: when you encounter a semantic conflict between edge matrix constraints
and validator expectations, stop and investigate. Check `references/data-model.md` for the
intended semantics. If the conflict is genuine, report it to the user rather than working around it.

### 4. Inconsistent precision across phases
**Symptom**: Phase N uses broad relation additions, Phase N+1 uses precise minimal additions.
**Why it's wrong**: the same rules must apply uniformly. If Phase 3 adds only 3 delivery
proxies, Phase 2 should not have added 15 for a similar scope.
**Correct approach**: establish the precision standard on the first phase, then apply it
consistently to all subsequent phases.
