import { CommandID } from "@agogo/proto";
import type { EditorTool } from "@/components/tool-rail-model";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

/**
 * Cross-domain tool activation: switches the active tool while cancelling or
 * committing any special mode the previous tool left behind (crop session,
 * free transform, vector/text editing) and starting the new tool's mode.
 */
export function useActivateTool() {
  const engine = useEngine();
  const { activeTool, setActiveTool } = useToolState();
  const {
    setCropDeletePixels,
    setCropContentAwareFill,
    setCropResolution,
    setCropOverlayType,
    setCropStraightenActive,
  } = useSelectionToolState();
  const { setIsPanMode } = useViewState();
  const editingVectorLayerID = useUiMeta((meta) => meta?.editingVectorLayerId ?? "");
  const editingTextLayerID = useUiMeta((meta) => meta?.editingTextLayerId ?? "");

  const activateTool = (tool: EditorTool) => {
    if (tool === "fill" && activeTool === "fill") {
      tool = "gradient";
    } else if (tool === "gradient" && activeTool === "gradient") {
      tool = "fill";
    }
    if (tool === activeTool) {
      return;
    }

    // Cancel active special modes when switching away
    if (activeTool === "crop" && tool !== "hand" && tool !== "zoom") {
      engine.dispatchCommand(CommandID.CancelCrop, {});
      setCropDeletePixels(false);
      setCropContentAwareFill(false);
      setCropResolution(72);
      setCropOverlayType("thirds");
      setCropStraightenActive(false);
    }
    if (activeTool === "transform" && tool !== "hand" && tool !== "zoom") {
      engine.dispatchCommand(CommandID.CancelFreeTransform, {});
    }
    if (
      (activeTool === "pen" || activeTool === "directSelect") &&
      tool !== "pen" &&
      tool !== "directSelect" &&
      tool !== "hand" &&
      tool !== "zoom"
    ) {
      engine.dispatchCommand(CommandID.SetActiveTool, { tool: "" });
      if (editingVectorLayerID) {
        engine.dispatchCommand(CommandID.CommitVectorEdit, {});
      }
    }
    if (activeTool === "type" && tool !== "type" && tool !== "hand" && tool !== "zoom") {
      if (editingTextLayerID) {
        engine.dispatchCommand(CommandID.CommitTextEdit, {});
      }
    }

    setActiveTool(tool);
    if (tool !== "hand") {
      setIsPanMode(false);
    }

    // Begin special modes
    if (tool === "crop") {
      setCropStraightenActive(false);
      engine.dispatchCommand(CommandID.BeginCrop, {});
    }
    if (tool === "pen") {
      engine.dispatchCommand(CommandID.SetActiveTool, { tool: "pen" });
    }
    if (tool === "directSelect") {
      engine.dispatchCommand(CommandID.SetActiveTool, {
        tool: "direct-select",
      });
    }
  };

  return activateTool;
}
