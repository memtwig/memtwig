import * as React from "react";
import {
  AnimatePresence,
  MotionConfig,
  motion,
  useInView,
  useMotionValue,
  useReducedMotion,
  useSpring,
} from "motion/react";
import {
  ArrowClockwise,
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

const diffLines = [
  { text: "- region: us-east-1", cls: "bg-red-500/12 text-red-300" },
  { text: "+ region: eu-central-1", cls: "bg-[#58cc7d]/14 text-[#7ee09e]" },
  { text: "+ pool_max_conns: 40", cls: "bg-[#58cc7d]/14 text-[#7ee09e]" },
];

/** Centre of column i in an n-column grid with `gap` px gutters, as a CSS length. */
function colCenter(i: number, n: number, gap: number): string {
  if (n === 1) return "50%";
  const col = `((100% - ${(n - 1) * gap}px) / ${n})`;
  return `calc(${col} * ${i + 0.5} + ${i * gap}px)`;
}

/**
 * Orthogonal routing rails between a single node and a row of n columns.
 * fork: one source above fans out to the columns below. merge: the reverse.
 * Inactive columns get a dark dashed rail and no signal.
 */
function RailFork({
  active,
  gap = 10,
  merge = false,
  delay = 0,
  className = "h-6",
}: {
  active: boolean[];
  gap?: number;
  merge?: boolean;
  delay?: number;
  className?: string;
}) {
  const n = active.length;
  const centers = active.map((_, i) => colCenter(i, n, gap));
  const mid = n % 2 === 1 ? Math.floor(n / 2) : -1;
  const stemDelay = merge ? delay + 1.4 : delay;
  const busDelay = delay + 0.7;
  const dropDelay = merge ? delay : delay + 1.4;
  return (
    <div className={`relative ${className}`} aria-hidden="true">
      <span
        className={`rail-v rail-on left-1/2 ${merge ? "top-1/2 bottom-0" : "top-0 bottom-1/2"}`}
      >
        <span className="rail-dot" style={{ ["--delay" as string]: `${stemDelay}s` }} />
      </span>
      {centers.map((c, i) => {
        const on = active[i];
        const rail = on ? "rail-on" : "rail-off";
        const left = i < n / 2 - (n % 2 ? 0.5 : 0);
        const showBus = i !== mid;
        // Fork: signal leaves the centre; merge: it travels toward the centre.
        const towardEdge = !merge;
        const busRev = left ? towardEdge : !towardEdge;
        return (
          <React.Fragment key={i}>
            {showBus && (
              <span
                className={`rail-h ${rail} top-1/2`}
                style={left ? { left: c, right: "50%" } : { left: "50%", right: `calc(100% - ${c})` }}
              >
                {on && (
                  <span
                    className={`rail-dot ${busRev ? "rail-rev" : ""}`}
                    style={{ ["--delay" as string]: `${busDelay}s` }}
                  />
                )}
              </span>
            )}
            <span
              className={`rail-v ${rail} ${merge ? "top-0 bottom-1/2" : "top-1/2 bottom-0"}`}
              style={{ left: c }}
            >
              {on && <span className="rail-dot" style={{ ["--delay" as string]: `${dropDelay}s` }} />}
            </span>
          </React.Fragment>
        );
      })}
    </div>
  );
}

export function ProductBento() {
  const [activeWs, setActiveWs] = React.useState("acme");
  const [mounted, setMounted] = React.useState(false);
  const [wsPaused, setWsPaused] = React.useState(false);
  const [wsInteracted, setWsInteracted] = React.useState(false);
  const [diffState, setDiffState] = React.useState<"pending" | "approved" | "rejected">("pending");

  const wsRef = React.useRef<HTMLDivElement>(null);
  const wsInView = useInView(wsRef, { amount: 0.35 });
  const prefersReduced = useReducedMotion();

  React.useEffect(() => {
    setMounted(true);
  }, []);

  const reduceMotion = mounted && Boolean(prefersReduced);

  React.useEffect(() => {
    if (!mounted || prefersReduced || wsPaused || wsInteracted || !wsInView) return;
    const timer = window.setInterval(() => {
      setActiveWs((cur) => {
        const idx = workspaces.findIndex((w) => w.id === cur);
        return workspaces[(idx + 1) % workspaces.length].id;
      });
    }, 4500);
    return () => window.clearInterval(timer);
  }, [mounted, prefersReduced, wsPaused, wsInteracted, wsInView]);

  // Mouse-only subtle 3D tilt for the Stored in plain Git frosted file card
  const tiltX = useMotionValue(0);
  const tiltY = useMotionValue(0);
  const smoothRotateX = useSpring(tiltX, { stiffness: 260, damping: 24 });
  const smoothRotateY = useSpring(tiltY, { stiffness: 260, damping: 24 });

  const handleVaultPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (reduceMotion || e.pointerType !== "mouse") return;
    const rect = e.currentTarget.getBoundingClientRect();
    const px = (e.clientX - rect.left) / rect.width - 0.5;
    const py = (e.clientY - rect.top) / rect.height - 0.5;
    tiltX.set(-py * 8);
    tiltY.set(px * 8);
  };

  const resetVaultTilt = () => {
    tiltX.set(0);
    tiltY.set(0);
  };

  return (
    <MotionConfig reducedMotion="user">
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-12">
        {/* Card 1: Live Vault Isolation Switcher */}
        <motion.div
          ref={wsRef}
          initial={{ opacity: 0, y: 18 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: "some" }}
          transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
          onMouseEnter={() => setWsPaused(true)}
          onMouseLeave={() => setWsPaused(false)}
          onFocus={() => setWsPaused(true)}
          onBlur={() => setWsPaused(false)}
          className="flex min-w-0 flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-7"
        >
          <div className="rounded-2xl border border-white/[0.07] bg-[#141415] p-4 sm:p-5">
            <Tabs
              value={activeWs}
              onValueChange={(next) => {
                setWsInteracted(true);
                setActiveWs(next);
              }}
              className="w-full"
            >
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <span className="text-xs font-medium text-zinc-400">Workspace</span>
                <TabsList className="grid w-full grid-cols-3 gap-0.5 sm:inline-flex sm:w-auto">
                  {workspaces.map((ws) => (
                    <TabsTrigger
                      key={ws.id}
                      value={ws.id}
                      className="relative z-10 px-2 text-xs text-zinc-400 transition-colors data-[state=active]:bg-transparent data-[state=active]:text-[#141415] data-[state=active]:shadow-none sm:px-3.5 sm:text-sm"
                    >
                      {activeWs === ws.id && (
                        <motion.span
                          layoutId="bento-ws-pill"
                          transition={{ type: "spring", stiffness: 420, damping: 32 }}
                          className="absolute inset-0 -z-10 rounded-full bg-white"
                          aria-hidden="true"
                        />
                      )}
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
                    transition={{ duration: 0.22 }}
                    className="space-y-3.5"
                  >
                    <div className="flex flex-wrap gap-x-5 gap-y-1 text-xs text-zinc-400 sm:justify-center">
                      <span className="truncate">cwd: {ws.cwd}</span>
                      <span className="truncate">remote: {ws.remote}</span>
                    </div>

                    <div className="hidden sm:block">
                      <RailFork key={ws.id} active={ws.vaults.map((v) => v.active)} className="-my-1 h-7" />
                    </div>

                    <div className="grid gap-2.5 sm:grid-cols-3">
                      {ws.vaults.map((v, idx) => (
                        <motion.div
                          key={v.name}
                          initial={{ opacity: 0, y: 6 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ duration: 0.2, delay: idx * 0.04 }}
                          className={
                            v.active
                              ? "rounded-xl border border-white/10 bg-[#1c1c1f] px-3.5 py-3"
                              : "rounded-xl border border-white/[0.05] bg-[#17171a] px-3.5 py-3 opacity-50"
                          }
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-medium text-white">{v.name}</span>
                            {v.active ? (
                              <span className="relative flex size-2" aria-label="active">
                                <span className="motion-safe:animate-ping absolute inline-flex h-full w-full rounded-full bg-[#58cc7d] opacity-60" />
                                <span className="relative inline-flex size-2 rounded-full bg-[#58cc7d]" />
                              </span>
                            ) : (
                              <LockSimple size={14} className="text-zinc-500" aria-label="isolated" />
                            )}
                          </div>
                          <div className="mt-1 text-xs text-zinc-400">{v.mode}</div>
                        </motion.div>
                      ))}
                    </div>

                    <div className="rounded-xl border border-white/[0.06] bg-[#19191c] p-4">
                      <div className="flex items-center justify-between gap-2 text-xs font-medium text-zinc-400">
                        <span>$ memtwig search &quot;deploy&quot;</span>
                        <MagnifyingGlass size={14} className="shrink-0 text-zinc-500" aria-hidden="true" />
                      </div>
                      <div className="mt-3 divide-y divide-white/[0.06]">
                        {ws.results.map((r, rIdx) => (
                          <motion.div
                            key={`${r.vault}:${r.path}`}
                            initial={{ opacity: 0, y: 4 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ duration: 0.2, delay: 0.08 + rIdx * 0.05 }}
                            className="py-2.5 first:pt-0 last:pb-0"
                          >
                            <div className="flex flex-wrap items-baseline gap-2 text-xs sm:text-sm">
                              <span className="font-medium text-[#58cc7d]">{r.vault}</span>
                              <span className="font-medium text-zinc-200 break-all">{r.path}</span>
                            </div>
                            <p className="mt-1 text-xs leading-relaxed text-zinc-400 sm:text-sm">{r.snippet}</p>
                          </motion.div>
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
        </motion.div>

        {/* Right Stack: Agent Marquee + Vivid Mint Stored in Plain Git */}
        <div className="flex min-w-0 flex-col gap-5 lg:col-span-5">
          {/* Card 2: Agent Marquee */}
          <motion.div
            initial={{ opacity: 0, y: 18 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, amount: "some" }}
            transition={{ duration: 0.4, delay: 0.06, ease: [0.22, 1, 0.36, 1] }}
            className="flex flex-col justify-between overflow-hidden rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] py-6 sm:py-7"
          >
            <div className="marquee-group marquee-mask overflow-hidden">
              <div className="marquee-track gap-2.5">
                {[...marqueeRowOne, ...marqueeRowOne].map((item, idx) => (
                  <a
                    key={`r1-${idx}-${item.label}`}
                    href={item.href}
                    className="inline-flex shrink-0 items-center gap-2 rounded-full border border-white/10 bg-[#252529] px-4 py-2 text-sm font-medium text-zinc-100 transition-all duration-200 hover:-translate-y-0.5 hover:scale-[1.03] hover:border-[#58cc7d]/60 hover:bg-[#2c2c32]"
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
                    className="inline-flex shrink-0 items-center gap-2 rounded-full border border-white/10 bg-[#252529] px-4 py-2 text-sm font-medium text-zinc-100 transition-all duration-200 hover:-translate-y-0.5 hover:scale-[1.03] hover:border-[#58cc7d]/60 hover:bg-[#2c2c32]"
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
          </motion.div>

          {/* Card 3: Vivid Mint Card — Stored in plain Git */}
          <motion.div
            initial={{ opacity: 0, y: 18 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, amount: "some" }}
            transition={{ duration: 0.4, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
            onPointerMove={handleVaultPointerMove}
            onPointerLeave={resetVaultTilt}
            className="group overflow-hidden rounded-[28px] bg-[#58cc7d] p-6 text-[#111312] sm:p-7"
          >
            <div className="grid items-center gap-5 sm:grid-cols-12">
              <div className="sm:col-span-6">
                <h3 className="text-2xl font-medium leading-[1.12] tracking-[-0.025em] text-[#111312]">
                  Stored in plain Git
                </h3>
                <p className="mt-2.5 text-sm leading-relaxed text-[#111312]/80">
                  Notes stay as readable Markdown files in your repos, indexed locally in SQLite FTS5. Zero cloud database.
                </p>
              </div>

              <div className="sm:col-span-6" style={{ perspective: "900px" }}>
                <motion.div
                  style={
                    reduceMotion
                      ? undefined
                      : { rotateX: smoothRotateX, rotateY: smoothRotateY }
                  }
                  className="rounded-2xl border border-black/12 bg-white/45 p-4 text-[#111312] shadow-[0_14px_34px_rgba(0,0,0,0.14)] backdrop-blur-xs"
                >
                  <div className="flex items-center justify-between gap-2 border-b border-black/10 pb-2.5 text-xs font-medium">
                    <span>Your vault</span>
                    <LockSimple size={14} aria-hidden="true" />
                  </div>
                  <ul className="mt-3 space-y-2 text-sm font-medium">
                    {["projects/", "clients/", "decisions/", "persona/"].map((dir) => (
                      <li
                        key={dir}
                        className="flex items-center gap-2.5 transition-transform duration-200 group-hover:translate-x-0.5"
                      >
                        <Folder size={16} className="shrink-0 text-[#111312]/75" aria-hidden="true" />
                        <span>{dir}</span>
                      </li>
                    ))}
                  </ul>
                </motion.div>
              </div>
            </div>
          </motion.div>
        </div>

        {/* Card 4: Live Interactive Diff Approval */}
        <motion.div
          initial={{ opacity: 0, y: 18 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: "some" }}
          transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
          className="flex min-w-0 flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-7"
        >
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
                    {diffLines.map((line, i) => (
                      <motion.div
                        key={line.text}
                        initial={{ opacity: 0, x: -6 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ duration: 0.2, delay: i * 0.05 }}
                        className={`rounded-lg px-3 py-1.5 ${line.cls}`}
                      >
                        {line.text}
                      </motion.div>
                    ))}
                  </div>
                  <div className="flex items-center justify-end gap-2.5 pt-1">
                    <motion.div whileTap={{ scale: 0.96 }}>
                      <Button variant="outline" size="sm" onClick={() => setDiffState("rejected")}>
                        Reject
                      </Button>
                    </motion.div>
                    <motion.div whileTap={{ scale: 0.96 }}>
                      <Button variant="default" size="sm" onClick={() => setDiffState("approved")}>
                        Approve
                      </Button>
                    </motion.div>
                  </div>
                </motion.div>
              ) : (
                <motion.div
                  key={`diff-${diffState}`}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.2 }}
                  className="flex flex-1 flex-col justify-between py-4"
                >
                  <div
                    className={
                      diffState === "approved"
                        ? "flex items-center gap-2.5 rounded-xl border border-[#58cc7d]/30 bg-[#58cc7d]/12 px-4 py-3 text-sm font-medium text-[#7ee09e]"
                        : "flex items-center gap-2.5 rounded-xl border border-white/10 bg-[#1c1c1f] px-4 py-3 text-sm font-medium text-zinc-300"
                    }
                  >
                    {diffState === "approved" && (
                      <svg
                        viewBox="0 0 20 20"
                        fill="none"
                        className="size-4 shrink-0 text-[#58cc7d]"
                        aria-hidden="true"
                      >
                        <motion.path
                          d="M4.5 10.5L8.2 14.2L15.8 6.5"
                          stroke="currentColor"
                          strokeWidth="2.2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          initial={{ pathLength: 0 }}
                          animate={{ pathLength: 1 }}
                          transition={{ duration: 0.32, ease: "easeOut" }}
                        />
                      </svg>
                    )}
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
        </motion.div>

        {/* Card 5: Automated Hygiene Pipeline */}
        <motion.div
          initial={{ opacity: 0, y: 18 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: "some" }}
          transition={{ duration: 0.4, delay: 0.08, ease: [0.22, 1, 0.36, 1] }}
          className="flex min-w-0 flex-col justify-between rounded-[28px] border border-white/[0.08] bg-[#1c1c1f] p-6 sm:p-8 lg:col-span-5"
        >
          <div>
            <div className="flex items-center justify-between gap-2 rounded-xl border border-white/[0.07] bg-[#141415] px-4 py-3">
              <div>
                <div className="text-xs font-medium text-white">Pre-turn hook</div>
                <div className="mt-0.5 text-xs text-zinc-400">Inject top-3 snippets (~400 chars)</div>
              </div>
              <span className="relative flex size-2 shrink-0" aria-hidden="true">
                <span className="motion-safe:animate-ping absolute inline-flex h-full w-full rounded-full bg-[#58cc7d] opacity-60" />
                <span className="relative inline-flex size-2 rounded-full bg-[#58cc7d]" />
              </span>
            </div>

            <RailFork active={[true, true]} className="h-7" />

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

            <RailFork active={[true, true]} merge delay={2.1} className="h-7" />

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
        </motion.div>
      </div>
    </MotionConfig>
  );
}
