# Validation Rules Reference

## Table of Contents
1. [Overview](#overview)
2. [Architecture Layer Checks](#architecture-layer-checks)
3. [Execution Layer Checks](#execution-layer-checks)
4. [Mapping Layer Checks](#mapping-layer-checks)
5. [Phase Validation Guide](#phase-validation-guide)
6. [Interpreting Validate Response](#interpreting-validate-response)

---

## Overview

Validation is organized into three layers matching the graph model. Each check belongs to
exactly one layer. The `--layer` flag restricts which checks run.

```bash
spec-graph validate                  # runs all default checks (all layers)
spec-graph validate --layer arch     # runs only arch checks
spec-graph validate --layer exec     # runs only exec checks
spec-graph validate --layer mapping  # runs only mapping checks
```

Each issue carries `check`, `severity`, `entity`, and `message`. There is no `layer` field in the
CLI output — infer the layer from the check name.

### Severity Levels

| Severity | Agent Behavior |
|----------|---------------|
| `high` | must resolve before proceeding. Blocks phase start and completion. |
| `medium` | resolution recommended. May proceed with explicit justification. |
| `low` | informational. Review and decide. |

---

## Architecture Layer Checks

Run with `--layer arch`. These verify the semantic integrity of arch entities and relations.

### orphans
Detects arch entities with zero relations.

```bash
spec-graph validate --layer arch --check orphans
```

| Condition | Severity |
|-----------|----------|
| Active or draft arch entity has no arch relations | medium |

Newly added entities without any relations are flagged. Draft-status orphans may be acceptable
during early modeling, but active-status orphans indicate a wiring problem.

### coverage
Detects missing implementations and verifications for arch entities.

```bash
spec-graph validate --layer arch --check coverage
```

| Condition | Severity |
|-----------|----------|
| Active requirement has no `implements` relation | high |
| Active requirement has no `has_criterion` | high |
| Active criterion has no `verifies` relation | high |
| Interface that triggers a state has no test linked via `verifies` | high |

- `high` severity blocks phase exit. Must be resolved.
- `medium` severity is recommended to resolve but may proceed with justification.

### cycles
Detects disallowed circular references in arch relations.

```bash
spec-graph validate --layer arch --check cycles
```

| Condition | Severity |
|-----------|----------|
| Circular chain in `depends_on` | high |

`conflicts_with` is bidirectional by nature and is not treated as a cycle.

### conflicts
Detects entities with active semantic conflicts.

```bash
spec-graph validate --layer arch --check conflicts
```

| Condition | Severity |
|-----------|----------|
| Two active entities in the same phase scope have a `conflicts_with` relation | high |

### invalid_edges
Detects relations that violate the arch edge matrix.

```bash
spec-graph validate --layer arch --check invalid_edges
```

| Condition | Severity |
|-----------|----------|
| Arch relation violates the arch edge matrix | high |

Normally not triggered when using `relation add` (which validates at insertion), but useful
after direct DB edits or bootstrap imports.

### superseded_refs
Detects active entities still referencing deprecated or superseded entities.

```bash
spec-graph validate --layer arch --check superseded_refs
```

| Condition | Severity |
|-----------|----------|
| Entity still references a superseded entity | high |

Always run this after revising or deprecating an entity. It detects references to entities that
have been **superseded** (i.e. have an inbound `supersedes` edge), not deprecated entities in
general. When flagged, update references to point to the replacement entity or remove the relation.

### unresolved
Detects open questions, unverified assumptions, and unmitigated risks.

```bash
spec-graph validate --layer arch --check unresolved
```

| Condition | Severity |
|-----------|----------|
| Active or draft question with no `answers` relation | medium |
| Active or draft assumption needing validation | medium |
| Active or draft risk with no `mitigates` relation | medium |

Run this before starting a phase to confirm all blocking items are resolved.

---

## Execution Layer Checks

Run with `--layer exec`. These verify the structural integrity of the plan and phase graph.

### phase_order
Detects duplicate phase `order` values.

```bash
spec-graph validate --layer exec --check phase_order
```

| Condition | Severity |
|-----------|----------|
| Two or more phases share the same numeric `order` | high |

Note: this check does **not** compare `precedes`/`blocks` against `order`. Sequencing itself is
enforced at activation time, where a phase with unresolved predecessors is blocked.

### single_active_plan
Enforces the constraint that only one plan may be active at a time.

```bash
spec-graph validate --layer exec --check single_active_plan
```

| Condition | Severity |
|-----------|----------|
| More than one plan has `status: active` | high |

This is a hard constraint in v1. Before activating a new plan, archive or deprecate the
existing active plan.

### orphan_phases
Detects phases not connected to any plan.

```bash
spec-graph validate --layer exec --check orphan_phases
```

| Condition | Severity |
|-----------|----------|
| Phase has no `belongs_to` relation pointing to a plan | medium |

Every phase should belong to exactly one plan; the check does not distinguish status.

### exec_cycles
Detects circular chains in exec relations.

```bash
spec-graph validate --layer exec --check exec_cycles
```

| Condition | Severity |
|-----------|----------|
| Circular chain in `blocks` | high |

Only `blocks` chains are examined.

### orphan_changes
Detects change entities with no relations at all.

```bash
spec-graph validate --layer exec --check orphan_changes
```

| Condition | Severity |
|-----------|----------|
| Active, resolved, or deprecated change with no relations | high |
| Change in any other status with no relations | medium |

A change exists to cover arch entities; one with no relations records nothing.

### invalid_exec_edges
Detects relations that violate the exec edge matrix.

```bash
spec-graph validate --layer exec --check invalid_exec_edges
```

| Condition | Severity |
|-----------|----------|
| Exec relation violates the exec edge matrix | high |

### task_graph
Checks each non-deprecated task has exactly one phase parent, each `task_depends_on` edge stays
within that phase and is stored dependent→prerequisite, and the dependency graph is acyclic.

```bash
spec-graph validate --layer exec --check task_graph
```

| Condition | Severity |
|-----------|----------|
| Task has zero or multiple parents, or a non-phase parent | high |
| `task_depends_on` points at itself | high |
| Dependency crosses phases | high |
| Dependency targets a deprecated task | high |
| Dependency cycle | high |

---

## Mapping Layer Checks

Run with `--layer mapping`. These verify the cross-layer connections between arch and exec.
`--phase` is valid with mapping checks to scope results to a specific phase.

### plan_coverage
Detects active arch requirements that are not covered by any phase.

```bash
spec-graph validate --layer mapping --check plan_coverage
```

| Condition | Severity |
|-----------|----------|
| Active requirement has no `covers` relation from any phase | high |

Run this before starting a phase to confirm all requirements are assigned somewhere.

### delivery_completeness
Detects arch entities that are covered by a phase but have no delivery evidence.

```bash
spec-graph validate --layer mapping --check delivery_completeness
spec-graph validate --layer mapping --phase "$PHS_ID" --check delivery_completeness
```

| Condition | Severity |
|-----------|----------|
| Phase covers an arch entity but no `delivers` relation exists for it | high |

This is the primary gate check before phase completion. Every covered arch entity must itself be
delivered within the phase's effective scope — covered IDs are compared against delivered IDs
directly. There is **no** proxy resolution: delivering an entity that `implements` or `verifies` the
covered one does not satisfy it. Entity types that cannot legally receive `delivers` from a phase
are skipped via the edge matrix.

### mapping_consistency
Detects mapping relations whose target arch entity is no longer valid.

```bash
spec-graph validate --layer mapping --check mapping_consistency
spec-graph validate --layer mapping --phase "$PHS_ID" --check mapping_consistency
```

| Condition | Severity |
|-----------|----------|
| `covers` or `delivers` target is deprecated | medium |
| `covers` or `delivers` target has been superseded by another entity | medium |

Mapping relations whose **source phase or task is `resolved`** are exempt. Completed execution must
keep pointing at the revision it actually delivered, so a deprecated or superseded target there is
a historical record, not an inconsistency. Without this exemption every `entity revise` of a
delivered entity would leave a permanent finding.

Edge-shape violations (e.g. `covers` source not a phase, target not an arch entity) are
caught by `invalid_mapping_edges`, not this check.

### invalid_mapping_edges
Detects relations that violate the mapping edge matrix.

```bash
spec-graph validate --layer mapping --check invalid_mapping_edges
```

| Condition | Severity |
|-----------|----------|
| Mapping relation violates the mapping edge matrix | high |

### task_scope
Checks each non-deprecated task covers at least one arch entity, task `delivers` is a subset of its
`covers`, and phases never mix direct mappings with child-task mappings.

```bash
spec-graph validate --layer mapping --check task_scope
spec-graph validate --layer mapping --phase "$PHS_ID" --check task_scope
```

| Condition | Severity |
|-----------|----------|
| Task covers nothing, or `delivers` is not a subset of `covers` | high |
| Phase mixes direct mappings with child-task mappings | high |

Task-managed phase scope is the union of child task mappings. Taskless phase scope remains direct
and unchanged. Tasks are exec entities and do not enter the architecture closure.

### Task Completion Gates
Task-managed completion rests on `task_graph`, `task_scope`, the task delivery/QA-evidence gate, and
phase child-resolution alongside the existing `delivery_completeness`/`gates` checks.

Task activation requires exactly one parent, that the parent is a phase, that the parent is active,
and that all prerequisites are resolved. Resolution requires resolved prerequisites, QA evidence
pointing at real repository files, and `delivers` for every deliverable target the task covers. A
phase cannot resolve until every non-deprecated, non-deleted child task is resolved.

### gates
Detects phase readiness blockers by checking arch entities in the phase scope for
unresolved questions, unmitigated risks, unverified assumptions, and dependencies on
draft decisions.

```bash
spec-graph validate --layer mapping --check gates
spec-graph validate --layer mapping --phase "$PHS_ID" --check gates
```

| Condition | Severity |
|-----------|----------|
| Active or draft question in phase scope with no `answers` relation | high |
| Active or draft risk in phase scope with no `mitigates` relation | high |
| Active or draft assumption in phase scope (needs validation) | medium |
| Requirement in phase scope depends on a draft decision | high |

When `--phase` is specified, only that phase is checked. Without `--phase`, all active
phases are checked. Run this before starting or completing a phase.

### phase_satisfaction
Evaluates whether the phase's covered architecture closure is satisfied by delivered
execution evidence. This is the unified phase exit gate — it answers a single question:
"Is each member of this phase's covered closure backed by appropriate evidence?"

**Opt-in only.** It is deliberately excluded from default validation runs, so a bare
`spec-graph validate` or `--layer mapping` will not execute it. Invoke it explicitly with
`--check phase_satisfaction`, normally together with `--phase`.

```bash
spec-graph validate --layer mapping --check phase_satisfaction --phase "$PHS_ID"
spec-graph validate --layer mapping --check phase_satisfaction --phase "$PHS_ID" --include-references
```

#### Closure Definition

For phase P, the closure is computed as:

| Class | Members |
|-------|---------|
| Mandatory | entities directly covered by P (`P --covers--> X`), plus 1-depth `depends_on` outbound neighbors of covered entities (`X --depends_on--> Y`), plus 1-depth `implements` inbound neighbors of covered entities (`Z --implements--> X`) |
| Advisory (opt-in only) | 1-depth `references` outbound neighbors of directly covered entities (`X --references--> R`), when `--include-references` is passed |

If an entity would qualify for both classes, mandatory wins.

#### Three-Layer Satisfaction Judgment

Each mandatory member is evaluated by applying the first matching rule:

| Layer | Rule | Applies To |
|-------|------|-----------|
| 1 | inbound evidence relation must exist | requirement (`delivers`), question (`answers`), risk (`mitigates`) |
| 2 | entity's own status must be in the allowlist | assumption (`verified`), decision (`active`, `resolved`) |
| 3 | when Layer 1 applies, the evidence source's status must be in the per-type allowlist | all evidence-bearing types |

Per-type Layer 3 allowlists (applied to the **evidence source's** status):

| Evidence Source Type | Allowed Status |
|---------------------|----------------|
| decision | active, resolved |
| interface | active, resolved |
| test | resolved |
| requirement | resolved |
| risk | resolved |
| phase | active, resolved |
| (fallback for other types) | active, resolved |

Layer 2 status-only rules (applied to the closure member's own status):

| Member Entity Type | Allowed Status |
|---|---|
| assumption | resolved |
| decision | active, resolved |

Advisory members are always reported as `advisory`. They never produce a `phase_satisfaction`
issue and do not count toward the satisfied/total ratio.

#### Validation Outcomes

| Condition | Severity |
|-----------|----------|
| Mandatory closure member fails Layer 1, 2, or 3 | high |
| Advisory closure member exists | (no issue; reported only) |

Each unsatisfied mandatory member produces a separate issue. The validate response also
includes a per-phase `satisfaction` report with the satisfied/total ratio, advisory count,
and a per-entity item list with the reason for each outcome.

#### Trigger

`phase_satisfaction` is **not** included in the default mapping checks. It is a phase-exit
gate that must be invoked explicitly with `--check phase_satisfaction` (typically with
`--phase`). This prevents in-progress phases from failing routine `validate --layer mapping`
runs.

#### Evidence Evaluation Semantics

For Layer 1 + Layer 3 evaluation, the check uses **existential** semantics: a member is
satisfied if **any** inbound evidence relation comes from a source whose status is in the
allowlist. If the first evidence source has a non-allowed status but a later source has an
allowed status, the member is satisfied.

#### Same-Phase Delivers Requirement

For the `delivers` evidence relation specifically, the source must be the **phase being validated
or one of its child tasks**. Delivery by an unrelated phase is reported diagnostically but does not
satisfy the current phase. This enforces phase-exit gate semantics: `phase A covers REQ-X` is not
satisfied by `phase B delivers REQ-X` — phase A, or a task belonging to it, must deliver `REQ-X`.

When only cross-phase deliveries exist, the unsatisfied reason names the other delivering
phases for diagnostic clarity, e.g. `no inbound "delivers" relation from phase
PHS-1752239600-5xq (found from [PHS-1752240100-w0q])`.

The other evidence relations (`answers`, `mitigates`) are not phase-scoped because their
valid sources per the edge matrix are arch entities (decisions, tests, crosscut), not
phases. Any source meeting Layer 3 satisfies regardless of phase membership.

---

## Phase Validation Guide

### Before Phase Start

```bash
# Exec: confirm plan is valid and phase ordering is correct
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
spec-graph validate --layer exec --check orphan_phases
spec-graph validate --layer exec --check task_graph

# Arch: confirm no blocking open items
spec-graph validate --layer arch --check unresolved

# Mapping: confirm all requirements are assigned
spec-graph validate --layer mapping --check plan_coverage
spec-graph validate --layer mapping --check task_scope
spec-graph phase context "$PHS_ID"
```

Purpose: confirm that all prerequisites for items assigned to this phase are met.

### During Phase (on change)

```bash
spec-graph validate
```

Purpose: ensure no rules are broken after mid-phase changes. Running all layers catches
cross-layer regressions.

### Before Phase Completion (required)

```bash
# Arch: confirm implementations and tests exist
spec-graph validate --layer arch --check coverage

# Arch: clean up stale references
spec-graph validate --layer arch --check superseded_refs

# Mapping: unified satisfaction gate — closure satisfied by evidence
spec-graph validate --layer mapping --phase "$PHS_ID" --check phase_satisfaction

# Mapping: confirm all covered items have delivery evidence
spec-graph validate --layer mapping --phase "$PHS_ID" --check delivery_completeness

# Mapping: confirm cross-layer integrity
spec-graph validate --layer mapping --phase "$PHS_ID" --check mapping_consistency
```

Purpose: verify implementation/test completeness, open-item resolution, and delivery evidence.

`phase_satisfaction` is the recommended single-shot gate. It computes the closure of the
phase (covered entities plus their 1-depth `depends_on` / `implements` neighbors) and
applies a three-layer judgment (evidence relation, status-only, target status allowlist)
to each member. Use `--include-references` when you want the report to also surface
`references`-linked items as advisory entries; advisory items never block satisfaction.

---

## Interpreting Validate Response

```json
{
  "valid": false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high",
      "entity": "REQ-1752239482-k3f",
      "message": "No implementation found"
    },
    {
      "check": "delivery_completeness",
      "severity": "high",
      "entity": "PHS-1752239600-5xq",
      "message": "Phase covers REQ-1752239482-k3f but no delivers relation exists"
    },
    {
      "check": "single_active_plan",
      "severity": "high",
      "entity": "PLN-1752240300-b2x",
      "message": "Multiple active plans found: PLN-1752239400-w0q, PLN-1752240300-b2x"
    }
  ],
  "summary": {
    "total_issues": 3,
    "by_severity": {"high": 3, "medium": 0, "low": 0}
  }
}
```

### Agent Decision Criteria
- `valid: true` → safe to proceed to the next step.
- `valid: false` + high severity → must resolve before proceeding.
- `valid: false` + medium/low only → report to the user and let them decide.

### Resolving Common Issues

| Issue | Resolution |
|-------|-----------|
| `coverage`: no implementation | add `implements` relation from an interface to the requirement |
| `coverage`: no criterion | add `has_criterion` relation and create an ACT entity |
| `unresolved`: open question | create a decision with `answers` relation, or set question to `resolved` |
| `unresolved`: unmitigated risk | add `mitigates` relation, or set risk to `resolved` |
| `single_active_plan`: multiple active | set all but one plan to `deprecated` or `resolved` |
| `orphan_phases`: phase not in plan | add `belongs_to` relation from phase to the active plan |
| `delivery_completeness`: no delivers | add `delivers` for that exact covered entity (from its child task in a task-managed phase), or narrow the `covers` scope |
| `mapping_consistency`: target deprecated or superseded | retarget the relation to the active replacement entity, or remove the stale relation |
| `phase_satisfaction`: no inbound evidence relation | add the required relation (`delivers` for requirements, `answers` for questions, `mitigates` for risks); for assumptions/decisions, advance the entity status |
| `phase_satisfaction`: evidence source status not in allowlist | progress the evidence source (e.g. activate the interface, mark the test verified, complete the phase) |
| `superseded_refs`: stale reference | update relation to point to the replacement entity, or remove it |
