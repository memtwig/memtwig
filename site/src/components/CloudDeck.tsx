import * as React from "react";
import { AnimatePresence, MotionConfig, motion, useInView, useReducedMotion } from "motion/react";
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
};

const cards: CloudCardItem[] = [
  {
    id: "curator",
    num: "01",
    badge: "NIGHTLY HYGIENE",
    title: "Cloud Curator",
    body: "Nightly and on-push LLM hygiene catches stale notes, contradictions, and oversized files as reviewable PRs.",
    bg: "#58cc7d",
  },
  {
    id: "mcp",
    num: "02",
    badge: "HTTP MCP + RBAC",
    title: "Hosted HTTPS MCP",
    body: "Instant https://<org>.memtwig.com/mcp endpoint with per-bot Bearer tokens and append-only audit logs.",
    bg: "#6bb5ff",
  },
  {
    id: "embeddings",
    num: "03",
    badge: "2-WAY GIT SYNC",
    title: "Managed Embeddings",
    body: "Two-way GitHub and GitLab sync keeps 1024-dim semantic search warm on every commit, zero keys needed.",
    bg: "#d5a8f8",
  },
  {
    id: "git",
    num: "04",
    badge: "ZERO LOCK-IN",
    title: "Your Git, Zero Lock-In",
    body: "Every approved diff commits straight to plain Markdown in your repo. Drop back to the local binary anytime.",
    bg: "#ebb85e",
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

const CYCLE_MS = 5500;

export function CloudDeck({ showHeader = true }: { showHeader?: boolean }) {
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [mounted, setMounted] = React.useState(false);
  const [paused, setPaused] = React.useState(false);
  const [manual, setManual] = React.useState(false);

  const deckRef = React.useRef<HTMLDivElement>(null);
  const inView = useInView(deckRef, { amount: 0.35 });
  const prefersReduced = useReducedMotion();

  React.useEffect(() => {
    setMounted(true);
  }, []);

  const shouldAutoPlay = mounted && !prefersReduced && !paused && !manual && inView;

  React.useEffect(() => {
    if (!shouldAutoPlay) return;
    const timer = window.setInterval(() => {
      setActiveIndex((i) => (i + 1) % cards.length);
    }, CYCLE_MS);
    return () => window.clearInterval(timer);
  }, [shouldAutoPlay]);

  const active = cards[activeIndex];
  const nextOne = cards[(activeIndex + 1) % cards.length];
  const nextTwo = cards[(activeIndex + 2) % cards.length];

  const selectCard = (idx: number) => {
    setManual(true);
    setActiveIndex(idx);
  };

  return (
    <MotionConfig reducedMotion="user">
      <div
        ref={deckRef}
        onMouseEnter={() => setPaused(true)}
        onMouseLeave={() => setPaused(false)}
        onFocus={() => setPaused(true)}
        onBlur={() => setPaused(false)}
        className="grid gap-10 lg:grid-cols-12 lg:items-center"
      >
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
                <a href="/cloud#waitlist" className={cn(buttonVariants({ variant: "default", size: "default" }))}>
                  Join waitlist
                </a>
                <a href="/cloud" className={cn(buttonVariants({ variant: "outline", size: "default" }))}>
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
                  onClick={() => selectCard(idx)}
                  className={cn(
                    "relative h-auto w-full justify-start rounded-none px-4 py-3 text-left transition-colors hover:bg-white/[0.04]",
                    selected ? "text-white" : "text-zinc-500 hover:text-zinc-200",
                  )}
                >
                  <span
                    className="absolute top-0 bottom-0 left-0 w-px bg-white/10"
                    aria-hidden="true"
                  />
                  {selected && (
                    <motion.span
                      layoutId="cloud-rail-indicator"
                      transition={{ type: "spring", stiffness: 380, damping: 30 }}
                      className="absolute top-0 bottom-0 left-0 w-0.5"
                      style={{ backgroundColor: card.bg }}
                      aria-hidden="true"
                    />
                  )}
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
            <motion.div
              key={`top-${nextTwo.id}`}
              initial={{ opacity: 0.5, y: 6 }}
              animate={{ opacity: 0.8, y: 0 }}
              transition={{ duration: 0.28 }}
              className="pointer-events-none absolute inset-x-4 top-0 h-12 rounded-[28px]"
              style={{ backgroundColor: nextTwo.bg }}
              aria-hidden="true"
            />

            <Button
              type="button"
              variant="ghost"
              aria-label={`Next card: ${nextOne.title}`}
              onClick={() => selectCard((activeIndex + 1) % cards.length)}
              className="absolute inset-x-4 bottom-0 z-0 h-12 w-auto rounded-[28px] p-0 opacity-85 transition-opacity hover:opacity-100"
              style={{ backgroundColor: nextOne.bg }}
            >
              <span className="sr-only">Next card</span>
            </Button>

            <AnimatePresence mode="wait">
              <motion.div
                key={active.id}
                initial={{ opacity: 0, y: 12, scale: 0.98 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                exit={{ opacity: 0, y: -12, scale: 0.98 }}
                transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
                className="relative z-10 flex min-h-[260px] flex-col justify-between overflow-hidden rounded-[28px] p-7 text-[#111312] shadow-[0_24px_70px_rgba(0,0,0,0.45)] bg-architectural-grid sm:min-h-[370px] sm:p-10"
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
                  <motion.div
                    initial={{ scale: 0.84, rotate: -8, opacity: 0 }}
                    animate={{ scale: 1, rotate: 0, opacity: 1 }}
                    transition={{ type: "spring", stiffness: 300, damping: 22 }}
                    className="self-end sm:self-center"
                  >
                    <DeckIcon id={active.id} />
                  </motion.div>
                </div>

                {shouldAutoPlay && (
                  <motion.div
                    key={`progress-${active.id}`}
                    initial={{ scaleX: 0 }}
                    animate={{ scaleX: 1 }}
                    transition={{ duration: CYCLE_MS / 1000, ease: "linear" }}
                    style={{ transformOrigin: "0% 50%" }}
                    className="absolute right-7 bottom-0 left-7 h-0.5 bg-black/20"
                    aria-hidden="true"
                  />
                )}
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>
    </MotionConfig>
  );
}
