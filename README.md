# memtwig

Git-native, multi-org Markdown memory layer for AI agents — shipped as a single static Go binary (`CGO_ENABLED=0`, zero external vector DB).

## Why memtwig

- **Git is the database, Markdown is the format:** human-readable, diffable, auditable, zero vendor lock-in.
- **Multi-org isolation:** route vaults automatically by `cwd` or `git remote`. Working in Client A's repo exposes your personal preferences (`always_include`) + Client A's vault, never Client B's.
- **Active hygiene:** deterministic linter (`check`), secret scanner, soft 8 KB cap per note, append-only archiving (`invalidate`), dated decisions (`decide`), deterministic + optional LLM semantic curation (`curate --llm`), and an idempotent proposal queue (`.curator/proposals.json` + append-only `history.jsonl`).
- **Cross-harness & team-ready:** single CLI, embedded local Web UI (`memtwig ui`), interactive terminal UI (`memtwig tui`), stdio MCP server (`memtwig mcp`), Streamable HTTP MCP + REST API server with RBAC & audit log (`memtwig serve`), and per-turn gated prefetch hooks (`memtwig install <claude|cursor|codex|antigravity>`).

## Install

```bash
curl -fsSL https://memtwig.com/install.sh | sh
# or with Go
go install github.com/memtwig/memtwig/cmd/memtwig@latest
# or build locally
make build
```

## Quick start

```bash
# Fastest one-command start (creates or imports ~/notes, indexes it, detects installed harnesses)
memtwig setup

# Or set up isolated client org vaults explicitly
memtwig init ~/notes-acme --name acme --kind org --match-path "$HOME/work/acme/**" --match-remote "github.com/acme-corp/*"
memtwig migrate personal ~/notes --kind personal --always-include "persona/**"

# 2. Check environment & build local SQLite FTS5 (+ optional embeddings) index
memtwig doctor
memtwig index

# 3. Search within the active workspace scope
memtwig search "deploy"

# 4. Curate, record decisions, archive stale lines, lint & sync
memtwig decide "Moved API to Fly.io" --project acme --note projects/infra.md
memtwig invalidate projects/infra.md --match "old-heroku" --reason "migrated"
memtwig curate             # add --llm for semantic contradiction check
memtwig proposal list
memtwig proposal show <id>
memtwig check
memtwig sync

# 5. Open the local Web UI or TUI, connect your AI harness, or run the team server
memtwig ui
memtwig tui
memtwig install claude --dry-run
memtwig install cursor --dry-run
memtwig install codex --dry-run
memtwig serve --addr 127.0.0.1:8787
```

## Documentation

- [Architecture](docs/architecture.md)
- [Multi-Org & Freelance Setup](docs/multi-org.md)
- [AI Harness Integration](docs/harnesses.md)
- [Governance, Proposals & RBAC](docs/governance.md)
- [Example Configuration](examples/config.toml)

## License

Apache 2.0 — see [LICENSE](LICENSE).
