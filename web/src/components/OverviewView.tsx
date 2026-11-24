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

const BAR_COLORS = ["#e4e4e7", "#94a3b8", "#58cc7d", "#a1a1aa"];

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

  // Mini graph coordinates for the bottom-left visual card: pick highest-degree hub + connected neighbors
  const miniGraph = React.useMemo(() => {
    const allNoteNodes = nodes.filter((n) => n.kind === "note");
    if (allNoteNodes.length === 0) {
      return { placed: [], map: new Map(), links: [] as GraphEdge[], hubId: "", vault: "" };
    }
    const nodeMap = new Map(allNoteNodes.map((n) => [n.id, n]));
    const deg = new Map<string, number>();
    const adj = new Map<string, Set<string>>();

    for (const e of noteLinks) {
      if (!nodeMap.has(e.source) || !nodeMap.has(e.target)) continue;
      deg.set(e.source, (deg.get(e.source) ?? 0) + 1);
      deg.set(e.target, (deg.get(e.target) ?? 0) + 1);
      if (!adj.has(e.source)) adj.set(e.source, new Set());
      if (!adj.has(e.target)) adj.set(e.target, new Set());
      adj.get(e.source)!.add(e.target);
      adj.get(e.target)!.add(e.source);
    }

    const byDegDesc = [...allNoteNodes].sort(
      (a, b) => (deg.get(b.id) ?? 0) - (deg.get(a.id) ?? 0) || a.label.localeCompare(b.label),
    );

    const picked: GraphNode[] = [];
    const pickedIds = new Set<string>();
    const maxNodes = Math.min(10, byDegDesc.length);

    if (byDegDesc.length > 0) {
      const seed = byDegDesc[0];
      picked.push(seed);
      pickedIds.add(seed.id);
      // Stay inside the hub's vault: the card shows one isolated cluster, never a cross-vault mix.
      const sameVault = (id: string) => nodeMap.get(id)?.vault === seed.vault;
      const queue: string[] = [seed.id];
      while (queue.length > 0 && picked.length < maxNodes) {
        const cur = queue.shift()!;
        const neighbors = Array.from(adj.get(cur) ?? [])
          .filter((id) => !pickedIds.has(id) && nodeMap.has(id) && sameVault(id))
          .sort((a, b) => (deg.get(b) ?? 0) - (deg.get(a) ?? 0));
        for (const nid of neighbors) {
          if (picked.length >= maxNodes) break;
          pickedIds.add(nid);
          picked.push(nodeMap.get(nid)!);
          queue.push(nid);
        }
      }
    }

    // Fan layout: hub on the left, its cluster in one column on the right. Labels sit to the
    // right of every node, edges stay in the gap between hub and column, so nothing overlaps.
    const hub = picked[0];
    const column = picked.slice(1, 7);
    const HX = 140;
    const CX = 228;
    const top = 16;
    const bottom = 128;
    const step = column.length > 1 ? Math.min(26, (bottom - top) / (column.length - 1)) : 0;
    const startY = 72 - (step * (column.length - 1)) / 2;
    const placed = [
      ...(hub ? [{ ...hub, x: HX, y: 72, anchor: "end" as const }] : []),
      ...column.map((n, i) => ({ ...n, x: CX, y: startY + i * step, anchor: "start" as const })),
    ];
    const map = new Map(placed.map((p) => [p.id, p]));
    const placedIds = new Set(placed.map((p) => p.id));
    const links = noteLinks.filter((e) => placedIds.has(e.source) && placedIds.has(e.target));
    return { placed, map, links, hubId: hub?.id ?? "", vault: hub?.vault ?? "" };
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
              {vaults.length} {vaults.length === 1 ? "vault" : "vaults"} · {totalNotes}{" "}
              {totalNotes === 1 ? "note" : "notes"} · {noteLinks.length}{" "}
              {noteLinks.length === 1 ? "link" : "links"}
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
                <div className="mt-0.5 text-[11px] text-muted-foreground">
                  {totalNotes} active {totalNotes === 1 ? "note" : "notes"}
                </div>
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
                  {auditEvents.length} audit {auditEvents.length === 1 ? "event" : "events"}
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
                      fill={highlight ? "#58cc7d" : "#2a2b33"}
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
              <span className="text-xs text-muted-foreground">
                {noteLinks.length === 1 ? "link" : "links"} · {vaults.length} isolated{" "}
                {vaults.length === 1 ? "vault" : "vaults"}
              </span>
            </div>

            <svg viewBox="0 0 360 145" className="mt-4 h-36 w-full" role="img" aria-label="Most connected notes">
              {miniGraph.links.map((e, i) => {
                const s = miniGraph.map.get(e.source);
                const t = miniGraph.map.get(e.target);
                if (!s || !t) return null;
                const decision = e.kind === "decision";
                const stroke = decision ? "#8a8b94" : "#58cc7d";
                if (s.x === t.x) {
                  // column-to-column: arc bulging toward the hub, away from the labels
                  const bulge = 14 + Math.abs(t.y - s.y) * 0.35;
                  return (
                    <path
                      key={i}
                      d={`M ${s.x} ${s.y} C ${s.x - bulge} ${s.y}, ${t.x - bulge} ${t.y}, ${t.x} ${t.y}`}
                      fill="none"
                      stroke={stroke}
                      strokeOpacity={0.4}
                      strokeWidth={1}
                      strokeDasharray={decision ? "3 3" : undefined}
                    />
                  );
                }
                const [a, b] = s.x < t.x ? [s, t] : [t, s];
                const mx = (a.x + b.x) / 2;
                return (
                  <path
                    key={i}
                    d={`M ${a.x} ${a.y} C ${mx} ${a.y}, ${mx} ${b.y}, ${b.x} ${b.y}`}
                    fill="none"
                    stroke={stroke}
                    strokeOpacity={decision ? 0.55 : 0.75}
                    strokeWidth={1.3}
                    strokeDasharray={decision ? "3 3" : undefined}
                  />
                );
              })}
              {miniGraph.placed.map((n) => {
                const isHub = n.id === miniGraph.hubId;
                const label = n.label.length > 22 ? `${n.label.slice(0, 21)}…` : n.label;
                return (
                  <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
                    <circle
                      r={isHub ? 5.5 : 3.6}
                      fill={isHub ? "#58cc7d" : "#0e1015"}
                      stroke={isHub ? "#58cc7d" : "#d4d4d8"}
                      strokeWidth={1.3}
                    />
                    <text
                      x={isHub ? -11 : 9}
                      y={3.2}
                      textAnchor={n.anchor}
                      fill={isHub ? "#f4f4f5" : "#a1a1aa"}
                      fontSize={isHub ? 10 : 9.5}
                    >
                      {label}
                    </text>
                    {isHub && (
                      <text x={-11} y={16} textAnchor="end" fill="#6b6b75" fontSize={8.5}>
                        {miniGraph.vault}
                      </text>
                    )}
                  </g>
                );
              })}
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
