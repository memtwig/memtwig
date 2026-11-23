import * as React from "react";
import { ArrowUpRight, GitDiff, LockSimple, MagnifyingGlassMinus, MagnifyingGlassPlus, ArrowsCounterClockwise } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export type GraphNode = {
  id: string;
  kind: "vault" | "note";
  vault: string;
  vault_kind: string;
  label: string;
  path?: string;
  folder?: string;
  status?: string;
  owner?: string;
  default_write?: string;
  size_bytes?: number;
  open_proposals?: number;
};

export type GraphEdge = {
  source: string;
  target: string;
  kind: "contains" | "link" | "decision";
};

type PositionedNode = GraphNode & {
  x: number;
  y: number;
  color: string;
};

type ClusterBox = {
  vault: string;
  vaultKind: string;
  defaultWrite: string;
  x: number;
  y: number;
  w: number;
  h: number;
  color: string;
};

const VAULT_COLORS = ["#2dd4bf", "#38bdf8", "#a78bfa", "#fb7185", "#34d399"];

function colorForVault(vault: string, vaultKind: string, index: number): string {
  if (vaultKind === "personal") return "#f59e0b";
  return VAULT_COLORS[index % VAULT_COLORS.length];
}

function truncateLabel(s: string, max = 26): string {
  return s.length > max ? s.slice(0, max - 1) + "…" : s;
}

export function GraphView({
  nodes,
  edges,
  vaultFilter,
  onOpenNote,
  onOpenProposals,
}: {
  nodes: GraphNode[];
  edges: GraphEdge[];
  vaultFilter: string;
  onOpenNote: (vault: string, path: string) => void;
  onOpenProposals: (vault: string) => void;
}) {
  const [hoveredId, setHoveredId] = React.useState<string | null>(null);
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [zoom, setZoom] = React.useState(1);
  const [pan, setPan] = React.useState({ x: 0, y: 0 });
  const dragRef = React.useRef<{ x: number; y: number; px: number; py: number } | null>(null);

  const filteredNodes = React.useMemo(
    () => (vaultFilter === "all" ? nodes : nodes.filter((n) => n.vault === vaultFilter)),
    [nodes, vaultFilter],
  );
  const visibleIds = React.useMemo(() => new Set(filteredNodes.map((n) => n.id)), [filteredNodes]);
  const filteredEdges = React.useMemo(
    () => edges.filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target)),
    [edges, visibleIds],
  );

  // Deterministic, crystal-clear clustered layout: each vault occupies its own isolated territory
  const { positioned, clusters, viewW, viewH } = React.useMemo(() => {
    const vaultNodes = filteredNodes.filter((n) => n.kind === "vault").sort((a, b) => a.vault.localeCompare(b.vault));
    const notesByVault = new Map<string, GraphNode[]>();
    for (const n of filteredNodes) {
      if (n.kind === "note") {
        const list = notesByVault.get(n.vault) ?? [];
        list.push(n);
        notesByVault.set(n.vault, list);
      }
    }

    const pos: PositionedNode[] = [];
    const boxes: ClusterBox[] = [];
    const colGap = 44;
    const padX = 36;
    const padY = 44;
    let cursorX = padX;
    let maxH = 440;

    vaultNodes.forEach((vn, idx) => {
      const col = colorForVault(vn.vault, vn.vault_kind, idx);
      const vNotes = (notesByVault.get(vn.vault) ?? []).sort((a, b) => (a.path ?? "").localeCompare(b.path ?? ""));
      const radius = Math.max(120, Math.min(200, 92 + vNotes.length * 18));
      const boxW = radius * 2 + 190;
      const boxH = radius * 2 + 128;
      const cx = cursorX + boxW / 2;
      const cy = padY + boxH / 2 + 12;

      boxes.push({
        vault: vn.vault,
        vaultKind: vn.vault_kind,
        defaultWrite: vn.default_write ?? "propose",
        x: cursorX,
        y: padY,
        w: boxW,
        h: boxH,
        color: col,
      });

      pos.push({ ...vn, x: cx, y: cy, color: col });

      vNotes.forEach((nn, ni) => {
        const angle = (2 * Math.PI * ni) / Math.max(1, vNotes.length) - Math.PI / 2;
        const r = vNotes.length === 1 ? radius * 0.72 : radius;
        pos.push({
          ...nn,
          x: cx + Math.cos(angle) * r,
          y: cy + Math.sin(angle) * r,
          color: col,
        });
      });

      cursorX += boxW + colGap;
      if (boxH + padY * 2 > maxH) maxH = boxH + padY * 2;
    });

    return {
      positioned: pos,
      clusters: boxes,
      viewW: Math.max(920, cursorX + padX - colGap),
      viewH: Math.max(540, maxH),
    };
  }, [filteredNodes]);

  const byId = React.useMemo(() => {
    const m = new Map<string, PositionedNode>();
    for (const n of positioned) m.set(n.id, n);
    return m;
  }, [positioned]);

  const activeFocus = hoveredId ?? selectedId;
  const connectedSet = React.useMemo(() => {
    const s = new Set<string>();
    if (!activeFocus) return s;
    s.add(activeFocus);
    for (const e of filteredEdges) {
      if (e.source === activeFocus) s.add(e.target);
      if (e.target === activeFocus) s.add(e.source);
    }
    return s;
  }, [activeFocus, filteredEdges]);

  const selectedNode = selectedId ? byId.get(selectedId) ?? null : positioned.find((n) => n.kind === "note") ?? null;
  const selectedLinks = React.useMemo(() => {
    if (!selectedNode) return [];
    return filteredEdges
      .filter((e) => e.kind !== "contains" && (e.source === selectedNode.id || e.target === selectedNode.id))
      .map((e) => {
        const otherId = e.source === selectedNode.id ? e.target : e.source;
        const dir = e.source === selectedNode.id ? "out" : "in";
        return { edge: e, dir, other: byId.get(otherId) };
      })
      .filter((x) => Boolean(x.other));
  }, [selectedNode, filteredEdges, byId]);

  const noteCount = positioned.filter((n) => n.kind === "note").length;
  const linkCount = filteredEdges.filter((e) => e.kind !== "contains").length;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden bg-[#0a0d0e]">
      {/* Interactive SVG Canvas */}
      <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border bg-[#0d1214] px-4 md:px-5">
          <div className="flex min-w-0 items-center gap-2.5 text-[13px]">
            <span className="shrink-0 font-medium text-foreground">Graph</span>
            <span className="truncate text-xs text-muted-foreground tabular-nums">
              {clusters.length} vaults · {noteCount} notes · {linkCount} links
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <Button
              variant="outline"
              size="icon"
              aria-label="Zoom out"
              onClick={() => setZoom((z) => Math.max(0.55, +(z - 0.15).toFixed(2)))}
            >
              <MagnifyingGlassMinus size={14} />
            </Button>
            <Button
              variant="outline"
              size="icon"
              aria-label="Zoom in"
              onClick={() => setZoom((z) => Math.min(2, +(z + 0.15).toFixed(2)))}
            >
              <MagnifyingGlassPlus size={14} />
            </Button>
            <Button
              variant="outline"
              size="icon"
              aria-label="Reset view"
              onClick={() => {
                setZoom(1);
                setPan({ x: 0, y: 0 });
              }}
            >
              <ArrowsCounterClockwise size={14} />
            </Button>
          </div>
        </div>

        <div
          className="relative flex-1 cursor-grab overflow-hidden active:cursor-grabbing"
          onMouseDown={(e) => {
            dragRef.current = { x: e.clientX, y: e.clientY, px: pan.x, py: pan.y };
          }}
          onMouseMove={(e) => {
            if (!dragRef.current) return;
            setPan({
              x: dragRef.current.px + (e.clientX - dragRef.current.x),
              y: dragRef.current.py + (e.clientY - dragRef.current.y),
            });
          }}
          onMouseUp={() => {
            dragRef.current = null;
          }}
          onMouseLeave={() => {
            dragRef.current = null;
          }}
          onWheel={(e) => {
            const delta = e.deltaY > 0 ? -0.08 : 0.08;
            setZoom((z) => Math.min(2, Math.max(0.55, +(z + delta).toFixed(2))));
          }}
        >
          {/* Subtle engineering dot grid */}
          <svg className="h-full w-full" viewBox={`0 0 ${viewW} ${viewH}`}>
            <defs>
              <pattern id="mt-grid" width="24" height="24" patternUnits="userSpaceOnUse">
                <circle cx="1" cy="1" r="0.8" fill="#1c272a" />
              </pattern>
              <marker id="arrow-link" viewBox="0 0 10 10" refX="16" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#2dd4bf" />
              </marker>
              <marker id="arrow-decision" viewBox="0 0 10 10" refX="16" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#f59e0b" />
              </marker>
            </defs>
            <rect width="100%" height="100%" fill="url(#mt-grid)" />

            <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
              {/* Isolated vault territory frames */}
              {clusters.map((c) => (
                <g key={c.vault}>
                  <rect
                    x={c.x}
                    y={c.y}
                    width={c.w}
                    height={c.h}
                    rx={6}
                    fill="#0e1416"
                    fillOpacity={0.72}
                    stroke={c.color}
                    strokeOpacity={0.28}
                    strokeWidth={1}
                    strokeDasharray="4 4"
                  />
                  <text
                    x={c.x + 14}
                    y={c.y + 22}
                    fill={c.color}
                    fontSize={12}
                    fontWeight={600}
                    letterSpacing="0.02em"
                  >
                    {c.vault}
                  </text>
                  <text
                    x={c.x + c.w - 14}
                    y={c.y + 22}
                    textAnchor="end"
                    fill="#6c8280"
                    fontSize={10.5}
                  >
                    {c.vaultKind} · {c.defaultWrite}
                  </text>
                </g>
              ))}

              {/* Edges (curved bezier for note-to-note links with directional arrowheads) */}
              {filteredEdges.map((e, idx) => {
                const s = byId.get(e.source);
                const t = byId.get(e.target);
                if (!s || !t) return null;
                const isSemantic = e.kind !== "contains";
                const highlighted =
                  !activeFocus || (connectedSet.has(e.source) && connectedSet.has(e.target));
                const stroke =
                  e.kind === "decision"
                    ? "#f59e0b"
                    : e.kind === "link"
                      ? "#2dd4bf"
                      : "#1f2c2f";
                if (!isSemantic) {
                  return (
                    <line
                      key={idx}
                      x1={s.x}
                      y1={s.y}
                      x2={t.x}
                      y2={t.y}
                      stroke={stroke}
                      strokeWidth={1}
                      strokeOpacity={highlighted ? 0.55 : 0.12}
                    />
                  );
                }
                const mx = (s.x + t.x) / 2;
                const my = (s.y + t.y) / 2;
                const dx = t.x - s.x;
                const dy = t.y - s.y;
                const cx = mx - dy * 0.16;
                const cy = my + dx * 0.16;
                return (
                  <path
                    key={idx}
                    d={`M ${s.x} ${s.y} Q ${cx} ${cy} ${t.x} ${t.y}`}
                    fill="none"
                    stroke={stroke}
                    strokeWidth={1.8}
                    strokeDasharray={e.kind === "decision" ? "5 3" : undefined}
                    strokeOpacity={highlighted ? 0.92 : 0.15}
                    markerEnd={e.kind === "decision" ? "url(#arrow-decision)" : "url(#arrow-link)"}
                  />
                );
              })}

              {/* Nodes */}
              {positioned.map((n) => {
                const isVault = n.kind === "vault";
                const isSelected = selectedNode?.id === n.id;
                const dimmed = Boolean(activeFocus && !connectedSet.has(n.id));
                const hasProps = (n.open_proposals ?? 0) > 0;
                const r = isVault ? 13 : isSelected ? 9 : 7;

                return (
                  <g
                    key={n.id}
                    transform={`translate(${n.x}, ${n.y})`}
                    opacity={dimmed ? 0.24 : 1}
                    className="cursor-pointer"
                    onMouseEnter={() => setHoveredId(n.id)}
                    onMouseLeave={() => setHoveredId(null)}
                    onClick={(ev) => {
                      ev.stopPropagation();
                      setSelectedId(n.id);
                    }}
                  >
                    {isSelected && (
                      <rect
                        x={-(r + 5)}
                        y={-(r + 5)}
                        width={(r + 5) * 2}
                        height={(r + 5) * 2}
                        rx={4}
                        fill="none"
                        stroke={n.color}
                        strokeWidth={1.5}
                        strokeOpacity={0.75}
                      />
                    )}
                    {isVault ? (
                      <rect
                        x={-r}
                        y={-r}
                        width={r * 2}
                        height={r * 2}
                        rx={4}
                        fill="#0f181a"
                        stroke={n.color}
                        strokeWidth={2}
                      />
                    ) : (
                      <circle
                        r={r}
                        fill={hasProps ? "#132926" : "#121a1c"}
                        stroke={hasProps ? "#2dd4bf" : n.color}
                        strokeWidth={hasProps ? 2.2 : 1.5}
                      />
                    )}
                    {!isVault && hasProps && (
                      <circle cx={r + 1} cy={-r + 1} r={3.5} fill="#2dd4bf" />
                    )}
                    <text
                      y={r + 16}
                      textAnchor="middle"
                      fill={isSelected ? "#ffffff" : isVault ? n.color : "#cfe0df"}
                      fontSize={isVault ? 12 : 11}
                      fontWeight={isVault || isSelected ? 600 : 400}
                    >
                      {truncateLabel(n.label, isVault ? 18 : 20)}
                    </text>
                  </g>
                );
              })}
            </g>
          </svg>

          {/* Compact Legend */}
          <div className="pointer-events-none absolute bottom-3 left-4 flex flex-wrap items-center gap-4 rounded-md border border-border bg-[#0d1214]/95 px-3 py-1.5 text-[11px] text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-2.5 w-2.5 rounded-[2px] border border-teal-400 bg-[#0f181a]" />
              Vault boundary (SQL isolated)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-0.5 w-4 bg-teal-400" />
              Note link
            </span>
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-0.5 w-4 border-t border-dashed border-amber-400" />
              Decision pointer
            </span>
          </div>
        </div>
      </div>

      {/* Right Node Inspector */}
      <aside className="hidden w-[300px] shrink-0 flex-col border-l border-border bg-[#0e1214] lg:flex">
        <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-4 text-[13px] font-medium text-foreground">
          <span>Inspector</span>
          {selectedNode && (
            <Badge variant={selectedNode.vault_kind === "org" ? "default" : "warn"}>
              {selectedNode.vault}
            </Badge>
          )}
        </div>
        {selectedNode ? (
          <div className="flex-1 space-y-5 overflow-y-auto p-4 text-xs">
            <div>
              <div className="text-[11px] uppercase tracking-wider text-muted-foreground">
                {selectedNode.kind}
              </div>
              <h2 className="mt-1 text-base font-medium text-foreground">{selectedNode.label}</h2>
              {selectedNode.path && (
                <div className="mt-0.5 text-muted-foreground">{selectedNode.path}</div>
              )}
            </div>

            <div className="divide-y divide-border/70 border-y border-border/70">
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground">Vault</span>
                <span className="font-medium text-foreground">{selectedNode.vault}</span>
              </div>
              {selectedNode.status && (
                <div className="flex justify-between py-2">
                  <span className="text-muted-foreground">Status</span>
                  <span className="font-medium text-foreground">{selectedNode.status}</span>
                </div>
              )}
              {selectedNode.default_write && (
                <div className="flex justify-between py-2">
                  <span className="text-muted-foreground">Write policy</span>
                  <span className="font-medium text-foreground">{selectedNode.default_write}</span>
                </div>
              )}
              {selectedNode.owner && (
                <div className="flex justify-between py-2">
                  <span className="text-muted-foreground">Owner lock</span>
                  <span className="inline-flex items-center gap-1 font-medium text-amber-300">
                    <LockSimple size={12} />
                    {selectedNode.owner}
                  </span>
                </div>
              )}
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground">Open proposals</span>
                <span className="font-medium tabular-nums text-foreground">
                  {selectedNode.open_proposals ?? 0}
                </span>
              </div>
            </div>

            <div>
              <div className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                Connections ({selectedLinks.length})
              </div>
              {selectedLinks.length === 0 ? (
                <div className="text-muted-foreground">No intra-vault links yet</div>
              ) : (
                <div className="space-y-1">
                  {selectedLinks.map((item, i) => (
                    <Button
                      key={i}
                      variant="ghost"
                      size="sm"
                      onClick={() => item.other && setSelectedId(item.other.id)}
                      className="h-auto w-full justify-between px-2 py-1.5 text-left"
                    >
                      <span className="truncate text-foreground">
                        {item.dir === "out" ? "→ " : "← "}
                        {item.other?.label}
                      </span>
                      <Badge variant={item.edge.kind === "decision" ? "warn" : "default"}>
                        {item.edge.kind}
                      </Badge>
                    </Button>
                  ))}
                </div>
              )}
            </div>

            <div className="space-y-2 pt-1">
              {selectedNode.kind === "note" && selectedNode.path && (
                <Button
                  className="w-full"
                  onClick={() => onOpenNote(selectedNode.vault, selectedNode.path!)}
                >
                  Open note
                  <ArrowUpRight size={14} />
                </Button>
              )}
              {(selectedNode.open_proposals ?? 0) > 0 && (
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={() => onOpenProposals(selectedNode.vault)}
                >
                  <GitDiff size={14} />
                  Review open proposals
                </Button>
              )}
            </div>
          </div>
        ) : (
          <div className="p-4 text-xs text-muted-foreground">Click any node to inspect links</div>
        )}
      </aside>
    </div>
  );
}
