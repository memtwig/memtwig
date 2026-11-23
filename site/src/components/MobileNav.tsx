import * as React from "react";
import { AnimatePresence, MotionConfig, motion } from "motion/react";
import { List, X } from "@phosphor-icons/react";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  MobileDropdown,
  MobileDropdownContent,
  MobileDropdownDescription,
  MobileDropdownTitle,
  MobileDropdownTrigger,
} from "@/components/ui/mobile-dropdown";
import { cn } from "@/lib/utils";

const repoUrl = "https://github.com/memtwig/memtwig";
const docsUrl = "https://github.com/memtwig/memtwig/tree/main/docs";

export function MobileNav({ currentPath = "/" }: { currentPath?: string }) {
  const [open, setOpen] = React.useState(false);

  const navLinks = [
    { href: currentPath === "/" ? "#studio" : "/#studio", label: "Studio", match: "/#studio" },
    { href: currentPath === "/" ? "#features" : "/#features", label: "How it works", match: "/#features" },
    { href: "/integrations", label: "Integrations", match: "/integrations" },
    { href: "/cloud", label: "Cloud", match: "/cloud" },
    { href: "/vs/mem0", label: "Compare", match: "/vs" },
    { href: docsUrl, label: "Docs", external: true, match: "docs" },
  ];

  const isActive = (match: string) => {
    if (match === "/integrations") return currentPath.startsWith("/integrations");
    if (match === "/cloud") return currentPath === "/cloud";
    if (match === "/vs") return currentPath.startsWith("/vs") || currentPath === "/mcp-memory";
    return false;
  };

  return (
    <MotionConfig reducedMotion="user">
      <MobileDropdown open={open} onOpenChange={setOpen}>
        <MobileDropdownTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label={open ? "Close navigation" : "Open navigation"}
            className="bg-[#1d1d20]/90 backdrop-blur-md"
          >
            <AnimatePresence mode="wait">
              {open ? (
                <motion.span
                  key="icon-close"
                  initial={{ rotate: -45, opacity: 0 }}
                  animate={{ rotate: 0, opacity: 1 }}
                  exit={{ rotate: 45, opacity: 0 }}
                  transition={{ duration: 0.14 }}
                  className="inline-flex"
                >
                  <X size={20} />
                </motion.span>
              ) : (
                <motion.span
                  key="icon-list"
                  initial={{ rotate: 45, opacity: 0 }}
                  animate={{ rotate: 0, opacity: 1 }}
                  exit={{ rotate: -45, opacity: 0 }}
                  transition={{ duration: 0.14 }}
                  className="inline-flex"
                >
                  <List size={20} />
                </motion.span>
              )}
            </AnimatePresence>
          </Button>
        </MobileDropdownTrigger>

        <MobileDropdownContent>
          <MobileDropdownTitle className="sr-only">Navigation</MobileDropdownTitle>
          <MobileDropdownDescription className="sr-only">
            Primary links and install actions
          </MobileDropdownDescription>

          <motion.div
            initial={{ opacity: 0, y: -10, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
            className="rounded-[24px] border border-white/12 bg-[#1a1a1d]/95 p-3 shadow-[0_24px_70px_rgba(0,0,0,0.75)] backdrop-blur-2xl"
          >
            <nav aria-label="Mobile navigation" className="grid grid-cols-2 gap-1">
              {navLinks.map((item) => (
                <a
                  key={item.label}
                  href={item.href}
                  target={item.external ? "_blank" : undefined}
                  rel={item.external ? "noreferrer" : undefined}
                  onClick={() => setOpen(false)}
                  className={cn(
                    "rounded-2xl px-3.5 py-2.5 text-sm font-medium transition-colors",
                    isActive(item.match)
                      ? "bg-white/10 text-white"
                      : "text-zinc-300 hover:bg-white/[0.07] hover:text-white",
                  )}
                >
                  {item.label}
                </a>
              ))}
            </nav>

            <div className="mt-2 grid grid-cols-2 gap-2 border-t border-white/[0.08] pt-3">
              <a
                href={repoUrl}
                target="_blank"
                rel="noreferrer"
                onClick={() => setOpen(false)}
                className={cn(buttonVariants({ variant: "outline", size: "default" }), "w-full")}
              >
                GitHub
              </a>
              <a
                href={currentPath === "/" ? "#install" : "/#install"}
                onClick={() => setOpen(false)}
                className={cn(buttonVariants({ variant: "default", size: "default" }), "w-full")}
              >
                Install
              </a>
            </div>
          </motion.div>
        </MobileDropdownContent>
      </MobileDropdown>
    </MotionConfig>
  );
}
