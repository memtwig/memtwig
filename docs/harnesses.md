# Harness Setup Guide

`memtwig` integrates with AI coding harnesses through two mechanisms:

1. **Per-turn hook (`memtwig hook <harness>`)** — automatically injects relevant memory snippets and correction nudges on each prompt, gated against noise.
2. **MCP server (`memtwig mcp`)** — exposes `memory_search`, `memory_read`, `memory_write`, `memory_propose`, `memory_decide`, and `memory_status` over stdio JSON-RPC 2.0.

All `memtwig install` commands create a `.bak` backup before modifying an existing config file and support `--dry-run` and `--bin <path>`.

## Claude Code

```bash
memtwig install claude
```

This adds a `UserPromptSubmit` hook to `~/.claude/settings.json` running `memtwig hook claude`.

To also expose the MCP tools in Claude Code, add `memtwig` to your MCP config (`~/.claude.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "memtwig": {
      "command": "memtwig",
      "args": ["mcp"]
    }
  }
}
```

## Cursor

```bash
memtwig install cursor
```

This registers `mcpServers.memtwig` (`command: "<bin>"`, `args: ["mcp"]`) in `~/.cursor/mcp.json`.

## OpenAI Codex

```bash
memtwig install codex
```

This appends `[mcp_servers.memtwig]` to `~/.codex/config.toml`:

```toml
[mcp_servers.memtwig]
command = "memtwig"
args = ["mcp"]
```

For per-turn hook injection, invoke `memtwig hook codex` from your Codex prompt hook, passing JSON with `prompt`, `cwd`, and `session_id` on stdin.

## Antigravity / T3 Code

```bash
memtwig install antigravity
```

Because Antigravity/T3 configuration paths vary by host, `install antigravity` prints the JSON snippet to paste into your MCP and hooks configuration:

```json
{
  "mcpServers": {
    "memtwig": {
      "command": "memtwig",
      "args": ["mcp"]
    }
  },
  "hooks": {
    "PreInvocation": [
      {
        "command": "memtwig hook antigravity"
      }
    ]
  }
}
```

The `antigravity` hook adapter reads `lastUserInput` (or `prompt`), `cwd`, and `conversation_id`, and returns `{"injectSteps": [{"role": "system", "content": "..."}]}`.

## Hermes

Hermes can use `memtwig` in two ways:

1. **MCP server**: register `memtwig mcp` as a stdio MCP server in your Hermes config so the agent can call `memory_search`, `memory_read`, `memory_propose`, and `memory_decide`.
2. **CLI / hook integration**: call `memtwig search --json --n 5 "<query>"` from a Hermes pre-turn hook or skill, or pipe a hook JSON payload into `memtwig hook claude` (which reads `prompt`, `cwd`, and `session_id` and emits `hookSpecificOutput.additionalContext`).

## Verifying the Setup

```bash
# Check which vaults resolve for the current directory
memtwig status

# Test gated retrieval
memtwig search "architecture decision" --n 5

# Test MCP stdio handshake
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | memtwig mcp
```
