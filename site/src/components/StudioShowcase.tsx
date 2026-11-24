import * as React from "react";
import {
  MotionConfig,
  motion,
  useReducedMotion,
  useScroll,
  useSpring,
  useTransform,
} from "motion/react";
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
    label: "Graph",
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
    id: "proposals",
    label: "Proposals",
    src: "/studio/proposals.png",
    alt: "Memtwig Studio Proposals queue with a line-numbered diff, author, reason, and approve or reject actions",
  },
  {
    id: "settings",
    label: "Settings",
    src: "/studio/settings.png",
    alt: "Memtwig Studio Settings with local embeddings, the agent-native curator, and five connected AI harnesses",
  },
];

// flat: no scroll-linked tilt (inside a dialog the page does not scroll under it).
export function StudioShowcase({ flat = false }: { flat?: boolean }) {
  const [active, setActive] = React.useState("overview");
  const [mounted, setMounted] = React.useState(false);
  const stageRef = React.useRef<HTMLDivElement>(null);
  const prefersReduced = useReducedMotion();

  React.useEffect(() => {
    setMounted(true);
  }, []);

  const reduceMotion = mounted && Boolean(prefersReduced);

  const { scrollYProgress } = useScroll({
    target: stageRef,
    offset: ["start end", "center center"],
  });

  const smoothProgress = useSpring(scrollYProgress, {
    stiffness: 160,
    damping: 26,
    mass: 0.5,
  });

  const rotateX = useTransform(smoothProgress, [0, 1], [6, 0]);
  const scale = useTransform(smoothProgress, [0, 1], [0.97, 1]);

  return (
    <MotionConfig reducedMotion="user">
      <div ref={stageRef} style={{ perspective: "1400px" }}>
        <motion.div
          style={reduceMotion || flat ? undefined : { rotateX, scale, transformOrigin: "50% 25%" }}
          className="overflow-hidden rounded-[24px] border border-white/12 bg-[#1a1a1d] shadow-[0_32px_100px_rgba(0,0,0,0.72)]"
        >
          <Tabs value={active} onValueChange={setActive} className="w-full">
            <div className="flex flex-col gap-3 border-b border-white/[0.08] bg-[#1f1f23] px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-1.5" aria-hidden="true">
                  <span className="size-3 rounded-full bg-[#ff5f57]" />
                  <span className="size-3 rounded-full bg-[#febc2e]" />
                  <span className="size-3 rounded-full bg-[#28c840]" />
                </div>
                <span className="hidden text-xs font-medium text-zinc-400 sm:inline tabular-nums">
                  memtwig ui · localhost:4197
                </span>
              </div>

              <TabsList className="grid w-full grid-cols-5 gap-0.5 rounded-2xl sm:inline-flex sm:w-auto sm:gap-0 sm:rounded-full">
                {views.map((view) => (
                  <TabsTrigger
                    key={view.id}
                    value={view.id}
                    className="relative z-10 min-w-0 rounded-xl px-1 text-[11px] text-zinc-400 transition-colors data-[state=active]:bg-transparent data-[state=active]:text-[#141415] data-[state=active]:shadow-none sm:rounded-full sm:px-3 sm:text-sm"
                  >
                    {active === view.id && (
                      <motion.span
                        layoutId="studio-tab-pill"
                        transition={{ type: "spring", stiffness: 420, damping: 32 }}
                        className="absolute inset-0 -z-10 rounded-xl bg-white sm:rounded-full"
                        aria-hidden="true"
                      />
                    )}
                    {view.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>

            {views.map((view) => (
              <TabsContent key={view.id} value={view.id} className="mt-0">
                <motion.div
                  key={active === view.id ? `active-${view.id}` : view.id}
                  initial={active === "overview" && !mounted ? false : { opacity: 0, y: 8, filter: "blur(3px)" }}
                  animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
                  transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
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
        </motion.div>
      </div>
    </MotionConfig>
  );
}
