import * as React from "react";
import { List } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet";

const links = [
  { href: "/#studio", label: "Studio" },
  { href: "/#features", label: "How it works" },
  { href: "/#isolation", label: "Isolation" },
  { href: "/integrations", label: "Integrations" },
  { href: "/#install", label: "Install" },
  { href: "https://github.com/memtwig/memtwig", label: "GitHub", external: true },
];

export function MobileNav() {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Open navigation">
          <List size={22} />
        </Button>
      </SheetTrigger>
      <SheetContent>
        <SheetTitle className="sr-only">Navigation</SheetTitle>
        <nav className="flex flex-col gap-1.5">
          {links.map((item) => (
            <SheetClose asChild key={item.href}>
              <a
                href={item.href}
                target={item.external ? "_blank" : undefined}
                rel={item.external ? "noreferrer" : undefined}
                className="rounded-md px-3 py-2.5 text-base font-medium text-foreground hover:bg-accent"
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
