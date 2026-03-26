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
9. [history](#history)
10. [export](#export)
11. [bootstrap](#bootstrap)

---

## General Rules

- All output is **JSON on stdout** — machine-parseable, not human prose.
- Error messages go to **stderr**.
- Exit codes: `0` success, `1` runtime error, `2` validation failure, `3` invalid input / schema violation.
- Commands with a `--reason` flag record a changeset entry.

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
  --id <PREFIX-NNN> \
  --title "Title" \
  [--description "Description"] \
  [--status draft|active] \
  [--metadata '{"key":"value"}'] \
  [--reason "Creation reason"]
```

- `--type`: requirement, decision, plan, phase, interface, state, test, crosscut, question, assumption, criterion, risk
- `--id`: type prefix + number. e.g. REQ-001, DEC-003, PLN-001, PHS-002
- `--status`: defaults to `draft`
- `--metadata`: type-specific required fields — see `references/data-model.md`

**Examples**:
```bash
spec-graph entity add --type requirement --id REQ-001 \
  --title "All APIs require authentication" \
  --description "No anonymous access allowed" \
  --metadata '{"priority":"must","kind":"functional","owner":"auth-team"}'

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
  [--metadata '{}'] \
  --reason "Change reason"
```

`--reason` is required. The change is recorded in a changeset.

### entity deprecate

```bash
spec-graph entity deprecate <ID> --reason "Reason"
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
spec-graph relation add --from PHS-001 --to PHS-002 --type precedes

# Mapping relations (v1 — use these)
spec-graph relation add --from PHS-001 --to REQ-001 --type covers
spec-graph relation add --from PHS-001 --to API-005 --type delivers

# Mapping relations (deprecated — do not use for new work)
# spec-graph relation add --from REQ-001 --to PHS-001 --type planned_in
# spec-graph relation add --from API-005 --to PHS-001 --type delivered_in
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

**Mapping layer** (`--layer mapping`):
- `plan_coverage`: all active requirements are covered by some phase
- `delivery_completeness`: covered arch entities have delivery evidence
- `mapping_consistency`: covers/delivers targets exist and are arch entities
- `invalid_mapping_edges`: mapping edge matrix violations

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
Returns all arch entities linked to the phase via `covers` or `delivers` (and the deprecated
`planned_in` / `delivered_in` for backward compatibility).

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

## history

Change history lookup.

### history changeset
```bash
spec-graph history changeset <CHG-ID>
```
Returns all changes (entity + relation) in a specific changeset.

### history entity
```bash
spec-graph history entity <ID>
```
Returns the full change history for a specific entity.

### history relation
```bash
spec-graph history relation <FROM>:<TO>:<TYPE>
```
Returns the change history for a specific relation.

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
Scans documents and extracts candidate entities and relations. Each candidate includes
`confidence`, `source_span`, and an inferred `layer`.

### bootstrap import
```bash
spec-graph bootstrap import --input extracted.json --mode review
```
- `--mode review` (default): presents candidates for approval before committing.
- Low-confidence items are never auto-imported.
