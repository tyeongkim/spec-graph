# spec-graph

CLI tool for managing software specifications as a typed graph. Track entities (requirements, decisions, interfaces, phases, etc.) and their relations in a SQLite-backed graph with built-in impact analysis, validation, and visualization.

> This tool is designed to be operated by AI agents, not humans. All commands output structured JSON for machine consumption. Human-friendly formatting is not a goal.

## Install

```bash
make build
# produces bin/spec-graph
```

## Quick Start

```bash
# Initialize project
spec-graph init

# Add entities
spec-graph entity add --type requirement --id REQ-001 --title "User authentication"
spec-graph entity add --type decision --id DEC-001 --title "Adopt JWT"

# Add relation
spec-graph relation add --from DEC-001 --to REQ-001 --type implements

# Export graph
spec-graph export --format mermaid
```

## Reference

For full command reference, entity types, and relation types, install the `spec-graph` skill into your AI agent. The skill provides all the context an agent needs to operate this tool.
```bash
bunx --bun skills add https://github.com/tyeongkim/spec-graph-cli.git --skill spec-graph
```

## Development

```bash
make test       # run tests
make check      # fmt + vet + test
make lint       # golangci-lint
```
