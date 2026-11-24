import * as React from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/dialog";
import { StudioShowcase } from "@/components/StudioShowcase";

// The interactive Studio preview, opened on demand instead of sitting in the scroll.
export function StudioDialog() {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="lg">
          Preview Studio UI
        </Button>
      </DialogTrigger>
      <DialogContent title="Studio UI" size="xl">
        <StudioShowcase flat />
      </DialogContent>
    </Dialog>
  );
}
