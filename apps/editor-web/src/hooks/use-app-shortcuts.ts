import { CommandID } from "@agogo/proto";
import { useActivateTool } from "@/hooks/use-activate-tool";
import { type ShortcutTool, useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";
import { type MenuActionIO, useMenuActions } from "@/hooks/use-menu-actions";
import { findLayerMetaInTree } from "@/lib/layer-tree";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useDialogState } from "@/state/dialog-state";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";
import { useEngine, useEngineStore } from "@/wasm/context";

/**
 * Headless wiring of the app-wide keyboard shortcuts to their cross-domain
 * actions. Takes the same IO seam as useMenuActions since save/open and the
 * menu disabled-state checks are shared with the menu bar.
 */
export function useAppShortcuts(io: MenuActionIO) {
  const engine = useEngine();
  // Shortcut callbacks read the freshest render snapshot at key-press time via
  // getSnapshot (identity-stable), so this hook never subscribes/re-renders on
  // engine commits.
  const { getSnapshot } = useEngineStore();
  const { isMenuActionDisabled, handleMenuAction } = useMenuActions(io);
  const activateTool = useActivateTool();
  const { setActiveTool } = useToolState();
  const { setTransformRefPoint } = useSelectionToolState();
  const { setIsPanMode } = useViewState();
  const { setNewDocumentOpen, setExportDialogOpen } = useDialogState();
  const { setBrushSize, setBrushHardness } = useBrushState();
  const { foregroundColor, setForegroundColor, backgroundColor, setBackgroundColor } =
    useColorState();

  useKeyboardShortcuts({
    onPanModeChange: setIsPanMode,
    onNewDocument() {
      setNewDocumentOpen(true);
    },
    onOpenDocument() {
      io.openProjectPicker();
    },
    onSaveDocument() {
      if (!isMenuActionDisabled("save-project")) {
        io.saveDocument("archive");
      }
    },
    onExportDocument() {
      if (!isMenuActionDisabled("export-project")) {
        setExportDialogOpen(true);
      }
    },
    onZoomIn() {
      const render = getSnapshot();
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom * 1.1);
    },
    onZoomOut() {
      const render = getSnapshot();
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom / 1.1);
    },
    onFitToView() {
      engine.fitToView();
    },
    onUndo() {
      engine.undo();
    },
    onRedo() {
      engine.redo();
    },
    onFill() {
      handleMenuAction("edit-fill");
    },
    onCanvasSize() {
      handleMenuAction("canvas-size");
    },
    onSelectAll() {
      engine.selectAll();
    },
    onDeselect() {
      engine.deselect();
    },
    onReselect() {
      engine.reselect();
    },
    onInvertSelection() {
      engine.invertSelection();
    },
    onNewLayer() {
      handleMenuAction("layer-new");
    },
    onDuplicateLayer() {
      handleMenuAction("layer-duplicate");
    },
    onMergeDown() {
      handleMenuAction("layer-merge-down");
    },
    onToolSelect(tool: ShortcutTool) {
      activateTool(tool);
    },
    onBeginTransform() {
      setActiveTool("transform");
      setTransformRefPoint([1, 1]);
      engine.dispatchCommand(CommandID.BeginFreeTransform, {});
    },
    onNudgeLayer(dx: number, dy: number) {
      const render = getSnapshot();
      if (!render?.uiMeta.activeLayerId) {
        return;
      }
      const activeLayer = findLayerMetaInTree(render.uiMeta.layers, render.uiMeta.activeLayerId);
      if (!activeLayer || activeLayer.lockMode === "position" || activeLayer.lockMode === "all") {
        return;
      }
      engine.translateLayer({ dx, dy });
    },
    onBrushSizeChange(delta: number) {
      setBrushSize((prev) => {
        const step = prev < 10 ? 1 : prev < 100 ? 5 : 10;
        return Math.max(1, Math.min(2500, prev + delta * step));
      });
    },
    onBrushHardnessChange(delta: number) {
      setBrushHardness((prev) => Math.max(0, Math.min(1, Math.round((prev + delta) * 100) / 100)));
    },
    onSwapColors() {
      setForegroundColor(backgroundColor);
      setBackgroundColor(foregroundColor);
    },
    onResetColors() {
      setForegroundColor([0, 0, 0, 255]);
      setBackgroundColor([255, 255, 255, 255]);
    },
    onReapplyFilter() {
      // handleMenuAction gates on isMenuActionDisabled internally (no last
      // filter / no active layer → no-op).
      handleMenuAction("filter-last");
    },
    onFadeFilter() {
      handleMenuAction("filter-fade");
    },
  });
}
