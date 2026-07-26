import { CommandID, type LayerBlendMode } from "@agogo/proto";
import { useState } from "react";
import type { FilterDialogEngine } from "@/components/filters/filter-dialog";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";

/** Blend modes offered for Fade, mirroring the engine's compositing support. */
const FADE_BLEND_MODES: { value: LayerBlendMode; label: string }[] = [
  { value: "normal", label: "Normal" },
  { value: "dissolve", label: "Dissolve" },
  { value: "darken", label: "Darken" },
  { value: "multiply", label: "Multiply" },
  { value: "color-burn", label: "Color Burn" },
  { value: "lighten", label: "Lighten" },
  { value: "screen", label: "Screen" },
  { value: "color-dodge", label: "Color Dodge" },
  { value: "overlay", label: "Overlay" },
  { value: "soft-light", label: "Soft Light" },
  { value: "hard-light", label: "Hard Light" },
  { value: "difference", label: "Difference" },
  { value: "exclusion", label: "Exclusion" },
  { value: "hue", label: "Hue" },
  { value: "saturation", label: "Saturation" },
  { value: "color", label: "Color" },
  { value: "luminosity", label: "Luminosity" },
];

export interface FadeDialogProps {
  engine: FilterDialogEngine;
  /** Name of the filter being faded, for the dialog title. */
  filterName: string;
  onClose: () => void;
  /** Called after Fade is dispatched (Fade is one-shot). */
  onFaded: () => void;
}

/**
 * "Filter ▸ Fade" — blends the most recently applied filter with the pre-filter
 * pixels at a chosen opacity and blend mode. Available only immediately after a
 * filter (the engine keeps a one-shot pre-filter snapshot).
 */
export function FadeDialog({ engine, filterName, onClose, onFaded }: FadeDialogProps) {
  const [opacity, setOpacity] = useState(100);
  const [blendMode, setBlendMode] = useState<LayerBlendMode>("normal");

  const handleApply = () => {
    engine.dispatchCommand(CommandID.FadeFilter, { opacity, blendMode });
    onFaded();
    onClose();
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={`Fade ${filterName}`}
      description="Blend the last filter result back toward the original pixels. Runs once, right after applying a filter."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <label className="block">
          <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Opacity</span>
            <span className="text-slate-300">{opacity}%</span>
          </div>
          <div className="flex items-center gap-2">
            <input
              className="h-2 flex-1 accent-accent focus-visible:outline-none"
              type="range"
              aria-label="Opacity"
              min={0}
              max={100}
              step={1}
              value={opacity}
              onChange={(event) =>
                setOpacity(
                  Math.max(0, Math.min(100, parseNumericInput(event.target.value, opacity))),
                )
              }
            />
            <input
              className="h-[var(--ui-h-sm)] w-20 rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
              type="number"
              min={0}
              max={100}
              step={1}
              value={opacity}
              onChange={(event) =>
                setOpacity(
                  Math.max(0, Math.min(100, parseNumericInput(event.target.value, opacity))),
                )
              }
            />
          </div>
        </label>

        <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
          <span>Mode</span>
          <select
            className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
            aria-label="Mode"
            value={blendMode}
            onChange={(event) => setBlendMode(event.target.value as LayerBlendMode)}
          >
            {FADE_BLEND_MODES.map((mode) => (
              <option key={mode.value} value={mode.value}>
                {mode.label}
              </option>
            ))}
          </select>
        </label>

        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleApply}>
            Apply
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
