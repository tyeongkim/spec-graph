# Data Model Reference

## Table of Contents
1. [Entity Types & Metadata Schema](#entity-types--metadata-schema)
2. [Entity Status Lifecycle](#entity-status-lifecycle)
3. [Relation Types](#relation-types)
4. [Allowed Edge Matrix](#allowed-edge-matrix)
5. [Impact Propagation Weights](#impact-propagation-weights)

---

## Entity Types & Metadata Schema

Every entity has `id`, `type`, `title`, `description`, `status`, and `metadata`.
`metadata` is a JSON string with type-specific required/optional fields listed below.

### requirement (REQ)
```json
{
  "priority": "must | should | could",
  "kind": "functional | non_functional",
  "owner": "string"
}
```

### decision (DEC)
```json
{
  "rationale": "string — reasoning behind the decision",
  "date": "ISO8601"
}
```

### phase (PHS)
```json
{
  "goal": "string — phase objective",
  "order": "integer — sequence number",
  "exit_criteria": ["string[]"]
}
```

### interface (API)
```json
{
  "kind": "http | event | module | storage"
}
```

### state (STT)
```json
{
  "entity": "string — the subject that holds state",
  "from": "string — source state",
  "to": "string — target state"
}
```

### test (TST)
```json
{
  "kind": "unit | integration | e2e | property"
}
```

### crosscut (XCT)
Free-form metadata. Typical: `{"concern": "auth | audit | idempotency | ..."}`.

### question (QST)
```json
{
  "owner": "string",
  "due_at": "ISO8601 | null"
}
```

### assumption (ASM)
```json
{
  "confidence": "low | medium | high"
}
```

### criterion (ACT)
```json
{
  "given": "string — precondition",
  "when": "string — action",
  "then": "string — expected outcome"
}
```

### risk (RSK)
Free-form metadata. Typical: `{"likelihood": "low|medium|high", "impact": "low|medium|high"}`.

---

## Entity Status Lifecycle

```
draft → active → deprecated
                → deleted
draft → active → resolved   (question, risk, assumption only)
```

- `draft`: initial state on creation
- `active`: confirmed and valid in the graph
- `deprecated`: no longer valid but preserved for history (used with supersedes)
- `resolved`: question answered, assumption verified, or risk mitigated
- `deleted`: permanently removed

---

## Relation Types

| Relation | Meaning | Directionality |
|----------|---------|----------------|
| `implements` | implementation fulfills requirement/criterion | bidirectional |
| `verifies` | test verifies a target | from→to strong, reverse weak |
| `depends_on` | from depends on to | from→to unidirectional |
| `constrained_by` | from is constrained by a constraint entity | from→to unidirectional |
| `planned_in` | target is scheduled in a phase | from→to unidirectional |
| `delivered_in` | artifact is actually delivered in a phase | from→to unidirectional |
| `triggers` | interface/decision causes a state transition | from→to unidirectional |
| `answers` | decision resolves a question | from→to unidirectional |
| `assumes` | target relies on an assumption | from→to unidirectional |
| `has_criterion` | requirement owns an acceptance criterion | bidirectional |
| `mitigates` | target mitigates a risk | from→to unidirectional |
| `supersedes` | new entity replaces an older one | new→old, reverse weak |
| `conflicts_with` | two entities are semantically conflicting | bidirectional |
| `references` | weak reference link | bidirectional weak |

---

## Allowed Edge Matrix

Adding a relation that violates this matrix is rejected with exit code 3.

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `implements` | interface | requirement, criterion |
| `verifies` | test | requirement, criterion, decision, interface, state |
| `depends_on` | requirement, decision, interface, phase, test, state | requirement, decision, interface, state, crosscut, assumption |
| `constrained_by` | requirement, decision, interface, phase, state | crosscut, decision, assumption |
| `planned_in` | requirement, decision, interface, test, question, risk | phase |
| `delivered_in` | interface, state, test, decision | phase |
| `triggers` | interface, decision | state |
| `answers` | decision | question |
| `assumes` | requirement, decision, phase, interface | assumption |
| `has_criterion` | requirement | criterion |
| `mitigates` | decision, test, crosscut, phase | risk |
| `supersedes` | **same type only** | **same type only** |
| `conflicts_with` | same or related semantic types | same or related semantic types |
| `references` | any | any |

### Common Mistakes
- `implements`: source must be `interface`. A requirement cannot implement another requirement.
- `verifies`: source must be `test`. A requirement does not verify a test — it is the other way around.
- `planned_in` vs `delivered_in`: planned_in is intent; delivered_in is actual delivery. Use both distinctly.
- `supersedes`: both sides must be the same type. REQ cannot supersede DEC.

---

## Impact Propagation Weights

Each relation type has different propagation weights across three dimensions during impact analysis.

| Relation | Direction | Structural | Behavioral | Planning |
|----------|-----------|:----------:|:----------:|:--------:|
| `implements` | bidirectional | 0.9 | 0.8 | 0.4 |
| `verifies` | from→target, reverse weak | 0.4 | 0.8 | 0.3 |
| `depends_on` | from→to | 0.8 | 0.7 | 0.4 |
| `constrained_by` | from→to | 0.5 | 0.8 | 0.4 |
| `planned_in` | from→to | 0.1 | 0.2 | 0.8 |
| `delivered_in` | from→to | 0.3 | 0.3 | 0.9 |
| `triggers` | from→to | 0.6 | 0.9 | 0.2 |
| `answers` | from→to | 0.2 | 0.7 | 0.3 |
| `assumes` | from→to | 0.3 | 0.8 | 0.5 |
| `has_criterion` | bidirectional | 0.3 | 0.9 | 0.2 |
| `mitigates` | from→to | 0.2 | 0.6 | 0.4 |
| `supersedes` | new→old, reverse weak | 0.4 | 0.5 | 0.3 |
| `conflicts_with` | bidirectional | 0.8 | 0.9 | 0.5 |
| `references` | bidirectional weak | 0.1 | 0.1 | 0.1 |

### Reading the Weights
- 0.8+: strong propagation. Almost always requires co-review on change.
- 0.5–0.7: moderate. May be affected depending on content.
- 0.3 or below: weak propagation. Usually review-only.

### Agent Tips
- Interface change → `--dimension structural` to focus on high structural-weight paths
- Policy change → `--dimension behavioral`
- Schedule change → `--dimension planning`
- `references` has 0.1 across all dimensions, so it rarely appears in impact results. This is by design.
