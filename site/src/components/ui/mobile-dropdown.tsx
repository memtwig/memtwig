import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

const MobileDropdown = DialogPrimitive.Root;
const MobileDropdownTrigger = DialogPrimitive.Trigger;
const MobileDropdownClose = DialogPrimitive.Close;
const MobileDropdownPortal = DialogPrimitive.Portal;
const MobileDropdownTitle = DialogPrimitive.Title;
const MobileDropdownDescription = DialogPrimitive.Description;

const MobileDropdownOverlay = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay
    ref={ref}
    className={cn("fixed inset-0 z-40 bg-black/50 backdrop-blur-[2px]", className)}
    {...props}
  />
));
MobileDropdownOverlay.displayName = DialogPrimitive.Overlay.displayName;

const MobileDropdownContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ className, children, ...props }, ref) => (
  <MobileDropdownPortal>
    <MobileDropdownOverlay />
    <DialogPrimitive.Content
      ref={ref}
      className={cn(
        "fixed top-[72px] inset-x-4 z-50 mx-auto max-w-md outline-none",
        className,
      )}
      {...props}
    >
      {children}
    </DialogPrimitive.Content>
  </MobileDropdownPortal>
));
MobileDropdownContent.displayName = DialogPrimitive.Content.displayName;

export {
  MobileDropdown,
  MobileDropdownTrigger,
  MobileDropdownClose,
  MobileDropdownContent,
  MobileDropdownTitle,
  MobileDropdownDescription,
};
