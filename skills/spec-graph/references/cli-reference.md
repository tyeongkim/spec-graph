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
10. [phase context](#phase-context)
11. [bootstrap](#bootstrap)

---

## General Rules

- All output is **JSON on stdout** — machine-parseable, not human prose.
- Error messages go to **stderr**.
- Exit codes: `0` success, `1` runtime error, `2` validation failure, `3` invalid input / schema violation.

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
spec-graph validate --phase PHS-002 --layer mapping
spec-graph validate --phase PHS-002 --layer all
spec-graph validate --phase PHS-002   # --layer all is the default

# Invalid — returns exit 3
spec-graph validate --phase PHS-002 --layer arch
spec-graph validate --phase PHS-002 --layer exec
```

---

## init

Initializes a `.spec-graph/` directory and SQLite database for the project.

```bash
spec-graph init                    # current directory
spec-graph init --path /other/dir  # custom path
```

Fails with exit 1 if already initialized.

---

## entity

### entity add

```bash
spec-graph entity add \
  --type <TYPE> \
  --title "Title" \
  [--id <PREFIX-NNN>] \
  [--description "Description"] \
  [--status draft|active] \
  [--metadata '{"key":"value"}']
```

- `--type`: requirement, decision, plan, phase, task, change, interface, state, test, crosscut, question, assumption, criterion, risk
- `--id`: optional. Type prefix + number (e.g. REQ-001, DEC-003). When omitted, auto-generated from `--type` using `MAX(existing number)+1`. Generated ID is returned in `.entity.id` of the JSON response. Format: unpadded (`REQ-1`, `REQ-2`) for fresh graphs; follows existing zero-padding if padded IDs already exist (e.g. after `REQ-001`, next auto is `REQ-002`). Counters are per-type and independent. Manual `--id` is still validated (prefix must match type) and auto-gen respects manually-set numbers.
- `--status`: defaults to `draft`
- `--metadata`: type-specific required fields — see `references/data-model.md`

**Examples**:
```bash
# Auto-generated ID (capture returned id for subsequent commands)
spec-graph entity add --type requirement \
  --title "All APIs require authentication" \
  --description "No anonymous access allowed" \
  --metadata '{"priority":"must","kind":"functional","owner":"auth-team"}'
# → returns {"entity":{"id":"REQ-1", ...}}

# Capture the generated ID for use in relation commands
REQ_ID=$(spec-graph entity add --type requirement \
  --title "Payments must be idempotent" \
  --metadata '{"priority":"must","kind":"non_functional"}' | jq -r '.entity.id')
spec-graph relation add --from PHS-001 --to "$REQ_ID" --type covers

# Explicit ID (recommended for batch/cross-referencing workflows)
spec-graph entity add --type plan --id PLN-001 \
  --title "v1 Delivery Plan" \
  --metadata '{"status":"active"}'

spec-graph entity add --type phase --id PHS-001 \
  --title "Phase 1 - Auth" \
  --metadata '{"goal":"Build authentication","order":1,"exit_criteria":["Auth API complete"]}'
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
  [--metadata '{}']
```

### entity deprecate

```bash
spec-graph entity deprecate <ID>
```

Sets status to `deprecated`. Run `validate --check superseded_refs` afterward to clean up references.

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

**Examples**:
```bash
# Arch relations
spec-graph relation add --from API-005 --to REQ-001 --type implements
spec-graph relation add --from TST-012 --to REQ-001 --type verifies

# Exec relations
spec-graph relation add --from PHS-001 --to PLN-001 --type belongs_to
spec-graph relation add --from TSK-001 --to PHS-001 --type belongs_to
spec-graph relation add --from TSK-002 --to TSK-001 --type task_depends_on
spec-graph relation add --from PHS-001 --to PHS-002 --type precedes

# Mapping relations (v1 — use these)
spec-graph relation add --from PHS-001 --to REQ-001 --type covers
spec-graph relation add --from PHS-001 --to API-005 --type delivers
spec-graph relation add --from TSK-001 --to REQ-001 --type covers
spec-graph relation add --from TSK-001 --to API-005 --type delivers
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
  "sources": ["REQ-001"],
  "affected": [
    {
      "id": "API-005",
      "type": "interface",
      "layer": "arch",
      "depth": 1,
      "path": ["REQ-001", "API-005"],
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
spec-graph impact REQ-001 DEC-003
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
```

### Check Types by Layer

**Architecture layer** (`--layer arch`):
- `orphans`: arch entities with no relations
- `coverage`: missing implementations / tests for requirements
- `cycles`: circular references in depends_on chains
- `conflicts`: semantic conflicts between entities
- `invalid_edges`: arch edge matrix violations
- `superseded_refs`: active references to deprecated entities
- `unresolved`: open questions, unverified assumptions, unmitigated risks

**Execution layer** (`--layer exec`):
- `phase_order`: phases with precedes/blocks form a valid sequence
- `single_active_plan`: only one plan has active status
- `orphan_phases`: phases not belonging to any plan
- `exec_cycles`: circular precedes/blocks chains
- `invalid_exec_edges`: exec edge matrix violations
- `task_graph`: exact task parents, same-phase dependencies, and dependency cycles

**Mapping layer** (`--layer mapping`):
- `plan_coverage`: all active requirements are covered by some phase
- `delivery_completeness`: covered arch entities have delivery evidence
- `mapping_consistency`: covers/delivers targets exist and are arch entities
- `invalid_mapping_edges`: mapping edge matrix violations
- `task_scope`: task coverage, delivery subset, and no mixed phase/task mappings

### Response Shape
```json
{
  "valid": true|false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high|medium|low",
      "entity": "REQ-007",
      "layer": "arch",
      "message": "No implementation found"
    }
  ],
  "summary": {
    "total_issues": 0,
    "by_severity": {"high": 0, "medium": 0, "low": 0}
  }
}
```

Each issue includes a `layer` field indicating which layer the check belongs to.

---

## query

Graph traversal and lookup.

### query neighbors
```bash
spec-graph query neighbors <ID> --depth <N>
spec-graph query neighbors <ID> --depth <N> --layer arch
```
Returns all entities within N hops from the given entity.

### query path
```bash
spec-graph query path <FROM-ID> <TO-ID>
```
Returns the shortest path between two entities. Empty result if no path exists.

### query scope
```bash
spec-graph query scope <PHS-ID>
```
Returns effective arch scope. Task-managed phases return the union of child task mappings;
taskless phases preserve direct mappings and their insertion order.

### query unresolved
```bash
spec-graph query unresolved --type question|assumption|risk
```
Returns items with status `draft` or `active` (i.e. not yet resolved).

### query sql
```bash
spec-graph query sql "SELECT id, type, layer, title FROM entities WHERE status = 'draft'"
```
Executes raw SQL. Only SELECT statements are allowed. The `layer` column is available on
both `entities` and `relations` tables.

---

## phase context

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
Extraction is based on entity ID patterns (`REQ-001`, `DEC-005`, `PHS-002`, etc.) — not
free-text NLP. Documents must already contain spec-graph ID format (`PREFIX-NNN`) for
candidates to be detected.

Each candidate includes `confidence` (0.4–0.9), `source` (file path with line number),
and an inferred type based on the ID prefix.

### bootstrap import
```bash
spec-graph bootstrap import --input extracted.json --mode review
```
- `--mode review` (default): presents candidates for approval before committing.
- Low-confidence items are never auto-imported.
