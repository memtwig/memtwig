import * as React from "react";
import { MotionConfig, motion } from "motion/react";
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
    alt: "Memtwig Studio Notes workspace with instant search, rendered Markdown reader, raw view, and editor",
  },
  {
    id: "settings",
    label: "Settings",
    src: "/studio/settings.png",
    alt: "Memtwig Studio Settings for configuring embedding providers, LLM curator, vaults, and AI harnesses",
  },
];

export function StudioShowcase() {
  const [active, setActive] = React.useState("overview");

  return (
    <MotionConfig reducedMotion="user">
      <div className="overflow-hidden rounded-[24px] border border-white/12 bg-[#1a1a1d] shadow-[0_32px_100px_rgba(0,0,0,0.72)]">
        <Tabs value={active} onValueChange={setActive} className="w-full">
          <div className="flex flex-col gap-3 border-b border-white/[0.08] bg-[#1f1f23] px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5">
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1.5" aria-hidden="true">
                <span className="size-3 rounded-full bg-[#ff5f57]" />
                <span className="size-3 rounded-full bg-[#febc2e]" />
                <span className="size-3 rounded-full bg-[#28c840]" />
              </div>
              <span className="hidden text-xs font-medium text-zinc-400 sm:inline tabular-nums">
                memtwig ui — localhost:4197
              </span>
            </div>

            <TabsList className="grid w-full grid-cols-2 gap-1 rounded-2xl sm:inline-flex sm:w-auto sm:gap-0 sm:rounded-full">
              {views.map((view) => (
                <TabsTrigger key={view.id} value={view.id} className="rounded-xl px-3 text-xs sm:rounded-full sm:text-sm">
                  {view.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          {views.map((view) => (
            <TabsContent key={view.id} value={view.id} className="mt-0">
              <motion.div
                key={active === view.id ? `active-${view.id}` : view.id}
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.22 }}
                className="bg-[#0e0e11]"
              >
                <img
                  src={view.src}
                  alt={view.alt}
                  width={1440}
                  height={900}
                  loading={view.id === "overview" ? "eager" : "lazy"}
                  className="block h-auto w-full"
                />
              </motion.div>
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </MotionConfig>
  );
}
