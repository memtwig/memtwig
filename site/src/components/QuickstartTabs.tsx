import * as React from "react";
import { MotionConfig, motion } from "motion/react";
import { CodeBlock } from "@/components/CodeBlock";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type Snippet = {
  id: string;
  label: string;
  title: string;
  code: string;
};

const snippets: Snippet[] = [
  {
    id: "install",
    title: "Install",
    label: "Install",
    code: `brew install memtwig/tap/memtwig
# or: curl -fsSL https://memtwig.com/install.sh | sh
# or: go install github.com/memtwig/memtwig/cmd/memtwig@latest

memtwig setup --install
memtwig ui`,
  },
  {
    id: "multiorg",
    title: "Multi-org vaults",
    label: "Multi-Org",
    code: `memtwig migrate personal ~/notes --kind personal --always-include "persona/**"
memtwig vault add acme ~/notes-acme --kind org --write propose \\
  --match-path "~/work/acme/**" \\
  --match-remote "github.com/acme/*"

memtwig status --cwd ~/work/acme/api`,
  },
  {
    id: "harnesses",
    title: "Connect harnesses",
    label: "Harnesses",
    code: `# Auto-connect Claude, Cursor, Codex, Antigravity & Hermes
memtwig install all

# Or inspect/run per harness, then verify MCP tools + instructions
memtwig install antigravity --dry-run
memtwig mcp`,
  },
  {
    id: "team",
    title: "Curator & HTTP server",
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
            >
              <CodeBlock code={s.code} lang="bash" title={s.title} meta="zsh" />
            </motion.div>
          </TabsContent>
        ))}
      </Tabs>
    </MotionConfig>
  );
}
