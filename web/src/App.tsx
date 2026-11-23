import * as React from "react";
import {
  ArrowLeft,
  Check,
  ClockCounterClockwise,
  Files,
  FolderSimple,
  GitDiff,
  LockSimple,
  MagnifyingGlass,
  X,
} from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownView } from "@/components/MarkdownView";
import { cn } from "@/lib/utils";

type Section = "notes" | "proposals" | "audit";
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

function vaultVariant(kind?: string): "default" | "blue" | "neutral" {
  if (kind === "org") return "default";
  return "neutral";
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  return `${(n / 1024).toFixed(1)} KB`;
}

function folderOf(path: string): string {
  const idx = path.indexOf("/");
  return idx === -1 ? "root" : path.slice(0, idx) + "/";
}

function countDiffAdditions(diff: string): number {
  return diff.split("\n").filter((l) => l.startsWith("+ ") || (l.startsWith("+") && !l.startsWith("+++"))).length;
}

export function App() {
  const initialSection = (): Section => {
    const p = new URLSearchParams(window.location.search).get("tab");
    if (p === "proposals" || p === "audit") return p;
    return "notes";
  };

  const [section, setSection] = React.useState<Section>(initialSection);
  const [vaults, setVaults] = React.useState<VaultInfo[]>([]);
  const [vaultFilter, setVaultFilter] = React.useState<string>("all");
  const [folderFilter, setFolderFilter] = React.useState<string>("all");

  const [notes, setNotes] = React.useState<NoteSummary[]>([]);
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
      const [stRes, notesRes, propRes, audRes] = await Promise.all([
        fetch("/api/v1/status"),
        fetch("/api/v1/notes"),
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

  // Derived folders and filtered lists
  const folders = React.useMemo(() => {
    const counts = new Map<string, number>();
    for (const n of notes) {
      if (vaultFilter !== "all" && n.vault !== vaultFilter) continue;
      const f = folderOf(n.path);
      counts.set(f, (counts.get(f) ?? 0) + 1);
    }
    return Array.from(counts.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [notes, vaultFilter]);

  const visibleNotes = React.useMemo(() => {
    if (searchHits) {
      return searchHits
        .filter((h) => vaultFilter === "all" || h.vault === vaultFilter)
        .map((h) => ({
          vault: h.vault,
          path: h.path,
          title: h.title || h.path,
          snippet: h.snippet,
          meta: `score ${h.score.toFixed(4)}`,
          owner: "",
        }));
    }
    return notes
      .filter((n) => (vaultFilter === "all" || n.vault === vaultFilter) && (folderFilter === "all" || folderOf(n.path) === folderFilter))
      .map((n) => ({
        vault: n.vault,
        path: n.path,
        title: n.title || n.path,
        snippet: n.path,
        meta: n.updated || n.status || "",
        owner: n.owner,
      }));
  }, [searchHits, notes, vaultFilter, folderFilter]);

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

  const vaultKindMap = React.useMemo(() => {
    const m = new Map<string, VaultInfo>();
    for (const v of vaults) m.set(v.name, v);
    return m;
  }, [vaults]);

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
          flash(`Proposal queued for ${selectedNote.path}`);
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
          flash(`Saved ${selectedNote.path}`);
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

  // Keyboard shortcuts: Cmd/Ctrl+K search, A/R on proposals, J/K navigate proposals
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

  const renderDiffLines = (diff: string) => {
    let oldLine = 1;
    let newLine = 1;
    return diff.trimEnd().split("\n").map((line, i) => {
      if (line.startsWith("--- ") || line.startsWith("+++ ")) {
        return null;
      }
      if (line.startsWith("@@")) {
        const m = line.match(/line\s+(\d+)/);
        if (m) {
          oldLine = Math.max(1, Number(m[1]) - 3);
          newLine = oldLine;
        }
        return (
          <div key={i} className="border-y border-border bg-[#f0f7f6] px-3 py-1.5 text-xs font-medium text-primary">
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
            isAdd && "bg-emerald-50/90 text-emerald-950",
            isDel && "bg-red-50/90 text-red-950",
            !isAdd && !isDel && "text-[#334241]",
          )}
        >
          <span className="select-none border-r border-border/60 pr-2 text-right text-[11px] tabular-nums text-muted-foreground/70">
            {leftNum}
          </span>
          <span className="select-none border-r border-border/60 pr-2 text-right text-[11px] tabular-nums text-muted-foreground/70">
            {rightNum}
          </span>
          <span
            className={cn(
              "select-none text-center font-semibold",
              isAdd && "text-emerald-700",
              isDel && "text-red-700",
              !isAdd && !isDel && "text-muted-foreground/50",
            )}
          >
            {sign}
          </span>
          <span className="whitespace-pre-wrap break-words px-2.5">{text || " "}</span>
        </div>
      );
    });
  };

  return (
    <div className="flex h-dvh w-full flex-col overflow-hidden bg-background md:flex-row">
      {/* Desktop Left Rail */}
      <aside className="hidden md:flex md:w-[228px] md:shrink-0 md:flex-col md:justify-between border-r border-border bg-[#f8f9f9]">
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-3 py-3.5">
          <div className="mb-3 flex items-center justify-between px-2">
            <div className="flex items-center gap-2">
              <span className="inline-flex h-6 w-6 items-center justify-center rounded-[5px] bg-[#111818] text-[11px] font-semibold text-[#5fd3c7]">
                mt
              </span>
              <span className="text-[14px] font-semibold tracking-tight text-foreground">memtwig</span>
            </div>
            <span className="text-[11px] text-muted-foreground">studio</span>
          </div>

          <nav className="space-y-0.5">
            <Button
              variant="ghost"
              onClick={() => setSection("notes")}
              className={cn(
                "h-8 w-full justify-between px-2 text-[13px] font-normal",
                section === "notes" && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
              )}
            >
              <span className="flex items-center gap-2">
                <Files size={15} weight={section === "notes" ? "fill" : "regular"} />
                <span>Notes</span>
              </span>
              <span className="text-[11px] tabular-nums text-muted-foreground">{notes.length}</span>
            </Button>

            <Button
              variant="ghost"
              onClick={() => setSection("proposals")}
              className={cn(
                "h-8 w-full justify-between px-2 text-[13px] font-normal",
                section === "proposals" && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
              )}
            >
              <span className="flex items-center gap-2">
                <GitDiff size={15} weight={section === "proposals" ? "fill" : "regular"} />
                <span>Proposals</span>
              </span>
              {proposals.length > 0 && (
                <Badge variant="default" className="tabular-nums">
                  {proposals.length}
                </Badge>
              )}
            </Button>

            <Button
              variant="ghost"
              onClick={() => setSection("audit")}
              className={cn(
                "h-8 w-full justify-between px-2 text-[13px] font-normal",
                section === "audit" && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
              )}
            >
              <span className="flex items-center gap-2">
                <ClockCounterClockwise size={15} weight={section === "audit" ? "fill" : "regular"} />
                <span>Audit log</span>
              </span>
              <span className="text-[11px] tabular-nums text-muted-foreground">{auditEvents.length}</span>
            </Button>
          </nav>

          <div className="mt-5 px-2 text-[10.5px] font-semibold uppercase tracking-wider text-muted-foreground">
            Vaults
          </div>
          <div className="mt-1 space-y-0.5">
            <Button
              variant="ghost"
              onClick={() => {
                setVaultFilter("all");
                setFolderFilter("all");
              }}
              className={cn(
                "h-7 w-full justify-between px-2 text-[12.5px] font-normal",
                vaultFilter === "all" && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
              )}
            >
              <span>All vaults</span>
              <span className="text-[11px] tabular-nums text-muted-foreground">{notes.length}</span>
            </Button>

            {vaults.map((v) => {
              const count = notes.filter((n) => n.vault === v.name).length;
              const active = vaultFilter === v.name;
              return (
                <Button
                  key={v.name}
                  variant="ghost"
                  onClick={() => {
                    setVaultFilter(v.name);
                    setFolderFilter("all");
                  }}
                  className={cn(
                    "h-7 w-full justify-between px-2 text-[12.5px] font-normal",
                    active && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
                  )}
                >
                  <span className="flex items-center gap-2 truncate">
                    <span
                      className={cn(
                        "h-1.5 w-1.5 shrink-0 rounded-[2px]",
                        v.kind === "org" ? "bg-primary" : "bg-amber-500",
                      )}
                    />
                    <span className="truncate">{v.name}</span>
                  </span>
                  <span className="text-[11px] tabular-nums text-muted-foreground">{count}</span>
                </Button>
              );
            })}
          </div>

          {section === "notes" && folders.length > 0 && (
            <>
              <div className="mt-5 px-2 text-[10.5px] font-semibold uppercase tracking-wider text-muted-foreground">
                Folders
              </div>
              <div className="mt-1 space-y-0.5">
                {folders.map(([folder, count]) => {
                  const active = folderFilter === folder;
                  return (
                    <Button
                      key={folder}
                      variant="ghost"
                      onClick={() => setFolderFilter(active ? "all" : folder)}
                      className={cn(
                        "h-7 w-full justify-between px-2 text-[12.5px] font-normal",
                        active && "bg-[#e9efef] font-medium text-foreground hover:bg-[#e9efef]",
                      )}
                    >
                      <span className="flex items-center gap-1.5 truncate">
                        <FolderSimple size={13} />
                        <span className="truncate">{folder}</span>
                      </span>
                      <span className="text-[11px] tabular-nums text-muted-foreground">{count}</span>
                    </Button>
                  );
                })}
              </div>
            </>
          )}
        </div>

        <div className="border-t border-border p-3">
          {notice && (
            <div className="mb-2 rounded-md border border-teal-200 bg-teal-50 px-2.5 py-1.5 text-[11.5px] font-medium text-teal-900">
              {notice}
            </div>
          )}
          <div className="grid grid-cols-2 gap-1.5">
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void triggerAction("/api/v1/curate", "Curation completed")}
            >
              Curate
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void triggerAction("/api/v1/reindex", "Index refreshed")}
            >
              Reindex
            </Button>
          </div>
        </div>
      </aside>

      {/* Mobile Top Navigation Bar (flat, crisp, zero pill bloat) */}
      <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-[#f8f9f9] px-3 md:hidden">
        <div className="flex items-center gap-2">
          <span className="inline-flex h-6 w-6 items-center justify-center rounded-[4px] bg-[#111818] text-[11px] font-semibold text-[#5fd3c7]">
            mt
          </span>
          <span className="text-sm font-semibold text-foreground">memtwig</span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant={section === "notes" ? "outline" : "ghost"}
            size="sm"
            onClick={() => {
              setSection("notes");
              setMobileShowDetail(false);
            }}
          >
            Notes
          </Button>
          <Button
            variant={section === "proposals" ? "outline" : "ghost"}
            size="sm"
            onClick={() => {
              setSection("proposals");
              setMobileShowDetail(false);
            }}
          >
            Proposals{proposals.length > 0 ? ` (${proposals.length})` : ""}
          </Button>
          <Button
            variant={section === "audit" ? "outline" : "ghost"}
            size="sm"
            onClick={() => {
              setSection("audit");
              setMobileShowDetail(false);
            }}
          >
            Audit
          </Button>
        </div>
      </header>

      {/* Main Workspace Area */}
      {section === "notes" && (
        <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
          {/* Middle Note List Pane */}
          <section
            className={cn(
              "flex w-full shrink-0 flex-col border-r border-border bg-[#fbfcfc] md:w-[304px]",
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
                  placeholder="Search vaults... (⌘K)"
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
                <MagnifyingGlass size={15} />
              </Button>
            </form>

            <div className="flex-1 overflow-y-auto divide-y divide-border/70">
              {visibleNotes.length === 0 ? (
                <div className="p-5 text-xs text-muted-foreground">No notes in scope</div>
              ) : (
                visibleNotes.map((n) => {
                  const isSel = selectedNote?.vault === n.vault && selectedNote?.path === n.path;
                  const vInfo = vaultKindMap.get(n.vault);
                  return (
                    <Button
                      key={`${n.vault}:${n.path}`}
                      variant="ghost"
                      onClick={() => void openNote(n.vault, n.path, true)}
                      className={cn(
                        "h-auto w-full flex-col items-start rounded-none border-l-2 border-l-transparent px-3.5 py-2.5 text-left whitespace-normal hover:bg-muted/60",
                        isSel && "border-l-primary bg-[#f0f5f5] hover:bg-[#f0f5f5]",
                      )}
                    >
                      <div className="flex w-full items-center justify-between gap-2">
                        <span className="truncate text-[13px] font-medium text-foreground">{n.title}</span>
                        <Badge variant={vaultVariant(vInfo?.kind)}>{n.vault}</Badge>
                      </div>
                      <div className="mt-0.5 flex w-full items-center justify-between gap-2 text-[11.5px] text-muted-foreground">
                        <span className="truncate">{n.snippet}</span>
                        {n.meta && <span className="shrink-0 tabular-nums">{n.meta}</span>}
                      </div>
                    </Button>
                  );
                })
              )}
            </div>
          </section>

          {/* Right Editorial Note Canvas */}
          <main
            className={cn(
              "flex min-h-0 min-w-0 flex-1 flex-col bg-white",
              !mobileShowDetail && "hidden md:flex",
            )}
          >
            {selectedNote ? (
              <>
                <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border px-4 md:px-7">
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
                    <div className="inline-flex rounded-md border border-border bg-muted p-0.5">
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
                            noteMode === m && "bg-white text-foreground shadow-none",
                          )}
                        >
                          {m}
                        </Button>
                      ))}
                    </div>

                    {noteMode === "edit" && (
                      <Button size="sm" disabled={busy} onClick={() => void saveOrProposeNote()}>
                        {mustProposeSelected ? "Propose edit" : "Save"}
                      </Button>
                    )}
                  </div>
                </div>

                <div className="flex-1 overflow-y-auto px-5 py-6 md:px-12 md:py-8">
                  <div className="mx-auto max-w-[700px]">
                    {openProposalsForNote.length > 0 && (
                      <div className="mb-5 flex items-center justify-between gap-3 rounded-md border border-teal-200 bg-teal-50/70 px-3.5 py-2 text-[12.5px] text-teal-950">
                        <span>
                          <strong>{openProposalsForNote.length} open proposal</strong> awaiting review on this note
                        </span>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-2 text-xs font-semibold text-primary"
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
                          <span>
                            Status: <strong className="font-medium text-foreground">{selectedNote.status || "active"}</strong>
                          </span>
                          <span>
                            Policy: <strong className="font-medium text-foreground">{selectedNote.default_write || "direct"}</strong>
                          </span>
                          {selectedNote.updated && (
                            <span className="tabular-nums">
                              Updated: <strong className="font-medium text-foreground">{selectedNote.updated}</strong>
                            </span>
                          )}
                          <span className="tabular-nums">
                            Size:{" "}
                            <strong className="font-medium text-foreground">
                              {formatBytes(selectedNote.size_bytes)}
                              {selectedNote.max_bytes ? ` / ${formatBytes(selectedNote.max_bytes)}` : ""}
                            </strong>
                          </span>
                          {selectedNote.owner && (
                            <span className="inline-flex items-center gap-1 text-amber-800">
                              <LockSimple size={12} />
                              owner: {selectedNote.owner}
                            </span>
                          )}
                        </div>
                        <div className="mt-5">
                          <MarkdownView raw={selectedNote.raw} skipTitle />
                        </div>
                      </>
                    )}

                    {noteMode === "raw" && (
                      <div className="rounded-md border border-border bg-[#f8f9f9] p-4 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-foreground">
                        {selectedNote.raw}
                      </div>
                    )}

                    {noteMode === "edit" && (
                      <div className="space-y-3">
                        {mustProposeSelected && (
                          <div className="space-y-2 rounded-md border border-border bg-muted/50 p-3">
                            <div className="text-xs text-muted-foreground">
                              This note is protected ({selectedNote.owner ? `owner: ${selectedNote.owner}` : `vault write: ${selectedNote.default_write}`}). Saving queues a reviewable proposal.
                            </div>
                            <Input
                              value={draftReason}
                              onChange={(e) => setDraftReason(e.target.value)}
                              placeholder="Reason for proposed edit..."
                            />
                          </div>
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
                Select a note to inspect
              </div>
            )}
          </main>
        </div>
      )}

      {section === "proposals" && (
        <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
          <section
            className={cn(
              "flex w-full shrink-0 flex-col border-r border-border bg-[#fbfcfc] md:w-[320px]",
              mobileShowDetail && "hidden md:flex",
            )}
          >
            <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-3.5 text-[13px] font-medium text-foreground">
              <span>Open proposals ({filteredProposals.length})</span>
              <span className="text-[11px] text-muted-foreground">J/K · A · R</span>
            </div>
            <div className="flex-1 overflow-y-auto divide-y divide-border/70">
              {filteredProposals.length === 0 ? (
                <div className="p-5 text-xs text-muted-foreground">
                  All clear. No open proposals waiting for review.
                </div>
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
                        "h-auto w-full flex-col items-start rounded-none border-l-2 border-l-transparent px-3.5 py-3 text-left whitespace-normal hover:bg-muted/60",
                        isSel && "border-l-primary bg-[#f0f5f5] hover:bg-[#f0f5f5]",
                      )}
                    >
                      <div className="flex w-full items-center justify-between gap-2">
                        <span className="truncate text-[13px] font-medium text-foreground">{p.path}</span>
                        <Badge variant="default">{p.vault}</Badge>
                      </div>
                      <span className="mt-1 line-clamp-2 w-full text-xs text-muted-foreground">
                        {p.reason || `${p.kind} proposal`}
                      </span>
                      <div className="mt-2 flex w-full items-center justify-between text-[11px] text-muted-foreground">
                        <span>
                          {p.author} · {p.created_at}
                        </span>
                        <span className="font-medium text-emerald-700 tabular-nums">+{adds}</span>
                      </div>
                    </Button>
                  );
                })
              )}
            </div>
          </section>

          <main
            className={cn(
              "flex min-h-0 min-w-0 flex-1 flex-col bg-white",
              !mobileShowDetail && "hidden md:flex",
            )}
          >
            {activeProposal ? (
              <>
                <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border px-4 md:px-7">
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
                    <Badge variant="neutral" className="hidden sm:inline-flex">
                      {activeProposal.id.slice(0, 12)}
                    </Badge>
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
                    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-[#f8f9f9] px-4 py-3 text-xs sm:grid-cols-4">
                      <div>
                        <div className="text-muted-foreground">Author</div>
                        <div className="mt-0.5 font-medium text-foreground">{activeProposal.author}</div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">Operation</div>
                        <div className="mt-0.5 font-medium text-foreground">{activeProposal.kind}</div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">Created</div>
                        <div className="mt-0.5 font-medium tabular-nums text-foreground">{activeProposal.created_at}</div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">Reason</div>
                        <div className="mt-0.5 truncate font-medium text-foreground">{activeProposal.reason || "-"}</div>
                      </div>
                    </div>

                    <div className="overflow-hidden rounded-md border border-border bg-white">
                      <div className="flex items-center justify-between border-b border-border bg-[#f8f9f9] px-3.5 py-2 text-xs text-muted-foreground">
                        <span className="font-medium text-foreground">{activeProposal.path}</span>
                        <span>Unified diff</span>
                      </div>
                      <div className="overflow-x-auto py-1">{renderDiffLines(activeProposal.diff)}</div>
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                No open proposals to review
              </div>
            )}
          </main>
        </div>
      )}

      {section === "audit" && (
        <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-white">
          <div className="flex h-12 shrink-0 items-center justify-between border-b border-border px-5 md:px-7">
            <span className="text-[13px] font-medium text-foreground">Audit log ({auditEvents.length})</span>
            <span className="text-xs text-muted-foreground">Append-only local history</span>
          </div>
          <div className="flex-1 overflow-auto">
            <table className="w-full min-w-[600px] border-collapse text-left text-[13px]">
              <thead>
                <tr className="border-b border-border bg-[#f8f9f9] text-xs text-muted-foreground">
                  <th className="px-5 py-2.5 font-medium">Timestamp</th>
                  <th className="px-5 py-2.5 font-medium">Actor</th>
                  <th className="px-5 py-2.5 font-medium">Action</th>
                  <th className="px-5 py-2.5 font-medium">Target</th>
                  <th className="px-5 py-2.5 font-medium">Outcome</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/70">
                {auditEvents.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-5 py-8 text-center text-xs text-muted-foreground">
                      No audit events recorded yet
                    </td>
                  </tr>
                ) : (
                  auditEvents.map((ev, idx) => (
                    <tr key={idx} className="hover:bg-muted/40">
                      <td className="px-5 py-2.5 tabular-nums text-muted-foreground">
                        {ev.timestamp.replace("T", " ").slice(0, 19)}
                      </td>
                      <td className="px-5 py-2.5 font-medium text-foreground">{ev.actor}</td>
                      <td className="px-5 py-2.5 text-foreground">{ev.action}</td>
                      <td className="px-5 py-2.5 text-muted-foreground">
                        {ev.vault ? `${ev.vault}${ev.path ? ":" + ev.path : ""}` : ev.detail || "-"}
                      </td>
                      <td className="px-5 py-2.5">
                        <Badge variant={ev.outcome === "ok" ? "default" : "warn"}>{ev.outcome}</Badge>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </main>
      )}
    </div>
  );
}
