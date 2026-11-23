import * as React from "react";
import { ArrowsOutSimple, GitDiff, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import type { GraphEdge, GraphNode } from "@/components/GraphView";

type VaultInfo = {
  name: string;
  kind: string;
  default_write: string;
  open_proposals: number;
};

type NoteSummary = {
  vault: string;
  path: string;
  title: string;
  status: string;
  updated: string;
  owner: string;
  size_bytes: number;
};

type ProposalItem = {
  id: string;
  vault: string;
  kind: string;
  path: string;
  reason: string;
  author: string;
  created_at: string;
  diff: string;
};

type AuditEvent = {
  timestamp: string;
  actor: string;
  action: string;
  vault?: string;
  path?: string;
  outcome: string;
};

const BAR_COLORS = ["#e4e4e7", "#94a3b8", "#5eead4", "#a1a1aa"];

export function OverviewView({
  vaults,
  notes,
  nodes,
  edges,
  proposals,
  auditEvents,
  busy,
  onCurate,
  onOpenSection,
  onOpenProposal,
}: {
  vaults: VaultInfo[];
  notes: NoteSummary[];
  nodes: GraphNode[];
  edges: GraphEdge[];
  proposals: ProposalItem[];
  auditEvents: AuditEvent[];
  busy: boolean;
  onCurate: () => void;
  onOpenSection: (sec: "notes" | "graph" | "proposals" | "settings") => void;
  onOpenProposal: (id: string) => void;
}) {
  const noteLinks = React.useMemo(() => edges.filter((e) => e.kind !== "contains"), [edges]);
  const linkedNoteIds = React.useMemo(() => {
    const s = new Set<string>();
    for (const e of noteLinks) {
      s.add(e.source);
      s.add(e.target);
    }
    return s;
  }, [noteLinks]);

  const totalNotes = notes.length;
  const connectedPct = totalNotes > 0 ? Math.round((linkedNoteIds.size / totalNotes) * 100) : 0;
  const totalKB = notes.reduce((acc, n) => acc + n.size_bytes, 0) / 1024;
  const maxVaultNotes = Math.max(1, ...vaults.map((v) => notes.filter((n) => n.vault === v.name).length));

  // Mini graph coordinates for the bottom-left visual card
  const miniGraph = React.useMemo(() => {
    const noteNodes = nodes.filter((n) => n.kind === "note").slice(0, 12);
    const idSet = new Set(noteNodes.map((n) => n.id));
    const placed = noteNodes.map((n, i) => {
      const cols = Math.max(2, Math.ceil(noteNodes.length / 2));
      const row = i % 2;
      const col = Math.floor(i / 2);
      const x = 36 + (col * 280) / Math.max(1, cols - 1);
      const y = row === 0 ? 44 + ((col % 2) * 14) : 112 - ((col % 2) * 14);
      return { ...n, x, y };
    });
    const map = new Map(placed.map((p) => [p.id, p]));
    const links = noteLinks.filter((e) => idSet.has(e.source) && idSet.has(e.target));
    return { placed, map, links };
  }, [nodes, noteLinks]);

  // Dot-matrix heights derived from note sizes (deterministic, real data)
  const dotCols = React.useMemo(() => {
    const cols = 18;
    return Array.from({ length: cols }, (_, i) => {
      const n = notes[i % Math.max(1, notes.length)];
      if (!n) return 1;
      return Math.max(2, Math.min(8, Math.round((n.size_bytes / 180) % 7) + 2));
    });
  }, [notes]);

  // Vertical bars for recent activity / note distribution
  const barHeights = React.useMemo(() => {
    const count = 28;
    return Array.from({ length: count }, (_, i) => {
      const wave = Math.sin((i / count) * Math.PI * 2.2) * 0.4 + 0.55;
      const boost = i > 14 && i < 22 ? 1.25 : 0.85;
      return Math.max(12, Math.min(72, Math.round(wave * boost * 56)));
    });
  }, []);

  return (
    <div className="flex-1 overflow-y-auto px-6 py-6 md:px-10 md:py-8">
      <div className="mx-auto max-w-[1040px] space-y-8">
        {/* Title row */}
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-[28px] font-medium tracking-tight text-foreground">Workspace</h1>
            <p className="mt-1 text-[13px] text-muted-foreground">
              {vaults.length} vaults · {totalNotes} notes · {noteLinks.length} links
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={busy} onClick={onCurate}>
              Run Curate
            </Button>
            <Button size="sm" onClick={() => onOpenSection("settings")}>
              <Plus size={14} />
              Add vault
            </Button>
          </div>
        </div>

        {/* Row 1: Connected metric + Vault bars (left) | Two compact visual metric cards (right) */}
        <div className="grid gap-6 lg:grid-cols-12">
          <div className="space-y-6 lg:col-span-6">
            <div className="flex items-baseline gap-3">
              <span className="text-[52px] font-light leading-none tracking-tight tabular-nums text-foreground">
                {connectedPct}%
              </span>
              <span className="text-xs text-muted-foreground">notes linked in graph</span>
            </div>

            <div className="space-y-3.5 pt-1">
              {vaults.map((v, idx) => {
                const count = notes.filter((n) => n.vault === v.name).length;
                const pct = Math.max(10, Math.round((count / maxVaultNotes) * 100));
                return (
                  <div key={v.name} className="grid grid-cols-[76px_1fr_36px] items-center gap-3 text-xs">
                    <span className="truncate text-muted-foreground">{v.name}</span>
                    <div className="relative h-2 overflow-hidden rounded-[3px] bg-[#1a1b20]">
                      <div
                        className="h-full rounded-[3px]"
                        style={{ width: `${pct}%`, backgroundColor: BAR_COLORS[idx % BAR_COLORS.length] }}
                      />
                    </div>
                    <span className="text-right tabular-nums text-muted-foreground">{count}</span>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:col-span-6">
            {/* Card 1: Dot-matrix (Vault Volume) */}
            <div className="flex flex-col justify-between rounded-lg border border-border bg-[#121317] p-5">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Indexed Volume</span>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Open Notes"
                  className="h-6 w-6"
                  onClick={() => onOpenSection("notes")}
                >
                  <ArrowsOutSimple size={13} />
                </Button>
              </div>
              <div className="my-3 text-center">
                <div className="text-3xl font-normal tracking-tight tabular-nums text-foreground">
                  {totalKB.toFixed(1)} <span className="text-sm text-muted-foreground">KB</span>
                </div>
                <div className="mt-0.5 text-[11px] text-muted-foreground">{totalNotes} active notes</div>
              </div>
              <svg viewBox="0 0 180 48" className="h-12 w-full">
                {dotCols.map((h, ci) =>
                  Array.from({ length: 8 }, (_, ri) => {
                    const active = 8 - ri <= h;
                    return (
                      <circle
                        key={`${ci}-${ri}`}
                        cx={6 + ci * 10}
                        cy={5 + ri * 5.5}
                        r={1.7}
                        fill={active ? "#d4d4d8" : "#23242b"}
                      />
                    );
                  }),
                )}
              </svg>
            </div>

            {/* Card 2: Audit & Proposals */}
            <div className="flex flex-col justify-between rounded-lg border border-border bg-[#121317] p-5">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>Open Queue</span>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Open Proposals"
                  className="h-6 w-6"
                  onClick={() => onOpenSection("proposals")}
                >
                  <ArrowsOutSimple size={13} />
                </Button>
              </div>
              <div className="my-3 text-center">
                <div className="text-3xl font-normal tracking-tight tabular-nums text-foreground">
                  {proposals.length}
                </div>
                <div className="mt-0.5 text-[11px] text-muted-foreground">
                  {auditEvents.length} audit events
                </div>
              </div>
              <svg viewBox="0 0 180 48" className="h-12 w-full">
                {Array.from({ length: 24 }, (_, i) => {
                  const h = 8 + ((i * 13 + proposals.length * 7) % 34);
                  const highlight = i === 16 || i === 17;
                  return (
                    <rect
                      key={i}
                      x={4 + i * 7.2}
                      y={46 - h}
                      width={2.5}
                      height={h}
                      rx={1}
                      fill={highlight ? "#5eead4" : "#2a2b33"}
                    />
                  );
                })}
              </svg>
            </div>
          </div>
        </div>

        {/* Row 2: Two large atmospheric dark visual cards (Knowledge Graph + Activity Spectrum) */}
        <div className="grid gap-5 lg:grid-cols-2">
          {/* Knowledge Graph preview card */}
          <div className="flex flex-col justify-between rounded-lg border border-border bg-gradient-to-br from-[#141a1d] via-[#12141a] to-[#15151c] p-6">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-foreground">Knowledge Graph</span>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Expand Graph"
                className="h-7 w-7"
                onClick={() => onOpenSection("graph")}
              >
                <ArrowsOutSimple size={14} />
              </Button>
            </div>

            <div className="mt-3 flex items-baseline justify-between">
              <span className="text-4xl font-light tracking-tight tabular-nums text-foreground">
                {noteLinks.length}
              </span>
              <span className="text-xs text-muted-foreground">Isolated by vault</span>
            </div>

            <svg viewBox="0 0 360 145" className="mt-4 h-36 w-full">
              {miniGraph.links.map((e, i) => {
                const s = miniGraph.map.get(e.source);
                const t = miniGraph.map.get(e.target);
                if (!s || !t) return null;
                const mx = (s.x + t.x) / 2;
                const my = (s.y + t.y) / 2 - 14;
                return (
                  <path
                    key={i}
                    d={`M ${s.x} ${s.y} Q ${mx} ${my} ${t.x} ${t.y}`}
                    fill="none"
                    stroke={e.kind === "decision" ? "#94a3b8" : "#5eead4"}
                    strokeOpacity={0.75}
                    strokeWidth={1.4}
                    strokeDasharray={e.kind === "decision" ? "3 3" : undefined}
                  />
                );
              })}
              {miniGraph.placed.map((n) => (
                <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
                  <circle r={4.5} fill="#0e1015" stroke="#e4e4e7" strokeWidth={1.5} />
                  <text y={15} textAnchor="middle" fill="#8a8b94" fontSize={8.5}>
                    {n.label.slice(0, 14)}
                  </text>
                </g>
              ))}
            </svg>
          </div>

          {/* Proposals & Hygiene card */}
          <div className="flex flex-col justify-between rounded-lg border border-border bg-gradient-to-br from-[#16151d] via-[#121318] to-[#17151a] p-6">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-foreground">Governance &amp; Diffs</span>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Expand Proposals"
                className="h-7 w-7"
                onClick={() => onOpenSection("proposals")}
              >
                <ArrowsOutSimple size={14} />
              </Button>
            </div>

            <div className="mt-3 flex items-baseline justify-between">
              <span className="text-4xl font-light tracking-tight tabular-nums text-foreground">
                {proposals.length === 0 ? "100%" : `${proposals.length} open`}
              </span>
              <span className="text-xs text-muted-foreground">
                {proposals.length === 0 ? "All reviewed" : "Awaiting review"}
              </span>
            </div>

            {proposals.length > 0 ? (
              <div className="mt-4 space-y-2">
                {proposals.slice(0, 3).map((p) => (
                  <Button
                    key={p.id}
                    variant="outline"
                    onClick={() => onOpenProposal(p.id)}
                    className="h-auto w-full justify-between bg-[#0e0f13]/80 px-3.5 py-2.5 text-left"
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <GitDiff size={14} className="shrink-0 text-muted-foreground" />
                      <span className="truncate text-xs text-foreground">
                        {p.vault} / {p.path}
                      </span>
                    </span>
                    <span className="shrink-0 text-[11px] text-muted-foreground">{p.author}</span>
                  </Button>
                ))}
              </div>
            ) : (
              <svg viewBox="0 0 360 90" className="mt-4 h-28 w-full">
                {barHeights.map((bh, i) => (
                  <rect
                    key={i}
                    x={6 + i * 12.5}
                    y={86 - bh}
                    width={4}
                    height={bh}
                    rx={1.5}
                    fill={i >= 15 && i <= 21 ? "#e4e4e7" : "#272830"}
                  />
                ))}
              </svg>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
