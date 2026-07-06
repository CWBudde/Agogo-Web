import { CommandID } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EditorCanvas } from "@/components/editor-canvas";
import type { EngineContextValue } from "@/wasm/types";

// EditorCanvas pulls the engine from React context; route useEngine() to a
// per-test mock instead of standing up the whole Wasm provider.
const { engineRef } = vi.hoisted(() => ({
  engineRef: { current: null as unknown as EngineContextValue },
}));

vi.mock("@/wasm/context", () => ({
  useEngine: () => engineRef.current,
}));

const layerMeta = {
  id: "layer-1",
  name: "Layer 1",
  layerType: "pixel",
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
};

const renderState = {
  frameId: 1,
  bufferPtr: 0,
  bufferLen: 0,
  viewport: {
    zoom: 1,
    centerX: 50,
    centerY: 50,
    rotation: 0,
    canvasW: 100,
    canvasH: 100,
    devicePixelRatio: 1,
    showGuides: false,
  },
  uiMeta: {
    documentWidth: 100,
    documentHeight: 100,
    activeLayerId: "layer-1",
    layers: [layerMeta],
    history: [],
    currentHistoryIndex: 0,
  },
} as unknown as EngineContextValue["render"];

function createEngine(): EngineContextValue & {
  beginTransaction: ReturnType<typeof vi.fn>;
  endTransaction: ReturnType<typeof vi.fn>;
  dispatchCommand: ReturnType<typeof vi.fn>;
} {
  return {
    status: "ready",
    handle: null,
    render: renderState,
    error: null,
    ready: null,
    createDocument: vi.fn(() => null),
    createSelection: vi.fn(() => null),
    selectAll: vi.fn(() => null),
    deselect: vi.fn(() => null),
    reselect: vi.fn(() => null),
    invertSelection: vi.fn(() => null),
    magicWand: vi.fn(() => null),
    quickSelect: vi.fn(() => null),
    magneticLassoSuggestPath: vi.fn(() => null),
    pickLayerAtPoint: vi.fn(() => renderState),
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
    dispatchCommand: vi.fn(() => null),
  } as unknown as EngineContextValue & {
    beginTransaction: ReturnType<typeof vi.fn>;
    endTransaction: ReturnType<typeof vi.fn>;
    dispatchCommand: ReturnType<typeof vi.fn>;
  };
}

const defaultProps = {
  activeTool: "move" as const,
  isPanMode: false,
  isZoomTool: false,
  selectionOptions: {
    marqueeShape: "rect" as const,
    marqueeStyle: "normal" as const,
    marqueeRatioW: 1,
    marqueeRatioH: 1,
    marqueeSizeW: 64,
    marqueeSizeH: 64,
    lassoMode: "freehand" as const,
    antiAlias: true,
    featherRadius: 0,
    wandMode: "magic" as const,
    wandTolerance: 32,
    wandContiguous: true,
    wandSampleMerged: false,
  },
  moveAutoSelectGroup: false,
  selectedLayerIds: [] as string[],
  onCursorChange: vi.fn(),
  brushSize: 10,
  brushHardness: 1,
  brushFlow: 1,
  brushOpacity: 1,
  brushBlendMode: "normal",
  brushAirbrush: false,
  brushSmoothing: 0,
  pressureAffectsSize: false,
  pressureAffectsOpacity: false,
  pressureAffectsFlow: false,
  mixerBrushWetness: 0.5,
  mixerBrushLoad: 0.5,
  mixerBrushSampleMerged: false,
  cloneStampOpacity: 1,
  cloneStampLoad: 1,
  cloneStampSampleMerged: false,
  cloneStampAligned: true,
  cloneStampAlignedOffset: null,
  cloneStampSource: null,
  onCloneStampSourceChange: vi.fn(),
  onCloneStampAlignedOffsetChange: vi.fn(),
  cloneStampUseHistorySource: false,
  cloneStampHistorySourceIndex: null,
  historyBrushOpacity: 1,
  historyBrushLoad: 1,
  historyBrushSourceIndex: null,
  historyBrushSourceLabel: null,
  historyBrushSampleMerged: false,
  pencilAutoErase: false,
  eraserMode: "normal" as const,
  eraserTolerance: 32,
  foregroundColor: [0, 0, 0, 255] as const,
  onForegroundColorChange: vi.fn(),
  onBackgroundColorChange: vi.fn(),
  fillSource: "foreground" as const,
  fillPatternId: "",
  fillTolerance: 32,
  fillContiguous: true,
  fillSampleMerged: false,
  fillCreateLayer: false,
  gradientType: "linear" as const,
  gradientReverse: false,
  gradientDither: false,
  gradientCreateLayer: false,
  gradientStops: [],
  eyedropperSampleSize: 1,
  eyedropperSampleMerged: true,
  eyedropperSampleAllLayersNoAdj: false,
  colorSamplerPoints: [],
  onColorSamplerAdd: vi.fn(),
  shapeOptions: {
    subTool: "rect" as const,
    mode: "shape" as const,
    cornerRadius: 0,
    polygonSides: 5,
    polygonInnerRadiusPct: 50,
    starMode: false,
    customPreset: null,
    fillColor: [0, 0, 0, 255] as [number, number, number, number],
    strokeColor: [0, 0, 0, 255] as [number, number, number, number],
    strokeWidth: 1,
  },
  artboardOptions: {
    presetSize: null,
    background: [255, 255, 255, 255] as [number, number, number, number],
  },
  cropDeletePixels: true,
  cropContentAwareFill: false,
  cropResolution: 72,
  cropOverlayType: "thirds" as never,
  cropStraightenActive: false,
  onCropStraightenActiveChange: vi.fn(),
  transformSelectionActive: false,
  onTransformSelectionCommit: vi.fn(),
  onTransformSelectionCancel: vi.fn(),
};

beforeEach(() => {
  // jsdom does not implement pointer capture or ResizeObserver.
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.hasPointerCapture = vi.fn(() => true);
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
});

describe("EditorCanvas gesture cancellation", () => {
  it("aborts a move-layer drag on pointercancel and stays clean afterwards", () => {
    const engine = createEngine();
    engineRef.current = engine;
    render(<EditorCanvas {...defaultProps} />);
    const surface = screen.getByRole("application");

    // pointerdown with the move tool opens a "Move layer" transaction.
    fireEvent.pointerDown(surface, {
      pointerId: 1,
      button: 0,
      buttons: 1,
      clientX: 50,
      clientY: 50,
    });
    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).not.toHaveBeenCalled();

    // The gesture gets interrupted (palm rejection, alt-tab, …): the open
    // transaction must be aborted with commit=false so the engine reverts it.
    fireEvent.pointerCancel(surface, { pointerId: 1 });
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(false);

    // pointercancel implicitly releases capture, which fires
    // lostpointercapture — that must not double-end the transaction.
    fireEvent.lostPointerCapture(surface, { pointerId: 1 });
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);

    // A subsequent pointerdown/pointerup cycle proceeds cleanly. Reusing
    // pointerId 1 (a mouse keeps the same id) exercises the guard-clear
    // branch in onPointerDown — without it a mouse could cancel at most
    // one gesture per session.
    fireEvent.pointerDown(surface, {
      pointerId: 1,
      button: 0,
      buttons: 1,
      clientX: 50,
      clientY: 50,
    });
    expect(engine.beginTransaction).toHaveBeenCalledTimes(2);
    fireEvent.pointerUp(surface, {
      pointerId: 1,
      button: 0,
      clientX: 50,
      clientY: 50,
    });
    expect(engine.endTransaction).toHaveBeenCalledTimes(2);

    // The capture release triggered by pointerup fires lostpointercapture —
    // again, no extra endTransaction.
    fireEvent.lostPointerCapture(surface, { pointerId: 1 });
    expect(engine.endTransaction).toHaveBeenCalledTimes(2);
  });

  it("aborts a move-layer drag when pointer capture is lost without pointerup", () => {
    const engine = createEngine();
    engineRef.current = engine;
    render(<EditorCanvas {...defaultProps} />);
    const surface = screen.getByRole("application");

    fireEvent.pointerDown(surface, {
      pointerId: 7,
      button: 0,
      buttons: 1,
      clientX: 50,
      clientY: 50,
    });
    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);

    // Capture stolen mid-gesture (element swap, programmatic capture change):
    // no pointerup/pointercancel ran first, so this must abort the gesture.
    fireEvent.lostPointerCapture(surface, { pointerId: 7 });
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(false);
  });

  it("ends an active paint stroke as a committed entry on pointercancel", () => {
    const engine = createEngine();
    engineRef.current = engine;
    render(<EditorCanvas {...defaultProps} activeTool="brush" />);
    const surface = screen.getByRole("application");

    fireEvent.pointerDown(surface, {
      pointerId: 3,
      button: 0,
      buttons: 1,
      clientX: 50,
      clientY: 50,
    });
    const beginCalls = engine.dispatchCommand.mock.calls.filter(
      (call) => call[0] === CommandID.BeginPaintStroke,
    );
    expect(beginCalls.length).toBe(1);

    fireEvent.pointerCancel(surface, { pointerId: 3 });
    const endCalls = engine.dispatchCommand.mock.calls.filter(
      (call) => call[0] === CommandID.EndPaintStroke,
    );
    expect(endCalls.length).toBe(1);
    // Paint strokes are committed, not reverted — no transaction involved.
    expect(engine.endTransaction).not.toHaveBeenCalled();
  });
});
