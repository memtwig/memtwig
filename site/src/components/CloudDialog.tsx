import * as React from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/components/ui/dialog";
import { CloudDeck } from "@/components/CloudDeck";

// The four Cloud building blocks, opened on demand from the Cloud band.
export function CloudDialog() {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="lg">
          See Cloud architecture
        </Button>
      </DialogTrigger>
      <DialogContent title="Memtwig Cloud" size="lg">
        <CloudDeck showHeader={false} />
      </DialogContent>
    </Dialog>
  );
}
