import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";

// Radix Dialog with the site's surfaces: focus trap, ESC and backdrop close, scroll inside the body only.
const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

const sizes = {
  md: "max-w-2xl",
  lg: "max-w-4xl",
  xl: "max-w-6xl",
} as const;

const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { title: React.ReactNode; size?: keyof typeof sizes }
>(({ className, children, title, size = "lg", ...props }, ref) => (
  <DialogPrimitive.Portal>
    <DialogPrimitive.Overlay className="dialog-overlay fixed inset-0 z-[60] bg-black/65 backdrop-blur-sm" />
    <DialogPrimitive.Content
      ref={ref}
      aria-describedby={undefined}
      className={cn(
        "dialog-content fixed top-1/2 left-1/2 z-[61] flex max-h-[88dvh] w-[calc(100%-1.5rem)] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-[28px] border border-white/[0.1] bg-[#161618] shadow-[0_40px_120px_-30px_rgba(0,0,0,0.9)] outline-none",
        sizes[size],
        className,
      )}
      {...props}
    >
      <div className="flex shrink-0 items-center justify-between gap-4 border-b border-white/[0.06] py-3 pr-3 pl-6 sm:pl-8">
        <DialogPrimitive.Title className="min-w-0 text-lg leading-snug text-balance font-medium tracking-[-0.01em] text-white sm:text-xl">
          {title}
        </DialogPrimitive.Title>
        <DialogPrimitive.Close
          aria-label="Close"
          className="inline-flex size-10 shrink-0 cursor-pointer items-center justify-center rounded-full text-zinc-400 transition-colors hover:bg-white/[0.08] hover:text-white focus-visible:ring-2 focus-visible:ring-[#58cc7d]/60 focus-visible:outline-none"
        >
          <X size={18} aria-hidden="true" />
        </DialogPrimitive.Close>
      </div>
      <div className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain px-6 py-6 sm:px-8 sm:py-8">{children}</div>
    </DialogPrimitive.Content>
  </DialogPrimitive.Portal>
));
DialogContent.displayName = "DialogContent";

export { Dialog, DialogTrigger, DialogClose, DialogContent };
