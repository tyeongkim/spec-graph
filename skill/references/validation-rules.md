# Validation Rules Reference

## Table of Contents
1. [Core Graph Checks](#core-graph-checks)
2. [Coverage Checks](#coverage-checks)
3. [Workflow Gate Checks](#workflow-gate-checks)
4. [Phase Validation Guide](#phase-validation-guide)
5. [Interpreting Validate Response](#interpreting-validate-response)

---

## Core Graph Checks

These verify the structural integrity of the graph itself.

### orphans
Detects entities with zero relations.
```bash
spec-graph validate --check orphans
```
- Newly added entities without any relations are flagged.
- Draft-status orphans may be acceptable, but active-status orphans indicate a problem.

### cycles
Detects disallowed circular references.
```bash
spec-graph validate --check cycles
```
- Catches cycles in `depends_on` chains.
- `conflicts_with` is bidirectional by nature and is not treated as a cycle.

### conflicts
Detects entities linked by `conflicts_with` or policy-level conflicts.
```bash
spec-graph validate --check conflicts
```
- If two entities within the same phase have a `conflicts_with` relation and both are active, an issue is reported.

### invalid_edges
Detects relations that violate the allowed edge matrix.
```bash
spec-graph validate --check invalid_edges
```
- Normally not triggered when using `relation add` (which validates at insertion), but useful after direct DB edits or bootstrap imports.

### superseded_refs
Detects active entities still referencing deprecated or superseded entities.
```bash
spec-graph validate --check superseded_refs
```
- Always run this after deprecating an entity.
- When flagged, update references to point to the replacement entity or remove the relation.

---

## Coverage Checks

Detects missing implementations and verifications.

```bash
spec-graph validate --check coverage
spec-graph validate --phase PHS-002 --check coverage
```

### Detection Rules

| Condition | Meaning | Severity |
|-----------|---------|----------|
| Active requirement has no `implements` | missing implementation | high |
| Active requirement has no `has_criterion` | missing acceptance criterion | medium |
| Active criterion has no `verifies` | missing test | high |
| Interface that triggers a state has no related test | missing state-transition test | medium |

### Agent Behavior
- high severity → blocks phase exit. Must be resolved.
- medium severity → resolution recommended, but may proceed with justification.

---

## Workflow Gate Checks

Verifies prerequisites for phase entry and exit.

```bash
spec-graph validate --check gates
spec-graph validate --phase PHS-003 --check gates
```

### Detection Rules

| Condition | Meaning | Severity |
|-----------|---------|----------|
| Phase contains unresolved questions | open questions remain | high |
| High-risk item in phase without mitigates | unmitigated risk | high |
| Active requirement depends on draft decision | relying on unconfirmed decision | high |
| Entity assumes an assumption with no verification plan | unverified assumption | medium |
| Phase completion but core planned requirement missing delivery | planned vs delivered gap | high |

### Agent Behavior
- If gate check returns high severity, do not start or complete the phase.
- Resolve the issue first:
  - Unresolved question → create a decision with `answers` relation, or set question status to `resolved`
  - Unmitigated risk → add `mitigates` relation, or set risk status to `resolved`
  - Draft decision → update decision status to `active`
  - Unverified assumption → create a verification plan, or set assumption status to `resolved`

---

## Phase Validation Guide

### Before Phase Start
```bash
spec-graph validate --phase PHS-003 --check gates
spec-graph validate --phase PHS-003 --check orphans
```
Purpose: confirm that all prerequisites for items assigned to this phase are met.

### During Phase (on change)
```bash
spec-graph validate --phase PHS-003
```
Purpose: ensure no rules are broken after mid-phase changes.

### Before Phase Completion (required)
```bash
spec-graph validate --phase PHS-003 --check coverage
spec-graph validate --phase PHS-003 --check gates
spec-graph validate --phase PHS-003 --check superseded_refs
```
Purpose: verify implementation/test completeness, open-item resolution, and stale-reference cleanup.

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
      "message": "No implementation found"
    },
    {
      "check": "gates",
      "severity": "high",
      "entity": "PHS-002",
      "message": "Unresolved question remains while attempting phase completion"
    }
  ],
  "summary": {
    "total_issues": 2,
    "by_severity": {"high": 2, "medium": 0, "low": 0}
  }
}
```

### Agent Decision Criteria
- `valid: true` → safe to proceed to the next step.
- `valid: false` + high severity → must resolve before proceeding.
- `valid: false` + medium/low only → report to the user and let them decide.
