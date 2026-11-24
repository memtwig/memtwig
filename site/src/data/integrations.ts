export type Integration = {
  slug:
    | "hermes"
    | "grok-bot"
    | "openbot"
    | "t3-code"
    | "claude-code"
    | "cursor"
    | "codex"
    | "antigravity";
  name: string;
  maker: string;
  transport: "stdio" | "http" | "hybrid";
  transportLabel: string;
  tagline: string;
  description: string;
  diagramNodes: {
    clientSub: string;
    channelLabel: string;
    channelSub: string;
    vaultSub: string;
  };
  steps: Array<{
    title: string;
    lang: string;
    code: string;
  }>;
  highlights: string[];
  faqs: Array<{ q: string; a: string }>;
};

export const integrations: Integration[] = [
  {
    slug: "hermes",
    name: "Hermes Agent",
    maker: "Nous Research",
    transport: "hybrid",
    transportLabel: "Stdio MCP + SOUL.md",
    tagline: "Share one SOUL.md identity and curated Markdown vaults across Hermes and coding agents.",
    description:
      "Use memtwig with Nous Research Hermes Agent via stdio MCP and shared SOUL.md identity files with Git-backed multi-org vault isolation.",
    diagramNodes: {
      clientSub: "config.yaml · SOUL.md",
      channelLabel: "stdio MCP",
      channelSub: "6 MCP tools · SOUL.md",
      vaultSub: "cwd & remote routed",
    },
    steps: [
      {
        title: "Register memtwig in ~/.hermes/config.yaml",
        lang: "yaml",
        code: `mcp_servers:
  memtwig:
    command: "memtwig"
    args: ["mcp"]

# Optional: point memtwig identity.soul_file in ~/.config/memtwig/config.toml
# soul_file = "~/.hermes/SOUL.md"`,
      },
      {
        title: "Initialize vaults & reload MCP inside Hermes",
        lang: "shell",
        code: `memtwig setup
memtwig status

# Inside an active Hermes session:
/reload-mcp`,
      },
    ],
    highlights: [
      "Hermes and your terminal coding tools read the same curated Git Markdown vaults.",
      "Share one ~/.hermes/SOUL.md across Hermes and Claude Code SessionStart hooks.",
      "Client vaults stay isolated by working directory and queue diff proposals for review.",
    ],
    faqs: [
      {
        q: "Does Hermes see client notes from every project at once?",
        a: "No. memtwig resolves active vaults from the current working directory and Git remote, keeping separate client workspaces isolated.",
      },
      {
        q: "Do I need a vector database for Hermes memory?",
        a: "No. Local SQLite FTS5 search runs offline inside memtwig, and optional 1024-dim embeddings can be enabled in Settings.",
      },
    ],
  },
  {
    slug: "grok-bot",
    name: "Grok Bot & CLI",
    maker: "xAI",
    transport: "hybrid",
    transportLabel: "Stdio + HTTP RBAC",
    tagline: "Give 24/7 Grok Bots scoped HTTP MCP access while keeping local Grok CLI synced.",
    description:
      "Connect xAI Grok Bot and Grok CLI to memtwig via stdio MCP locally and RBAC-gated Streamable HTTP MCP for cloud workstations.",
    diagramNodes: {
      clientSub: "Grok CLI & Cloud Bot",
      channelLabel: "stdio + HTTP",
      channelSub: "POST /mcp · Bearer RBAC",
      vaultSub: "propose-only client vault",
    },
    steps: [
      {
        title: "Configure a scoped RBAC token in config.toml",
        lang: "toml",
        code: `[server]
  addr = "127.0.0.1:8787"
  audit_log_path = "~/.local/share/memtwig/audit.jsonl"

[[server.tokens]]
  name = "grok-bot"
  token_env = "MEMTWIG_GROK_TOKEN"
  role = "proposer"
  vaults = ["acme"]
  path_globs = ["projects/**", "infra/**"]`,
      },
      {
        title: "Run HTTP server for Grok Bot (or stdio for Grok CLI)",
        lang: "shell",
        code: `# Cloud Grok Bot (expose via HTTPS / Tailscale to POST /mcp):
export MEMTWIG_GROK_TOKEN="replace-with-random-token"
memtwig serve --addr 127.0.0.1:8787

# Local Grok CLI: register stdio MCP command:
memtwig mcp`,
      },
    ],
    highlights: [
      "Autonomous 24/7 Grok Bots run with role = 'proposer' and never overwrite notes directly.",
      "Every memory search, read, and proposal from the bot is recorded in audit.jsonl.",
      "Local Grok CLI sessions connect via memtwig mcp with zero network setup.",
    ],
    faqs: [
      {
        q: "How do I prevent an always-on Grok Bot from corrupting notes?",
        a: "Assign role = 'proposer' and specific vault/path globs in [[server.tokens]]. All edits queue as diffs for you to approve in memtwig ui.",
      },
      {
        q: "Can a Grok Bot access multiple clients with one token?",
        a: "Only if you explicitly list them in vaults. By default, a token only sees the vaults and path globs you allow.",
      },
    ],
  },
  {
    slug: "openbot",
    name: "OpenBot",
    maker: "CopilotKit",
    transport: "http",
    transportLabel: "HTTP MCP + RBAC",
    tagline: "Give each containerized OpenBot coworker scoped vault access with diff governance.",
    description:
      "Integrate memtwig with CopilotKit OpenBot AI coworkers over Streamable HTTP MCP with per-coworker RBAC tokens, owner-lock, and audit logs.",
    diagramNodes: {
      clientSub: "Coworker containers",
      channelLabel: "HTTP /mcp",
      channelSub: "Bearer RBAC · audit.jsonl",
      vaultSub: "scoped per coworker",
    },
    steps: [
      {
        title: "Define per-coworker RBAC policies in config.toml",
        lang: "toml",
        code: `[server]
  addr = "127.0.0.1:8787"
  audit_log_path = "~/.local/share/memtwig/audit.jsonl"

[[server.tokens]]
  name = "openbot-research"
  token_env = "OPENBOT_MEMTWIG_TOKEN"
  role = "proposer"
  vaults = ["acme"]
  path_globs = ["projects/**", "decisions/**"]`,
      },
      {
        title: "Start memtwig serve & review diffs in Studio UI",
        lang: "shell",
        code: `# Expose Streamable HTTP MCP endpoint (/mcp) to OpenBot:
export OPENBOT_MEMTWIG_TOKEN="replace-with-random-token"
memtwig serve --addr 127.0.0.1:8787

# Review coworker proposals and audit log locally:
memtwig ui`,
      },
    ],
    highlights: [
      "Give each OpenBot coworker container its own Bearer token and vault scope.",
      "Approve or reject coworker memory updates from memtwig ui or memtwig tui.",
      "Built-in secret scanner blocks accidental tokens before vaults sync to Git.",
    ],
    faqs: [
      {
        q: "How does OpenBot connect to memtwig?",
        a: "Point OpenBot's MCP configuration at your memtwig serve endpoint (/mcp) with a Bearer token from [[server.tokens]].",
      },
      {
        q: "Can OpenBot coworkers overwrite owner-locked files?",
        a: "No. Notes marked owner: ... and vaults set to write = propose always queue a reviewable diff instead of writing directly.",
      },
    ],
  },
  {
    slug: "t3-code",
    name: "T3 Code",
    maker: "T3",
    transport: "hybrid",
    transportLabel: "Hooks + MCP",
    tagline: "Keep one Git-backed memory across Claude, Codex, and Antigravity threads and worktrees.",
    description:
      "Use memtwig with T3 Code so orchestrated Claude Code, Codex, and Antigravity sessions across Git worktrees share scoped Markdown vaults.",
    diagramNodes: {
      clientSub: "Multi-provider threads",
      channelLabel: "hooks + MCP",
      channelSub: "6 MCP tools · worktree safe",
      vaultSub: "git remote routed",
    },
    steps: [
      {
        title: "Initialize vaults & wire your T3 Code providers",
        lang: "shell",
        code: `memtwig setup
memtwig install claude
memtwig install codex
memtwig install antigravity`,
      },
      {
        title: "Verify worktree & multi-org vault resolution",
        lang: "shell",
        code: `# Works inside any worktree because routing checks cwd and git remote:
memtwig status --cwd ~/work/acme/payments-api

# Inspect Knowledge Graph and queued agent proposals:
memtwig ui`,
      },
    ],
    highlights: [
      "Switch providers mid-project in T3 Code without losing shared project facts.",
      "Git worktrees automatically match the same org vault via match_git_remotes.",
      "Gated per-turn hooks inject relevant snippets while skipping noisy courtesy turns.",
    ],
    faqs: [
      {
        q: "Do subagents running in separate Git worktrees resolve the right vault?",
        a: "Yes. Matching on git remote (for example github.com/acme/*) resolves the same client vault inside every worktree checkout.",
      },
      {
        q: "Can subagents edit my personal preferences?",
        a: "Personal notes stay direct or owner-locked, while org vaults require diff approval before changes land.",
      },
    ],
  },
  {
    slug: "claude-code",
    name: "Claude Code",
    maker: "Anthropic",
    transport: "hybrid",
    transportLabel: "Native Hooks + MCP",
    tagline: "Wire UserPromptSubmit, SessionStart, and MCP tools into Claude Code in one command.",
    description:
      "Install memtwig in Anthropic Claude Code with one command to enable gated per-turn hooks, SOUL.md injection, and 6 stdio MCP tools.",
    diagramNodes: {
      clientSub: "~/.claude/settings.json",
      channelLabel: "hooks + MCP",
      channelSub: "SessionStart + per-turn",
      vaultSub: "cwd & remote routed",
    },
    steps: [
      {
        title: "Install hooks and MCP server into Claude Code",
        lang: "shell",
        code: `memtwig setup
memtwig install claude`,
      },
      {
        title: "Check active vaults & launch Studio UI",
        lang: "shell",
        code: `memtwig status
memtwig search "deploy"
memtwig ui`,
      },
    ],
    highlights: [
      "Adds UserPromptSubmit and SessionStart hooks to ~/.claude/settings.json automatically.",
      "Registers memtwig mcp in ~/.claude.json with automatic .bak backups.",
      "Keeps repo CLAUDE.md focused on build commands while offloading cross-repo facts.",
    ],
    faqs: [
      {
        q: "Does memtwig install claude overwrite my existing settings?",
        a: "No. It merges entries idempotently and saves a .bak backup before modifying any existing config file.",
      },
      {
        q: "Why use hooks alongside MCP?",
        a: "Hooks surface matching context automatically on relevant prompts, while MCP lets Claude explicitly read, propose diffs, and log decisions.",
      },
    ],
  },
  {
    slug: "cursor",
    name: "Cursor",
    maker: "Cursor",
    transport: "stdio",
    transportLabel: "Stdio MCP",
    tagline: "Give Cursor scoped search and proposal tools across client repos instead of static .cursorrules.",
    description:
      "Connect Cursor to memtwig via stdio MCP in ~/.cursor/mcp.json so agents search client-isolated Markdown vaults without bloated rules files.",
    diagramNodes: {
      clientSub: "~/.cursor/mcp.json",
      channelLabel: "stdio MCP",
      channelSub: "6 MCP tools · zero network",
      vaultSub: "per-client isolation",
    },
    steps: [
      {
        title: "Register memtwig in ~/.cursor/mcp.json",
        lang: "shell",
        code: `memtwig setup
memtwig install cursor`,
      },
      {
        title: "Add a client vault & inspect routing",
        lang: "shell",
        code: `memtwig vault add acme ~/notes-acme --kind org --write propose \\
  --match-path "~/work/acme/**" \\
  --match-remote "github.com/acme/*"

memtwig status --cwd ~/work/acme/api`,
      },
    ],
    highlights: [
      "One command writes mcpServers.memtwig into ~/.cursor/mcp.json.",
      "Automatically isolates Client A notes from Client B repos.",
      "Review every memory update Cursor proposes in the local Studio UI.",
    ],
    faqs: [
      {
        q: "Can I preview what install cursor will write?",
        a: "Yes. Run 'memtwig install cursor --dry-run' first to inspect the exact JSON patch.",
      },
      {
        q: "Does Cursor share memory with Claude Code and Codex?",
        a: "Yes. All harnesses read and propose against the exact same local Markdown vaults and SQLite index.",
      },
    ],
  },
  {
    slug: "codex",
    name: "OpenAI Codex",
    maker: "OpenAI",
    transport: "hybrid",
    transportLabel: "Stdio MCP + Hooks",
    tagline: "Add memtwig MCP tools to ~/.codex/config.toml in one command with multi-org routing.",
    description:
      "Configure OpenAI Codex CLI with the memtwig stdio MCP server and prompt hooks for Git-native, client-isolated Markdown memory in one command.",
    diagramNodes: {
      clientSub: "~/.codex/config.toml",
      channelLabel: "stdio MCP",
      channelSub: "MCP + hook adapter",
      vaultSub: "cwd & remote routed",
    },
    steps: [
      {
        title: "Append [mcp_servers.memtwig] to ~/.codex/config.toml",
        lang: "shell",
        code: `memtwig setup
memtwig install codex`,
      },
      {
        title: "Verify MCP tools & open proposals",
        lang: "shell",
        code: `memtwig status
memtwig proposal list
memtwig ui`,
      },
    ],
    highlights: [
      "Writes the [mcp_servers.memtwig] TOML block into ~/.codex/config.toml idempotently.",
      "Includes a dedicated 'memtwig hook codex' stdin/stdout adapter for prompt hooks.",
      "Codex proposals stay queued as diffs until you approve them.",
    ],
    faqs: [
      {
        q: "Is memtwig install codex safe to run twice?",
        a: "Yes. It checks whether [mcp_servers.memtwig] is already present and backs up config.toml before editing.",
      },
      {
        q: "Does Codex work offline with memtwig?",
        a: "Yes. Local FTS5 retrieval runs directly inside the memtwig binary without any network calls.",
      },
    ],
  },
  {
    slug: "antigravity",
    name: "Antigravity",
    maker: "Google DeepMind",
    transport: "hybrid",
    transportLabel: "PreInvocation + MCP",
    tagline: "Inject gated vault context via PreInvocation hooks and expose 6 scoped MCP tools.",
    description:
      "Use memtwig with Google Antigravity using native PreInvocation hooks (injectSteps) and stdio MCP tools with per-client vault isolation.",
    diagramNodes: {
      clientSub: "PreInvocation + MCP",
      channelLabel: "hooks + MCP",
      channelSub: "injectSteps + 6 tools",
      vaultSub: "cwd & remote routed",
    },
    steps: [
      {
        title: "Print the Antigravity hook & MCP config snippet",
        lang: "shell",
        code: `memtwig setup
memtwig install antigravity`,
      },
      {
        title: "Snippet generated for your Antigravity config",
        lang: "json",
        code: `{
  "mcpServers": {
    "memtwig": {
      "command": "memtwig",
      "args": ["mcp"]
    }
  },
  "hooks": {
    "PreInvocation": [
      { "command": "memtwig hook antigravity" }
    ]
  }
}`,
      },
    ],
    highlights: [
      "Native PreInvocation adapter emits standard injectSteps payloads automatically.",
      "Gated query filter avoids wasting context on short acknowledgements.",
      "Full Studio UI and TUI let you browse the Knowledge Graph and review diffs.",
    ],
    faqs: [
      {
        q: "How does the Antigravity hook avoid injecting noise?",
        a: "memtwig hook antigravity gates short or courtesy prompts and skips retrieval when no relevant snippets pass score thresholds.",
      },
      {
        q: "Can Antigravity log architectural decisions automatically?",
        a: "Yes. Calling memory_decide logs a dated entry in decisions/YYYY-MM.md and queues a proposal to update the project note.",
      },
    ],
  },
];
