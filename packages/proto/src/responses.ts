// RenderResult — returned by the engine after each command dispatch.
// bufferPtr and bufferLen reference a region inside the Wasm linear memory.

export interface DirtyRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface ViewportMeta {
  centerX: number;
  centerY: number;
  zoom: number;
  rotation: number;
  canvasW: number;
  canvasH: number;
  devicePixelRatio: number;
}

export interface UIMeta {
  activeLayerId: string | null;
  activeLayerName: string | null;
  cursorType: string;
  statusText: string;
  rulerOriginX: number;
  rulerOriginY: number;
  history: HistoryEntry[];
  currentHistoryIndex: number;
  canUndo: boolean;
  canRedo: boolean;
  activeDocumentId: string;
  activeDocumentName: string;
  documentWidth: number;
  documentHeight: number;
  documentBackground: string;
  layers: LayerNodeMeta[];
}

export interface LayerNodeMeta {
  id: string;
  name: string;
  layerType: "pixel" | "group" | "adjustment" | "text" | "vector";
  parentId?: string;
  visible: boolean;
  lockMode: "none" | "pixels" | "position" | "all";
  opacity: number;
  fillOpacity: number;
  blendMode: string;
  clippingBase: boolean;
  hasMask: boolean;
  hasVectorMask: boolean;
  isolated?: boolean;
  children?: LayerNodeMeta[];
}

export interface HistoryEntry {
  id: number;
  description: string;
  state: "done" | "current" | "undone";
}

export interface RenderResult {
  frameId: number;
  viewport: ViewportMeta;
  dirtyRects: DirtyRect[];
  pixelFormat: "rgba8-premultiplied";
  bufferPtr: number;
  bufferLen: number;
  uiMeta: UIMeta;
}
