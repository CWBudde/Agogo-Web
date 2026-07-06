import { CommandID, type RenderResult, type UIMeta, type ViewportMeta } from "@agogo/proto";
import { act, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { EngineProvider, useEngine } from "./context";
import type { EngineContextValue, EngineHandle, EngineRenderState } from "./types";
import { useEngineRender, useHasDocument, useUiMeta, useViewport } from "./use-engine-render";

// Queue of results the fake handle returns from dispatchCommand.
let dispatchResults: RenderResult[] = [];
let renderFrameResult: EngineRenderState;

function makeViewport(overrides: Partial<ViewportMeta> = {}): ViewportMeta {
  return {
    centerX: 0,
    centerY: 0,
    zoom: 1,
    rotation: 0,
    canvasW: 100,
    canvasH: 100,
    devicePixelRatio: 1,
    ...overrides,
  };
}

function makeUiMeta(overrides: Partial<UIMeta> = {}): UIMeta {
  return {
    version: 1,
    cursorType: "default",
    statusText: "ready",
    layers: [],
    history: [],
    documentWidth: 0,
    documentHeight: 0,
    ...overrides,
  } as unknown as UIMeta;
}

function makeFullRender(overrides: Partial<EngineRenderState> = {}): EngineRenderState {
  return {
    frameId: 1,
    viewport: makeViewport(),
    dirtyRects: [],
    pixelFormat: "rgba8-straight",
    bufferPtr: 0,
    bufferLen: 0,
    uiMetaVersion: 1,
    uiMeta: makeUiMeta(),
    ...overrides,
  };
}

function makeAck(overrides: Partial<RenderResult> = {}): RenderResult {
  return {
    frameId: 2,
    viewport: makeViewport(),
    bufferPtr: 0,
    bufferLen: 0,
    uiMetaVersion: 1,
    cursorType: "default",
    statusText: "ready",
    contentVersion: 1,
    ...overrides,
  };
}

const fakeHandle: EngineHandle = {
  handle: 1,
  memory: {} as WebAssembly.Memory,
  dispatchCommand: vi.fn(() => {
    const next = dispatchResults.shift();
    if (!next) {
      throw new Error("no queued dispatch result");
    }
    return next;
  }),
  renderFrame: vi.fn(() => renderFrameResult),
  renderFrameRaw: vi.fn(),
  exportProject: vi.fn(() => ""),
  exportDocument: vi.fn(() => ""),
  importProject: vi.fn(),
  readPixels: vi.fn(),
  free: vi.fn(),
  dispose: vi.fn(),
} as unknown as EngineHandle;

vi.mock("./loader", () => ({
  loadEngine: () => Promise.resolve(fakeHandle),
}));

// --- Probe components -------------------------------------------------------

let engineValue: EngineContextValue | null = null;

function Controller() {
  engineValue = useEngine();
  return null;
}

let layersRenderCount = 0;
let layersValue: UIMeta["layers"] | null = null;

function LayersProbe() {
  layersRenderCount += 1;
  layersValue = useUiMeta((meta) => meta?.layers ?? null);
  return null;
}

let viewportRenderCount = 0;
let viewportValue: ViewportMeta | null = null;

function ViewportProbe() {
  viewportRenderCount += 1;
  viewportValue = useViewport();
  return null;
}

let hasDocRenderCount = 0;
let hasDocValue = false;

function HasDocProbe() {
  hasDocRenderCount += 1;
  hasDocValue = useHasDocument();
  return null;
}

// Selector that returns a FRESH object every call — only safe together with a
// custom isEqual. Guards against the selector-cache infinite-loop failure mode.
let freshObjectRenderCount = 0;
let freshObjectValue: { width: number } | null = null;

function FreshObjectProbe() {
  freshObjectRenderCount += 1;
  freshObjectValue = useUiMeta(
    (meta) => ({ width: meta?.documentWidth ?? 0 }),
    (a, b) => a.width === b.width,
  );
  return null;
}

let rawSnapshotRenderCount = 0;
let rawSnapshotValue: EngineRenderState | null = null;

function RawSnapshotProbe() {
  rawSnapshotRenderCount += 1;
  rawSnapshotValue = useEngineRender((state) => state);
  return null;
}

async function mountAll() {
  render(
    <EngineProvider>
      <Controller />
      <LayersProbe />
      <ViewportProbe />
      <HasDocProbe />
      <FreshObjectProbe />
      <RawSnapshotProbe />
    </EngineProvider>,
  );
  await waitFor(() => {
    expect(engineValue?.status).toBe("ready");
  });
}

function dispatchQueued(commandId: number = CommandID.ContinuePaintStroke, payload: unknown = {}) {
  act(() => {
    engineValue?.dispatchCommand(commandId, payload);
  });
}

beforeEach(() => {
  dispatchResults = [];
  renderFrameResult = makeFullRender();
  engineValue = null;
  layersRenderCount = 0;
  layersValue = null;
  viewportRenderCount = 0;
  viewportValue = null;
  hasDocRenderCount = 0;
  hasDocValue = false;
  freshObjectRenderCount = 0;
  freshObjectValue = null;
  rawSnapshotRenderCount = 0;
  rawSnapshotValue = null;
  vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
    // Never auto-run scheduled refreshes; tests keep uiMetaVersion consistent.
    void cb;
    return 1;
  });
  vi.stubGlobal("cancelAnimationFrame", () => {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("engine render subscription", () => {
  it("exposes subscribe/getSnapshot on the context and reflects the initial render", async () => {
    await mountAll();

    expect(typeof engineValue?.subscribe).toBe("function");
    expect(typeof engineValue?.getSnapshot).toBe("function");
    expect(engineValue?.getSnapshot?.()).toBe(engineValue?.render);
    expect(rawSnapshotValue).toBe(engineValue?.render);
    expect(layersValue).toBe(engineValue?.render?.uiMeta.layers);
    expect(viewportValue).toEqual(makeViewport());
  });

  it("notifies subscribers on a full render commit; layers probe re-renders only on layers identity change", async () => {
    await mountAll();
    const newLayers = [{ id: "l1" }] as unknown as UIMeta["layers"];
    const layersBefore = layersRenderCount;
    const viewportBefore = viewportRenderCount;

    dispatchResults = [
      makeFullRender({
        frameId: 5,
        uiMetaVersion: 2,
        uiMeta: makeUiMeta({ version: 2, layers: newLayers, statusText: "layer added" }),
      }),
    ];
    dispatchQueued();

    expect(layersRenderCount).toBe(layersBefore + 1);
    expect(layersValue).toBe(newLayers);
    // Viewport values unchanged (fresh but equal object) — value-equal
    // selector keeps the viewport probe quiet.
    expect(viewportRenderCount).toBe(viewportBefore);

    // A second full render that PRESERVES the layers array identity must not
    // re-render the layers probe, even though the snapshot changed.
    const layersAfterFirst = layersRenderCount;
    const rawBefore = rawSnapshotRenderCount;
    dispatchResults = [
      makeFullRender({
        frameId: 6,
        uiMetaVersion: 3,
        uiMeta: makeUiMeta({ version: 3, layers: newLayers, statusText: "renamed" }),
      }),
    ];
    dispatchQueued();

    // The raw-snapshot probe proves the notification fired…
    expect(rawSnapshotRenderCount).toBe(rawBefore + 1);
    // …while the layers probe stayed put (same array reference).
    expect(layersRenderCount).toBe(layersAfterFirst);
    expect(layersValue).toBe(newLayers);
  });

  it("notifies viewport subscribers on a hot-path viewport-only ack without touching the layers probe", async () => {
    await mountAll();
    const layersBefore = layersRenderCount;
    const viewportBefore = viewportRenderCount;
    const uiMetaBefore = engineValue?.render?.uiMeta;

    dispatchResults = [makeAck({ viewport: makeViewport({ centerX: 42 }) })];
    dispatchQueued(CommandID.PointerEvent, { phase: "move" });

    expect(viewportRenderCount).toBe(viewportBefore + 1);
    expect(viewportValue?.centerX).toBe(42);
    expect(layersRenderCount).toBe(layersBefore);
    // The ack merge preserved the uiMeta reference.
    expect(engineValue?.render?.uiMeta).toBe(uiMetaBefore);
  });

  it("leaves layers and viewport probes untouched on a cursor/status-only ack", async () => {
    await mountAll();
    const layersBefore = layersRenderCount;
    const viewportBefore = viewportRenderCount;
    const rawBefore = rawSnapshotRenderCount;

    dispatchResults = [makeAck({ cursorType: "grabbing" })];
    dispatchQueued(CommandID.PointerEvent, { phase: "move" });

    // The snapshot did change (new uiMeta object carrying the cursor)…
    expect(rawSnapshotRenderCount).toBe(rawBefore + 1);
    expect(rawSnapshotValue?.uiMeta.cursorType).toBe("grabbing");
    // …but neither the layers array nor the viewport values changed.
    expect(layersRenderCount).toBe(layersBefore);
    expect(viewportRenderCount).toBe(viewportBefore);
  });

  it("does not notify anyone for a no-change ack", async () => {
    await mountAll();
    const layersBefore = layersRenderCount;
    const viewportBefore = viewportRenderCount;
    const rawBefore = rawSnapshotRenderCount;

    dispatchResults = [makeAck()];
    dispatchQueued();

    expect(rawSnapshotRenderCount).toBe(rawBefore);
    expect(layersRenderCount).toBe(layersBefore);
    expect(viewportRenderCount).toBe(viewportBefore);
  });

  it("flips useHasDocument false→true when a document is created", async () => {
    await mountAll();
    expect(hasDocValue).toBe(false);
    const hasDocBefore = hasDocRenderCount;

    dispatchResults = [
      makeFullRender({
        frameId: 7,
        uiMetaVersion: 2,
        uiMeta: makeUiMeta({ version: 2, documentWidth: 800, documentHeight: 600 }),
      }),
    ];
    dispatchQueued();

    expect(hasDocRenderCount).toBe(hasDocBefore + 1);
    expect(hasDocValue).toBe(true);
  });

  it("keeps a stable reference for fresh-object selectors with a custom isEqual (no loops)", async () => {
    await mountAll();
    const firstValue = freshObjectValue;
    expect(firstValue).toEqual({ width: 0 });
    const countAfterMount = freshObjectRenderCount;

    // A cursor-only ack changes the uiMeta identity but not documentWidth:
    // the selector produces a fresh object, isEqual says "same", and the hook
    // must return the SAME reference without re-rendering (and without
    // looping on an ever-changing getSnapshot result).
    dispatchResults = [makeAck({ cursorType: "grabbing" })];
    dispatchQueued(CommandID.PointerEvent, { phase: "move" });

    expect(freshObjectRenderCount).toBe(countAfterMount);
    expect(freshObjectValue).toBe(firstValue);

    // A real width change flows through and produces a new value.
    dispatchResults = [
      makeFullRender({
        frameId: 8,
        uiMetaVersion: 2,
        uiMeta: makeUiMeta({ version: 2, documentWidth: 640 }),
      }),
    ];
    dispatchQueued();

    expect(freshObjectRenderCount).toBe(countAfterMount + 1);
    expect(freshObjectValue).toEqual({ width: 640 });
    expect(freshObjectValue).not.toBe(firstValue);
  });
});
