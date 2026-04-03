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
  SetShowGuides = 0x0017,

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
  TranslateLayer = 0x0119,
  PickLayerAtPoint = 0x011a,
  SetAdjustmentParams = 0x011b,

  // Phase 5.2: Adjustment layer tools
  ComputeHistogram = 0x011c,
  SetPointFromSample = 0x011d,
  IdentifyHueRange = 0x011e,

  // Phase 3: Selection
  NewSelection = 0x0200,
  SelectAll = 0x0201,
  Deselect = 0x0202,
  Reselect = 0x0203,
  InvertSelection = 0x0204,
  FeatherSelection = 0x0205,
  ExpandSelection = 0x0206,
  ContractSelection = 0x0207,
  SmoothSelection = 0x0208,
  BorderSelection = 0x0209,
  TransformSelection = 0x020a,
  SelectColorRange = 0x020b,
  QuickSelect = 0x020c,
  MagicWand = 0x020d,
  MagneticLassoSuggestPath = 0x020e,

  // Phase 3.3: Free Transform
  BeginFreeTransform = 0x0300,
  UpdateFreeTransform = 0x0301,
  CommitFreeTransform = 0x0302,
  CancelFreeTransform = 0x0303,
  FlipLayerH = 0x0304,
  FlipLayerV = 0x0305,
  RotateLayer90CW = 0x0306,
  RotateLayer90CCW = 0x0307,
  RotateLayer180 = 0x0308,
  TransformAgain = 0x0309,

  // Phase 3.4: Crop
  BeginCrop = 0x0320,
  UpdateCrop = 0x0321,
  CommitCrop = 0x0322,
  CancelCrop = 0x0323,
  ResizeCanvas = 0x0324,

  // Phase 4: Painting
  BeginPaintStroke = 0x0400,
  ContinuePaintStroke = 0x0401,
  EndPaintStroke = 0x0402,
  SetForegroundColor = 0x0410,
  SetBackgroundColor = 0x0411,
  SampleMergedColor = 0x0412,
  MagicErase = 0x0413,
  Fill = 0x0414,
  ApplyGradient = 0x0415,

  // Phase 5.4: Filters
  ApplyFilter = 0x0500,
  ReapplyFilter = 0x0501,
  PreviewFilter = 0x0502,
  CancelFilterPreview = 0x0503,
  CommitFilterPreview = 0x0504,
  FadeFilter = 0x0505,

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

export type AdjustmentKind =
  | "levels"
  | "curves"
  | "hue-sat"
  | "color-balance"
  | "brightness-contrast"
  | "exposure"
  | "vibrance"
  | "black-white"
  | "invert"
  | "channel-mixer"
  | "threshold"
  | "posterize"
  | "photo-filter"
  | "selective-color"
  | "gradient-map";

export interface LevelsAdjustmentParams {
  channel?: string;
  inputBlack?: number;
  inputWhite?: number;
  gamma?: number;
  outputBlack?: number;
  outputWhite?: number;
  auto?: boolean;
  shadowClipPercent?: number;
  highlightClipPercent?: number;
}

export interface CurvesPointCommand {
  x: number;
  y: number;
}

export interface CurvesAdjustmentParams {
  channel?: string;
  points?: CurvesPointCommand[];
}

export interface HueSatRangeAdjustmentParams {
  hueShift?: number;
  saturation?: number;
  lightness?: number;
}

export interface HueSatAdjustmentParams {
  hueShift?: number;
  saturation?: number;
  lightness?: number;
  colorize?: boolean;
  reds?: HueSatRangeAdjustmentParams;
  yellows?: HueSatRangeAdjustmentParams;
  greens?: HueSatRangeAdjustmentParams;
  cyans?: HueSatRangeAdjustmentParams;
  blues?: HueSatRangeAdjustmentParams;
  magentas?: HueSatRangeAdjustmentParams;
}

export interface ColorBalanceToneAdjustmentParams {
  cyanRed?: number;
  magentaGreen?: number;
  yellowBlue?: number;
}

export interface ColorBalanceAdjustmentParams {
  shadows?: ColorBalanceToneAdjustmentParams;
  midtones?: ColorBalanceToneAdjustmentParams;
  highlights?: ColorBalanceToneAdjustmentParams;
  preserveLuminosity?: boolean;
}

export interface BrightnessContrastAdjustmentParams {
  brightness?: number;
  contrast?: number;
  legacy?: boolean;
}

export interface ExposureAdjustmentParams {
  exposure?: number;
  offset?: number;
  gamma?: number;
}

export interface VibranceAdjustmentParams {
  vibrance?: number;
  saturation?: number;
}

export interface BlackWhiteAdjustmentParams {
  reds?: number;
  yellows?: number;
  greens?: number;
  cyans?: number;
  blues?: number;
  magentas?: number;
  auto?: boolean;
  tint?: boolean;
  tintColor?: [number, number, number];
  tintStrength?: number;
}

export interface ChannelMixerAdjustmentParams {
  monochrome?: boolean;
  red?: [number, number, number];
  green?: [number, number, number];
  blue?: [number, number, number];
}

export interface ThresholdAdjustmentParams {
  threshold?: number;
}

export interface PosterizeAdjustmentParams {
  levels?: number;
}

export interface PhotoFilterAdjustmentParams {
  color?: [number, number, number, number];
  density?: number;
  preserveLuminosity?: boolean;
}

export interface SelectiveColorToneAdjustmentParams {
  cyanRed?: number;
  magentaGreen?: number;
  yellowBlue?: number;
  black?: number;
}

export interface SelectiveColorAdjustmentParams {
  mode?: "relative" | "absolute" | string;
  reds?: SelectiveColorToneAdjustmentParams;
  yellows?: SelectiveColorToneAdjustmentParams;
  greens?: SelectiveColorToneAdjustmentParams;
  cyans?: SelectiveColorToneAdjustmentParams;
  blues?: SelectiveColorToneAdjustmentParams;
  magentas?: SelectiveColorToneAdjustmentParams;
  whites?: SelectiveColorToneAdjustmentParams;
  neutrals?: SelectiveColorToneAdjustmentParams;
  blacks?: SelectiveColorToneAdjustmentParams;
}

export interface GradientMapAdjustmentParams {
  stops?: GradientStopCommand[];
  reverse?: boolean;
}

export interface AdjustmentParamsByKind {
  levels: LevelsAdjustmentParams;
  curves: CurvesAdjustmentParams;
  "hue-sat": HueSatAdjustmentParams;
  "color-balance": ColorBalanceAdjustmentParams;
  "brightness-contrast": BrightnessContrastAdjustmentParams;
  exposure: ExposureAdjustmentParams;
  vibrance: VibranceAdjustmentParams;
  "black-white": BlackWhiteAdjustmentParams;
  invert: Record<string, never>;
  "channel-mixer": ChannelMixerAdjustmentParams;
  threshold: ThresholdAdjustmentParams;
  posterize: PosterizeAdjustmentParams;
  "photo-filter": PhotoFilterAdjustmentParams;
  "selective-color": SelectiveColorAdjustmentParams;
  "gradient-map": GradientMapAdjustmentParams;
}

export type AdjustmentLayerParams<K extends AdjustmentKind = AdjustmentKind> = AdjustmentParamsByKind[K];

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
  pressure?: number; // 0.0–1.0, defaults to 0.5
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
  adjustmentKind?: AdjustmentKind;
  params?: AdjustmentLayerParams;
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
export type SelectionCombineMode = "replace" | "add" | "subtract" | "intersect";
export type SelectionShape = "rect" | "ellipse" | "polygon";

export interface AddLayerMaskCommand {
  layerId: string;
  mode: AddLayerMaskMode;
}

export interface SelectionPointCommand {
  x: number;
  y: number;
}

export interface CreateSelectionCommand {
  shape: SelectionShape;
  mode?: SelectionCombineMode;
  rect?: LayerBoundsCommand;
  polygon?: SelectionPointCommand[];
  antiAlias?: boolean;
}

export interface FeatherSelectionCommand {
  radius: number;
}

export interface ExpandSelectionCommand {
  pixels: number;
}

export interface ContractSelectionCommand {
  pixels: number;
}

export interface SmoothSelectionCommand {
  radius: number;
}

export interface BorderSelectionCommand {
  width: number;
}

export interface TransformSelectionCommand {
  a: number;
  b: number;
  c: number;
  d: number;
  tx: number;
  ty: number;
}

export interface SelectColorRangeCommand {
  layerId?: string;
  targetColor: [number, number, number, number];
  fuzziness: number;
  sampleMerged?: boolean;
  mode?: SelectionCombineMode;
}

export interface QuickSelectCommand {
  x: number;
  y: number;
  tolerance: number;
  edgeSensitivity?: number;
  layerId?: string;
  sampleMerged?: boolean;
  mode?: SelectionCombineMode;
}

export interface MagicWandCommand {
  x: number;
  y: number;
  tolerance: number;
  layerId?: string;
  sampleMerged?: boolean;
  contiguous?: boolean;
  antiAlias?: boolean;
  mode?: SelectionCombineMode;
}

export interface MagneticLassoSuggestPathCommand {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  layerId?: string;
  sampleMerged?: boolean;
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

export interface TranslateLayerCommand {
  layerId?: string;
  dx: number;
  dy: number;
}

export interface PickLayerAtPointCommand {
  x: number;
  y: number;
}

export interface SetAdjustmentParamsCommand {
  layerId: string;
  adjustmentKind?: AdjustmentKind;
  params?: AdjustmentLayerParams;
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

// Phase 3.3 – Free Transform

export type InterpolMode = "nearest" | "bilinear" | "bicubic";

export interface BeginFreeTransformCommand {
  layerId?: string;
  /** When "warp", initialises a 4×4 control-point mesh immediately. */
  mode?: "warp";
}

/** Affine matrix: docX = a*lx + c*ly + tx,  docY = b*lx + d*ly + ty */
export interface UpdateFreeTransformCommand {
  a: number;
  b: number;
  c: number;
  d: number;
  tx: number;
  ty: number;
  pivotX: number;
  pivotY: number;
  interpolation?: InterpolMode;
  /** When present, switches to homography/distort mode. Order: TL, TR, BR, BL. */
  corners?: [[number, number], [number, number], [number, number], [number, number]];
  /** When present, switches to mesh-warp mode. 4×4 grid [row][col] in doc space. */
  warpGrid?: [[number, number], [number, number], [number, number], [number, number]][];
}

export interface DiscreteTransformCommand {
  layerId?: string;
}

export interface FreeTransformMeta {
  active: boolean;
  layerId?: string;
  origX: number;
  origY: number;
  origW: number;
  origH: number;
  a: number;
  b: number;
  c: number;
  d: number;
  tx: number;
  ty: number;
  pivotX: number;
  pivotY: number;
  interpolation: InterpolMode;
  /** Corners of source bbox after transform in doc space: TL, TR, BR, BL */
  corners: [[number, number], [number, number], [number, number], [number, number]];
  /** Present in mesh-warp mode: 4×4 control-point grid [row][col] in doc space. */
  warpGrid?: [[number, number], [number, number], [number, number], [number, number]][];
  scaleX: number;
  scaleY: number;
  rotation: number;
  skewX: number;
  skewY: number;
}

// Phase 3.4 - Crop
export interface UpdateCropCommand {
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
  deletePixels: boolean;
}

export interface ResizeCanvasCommand {
  width: number;
  height: number;
  anchor: "top-left" | "top-center" | "top-right" | "middle-left" | "center" | "middle-right" | "bottom-left" | "bottom-center" | "bottom-right";
}

export interface CropMeta {
  active: boolean;
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
  deletePixels: boolean;
}

// Phase 4: Painting

export interface SetColorCommand {
  color: [number, number, number, number]; // [R, G, B, A] each 0-255
}

export interface BrushParams {
  size: number;       // diameter in document pixels
  hardness: number;   // 0.0–1.0
  flow: number;       // 0.0–1.0
  color: [number, number, number, number]; // RGBA 0-255
  blendMode?: string;    // AGG blend mode, e.g. "multiply", "screen", "overlay" (omit for normal)
  wetEdges?: boolean;    // accumulate paint at stroke edges (watercolour effect)
  scatter?: number;      // max random dab offset as fraction of diameter, 0 = none
  stabilizer?: number;   // moving-average lag: number of past input points to average (0 = off)
  sampleMerged?: boolean; // read from composite (all layers) rather than active layer
  autoErase?: boolean;       // if stroke starts on foreground color, paint with background color instead
  erase?: boolean;           // erase to transparency (normal eraser mode)
  eraseBackground?: boolean; // erase only pixels matching the sampled base color (background eraser mode)
  eraseTolerance?: number;   // color tolerance for background eraser, 0–255 Euclidean RGB distance
  mixerBrush?: boolean;      // mix brush color with sampled canvas color before painting
  mixerMix?: number;         // 0.0–1.0 mix strength for the sampled color
  cloneStamp?: boolean;      // paint from a sampled source point
  cloneSourceX?: number;     // source point X in document space
  cloneSourceY?: number;     // source point Y in document space
  historyBrush?: boolean;    // paint from the previous history state
}

export interface BeginPaintStrokeCommand {
  x: number;
  y: number;
  pressure?: number; // 0.0–1.0, defaults to 0.5
  tiltX?: number;   // stylus tilt, degrees −90…+90
  tiltY?: number;   // stylus tilt, degrees −90…+90
  brush: BrushParams;
}

export interface ContinuePaintStrokeCommand {
  x: number;
  y: number;
  pressure?: number;
  tiltX?: number;   // stylus tilt, degrees −90…+90
  tiltY?: number;   // stylus tilt, degrees −90…+90
}

/** Sample the RGBA color of the composite image at a document-space point.
 *  The result is returned in RenderResult.sampledColor. */
export interface SampleMergedColorCommand {
  x: number;
  y: number;
  sampleSize?: number;
  sampleMerged?: boolean;
}

/** Flood-erase pixels by color similarity (Magic Eraser tool). */
export interface MagicEraseCommand {
  x: number;           // document-space click position
  y: number;
  tolerance: number;   // 0–255 Euclidean RGB distance
  contiguous: boolean; // true = flood-fill, false = erase all matching pixels in layer
  sampleMerged: boolean;
}

export type FillSource = "foreground" | "background" | "color" | "pattern";

export interface FillCommand {
  hasPoint?: boolean;
  x?: number;
  y?: number;
  tolerance?: number;
  contiguous?: boolean;
  sampleMerged?: boolean;
  source?: FillSource;
  color?: [number, number, number, number];
  createLayer?: boolean;
}

export type GradientType = "linear" | "radial" | "angle" | "reflected" | "diamond";

export interface GradientStopCommand {
  position: number;
  color: [number, number, number, number];
}

export interface ApplyGradientCommand {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  type: GradientType;
  reverse?: boolean;
  dither?: boolean;
  createLayer?: boolean;
  stops?: GradientStopCommand[];
}
