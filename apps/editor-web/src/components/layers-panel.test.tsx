import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  CommandID,
  type DocumentStylePresetEntry,
  type LayerNodeMeta,
  type ThumbnailEntry,
} from "@agogo/proto";
import { LayerPropertiesDialog, LayersPanel } from "@/components/layers-panel";
import type { EngineContextValue } from "@/wasm/types";

const layerStyleDialogSpy = vi.fn();

vi.mock("@/components/layer-style-dialog", () => ({
  LayerStyleDialog: (props: {
    open: boolean;
    layer: LayerNodeMeta | null;
    presets: DocumentStylePresetEntry[];
    onClose: () => void;
  }) => {
    layerStyleDialogSpy(props);

    if (!props.open) {
      return null;
    }

    return (
      <div data-testid="mock-layer-style-dialog">
        <span>{props.layer?.name ?? "no-layer"}</span>
        <span>{props.presets.map((preset) => preset.name).join(",")}</span>
      </div>
    );
  },
}));

function makeLayer(
  id: string,
  name: string,
  overrides: Partial<LayerNodeMeta> = {},
): LayerNodeMeta {
  return {
    id,
    name,
    layerType: "adjustment",
    adjustmentKind: "levels",
    params: {},
    visible: true,
    lockMode: "none",
    opacity: 1,
    fillOpacity: 1,
    blendMode: "normal",
    clipToBelow: false,
    clippingBase: false,
    hasMask: false,
    maskEnabled: false,
    hasVectorMask: false,
    ...overrides,
  };
}

function createEngine(
  overrides: Partial<EngineContextValue> = {},
): EngineContextValue & { dispatchCommand: ReturnType<typeof vi.fn> } {
  const { dispatchCommand: overrideDispatchCommand, ...restOverrides } = overrides;
  const dispatchCommand = vi.fn(() => null);

  return {
    status: "ready",
    handle: null,
    error: null,
    ready: null,
    subscribe: () => () => {},
    getSnapshot: () => null,
    createDocument: vi.fn(() => null),
    createSelection: vi.fn(() => null),
    selectAll: vi.fn(() => null),
    deselect: vi.fn(() => null),
    reselect: vi.fn(() => null),
    invertSelection: vi.fn(() => null),
    magicWand: vi.fn(() => null),
    quickSelect: vi.fn(() => null),
    magneticLassoSuggestPath: vi.fn(() => null),
    pickLayerAtPoint: vi.fn(() => null),
    translateLayer: vi.fn(() => null),
    transformSelection: vi.fn(() => null),
    resizeViewport: vi.fn(() => null),
    setZoom: vi.fn(() => null),
    setPan: vi.fn(() => null),
    dispatchPointerEvent: vi.fn(() => null),
    beginTransaction: vi.fn(() => null),
    endTransaction: vi.fn(() => null),
    jumpHistory: vi.fn(() => null),
    clearHistory: vi.fn(() => null),
    setRotation: vi.fn(() => null),
    fitToView: vi.fn(() => null),
    setShowGuides: vi.fn(() => null),
    exportProject: vi.fn(() => null),
    importProject: vi.fn(() => null),
    undo: vi.fn(() => null),
    redo: vi.fn(() => null),
    reload: vi.fn(),
    exportDocument: vi.fn(() => null),
    ...restOverrides,
    dispatchCommand: (overrideDispatchCommand ??
      dispatchCommand) as unknown as EngineContextValue["dispatchCommand"] &
      ReturnType<typeof vi.fn>,
  };
}

describe("LayerPropertiesDialog", () => {
  it("syncs its draft state when the selected layer changes", () => {
    const onRename = vi.fn();
    const onColorTag = vi.fn();
    const onClose = vi.fn();

    const { rerender } = render(
      <LayerPropertiesDialog
        layer={makeLayer("layer-1", "Levels 1")}
        colorTag="none"
        onRename={onRename}
        onColorTag={onColorTag}
        onOpenLayerStyle={vi.fn()}
        onClose={onClose}
      />,
    );

    const input = screen.getByLabelText("Name") as HTMLInputElement;
    expect(input.value).toBe("Levels 1");

    fireEvent.change(input, { target: { value: "Edited Name" } });
    expect(input.value).toBe("Edited Name");

    rerender(
      <LayerPropertiesDialog
        layer={makeLayer("layer-2", "Photo Filter 1")}
        colorTag="none"
        onRename={onRename}
        onColorTag={onColorTag}
        onOpenLayerStyle={vi.fn()}
        onClose={onClose}
      />,
    );

    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Photo Filter 1");
  });

  it("offers a layer style entry point", () => {
    const onOpenLayerStyle = vi.fn();

    render(
      <LayerPropertiesDialog
        layer={makeLayer("layer-1", "Levels 1")}
        colorTag="none"
        onRename={vi.fn()}
        onColorTag={vi.fn()}
        onOpenLayerStyle={onOpenLayerStyle}
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Layer Style..." }));

    expect(onOpenLayerStyle).toHaveBeenCalledTimes(1);
  });
});

describe("LayersPanel", () => {
  it("dispatches copy, paste, and clear layer style commands from the context menu", () => {
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1");

    render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    fireEvent.click(screen.getByRole("button", { name: "Copy Layer Style" }));

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    fireEvent.click(screen.getByRole("button", { name: "Paste Layer Style" }));

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    fireEvent.click(screen.getByRole("button", { name: "Clear Layer Style" }));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.CopyLayerStyle, {
      layerId: layer.id,
    });
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.PasteLayerStyle, {
      layerId: layer.id,
    });
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.ClearLayerStyle, {
      layerId: layer.id,
    });
  });

  it("runs a context menu action when clicked with a real pointer and closes the menu", () => {
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1");

    render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    fireEvent.contextMenu(screen.getByRole("treeitem"));

    // A real pointer fires pointerdown on the menu item before the click.
    const menuItem = screen.getByRole("button", { name: "Duplicate Layer" });
    fireEvent.pointerDown(menuItem);
    fireEvent.click(menuItem);

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.DuplicateLayer, {
      layerId: layer.id,
    });
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("closes the context menu on an outside pointerdown without running an action", () => {
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1");

    render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    expect(screen.getByRole("menu")).toBeTruthy();

    const callsBefore = engine.dispatchCommand.mock.calls.length;
    fireEvent.pointerDown(document.body);

    expect(screen.queryByRole("menu")).toBeNull();
    expect(engine.dispatchCommand.mock.calls.length).toBe(callsBefore);
  });

  it("closes the context menu on Escape", () => {
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1");

    render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    expect(screen.getByRole("menu")).toBeTruthy();

    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("wraps an opacity slider drag in a single history transaction and dispatches the last value", () => {
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1");

    const { container } = render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    // First range input is the Opacity slider, second is Fill.
    const opacityRange = container.querySelector('input[type="range"]') as HTMLInputElement;
    expect(opacityRange).toBeTruthy();

    fireEvent.change(opacityRange, { target: { value: "80" } });
    fireEvent.change(opacityRange, { target: { value: "60" } });
    fireEvent.change(opacityRange, { target: { value: "40" } });
    fireEvent.pointerUp(opacityRange);

    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.beginTransaction).toHaveBeenCalledWith("Change layer opacity");
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(true);

    const opacityCalls = engine.dispatchCommand.mock.calls.filter(
      ([commandId]) => commandId === CommandID.SetLayerOpacity,
    );
    expect(opacityCalls.length).toBeGreaterThanOrEqual(1);
    expect(opacityCalls[opacityCalls.length - 1][1]).toEqual({
      layerId: layer.id,
      opacity: 0.4,
    });
  });

  it("does not re-render layer rows when unrelated props change and layer node identities are preserved", () => {
    const engine = createEngine();
    const layerOne = makeLayer("layer-1", "Levels 1");
    const layerTwo = makeLayer("layer-2", "Curves 1");
    const selectedLayerIds = [layerOne.id];
    const onSelectedLayerIdsChange = vi.fn();

    // Each layer row resolves `thumbnails[layer.id]` exactly once while it
    // renders. A getter-backed thumbnails object (kept at a stable reference)
    // therefore counts how often each row actually re-runs.
    const rowRenderCounts: Record<string, number> = { "layer-1": 0, "layer-2": 0 };
    const thumbnails = {} as Record<string, ThumbnailEntry>;
    for (const id of Object.keys(rowRenderCounts)) {
      Object.defineProperty(thumbnails, id, {
        configurable: true,
        enumerable: true,
        get() {
          rowRenderCounts[id] += 1;
          return undefined;
        },
      });
    }

    const renderPanel = (layers: LayerNodeMeta[], documentWidth: number) => (
      <LayersPanel
        engine={engine}
        layers={layers}
        activeLayerId={layerOne.id}
        maskEditLayerId={null}
        documentWidth={documentWidth}
        documentHeight={480}
        thumbnails={thumbnails}
        selectedLayerIds={selectedLayerIds}
        onSelectedLayerIdsChange={onSelectedLayerIdsChange}
      />
    );

    const { rerender } = render(renderPanel([layerOne, layerTwo], 640));

    const countsAfterInitial = { ...rowRenderCounts };
    expect(countsAfterInitial["layer-1"]).toBeGreaterThan(0);
    expect(countsAfterInitial["layer-2"]).toBeGreaterThan(0);

    // Simulate an engine viewport-only commit: a brand-new top-level layers
    // array plus a changed unrelated prop (documentWidth), but the same layer
    // node object identities. Memoized rows must bail out.
    rerender(renderPanel([layerOne, layerTwo], 800));

    expect(rowRenderCounts).toEqual(countsAfterInitial);
  });

  it("opens the layer style dialog from layer properties and passes the selected layer metadata and presets", () => {
    layerStyleDialogSpy.mockClear();

    const presets: DocumentStylePresetEntry[] = [
      {
        id: "preset-1",
        name: "Soft Shadow",
        styles: [],
      },
    ];
    const engine = createEngine();
    const layer = makeLayer("layer-1", "Levels 1", {
      styleStack: [
        {
          kind: "stroke",
          enabled: true,
          params: { size: 4 },
        },
      ],
    });

    render(
      <LayersPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        maskEditLayerId={null}
        documentWidth={640}
        documentHeight={480}
        stylePresets={presets}
        thumbnails={{}}
        selectedLayerIds={[layer.id]}
        onSelectedLayerIdsChange={vi.fn()}
      />,
    );

    fireEvent.contextMenu(screen.getByRole("treeitem"));
    fireEvent.click(screen.getByRole("button", { name: "Layer Properties..." }));
    fireEvent.click(screen.getByRole("button", { name: "Layer Style..." }));

    expect(screen.getByTestId("mock-layer-style-dialog")).toBeTruthy();
    expect(layerStyleDialogSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        open: true,
        layer: expect.objectContaining({
          id: layer.id,
          name: layer.name,
          styleStack: layer.styleStack,
        }),
        presets,
      }),
    );
  });
});
