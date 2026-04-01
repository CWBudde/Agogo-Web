import type { RenderResult } from "@agogo/proto";
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
    ExportProject?: (handle: number) => string;
    ImportProject?: (handle: number, payloadJSON?: string) => string;
    Free?: (pointer: number) => void;
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

export async function loadEngine({
  wasmUrl = DEFAULT_WASM_URL,
  wasmExecUrl = DEFAULT_WASM_EXEC_URL,
  config = {},
}: EngineLoaderOptions = {}): Promise<EngineHandle> {
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

  void go.run(instance);

  const init = window.EngineInit;
  const dispatch = window.DispatchCommand;
  const renderFrame = window.RenderFrame;
  const exportProject = window.ExportProject;
  const importProject = window.ImportProject;

  if (!init || !dispatch || !renderFrame || !exportProject || !importProject) {
    throw new WasmEngineLoadError("The Go runtime did not register the expected engine functions.");
  }

  const handle = init(JSON.stringify(config));
  if (typeof handle !== "number") {
    throw new WasmEngineLoadError("EngineInit did not return a numeric handle.");
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
    handle,
    memory,
    dispatchCommand(commandId: number, payload?: unknown) {
      return parseRenderResult(dispatch(handle, commandId, JSON.stringify(payload ?? {})));
    },
    renderFrame() {
      return parseRenderResult(renderFrame(handle));
    },
    exportProject() {
      return exportProject(handle);
    },
    importProject(projectJSON: string) {
      return parseRenderResult(importProject(handle, projectJSON));
    },
    readPixels(render: RenderResult) {
      return new Uint8ClampedArray(memory.buffer, render.bufferPtr, render.bufferLen);
    },
    free(pointer: number) {
      window.Free?.(pointer);
    },
  };
}
