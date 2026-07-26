import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { hexToRgba, rgbaToCss, rgbaToHex, toMutableRgba } from "@/lib/color";
import { parseNumericInput } from "@/lib/utils";
import { useDialogState } from "@/state/dialog-state";
import { useCreateAdjustmentLayer } from "./use-create-adjustment-layer";

/**
 * Photo Filter adjustment dialog. Owns the filter color / density /
 * preserve-luminosity draft and creates a photo-filter adjustment layer on
 * apply.
 */
export function PhotoFilterDialog() {
  const { photoFilterDialogOpen, setPhotoFilterDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const [photoFilterColor, setPhotoFilterColor] = useState<[number, number, number, number]>([
    255, 190, 120, 255,
  ]);
  const [photoFilterDensity, setPhotoFilterDensity] = useState(40);
  const [photoFilterPreserveLuminosity, setPhotoFilterPreserveLuminosity] = useState(true);

  return (
    <Dialog
      open={photoFilterDialogOpen}
      onClose={() => setPhotoFilterDialogOpen(false)}
      title="Photo Filter"
      description="Simulate a gel filter by blending the image toward a tinted filter color. Preserve luminosity keeps the original brightness."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Filter Color</p>
          <div className="mt-2 flex items-center gap-3">
            <input
              type="color"
              className="h-10 w-14 cursor-pointer rounded border border-white/10 bg-transparent"
              value={rgbaToHex(photoFilterColor)}
              onChange={(event) => {
                const next = hexToRgba(event.target.value);
                if (next) {
                  setPhotoFilterColor(toMutableRgba(next));
                }
              }}
            />
            <div className="min-w-0 flex-1">
              <div
                className="h-10 rounded border border-white/10"
                style={{ backgroundColor: rgbaToCss(photoFilterColor) }}
              />
              <div className="mt-1 text-[11px] text-slate-500">
                {rgbaToHex(photoFilterColor).toUpperCase()}
              </div>
            </div>
          </div>
        </div>
        <label className="block">
          <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Density</span>
            <span className="text-slate-300">{photoFilterDensity}</span>
          </div>
          <input
            className="h-2 w-full accent-accent focus-visible:outline-none"
            type="range"
            min={0}
            max={100}
            step={1}
            value={photoFilterDensity}
            onChange={(event) =>
              setPhotoFilterDensity(parseNumericInput(event.target.value, photoFilterDensity))
            }
          />
        </label>
        <label className="flex items-center gap-2 text-[11px] text-slate-300">
          <input
            type="checkbox"
            checked={photoFilterPreserveLuminosity}
            onChange={(event) => setPhotoFilterPreserveLuminosity(event.target.checked)}
          />
          Preserve luminosity
        </label>
        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={() => setPhotoFilterDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              createAdjustmentLayer("Photo Filter", "photo-filter", {
                color: photoFilterColor,
                density: photoFilterDensity,
                preserveLuminosity: photoFilterPreserveLuminosity,
              });
              setPhotoFilterDialogOpen(false);
            }}
          >
            Create Adjustment Layer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
