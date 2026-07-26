import { CommandID } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

/**
 * Load Selection dialog. Owns the selected channel-name draft locally and seeds
 * it with the first saved channel on each open (matching the former menu-driven
 * seeding). Restores the chosen alpha channel as the current selection.
 */
export function LoadSelectionDialog() {
  const engine = useEngine();
  const { loadSelectionOpen, setLoadSelectionOpen } = useDialogState();
  const [loadSelectionName, setLoadSelectionName] = useState("");

  const savedSelectionChannels = useUiMeta((meta) => meta?.savedSelectionChannels) ?? [];

  const wasOpenRef = useRef(false);
  // Seed the selected channel from the engine's saved channels only on the
  // closed -> open transition; it must not re-run as channels change while
  // the dialog is open.
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally seeds only on open transition
  useEffect(() => {
    if (!loadSelectionOpen) {
      wasOpenRef.current = false;
      return;
    }
    if (wasOpenRef.current) {
      return;
    }
    wasOpenRef.current = true;
    setLoadSelectionName(savedSelectionChannels[0]?.name ?? "");
  }, [loadSelectionOpen]);

  const commitLoadSelection = () => {
    if (!loadSelectionName) {
      return;
    }
    engine.dispatchCommand(CommandID.LoadSelectionFromChannel, {
      name: loadSelectionName,
      mode: "replace",
    });
    setLoadSelectionOpen(false);
  };

  return (
    <Dialog
      open={loadSelectionOpen}
      onClose={() => setLoadSelectionOpen(false)}
      title="Load Selection"
      description="Restore a saved alpha channel as the current selection."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <Field label="Saved Channel">
          <select
            className={fieldClassName}
            value={loadSelectionName}
            onChange={(e) => setLoadSelectionName(e.target.value)}
          >
            {savedSelectionChannels.map((channel) => (
              <option key={channel.name} value={channel.name}>
                {channel.name}
              </option>
            ))}
          </select>
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" size="sm" onClick={() => setLoadSelectionOpen(false)}>
            Cancel
          </Button>
          <Button size="sm" disabled={!loadSelectionName} onClick={commitLoadSelection}>
            Load
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
