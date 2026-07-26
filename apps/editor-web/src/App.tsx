import { CommandID, type CreateDocumentCommand, type FillCommand } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { ColorPickerDialog } from "@/components/brush-color-panels";
import { CanvasHost } from "@/components/canvas-host";
import { ChannelMixerDialog } from "@/components/dialogs/adjustment-dialogs/channel-mixer-dialog";
import { GradientMapDialog } from "@/components/dialogs/adjustment-dialogs/gradient-map-dialog";
import { PhotoFilterDialog } from "@/components/dialogs/adjustment-dialogs/photo-filter-dialog";
import { SelectiveColorDialog } from "@/components/dialogs/adjustment-dialogs/selective-color-dialog";
import {
  PosterizeDialog,
  ThresholdDialog,
} from "@/components/dialogs/adjustment-dialogs/threshold-posterize-dialogs";
import { useCreateAdjustmentLayer } from "@/components/dialogs/adjustment-dialogs/use-create-adjustment-layer";
import { CanvasSizeDialog } from "@/components/dialogs/canvas-size-dialog";
import { ColorRangeDialog } from "@/components/dialogs/color-range-dialog";
import { ExportDialog } from "@/components/dialogs/export-dialog";
import { FeatherDialog } from "@/components/dialogs/feather-dialog";
import { LoadSelectionDialog } from "@/components/dialogs/load-selection-dialog";
import { ModifyDialog, type ModifyKind } from "@/components/dialogs/modify-dialog";
import { NewDocumentDialog } from "@/components/dialogs/new-document-dialog";
import { OpenRecentDialog } from "@/components/dialogs/open-recent-dialog";
import { SaveSelectionDialog } from "@/components/dialogs/save-selection-dialog";
import { EngineLoadErrorScreen } from "@/components/engine-load-error";
import { FilterDialogHost } from "@/components/filters/filter-dialog-host";
import { GradientEditorDialog } from "@/components/gradient-editor";
import { MenuBar } from "@/components/menu-bar/menu-bar";
import { RightDock } from "@/components/right-dock";
import { SelectAndMaskWorkspace } from "@/components/select-and-mask";
import { StatusBar } from "@/components/status-bar";
import { TextEditOverlay } from "@/components/text-edit-overlay";
import { ToolOptions } from "@/components/tool-options";
import {
  ToolChoiceButton,
  ToolNumberField,
  ToolOptionGroup,
} from "@/components/tool-options/controls";
import { ToolRail } from "@/components/tool-rail";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { ToastViewport } from "@/components/ui/toast";
import { WelcomeScreen } from "@/components/welcome-screen";
import { useAppShortcuts } from "@/hooks/use-app-shortcuts";
import { useAutosave } from "@/hooks/use-autosave";
import { useDocumentIo } from "@/hooks/use-document-io";
import type { MenuActionIO } from "@/hooks/use-menu-actions";
import { type Rgba, toMutableRgba } from "@/lib/color";
import { findLayerMetaInTree } from "@/lib/layer-tree";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useDialogState } from "@/state/dialog-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useShapeState } from "@/state/shape-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

const defaultDocumentDraft: CreateDocumentCommand = {
  name: "Untitled",
  width: 1920,
  height: 1080,
  resolution: 72,
  colorMode: "rgb",
  bitDepth: 8,
  background: "transparent",
};

export default function App() {
  const engine = useEngine();
  // App reads only uiMeta slices (never the viewport), so subscribing to the
  // whole uiMeta keeps it reactive to document/layer/history changes while
  // staying quiet during pan/zoom (viewport-only) frames.
  const uiMeta = useUiMeta((meta) => meta);
  const {
    foregroundColor,
    backgroundColor,
    colorPickerOpen,
    setColorPickerOpen,
    colorPickerTarget,
    colorChannelMode,
    setColorChannelMode,
    onlyWebColors,
    setOnlyWebColors,
    recentColors,
    pushRecentColor,
    applyColorToTarget,
  } = useColorState();
  const {
    cloneStampUseHistorySource,
    setCloneStampUseHistorySource,
    cloneStampHistorySourceIndex,
    setCloneStampHistorySourceIndex,
    historyBrushSourceIndex,
    setHistoryBrushSourceIndex,
  } = useBrushState();
  const {
    fillSource,
    setFillSource,
    fillPatternId,
    fillTolerance,
    setFillTolerance,
    fillContiguous,
    setFillContiguous,
    fillSampleMerged,
    setFillSampleMerged,
    fillCreateLayer,
    setFillCreateLayer,
    fillDialogOpen,
    setFillDialogOpen,
    gradientStops,
    setGradientStops,
    gradientEditorOpen,
    setGradientEditorOpen,
  } = useFillGradientState();
  const { shapeSubTool, setArtboardBackground } = useShapeState();
  const { panelCollapsed, panelWidth, setActiveAuxPanel } = useViewState();
  const { activeTool } = useToolState();
  const { setNewDocumentOpen, setCanvasSizeOpen, selectAndMaskOpen, setSelectAndMaskOpen } =
    useDialogState();
  const [modifyDialog, setModifyDialog] = useState<{ open: boolean; kind: ModifyKind }>({
    open: false,
    kind: "expand",
  });
  const openModifyDialog = (kind: ModifyKind) => setModifyDialog({ open: true, kind });
  const [draft, setDraft] = useState<CreateDocumentCommand>(defaultDocumentDraft);
  const documentIo = useDocumentIo({ draft, setDraft });
  const createAdjustmentLayer = useCreateAdjustmentLayer();

  const contentVersion = uiMeta?.contentVersion;

  useAutosave({ engine, contentVersion, enabled: engine.handle !== null });

  const wasCustomShapeActiveRef = useRef(false);
  useEffect(() => {
    const customShapeActive = activeTool === "shape" && shapeSubTool === "custom-shape";
    if (customShapeActive && !wasCustomShapeActiveRef.current) {
      setActiveAuxPanel("shapes");
    }
    wasCustomShapeActiveRef.current = customShapeActive;
  }, [activeTool, shapeSubTool, setActiveAuxPanel]);

  const editingVectorLayerID = uiMeta?.editingVectorLayerId ?? "";
  const editingTextLayerID = uiMeta?.editingTextLayerId ?? "";
  const activeArtboard = uiMeta?.activeLayerId
    ? findLayerMetaInTree(uiMeta.layers, uiMeta.activeLayerId)
    : null;
  const fillSourceName =
    fillSource === "foreground" ? "Color" : fillSource === "background" ? "Background" : "Pattern";
  const fillModeSummary = `${fillSourceName} fill · ${fillContiguous ? "contiguous" : "all matching"} · ${fillSampleMerged ? "sample merged" : "active layer"} · ${fillCreateLayer ? "new layer" : "paint in place"}`;

  useEffect(() => {
    if (activeArtboard?.isArtboard && activeArtboard.artboardBackground) {
      const background = activeArtboard.artboardBackground;
      setArtboardBackground((current) =>
        current.every((value, index) => value === background[index]) ? current : background,
      );
    }
  }, [activeArtboard?.isArtboard, activeArtboard?.artboardBackground, setArtboardBackground]);

  // The canvas-size draft is seeded from the document inside <CanvasSizeDialog/>
  // on open, so opening the dialog only needs to flip the flag.
  const openCanvasSizeDialog = () => setCanvasSizeOpen(true);

  // Seam for the menu/shortcut hooks: document I/O and dialog-draft callbacks
  // that still live in App (use-document-io and the dialog extraction will
  // take these over).
  const menuIO: MenuActionIO = {
    openProjectPicker: documentIo.openProjectPicker,
    saveDocument: documentIo.saveDocument,
    openCanvasSizeDialog,
    openModifyDialog,
    createAdjustmentLayer,
  };

  useAppShortcuts(menuIO);

  const hasDocument = (uiMeta?.documentWidth ?? 0) > 0;

  const historyEntries = uiMeta?.history ?? [];
  const currentHistoryIndex = uiMeta?.currentHistoryIndex ?? 0;
  useEffect(() => {
    if (!cloneStampUseHistorySource) {
      return;
    }
    if (historyEntries.length === 0) {
      setCloneStampUseHistorySource(false);
      setCloneStampHistorySourceIndex(null);
      return;
    }
    if (
      cloneStampHistorySourceIndex !== null &&
      historyEntries.some((entry) => entry.id === cloneStampHistorySourceIndex)
    ) {
      return;
    }
    const fallback =
      historyEntries.find((entry) => entry.id === currentHistoryIndex)?.id ??
      historyEntries[historyEntries.length - 1]?.id ??
      null;
    setCloneStampHistorySourceIndex(fallback);
  }, [
    cloneStampHistorySourceIndex,
    cloneStampUseHistorySource,
    currentHistoryIndex,
    historyEntries,
    setCloneStampHistorySourceIndex,
    setCloneStampUseHistorySource,
  ]);
  useEffect(() => {
    if (historyEntries.length === 0) {
      setHistoryBrushSourceIndex(null);
      return;
    }
    if (
      historyBrushSourceIndex !== null &&
      historyEntries.some((entry) => entry.id === historyBrushSourceIndex)
    ) {
      return;
    }
    const fallback =
      historyEntries.find((entry) => entry.id === currentHistoryIndex)?.id ??
      historyEntries[historyEntries.length - 1]?.id ??
      null;
    setHistoryBrushSourceIndex(fallback);
  }, [currentHistoryIndex, historyBrushSourceIndex, historyEntries, setHistoryBrushSourceIndex]);
  const activeColor = colorPickerTarget === "foreground" ? foregroundColor : backgroundColor;
  const setActiveColor = (next: Rgba) => applyColorToTarget(colorPickerTarget, next);

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#202329_0%,#171a1f_100%)] text-slate-100">
      <input
        ref={documentIo.brushPresetInputRef}
        type="file"
        accept=".abr,.json,application/json"
        className="hidden"
        onChange={documentIo.handleBrushPresetInputChange}
      />
      <input
        ref={documentIo.shapePresetInputRef}
        type="file"
        accept=".csh,.json,application/json"
        className="hidden"
        onChange={documentIo.handleShapePresetInputChange}
      />
      <input
        ref={documentIo.swatchSetInputRef}
        type="file"
        accept=".aco,.json,application/json"
        className="hidden"
        onChange={documentIo.handleSwatchInputChange}
      />
      <input
        ref={documentIo.projectInputRef}
        type="file"
        accept=".agp,.psd,.psb,application/json,image/png,image/jpeg,image/gif,image/webp,image/bmp"
        className="hidden"
        onChange={documentIo.handleProjectInputChange}
      />

      <div className="mx-auto min-h-screen max-w-[1920px] px-0">
        <div className="flex min-h-screen flex-col bg-[#1d2026]">
          <MenuBar io={menuIO} />

          {documentIo.hasAutosave && engine.status === "ready" ? (
            <div className="flex items-center gap-2 border-b border-warning/30 bg-warning/10 px-3 py-1.5 text-[12px] text-warning">
              <span>Unsaved session detected.</span>
              <button
                type="button"
                className="rounded bg-warning px-2 py-0.5 text-background hover:bg-warning/80 focus-visible:outline-none"
                onClick={documentIo.recoverAutosave}
              >
                Restore
              </button>
              <button
                type="button"
                className="rounded px-2 py-0.5 text-warning hover:text-warning/80 focus-visible:outline-none"
                onClick={documentIo.dismissAutosave}
              >
                Discard
              </button>
            </div>
          ) : null}

          <ToolOptions openBrushPresetImport={documentIo.openBrushPresetImport} />

          {editingVectorLayerID ? (
            <div className="editor-chrome flex min-h-[34px] items-center gap-3 border-b border-warning/30 bg-warning/8 px-3 py-1">
              <span className="text-[11px] text-warning">
                Editing shape path — switch tool to commit changes
              </span>
              <button
                type="button"
                className="ml-auto rounded border border-warning/40 px-2 py-0.5 text-[11px] text-warning hover:bg-warning/15 focus-visible:outline-none"
                onClick={() => engine.dispatchCommand(CommandID.CommitVectorEdit, {})}
              >
                Done
              </button>
            </div>
          ) : null}

          {editingTextLayerID ? (
            <TextEditOverlay
              engine={engine}
              initialText={
                findLayerMetaInTree(uiMeta?.layers ?? [], editingTextLayerID)?.text ?? ""
              }
            />
          ) : null}

          <section
            className="grid min-h-0 flex-1"
            style={{
              gridTemplateColumns: `46px minmax(0,1fr) ${panelCollapsed ? "34px" : `${panelWidth}px`}`,
            }}
          >
            <ToolRail />

            <main className="editor-stage flex min-w-0 min-h-[36rem] flex-col p-[var(--ui-gap-2)]">
              <section
                className={`min-h-0 flex-1 pt-[var(--ui-gap-2)]${documentIo.isDragOver && hasDocument ? " ring-2 ring-inset ring-blue-500" : ""}`}
                aria-label="Canvas drop zone"
                onDragOver={hasDocument ? documentIo.handleDragOver : undefined}
                onDragLeave={hasDocument ? documentIo.handleDragLeave : undefined}
                onDrop={hasDocument ? documentIo.handleDrop : undefined}
              >
                {hasDocument ? (
                  <CanvasHost />
                ) : engine.status === "error" ? (
                  <EngineLoadErrorScreen
                    message={engine.error?.message ?? "The Wasm engine could not be initialized."}
                  />
                ) : (
                  <WelcomeScreen
                    isDragOver={documentIo.isDragOver}
                    hasAutosave={documentIo.hasAutosave}
                    onNew={() => setNewDocumentOpen(true)}
                    onOpen={documentIo.openProjectPicker}
                    onResume={documentIo.recoverAutosave}
                    onDragOver={documentIo.handleDragOver}
                    onDragLeave={documentIo.handleDragLeave}
                    onDrop={documentIo.handleDrop}
                  />
                )}
              </section>
            </main>

            <RightDock
              draft={draft}
              openBrushPresetImport={documentIo.openBrushPresetImport}
              openShapePresetImport={documentIo.openShapePresetImport}
              openSwatchImport={documentIo.openSwatchImport}
              exportSwatchSet={documentIo.exportSwatchSet}
            />
          </section>

          <StatusBar documentName={draft.name} />
        </div>
      </div>

      <NewDocumentDialog draft={draft} setDraft={setDraft} />

      <OpenRecentDialog onOpenProject={documentIo.openProjectPicker} />

      <ExportDialog draftName={draft.name} onSave={documentIo.saveDocument} />

      <CanvasSizeDialog draft={draft} />

      <FeatherDialog />

      <ModifyDialog
        open={modifyDialog.open}
        kind={modifyDialog.kind}
        onClose={() => setModifyDialog((d) => ({ ...d, open: false }))}
      />

      <Dialog
        open={fillDialogOpen}
        onClose={() => setFillDialogOpen(false)}
        title="Fill"
        description={fillModeSummary}
        className="max-w-sm"
      >
        <div className="space-y-4">
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
          <ToolNumberField
            label="Tolerance"
            min={0}
            max={255}
            step={1}
            value={fillTolerance}
            onChange={setFillTolerance}
          />
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillContiguous}
                onChange={(e) => setFillContiguous(e.target.checked)}
              />
              Contiguous
            </label>
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillSampleMerged}
                onChange={(e) => setFillSampleMerged(e.target.checked)}
              />
              Sample Merged
            </label>
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillCreateLayer}
                onChange={(e) => setFillCreateLayer(e.target.checked)}
              />
              New Layer
            </label>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setFillDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                const activeLayer = uiMeta?.activeLayerId;
                if (activeLayer) {
                  engine.dispatchCommand(CommandID.Fill, {
                    hasPoint: false,
                    tolerance: fillTolerance,
                    contiguous: fillContiguous,
                    sampleMerged: fillSampleMerged,
                    source: fillSource,
                    color: toMutableRgba(
                      fillSource === "background" ? backgroundColor : foregroundColor,
                    ),
                    createLayer: fillCreateLayer,
                    patternId: fillPatternId,
                    patternScale: 1,
                  } satisfies FillCommand);
                }
                setFillDialogOpen(false);
              }}
            >
              Fill
            </Button>
          </div>
        </div>
      </Dialog>

      <GradientEditorDialog
        open={gradientEditorOpen}
        description="Edit the stop list, alpha, and reusable presets for the current gradient."
        stops={gradientStops}
        onStopsChange={setGradientStops}
        recentColors={recentColors}
        onRecentColorSelect={pushRecentColor}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        onClose={() => setGradientEditorOpen(false)}
      />

      <ThresholdDialog />

      <PosterizeDialog />

      <PhotoFilterDialog />

      <ChannelMixerDialog />

      <SelectiveColorDialog />

      <GradientMapDialog />

      <FilterDialogHost />

      <SelectAndMaskWorkspace
        open={selectAndMaskOpen}
        onClose={() => setSelectAndMaskOpen(false)}
        engine={engine}
        activeLayerId={uiMeta?.activeLayerId ?? null}
      />

      <ColorRangeDialog />

      <SaveSelectionDialog />

      <LoadSelectionDialog />

      <ColorPickerDialog
        open={colorPickerOpen}
        title={colorPickerTarget === "foreground" ? "Foreground Color" : "Background Color"}
        description="Pick a color using RGB or HSB controls. The picker updates the active swatch live."
        color={activeColor}
        onChange={setActiveColor}
        onCommit={() => setColorPickerOpen(false)}
        onClose={() => setColorPickerOpen(false)}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        recentColors={recentColors}
        onRecentColorSelect={setActiveColor}
      />

      <ToastViewport />
    </div>
  );
}
