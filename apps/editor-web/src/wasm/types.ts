import type {
  CreateDocumentCommand,
  CreateSelectionCommand,
  MagicWandCommand,
  PickLayerAtPointCommand,
  PointerEventCommand,
  QuickSelectCommand,
  RenderResult,
  TransformSelectionCommand,
  TranslateLayerCommand,
} from "@agogo/proto";

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
  exportProject(): string;
  importProject(projectJSON: string): RenderResult;
  readPixels(render: RenderResult): Uint8ClampedArray;
  free(pointer: number): void;
}

export interface EngineContextValue {
  status: "idle" | "loading" | "ready" | "error";
  handle: EngineHandle | null;
  render: RenderResult | null;
  error: Error | null;
  ready: Promise<EngineHandle> | null;
  dispatchCommand(commandId: number, payload?: unknown): RenderResult | null;
  createDocument(command: CreateDocumentCommand): RenderResult | null;
  createSelection(command: CreateSelectionCommand): RenderResult | null;
  selectAll(): RenderResult | null;
  deselect(): RenderResult | null;
  reselect(): RenderResult | null;
  invertSelection(): RenderResult | null;
  magicWand(command: MagicWandCommand): RenderResult | null;
  quickSelect(command: QuickSelectCommand): RenderResult | null;
  pickLayerAtPoint(command: PickLayerAtPointCommand): RenderResult | null;
  translateLayer(command: TranslateLayerCommand): RenderResult | null;
  transformSelection(command: TransformSelectionCommand): RenderResult | null;
  resizeViewport(
    canvasW: number,
    canvasH: number,
    devicePixelRatio: number,
  ): RenderResult | null;
  setZoom(
    zoom: number,
    anchorX?: number,
    anchorY?: number,
  ): RenderResult | null;
  setPan(centerX: number, centerY: number): RenderResult | null;
  dispatchPointerEvent(command: PointerEventCommand): RenderResult | null;
  beginTransaction(description: string): RenderResult | null;
  endTransaction(commit?: boolean): RenderResult | null;
  jumpHistory(historyIndex: number): RenderResult | null;
  clearHistory(): RenderResult | null;
  setRotation(rotation: number): RenderResult | null;
  fitToView(): RenderResult | null;
  exportProject(): string | null;
  importProject(projectJSON: string): RenderResult | null;
  undo(): RenderResult | null;
  redo(): RenderResult | null;
  reload(): void;
}
