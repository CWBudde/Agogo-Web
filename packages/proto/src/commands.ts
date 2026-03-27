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
  DuplicateLayer = 0x0106,
  SetLayerLock = 0x0107,
  FlattenLayer = 0x0108,
  MergeDown = 0x0109,
  MergeVisible = 0x010a,
  AddLayerMask = 0x010b,
  DeleteLayerMask = 0x010c,
  ApplyLayerMask = 0x010d,
  InvertLayerMask = 0x010e,
  SetLayerMaskEnabled = 0x010f,
  SetLayerClipToBelow = 0x0110,
  SetActiveLayer = 0x0111,
  SetLayerName = 0x0112,
  AddVectorMask = 0x0113,
  DeleteVectorMask = 0x0114,
  SetMaskEditMode = 0x0115,
  GetLayerThumbnails = 0x0116,
  FlattenImage = 0x0117,
  OpenImageFile = 0x0118,

  // Undo/Redo
  BeginTransaction = 0xffe0,
  EndTransaction = 0xffe1,
  ClearHistory = 0xffe2,
  Undo = 0xfff0,
  Redo = 0xfff1,
}

export type DocumentBackground = "transparent" | "white" | "color";
export type DocumentColorMode = "rgb" | "gray";
export type LayerType = "pixel" | "group" | "adjustment" | "text" | "vector";
export type LayerLockMode = "none" | "pixels" | "position" | "all";
export type LayerBlendMode =
  | "normal"
  | "dissolve"
  | "multiply"
  | "color-burn"
  | "linear-burn"
  | "darken"
  | "darker-color"
  | "screen"
  | "color-dodge"
  | "linear-dodge"
  | "lighten"
  | "lighter-color"
  | "overlay"
  | "soft-light"
  | "hard-light"
  | "vivid-light"
  | "linear-light"
  | "pin-light"
  | "hard-mix"
  | "difference"
  | "exclusion"
  | "subtract"
  | "divide"
  | "hue"
  | "saturation"
  | "color"
  | "luminosity";

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

export interface LayerBoundsCommand {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface PathPointCommand {
  x: number;
  y: number;
  inX?: number;
  inY?: number;
  outX?: number;
  outY?: number;
  hasCurve?: boolean;
}

export interface PathCommand {
  closed: boolean;
  points: PathPointCommand[];
}

export interface AddLayerCommand {
  layerType: LayerType;
  name?: string;
  parentLayerId?: string;
  index?: number;
  bounds?: LayerBoundsCommand;
  pixels?: number[];
  adjustmentKind?: string;
  params?: unknown;
  text?: string;
  fontFamily?: string;
  fontSize?: number;
  color?: [number, number, number, number];
  path?: PathCommand;
  fillColor?: [number, number, number, number];
  strokeColor?: [number, number, number, number];
  strokeWidth?: number;
  cachedRaster?: number[];
  isolated?: boolean;
}

export interface DeleteLayerCommand {
  layerId: string;
}

export interface DuplicateLayerCommand {
  layerId: string;
  parentLayerId?: string;
  index?: number;
}

export interface MoveLayerCommand {
  layerId: string;
  parentLayerId?: string;
  index?: number;
}

export interface SetLayerVisibilityCommand {
  layerId: string;
  visible: boolean;
}

export interface SetLayerOpacityCommand {
  layerId: string;
  opacity?: number;
  fillOpacity?: number;
}

export interface SetLayerBlendModeCommand {
  layerId: string;
  blendMode: LayerBlendMode;
}

export interface SetLayerLockCommand {
  layerId: string;
  lockMode: LayerLockMode;
}

export interface FlattenLayerCommand {
  layerId: string;
}

export interface MergeDownCommand {
  layerId: string;
}

export type AddLayerMaskMode = "reveal-all" | "hide-all" | "from-selection";

export interface AddLayerMaskCommand {
  layerId: string;
  mode: AddLayerMaskMode;
}

export interface DeleteLayerMaskCommand {
  layerId: string;
}

export interface ApplyLayerMaskCommand {
  layerId: string;
}

export interface InvertLayerMaskCommand {
  layerId: string;
}

export interface SetLayerMaskEnabledCommand {
  layerId: string;
  enabled: boolean;
}

export interface SetLayerClipToBelowCommand {
  layerId: string;
  clipToBelow: boolean;
}

export interface SetActiveLayerCommand {
  layerId: string;
}

export interface SetLayerNameCommand {
  layerId: string;
  name: string;
}

export interface AddVectorMaskCommand {
  layerId: string;
}

export interface DeleteVectorMaskCommand {
  layerId: string;
}

export interface SetMaskEditModeCommand {
  layerId: string;
  editing: boolean;
}
