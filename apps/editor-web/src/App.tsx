import {
  type AddLayerCommand,
  type AdjustmentKind,
  type AdjustmentLayerParams,
  CommandID,
  type CreateDocumentCommand,
  type FillCommand,
} from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { ColorPickerDialog } from "@/components/brush-color-panels";
import { CanvasHost } from "@/components/canvas-host";
import { CompactRange } from "@/components/compact-range";
import { EngineLoadErrorScreen } from "@/components/engine-load-error";
import { Field, fieldClassName } from "@/components/field";
import { GradientEditorDialog } from "@/components/gradient-editor";
import { MenuBar } from "@/components/menu-bar/menu-bar";
import { RightDock } from "@/components/right-dock";
import { SelectAndMaskWorkspace } from "@/components/select-and-mask";
import { StatusBar } from "@/components/status-bar";
import { TextEditOverlay } from "@/components/text-edit-overlay";
import { ToolOptions } from "@/components/tool-options";
import { artboardPresetMap } from "@/components/tool-options/artboard-options";
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
import { AUTOSAVE_KEY, useAutosave } from "@/hooks/use-autosave";
import type { MenuActionIO } from "@/hooks/use-menu-actions";
import { loadBrushPresetFile, mergeImportedBrushPresets } from "@/lib/brush-preset-io";
import { hexToRgba, type Rgba, rgbaToCss, rgbaToHex, toMutableRgba } from "@/lib/color";
import { findLayerMetaInTree, findLayerPositionInTree } from "@/lib/layer-tree";
import { loadShapePresetFile, mergeImportedShapePresets } from "@/lib/shape-preset-io";
import { exportSwatchesAsAco, loadSwatchSetFile } from "@/lib/swatch-io";
import {
  type DocumentUnit,
  formatDimension,
  pixelsToUnit,
  unitSteps,
  unitToPixels,
} from "@/lib/units";
import { parseNumericInput } from "@/lib/utils";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useDialogState } from "@/state/dialog-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useShapeState } from "@/state/shape-state";
import { useToolState } from "@/state/tool-state";
import { useCursorState, useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";

const defaultDocumentDraft: CreateDocumentCommand = {
  name: "Untitled",
  width: 1920,
  height: 1080,
  resolution: 72,
  colorMode: "rgb",
  bitDepth: 8,
  background: "transparent",
};

const presets = [
  { id: "web", label: "Web", width: 1920, height: 1080, resolution: 72 },
  { id: "photo", label: "Photo", width: 4032, height: 3024, resolution: 300 },
  { id: "print", label: "Print", width: 2480, height: 3508, resolution: 300 },
  { id: "square", label: "Custom", width: 2048, height: 2048, resolution: 144 },
];

const MAX_SWATCHES = 96;

function fileStem(value: string) {
  const normalized = value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, " ")
    .replace(/\.+$/g, "");
  return normalized || "swatches";
}

export default function App() {
  const engine = useEngine();
  const render = engine.render;
  const projectInputRef = useRef<HTMLInputElement | null>(null);
  const brushPresetInputRef = useRef<HTMLInputElement | null>(null);
  const shapePresetInputRef = useRef<HTMLInputElement | null>(null);
  const swatchSetInputRef = useRef<HTMLInputElement | null>(null);
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
    swatches,
    setSwatches,
    swatchSetName,
    setSwatchSetName,
    setSwatchStatus,
    setColorSamplerPoints,
    pushRecentColor,
    applyColorToTarget,
  } = useColorState();
  const {
    cloneStampAligned,
    cloneStampAlignedOffset,
    cloneStampUseHistorySource,
    setCloneStampUseHistorySource,
    cloneStampHistorySourceIndex,
    setCloneStampHistorySourceIndex,
    cloneStampSource,
    historyBrushSourceIndex,
    setHistoryBrushSourceIndex,
    customBrushPresets,
    setCustomBrushPresets,
    setBrushPresetStatus,
    applyBrushPreset,
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
    gradientReverse,
    setGradientReverse,
    gradientStops,
    setGradientStops,
    gradientEditorOpen,
    setGradientEditorOpen,
  } = useFillGradientState();
  const {
    shapeSubTool,
    setShapeSubTool,
    setShapePresetId,
    customShapePresets,
    setCustomShapePresets,
    setShapePresetStatus,
    artboardPreset,
    setArtboardBackground,
  } = useShapeState();
  const { panelCollapsed, panelWidth, documentUnit, setDocumentUnit, setActiveAuxPanel } =
    useViewState();
  const { cursor } = useCursorState();
  const { activeTool } = useToolState();
  const {
    newDocumentOpen,
    setNewDocumentOpen,
    canvasSizeOpen,
    setCanvasSizeOpen,
    openRecentOpen,
    setOpenRecentOpen,
    exportDialogOpen,
    setExportDialogOpen,
    featherDialogOpen,
    setFeatherDialogOpen,
    colorRangeOpen,
    setColorRangeOpen,
    saveSelectionOpen,
    setSaveSelectionOpen,
    loadSelectionOpen,
    setLoadSelectionOpen,
    selectAndMaskOpen,
    setSelectAndMaskOpen,
    thresholdDialogOpen,
    setThresholdDialogOpen,
    posterizeDialogOpen,
    setPosterizeDialogOpen,
    channelMixerDialogOpen,
    setChannelMixerDialogOpen,
    selectiveColorDialogOpen,
    setSelectiveColorDialogOpen,
    photoFilterDialogOpen,
    setPhotoFilterDialogOpen,
    gradientMapDialogOpen,
    setGradientMapDialogOpen,
  } = useDialogState();
  const [featherDialogValue, setFeatherDialogValue] = useState(5);
  type ModifyKind = "expand" | "contract" | "smooth" | "border";
  const [modifyDialog, setModifyDialog] = useState<{
    open: boolean;
    kind: ModifyKind;
    value: number;
  }>({ open: false, kind: "expand", value: 4 });
  const openModifyDialog = (kind: ModifyKind) =>
    setModifyDialog({ open: true, kind, value: kind === "smooth" ? 2 : 4 });
  const [colorRangeColor, setColorRangeColor] = useState<Rgba>([128, 128, 128, 255]);
  const [colorRangeFuzziness, setColorRangeFuzziness] = useState(40);
  const [colorRangeSampleMerged, setColorRangeSampleMerged] = useState(false);
  const [saveSelectionName, setSaveSelectionName] = useState("Alpha 1");
  const [loadSelectionName, setLoadSelectionName] = useState("");
  const [draft, setDraft] = useState<CreateDocumentCommand>(defaultDocumentDraft);
  const [canvasSizeDraft, setCanvasSizeDraft] = useState<{
    width: number;
    height: number;
    anchor:
      | "top-left"
      | "top-center"
      | "top-right"
      | "middle-left"
      | "center"
      | "middle-right"
      | "bottom-left"
      | "bottom-center"
      | "bottom-right";
  }>({
    width: 0,
    height: 0,
    anchor: "center",
  });
  const [isDragOver, setIsDragOver] = useState(false);
  const [thresholdValue, setThresholdValue] = useState(128);
  const [posterizeLevels, setPosterizeLevels] = useState(6);
  const [channelMixerMonochrome, setChannelMixerMonochrome] = useState(false);
  const [channelMixerMatrix, setChannelMixerMatrix] = useState<{
    red: [number, number, number];
    green: [number, number, number];
    blue: [number, number, number];
  }>({
    red: [100, 0, 0],
    green: [0, 100, 0],
    blue: [0, 0, 100],
  });
  const [selectiveColorMode, setSelectiveColorMode] = useState<"relative" | "absolute">("relative");
  const [selectiveColorAdjustments, setSelectiveColorAdjustments] = useState({
    reds: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    yellows: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    greens: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    cyans: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blues: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    magentas: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    whites: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    neutrals: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blacks: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
  });
  const [photoFilterColor, setPhotoFilterColor] = useState<[number, number, number, number]>([
    255, 190, 120, 255,
  ]);
  const [photoFilterDensity, setPhotoFilterDensity] = useState(40);
  const [photoFilterPreserveLuminosity, setPhotoFilterPreserveLuminosity] = useState(true);
  const [hasAutosave, setHasAutosave] = useState(() => {
    return localStorage.getItem(AUTOSAVE_KEY) !== null;
  });

  const savedSelectionChannels = render?.uiMeta.savedSelectionChannels ?? [];
  const nextSavedSelectionName = () => {
    const existing = new Set(savedSelectionChannels.map((channel) => channel.name.toLowerCase()));
    let index = 1;
    while (existing.has(`alpha ${index}`)) {
      index += 1;
    }
    return `Alpha ${index}`;
  };

  const contentVersion = render?.uiMeta.contentVersion;

  useAutosave({ engine, contentVersion, enabled: engine.handle !== null });

  const wasCustomShapeActiveRef = useRef(false);
  useEffect(() => {
    const customShapeActive = activeTool === "shape" && shapeSubTool === "custom-shape";
    if (customShapeActive && !wasCustomShapeActiveRef.current) {
      setActiveAuxPanel("shapes");
    }
    wasCustomShapeActiveRef.current = customShapeActive;
  }, [activeTool, shapeSubTool, setActiveAuxPanel]);

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const editingVectorLayerID = render?.uiMeta.editingVectorLayerId ?? "";
  const editingTextLayerID = render?.uiMeta.editingTextLayerId ?? "";
  const activeDocumentName = render?.uiMeta.activeDocumentName ?? draft.name;
  const activeArtboard = render?.uiMeta.activeLayerId
    ? findLayerMetaInTree(render.uiMeta.layers, render.uiMeta.activeLayerId)
    : null;
  const fillSourceName =
    fillSource === "foreground" ? "Color" : fillSource === "background" ? "Background" : "Pattern";
  const fillModeSummary = `${fillSourceName} fill · ${fillContiguous ? "contiguous" : "all matching"} · ${fillSampleMerged ? "sample merged" : "active layer"} · ${fillCreateLayer ? "new layer" : "paint in place"}`;
  const _artboardPresetSize =
    artboardPreset === "custom" ? null : artboardPresetMap[artboardPreset];
  const channelMixerRows = [
    { key: "red", label: "Red Output" },
    { key: "green", label: "Green Output" },
    { key: "blue", label: "Blue Output" },
  ] as const;
  const channelMixerColumns = [
    { index: 0, label: "Source Red" },
    { index: 1, label: "Source Green" },
    { index: 2, label: "Source Blue" },
  ] as const;
  const selectiveColorRanges = [
    { key: "reds", label: "Reds" },
    { key: "yellows", label: "Yellows" },
    { key: "greens", label: "Greens" },
    { key: "cyans", label: "Cyans" },
    { key: "blues", label: "Blues" },
    { key: "magentas", label: "Magentas" },
    { key: "whites", label: "Whites" },
    { key: "neutrals", label: "Neutrals" },
    { key: "blacks", label: "Blacks" },
  ] as const;

  const openBrushPresetImport = () => {
    brushPresetInputRef.current?.click();
  };

  const openShapePresetImport = () => {
    shapePresetInputRef.current?.click();
  };

  const openSwatchImport = () => {
    swatchSetInputRef.current?.click();
  };

  const exportSwatchSet = () => {
    if (swatches.length === 0) {
      setSwatchStatus("No swatches to export.");
      return;
    }
    const exportName = fileStem(swatchSetName || activeDocumentName);
    const bytes = exportSwatchesAsAco(swatches);
    downloadBlob(new Blob([bytes], { type: "application/octet-stream" }), `${exportName}.aco`);
    setSwatchStatus(`Saved ${swatches.length} swatches to ${exportName}.aco.`);
  };

  useEffect(() => {
    if (activeArtboard?.isArtboard && activeArtboard.artboardBackground) {
      const background = activeArtboard.artboardBackground;
      setArtboardBackground((current) =>
        current.every((value, index) => value === background[index]) ? current : background,
      );
    }
  }, [activeArtboard?.isArtboard, activeArtboard?.artboardBackground, setArtboardBackground]);

  const selectiveColorFields = [
    { key: "cyanRed", label: "Cyan / Red" },
    { key: "magentaGreen", label: "Magenta / Green" },
    { key: "yellowBlue", label: "Yellow / Blue" },
    { key: "black", label: "Black" },
  ] as const;

  const openProjectPicker = () => {
    projectInputRef.current?.click();
  };

  const createAdjustmentLayer = <K extends AdjustmentKind>(
    name: string,
    adjustmentKind: K,
    params: AdjustmentLayerParams<K> = {} as AdjustmentLayerParams<K>,
  ) => {
    if (!render?.uiMeta.activeLayerId) {
      return;
    }
    const position = findLayerPositionInTree(render.uiMeta.layers, render.uiMeta.activeLayerId);
    if (!position) {
      return;
    }
    engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "adjustment",
      name,
      adjustmentKind,
      params,
      parentLayerId: position.parentId,
      index: position.index + 1,
    } satisfies AddLayerCommand);
  };

  const saveDocument = (format: "archive" | "psd" | "psb") => {
    const documentWidth = render?.uiMeta.documentWidth ?? 0;
    const documentHeight = render?.uiMeta.documentHeight ?? 0;
    const normalizedFormat =
      format === "psd" && (documentWidth > PSD_MAX_DIMENSION || documentHeight > PSD_MAX_DIMENSION)
        ? "psb"
        : format;
    const base64Data =
      normalizedFormat === "archive"
        ? engine.exportProject()
        : engine.exportDocument(normalizedFormat);
    if (!base64Data) {
      return;
    }
    const binary = atob(base64Data);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    const extension = normalizedFormat === "archive" ? "agp" : normalizedFormat;
    const mimeType =
      normalizedFormat === "archive" ? "application/zip" : "image/vnd.adobe.photoshop";
    const fileName = `${activeDocumentName}.${extension}`;
    const blob = new Blob([bytes], { type: mimeType });
    downloadBlob(blob, fileName);
  };

  const openProject = async (file: File) => {
    const buffer = await file.arrayBuffer();
    const bytes = new Uint8Array(buffer);
    let payload: string;
    const isLikelyJSON = (() => {
      for (const value of bytes) {
        if (value === 0x20 || value === 0x09 || value === 0x0a || value === 0x0d) {
          continue;
        }
        return value === 0x7b;
      }
      return false;
    })();
    if (!isLikelyJSON) {
      const chunkSize = 0x8000;
      let binary = "";
      for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
      }
      payload = btoa(binary);
    } else {
      payload = new TextDecoder().decode(bytes);
    }
    setColorSamplerPoints([]);
    const imported = engine.importProject(payload);
    if (imported) {
      setDraft((current) => ({
        ...current,
        name: imported.uiMeta.activeDocumentName || current.name,
        width: imported.uiMeta.documentWidth || current.width,
        height: imported.uiMeta.documentHeight || current.height,
        background: imported.uiMeta.documentBackground as CreateDocumentCommand["background"],
      }));
    }
  };

  const openImageFile = async (file: File) => {
    const bitmap = await createImageBitmap(file);
    const { width, height } = bitmap;
    const canvas = new OffscreenCanvas(width, height);
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.drawImage(bitmap, 0, 0);
    bitmap.close();
    const imageData = ctx.getImageData(0, 0, width, height);
    const data = imageData.data;
    const chunkSize = 0x8000;
    let binary = "";
    for (let i = 0; i < data.length; i += chunkSize) {
      binary += String.fromCharCode(...data.subarray(i, i + chunkSize));
    }
    setColorSamplerPoints([]);
    const result = engine.dispatchCommand(CommandID.OpenImageFile, {
      name: file.name,
      width,
      height,
      pixels: btoa(binary),
    });
    if (result) {
      setDraft((current) => ({
        ...current,
        name: result.uiMeta.activeDocumentName || file.name,
        width,
        height,
      }));
    }
  };

  const recoverAutosave = () => {
    const saved = localStorage.getItem(AUTOSAVE_KEY);
    if (!saved) {
      setHasAutosave(false);
      return;
    }
    setColorSamplerPoints([]);
    const imported = engine.importProject(saved);
    if (imported) {
      setDraft((current) => ({
        ...current,
        name: imported.uiMeta.activeDocumentName || current.name,
        width: imported.uiMeta.documentWidth || current.width,
        height: imported.uiMeta.documentHeight || current.height,
        background: imported.uiMeta.documentBackground as CreateDocumentCommand["background"],
      }));
    }
    localStorage.removeItem(AUTOSAVE_KEY);
    setHasAutosave(false);
  };

  const dismissAutosave = () => {
    localStorage.removeItem(AUTOSAVE_KEY);
    setHasAutosave(false);
  };

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer.types.includes("Files")) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (event: React.DragEvent) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) {
      setIsDragOver(false);
    }
  };

  const handleDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    setIsDragOver(false);
    const file = event.dataTransfer.files[0];
    if (!file) return;
    if (file.name.endsWith(".agp") || file.type === "application/json") {
      await openProject(file);
    } else if (file.type.startsWith("image/")) {
      await openImageFile(file);
    }
  };

  const commitFeather = () => {
    engine.dispatchCommand(CommandID.FeatherSelection, { radius: featherDialogValue });
    setFeatherDialogOpen(false);
  };

  const commitModify = () => {
    const { kind, value } = modifyDialog;
    switch (kind) {
      case "expand":
        engine.dispatchCommand(CommandID.ExpandSelection, { pixels: value });
        break;
      case "contract":
        engine.dispatchCommand(CommandID.ContractSelection, { pixels: value });
        break;
      case "smooth":
        engine.dispatchCommand(CommandID.SmoothSelection, { radius: value });
        break;
      case "border":
        engine.dispatchCommand(CommandID.BorderSelection, { width: value });
        break;
    }
    setModifyDialog((d) => ({ ...d, open: false }));
  };

  const commitColorRange = () => {
    engine.dispatchCommand(CommandID.SelectColorRange, {
      targetColor: toMutableRgba(colorRangeColor),
      fuzziness: colorRangeFuzziness,
      sampleMerged: colorRangeSampleMerged,
      mode: "replace",
    });
    setColorRangeOpen(false);
  };

  const commitSaveSelection = () => {
    engine.dispatchCommand(CommandID.SaveSelectionToChannel, {
      name: saveSelectionName.trim() || nextSavedSelectionName(),
    });
    setSaveSelectionOpen(false);
  };

  const commitLoadSelection = () => {
    if (!loadSelectionName) {
      return;
    }
    engine.dispatchCommand(CommandID.LoadSelectionFromChannel, {
      name: loadSelectionName,
      mode: "replace",
    });
    setLoadSelectionOpen(false);
  };

  const openCanvasSizeDialog = () => {
    setCanvasSizeDraft({
      width: render?.uiMeta.documentWidth ?? draft.width,
      height: render?.uiMeta.documentHeight ?? draft.height,
      anchor: "center",
    });
    setCanvasSizeOpen(true);
  };

  // Seam for the menu/shortcut hooks: document I/O and dialog-draft callbacks
  // that still live in App (use-document-io and the dialog extraction will
  // take these over).
  const menuIO: MenuActionIO = {
    openProjectPicker,
    saveDocument,
    openCanvasSizeDialog,
    openModifyDialog,
    setSaveSelectionName,
    setLoadSelectionName,
    createAdjustmentLayer,
  };

  useAppShortcuts(menuIO);

  const hasDocument = (render?.uiMeta.documentWidth ?? 0) > 0;

  const historyEntries = render?.uiMeta.history ?? [];
  const currentHistoryIndex = render?.uiMeta.currentHistoryIndex ?? 0;
  const _selectedCloneHistoryEntry =
    cloneStampHistorySourceIndex === null
      ? null
      : (historyEntries.find((entry) => entry.id === cloneStampHistorySourceIndex) ?? null);
  const _selectedHistoryBrushEntry =
    historyBrushSourceIndex === null
      ? null
      : (historyEntries.find((entry) => entry.id === historyBrushSourceIndex) ?? null);
  const _cloneStampOffsetDisplay =
    cloneStampSource && cursor
      ? cloneStampAligned && cloneStampAlignedOffset
        ? cloneStampAlignedOffset
        : {
            x: cloneStampSource.x - cursor.x,
            y: cloneStampSource.y - cursor.y,
          }
      : cloneStampAlignedOffset;
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
  const widthValue = formatDimension(
    pixelsToUnit(draft.width, draft.resolution, documentUnit),
    documentUnit,
  );
  const heightValue = formatDimension(
    pixelsToUnit(draft.height, draft.resolution, documentUnit),
    documentUnit,
  );
  const activeColor = colorPickerTarget === "foreground" ? foregroundColor : backgroundColor;
  const setActiveColor = (next: Rgba) => applyColorToTarget(colorPickerTarget, next);

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#202329_0%,#171a1f_100%)] text-slate-100">
      <input
        ref={brushPresetInputRef}
        type="file"
        accept=".abr,.json,application/json"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (!file) {
            return;
          }

          try {
            const { presets, sourceName } = await loadBrushPresetFile(file);
            const mergedPresets = mergeImportedBrushPresets(customBrushPresets, presets);
            const addedCount = mergedPresets.length - customBrushPresets.length;
            if (addedCount === 0) {
              setBrushPresetStatus(`No new presets were added from ${sourceName}.`);
              return;
            }
            const firstNewPreset = mergedPresets[mergedPresets.length - addedCount];
            setCustomBrushPresets(mergedPresets);
            applyBrushPreset(firstNewPreset);
            setBrushPresetStatus(
              `Imported ${addedCount} brush preset${addedCount === 1 ? "" : "s"} from ${sourceName}.`,
            );
          } catch (error) {
            const message = error instanceof Error ? error.message : "Brush preset import failed.";
            setBrushPresetStatus(message);
          }
        }}
      />
      <input
        ref={shapePresetInputRef}
        type="file"
        accept=".csh,.json,application/json"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (!file) {
            return;
          }

          try {
            const { presets, sourceName } = await loadShapePresetFile(file);
            const mergedPresets = mergeImportedShapePresets(customShapePresets, presets);
            const addedCount = mergedPresets.length - customShapePresets.length;
            if (addedCount === 0) {
              setShapePresetStatus(`No new shapes were added from ${sourceName}.`);
              return;
            }
            const firstNewPreset = mergedPresets[mergedPresets.length - addedCount];
            setCustomShapePresets(mergedPresets);
            setShapePresetId(firstNewPreset.id);
            setShapePresetStatus(
              `Imported ${addedCount} custom shape${addedCount === 1 ? "" : "s"} from ${sourceName}.`,
            );
            setActiveAuxPanel("shapes");
            setShapeSubTool("custom-shape");
          } catch (error) {
            const message = error instanceof Error ? error.message : "Shape import failed.";
            setShapePresetStatus(message);
          }
        }}
      />
      <input
        ref={swatchSetInputRef}
        type="file"
        accept=".aco,.json,application/json"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          event.target.value = "";
          if (!file) {
            return;
          }

          try {
            const { name, swatches: importedSwatches } = await loadSwatchSetFile(file);
            setSwatches(importedSwatches.slice(0, MAX_SWATCHES));
            setSwatchSetName(name);
            setSwatchStatus(
              `Loaded ${Math.min(importedSwatches.length, MAX_SWATCHES)} swatches from ${name}.`,
            );
          } catch (error) {
            const message = error instanceof Error ? error.message : "Swatch import failed.";
            setSwatchStatus(message);
          }
        }}
      />
      <input
        ref={projectInputRef}
        type="file"
        accept=".agp,.psd,.psb,application/json,image/png,image/jpeg,image/gif,image/webp,image/bmp"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          if (!file) {
            return;
          }

          const lowerName = file.name.toLowerCase();
          if (
            lowerName.endsWith(".agp") ||
            lowerName.endsWith(".psd") ||
            lowerName.endsWith(".psb") ||
            file.type === "application/json"
          ) {
            await openProject(file);
          } else if (file.type.startsWith("image/")) {
            await openImageFile(file);
          }
          event.target.value = "";
        }}
      />

      <div className="mx-auto min-h-screen max-w-[1920px] px-0">
        <div className="flex min-h-screen flex-col bg-[#1d2026]">
          <MenuBar io={menuIO} />

          {hasAutosave && engine.status === "ready" ? (
            <div className="flex items-center gap-2 border-b border-warning/30 bg-warning/10 px-3 py-1.5 text-[12px] text-warning">
              <span>Unsaved session detected.</span>
              <button
                type="button"
                className="rounded bg-warning px-2 py-0.5 text-background hover:bg-warning/80 focus-visible:outline-none"
                onClick={recoverAutosave}
              >
                Restore
              </button>
              <button
                type="button"
                className="rounded px-2 py-0.5 text-warning hover:text-warning/80 focus-visible:outline-none"
                onClick={dismissAutosave}
              >
                Discard
              </button>
            </div>
          ) : null}

          <ToolOptions openBrushPresetImport={openBrushPresetImport} />

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
                findLayerMetaInTree(render?.uiMeta.layers ?? [], editingTextLayerID)?.text ?? ""
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
                className={`min-h-0 flex-1 pt-[var(--ui-gap-2)]${isDragOver && hasDocument ? " ring-2 ring-inset ring-blue-500" : ""}`}
                aria-label="Canvas drop zone"
                onDragOver={hasDocument ? handleDragOver : undefined}
                onDragLeave={hasDocument ? handleDragLeave : undefined}
                onDrop={hasDocument ? handleDrop : undefined}
              >
                {hasDocument ? (
                  <CanvasHost />
                ) : engine.status === "error" ? (
                  <EngineLoadErrorScreen
                    message={engine.error?.message ?? "The Wasm engine could not be initialized."}
                  />
                ) : (
                  <WelcomeScreen
                    isDragOver={isDragOver}
                    hasAutosave={hasAutosave}
                    onNew={() => setNewDocumentOpen(true)}
                    onOpen={() => projectInputRef.current?.click()}
                    onResume={recoverAutosave}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                  />
                )}
              </section>
            </main>

            <RightDock
              draft={draft}
              openBrushPresetImport={openBrushPresetImport}
              openShapePresetImport={openShapePresetImport}
              openSwatchImport={openSwatchImport}
              exportSwatchSet={exportSwatchSet}
            />
          </section>

          <StatusBar documentName={draft.name} />
        </div>
      </div>

      <Dialog
        open={newDocumentOpen}
        onClose={() => setNewDocumentOpen(false)}
        title="Create Document"
        description="Presets, dimensions, resolution, color mode, bit depth, and background feed the Go engine document manager."
      >
        <div className="grid gap-4 md:grid-cols-[11rem_minmax(0,1fr)]">
          <div className="space-y-[var(--ui-gap-2)]">
            {presets.map((preset) => (
              <button
                key={preset.id}
                type="button"
                className="w-full rounded-[var(--ui-radius-sm)] border border-white/8 bg-panel-soft px-3 py-2 text-left transition hover:border-cyan-400/30 hover:bg-cyan-400/8"
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    width: preset.width,
                    height: preset.height,
                    resolution: preset.resolution,
                  }))
                }
              >
                <div className="text-[12px] font-medium text-slate-100">{preset.label}</div>
                <div className="mt-1 text-[11px] text-slate-400">
                  {preset.width} x {preset.height} · {preset.resolution} DPI
                </div>
              </button>
            ))}
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Name">
              <input
                className={fieldClassName}
                value={draft.name}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label="Background">
              <select
                className={fieldClassName}
                value={draft.background}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    background: event.target.value as CreateDocumentCommand["background"],
                  }))
                }
              >
                <option value="transparent">Transparent</option>
                <option value="white">White</option>
                <option value="color">Color</option>
              </select>
            </Field>
            <Field label="Units">
              <select
                className={fieldClassName}
                value={documentUnit}
                onChange={(event) => setDocumentUnit(event.target.value as DocumentUnit)}
              >
                <option value="px">Pixels</option>
                <option value="in">Inches</option>
                <option value="cm">Centimeters</option>
                <option value="mm">Millimeters</option>
              </select>
            </Field>
            <Field label={`Width (${documentUnit})`}>
              <input
                className={fieldClassName}
                type="number"
                min={documentUnit === "px" ? 1 : 0.01}
                step={unitSteps[documentUnit]}
                value={widthValue}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    width: Math.max(
                      1,
                      Math.round(
                        unitToPixels(
                          parseNumericInput(
                            event.target.value,
                            pixelsToUnit(current.width, current.resolution, documentUnit),
                          ),
                          current.resolution,
                          documentUnit,
                        ),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label={`Height (${documentUnit})`}>
              <input
                className={fieldClassName}
                type="number"
                min={documentUnit === "px" ? 1 : 0.01}
                step={unitSteps[documentUnit]}
                value={heightValue}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    height: Math.max(
                      1,
                      Math.round(
                        unitToPixels(
                          parseNumericInput(
                            event.target.value,
                            pixelsToUnit(current.height, current.resolution, documentUnit),
                          ),
                          current.resolution,
                          documentUnit,
                        ),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label="Resolution (DPI)">
              <input
                className={fieldClassName}
                type="number"
                min={1}
                value={draft.resolution}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    resolution: parseNumericInput(event.target.value, current.resolution),
                  }))
                }
              />
            </Field>
            <Field label="Bit Depth">
              <select
                className={fieldClassName}
                value={draft.bitDepth}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    bitDepth: parseNumericInput(event.target.value, current.bitDepth) as
                      | 8
                      | 16
                      | 32,
                  }))
                }
              >
                <option value={8}>8-bit</option>
                <option value={16}>16-bit</option>
                <option value={32}>32-bit</option>
              </select>
            </Field>
            <Field label="Color Mode">
              <select
                className={fieldClassName}
                value={draft.colorMode}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    colorMode: event.target.value as CreateDocumentCommand["colorMode"],
                  }))
                }
              >
                <option value="rgb">RGB</option>
                <option value="gray">Grayscale</option>
              </select>
            </Field>
          </div>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button variant="ghost" size="sm" onClick={() => setNewDocumentOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              engine.createDocument(draft);
              setColorSamplerPoints([]);
              setNewDocumentOpen(false);
            }}
          >
            Create Document
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={openRecentOpen}
        onClose={() => setOpenRecentOpen(false)}
        title="Open Recent"
        description="The browser build cannot reopen local files automatically yet, so recent documents are informational only for now."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <p>
            Recent document tracking needs a persistent file-access layer. That is not wired into
            the web shell yet.
          </p>
          <p className="text-slate-400">
            Use Open to pick an `.agp`, `.psd`, `.psb`, or legacy JSON project from disk.
          </p>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button variant="ghost" size="sm" onClick={() => setOpenRecentOpen(false)}>
            Close
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setOpenRecentOpen(false);
              openProjectPicker();
            }}
          >
            Open...
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={exportDialogOpen}
        onClose={() => setExportDialogOpen(false)}
        title="Export As"
        description="Choose a layered archive, PSD, or PSB export."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
            <div className="text-[12px] font-medium text-slate-100">Project Archive (.agp)</div>
            <div className="mt-1 text-[12px] text-slate-400">
              Saves the current document state, layer tree, and history as {activeDocumentName}.agp.
            </div>
          </div>
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
            <div className="text-[12px] font-medium text-slate-100">Photoshop Document (.psd)</div>
            <div className="mt-1 text-[12px] text-slate-400">
              Writes a layered PSD with merged image data and embedded Agogo metadata for lossless
              reopen.
            </div>
          </div>
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
            <div className="text-[12px] font-medium text-slate-100">
              Large Document Format (.psb)
            </div>
            <div className="mt-1 text-[12px] text-slate-400">
              Use PSB explicitly for large canvases. PSD exports automatically switch to PSB above
              30,000 px.
            </div>
          </div>
        </div>

        <div className="mt-4 flex flex-wrap justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button variant="ghost" size="sm" onClick={() => setExportDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              saveDocument("archive");
              setExportDialogOpen(false);
            }}
          >
            Export Archive
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              saveDocument("psd");
              setExportDialogOpen(false);
            }}
          >
            Export PSD
          </Button>
          <Button
            size="sm"
            onClick={() => {
              saveDocument("psb");
              setExportDialogOpen(false);
            }}
          >
            Export PSB
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={canvasSizeOpen}
        onClose={() => setCanvasSizeOpen(false)}
        title="Canvas Size"
        description="Resizing the canvas shifts layers relative to the selected anchor."
        className="max-w-md"
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Width">
            <input
              type="number"
              className={fieldClassName}
              value={canvasSizeDraft.width}
              onChange={(e) =>
                setCanvasSizeDraft((c) => ({
                  ...c,
                  width: parseNumericInput(e.target.value, c.width),
                }))
              }
            />
          </Field>
          <Field label="Height">
            <input
              type="number"
              className={fieldClassName}
              value={canvasSizeDraft.height}
              onChange={(e) =>
                setCanvasSizeDraft((c) => ({
                  ...c,
                  height: parseNumericInput(e.target.value, c.height),
                }))
              }
            />
          </Field>
        </div>
        <div className="mt-4">
          <Field label="Anchor">
            <div className="grid grid-cols-3 gap-1 w-24 h-24 mt-1">
              {(
                [
                  "top-left",
                  "top-center",
                  "top-right",
                  "middle-left",
                  "center",
                  "middle-right",
                  "bottom-left",
                  "bottom-center",
                  "bottom-right",
                ] as const
              ).map((a) => (
                <button
                  key={a}
                  type="button"
                  className={[
                    "w-full h-full border transition",
                    canvasSizeDraft.anchor === a
                      ? "border-cyan-400 bg-cyan-400/20"
                      : "border-white/10 bg-black/20 hover:border-white/20",
                  ].join(" ")}
                  onClick={() => setCanvasSizeDraft((c) => ({ ...c, anchor: a }))}
                />
              ))}
            </div>
          </Field>
        </div>
        <div className="mt-6 flex justify-end gap-2">
          <Button variant="secondary" size="sm" onClick={() => setCanvasSizeOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              engine.dispatchCommand(CommandID.ResizeCanvas, canvasSizeDraft);
              setCanvasSizeOpen(false);
            }}
          >
            Resize
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={featherDialogOpen}
        onClose={() => setFeatherDialogOpen(false)}
        title="Feather Selection"
        description="Softens the selection edges by blurring."
        className="max-w-xs"
      >
        <div className="space-y-3">
          <Field label="Feather Radius (px)">
            <input
              type="number"
              className={fieldClassName}
              min={0}
              max={250}
              step={0.5}
              value={featherDialogValue}
              onChange={(e) =>
                setFeatherDialogValue(parseNumericInput(e.target.value, featherDialogValue))
              }
            />
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setFeatherDialogOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitFeather}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={modifyDialog.open}
        onClose={() => setModifyDialog((d) => ({ ...d, open: false }))}
        title={
          {
            expand: "Expand Selection",
            contract: "Contract Selection",
            smooth: "Smooth Selection",
            border: "Border Selection",
          }[modifyDialog.kind]
        }
        description={
          {
            expand: "Grow the selection outward.",
            contract: "Shrink the selection inward.",
            smooth: "Smooth the selection edges.",
            border: "Create a border of the specified width.",
          }[modifyDialog.kind]
        }
        className="max-w-xs"
      >
        <div className="space-y-3">
          <Field
            label={
              {
                expand: "Expand By (px)",
                contract: "Contract By (px)",
                smooth: "Radius (px)",
                border: "Width (px)",
              }[modifyDialog.kind]
            }
          >
            <input
              type="number"
              className={fieldClassName}
              min={1}
              max={500}
              step={1}
              value={modifyDialog.value}
              onChange={(e) =>
                setModifyDialog((d) => ({
                  ...d,
                  value: parseNumericInput(e.target.value, d.value),
                }))
              }
            />
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setModifyDialog((d) => ({ ...d, open: false }))}
            >
              Cancel
            </Button>
            <Button size="sm" onClick={commitModify}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

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
                const activeLayer = render?.uiMeta.activeLayerId;
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

      <Dialog
        open={thresholdDialogOpen}
        onClose={() => setThresholdDialogOpen(false)}
        title="Threshold"
        description="Threshold uses Rec. 601 luminance: pixels at or above the slider become white, below become black."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
            <div className="mt-2 h-5 overflow-hidden rounded border border-white/10 bg-slate-950">
              <div className="flex h-full w-full">
                <div
                  className="h-full bg-black"
                  style={{ width: `${(thresholdValue / 255) * 100}%` }}
                />
                <div className="h-full flex-1 bg-white" />
              </div>
            </div>
            <div
              className="mt-2 h-1 rounded-full bg-gradient-to-r from-black via-slate-500 to-white"
              style={{
                backgroundImage:
                  "linear-gradient(90deg, rgba(0,0,0,1) 0%, rgba(0,0,0,1) 45%, rgba(255,255,255,1) 55%, rgba(255,255,255,1) 100%)",
              }}
            />
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Threshold</span>
              <span className="text-slate-300">{thresholdValue}</span>
            </div>
            <input
              className="h-2 w-full accent-accent focus-visible:outline-none"
              type="range"
              min={0}
              max={255}
              step={1}
              value={thresholdValue}
              onChange={(event) =>
                setThresholdValue(parseNumericInput(event.target.value, thresholdValue))
              }
            />
          </label>
          <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Threshold Value</span>
            <input
              className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
              type="number"
              min={0}
              max={255}
              step={1}
              value={thresholdValue}
              onChange={(event) =>
                setThresholdValue(Math.max(0, Math.min(255, Number(event.target.value) || 0)))
              }
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setThresholdDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Threshold", "threshold", { threshold: thresholdValue });
                setThresholdDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={posterizeDialogOpen}
        onClose={() => setPosterizeDialogOpen(false)}
        title="Posterize"
        description="Posterize reduces each RGB channel to a fixed number of levels. Alpha is preserved."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
            <div
              className="mt-2 h-5 rounded border border-white/10"
              style={{
                backgroundImage:
                  "linear-gradient(90deg, rgb(0,0,0) 0%, rgb(0,0,0) 14%, rgb(85,85,85) 14%, rgb(85,85,85) 28%, rgb(170,170,170) 28%, rgb(170,170,170) 42%, rgb(255,255,255) 42%, rgb(255,255,255) 100%)",
              }}
            />
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Levels</span>
              <span className="text-slate-300">{posterizeLevels}</span>
            </div>
            <input
              className="h-2 w-full accent-accent focus-visible:outline-none"
              type="range"
              min={2}
              max={255}
              step={1}
              value={posterizeLevels}
              onChange={(event) =>
                setPosterizeLevels(parseNumericInput(event.target.value, posterizeLevels))
              }
            />
          </label>
          <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Levels Value</span>
            <input
              className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
              type="number"
              min={2}
              max={255}
              step={1}
              value={posterizeLevels}
              onChange={(event) =>
                setPosterizeLevels(Math.max(2, Math.min(255, Number(event.target.value) || 2)))
              }
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setPosterizeDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Posterize", "posterize", { levels: posterizeLevels });
                setPosterizeDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={photoFilterDialogOpen}
        onClose={() => setPhotoFilterDialogOpen(false)}
        title="Photo Filter"
        description="Simulate a gel filter by blending the image toward a tinted filter color. Preserve luminosity keeps the original brightness."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Filter Color</p>
            <div className="mt-2 flex items-center gap-3">
              <input
                type="color"
                className="h-10 w-14 cursor-pointer rounded border border-white/10 bg-transparent"
                value={rgbaToHex(photoFilterColor)}
                onChange={(event) => {
                  const next = hexToRgba(event.target.value);
                  if (next) {
                    setPhotoFilterColor(toMutableRgba(next));
                  }
                }}
              />
              <div className="min-w-0 flex-1">
                <div
                  className="h-10 rounded border border-white/10"
                  style={{ backgroundColor: rgbaToCss(photoFilterColor) }}
                />
                <div className="mt-1 text-[11px] text-slate-500">
                  {rgbaToHex(photoFilterColor).toUpperCase()}
                </div>
              </div>
            </div>
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Density</span>
              <span className="text-slate-300">{photoFilterDensity}</span>
            </div>
            <input
              className="h-2 w-full accent-accent focus-visible:outline-none"
              type="range"
              min={0}
              max={100}
              step={1}
              value={photoFilterDensity}
              onChange={(event) =>
                setPhotoFilterDensity(parseNumericInput(event.target.value, photoFilterDensity))
              }
            />
          </label>
          <label className="flex items-center gap-2 text-[11px] text-slate-300">
            <input
              type="checkbox"
              checked={photoFilterPreserveLuminosity}
              onChange={(event) => setPhotoFilterPreserveLuminosity(event.target.checked)}
            />
            Preserve luminosity
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setPhotoFilterDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Photo Filter", "photo-filter", {
                  color: photoFilterColor,
                  density: photoFilterDensity,
                  preserveLuminosity: photoFilterPreserveLuminosity,
                });
                setPhotoFilterDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={channelMixerDialogOpen}
        onClose={() => setChannelMixerDialogOpen(false)}
        title="Channel Mixer"
        description="Mix source RGB into each output channel. Monochrome collapses the mixed result to grayscale."
        className="max-w-4xl"
      >
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            {channelMixerRows.map((row) => (
              <div
                key={row.key}
                className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
              >
                <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                  {row.label}
                </p>
                <div className="mt-3 space-y-3">
                  {channelMixerColumns.map((column) => (
                    <CompactRange
                      key={column.index}
                      id={`channel-mixer-${row.key}-${column.index}`}
                      label={column.label}
                      min={-200}
                      max={200}
                      step={1}
                      value={channelMixerMatrix[row.key][column.index]}
                      onChange={(next) =>
                        setChannelMixerMatrix((current) => ({
                          ...current,
                          [row.key]: current[row.key].map((entry, index) =>
                            index === column.index ? next : entry,
                          ),
                        }))
                      }
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
          <label className="flex items-center gap-2 text-[11px] text-slate-300">
            <input
              type="checkbox"
              checked={channelMixerMonochrome}
              onChange={(event) => setChannelMixerMonochrome(event.target.checked)}
            />
            Monochrome output
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setChannelMixerDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Channel Mixer", "channel-mixer", {
                  monochrome: channelMixerMonochrome,
                  red: channelMixerMatrix.red,
                  green: channelMixerMatrix.green,
                  blue: channelMixerMatrix.blue,
                });
                setChannelMixerDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={selectiveColorDialogOpen}
        onClose={() => setSelectiveColorDialogOpen(false)}
        title="Selective Color"
        description="Adjust CMYK-style components inside named color ranges. Relative mode scales the effect by pixel strength; Absolute applies the full offsets."
        className="max-w-6xl"
      >
        <div className="space-y-4">
          <ToolOptionGroup label="Mode">
            <ToolChoiceButton
              active={selectiveColorMode === "relative"}
              onClick={() => setSelectiveColorMode("relative")}
            >
              Relative
            </ToolChoiceButton>
            <ToolChoiceButton
              active={selectiveColorMode === "absolute"}
              onClick={() => setSelectiveColorMode("absolute")}
            >
              Absolute
            </ToolChoiceButton>
          </ToolOptionGroup>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {selectiveColorRanges.map((range) => (
              <div
                key={range.key}
                className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
              >
                <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                  {range.label}
                </p>
                <div className="mt-3 space-y-3">
                  {selectiveColorFields.map((field) => (
                    <CompactRange
                      key={field.key}
                      id={`selective-color-${range.key}-${field.key}`}
                      label={field.label}
                      min={-100}
                      max={100}
                      step={1}
                      value={selectiveColorAdjustments[range.key][field.key]}
                      onChange={(next) =>
                        setSelectiveColorAdjustments((current) => ({
                          ...current,
                          [range.key]: {
                            ...current[range.key],
                            [field.key]: next,
                          },
                        }))
                      }
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setSelectiveColorDialogOpen(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Selective Color", "selective-color", {
                  mode: selectiveColorMode,
                  ...selectiveColorAdjustments,
                });
                setSelectiveColorDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <GradientEditorDialog
        open={gradientMapDialogOpen}
        title="Gradient Map"
        description="Create an adjustment layer that remaps luminance through the current gradient."
        stops={gradientStops}
        onStopsChange={setGradientStops}
        recentColors={recentColors}
        onRecentColorSelect={pushRecentColor}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        reverse={gradientReverse}
        onReverseChange={setGradientReverse}
        primaryActionLabel="Create Adjustment Layer"
        onPrimaryAction={() => {
          createAdjustmentLayer("Gradient Map", "gradient-map", {
            stops: gradientStops,
            reverse: gradientReverse,
          });
          setGradientMapDialogOpen(false);
        }}
        onClose={() => setGradientMapDialogOpen(false)}
      />

      <SelectAndMaskWorkspace
        open={selectAndMaskOpen}
        onClose={() => setSelectAndMaskOpen(false)}
        engine={engine}
        activeLayerId={render?.uiMeta.activeLayerId ?? null}
      />

      <Dialog
        open={colorRangeOpen}
        onClose={() => setColorRangeOpen(false)}
        title="Color Range"
        description="Select pixels by color similarity."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <Field label="Sample Color">
            <input
              type="color"
              className="h-8 w-full cursor-pointer rounded border border-white/10 bg-transparent"
              value={rgbaToHex(colorRangeColor)}
              onChange={(e) => {
                const next = hexToRgba(e.target.value);
                if (next) {
                  setColorRangeColor(next);
                }
              }}
            />
          </Field>
          <Field label={`Fuzziness: ${colorRangeFuzziness}`}>
            <input
              type="range"
              className="w-full accent-accent"
              min={0}
              max={200}
              step={1}
              value={colorRangeFuzziness}
              onChange={(e) =>
                setColorRangeFuzziness(parseNumericInput(e.target.value, colorRangeFuzziness))
              }
            />
          </Field>
          <label className="flex cursor-pointer select-none items-center gap-2 text-xs text-slate-300">
            <input
              type="checkbox"
              checked={colorRangeSampleMerged}
              onChange={(e) => setColorRangeSampleMerged(e.target.checked)}
            />
            Sample all layers
          </label>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setColorRangeOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitColorRange}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={saveSelectionOpen}
        onClose={() => setSaveSelectionOpen(false)}
        title="Save Selection"
        description="Store the current selection as an alpha channel."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <Field label="Channel Name">
            <input
              type="text"
              className={fieldClassName}
              value={saveSelectionName}
              onChange={(e) => setSaveSelectionName(e.target.value)}
            />
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setSaveSelectionOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitSaveSelection}>
              Save
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={loadSelectionOpen}
        onClose={() => setLoadSelectionOpen(false)}
        title="Load Selection"
        description="Restore a saved alpha channel as the current selection."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <Field label="Saved Channel">
            <select
              className={fieldClassName}
              value={loadSelectionName}
              onChange={(e) => setLoadSelectionName(e.target.value)}
            >
              {savedSelectionChannels.map((channel) => (
                <option key={channel.name} value={channel.name}>
                  {channel.name}
                </option>
              ))}
            </select>
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setLoadSelectionOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" disabled={!loadSelectionName} onClick={commitLoadSelection}>
              Load
            </Button>
          </div>
        </div>
      </Dialog>

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

const PSD_MAX_DIMENSION = 30000;
