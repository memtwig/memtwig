import * as React from "react";
import {
  ClockCounterClockwise,
  Files,
  GitDiff,
  HardDrives,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type Section = "notes" | "proposals" | "vaults" | "audit";

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

const navItems: Array<{ id: Section; label: string; icon: React.ComponentType<{ size?: number; weight?: "regular" | "fill" }> }> = [
  { id: "notes", label: "Notes", icon: Files },
  { id: "proposals", label: "Proposals", icon: GitDiff },
  { id: "vaults", label: "Vaults", icon: HardDrives },
  { id: "audit", label: "Audit", icon: ClockCounterClockwise },
];

export function App() {
  const initialSection = (): Section => {
    const p = new URLSearchParams(window.location.search).get("tab");
    if (p === "proposals" || p === "vaults" || p === "audit") return p;
    return "notes";
  };
  const [section, setSection] = React.useState<Section>(initialSection);
  const [vaults, setVaults] = React.useState<VaultInfo[]>([]);
  const [notes, setNotes] = React.useState<NoteSummary[]>([]);
  const [searchHits, setSearchHits] = React.useState<SearchHit[] | null>(null);
  const [query, setQuery] = React.useState("");
  const [selectedNote, setSelectedNote] = React.useState<{ vault: string; path: string; raw: string; title: string; updated: string; owner: string } | null>(null);
  const [proposals, setProposals] = React.useState<ProposalItem[]>([]);
  const [selectedPropId, setSelectedPropId] = React.useState<string | null>(null);
  const [auditEvents, setAuditEvents] = React.useState<AuditEvent[]>([]);
  const [busy, setBusy] = React.useState(false);
  const [notice, setNotice] = React.useState<string | null>(null);

  const loadAll = React.useCallback(async () => {
    try {
      const [stRes, notesRes, propRes, audRes] = await Promise.all([
        fetch("/api/v1/status"),
        fetch("/api/v1/notes"),
        fetch("/api/v1/proposals"),
        fetch("/api/v1/audit?n=40"),
      ]);
      if (stRes.ok) {
        const d = await stRes.json();
        setVaults(d.vaults ?? []);
      }
      if (notesRes.ok) {
        const d = await notesRes.json();
        const list: NoteSummary[] = d.notes ?? [];
        setNotes(list);
      }
      if (propRes.ok) {
        const d = await propRes.json();
        const list: ProposalItem[] = d.proposals ?? [];
        setProposals(list);
        if (list.length > 0) {
          setSelectedPropId((prev) => prev ?? list[0].id);
        } else {
          setSelectedPropId(null);
        }
      }
      if (audRes.ok) {
        const d = await audRes.json();
        setAuditEvents((d.events ?? []).slice().reverse());
      }
    } catch {
      setNotice("Backend unreachable");
    }
  }, []);

  React.useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const openNote = async (vault: string, path: string) => {
    const res = await fetch(`/api/v1/note?vault=${encodeURIComponent(vault)}&path=${encodeURIComponent(path)}`);
    if (res.ok) {
      const d = await res.json();
      setSelectedNote(d);
    }
  };

  React.useEffect(() => {
    if (!selectedNote && notes.length > 0) {
      void openNote(notes[0].vault, notes[0].path);
    }
  }, [notes, selectedNote]);

  const runSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    const q = query.trim();
    if (!q) {
      setSearchHits(null);
      return;
    }
    const res = await fetch(`/api/v1/search?q=${encodeURIComponent(q)}&n=20`);
    if (res.ok) {
      const d = await res.json();
      const hits: SearchHit[] = d.results ?? [];
      setSearchHits(hits);
      if (hits.length > 0) {
        void openNote(hits[0].vault, hits[0].path);
      }
    }
  };

  const handleProposal = async (item: ProposalItem, action: "approve" | "reject") => {
    setBusy(true);
    try {
      const res = await fetch(`/api/v1/proposals/${encodeURIComponent(item.vault)}/${encodeURIComponent(item.id)}/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(action === "reject" ? { reason: "rejected in studio" } : {}),
      });
      if (res.ok) {
        setNotice(action === "approve" ? `Approved ${item.path}` : `Rejected ${item.id}`);
        await loadAll();
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
        setNotice(label);
        await loadAll();
      }
    } finally {
      setBusy(false);
    }
  };

  const visibleNotes = searchHits
    ? searchHits.map((h) => ({ vault: h.vault, path: h.path, title: h.title, subtitle: h.snippet }))
    : notes.map((n) => ({ vault: n.vault, path: n.path, title: n.title, subtitle: n.updated || n.status }));

  const activeProposal = proposals.find((p) => p.id === selectedPropId) ?? proposals[0] ?? null;

  return (
    <div className="min-h-dvh flex">
      {/* Desktop sidebar */}
      <aside className="hidden md:flex md:w-60 md:shrink-0 md:flex-col md:justify-between sticky top-0 h-dvh border-r border-border/80 bg-white/55 px-4 py-5 backdrop-blur-xl">
        <div className="space-y-6">
          <div className="flex items-center gap-2.5 px-2">
            <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-[#0f2423] text-xs font-semibold text-[#5fd3c7]">
              mt
            </span>
            <span className="text-base font-semibold tracking-tight text-foreground">memtwig</span>
          </div>

          <nav className="space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const active = section === item.id;
              return (
                <Button
                  key={item.id}
                  variant="ghost"
                  onClick={() => setSection(item.id)}
                  className={cn(
                    "w-full justify-between px-3.5 h-10 rounded-full",
                    active && "bg-muted text-foreground font-semibold",
                  )}
                >
                  <span className="flex items-center gap-2.5">
                    <Icon size={18} weight={active ? "fill" : "regular"} />
                    <span>{item.label}</span>
                  </span>
                  {item.id === "proposals" && proposals.length > 0 && (
                    <Badge variant="default" className="tabular-nums">
                      {proposals.length}
                    </Badge>
                  )}
                </Button>
              );
            })}
          </nav>
        </div>

        <div className="space-y-2 px-1">
          <Button
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => void triggerAction("/api/v1/curate", "Curation completed")}
            className="w-full"
          >
            Run Curate
          </Button>
          <Button
            variant="ghost"
            size="sm"
            disabled={busy}
            onClick={() => void triggerAction("/api/v1/reindex", "Index refreshed")}
            className="w-full"
          >
            Reindex
          </Button>
        </div>
      </aside>

      {/* Main content */}
      <main className="min-w-0 flex-1 px-4 pt-6 pb-24 sm:px-8 md:py-8">
        <div className="mx-auto max-w-6xl space-y-6">
          <header className="flex flex-wrap items-center justify-between gap-3">
            <h1 className="text-3xl font-medium tracking-tight text-foreground sm:text-4xl capitalize">
              {section}
            </h1>
            {notice && <Badge variant="default">{notice}</Badge>}
          </header>

          {section === "notes" && (
            <div className="space-y-4">
              <form onSubmit={runSearch} className="flex items-center gap-2">
                <div className="relative flex-1 min-w-0">
                  <Input
                    value={query}
                    onChange={(e) => {
                      setQuery(e.target.value);
                      if (e.target.value === "") setSearchHits(null);
                    }}
                    placeholder="Search across active vaults..."
                  />
                </div>
                <Button type="submit" size="icon" aria-label="Search">
                  <MagnifyingGlass size={18} />
                </Button>
              </form>

              <div className="grid items-start gap-4 lg:grid-cols-12">
                <Card className="lg:col-span-5 divide-y divide-border/70 overflow-hidden max-h-[68vh] overflow-y-auto">
                  {visibleNotes.length === 0 ? (
                    <div className="p-6 text-sm text-muted-foreground">No notes in scope</div>
                  ) : (
                    visibleNotes.map((n) => {
                      const isSel = selectedNote?.vault === n.vault && selectedNote?.path === n.path;
                      return (
                        <Button
                          key={`${n.vault}:${n.path}`}
                          variant="ghost"
                          onClick={() => void openNote(n.vault, n.path)}
                          className={cn(
                            "h-auto w-full flex-col items-start rounded-none px-4 py-3 text-left whitespace-normal",
                            isSel && "bg-muted/70 text-foreground",
                          )}
                        >
                          <div className="flex w-full items-center justify-between gap-2">
                            <span className="truncate font-medium text-foreground">{n.title || n.path}</span>
                            <Badge variant="neutral">{n.vault}</Badge>
                          </div>
                          <span className="mt-0.5 w-full truncate text-xs text-muted-foreground">{n.path}</span>
                        </Button>
                      );
                    })
                  )}
                </Card>

                <Card className="lg:col-span-7 p-5 sm:p-6 min-w-0">
                  {selectedNote ? (
                    <div className="space-y-4 min-w-0">
                      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/70 pb-3">
                        <div className="min-w-0">
                          <h2 className="truncate text-xl font-medium text-foreground">{selectedNote.title}</h2>
                          <div className="truncate text-xs text-muted-foreground">
                            {selectedNote.vault}:{selectedNote.path}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {selectedNote.owner && <Badge variant="warn">owner: {selectedNote.owner}</Badge>}
                          {selectedNote.updated && <Badge variant="neutral" className="tabular-nums">{selectedNote.updated}</Badge>}
                        </div>
                      </div>
                      <div className="max-h-[56vh] overflow-auto whitespace-pre-wrap break-words font-sans text-sm leading-relaxed text-foreground">
                        {selectedNote.raw}
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">Select a note</div>
                  )}
                </Card>
              </div>
            </div>
          )}

          {section === "proposals" && (
            <div className="grid items-start gap-4 lg:grid-cols-12">
              <Card className="lg:col-span-5 divide-y divide-border/70 overflow-hidden">
                {proposals.length === 0 ? (
                  <div className="p-6 text-sm text-muted-foreground">No open proposals</div>
                ) : (
                  proposals.map((p) => (
                    <Button
                      key={p.id}
                      variant="ghost"
                      onClick={() => setSelectedPropId(p.id)}
                      className={cn(
                        "h-auto w-full flex-col items-start rounded-none px-4 py-3.5 text-left whitespace-normal",
                        activeProposal?.id === p.id && "bg-muted/70 text-foreground",
                      )}
                    >
                      <div className="flex w-full items-center justify-between gap-2">
                        <span className="truncate font-medium text-foreground">{p.path}</span>
                        <Badge variant="default">{p.vault}</Badge>
                      </div>
                      <span className="mt-1 truncate w-full text-xs text-muted-foreground">
                        {p.kind} · {p.reason || p.author}
                      </span>
                    </Button>
                  ))
                )}
              </Card>

              <Card className="lg:col-span-7 p-5 sm:p-6 min-w-0">
                {activeProposal ? (
                  <div className="space-y-4">
                    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/70 pb-3">
                      <div className="min-w-0">
                        <h2 className="truncate text-lg font-medium text-foreground">{activeProposal.path}</h2>
                        <div className="text-xs text-muted-foreground">
                          {activeProposal.id} · {activeProposal.author} · {activeProposal.created_at}
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button size="sm" disabled={busy} onClick={() => void handleProposal(activeProposal, "approve")}>
                          Approve
                        </Button>
                        <Button size="sm" variant="destructive" disabled={busy} onClick={() => void handleProposal(activeProposal, "reject")}>
                          Reject
                        </Button>
                      </div>
                    </div>

                    <div className="overflow-x-auto rounded-2xl border border-border/70 bg-white/80 p-4 text-sm leading-relaxed">
                      {activeProposal.diff.split("\n").map((line, i) => (
                        <div
                          key={i}
                          className={cn(
                            "whitespace-pre-wrap break-words px-2 py-0.5 rounded",
                            line.startsWith("+ ") && "bg-emerald-500/10 text-emerald-900",
                            line.startsWith("- ") && "bg-red-500/10 text-red-900",
                            line.startsWith("@@") && "text-primary font-medium",
                          )}
                        >
                          {line || " "}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">No proposal selected</div>
                )}
              </Card>
            </div>
          )}

          {section === "vaults" && (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {vaults.map((v) => (
                <Card key={v.name} className="p-6 space-y-3">
                  <div className="flex items-center justify-between gap-2">
                    <h2 className="text-xl font-medium text-foreground">{v.name}</h2>
                    <Badge variant={v.kind === "org" ? "default" : "neutral"}>{v.kind}</Badge>
                  </div>
                  <div className="text-3xl font-light tabular-nums text-foreground">
                    {v.open_proposals}
                  </div>
                  <div className="text-sm text-muted-foreground">
                    Open proposals · write: {v.default_write}
                  </div>
                </Card>
              ))}
            </div>
          )}

          {section === "audit" && (
            <Card className="overflow-x-auto">
              <table className="w-full min-w-[560px] border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40">
                    <th className="px-5 py-3.5 font-semibold text-foreground">Time</th>
                    <th className="px-5 py-3.5 font-semibold text-foreground">Actor</th>
                    <th className="px-5 py-3.5 font-semibold text-foreground">Action</th>
                    <th className="px-5 py-3.5 font-semibold text-foreground">Target</th>
                    <th className="px-5 py-3.5 font-semibold text-foreground">Outcome</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/70">
                  {auditEvents.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-5 py-6 text-muted-foreground">
                        No audit events yet
                      </td>
                    </tr>
                  ) : (
                    auditEvents.map((ev, idx) => (
                      <tr key={idx}>
                        <td className="px-5 py-3 tabular-nums text-muted-foreground">{ev.timestamp.replace("T", " ").slice(0, 19)}</td>
                        <td className="px-5 py-3 font-medium text-foreground">{ev.actor}</td>
                        <td className="px-5 py-3 text-foreground">{ev.action}</td>
                        <td className="px-5 py-3 text-muted-foreground">{ev.vault ? `${ev.vault}${ev.path ? ":" + ev.path : ""}` : ev.detail || "-"}</td>
                        <td className="px-5 py-3">
                          <Badge variant={ev.outcome === "ok" ? "default" : "warn"}>{ev.outcome}</Badge>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </Card>
          )}
        </div>
      </main>

      {/* Mobile floating pill navigation (icons only) */}
      <nav
        aria-label="Mobile navigation"
        className="fixed bottom-4 left-1/2 z-40 flex -translate-x-1/2 items-center gap-1 rounded-full border border-white/85 bg-white/80 p-1.5 shadow-[0_12px_36px_rgba(15,58,55,0.14)] ring-1 ring-border backdrop-blur-xl md:hidden"
      >
        {navItems.map((item) => {
          const Icon = item.icon;
          const active = section === item.id;
          return (
            <Button
              key={item.id}
              variant="ghost"
              size="icon"
              aria-label={item.label}
              onClick={() => setSection(item.id)}
              className={cn("h-11 w-11 rounded-full", active && "bg-muted text-primary")}
            >
              <Icon size={20} weight={active ? "fill" : "regular"} />
            </Button>
          );
        })}
      </nav>
    </div>
  );
}
