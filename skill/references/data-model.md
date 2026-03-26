# Data Model Reference

## Table of Contents
1. [Three-Layer Architecture](#three-layer-architecture)
2. [Entity Types & Metadata Schema](#entity-types--metadata-schema)
3. [Entity Status Lifecycle](#entity-status-lifecycle)
4. [Relation Types](#relation-types)
5. [Edge Matrices](#edge-matrices)
6. [Impact Propagation Weights](#impact-propagation-weights)

---

## Three-Layer Architecture

v1 organizes all entities and relations into three layers. Layer is always deterministic
from the entity type prefix or relation type — there is no ambiguity.

| Layer | Contains | Purpose |
|-------|----------|---------|
| `arch` | REQ, DEC, API, STT, TST, XCT, ACT, ASM, RSK, QST | Semantic meaning: what and why |
| `exec` | PLN, PHS | Delivery structure: when and how |
| `mapping` | covers, delivers (+ deprecated planned_in, delivered_in) | Cross-layer links: intent and completion |

### Layer Classification Rules

Entity layer is derived from type prefix:

```
REQ, DEC, API, STT, TST, XCT, ACT, ASM, RSK, QST  →  arch
PLN, PHS                                             →  exec
```

Relation layer is fixed per relation type:

```
implements, verifies, depends_on, constrained_by, triggers,
answers, assumes, has_criterion, mitigates, supersedes,
conflicts_with, references                           →  arch

belongs_to, precedes, blocks                         →  exec

covers, delivers, planned_in, delivered_in           →  mapping
```

---

## Entity Types & Metadata Schema

Every entity has `id`, `type`, `layer`, `title`, `description`, `status`, and `metadata`.
`metadata` is a JSON string with type-specific required/optional fields listed below.

### Architecture Layer Entities

#### requirement (REQ)
```json
{
  "priority": "must | should | could",
  "kind": "functional | non_functional",
  "owner": "string"
}
```

#### decision (DEC)
```json
{
  "rationale": "string — reasoning behind the decision",
  "date": "ISO8601"
}
```

#### interface (API)
```json
{
  "kind": "http | event | module | storage"
}
```

#### state (STT)
```json
{
  "entity": "string — the subject that holds state",
  "from": "string — source state",
  "to": "string — target state"
}
```

#### test (TST)
```json
{
  "kind": "unit | integration | e2e | property"
}
```

#### crosscut (XCT)
Free-form metadata. Typical: `{"concern": "auth | audit | idempotency | ..."}`.

#### question (QST)
```json
{
  "owner": "string",
  "due_at": "ISO8601 | null"
}
```

#### assumption (ASM)
```json
{
  "confidence": "low | medium | high"
}
```

#### criterion (ACT)
```json
{
  "given": "string — precondition",
  "when": "string — action",
  "then": "string — expected outcome"
}
```

#### risk (RSK)
Free-form metadata. Typical: `{"likelihood": "low|medium|high", "impact": "low|medium|high"}`.

### Execution Layer Entities

#### plan (PLN)
```json
{
  "status": "active | archived"
}
```

Only one plan may have `status: active` at a time. The `single_active_plan` exec check
enforces this constraint.

#### phase (PHS)
```json
{
  "goal": "string — phase objective",
  "order": "integer — sequence number",
  "exit_criteria": ["string[]"]
}
```

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

### Architecture Layer Relations (12)

| Relation | Meaning | Directionality |
|----------|---------|----------------|
| `implements` | implementation fulfills requirement/criterion | bidirectional |
| `verifies` | test verifies a target | from→to strong, reverse weak |
| `depends_on` | from depends on to | from→to unidirectional |
| `constrained_by` | from is constrained by a constraint entity | from→to unidirectional |
| `triggers` | interface/decision causes a state transition | from→to unidirectional |
| `answers` | decision resolves a question | from→to unidirectional |
| `assumes` | target relies on an assumption | from→to unidirectional |
| `has_criterion` | requirement owns an acceptance criterion | bidirectional |
| `mitigates` | target mitigates a risk | from→to unidirectional |
| `supersedes` | new entity replaces an older one | new→old, reverse weak |
| `conflicts_with` | two entities are semantically conflicting | bidirectional |
| `references` | weak reference link (cross-layer allowed) | bidirectional weak |

### Execution Layer Relations (3)

| Relation | Meaning | Directionality |
|----------|---------|----------------|
| `belongs_to` | phase belongs to a plan | phase→plan |
| `precedes` | phase must complete before another starts | phase→phase |
| `blocks` | phase blocks another from starting | phase→phase |

### Mapping Layer Relations (4)

| Relation | Meaning | Directionality | Status |
|----------|---------|----------------|--------|
| `covers` | phase covers an arch entity (intent) | phase→arch | current |
| `delivers` | phase delivers an arch entity (completion) | phase→arch | current |
| `planned_in` | arch entity is scheduled in a phase | arch→phase | **deprecated** |
| `delivered_in` | arch artifact is delivered in a phase | arch→phase | **deprecated** |

`covers` replaces `planned_in`. Direction is inverted: `phase --covers--> arch_entity`.
`delivers` replaces `delivered_in`. Direction is inverted: `phase --delivers--> arch_entity`.

`planned_in` and `delivered_in` remain functional for backward compatibility but will be
removed in a future release. Do not use them for new relations.

---

## Edge Matrices

Adding a relation that violates the applicable matrix is rejected with exit code 3.
Three separate matrices exist, one per layer.

### Architecture Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `implements` | interface | requirement, criterion |
| `verifies` | test | requirement, criterion, decision, interface, state |
| `depends_on` | requirement, decision, interface, phase, test, state | requirement, decision, interface, state, crosscut, assumption |
| `constrained_by` | requirement, decision, interface, phase, state | crosscut, decision, assumption |
| `planned_in` *(deprecated)* | requirement, decision, interface, test, question, risk, criterion | phase |
| `delivered_in` *(deprecated)* | interface, state, test, decision | phase |
| `triggers` | interface, decision | state |
| `answers` | decision | question |
| `assumes` | requirement, decision, phase, interface | assumption |
| `has_criterion` | requirement | criterion |
| `mitigates` | decision, test, crosscut, phase | risk |
| `supersedes` | **same type only** | **same type only** |
| `conflicts_with` | any | any |
| `references` | any | any (cross-layer allowed) |

### Execution Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `belongs_to` | phase | plan |
| `precedes` | phase | phase |
| `blocks` | phase | phase |

### Mapping Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `covers` | phase | requirement, decision, interface, test, question, risk, criterion, assumption |
| `delivers` | phase | requirement, interface, state, test, decision, criterion |

### Common Mistakes
- `implements`: source must be `interface`. A requirement cannot implement another requirement.
- `verifies`: source must be `test`. A requirement does not verify a test — it is the other way around.
- `covers` vs `delivers`: `covers` is intent (what the phase plans to address); `delivers` is
  completion evidence (what was actually built). Use both distinctly.
- `covers`/`delivers` direction: source is `phase`, target is the arch entity. This is the
  opposite of the deprecated `planned_in`/`delivered_in`.
- `belongs_to`: source is `phase`, target is `plan`. A plan does not belong to a phase.
- `supersedes`: both sides must be the same type. REQ cannot supersede DEC.
- `planned_in`/`delivered_in`: do not use for new relations. Use `covers`/`delivers` instead.

---

## Impact Propagation Weights

Each relation type has different propagation weights across three dimensions during impact analysis.

| Relation | Direction | Structural | Behavioral | Planning |
|----------|-----------|:----------:|:----------:|:--------:|
| `implements` | bidirectional | 0.9 | 0.8 | 0.4 |
| `verifies` | from→target, reverse weak | 0.4 | 0.8 | 0.3 |
| `depends_on` | from→to | 0.8 | 0.7 | 0.4 |
| `constrained_by` | from→to | 0.5 | 0.8 | 0.4 |
| `covers` | from→to | 0.1 | 0.2 | 0.8 |
| `delivers` | from→to | 0.3 | 0.3 | 0.9 |
| `planned_in` *(deprecated)* | from→to | 0.1 | 0.2 | 0.8 |
| `delivered_in` *(deprecated)* | from→to | 0.3 | 0.3 | 0.9 |
| `triggers` | from→to | 0.6 | 0.9 | 0.2 |
| `answers` | from→to | 0.2 | 0.7 | 0.3 |
| `assumes` | from→to | 0.3 | 0.8 | 0.5 |
| `has_criterion` | bidirectional | 0.3 | 0.9 | 0.2 |
| `mitigates` | from→to | 0.2 | 0.6 | 0.4 |
| `supersedes` | new→old, reverse weak | 0.4 | 0.5 | 0.3 |
| `conflicts_with` | bidirectional | 0.8 | 0.9 | 0.5 |
| `references` | bidirectional weak | 0.1 | 0.1 | 0.1 |
| `belongs_to` | from→to | 0.1 | 0.1 | 0.7 |
| `precedes` | from→to | 0.1 | 0.1 | 0.9 |
| `blocks` | from→to | 0.1 | 0.1 | 0.9 |

### Reading the Weights
- 0.8+: strong propagation. Almost always requires co-review on change.
- 0.5–0.7: moderate. May be affected depending on content.
- 0.3 or below: weak propagation. Usually review-only.

### Agent Tips
- Interface change → `--dimension structural` to focus on high structural-weight paths
- Policy change → `--dimension behavioral`
- Schedule change → `--dimension planning`
- `references` has 0.1 across all dimensions, so it rarely appears in impact results. This is by design.
- `belongs_to`, `precedes`, `blocks` have high planning weight — exec changes ripple into planning impact.
