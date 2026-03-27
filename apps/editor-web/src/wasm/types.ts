import type { CreateDocumentCommand, PointerEventCommand, RenderResult } from "@agogo/proto";

export interface EngineConfig {
  documentWidth?: number;
  documentHeight?: number;
  background?: "transparent" | "white" | "color";
  resolution?: number;
}

export interface EngineHandle {
  readonly handle: number;
  readonly memory: WebAssembly.Memory;
  dispatch(commandId: number, payload?: unknown): RenderResult;
  renderFrame(): RenderResult;
  readPixels(render: RenderResult): Uint8ClampedArray;
  free(pointer: number): void;
}

export interface EngineContextValue {
  status: "idle" | "loading" | "ready" | "error";
  handle: EngineHandle | null;
  render: RenderResult | null;
  error: Error | null;
  ready: Promise<EngineHandle> | null;
  createDocument(command: CreateDocumentCommand): RenderResult | null;
  resizeViewport(canvasW: number, canvasH: number, devicePixelRatio: number): RenderResult | null;
  setZoom(zoom: number, anchorX?: number, anchorY?: number): RenderResult | null;
  setPan(centerX: number, centerY: number): RenderResult | null;
  dispatchPointerEvent(command: PointerEventCommand): RenderResult | null;
  beginTransaction(description: string): RenderResult | null;
  endTransaction(commit?: boolean): RenderResult | null;
  jumpHistory(historyIndex: number): RenderResult | null;
  clearHistory(): RenderResult | null;
  setRotation(rotation: number): RenderResult | null;
  fitToView(): RenderResult | null;
  undo(): RenderResult | null;
  redo(): RenderResult | null;
  reload(): void;
}
