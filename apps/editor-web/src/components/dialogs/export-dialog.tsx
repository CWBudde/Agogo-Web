import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import type { DocumentSaveFormat } from "@/hooks/use-menu-actions";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";

/**
 * Export As dialog. Save/export is App-owned (document I/O), passed in as
 * onSave. The active document name falls back to the App document draft name
 * when the engine has no active document.
 */
export function ExportDialog({
  draftName,
  onSave,
}: {
  draftName: string;
  onSave: (format: DocumentSaveFormat) => void;
}) {
  const engine = useEngine();
  const { exportDialogOpen, setExportDialogOpen } = useDialogState();
  const activeDocumentName = engine.render?.uiMeta.activeDocumentName ?? draftName;

  return (
    <Dialog
      open={exportDialogOpen}
      onClose={() => setExportDialogOpen(false)}
      title="Export As"
      description="Choose a layered archive, PSD, or PSB export."
      className="max-w-lg"
    >
      <div className="space-y-3 text-[13px] text-slate-300">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
          <div className="text-[12px] font-medium text-slate-100">Project Archive (.agp)</div>
          <div className="mt-1 text-[12px] text-slate-400">
            Saves the current document state, layer tree, and history as {activeDocumentName}.agp.
          </div>
        </div>
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
          <div className="text-[12px] font-medium text-slate-100">Photoshop Document (.psd)</div>
          <div className="mt-1 text-[12px] text-slate-400">
            Writes a layered PSD with merged image data and embedded Agogo metadata for lossless
            reopen.
          </div>
        </div>
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
          <div className="text-[12px] font-medium text-slate-100">Large Document Format (.psb)</div>
          <div className="mt-1 text-[12px] text-slate-400">
            Use PSB explicitly for large canvases. PSD exports automatically switch to PSB above
            30,000 px.
          </div>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
        <Button variant="ghost" size="sm" onClick={() => setExportDialogOpen(false)}>
          Cancel
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            onSave("archive");
            setExportDialogOpen(false);
          }}
        >
          Export Archive
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            onSave("psd");
            setExportDialogOpen(false);
          }}
        >
          Export PSD
        </Button>
        <Button
          size="sm"
          onClick={() => {
            onSave("psb");
            setExportDialogOpen(false);
          }}
        >
          Export PSB
        </Button>
      </div>
    </Dialog>
  );
}
