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
| `exec` | PLN, PHS, TSK, CHG | Delivery structure: when and how |
| `mapping` | covers, delivers | Cross-layer links: intent and completion |

### Layer Classification Rules

Entity layer is derived from type prefix:

```
REQ, DEC, API, STT, TST, XCT, ACT, ASM, RSK, QST  →  arch
PLN, PHS, TSK, CHG                                   →  exec
```

Relation layer is fixed per relation type:

```
implements, verifies, depends_on, constrained_by, triggers,
answers, assumes, has_criterion, mitigates, supersedes,
conflicts_with, references                           →  arch

belongs_to, task_depends_on, precedes, blocks        →  exec

covers, delivers                                     →  mapping
```

---

## Entity Types & Metadata Schema

Every entity has `id`, `type`, `layer`, `title`, `description`, `status`, and `metadata`.
`metadata` is a JSON string with type-specific fields listed below.

> **Enforcement note**: only **task** metadata is validated against a closed contract (required
> fields, no unknown keys). For every other entity type the CLI validates only that `metadata` is a
> JSON object — the fields and enum values below are the project convention, not enforced
> constraints. Follow them anyway so the graph stays queryable and consistent; nothing will reject a
> typo or a missing field.

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

#### task (TSK)
Task metadata is a closed contract; unknown keys are rejected:
```json
{
  "order": 1,
  "instructions": ["non-empty instruction"],
  "acceptance": ["non-empty acceptance condition"],
  "must_not": [],
  "references": [],
  "qa": [{"command":"go test ./...","expected":"exit 0","evidence":""}]
}
```

`order` is positive. `instructions`, `acceptance`, and `qa` are non-empty arrays.
`must_not` and `references` are required arrays that may be empty. QA `command` and `expected`
are non-empty; `evidence` is empty before resolution and must name a repository-relative regular
file at resolution.

#### change (CHG)
```json
{
  "changed_entities": ["string[] — optional, list of affected file paths or identifiers"]
}
```

A change is a lightweight, independent work unit (PR, bugfix, patch). It does NOT
belong to any plan or phase. It connects directly to arch entities via `covers` to
mark scope. CHG is NOT a delivery/evidence unit — use PHS `delivers` for that.

Constraints:
- No `belongs_to`, `precedes`, or `blocks` relations allowed
- No CHG↔CHG relations of any kind
- Must have at least one relation to a non-CHG entity (validated as `orphan_changes`)

---

## Entity Status Lifecycle

```
draft → active → deprecated
                → deleted
draft → active → resolved
```

- `draft`: initial state on creation
- `active`: confirmed and valid in the graph
- `deprecated`: no longer valid but preserved for history (used with supersedes)
- `resolved`: question answered, assumption verified, risk mitigated, or work completed
- `deleted`: permanently removed

Enforcement: the schema allows all five statuses for every entity type except `question`, which
does not permit `deprecated`. The sequence above is the intended convention rather than a validated
state machine — for arch entities no generic transition guard exists, so `draft → resolved` will be
accepted even though it skips a step. Real gating applies only to plan, phase, and task.

Tasks use the stricter lifecycle `draft → active → resolved` or `draft|active → deprecated`.
`resolved` and `deprecated` are terminal; deprecation requires a reason. Activation requires exactly
one parent, that parent must be an active phase, and prerequisites must be resolved. Resolution
requires resolved prerequisites, QA evidence, and delivery for every deliverable covered target.

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

### Execution Layer Relations (4)

| Relation | Meaning | Directionality |
|----------|---------|----------------|
| `belongs_to` | membership, exactly task→phase or phase→plan | child→parent |
| `task_depends_on` | task depends on a prerequisite in the same phase | dependent→prerequisite |
| `precedes` | phase must complete before another starts | phase→phase |
| `blocks` | phase blocks another from starting | phase→phase |

### Mapping Layer Relations (2)

| Relation | Meaning | Directionality |
|----------|---------|----------------|
| `covers` | phase/change/task covers an arch entity (intent) | exec→arch |
| `delivers` | phase/task delivers an arch entity (completion) | phase/task→arch |

`covers` replaced the removed `planned_in`. Direction is inverted: `phase --covers--> arch_entity`.
`delivers` replaced the removed `delivered_in`. Direction is inverted: `phase --delivers--> arch_entity`.

`planned_in` and `delivered_in` were removed in v1. Use `covers` and `delivers` instead.

---

## Edge Matrices

Adding a relation that violates the applicable matrix is rejected with exit code 3.
Three separate matrices exist, one per layer.

### Architecture Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `implements` | interface | requirement, criterion |
| `verifies` | test | requirement, criterion, decision, interface, state |
| `depends_on` | requirement, decision, interface, test, state | requirement, decision, interface, state, crosscut, assumption |
| `constrained_by` | requirement, decision, interface, state | crosscut, decision, assumption |
| `triggers` | interface, decision | state |
| `answers` | decision | question |
| `assumes` | requirement, decision, interface | assumption |
| `has_criterion` | requirement | criterion |
| `mitigates` | decision, test, crosscut | risk |
| `supersedes` | **same type only** | **same type only** |
| `conflicts_with` | any | any |
| `references` | any | any (cross-layer allowed) |

### Execution Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `belongs_to` | task or phase | task→phase, phase→plan only |
| `task_depends_on` | task | task in the same phase |
| `precedes` | phase | phase |
| `blocks` | phase | phase |

### Mapping Edge Matrix

| Relation | From (allowed source types) | To (allowed target types) |
|----------|----------------------------|--------------------------|
| `covers` | phase, change, task | requirement, decision, interface, test, question, risk, criterion, assumption |
| `delivers` | phase, task | requirement, interface, state, test, decision, criterion |

For a task-managed phase, effective scope and delivery are the union of child task mappings and
direct phase mappings are forbidden. A taskless phase retains direct mappings unchanged. Tasks are
exec entities and never enter architecture satisfaction closure.

### Common Mistakes
- `implements`: source must be `interface`. A requirement cannot implement another requirement.
- `verifies`: source must be `test`. A requirement does not verify a test — it is the other way around.
- `covers` vs `delivers`: `covers` is intent (what the phase plans to address); `delivers` is
  completion evidence (what was actually built). Use both distinctly.
- `covers`/`delivers` direction: source is `phase`, target is the arch entity. This is the
  opposite of the removed `planned_in`/`delivered_in` (which were arch→phase).
- `belongs_to`: only task→phase and phase→plan are legal.
- `supersedes`: both sides must be the same type. REQ cannot supersede DEC. Prefer creating this
  edge with `entity revise`, which also moves inbound relations and deprecates the prior entity;
  adding it by hand does neither. Only arch entities form revision chains.
- `planned_in`/`delivered_in`: removed in v1. Use `covers`/`delivers` instead.
- `covers` from CHG: CHG can cover arch entities, but CHG CANNOT use `delivers`. Only phases deliver.
- `conflicts_with`: the edge matrix permits any type pair, but self-loops are rejected (as they are
  for every relation type). It is symmetric and stored once, in the file of the lexicographically
  smaller endpoint ID; both directions remain queryable through the index.

---

## Impact Propagation Weights

Each relation type has different propagation weights across three dimensions during impact analysis.

| Relation | Direction | Structural | Behavioral | Planning |
|----------|-----------|:----------:|:----------:|:--------:|
| `implements` | bidirectional | 0.9 | 0.8 | 0.4 |
| `verifies` | forward, reverse weak | 0.4 | 0.8 | 0.3 |
| `depends_on` | forward | 0.8 | 0.7 | 0.4 |
| `constrained_by` | forward | 0.5 | 0.8 | 0.4 |
| `covers` | forward, reverse weak | 0.1 | 0.2 | 0.8 |
| `delivers` | forward | 0.3 | 0.3 | 0.9 |
| `triggers` | forward | 0.6 | 0.9 | 0.2 |
| `answers` | forward | 0.2 | 0.7 | 0.3 |
| `assumes` | forward | 0.3 | 0.8 | 0.5 |
| `has_criterion` | bidirectional | 0.3 | 0.9 | 0.2 |
| `mitigates` | forward | 0.2 | 0.6 | 0.4 |
| `supersedes` | forward (new→old), reverse weak | 0.4 | 0.5 | 0.3 |
| `conflicts_with` | bidirectional | 0.8 | 0.9 | 0.5 |
| `references` | bidirectional weak | 0.1 | 0.1 | 0.1 |
| `belongs_to` | forward | 0.1 | 0.1 | 0.7 |
| `task_depends_on` | forward, reverse weak | 0.2 | 0.2 | 0.8 |
| `precedes` | forward | 0.1 | 0.1 | 0.6 |
| `blocks` | forward | 0.2 | 0.2 | 0.8 |

"Reverse weak" means reverse traversal applies the weight multiplied by 0.5.
Severity thresholds: score ≥ 0.7 is `high`, ≥ 0.4 is `medium`, below that is `low`.

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
