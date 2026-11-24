import * as React from "react";
import { AnimatePresence, MotionConfig, motion } from "motion/react";
import { Check, Copy } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type Snippet = {
  id: string;
  label: string;
  code: string;
};

const snippets: Snippet[] = [
  {
    id: "install",
    label: "Install",
    code: `brew install memtwig/tap/memtwig
# or: curl -fsSL https://memtwig.com/install.sh | sh
# or: go install github.com/memtwig/memtwig/cmd/memtwig@latest

memtwig setup --install
memtwig ui`,
  },
  {
    id: "multiorg",
    label: "Multi-Org",
    code: `memtwig migrate personal ~/notes --kind personal --always-include "persona/**"
memtwig vault add acme ~/notes-acme --kind org --write propose \\
  --match-path "~/work/acme/**" \\
  --match-remote "github.com/acme/*"

memtwig status --cwd ~/work/acme/api`,
  },
  {
    id: "harnesses",
    label: "Harnesses",
    code: `# Auto-connect Claude, Cursor, Codex, Antigravity & Hermes
memtwig install all

# Or inspect/run per harness, then verify MCP tools + instructions
memtwig install antigravity --dry-run
memtwig mcp`,
  },
  {
    id: "team",
    label: "Curator & HTTP",
    code: `# Vector candidate pairs for your agent (no extra LLM key) or HTTP RBAC server
memtwig curate --candidates --max-pairs 6 --json
memtwig curate --print-routine

# Or run Streamable HTTP MCP + REST API + Studio UI
memtwig serve --addr 127.0.0.1:8787`,
  },
];

export function QuickstartTabs() {
  const [activeTab, setActiveTab] = React.useState("install");
  const [copiedId, setCopiedId] = React.useState<string | null>(null);

  const handleCopy = async (id: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      window.setTimeout(() => setCopiedId((cur) => (cur === id ? null : cur)), 1800);
    } catch {
      // Clipboard unavailable in insecure context
    }
  };

  return (
    <MotionConfig reducedMotion="user">
      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid w-full grid-cols-2 gap-1 rounded-2xl sm:inline-flex sm:w-auto sm:gap-0 sm:rounded-full">
          {snippets.map((s) => (
            <TabsTrigger
              key={s.id}
              value={s.id}
              className="relative z-10 rounded-xl px-3 text-xs text-zinc-400 transition-colors data-[state=active]:bg-transparent data-[state=active]:text-[#141415] data-[state=active]:shadow-none sm:rounded-full sm:px-4 sm:text-sm"
            >
              {activeTab === s.id && (
                <motion.span
                  layoutId="quickstart-tab-pill"
                  transition={{ type: "spring", stiffness: 420, damping: 32 }}
                  className="absolute inset-0 -z-10 rounded-xl bg-white sm:rounded-full"
                  aria-hidden="true"
                />
              )}
              {s.label}
            </TabsTrigger>
          ))}
        </TabsList>

        {snippets.map((s) => (
          <TabsContent key={s.id} value={s.id} className="mt-5">
            <motion.div
              key={activeTab === s.id ? `code-${s.id}` : s.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2 }}
              className="relative rounded-[24px] border border-white/[0.08] bg-[#1c1c1f] p-6 pr-16 sm:p-7 sm:pr-20"
            >
              <Button
                variant="outline"
                size="icon"
                aria-label={`Copy ${s.label} commands`}
                onClick={() => handleCopy(s.id, s.code)}
                className="absolute top-4 right-4"
              >
                <AnimatePresence mode="wait">
                  {copiedId === s.id ? (
                    <motion.span
                      key="copied"
                      initial={{ scale: 0.7, opacity: 0 }}
                      animate={{ scale: 1, opacity: 1 }}
                      exit={{ scale: 0.7, opacity: 0 }}
                      transition={{ duration: 0.15 }}
                      className="inline-flex"
                    >
                      <Check size={17} className="text-[#58cc7d]" />
                    </motion.span>
                  ) : (
                    <motion.span
                      key="copy"
                      initial={{ scale: 0.7, opacity: 0 }}
                      animate={{ scale: 1, opacity: 1 }}
                      exit={{ scale: 0.7, opacity: 0 }}
                      transition={{ duration: 0.15 }}
                      className="inline-flex"
                    >
                      <Copy size={17} />
                    </motion.span>
                  )}
                </AnimatePresence>
              </Button>
              <pre className="overflow-x-auto text-sm leading-relaxed text-zinc-100 whitespace-pre">
                <code>{s.code}</code>
              </pre>
            </motion.div>
          </TabsContent>
        ))}
      </Tabs>
    </MotionConfig>
  );
}
