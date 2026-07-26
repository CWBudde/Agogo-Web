import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";
import { useDialogState } from "@/state/dialog-state";
import { useCreateAdjustmentLayer } from "./use-create-adjustment-layer";

/**
 * Threshold adjustment dialog. Owns the threshold-value draft and creates a
 * threshold adjustment layer on apply.
 */
export function ThresholdDialog() {
  const { thresholdDialogOpen, setThresholdDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const [thresholdValue, setThresholdValue] = useState(128);

  return (
    <Dialog
      open={thresholdDialogOpen}
      onClose={() => setThresholdDialogOpen(false)}
      title="Threshold"
      description="Threshold uses Rec. 601 luminance: pixels at or above the slider become white, below become black."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
          <div className="mt-2 h-5 overflow-hidden rounded border border-white/10 bg-slate-950">
            <div className="flex h-full w-full">
              <div
                className="h-full bg-black"
                style={{ width: `${(thresholdValue / 255) * 100}%` }}
              />
              <div className="h-full flex-1 bg-white" />
            </div>
          </div>
          <div
            className="mt-2 h-1 rounded-full bg-gradient-to-r from-black via-slate-500 to-white"
            style={{
              backgroundImage:
                "linear-gradient(90deg, rgba(0,0,0,1) 0%, rgba(0,0,0,1) 45%, rgba(255,255,255,1) 55%, rgba(255,255,255,1) 100%)",
            }}
          />
        </div>
        <label className="block">
          <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Threshold</span>
            <span className="text-slate-300">{thresholdValue}</span>
          </div>
          <input
            className="h-2 w-full accent-accent focus-visible:outline-none"
            type="range"
            min={0}
            max={255}
            step={1}
            value={thresholdValue}
            onChange={(event) =>
              setThresholdValue(parseNumericInput(event.target.value, thresholdValue))
            }
          />
        </label>
        <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
          <span>Threshold Value</span>
          <input
            className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
            type="number"
            min={0}
            max={255}
            step={1}
            value={thresholdValue}
            onChange={(event) =>
              setThresholdValue(Math.max(0, Math.min(255, Number(event.target.value) || 0)))
            }
          />
        </label>
        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={() => setThresholdDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              createAdjustmentLayer("Threshold", "threshold", { threshold: thresholdValue });
              setThresholdDialogOpen(false);
            }}
          >
            Create Adjustment Layer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

/**
 * Posterize adjustment dialog. Owns the levels draft and creates a posterize
 * adjustment layer on apply.
 */
export function PosterizeDialog() {
  const { posterizeDialogOpen, setPosterizeDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const [posterizeLevels, setPosterizeLevels] = useState(6);

  return (
    <Dialog
      open={posterizeDialogOpen}
      onClose={() => setPosterizeDialogOpen(false)}
      title="Posterize"
      description="Posterize reduces each RGB channel to a fixed number of levels. Alpha is preserved."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
          <div
            className="mt-2 h-5 rounded border border-white/10"
            style={{
              backgroundImage:
                "linear-gradient(90deg, rgb(0,0,0) 0%, rgb(0,0,0) 14%, rgb(85,85,85) 14%, rgb(85,85,85) 28%, rgb(170,170,170) 28%, rgb(170,170,170) 42%, rgb(255,255,255) 42%, rgb(255,255,255) 100%)",
            }}
          />
        </div>
        <label className="block">
          <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Levels</span>
            <span className="text-slate-300">{posterizeLevels}</span>
          </div>
          <input
            className="h-2 w-full accent-accent focus-visible:outline-none"
            type="range"
            min={2}
            max={255}
            step={1}
            value={posterizeLevels}
            onChange={(event) =>
              setPosterizeLevels(parseNumericInput(event.target.value, posterizeLevels))
            }
          />
        </label>
        <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
          <span>Levels Value</span>
          <input
            className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
            type="number"
            min={2}
            max={255}
            step={1}
            value={posterizeLevels}
            onChange={(event) =>
              setPosterizeLevels(Math.max(2, Math.min(255, Number(event.target.value) || 2)))
            }
          />
        </label>
        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={() => setPosterizeDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              createAdjustmentLayer("Posterize", "posterize", { levels: posterizeLevels });
              setPosterizeDialogOpen(false);
            }}
          >
            Create Adjustment Layer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
