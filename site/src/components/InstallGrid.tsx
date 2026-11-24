import * as React from "react";
import { MotionConfig, motion } from "motion/react";
import { Code, Desktop, GitFork, PlugsConnected, Terminal } from "@phosphor-icons/react";

const releasesUrl = "https://github.com/memtwig/memtwig/releases";

type CardSpec = {
  id: string;
  badge: string;
  title: string;
  body: string;
  bg: string;
  span: string;
  badgeClass?: string;
  links: Array<{ label: string; href: string; external?: boolean }>;
};

const installCards: CardSpec[] = [
  {
    id: "claude",
    badge: "HOOKS + MCP",
    title: "Claude Code",
    body: "Native UserPromptSubmit & SessionStart hooks wired with one command.",
    bg: "#d5a8f8",
    span: "md:col-span-2 min-h-[240px]",
    links: [{ label: "Setup guide →", href: "/integrations/claude-code" }],
  },
  {
    id: "cursor-codex",
    badge: "STDIO MCP",
    title: "Cursor & Codex",
    body: "Shared multi-org vaults without bloated rules files copied per repo.",
    bg: "#58cc7d",
    span: "md:col-span-2 min-h-[240px]",
    links: [
      { label: "Cursor →", href: "/integrations/cursor" },
      { label: "Codex →", href: "/integrations/codex" },
    ],
  },
  {
    id: "hermes-t3",
    badge: "WORKTREE SAFE",
    title: "Hermes & T3 Code",
    body: "One SOUL.md identity and Git-remote vault routing across providers.",
    bg: "#ebb85e",
    span: "md:col-span-2 min-h-[240px]",
    links: [
      { label: "Hermes →", href: "/integrations/hermes" },
      { label: "T3 Code →", href: "/integrations/t3-code" },
    ],
  },
  {
    id: "grok-openbot",
    badge: "HTTP RBAC",
    title: "Grok Bot & OpenBot",
    body: "Scoped Bearer tokens, path globs, and append-only audit logs for always-on bots.",
    bg: "#6bb5ff",
    span: "md:col-span-3 min-h-[230px]",
    links: [
      { label: "Grok Bot →", href: "/integrations/grok-bot" },
      { label: "OpenBot →", href: "/integrations/openbot" },
    ],
  },
  {
    id: "desktop-os",
    badge: "v1.6.0",
    title: "macOS, Linux & Windows",
    body: "Zero runtime dependencies, Antigravity PreInvocation hooks, embedded Studio UI & TUI.",
    bg: "#f09a7a",
    span: "md:col-span-3 min-h-[230px]",
    badgeClass: "tabular-nums",
    links: [
      { label: "Prebuilt releases ↗", href: releasesUrl, external: true },
      { label: "All 8 integrations →", href: "/integrations" },
    ],
  },
];

function CardIcon({ id }: { id: string }) {
  const cls = "text-[#111312]/80 transition-transform duration-200 group-hover:scale-110 group-hover:-rotate-3";
  if (id === "claude") return <Terminal size={30} weight="regular" className={cls} aria-hidden="true" />;
  if (id === "cursor-codex") return <Code size={30} weight="regular" className={cls} aria-hidden="true" />;
  if (id === "hermes-t3") return <GitFork size={30} weight="regular" className={cls} aria-hidden="true" />;
  if (id === "grok-openbot") return <PlugsConnected size={30} weight="regular" className={cls} aria-hidden="true" />;
  return <Desktop size={30} weight="regular" className={cls} aria-hidden="true" />;
}

export function InstallGrid() {
  return (
    <MotionConfig reducedMotion="user">
      <div className="grid gap-4 md:grid-cols-6">
        {installCards.map((card, idx) => (
          <motion.div
            key={card.id}
            initial={{ opacity: 0, y: 18 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, amount: "some" }}
            transition={{
              duration: 0.35,
              delay: idx * 0.06,
              ease: [0.22, 1, 0.36, 1],
            }}
            whileHover={{ y: -5 }}
            className={`group flex flex-col justify-between rounded-[26px] p-7 text-[#111312] sm:p-8 ${card.span}`}
            style={{ backgroundColor: card.bg }}
          >
            <div className="flex items-start justify-between gap-2">
              <CardIcon id={card.id} />
              <span
                className={`rounded-full bg-black/10 px-3 py-1 text-xs font-medium text-[#111312]/80 ${card.badgeClass ?? ""}`}
              >
                {card.badge}
              </span>
            </div>
            <div className="mt-8">
              <h3 className="text-2xl font-medium tracking-[-0.025em] text-[#111312]">
                {card.title}
              </h3>
              <p className="mt-2 max-w-md text-sm leading-relaxed text-[#111312]/80">
                {card.body}
              </p>
              <div className="mt-5 flex flex-wrap gap-5 text-sm font-medium">
                {card.links.map((link) => (
                  <a
                    key={link.href}
                    href={link.href}
                    target={link.external ? "_blank" : undefined}
                    rel={link.external ? "noreferrer" : undefined}
                    className="hover:underline"
                  >
                    {link.label}
                  </a>
                ))}
              </div>
            </div>
          </motion.div>
        ))}
      </div>
    </MotionConfig>
  );
}
