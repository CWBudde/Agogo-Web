import type {
  CreateDocumentCommand,
  CreateSelectionCommand,
  MagicWandCommand,
  MagneticLassoSuggestPathCommand,
  PickLayerAtPointCommand,
  PointerEventCommand,
  QuickSelectCommand,
  RawRenderResult,
  RenderResult,
  TransformSelectionCommand,
  TranslateLayerCommand,
  UIMeta,
} from "@agogo/proto";

/**
 * A render result that is guaranteed to carry a full UIMeta. The engine
 * context maintains this invariant by merging hot-path acks (which omit
 * uiMeta) into the last full result, so consumers never see missing fields.
 */
export type EngineRenderState = RenderResult & { uiMeta: UIMeta };

export interface EngineConfig {
  documentWidth?: number;
  documentHeight?: number;
  background?: "transparent" | "white" | "color";
  resolution?: number;
}

export interface EngineHandle {
  readonly handle: number;
  readonly memory: WebAssembly.Memory;
  dispatchCommand(commandId: number, payload?: unknown): RenderResult;
  renderFrame(): RenderResult;
  renderFrameRaw(): RawRenderResult;
  exportProject(): string;
  exportDocument(format: string): string;
  importProject(projectJSON: string): RenderResult;
  readPixels(render: { bufferPtr: number; bufferLen: number }): Uint8ClampedArray;
  free(pointer: number): void;
  dispose(): void;
}

/**
 * Minimal external-store surface over the engine's latest render state.
 * `subscribe` registers a listener invoked whenever a new render state is
 * committed (full renders AND hot-path ack merges that changed something);
 * `getSnapshot` returns the latest merged render state synchronously.
 * Designed for React's `useSyncExternalStore` (see use-engine-render.ts).
 */
export interface EngineStore {
  subscribe(listener: () => void): () => void;
  getSnapshot(): EngineRenderState | null;
}

/**
 * The engine command surface plus the identity-stable store methods
 * (subscribe/getSnapshot). The context value no longer carries the per-frame
 * `render` state: the value is rebuilt only on status/handle/error changes, so
 * reading it via useEngine() never re-renders per frame. Read render state
 * through the selector hooks in use-engine-render.ts instead.
 */
export interface EngineContextValue extends EngineStore {
  status: "idle" | "loading" | "ready" | "error";
  handle: EngineHandle | null;
  error: Error | null;
  ready: Promise<EngineHandle> | null;
  dispatchCommand(commandId: number, payload?: unknown): EngineRenderState | null;
  createDocument(command: CreateDocumentCommand): EngineRenderState | null;
  createSelection(command: CreateSelectionCommand): EngineRenderState | null;
  selectAll(): EngineRenderState | null;
  deselect(): EngineRenderState | null;
  reselect(): EngineRenderState | null;
  invertSelection(): EngineRenderState | null;
  magicWand(command: MagicWandCommand): EngineRenderState | null;
  quickSelect(command: QuickSelectCommand): EngineRenderState | null;
  magneticLassoSuggestPath(command: MagneticLassoSuggestPathCommand): EngineRenderState | null;
  pickLayerAtPoint(command: PickLayerAtPointCommand): EngineRenderState | null;
  translateLayer(command: TranslateLayerCommand): EngineRenderState | null;
  transformSelection(command: TransformSelectionCommand): EngineRenderState | null;
  resizeViewport(
    canvasW: number,
    canvasH: number,
    devicePixelRatio: number,
  ): EngineRenderState | null;
  setZoom(zoom: number, anchorX?: number, anchorY?: number): EngineRenderState | null;
  setPan(centerX: number, centerY: number): EngineRenderState | null;
  dispatchPointerEvent(command: PointerEventCommand): EngineRenderState | null;
  beginTransaction(description: string): EngineRenderState | null;
  endTransaction(commit?: boolean): EngineRenderState | null;
  jumpHistory(historyIndex: number): EngineRenderState | null;
  clearHistory(): EngineRenderState | null;
  setRotation(rotation: number): EngineRenderState | null;
  fitToView(): EngineRenderState | null;
  setShowGuides(show: boolean): EngineRenderState | null;
  exportProject(): string | null;
  exportDocument(format: string): string | null;
  importProject(projectJSON: string): EngineRenderState | null;
  undo(): EngineRenderState | null;
  redo(): EngineRenderState | null;
  reload(): void;
}
