# CLI Reference

## Table of Contents
1. [General Rules](#general-rules)
2. [--layer Flag](#--layer-flag)
3. [init](#init)
4. [entity](#entity)
5. [relation](#relation)
6. [impact](#impact)
7. [validate](#validate)
8. [query](#query)
9. [export](#export)
10. [phase](#phase)
11. [bootstrap](#bootstrap)
12. [migrate](#migrate)
13. [doctor](#doctor)
14. [serve / mcp](#serve--mcp)

---

## General Rules

- Command output is **JSON on stdout** — machine-parseable, not human prose. Exceptions:
  `export --format dot|mermaid` emits the raw diagram format, and `serve` speaks JSON-RPC.
- Error messages go to **stderr**.
- Exit codes: `0` success, `1` runtime error, `2` validation failure / gate blocked, `3` invalid input / schema violation.
- Root persistent flags: `--layer` and `--db <PATH>`.

---

## --layer Flag

`--layer` is a **persistent flag** available on all commands. It filters results and operations
to a specific layer of the graph.

```bash
spec-graph <command> --layer arch     # architecture layer only
spec-graph <command> --layer exec     # execution layer only
spec-graph <command> --layer mapping  # mapping layer only
spec-graph <command> --layer all      # no filter (default)
```

**Values**: `arch`, `exec`, `mapping`, `all`
**Default**: `all`

### Layer Semantics per Command

| Command | Effect of --layer |
|---------|-------------------|
| `entity list` | returns only entities belonging to the specified layer |
| `relation list` | returns only relations classified in the specified layer |
| `validate` | runs only checks belonging to the specified layer |
| `impact` | restricts traversal to relations in the specified layer |
| `export` | exports only entities/relations in the specified layer |
| `query neighbors` | traverses only relations in the specified layer |

### --phase + --layer Interaction

`--phase` is only valid with `--layer mapping` or `--layer all`. Using `--phase` with
`--layer arch` or `--layer exec` returns exit code 3 (invalid input).

```bash
# Valid
spec-graph validate --phase "$PHS_ID" --layer mapping
spec-graph validate --phase "$PHS_ID" --layer all
spec-graph validate --phase "$PHS_ID"   # --layer all is the default

# Invalid — returns exit 3
spec-graph validate --phase "$PHS_ID" --layer arch
spec-graph validate --phase "$PHS_ID" --layer exec
```

---

## init

Initializes a `.spec-graph/` directory and SQLite database for the project.

```bash
spec-graph init                    # current directory
spec-graph init --path /other/dir  # custom path
```

Fails with exit 1 only on a real filesystem/store error. Re-running in an initialized directory is
idempotent and succeeds.

---

## entity

### entity add

```bash
spec-graph entity add \
  --type <TYPE> \
  --title "Title" \
  [--description "Description"] \
  [--status draft|active] \
  [--metadata '{"key":"value"}'] \
  [--metadata-file <PATH>] \
  [--id <ID>]
```

- `--type`: requirement, decision, plan, phase, task, change, interface, state, test, crosscut, question, assumption, criterion, risk
- `--id`: **omit this.** IDs are generated as `PREFIX-<unixSeconds>-<rand3>` (e.g. `REQ-1752239482-k3f`) and returned in `.entity.id`. There is no counter, so there is no next number to supply. Pass `--id` only to reproduce a specific pre-existing ID, such as restoring a deleted entity or importing from an external system. A supplied ID is validated: the prefix must match `--type`, and both the new form and legacy `PREFIX-NNN` are accepted.
- `--status`: defaults to `draft`. Tasks must be created in `draft`.
- `--metadata`: type-specific required fields — see `references/data-model.md`
- `--metadata-file`: read metadata JSON from a file; mutually exclusive with `--metadata`

**Examples**:
```bash
# Always capture the generated ID
REQ_ID=$(spec-graph entity add --type requirement \
  --title "All APIs require authentication" \
  --description "No anonymous access allowed" \
  --metadata '{"priority":"must","kind":"functional","owner":"auth-team"}' | jq -r '.entity.id')
# → REQ_ID is e.g. REQ-1752239482-k3f

# Reference the captured ID in later commands
spec-graph relation add --from "$PHS_ID" --to "$REQ_ID" --type covers

# Multi-entity flow: capture each ID as it is created
# Note: --status sets entity status; a "status" key in --metadata does not.
PLN_ID=$(spec-graph entity add --type plan \
  --title "v1 Delivery Plan" \
  --status active | jq -r '.entity.id')

PHS_ID=$(spec-graph entity add --type phase \
  --title "Phase 1 - Auth" \
  --metadata '{"goal":"Build authentication","order":1,"exit_criteria":["Auth API complete"]}' | jq -r '.entity.id')

spec-graph relation add --from "$PHS_ID" --to "$PLN_ID" --type belongs_to
```

Recovering IDs in a later session:
```bash
spec-graph entity list --type phase --status active | jq -r '.entities[] | "\(.id)\t\(.title)"'
```

### entity get

```bash
spec-graph entity get <ID>
```

Returns full entity details as JSON, including the `layer` field.

### entity list

```bash
spec-graph entity list [--type <TYPE>] [--status <STATUS>] [--layer arch|exec|mapping|all]
```

Returns entities matching the filter. Omitting all flags returns all entities.

```bash
spec-graph entity list --layer arch           # all arch entities
spec-graph entity list --layer exec           # plans and phases only
spec-graph entity list --layer exec --status active
spec-graph entity list --type requirement
```

### entity update

```bash
spec-graph entity update <ID> \
  [--title "New title"] \
  [--description "New description"] \
  [--status <STATUS>] \
  [--metadata '{}'] \
  [--metadata-file <PATH>] \
  [--force] [--reason "..."]
```

Transitions to `resolved` on a phase or plan are gated. The response carries an `outcome` of
`applied`, `applied_with_force`, or `blocked`; `blocked` exits 2 and leaves the TOML unchanged.
`--force --reason "..."` accepts completion findings but cannot bypass structural ones.

To deprecate, pass `--status deprecated`. Run `validate --check superseded_refs` afterward to clean
up references. Task deprecation requires `--reason`; elsewhere `--reason` is optional and is only
persisted (as `completion_reason`) when it accompanies a `--force` bypass, so a reason given on an
arch deprecation is accepted but not recorded. `question` entities do not accept `deprecated`
(INVALID_INPUT, exit 3) — resolve them instead.

For a change in what an arch entity *means*, use `entity revise` instead — `update` discards the
prior wording. To deprecate *and* replace an arch entity, `entity revise` does both atomically.

### entity revise

```bash
spec-graph entity revise <ID> \
  --reason "why the entity was revised" \
  [--title "New title"] \
  [--description "New description"] \
  [--metadata '{}'] \
  [--metadata-file <PATH>]
```

Supersedes an arch entity with a new revision in one atomic operation: creates the revision with a
newly generated ID in `draft`, carries the prior outbound relations, adds a `supersedes` edge whose
metadata records `--reason`, repoints inbound relations, and deprecates the prior entity.

`--title`, `--description`, and `--metadata` carry the prior value forward when omitted.
`--metadata` replaces the metadata object wholesale.

Response:
```json
{
  "revision":   {"id": "REQ-1752240001-9dm", "status": "draft",      "...": "..."},
  "superseded": {"id": "REQ-1752239482-k3f", "status": "deprecated", "...": "..."},
  "carried_relations":  [{"from_id": "API-1752239500-t7n", "type": "implements"}],
  "retained_relations": [{"from_id": "PHS-1752239600-5xq", "type": "covers"}]
}
```

`carried_relations` moved onto the revision. `retained_relations` stayed on the superseded entity:
mapping edges (`covers`/`delivers`) from a **resolved** phase or task are left behind, because a
completed phase delivered the prior wording, not the revision. Both fields are `null` when empty.
`mapping_consistency` exempts resolved sources, so retained edges produce no findings.

Capture the new ID — it differs from the original:
```bash
NEW_ID=$(spec-graph entity revise "$REQ_ID" --reason "Dedup window was unspecified" | jq -r '.revision.id')
```

Errors:

| Condition | Code | Exit |
|-----------|------|------|
| `--reason` omitted | INVALID_INPUT | 3 |
| target is an exec entity (PLN/PHS/TSK/CHG) | INVALID_INPUT | 3 |
| target already superseded (message names the successor) | CONFLICT | 2 |
| target is `deprecated` | INVALID_INPUT | 3 |
| target does not exist | ENTITY_NOT_FOUND | 1 |

CLI-only; not exposed over RPC or MCP.

### entity import

```bash
spec-graph entity import --input <PATH>
```

Bulk-creates entities from a JSON array in one transaction and one index refresh. Each item requires
`id`, `type`, and `title`; `description`, `status`, and `metadata` are optional. Because `id` is
required here, this command is for round-tripping known IDs, not for creating new entities. Existing
IDs are skipped rather than overwritten, keeping a re-run of the same input idempotent. No other
process can interleave items.

Response:
```json
{
  "created": ["REQ-1752239482-k3f"],
  "skipped": [
    {"id": "DEC-1752239490-b2x", "reason": "already exists"}
  ]
}
```

Any non-duplicate failure, including an unknown entity type, malformed entity ID, missing required
field, or write failure, aborts the entire import without writing and exits non-zero.

### entity delete

```bash
spec-graph entity delete <ID>
```

Deletes the entity and all connected relations.

---

## relation

### relation add

```bash
spec-graph relation add \
  --from <FROM-ID> \
  --to <TO-ID> \
  --type <RELATION_TYPE> \
  [--weight <FLOAT>] \
  [--metadata '{}']
```

Automatic validations on add:
1. from/to entity existence
2. relation type validity
3. **allowed edge matrix** compliance for the relation's layer (violation → exit 3)
4. duplicate edge check
5. self-loop prohibition (supersedes, conflicts_with, etc.)

**Examples** (all IDs are shell variables captured from `entity add`):
```bash
# Arch relations
spec-graph relation add --from "$API_ID" --to "$REQ_ID" --type implements
spec-graph relation add --from "$TST_ID" --to "$REQ_ID" --type verifies

# Exec relations
spec-graph relation add --from "$PHS_ID" --to "$PLN_ID" --type belongs_to
spec-graph relation add --from "$TSK1_ID" --to "$PHS_ID" --type belongs_to
spec-graph relation add --from "$TSK2_ID" --to "$TSK1_ID" --type task_depends_on
spec-graph relation add --from "$PHS1_ID" --to "$PHS2_ID" --type precedes

# Mapping relations (v1 — use these)
spec-graph relation add --from "$PHS_ID" --to "$REQ_ID" --type covers
spec-graph relation add --from "$PHS_ID" --to "$API_ID" --type delivers
spec-graph relation add --from "$TSK1_ID" --to "$REQ_ID" --type covers
spec-graph relation add --from "$TSK1_ID" --to "$API_ID" --type delivers
```

### relation list

```bash
spec-graph relation list --from <ID>                   # outgoing relations for an entity
spec-graph relation list --to <ID>                     # incoming relations for an entity
spec-graph relation list --type <TYPE>                 # all relations of a given type
spec-graph relation list --from <ID> --layer arch      # outgoing arch relations only
spec-graph relation list --from <ID> --layer mapping   # outgoing mapping relations only
```

### relation delete

```bash
spec-graph relation delete --from <FROM-ID> --to <TO-ID> --type <RELATION_TYPE>
```

---

## impact

Change impact analysis. This is a core command.

```bash
spec-graph impact <ID> [<ID>...]                          # default analysis
spec-graph impact <ID> --follow implements,verifies       # traverse only specified relation types
spec-graph impact <ID> --min-severity medium              # filter by minimum severity
spec-graph impact <ID> --dimension structural             # single dimension only
spec-graph impact <ID> --dimension behavioral
spec-graph impact <ID> --dimension planning
spec-graph impact <ID> --layer arch                       # restrict traversal to arch relations
spec-graph impact <ID> --layer mapping                    # restrict traversal to mapping relations
```

### Options
- `--follow <types>`: comma-separated. Only traverse the listed relation types.
- `--min-severity <low|medium|high>`: include only results at or above this severity.
- `--dimension <structural|behavioral|planning>`: compute impact for a single dimension.
- `--layer <arch|exec|mapping|all>`: restrict traversal to relations in the specified layer.

### Response Shape
```json
{
  "sources": ["REQ-1752239482-k3f"],
  "affected": [
    {
      "id": "API-1752239500-t7n",
      "type": "interface",
      "depth": 1,
      "path": ["REQ-1752239482-k3f", "API-1752239500-t7n"],
      "relation_chain": ["implements"],
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
    "total": 1,
    "by_type": {"interface": 1},
    "by_impact": {"high": 1}
  }
}
```

### Multi-source Analysis
```bash
spec-graph impact "$REQ_ID" "$DEC_ID"
```
Computes the combined impact when both entities change simultaneously.

---

## validate

Graph verification and workflow gate checks.

```bash
spec-graph validate                                        # full validation (all layers)
spec-graph validate --layer arch                           # arch checks only
spec-graph validate --layer exec                           # exec checks only
spec-graph validate --layer mapping                        # mapping checks only
spec-graph validate --check <CHECK_NAME>                   # specific check only
spec-graph validate --phase <PHS-ID>                       # scope to a phase (mapping/all only)
spec-graph validate --entity <ID>                          # scope to a single entity
spec-graph validate --layer mapping --phase <PHS-ID>       # combinable
spec-graph validate --layer arch --check coverage          # combinable
spec-graph validate --check phase_satisfaction --phase <PHS-ID> --include-references
```

`--include-references` applies to `phase_satisfaction` and additionally reports `references`
evidence as advisory findings.

### Check Types by Layer

**Architecture layer** (`--layer arch`):
- `orphans`: arch entities with no relations
- `coverage`: missing implementations / tests for requirements
- `cycles`: circular references in depends_on chains
- `conflicts`: semantic conflicts between entities
- `invalid_edges`: arch edge matrix violations
- `superseded_refs`: active references to superseded entities
- `unresolved`: open questions, unverified assumptions, unmitigated risks

**Execution layer** (`--layer exec`):
- `phase_order`: detects duplicate numeric phase `order` values
- `single_active_plan`: only one plan has active status
- `orphan_phases`: phases not belonging to any plan
- `exec_cycles`: circular `blocks` chains
- `invalid_exec_edges`: exec edge matrix violations
- `orphan_changes`: changes with no relations
- `task_graph`: exact task parents, same-phase dependencies, and dependency cycles

**Mapping layer** (`--layer mapping`):
- `plan_coverage`: all active requirements are covered by some phase
- `delivery_completeness`: each covered arch entity is itself delivered (exact ID match, no proxies)
- `mapping_consistency`: covers/delivers targets are valid; sources that are `resolved` are exempt
- `invalid_mapping_edges`: mapping edge matrix violations
- `gates`: unresolved questions, unmitigated risks, draft decisions in phase scope
- `task_scope`: task coverage, delivery subset, and no mixed phase/task mappings
- `phase_satisfaction`: opt-in only — never runs as part of a default validation pass

### Response Shape
```json
{
  "valid": true|false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high|medium|low",
      "entity": "REQ-1752239482-k3f",
      "message": "No implementation found"
    }
  ],
  "summary": {
    "total_issues": 0,
    "by_severity": {"high": 0, "medium": 0, "low": 0}
  }
}
```

Each issue carries exactly `check`, `severity`, `entity`, and `message`. The CLI does not emit a
`layer` field on issues; infer the layer from the check name.

---

## query

Graph traversal and lookup.

### query neighbors
```bash
spec-graph query neighbors <ID> --depth <N>
```
Returns all entities within N hops from the given entity. Note: `--layer` is accepted as a root
persistent flag but is **not** applied to neighbor traversal, so it will not filter these results.

### query path
```bash
spec-graph query path <FROM-ID> <TO-ID>
spec-graph query path <FROM-ID> <TO-ID> --layer arch
```
Returns the shortest path between two entities. Empty result if no path exists. `--layer` restricts
which relations the path may use.

### query scope
```bash
spec-graph query scope <PHS-ID>
spec-graph query scope <PHS-ID> --layer mapping
```
Returns effective arch scope. Task-managed phases return the union of child task mappings;
taskless phases preserve direct mappings and their insertion order.

### query unresolved
```bash
spec-graph query unresolved --type question|assumption|risk
spec-graph query unresolved --type question --phase <PHS-ID>
```
Returns items with status `draft` or `active` (i.e. not yet resolved). `--phase` restricts results
to the phase's effective scope.

### query sql
```bash
spec-graph query sql "SELECT id, type, layer, title FROM entities WHERE status = 'draft'"
```
Executes raw SQL. Only SELECT statements are allowed. The `layer` column is available on
both `entities` and `relations` tables.

---

## phase

### phase next

```bash
spec-graph phase next [--activate]
```

Selects the next eligible phase — only phases whose `precedes` predecessors are resolved qualify.
`--activate` transitions the selected phase from `draft` to `active`.

### phase context

```bash
spec-graph phase context <PHS-ID>
```

Returns the non-persisted execution contract:

```json
{
  "plan": {}, "phase": {},
  "tasks": [{"entity":{},"contract":{},"prerequisite_ids":[],"covers":[],"delivers":[]}],
  "scope": [], "delivery": [], "blockers": {},
  "ready_task_ids": [], "blocked_task_ids": []
}
```

RPC method `phase.context` and MCP tool `phase_context` return the same shape. Task contracts use
the six closed fields `order`, `instructions`, `acceptance`, `must_not`, `references`, and `qa`.

---

## export

Export the graph in various formats.

```bash
spec-graph export --format json                          # full graph as JSON
spec-graph export --format dot                           # Graphviz DOT
spec-graph export --format mermaid                       # Mermaid diagram
spec-graph export --center <ID> --depth 3                # subgraph centered on an entity
spec-graph export --layer arch --format dot              # arch layer only
spec-graph export --layer exec --format mermaid          # exec layer only
spec-graph export --layer mapping --format json          # mapping relations only
```

---

## bootstrap

Extract entity/relation candidates from existing documents.

### bootstrap scan
```bash
spec-graph bootstrap scan --input ./docs/
spec-graph bootstrap scan --input ./docs/ --format json
```
Scans `.md` files and extracts candidate entities and relations using regex pattern matching.
Extraction is based on entity ID patterns — not free-text NLP. Both ID forms are recognized:
the current `PREFIX-<unixSeconds>-<rand3>` (e.g. `REQ-1752239482-k3f`) and legacy `PREFIX-NNN`
(e.g. `REQ-001`). Documents must already contain one of these forms for candidates to be detected.

Each candidate includes `confidence` (0.4–0.9), `source` (file path with line number),
and an inferred type based on the ID prefix.

### bootstrap import
```bash
spec-graph bootstrap import --input extracted.json --mode review
```
- `--mode review` (default): presents candidates for approval and does not write.
- `--mode apply`: imports candidates as one transaction.

Response for `--mode apply`:
```json
{
  "created": ["REQ-1752239482-k3f"],
  "skipped": [
    {"id": "API-1752239500-t7n:REQ-1752239482-k3f:implements", "reason": "invalid edge"}
  ]
}
```

- `skipped` contains candidates below `0.5` confidence, entities or relations that already exist,
  and relation candidates whose endpoint types fail the edge matrix.
- An unknown entity or relation type, a malformed entity ID, a missing relation endpoint, or a
  write failure aborts the entire import without writing and exits non-zero. Malformed input is
  rejected regardless of its confidence.

---

## migrate

One-shot migration from the legacy SQLite-only layout to TOML-first storage.

```bash
spec-graph migrate [--dry-run] [--keep-db]
```

- `--dry-run`: preview the migration without writing files.
- `--keep-db`: keep the old `graph.db` instead of renaming it to `.bak`.

---

## doctor

Integrity validation of the TOML store. Run after `git merge` or `git pull` resolves conflicts.

```bash
spec-graph doctor [--check <names>] [--fix]
```

- `--check`: comma-separated list of checks; defaults to all.
- `--fix`: reserved, not yet supported.

---

## serve / mcp

```bash
spec-graph serve   # JSON-RPC over stdio
spec-graph mcp     # MCP server over stdio
```

`serve` exposes engine operations as JSON-RPC methods (e.g. `phase.context`); `mcp` exposes them as
MCP tools (e.g. `phase_context`). Note that `entity revise` is CLI-only and is not available on
either transport.
