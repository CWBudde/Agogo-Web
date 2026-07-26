import type { GradientType } from "@agogo/proto";
import { PatternPicker } from "@/components/pattern-picker";
import type { EditorTool } from "@/components/tool-rail-model";
import { gradientStopsToCss, rgbaToCss } from "@/lib/color";
import { useColorState } from "@/state/color-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";
import { ToolChoiceButton, ToolNumberField, ToolOptionGroup } from "./controls";

export function FillGradientOptions({ activeTool }: { activeTool: EditorTool }) {
  const engine = useEngine();
  const patterns = useUiMeta((meta) => meta?.patterns);
  const { foregroundColor, backgroundColor } = useColorState();
  const {
    fillSource,
    setFillSource,
    fillPatternId,
    setFillPatternId,
    fillTolerance,
    setFillTolerance,
    fillContiguous,
    setFillContiguous,
    fillSampleMerged,
    setFillSampleMerged,
    fillCreateLayer,
    setFillCreateLayer,
    setFillDialogOpen,
    gradientType,
    setGradientType,
    gradientReverse,
    setGradientReverse,
    gradientDither,
    setGradientDither,
    gradientCreateLayer,
    setGradientCreateLayer,
    gradientStops,
    setGradientEditorOpen,
  } = useFillGradientState();

  const fillSourceName =
    fillSource === "foreground" ? "Color" : fillSource === "background" ? "Background" : "Pattern";
  const fillModeSummary = `${fillSourceName} fill · ${fillContiguous ? "contiguous" : "all matching"} · ${fillSampleMerged ? "sample merged" : "active layer"} · ${fillCreateLayer ? "new layer" : "paint in place"}`;
  const gradientModeSummary = `${gradientType.charAt(0).toUpperCase() + gradientType.slice(1)} · ${gradientStops.length} stops · ${gradientReverse ? "reversed" : "forward"} · ${gradientDither ? "dither" : "no dither"} · ${gradientCreateLayer ? "new layer" : "paint in place"}`;
  const gradientPreviewStyle = {
    backgroundImage: gradientStopsToCss(gradientStops),
  };

  if (activeTool === "fill") {
    return (
      <>
        <ToolOptionGroup label="Source">
          <ToolChoiceButton
            active={fillSource === "foreground"}
            onClick={() => setFillSource("foreground")}
          >
            Color
          </ToolChoiceButton>
          <ToolChoiceButton
            active={fillSource === "background"}
            onClick={() => setFillSource("background")}
          >
            Background
          </ToolChoiceButton>
          <ToolChoiceButton
            active={fillSource === "pattern"}
            onClick={() => setFillSource("pattern")}
          >
            Pattern
          </ToolChoiceButton>
        </ToolOptionGroup>
        {fillSource === "pattern" ? (
          <PatternPicker
            engine={engine}
            patterns={patterns ?? []}
            value={fillPatternId}
            onChange={setFillPatternId}
          />
        ) : null}
        <ToolNumberField
          label="Tolerance"
          min={0}
          max={255}
          step={1}
          value={fillTolerance}
          onChange={setFillTolerance}
        />
        <ToolChoiceButton active={fillContiguous} onClick={() => setFillContiguous((v) => !v)}>
          Contiguous
        </ToolChoiceButton>
        <ToolChoiceButton active={fillSampleMerged} onClick={() => setFillSampleMerged((v) => !v)}>
          Sample Merged
        </ToolChoiceButton>
        <ToolChoiceButton active={fillCreateLayer} onClick={() => setFillCreateLayer((v) => !v)}>
          New Layer
        </ToolChoiceButton>
        <div className="flex items-center gap-2 text-[11px] text-slate-400">
          <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">Preview</span>
          <span
            className="h-4 w-12 rounded border border-white/10"
            style={
              fillSource === "pattern"
                ? {
                    backgroundColor: "rgba(15, 23, 42, 1)",
                    backgroundImage:
                      "linear-gradient(45deg, rgba(148, 163, 184, 0.35) 25%, transparent 25%, transparent 50%, rgba(148, 163, 184, 0.35) 50%, rgba(148, 163, 184, 0.35) 75%, transparent 75%, transparent)",
                    backgroundSize: "10px 10px",
                  }
                : {
                    backgroundColor:
                      fillSource === "background"
                        ? rgbaToCss(backgroundColor)
                        : rgbaToCss(foregroundColor),
                  }
            }
          />
          <span>{fillModeSummary}</span>
        </div>
        <button
          type="button"
          className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-0.5 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
          onClick={() => setFillDialogOpen(true)}
        >
          Fill Dialog
        </button>
      </>
    );
  }

  if (activeTool === "gradient") {
    return (
      <>
        <ToolOptionGroup label="Type">
          {(["linear", "radial", "angle", "reflected", "diamond"] as GradientType[]).map((type) => (
            <ToolChoiceButton
              key={type}
              active={gradientType === type}
              onClick={() => setGradientType(type)}
            >
              {type.charAt(0).toUpperCase() + type.slice(1)}
            </ToolChoiceButton>
          ))}
        </ToolOptionGroup>
        <ToolChoiceButton active={gradientReverse} onClick={() => setGradientReverse((v) => !v)}>
          Reverse
        </ToolChoiceButton>
        <ToolChoiceButton active={gradientDither} onClick={() => setGradientDither((v) => !v)}>
          Dither
        </ToolChoiceButton>
        <ToolChoiceButton
          active={gradientCreateLayer}
          onClick={() => setGradientCreateLayer((v) => !v)}
        >
          New Layer
        </ToolChoiceButton>
        <div className="flex items-center gap-2 text-[11px] text-slate-400">
          <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">Preview</span>
          <span className="h-4 w-24 rounded border border-white/10" style={gradientPreviewStyle} />
          <span>{gradientModeSummary}</span>
        </div>
        <button
          type="button"
          className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-0.5 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
          onClick={() => setGradientEditorOpen(true)}
        >
          Edit Gradient
        </button>
        <span className="text-[11px] text-slate-400">Drag on the canvas to set the gradient.</span>
      </>
    );
  }

  return null;
}
