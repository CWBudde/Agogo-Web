import { CommandID, type CreateDocumentCommand } from "@agogo/proto";
import {
  type ChangeEvent,
  type Dispatch,
  type DragEvent,
  type SetStateAction,
  useRef,
  useState,
} from "react";
import { AUTOSAVE_KEY } from "@/hooks/use-autosave";
import { loadBrushPresetFile, mergeImportedBrushPresets } from "@/lib/brush-preset-io";
import { loadShapePresetFile, mergeImportedShapePresets } from "@/lib/shape-preset-io";
import { exportSwatchesAsAco, loadSwatchSetFile } from "@/lib/swatch-io";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useShapeState } from "@/state/shape-state";
import { useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";

const MAX_SWATCHES = 96;
const PSD_MAX_DIMENSION = 30000;

function fileStem(value: string) {
  const normalized = value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, " ")
    .replace(/\.+$/g, "");
  return normalized || "swatches";
}

interface UseDocumentIoParams {
  /** The App-owned document draft (dialog draft, mirrored from the engine). */
  draft: CreateDocumentCommand;
  setDraft: Dispatch<SetStateAction<CreateDocumentCommand>>;
}

/**
 * Owns all document input/output for the editor: open/import project + image
 * files, save/export in each format, swatch/brush/shape preset import + swatch
 * export, drag-and-drop onto the canvas, and autosave recovery. Also owns the
 * hidden file-input refs and their change handlers; callers render the inputs
 * with the returned refs/handlers.
 *
 * Reads the color/brush/shape/view domain hooks + engine directly; only the
 * document draft (which still lives in App) is passed in.
 */
export function useDocumentIo({ draft, setDraft }: UseDocumentIoParams) {
  const engine = useEngine();
  const render = engine.render;
  const {
    setColorSamplerPoints,
    swatches,
    setSwatches,
    swatchSetName,
    setSwatchSetName,
    setSwatchStatus,
  } = useColorState();
  const { customBrushPresets, setCustomBrushPresets, setBrushPresetStatus, applyBrushPreset } =
    useBrushState();
  const {
    customShapePresets,
    setCustomShapePresets,
    setShapePresetId,
    setShapePresetStatus,
    setShapeSubTool,
  } = useShapeState();
  const { setActiveAuxPanel } = useViewState();

  const projectInputRef = useRef<HTMLInputElement | null>(null);
  const brushPresetInputRef = useRef<HTMLInputElement | null>(null);
  const shapePresetInputRef = useRef<HTMLInputElement | null>(null);
  const swatchSetInputRef = useRef<HTMLInputElement | null>(null);

  const [isDragOver, setIsDragOver] = useState(false);
  const [hasAutosave, setHasAutosave] = useState(() => {
    return localStorage.getItem(AUTOSAVE_KEY) !== null;
  });

  const activeDocumentName = render?.uiMeta.activeDocumentName ?? draft.name;

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const openBrushPresetImport = () => {
    brushPresetInputRef.current?.click();
  };

  const openShapePresetImport = () => {
    shapePresetInputRef.current?.click();
  };

  const openSwatchImport = () => {
    swatchSetInputRef.current?.click();
  };

  const openProjectPicker = () => {
    projectInputRef.current?.click();
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

  const handleDragOver = (event: DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer.types.includes("Files")) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (event: DragEvent) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) {
      setIsDragOver(false);
    }
  };

  const handleDrop = async (event: DragEvent) => {
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

  const handleBrushPresetInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
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
  };

  const handleShapePresetInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
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
  };

  const handleSwatchInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
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
  };

  const handleProjectInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
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
  };

  return {
    // Hidden file-input refs (callers render the inputs).
    projectInputRef,
    brushPresetInputRef,
    shapePresetInputRef,
    swatchSetInputRef,
    // Change handlers for those inputs.
    handleProjectInputChange,
    handleBrushPresetInputChange,
    handleShapePresetInputChange,
    handleSwatchInputChange,
    // Import triggers (click the hidden inputs).
    openProjectPicker,
    openBrushPresetImport,
    openShapePresetImport,
    openSwatchImport,
    // Save / export.
    saveDocument,
    exportSwatchSet,
    // Drag & drop.
    isDragOver,
    handleDragOver,
    handleDragLeave,
    handleDrop,
    // Autosave recovery.
    hasAutosave,
    recoverAutosave,
    dismissAutosave,
  };
}
