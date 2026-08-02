import { CommandID, type LayerNodeMeta, type SetVectorLayerStyleCommand } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { ContextualColorPickerDialog } from "@/components/brush-color-panels";
import { type Rgba, toMutableRgba, toRgba } from "@/lib/color";
import type { EngineContextValue } from "@/wasm/types";

type VectorColorSession = {
  layerId: string;
  target: "fill" | "stroke";
  initialColor: Rgba;
  fillColor: Rgba;
  strokeColor: Rgba;
  strokeWidth: number;
};

export function VectorPropertiesPanel({
  engine,
  layer,
}: {
  engine: EngineContextValue;
  layer: LayerNodeMeta;
}) {
  const fillColor = layer.fillColor ?? [0, 0, 0, 255];
  const strokeColor = layer.strokeColor ?? [0, 0, 0, 0];
  const strokeWidth = layer.strokeWidth ?? 0;
  const [colorSession, setColorSession] = useState<VectorColorSession | null>(null);
  const colorSessionRef = useRef(colorSession);
  colorSessionRef.current = colorSession;
  const colorSessionLayerId = layer.id;

  useEffect(() => {
    return () => {
      if (colorSessionRef.current?.layerId === colorSessionLayerId) {
        colorSessionRef.current = null;
        setColorSession(null);
        engine.endTransaction(false);
      }
    };
  }, [colorSessionLayerId, engine]);

  const apply = (
    fill: [number, number, number, number],
    stroke: [number, number, number, number],
    width: number,
  ) => {
    engine.dispatchCommand(CommandID.SetVectorLayerStyle, {
      layerId: layer.id,
      fillColor: fill,
      strokeColor: stroke,
      strokeWidth: width,
    } satisfies SetVectorLayerStyleCommand);
  };

  const openColorEditor = (target: VectorColorSession["target"]) => {
    if (colorSessionRef.current) {
      return;
    }
    const currentFill = toRgba(fillColor);
    const currentStroke = toRgba(strokeColor);
    const session: VectorColorSession = {
      layerId: layer.id,
      target,
      initialColor: target === "fill" ? currentFill : currentStroke,
      fillColor: currentFill,
      strokeColor: currentStroke,
      strokeWidth,
    };
    engine.beginTransaction(target === "fill" ? "Change vector fill" : "Change vector stroke");
    colorSessionRef.current = session;
    setColorSession(session);
  };

  const previewColor = (next: Rgba) => {
    const session = colorSessionRef.current;
    if (!session) {
      return;
    }
    apply(
      toMutableRgba(session.target === "fill" ? next : session.fillColor),
      toMutableRgba(session.target === "stroke" ? next : session.strokeColor),
      session.strokeWidth,
    );
  };

  const finishColorEditor = (commit: boolean) => {
    if (!colorSessionRef.current) {
      return;
    }
    colorSessionRef.current = null;
    setColorSession(null);
    engine.endTransaction(commit);
  };

  return (
    <div className="flex flex-col gap-3 p-2">
      <div className="text-[10px] uppercase tracking-[0.18em] text-muted-foreground/70">Shape</div>

      {/* Fill */}
      <div className="flex items-center gap-2">
        <span className="w-14 text-[11px] text-muted-foreground">Fill</span>
        <button
          type="button"
          title="Fill color"
          aria-label="Fill color"
          style={{
            background:
              fillColor[3] === 0
                ? "repeating-conic-gradient(#555 0% 25%, #333 0% 50%) 0 0/8px 8px"
                : `rgba(${fillColor[0]},${fillColor[1]},${fillColor[2]},${fillColor[3] / 255})`,
          }}
          className="h-6 w-6 flex-shrink-0 rounded border border-border focus-visible:outline-none"
          onClick={() => openColorEditor("fill")}
        />
        <span className="text-[10px] text-muted-foreground/70">
          {fillColor[3] === 0
            ? "None"
            : `rgba(${fillColor[0]},${fillColor[1]},${fillColor[2]},${(fillColor[3] / 255).toFixed(2)})`}
        </span>
      </div>

      {/* Stroke */}
      <div className="flex items-center gap-2">
        <span className="w-14 text-[11px] text-muted-foreground">Stroke</span>
        <button
          type="button"
          title="Stroke color"
          aria-label="Stroke color"
          style={{
            background:
              strokeColor[3] === 0
                ? "repeating-conic-gradient(#555 0% 25%, #333 0% 50%) 0 0/8px 8px"
                : `rgba(${strokeColor[0]},${strokeColor[1]},${strokeColor[2]},${strokeColor[3] / 255})`,
          }}
          className="h-6 w-6 flex-shrink-0 rounded border border-border focus-visible:outline-none"
          onClick={() => openColorEditor("stroke")}
        />
        <input
          type="number"
          min={0}
          max={200}
          step={1}
          value={strokeWidth}
          disabled={strokeColor[3] === 0}
          onChange={(e) => apply(fillColor, strokeColor, Math.max(0, Number(e.target.value)))}
          className="w-14 rounded border border-border bg-transparent px-1 py-0.5 text-[11px] text-foreground disabled:opacity-40 focus-visible:outline-none"
        />
        <span className="text-[10px] text-muted-foreground/70">px</span>
      </div>

      {colorSession?.layerId === layer.id ? (
        <ContextualColorPickerDialog
          title={colorSession.target === "fill" ? "Vector Fill" : "Vector Stroke"}
          description="Preview the paint, choose None for transparency, or cancel to restore it."
          initialColor={colorSession.initialColor}
          onPreview={previewColor}
          onApply={() => finishColorEditor(true)}
          onCancel={() => finishColorEditor(false)}
          allowNone
        />
      ) : null}

      {/* Edit Path button */}
      <button
        type="button"
        className="mt-1 rounded border border-accent/40 bg-accent/15 px-2 py-1 text-[11px] text-accent hover:bg-accent/25 focus-visible:outline-none"
        onClick={() => {
          engine.dispatchCommand(CommandID.EnterVectorEditMode, {
            layerId: layer.id,
          });
        }}
      >
        Edit Path
      </button>
    </div>
  );
}
