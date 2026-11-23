# Multi-Org Isolation Guide

Freelancers and consultants often switch between personal notes and multiple client repositories on the same machine. `memtwig` isolates client vaults by default so client A's context never leaks into client B's session.

## Setup: Personal + Client Vaults

Initialize or register each vault in `~/.config/memtwig/config.toml`:

```bash
# Personal vault (default_write = "direct")
memtwig init ~/notes --name personal --kind personal

# Client vaults (default_write = "propose" for kind = "org")
memtwig vault add acme ~/work/acme/notes \
  --kind org \
  --match-path "~/work/acme/**" \
  --match-remote "github.com/acme-corp/*"

memtwig vault add globex ~/work/globex/notes \
  --kind org \
  --match-path "~/work/globex/**" \
  --match-remote "github.com/globex/*"
```

Then index all configured vaults:

```bash
memtwig index
```

## How Scope Resolution Works

Run `memtwig status` inside any directory to inspect the resolved scope:

- **Inside `~/work/acme/api`** (or any repo whose `origin` remote matches `github.com/acme-corp/*`):
  - `acme` is active with `full` access (`match_path` or `match_remote`).
  - `globex` is **excluded** completely.
  - `personal` is **excluded** unless `always_include` is set (in which case only those globs are readable).
- **Inside `~/personale/side-project`** (no org vault matches):
  - `personal` is active with `full` access (`default_personal`).
  - All `org` vaults (`acme`, `globex`) remain hidden.

## Sharing Preferences with `always_include`

To keep your personal coding style or tone rules available during client sessions without exposing personal project notes, set `always_include` on your `personal` vault:

```toml
[vaults.personal]
path = "~/notes"
kind = "personal"
default_write = "direct"
always_include = ["persona/**"]
```

Supported glob patterns in `always_include` and `match_paths`:

- `dir/**` — matches `dir` and all nested paths underneath it
- `dir/*` — matches direct children under `dir/`
- Standard `filepath.Match` patterns (e.g., `persona/*.md`)

## Matching Git Remotes (`match_git_remotes`)

`memtwig` runs `git -C <cwd> config --get remote.origin.url` and normalizes the URL before matching:

- Strips `https://`, `http://`, `ssh://`, and trailing `.git`
- Converts `git@host:owner/repo` to `host/owner/repo`
- Matches case-insensitively against `match_git_remotes`

This means a fresh clone in `/tmp/acme-hotfix` still activates the `acme` vault automatically even if it sits outside `~/work/acme/**`.

## Isolation Guarantees

- **SQL-level filtering**: both FTS5 BM25 and vector queries filter `WHERE vault_id IN (<active_vaults>)`.
- **Path-glob enforcement**: `ActiveVault.AllowsPath(relPath)` is checked on every search hit, `memory_read`, and `memory_write`.
- **Traversal protection**: `safeJoin` resolves symlinks/relative segments and rejects any path escaping the vault root.
- **Explicit overrides**: `--vault <name>` or `MEMTWIG_VAULT=<name>` restricts scope to that single vault (plus `personal.always_include`). Use `--cwd <path>` to test scope resolution for another directory.
