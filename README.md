# spec-graph

CLI tool for managing software specifications as a typed graph. Track entities (requirements, decisions, interfaces, phases, etc.) and their relations in a TOML-file-backed graph with built-in impact analysis, validation, and export.

> This tool is designed to be operated by AI agents, not humans. All commands output structured JSON for machine consumption. Human-friendly formatting is not a goal.

## Graph-Native Plans

New plans are stored entirely in `.spec-graph/` as plan (`PLN`), phase (`PHS`), and task (`TSK`)
entities. Tasks belong to phases, carry a closed execution contract, map to architecture entities
with `covers`/`delivers`, and are consumed through `spec-graph phase context <PHS-ID>`. New-plan
workflows do not create Markdown plan files.

Pre-existing taskless phases keep their direct phase mappings and existing Markdown files exactly
as they are. spec-graph does not auto-import, delete, or reinterpret legacy Markdown.

## Install

```bash
# Homebrew
brew install tyeongkim/tap/spec-graph

# Go
go install github.com/tyeongkim/spec-graph/cmd/spec-graph@latest

# From source
make build
# produces bin/spec-graph
```

## Quick Start

```bash
# Initialize project
spec-graph init

# Add entities
spec-graph entity add --type requirement --id REQ-1 --title "User authentication"
# → {"entity":{"id":"REQ-1", ...}}

spec-graph entity add --type decision --id DEC-1 --title "Adopt JWT"
# → {"entity":{"id":"DEC-1", ...}}

# Add relations between entities
spec-graph relation add --from DEC-1 --to REQ-1 --type depends_on

# Export graph (dot, mermaid, json)
spec-graph export --format mermaid
```

## Reference

For full command reference, entity types, and relation types, install the `spec-graph` skill into your AI agent. The skill provides all the context an agent needs to operate this tool.
```bash
bunx --bun skills add https://github.com/tyeongkim/spec-graph.git --skill '*'
```

## Development

```bash
make test       # run tests
make check      # fmt + vet + test
make lint       # golangci-lint
```
