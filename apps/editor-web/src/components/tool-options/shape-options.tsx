import { parseNumericInput } from "@/lib/utils";
import { useColorState } from "@/state/color-state";
import { type ShapeMode, type ShapeSubTool, useShapeState } from "@/state/shape-state";
import { useViewState } from "@/state/view-state";
import { ToolChoiceButton, ToolOptionGroup } from "./controls";

export function ShapeOptions() {
  const { foregroundColor } = useColorState();
  const { setActiveAuxPanel } = useViewState();
  const {
    shapeSubTool,
    setShapeSubTool,
    shapeMode,
    setShapeMode,
    shapeCornerRadius,
    setShapeCornerRadius,
    shapePolygonSides,
    setShapePolygonSides,
    shapePolygonInnerRadiusPct,
    setShapePolygonInnerRadiusPct,
    shapeStarMode,
    setShapeStarMode,
    shapeFillColor,
    setShapeFillColor,
    shapeStrokeColor,
    setShapeStrokeColor,
    shapeStrokeWidth,
    setShapeStrokeWidth,
    selectedShapePreset,
  } = useShapeState();

  return (
    <>
      <ToolOptionGroup label="Shape">
        {(
          ["rect", "rounded-rect", "ellipse", "polygon", "line", "custom-shape"] as ShapeSubTool[]
        ).map((s) => (
          <ToolChoiceButton
            key={s}
            active={shapeSubTool === s}
            onClick={() => {
              setShapeSubTool(s);
              if (s === "custom-shape") {
                setActiveAuxPanel("shapes");
              }
            }}
          >
            {s === "rect"
              ? "Rect"
              : s === "rounded-rect"
                ? "Round"
                : s === "ellipse"
                  ? "Ellipse"
                  : s === "polygon"
                    ? "Polygon"
                    : s === "line"
                      ? "Line"
                      : "Custom Shape"}
          </ToolChoiceButton>
        ))}
      </ToolOptionGroup>
      <ToolOptionGroup label="Mode">
        {(["shape", "path", "pixels"] as ShapeMode[]).map((m) => (
          <ToolChoiceButton key={m} active={shapeMode === m} onClick={() => setShapeMode(m)}>
            {m.charAt(0).toUpperCase() + m.slice(1)}
          </ToolChoiceButton>
        ))}
      </ToolOptionGroup>
      {shapeMode !== "path" && (
        <div className="flex items-center gap-1">
          <span className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Fill</span>
          <button
            type="button"
            title="Fill color"
            aria-label="Fill color"
            style={{
              background: `rgba(${shapeFillColor[0]},${shapeFillColor[1]},${shapeFillColor[2]},${shapeFillColor[3] / 255})`,
            }}
            className="h-5 w-5 rounded border border-white/20 focus-visible:outline-none"
            onClick={() => {
              setShapeFillColor([...foregroundColor] as [number, number, number, number]);
            }}
          />
          <button
            type="button"
            title="Use foreground color"
            className="rounded border border-white/10 px-1 py-0.5 text-[10px] text-slate-300 hover:bg-white/5"
            onClick={() =>
              setShapeFillColor([...foregroundColor] as [number, number, number, number])
            }
          >
            FG
          </button>
          <button
            type="button"
            title="No fill"
            className="rounded border border-white/10 px-1 py-0.5 text-[10px] text-slate-300 hover:bg-white/5"
            onClick={() => setShapeFillColor([0, 0, 0, 0])}
          >
            None
          </button>
        </div>
      )}
      {shapeMode !== "path" && (
        <div className="flex items-center gap-1">
          <span className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Stroke</span>
          <button
            type="button"
            title="Stroke color"
            aria-label="Stroke color"
            style={{
              background: `rgba(${shapeStrokeColor[0]},${shapeStrokeColor[1]},${shapeStrokeColor[2]},${shapeStrokeColor[3] / 255})`,
            }}
            className="h-5 w-5 rounded border border-white/20 focus-visible:outline-none"
            onClick={() =>
              setShapeStrokeColor([...foregroundColor] as [number, number, number, number])
            }
          />
          <input
            type="number"
            min={0}
            max={100}
            step={1}
            value={shapeStrokeWidth}
            onChange={(e) =>
              setShapeStrokeWidth(Math.max(0, parseNumericInput(e.target.value, shapeStrokeWidth)))
            }
            className="w-12 rounded border border-white/10 bg-transparent px-1 py-0.5 text-[11px] text-slate-200 focus-visible:outline-none"
          />
          <span className="text-[10px] text-slate-500">px</span>
        </div>
      )}
      {shapeSubTool === "rounded-rect" && (
        <div className="flex items-center gap-1">
          <span className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Radius</span>
          <input
            type="number"
            min={0}
            max={500}
            step={1}
            value={shapeCornerRadius}
            onChange={(e) =>
              setShapeCornerRadius(
                Math.max(0, parseNumericInput(e.target.value, shapeCornerRadius)),
              )
            }
            className="w-14 rounded border border-white/10 bg-transparent px-1 py-0.5 text-[11px] text-slate-200 focus-visible:outline-none"
          />
          <span className="text-[10px] text-slate-500">px</span>
        </div>
      )}
      {shapeSubTool === "polygon" && (
        <>
          <div className="flex items-center gap-1">
            <span className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Sides</span>
            <input
              type="number"
              min={3}
              max={100}
              step={1}
              value={shapePolygonSides}
              onChange={(e) =>
                setShapePolygonSides(
                  Math.max(3, parseNumericInput(e.target.value, shapePolygonSides)),
                )
              }
              className="w-12 rounded border border-white/10 bg-transparent px-1 py-0.5 text-[11px] text-slate-200 focus-visible:outline-none"
            />
          </div>
          <ToolChoiceButton active={shapeStarMode} onClick={() => setShapeStarMode((v) => !v)}>
            Star
          </ToolChoiceButton>
          {shapeStarMode ? (
            <div className="flex items-center gap-1">
              <span className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Inner</span>
              <input
                type="number"
                min={1}
                max={99}
                step={1}
                value={shapePolygonInnerRadiusPct}
                onChange={(e) =>
                  setShapePolygonInnerRadiusPct(
                    Math.min(99, Math.max(1, Number(e.target.value) || 1)),
                  )
                }
                className="w-12 rounded border border-white/10 bg-transparent px-1 py-0.5 text-[11px] text-slate-200 focus-visible:outline-none"
              />
              <span className="text-[10px] text-slate-500">%</span>
            </div>
          ) : null}
        </>
      )}
      {shapeSubTool === "custom-shape" && selectedShapePreset ? (
        <div className="flex items-center gap-2 text-[11px] text-slate-400">
          <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">Preset</span>
          <span className="rounded border border-white/10 bg-black/20 px-2 py-0.5 text-slate-200">
            {selectedShapePreset.name}
          </span>
          <button
            type="button"
            className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-0.5 text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
            onClick={() => setActiveAuxPanel("shapes")}
          >
            Library
          </button>
        </div>
      ) : null}
      <span className="text-[11px] text-slate-400">
        Drag to draw. Shift = constrain aspect ratio.
      </span>
    </>
  );
}
