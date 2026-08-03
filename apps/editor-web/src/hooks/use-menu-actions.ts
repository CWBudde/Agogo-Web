import {
  type AdjustmentKind,
  type AdjustmentLayerParams,
  CommandID,
  type LayerNodeMeta,
} from "@agogo/proto";
import { defaultFilterParams, getFilterDefinition } from "@/components/filters/filter-catalog";
import type { MenuActionId, MenuPreviewItem } from "@/components/menu-bar/model";
import { flushCanvasInput } from "@/lib/canvas-input-flush";
import { findLayerMetaInTree, findLayerPositionInTree } from "@/lib/layer-tree";
import { useDialogState } from "@/state/dialog-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useFilterState } from "@/state/filter-state";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";
import { useEngine, useEngineStore } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

export type DocumentSaveFormat = "archive" | "psd" | "psb";
export type SelectionModifyKind = "expand" | "contract" | "smooth" | "border";

/**
 * Callbacks the menu actions need but that still live in App: document I/O
 * (owned by App until the use-document-io extraction) and dialog draft
 * seeding (owned by App until the dialog extraction). Slot replacements in
 * here as those pieces move out of App.
 */
export interface MenuActionIO {
  /** Click the hidden project file input (document I/O). */
  openProjectPicker: () => void;
  /** Save/export the active document in the given format (document I/O). */
  saveDocument: (format: DocumentSaveFormat) => void;
  /** Seed the canvas-size draft from the document and open the dialog (dialog draft). */
  openCanvasSizeDialog: () => void;
  /** Open the selection Modify dialog for the given operation (dialog draft). */
  openModifyDialog: (kind: SelectionModifyKind) => void;
  /** Insert an adjustment layer above the active layer (App-owned helper). */
  createAdjustmentLayer: <K extends AdjustmentKind>(
    name: string,
    adjustmentKind: K,
    params?: AdjustmentLayerParams<K>,
  ) => void;
}

/**
 * The cross-domain menu action switch: disabled/checked state per action id
 * plus the dispatcher that executes an action. Closing the open menu is the
 * caller's job (the menu-open state lives in <MenuBar/>).
 */
export function useMenuActions(io: MenuActionIO) {
  const engine = useEngine();
  const { getSnapshot } = useEngineStore();
  // Menu disabled/checked state derives from uiMeta; subscribing to the whole
  // uiMeta slice keeps it reactive without re-rendering on viewport-only frames.
  const uiMeta = useUiMeta((meta) => meta);
  const { setActiveTool } = useToolState();
  const { setTransformRefPoint, setTransformSelectionActive } = useSelectionToolState();
  const {
    showGuides,
    setShowGuides,
    panelCollapsed,
    setPanelCollapsed,
    activeAuxPanel,
    setActiveAuxPanel,
  } = useViewState();
  const { setFillDialogOpen } = useFillGradientState();
  const { lastFilter, canFade, openFilter, openFade, noteFilterApplied } = useFilterState();
  const {
    setNewDocumentOpen,
    setOpenRecentOpen,
    setExportDialogOpen,
    setFeatherDialogOpen,
    setColorRangeOpen,
    setSaveSelectionOpen,
    setLoadSelectionOpen,
    setSelectAndMaskOpen,
    setThresholdDialogOpen,
    setPosterizeDialogOpen,
    setChannelMixerDialogOpen,
    setSelectiveColorDialogOpen,
    setPhotoFilterDialogOpen,
    setGradientMapDialogOpen,
  } = useDialogState();

  const savedSelectionChannels = uiMeta?.savedSelectionChannels ?? [];
  const activeLayer = uiMeta?.activeLayerId
    ? findLayerMetaInTree(uiMeta.layers, uiMeta.activeLayerId)
    : null;
  const activeLayerPosition = uiMeta?.activeLayerId
    ? findLayerPositionInTree(uiMeta.layers, uiMeta.activeLayerId)
    : null;

  const checkedMenuActionIds = new Set<MenuActionId>();
  if (showGuides) {
    checkedMenuActionIds.add("view-toggle-guides");
  }
  if (!panelCollapsed) {
    checkedMenuActionIds.add("window-layers");
    if (activeAuxPanel === "navigator") {
      checkedMenuActionIds.add("window-navigator");
    } else if (activeAuxPanel === "history") {
      checkedMenuActionIds.add("window-history");
    }
  }

  const isMenuActionDisabled = (actionId: MenuActionId) => {
    switch (actionId) {
      case "save-project":
      case "save-psd":
      case "save-psb":
      case "export-project":
      case "canvas-size":
      case "layer-new":
      case "layer-new-group":
      case "view-zoom-in":
      case "view-zoom-out":
      case "view-fit-screen":
        return !uiMeta;
      case "edit-undo":
        return !uiMeta?.canUndo;
      case "edit-redo":
        return !uiMeta?.canRedo;
      case "edit-copy":
        return !activeLayer || activeLayer.layerType === "adjustment";
      case "edit-cut":
        return (
          !activeLayer ||
          activeLayer.layerType !== "pixel" ||
          activeLayer.lockMode === "pixels" ||
          activeLayer.lockMode === "all"
        );
      case "edit-paste":
        return !uiMeta?.canPaste;
      case "transform-again":
        return !uiMeta?.canTransformAgain;
      case "image-invert":
      case "image-levels":
      case "image-curves":
      case "image-hue-sat":
      case "image-channel-mixer":
      case "image-threshold":
      case "image-posterize":
      case "image-selective-color":
      case "image-photo-filter":
      case "image-gradient-map":
        return !uiMeta?.activeLayerId;
      case "transform-free":
      case "transform-warp":
        return !uiMeta?.activeLayerId;
      case "transform-flip-h":
      case "transform-flip-v":
      case "transform-rotate-cw":
      case "transform-rotate-ccw":
      case "transform-rotate-180":
        return !uiMeta?.activeLayerId;
      case "layer-add-mask":
        return !activeLayer || activeLayer.hasMask;
      case "layer-duplicate":
        return !activeLayer;
      case "layer-merge-down":
        return !activeLayer || !activeLayerPosition || activeLayerPosition.index === 0;
      case "layer-rasterize":
        return (
          !activeLayer ||
          activeLayer.layerType === "pixel" ||
          activeLayer.layerType === "adjustment"
        );
      case "select-all":
      case "select-deselect":
      case "select-reselect":
      case "select-invert":
      case "select-feather":
      case "select-expand":
      case "select-contract":
      case "select-smooth":
      case "select-border":
      case "select-transform":
      case "select-color-range":
      case "select-and-mask":
        return !uiMeta;
      case "select-save-channel":
        return !uiMeta?.selection.active;
      case "select-load-channel":
        return !uiMeta || savedSelectionChannels.length === 0;
      case "edit-fill":
        return !uiMeta?.activeLayerId;
      case "filter-last":
        return !uiMeta?.activeLayerId || !lastFilter;
      case "filter-fade":
        return !uiMeta?.activeLayerId || !canFade;
      default:
        return false;
    }
  };

  const isMenuItemDisabled = (item: MenuPreviewItem) => {
    if (item.disabled) {
      return true;
    }
    if (item.filterId) {
      // Filters run destructively on the active pixel layer.
      return !uiMeta?.activeLayerId;
    }
    if (!item.actionId) {
      return true;
    }
    return isMenuActionDisabled(item.actionId);
  };

  /** Open a filter dialog, or apply a parameterless filter immediately. */
  const handleFilter = (filterId: string) => {
    if (!uiMeta?.activeLayerId) {
      return;
    }
    const def = getFilterDefinition(filterId);
    if (!def) {
      return;
    }
    if (def.hasDialog) {
      openFilter(def.id);
      return;
    }
    engine.dispatchCommand(CommandID.ApplyFilter, {
      filterId: def.id,
      params: defaultFilterParams(def),
    });
    noteFilterApplied(def.id, def.name);
  };

  const handleMenuAction = (actionId: MenuActionId) => {
    if (isMenuActionDisabled(actionId)) {
      return;
    }

    switch (actionId) {
      case "new-document":
        setNewDocumentOpen(true);
        break;
      case "open-project":
        io.openProjectPicker();
        break;
      case "open-recent":
        setOpenRecentOpen(true);
        break;
      case "save-project":
        io.saveDocument("archive");
        break;
      case "save-psd":
        io.saveDocument("psd");
        break;
      case "save-psb":
        io.saveDocument("psb");
        break;
      case "export-project":
        setExportDialogOpen(true);
        break;
      case "canvas-size":
        io.openCanvasSizeDialog();
        break;
      case "edit-undo":
        engine.undo();
        break;
      case "edit-redo":
        engine.redo();
        break;
      case "edit-copy":
        engine.dispatchCommand(CommandID.Copy, {});
        break;
      case "edit-cut":
        engine.dispatchCommand(CommandID.Cut, {});
        break;
      case "edit-paste":
        engine.dispatchCommand(CommandID.Paste, {});
        break;
      case "transform-again":
        flushCanvasInput();
        engine.dispatchCommand(CommandID.TransformAgain, {});
        break;
      case "transform-free":
        setActiveTool("transform");
        setTransformRefPoint([1, 1]);
        engine.dispatchCommand(CommandID.BeginFreeTransform, {});
        break;
      case "transform-warp":
        setActiveTool("transform");
        setTransformRefPoint([1, 1]);
        engine.dispatchCommand(CommandID.BeginFreeTransform, { mode: "warp" });
        break;
      case "transform-flip-h":
        engine.dispatchCommand(CommandID.FlipLayerH, {});
        break;
      case "transform-flip-v":
        engine.dispatchCommand(CommandID.FlipLayerV, {});
        break;
      case "transform-rotate-cw":
        engine.dispatchCommand(CommandID.RotateLayer90CW, {});
        break;
      case "transform-rotate-ccw":
        engine.dispatchCommand(CommandID.RotateLayer90CCW, {});
        break;
      case "transform-rotate-180":
        engine.dispatchCommand(CommandID.RotateLayer180, {});
        break;
      case "select-all":
        engine.selectAll();
        break;
      case "select-deselect":
        engine.deselect();
        break;
      case "select-reselect":
        engine.reselect();
        break;
      case "select-invert":
        engine.invertSelection();
        break;
      case "select-feather":
        setFeatherDialogOpen(true);
        break;
      case "select-expand":
        io.openModifyDialog("expand");
        break;
      case "select-contract":
        io.openModifyDialog("contract");
        break;
      case "select-smooth":
        io.openModifyDialog("smooth");
        break;
      case "select-border":
        io.openModifyDialog("border");
        break;
      case "select-transform":
        setTransformSelectionActive(true);
        break;
      case "select-color-range":
        setColorRangeOpen(true);
        break;
      case "select-save-channel":
        setSaveSelectionOpen(true);
        break;
      case "select-load-channel":
        setLoadSelectionOpen(true);
        break;
      case "select-and-mask":
        setSelectAndMaskOpen(true);
        break;
      case "edit-fill":
        setFillDialogOpen(true);
        break;
      case "image-levels":
        io.createAdjustmentLayer("Levels", "levels");
        setPanelCollapsed(false);
        setActiveAuxPanel("properties");
        break;
      case "image-curves":
        io.createAdjustmentLayer("Curves", "curves");
        setPanelCollapsed(false);
        setActiveAuxPanel("properties");
        break;
      case "image-hue-sat":
        io.createAdjustmentLayer("Hue/Saturation", "hue-sat");
        setPanelCollapsed(false);
        setActiveAuxPanel("properties");
        break;
      case "image-invert":
        io.createAdjustmentLayer("Invert", "invert");
        break;
      case "image-channel-mixer":
        setChannelMixerDialogOpen(true);
        break;
      case "image-threshold":
        setThresholdDialogOpen(true);
        break;
      case "image-posterize":
        setPosterizeDialogOpen(true);
        break;
      case "image-selective-color":
        setSelectiveColorDialogOpen(true);
        break;
      case "image-photo-filter":
        setPhotoFilterDialogOpen(true);
        break;
      case "image-gradient-map":
        setGradientMapDialogOpen(true);
        break;
      case "layer-new":
        if (uiMeta) {
          engine.dispatchCommand(CommandID.AddLayer, {
            layerType: "pixel",
            name: `Layer ${countLayers(uiMeta.layers) + 1}`,
            bounds: { x: 0, y: 0, w: uiMeta.documentWidth, h: uiMeta.documentHeight },
          });
        }
        break;
      case "layer-new-group":
        if (uiMeta) {
          engine.dispatchCommand(CommandID.AddLayer, {
            layerType: "group",
            name: `Group ${countLayers(uiMeta.layers) + 1}`,
            isolated: true,
          });
        }
        break;
      case "layer-add-mask":
        if (activeLayer) {
          engine.dispatchCommand(CommandID.AddLayerMask, {
            layerId: activeLayer.id,
            mode: "reveal-all",
          });
        }
        break;
      case "layer-duplicate":
        if (activeLayer) {
          engine.dispatchCommand(CommandID.DuplicateLayer, { layerId: activeLayer.id });
        }
        break;
      case "layer-merge-down":
        if (activeLayer) {
          engine.dispatchCommand(CommandID.MergeDown, { layerId: activeLayer.id });
        }
        break;
      case "layer-rasterize":
        if (activeLayer) {
          engine.dispatchCommand(CommandID.FlattenLayer, { layerId: activeLayer.id });
        }
        break;
      case "view-zoom-in": {
        const render = getSnapshot();
        if (render) {
          engine.setZoom(render.viewport.zoom * 1.1);
        }
        break;
      }
      case "view-zoom-out": {
        const render = getSnapshot();
        if (render) {
          engine.setZoom(render.viewport.zoom / 1.1);
        }
        break;
      }
      case "view-fit-screen":
        engine.fitToView();
        break;
      case "view-toggle-guides": {
        const next = !showGuides;
        setShowGuides(next);
        engine.setShowGuides(next);
        break;
      }
      case "window-layers":
        setPanelCollapsed(false);
        break;
      case "window-navigator":
        setPanelCollapsed(false);
        setActiveAuxPanel("navigator");
        break;
      case "window-history":
        setPanelCollapsed(false);
        setActiveAuxPanel("history");
        break;
      case "filter-last":
        if (lastFilter) {
          engine.dispatchCommand(CommandID.ReapplyFilter, {});
          // Reapply re-arms the pre-filter snapshot, so Fade stays available.
          noteFilterApplied(lastFilter.id, lastFilter.name);
        }
        break;
      case "filter-fade":
        openFade();
        break;
      default:
        break;
    }
  };

  return {
    handleMenuAction,
    handleFilter,
    isMenuActionDisabled,
    isMenuItemDisabled,
    checkedMenuActionIds,
  };
}

function countLayers(layers: LayerNodeMeta[]): number {
  return layers.reduce((count, layer) => count + 1 + countLayers(layer.children ?? []), 0);
}
