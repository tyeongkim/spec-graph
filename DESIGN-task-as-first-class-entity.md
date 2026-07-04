# Design Report — Level 2: Promoting Tasks to First-Class Graph Entities

> Status: Proposal (discussion / decision-pending) — **revised after Oracle design review**
> Scope: spec-graph entity model, relation model, edge matrix, schema, analysis engine
> Prerequisite: Level 1 (nested-metadata serialization fix in `internal/toml/writer.go`) — **DONE**
>
> **Review note (v2):** An Oracle code-verification review confirmed the architecture catalog and line anchors are accurate, and the core design (task in `exec`, dedicated `task_depends_on`, extended `covers`/`delivers`) is sound. It also caught four issues that materially change the plan, now folded in below and marked `[rev]`:
> 1. arch→exec impact does **not** flow today (`covers`/`delivers` are `Forward`-only, no reverse) — see §6.1.
> 2. `query scope` is **not** generic — it hardcodes `covers`/`delivers` — see §6.5 + §8.
> 3. `checkDeliveryCompleteness` is **dead code** (filters on non-existent status `"completed"`) — must be fixed before building on it — see §6.2/§6.3.
> 4. The agent-facing `skills/*/SKILL.md` catalog and several secondary files were missing from the checklist — see §8.

---

## 1. Executive Summary

spec-graph today stores task-level planning data (task lists, task dependencies, parallelization groups, per-task exit criteria) inside the free-form `metadata` field of `phase` (PHS) entities. Even after the Level 1 fix — which makes nested JSON round-trip losslessly — this remains an **architectural anti-pattern**: we are re-implementing a graph (tasks + dependencies) *inside* an opaque blob that sits *inside* a node of the real graph.

The tool already owns a mature, data-driven analysis engine: multi-dimensional impact propagation, 19 validation checks across three layers, policy-driven phase/plan gates, phase-satisfaction closure computation, and three export renderers. None of that machinery can see *inside* a metadata blob. Task dependencies, task→requirement delivery, and task completion state are therefore invisible to impact analysis, validation, and gates.

**Proposal:** introduce `task` (prefix `TSK`) as a first-class entity type in the `exec` layer, connect it with a small set of new/extended relations, and register it in the schema and edge matrix. Because the entire analysis engine is table-driven, most of the work is *data* (map entries), not new algorithms. This turns tasks into fully analyzable graph citizens and lets `metadata` return to holding only flat scalars.

This is **Level 2**. It is independent of, and strictly additive to, the Level 1 fix. Level 1 made the current design *safe*; Level 2 makes it *correct*.

---

## 2. Background

### 2.1 The Level 1 fix (context)

`internal/toml/writer.go`'s `formatValue()` previously stringified any non-scalar via `fmt.Sprint(v)`, corrupting nested objects/arrays into Go's `map[...]` representation (e.g. `tasks = "[map[agent:quick ...]]"`). Level 1 added `map[string]any`, `[]any`, and `nil` cases so nested JSON now round-trips losslessly through the full production path (`json.RawMessage` → `EntityFileFrom` → `MarshalEntityFile` → `toml.Decode` → `ToEntity`), verified by `TestEntityFileFrom_NestedMetadataRoundTrip`.

### 2.2 Why Level 1 is not enough

Lossless *storage* of nested task data is not the same as *modeling* it. With tasks living inside `PHS.metadata`:

- **Impact analysis is blind.** `spec-graph impact PHS-007` cannot report "3 tasks affected" because tasks are not nodes. (Note: the *reverse* direction, `impact REQ-001` → tasks, additionally requires a propagation-direction change — see §6.1 `[rev]`.)
- **Validation is blind.** No check can detect "requirement covered by a phase but no task delivers it" or "task depends on an incomplete task."
- **Gates are blind.** Phase resolution cannot be blocked on "tasks incomplete" because task completion is a JSON field, not a status.
- **Dependencies are un-navigable.** `parallelization` / `depends_on` between tasks is expressed as ID strings inside a blob; there is no `query path`, no cycle detection, no topological reasoning.
- **Schema cannot validate it.** `internal/toml/schema.go` performs *zero* validation of metadata contents (it only checks ID prefix + status). Task structure can drift or typo silently.

We are, in effect, maintaining a second graph model in JSON that the graph tool refuses to look at.

---

## 3. Current Architecture (authoritative reference)

### 3.1 Entity types (13) — `internal/model/entity.go`

| String | Go constant | Prefix | Layer |
|---|---|---|---|
| `requirement` | `EntityTypeRequirement` | `REQ` | arch |
| `decision` | `EntityTypeDecision` | `DEC` | arch |
| `interface` | `EntityTypeInterface` | `API` | arch |
| `state` | `EntityTypeState` | `STT` | arch |
| `test` | `EntityTypeTest` | `TST` | arch |
| `crosscut` | `EntityTypeCrosscut` | `XCT` | arch |
| `criterion` | `EntityTypeCriterion` | `ACT` | arch |
| `assumption` | `EntityTypeAssumption` | `ASM` | arch |
| `risk` | `EntityTypeRisk` | `RSK` | arch |
| `question` | `EntityTypeQuestion` | `QST` | arch |
| `plan` | `EntityTypePlan` | `PLN` | exec |
| `phase` | `EntityTypePhase` | `PHS` | exec |
| `change` | `EntityTypeChange` | `CHG` | exec |

### 3.2 Relation types (17) — `internal/model/relation.go`

- **arch (12):** `implements`, `verifies`, `depends_on`, `constrained_by`, `triggers`, `answers`, `assumes`, `has_criterion`, `mitigates`, `supersedes`, `conflicts_with`, `references`
- **exec (3):** `belongs_to` (phase→plan), `precedes` (phase→phase), `blocks` (phase→phase)
- **mapping (2):** `covers` (phase/change→arch), `delivers` (phase→arch)

### 3.3 The three layers — `internal/model/layer.go`

- `arch` — the "what/why" (requirements, decisions, interfaces, tests, risks, …)
- `exec` — the "when/how-sequenced" (plan, phase, change)
- `mapping` — relations only; the bridge between exec and arch (`covers`, `delivers`)

### 3.4 The analysis engine is data-driven (the crux of this proposal)

Every analytical capability dispatches off tables and type maps, not hardcoded logic:

- **Impact** (`internal/graph/impact.go`, `propagation.go`): priority-queue BFS. Each relation type has an entry in `PropagationTable` with a direction rule (`Forward` / `Bidirectional` / `ForwardReverseWeak`) and per-dimension multipliers (`Structural`, `Behavioral`, `Planning`). Adding a relation = adding a table row.
- **Validation** (`internal/validate/{arch,exec,mapping}.go`): 19 named checks, each a switch-case in a per-layer validator. Adding a check = adding a case.
- **Gates** (`internal/gate/policy.go`): entity-type→to-status policies map to a set of checks + blocking severities. Adding a gate = adding a map entry.
- **Satisfaction** (`internal/validate/satisfaction.go`): computes a closure of entities reachable from a phase via `covers` + 1-depth `depends_on`/`implements`. Expanding the closure = adding a relation to the walk.
- **Export** (`internal/graph/export.go`): DOT/Mermaid/JSON keyed by entity-type→shape/bracket/color maps. Adding a type = adding a style entry.

**Implication:** first-class tasks inherit the entire engine primarily through *configuration*, not new algorithms.

### 3.5 Schema is self-describing but does not touch metadata — `internal/toml/schema.go`

- Valid layers: `arch`, `exec`, `mapping` (`validate()`, L181).
- `ValidateEntity` (L106) checks only ID-prefix match + allowed status.
- `ValidateRelation` (L132) enforces `from`/`to` entity-type lists, plus specials `same_type` and `any_to_any`.
- **`metadata` is entirely unvalidated** — there is no per-type field schema.

### 3.6 Known pre-existing inconsistency (relevant precedent)

`change` (CHG, exec) exists in the Go model constants (`entity.go` L25, L41) **but is absent from `DefaultSchema()`** (`schema.go` L65–102). The drift is wider than a single missing entry: the edge matrix allows `covers` from `phase`+`change` (`edge_matrix.go:69`), but `DefaultSchema()`'s `covers.From` is `["phase"]` only (`schema.go:98`) — so the two edge-rule sources of truth **already disagree today**. `change` also has no exec relations and is not in `delivers`, and is missing from the legacy DB CHECK constraint. This is both (a) a bug to be aware of and (b) the closest existing precedent for a lightweight exec-layer work unit — and a warning that adding a type means touching the model constants *and* `DefaultSchema()` *and* the edge matrix in lockstep, or they drift. See §5.3 `[rev]`: this makes adding `change` a **hard prerequisite** of our schema change, not optional cleanup.

---

## 4. The Anti-Pattern, Stated Precisely

A `phase` metadata blob currently encodes:

```json
{
  "tasks": [
    {"id": "T1", "agent": "quick", "files": ["a.ts", "b.ts"], "commit": "...", "must_not": "..."},
    {"id": "T2", "agent": "deep", "depends_on": ["T1"]}
  ],
  "parallelization": {"groups": [{"name": "g1", "task_ids": ["T1", "T2"]}]},
  "plan_task_ids": ["T1", "T2"],
  "exit_criteria": ["builds", "tests pass"]
}
```

Three of these keys are **graph-shaped data forced into a scalar container**:

1. `tasks[].id` → node identity
2. `tasks[].depends_on` / `parallelization.groups[].task_ids` → edges between nodes
3. `plan_task_ids` → membership edges

Encoding nodes-and-edges as nested JSON inside a node means the graph engine can never traverse them. `exit_criteria`, by contrast, is legitimately flat per-phase metadata and can stay (or become criterion entities — out of scope here).

---

## 5. Proposed Design

### 5.1 New entity type: `task` (TSK)

| Field | Value |
|---|---|
| String value | `task` |
| Go constant | `EntityTypeTask` |
| Prefix | `TSK` |
| Layer | `exec` |
| Allowed status | `draft`, `active`, `resolved`, `deleted` (mirrors `question`; a task is either to-do/in-progress/done/dropped — `deprecated` is not meaningful) |

**Layer rationale:** a task is an execution artifact, sequenced and owned, exactly like `phase`/`plan`/`change`. It belongs in `exec`. Placing it in `arch` would be wrong (it is not a "what/why" spec element) and would break the layer invariant that arch entities carry no exec relations.

**Metadata after promotion (flat only):** `agent`, `commit`, `order`, and free-form guardrail strings (`must`, `must_not`) remain as flat scalars in `TSK.metadata`. Everything structural moves to relations.

### 5.2 New / extended relations

| Relation | Layer | From → To | New or Extended | Purpose |
|---|---|---|---|---|
| `has_task` | exec | `phase` → `task` | **new** | phase membership of a task (parallels `belongs_to`) |
| `task_depends_on` | exec | `task` → `task` | **new** | task-level ordering / parallelization; cycle-checkable |
| `covers` | mapping | add `task` to `from` | **extended** | a task addresses an arch entity |
| `delivers` | mapping | add `task` to `from` | **extended** | a task delivers an arch entity (completion evidence) |

Design notes:

- **Why a dedicated `task_depends_on` rather than reusing arch `depends_on`?** The existing `depends_on` is an *arch-layer* relation with an arch-only edge matrix (`edge_matrix.go` L20–23). Reusing it for exec-layer task ordering would blur the layer model and force awkward matrix widening. Critically, the relation's layer is stamped at creation time from the relation type (`engine_relation.go:179`) and drives every layer filter and validation dispatch — reusing `depends_on` would tag exec-layer task edges as `layer=arch` and corrupt filtering. A separate exec relation is the only clean option, and it lets impact/validation treat task ordering distinctly (planning-dimension-heavy, like `precedes`/`blocks`).
- **Why extend `covers`/`delivers` instead of new relations?** These *are* the mapping-layer bridge from exec to arch. `covers` already admits `change` as a source, so admitting `task` is a natural, minimal widening. `delivers` from a task gives us per-task delivery evidence — the foundation for task-aware gates.
- **`has_task` vs. reversing to `belongs_to`:** `belongs_to` is currently phase→plan. We keep direction parent→child by mirroring with `has_task` (phase→task) to avoid overloading `belongs_to` across two child types with different matrices. (Alternative: widen `belongs_to` to `task → phase`; see §10.)

### 5.3 Schema changes — `internal/toml/schema.go` `DefaultSchema()`

Add to `EntityTypes`:

```toml
"task": {Prefix: "TSK", Layer: "exec", AllowedStatus: ["draft", "active", "resolved", "deleted"]}
```

Add to `RelationTypes`:

```toml
"has_task":        {Layer: "exec",    From: ["phase"], To: ["task"]}
"task_depends_on": {Layer: "exec",    From: ["task"],  To: ["task"]}
# extend existing:
"covers":   {..., From: ["phase", "change", "task"], ...}
"delivers": {..., From: ["phase", "task"], ...}
```

**`[rev]` Adding `change` is a hard dependency, not optional hygiene.** `DefaultSchema()` currently lists `covers.From = ["phase"]` (`schema.go:98`), which already diverges from the edge matrix (`edge_matrix.go:69` allows `phase` + `change`). The moment we extend `covers.From`/`delivers.From` to reference `change` or `task`, `Schema.validate()` (`schema.go:221-229`) will **fail its own from/to reference check** unless every referenced entity type is defined. So `change` (currently absent from `DefaultSchema()` despite existing in the Go model) **must** be added in the same pass, or the schema fails to load.

### 5.4 Edge matrix changes — `internal/model/edge_matrix.go`

- Exec matrix (L51): add `has_task` (phase→task) and `task_depends_on` (task→task).
- Mapping matrix (L67): add `task` to the `from` sets of `covers` and `delivers`.

### 5.5 Model changes — `internal/model/{entity.go, relation.go, layer.go}`

- `entity.go`: add `EntityTypeTask = "task"`, `TypePrefixMap[EntityTypeTask] = "TSK"`.
- `relation.go`: add `RelationHasTask = "has_task"`, `RelationTaskDependsOn = "task_depends_on"`.
- `layer.go`: classify `task` as `exec`; classify `has_task` and `task_depends_on` as `exec`.

---

## 6. What This Unlocks

Because the engine is table-driven (§3.4), the following become available largely for free once the type/relations/rows exist:

### 6.1 Impact analysis through tasks
Add `PropagationTable` + `ReasonTemplates` rows for the new relations (`internal/graph/propagation.go`; note both maps must be updated or the impact `Reason` string is empty). Suggested profiles, consistent with existing planning-heavy relations (`precedes` 0.1/0.1/0.6, `blocks` 0.2/0.2/0.8, `delivers` 0.3/0.3/0.9):

| Relation | Direction | Structural | Behavioral | Planning |
|---|---|---|---|---|
| `has_task` | Forward (or `ForwardReverseWeak` — see below) | 0.1 | 0.1 | 0.7 |
| `task_depends_on` | Forward | 0.2 | 0.2 | 0.8 |

**What works with these rows:** `spec-graph impact PHS-007` → `has_task → task → task_depends_on → task`, and `impact TSK-042` → `covers/delivers → arch entity`. `ImpactSummary.ByType` reports affected task counts automatically for these directions.

**`[rev]` What does NOT work — and why the original flagship example was wrong:** `spec-graph impact REQ-001` does **not** reach phases or tasks today, and adding these rows does not change that. `covers` and `delivers` are `Direction: Forward` (`propagation.go:104-111`), and `resolveNeighbor` (`impact.go:232-256`) yields no neighbor when the current node is the *target* of a Forward edge. `ImpactOptions` (`graph/types.go:24-34`) has no reverse flag. So arch→exec impact **does not exist in the engine at all right now** — an independent pre-existing limitation, not something task-promotion introduces.

To deliver "`impact REQ-001` reports N affected tasks," a **separate, explicit decision** is required: change `covers` and `delivers` to `ForwardReverseWeak` (reverse at 0.5×, matching how `verifies` already propagates backward). This is a **behavior change to existing impact output** for every current user of `covers`/`delivers`, so it must be evaluated on its own merits (blast-radius: any `impact <arch-id>` call would newly surface covering phases). It is **out of scope for the minimal Level 2** and tracked as open question Q6. Absent that change, task impact analysis is exec-source-only, which is still useful ("what does changing this phase/task affect?").

### 6.2 Task-aware validation checks
Each is a new switch-case in the existing per-layer validators (plus registration in `internal/validate/types.go`):
- **exec** (`exec.go`): `task_cycles` — DFS over `task_depends_on` (mirror of existing `exec_cycles` for `blocks`). Severity high.
- **exec**: `orphan_tasks` — tasks with no inbound `has_task` edge (mirror of `orphan_phases`). Severity medium.
- **mapping** (`mapping.go`): task delivery completeness — an entity covered by a resolved task but with no `delivers` from that task.

**`[rev]` Prerequisite bug fix — `delivery_completeness` is currently dead code.** `checkDeliveryCompleteness` (`mapping.go:117`) filters phases by status `"completed"` (L119-121), but `"completed"` is not a legal status anywhere (`entity.go:64-70`; allowed = draft/active/deprecated/resolved/deleted). The check therefore **always returns nil today** — including inside the phase→resolved gate that depends on it. We cannot "extend" a no-op. This must first be fixed to filter on the real status (`resolved`, and/or scope via `opts.Phase` during gating). Treat this as an explicit prerequisite task, not a freebie. (Related pre-existing bug worth fixing in the same pass: the satisfaction `test` allowlist is `{verified, passed}` — both unreachable statuses; see §6.4.)

### 6.3 Task-aware gates
Add to `internal/gate/policy.go`:
- A `phase → resolved` policy addition: block if any `has_task` child is not `resolved` (task completeness). This makes "phase done" mean "all its tasks done" — enforced, not asserted.
- Optionally a `task → resolved` policy checking the task has `delivers` evidence.

**`[rev]` Gate-layer registration trap.** `gate.Enforce` builds validate options with `Layer = LayerMapping` hardcoded (`gate.go:46`), and each validator skips checks whose registered layer ≠ the requested layer (`exec.go:19-21`). So the new task-completeness check invoked *by the gate* **must be registered as a mapping-layer check** (like the existing `gates` check), even though `task_cycles`/`orphan_tasks` in §6.2 are correctly exec-layer for the `validate` command. Either register the gate's task check under mapping, or change `gate.go` to also run exec checks. This is a real incompatibility, not a naming detail.

### 6.4 Task satisfaction in phase closure
`internal/validate/satisfaction.go` computes a closure from a phase. Adding `has_task` as a closure-expansion edge pulls tasks into phase satisfaction, so a phase is "satisfied" only when its tasks meet their evidence rules.

**`[rev]` Specify the task allowlist explicitly.** Closure members are evaluated against `satisfactionTargetStatusAllowlist` (`satisfaction.go:31-34`); a type with no entry falls back to `{active, resolved}`. Without an explicit entry, an *active* (in-progress) task would count as satisfied — probably not intended. Add `satisfactionTargetStatusAllowlist[task] = {resolved}` so a phase is satisfied only when its tasks are actually done. Do not rely on the fallback by accident.

### 6.5 Navigable task queries
`query path REQ-001 TSK-042` (shortest connection) and `query neighbors PHS-007 --depth 2` (phase's tasks and their deps) work generically via BFS in `internal/graph/query.go` — no change needed.

**`[rev]` `query scope` is NOT generic and needs a code change.** `QueryScope` hardcodes `rel.Type == RelationCovers || rel.Type == RelationDelivers` (`internal/graph/query.go:36`). Task membership via `has_task` will **not** appear in `query scope PHS-007` until that predicate is extended to include `has_task` (and transitively enumerate a phase's tasks). `internal/graph/query.go` was missing from the original §8 checklist and is now added.

### 6.6 Tasks in exports
Add entity-type style entries in `internal/graph/export.go` (a shape for DOT, a bracket style + `exec` CSS class for Mermaid). Task nodes and `task_depends_on` edges then render in graph visualizations.

---

## 7. Migration Path

This is the sensitive part; ordering matters because the pre–Level-1 corruption (`fmt.Sprint`) was one-way and the only intact source of task detail is the `.md` plan files.

1. **Land Level 1** (writer fix) — DONE.
2. **Land Level 2 schema/model/matrix additions** (this report) behind tests. No data touched yet. A binary upgrade is sufficient — the engine validates against `DefaultSchema()` inline (`engine_entity.go:193,404`) and never loads a project's `schema.toml` at runtime (`LoadSchema` has zero callers), so existing projects need no schema regeneration.
3. **`[rev]` Commit `.spec-graph/` to version control** before any import run. The TOML files are the real rollback mechanism; a clean commit is the safety net if the importer misbehaves.
4. **Write a one-shot importer in `internal/bootstrap`** (NOT `internal/cli/migrate.go`). `[rev]` `migrate.go` is the *legacy SQLite→TOML migrator* wired to `internal/db`, whose CHECK constraints reject `task`/`change` — wrong tool. `internal/bootstrap` already is a pattern-based `.md`→entities importer (`scan.go`, `engine_bootstrap.go:140`); extend it. For each phase, read task detail from the **intact `.md` plan** (not from corrupted `PHS.metadata`), and:
   - create `TSK-*` entities (flat metadata only),
   - create `has_task` edges phase→task,
   - create `task_depends_on` edges from the old `depends_on`/`parallelization` data,
   - create `covers`/`delivers` edges task→arch where the plan specifies them,
   - strip the now-migrated structural keys (`tasks`, `parallelization`, `plan_task_ids`) from `PHS.metadata`, leaving flat scalars.
   **`[rev]` Make it idempotent / re-runnable per phase.** A crash between task-creation and metadata-stripping must not leave partial state that a re-run duplicates — key on phase ID and skip phases already migrated (detect existing `has_task` edges).
5. **Validate** with `spec-graph doctor` + `spec-graph validate` (new `task_cycles`, `orphan_tasks`).
6. **Only then** delete the `.md` plan files. Before this step, `.md` remains the sole source of truncation-free task detail; deleting earlier is irreversible data loss.

Corrupted `PHS-068`-style entities cannot be auto-recovered from the graph — their structure survives only in `.md`. The importer must source from `.md`, never from the mangled metadata string.

---

## 8. Implementation Checklist (files to touch)

| File | Change |
|---|---|
| `internal/model/entity.go` | add `EntityTypeTask`, prefix map entry; (also add missing `EntityTypeChange` to schema, see below) |
| `internal/model/relation.go` | add `RelationHasTask`, `RelationTaskDependsOn` |
| `internal/model/layer.go` | classify `task`, `has_task`, `task_depends_on` as exec |
| `internal/model/edge_matrix.go` | exec: `has_task`, `task_depends_on`; mapping: add `task` to `covers`/`delivers` from-sets |
| `internal/toml/schema.go` | `DefaultSchema()`: add `task` entity + 2 relations + extend `covers`/`delivers`; **add missing `change`** |
| `internal/toml/testdata/schema.toml` | mirror the above |
| `internal/graph/propagation.go` | `PropagationTable` **and** `ReasonTemplates` rows for `has_task`, `task_depends_on` `[rev]` |
| `internal/graph/query.go` | `[rev]` extend `QueryScope`'s hardcoded `covers`/`delivers` predicate (L36) to include `has_task` |
| `internal/graph/export.go` | DOT shape + Mermaid bracket/class for `task` (falls back to `box`/plain quotes if omitted, so cosmetic-only) |
| `internal/validate/mapping.go` | `[rev]` **first** fix `checkDeliveryCompleteness` dead status filter (`"completed"` → `resolved`), then add task delivery |
| `internal/validate/exec.go` | new checks `task_cycles`, `orphan_tasks` |
| `internal/validate/satisfaction.go` | expand closure via `has_task`; add `satisfactionTargetStatusAllowlist[task] = {resolved}` `[rev]` |
| `internal/gate/policy.go` | phase→resolved: require tasks resolved; optional task→resolved policy — **register gate's task check as mapping-layer** (see §6.3 `[rev]`) |
| `internal/validate/types.go` | register new check names |
| `internal/bootstrap/*` | `[rev]` one-shot `.md` → task-entity importer (NOT `cli/migrate.go` — that's the legacy SQLite migrator) |
| `skills/spec-graph/SKILL.md` + `skills/spec-{planner,executor,verifier}/` | `[rev]` agent-facing catalog — enumerate `task`/`TSK` + new relations; this is the tool's public API for agents (README: agent-operated) |
| `internal/mcp/server.go` | `[rev]` review tool descriptions that enumerate entity types (e.g. L66 filter description) |
| tests | table-driven tests mirroring existing `*_test.go` per package; **add a drift-guard test** asserting `model.TypePrefixMap` keys == `DefaultSchema().EntityTypes` keys |

**`[rev]` No live SQL migration needed.** The live query index (`internal/index/schema.sql`) has no CHECK constraints on `type`/`layer`, so index rebuilds accept `task` freely. `internal/db`'s CHECK-constrained migrations are legacy (only the `migrate` command touches them) — do not add a migration there.

Estimated shape: predominantly additive map/table entries + a handful of DFS/switch checks copied from existing analogues + one bootstrap importer + two prerequisite bug fixes (`delivery_completeness` status, gate layer). No changes to the impact algorithm or export renderers themselves; `QueryScope` needs a one-line predicate extension.

---

## 9. Risks & Tradeoffs

- **Entity-count inflation.** A plan with many tasks multiplies node count. Mitigation: tasks are `exec`-layer and layer-filterable (`--layer exec`); exports/queries already support layer filtering.
- **Migration is one-way-sensitive.** See §7 — the `.md` deletion gate must be strictly last.
- **ID management.** `TSK-*` IDs must be allocated without collision; the existing `PREFIX-NNN` scheme and `entity add` ID handling apply unchanged.
- **Schema/model drift.** The `change` precedent shows model constants and `DefaultSchema()` can diverge. Mitigation: add both in the same change, and consider a test asserting `model.TypePrefixMap` keys == `DefaultSchema().EntityTypes` keys to prevent recurrence.
- **Scope creep into arch.** Tempting to also model per-task tests/criteria as arch entities. Out of scope; keep Level 2 to task nodes + the four relations.

---

## 10. Alternatives Considered

1. **Keep tasks in metadata (Level 1 only).** Rejected as the *complete-SSOT* answer: storage is safe but tasks stay invisible to every analytical feature. Acceptable only if task-level analysis is explicitly a non-goal.
2. **Reuse `belongs_to` (task→phase) instead of `has_task`.** `[rev]` The real difference is **propagation direction**, not just matrix bookkeeping. `belongs_to` is `Forward` = child→parent flow, so a widened `belongs_to (task→phase)` would make a **task change propagate up to its phase** ("task slipped → phase at risk"). `has_task (phase→task)` `Forward` flows the opposite way: a **phase change propagates down to tasks**, but a task change never bubbles up. These produce genuinely different impact semantics:
   - If you want "changing/slipping a task raises phase impact" → widen `belongs_to`, or make `has_task` `ForwardReverseWeak`.
   - If you only want "changing a phase flags its tasks" → plain `Forward` `has_task` suffices.
   The economical option (widen `belongs_to`) also gives the more useful default direction. Downside: `belongs_to` then spans two child types (plan-children and phase-children) with divergent matrices. This is now the key content of open question Q1 — decide the *direction* you want first, then pick the relation.
3. **Reuse arch `depends_on` for task ordering.** Rejected: pollutes the arch layer and its matrix with exec-layer semantics; muddies impact dimension profiles.
4. **A new `task` layer (4th layer).** Rejected: `validate()` hardcodes `{arch, exec, mapping}` and the whole engine assumes three layers; introducing a fourth is a far larger change for no clear benefit — tasks fit cleanly in `exec`.

---

## 11. Open Questions (decisions needed before implementation)

1. **Direction first, then relation:** do we want task changes to propagate *up* to their phase? If yes → widen `belongs_to (task→phase)` or make `has_task` `ForwardReverseWeak`. If no → plain `Forward` `has_task`. (§10.2 `[rev]`)
2. Should `task → resolved` be gated on `delivers` evidence, or is `has_task`-completeness at the phase gate sufficient?
3. Do per-task guardrails (`must` / `must_not`) stay as flat metadata strings, or become `criterion` (ACT) entities linked via `has_criterion`? (Recommend: strings now; entities later if we want them verifiable.)
4. ~~Fix the `change` schema drift in this same change set?~~ **Resolved: mandatory** — the schema fails to load otherwise once `covers.From` references new types (§5.3 `[rev]`).
5. ~~Migration command home?~~ **Resolved: `internal/bootstrap`** — `migrate.go` is the legacy SQLite migrator (§7 `[rev]`).
6. **`[rev]` New:** Do we accept the pre-existing limitation that `impact <arch-id>` never reaches exec (§6.1)? Delivering "`impact REQ-001` → affected tasks" requires flipping `covers`/`delivers` to `ForwardReverseWeak` — a behavior change to *all* existing arch-source impact output. In scope for Level 2, or a separate follow-up?

---

## Appendix A — Source Anchors

- Entity types: `internal/model/entity.go` L12–42
- Relation types: `internal/model/relation.go` L8–28
- Layer classification: `internal/model/layer.go` L21–60
- Edge matrix: `internal/model/edge_matrix.go` L11–95
- Default schema: `internal/toml/schema.go` `DefaultSchema()` L65–102; validation L106–234
- Impact engine: `internal/graph/impact.go` L63 (`Impact`), L259 (`computeScores`), L232–256 (`resolveNeighbor` — Forward direction gate); `propagation.go` L54–124 (`PropagationTable`), L104–111 (`covers`/`delivers` = Forward), L128–146 (`ReasonTemplates`)
- Impact options (no reverse flag): `internal/graph/types.go` L24–34
- Query scope hardcoded predicate: `internal/graph/query.go` L36 (`covers`/`delivers` only)
- Validation: `internal/validate/{arch.go, exec.go (L19–21 layer skip), mapping.go (L117 checkDeliveryCompleteness, L119–121 dead "completed" filter), satisfaction.go (L31–34 allowlist)}`
- Gates: `internal/gate/{gate.go L24, L46 hardcoded LayerMapping, policy.go L20–32, report.go}`
- Export: `internal/graph/export.go` L69 (DOT), L138 (Mermaid), L197 (JSON)
- Runtime schema use (DefaultSchema inline, LoadSchema unused): `pkg/specgraph/engine_entity.go` L193, L404; relation layer stamping: `engine_relation.go` L179
- Bootstrap importer precedent: `internal/bootstrap/{scan.go, engine_bootstrap.go L140}`
- Live index (no CHECK constraints): `internal/index/schema.sql`; legacy CHECK migrations: `internal/db/migrations/{003_layers.sql, 004_rename_relations.sql}`
- Agent-facing catalog: `skills/spec-graph/SKILL.md` (+ `spec-planner`/`spec-executor`/`spec-verifier`)
- Level 1 fix + tests: `internal/toml/writer.go` `formatValue`/`formatInlineArray`; `internal/toml/model_test.go` `TestEntityFileFrom_NestedMetadataRoundTrip`
</content>
</invoke>
