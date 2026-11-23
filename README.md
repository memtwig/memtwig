# memtwig

Git-native, multi-org Markdown memory layer for AI agents — shipped as a single static Go binary (zero CGO, zero external vector DB).

## Why memtwig

- **Git is the database, Markdown is the format:** human-readable, diffable, auditable.
- **Multi-org isolation:** route vaults automatically by `cwd` or `git remote`. Working in Client A's repo exposes your personal preferences (`always_include`) + Client A's vault, never Client B's.
- **Active hygiene:** deterministic linter (`check`), secret scanner, soft 8 KB cap per note, append-only archiving (`invalidate`), dated decisions (`decide`), and an idempotent proposal queue (`.curator/proposals.json` + append-only `history.jsonl`).
- **Cross-harness:** single CLI, stdio MCP server (`memtwig mcp`), and per-turn gated prefetch hooks for Claude Code, Codex, and Antigravity (`memtwig install <harness>`).

## Quick start

```bash
make build

# Initialize a new vault or register an existing folder (e.g. ~/notes)
./bin/memtwig init ~/notes-acme --name acme --kind org
./bin/memtwig vault add personal ~/notes --kind personal --always-include "persona/**"

# Build local SQLite FTS5 (+ optional Ollama/OpenAI/Cloudflare embeddings) index
./bin/memtwig index

# Search within active workspace scope
./bin/memtwig search "deploy"

# Log a decision, archive outdated lines, run curation, lint & sync
./bin/memtwig decide "Moved API to Fly.io" --project acme --note projects/infra.md
./bin/memtwig invalidate projects/infra.md --match "old-heroku" --reason "migrated"
./bin/memtwig curate
./bin/memtwig check
./bin/memtwig sync

# Wire up your AI harness (supports --dry-run)
./bin/memtwig install claude --dry-run
./bin/memtwig install cursor --dry-run
./bin/memtwig install codex --dry-run
```
