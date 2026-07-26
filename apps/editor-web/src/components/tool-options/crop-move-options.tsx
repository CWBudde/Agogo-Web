import { CommandID, type UpdateCropCommand } from "@agogo/proto";
import type { EditorTool } from "@/components/tool-rail-model";
import { Button } from "@/components/ui/button";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";
import { ToolChoiceButton, ToolNumberField, ToolOptionGroup } from "./controls";

export function CropMoveOptions({ activeTool }: { activeTool: EditorTool }) {
  const engine = useEngine();
  const activeCrop = useUiMeta((meta) => meta?.crop);
  const {
    moveAutoSelectGroup,
    setMoveAutoSelectGroup,
    cropDeletePixels,
    setCropDeletePixels,
    cropContentAwareFill,
    setCropContentAwareFill,
    cropResolution,
    setCropResolution,
    cropOverlayType,
    setCropOverlayType,
    cropStraightenActive,
    setCropStraightenActive,
  } = useSelectionToolState();

  const dispatchCropUpdate = (overrides: Partial<UpdateCropCommand>) => {
    if (!activeCrop?.active) {
      return;
    }
    engine.dispatchCommand(CommandID.UpdateCrop, {
      x: activeCrop.x,
      y: activeCrop.y,
      w: activeCrop.w,
      h: activeCrop.h,
      rotation: activeCrop.rotation ?? 0,
      deletePixels: cropDeletePixels,
      contentAwareFill: cropContentAwareFill,
      resolution: cropResolution,
      overlayType: cropOverlayType,
      ...overrides,
    } satisfies UpdateCropCommand);
  };

  if (activeTool === "move") {
    return (
      <ToolChoiceButton
        active={moveAutoSelectGroup}
        onClick={() => setMoveAutoSelectGroup((v) => !v)}
      >
        Groups
      </ToolChoiceButton>
    );
  }

  if (activeTool === "crop") {
    return (
      <>
        <ToolNumberField
          label="W"
          min={1}
          max={99999}
          step={1}
          value={Math.round(activeCrop?.w ?? 0)}
          onChange={(v) => {
            dispatchCropUpdate({ w: v });
          }}
        />
        <ToolNumberField
          label="H"
          min={1}
          max={99999}
          step={1}
          value={Math.round(activeCrop?.h ?? 0)}
          onChange={(v) => {
            dispatchCropUpdate({ h: v });
          }}
        />
        <ToolNumberField
          label="DPI"
          min={1}
          max={99999}
          step={1}
          value={Math.round(cropResolution)}
          onChange={(v) => {
            const next = Math.max(1, v);
            setCropResolution(next);
            dispatchCropUpdate({ resolution: next });
          }}
        />
        <ToolChoiceButton
          active={cropStraightenActive}
          onClick={() => setCropStraightenActive((current) => !current)}
        >
          Straighten
        </ToolChoiceButton>
        <ToolOptionGroup label="Overlay">
          <ToolChoiceButton
            active={cropOverlayType === "thirds"}
            onClick={() => {
              setCropOverlayType("thirds");
              dispatchCropUpdate({ overlayType: "thirds" });
            }}
          >
            Thirds
          </ToolChoiceButton>
          <ToolChoiceButton
            active={cropOverlayType === "grid"}
            onClick={() => {
              setCropOverlayType("grid");
              dispatchCropUpdate({ overlayType: "grid" });
            }}
          >
            Grid
          </ToolChoiceButton>
          <ToolChoiceButton
            active={cropOverlayType === "diagonal"}
            onClick={() => {
              setCropOverlayType("diagonal");
              dispatchCropUpdate({ overlayType: "diagonal" });
            }}
          >
            Diagonal
          </ToolChoiceButton>
          <ToolChoiceButton
            active={cropOverlayType === "none"}
            onClick={() => {
              setCropOverlayType("none");
              dispatchCropUpdate({ overlayType: "none" });
            }}
          >
            None
          </ToolChoiceButton>
        </ToolOptionGroup>
        <label className="ml-3 flex items-center gap-1 text-[10px]">
          <input
            type="checkbox"
            checked={cropDeletePixels}
            onChange={(e) => {
              setCropDeletePixels(e.target.checked);
              dispatchCropUpdate({ deletePixels: e.target.checked });
            }}
          />
          Delete
        </label>
        <label className="ml-2 flex items-center gap-1 text-[10px]">
          <input
            type="checkbox"
            checked={cropContentAwareFill}
            onChange={(e) => {
              setCropContentAwareFill(e.target.checked);
              dispatchCropUpdate({ contentAwareFill: e.target.checked });
            }}
          />
          Content-Aware
        </label>
        <Button
          size="sm"
          className="ml-2 h-6 px-3 text-[10px]"
          onClick={() => {
            setCropStraightenActive(false);
            engine.dispatchCommand(CommandID.CommitCrop);
          }}
        >
          Apply
        </Button>
        <Button
          variant="secondary"
          size="sm"
          className="ml-1 h-6 px-3 text-[10px]"
          onClick={() => {
            setCropStraightenActive(false);
            engine.dispatchCommand(CommandID.CancelCrop);
          }}
        >
          Cancel
        </Button>
      </>
    );
  }

  return null;
}
