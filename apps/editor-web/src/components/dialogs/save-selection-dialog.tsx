import { CommandID } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";

/**
 * Save Selection dialog. Owns the channel-name draft locally and seeds it with
 * the next free "Alpha N" name on each open (matching the former menu-driven
 * seeding). Saves the current selection to an alpha channel on commit.
 */
export function SaveSelectionDialog() {
  const engine = useEngine();
  const { saveSelectionOpen, setSaveSelectionOpen } = useDialogState();
  const [saveSelectionName, setSaveSelectionName] = useState("Alpha 1");

  const savedSelectionChannels = engine.render?.uiMeta.savedSelectionChannels ?? [];
  const nextSavedSelectionName = () => {
    const existing = new Set(savedSelectionChannels.map((channel) => channel.name.toLowerCase()));
    let index = 1;
    while (existing.has(`alpha ${index}`)) {
      index += 1;
    }
    return `Alpha ${index}`;
  };

  const wasOpenRef = useRef(false);
  // Seed the channel name from the engine's saved channels only on the
  // closed -> open transition (nextSavedSelectionName is read fresh at that
  // point); it must not re-run as channels change while the dialog is open.
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally seeds only on open transition
  useEffect(() => {
    if (!saveSelectionOpen) {
      wasOpenRef.current = false;
      return;
    }
    if (wasOpenRef.current) {
      return;
    }
    wasOpenRef.current = true;
    setSaveSelectionName(nextSavedSelectionName());
  }, [saveSelectionOpen]);

  const commitSaveSelection = () => {
    engine.dispatchCommand(CommandID.SaveSelectionToChannel, {
      name: saveSelectionName.trim() || nextSavedSelectionName(),
    });
    setSaveSelectionOpen(false);
  };

  return (
    <Dialog
      open={saveSelectionOpen}
      onClose={() => setSaveSelectionOpen(false)}
      title="Save Selection"
      description="Store the current selection as an alpha channel."
      className="max-w-sm"
    >
      <div className="space-y-4">
        <Field label="Channel Name">
          <input
            type="text"
            className={fieldClassName}
            value={saveSelectionName}
            onChange={(e) => setSaveSelectionName(e.target.value)}
          />
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" size="sm" onClick={() => setSaveSelectionOpen(false)}>
            Cancel
          </Button>
          <Button size="sm" onClick={commitSaveSelection}>
            Save
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
