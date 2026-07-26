import { useState } from "react";
import { CompactRange } from "@/components/compact-range";
import { ToolChoiceButton, ToolOptionGroup } from "@/components/tool-options/controls";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDialogState } from "@/state/dialog-state";
import { useCreateAdjustmentLayer } from "./use-create-adjustment-layer";

const selectiveColorRanges = [
  { key: "reds", label: "Reds" },
  { key: "yellows", label: "Yellows" },
  { key: "greens", label: "Greens" },
  { key: "cyans", label: "Cyans" },
  { key: "blues", label: "Blues" },
  { key: "magentas", label: "Magentas" },
  { key: "whites", label: "Whites" },
  { key: "neutrals", label: "Neutrals" },
  { key: "blacks", label: "Blacks" },
] as const;

const selectiveColorFields = [
  { key: "cyanRed", label: "Cyan / Red" },
  { key: "magentaGreen", label: "Magenta / Green" },
  { key: "yellowBlue", label: "Yellow / Blue" },
  { key: "black", label: "Black" },
] as const;

/**
 * Selective Color adjustment dialog. Owns the per-range CMYK-style adjustments
 * and the relative/absolute mode draft, and creates a selective-color
 * adjustment layer on apply.
 */
export function SelectiveColorDialog() {
  const { selectiveColorDialogOpen, setSelectiveColorDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const [selectiveColorMode, setSelectiveColorMode] = useState<"relative" | "absolute">("relative");
  const [selectiveColorAdjustments, setSelectiveColorAdjustments] = useState({
    reds: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    yellows: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    greens: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    cyans: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blues: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    magentas: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    whites: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    neutrals: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blacks: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
  });

  return (
    <Dialog
      open={selectiveColorDialogOpen}
      onClose={() => setSelectiveColorDialogOpen(false)}
      title="Selective Color"
      description="Adjust CMYK-style components inside named color ranges. Relative mode scales the effect by pixel strength; Absolute applies the full offsets."
      className="max-w-6xl"
    >
      <div className="space-y-4">
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={selectiveColorMode === "relative"}
            onClick={() => setSelectiveColorMode("relative")}
          >
            Relative
          </ToolChoiceButton>
          <ToolChoiceButton
            active={selectiveColorMode === "absolute"}
            onClick={() => setSelectiveColorMode("absolute")}
          >
            Absolute
          </ToolChoiceButton>
        </ToolOptionGroup>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {selectiveColorRanges.map((range) => (
            <div
              key={range.key}
              className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
            >
              <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                {range.label}
              </p>
              <div className="mt-3 space-y-3">
                {selectiveColorFields.map((field) => (
                  <CompactRange
                    key={field.key}
                    id={`selective-color-${range.key}-${field.key}`}
                    label={field.label}
                    min={-100}
                    max={100}
                    step={1}
                    value={selectiveColorAdjustments[range.key][field.key]}
                    onChange={(next) =>
                      setSelectiveColorAdjustments((current) => ({
                        ...current,
                        [range.key]: {
                          ...current[range.key],
                          [field.key]: next,
                        },
                      }))
                    }
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={() => setSelectiveColorDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              createAdjustmentLayer("Selective Color", "selective-color", {
                mode: selectiveColorMode,
                ...selectiveColorAdjustments,
              });
              setSelectiveColorDialogOpen(false);
            }}
          >
            Create Adjustment Layer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
