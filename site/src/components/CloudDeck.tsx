import * as React from "react";
import { MotionConfig, motion, useInView, useReducedMotion } from "motion/react";
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
  const [swipeDir, setSwipeDir] = React.useState<1 | -1>(1);
  // Card currently flying from the front to the back of the deck.
  const [leavingId, setLeavingId] = React.useState<string | null>(null);
  // While "out", the leaving card stays above the deck; then it drops behind.
  const [leavingOut, setLeavingOut] = React.useState(false);
  const dropTimer = React.useRef<number | undefined>(undefined);
  const sendToBack = React.useCallback((id: string) => {
    setLeavingId(id);
    setLeavingOut(true);
    window.clearTimeout(dropTimer.current);
    dropTimer.current = window.setTimeout(() => setLeavingOut(false), 270);
  }, []);
  React.useEffect(() => () => window.clearTimeout(dropTimer.current), []);

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
      setActiveIndex((i) => {
        sendToBack(cards[i].id);
        return (i + 1) % cards.length;
      });
    }, CYCLE_MS);
    return () => window.clearInterval(timer);
  }, [shouldAutoPlay]);

  const goTo = (idx: number) => {
    if (idx === activeIndex) return;
    sendToBack(cards[activeIndex].id);
    setActiveIndex(idx);
  };

  const selectCard = (idx: number) => {
    setManual(true);
    goTo(idx);
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

        {/* Right: stacked deck. Every card stays mounted; depth drives position. */}
        <div className="min-w-0 lg:col-span-7">
          <div className="grid pt-14 sm:pt-16" style={{ perspective: 1400 }}>
            {cards.map((card, idx) => {
              const n = cards.length;
              const depth = (idx - activeIndex + n) % n;
              const sentToBack = leavingId === card.id && depth !== 0;
              const isFront = depth === 0;
              const y = -depth * 18;
              const scale = 1 - depth * 0.05;
              const opacity = 1;
              const filter = `brightness(${1 - depth * 0.14})`;

              const animate = sentToBack
                ? {
                    y: [0, 90, y],
                    scale: [1, 0.96, scale],
                    rotate: [0, swipeDir * 5, 0],
                    x: [0, swipeDir * 60, 0],
                    opacity: [1, 1, opacity],
                    filter: ["brightness(1)", "brightness(1)", filter],
                  }
                : { y, scale, rotate: 0, x: 0, opacity, filter };
              const zIndex = sentToBack && leavingOut ? n + 1 : n - depth;

              return (
                <motion.div
                  key={card.id}
                  initial={false}
                  animate={animate}
                  onAnimationComplete={() => {
                    if (sentToBack) setLeavingId(null);
                  }}
                  transition={
                    sentToBack
                      ? { duration: 0.62, times: [0, 0.42, 1], ease: [0.22, 1, 0.36, 1] }
                      : { type: "spring", stiffness: 260, damping: 28, mass: 0.9 }
                  }
                  drag={isFront ? "x" : false}
                  dragConstraints={{ left: 0, right: 0 }}
                  dragElastic={0.55}
                  onDragEnd={(_, info) => {
                    if (Math.abs(info.offset.x) > 80 || Math.abs(info.velocity.x) > 500) {
                      setSwipeDir(info.offset.x < 0 ? -1 : 1);
                      selectCard((activeIndex + 1) % n);
                    }
                  }}
                  onClick={() => {
                    if (!isFront) selectCard(idx);
                  }}
                  aria-hidden={!isFront}
                  className={cn(
                    "relative flex min-h-[260px] flex-col justify-between overflow-hidden rounded-[28px] p-7 text-[#111312] bg-architectural-grid will-change-transform [grid-area:1/1] sm:min-h-[370px] sm:p-10",
                    isFront
                      ? "cursor-grab shadow-[0_24px_70px_rgba(0,0,0,0.45)] active:cursor-grabbing"
                      : "shadow-[0_10px_30px_rgba(0,0,0,0.35)]",
                    !isFront && "cursor-pointer",
                  )}
                  style={{ backgroundColor: card.bg, transformOrigin: "50% 0%", touchAction: "pan-y", zIndex }}
                >
                  <div className="flex items-center justify-between gap-3">
                    <span className="rounded-full bg-black/10 px-3 py-1 text-xs font-medium tracking-wide text-[#111312]/80">
                      {card.badge}
                    </span>
                    <span className="text-xs font-medium tabular-nums text-[#111312]/70">
                      {card.num} / 0{n}
                    </span>
                  </div>

                  <motion.div
                    animate={{ opacity: isFront ? 1 : 0 }}
                    transition={{ duration: isFront ? 0.35 : 0.15, delay: isFront ? 0.12 : 0 }}
                    className="mt-8 flex flex-col-reverse gap-6 sm:flex-row sm:items-end sm:justify-between"
                  >
                    <div className="max-w-md">
                      <h3 className="text-3xl font-medium leading-[1.04] tracking-[-0.035em] text-[#111312] sm:text-5xl">
                        {card.title}
                      </h3>
                      <p className="mt-3 text-sm leading-relaxed text-[#111312]/80 sm:text-base">{card.body}</p>
                    </div>
                    <motion.div
                      animate={isFront ? { scale: 1, rotate: 0 } : { scale: 0.84, rotate: -8 }}
                      transition={{ type: "spring", stiffness: 300, damping: 22, delay: isFront ? 0.15 : 0 }}
                      className="self-end sm:self-center"
                    >
                      <DeckIcon id={card.id} />
                    </motion.div>
                  </motion.div>

                  {isFront && shouldAutoPlay && (
                    <motion.div
                      key={`progress-${card.id}-${activeIndex}`}
                      initial={{ scaleX: 0 }}
                      animate={{ scaleX: 1 }}
                      transition={{ duration: CYCLE_MS / 1000, ease: "linear" }}
                      style={{ transformOrigin: "0% 50%" }}
                      className="absolute right-7 bottom-0 left-7 h-0.5 bg-black/20"
                      aria-hidden="true"
                    />
                  )}
                </motion.div>
              );
            })}
          </div>
        </div>
      </div>
    </MotionConfig>
  );
}
