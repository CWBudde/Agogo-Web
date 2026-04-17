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

  // Phase 6.5: Layer styles
  SetLayerStyleStack = 0x011f,
  SetLayerStyleEnabled = 0x0120,
  SetLayerStyleParams = 0x0121,
  CopyLayerStyle = 0x0122,
  PasteLayerStyle = 0x0123,
  ClearLayerStyle = 0x0124,
  CreateDocumentStylePreset = 0x0125,
  UpdateDocumentStylePreset = 0x0126,
  DeleteDocumentStylePreset = 0x0127,
  ApplyDocumentStylePreset = 0x0128,
  SetArtboard = 0x0129,

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

  // Phase 6.1: Vector Path
  SetActiveTool = 0x0600,
  PenToolClick = 0x0601,
  PenToolClose = 0x0602,
  DirectSelectMove = 0x0603,
  DirectSelectMarquee = 0x0604,
  BreakHandle = 0x0605,
  DeleteAnchor = 0x0606,
  AddAnchorOnSegment = 0x0607,
  PathCombine = 0x0610,
  PathSubtract = 0x0611,
  PathIntersect = 0x0612,
  PathExclude = 0x0613,
  FlattenPath = 0x0614,
  RasterizePath = 0x0615,
  RasterizeLayer = 0x0616,
  CreatePath = 0x0620,
  DeletePath = 0x0621,
  RenamePath = 0x0622,
  DuplicatePath = 0x0623,
  MakeSelectionFromPath = 0x0624,
  StrokePath = 0x0625,
  FillPath = 0x0626,

  // Phase 6.2: Shape Tools
  DrawShape = 0x0630,
  EnterVectorEditMode = 0x0631,
  CommitVectorEdit = 0x0632,
  SetVectorLayerStyle = 0x0633,

  // Phase 6.3: Text Engine
  AddTextLayer = 0x0640,
  SetTextContent = 0x0641,
  SetTextStyle = 0x0642,
  EnterTextEditMode = 0x0643,
  TextEditInput = 0x0644,
  CommitTextEdit = 0x0645,
  ConvertTextToPath = 0x0646,

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

export type LayerStyleKind =
  | "drop-shadow"
  | "inner-shadow"
  | "outer-glow"
  | "inner-glow"
  | "bevel-emboss"
  | "satin"
  | "color-overlay"
  | "gradient-overlay"
  | "pattern-overlay"
  | "stroke"
  | "blend-if";

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
  handleType?: number; // 0=corner, 1=smooth, 2=symmetric
}

export interface SubpathCommand {
  closed: boolean;
  points: PathPointCommand[];
}

export interface PathCommand {
  subpaths: SubpathCommand[];
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
  isArtboard?: boolean;
  artboardBounds?: LayerBoundsCommand;
  artboardBackground?: [number, number, number, number];
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

export interface SetArtboardCommand {
  layerId: string;
  bounds: LayerBoundsCommand;
  background?: [number, number, number, number];
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

export interface LayerStyleEntryCommand {
  kind: LayerStyleKind;
  enabled: boolean;
  params?: Record<string, unknown>;
}

export interface DocumentStylePresetEntry {
  id: string;
  name: string;
  styles: LayerStyleEntryCommand[];
  thumbnailBase64?: string;
}

export interface SetLayerStyleStackCommand {
  layerId: string;
  styles: LayerStyleEntryCommand[];
}

export interface SetLayerStyleEnabledCommand {
  layerId: string;
  kind: LayerStyleKind;
  enabled: boolean;
}

export interface SetLayerStyleParamsCommand {
  layerId: string;
  kind: LayerStyleKind;
  params: Record<string, unknown>;
}

export interface CopyLayerStyleCommand {
  layerId: string;
}

export interface PasteLayerStyleCommand {
  layerId: string;
}

export interface ClearLayerStyleCommand {
  layerId: string;
}

export interface CreateDocumentStylePresetCommand {
  name: string;
  styles: LayerStyleEntryCommand[];
}

export interface UpdateDocumentStylePresetCommand {
  presetId: string;
  name?: string;
  styles?: LayerStyleEntryCommand[];
}

export interface DeleteDocumentStylePresetCommand {
  presetId: string;
}

export interface ApplyDocumentStylePresetCommand {
  presetId: string;
  layerId: string;
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

// Phase 6.1: Vector Path payload types

export interface SetActiveToolCommand {
  tool: string;
}

export interface PenToolClickCommand {
  x: number;
  y: number;
  dragX?: number;
  dragY?: number;
  shift?: boolean;
}

export interface DirectSelectMoveCommand {
  subpathIndex: number;
  anchorIndex: number;
  handleKind: "anchor" | "in" | "out";
  x: number;
  y: number;
}

export interface DirectSelectMarqueeCommand {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  shift?: boolean;
}

export interface BreakHandleCommand {
  subpathIndex: number;
  anchorIndex: number;
}

export interface DeleteAnchorCommand {
  subpathIndex: number;
  anchorIndices: number[];
}

export interface AddAnchorOnSegmentCommand {
  subpathIndex: number;
  segmentIndex: number;
  t: number;
}

export interface CreatePathCommand {
  name: string;
  path?: PathCommand;
}

export interface RenamePathCommand {
  pathIndex: number;
  name: string;
}

export interface DuplicatePathCommand {
  pathIndex: number;
}

export interface DeletePathCommand {
  pathIndex: number;
}

export interface MakeSelectionFromPathCommand {
  pathIndex?: number;
  featherRadius?: number;
  antiAlias?: boolean;
}

export interface StrokePathCommand {
  pathIndex?: number;
  toolWidth?: number;
  color?: [number, number, number, number];
}

export interface FillPathCommand {
  pathIndex?: number;
  color?: [number, number, number, number];
}

// Path overlay in UIMeta

export interface PathOverlayAnchor {
  x: number;
  y: number;
  selected: boolean;
  first: boolean;
}

export interface PathOverlayLine {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface PathOverlayPolyline {
  points: Array<{ x: number; y: number }>;
}

export interface PathOverlay {
  segments: PathOverlayPolyline[];
  anchors: PathOverlayAnchor[];
  handleLines: PathOverlayLine[];
  rubberBand?: PathOverlayPolyline;
}

// Phase 6.2: Shape Tools

export type ShapeType = "rect" | "rounded-rect" | "ellipse" | "polygon" | "line" | "custom-shape";
export type ShapeMode = "shape" | "path" | "pixels";

export interface DrawShapeCommand {
  shapeType: ShapeType;
  /** Bounding box in document coordinates. For "line", x/y is start point and w/h is the delta to end. */
  x: number;
  y: number;
  w: number;
  h: number;
  /** Corner radius for "rounded-rect" (px). Default 0. */
  cornerRadius?: number;
  /** Number of sides for "polygon". Default 6. */
  sides?: number;
  /** Star mode for "polygon": alternating inner/outer vertices. */
  starMode?: boolean;
  /** Inner radius as fraction of outer radius for star mode (0–1). Default 0.5. */
  innerRadiusPct?: number;
  /** Fill color [r, g, b, a]. Omit for no fill. */
  fillColor?: [number, number, number, number];
  /** Stroke color [r, g, b, a]. Omit for no stroke. */
  strokeColor?: [number, number, number, number];
  /** Stroke width in pixels. Default 0. */
  strokeWidth?: number;
  /** Output mode: creates a VectorLayer, adds to Paths panel, or rasterizes. Default "shape". */
  mode?: ShapeMode;
  /** Whether the custom-shape subpath is closed. */
  closed?: boolean;
  /** Explicit points for "custom-shape". */
  points?: PathPointCommand[];
}

export interface EnterVectorEditModeCommand {
  layerId: string;
}

export interface CommitVectorEditCommand {}

export interface SetVectorLayerStyleCommand {
  layerId: string;
  fillColor: [number, number, number, number];
  strokeColor: [number, number, number, number];
  strokeWidth: number;
}

// Phase 6.3: Text Engine

export interface AddTextLayerCommand {
  /** X coordinate (doc-space) for the text origin. */
  x: number;
  /** Y coordinate (doc-space) for the text origin. */
  y: number;
  /** Font size in pixels. Defaults to 36 if omitted. */
  fontSize?: number;
  /** Text color [r, g, b, a]. Defaults to black opaque. */
  color?: [number, number, number, number];
  /** "point" (no wrap) or "area" (wraps within bounds). Defaults to "point". */
  textType?: "point" | "area";
}

export interface SetTextContentCommand {
  layerId: string;
  text: string;
}

export interface SetTextStyleCommand {
  layerId: string;
  fontFamily?: string;
  fontStyle?: string;
  fontSize?: number;
  bold?: boolean;
  italic?: boolean;
  color?: [number, number, number, number];
  alignment?: "left" | "center" | "right" | "justify";
  leading?: number;
  textType?: "point" | "area";
  tracking?: number;
  antiAlias?: string;
  kerning?: number;
  language?: string;
  baselineShift?: number;
  superscript?: boolean;
  subscript?: boolean;
  orientation?: string;
  underline?: boolean;
  strikethrough?: boolean;
  allCaps?: boolean;
  smallCaps?: boolean;
  indentLeft?: number;
  indentRight?: number;
  indentFirst?: number;
  spaceBefore?: number;
  spaceAfter?: number;
}

export interface EnterTextEditModeCommand {
  layerId: string;
}

/** The frontend sends the complete current text string on every keystroke. */
export interface TextEditInputCommand {
  text: string;
}

export interface CommitTextEditCommand {}

export interface ConvertTextToPathCommand {
  layerId: string;
}
