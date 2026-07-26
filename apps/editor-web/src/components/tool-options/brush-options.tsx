import { CommandID, type LayerBlendMode } from "@agogo/proto";
import { BrushPresetPicker, MIXER_BRUSH_PRESETS } from "@/components/brush-color-panels";
import type { EditorTool } from "@/components/tool-rail-model";
import { useBrushState } from "@/state/brush-state";
import { useCursorState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";
import { ToolChoiceButton, ToolNumberField, ToolOptionGroup, ToolSelectField } from "./controls";

const paintBlendModeOptions: { value: LayerBlendMode; label: string }[] = [
  { value: "normal", label: "Normal" },
  { value: "multiply", label: "Multiply" },
  { value: "screen", label: "Screen" },
  { value: "overlay", label: "Overlay" },
  { value: "soft-light", label: "Soft Light" },
  { value: "hard-light", label: "Hard Light" },
  { value: "color-dodge", label: "Color Dodge" },
  { value: "color-burn", label: "Color Burn" },
  { value: "difference", label: "Difference" },
  { value: "color", label: "Color" },
  { value: "luminosity", label: "Luminosity" },
];

export function BrushOptions({
  activeTool,
  openBrushPresetImport,
}: {
  activeTool: EditorTool;
  openBrushPresetImport: () => void;
}) {
  const engine = useEngine();
  const history = useUiMeta((meta) => meta?.history);
  const historyIndex = useUiMeta((meta) => meta?.currentHistoryIndex);
  const { cursor } = useCursorState();
  const {
    brushPresetId,
    brushBlendMode,
    setBrushBlendMode,
    brushOpacity,
    setBrushOpacity,
    brushAirbrush,
    setBrushAirbrush,
    brushSmoothing,
    setBrushSmoothing,
    pressureAffectsSize,
    setPressureAffectsSize,
    pressureAffectsOpacity,
    setPressureAffectsOpacity,
    pressureAffectsFlow,
    setPressureAffectsFlow,
    brushFlow,
    setBrushFlow,
    brushSize,
    setBrushSize,
    brushHardness,
    setBrushHardness,
    mixerBrushWetness,
    setMixerBrushWetness,
    mixerBrushLoad,
    setMixerBrushLoad,
    mixerBrushSampleMerged,
    setMixerBrushSampleMerged,
    cloneStampAligned,
    setCloneStampAligned,
    cloneStampAlignedOffset,
    cloneStampSampleMerged,
    setCloneStampSampleMerged,
    cloneStampOpacity,
    setCloneStampOpacity,
    cloneStampLoad,
    setCloneStampLoad,
    cloneStampUseHistorySource,
    setCloneStampUseHistorySource,
    cloneStampHistorySourceIndex,
    setCloneStampHistorySourceIndex,
    cloneStampSource,
    historyBrushSourceIndex,
    setHistoryBrushSourceIndex,
    historyBrushOpacity,
    setHistoryBrushOpacity,
    historyBrushLoad,
    setHistoryBrushLoad,
    historyBrushSampleMerged,
    setHistoryBrushSampleMerged,
    pencilAutoErase,
    setPencilAutoErase,
    eraserMode,
    setEraserMode,
    eraserTolerance,
    setEraserTolerance,
    brushPresetStatus,
    brushPresets,
    customBrushPresetIds,
    currentMixerPreset,
    applyBrushPreset,
    applyMixerBrushPreset,
  } = useBrushState();

  const historyEntries = history ?? [];
  const currentHistoryIndex = historyIndex ?? 0;
  const selectedCloneHistoryEntry =
    cloneStampHistorySourceIndex === null
      ? null
      : (historyEntries.find((entry) => entry.id === cloneStampHistorySourceIndex) ?? null);
  const selectedHistoryBrushEntry =
    historyBrushSourceIndex === null
      ? null
      : (historyEntries.find((entry) => entry.id === historyBrushSourceIndex) ?? null);
  const cloneStampOffsetDisplay =
    cloneStampSource && cursor
      ? cloneStampAligned && cloneStampAlignedOffset
        ? cloneStampAlignedOffset
        : {
            x: cloneStampSource.x - cursor.x,
            y: cloneStampSource.y - cursor.y,
          }
      : cloneStampAlignedOffset;

  if (
    activeTool === "brush" ||
    activeTool === "pencil" ||
    activeTool === "mixerBrush" ||
    activeTool === "cloneStamp" ||
    activeTool === "historyBrush"
  ) {
    return (
      <>
        {activeTool === "mixerBrush" ? (
          <>
            <ToolOptionGroup label="Preset">
              {MIXER_BRUSH_PRESETS.map((preset) => (
                <ToolChoiceButton
                  key={preset.id}
                  active={currentMixerPreset?.id === preset.id}
                  onClick={() => applyMixerBrushPreset(preset)}
                >
                  {preset.name}
                </ToolChoiceButton>
              ))}
            </ToolOptionGroup>
            <span className="text-[11px] text-slate-400">
              {currentMixerPreset?.description ??
                "Custom mix. Tip or paint settings no longer match a saved preset."}
            </span>
          </>
        ) : (
          <BrushPresetPicker
            selectedPresetId={brushPresetId}
            onSelectPreset={applyBrushPreset}
            presets={brushPresets}
            customPresetIds={customBrushPresetIds}
            onImportPresets={openBrushPresetImport}
            importStatus={brushPresetStatus}
          />
        )}
        <ToolSelectField
          label="Blend"
          value={brushBlendMode}
          onChange={(value) => setBrushBlendMode(value as LayerBlendMode)}
          options={paintBlendModeOptions}
        />
        {activeTool !== "cloneStamp" && activeTool !== "historyBrush" ? (
          <ToolNumberField
            label="Opacity"
            min={0}
            max={1}
            step={0.05}
            value={brushOpacity}
            onChange={setBrushOpacity}
          />
        ) : null}
        <ToolNumberField
          label="Flow"
          min={0}
          max={1}
          step={0.05}
          value={brushFlow}
          onChange={setBrushFlow}
        />
        <ToolChoiceButton
          active={brushAirbrush}
          onClick={() => setBrushAirbrush((value) => !value)}
        >
          Airbrush
        </ToolChoiceButton>
        <ToolNumberField
          label="Smooth"
          min={0}
          max={20}
          step={1}
          value={brushSmoothing}
          onChange={(value) => setBrushSmoothing(Math.max(0, Math.round(value)))}
        />
        <ToolOptionGroup label="Pressure">
          <ToolChoiceButton
            active={pressureAffectsSize}
            onClick={() => setPressureAffectsSize((value) => !value)}
          >
            Size
          </ToolChoiceButton>
          <ToolChoiceButton
            active={pressureAffectsOpacity}
            onClick={() => setPressureAffectsOpacity((value) => !value)}
          >
            Opacity
          </ToolChoiceButton>
          <ToolChoiceButton
            active={pressureAffectsFlow}
            onClick={() => setPressureAffectsFlow((value) => !value)}
          >
            Flow
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Size"
          min={1}
          max={2500}
          step={1}
          value={brushSize}
          onChange={setBrushSize}
        />
        {activeTool === "mixerBrush" ? (
          <>
            <ToolNumberField
              label="Wetness"
              min={0}
              max={1}
              step={0.05}
              value={mixerBrushWetness}
              onChange={setMixerBrushWetness}
            />
            <ToolNumberField
              label="Load"
              min={0}
              max={1}
              step={0.05}
              value={mixerBrushLoad}
              onChange={setMixerBrushLoad}
            />
            <ToolChoiceButton
              active={mixerBrushSampleMerged}
              onClick={() => setMixerBrushSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
            <ToolChoiceButton
              active={false}
              onClick={() => engine.dispatchCommand(CommandID.ResetMixerBrushState, {})}
            >
              Clean Brush
            </ToolChoiceButton>
            <span className="text-[11px] text-slate-500">
              {currentMixerPreset?.name ?? "Custom Mix"} · {Math.round(mixerBrushWetness * 100)}%
              wet · {Math.round(mixerBrushLoad * 100)}% loaded
            </span>
          </>
        ) : activeTool === "cloneStamp" ? (
          <>
            <ToolNumberField
              label="Opacity"
              min={0}
              max={1}
              step={0.05}
              value={cloneStampOpacity}
              onChange={setCloneStampOpacity}
            />
            <ToolNumberField
              label="Load"
              min={0}
              max={1}
              step={0.05}
              value={cloneStampLoad}
              onChange={setCloneStampLoad}
            />
            <ToolChoiceButton
              active={cloneStampAligned}
              onClick={() => setCloneStampAligned((value) => !value)}
            >
              Aligned
            </ToolChoiceButton>
            <ToolChoiceButton
              active={cloneStampSampleMerged}
              onClick={() => setCloneStampSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
            <ToolChoiceButton
              active={cloneStampUseHistorySource}
              onClick={() => {
                setCloneStampUseHistorySource((value) => !value);
                if (!cloneStampUseHistorySource && cloneStampHistorySourceIndex === null) {
                  setCloneStampHistorySourceIndex(
                    historyEntries.find((entry) => entry.id === currentHistoryIndex)?.id ??
                      historyEntries[historyEntries.length - 1]?.id ??
                      null,
                  );
                }
              }}
            >
              History Source
            </ToolChoiceButton>
            {cloneStampUseHistorySource && historyEntries.length > 0 ? (
              <ToolSelectField
                label="State"
                value={String(cloneStampHistorySourceIndex ?? currentHistoryIndex)}
                onChange={(value) => setCloneStampHistorySourceIndex(Number(value))}
                options={historyEntries.map((entry) => ({
                  value: String(entry.id),
                  label:
                    entry.state === "undone" ? `${entry.description} (Undone)` : entry.description,
                }))}
              />
            ) : null}
            <div className="text-[11px] text-slate-400">
              {cloneStampSource
                ? `${cloneStampAligned ? "Aligned" : "Non-aligned"} source at ${Math.round(cloneStampSource.x)}, ${Math.round(cloneStampSource.y)}`
                : "Alt-click the canvas to set a clone source."}
            </div>
            {cloneStampOffsetDisplay ? (
              <div className="text-[11px] text-slate-500">
                Offset {Math.round(cloneStampOffsetDisplay.x)},{" "}
                {Math.round(cloneStampOffsetDisplay.y)}
                {selectedCloneHistoryEntry ? ` · ${selectedCloneHistoryEntry.description}` : ""}
              </div>
            ) : selectedCloneHistoryEntry ? (
              <div className="text-[11px] text-slate-500">
                Source state: {selectedCloneHistoryEntry.description}
              </div>
            ) : null}
          </>
        ) : activeTool === "historyBrush" ? (
          <>
            <ToolNumberField
              label="Opacity"
              min={0}
              max={1}
              step={0.05}
              value={historyBrushOpacity}
              onChange={setHistoryBrushOpacity}
            />
            <ToolNumberField
              label="Load"
              min={0}
              max={1}
              step={0.05}
              value={historyBrushLoad}
              onChange={setHistoryBrushLoad}
            />
            <ToolChoiceButton
              active={historyBrushSampleMerged}
              onClick={() => setHistoryBrushSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
            {historyEntries.length > 0 ? (
              <ToolSelectField
                label="State"
                value={String(historyBrushSourceIndex ?? currentHistoryIndex)}
                onChange={(value) => setHistoryBrushSourceIndex(Number(value))}
                options={historyEntries.map((entry) => ({
                  value: String(entry.id),
                  label:
                    entry.state === "undone" ? `${entry.description} (Undone)` : entry.description,
                }))}
              />
            ) : null}
            <div className="text-[11px] text-slate-400">
              Paints from the selected history state at the current cursor position.
            </div>
            {selectedHistoryBrushEntry ? (
              <div className="text-[11px] text-slate-500">
                Source state: {selectedHistoryBrushEntry.description}
              </div>
            ) : null}
          </>
        ) : null}
        {activeTool === "pencil" ? (
          <label className="flex items-center gap-1 text-[10px]">
            <input
              type="checkbox"
              checked={pencilAutoErase}
              onChange={(e) => setPencilAutoErase(e.target.checked)}
            />
            Auto-erase
          </label>
        ) : null}
      </>
    );
  }

  if (activeTool === "eraser") {
    return (
      <>
        <BrushPresetPicker
          selectedPresetId={brushPresetId}
          onSelectPreset={applyBrushPreset}
          presets={brushPresets}
          customPresetIds={customBrushPresetIds}
          onImportPresets={openBrushPresetImport}
          importStatus={brushPresetStatus}
        />
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={eraserMode === "normal"}
            onClick={() => setEraserMode("normal")}
          >
            Normal
          </ToolChoiceButton>
          <ToolChoiceButton
            active={eraserMode === "background"}
            onClick={() => setEraserMode("background")}
          >
            Background
          </ToolChoiceButton>
          <ToolChoiceButton active={eraserMode === "magic"} onClick={() => setEraserMode("magic")}>
            Magic
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Opacity"
          min={0}
          max={1}
          step={0.05}
          value={brushOpacity}
          onChange={setBrushOpacity}
        />
        <ToolNumberField
          label="Flow"
          min={0}
          max={1}
          step={0.05}
          value={brushFlow}
          onChange={setBrushFlow}
        />
        <ToolChoiceButton
          active={brushAirbrush}
          onClick={() => setBrushAirbrush((value) => !value)}
        >
          Airbrush
        </ToolChoiceButton>
        <ToolNumberField
          label="Smooth"
          min={0}
          max={20}
          step={1}
          value={brushSmoothing}
          onChange={(value) => setBrushSmoothing(Math.max(0, Math.round(value)))}
        />
        <ToolOptionGroup label="Pressure">
          <ToolChoiceButton
            active={pressureAffectsSize}
            onClick={() => setPressureAffectsSize((value) => !value)}
          >
            Size
          </ToolChoiceButton>
          <ToolChoiceButton
            active={pressureAffectsOpacity}
            onClick={() => setPressureAffectsOpacity((value) => !value)}
          >
            Opacity
          </ToolChoiceButton>
          <ToolChoiceButton
            active={pressureAffectsFlow}
            onClick={() => setPressureAffectsFlow((value) => !value)}
          >
            Flow
          </ToolChoiceButton>
        </ToolOptionGroup>
        {eraserMode !== "magic" ? (
          <>
            <ToolNumberField
              label="Size"
              min={1}
              max={2500}
              step={1}
              value={brushSize}
              onChange={setBrushSize}
            />
            <ToolNumberField
              label="Hardness"
              min={0}
              max={1}
              step={0.05}
              value={brushHardness}
              onChange={setBrushHardness}
            />
          </>
        ) : null}
        {eraserMode !== "normal" ? (
          <ToolNumberField
            label="Tolerance"
            min={0}
            max={255}
            step={1}
            value={eraserTolerance}
            onChange={setEraserTolerance}
          />
        ) : null}
      </>
    );
  }

  return null;
}
