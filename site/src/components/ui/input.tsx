import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        "flex h-11 w-full rounded-xl border border-white/14 bg-[#141415] px-4 text-sm text-foreground placeholder:text-muted-foreground/70 focus-visible:border-[#58cc7d] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#58cc7d]/30 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";
