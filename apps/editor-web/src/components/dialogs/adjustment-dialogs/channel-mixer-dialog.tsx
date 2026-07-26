import { useState } from "react";
import { CompactRange } from "@/components/compact-range";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDialogState } from "@/state/dialog-state";
import { useCreateAdjustmentLayer } from "./use-create-adjustment-layer";

const channelMixerRows = [
  { key: "red", label: "Red Output" },
  { key: "green", label: "Green Output" },
  { key: "blue", label: "Blue Output" },
] as const;

const channelMixerColumns = [
  { index: 0, label: "Source Red" },
  { index: 1, label: "Source Green" },
  { index: 2, label: "Source Blue" },
] as const;

/**
 * Channel Mixer adjustment dialog. Owns the mix matrix and monochrome draft and
 * creates a channel-mixer adjustment layer on apply.
 */
export function ChannelMixerDialog() {
  const { channelMixerDialogOpen, setChannelMixerDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const [channelMixerMonochrome, setChannelMixerMonochrome] = useState(false);
  const [channelMixerMatrix, setChannelMixerMatrix] = useState<{
    red: [number, number, number];
    green: [number, number, number];
    blue: [number, number, number];
  }>({
    red: [100, 0, 0],
    green: [0, 100, 0],
    blue: [0, 0, 100],
  });

  return (
    <Dialog
      open={channelMixerDialogOpen}
      onClose={() => setChannelMixerDialogOpen(false)}
      title="Channel Mixer"
      description="Mix source RGB into each output channel. Monochrome collapses the mixed result to grayscale."
      className="max-w-4xl"
    >
      <div className="space-y-4">
        <div className="grid gap-3 md:grid-cols-3">
          {channelMixerRows.map((row) => (
            <div
              key={row.key}
              className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
            >
              <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{row.label}</p>
              <div className="mt-3 space-y-3">
                {channelMixerColumns.map((column) => (
                  <CompactRange
                    key={column.index}
                    id={`channel-mixer-${row.key}-${column.index}`}
                    label={column.label}
                    min={-200}
                    max={200}
                    step={1}
                    value={channelMixerMatrix[row.key][column.index]}
                    onChange={(next) =>
                      setChannelMixerMatrix((current) => ({
                        ...current,
                        [row.key]: current[row.key].map((entry, index) =>
                          index === column.index ? next : entry,
                        ),
                      }))
                    }
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
        <label className="flex items-center gap-2 text-[11px] text-slate-300">
          <input
            type="checkbox"
            checked={channelMixerMonochrome}
            onChange={(event) => setChannelMixerMonochrome(event.target.checked)}
          />
          Monochrome output
        </label>
        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={() => setChannelMixerDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              createAdjustmentLayer("Channel Mixer", "channel-mixer", {
                monochrome: channelMixerMonochrome,
                red: channelMixerMatrix.red,
                green: channelMixerMatrix.green,
                blue: channelMixerMatrix.blue,
              });
              setChannelMixerDialogOpen(false);
            }}
          >
            Create Adjustment Layer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
