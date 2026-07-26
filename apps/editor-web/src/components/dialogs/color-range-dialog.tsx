import { CommandID } from "@agogo/proto";
import { useState } from "react";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { hexToRgba, type Rgba, rgbaToHex, toMutableRgba } from "@/lib/color";
import { parseNumericInput } from "@/lib/utils";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";

/**
 * Color Range dialog. Owns the sample color / fuzziness / sample-merged draft
 * locally and dispatches a color-range selection to the engine on commit.
 */
export function ColorRangeDialog() {
  const engine = useEngine();
  const { colorRangeOpen, setColorRangeOpen } = useDialogState();
  const [colorRangeColor, setColorRangeColor] = useState<Rgba>([128, 128, 128, 255]);
  const [colorRangeFuzziness, setColorRangeFuzziness] = useState(40);
  const [colorRangeSampleMerged, setColorRangeSampleMerged] = useState(false);

  const commitColorRange = () => {
    engine.dispatchCommand(CommandID.SelectColorRange, {
      targetColor: toMutableRgba(colorRangeColor),
      fuzziness: colorRangeFuzziness,
      sampleMerged: colorRangeSampleMerged,
      mode: "replace",
    });
    setColorRangeOpen(false);
  };

  return (
    <Dialog
      open={colorRangeOpen}
      onClose={() => setColorRangeOpen(false)}
      title="Color Range"
      description="Select pixels by color similarity."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <Field label="Sample Color">
          <input
            type="color"
            className="h-8 w-full cursor-pointer rounded border border-white/10 bg-transparent"
            value={rgbaToHex(colorRangeColor)}
            onChange={(e) => {
              const next = hexToRgba(e.target.value);
              if (next) {
                setColorRangeColor(next);
              }
            }}
          />
        </Field>
        <Field label={`Fuzziness: ${colorRangeFuzziness}`}>
          <input
            type="range"
            className="w-full accent-accent"
            min={0}
            max={200}
            step={1}
            value={colorRangeFuzziness}
            onChange={(e) =>
              setColorRangeFuzziness(parseNumericInput(e.target.value, colorRangeFuzziness))
            }
          />
        </Field>
        <label className="flex cursor-pointer select-none items-center gap-2 text-xs text-slate-300">
          <input
            type="checkbox"
            checked={colorRangeSampleMerged}
            onChange={(e) => setColorRangeSampleMerged(e.target.checked)}
          />
          Sample all layers
        </label>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" size="sm" onClick={() => setColorRangeOpen(false)}>
            Cancel
          </Button>
          <Button size="sm" onClick={commitColorRange}>
            OK
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
