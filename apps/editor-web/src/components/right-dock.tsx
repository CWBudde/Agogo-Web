import { CommandID, type CreateDocumentCommand } from "@agogo/proto";
import { AdjPropertiesPanel, AdjustmentsPanel } from "@/components/adjustments-panel";
import {
  BrushSettingsPanel,
  ColorPanel,
  MIXER_BRUSH_PRESETS,
  SwatchesPanel,
} from "@/components/brush-color-panels";
import { ChannelsPanel } from "@/components/channels-panel";
import { CompactRange } from "@/components/compact-range";
import { type AuxPanel, DockSection, dockTitle } from "@/components/dock-section";
import { InfoPanel } from "@/components/info-panel";
import { LayersPanel } from "@/components/layers-panel";
import { NavigatorPreview } from "@/components/navigator-preview";
import { PathsPanel } from "@/components/paths-panel";
import { PropertyGridRow } from "@/components/property-grid-row";
import { ShapesPanel } from "@/components/shapes-panel";
import { StylesPanel } from "@/components/styles-panel";
import {
  ToolChoiceButton,
  ToolNumberField,
  ToolSelectField,
} from "@/components/tool-options/controls";
import { Button } from "@/components/ui/button";
import { useLayerThumbnails } from "@/hooks/use-layer-thumbnails";
import { formatPercent, type Rgba } from "@/lib/color";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useShapeState } from "@/state/shape-state";
import { useToolState } from "@/state/tool-state";
import { useCursorState, useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";
import { useEngineRender } from "@/wasm/use-engine-render";

const MAX_SWATCHES = 96;

interface RightDockProps {
  draft: CreateDocumentCommand;
  openBrushPresetImport: () => void;
  openShapePresetImport: () => void;
  openSwatchImport: () => void;
  exportSwatchSet: () => void;
}

export function RightDock({
  draft,
  openBrushPresetImport,
  openShapePresetImport,
  openSwatchImport,
  exportSwatchSet,
}: RightDockProps) {
  const engine = useEngine();
  // The right dock displays viewport (zoom/rotation) and most of uiMeta, so it
  // genuinely tracks the whole render snapshot; it re-renders on committed
  // frames but no longer via the engine context value.
  const render = useEngineRender((state) => state);
  const { activeTool } = useToolState();
  const { cursor } = useCursorState();
  const {
    panelCollapsed,
    setPanelCollapsed,
    panelWidth,
    setPanelWidth,
    activeAuxPanel,
    setActiveAuxPanel,
    selectedLayerIds,
    setSelectedLayerIds,
  } = useViewState();
  const {
    foregroundColor,
    backgroundColor,
    colorPickerTarget,
    setColorPickerTarget,
    setColorPickerOpen,
    colorChannelMode,
    setColorChannelMode,
    onlyWebColors,
    setOnlyWebColors,
    recentColors,
    swatches,
    setSwatches,
    swatchSetName,
    swatchStatus,
    colorSamplerPoints,
    setColorSamplerPoints,
    applyColorToTarget,
  } = useColorState();
  const {
    brushPresetId,
    brushTipShape,
    setBrushTipShape,
    setBrushTipResourceId,
    brushSize,
    setBrushSize,
    brushHardness,
    setBrushHardness,
    brushAngle,
    setBrushAngle,
    brushRoundness,
    setBrushRoundness,
    brushSpacing,
    setBrushSpacing,
    brushSizeJitter,
    setBrushSizeJitter,
    brushOpacityJitter,
    setBrushOpacityJitter,
    brushFlowJitter,
    setBrushFlowJitter,
    brushControlSource,
    setBrushControlSource,
    mixerBrushWetness,
    setMixerBrushWetness,
    mixerBrushLoad,
    setMixerBrushLoad,
    mixerBrushSampleMerged,
    setMixerBrushSampleMerged,
    cloneStampAligned,
    setCloneStampAligned,
    cloneStampAlignedOffset,
    setCloneStampAlignedOffset,
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
    setCloneStampSource,
    historyBrushSourceIndex,
    setHistoryBrushSourceIndex,
    historyBrushOpacity,
    setHistoryBrushOpacity,
    historyBrushLoad,
    setHistoryBrushLoad,
    historyBrushSampleMerged,
    setHistoryBrushSampleMerged,
    brushPresetStatus,
    brushPresets,
    customBrushPresetIds,
    currentMixerPreset,
    applyBrushPreset,
    applyMixerBrushPreset,
  } = useBrushState();
  const {
    shapeSubTool,
    setShapePresetId,
    shapePresetStatus,
    shapePresets,
    customShapePresetIds,
    selectedShapePreset,
  } = useShapeState();

  const layerThumbnails = useLayerThumbnails();

  const savedSelectionChannels = render?.uiMeta.savedSelectionChannels ?? [];
  const documentSize = render
    ? `${render.uiMeta.documentWidth} x ${render.uiMeta.documentHeight}`
    : "No document";
  const zoomPercent = render ? `${Math.round(render.viewport.zoom * 100)}%` : "0%";
  const cursorText = cursor ? `${cursor.x}, ${cursor.y}` : "—";

  const historyEntries = render?.uiMeta.history ?? [];
  const currentHistoryIndex = render?.uiMeta.currentHistoryIndex ?? 0;
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

  const activeColor = colorPickerTarget === "foreground" ? foregroundColor : backgroundColor;
  const setActiveColor = (next: Rgba) => applyColorToTarget(colorPickerTarget, next);

  return (
    <aside className="relative min-h-[36rem]">
      <div
        className="absolute inset-y-[var(--ui-gap-2)] left-0 z-10 w-2 -translate-x-1/2 cursor-col-resize"
        onPointerDown={(event) => {
          if (panelCollapsed) {
            return;
          }
          const startX = event.clientX;
          const startWidth = panelWidth;
          const handleMove = (moveEvent: PointerEvent) => {
            setPanelWidth(Math.min(420, Math.max(280, startWidth - (moveEvent.clientX - startX))));
          };
          const handleUp = () => {
            window.removeEventListener("pointermove", handleMove);
            window.removeEventListener("pointerup", handleUp);
          };
          window.addEventListener("pointermove", handleMove);
          window.addEventListener("pointerup", handleUp);
        }}
      />

      {panelCollapsed ? (
        <div className="editor-panel flex h-full flex-col items-center gap-[var(--ui-gap-1)] border-l border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
          <Button
            variant="ghost"
            size="icon"
            className="text-[11px]"
            aria-label="Expand panel"
            onClick={() => setPanelCollapsed(false)}
          >
            »
          </Button>
          {["P", "C", "H", "N", "L"].map((label) => (
            <div
              key={label}
              className="flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] text-slate-400"
            >
              {label}
            </div>
          ))}
        </div>
      ) : (
        <div className="editor-panel flex h-full flex-col overflow-hidden border-l border-border">
          <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
            <div className="flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-[var(--ui-gap-1)] overflow-x-auto pb-1">
                {[
                  ["properties", "Properties"],
                  ["info", "Info"],
                  ["adjustments", "Adjust"],
                  ["styles", "Styles"],
                  ["brush", "Brush"],
                  ["color", "Color"],
                  ["swatches", "Swatches"],
                  ["channels", "Channels"],
                  ["paths", "Paths"],
                  ["shapes", "Shapes"],
                  ["history", "History"],
                  ["navigator", "Navigator"],
                ].map(([id, label]) => (
                  <button
                    key={id}
                    type="button"
                    className={[
                      "rounded-[1px] border px-2 py-1 text-[11px] transition focus-visible:outline-none",
                      activeAuxPanel === id
                        ? "border-white/12 bg-panel-soft text-slate-100"
                        : "border-transparent text-slate-400 hover:border-white/8 hover:bg-white/5 hover:text-slate-200",
                    ].join(" ")}
                    onClick={() => setActiveAuxPanel(id as AuxPanel)}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="text-[11px]"
                aria-label="Collapse panel"
                onClick={() => setPanelCollapsed(true)}
              >
                «
              </Button>
            </div>
          </div>

          <div className="grid min-h-0 flex-1 grid-rows-[minmax(15rem,18rem)_minmax(0,1fr)]">
            <DockSection title={dockTitle(activeAuxPanel)}>
              {activeAuxPanel === "properties" ? (
                <AdjPropertiesPanel
                  engine={engine}
                  layers={render?.uiMeta.layers ?? []}
                  activeLayerId={render?.uiMeta.activeLayerId ?? null}
                  availableFonts={render?.uiMeta.availableFonts ?? []}
                  fallback={
                    <div className="space-y-[var(--ui-gap-3)]">
                      <PropertyGridRow label="Document" value={documentSize} />
                      <PropertyGridRow label="Zoom" value={zoomPercent} />
                      <PropertyGridRow
                        label="Rotation"
                        value={`${render?.viewport.rotation.toFixed(0) ?? 0}°`}
                      />
                      <PropertyGridRow label="DPI" value={draft.resolution.toString()} />
                      <CompactRange
                        id="rotate-view-range"
                        label="Rotate View"
                        min={0}
                        max={360}
                        step={1}
                        value={render?.viewport.rotation ?? 0}
                        onChange={(value) => engine.setRotation(value)}
                      />
                    </div>
                  }
                />
              ) : null}

              {activeAuxPanel === "info" ? (
                <InfoPanel
                  cursorText={cursorText}
                  documentSize={documentSize}
                  zoomPercent={zoomPercent}
                  samplerPoints={colorSamplerPoints}
                  onRemoveSampler={(id) =>
                    setColorSamplerPoints((current) => current.filter((point) => point.id !== id))
                  }
                  onClearSamplers={() => setColorSamplerPoints([])}
                />
              ) : null}

              {activeAuxPanel === "adjustments" ? (
                <AdjustmentsPanel
                  engine={engine}
                  layers={render?.uiMeta.layers ?? []}
                  activeLayerId={render?.uiMeta.activeLayerId ?? null}
                />
              ) : null}

              {activeAuxPanel === "styles" ? (
                <StylesPanel
                  engine={engine}
                  render={render}
                  activeLayerId={render?.uiMeta.activeLayerId ?? null}
                />
              ) : null}

              {activeAuxPanel === "shapes" ? (
                <ShapesPanel
                  active={activeTool === "shape" && shapeSubTool === "custom-shape"}
                  presets={shapePresets}
                  customPresetIds={customShapePresetIds}
                  selectedPresetId={selectedShapePreset?.id ?? ""}
                  onSelectPreset={(preset) => setShapePresetId(preset.id)}
                  onImportPresets={openShapePresetImport}
                  importStatus={shapePresetStatus}
                />
              ) : null}

              {activeAuxPanel === "brush" ? (
                <div className="space-y-3">
                  <BrushSettingsPanel
                    selectedPresetId={brushPresetId}
                    onSelectPreset={applyBrushPreset}
                    presets={brushPresets}
                    customPresetIds={customBrushPresetIds}
                    onImportPresets={openBrushPresetImport}
                    presetStatus={brushPresetStatus}
                    title={activeTool === "mixerBrush" ? "Mixer Tip" : undefined}
                    subtitle={
                      activeTool === "mixerBrush"
                        ? (currentMixerPreset?.name ?? "Custom tip profile")
                        : undefined
                    }
                    hidePresetPicker={activeTool === "mixerBrush"}
                    tipShape={brushTipShape}
                    onTipShapeChange={(shape) => {
                      setBrushTipResourceId(null);
                      setBrushTipShape(shape);
                    }}
                    size={brushSize}
                    onSizeChange={setBrushSize}
                    hardness={brushHardness}
                    onHardnessChange={setBrushHardness}
                    angle={brushAngle}
                    onAngleChange={setBrushAngle}
                    roundness={brushRoundness}
                    onRoundnessChange={setBrushRoundness}
                    spacing={brushSpacing}
                    onSpacingChange={setBrushSpacing}
                    sizeJitter={brushSizeJitter}
                    onSizeJitterChange={setBrushSizeJitter}
                    opacityJitter={brushOpacityJitter}
                    onOpacityJitterChange={setBrushOpacityJitter}
                    flowJitter={brushFlowJitter}
                    onFlowJitterChange={setBrushFlowJitter}
                    controlSource={brushControlSource}
                    onControlSourceChange={setBrushControlSource}
                  />
                  {activeTool === "mixerBrush" ? (
                    <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                            Mixer Brush
                          </p>
                          <p className="mt-1 text-[12px] text-slate-100">
                            {currentMixerPreset?.name ?? "Custom Mix"}
                          </p>
                          <p className="mt-1 max-w-[26rem] text-[11px] text-slate-400">
                            {currentMixerPreset?.description ??
                              "Current wetness, load, or tip settings differ from the saved Mixer Brush presets."}
                          </p>
                        </div>
                        <ToolChoiceButton
                          active={false}
                          onClick={() => engine.dispatchCommand(CommandID.ResetMixerBrushState, {})}
                        >
                          Clean Brush
                        </ToolChoiceButton>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {MIXER_BRUSH_PRESETS.map((preset) => (
                          <button
                            key={preset.id}
                            type="button"
                            className={[
                              "rounded-[var(--ui-radius-sm)] border px-2.5 py-1.5 text-left transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/30",
                              currentMixerPreset?.id === preset.id
                                ? "border-cyan-400/35 bg-cyan-400/12 text-slate-50"
                                : "border-white/10 bg-black/20 text-slate-300 hover:border-white/20 hover:bg-black/30",
                            ].join(" ")}
                            onClick={() => applyMixerBrushPreset(preset)}
                          >
                            <div className="text-[12px]">{preset.name}</div>
                            <div className="text-[10px] text-slate-500">
                              {Math.round(preset.wetness * 100)}% wet ·{" "}
                              {Math.round(preset.load * 100)}% load
                            </div>
                          </button>
                        ))}
                      </div>
                      <div className="mt-3 flex flex-wrap items-center gap-2">
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
                      </div>
                      <div className="mt-2 text-[11px] text-slate-500">
                        Tip: {brushTipShape} · {Math.round(brushHardness * 100)}% hard ·{" "}
                        {formatPercent(brushSpacing)} spacing
                      </div>
                    </div>
                  ) : activeTool === "cloneStamp" ? (
                    <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                      <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                        Clone Stamp
                      </p>
                      <div className="space-y-2 text-[11px] text-slate-400">
                        <div>
                          {cloneStampSource
                            ? `Source: ${Math.round(cloneStampSource.x)}, ${Math.round(cloneStampSource.y)}`
                            : "Alt-click on the canvas to define the source point."}
                        </div>
                        {cloneStampSource ? (
                          <div className="flex flex-wrap gap-2">
                            <ToolNumberField
                              label="Source X"
                              min={0}
                              max={render?.uiMeta.documentWidth ?? draft.width}
                              step={1}
                              value={cloneStampSource.x}
                              onChange={(value) => {
                                setCloneStampSource((current) =>
                                  current ? { ...current, x: value } : { x: value, y: 0 },
                                );
                                setCloneStampAlignedOffset(null);
                              }}
                            />
                            <ToolNumberField
                              label="Source Y"
                              min={0}
                              max={render?.uiMeta.documentHeight ?? draft.height}
                              step={1}
                              value={cloneStampSource.y}
                              onChange={(value) => {
                                setCloneStampSource((current) =>
                                  current ? { ...current, y: value } : { x: 0, y: value },
                                );
                                setCloneStampAlignedOffset(null);
                              }}
                            />
                          </div>
                        ) : null}
                        <div className="flex flex-wrap gap-2">
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
                        </div>
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
                            if (
                              !cloneStampUseHistorySource &&
                              cloneStampHistorySourceIndex === null
                            ) {
                              setCloneStampHistorySourceIndex(
                                historyEntries.find((entry) => entry.id === currentHistoryIndex)
                                  ?.id ??
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
                            label="History State"
                            value={String(cloneStampHistorySourceIndex ?? currentHistoryIndex)}
                            onChange={(value) => setCloneStampHistorySourceIndex(Number(value))}
                            options={historyEntries.map((entry) => ({
                              value: String(entry.id),
                              label:
                                entry.state === "undone"
                                  ? `${entry.description} (Undone)`
                                  : entry.description,
                            }))}
                          />
                        ) : null}
                        {cloneStampOffsetDisplay ? (
                          <div className="text-[11px] text-slate-500">
                            Offset: {Math.round(cloneStampOffsetDisplay.x)},{" "}
                            {Math.round(cloneStampOffsetDisplay.y)}
                          </div>
                        ) : null}
                        {selectedCloneHistoryEntry ? (
                          <div className="text-[11px] text-slate-500">
                            Source state: {selectedCloneHistoryEntry.description}
                          </div>
                        ) : null}
                        <div className="text-[11px] text-slate-500">
                          {cloneStampAligned
                            ? "Keeps the sampled offset locked across strokes until the source changes."
                            : "Restarts sampling from the source point at the beginning of each new stroke."}
                        </div>
                      </div>
                    </div>
                  ) : activeTool === "historyBrush" ? (
                    <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                      <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                        History Brush
                      </p>
                      <div className="space-y-2 text-[11px] text-slate-400">
                        <div>
                          Paints from the selected history state at the current cursor position.
                        </div>
                        <div className="flex flex-wrap gap-2">
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
                        </div>
                        <ToolChoiceButton
                          active={historyBrushSampleMerged}
                          onClick={() => setHistoryBrushSampleMerged((value) => !value)}
                        >
                          Sample Merged
                        </ToolChoiceButton>
                        {historyEntries.length > 0 ? (
                          <ToolSelectField
                            label="History State"
                            value={String(historyBrushSourceIndex ?? currentHistoryIndex)}
                            onChange={(value) => setHistoryBrushSourceIndex(Number(value))}
                            options={historyEntries.map((entry) => ({
                              value: String(entry.id),
                              label:
                                entry.state === "undone"
                                  ? `${entry.description} (Undone)`
                                  : entry.description,
                            }))}
                          />
                        ) : null}
                        {selectedHistoryBrushEntry ? (
                          <div className="text-[11px] text-slate-500">
                            Source state: {selectedHistoryBrushEntry.description}
                          </div>
                        ) : null}
                        <div className="text-[11px] text-slate-500">
                          If the chosen history state is truncated or replaced by branching, the
                          brush falls back to the current active history entry.
                        </div>
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {activeAuxPanel === "color" ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        className={[
                          "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                          colorPickerTarget === "foreground"
                            ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                            : "border-white/8 text-slate-400 hover:bg-white/5",
                        ].join(" ")}
                        onClick={() => setColorPickerTarget("foreground")}
                      >
                        Foreground
                      </button>
                      <button
                        type="button"
                        className={[
                          "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                          colorPickerTarget === "background"
                            ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                            : "border-white/8 text-slate-400 hover:bg-white/5",
                        ].join(" ")}
                        onClick={() => setColorPickerTarget("background")}
                      >
                        Background
                      </button>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-[11px]"
                      onClick={() => setColorPickerOpen(true)}
                    >
                      Open picker
                    </Button>
                  </div>
                  <ColorPanel
                    color={activeColor}
                    onChange={setActiveColor}
                    channelMode={colorChannelMode}
                    onChannelModeChange={setColorChannelMode}
                    onlyWebColors={onlyWebColors}
                    onOnlyWebColorsChange={setOnlyWebColors}
                    recentColors={recentColors}
                    onRecentColorSelect={(color) => setActiveColor(color)}
                  />
                </div>
              ) : null}

              {activeAuxPanel === "swatches" ? (
                <SwatchesPanel
                  swatches={swatches}
                  activeColor={activeColor}
                  setName={swatchSetName}
                  statusMessage={swatchStatus}
                  onPickForeground={(color) => applyColorToTarget("foreground", color)}
                  onPickBackground={(color) => applyColorToTarget("background", color)}
                  onAddSwatch={() =>
                    setSwatches((current) => [foregroundColor, ...current].slice(0, MAX_SWATCHES))
                  }
                  onImportSwatches={openSwatchImport}
                  onExportSwatches={exportSwatchSet}
                  onDeleteSwatch={(index) =>
                    setSwatches((current) =>
                      current.filter((_, swatchIndex) => swatchIndex !== index),
                    )
                  }
                />
              ) : null}

              {activeAuxPanel === "history" ? (
                <div className="flex h-full min-h-0 flex-col gap-[var(--ui-gap-2)]">
                  <div className="flex items-center justify-end">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={historyEntries.length === 0}
                      onClick={() => engine.clearHistory()}
                    >
                      Clear
                    </Button>
                  </div>
                  <div className="min-h-0 flex-1 overflow-auto">
                    <div className="space-y-[var(--ui-gap-1)]">
                      {historyEntries.length === 0 ? (
                        <p className="text-[12px] text-slate-400">No history entries yet.</p>
                      ) : (
                        historyEntries.map((entry) => (
                          <button
                            key={entry.id}
                            type="button"
                            className={[
                              "w-full rounded-[var(--ui-radius-sm)] border px-2 py-1.5 text-left text-[12px] transition focus-visible:outline-none",
                              entry.id === currentHistoryIndex
                                ? "border-cyan-400/35 bg-cyan-400/10 text-slate-100"
                                : entry.state === "undone"
                                  ? "border-white/8 bg-black/10 text-slate-500 hover:text-slate-300"
                                  : "border-white/8 bg-black/10 text-slate-200 hover:border-white/12 hover:bg-black/20",
                            ].join(" ")}
                            onClick={() => engine.jumpHistory(entry.id)}
                          >
                            {entry.description}
                          </button>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              ) : null}

              {activeAuxPanel === "navigator" ? (
                <div className="space-y-[var(--ui-gap-3)]">
                  <div className="border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0.01))] p-[var(--ui-gap-2)]">
                    <NavigatorPreview />
                  </div>
                  <CompactRange
                    id="navigator-zoom-range"
                    label="Zoom"
                    min={5}
                    max={3200}
                    step={5}
                    value={Math.round((render?.viewport.zoom ?? 1) * 100)}
                    onChange={(value) => engine.setZoom(value / 100)}
                  />
                </div>
              ) : null}

              {activeAuxPanel === "channels" ? (
                <ChannelsPanel savedSelections={savedSelectionChannels} />
              ) : null}

              {activeAuxPanel === "paths" ? (
                <PathsPanel engine={engine} paths={render?.uiMeta.paths ?? []} />
              ) : null}
            </DockSection>

            <DockSection title="Layers" className="border-t border-border">
              <LayersPanel
                engine={engine}
                layers={render?.uiMeta.layers ?? []}
                activeLayerId={render?.uiMeta.activeLayerId ?? null}
                maskEditLayerId={render?.uiMeta.maskEditLayerId ?? null}
                documentWidth={render?.uiMeta.documentWidth ?? draft.width}
                documentHeight={render?.uiMeta.documentHeight ?? draft.height}
                stylePresets={render?.uiMeta.stylePresets ?? []}
                thumbnails={layerThumbnails}
                selectedLayerIds={selectedLayerIds}
                onSelectedLayerIdsChange={setSelectedLayerIds}
              />
            </DockSection>
          </div>
        </div>
      )}
    </aside>
  );
}
