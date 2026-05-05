---
description: Replicator CLI quick reference
tags: [cli, reference, replicator]
---

# Replicator CLI

Quick reference for all replicator commands.

## Commands

| Command | Purpose |
|---------|---------|
| `replicator init` | Per-repo setup: creates `.uf/replicator/` + agent kit |
| `replicator setup` | Per-machine setup: creates global SQLite DB |
| `replicator serve` | Start MCP JSON-RPC server on stdio |
| `replicator cells` | List work items (cells) |
| `replicator doctor` | Check environment health |
| `replicator stats` | Display activity summary |
| `replicator query` | Run preset SQL analytics queries |
| `replicator docs` | Generate MCP tool reference (markdown) |
| `replicator version` | Print version, commit, build date |

## Build Targets

```bash
make build    # Build binary to ./bin/replicator
make test     # Run all tests
make vet      # Go vet
make check    # Vet + test
make serve    # Build and run MCP server
make install  # Install to GOPATH/bin
```

## Init Flags

- `--path <dir>` — target directory (default: `.`)
- `--force` — overwrite existing agent kit files

## MCP Tool Categories

- `org_*` (11 tools) — work item management
- `comms_*` (10 tools) — agent messaging and file reservations
- `forge_*` (24 tools) — multi-agent coordination
- `hivemind_*` (8 tools) — learning storage and retrieval
