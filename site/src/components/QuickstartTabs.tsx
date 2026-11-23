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
      <TabsList className="grid w-full grid-cols-2 gap-1 sm:inline-flex sm:w-auto sm:gap-0">
        {snippets.map((s) => (
          <TabsTrigger key={s.id} value={s.id} className="px-2.5 text-xs sm:px-3.5 sm:text-sm">
            {s.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {snippets.map((s) => (
        <TabsContent key={s.id} value={s.id}>
          <div className="relative rounded-lg border border-border bg-card p-5 pr-14 sm:p-6 sm:pr-16">
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Copy ${s.label} commands`}
              onClick={() => handleCopy(s.id, s.code)}
              className="absolute top-3.5 right-3.5"
            >
              {copiedId === s.id ? <Check size={18} className="text-emerald-400" /> : <Copy size={18} />}
            </Button>
            <pre className="overflow-x-auto text-sm leading-relaxed text-foreground whitespace-pre">
              <code>{s.code}</code>
            </pre>
          </div>
        </TabsContent>
      ))}
    </Tabs>
  );
}
