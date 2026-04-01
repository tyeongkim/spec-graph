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
spec-graph validate                  # runs all checks (all layers)
spec-graph validate --layer arch     # runs only arch checks
spec-graph validate --layer exec     # runs only exec checks
spec-graph validate --layer mapping  # runs only mapping checks
```

Each issue in the response includes a `layer` field so you know which layer it came from.

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
| Active arch entity has no relations | medium |
| Draft arch entity has no relations | low |

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
| Active requirement has no `has_criterion` | medium |
| Active criterion has no `verifies` relation | high |
| Interface that triggers a state has no related test | medium |

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
| Active entity references a deprecated entity | medium |

Always run this after deprecating an entity. When flagged, update references to point to
the replacement entity or remove the relation.

### unresolved
Detects open questions, unverified assumptions, and unmitigated risks.

```bash
spec-graph validate --layer arch --check unresolved
```

| Condition | Severity |
|-----------|----------|
| Active question with no `answers` relation | high |
| Active assumption with `confidence: low` and no verification plan | medium |
| Active risk with no `mitigates` relation | high |

Run this before starting a phase to confirm all blocking items are resolved.

---

## Execution Layer Checks

Run with `--layer exec`. These verify the structural integrity of the plan and phase graph.

### phase_order
Detects ordering problems in the phase sequence.

```bash
spec-graph validate --layer exec --check phase_order
```

| Condition | Severity |
|-----------|----------|
| Phase with `precedes` relation has a higher `order` value than its successor | high |
| Phase with `blocks` relation is not ordered before the blocked phase | medium |

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
| Active phase has no `belongs_to` relation pointing to a plan | high |
| Draft phase has no `belongs_to` relation | low |

Every active phase must belong to exactly one plan.

### exec_cycles
Detects circular chains in exec relations.

```bash
spec-graph validate --layer exec --check exec_cycles
```

| Condition | Severity |
|-----------|----------|
| Circular chain in `precedes` | high |
| Circular chain in `blocks` | high |

### invalid_exec_edges
Detects relations that violate the exec edge matrix.

```bash
spec-graph validate --layer exec --check invalid_exec_edges
```

| Condition | Severity |
|-----------|----------|
| Exec relation violates the exec edge matrix | high |

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
spec-graph validate --layer mapping --phase PHS-002 --check delivery_completeness
```

| Condition | Severity |
|-----------|----------|
| Phase covers an arch entity but no `delivers` relation exists for it | high |

This is the primary gate check before phase completion. Every covered arch entity must have
at least one `delivers` relation from the phase (or from a phase that delivers its implementing
entities as proxies).

### mapping_consistency
Detects cross-layer integrity problems.

```bash
spec-graph validate --layer mapping --check mapping_consistency
spec-graph validate --layer mapping --phase PHS-002 --check mapping_consistency
```

| Condition | Severity |
|-----------|----------|
| `covers` target is not an arch entity | high |
| `delivers` target is not an arch entity | high |
| `covers` source is not a phase | high |
| `delivers` source is not a phase | high |
| Phase delivers an entity it does not cover | medium |

The last condition (delivers without covers) is a warning: it may indicate a delivery was
recorded without the corresponding intent being modeled.

### invalid_mapping_edges
Detects relations that violate the mapping edge matrix.

```bash
spec-graph validate --layer mapping --check invalid_mapping_edges
```

| Condition | Severity |
|-----------|----------|
| Mapping relation violates the mapping edge matrix | high |

### gates
Detects phase readiness blockers by checking arch entities in the phase scope for
unresolved questions, unmitigated risks, unverified assumptions, and dependencies on
draft decisions.

```bash
spec-graph validate --layer mapping --check gates
spec-graph validate --layer mapping --phase PHS-002 --check gates
```

| Condition | Severity |
|-----------|----------|
| Active question in phase scope with no `answers` relation | high |
| Active risk in phase scope with no `mitigates` relation | high |
| Active assumption in phase scope (needs validation) | medium |
| Requirement in phase scope depends on a draft decision | high |

When `--phase` is specified, only that phase is checked. Without `--phase`, all active
phases are checked. Run this before starting or completing a phase.

---

## Phase Validation Guide

### Before Phase Start

```bash
# Exec: confirm plan is valid and phase ordering is correct
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
spec-graph validate --layer exec --check orphan_phases

# Arch: confirm no blocking open items
spec-graph validate --layer arch --check unresolved

# Mapping: confirm all requirements are assigned
spec-graph validate --layer mapping --check plan_coverage
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

# Mapping: confirm all covered items have delivery evidence
spec-graph validate --layer mapping --phase PHS-003 --check delivery_completeness

# Mapping: confirm cross-layer integrity
spec-graph validate --layer mapping --phase PHS-003 --check mapping_consistency
```

Purpose: verify implementation/test completeness, open-item resolution, and delivery evidence.

---

## Interpreting Validate Response

```json
{
  "valid": false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high",
      "entity": "REQ-007",
      "layer": "arch",
      "message": "No implementation found"
    },
    {
      "check": "delivery_completeness",
      "severity": "high",
      "entity": "PHS-002",
      "layer": "mapping",
      "message": "Phase covers REQ-007 but no delivers relation exists"
    },
    {
      "check": "single_active_plan",
      "severity": "high",
      "entity": "PLN-002",
      "layer": "exec",
      "message": "Multiple active plans found: PLN-001, PLN-002"
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
| `single_active_plan`: multiple active | set all but one plan to `archived` or `deprecated` |
| `orphan_phases`: phase not in plan | add `belongs_to` relation from phase to the active plan |
| `delivery_completeness`: no delivers | add `delivers` from the phase to the implementing entity (minimal proxy set) |
| `mapping_consistency`: delivers without covers | add `covers` relation, or investigate whether the delivery is correct |
| `superseded_refs`: stale reference | update relation to point to the replacement entity, or remove it |
