import * as React from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type StudioView = {
  id: string;
  label: string;
  src: string;
  alt: string;
  caption: string;
};

const views: StudioView[] = [
  {
    id: "overview",
    label: "Overview",
    src: "/studio/overview.png",
    alt: "Memtwig Studio Overview dashboard showing vault hygiene gauge, indexed volume, graph preview, and open proposals queue",
    caption: "memtwig ui — vault hygiene, indexed volume, graph summary, and pending agent proposals",
  },
  {
    id: "graph",
    label: "Knowledge Graph",
    src: "/studio/graph.png",
    alt: "Memtwig Studio Knowledge Graph showing isolated per-vault clusters and note connections",
    caption: "memtwig ui — isolated vault topology and cross-note links with live inspector",
  },
  {
    id: "notes",
    label: "Notes",
    src: "/studio/notes.png",
    alt: "Memtwig Studio Notes 3-pane workspace with search, rendered Markdown reader, raw view, and in-place editor",
    caption: "memtwig ui — 3-pane Markdown reader, raw source, and in-place editor or proposal flow",
  },
  {
    id: "settings",
    label: "Settings",
    src: "/studio/settings.png",
    alt: "Memtwig Studio Settings for configuring embedding providers, LLM curator, vaults, and AI harnesses",
    caption: "memtwig ui — self-service configuration for embeddings, LLM curator, vaults, and harnesses",
  },
];

export function StudioShowcase() {
  return (
    <Tabs defaultValue="overview" className="w-full">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-sm text-muted-foreground">Embedded local Web UI · memtwig ui</span>
        <TabsList className="grid w-full grid-cols-2 gap-1 sm:inline-flex sm:w-auto sm:gap-0">
          {views.map((view) => (
            <TabsTrigger key={view.id} value={view.id} className="px-3 text-xs sm:text-sm">
              {view.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </div>

      {views.map((view) => (
        <TabsContent key={view.id} value={view.id} className="mt-4">
          <div className="overflow-hidden rounded-lg border border-border bg-card shadow-[0_24px_60px_rgba(0,0,0,0.55)]">
            <img
              src={view.src}
              alt={view.alt}
              width={1440}
              height={900}
              loading={view.id === "overview" ? "eager" : "lazy"}
              className="block w-full h-auto"
            />
          </div>
          <p className="mt-3 text-xs text-muted-foreground sm:text-sm">{view.caption}</p>
        </TabsContent>
      ))}
    </Tabs>
  );
}
