import type { RenderResult } from "@agogo/proto";
import { act, render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EngineProvider, useEngine } from "./context";
import type { EngineContextValue, EngineHandle, EngineRenderState } from "./types";

// Queue of results the fake handle returns from dispatchCommand.
let dispatchResults: RenderResult[] = [];
let renderFrameResult: EngineRenderState;
let renderFrameCalls = 0;

function makeFullRender(overrides: Partial<EngineRenderState> = {}): EngineRenderState {
  return {
    frameId: 1,
    viewport: {
      centerX: 0,
      centerY: 0,
      zoom: 1,
      rotation: 0,
      canvasW: 100,
      canvasH: 100,
      devicePixelRatio: 1,
    },
    dirtyRects: [],
    pixelFormat: "rgba8-premultiplied",
    bufferPtr: 0,
    bufferLen: 0,
    uiMetaVersion: 1,
    uiMeta: {
      version: 1,
      cursorType: "default",
      statusText: "doc 100 x 100",
      layers: [],
      history: [],
    } as unknown as EngineRenderState["uiMeta"],
    ...overrides,
  };
}

function makeAck(overrides: Partial<RenderResult> = {}): RenderResult {
  const full = makeFullRender();
  return {
    frameId: full.frameId,
    viewport: full.viewport,
    bufferPtr: 0,
    bufferLen: 0,
    uiMetaVersion: 1,
    cursorType: "default",
    statusText: "doc 100 x 100",
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
  renderFrame: vi.fn(() => {
    renderFrameCalls += 1;
    return renderFrameResult;
  }),
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

// Deterministic rAF: callbacks queue up and run only via flushRaf().
let rafQueue: FrameRequestCallback[] = [];
function flushRaf() {
  const queue = rafQueue;
  rafQueue = [];
  for (const cb of queue) {
    cb(performance.now());
  }
}

let engineValue: EngineContextValue | null = null;
let consumerRenderCount = 0;

function Consumer() {
  consumerRenderCount += 1;
  engineValue = useEngine();
  return null;
}

async function mountProvider() {
  render(
    <EngineProvider>
      <Consumer />
    </EngineProvider>,
  );
  await waitFor(() => {
    expect(engineValue?.status).toBe("ready");
  });
}

describe("EngineProvider ack merging", () => {
  beforeEach(() => {
    dispatchResults = [];
    renderFrameResult = makeFullRender();
    renderFrameCalls = 0;
    rafQueue = [];
    engineValue = null;
    consumerRenderCount = 0;
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      rafQueue.push(cb);
      return rafQueue.length;
    });
    vi.stubGlobal("cancelAnimationFrame", () => {});
  });

  it("keeps the full uiMeta and skips the React update for a no-change ack", async () => {
    await mountProvider();
    const before = engineValue?.render;
    expect(before?.uiMeta).toBeDefined();
    const rendersBefore = consumerRenderCount;

    dispatchResults = [makeAck()];
    const returned: { value: EngineRenderState | null } = { value: null };
    act(() => {
      returned.value = engineValue?.dispatchCommand(0x0401, {}) ?? null;
    });

    // The merged result still carries the previous full uiMeta (same object)
    // and no re-render happened because nothing visible changed.
    expect(returned.value?.uiMeta).toBe(before?.uiMeta);
    expect(engineValue?.render).toBe(before);
    expect(consumerRenderCount).toBe(rendersBefore);
  });

  it("applies viewport changes from an ack while keeping the uiMeta reference", async () => {
    await mountProvider();
    const before = engineValue?.render;

    dispatchResults = [
      makeAck({
        viewport: { ...makeFullRender().viewport, centerX: 42 },
      }),
    ];
    act(() => {
      engineValue?.dispatchCommand(0x0015, { phase: "move" });
    });

    expect(engineValue?.render?.viewport.centerX).toBe(42);
    expect(engineValue?.render?.uiMeta).toBe(before?.uiMeta);
  });

  it("updates cursor/status from an ack without discarding the rest of the uiMeta", async () => {
    await mountProvider();
    const beforeMeta = engineValue?.render?.uiMeta;

    dispatchResults = [makeAck({ cursorType: "grabbing" })];
    act(() => {
      engineValue?.dispatchCommand(0x0015, { phase: "move" });
    });

    const meta = engineValue?.render?.uiMeta;
    expect(meta?.cursorType).toBe("grabbing");
    expect(meta).not.toBe(beforeMeta);
    expect(meta?.layers).toBe(beforeMeta?.layers);
  });

  it("schedules at most one renderFrame refresh per frame when the ack version is stale", async () => {
    await mountProvider();
    renderFrameCalls = 0;

    renderFrameResult = makeFullRender({
      uiMetaVersion: 2,
      uiMeta: {
        ...makeFullRender().uiMeta,
        version: 2,
        statusText: "refreshed",
      },
    });
    dispatchResults = [makeAck({ uiMetaVersion: 2 }), makeAck({ uiMetaVersion: 2 })];
    act(() => {
      engineValue?.dispatchCommand(0x0401, {});
      engineValue?.dispatchCommand(0x0401, {});
    });
    expect(renderFrameCalls).toBe(0);

    act(() => {
      flushRaf();
    });
    expect(renderFrameCalls).toBe(1);
    expect(engineValue?.render?.uiMeta.version).toBe(2);
    expect(engineValue?.render?.uiMeta.statusText).toBe("refreshed");
  });

  it("dispatches full results directly", async () => {
    await mountProvider();

    const full = makeFullRender({
      uiMetaVersion: 3,
      uiMeta: { ...makeFullRender().uiMeta, version: 3, statusText: "renamed" },
    });
    dispatchResults = [full];
    act(() => {
      engineValue?.dispatchCommand(0x0112, { name: "x" });
    });

    expect(engineValue?.render).toBe(full);
  });
});
