import * as React from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type WorkspaceCase = {
  id: string;
  label: string;
  cwd: string;
  remote: string;
  vaults: Array<{
    name: string;
    access: string;
    write: string;
    state: "active" | "isolated";
  }>;
  results: Array<{
    vault: string;
    path: string;
    snippet: string;
  }>;
};

const workspaces: WorkspaceCase[] = [
  {
    id: "acme",
    label: "~/work/acme",
    cwd: "~/work/acme/payments-api",
    remote: "github.com/acme/payments-api",
    vaults: [
      { name: "acme", access: "full", write: "propose", state: "active" },
      { name: "personal", access: "persona/**", write: "direct", state: "active" },
      { name: "beta", access: "blocked", write: "none", state: "isolated" },
    ],
    results: [
      {
        vault: "acme",
        path: "infra/deploy.md",
        snippet: "Production runs on Fly.io (fra). Rollback via fly releases.",
      },
      {
        vault: "personal",
        path: "persona/style.md",
        snippet: "Keep commit messages short. Run tests before proposing edits.",
      },
    ],
  },
  {
    id: "beta",
    label: "~/work/beta",
    cwd: "~/work/beta/storefront",
    remote: "gitlab.com/beta/storefront",
    vaults: [
      { name: "beta", access: "full", write: "propose", state: "active" },
      { name: "personal", access: "persona/**", write: "direct", state: "active" },
      { name: "acme", access: "blocked", write: "none", state: "isolated" },
    ],
    results: [
      {
        vault: "beta",
        path: "projects/shipping.md",
        snippet: "Free shipping threshold updated to 115 EUR on 2026-11-23.",
      },
      {
        vault: "personal",
        path: "persona/style.md",
        snippet: "Keep commit messages short. Run tests before proposing edits.",
      },
    ],
  },
  {
    id: "personal",
    label: "~/notes",
    cwd: "~/notes",
    remote: "github.com/you/notes",
    vaults: [
      { name: "personal", access: "full", write: "direct", state: "active" },
      { name: "acme", access: "blocked", write: "none", state: "isolated" },
      { name: "beta", access: "blocked", write: "none", state: "isolated" },
    ],
    results: [
      {
        vault: "personal",
        path: "projects/memtwig.md",
        snippet: "Single Go binary, SQLite FTS5 + RRF, v1.0.0 released.",
      },
      {
        vault: "personal",
        path: "decisions/2026-11.md",
        snippet: "2026-11-23 memtwig: CGO_ENABLED=0 pure Go build for zero runtime deps.",
      },
    ],
  },
];

export function ScopeDemo() {
  return (
    <div className="rounded-3xl border border-white/80 bg-white/60 p-5 shadow-[0_14px_45px_rgba(15,58,55,0.07)] ring-1 ring-border backdrop-blur-xl sm:p-7">
      <Tabs defaultValue="acme">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-sm font-medium text-muted-foreground">Active workspace</span>
          <TabsList className="grid w-full grid-cols-3 sm:inline-flex sm:w-auto">
            {workspaces.map((ws) => (
              <TabsTrigger key={ws.id} value={ws.id} className="px-2 text-xs sm:px-3.5 sm:text-sm">
                {ws.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>

        {workspaces.map((ws) => (
          <TabsContent key={ws.id} value={ws.id} className="mt-5 space-y-5">
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-muted-foreground">
              <span className="truncate">cwd: {ws.cwd}</span>
              <span className="truncate">remote: {ws.remote}</span>
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              {ws.vaults.map((v) => (
                <div
                  key={v.name}
                  className="rounded-2xl border border-white/80 bg-white/75 px-4 py-3.5 shadow-[0_2px_12px_rgba(15,58,55,0.04)] ring-1 ring-border/70 backdrop-blur-md"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-foreground">{v.name}</span>
                    <span
                      className={
                        v.state === "active"
                          ? "h-2 w-2 rounded-full bg-primary"
                          : "h-2 w-2 rounded-full bg-border"
                      }
                      aria-label={v.state}
                    />
                  </div>
                  <div className="mt-1.5 text-sm text-muted-foreground">
                    {v.state === "active" ? `${v.access} · ${v.write}` : "isolated"}
                  </div>
                </div>
              ))}
            </div>

            <div className="rounded-2xl border border-white/80 bg-white/75 p-4 shadow-[0_2px_12px_rgba(15,58,55,0.04)] ring-1 ring-border/70 backdrop-blur-md sm:p-5">
              <div className="text-sm font-medium text-muted-foreground">
                $ memtwig search "deploy"
              </div>
              <div className="mt-3 divide-y divide-border/80">
                {ws.results.map((r) => (
                  <div key={`${r.vault}:${r.path}`} className="py-2.5 first:pt-0 last:pb-0">
                    <div className="flex flex-wrap items-baseline gap-2 text-sm">
                      <span className="font-medium text-primary">{r.vault}</span>
                      <span className="font-medium text-foreground break-all">{r.path}</span>
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">{r.snippet}</p>
                  </div>
                ))}
              </div>
            </div>
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
