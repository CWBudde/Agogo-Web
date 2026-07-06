import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * Tests for the module-level runtime cache in loader.ts.
 *
 * React StrictMode mounts EngineProvider twice, so loadEngine() runs twice.
 * The Go runtime registers its exports (EngineInit, DispatchCommand, ...) on
 * window; starting a second runtime overwrites them last-writer-wins and
 * leaks the first wasm instance. The loader must therefore create the Go
 * runtime exactly once, capture the registered functions immediately, and
 * hand out per-call handles from that single shared runtime.
 *
 * The module-level promise cache is reset between tests via vi.resetModules()
 * and a dynamic import of ./loader.
 */

type LoaderModule = typeof import("./loader");

// Seam stubs. window.Go is predefined so ensureGoRuntime resolves without
// injecting a <script>; fetch + WebAssembly.instantiate* are stubbed so
// instantiateWasm produces a fake instance whose exports carry real memory.
let goConstructorCalls = 0;
let goRun: ReturnType<typeof vi.fn>;
let rejectGoRun: (error: unknown) => void;
let engineInit: ReturnType<typeof vi.fn>;
let engineFree: ReturnType<typeof vi.fn>;
let fetchMock: ReturnType<typeof vi.fn>;
let instantiateSpy: ReturnType<typeof vi.spyOn>;
let fetchShouldFail: boolean;
let nextHandle: number;
let memory: WebAssembly.Memory;

function makeAbiStubs() {
  const renderJSON = JSON.stringify({
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
    bufferPtr: 0,
    bufferLen: 0,
    uiMetaVersion: 1,
  });

  window.EngineInit = engineInit as unknown as Window["EngineInit"];
  window.DispatchCommand = vi.fn(() => renderJSON);
  window.RenderFrame = vi.fn(() => renderJSON);
  window.RenderFrameRaw = vi.fn(() => renderJSON);
  window.ExportProject = vi.fn(() => "{}");
  window.ExportDocument = vi.fn(() => "{}");
  window.ImportProject = vi.fn(() => renderJSON);
  window.Free = vi.fn();
  window.EngineFree = engineFree as unknown as Window["EngineFree"];
}

async function importLoader(): Promise<LoaderModule> {
  return await import("./loader");
}

beforeEach(() => {
  vi.resetModules();

  goConstructorCalls = 0;
  fetchShouldFail = false;
  nextHandle = 1;
  memory = new WebAssembly.Memory({ initial: 1 });

  // Like the real wasm_exec.js, run() returns a promise that stays pending
  // while the Go program is alive and settles only when it exits or panics.
  goRun = vi.fn(
    () =>
      new Promise<void>((_resolve, reject) => {
        rejectGoRun = reject;
      }),
  );
  engineInit = vi.fn(() => nextHandle++);
  engineFree = vi.fn();

  class FakeGo {
    importObject: WebAssembly.Imports = {};
    run = goRun;
    constructor() {
      goConstructorCalls++;
    }
  }
  window.Go = FakeGo as unknown as Window["Go"];

  makeAbiStubs();

  fetchMock = vi.fn(async () => {
    if (fetchShouldFail) {
      return {
        ok: false,
        status: 500,
        statusText: "Internal Server Error",
      } as Response;
    }
    const response = {
      ok: true,
      status: 200,
      statusText: "OK",
      clone() {
        return response;
      },
      arrayBuffer: async () => new ArrayBuffer(8),
    };
    return response as unknown as Response;
  });
  vi.stubGlobal("fetch", fetchMock);

  // Force the non-streaming fallback path and return a fake instance.
  vi.spyOn(WebAssembly, "instantiateStreaming").mockRejectedValue(
    new Error("streaming disabled in tests"),
  );
  instantiateSpy = vi.spyOn(WebAssembly, "instantiate").mockImplementation(
    async () =>
      ({
        instance: { exports: { memory } },
        module: {},
      }) as unknown as WebAssembly.WebAssemblyInstantiatedSource,
  ) as unknown as ReturnType<typeof vi.spyOn>;
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("loadEngine runtime sharing", () => {
  it("creates exactly one Go runtime for two concurrent loadEngine calls", async () => {
    const { loadEngine } = await importLoader();

    const [a, b] = await Promise.all([
      loadEngine({ config: { documentWidth: 640 } }),
      loadEngine({ config: { documentWidth: 320 } }),
    ]);

    // One runtime: one Go instance, one wasm instantiation, one go.run.
    expect(goConstructorCalls).toBe(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(instantiateSpy).toHaveBeenCalledTimes(1);
    expect(goRun).toHaveBeenCalledTimes(1);

    // Two logical engines: two EngineInit calls with distinct handles.
    expect(engineInit).toHaveBeenCalledTimes(2);
    expect(engineInit).toHaveBeenCalledWith(JSON.stringify({ documentWidth: 640 }));
    expect(engineInit).toHaveBeenCalledWith(JSON.stringify({ documentWidth: 320 }));
    expect(a.handle).not.toBe(b.handle);
    expect(a.memory).toBe(memory);
    expect(b.memory).toBe(memory);
  });

  it("reuses the runtime for sequential loadEngine calls", async () => {
    const { loadEngine } = await importLoader();

    const a = await loadEngine();
    const b = await loadEngine();

    expect(goConstructorCalls).toBe(1);
    expect(goRun).toHaveBeenCalledTimes(1);
    expect(instantiateSpy).toHaveBeenCalledTimes(1);
    expect(engineInit).toHaveBeenCalledTimes(2);
    expect(a.handle).not.toBe(b.handle);
  });

  it("dispose frees only its own handle using the captured EngineFree", async () => {
    const { loadEngine } = await importLoader();

    const a = await loadEngine();
    const b = await loadEngine();

    // Simulate a second runtime overwriting the window globals after load:
    // dispose must still hit the runtime that created the handle.
    const imposter = vi.fn();
    window.EngineFree = imposter;

    a.dispose();

    expect(engineFree).toHaveBeenCalledTimes(1);
    expect(engineFree).toHaveBeenCalledWith(a.handle);
    expect(engineFree).not.toHaveBeenCalledWith(b.handle);
    expect(imposter).not.toHaveBeenCalled();
  });

  it("resets the cached runtime when loading fails so a retry can succeed", async () => {
    const { loadEngine, WasmEngineLoadError } = await importLoader();

    fetchShouldFail = true;
    await expect(loadEngine()).rejects.toBeInstanceOf(WasmEngineLoadError);
    expect(engineInit).not.toHaveBeenCalled();

    fetchShouldFail = false;
    const handle = await loadEngine();

    expect(handle.handle).toBe(1);
    expect(goRun).toHaveBeenCalledTimes(1);
    expect(engineInit).toHaveBeenCalledTimes(1);
  });

  it("keeps the runtime cache when EngineInit returns a non-numeric handle", async () => {
    const { loadEngine, WasmEngineLoadError } = await importLoader();

    const a = await loadEngine();

    engineInit.mockReturnValueOnce("not-a-handle");
    await expect(loadEngine()).rejects.toBeInstanceOf(WasmEngineLoadError);

    // The runtime is healthy — only handle creation failed. The cache must
    // be retained: no second runtime, and the next load succeeds.
    const b = await loadEngine();

    expect(goConstructorCalls).toBe(1);
    expect(goRun).toHaveBeenCalledTimes(1);
    expect(instantiateSpy).toHaveBeenCalledTimes(1);
    expect(engineInit).toHaveBeenCalledTimes(3);
    expect(b.handle).not.toBe(a.handle);
  });

  it("wraps a thrown EngineInit failure in WasmEngineLoadError and keeps the cache", async () => {
    const { loadEngine, WasmEngineLoadError } = await importLoader();

    await loadEngine();

    engineInit.mockImplementationOnce(() => {
      throw new Error("wasm trap");
    });
    await expect(loadEngine()).rejects.toBeInstanceOf(WasmEngineLoadError);

    const b = await loadEngine();

    expect(goConstructorCalls).toBe(1);
    expect(goRun).toHaveBeenCalledTimes(1);
    expect(b.handle).toBeTypeOf("number");
  });

  it("clears the cached runtime when the Go program crashes so it can be recreated", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const { loadEngine } = await importLoader();

    const a = await loadEngine();
    expect(goConstructorCalls).toBe(1);

    // Simulate a Go panic: the promise returned by go.run rejects.
    rejectGoRun(new Error("Go program panicked"));
    // Flush microtasks so the exit handler clears the cache.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(errorSpy).toHaveBeenCalled();

    // The next load must build a fresh runtime instead of reusing the dead one.
    const b = await loadEngine();

    expect(goConstructorCalls).toBe(2);
    expect(goRun).toHaveBeenCalledTimes(2);
    expect(instantiateSpy).toHaveBeenCalledTimes(2);
    expect(b.handle).not.toBe(a.handle);
  });
});
