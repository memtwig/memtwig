import * as React from "react";
import {
  ArrowLeft,
  ArrowsClockwise,
  Check,
  ClockCounterClockwise,
  Files,
  GitDiff,
  Graph,
  LockSimple,
  MagnifyingGlass,
  SlidersHorizontal,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownView } from "@/components/MarkdownView";
import { GraphView, type GraphEdge, type GraphNode } from "@/components/GraphView";
import { SettingsView } from "@/components/SettingsView";
import { OverviewView } from "@/components/OverviewView";
import { cn } from "@/lib/utils";

type Section = "overview" | "notes" | "graph" | "proposals" | "audit" | "settings";
type NoteMode = "read" | "raw" | "edit";

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

type NoteDetail = {
  vault: string;
  path: string;
  title: string;
  status: string;
  valid_from?: string;
  updated: string;
  owner: string;
  size_bytes: number;
  max_bytes?: number;
  default_write?: string;
  raw: string;
};

type SearchHit = {
  vault: string;
  path: string;
  title: string;
  snippet: string;
  score: number;
};

type ProposalItem = {
  id: string;
  vault: string;
  kind: string;
  path: string;
  reason: string;
  author: string;
  status: string;
  created_at: string;
  diff: string;
};

type AuditEvent = {
  timestamp: string;
  actor: string;
  role?: string;
  action: string;
  vault?: string;
  path?: string;
  outcome: string;
  detail?: string;
};

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  return `${(n / 1024).toFixed(1)} KB`;
}

function countDiffAdditions(diff: string): number {
  return diff
    .trimEnd()
    .split("\n")
    .filter((l) => l.startsWith("+ ") || (l.startsWith("+") && !l.startsWith("+++"))).length;
}

export function App() {
  const initialSection = (): Section => {
    const p = new URLSearchParams(window.location.search).get("tab");
    if (p === "notes" || p === "graph" || p === "proposals" || p === "audit" || p === "settings") {
      return p;
    }
    return "overview";
  };

  const [section, setSection] = React.useState<Section>(initialSection);
  const [vaults, setVaults] = React.useState<VaultInfo[]>([]);
  const [vaultFilter, setVaultFilter] = React.useState<string>("all");

  const [notes, setNotes] = React.useState<NoteSummary[]>([]);
  const [graphNodes, setGraphNodes] = React.useState<GraphNode[]>([]);
  const [graphEdges, setGraphEdges] = React.useState<GraphEdge[]>([]);

  const [searchHits, setSearchHits] = React.useState<SearchHit[] | null>(null);
  const [query, setQuery] = React.useState("");
  const searchInputRef = React.useRef<HTMLInputElement>(null);

  const [selectedNote, setSelectedNote] = React.useState<NoteDetail | null>(null);
  const [noteMode, setNoteMode] = React.useState<NoteMode>("read");
  const [draftContent, setDraftContent] = React.useState("");
  const [draftReason, setDraftReason] = React.useState("");
  const [mobileShowDetail, setMobileShowDetail] = React.useState(false);

  const [proposals, setProposals] = React.useState<ProposalItem[]>([]);
  const [selectedPropId, setSelectedPropId] = React.useState<string | null>(null);
  const [auditEvents, setAuditEvents] = React.useState<AuditEvent[]>([]);

  const [busy, setBusy] = React.useState(false);
  const [notice, setNotice] = React.useState<string | null>(null);

  const flash = React.useCallback((msg: string) => {
    setNotice(msg);
    window.setTimeout(() => {
      setNotice((curr) => (curr === msg ? null : curr));
    }, 3500);
  }, []);

  const loadAll = React.useCallback(async () => {
    try {
      const [stRes, notesRes, graphRes, propRes, audRes] = await Promise.all([
        fetch("/api/v1/status"),
        fetch("/api/v1/notes"),
        fetch("/api/v1/graph"),
        fetch("/api/v1/proposals"),
        fetch("/api/v1/audit?n=60"),
      ]);
      if (stRes.ok) {
        const d = await stRes.json();
        setVaults(d.vaults ?? []);
      }
      if (notesRes.ok) {
        const d = await notesRes.json();
        setNotes(d.notes ?? []);
      }
      if (graphRes.ok) {
        const d = await graphRes.json();
        setGraphNodes(d.nodes ?? []);
        setGraphEdges(d.edges ?? []);
      }
      if (propRes.ok) {
        const d = await propRes.json();
        const list: ProposalItem[] = d.proposals ?? [];
        setProposals(list);
        setSelectedPropId((prev) => (prev && list.some((p) => p.id === prev) ? prev : list[0]?.id ?? null));
      }
      if (audRes.ok) {
        const d = await audRes.json();
        setAuditEvents((d.events ?? []).slice().reverse());
      }
    } catch {
      flash("Backend unreachable");
    }
  }, [flash]);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const openNote = React.useCallback(async (vault: string, path: string, openMobile = false) => {
    const res = await fetch(`/api/v1/note?vault=${encodeURIComponent(vault)}&path=${encodeURIComponent(path)}`);
    if (res.ok) {
      const d: NoteDetail = await res.json();
      setSelectedNote(d);
      setDraftContent(d.raw);
      setDraftReason("");
      setNoteMode("read");
      if (openMobile) setMobileShowDetail(true);
    }
  }, []);

  const visibleNotes = React.useMemo(() => {
    if (searchHits) {
      return searchHits
        .filter((h) => vaultFilter === "all" || h.vault === vaultFilter)
        .map((h) => ({
          vault: h.vault,
          path: h.path,
          title: h.title || h.path,
          meta: `score ${h.score.toFixed(3)}`,
        }));
    }
    return notes
      .filter((n) => vaultFilter === "all" || n.vault === vaultFilter)
      .map((n) => ({
        vault: n.vault,
        path: n.path,
        title: n.title || n.path,
        meta: n.updated || "",
      }));
  }, [searchHits, notes, vaultFilter]);

  React.useEffect(() => {
    if (!selectedNote && visibleNotes.length > 0) {
      void openNote(visibleNotes[0].vault, visibleNotes[0].path);
    }
  }, [visibleNotes, selectedNote, openNote]);

  const filteredProposals = React.useMemo(() => {
    if (vaultFilter === "all") return proposals;
    return proposals.filter((p) => p.vault === vaultFilter);
  }, [proposals, vaultFilter]);

  const activeProposal =
    filteredProposals.find((p) => p.id === selectedPropId) ?? filteredProposals[0] ?? null;

  const openProposalsForNote = React.useMemo(() => {
    if (!selectedNote) return [];
    return proposals.filter((p) => p.vault === selectedNote.vault && p.path === selectedNote.path);
  }, [proposals, selectedNote]);

  const noteConnections = React.useMemo(() => {
    if (!selectedNote) return [];
    const curId = `note:${selectedNote.vault}:${selectedNote.path}`;
    const nodeMap = new Map(graphNodes.map((n) => [n.id, n]));
    return graphEdges
      .filter((e) => e.kind !== "contains" && (e.source === curId || e.target === curId))
      .map((e) => {
        const other = nodeMap.get(e.source === curId ? e.target : e.source);
        return {
          dir: e.source === curId ? "out" : "in",
          kind: e.kind,
          node: other,
        };
      })
      .filter((x) => Boolean(x.node && x.node.path));
  }, [selectedNote, graphNodes, graphEdges]);

  const runSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    const q = query.trim();
    if (!q) {
      setSearchHits(null);
      return;
    }
    const vParam = vaultFilter !== "all" ? `&vault=${encodeURIComponent(vaultFilter)}` : "";
    const res = await fetch(`/api/v1/search?q=${encodeURIComponent(q)}&n=25${vParam}`);
    if (res.ok) {
      const d = await res.json();
      const hits: SearchHit[] = d.results ?? [];
      setSearchHits(hits);
      if (hits.length > 0) {
        void openNote(hits[0].vault, hits[0].path);
      }
    }
  };

  const handleProposal = React.useCallback(
    async (item: ProposalItem, action: "approve" | "reject") => {
      setBusy(true);
      try {
        const res = await fetch(
          `/api/v1/proposals/${encodeURIComponent(item.vault)}/${encodeURIComponent(item.id)}/${action}`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(action === "reject" ? { reason: "rejected in studio" } : {}),
          },
        );
        if (res.ok) {
          flash(action === "approve" ? `Approved ${item.path}` : `Rejected ${item.id.slice(0, 8)}`);
          await loadAll();
        }
      } finally {
        setBusy(false);
      }
    },
    [flash, loadAll],
  );

  const saveOrProposeNote = async () => {
    if (!selectedNote) return;
    const mustPropose = selectedNote.default_write !== "direct" || Boolean(selectedNote.owner);
    setBusy(true);
    try {
      if (mustPropose) {
        const res = await fetch("/api/v1/proposals", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            vault: selectedNote.vault,
            path: selectedNote.path,
            kind: "replace",
            content: draftContent,
            reason: draftReason.trim() || "Edited in Memtwig Studio",
          }),
        });
        if (res.ok) {
          flash("Proposal queued");
          setNoteMode("read");
          await loadAll();
        }
      } else {
        const res = await fetch("/api/v1/note", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            vault: selectedNote.vault,
            path: selectedNote.path,
            content: draftContent,
          }),
        });
        if (res.ok) {
          flash("Saved");
          await openNote(selectedNote.vault, selectedNote.path);
          await loadAll();
        }
      }
    } finally {
      setBusy(false);
    }
  };

  const triggerAction = async (endpoint: "/api/v1/reindex" | "/api/v1/curate", label: string) => {
    setBusy(true);
    try {
      const res = await fetch(endpoint, { method: "POST" });
      if (res.ok) {
        flash(label);
        await loadAll();
      }
    } finally {
      setBusy(false);
    }
  };

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setSection("notes");
        setTimeout(() => searchInputRef.current?.focus(), 0);
        return;
      }
      const tag = (e.target as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;

      if (section === "proposals" && activeProposal && !busy) {
        if (e.key.toLowerCase() === "a") {
          e.preventDefault();
          void handleProposal(activeProposal, "approve");
        } else if (e.key.toLowerCase() === "r") {
          e.preventDefault();
          void handleProposal(activeProposal, "reject");
        } else if (e.key.toLowerCase() === "j" || e.key.toLowerCase() === "k") {
          const idx = filteredProposals.findIndex((p) => p.id === activeProposal.id);
          const next = e.key.toLowerCase() === "j" ? idx + 1 : idx - 1;
          if (next >= 0 && next < filteredProposals.length) {
            setSelectedPropId(filteredProposals[next].id);
          }
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [section, activeProposal, filteredProposals, busy, handleProposal]);

  const mustProposeSelected =
    selectedNote !== null && (selectedNote.default_write !== "direct" || Boolean(selectedNote.owner));

  // Health gauge score for the left atmospheric focus panel
  const gaugePct = React.useMemo(() => {
    if (notes.length === 0) return 100;
    const isDecisionLog = (p: string) => p.startsWith("decisions/") || p.startsWith("decisioni/");
    const healthy = notes.filter((n) => isDecisionLog(n.path) || n.size_bytes <= 8192).length;
    return Math.round((healthy / notes.length) * 100);
  }, [notes]);

  const renderDiffLines = (diff: string) => {
    let oldLine = 1;
    let newLine = 1;
    return diff
      .trimEnd()
      .split("\n")
      .map((line, i) => {
        if (line.startsWith("--- ") || line.startsWith("+++ ")) return null;
        if (line.startsWith("@@")) {
          const m = line.match(/line\s+(\d+)/);
          if (m) {
            oldLine = Math.max(1, Number(m[1]) - 3);
            newLine = oldLine;
          }
          return (
            <div key={i} className="border-y border-border bg-[#141418] px-3 py-1.5 text-xs text-muted-foreground">
              {line}
            </div>
          );
        }
        const isAdd = line.startsWith("+ ") || line.startsWith("+");
        const isDel = line.startsWith("- ") || line.startsWith("-");
        const leftNum = isAdd ? "" : String(oldLine++);
        const rightNum = isDel ? "" : String(newLine++);
        const sign = isAdd ? "+" : isDel ? "−" : " ";
        const text = isAdd || isDel ? line.replace(/^[+-]\s?/, "") : line.replace(/^ {1,2}/, "");

        return (
          <div
            key={i}
            className={cn(
              "grid grid-cols-[36px_36px_22px_1fr] text-[13px] leading-6",
              isAdd && "bg-emerald-500/10 text-emerald-200",
              isDel && "bg-red-500/10 text-red-200",
              !isAdd && !isDel && "text-zinc-400",
            )}
          >
            <span className="select-none border-r border-border/60 pr-2 text-right text-[11px] tabular-nums text-zinc-600">
              {leftNum}
            </span>
            <span className="select-none border-r border-border/60 pr-2 text-right text-[11px] tabular-nums text-zinc-600">
              {rightNum}
            </span>
            <span
              className={cn(
                "select-none text-center font-semibold",
                isAdd && "text-emerald-400",
                isDel && "text-red-400",
                !isAdd && !isDel && "text-zinc-600",
              )}
            >
              {sign}
            </span>
            <span className="whitespace-pre-wrap break-words px-2.5">{text || " "}</span>
          </div>
        );
      });
  };

  const navTabs: Array<{ id: Section; label: string; count?: number }> = [
    { id: "overview", label: "Overview" },
    { id: "notes", label: "Notes", count: notes.length },
    { id: "graph", label: "Graph" },
    { id: "proposals", label: "Proposals", count: proposals.length || undefined },
    { id: "audit", label: "Audit" },
    { id: "settings", label: "Settings" },
  ];

  // Semicircle ticks for the atmospheric left gauge (inspired by reference)
  const ticks = 36;
  const activeTicks = Math.round((gaugePct / 100) * ticks);

  return (
    <div className="flex h-dvh w-full overflow-hidden bg-[#09090b] text-foreground">
      {/* Left Atmospheric Focus Panel (desktop) */}
      <aside className="hidden lg:flex lg:w-[292px] lg:shrink-0 lg:flex-col lg:justify-between border-r border-border bg-gradient-to-b from-[#161a20] via-[#13151b] to-[#17151c] p-5">
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="inline-flex h-6 w-6 items-center justify-center rounded-[5px] border border-white/15 bg-white/5 text-[11px] font-semibold text-white">
                mt
              </span>
              <span className="text-sm font-medium tracking-tight text-foreground">memtwig</span>
            </div>
            <span className="text-[11px] text-muted-foreground">local</span>
          </div>

          {/* Vault quick selector */}
          <div className="flex flex-wrap items-center gap-1.5">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setVaultFilter("all")}
              className={cn(
                "h-7 rounded-md border border-transparent px-2.5 text-xs",
                vaultFilter === "all" && "border-white/15 bg-white/10 text-white hover:bg-white/15",
              )}
            >
              All
            </Button>
            {vaults.map((v) => (
              <Button
                key={v.name}
                variant="ghost"
                size="sm"
                onClick={() => setVaultFilter(v.name)}
                className={cn(
                  "h-7 rounded-md border border-transparent px-2.5 text-xs",
                  vaultFilter === v.name && "border-white/15 bg-white/10 text-white hover:bg-white/15",
                )}
              >
                {v.name}
              </Button>
            ))}
          </div>

          {/* Minimal Arc Gauge */}
          <div className="pt-2 text-center">
            <svg viewBox="0 0 220 124" className="mx-auto w-[210px]">
              {Array.from({ length: ticks }, (_, i) => {
                const angle = Math.PI + (i / (ticks - 1)) * Math.PI;
                const x1 = 110 + Math.cos(angle) * 78;
                const y1 = 108 + Math.sin(angle) * 78;
                const x2 = 110 + Math.cos(angle) * 90;
                const y2 = 108 + Math.sin(angle) * 90;
                const lit = i < activeTicks;
                return (
                  <line
                    key={i}
                    x1={x1}
                    y1={y1}
                    x2={x2}
                    y2={y2}
                    stroke={lit ? "#5eead4" : "#2b2d36"}
                    strokeOpacity={lit ? 0.85 : 0.7}
                    strokeWidth={2}
                    strokeLinecap="round"
                  />
                );
              })}
              <text
                x="110"
                y="92"
                textAnchor="middle"
                fill="#fafafa"
                fontSize="30"
                fontWeight="300"
              >
                {gaugePct}%
              </text>
              <text x="110" y="112" textAnchor="middle" fill="#8a8b94" fontSize="11">
                Vault hygiene
              </text>
            </svg>
          </div>

          {/* Focused queue list */}
          <div className="space-y-2">
            <div className="flex items-center justify-between px-1 text-[11px] text-muted-foreground">
              <span>Queue</span>
              <span>{proposals.length} open</span>
            </div>
            {proposals.length === 0 ? (
              <div className="rounded-md border border-white/8 bg-white/[0.03] px-3.5 py-3 text-xs text-muted-foreground">
                No pending proposals
              </div>
            ) : (
              proposals.slice(0, 4).map((p) => (
                <Button
                  key={p.id}
                  variant="ghost"
                  onClick={() => {
                    setSelectedPropId(p.id);
                    setSection("proposals");
                  }}
                  className="h-auto w-full flex-col items-start rounded-md border border-white/8 bg-white/[0.04] px-3.5 py-2.5 text-left hover:bg-white/[0.08]"
                >
                  <div className="flex w-full items-center justify-between gap-2">
                    <span className="truncate text-xs font-medium text-foreground">{p.path}</span>
                    <span className="shrink-0 text-[11px] text-muted-foreground">{p.vault}</span>
                  </div>
                  <span className="mt-0.5 truncate w-full text-[11px] text-muted-foreground">
                    {p.reason || p.author}
                  </span>
                </Button>
              ))
            )}
          </div>
        </div>

        <div className="space-y-2 border-t border-white/10 pt-3">
          {notice && (
            <div className="rounded-md border border-white/10 bg-white/5 px-3 py-1.5 text-xs text-foreground">
              {notice}
            </div>
          )}
          <div className="flex items-center justify-between rounded-md border border-white/10 bg-white/[0.03] px-2.5 py-1.5 text-xs text-muted-foreground">
            <span>SQLite FTS5 · 1024d</span>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Reindex"
              disabled={busy}
              className="h-6 w-6"
              onClick={() => void triggerAction("/api/v1/reindex", "Index refreshed")}
            >
              <ArrowsClockwise size={13} />
            </Button>
          </div>
        </div>
      </aside>

      {/* Main Column with Clean Horizontal Top Navigation */}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-border bg-[#0c0c0f] px-3 md:px-8">
          <div className="flex min-w-0 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            <span className="mr-1.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-[5px] border border-white/15 bg-white/5 text-[11px] font-semibold text-white lg:hidden">
              mt
            </span>
            {navTabs.map((t) => {
              const active = section === t.id;
              return (
                <Button
                  key={t.id}
                  variant={active ? "default" : "ghost"}
                  size="sm"
                  onClick={() => {
                    setSection(t.id);
                    setMobileShowDetail(false);
                  }}
                  className={cn("h-8 shrink-0 px-2.5 text-xs md:px-3", !active && "text-muted-foreground")}
                >
                  {t.label}
                  {t.count !== undefined && (
                    <span className={cn("ml-1 tabular-nums", active ? "opacity-75" : "text-muted-foreground")}>
                      {t.count}
                    </span>
                  )}
                </Button>
              );
            })}
          </div>

          <div className="hidden shrink-0 items-center gap-1.5 sm:flex">
            <Button
              variant="ghost"
              size="icon"
              aria-label="Reindex"
              disabled={busy}
              onClick={() => void triggerAction("/api/v1/reindex", "Index refreshed")}
            >
              <ArrowsClockwise size={15} />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Settings"
              onClick={() => setSection("settings")}
            >
              <SlidersHorizontal size={15} />
            </Button>
          </div>
        </header>

        {/* OVERVIEW */}
        {section === "overview" && (
          <OverviewView
            vaults={vaults}
            notes={notes}
            nodes={graphNodes}
            edges={graphEdges}
            proposals={proposals}
            auditEvents={auditEvents}
            busy={busy}
            onCurate={() => void triggerAction("/api/v1/curate", "Curation completed")}
            onOpenSection={(s) => setSection(s)}
            onOpenProposal={(id) => {
              setSelectedPropId(id);
              setSection("proposals");
            }}
          />
        )}

        {/* NOTES */}
        {section === "notes" && (
          <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
            <section
              className={cn(
                "flex w-full shrink-0 flex-col border-r border-border bg-[#0d0d10] md:w-[296px]",
                mobileShowDetail && "hidden md:flex",
              )}
            >
              <form onSubmit={runSearch} className="flex items-center gap-1.5 border-b border-border p-2.5">
                <div className="relative flex-1 min-w-0">
                  <Input
                    ref={searchInputRef}
                    value={query}
                    onChange={(e) => {
                      setQuery(e.target.value);
                      if (e.target.value === "") setSearchHits(null);
                    }}
                    placeholder="Search... (⌘K)"
                    className="pr-7"
                  />
                  {query && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label="Clear search"
                      onClick={() => {
                        setQuery("");
                        setSearchHits(null);
                      }}
                      className="absolute right-0.5 top-0.5 h-7 w-7"
                    >
                      <X size={13} />
                    </Button>
                  )}
                </div>
                <Button type="submit" variant="outline" size="icon" aria-label="Search">
                  <MagnifyingGlass size={14} />
                </Button>
              </form>

              <div className="flex-1 overflow-y-auto divide-y divide-border/60">
                {visibleNotes.length === 0 ? (
                  <div className="p-5 text-xs text-muted-foreground">No notes</div>
                ) : (
                  visibleNotes.map((n) => {
                    const isSel = selectedNote?.vault === n.vault && selectedNote?.path === n.path;
                    return (
                      <Button
                        key={`${n.vault}:${n.path}`}
                        variant="ghost"
                        onClick={() => void openNote(n.vault, n.path, true)}
                        className={cn(
                          "h-auto w-full flex-col items-start rounded-none border-l-2 border-l-transparent px-3.5 py-2.5 text-left whitespace-normal hover:bg-[#141418]",
                          isSel && "border-l-zinc-200 bg-[#17171c] hover:bg-[#17171c]",
                        )}
                      >
                        <div className="flex w-full items-center justify-between gap-2">
                          <span className="truncate text-[13px] font-medium text-foreground">{n.title}</span>
                          <span className="shrink-0 text-[11px] text-muted-foreground">{n.vault}</span>
                        </div>
                        <div className="mt-0.5 flex w-full items-center justify-between gap-2 text-[11.5px] text-muted-foreground">
                          <span className="truncate">{n.path}</span>
                          {n.meta && <span className="shrink-0 tabular-nums">{n.meta}</span>}
                        </div>
                      </Button>
                    );
                  })
                )}
              </div>
            </section>

            <main
              className={cn(
                "flex min-h-0 min-w-0 flex-1 flex-col bg-background",
                !mobileShowDetail && "hidden md:flex",
              )}
            >
              {selectedNote ? (
                <>
                  <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border bg-[#0c0c0e] px-4 md:px-7">
                    <div className="flex min-w-0 items-center gap-2 text-[12.5px] text-muted-foreground">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Back to list"
                        onClick={() => setMobileShowDetail(false)}
                        className="h-7 w-7 md:hidden"
                      >
                        <ArrowLeft size={15} />
                      </Button>
                      <span className="truncate">
                        {selectedNote.vault} / <strong className="font-medium text-foreground">{selectedNote.path}</strong>
                      </span>
                    </div>

                    <div className="flex shrink-0 items-center gap-2">
                      <div className="inline-flex rounded-md border border-border bg-background p-0.5">
                        {(["read", "raw", "edit"] as NoteMode[]).map((m) => (
                          <Button
                            key={m}
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              if (m === "edit") setDraftContent(selectedNote.raw);
                              setNoteMode(m);
                            }}
                            className={cn(
                              "h-6 rounded-[4px] px-2 text-xs capitalize",
                              noteMode === m && "bg-[#1d1d22] text-foreground hover:bg-[#1d1d22]",
                            )}
                          >
                            {m}
                          </Button>
                        ))}
                      </div>
                      {noteMode === "edit" && (
                        <Button size="sm" disabled={busy} onClick={() => void saveOrProposeNote()}>
                          {mustProposeSelected ? "Propose" : "Save"}
                        </Button>
                      )}
                    </div>
                  </div>

                  <div className="flex-1 overflow-y-auto px-5 py-6 md:px-12 md:py-8">
                    <div className="mx-auto max-w-[700px]">
                      {openProposalsForNote.length > 0 && (
                        <div className="mb-5 flex items-center justify-between gap-3 rounded-md border border-border bg-[#111115] px-3.5 py-2 text-[12.5px] text-zinc-200">
                          <span>{openProposalsForNote.length} open proposal on this note</span>
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-6 px-2 text-xs"
                            onClick={() => {
                              setSelectedPropId(openProposalsForNote[0].id);
                              setSection("proposals");
                            }}
                          >
                            Review diff
                          </Button>
                        </div>
                      )}

                      {noteMode === "read" && (
                        <>
                          <h1 className="text-[26px] font-medium tracking-tight text-foreground leading-tight">
                            {selectedNote.title || selectedNote.path}
                          </h1>
                          <div className="mt-2.5 flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border pb-4 text-xs text-muted-foreground">
                            <span>{selectedNote.status || "active"}</span>
                            <span>{selectedNote.default_write || "direct"}</span>
                            {selectedNote.updated && <span className="tabular-nums">{selectedNote.updated}</span>}
                            <span className="tabular-nums">
                              {formatBytes(selectedNote.size_bytes)}
                              {selectedNote.max_bytes ? ` / ${formatBytes(selectedNote.max_bytes)}` : ""}
                            </span>
                            {selectedNote.owner && (
                              <span className="inline-flex items-center gap-1 text-zinc-300">
                                <LockSimple size={12} />
                                {selectedNote.owner}
                              </span>
                            )}
                          </div>
                          <div className="mt-5">
                            <MarkdownView raw={selectedNote.raw} skipTitle />
                          </div>

                          {noteConnections.length > 0 && (
                            <div className="mt-8 border-t border-border pt-5">
                              <div className="mb-2.5 flex items-center justify-between text-xs">
                                <span className="font-medium text-muted-foreground">
                                  Linked ({noteConnections.length})
                                </span>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="h-6 px-2 text-xs text-foreground"
                                  onClick={() => setSection("graph")}
                                >
                                  View graph
                                </Button>
                              </div>
                              <div className="grid gap-1.5 sm:grid-cols-2">
                                {noteConnections.map((c, idx) => (
                                  <Button
                                    key={idx}
                                    variant="outline"
                                    size="sm"
                                    onClick={() => c.node?.path && void openNote(c.node.vault, c.node.path)}
                                    className="h-auto justify-between px-3 py-2 text-left"
                                  >
                                    <span className="truncate text-foreground">
                                      {c.dir === "out" ? "→ " : "← "}
                                      {c.node?.label}
                                    </span>
                                    <span className="text-[11px] text-muted-foreground">{c.kind}</span>
                                  </Button>
                                ))}
                              </div>
                            </div>
                          )}
                        </>
                      )}

                      {noteMode === "raw" && (
                        <div className="rounded-md border border-border bg-[#0e0e11] p-4 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-zinc-300">
                          {selectedNote.raw}
                        </div>
                      )}

                      {noteMode === "edit" && (
                        <div className="space-y-3">
                          {mustProposeSelected && (
                            <Input
                              value={draftReason}
                              onChange={(e) => setDraftReason(e.target.value)}
                              placeholder="Reason for proposed edit..."
                            />
                          )}
                          <Textarea
                            value={draftContent}
                            onChange={(e) => setDraftContent(e.target.value)}
                            rows={22}
                            className="min-h-[60vh]"
                          />
                        </div>
                      )}
                    </div>
                  </div>
                </>
              ) : (
                <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                  Select a note
                </div>
              )}
            </main>
          </div>
        )}

        {/* GRAPH */}
        {section === "graph" && (
          <GraphView
            nodes={graphNodes}
            edges={graphEdges}
            vaultFilter={vaultFilter}
            onOpenNote={(v, p) => {
              setSection("notes");
              void openNote(v, p, true);
            }}
            onOpenProposals={(v) => {
              setVaultFilter(v);
              setSection("proposals");
            }}
          />
        )}

        {/* PROPOSALS */}
        {section === "proposals" && (
          <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
            <section
              className={cn(
                "flex w-full shrink-0 flex-col border-r border-border bg-[#0d0d10] md:w-[310px]",
                mobileShowDetail && "hidden md:flex",
              )}
            >
              <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-3.5 text-[13px] font-medium text-foreground">
                <span>Proposals ({filteredProposals.length})</span>
                <span className="text-[11px] text-muted-foreground">J/K · A · R</span>
              </div>
              <div className="flex-1 overflow-y-auto divide-y divide-border/60">
                {filteredProposals.length === 0 ? (
                  <div className="p-5 text-xs text-muted-foreground">No open proposals</div>
                ) : (
                  filteredProposals.map((p) => {
                    const isSel = activeProposal?.id === p.id;
                    const adds = countDiffAdditions(p.diff);
                    return (
                      <Button
                        key={p.id}
                        variant="ghost"
                        onClick={() => {
                          setSelectedPropId(p.id);
                          setMobileShowDetail(true);
                        }}
                        className={cn(
                          "h-auto w-full flex-col items-start rounded-none border-l-2 border-l-transparent px-3.5 py-3 text-left whitespace-normal hover:bg-[#141418]",
                          isSel && "border-l-zinc-200 bg-[#17171c] hover:bg-[#17171c]",
                        )}
                      >
                        <div className="flex w-full items-center justify-between gap-2">
                          <span className="truncate text-[13px] font-medium text-foreground">{p.path}</span>
                          <span className="text-[11px] text-muted-foreground">{p.vault}</span>
                        </div>
                        <span className="mt-1 line-clamp-2 w-full text-xs text-muted-foreground">
                          {p.reason || p.kind}
                        </span>
                        <div className="mt-2 flex w-full items-center justify-between text-[11px] text-muted-foreground">
                          <span>
                            {p.author} · {p.created_at}
                          </span>
                          <span className="text-emerald-400 tabular-nums">+{adds}</span>
                        </div>
                      </Button>
                    );
                  })
                )}
              </div>
            </section>

            <main
              className={cn(
                "flex min-h-0 min-w-0 flex-1 flex-col bg-background",
                !mobileShowDetail && "hidden md:flex",
              )}
            >
              {activeProposal ? (
                <>
                  <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border bg-[#0c0c0e] px-4 md:px-7">
                    <div className="flex min-w-0 items-center gap-2 text-[13px]">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Back to proposals"
                        onClick={() => setMobileShowDetail(false)}
                        className="h-7 w-7 md:hidden"
                      >
                        <ArrowLeft size={15} />
                      </Button>
                      <strong className="truncate font-medium text-foreground">
                        {activeProposal.vault} / {activeProposal.path}
                      </strong>
                      <span className="hidden text-xs text-muted-foreground sm:inline">
                        {activeProposal.id.slice(0, 12)}
                      </span>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <Button
                        size="sm"
                        variant="destructive"
                        disabled={busy}
                        onClick={() => void handleProposal(activeProposal, "reject")}
                      >
                        Reject
                      </Button>
                      <Button
                        size="sm"
                        disabled={busy}
                        onClick={() => void handleProposal(activeProposal, "approve")}
                      >
                        <Check size={14} />
                        Approve
                      </Button>
                    </div>
                  </div>

                  <div className="flex-1 overflow-y-auto p-4 md:p-7">
                    <div className="mx-auto max-w-[820px] space-y-4">
                      <div className="flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-muted-foreground">
                        <span>{activeProposal.author}</span>
                        <span>{activeProposal.kind}</span>
                        <span className="tabular-nums">{activeProposal.created_at}</span>
                        {activeProposal.reason && <span className="text-foreground">{activeProposal.reason}</span>}
                      </div>

                      <div className="overflow-hidden rounded-md border border-border bg-[#0d0d10]">
                        <div className="overflow-x-auto py-1">{renderDiffLines(activeProposal.diff)}</div>
                      </div>
                    </div>
                  </div>
                </>
              ) : (
                <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                  No open proposals
                </div>
              )}
            </main>
          </div>
        )}

        {/* AUDIT */}
        {section === "audit" && (
          <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background">
            <div className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-[#0c0c0e] px-5 md:px-7">
              <span className="text-[13px] font-medium text-foreground">Audit ({auditEvents.length})</span>
            </div>
            <div className="flex-1 overflow-auto">
              <table className="w-full min-w-[600px] border-collapse text-left text-[13px]">
                <thead>
                  <tr className="border-b border-border text-xs text-muted-foreground">
                    <th className="px-5 py-2.5 font-medium">Time</th>
                    <th className="px-5 py-2.5 font-medium">Actor</th>
                    <th className="px-5 py-2.5 font-medium">Action</th>
                    <th className="px-5 py-2.5 font-medium">Target</th>
                    <th className="px-5 py-2.5 font-medium">Outcome</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/60">
                  {auditEvents.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-5 py-8 text-center text-xs text-muted-foreground">
                        No events
                      </td>
                    </tr>
                  ) : (
                    auditEvents.map((ev, idx) => (
                      <tr key={idx} className="hover:bg-[#111114]">
                        <td className="px-5 py-2.5 tabular-nums text-muted-foreground">
                          {ev.timestamp.replace("T", " ").slice(0, 19)}
                        </td>
                        <td className="px-5 py-2.5 font-medium text-foreground">{ev.actor}</td>
                        <td className="px-5 py-2.5 text-foreground">{ev.action}</td>
                        <td className="px-5 py-2.5 text-muted-foreground">
                          {ev.vault ? `${ev.vault}${ev.path ? ":" + ev.path : ""}` : ev.detail || "-"}
                        </td>
                        <td className="px-5 py-2.5 text-xs text-muted-foreground">{ev.outcome}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </main>
        )}

        {/* SETTINGS */}
        {section === "settings" && <SettingsView onChanged={loadAll} flash={flash} />}
      </div>
    </div>
  );
}
