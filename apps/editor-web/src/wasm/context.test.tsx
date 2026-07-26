import type { RenderResult } from "@agogo/proto";
import { act, render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { subscribeToasts, type ToastOptions } from "@/lib/toast-bus";
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
    pixelFormat: "rgba8-straight",
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
    const before = engineValue?.getSnapshot();
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
    expect(engineValue?.getSnapshot()).toBe(before);
    expect(consumerRenderCount).toBe(rendersBefore);
  });

  it("applies viewport changes from an ack while keeping the uiMeta reference", async () => {
    await mountProvider();
    const before = engineValue?.getSnapshot();

    dispatchResults = [
      makeAck({
        viewport: { ...makeFullRender().viewport, centerX: 42 },
      }),
    ];
    act(() => {
      engineValue?.dispatchCommand(0x0015, { phase: "move" });
    });

    expect(engineValue?.getSnapshot()?.viewport.centerX).toBe(42);
    expect(engineValue?.getSnapshot()?.uiMeta).toBe(before?.uiMeta);
  });

  it("updates cursor/status from an ack without discarding the rest of the uiMeta", async () => {
    await mountProvider();
    const beforeMeta = engineValue?.getSnapshot()?.uiMeta;

    dispatchResults = [makeAck({ cursorType: "grabbing" })];
    act(() => {
      engineValue?.dispatchCommand(0x0015, { phase: "move" });
    });

    const meta = engineValue?.getSnapshot()?.uiMeta;
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
    expect(engineValue?.getSnapshot()?.uiMeta.version).toBe(2);
    expect(engineValue?.getSnapshot()?.uiMeta.statusText).toBe("refreshed");
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

    expect(engineValue?.getSnapshot()).toBe(full);
  });

  // B6 tripwire: the whole point of the flip is that a committed frame must not
  // re-create the useEngine() context value nor re-render pure-context consumers.
  it("keeps the useEngine() context value stable across a full-render commit", async () => {
    await mountProvider();
    const stableValue = engineValue;
    const rendersBefore = consumerRenderCount;

    const full = makeFullRender({
      frameId: 9,
      uiMetaVersion: 9,
      uiMeta: {
        ...makeFullRender().uiMeta,
        version: 9,
        layers: [{ id: "l9" }] as unknown as EngineRenderState["uiMeta"]["layers"],
        statusText: "committed",
      },
    });
    dispatchResults = [full];
    act(() => {
      engineValue?.dispatchCommand(0x0112, { name: "x" });
    });

    // The commit landed (the store advanced)…
    expect(engineValue?.getSnapshot()).toBe(full);
    // …but the context value identity is unchanged and the Consumer, which reads
    // only useEngine(), did NOT re-render. Per-frame fan-out is gone.
    expect(engineValue).toBe(stableValue);
    expect(consumerRenderCount).toBe(rendersBefore);
  });
});

describe("EngineProvider error surfacing", () => {
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

  it("returns null and emits an error toast when dispatchCommand throws", async () => {
    await mountProvider();
    const before = engineValue?.getSnapshot();
    const toasts: ToastOptions[] = [];
    const unsubscribe = subscribeToasts((toast) => toasts.push(toast));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    // The fake handle throws when no dispatch result is queued.
    dispatchResults = [];
    let returned: EngineRenderState | null | undefined = makeFullRender();
    expect(() => {
      act(() => {
        returned = engineValue?.dispatchCommand(0x0401, {});
      });
    }).not.toThrow();

    expect(returned).toBeNull();
    expect(toasts).toHaveLength(1);
    expect(toasts[0]?.kind).toBe("error");
    expect(toasts[0]?.title).toBe("Engine command failed");
    expect(toasts[0]?.message).toContain("no queued dispatch result");
    expect(consoleError).toHaveBeenCalled();
    // The render state stays untouched by the failed command.
    expect(engineValue?.getSnapshot()).toBe(before);

    unsubscribe();
    consoleError.mockRestore();
  });

  it("returns null and emits an error toast when importProject throws", async () => {
    await mountProvider();
    const toasts: ToastOptions[] = [];
    const unsubscribe = subscribeToasts((toast) => toasts.push(toast));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(fakeHandle.importProject).mockImplementationOnce(() => {
      throw new Error("corrupt archive");
    });

    let returned: EngineRenderState | null | undefined;
    expect(() => {
      act(() => {
        returned = engineValue?.importProject("{}");
      });
    }).not.toThrow();

    expect(returned).toBeNull();
    expect(toasts).toHaveLength(1);
    expect(toasts[0]?.kind).toBe("error");
    expect(toasts[0]?.message).toContain("corrupt archive");

    unsubscribe();
    consoleError.mockRestore();
  });

  it("returns null and emits an error toast when exportProject throws", async () => {
    await mountProvider();
    const toasts: ToastOptions[] = [];
    const unsubscribe = subscribeToasts((toast) => toasts.push(toast));
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(fakeHandle.exportProject).mockImplementationOnce(() => {
      throw new Error("zip failed");
    });

    let returned: string | null | undefined;
    expect(() => {
      returned = engineValue?.exportProject();
    }).not.toThrow();

    expect(returned).toBeNull();
    expect(toasts).toHaveLength(1);
    expect(toasts[0]?.kind).toBe("error");
    expect(toasts[0]?.message).toContain("zip failed");

    unsubscribe();
    consoleError.mockRestore();
  });
});
