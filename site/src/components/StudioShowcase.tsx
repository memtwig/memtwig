import * as React from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type StudioView = {
  id: string;
  label: string;
  src: string;
  alt: string;
};

const views: StudioView[] = [
  {
    id: "overview",
    label: "Overview",
    src: "/studio/overview.png",
    alt: "Memtwig Studio Overview dashboard showing vault hygiene gauge, indexed volume, graph summary, and open proposals queue",
  },
  {
    id: "graph",
    label: "Knowledge Graph",
    src: "/studio/graph.png",
    alt: "Memtwig Studio Knowledge Graph showing isolated per-vault clusters and note connections",
  },
  {
    id: "notes",
    label: "Notes",
    src: "/studio/notes.png",
    alt: "Memtwig Studio Notes 3-pane workspace with search, rendered Markdown reader, raw view, and in-place editor",
  },
  {
    id: "settings",
    label: "Settings",
    src: "/studio/settings.png",
    alt: "Memtwig Studio Settings for configuring embedding providers, LLM curator, vaults, and AI harnesses",
  },
];

export function StudioShowcase() {
  return (
    <Tabs defaultValue="overview" className="w-full">
      <div className="flex justify-center">
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
        </TabsContent>
      ))}
    </Tabs>
  );
}
