# Architecture

`memtwig` is a single-binary, git-native memory layer for AI coding agents. Durable knowledge lives as plain Markdown files inside one or more Git-backed vaults; a local SQLite database acts strictly as a disposable, rebuildable search index.

## System Overview

```text
  AI Harness (Claude / Cursor / Codex / Antigravity / Hermes)
        │                          │
        │ per-turn hook            │ MCP (stdio / HTTP)
        ▼                          ▼
  memtwig hook <harness>     memtwig mcp
        │                          │
        └──────────┬───────────────┘
                   ▼
          Scope Router (internal/scope)
      [cwd + git remote + MEMTWIG_VAULT]
                   │
        ┌──────────┴──────────┐
        ▼                     ▼
  Hybrid Search          Governance & Curator
  (SQLite FTS5 +         (default_write, owner-lock,
   Cosine RRF)            proposals, lint, audit)
        │                     │
        └──────────┬──────────┘
                   ▼
       Git-backed Markdown Vaults
```

## 1. Storage: Multi-Vault Git + Markdown

Each vault is a standard directory initialized with `memtwig init`:

- `projects/` — active project facts and state
- `decisions/` — monthly decision logs (`decisions/YYYY-MM.md`)
- `persona/` — user preferences, identity, and style rules
- `infra/` — machines, services, and operational notes
- `.curator/` — proposal queue (`proposals.json`) and closed-proposal ledger (`history.jsonl`)
- `memtwig.toml` — vault-local defaults (`version`, `max_note_bytes`)
- `index.md` — generated table of contents

`WalkNotes` skips `.git/`, `.curator/`, `private/`, `archive/`, `archivio/`, `bin/`, `node_modules/`, `index.md`, and `README.md`.

Notes support both YAML frontmatter (`status`/`stato`, `valid_from`, `updated`, `owner`, `tags`) and lightweight key-value headers in the first 12 lines (`Status:`, `Updated:`, `Owner:`, `Valid from:`, or their Italian equivalents).

## 2. Scope Router (`internal/scope`)

Before any read, search, or write, `scope.Resolve(cfg, cwd, explicitVault)` determines which vaults are visible:

1. **Explicit override** (`--vault` or `MEMTWIG_VAULT`): activates only the named vault (plus any `personal` vault's `always_include` globs).
2. **Org matching**: checks every `kind = "org"` vault against `cwd` (`match_paths`) and `git config --get remote.origin.url` (`match_git_remotes`, normalized to `host/owner/repo`).
3. **Personal fallback**: if no org vault matched, `personal` vaults are active with full access (`default_personal`). If an org vault matched, `personal` vaults are restricted to their `always_include` path globs unless the personal vault itself explicitly matches `cwd` or the git remote.

## 3. Hybrid Search Engine (`internal/index`, `internal/search`)

The local index lives at `~/.cache/memtwig/index.db` (overridable via `--db` or `MEMTWIG_DB`) using pure-Go SQLite (`modernc.org/sqlite`, `CGO_ENABLED=0`).

- **Chunking**: Markdown files are split into heading-aware chunks and indexed in an FTS5 virtual table (`chunks_fts`).
- **BM25 branch**: extracts non-stopword terms (`>= 2` runes, English + Italian stopword list), queries `chunks_fts` with `WHERE c.vault_id IN (...)`, and ranks up to 40 candidates.
- **Vector branch (optional)**: when `[embeddings].provider` is `ollama`, `openai`, or `cloudflare`, `memtwig index` stores `float32` vectors per chunk. At query time, chunks above `cosine_threshold` (default `0.45`) are ranked by cosine similarity. If the embedding endpoint is unreachable, search degrades gracefully to BM25 with a stderr warning.
- **RRF fusion**: BM25 and vector rankings are fused with Reciprocal Rank Fusion (`1 / (rrf_k + rank)`, default `rrf_k = 60`). Every candidate is additionally checked against `ActiveVault.AllowsPath(relPath)`.

## 4. Anti-Noise Hook Gating (`internal/hook`)

`memtwig hook <claude|codex|antigravity>` is designed for per-turn execution without polluting context:

1. **Tag stripping**: extracts `<user_request>` or strips `<t3_code_instructions>` and `<runtime_info>` wrappers.
2. **Courtesy gate (`IsCourtesy`)**: short acknowledgments (`ok`, `thanks`, `ciao`, `sounds good`, etc.) return `{}` immediately.
3. **Deduplication**: hashes `(conversation_id, prompt)` into `~/.cache/memtwig/hook-seen.json` so identical retries are skipped.
4. **Term gate (`RequireGate`)**: prompts with zero non-stopword terms return `{}` without querying.
5. **Correction nudge (`IsCorrection`)**: if the user appears to be correcting a preference or fact, appends a reminder to persist the rule in the appropriate vault before finishing.

## 5. MCP Server (`internal/mcp`)

`memtwig mcp` runs a line-delimited JSON-RPC 2.0 server over stdio (`protocolVersion: 2024-11-05`) exposing seven scoped tools:

- `memory_search` — hybrid BM25 + vector search across active vaults
- `memory_read` — read a note by `(vault, path)` with path-traversal protection (`safeJoin`)
- `memory_write` — direct write when `default_write = "direct"` and the note is not owner-locked
- `memory_delete` — direct delete of a live Markdown note when `default_write = "direct"`, not owner-locked, and not a protected file/archive/decision log (removes file and index entry)
- `memory_propose` — queue a `create`, `update`, `append`, or `delete` proposal in `.curator/proposals.json`
- `memory_decide` — record a dated decision in `decisions/YYYY-MM.md` and update or propose on a target note
- `memory_status` — list active vaults, `default_write` modes, match reasons, and open proposal counts

## 6. RBAC & Audit Log (`internal/config`, `internal/audit`)

For shared or HTTP deployments, `[server]` in `config.toml` defines:

- `addr` (default `127.0.0.1:8787`)
- `audit_log_path` — append-only JSONL log written by `audit.Logger` (`timestamp`, `actor`, `role`, `harness`, `action`, `vault`, `path`, `outcome`, `detail`)
- `[[server.tokens]]` — bearer token policies (`name`, `token_env`, `role`: `reader | proposer | writer | admin`, `vaults`, `path_globs`)
