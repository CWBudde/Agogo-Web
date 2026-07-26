import { CommandID, type CreateDocumentCommand } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";

type CanvasSizeAnchor =
  | "top-left"
  | "top-center"
  | "top-right"
  | "middle-left"
  | "center"
  | "middle-right"
  | "bottom-left"
  | "bottom-center"
  | "bottom-right";

interface CanvasSizeDraft {
  width: number;
  height: number;
  anchor: CanvasSizeAnchor;
}

/**
 * Canvas Size dialog. Owns its draft locally and seeds it from the current
 * document dimensions on each open (closed -> open transition), falling back
 * to the App document draft when the engine has no document yet.
 */
export function CanvasSizeDialog({ draft }: { draft: CreateDocumentCommand }) {
  const engine = useEngine();
  const render = engine.render;
  const { canvasSizeOpen, setCanvasSizeOpen } = useDialogState();
  const [canvasSizeDraft, setCanvasSizeDraft] = useState<CanvasSizeDraft>({
    width: 0,
    height: 0,
    anchor: "center",
  });

  const wasOpenRef = useRef(false);
  useEffect(() => {
    if (!canvasSizeOpen) {
      wasOpenRef.current = false;
      return;
    }
    if (wasOpenRef.current) {
      return;
    }
    wasOpenRef.current = true;
    setCanvasSizeDraft({
      width: render?.uiMeta.documentWidth ?? draft.width,
      height: render?.uiMeta.documentHeight ?? draft.height,
      anchor: "center",
    });
  }, [canvasSizeOpen, render, draft.width, draft.height]);

  return (
    <Dialog
      open={canvasSizeOpen}
      onClose={() => setCanvasSizeOpen(false)}
      title="Canvas Size"
      description="Resizing the canvas shifts layers relative to the selected anchor."
      className="max-w-md"
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Width">
          <input
            type="number"
            className={fieldClassName}
            value={canvasSizeDraft.width}
            onChange={(e) =>
              setCanvasSizeDraft((c) => ({
                ...c,
                width: parseNumericInput(e.target.value, c.width),
              }))
            }
          />
        </Field>
        <Field label="Height">
          <input
            type="number"
            className={fieldClassName}
            value={canvasSizeDraft.height}
            onChange={(e) =>
              setCanvasSizeDraft((c) => ({
                ...c,
                height: parseNumericInput(e.target.value, c.height),
              }))
            }
          />
        </Field>
      </div>
      <div className="mt-4">
        <Field label="Anchor">
          <div className="grid grid-cols-3 gap-1 w-24 h-24 mt-1">
            {(
              [
                "top-left",
                "top-center",
                "top-right",
                "middle-left",
                "center",
                "middle-right",
                "bottom-left",
                "bottom-center",
                "bottom-right",
              ] as const
            ).map((a) => (
              <button
                key={a}
                type="button"
                className={[
                  "w-full h-full border transition",
                  canvasSizeDraft.anchor === a
                    ? "border-cyan-400 bg-cyan-400/20"
                    : "border-white/10 bg-black/20 hover:border-white/20",
                ].join(" ")}
                onClick={() => setCanvasSizeDraft((c) => ({ ...c, anchor: a }))}
              />
            ))}
          </div>
        </Field>
      </div>
      <div className="mt-6 flex justify-end gap-2">
        <Button variant="secondary" size="sm" onClick={() => setCanvasSizeOpen(false)}>
          Cancel
        </Button>
        <Button
          size="sm"
          onClick={() => {
            engine.dispatchCommand(CommandID.ResizeCanvas, canvasSizeDraft);
            setCanvasSizeOpen(false);
          }}
        >
          Resize
        </Button>
      </div>
    </Dialog>
  );
}
