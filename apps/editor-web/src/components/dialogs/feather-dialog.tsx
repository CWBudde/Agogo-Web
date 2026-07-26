import { CommandID } from "@agogo/proto";
import { useState } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";

/**
 * Feather Selection dialog. Owns its radius draft locally and dispatches the
 * feather command to the engine on commit.
 */
export function FeatherDialog() {
  const engine = useEngine();
  const { featherDialogOpen, setFeatherDialogOpen } = useDialogState();
  const [featherDialogValue, setFeatherDialogValue] = useState(5);

  const commitFeather = () => {
    engine.dispatchCommand(CommandID.FeatherSelection, { radius: featherDialogValue });
    setFeatherDialogOpen(false);
  };

  return (
    <Dialog
      open={featherDialogOpen}
      onClose={() => setFeatherDialogOpen(false)}
      title="Feather Selection"
      description="Softens the selection edges by blurring."
      className="max-w-xs"
    >
      <div className="space-y-3">
        <Field label="Feather Radius (px)">
          <input
            type="number"
            className={fieldClassName}
            min={0}
            max={250}
            step={0.5}
            value={featherDialogValue}
            onChange={(e) =>
              setFeatherDialogValue(parseNumericInput(e.target.value, featherDialogValue))
            }
          />
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" size="sm" onClick={() => setFeatherDialogOpen(false)}>
            Cancel
          </Button>
          <Button size="sm" onClick={commitFeather}>
            OK
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
