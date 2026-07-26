import { CommandID, type InterpolMode } from "@agogo/proto";
import { TransformRefGrid } from "@/components/transform-ref-grid";
import { applyTransformFieldChange, buildWarpGrid, refPointToPivot } from "@/lib/transform-math";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";
import { ToolChoiceButton, ToolNumberField, ToolOptionGroup } from "./controls";

export function TransformOptions() {
  const engine = useEngine();
  const freeTransform = useUiMeta((meta) => meta?.freeTransform);
  const { transformRefPoint, setTransformRefPoint } = useSelectionToolState();

  if (!freeTransform?.active) {
    return (
      <span className="text-[11px] text-slate-400">
        Click a layer to begin free transform · Enter to commit · Esc to cancel
      </span>
    );
  }

  return (
    <>
      <TransformRefGrid
        active={transformRefPoint}
        onChange={(row, col) => {
          const ft = freeTransform;
          if (!ft) return;
          const [px, py] = refPointToPivot(ft.corners, row, col);
          setTransformRefPoint([row, col]);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            a: ft.a,
            b: ft.b,
            c: ft.c,
            d: ft.d,
            tx: ft.tx,
            ty: ft.ty,
            pivotX: px,
            pivotY: py,
            interpolation: ft.interpolation as InterpolMode,
          });
        }}
      />
      <ToolNumberField
        label="X"
        min={-99999}
        max={99999}
        step={1}
        value={Math.round(freeTransform.tx)}
        onChange={(value) => {
          const ft = freeTransform;
          if (!ft) return;
          const updated = applyTransformFieldChange(ft, "x", value);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            ...updated,
            pivotX: ft.pivotX,
            pivotY: ft.pivotY,
            interpolation: ft.interpolation as InterpolMode,
            ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}),
          });
        }}
      />
      <ToolNumberField
        label="Y"
        min={-99999}
        max={99999}
        step={1}
        value={Math.round(freeTransform.ty)}
        onChange={(value) => {
          const ft = freeTransform;
          if (!ft) return;
          const updated = applyTransformFieldChange(ft, "y", value);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            ...updated,
            pivotX: ft.pivotX,
            pivotY: ft.pivotY,
            interpolation: ft.interpolation as InterpolMode,
            ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}),
          });
        }}
      />
      <ToolNumberField
        label="W%"
        min={-99999}
        max={99999}
        step={1}
        value={Math.round(freeTransform.scaleX * 100)}
        onChange={(value) => {
          const ft = freeTransform;
          if (!ft) return;
          const updated = applyTransformFieldChange(ft, "w", value);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            ...updated,
            pivotX: ft.pivotX,
            pivotY: ft.pivotY,
            interpolation: ft.interpolation as InterpolMode,
            ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}),
          });
        }}
      />
      <ToolNumberField
        label="H%"
        min={-99999}
        max={99999}
        step={1}
        value={Math.round(freeTransform.scaleY * 100)}
        onChange={(value) => {
          const ft = freeTransform;
          if (!ft) return;
          const updated = applyTransformFieldChange(ft, "h", value);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            ...updated,
            pivotX: ft.pivotX,
            pivotY: ft.pivotY,
            interpolation: ft.interpolation as InterpolMode,
            ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}),
          });
        }}
      />
      <ToolNumberField
        label="°"
        min={-360}
        max={360}
        step={0.1}
        value={Math.round(freeTransform.rotation * 10) / 10}
        onChange={(value) => {
          const ft = freeTransform;
          if (!ft) return;
          const updated = applyTransformFieldChange(ft, "r", value);
          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            ...updated,
            pivotX: ft.pivotX,
            pivotY: ft.pivotY,
            interpolation: ft.interpolation as InterpolMode,
            ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}),
          });
        }}
      />
      <ToolOptionGroup label="Interp">
        {(["nearest", "bilinear", "bicubic"] as InterpolMode[]).map((mode) => (
          <ToolChoiceButton
            key={mode}
            active={freeTransform?.interpolation === mode}
            onClick={() => {
              const ft = freeTransform;
              if (!ft) return;
              engine.dispatchCommand(CommandID.UpdateFreeTransform, {
                a: ft.a,
                b: ft.b,
                c: ft.c,
                d: ft.d,
                tx: ft.tx,
                ty: ft.ty,
                pivotX: ft.pivotX,
                pivotY: ft.pivotY,
                interpolation: mode,
              });
            }}
          >
            {mode.charAt(0).toUpperCase() + mode.slice(1)}
          </ToolChoiceButton>
        ))}
      </ToolOptionGroup>
      <ToolChoiceButton
        active={!!freeTransform?.warpGrid}
        onClick={() => {
          const ft = freeTransform;
          if (!ft) return;
          if (ft.warpGrid) {
            // Exit warp mode → back to affine.
            engine.dispatchCommand(CommandID.UpdateFreeTransform, {
              a: ft.a,
              b: ft.b,
              c: ft.c,
              d: ft.d,
              tx: ft.tx,
              ty: ft.ty,
              pivotX: ft.pivotX,
              pivotY: ft.pivotY,
              interpolation: ft.interpolation as InterpolMode,
            });
          } else {
            // Enter warp mode: initialize grid from corners.
            engine.dispatchCommand(CommandID.UpdateFreeTransform, {
              a: ft.a,
              b: ft.b,
              c: ft.c,
              d: ft.d,
              tx: ft.tx,
              ty: ft.ty,
              pivotX: ft.pivotX,
              pivotY: ft.pivotY,
              interpolation: ft.interpolation as InterpolMode,
              warpGrid: buildWarpGrid(ft),
            });
          }
        }}
      >
        Warp
      </ToolChoiceButton>
      <button
        type="button"
        className="rounded border border-green-600/50 bg-green-600/20 px-2 py-0.5 text-[11px] text-green-300 hover:bg-green-600/30 focus-visible:outline-none"
        onClick={() => engine.dispatchCommand(CommandID.CommitFreeTransform, {})}
      >
        ✓ Commit
      </button>
      <button
        type="button"
        className="rounded border border-red-600/50 bg-red-600/20 px-2 py-0.5 text-[11px] text-red-300 hover:bg-red-600/30 focus-visible:outline-none"
        onClick={() => engine.dispatchCommand(CommandID.CancelFreeTransform, {})}
      >
        ✗ Cancel
      </button>
    </>
  );
}
