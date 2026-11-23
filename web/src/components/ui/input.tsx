import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        "flex h-8 w-full rounded-md border border-input bg-[#0c0c0e] px-2.5 text-[13px] text-foreground placeholder:text-muted-foreground/70 focus-visible:outline-none focus-visible:border-zinc-400",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";
