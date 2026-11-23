# Governance, Proposals & Hygiene

`memtwig` separates **direct personal memory** from **reviewed team/client memory** so agents can capture facts without silently overwriting human-owned notes or leaking secrets.

## Write Modes (`default_write`)

Each vault in `config.toml` sets `default_write`:

| Mode | Default for | `memory_write` | `memory_propose` / `memory_decide` |
| --- | --- | --- | --- |
| `direct` | `kind = "personal"` | Allowed (unless owner-locked) | Allowed |
| `propose` | `kind = "org"` | Rejected (must use `memory_propose`) | Queued in `.curator/proposals.json` |
| `readonly` | — | Rejected | Rejected |

## Owner-Locked Notes

Any note with an `owner` field in YAML frontmatter or an `Owner:` / `Proprietario:` header in its first 12 lines is **owner-locked**:

```markdown
---
status: active
owner: vittorio
valid_from: 2026-11-23
updated: 2026-11-23
---
```

When an agent calls `memory_write` on an owner-locked note (even in a `direct` vault), `memtwig` rejects the write and instructs the agent to use `memory_propose`. Similarly, `memtwig decide --note <path>` queues a proposal instead of appending directly.

## Proposals & Deterministic IDs

Proposals are stored in `<vault>/.curator/proposals.json` with a deterministic 12-character hex SHA-1 ID computed from:

```text
sha1(kind + "|" + normalized_rel_path + "|" + lowercased_whitespace_collapsed_content)[:12]
```

Closed proposals (both approved and rejected) are appended to `<vault>/.curator/history.jsonl`. If an agent proposes the exact same change again after it was rejected, `proposal.Add` sees the ID in `history.jsonl` and refuses to reopen it.

Manage proposals from the CLI:

```bash
# List open proposals (add --all to include approved/rejected)
memtwig proposal list

# Approve and apply a proposal to the target Markdown file
memtwig proposal ok <id>

# Reject a proposal (recorded in history.jsonl so it will not reopen)
memtwig proposal no <id> "duplicate of infra/redis.md"
```

## Linting & Secret Scanning (`memtwig check`)

`memtwig check [--json]` runs deterministic checks across all active vaults and exits non-zero if any `error` is found:

- **Blocking errors**:
  - `secret_detected`: OpenAI (`sk-...`), Anthropic (`sk-ant-...`), GitHub (`ghp_...`, `github_pat_...`), AWS (`AKIA...`), Slack (`xox...`), and PEM private key headers (`-----BEGIN ... PRIVATE KEY-----`)
  - `invalid_utf8`: non-UTF-8 Markdown files
  - `future_date`: `updated` / `Aggiornato` date later than today
  - `broken_link`: relative `.md` links in `index.md` that do not exist on disk
- **Warnings**:
  - `oversize_note`: file exceeds `max_note_bytes` (default `8192`)
  - `missing_updated`: missing `updated` / `Aggiornato` header (outside `decisions/` and `decisioni/`)
  - `duplicate_link`: duplicate `.md` links in `index.md`

## Decisions, Invalidation & Curation

- **`memtwig decide <summary> [--project <name>] [--note <rel-path>]`**: prepends a dated bullet to `decisions/YYYY-MM.md` (newest at top) and either appends to `--note` (if `direct` and not owner-locked) or creates a proposal.
- **`memtwig invalidate <rel-path> --match <substring> [--reason <text>]`**: moves matching lines out of a live note into `<dir>/archive/<file>` (append-only) and leaves a dated pointer in the live note.
- **`memtwig curate [--stale-days 90]`**: regenerates `index.md` and creates proposals for oversize notes, stale active notes, and unsynced decisions. When `[llm]` is configured (`openai` or `ollama`), semantic curation compares related note pairs and creates `review` proposals for contradictions or overlaps without editing live notes.
- **`memtwig sync [--push] [-m <msg>]`**: runs `check`, commits dirty vault changes, optionally runs `git pull --rebase && git push`, and refreshes the SQLite index.

## Team RBAC & Audit Log

For shared HTTP deployments, `[server]` supports token-scoped RBAC and an append-only JSONL audit log:

- **Roles**: `reader`, `proposer`, `writer`, `admin`
- **Scoping**: each `[[server.tokens]]` entry restricts `vaults` and `path_globs`
- **Audit trail**: every event records `timestamp`, `actor`, `role`, `harness`, `action`, `vault`, `path`, `outcome` (`ok | denied | error`), and `detail` in `audit_log_path`
