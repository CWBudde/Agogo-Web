import { CommandID } from "@agogo/proto";
import { useLayoutEffect, useRef, useState } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";
import { useEngine } from "@/wasm/context";

export type ModifyKind = "expand" | "contract" | "smooth" | "border";

/**
 * Modify Selection dialog (expand / contract / smooth / border). The open flag
 * and the operation kind are driven by the menu/shortcut layer (which needs to
 * open the dialog for a specific operation), so they arrive as props. The value
 * draft is owned locally and re-seeded from the kind on each open.
 */
export function ModifyDialog({
  open,
  kind,
  onClose,
}: {
  open: boolean;
  kind: ModifyKind;
  onClose: () => void;
}) {
  const engine = useEngine();
  const [value, setValue] = useState(4);

  // Seed (before paint) on open and whenever the operation kind changes while
  // the dialog is already open — the old openModifyDialog(kind) reset value on
  // every call. Tracking the seeded kind covers a menu re-trigger over the modal.
  const seededKindRef = useRef<ModifyKind | null>(null);
  useLayoutEffect(() => {
    if (!open) {
      seededKindRef.current = null;
      return;
    }
    if (seededKindRef.current === kind) {
      return;
    }
    seededKindRef.current = kind;
    setValue(kind === "smooth" ? 2 : 4);
  }, [open, kind]);

  const commitModify = () => {
    switch (kind) {
      case "expand":
        engine.dispatchCommand(CommandID.ExpandSelection, { pixels: value });
        break;
      case "contract":
        engine.dispatchCommand(CommandID.ContractSelection, { pixels: value });
        break;
      case "smooth":
        engine.dispatchCommand(CommandID.SmoothSelection, { radius: value });
        break;
      case "border":
        engine.dispatchCommand(CommandID.BorderSelection, { width: value });
        break;
    }
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={
        {
          expand: "Expand Selection",
          contract: "Contract Selection",
          smooth: "Smooth Selection",
          border: "Border Selection",
        }[kind]
      }
      description={
        {
          expand: "Grow the selection outward.",
          contract: "Shrink the selection inward.",
          smooth: "Smooth the selection edges.",
          border: "Create a border of the specified width.",
        }[kind]
      }
      className="max-w-xs"
    >
      <div className="space-y-3">
        <Field
          label={
            {
              expand: "Expand By (px)",
              contract: "Contract By (px)",
              smooth: "Radius (px)",
              border: "Width (px)",
            }[kind]
          }
        >
          <input
            type="number"
            className={fieldClassName}
            min={1}
            max={500}
            step={1}
            value={value}
            onChange={(e) => setValue(parseNumericInput(e.target.value, value))}
          />
        </Field>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={commitModify}>
            OK
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
