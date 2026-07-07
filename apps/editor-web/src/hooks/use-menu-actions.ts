import { type AdjustmentKind, type AdjustmentLayerParams, CommandID } from "@agogo/proto";
import type { MenuActionId, MenuPreviewItem } from "@/components/menu-bar/model";
import { useDialogState } from "@/state/dialog-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";

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
  /** Seed the Save Selection dialog's channel name (dialog draft). */
  setSaveSelectionName: (name: string) => void;
  /** Seed the Load Selection dialog's channel choice (dialog draft). */
  setLoadSelectionName: (name: string) => void;
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
  const render = engine.render;
  const { setActiveTool } = useToolState();
  const { setTransformRefPoint, setTransformSelectionActive } = useSelectionToolState();
  const { showGuides, setShowGuides } = useViewState();
  const { setFillDialogOpen } = useFillGradientState();
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

  const savedSelectionChannels = render?.uiMeta.savedSelectionChannels ?? [];
  const nextSavedSelectionName = () => {
    const existing = new Set(savedSelectionChannels.map((channel) => channel.name.toLowerCase()));
    let index = 1;
    while (existing.has(`alpha ${index}`)) {
      index += 1;
    }
    return `Alpha ${index}`;
  };

  const checkedMenuActionIds = new Set<MenuActionId>(
    showGuides ? (["view-toggle-guides"] as MenuActionId[]) : [],
  );

  const isMenuActionDisabled = (actionId: MenuActionId) => {
    switch (actionId) {
      case "save-project":
      case "save-psd":
      case "save-psb":
      case "export-project":
      case "generate-assets":
      case "canvas-size":
        return !render || actionId === "generate-assets";
      case "image-invert":
      case "image-channel-mixer":
      case "image-threshold":
      case "image-posterize":
      case "image-selective-color":
      case "image-photo-filter":
      case "image-gradient-map":
        return !render?.uiMeta.activeLayerId;
      case "transform-free":
      case "transform-scale":
      case "transform-rotate":
      case "transform-skew":
      case "transform-distort":
      case "transform-perspective":
      case "transform-warp":
        return !render?.uiMeta.activeLayerId;
      case "transform-flip-h":
      case "transform-flip-v":
      case "transform-rotate-cw":
      case "transform-rotate-ccw":
      case "transform-rotate-180":
        return !render?.uiMeta.activeLayerId;
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
        return !render;
      case "select-save-channel":
        return !render?.uiMeta.selection.active;
      case "select-load-channel":
        return !render || savedSelectionChannels.length === 0;
      case "edit-fill":
        return !render?.uiMeta.activeLayerId;
      default:
        return false;
    }
  };

  const isMenuItemDisabled = (item: MenuPreviewItem) => {
    if (item.disabled) {
      return true;
    }
    if (!item.actionId) {
      return true;
    }
    return isMenuActionDisabled(item.actionId);
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
      case "transform-free":
      case "transform-scale":
      case "transform-rotate":
      case "transform-skew":
      case "transform-distort":
      case "transform-perspective":
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
        io.setSaveSelectionName(nextSavedSelectionName());
        setSaveSelectionOpen(true);
        break;
      case "select-load-channel":
        io.setLoadSelectionName(savedSelectionChannels[0]?.name ?? "");
        setLoadSelectionOpen(true);
        break;
      case "select-and-mask":
        setSelectAndMaskOpen(true);
        break;
      case "edit-fill":
        setFillDialogOpen(true);
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
      case "view-toggle-guides": {
        const next = !showGuides;
        setShowGuides(next);
        engine.setShowGuides(next);
        break;
      }
      default:
        break;
    }
  };

  return {
    handleMenuAction,
    isMenuActionDisabled,
    isMenuItemDisabled,
    checkedMenuActionIds,
  };
}
