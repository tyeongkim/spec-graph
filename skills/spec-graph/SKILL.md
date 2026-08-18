---
name: spec-graph
description: >
  Use this skill whenever the project uses spec-graph for managing requirements,
  decisions, phases, changes, interfaces, states, tests, and other semantic entities in a
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
Contains the "when" and "how" of delivery: plans, phases, tasks, and changes. A plan groups phases into
a single active delivery sequence. Only one plan may be active at a time. A change is a lightweight
independent work unit (PR, bugfix, patch) that covers arch entities without belonging to any plan or phase.

Entity types: `plan`, `phase`, `task`, `change`

Relation types: `belongs_to` exactly for phase→plan and task→phase, `task_depends_on`
(dependent task→prerequisite task in the same phase), `precedes` (phase→phase), and
`blocks` (phase→phase).

Note: `change` entities do NOT participate in exec relations (belongs_to, precedes, blocks). They are independent units.

### mapping (cross-layer)
Connects arch entities to exec entities. This is where intent meets delivery.

Relation types: `covers` (phase/change/task→arch entity), `delivers` (phase/task→arch entity).
When a phase has any child task, task mappings are canonical and direct phase mappings are forbidden.

### Layer Classification

Layer is determined by entity type prefix. It is always deterministic:

| Prefix | Type | Layer |
|--------|------|-------|
| REQ | requirement | arch |
| DEC | decision | arch |
| API | interface | arch |
| STT | state | arch |
| TST | test | arch |
| XCT | crosscut | arch |
| ACT | criterion | arch |
| ASM | assumption | arch |
| RSK | risk | arch |
| QST | question | arch |
| PLN | plan | exec |
| PHS | phase | exec |
| TSK | task | exec |
| CHG | change | exec |

### Entity IDs

IDs are decentralized and sortable: `PREFIX-<unixSeconds>-<rand3>`, e.g.
`REQ-1752239482-k3f`. The unix-seconds segment makes IDs sort chronologically as
strings; the three-character random suffix (Crockford base32 lowercase, minus
`i`/`l`/`o`/`u`) prevents same-second collisions.

**Never invent or predict an ID.** There is no counter to continue and no next number
to guess. Always omit `--id` and read the assigned ID from the response:

```bash
REQ_ID=$(spec-graph entity add --type requirement \
  --title "All payments must be idempotent" \
  --metadata '{"priority":"must","kind":"non_functional"}' | jq -r '.entity.id')
```

Then reference `"$REQ_ID"` in later commands. For multi-entity flows, capture every ID
into a shell variable before wiring relations.

Why this matters: the old scheme was `MAX+1` per type, which collided whenever two
branches created entities in parallel — both produced `REQ-6` and clashed on merge.
The current scheme needs no central coordination, so concurrent and branch-parallel
creation is safe.

`--id` still exists and legacy `PREFIX-NNN` IDs remain valid (the validation regex
accepts both forms, so no migration is needed). Pass `--id` only when reproducing a
specific existing ID, such as restoring an entity or importing from an external
system. Do not use it to keep sequential numbering.

---

## Core Principles

0. **English only**: all spec-graph content — entity titles, descriptions, metadata, `--reason` messages, criteria text, and any other field written into the graph — MUST be in English. Regardless of the language used in conversation, never write non-English text into spec-graph entities or relations.
1. **Compute first**: never modify by guesswork. Always run `impact` and `validate` to identify targets before making changes.
2. **JSON contract**: command output is JSON on stdout (except `export --format dot|mermaid`). Parse it to decide the next action.
3. **Layer discipline**: arch entities belong in arch, exec entities in exec. Do not mix concerns.
4. **Phase gates**: always run `validate --layer mapping --phase` before starting or completing a phase.
5. **Git as audit log**: each commit is a logical changeset. The project's git history is the sole audit trail for spec-graph changes.
6. **covers/delivers**: use the v1 mapping relations. `covers` expresses planning intent, `delivers` expresses completion.
7. **Graph-native plans**: new plans create `TSK` entities and no Markdown plan files. Never auto-import,
   delete, or reinterpret old Markdown.
8. **Never guess IDs**: IDs are generated, not sequential. Omit `--id` on create and capture
   `.entity.id` from the response. Never construct an ID by incrementing a number you saw.

## Task Contract and Scope

Every task has a non-empty title and description plus a closed six-field metadata contract. Unknown
keys are rejected:

```json
{
  "order": 1,
  "instructions": ["Implement the scoped behavior."],
  "acceptance": ["The behavior is verified."],
  "must_not": [],
  "references": [],
  "qa": [{"command":"go test ./...","expected":"exit 0","evidence":""}]
}
```

- `order` is a positive integer.
- `instructions`, `acceptance`, and `qa` are non-empty arrays.
- `must_not` and `references` are required arrays and may be empty.
- Every QA item has non-empty `command` and `expected`. `evidence` is empty before resolution and
  must identify a repository-relative regular file when the task resolves.

Canonical scope is selected per phase: a task-managed phase (any task `belongs_to` it) derives
scope and delivery from the union of child task `covers`/`delivers`; a taskless phase keeps its
direct phase mappings unchanged. Never mix direct phase mappings with child-task mappings. Tasks
remain exec entities and are never members of the architecture satisfaction closure.

Task lifecycle is `draft → active → resolved` or `draft|active → deprecated`; `resolved` and
`deprecated` are terminal, and deprecation requires a reason. Activation requires an active parent
phase and resolved prerequisites. Resolution requires resolved prerequisites, QA evidence, and
`delivers` for every deliverable target the task covers.

---

## Storage Architecture

v0.3.0 uses TOML-first storage with SQLite as a disposable index.

- **Source of truth**: TOML files at `.spec-graph/entities/{type}/{id}.toml`
- **Relations**: embedded in entity TOML files (outbound only)
- **SQLite index**: `.spec-graph/graph.db`, disposable, auto-rebuilt from TOML on any command if stale
- **Staleness detection**: content-hash fingerprint per entity file
- **Gitignored**: `.spec-graph/graph.db*` and `.lock` are never committed

The SQLite index exists purely for fast queries (neighbors, impact, path). If deleted or corrupted, it rebuilds automatically. Never treat it as authoritative.

## TOML File Format

Canonical entity file at `.spec-graph/entities/requirement/REQ-1752239482-k3f.toml`:

```toml
schema = 1
id = "REQ-1752239482-k3f"
type = "requirement"
title = "User authentication"
description = "All APIs require OAuth2"
status = "active"
created_at = 2026-05-23T17:00:00+09:00
updated_at = 2026-05-23T17:30:00+09:00

[metadata]
priority = "must"
kind = "non_functional"

[[relations]]
to = "ACT-1752239485-q7m"
type = "has_criterion"

[[relations]]
to = "DEC-1752239490-b2x"
type = "constrained_by"
weight = 0.8
```

Fields: `schema` (always 1), `id`, `type`, `title`, `description` (optional), `status`,
`created_at`, `updated_at`, `[metadata]` (type-specific), `[[relations]]` (outbound edges).

## Git Workflow

TOML files are designed for git-friendly collaboration:

- Each entity is a separate file, so merge conflicts are entity-scoped
- After `git merge` or `git pull` with conflicts, resolve TOML files then run `spec-graph doctor`
- SQLite index is never committed (listed in `.gitignore`)
- Commit messages serve as the audit log. Git history is the sole mechanism for tracking entity changes over time.

### Audit Trail

spec-graph does NOT maintain its own history. The project's git history is the sole audit trail.

- Entity changes: `git log -- .spec-graph/entities/{type}/{id}.toml`
- Relation changes: tracked via the owning entity's file history
- Phase transitions: `git log -- .spec-graph/entities/phase/PHS-XXX.toml`

**Recommendation**: Commit `.spec-graph/` changes after each logical unit of work (phase activation, delivers batch, entity registration).

---

## Quick Reference: CLI Commands

See `references/cli-reference.md` for full options.

### Project Init
```bash
spec-graph init
spec-graph init --path /custom/path
```

Do not duplicate `.spec-graph/.gitignore` entries in the root `.gitignore`.

### Entity CRUD
```bash
spec-graph entity add --type <TYPE> --title "..." [--description "..."] [--metadata '{}'] [--metadata-file <PATH>]
# Omit --id. The ID is generated; capture it with jq -r '.entity.id'.
spec-graph entity get <ID>
spec-graph entity list --type <TYPE> [--status <STATUS>] [--layer arch|exec|mapping|all]
spec-graph entity update <ID> --title "..."
spec-graph entity update <ID> --status resolved [--force] [--reason "..."]
spec-graph entity revise <ID> --reason "..." [--title "..."] [--description "..."] [--metadata '{}']
spec-graph entity deprecate <ID> [--reason "..."]
spec-graph entity delete <ID>
spec-graph entity import --input <PATH>
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
spec-graph query unresolved --type question|assumption|risk [--phase <PHS-ID>]
spec-graph query sql "SELECT ..."
```

### Phase Lifecycle
```bash
spec-graph phase next [--activate]
spec-graph phase context <PHS-ID>
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

### Migration & Integrity
```bash
# One-shot migration from old SQLite-only format
spec-graph migrate [--dry-run] [--keep-db]

# Integrity validation (run after git merge/pull)
spec-graph doctor [--check <name>] [--fix]
```

---

## Entity & Relation Quick Reference

See `references/data-model.md` for full type catalog, metadata schemas, and edge matrices.

### Entity Types (14)

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
| TSK | task | exec | graph-native unit of implementation work |
| CHG | change | exec | lightweight work unit (PR, bugfix, patch) |

### Entity Status: `draft` → `active` → `deprecated` / `resolved` / `deleted`

**Auto-activation (v0.4.0+):** When a `delivers` relation is added targeting a **draft** arch
entity, the CLI transitions it to `active`. The source may be a phase or a task. Entities in any
other status are left alone. This means:
- `draft` = registered, no delivery evidence yet
- `active` = either set explicitly (on create with `--status active`, or via update), or
  auto-promoted from `draft` by a `delivers` edge
- `resolved` = verified complete (only spec-verifier should set this)

Do NOT manually transition arch entities to `active` after adding `delivers` — the CLI
handles it. Do NOT expect `delivers` to auto-resolve entities; resolution requires
explicit verification.

**Gated transitions (v0.3.1+):** Transitioning a phase or plan to `resolved` is gated.
The CLI automatically runs `delivery_completeness` + `gates` checks (for phases) or
`plan_coverage` (for plans). If issues are found, the transition is blocked (exit 2).
Completion findings can be accepted with `--force --reason "..."`; structural findings
cannot be bypassed. A successful forced completion emits `outcome = "applied_with_force"`,
reports the accepted findings, and records top-level `completion_forced = true` plus
`completion_reason = "..."` in the entity TOML. A blocked transition emits
`outcome = "blocked"` and exits 2. Phase gate checks evaluate only the target phase,
its child tasks, and their effective mapping scope; unscoped `validate` remains graph-wide.

### PLN / PHS Lifecycle

#### Status State Machine

```
PLN:  draft → active → resolved (gated: plan_coverage)
                     → deprecated

PHS:  draft → active (gated: predecessors resolved)
                     → resolved (gated: delivery_completeness + gates)
                     → deprecated
```

#### Transition Ownership

| Transition | Owner | Precondition |
|------------|-------|--------------|
| PLN: draft → active | spec-planner | Only one active plan allowed |
| PHS: draft → active | spec-executor | Predecessor phases resolved (blocking) |
| PHS: active → resolved | spec-verifier | All deliverables verified, gate passes |
| Any → deprecated | User (manual) | Reason recommended; no `--force` needed |

#### Rules

1. **Only one active PLN** at a time — `single_active_plan` check enforces this.
2. **PHS activation order is enforced**: activating a phase whose `precedes` predecessors are not
   yet resolved is **blocked** (`outcome: blocked`), not merely warned. Activate in order, or use
   `phase next --activate`, which only selects phases whose predecessors are resolved.
3. **PHS resolution is gated**: `entity update <PHS-ID> --status resolved` auto-runs
   `delivery_completeness` + `gates`. Blocked (exit 2) if issues exist.
4. **PLN resolution is gated**: requires `plan_coverage` — all active arch entities must be covered.
5. **No skipping states**: `draft → resolved` is invalid. Must pass through `active`.
6. **deprecated is terminal**: no transitions out of `deprecated`. Deprecating does not require
   `--force`; `entity deprecate <ID> --reason "..."` is enough. Task deprecation requires a reason.

### Relation Types (18)

**Architecture layer (12):**
`implements`, `verifies`, `depends_on`, `constrained_by`, `triggers`, `answers`,
`assumes`, `has_criterion`, `mitigates`, `supersedes`, `conflicts_with`, `references`

**Execution layer (4):**
`belongs_to`, `task_depends_on`, `precedes`, `blocks`

**Mapping layer (2):**
`covers`, `delivers`

---

## Agent Workflow Patterns

This section is the heart of this skill. Agents follow these patterns.

### Pattern 1: Plan and Phase Setup

Create a graph-native plan, add phases and tasks, then wire task scope. This path creates no
Markdown. Direct phase mappings shown in older projects are a legacy taskless path only.

> **Note**: IDs are generated, so a cross-referencing flow like this must capture each ID into a
> shell variable as it creates the entity. Do not pass `--id`, and do not assume the plan is
> `PLN-001`. `$REQ_AUTH` and `$REQ_SESSION` below are requirement IDs captured the same way when
> those requirements were created.

```bash
# 1. Create the plan (only one active plan allowed)
#    --status active sets the entity status; a "status" key inside --metadata does NOT.
PLN=$(spec-graph entity add --type plan \
  --title "v1 Delivery Plan" \
  --status active | jq -r '.entity.id')

# 2. Create a phase
PHS=$(spec-graph entity add --type phase \
  --title "Phase 1 - Auth" \
  --metadata '{"goal":"Build authentication","order":1,"exit_criteria":["Auth API complete","E2E tests pass"]}' | jq -r '.entity.id')

# 3. Assign the phase to the plan
spec-graph relation add --from "$PHS" --to "$PLN" --type belongs_to

# 4. Create tasks with the closed TaskContract
TSK1=$(spec-graph entity add --type task --title "Implement auth API" \
  --description "Implement the authentication API and tests." \
  --metadata '{"order":1,"instructions":["Implement the auth API."],"acceptance":["Auth tests pass."],"must_not":[],"references":[],"qa":[{"command":"go test ./...","expected":"exit 0","evidence":""}]}' | jq -r '.entity.id')
TSK2=$(spec-graph entity add --type task --title "Integrate auth flow" \
  --description "Integrate the completed authentication API." \
  --metadata '{"order":2,"instructions":["Integrate the auth flow."],"acceptance":["Integration tests pass."],"must_not":[],"references":[],"qa":[{"command":"go test ./...","expected":"exit 0","evidence":""}]}' | jq -r '.entity.id')

# 5. Wire membership, dependency, and canonical task scope
spec-graph relation add --from "$TSK1" --to "$PHS" --type belongs_to
spec-graph relation add --from "$TSK2" --to "$PHS" --type belongs_to
spec-graph relation add --from "$TSK2" --to "$TSK1" --type task_depends_on
spec-graph relation add --from "$TSK1" --to "$REQ_AUTH" --type covers
spec-graph relation add --from "$TSK2" --to "$REQ_SESSION" --type covers

# 6. Validate and obtain the executor/verifier contract
spec-graph validate --layer exec --check task_graph
spec-graph validate --layer mapping --phase "$PHS" --check task_scope
spec-graph phase context "$PHS"
```

If you are working across separate command invocations rather than one script, recover IDs by
querying instead of guessing:

```bash
spec-graph entity list --type phase --status active | jq -r '.entities[].id'
spec-graph query sql "SELECT id, title FROM entities WHERE type = 'task' ORDER BY id"
```

`phase context` returns `{plan,phase,tasks,scope,delivery,blockers,ready_task_ids,blocked_task_ids}`.
Each task entry is `{entity,contract,prerequisite_ids,covers,delivers}`. The same result is available
as RPC `phase.context` and MCP `phase_context`.

### Pattern 2: Change Handling

When an existing entity changes, always run impact first:

```bash
# 1. Compute impact — what else must change
spec-graph impact "$DEC_ID" --dimension behavioral

# 2. Inspect affected targets (parse JSON)
spec-graph impact "$DEC_ID" | jq '.affected[] | {id, type, impact, reason}'

# 3. Check unresolved items
spec-graph query unresolved --type question

# 4. Modify only affected targets (do not touch unrelated entities)
spec-graph entity update "$DEC_ID" --title "New decision"

# 5. Full validation
spec-graph validate
```

Never modify related entities by guesswork without running impact first.

### Pattern 3: Phase Exit

Phase completion is gated by the CLI (v0.3.1+). Running `entity update --status resolved`
automatically enforces `delivery_completeness` + `gates` checks. If issues exist, the
transition is blocked with exit code 2.

```bash
# Direct completion attempt — gate runs automatically
spec-graph entity update "$PHS_ID" --status resolved

# If blocked, resolve issues first:
# 1. Review graph-native phase context
spec-graph phase context "$PHS_ID"

# 2. Check what's missing
spec-graph validate --layer mapping --phase "$PHS_ID" --check delivery_completeness
spec-graph validate --layer mapping --phase "$PHS_ID" --check gates

# 3. Fix issues (add delivers, answer questions, mitigate risks)
spec-graph relation add --from "$TSK_ID" --to "$REQ_ID" --type delivers

# 4. Retry
spec-graph entity update "$PHS_ID" --status resolved

# Force completion findings only (structural findings remain blocked)
spec-graph entity update "$PHS_ID" --status resolved --force \
  --reason "Accept the documented completion risk"
```

Update responses expose one of three outcomes: `applied`, `applied_with_force`, or
`blocked`. `blocked` always means the TOML was left unchanged; a non-nil gate report
does not by itself mean the update was blocked.

**Pre-flight checks (optional, for visibility before attempting completion):**

```bash
# Review scope
spec-graph query scope "$PHS_ID"

# Arch coverage
spec-graph validate --layer arch --check coverage

# Mapping consistency
spec-graph validate --layer mapping --phase "$PHS_ID" --check mapping_consistency

# Exec ordering
spec-graph validate --layer exec --check phase_order
```

If validate reports issues, resolve them before attempting `--status resolved`.

#### Handling "covered but not delivered" mapping failures

When `delivery_completeness` reports a covered arch entity has no `delivers` relation, the fix is
to deliver **that exact entity**. The check compares covered IDs against delivered IDs directly —
it does not traverse `implements`/`verifies`. Delivering an implementing interface or test does
**not** satisfy a covered requirement.

```bash
# 1. Identify what the phase covers vs delivers
spec-graph query scope "$PHS_ID"

# 2. Add delivers for the covered entity that lacks it.
#    In a task-managed phase, delivers must come from the child task that covers it,
#    not from the phase.
spec-graph relation add --from "$TSK_ID" --to "$REQ_ID" --type delivers

# 3. Re-validate
spec-graph validate --layer mapping --phase "$PHS_ID" --check delivery_completeness
```

Rules:
- Deliver the covered entity itself. There is no proxy resolution — a "minimal proxy set" of
  implementing entities will not clear the finding.
- If a phase covers something it did not actually deliver, the honest fix is to narrow the
  `covers` scope, not to add a `delivers` edge that misstates what happened.
- Only entity types that may legally receive `delivers` from a phase are checked; others in scope
  are skipped by the edge matrix.
- Before adding any `delivers`, ask whether it accurately represents work completed in this phase,
  or is just silencing the check.

### Pattern 4: Full Patch Orchestration (recommended)

The safest change-handling flow:

```
1. Identify change target
2. spec-graph impact → compute affected set
3. spec-graph validate → check currently broken rules
4. Modify only affected targets (entity update, relation add/delete, etc.)
5. Semantic review → does each added relation accurately represent the intended meaning?
6. spec-graph validate → re-verify after modifications
```

The agent modifies only entities in the `affected` list from step 2.
If an entity outside the list needs modification, first run `query neighbors` to verify the relationship.

Step 5 (semantic review) is critical. Before re-validating, review every relation you added
and ask: "Does this relation reflect a real semantic relationship, or am I adding it to pass a check?"
Check passage alone does not prove graph correctness. A graph that passes all checks but contains
over-broad relations is worse than one that fails a check with an honest gap.

### Pattern 5: Adding a Requirement

Typical flow for adding a new requirement and wiring it into the graph. `$PHS` and `$XCT` are
existing IDs obtained from `entity list` or a prior capture.

```bash
# 1. Create requirement, capturing the generated ID
REQ=$(spec-graph entity add --type requirement \
  --title "All payments must be idempotent" \
  --metadata '{"priority":"must","kind":"non_functional","owner":"payment-team"}' | jq -r '.entity.id')

# 2. Attach acceptance criterion
ACT=$(spec-graph entity add --type criterion \
  --title "Duplicate request within window processed only once" \
  --metadata '{"given":"Payment request already sent","when":"Same request resent","then":"No duplicate processing; return existing result"}' | jq -r '.entity.id')
spec-graph relation add --from "$REQ" --to "$ACT" --type has_criterion

# 3. Map to phase using covers (not planned_in)
spec-graph relation add --from "$PHS" --to "$REQ" --type covers

# 4. Link crosscut constraint (if applicable)
spec-graph relation add --from "$REQ" --to "$XCT" --type constrained_by

# 5. Validate arch layer
spec-graph validate --layer arch --entity "$REQ"
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

### Pattern 7: Revising an Arch Entity

When an arch entity's meaning changes and the prior wording must stay on the record, use
`entity revise` rather than editing in place. `entity update` rewrites history; `revise`
preserves it as a chain.

```bash
# Compute impact first — revision moves relations
spec-graph impact "$REQ" --dimension behavioral

spec-graph entity revise "$REQ" \
  --title "All payments must be idempotent within a 24h window" \
  --reason "Original requirement left the dedup window unspecified"
```

This is one atomic operation that:
1. creates a new entity with a newly generated ID, in `draft` status
2. carries the prior entity's outbound relations onto the revision
3. adds `revision supersedes prior`, recording `--reason` in the edge metadata
4. repoints inbound relations onto the revision
5. deprecates the prior entity

Capture the new ID from the response — the revision has a different ID than the original:

```bash
NEW=$(spec-graph entity revise "$REQ" --reason "..." | jq -r '.revision.id')
```

The response is `{revision, superseded, carried_relations, retained_relations}`.
`carried_relations` lists inbound relations moved onto the revision; `retained_relations` lists
those deliberately left on the superseded entity. Both are `null` rather than `[]` when empty.

**Which relations stay behind**: mapping relations (`covers`/`delivers`) from a *resolved* phase
or task are retained, not moved. A resolved phase delivered the wording that existed at the time,
so crediting it with delivering a later revision would be false. Mapping relations from
non-resolved phases and tasks move forward with everything else. `mapping_consistency` exempts
resolved sources, so these retained edges do not produce findings.

Constraints:
- arch entities only. Revising a `PLN`/`PHS`/`TSK`/`CHG` fails with exit 3 — exec entities do not
  form revision chains.
- `--reason` is required (exit 3 when omitted).
- an entity that is already superseded cannot be revised again (`CONFLICT`, exit 2). Revise the
  latest revision instead; the error message names it.
- a `deprecated` entity cannot be revised — chains start from a live entity.
- `--title`, `--description`, and `--metadata` are each optional and carry the prior value
  forward when omitted. Supplying `--metadata` replaces the metadata object wholesale.
- the revision is created in `draft`, so it needs the usual `covers`/`delivers` wiring to be
  delivered by a phase.

`revise` is CLI-only. It is not exposed over RPC or MCP.

---

## Validation Checks Guide

When to use each check. See `references/validation-rules.md` for detailed rules.

### Architecture Layer Checks (`--layer arch`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `orphans` | isolated arch entities with no relations | periodic cleanup, before phase start |
| `coverage` | missing implementations / tests | required before phase exit |
| `cycles` | circular references in depends_on chains | after adding relations |
| `conflicts` | semantic conflicts between entities | after changes |
| `invalid_edges` | arch edge matrix violations | after adding relations |
| `superseded_refs` | active refs to deprecated entities | after deprecation |
| `unresolved` | open questions, unverified assumptions, unmitigated risks | before phase start |

### Execution Layer Checks (`--layer exec`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `phase_order` | phases with precedes/blocks form a valid sequence | after adding exec relations |
| `single_active_plan` | only one plan is active | after plan creation or status change |
| `orphan_phases` | phases not belonging to any plan | after adding phases |
| `exec_cycles` | circular precedes/blocks chains | after adding exec relations |
| `invalid_exec_edges` | exec edge matrix violations | after adding exec relations |
| `orphan_changes` | changes with no relations to other entities | after adding changes |
| `task_graph` | task parents, same-phase dependencies, and cycles | after changing tasks or dependencies |

### Mapping Layer Checks (`--layer mapping`)

| Check | Purpose | When to Run |
|-------|---------|-------------|
| `plan_coverage` | all active requirements are covered by some phase | before phase start |
| `delivery_completeness` | covered arch entities have delivery evidence | auto-enforced on phase → resolved |
| `mapping_consistency` | covers/delivers targets exist and are arch entities | after adding mapping relations |
| `invalid_mapping_edges` | mapping edge matrix violations | after adding mapping relations |
| `gates` | unresolved questions, unmitigated risks, draft decisions | auto-enforced on phase → resolved |
| `task_scope` | task coverage, delivery subset, and no mixed mappings | after changing task mappings |

For task-managed completion, four checks/gates matter: structural `task_graph`, mapping
`task_scope`, task completion delivery/evidence gates, and phase child-resolution plus existing
`delivery_completeness`/`gates` checks.

### Common Combinations

```bash
# Before phase start
spec-graph validate --layer exec --check single_active_plan
spec-graph validate --layer exec --check phase_order
spec-graph validate --layer arch --check unresolved
spec-graph validate --layer mapping --check plan_coverage

# Before phase completion (now auto-enforced by entity update --status resolved)
# These are still useful for pre-flight visibility:
spec-graph validate --layer arch --check coverage
spec-graph validate --layer mapping --phase "$PHS_ID" --check delivery_completeness
spec-graph validate --layer mapping --phase "$PHS_ID" --check gates

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
      "id": "API-1752239500-t7n",
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
| 2 | validation failure / gate blocked | resolve issues from output, or use --force |
| 3 | invalid input | check arguments / schema, retry |

---

## Caveats

- `bootstrap import` defaults to `--mode review`. Never use `--mode auto`.
- `supersedes` requires both entities to be the same type. It is directional: stored in the `from` entity's file, so `revision supersedes prior` means the `from` entity is newer. Prefer `entity revise` over adding this edge by hand — it wires the edge, moves relations, and deprecates the prior entity in one operation.
- `conflicts_with` does not allow self-loops. It is symmetric: stored in the lexicographically smaller entity's file. Both directions are queryable via the index.
- Adding a relation that violates the allowed edge matrix fails with exit code 3.
  On failure, consult the edge matrix in `references/data-model.md`.
- `metadata` is a JSON string. Each type has required fields — see `references/data-model.md`.
- `--phase` is only valid with `--layer mapping` or `--layer all`. Using `--phase` with
  `--layer arch` or `--layer exec` returns an error.
- Only one plan may have `active` status at a time. The `single_active_plan` exec check
  enforces this.
- Entity timestamps (`created_at`, `updated_at`) are stored in TOML and populated automatically on create/update.
- After `git merge` with conflicts in TOML files, run `spec-graph doctor` to validate integrity.
- The SQLite index is rebuilt automatically on each command if TOML files changed. No manual sync needed.
- Generated IDs are `PREFIX-<unixSeconds>-<rand3>`. Collisions are avoided by a crypto-random suffix plus an existence retry, so parallel branches are safe in practice. Never predict one; read it from `.entity.id`. Legacy `PREFIX-NNN` IDs remain valid — both forms pass validation and no migration is required.

## Anti-Patterns

These are known failure modes. If you catch yourself doing any of these, stop and reconsider.

### 1. Inventing or sequencing entity IDs
**Symptom**: passing `--id REQ-004` because `REQ-003` exists, or referencing `PHS-001` in a
relation command without ever reading that ID from a response.
**Why it's wrong**: IDs are generated as `PREFIX-<unixSeconds>-<rand3>`. There is no counter to
continue. A guessed ID either fails as not-found or, worse, silently attaches a relation to an
unrelated entity.
**Correct approach**: omit `--id`, capture `.entity.id` into a variable, and reference the
variable. Recover forgotten IDs with `entity list` or `query sql`, never by guessing.

### 2. Mixing arch and exec concerns
**Symptom**: adding a requirement directly to a phase using arch-only relations,
or treating a phase as an arch entity by linking it with arch-only relations.
**Why it's wrong**: arch and exec are separate layers with separate edge matrices. Cross-layer
connections belong in the mapping layer using `covers` and `delivers`.
**Correct approach**: use `covers` (phase→arch) to express intent, `delivers` (phase→arch)
to express completion.

### 3. Editing SQLite directly
**Symptom**: modifying `.spec-graph/graph.db` manually or treating it as the source of truth.
**Why it's wrong**: the SQLite index is disposable and auto-rebuilt from TOML. Any manual
edits are lost on the next rebuild.
**Correct approach**: always use CLI commands to modify entities. The TOML files are the source of truth.

### 4. Rewriting an arch entity that should be revised
**Symptom**: using `entity update --title/--description` to change what a requirement or decision
means, discarding the prior wording.
**Why it's wrong**: `update` leaves no trace of the superseded meaning, so resolved phases appear
to have delivered text that never existed when they closed.
**Correct approach**: use `entity revise --reason "..."` for meaning changes. Reserve `update` for
corrections that do not change intent, and for status transitions.

### 5. Check-driven patching
**Symptom**: check fails → add relations broadly until check passes → commit.
**Why it's wrong**: passing a check does not mean the graph is correct. Over-broad relations
pollute the graph and produce inaccurate impact analysis downstream.
**Correct approach**: diagnose why the check fails, compute the minimal fix, verify semantic
accuracy, then re-validate.

### 6. Bulk delivers expansion
**Symptom**: a requirement is "covered but not delivered" → add `delivers` for every
related interface, state, and test to the phase.
**Why it's wrong**: it does not even work — `delivery_completeness` matches covered IDs against
delivered IDs exactly, so delivering implementing entities never clears a finding on the
requirement. It only pollutes the graph and skews impact analysis.
**Correct approach**: add `delivers` for the covered entity itself, from the task that covers it in
a task-managed phase. If the phase did not actually deliver it, narrow the `covers` scope instead.

### 7. Semantic ambiguity bypass
**Symptom**: discover a model-level conflict (e.g. edge matrix prevents a relation type
the check seems to require) → work around it by expanding other relations instead of
investigating the conflict.
**Why it's wrong**: the conflict is a signal that either (a) the graph model needs revision,
(b) the validator semantics need clarification, or (c) the agent's understanding is incomplete.
**Correct approach**: when you encounter a semantic conflict between edge matrix constraints
and validator expectations, stop and investigate. Check `references/data-model.md` for the
intended semantics. If the conflict is genuine, report it to the user rather than working around it.

### 8. Inconsistent precision across phases
**Symptom**: Phase N uses broad relation additions, Phase N+1 uses precise minimal additions.
**Why it's wrong**: the same rules must apply uniformly. If Phase 3 adds only 3 delivery
proxies, Phase 2 should not have added 15 for a similar scope.
**Correct approach**: establish the precision standard on the first phase, then apply it
consistently to all subsequent phases.
