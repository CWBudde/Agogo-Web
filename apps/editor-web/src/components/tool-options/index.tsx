import type { ReactNode } from "react";
import { useToolState } from "@/state/tool-state";
import { ArtboardOptions } from "./artboard-options";
import { BrushOptions } from "./brush-options";
import { CropMoveOptions } from "./crop-move-options";
import { EyedropperOptions } from "./eyedropper-options";
import { FillGradientOptions } from "./fill-gradient-options";
import { SelectionOptions } from "./selection-options";
import { ShapeOptions } from "./shape-options";
import { TransformOptions } from "./transform-options";

export function ToolOptions({ openBrushPresetImport }: { openBrushPresetImport: () => void }) {
  const { activeTool } = useToolState();

  let content: ReactNode = null;
  switch (activeTool) {
    case "move":
    case "crop":
      content = <CropMoveOptions activeTool={activeTool} />;
      break;
    case "marquee":
    case "lasso":
    case "wand":
      content = <SelectionOptions activeTool={activeTool} />;
      break;
    case "transform":
      content = <TransformOptions />;
      break;
    case "brush":
    case "pencil":
    case "mixerBrush":
    case "cloneStamp":
    case "historyBrush":
    case "eraser":
      content = (
        <BrushOptions activeTool={activeTool} openBrushPresetImport={openBrushPresetImport} />
      );
      break;
    case "fill":
    case "gradient":
      content = <FillGradientOptions activeTool={activeTool} />;
      break;
    case "eyedropper":
      content = <EyedropperOptions />;
      break;
    case "shape":
      content = <ShapeOptions />;
      break;
    case "artboard":
      content = <ArtboardOptions />;
      break;
    default:
      content = null;
  }

  if (!content) {
    return null;
  }

  return (
    <div className="editor-chrome flex min-h-[38px] items-center justify-between gap-3 border-b border-border px-2 py-1.5">
      <div className="flex min-w-0 items-center gap-2 overflow-x-auto pb-0.5">{content}</div>
      <div className="hidden shrink-0 text-[11px] text-slate-400 xl:block">
        Shift add, Alt subtract, Shift+Alt intersect
      </div>
    </div>
  );
}
