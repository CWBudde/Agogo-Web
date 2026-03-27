// Command IDs — each maps to a Go engine handler.
// Populated incrementally as phases are implemented.
export enum CommandID {
  // Phase 1: Document & Viewport
  CreateDocument = 0x0001,
  CloseDocument = 0x0002,
  ZoomSet = 0x0010,
  PanSet = 0x0011,
  RotateViewSet = 0x0012,
  Resize = 0x0013,
  FitToView = 0x0014,
  PointerEvent = 0x0015,
  JumpHistory = 0x0016,

  // Phase 2: Layers
  AddLayer = 0x0100,
  DeleteLayer = 0x0101,
  MoveLayer = 0x0102,
  SetLayerVisibility = 0x0103,
  SetLayerOpacity = 0x0104,
  SetLayerBlendMode = 0x0105,

  // Undo/Redo
  BeginTransaction = 0xffe0,
  EndTransaction = 0xffe1,
  ClearHistory = 0xffe2,
  Undo = 0xfff0,
  Redo = 0xfff1,
}

export type DocumentBackground = "transparent" | "white" | "color";
export type DocumentColorMode = "rgb" | "gray";

export interface CreateDocumentCommand {
  name: string;
  width: number;
  height: number;
  resolution: number;
  colorMode: DocumentColorMode;
  bitDepth: 8 | 16 | 32;
  background: DocumentBackground;
}

export interface ZoomCommand {
  zoom: number;
  hasAnchor?: boolean;
  anchorX?: number;
  anchorY?: number;
}

export interface PanCommand {
  centerX: number;
  centerY: number;
}

export interface RotateViewCommand {
  rotation: number;
}

export interface ResizeViewportCommand {
  canvasW: number;
  canvasH: number;
  devicePixelRatio: number;
}

export type PointerEventPhase = "down" | "move" | "up";

export interface PointerEventCommand {
  phase: PointerEventPhase;
  pointerId: number;
  x: number;
  y: number;
  button: number;
  buttons: number;
  panMode: boolean;
}

export interface BeginTransactionCommand {
  description: string;
}

export interface EndTransactionCommand {
  commit?: boolean;
}

export interface JumpHistoryCommand {
  historyIndex: number;
}
