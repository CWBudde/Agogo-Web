import type { RawRenderResult, RenderResult } from "@agogo/proto";
import type { EngineConfig, EngineHandle } from "./types";

declare global {
  interface Window {
    Go?: new () => {
      importObject: WebAssembly.Imports;
      run(instance: WebAssembly.Instance): Promise<void> | void;
    };
    EngineInit?: (configJSON: string) => number;
    DispatchCommand?: (handle: number, commandId: number, payloadJSON?: string) => string;
    RenderFrame?: (handle: number) => string;
    RenderFrameRaw?: (handle: number) => string;
    ExportProject?: (handle: number) => string;
    ExportDocument?: (handle: number, format?: string) => string;
    ImportProject?: (handle: number, payloadJSON?: string) => string;
    Free?: (pointer: number) => void;
    EngineFree?: (handle: number) => void;
  }
}

export class WasmEngineLoadError extends Error {
  constructor(message: string, cause?: unknown) {
    super(message);
    this.name = "WasmEngineLoadError";
    if (cause !== undefined) {
      this.cause = cause;
    }
  }
}

type EngineLoaderOptions = {
  wasmUrl?: string;
  wasmExecUrl?: string;
  config?: EngineConfig;
};

const DEFAULT_WASM_URL = `${import.meta.env.BASE_URL}engine.wasm`;
const DEFAULT_WASM_EXEC_URL = `${import.meta.env.BASE_URL}wasm_exec.js`;

function ensureGoRuntime(wasmExecUrl: string) {
  if (window.Go) {
    return Promise.resolve();
  }

  return new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = wasmExecUrl;
    script.async = true;
    script.onload = () => {
      if (!window.Go) {
        reject(
          new WasmEngineLoadError(
            "wasm_exec.js loaded, but the Go runtime constructor was not registered.",
          ),
        );
        return;
      }

      resolve();
    };
    script.onerror = () => {
      reject(new WasmEngineLoadError(`Failed to load Go runtime script from ${wasmExecUrl}.`));
    };
    document.head.appendChild(script);
  });
}

async function instantiateWasm(url: string, go: InstanceType<NonNullable<Window["Go"]>>) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new WasmEngineLoadError(
      `Failed to fetch Wasm bundle from ${url}: ${response.status} ${response.statusText}`,
    );
  }

  try {
    return await WebAssembly.instantiateStreaming(response.clone(), go.importObject);
  } catch {
    return WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  }
}

function parseRenderResult(payload: string): RenderResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(payload);
  } catch (error) {
    throw new WasmEngineLoadError("The engine returned invalid JSON.", error);
  }

  if (
    typeof parsed === "object" &&
    parsed !== null &&
    "error" in parsed &&
    typeof parsed.error === "string"
  ) {
    throw new WasmEngineLoadError(parsed.error);
  }

  return parsed as RenderResult;
}

function parseRawRenderResult(payload: string): RawRenderResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(payload);
  } catch (error) {
    throw new WasmEngineLoadError("The engine returned invalid JSON.", error);
  }

  if (
    typeof parsed === "object" &&
    parsed !== null &&
    "error" in parsed &&
    typeof parsed.error === "string"
  ) {
    throw new WasmEngineLoadError(parsed.error);
  }

  return parsed as RawRenderResult;
}

/**
 * Everything that must exist exactly once per page: the running Go program,
 * its wasm memory, and the ABI functions it registered on `window`. The
 * functions are captured immediately after `go.run` so that a stray second
 * runtime overwriting the window globals can never redirect our calls.
 */
type Runtime = {
  memory: WebAssembly.Memory;
  init: (configJSON: string) => number;
  dispatch: (handle: number, commandId: number, payloadJSON?: string) => string;
  renderFrame: (handle: number) => string;
  renderFrameRaw: (handle: number) => string;
  exportProject: (handle: number) => string;
  exportDocument: (handle: number, format?: string) => string;
  importProject: (handle: number, payloadJSON?: string) => string;
  free?: (pointer: number) => void;
  engineFree?: (handle: number) => void;
};

/**
 * Module-level cache: React StrictMode mounts EngineProvider twice, and both
 * effects call loadEngine(). Each Go runtime overwrites the same window
 * globals, so starting two runtimes races last-writer-wins and leaks the
 * first wasm instance. All loadEngine calls therefore share one runtime and
 * only differ in the EngineInit handle they hold.
 */
let runtimePromise: Promise<Runtime> | null = null;

async function createRuntime(
  wasmUrl: string,
  wasmExecUrl: string,
  onRuntimeExit: () => void,
): Promise<Runtime> {
  await ensureGoRuntime(wasmExecUrl);

  if (!window.Go) {
    throw new WasmEngineLoadError("The Go runtime is unavailable.");
  }

  const go = new window.Go();
  const result = await instantiateWasm(wasmUrl, go);
  const instance = result instanceof WebAssembly.Instance ? result : result.instance;
  const exports = instance.exports as WebAssembly.Exports & {
    memory?: WebAssembly.Memory;
    mem?: WebAssembly.Memory;
  };

  // go.run's promise stays pending while the Go program is alive. If it ever
  // settles (clean exit or panic), this runtime is dead: report it and let
  // the caller drop the cache so the next loadEngine builds a fresh runtime.
  void Promise.resolve(go.run(instance))
    .then(
      () => {
        console.error(
          "The Go engine runtime exited; it will be recreated on the next loadEngine call.",
        );
      },
      (error: unknown) => {
        console.error(
          "The Go engine runtime crashed; it will be recreated on the next loadEngine call.",
          error,
        );
      },
    )
    .finally(onRuntimeExit);

  // Capture the ABI functions right after go.run registers them, before any
  // other runtime could overwrite the window globals.
  const init = window.EngineInit;
  const dispatch = window.DispatchCommand;
  const renderFrame = window.RenderFrame;
  const renderFrameRaw = window.RenderFrameRaw;
  const exportProject = window.ExportProject;
  const exportDocument = window.ExportDocument;
  const importProject = window.ImportProject;
  const free = window.Free;
  const engineFree = window.EngineFree;

  if (
    !init ||
    !dispatch ||
    !renderFrame ||
    !renderFrameRaw ||
    !exportProject ||
    !exportDocument ||
    !importProject
  ) {
    throw new WasmEngineLoadError("The Go runtime did not register the expected engine functions.");
  }

  const memory =
    exports.memory instanceof WebAssembly.Memory
      ? exports.memory
      : exports.mem instanceof WebAssembly.Memory
        ? exports.mem
        : undefined;
  if (!(memory instanceof WebAssembly.Memory)) {
    throw new WasmEngineLoadError("The Wasm module did not export linear memory.");
  }

  return {
    memory,
    init,
    dispatch,
    renderFrame,
    renderFrameRaw,
    exportProject,
    exportDocument,
    importProject,
    free,
    engineFree,
  };
}

function ensureRuntime(wasmUrl: string, wasmExecUrl: string): Promise<Runtime> {
  if (!runtimePromise) {
    // Drop the cache when this runtime fails to load or later dies, but only
    // if it is still the cached one — a newer runtime must not be clobbered.
    const resetIfCurrent = () => {
      if (runtimePromise === promise) {
        runtimePromise = null;
      }
    };
    const promise: Promise<Runtime> = createRuntime(wasmUrl, wasmExecUrl, resetIfCurrent).catch(
      (error: unknown) => {
        // Reset on load failure so a retry (e.g. after a transient network
        // error or a page-level "reload engine" action) can succeed.
        resetIfCurrent();
        throw error;
      },
    );
    runtimePromise = promise;
  }
  return runtimePromise;
}

/**
 * Load the shared engine runtime and create a new engine handle on it.
 *
 * The Go/wasm runtime is created once per page and shared by all callers:
 * `wasmUrl` and `wasmExecUrl` are only honoured by the call that creates the
 * runtime — later calls reuse the existing runtime and ignore differing URLs.
 * `config` applies per call: every call gets its own EngineInit handle.
 */
export async function loadEngine({
  wasmUrl = DEFAULT_WASM_URL,
  wasmExecUrl = DEFAULT_WASM_EXEC_URL,
  config = {},
}: EngineLoaderOptions = {}): Promise<EngineHandle> {
  const runtime = await ensureRuntime(wasmUrl, wasmExecUrl);

  // A handle-creation failure is NOT a runtime failure: the shared runtime
  // stays cached and later loadEngine calls may still succeed.
  let handle: number;
  try {
    handle = runtime.init(JSON.stringify(config));
  } catch (error) {
    throw new WasmEngineLoadError("EngineInit threw while creating an engine handle.", error);
  }
  if (typeof handle !== "number") {
    throw new WasmEngineLoadError("EngineInit did not return a numeric handle.");
  }

  const { memory } = runtime;

  return {
    handle,
    memory,
    dispatchCommand(commandId: number, payload?: unknown) {
      return parseRenderResult(runtime.dispatch(handle, commandId, JSON.stringify(payload ?? {})));
    },
    renderFrame() {
      return parseRenderResult(runtime.renderFrame(handle));
    },
    renderFrameRaw() {
      return parseRawRenderResult(runtime.renderFrameRaw(handle));
    },
    exportProject() {
      return runtime.exportProject(handle);
    },
    exportDocument(format: string) {
      return runtime.exportDocument(handle, format);
    },
    importProject(projectJSON: string) {
      return parseRenderResult(runtime.importProject(handle, projectJSON));
    },
    readPixels(render: RenderResult) {
      return new Uint8ClampedArray(memory.buffer, render.bufferPtr, render.bufferLen);
    },
    free(pointer: number) {
      runtime.free?.(pointer);
    },
    dispose() {
      runtime.engineFree?.(handle);
    },
  };
}
