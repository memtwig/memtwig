import * as React from "react";
import { AnimatePresence, MotionConfig, motion } from "motion/react";
import {
  ArrowClockwise,
  Check,
  Folder,
  LockSimple,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type WorkspaceCase = {
  id: string;
  label: string;
  cwd: string;
  remote: string;
  vaults: Array<{
    name: string;
    mode: string;
    active: boolean;
  }>;
  results: Array<{
    vault: string;
    path: string;
    snippet: string;
  }>;
};

const workspaces: WorkspaceCase[] = [
  {
    id: "acme",
    label: "~/work/acme",
    cwd: "~/work/acme/payments-api",
    remote: "github.com/acme/payments-api",
    vaults: [
      { name: "acme", mode: "full · propose", active: true },
      { name: "personal", mode: "persona/** · direct", active: true },
      { name: "beta", mode: "isolated", active: false },
    ],
    results: [
      {
        vault: "acme",
        path: "infra/deploy.md",
        snippet: "Production runs on Fly.io (fra). Rollback via fly releases.",
      },
      {
        vault: "personal",
        path: "persona/style.md",
        snippet: "Keep commit messages short. Run tests before proposing edits.",
      },
    ],
  },
  {
    id: "beta",
    label: "~/work/beta",
    cwd: "~/work/beta/storefront",
    remote: "gitlab.com/beta/storefront",
    vaults: [
      { name: "beta", mode: "full · propose", active: true },
      { name: "personal", mode: "persona/** · direct", active: true },
      { name: "acme", mode: "isolated", active: false },
    ],
    results: [
      {
        vault: "beta",
        path: "projects/shipping.md",
        snippet: "Free shipping threshold updated to 115 EUR on 2026-11-23.",
      },
      {
        vault: "personal",
        path: "persona/style.md",
        snippet: "Keep commit messages short. Run tests before proposing edits.",
      },
    ],
  },
  {
    id: "personal",
    label: "~/notes",
    cwd: "~/notes",
    remote: "github.com/you/notes",
    vaults: [
      { name: "personal", mode: "full · direct", active: true },
      { name: "acme", mode: "isolated", active: false },
      { name: "beta", mode: "isolated", active: false },
    ],
    results: [
      {
        vault: "personal",
        path: "projects/memtwig.md",
        snippet: "Single Go binary, SQLite FTS5 + RRF, embedded Studio UI.",
      },
      {
        vault: "personal",
        path: "decisions/2026-11.md",
        snippet: "2026-11-23 memtwig: CGO_ENABLED=0 pure Go build for zero deps.",
      },
    ],
  },
];

const marqueeRowOne = [
  { label: "Claude Code", href: "/integrations/claude-code", dot: "#d5a8f8" },
  { label: "Cursor", href: "/integrations/cursor", dot: "#58cc7d" },
  { label: "OpenAI Codex", href: "/integrations/codex", dot: "#6bb5ff" },
  { label: "Hermes Agent", href: "/integrations/hermes", dot: "#ebb85e" },
  { label: "T3 Code", href: "/integrations/t3-code", dot: "#f09a7a" },
];

const marqueeRowTwo = [
  { label: "Grok Bot", href: "/integrations/grok-bot", dot: "#6bb5ff" },
  { label: "OpenBot", href: "/integrations/openbot", dot: "#d5a8f8" },
  { label: "Antigravity", href: "/integrations/antigravity", dot: "#58cc7d" },
  { label: "Stdio MCP", href: "/mcp-memory", dot: "#ebb85e" },
  { label: "HTTP RBAC", href: "/mcp-memory", dot: "#f09a7a" },
];

export function ProductBento() {
  const [activeWs, setActiveWs] = React.useState("acme");
  const [diffState, setDiffState] = React.useState<"pending" | "approved" | "rejected">("pending");

  return (
    <MotionConfig reducedMotion="user">
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-12">
        {/* Card 1: Live Vault Isolation Switcher */}
        <div className="flex min-w-0 flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-7">
          <div className="rounded-2xl border border-white/[0.07] bg-[#141415] p-4 sm:p-5">
            <Tabs value={activeWs} onValueChange={setActiveWs} className="w-full">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <span className="text-xs font-medium text-zinc-400">Workspace</span>
                <TabsList className="grid w-full grid-cols-3 gap-0.5 sm:inline-flex sm:w-auto">
                  {workspaces.map((ws) => (
                    <TabsTrigger
                      key={ws.id}
                      value={ws.id}
                      className="px-2 text-xs sm:px-3.5 sm:text-sm"
                    >
                      {ws.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </div>

              {workspaces.map((ws) => (
                <TabsContent key={ws.id} value={ws.id} className="mt-4">
                  <motion.div
                    key={activeWs === ws.id ? `ws-${ws.id}` : ws.id}
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.2 }}
                    className="space-y-3.5"
                  >
                    <div className="flex flex-wrap gap-x-5 gap-y-1 text-xs text-zinc-400">
                      <span className="truncate">cwd: {ws.cwd}</span>
                      <span className="truncate">remote: {ws.remote}</span>
                    </div>

                    <div className="grid gap-2.5 sm:grid-cols-3">
                      {ws.vaults.map((v) => (
                        <div
                          key={v.name}
                          className={
                            v.active
                              ? "rounded-xl border border-white/10 bg-[#1c1c1f] px-3.5 py-3"
                              : "rounded-xl border border-white/[0.05] bg-[#17171a] px-3.5 py-3 opacity-50"
                          }
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-medium text-white">{v.name}</span>
                            {v.active ? (
                              <span className="size-2 rounded-full bg-[#58cc7d]" aria-label="active" />
                            ) : (
                              <LockSimple size={14} className="text-zinc-500" aria-label="isolated" />
                            )}
                          </div>
                          <div className="mt-1 text-xs text-zinc-400">{v.mode}</div>
                        </div>
                      ))}
                    </div>

                    <div className="rounded-xl border border-white/[0.06] bg-[#19191c] p-4">
                      <div className="flex items-center justify-between gap-2 text-xs font-medium text-zinc-400">
                        <span>$ memtwig search &quot;deploy&quot;</span>
                        <MagnifyingGlass size={14} className="shrink-0 text-zinc-500" aria-hidden="true" />
                      </div>
                      <div className="mt-3 divide-y divide-white/[0.06]">
                        {ws.results.map((r) => (
                          <div key={`${r.vault}:${r.path}`} className="py-2.5 first:pt-0 last:pb-0">
                            <div className="flex flex-wrap items-baseline gap-2 text-xs sm:text-sm">
                              <span className="font-medium text-[#58cc7d]">{r.vault}</span>
                              <span className="font-medium text-zinc-200 break-all">{r.path}</span>
                            </div>
                            <p className="mt-1 text-xs leading-relaxed text-zinc-400 sm:text-sm">{r.snippet}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  </motion.div>
                </TabsContent>
              ))}
            </Tabs>
          </div>

          <div className="mt-7">
            <h3 className="text-2xl font-medium tracking-[-0.02em] text-white">One vault per client</h3>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-zinc-400 sm:text-base">
              Switch client repositories and memtwig isolates context automatically by directory and Git remote.
            </p>
          </div>
        </div>

        {/* Right Stack: Agent Marquee + Vivid Mint Stored in Plain Git */}
        <div className="flex min-w-0 flex-col gap-5 lg:col-span-5">
          {/* Card 2: Agent Marquee */}
          <div className="flex flex-col justify-between overflow-hidden rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] py-6 sm:py-7">
            <div className="marquee-group marquee-mask overflow-hidden">
              <div className="marquee-track gap-2.5">
                {[...marqueeRowOne, ...marqueeRowOne].map((item, idx) => (
                  <a
                    key={`r1-${idx}-${item.label}`}
                    href={item.href}
                    className="inline-flex shrink-0 items-center gap-2 rounded-full border border-white/10 bg-[#252529] px-4 py-2 text-sm font-medium text-zinc-100 transition-colors hover:border-[#58cc7d]/60 hover:bg-[#2c2c32]"
                  >
                    <span
                      className="size-2 shrink-0 rounded-full"
                      style={{ backgroundColor: item.dot }}
                      aria-hidden="true"
                    />
                    <span>{item.label}</span>
                  </a>
                ))}
              </div>
              <div className="marquee-track-reverse mt-2.5 gap-2.5">
                {[...marqueeRowTwo, ...marqueeRowTwo].map((item, idx) => (
                  <a
                    key={`r2-${idx}-${item.label}`}
                    href={item.href}
                    className="inline-flex shrink-0 items-center gap-2 rounded-full border border-white/10 bg-[#252529] px-4 py-2 text-sm font-medium text-zinc-100 transition-colors hover:border-[#58cc7d]/60 hover:bg-[#2c2c32]"
                  >
                    <span
                      className="size-2 shrink-0 rounded-full"
                      style={{ backgroundColor: item.dot }}
                      aria-hidden="true"
                    />
                    <span>{item.label}</span>
                  </a>
                ))}
              </div>
            </div>

            <div className="mt-6 px-6 sm:px-7">
              <h3 className="text-2xl font-medium tracking-[-0.02em] text-white">Use every agent you run</h3>
              <p className="mt-2 text-sm leading-relaxed text-zinc-400">
                Share one curated memory layer across terminal agents and always-on bots via MCP and native hooks.{" "}
                <a href="/integrations" className="font-medium text-zinc-200 underline hover:text-white">
                  All 8 integrations
                </a>
              </p>
            </div>
          </div>

          {/* Card 3: Vivid Mint Card — Stored in plain Git */}
          <div className="overflow-hidden rounded-[28px] bg-[#58cc7d] p-6 text-[#111312] sm:p-7">
            <div className="grid items-center gap-5 sm:grid-cols-12">
              <div className="sm:col-span-6">
                <h3 className="text-2xl font-medium leading-[1.12] tracking-[-0.025em] text-[#111312]">
                  Stored in plain Git
                </h3>
                <p className="mt-2.5 text-sm leading-relaxed text-[#111312]/80">
                  Notes stay as readable Markdown files in your repos, indexed locally in SQLite FTS5. Zero cloud database.
                </p>
              </div>

              <div className="sm:col-span-6">
                <div className="rounded-2xl border border-black/12 bg-white/45 p-4 text-[#111312] shadow-[0_14px_34px_rgba(0,0,0,0.14)] backdrop-blur-xs">
                  <div className="flex items-center justify-between gap-2 border-b border-black/10 pb-2.5 text-xs font-medium">
                    <span>Your vault</span>
                    <LockSimple size={14} aria-hidden="true" />
                  </div>
                  <ul className="mt-3 space-y-2 text-sm font-medium">
                    {["projects/", "clients/", "decisions/", "persona/"].map((dir) => (
                      <li key={dir} className="flex items-center gap-2.5">
                        <Folder size={16} className="shrink-0 text-[#111312]/75" aria-hidden="true" />
                        <span>{dir}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Card 4: Live Interactive Diff Approval */}
        <div className="flex flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-7">
          <div className="flex min-h-[220px] flex-col justify-between rounded-2xl border border-white/[0.07] bg-[#141415] p-5">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/[0.07] pb-3.5">
              <div className="flex items-center gap-2 text-sm font-medium text-white">
                <LockSimple size={15} className="text-zinc-400" aria-hidden="true" />
                <span>acme / infra/postgres.md</span>
              </div>
              <span className="rounded-full bg-white/[0.07] px-2.5 py-0.5 text-xs font-medium text-zinc-300">
                claude-code
              </span>
            </div>

            <AnimatePresence mode="wait">
              {diffState === "pending" ? (
                <motion.div
                  key="diff-pending"
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.18 }}
                  className="mt-4 space-y-4"
                >
                  <div className="space-y-1.5 text-xs sm:text-sm">
                    <div className="rounded-lg bg-red-500/12 px-3 py-1.5 text-red-300">
                      - region: us-east-1
                    </div>
                    <div className="rounded-lg bg-[#58cc7d]/14 px-3 py-1.5 text-[#7ee09e]">
                      + region: eu-central-1
                    </div>
                    <div className="rounded-lg bg-[#58cc7d]/14 px-3 py-1.5 text-[#7ee09e]">
                      + pool_max_conns: 40
                    </div>
                  </div>
                  <div className="flex items-center justify-end gap-2.5 pt-1">
                    <Button variant="outline" size="sm" onClick={() => setDiffState("rejected")}>
                      Reject
                    </Button>
                    <Button variant="default" size="sm" onClick={() => setDiffState("approved")}>
                      Approve
                    </Button>
                  </div>
                </motion.div>
              ) : (
                <motion.div
                  key={`diff-${diffState}`}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.18 }}
                  className="flex flex-1 flex-col justify-between py-4"
                >
                  <div
                    className={
                      diffState === "approved"
                        ? "flex items-center gap-2.5 rounded-xl border border-[#58cc7d]/30 bg-[#58cc7d]/12 px-4 py-3 text-sm font-medium text-[#7ee09e]"
                        : "flex items-center gap-2.5 rounded-xl border border-white/10 bg-[#1c1c1f] px-4 py-3 text-sm font-medium text-zinc-300"
                    }
                  >
                    {diffState === "approved" && <Check size={17} className="shrink-0 text-[#58cc7d]" />}
                    <span>
                      {diffState === "approved"
                        ? "Committed to Git · acme/infra/postgres.md"
                        : "Proposal dismissed — vault untouched"}
                    </span>
                  </div>
                  <div className="mt-4 flex justify-end">
                    <Button variant="ghost" size="sm" onClick={() => setDiffState("pending")}>
                      <ArrowClockwise size={14} />
                      <span>Reset</span>
                    </Button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          <div className="mt-7">
            <h3 className="text-2xl font-medium tracking-[-0.02em] text-white">Agents propose, you approve</h3>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-zinc-400 sm:text-base">
              Protected org notes never get silently overwritten. Every edit queues as a clean Git diff.
            </p>
          </div>
        </div>

        {/* Card 5: Automated Hygiene Pipeline */}
        <div className="flex flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-5">
          <div>
            <div className="rounded-xl border border-white/[0.07] bg-[#141415] px-4 py-3">
              <div className="text-xs font-medium text-white">Pre-turn hook</div>
              <div className="mt-0.5 text-xs text-zinc-400">Inject top-3 snippets (~400 chars)</div>
            </div>

            <div className="mx-auto h-4 w-px border-l border-dashed border-white/20" aria-hidden="true" />

            <div className="grid grid-cols-2 gap-2.5">
              <div className="rounded-xl border border-white/[0.07] bg-[#141415] p-3">
                <div className="text-xs font-medium text-white">8 KB cap</div>
                <div className="mt-0.5 text-xs text-zinc-400">Auto-archive stale notes</div>
              </div>
              <div className="rounded-xl border border-white/[0.07] bg-[#141415] p-3">
                <div className="text-xs font-medium text-white">Secret scan</div>
                <div className="mt-0.5 text-xs text-zinc-400">Block tokens pre-sync</div>
              </div>
            </div>

            <div className="mx-auto h-4 w-px border-l border-dashed border-white/20" aria-hidden="true" />

            <div className="flex items-center justify-between gap-2 rounded-xl border border-white/[0.07] bg-[#141415] px-4 py-3">
              <div>
                <div className="text-xs font-medium text-white">Curator review</div>
                <div className="mt-0.5 text-xs text-zinc-400">Open diff in Studio UI</div>
              </div>
              <span className="shrink-0 rounded-full bg-white px-3 py-1 text-xs font-medium text-[#141415]">
                Approve
              </span>
            </div>
          </div>

          <div className="mt-7">
            <h3 className="text-2xl font-medium tracking-[-0.02em] text-white">Memory that stays sharp</h3>
            <p className="mt-2 text-sm leading-relaxed text-zinc-400">
              Curation, secret scanning, and append-only archives run locally without bloating prompts.
            </p>
          </div>
        </div>
      </div>
    </MotionConfig>
  );
}
