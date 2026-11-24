import * as React from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/dialog";

// A discreet trigger that opens static content (an Astro slot) in a dialog.
export function DetailsDialog({
  label,
  title,
  size = "lg",
  variant = "outline",
  children,
}: {
  label: string;
  title: string;
  size?: "md" | "lg" | "xl";
  variant?: "outline" | "ghost";
  children?: React.ReactNode;
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant={variant} size={variant === "ghost" ? "default" : "lg"} className={variant === "ghost" ? "text-zinc-300" : undefined}>
          {label}
        </Button>
      </DialogTrigger>
      <DialogContent title={title} size={size}>
        {children}
      </DialogContent>
    </Dialog>
  );
}
