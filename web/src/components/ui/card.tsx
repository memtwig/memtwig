import * as React from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-3xl border border-white/85 bg-white/70 shadow-[0_10px_32px_rgba(15,58,55,0.05)] ring-1 ring-border backdrop-blur-xl",
        className,
      )}
      {...props}
    />
  );
}
