import * as React from "react";
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
    code: `curl -fsSL https://memtwig.com/install.sh | sh
# or: go install github.com/memtwig/memtwig/cmd/memtwig@latest

memtwig setup
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
    code: `memtwig install claude --dry-run
memtwig install cursor --dry-run
memtwig install codex --dry-run

# Or run the stdio MCP server directly
memtwig mcp`,
  },
  {
    id: "team",
    label: "Team HTTP",
    code: `# Streamable HTTP MCP + REST API with RBAC tokens and audit.jsonl
memtwig serve --addr 127.0.0.1:8787

# Review agent proposals from any terminal
memtwig proposal list
memtwig proposal show <id>
memtwig proposal ok <id>`,
  },
];

export function QuickstartTabs() {
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
    <Tabs defaultValue="install" className="w-full">
      <TabsList className="grid w-full grid-cols-2 gap-1 rounded-2xl sm:inline-flex sm:w-auto sm:gap-0 sm:rounded-full">
        {snippets.map((s) => (
          <TabsTrigger key={s.id} value={s.id} className="rounded-xl px-3 text-xs sm:rounded-full sm:px-4 sm:text-sm">
            {s.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {snippets.map((s) => (
        <TabsContent key={s.id} value={s.id} className="mt-5">
          <div className="relative rounded-[24px] border border-white/[0.08] bg-[#1c1c1f] p-6 pr-16 sm:p-7 sm:pr-20">
            <Button
              variant="outline"
              size="icon"
              aria-label={`Copy ${s.label} commands`}
              onClick={() => handleCopy(s.id, s.code)}
              className="absolute top-4 right-4"
            >
              {copiedId === s.id ? <Check size={17} className="text-[#58cc7d]" /> : <Copy size={17} />}
            </Button>
            <pre className="overflow-x-auto text-sm leading-relaxed text-zinc-100 whitespace-pre">
              <code>{s.code}</code>
            </pre>
          </div>
        </TabsContent>
      ))}
    </Tabs>
  );
}
