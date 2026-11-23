import * as React from "react";
import { List } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet";

const links = [
  { href: "/#studio", label: "Studio" },
  { href: "/#features", label: "How it works" },
  { href: "/integrations", label: "Integrations" },
  { href: "/cloud", label: "Cloud" },
  { href: "/vs/mem0", label: "vs Mem0" },
  { href: "/vs/claude-md", label: "vs CLAUDE.md" },
  { href: "/mcp-memory", label: "MCP Memory" },
  { href: "/#install", label: "Install" },
  { href: "https://github.com/memtwig/memtwig/tree/main/docs", label: "Docs", external: true },
  { href: "https://github.com/memtwig/memtwig", label: "GitHub", external: true },
];

export function MobileNav() {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button
          variant="outline"
          size="icon"
          aria-label="Open navigation"
          className="bg-[#1d1d20]/90 backdrop-blur-md"
        >
          <List size={20} />
        </Button>
      </SheetTrigger>
      <SheetContent>
        <SheetTitle className="sr-only">Navigation</SheetTitle>
        <nav className="flex flex-col gap-1">
          {links.map((item) => (
            <SheetClose asChild key={item.href}>
              <a
                href={item.href}
                target={item.external ? "_blank" : undefined}
                rel={item.external ? "noreferrer" : undefined}
                className="rounded-xl px-3.5 py-2.5 text-base font-medium text-zinc-200 transition-colors hover:bg-white/[0.08] hover:text-white"
              >
                {item.label}
              </a>
            </SheetClose>
          ))}
        </nav>
      </SheetContent>
    </Sheet>
  );
}
