import * as React from "react";
import { AnimatePresence, MotionConfig, motion } from "motion/react";
import { CloudArrowUp, GitBranch, GitDiff, Graph } from "@phosphor-icons/react";
import { Button, buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type CloudCardItem = {
  id: string;
  num: string;
  badge: string;
  title: string;
  body: string;
  bg: string;
  borderAccent: string;
};

const cards: CloudCardItem[] = [
  {
    id: "curator",
    num: "01",
    badge: "NIGHTLY HYGIENE",
    title: "Cloud Curator",
    body: "Nightly and on-push LLM hygiene catches stale notes, contradictions, and oversized files as reviewable PRs.",
    bg: "#58cc7d",
    borderAccent: "border-[#58cc7d]",
  },
  {
    id: "mcp",
    num: "02",
    badge: "HTTP MCP + RBAC",
    title: "Hosted HTTPS MCP",
    body: "Instant https://<org>.memtwig.com/mcp endpoint with per-bot Bearer tokens and append-only audit logs.",
    bg: "#6bb5ff",
    borderAccent: "border-[#6bb5ff]",
  },
  {
    id: "embeddings",
    num: "03",
    badge: "2-WAY GIT SYNC",
    title: "Managed Embeddings",
    body: "Two-way GitHub and GitLab sync keeps 1024-dim semantic search warm on every commit, zero keys needed.",
    bg: "#d5a8f8",
    borderAccent: "border-[#d5a8f8]",
  },
  {
    id: "git",
    num: "04",
    badge: "ZERO LOCK-IN",
    title: "Your Git, Zero Lock-In",
    body: "Every approved diff commits straight to plain Markdown in your repo. Drop back to the local binary anytime.",
    bg: "#ebb85e",
    borderAccent: "border-[#ebb85e]",
  },
];

function DeckIcon({ id }: { id: string }) {
  const props = {
    weight: "light" as const,
    className: "size-16 sm:size-24 text-[#111312]/80 shrink-0",
    "aria-hidden": true as const,
  };
  if (id === "curator") return <GitDiff {...props} />;
  if (id === "mcp") return <CloudArrowUp {...props} />;
  if (id === "embeddings") return <Graph {...props} />;
  return <GitBranch {...props} />;
}

export function CloudDeck({ showHeader = true }: { showHeader?: boolean }) {
  const [activeIndex, setActiveIndex] = React.useState(0);
  const active = cards[activeIndex];
  const nextOne = cards[(activeIndex + 1) % cards.length];
  const nextTwo = cards[(activeIndex + 2) % cards.length];

  return (
    <MotionConfig reducedMotion="user">
      <div className="grid gap-10 lg:grid-cols-12 lg:items-center">
        {/* Left Navigation Column */}
        <div className="min-w-0 lg:col-span-5">
          {showHeader && (
            <div className="mb-8">
              <h2 className="text-4xl font-medium tracking-[-0.035em] text-white sm:text-5xl">
                Memtwig Cloud
              </h2>
              <p className="mt-3 text-base leading-relaxed text-zinc-400">
                Cloud Curator, hosted HTTPS MCP, and managed Git sync for teams and always-on bots.
              </p>
              <div className="mt-6 flex flex-wrap items-center gap-3">
                <a href="/cloud#waitlist" class={cn(buttonVariants({ variant: "default", size: "default" }))}>
                  Join waitlist
                </a>
                <a href="/cloud" class={cn(buttonVariants({ variant: "outline", size: "default" }))}>
                  See pricing →
                </a>
              </div>
            </div>
          )}

          <div className="space-y-1">
            {cards.map((card, idx) => {
              const selected = idx === activeIndex;
              return (
                <Button
                  key={card.id}
                  type="button"
                  variant="ghost"
                  onClick={() => setActiveIndex(idx)}
                  className={cn(
                    "h-auto w-full justify-start rounded-none border-l-2 px-4 py-3 text-left transition-colors hover:bg-white/[0.04]",
                    selected
                      ? `${card.borderAccent} text-white`
                      : "border-white/10 text-zinc-500 hover:text-zinc-200",
                  )}
                >
                  <span className="w-7 text-xs font-medium tabular-nums opacity-70">{card.num}</span>
                  <span className="text-base font-medium">{card.title}</span>
                </Button>
              );
            })}
          </div>
        </div>

        {/* Right Interactive Stacked Card */}
        <div className="min-w-0 lg:col-span-7">
          <div className="relative py-5">
            <div
              className="pointer-events-none absolute inset-x-4 top-0 h-12 rounded-[28px] opacity-80"
              style={{ backgroundColor: nextTwo.bg }}
              aria-hidden="true"
            />
            <div
              className="pointer-events-none absolute inset-x-4 bottom-0 h-12 rounded-[28px] opacity-85"
              style={{ backgroundColor: nextOne.bg }}
              aria-hidden="true"
            />

            <AnimatePresence mode="wait">
              <motion.div
                key={active.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.22 }}
                className="relative z-10 flex min-h-[260px] flex-col justify-between rounded-[28px] p-7 text-[#111312] shadow-[0_24px_70px_rgba(0,0,0,0.45)] bg-architectural-grid sm:min-h-[370px] sm:p-10"
                style={{ backgroundColor: active.bg }}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="rounded-full bg-black/10 px-3 py-1 text-xs font-medium tracking-wide text-[#111312]/80">
                    {active.badge}
                  </span>
                  <span className="text-xs font-medium tabular-nums text-[#111312]/70">
                    {active.num} / 04
                  </span>
                </div>

                <div className="mt-8 flex flex-col-reverse gap-6 sm:flex-row sm:items-end sm:justify-between">
                  <div className="max-w-md">
                    <h3 className="text-3xl font-medium leading-[1.04] tracking-[-0.035em] text-[#111312] sm:text-5xl">
                      {active.title}
                    </h3>
                    <p className="mt-3 text-sm leading-relaxed text-[#111312]/80 sm:text-base">
                      {active.body}
                    </p>
                  </div>
                  <div className="self-end sm:self-center">
                    <DeckIcon id={active.id} />
                  </div>
                </div>
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>
    </MotionConfig>
  );
}
