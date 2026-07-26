import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDialogState } from "@/state/dialog-state";

/**
 * Open Recent dialog. Informational only for now; the primary action defers
 * to the App-owned project picker (document I/O).
 */
export function OpenRecentDialog({ onOpenProject }: { onOpenProject: () => void }) {
  const { openRecentOpen, setOpenRecentOpen } = useDialogState();

  return (
    <Dialog
      open={openRecentOpen}
      onClose={() => setOpenRecentOpen(false)}
      title="Open Recent"
      description="The browser build cannot reopen local files automatically yet, so recent documents are informational only for now."
      className="max-w-lg"
    >
      <div className="space-y-3 text-[13px] text-slate-300">
        <p>
          Recent document tracking needs a persistent file-access layer. That is not wired into the
          web shell yet.
        </p>
        <p className="text-slate-400">
          Use Open to pick an `.agp`, `.psd`, `.psb`, or legacy JSON project from disk.
        </p>
      </div>

      <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
        <Button variant="ghost" size="sm" onClick={() => setOpenRecentOpen(false)}>
          Close
        </Button>
        <Button
          size="sm"
          onClick={() => {
            setOpenRecentOpen(false);
            onOpenProject();
          }}
        >
          Open...
        </Button>
      </div>
    </Dialog>
  );
}
