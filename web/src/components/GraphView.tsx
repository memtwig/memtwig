import * as React from "react";
import {
  ArrowUpRight,
  ArrowsCounterClockwise,
  GitDiff,
  LockSimple,
  MagnifyingGlassMinus,
  MagnifyingGlassPlus,
} from "@phosphor-icons/react";
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
};

type ClusterBox = {
  vault: string;
  vaultKind: string;
  defaultWrite: string;
  x: number;
  y: number;
  w: number;
  h: number;
};

function truncateLabel(s: string, max = 20): string {
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
  const didDragRef = React.useRef(false);

  const filteredNodes = React.useMemo(
    () => (vaultFilter === "all" ? nodes : nodes.filter((n) => n.vault === vaultFilter)),
    [nodes, vaultFilter],
  );
  const visibleIds = React.useMemo(() => new Set(filteredNodes.map((n) => n.id)), [filteredNodes]);
  const filteredEdges = React.useMemo(
    () => edges.filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target)),
    [edges, visibleIds],
  );

  const degreeById = React.useMemo(() => {
    const deg = new Map<string, number>();
    for (const e of filteredEdges) {
      if (e.kind === "contains") continue;
      deg.set(e.source, (deg.get(e.source) ?? 0) + 1);
      deg.set(e.target, (deg.get(e.target) ?? 0) + 1);
    }
    return deg;
  }, [filteredEdges]);

  const { positioned, clusters, vaultNoteCounts, viewW, viewH } = React.useMemo(() => {
    const vaultNodes = filteredNodes
      .filter((n) => n.kind === "vault")
      .sort((a, b) => a.vault.localeCompare(b.vault));
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
    const counts = new Map<string, number>();
    const colGap = 40;
    const padX = 36;
    const padY = 44;
    let cursorX = padX;
    let maxH = 440;

    const placeRing = (ringNotes: GraphNode[], cx: number, cy: number, r: number, angleOffset = 0) => {
      ringNotes.forEach((nn, ni) => {
        const angle = (2 * Math.PI * ni) / Math.max(1, ringNotes.length) - Math.PI / 2 + angleOffset;
        pos.push({
          ...nn,
          x: cx + Math.cos(angle) * r,
          y: cy + Math.sin(angle) * r,
        });
      });
    };

    vaultNodes.forEach((vn) => {
      const vNotes = (notesByVault.get(vn.vault) ?? []).sort(
        (a, b) => (a.folder ?? "").localeCompare(b.folder ?? "") || (a.path ?? "").localeCompare(b.path ?? ""),
      );
      counts.set(vn.vault, vNotes.length);

      if (vNotes.length <= 20) {
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
        });
        pos.push({ ...vn, x: cx, y: cy });
        placeRing(vNotes, cx, cy, vNotes.length === 1 ? radius * 0.72 : radius);

        cursorX += boxW + colGap;
        if (boxH + padY * 2 > maxH) maxH = boxH + padY * 2;
        return;
      }

      // Dense vault (>20 notes): pull hub notes to inner ring and stagger remaining notes across mid & outer rings
      const rInner = 115;
      const rMid = 225;
      const rOuter = 320;
      const boxW = rOuter * 2 + 200;
      const boxH = rOuter * 2 + 140;
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
      });
      pos.push({ ...vn, x: cx, y: cy });

      const hubs = vNotes
        .filter((n) => (degreeById.get(n.id) ?? 0) >= 4)
        .sort((a, b) => (degreeById.get(b.id) ?? 0) - (degreeById.get(a.id) ?? 0) || (a.path ?? "").localeCompare(b.path ?? ""))
        .slice(0, 10);
      const hubSet = new Set(hubs.map((n) => n.id));
      const remaining = vNotes.filter((n) => !hubSet.has(n.id));
      const midCount = Math.round(remaining.length * 0.4);
      const midNotes = remaining.slice(0, midCount);
      const outerNotes = remaining.slice(midCount);

      if (hubs.length > 0) {
        placeRing(hubs, cx, cy, rInner);
      }
      if (midNotes.length > 0) {
        placeRing(midNotes, cx, cy, rMid, Math.PI / Math.max(1, midNotes.length));
      }
      if (outerNotes.length > 0) {
        placeRing(outerNotes, cx, cy, rOuter);
      }

      cursorX += boxW + colGap;
      if (boxH + padY * 2 > maxH) maxH = boxH + padY * 2;
    });

    return {
      positioned: pos,
      clusters: boxes,
      vaultNoteCounts: counts,
      viewW: Math.max(920, cursorX + padX - colGap),
      viewH: Math.max(540, maxH),
    };
  }, [filteredNodes, degreeById]);

  const byId = React.useMemo(() => {
    const m = new Map<string, PositionedNode>();
    for (const n of positioned) m.set(n.id, n);
    return m;
  }, [positioned]);

  const noteCount = positioned.filter((n) => n.kind === "note").length;
  const linkCount = filteredEdges.filter((e) => e.kind !== "contains").length;
  const isDenseGraph = noteCount > 20;

  const topHubIds = React.useMemo(() => {
    const s = new Set<string>();
    const byVault = new Map<string, PositionedNode[]>();
    for (const n of positioned) {
      if (n.kind !== "note") continue;
      if ((degreeById.get(n.id) ?? 0) < 2) continue;
      const list = byVault.get(n.vault) ?? [];
      list.push(n);
      byVault.set(n.vault, list);
    }
    for (const list of byVault.values()) {
      list
        .sort((a, b) => (degreeById.get(b.id) ?? 0) - (degreeById.get(a.id) ?? 0) || a.label.localeCompare(b.label))
        .slice(0, 8)
        .forEach((n) => s.add(n.id));
    }
    return s;
  }, [positioned, degreeById]);

  const activeFocus = hoveredId ?? selectedId;

  const focusedNeighborLabels = React.useMemo(() => {
    const s = new Set<string>();
    if (!activeFocus) return s;
    const neighborIds: string[] = [];
    for (const e of filteredEdges) {
      if (e.kind === "contains") continue;
      if (e.source === activeFocus) neighborIds.push(e.target);
      else if (e.target === activeFocus) neighborIds.push(e.source);
    }
    neighborIds
      .sort((a, b) => (degreeById.get(b) ?? 0) - (degreeById.get(a) ?? 0))
      .slice(0, 12)
      .forEach((id) => s.add(id));
    return s;
  }, [activeFocus, filteredEdges, degreeById]);

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

  const defaultInspectorNode = React.useMemo(() => {
    const noteNodes = positioned.filter((n) => n.kind === "note");
    if (noteNodes.length === 0) return null;
    let best = noteNodes[0];
    let bestDeg = degreeById.get(best.id) ?? 0;
    for (const n of noteNodes) {
      const d = degreeById.get(n.id) ?? 0;
      if (d > bestDeg) {
        best = n;
        bestDeg = d;
      }
    }
    return best;
  }, [positioned, degreeById]);

  const selectedNode = selectedId ? byId.get(selectedId) ?? null : defaultInspectorNode;

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

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
      <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border bg-[#0c0c0e] px-4 md:px-5">
          <div className="flex min-w-0 items-center gap-3 text-[13px]">
            <span className="shrink-0 font-medium text-foreground">Graph</span>
            <span className="truncate text-xs text-muted-foreground tabular-nums">
              {clusters.length} {clusters.length === 1 ? "vault" : "vaults"} · {noteCount}{" "}
              {noteCount === 1 ? "note" : "notes"} · {linkCount} {linkCount === 1 ? "link" : "links"}
            </span>
          </div>
          <div className="flex items-center gap-1">
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
            didDragRef.current = false;
            dragRef.current = { x: e.clientX, y: e.clientY, px: pan.x, py: pan.y };
          }}
          onMouseMove={(e) => {
            if (!dragRef.current) return;
            const dx = e.clientX - dragRef.current.x;
            const dy = e.clientY - dragRef.current.y;
            if (Math.abs(dx) > 4 || Math.abs(dy) > 4) {
              didDragRef.current = true;
            }
            setPan({
              x: dragRef.current.px + dx,
              y: dragRef.current.py + dy,
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
          <svg
            className="h-full w-full"
            viewBox={`0 0 ${viewW} ${viewH}`}
            onClick={() => {
              if (!didDragRef.current) setSelectedId(null);
            }}
          >
            <defs>
              <pattern id="mt-grid" width="24" height="24" patternUnits="userSpaceOnUse">
                <circle cx="1" cy="1" r="0.7" fill="#1e1e23" />
              </pattern>
              <marker id="arrow-link" viewBox="0 0 10 10" refX="16" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#5eead4" />
              </marker>
              <marker id="arrow-decision" viewBox="0 0 10 10" refX="16" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#a1a1aa" />
              </marker>
            </defs>
            <rect width="100%" height="100%" fill="url(#mt-grid)" />

            <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
              {clusters.map((c) => (
                <g key={c.vault}>
                  <rect
                    x={c.x}
                    y={c.y}
                    width={c.w}
                    height={c.h}
                    rx={6}
                    fill="#0d0d10"
                    fillOpacity={0.85}
                    stroke="#27272c"
                    strokeWidth={1}
                    strokeDasharray="4 4"
                  />
                  <text x={c.x + 14} y={c.y + 22} fill="#e4e4e7" fontSize={12} fontWeight={600}>
                    {c.vault}
                  </text>
                  <text x={c.x + c.w - 14} y={c.y + 22} textAnchor="end" fill="#71717a" fontSize={10.5}>
                    {c.vaultKind} · {c.defaultWrite}
                  </text>
                </g>
              ))}

              {filteredEdges.map((e, idx) => {
                const s = byId.get(e.source);
                const t = byId.get(e.target);
                if (!s || !t) return null;
                if (e.kind === "contains") {
                  if ((vaultNoteCounts.get(s.vault) ?? 0) > 20) return null;
                  const highlighted =
                    !activeFocus || (connectedSet.has(e.source) && connectedSet.has(e.target));
                  return (
                    <line
                      key={idx}
                      x1={s.x}
                      y1={s.y}
                      x2={t.x}
                      y2={t.y}
                      stroke="#222227"
                      strokeWidth={1}
                      strokeOpacity={highlighted ? 0.65 : 0.14}
                    />
                  );
                }

                const isFocused = Boolean(activeFocus && (e.source === activeFocus || e.target === activeFocus));
                const mx = (s.x + t.x) / 2;
                const my = (s.y + t.y) / 2;
                const dx = t.x - s.x;
                const dy = t.y - s.y;
                const cx = mx - dy * 0.16;
                const cy = my + dx * 0.16;

                let stroke = e.kind === "decision" ? "#a1a1aa" : "#d4d4d8";
                let strokeWidth = 1.6;
                let strokeOpacity = !activeFocus || isFocused ? 0.85 : 0.12;
                let markerEnd: string | undefined =
                  e.kind === "decision" ? "url(#arrow-decision)" : "url(#arrow-link)";

                if (isDenseGraph) {
                  if (!activeFocus) {
                    strokeOpacity = 0.14;
                    strokeWidth = 1;
                    markerEnd = undefined;
                  } else if (isFocused) {
                    stroke = e.kind === "decision" ? "#a1a1aa" : "#5eead4";
                    strokeOpacity = 0.9;
                    strokeWidth = 1.6;
                  } else {
                    strokeOpacity = 0.04;
                    strokeWidth = 1;
                    markerEnd = undefined;
                  }
                }

                return (
                  <path
                    key={idx}
                    d={`M ${s.x} ${s.y} Q ${cx} ${cy} ${t.x} ${t.y}`}
                    fill="none"
                    stroke={stroke}
                    strokeWidth={strokeWidth}
                    strokeDasharray={e.kind === "decision" ? "4 3" : undefined}
                    strokeOpacity={strokeOpacity}
                    markerEnd={markerEnd}
                  />
                );
              })}

              {positioned.map((n) => {
                const isVault = n.kind === "vault";
                const isSelected = selectedId === n.id || (!selectedId && selectedNode?.id === n.id);
                const dimmed = Boolean(activeFocus && !connectedSet.has(n.id));
                const hasProps = (n.open_proposals ?? 0) > 0;
                const deg = degreeById.get(n.id) ?? 0;
                const r = isVault ? 11 : isSelected ? 8 : isDenseGraph ? (deg >= 4 ? 6.5 : 4.5) : 6.5;
                const showLabel =
                  !isDenseGraph ||
                  isVault ||
                  n.id === activeFocus ||
                  (activeFocus ? focusedNeighborLabels.has(n.id) : topHubIds.has(n.id));

                return (
                  <g
                    key={n.id}
                    transform={`translate(${n.x}, ${n.y})`}
                    opacity={dimmed ? 0.2 : 1}
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
                        stroke="#f4f4f5"
                        strokeWidth={1.2}
                        strokeOpacity={0.65}
                      />
                    )}
                    {isVault ? (
                      <rect
                        x={-r}
                        y={-r}
                        width={r * 2}
                        height={r * 2}
                        rx={3}
                        fill="#141418"
                        stroke="#52525b"
                        strokeWidth={1.5}
                      />
                    ) : (
                      <circle
                        r={r}
                        fill={isSelected ? "#fafafa" : "#18181c"}
                        stroke={isSelected ? "#fafafa" : deg >= 4 ? "#d4d4d8" : "#71717a"}
                        strokeWidth={1.5}
                      />
                    )}
                    {!isVault && hasProps && <circle cx={r + 2} cy={-r + 1} r={3} fill="#22c55e" />}
                    {showLabel && (
                      <text
                        y={r + 15}
                        textAnchor="middle"
                        fill={isSelected ? "#ffffff" : isVault ? "#a1a1aa" : "#d4d4d8"}
                        fontSize={11}
                        fontWeight={isSelected || isVault ? 500 : 400}
                        paintOrder="stroke"
                        stroke="#0d0d10"
                        strokeWidth={3}
                      >
                        {truncateLabel(n.label, 20)}
                      </text>
                    )}
                  </g>
                );
              })}
            </g>
          </svg>
        </div>
      </div>

      {/* Right Inspector */}
      <aside className="hidden w-[290px] shrink-0 flex-col border-l border-border bg-[#0c0c0e] lg:flex">
        <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-4 text-[13px] font-medium text-foreground">
          <span>Inspector</span>
          {selectedNode && <span className="text-xs text-muted-foreground">{selectedNode.vault}</span>}
        </div>
        {selectedNode ? (
          <div className="flex-1 space-y-5 overflow-y-auto p-4 text-xs">
            <div>
              <h2 className="text-sm font-medium text-foreground">{selectedNode.label}</h2>
              {selectedNode.path && <div className="mt-0.5 text-muted-foreground">{selectedNode.path}</div>}
            </div>

            <div className="divide-y divide-border/70 border-y border-border/70">
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground">Vault</span>
                <span className="text-foreground">{selectedNode.vault}</span>
              </div>
              {selectedNode.status && (
                <div className="flex justify-between py-2">
                  <span className="text-muted-foreground">Status</span>
                  <span className="text-foreground">{selectedNode.status}</span>
                </div>
              )}
              {selectedNode.owner && (
                <div className="flex justify-between py-2">
                  <span className="text-muted-foreground">Owner</span>
                  <span className="inline-flex items-center gap-1 text-foreground">
                    <LockSimple size={12} />
                    {selectedNode.owner}
                  </span>
                </div>
              )}
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground">Open proposals</span>
                <span className="tabular-nums text-foreground">{selectedNode.open_proposals ?? 0}</span>
              </div>
            </div>

            <div>
              <div className="mb-2 text-[11px] font-medium text-muted-foreground">
                Connections ({selectedLinks.length})
              </div>
              {selectedLinks.length === 0 ? (
                <div className="text-muted-foreground">No links</div>
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
                      <span className="text-[11px] text-muted-foreground">{item.edge.kind}</span>
                    </Button>
                  ))}
                </div>
              )}
            </div>

            <div className="space-y-2 pt-1">
              {selectedNode.kind === "note" && selectedNode.path && (
                <Button className="w-full" onClick={() => onOpenNote(selectedNode.vault, selectedNode.path!)}>
                  Open note
                  <ArrowUpRight size={14} />
                </Button>
              )}
              {(selectedNode.open_proposals ?? 0) > 0 && (
                <Button variant="outline" className="w-full" onClick={() => onOpenProposals(selectedNode.vault)}>
                  <GitDiff size={14} />
                  Review proposals
                </Button>
              )}
            </div>
          </div>
        ) : (
          <div className="p-4 text-xs text-muted-foreground">Select a node</div>
        )}
      </aside>
    </div>
  );
}
