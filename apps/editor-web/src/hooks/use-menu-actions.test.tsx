import { act, renderHook } from "@testing-library/react";
import { CommandID, type LayerNodeMeta, type UIMeta } from "@agogo/proto";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { type MenuActionIO, useMenuActions } from "@/hooks/use-menu-actions";

const mocks = vi.hoisted(() => ({
  uiMeta: null as UIMeta | null,
  render: null as { viewport: { zoom: number } } | null,
  engine: {
    dispatchCommand: vi.fn(),
    setZoom: vi.fn(),
    fitToView: vi.fn(),
    undo: vi.fn(),
    redo: vi.fn(),
    selectAll: vi.fn(),
    deselect: vi.fn(),
    reselect: vi.fn(),
    invertSelection: vi.fn(),
    setShowGuides: vi.fn(),
  },
  view: {
    showGuides: false,
    setShowGuides: vi.fn(),
    panelCollapsed: false,
    setPanelCollapsed: vi.fn(),
    activeAuxPanel: "properties",
    setActiveAuxPanel: vi.fn(),
  },
  dialogs: {
    setNewDocumentOpen: vi.fn(),
    setOpenRecentOpen: vi.fn(),
    setExportDialogOpen: vi.fn(),
    setFeatherDialogOpen: vi.fn(),
    setColorRangeOpen: vi.fn(),
    setSaveSelectionOpen: vi.fn(),
    setLoadSelectionOpen: vi.fn(),
    setSelectAndMaskOpen: vi.fn(),
    setThresholdDialogOpen: vi.fn(),
    setPosterizeDialogOpen: vi.fn(),
    setChannelMixerDialogOpen: vi.fn(),
    setSelectiveColorDialogOpen: vi.fn(),
    setPhotoFilterDialogOpen: vi.fn(),
    setGradientMapDialogOpen: vi.fn(),
  },
  fill: { setFillDialogOpen: vi.fn() },
  filters: {
    lastFilter: null,
    canFade: false,
    openFilter: vi.fn(),
    openFade: vi.fn(),
    noteFilterApplied: vi.fn(),
  },
  selection: { setTransformRefPoint: vi.fn(), setTransformSelectionActive: vi.fn() },
  tools: { setActiveTool: vi.fn() },
}));

vi.mock("@/wasm/context", () => ({
  useEngine: () => mocks.engine,
  useEngineStore: () => ({ getSnapshot: () => mocks.render }),
}));
vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (selector: (meta: UIMeta | null) => unknown) => selector(mocks.uiMeta),
}));
vi.mock("@/state/view-state", () => ({ useViewState: () => mocks.view }));
vi.mock("@/state/dialog-state", () => ({ useDialogState: () => mocks.dialogs }));
vi.mock("@/state/fill-gradient-state", () => ({ useFillGradientState: () => mocks.fill }));
vi.mock("@/state/filter-state", () => ({ useFilterState: () => mocks.filters }));
vi.mock("@/state/selection-tool-state", () => ({ useSelectionToolState: () => mocks.selection }));
vi.mock("@/state/tool-state", () => ({ useToolState: () => mocks.tools }));

const io: MenuActionIO = {
  openProjectPicker: vi.fn(),
  saveDocument: vi.fn(),
  openCanvasSizeDialog: vi.fn(),
  openModifyDialog: vi.fn(),
  createAdjustmentLayer: vi.fn(),
};

function layer(id: string, layerType: LayerNodeMeta["layerType"], overrides = {}): LayerNodeMeta {
  return {
    id,
    name: id,
    layerType,
    visible: true,
    lockMode: "none",
    opacity: 1,
    fillOpacity: 1,
    blendMode: "normal",
    clipToBelow: false,
    clippingBase: false,
    hasMask: false,
    maskEnabled: true,
    hasVectorMask: false,
    ...overrides,
  };
}

function meta(layers: LayerNodeMeta[], activeLayerId: string | null): UIMeta {
  return {
    version: 1,
    activeLayerId,
    activeLayerName: activeLayerId,
    cursorType: "default",
    statusText: "",
    rulerOriginX: 0,
    rulerOriginY: 0,
    history: [],
    currentHistoryIndex: 0,
    canUndo: true,
    canRedo: true,
    canPaste: false,
    activeDocumentId: "doc-1",
    activeDocumentName: "Document",
    documentWidth: 640,
    documentHeight: 480,
    documentBackground: "transparent",
    layers,
    contentVersion: 1,
    selection: { active: false, pixelCount: 0, lastSelectionAvailable: false },
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.uiMeta = meta([layer("bottom", "pixel"), layer("active", "vector")], "active");
  mocks.render = { viewport: { zoom: 2 } };
  mocks.view.showGuides = false;
  mocks.view.panelCollapsed = false;
  mocks.view.activeAuxPanel = "properties";
});

describe("useMenuActions", () => {
  it("wires history and viewport actions to the engine", () => {
    const { result } = renderHook(() => useMenuActions(io));

    act(() => {
      result.current.handleMenuAction("edit-undo");
      result.current.handleMenuAction("edit-redo");
      result.current.handleMenuAction("view-zoom-in");
      result.current.handleMenuAction("view-zoom-out");
      result.current.handleMenuAction("view-fit-screen");
    });

    expect(mocks.engine.undo).toHaveBeenCalledTimes(1);
    expect(mocks.engine.redo).toHaveBeenCalledTimes(1);
    expect(mocks.engine.setZoom).toHaveBeenNthCalledWith(1, 2.2);
    expect(mocks.engine.setZoom).toHaveBeenNthCalledWith(2, 2 / 1.1);
    expect(mocks.engine.fitToView).toHaveBeenCalledTimes(1);
  });

  it("dispatches clipboard actions and derives their eligibility", () => {
    mocks.uiMeta = {
      ...meta([layer("active", "pixel")], "active"),
      canPaste: true,
    };
    const { result } = renderHook(() => useMenuActions(io));

    expect(result.current.isMenuActionDisabled("edit-cut")).toBe(false);
    expect(result.current.isMenuActionDisabled("edit-copy")).toBe(false);
    expect(result.current.isMenuActionDisabled("edit-paste")).toBe(false);

    act(() => {
      result.current.handleMenuAction("edit-cut");
      result.current.handleMenuAction("edit-copy");
      result.current.handleMenuAction("edit-paste");
    });

    expect(mocks.engine.dispatchCommand).toHaveBeenNthCalledWith(1, CommandID.Cut, {});
    expect(mocks.engine.dispatchCommand).toHaveBeenNthCalledWith(2, CommandID.Copy, {});
    expect(mocks.engine.dispatchCommand).toHaveBeenNthCalledWith(3, CommandID.Paste, {});
  });

  it("disables cut for non-pixel or locked layers and paste for an empty clipboard", () => {
    mocks.uiMeta = meta([layer("active", "vector", { lockMode: "all" })], "active");
    const { result } = renderHook(() => useMenuActions(io));

    expect(result.current.isMenuActionDisabled("edit-cut")).toBe(true);
    expect(result.current.isMenuActionDisabled("edit-copy")).toBe(false);
    expect(result.current.isMenuActionDisabled("edit-paste")).toBe(true);
  });

  it("dispatches the Layer menu through the existing engine commands", () => {
    const { result } = renderHook(() => useMenuActions(io));

    for (const action of [
      "layer-new",
      "layer-new-group",
      "layer-add-mask",
      "layer-duplicate",
      "layer-merge-down",
      "layer-rasterize",
    ] as const) {
      act(() => result.current.handleMenuAction(action));
    }

    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.AddLayer, {
      layerType: "pixel",
      name: "Layer 3",
      bounds: { x: 0, y: 0, w: 640, h: 480 },
    });
    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.AddLayer, {
      layerType: "group",
      name: "Group 3",
      isolated: true,
    });
    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.AddLayerMask, {
      layerId: "active",
      mode: "reveal-all",
    });
    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.DuplicateLayer, {
      layerId: "active",
    });
    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.MergeDown, {
      layerId: "active",
    });
    expect(mocks.engine.dispatchCommand).toHaveBeenCalledWith(CommandID.FlattenLayer, {
      layerId: "active",
    });
  });

  it("applies layer eligibility rules before dispatch", () => {
    mocks.uiMeta = meta([layer("active", "pixel", { hasMask: true })], "active");
    const { result } = renderHook(() => useMenuActions(io));

    expect(result.current.isMenuActionDisabled("layer-add-mask")).toBe(true);
    expect(result.current.isMenuActionDisabled("layer-merge-down")).toBe(true);
    expect(result.current.isMenuActionDisabled("layer-rasterize")).toBe(true);

    act(() => {
      result.current.handleMenuAction("layer-add-mask");
      result.current.handleMenuAction("layer-merge-down");
      result.current.handleMenuAction("layer-rasterize");
    });
    expect(mocks.engine.dispatchCommand).not.toHaveBeenCalled();
  });

  it("creates core adjustment layers and reveals their properties", () => {
    const { result } = renderHook(() => useMenuActions(io));

    act(() => {
      result.current.handleMenuAction("image-levels");
      result.current.handleMenuAction("image-curves");
      result.current.handleMenuAction("image-hue-sat");
    });

    expect(io.createAdjustmentLayer).toHaveBeenNthCalledWith(1, "Levels", "levels");
    expect(io.createAdjustmentLayer).toHaveBeenNthCalledWith(2, "Curves", "curves");
    expect(io.createAdjustmentLayer).toHaveBeenNthCalledWith(3, "Hue/Saturation", "hue-sat");
    expect(mocks.view.setPanelCollapsed).toHaveBeenCalledWith(false);
    expect(mocks.view.setActiveAuxPanel).toHaveBeenCalledWith("properties");
  });

  it("opens existing Window panels and reports their checked state", () => {
    mocks.view.activeAuxPanel = "navigator";
    const { result } = renderHook(() => useMenuActions(io));

    expect(result.current.checkedMenuActionIds).toEqual(
      new Set(["window-layers", "window-navigator"]),
    );

    act(() => result.current.handleMenuAction("window-history"));
    expect(mocks.view.setPanelCollapsed).toHaveBeenCalledWith(false);
    expect(mocks.view.setActiveAuxPanel).toHaveBeenCalledWith("history");
  });
});
