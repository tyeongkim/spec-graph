# CLI Reference

## Table of Contents
1. [General Rules](#general-rules)
2. [init](#init)
3. [entity](#entity)
4. [relation](#relation)
5. [impact](#impact)
6. [validate](#validate)
7. [query](#query)
8. [history](#history)
9. [export](#export)
10. [bootstrap](#bootstrap)

---

## General Rules

- All output is **JSON on stdout** — machine-parseable, not human prose.
- Error messages go to **stderr**.
- Exit codes: `0` success, `1` runtime error, `2` validation failure, `3` invalid input / schema violation.
- Commands with a `--reason` flag record a changeset entry.

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

- `--type`: requirement, decision, phase, interface, state, test, crosscut, question, assumption, criterion, risk
- `--id`: type prefix + number. e.g. REQ-001, DEC-003, PHS-002
- `--status`: defaults to `draft`
- `--metadata`: type-specific required fields — see `references/data-model.md`

**Example**:
```bash
spec-graph entity add --type requirement --id REQ-001 \
  --title "All APIs require authentication" \
  --description "No anonymous access allowed" \
  --metadata '{"priority":"must","kind":"functional","owner":"auth-team"}'
```

### entity get

```bash
spec-graph entity get <ID>
```

Returns full entity details as JSON.

### entity list

```bash
spec-graph entity list [--type <TYPE>] [--status <STATUS>]
```

Returns entities matching the filter. Omitting both returns all entities.

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
3. **allowed edge matrix** compliance (violation → exit 3)
4. duplicate edge check
5. self-loop prohibition (supersedes, conflicts_with, etc.)

**Examples**:
```bash
spec-graph relation add --from API-005 --to REQ-001 --type implements
spec-graph relation add --from TST-012 --to REQ-001 --type verifies
spec-graph relation add --from REQ-001 --to PHS-002 --type planned_in
```

### relation list

```bash
spec-graph relation list --from <ID>    # outgoing relations for an entity
spec-graph relation list --to <ID>      # incoming relations for an entity
spec-graph relation list --type <TYPE>  # all relations of a given type
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
```

### Options
- `--follow <types>`: comma-separated. Only traverse the listed relation types.
- `--min-severity <low|medium|high>`: include only results at or above this severity.
- `--dimension <structural|behavioral|planning>`: compute impact for a single dimension.

### Response Shape
```json
{
  "sources": ["REQ-001"],
  "affected": [
    {
      "id": "API-005",
      "type": "interface",
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
spec-graph validate                                # full validation
spec-graph validate --check <CHECK_NAME>           # specific check only
spec-graph validate --phase <PHS-ID>               # scope to a phase
spec-graph validate --entity <ID>                  # scope to a single entity
spec-graph validate --phase <PHS-ID> --check gates # combinable
```

### Check Types
- `orphans`: entities with no relations
- `coverage`: missing implementations / tests
- `cycles`: circular references
- `conflicts`: semantic conflicts
- `gates`: workflow prerequisites
- `invalid_edges`: edge matrix violations
- `superseded_refs`: active references to deprecated entities

### Response Shape
```json
{
  "valid": true|false,
  "issues": [
    {
      "check": "coverage",
      "severity": "high|medium|low",
      "entity": "REQ-007",
      "message": "No implementation found"
    }
  ],
  "summary": {
    "total_issues": 0,
    "by_severity": {"high": 0, "medium": 0, "low": 0}
  }
}
```

---

## query

Graph traversal and lookup.

### query neighbors
```bash
spec-graph query neighbors <ID> --depth <N>
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
Returns all entities linked to the phase via `planned_in` or `delivered_in`.

### query unresolved
```bash
spec-graph query unresolved --type question|assumption|risk
```
Returns items with status `draft` or `active` (i.e. not yet resolved).

### query sql
```bash
spec-graph query sql "SELECT id, type, title FROM entities WHERE status = 'draft'"
```
Executes raw SQL. Only SELECT statements are allowed.

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
spec-graph export --format json              # full graph as JSON
spec-graph export --format dot               # Graphviz DOT
spec-graph export --format mermaid           # Mermaid diagram
spec-graph export --center <ID> --depth 3    # subgraph centered on an entity
```

---

## bootstrap

Extract entity/relation candidates from existing documents.

### bootstrap scan
```bash
spec-graph bootstrap scan --input ./docs/
spec-graph bootstrap scan --input ./docs/ --format json
```
Scans documents and extracts candidate entities and relations. Each candidate includes `confidence` and `source_span`.

### bootstrap import
```bash
spec-graph bootstrap import --input extracted.json --mode review
```
- `--mode review` (default): presents candidates for approval before committing.
- Low-confidence items are never auto-imported.
