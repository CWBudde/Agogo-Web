import {
  CommandID,
  type AdjustmentKind,
  type AdjustmentLayerParams,
  type AddLayerCommand,
  type CreateDocumentCommand,
  type GradientStopCommand,
  type FreeTransformMeta,
  type InterpolMode,
  type FillCommand,
  type FillSource,
  type GradientType,
  type LayerNodeMeta,
  type SetColorCommand,
  type ThumbnailEntry,
} from "@agogo/proto";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { EditorCanvas } from "@/components/editor-canvas";
import {
  BRUSH_PRESETS,
  BrushPresetPicker,
  BrushSettingsPanel,
  type BrushControlSource,
  type BrushPreset,
  type BrushTipShape,
  ColorPickerDialog,
  ColorPanel,
  SwatchesPanel,
  type ColorChannelMode,
} from "@/components/brush-color-panels";
import { GradientEditorDialog } from "@/components/gradient-editor";
import { SelectAndMaskWorkspace } from "@/components/select-and-mask";
import { WelcomeScreen } from "@/components/welcome-screen";
import {
  BrushToolIcon,
  ClipboardIcon,
  CopyIcon,
  CropToolIcon,
  EraserToolIcon,
  EyedropperToolIcon,
  FitScreenIcon,
  FillToolIcon,
  GradientToolIcon,
  HandToolIcon,
  InfoIcon,
  LassoToolIcon,
  LayersIcon,
  MarqueeToolIcon,
  MoveToolIcon,
  NewDocumentIcon,
  OpenFolderIcon,
  PanelsIcon,
  PencilToolIcon,
  RedoIcon,
  SaveIcon,
  ScissorsIcon,
  SelectionIcon,
  ShapeToolIcon,
  SlidersIcon,
  TypeToolIcon,
  UndoIcon,
  ZoomToolIcon,
  PenToolIcon,
  DirectSelectIcon,
} from "@/components/editor-icons";
import { AdjPropertiesPanel, AdjustmentsPanel } from "@/components/adjustments-panel";
import { LayersPanel } from "@/components/layers-panel";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import {
  DropdownMenuItem,
  DropdownMenuContent,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import {
  type ShortcutTool,
  useKeyboardShortcuts,
} from "@/hooks/use-keyboard-shortcuts";
import { hexToRgba, rgbaToCss, rgbaToHex, snapToWebSafeColor, toMutableRgba, toRgba, type Rgba } from "@/lib/color";
import { useEngine } from "@/wasm/context";

type MenuPreviewTone = "default" | "accent" | "muted";

type MenuActionId =
  | "new-document"
  | "open-project"
  | "open-recent"
  | "save-project"
  | "export-project"
  | "generate-assets"
  | "canvas-size"
  | "transform-free"
  | "transform-scale"
  | "transform-rotate"
  | "transform-skew"
  | "transform-distort"
  | "transform-perspective"
  | "transform-warp"
  | "transform-flip-h"
  | "transform-flip-v"
  | "transform-rotate-cw"
  | "transform-rotate-ccw"
  | "transform-rotate-180"
  | "edit-fill"
  | "image-invert"
  | "image-channel-mixer"
  | "image-threshold"
  | "image-posterize"
  | "image-selective-color"
  | "image-photo-filter"
  | "image-gradient-map"
  | "select-all"
  | "select-deselect"
  | "select-reselect"
  | "select-invert"
  | "select-feather"
  | "select-expand"
  | "select-contract"
  | "select-smooth"
  | "select-border"
  | "select-transform"
  | "select-color-range"
  | "select-and-mask"
  | "view-toggle-guides";

type MenuPreviewItem = {
  label: string;
  shortcut?: string;
  tone?: MenuPreviewTone;
  actionId?: MenuActionId;
  disabled?: boolean;
  checked?: boolean;
};

type MenuPreviewMenu = {
  label: string;
  caption: string;
  align?: "left" | "right";
  sections: { title: string; items: MenuPreviewItem[] }[];
};

const menuItems: MenuPreviewMenu[] = [
  {
    label: "File",
    caption: "Document lifecycle and export flow preview.",
    sections: [
      {
        title: "Document",
        items: [
          {
            label: "New Document",
            shortcut: "Ctrl+N",
            tone: "accent",
            actionId: "new-document",
          },
          { label: "Open...", shortcut: "Ctrl+O", actionId: "open-project" },
          { label: "Open Recent", actionId: "open-recent" },
        ],
      },
      {
        title: "Output",
        items: [
          { label: "Save", shortcut: "Ctrl+S", actionId: "save-project" },
          {
            label: "Export As...",
            shortcut: "Ctrl+Shift+E",
            actionId: "export-project",
          },
          {
            label: "Generate Assets",
            tone: "muted",
            actionId: "generate-assets",
            disabled: true,
          },
        ],
      },
    ],
  },
  {
    label: "Edit",
    caption: "History, clipboard, and transform placeholders.",
    sections: [
      {
        title: "History",
        items: [
          { label: "Undo", shortcut: "Ctrl+Z", tone: "accent" },
          { label: "Redo", shortcut: "Ctrl+Shift+Z" },
        ],
      },
      {
        title: "Clipboard",
        items: [
          { label: "Cut", shortcut: "Ctrl+X" },
          { label: "Copy", shortcut: "Ctrl+C" },
          { label: "Paste", shortcut: "Ctrl+V" },
        ],
      },
      {
        title: "Fill",
        items: [
          { label: "Fill...", shortcut: "Shift+F5", actionId: "edit-fill" as const, tone: "accent" },
        ],
      },
      {
        title: "Transform",
        items: [
          { label: "Free Transform", shortcut: "Ctrl+T", tone: "accent", actionId: "transform-free" as const },
          { label: "Scale",          actionId: "transform-scale"       as const },
          { label: "Rotate",         actionId: "transform-rotate"      as const },
          { label: "Skew",           actionId: "transform-skew"        as const },
          { label: "Distort",        actionId: "transform-distort"     as const },
          { label: "Perspective",    actionId: "transform-perspective" as const },
          { label: "Warp",           actionId: "transform-warp"        as const },
          { label: "Flip Horizontal", actionId: "transform-flip-h" as const },
          { label: "Flip Vertical",   actionId: "transform-flip-v" as const },
          { label: "Rotate 90° CW",   actionId: "transform-rotate-cw" as const },
          { label: "Rotate 90° CCW",  actionId: "transform-rotate-ccw" as const },
          { label: "Rotate 180°",     actionId: "transform-rotate-180" as const },
        ],
      },
    ],
  },
  {
    label: "Image",
    caption: "Canvas-wide operations and color management preview.",
    sections: [
      {
        title: "Adjustments",
        items: [
          { label: "Levels..." },
          { label: "Curves..." },
          { label: "Hue/Saturation..." },
          { label: "Invert", actionId: "image-invert" as const },
          { label: "Channel Mixer...", actionId: "image-channel-mixer" as const },
          { label: "Threshold...", actionId: "image-threshold" as const },
          { label: "Posterize...", actionId: "image-posterize" as const },
          { label: "Selective Color...", actionId: "image-selective-color" as const },
          { label: "Photo Filter...", actionId: "image-photo-filter" as const },
          { label: "Gradient Map...", actionId: "image-gradient-map" as const },
        ],
      },
      {
        title: "Geometry",
        items: [
          { label: "Image Size..." },
          { label: "Canvas Size...", shortcut: "Ctrl+Alt+C", actionId: "canvas-size" as const },
          { label: "Trim" },
        ],
      },
    ],
  },
  {
    label: "Layer",
    caption: "Layer stack actions matching the right-side dock.",
    sections: [
      {
        title: "Create",
        items: [
          { label: "New Layer", shortcut: "Shift+Ctrl+N", tone: "accent" },
          { label: "New Group" },
          { label: "Layer Mask" },
        ],
      },
      {
        title: "Arrange",
        items: [
          { label: "Duplicate Layer", shortcut: "Ctrl+J" },
          { label: "Merge Down", shortcut: "Ctrl+E" },
          { label: "Rasterize", tone: "muted" },
        ],
      },
    ],
  },
  {
    label: "Select",
    caption: "Selection workflows and edge refinement.",
    sections: [
      {
        title: "Global",
        items: [
          { label: "All",      shortcut: "Ctrl+A",       actionId: "select-all"      as const },
          { label: "Deselect", shortcut: "Ctrl+D",       actionId: "select-deselect" as const },
          { label: "Reselect", shortcut: "Ctrl+Shift+D", actionId: "select-reselect" as const },
          { label: "Inverse",  shortcut: "Ctrl+Shift+I", actionId: "select-invert"   as const },
        ],
      },
      {
        title: "Modify",
        items: [
          { label: "Feather...",          actionId: "select-feather"      as const },
          { label: "Expand...",           actionId: "select-expand"       as const },
          { label: "Contract...",         actionId: "select-contract"     as const },
          { label: "Smooth...",           actionId: "select-smooth"       as const },
          { label: "Border...",           actionId: "select-border"       as const },
          { label: "Transform Selection", actionId: "select-transform"    as const },
          { label: "Color Range...",      actionId: "select-color-range"  as const },
        ],
      },
      {
        title: "Refine",
        items: [
          { label: "Select and Mask", actionId: "select-and-mask" as const },
        ],
      },
    ],
  },
  {
    label: "Filter",
    caption: "Effect categories and future gallery entry points.",
    sections: [
      {
        title: "Recent",
        items: [
          { label: "Last Filter", shortcut: "Ctrl+F" },
          { label: "Fade Last Filter", tone: "muted" },
        ],
      },
      {
        title: "Families",
        items: [{ label: "Blur" }, { label: "Noise" }, { label: "Stylize" }],
      },
    ],
  },
  {
    label: "View",
    caption: "Viewport controls that mirror the current chrome.",
    sections: [
      {
        title: "Zoom",
        items: [
          { label: "Zoom In", shortcut: "Ctrl++", tone: "accent" },
          { label: "Zoom Out", shortcut: "Ctrl+-" },
          { label: "Fit on Screen", shortcut: "Ctrl+0" },
        ],
      },
      {
        title: "Overlays",
        items: [
          { label: "Pixel Grid" },
          { label: "Rulers" },
          { label: "Guides", actionId: "view-toggle-guides" },
        ],
      },
    ],
  },
  {
    label: "Window",
    caption: "Dock and workspace organization preview.",
    align: "right",
    sections: [
      {
        title: "Panels",
        items: [
          { label: "Layers", tone: "accent" },
          { label: "Navigator" },
          { label: "History" },
        ],
      },
      {
        title: "Workspace",
        items: [
          { label: "Essentials" },
          { label: "Painting" },
          { label: "Reset Workspace" },
        ],
      },
    ],
  },
  {
    label: "Help",
    caption: "Support, onboarding, and diagnostics preview.",
    align: "right",
    sections: [
      {
        title: "Learn",
        items: [
          { label: "Welcome Tour" },
          { label: "Keyboard Shortcuts" },
          { label: "What’s New" },
        ],
      },
      {
        title: "Support",
        items: [
          { label: "Report Feedback" },
          { label: "System Info" },
          { label: "Release Notes", tone: "muted" },
        ],
      },
    ],
  },
];

type EditorTool = ShortcutTool | "mixerBrush" | "type" | "shape" | "transform";
type MarqueeShape = "rect" | "ellipse" | "row" | "col";
type MarqueeStyle = "normal" | "fixed-ratio" | "fixed-size";
type LassoMode = "freehand" | "polygon" | "magnetic";
type WandMode = "magic" | "quick";

const toolItems: {
  id: EditorTool;
  label: string;
  Icon: typeof MoveToolIcon;
}[] = [
  { id: "move", label: "Move", Icon: MoveToolIcon },
  { id: "marquee", label: "Marquee", Icon: MarqueeToolIcon },
  { id: "lasso", label: "Lasso", Icon: LassoToolIcon },
  { id: "crop", label: "Crop", Icon: CropToolIcon },
  { id: "wand", label: "Wand", Icon: SelectionIcon },
  { id: "brush", label: "Brush", Icon: BrushToolIcon },
  { id: "mixerBrush", label: "Mixer Brush", Icon: BrushToolIcon },
  { id: "cloneStamp", label: "Clone Stamp", Icon: CopyIcon },
  { id: "historyBrush", label: "History Brush", Icon: UndoIcon },
  { id: "pencil", label: "Pencil", Icon: PencilToolIcon },
  { id: "eraser", label: "Eraser", Icon: EraserToolIcon },
  { id: "fill", label: "Fill", Icon: FillToolIcon },
  { id: "gradient", label: "Gradient", Icon: GradientToolIcon },
  { id: "eyedropper", label: "Eyedropper", Icon: EyedropperToolIcon },
  { id: "pen", label: "Pen", Icon: PenToolIcon },
  { id: "directSelect", label: "Direct Selection", Icon: DirectSelectIcon },
  { id: "type", label: "Type", Icon: TypeToolIcon },
  { id: "shape", label: "Shape", Icon: ShapeToolIcon },
  { id: "transform", label: "Transform", Icon: SlidersIcon },
  { id: "hand", label: "Hand", Icon: HandToolIcon },
  { id: "zoom", label: "Zoom", Icon: ZoomToolIcon },
];

const defaultDocumentDraft: CreateDocumentCommand = {
  name: "Untitled",
  width: 1920,
  height: 1080,
  resolution: 72,
  colorMode: "rgb",
  bitDepth: 8,
  background: "transparent",
};

const presets = [
  { id: "web", label: "Web", width: 1920, height: 1080, resolution: 72 },
  { id: "photo", label: "Photo", width: 4032, height: 3024, resolution: 300 },
  { id: "print", label: "Print", width: 2480, height: 3508, resolution: 300 },
  { id: "square", label: "Custom", width: 2048, height: 2048, resolution: 144 },
];

type DocumentUnit = "px" | "in" | "cm" | "mm";
type AuxPanel = "properties" | "adjustments" | "history" | "navigator" | "channels" | "brush" | "color" | "swatches";

const unitSteps: Record<DocumentUnit, number> = {
  px: 1,
  in: 0.01,
  cm: 0.1,
  mm: 1,
};

const RECENT_COLORS_KEY = "agogo:recent-colors";
const CUSTOM_SWATCHES_KEY = "agogo:custom-swatches";
const GRADIENT_STOPS_KEY = "agogo:gradient-stops";

function pixelsToUnit(pixels: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return pixels / resolution;
    case "cm":
      return (pixels / resolution) * 2.54;
    case "mm":
      return (pixels / resolution) * 25.4;
    default:
      return pixels;
  }
}

function unitToPixels(value: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return value * resolution;
    case "cm":
      return (value / 2.54) * resolution;
    case "mm":
      return (value / 25.4) * resolution;
    default:
      return value;
  }
}

function formatDimension(value: number, unit: DocumentUnit) {
  if (unit === "px" || unit === "mm") {
    return Math.round(value).toString();
  }
  return value.toFixed(2);
}

function loadColorList(key: string, fallback: Rgba[]): Rgba[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return fallback;
    }
    return parsed
      .map((entry) => {
        if (!Array.isArray(entry) || entry.length < 4) {
          return null;
        }
        return [
          Number(entry[0]),
          Number(entry[1]),
          Number(entry[2]),
          Number(entry[3]),
        ] as Rgba;
      })
      .filter((entry): entry is Rgba => entry !== null);
  } catch {
    return fallback;
  }
}

function loadGradientStops(key: string, fallback: GradientStopCommand[]): GradientStopCommand[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return fallback;
    }
    return parsed
      .map((entry) => {
        if (typeof entry !== "object" || entry === null) {
          return null;
        }
        const candidate = entry as { position?: unknown; color?: unknown };
        if (
          typeof candidate.position !== "number" ||
          !Array.isArray(candidate.color) ||
          candidate.color.length < 4
        ) {
          return null;
        }
        return {
          position: candidate.position,
          color: toMutableRgba(candidate.color),
        } satisfies GradientStopCommand;
      })
      .filter((entry): entry is GradientStopCommand => entry !== null);
  } catch {
    return fallback;
  }
}

function gradientStopsToCss(stops: GradientStopCommand[]) {
  const normalized = stops
    .map((stop) => ({
      position: Math.max(0, Math.min(1, stop.position)),
      color: stop.color,
    }))
    .sort((a, b) => a.position - b.position);
  if (normalized.length === 0) {
    return "linear-gradient(90deg, rgba(0, 0, 0, 1), rgba(255, 255, 255, 1))";
  }
  return `linear-gradient(90deg, ${normalized
    .map(
      (stop) =>
        `${rgbaToCss(toRgba(stop.color))} ${Math.round(stop.position * 100)}%`,
    )
    .join(", ")})`;
}

export default function App() {
  const engine = useEngine();
  const render = engine.render;
  const menuBarRef = useRef<HTMLDivElement | null>(null);
  const projectInputRef = useRef<HTMLInputElement | null>(null);
  const lastSavedVersionRef = useRef<number>(0);
  const [activeTool, setActiveTool] = useState<EditorTool>("marquee");
  const [marqueeShape, setMarqueeShape] = useState<MarqueeShape>("rect");
  const [marqueeStyle, setMarqueeStyle] = useState<MarqueeStyle>("normal");
  const [marqueeRatioW, setMarqueeRatioW] = useState(1);
  const [marqueeRatioH, setMarqueeRatioH] = useState(1);
  const [marqueeSizeW, setMarqueeSizeW] = useState(100);
  const [marqueeSizeH, setMarqueeSizeH] = useState(100);
  const [lassoMode, setLassoMode] = useState<LassoMode>("freehand");
  const [selectionAntiAlias, setSelectionAntiAlias] = useState(true);
  const [selectionFeatherRadius, setSelectionFeatherRadius] = useState(0);
  const [wandMode, setWandMode] = useState<WandMode>("magic");
  const [wandTolerance, setWandTolerance] = useState(24);
  const [wandContiguous, setWandContiguous] = useState(true);
  const [wandSampleMerged, setWandSampleMerged] = useState(false);
  const [moveAutoSelectGroup, setMoveAutoSelectGroup] = useState(false);
  const [transformRefPoint, setTransformRefPoint] = useState<[number, number]>([1, 1]);
  const [cropDeletePixels, setCropDeletePixels] = useState(false);
  const [selectedLayerIds, setSelectedLayerIds] = useState<string[]>([]);
  const [activeAuxPanel, setActiveAuxPanel] = useState<AuxPanel>("properties");
  const [newDocumentOpen, setNewDocumentOpen] = useState(false);
  const [canvasSizeOpen, setCanvasSizeOpen] = useState(false);
  const [openRecentOpen, setOpenRecentOpen] = useState(false);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [featherDialogOpen, setFeatherDialogOpen] = useState(false);
  const [featherDialogValue, setFeatherDialogValue] = useState(5);
  type ModifyKind = "expand" | "contract" | "smooth" | "border";
  const [modifyDialog, setModifyDialog] = useState<{ open: boolean; kind: ModifyKind; value: number }>({ open: false, kind: "expand", value: 4 });
  const openModifyDialog = (kind: ModifyKind) =>
    setModifyDialog({ open: true, kind, value: kind === "smooth" ? 2 : 4 });
  const [colorRangeOpen, setColorRangeOpen] = useState(false);
  const [colorRangeColor, setColorRangeColor] = useState<Rgba>([128, 128, 128, 255]);
  const [colorRangeFuzziness, setColorRangeFuzziness] = useState(40);
  const [colorRangeSampleMerged, setColorRangeSampleMerged] = useState(false);
  const [transformSelectionActive, setTransformSelectionActive] = useState(false);
  const [selectAndMaskOpen, setSelectAndMaskOpen] = useState(false);
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [draft, setDraft] =
    useState<CreateDocumentCommand>(defaultDocumentDraft);
  const [canvasSizeDraft, setCanvasSizeDraft] = useState<{
    width: number;
    height: number;
    anchor: "top-left" | "top-center" | "top-right" | "middle-left" | "center" | "middle-right" | "bottom-left" | "bottom-center" | "bottom-right";
  }>({
    width: 0,
    height: 0,
    anchor: "center",
  });
  const [cursor, setCursor] = useState<{ x: number; y: number } | null>(null);
  const [zoomMenuOpen, setZoomMenuOpen] = useState(false);
  const zoomClickTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [isPanMode, setIsPanMode] = useState(false);
  const [showGuides, setShowGuides] = useState(false);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [panelWidth, setPanelWidth] = useState(328);
  const [documentUnit, setDocumentUnit] = useState<DocumentUnit>("px");
  const [layerThumbnails, setLayerThumbnails] = useState<
    Record<string, ThumbnailEntry>
  >({});
  const [isDragOver, setIsDragOver] = useState(false);
  const [foregroundColor, setForegroundColor] = useState<Rgba>([0, 0, 0, 255]);
  const [backgroundColor, setBackgroundColor] = useState<Rgba>([255, 255, 255, 255]);
  const [colorPickerOpen, setColorPickerOpen] = useState(false);
  const [colorPickerTarget, setColorPickerTarget] = useState<"foreground" | "background">("foreground");
  const [colorChannelMode, setColorChannelMode] = useState<ColorChannelMode>("rgb");
  const [onlyWebColors, setOnlyWebColors] = useState(false);
  const [recentColors, setRecentColors] = useState<Rgba[]>(() =>
    loadColorList(RECENT_COLORS_KEY, [
      [0, 0, 0, 255],
      [255, 255, 255, 255],
      [56, 189, 248, 255],
      [244, 63, 94, 255],
    ]),
  );
  const [swatches, setSwatches] = useState<Rgba[]>(() =>
    loadColorList(CUSTOM_SWATCHES_KEY, [
      [0, 0, 0, 255],
      [255, 255, 255, 255],
      [244, 114, 182, 255],
      [59, 130, 246, 255],
      [34, 197, 94, 255],
      [251, 191, 36, 255],
    ]),
  );
  const [brushSize, setBrushSize] = useState(20);
  const [brushHardness, setBrushHardness] = useState(0.8);
  const [brushAngle, setBrushAngle] = useState(0);
  const [brushRoundness, setBrushRoundness] = useState(0.75);
  const [brushSpacing, setBrushSpacing] = useState(0.14);
  const [brushTipShape, setBrushTipShape] = useState<BrushTipShape>("round");
  const [brushPresetId, setBrushPresetId] = useState(BRUSH_PRESETS[0].id);
  const [brushSizeJitter, setBrushSizeJitter] = useState(0);
  const [brushOpacityJitter, setBrushOpacityJitter] = useState(0);
  const [brushFlowJitter, setBrushFlowJitter] = useState(0);
  const [brushControlSource, setBrushControlSource] = useState<BrushControlSource>("pressure");
  const [mixerBrushMix, setMixerBrushMix] = useState(0.65);
  const [mixerBrushSampleMerged, setMixerBrushSampleMerged] = useState(true);
  const [cloneStampSampleMerged, setCloneStampSampleMerged] = useState(true);
  const [cloneStampSource, setCloneStampSource] = useState<{ x: number; y: number } | null>(null);
  const [historyBrushSampleMerged, setHistoryBrushSampleMerged] = useState(true);
  const [pencilAutoErase, setPencilAutoErase] = useState(false);
  const [eraserMode, setEraserMode] = useState<"normal" | "background" | "magic">("normal");
  const [eraserTolerance, setEraserTolerance] = useState(30);
  const [brushFlow, setBrushFlow] = useState(1.0);
  const [fillSource, setFillSource] = useState<FillSource>("foreground");
  const [fillTolerance, setFillTolerance] = useState(24);
  const [fillContiguous, setFillContiguous] = useState(true);
  const [fillSampleMerged, setFillSampleMerged] = useState(false);
  const [fillCreateLayer, setFillCreateLayer] = useState(false);
  const [fillDialogOpen, setFillDialogOpen] = useState(false);
  const [gradientType, setGradientType] = useState<GradientType>("linear");
  const [gradientReverse, setGradientReverse] = useState(false);
  const [gradientDither, setGradientDither] = useState(false);
  const [gradientCreateLayer, setGradientCreateLayer] = useState(true);
  const [gradientStops, setGradientStops] = useState<GradientStopCommand[]>(() =>
    loadGradientStops(GRADIENT_STOPS_KEY, [
      { position: 0, color: toMutableRgba(foregroundColor) },
      { position: 1, color: toMutableRgba(backgroundColor) },
    ]),
  );
  const [gradientEditorOpen, setGradientEditorOpen] = useState(false);
  const [thresholdDialogOpen, setThresholdDialogOpen] = useState(false);
  const [thresholdValue, setThresholdValue] = useState(128);
  const [posterizeDialogOpen, setPosterizeDialogOpen] = useState(false);
  const [posterizeLevels, setPosterizeLevels] = useState(6);
  const [channelMixerDialogOpen, setChannelMixerDialogOpen] = useState(false);
  const [channelMixerMonochrome, setChannelMixerMonochrome] = useState(false);
  const [channelMixerMatrix, setChannelMixerMatrix] = useState<{
    red: [number, number, number];
    green: [number, number, number];
    blue: [number, number, number];
  }>({
    red: [100, 0, 0],
    green: [0, 100, 0],
    blue: [0, 0, 100],
  });
  const [selectiveColorDialogOpen, setSelectiveColorDialogOpen] = useState(false);
  const [selectiveColorMode, setSelectiveColorMode] = useState<"relative" | "absolute">("relative");
  const [selectiveColorAdjustments, setSelectiveColorAdjustments] = useState({
    reds: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    yellows: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    greens: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    cyans: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blues: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    magentas: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    whites: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    neutrals: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
    blacks: { cyanRed: 0, magentaGreen: 0, yellowBlue: 0, black: 0 },
  });
  const [photoFilterDialogOpen, setPhotoFilterDialogOpen] = useState(false);
  const [photoFilterColor, setPhotoFilterColor] = useState<[number, number, number, number]>([255, 190, 120, 255]);
  const [photoFilterDensity, setPhotoFilterDensity] = useState(40);
  const [photoFilterPreserveLuminosity, setPhotoFilterPreserveLuminosity] = useState(true);
  const [gradientMapDialogOpen, setGradientMapDialogOpen] = useState(false);
  const [eyedropperSampleSize, setEyedropperSampleSize] = useState(1);
  const [eyedropperSampleMerged, setEyedropperSampleMerged] = useState(true);
  const [eyedropperSampleAllLayersNoAdj, setEyedropperSampleAllLayersNoAdj] = useState(false);
  const [hasAutosave, setHasAutosave] = useState(() => {
    return localStorage.getItem(AUTOSAVE_KEY) !== null;
  });

  const contentVersion = render?.uiMeta.contentVersion;
  useEffect(() => {
    if (contentVersion === undefined || !engine.handle) {
      return;
    }
    const result = engine.dispatchCommand(CommandID.GetLayerThumbnails);
    if (result?.thumbnails) {
      setLayerThumbnails(result.thumbnails);
    }
  }, [contentVersion, engine.dispatchCommand, engine.handle]);

  useEffect(() => {
    if (
      !engine.handle ||
      contentVersion === undefined ||
      contentVersion === 0
    ) {
      return;
    }
    if (
      contentVersion - lastSavedVersionRef.current <
      AUTOSAVE_EVERY_N_VERSIONS
    ) {
      return;
    }
    const base64Zip = engine.exportProject();
    if (!base64Zip) {
      return;
    }
    try {
      localStorage.setItem(AUTOSAVE_KEY, base64Zip);
      lastSavedVersionRef.current = contentVersion;
    } catch {
      // localStorage quota exceeded — silently skip
    }
  }, [contentVersion, engine.exportProject, engine.handle]);

  useEffect(() => {
    if (!engine.handle) return;
    engine.dispatchCommand(CommandID.SetForegroundColor, {
      color: toMutableRgba(foregroundColor),
    } satisfies SetColorCommand);
  }, [engine.handle, engine.dispatchCommand, foregroundColor]);

  useEffect(() => {
    if (!engine.handle) return;
    engine.dispatchCommand(CommandID.SetBackgroundColor, {
      color: toMutableRgba(backgroundColor),
    } satisfies SetColorCommand);
  }, [engine.handle, engine.dispatchCommand, backgroundColor]);

  useEffect(() => {
    try {
      window.localStorage.setItem(RECENT_COLORS_KEY, JSON.stringify(recentColors));
    } catch {
      // Ignore localStorage failures.
    }
  }, [recentColors]);

  useEffect(() => {
    try {
      window.localStorage.setItem(CUSTOM_SWATCHES_KEY, JSON.stringify(swatches));
    } catch {
      // Ignore localStorage failures.
    }
  }, [swatches]);

  useEffect(() => {
    try {
      window.localStorage.setItem(GRADIENT_STOPS_KEY, JSON.stringify(gradientStops));
    } catch {
      // Ignore localStorage failures.
    }
  }, [gradientStops]);

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const activeDocumentName = render?.uiMeta.activeDocumentName ?? draft.name;
  const fillSourceName =
    fillSource === "foreground" ? "Color" : fillSource === "background" ? "Background" : "Pattern";
  const fillModeSummary = `${fillSourceName} fill · ${fillContiguous ? "contiguous" : "all matching"} · ${fillSampleMerged ? "sample merged" : "active layer"} · ${fillCreateLayer ? "new layer" : "paint in place"}`;
  const gradientModeSummary = `${gradientType.charAt(0).toUpperCase() + gradientType.slice(1)} · ${gradientStops.length} stops · ${gradientReverse ? "reversed" : "forward"} · ${gradientDither ? "dither" : "no dither"} · ${gradientCreateLayer ? "new layer" : "paint in place"}`;
  const eyedropperModeSummary = `${eyedropperSampleSize === 1 ? "Point sample" : `${eyedropperSampleSize}x${eyedropperSampleSize} average`} · ${eyedropperSampleMerged ? "sample merged" : "active layer"} · ${eyedropperSampleAllLayersNoAdj ? "no adjustments" : "with adjustments"}`;
  const gradientPreviewStyle = {
    backgroundImage: gradientStopsToCss(gradientStops),
  };
  const channelMixerRows = [
    { key: "red", label: "Red Output" },
    { key: "green", label: "Green Output" },
    { key: "blue", label: "Blue Output" },
  ] as const;
  const channelMixerColumns = [
    { index: 0, label: "Source Red" },
    { index: 1, label: "Source Green" },
    { index: 2, label: "Source Blue" },
  ] as const;
  const selectiveColorRanges = [
    { key: "reds", label: "Reds" },
    { key: "yellows", label: "Yellows" },
    { key: "greens", label: "Greens" },
    { key: "cyans", label: "Cyans" },
    { key: "blues", label: "Blues" },
    { key: "magentas", label: "Magentas" },
    { key: "whites", label: "Whites" },
    { key: "neutrals", label: "Neutrals" },
    { key: "blacks", label: "Blacks" },
  ] as const;
  const selectiveColorFields = [
    { key: "cyanRed", label: "Cyan / Red" },
    { key: "magentaGreen", label: "Magenta / Green" },
    { key: "yellowBlue", label: "Yellow / Blue" },
    { key: "black", label: "Black" },
  ] as const;

  const openProjectPicker = () => {
    projectInputRef.current?.click();
  };

  const pushRecentColor = (color: Rgba) => {
    const normalized = color;
    setRecentColors((current) => {
      const withoutDuplicate = current.filter((entry) => !entry.every((value, index) => value === normalized[index]));
      return [normalized, ...withoutDuplicate].slice(0, 10);
    });
  };

  const applyColorToTarget = (target: "foreground" | "background", color: Rgba) => {
    const next = onlyWebColors ? snapToWebSafeColor(color) : color;
    if (target === "foreground") {
      setForegroundColor(next);
    } else {
      setBackgroundColor(next);
    }
    pushRecentColor(next);
  };

  const openColorPicker = (target: "foreground" | "background") => {
    setColorPickerTarget(target);
    setColorPickerOpen(true);
  };

  const activateTool = (tool: EditorTool) => {
    if (tool === "fill" && activeTool === "fill") {
      tool = "gradient";
    } else if (tool === "gradient" && activeTool === "gradient") {
      tool = "fill";
    }
    if (tool === activeTool) {
      return;
    }

    // Cancel active special modes when switching away
    if (activeTool === "crop" && tool !== "hand" && tool !== "zoom") {
      engine.dispatchCommand(CommandID.CancelCrop, {});
      setCropDeletePixels(false);
    }
    if (activeTool === "transform" && tool !== "hand" && tool !== "zoom") {
      engine.dispatchCommand(CommandID.CancelFreeTransform, {});
    }
    if (
      (activeTool === "pen" || activeTool === "directSelect") &&
      tool !== "pen" &&
      tool !== "directSelect" &&
      tool !== "hand" &&
      tool !== "zoom"
    ) {
      engine.dispatchCommand(CommandID.SetActiveTool, { tool: "" });
    }

    setActiveTool(tool);
    if (tool !== "hand") {
      setIsPanMode(false);
    }

    // Begin special modes
    if (tool === "crop") {
      engine.dispatchCommand(CommandID.BeginCrop, {});
    }
    if (tool === "pen") {
      engine.dispatchCommand(CommandID.SetActiveTool, { tool: "pen" });
    }
    if (tool === "directSelect") {
      engine.dispatchCommand(CommandID.SetActiveTool, {
        tool: "direct-select",
      });
    }
  };

  const openNewDocumentDialog = () => {
    setNewDocumentOpen(true);
  };

  const createAdjustmentLayer = <K extends AdjustmentKind>(
    name: string,
    adjustmentKind: K,
    params: AdjustmentLayerParams<K> = {} as AdjustmentLayerParams<K>,
  ) => {
    if (!render?.uiMeta.activeLayerId) {
      return;
    }
    const position = findLayerPositionInTree(
      render.uiMeta.layers,
      render.uiMeta.activeLayerId,
    );
    if (!position) {
      return;
    }
    engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "adjustment",
      name,
      adjustmentKind,
      params,
      parentLayerId: position.parentId,
      index: position.index + 1,
    } satisfies AddLayerCommand);
  };

  const saveProject = () => {
    const base64Zip = engine.exportProject();
    if (!base64Zip) {
      return;
    }
    const binary = atob(base64Zip);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    const fileName = `${activeDocumentName}.agp`;
    const blob = new Blob([bytes], { type: "application/zip" });
    downloadBlob(blob, fileName);
  };

  const openProject = async (file: File) => {
    const buffer = await file.arrayBuffer();
    const bytes = new Uint8Array(buffer);
    let payload: string;
    // PK magic bytes 0x50 0x4B = ZIP file
    if (bytes[0] === 0x50 && bytes[1] === 0x4b) {
      const chunkSize = 0x8000;
      let binary = "";
      for (let i = 0; i < bytes.length; i += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
      }
      payload = btoa(binary);
    } else {
      // Legacy JSON — pass as plain text
      payload = new TextDecoder().decode(bytes);
    }
    const imported = engine.importProject(payload);
    if (imported) {
      setDraft((current) => ({
        ...current,
        name: imported.uiMeta.activeDocumentName || current.name,
        width: imported.uiMeta.documentWidth || current.width,
        height: imported.uiMeta.documentHeight || current.height,
        background: imported.uiMeta
          .documentBackground as CreateDocumentCommand["background"],
      }));
    }
  };

  const openImageFile = async (file: File) => {
    const bitmap = await createImageBitmap(file);
    const { width, height } = bitmap;
    const canvas = new OffscreenCanvas(width, height);
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.drawImage(bitmap, 0, 0);
    bitmap.close();
    const imageData = ctx.getImageData(0, 0, width, height);
    const data = imageData.data;
    const chunkSize = 0x8000;
    let binary = "";
    for (let i = 0; i < data.length; i += chunkSize) {
      binary += String.fromCharCode(...data.subarray(i, i + chunkSize));
    }
    const result = engine.dispatchCommand(CommandID.OpenImageFile, {
      name: file.name,
      width,
      height,
      pixels: btoa(binary),
    });
    if (result) {
      setDraft((current) => ({
        ...current,
        name: result.uiMeta.activeDocumentName || file.name,
        width,
        height,
      }));
    }
  };

  const recoverAutosave = () => {
    const saved = localStorage.getItem(AUTOSAVE_KEY);
    if (!saved) {
      setHasAutosave(false);
      return;
    }
    const imported = engine.importProject(saved);
    if (imported) {
      setDraft((current) => ({
        ...current,
        name: imported.uiMeta.activeDocumentName || current.name,
        width: imported.uiMeta.documentWidth || current.width,
        height: imported.uiMeta.documentHeight || current.height,
        background: imported.uiMeta
          .documentBackground as CreateDocumentCommand["background"],
      }));
    }
    localStorage.removeItem(AUTOSAVE_KEY);
    setHasAutosave(false);
  };

  const dismissAutosave = () => {
    localStorage.removeItem(AUTOSAVE_KEY);
    setHasAutosave(false);
  };

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer.types.includes("Files")) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (event: React.DragEvent) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) {
      setIsDragOver(false);
    }
  };

  const handleDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    setIsDragOver(false);
    const file = event.dataTransfer.files[0];
    if (!file) return;
    if (file.name.endsWith(".agp") || file.type === "application/json") {
      await openProject(file);
    } else if (file.type.startsWith("image/")) {
      await openImageFile(file);
    }
  };

  const checkedMenuActionIds = new Set<MenuActionId>(
    showGuides ? (["view-toggle-guides"] as MenuActionId[]) : [],
  );

  const isMenuActionDisabled = (actionId: MenuActionId) => {
    switch (actionId) {
      case "save-project":
      case "export-project":
      case "generate-assets":
      case "canvas-size":
        return !render || actionId === "generate-assets";
      case "image-invert":
      case "image-channel-mixer":
      case "image-threshold":
      case "image-posterize":
      case "image-selective-color":
      case "image-photo-filter":
      case "image-gradient-map":
        return !render?.uiMeta.activeLayerId;
      case "transform-free":
      case "transform-scale":
      case "transform-rotate":
      case "transform-skew":
      case "transform-distort":
      case "transform-perspective":
      case "transform-warp":
        return !render?.uiMeta.activeLayerId;
      case "transform-flip-h":
      case "transform-flip-v":
      case "transform-rotate-cw":
      case "transform-rotate-ccw":
      case "transform-rotate-180":
        return !render?.uiMeta.activeLayerId;
      case "select-all":
      case "select-deselect":
      case "select-reselect":
      case "select-invert":
      case "select-feather":
      case "select-expand":
      case "select-contract":
      case "select-smooth":
      case "select-border":
      case "select-transform":
      case "select-color-range":
      case "select-and-mask":
        return !render;
      case "edit-fill":
        return !render?.uiMeta.activeLayerId;
      default:
        return false;
    }
  };

  const isMenuItemDisabled = (item: MenuPreviewItem) => {
    if (item.disabled) {
      return true;
    }
    if (!item.actionId) {
      return true;
    }
    return isMenuActionDisabled(item.actionId);
  };

  const commitFeather = () => {
    engine.dispatchCommand(CommandID.FeatherSelection, { radius: featherDialogValue });
    setFeatherDialogOpen(false);
  };

  const commitModify = () => {
    const { kind, value } = modifyDialog;
    switch (kind) {
      case "expand":
        engine.dispatchCommand(CommandID.ExpandSelection, { pixels: value });
        break;
      case "contract":
        engine.dispatchCommand(CommandID.ContractSelection, { pixels: value });
        break;
      case "smooth":
        engine.dispatchCommand(CommandID.SmoothSelection, { radius: value });
        break;
      case "border":
        engine.dispatchCommand(CommandID.BorderSelection, { width: value });
        break;
    }
    setModifyDialog((d) => ({ ...d, open: false }));
  };

  const commitColorRange = () => {
    engine.dispatchCommand(CommandID.SelectColorRange, {
      targetColor: toMutableRgba(colorRangeColor),
      fuzziness: colorRangeFuzziness,
      sampleMerged: colorRangeSampleMerged,
      mode: "replace",
    });
    setColorRangeOpen(false);
  };

  const handleMenuAction = (actionId: MenuActionId) => {
    if (isMenuActionDisabled(actionId)) {
      return;
    }

    setOpenMenu(null);

    switch (actionId) {
      case "new-document":
        openNewDocumentDialog();
        break;
      case "open-project":
        openProjectPicker();
        break;
      case "open-recent":
        setOpenRecentOpen(true);
        break;
      case "save-project":
        saveProject();
        break;
      case "export-project":
        setExportDialogOpen(true);
        break;
      case "canvas-size":
        setCanvasSizeDraft({
          width: render?.uiMeta.documentWidth ?? draft.width,
          height: render?.uiMeta.documentHeight ?? draft.height,
          anchor: "center",
        });
        setCanvasSizeOpen(true);
        break;
      case "transform-free":
      case "transform-scale":
      case "transform-rotate":
      case "transform-skew":
      case "transform-distort":
      case "transform-perspective":
        setActiveTool("transform");
        setTransformRefPoint([1, 1]);
        engine.dispatchCommand(CommandID.BeginFreeTransform, {});
        break;
      case "transform-warp":
        setActiveTool("transform");
        setTransformRefPoint([1, 1]);
        engine.dispatchCommand(CommandID.BeginFreeTransform, { mode: "warp" });
        break;
      case "transform-flip-h":
        engine.dispatchCommand(CommandID.FlipLayerH, {});
        break;
      case "transform-flip-v":
        engine.dispatchCommand(CommandID.FlipLayerV, {});
        break;
      case "transform-rotate-cw":
        engine.dispatchCommand(CommandID.RotateLayer90CW, {});
        break;
      case "transform-rotate-ccw":
        engine.dispatchCommand(CommandID.RotateLayer90CCW, {});
        break;
      case "transform-rotate-180":
        engine.dispatchCommand(CommandID.RotateLayer180, {});
        break;
      case "select-all":
        engine.selectAll();
        break;
      case "select-deselect":
        engine.deselect();
        break;
      case "select-reselect":
        engine.reselect();
        break;
      case "select-invert":
        engine.invertSelection();
        break;
      case "select-feather":
        setFeatherDialogOpen(true);
        break;
      case "select-expand":
        openModifyDialog("expand");
        break;
      case "select-contract":
        openModifyDialog("contract");
        break;
      case "select-smooth":
        openModifyDialog("smooth");
        break;
      case "select-border":
        openModifyDialog("border");
        break;
      case "select-transform":
        setTransformSelectionActive(true);
        break;
      case "select-color-range":
        setColorRangeOpen(true);
        break;
      case "select-and-mask":
        setSelectAndMaskOpen(true);
        break;
      case "edit-fill":
        setFillDialogOpen(true);
        break;
      case "image-invert":
        createAdjustmentLayer("Invert", "invert");
        break;
      case "image-channel-mixer":
        setChannelMixerDialogOpen(true);
        break;
      case "image-threshold":
        setThresholdDialogOpen(true);
        break;
      case "image-posterize":
        setPosterizeDialogOpen(true);
        break;
      case "image-selective-color":
        setSelectiveColorDialogOpen(true);
        break;
      case "image-photo-filter":
        setPhotoFilterDialogOpen(true);
        break;
      case "image-gradient-map":
        setGradientMapDialogOpen(true);
        break;
      case "view-toggle-guides": {
        const next = !showGuides;
        setShowGuides(next);
        engine.setShowGuides(next);
        break;
      }
      default:
        break;
    }
  };

  useKeyboardShortcuts({
    onPanModeChange: setIsPanMode,
    onNewDocument() {
      openNewDocumentDialog();
    },
    onOpenDocument() {
      openProjectPicker();
    },
    onSaveDocument() {
      if (!isMenuActionDisabled("save-project")) {
        saveProject();
      }
    },
    onExportDocument() {
      if (!isMenuActionDisabled("export-project")) {
        setExportDialogOpen(true);
      }
    },
    onZoomIn() {
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom * 1.1);
    },
    onZoomOut() {
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom / 1.1);
    },
    onFitToView() {
      engine.fitToView();
    },
    onUndo() {
      engine.undo();
    },
    onRedo() {
      engine.redo();
    },
    onSelectAll() {
      engine.selectAll();
    },
    onDeselect() {
      engine.deselect();
    },
    onInvertSelection() {
      engine.invertSelection();
    },
    onToolSelect(tool: ShortcutTool) {
      activateTool(tool);
    },
    onBeginTransform() {
      setActiveTool("transform");
      setTransformRefPoint([1, 1]);
      engine.dispatchCommand(CommandID.BeginFreeTransform, {});
    },
    onNudgeLayer(dx: number, dy: number) {
      if (!render?.uiMeta.activeLayerId) {
        return;
      }
      const activeLayer = findLayerMetaInTree(
        render.uiMeta.layers,
        render.uiMeta.activeLayerId,
      );
      if (
        !activeLayer ||
        activeLayer.lockMode === "position" ||
        activeLayer.lockMode === "all"
      ) {
        return;
      }
      engine.translateLayer({ dx, dy });
    },
    onBrushSizeChange(delta: number) {
      setBrushSize((prev) => {
        const step = prev < 10 ? 1 : prev < 100 ? 5 : 10;
        return Math.max(1, Math.min(2500, prev + delta * step));
      });
    },
    onBrushHardnessChange(delta: number) {
      setBrushHardness((prev) =>
        Math.max(0, Math.min(1, Math.round((prev + delta) * 100) / 100)),
      );
    },
    onSwapColors() {
      setForegroundColor(backgroundColor);
      setBackgroundColor(foregroundColor);
    },
    onResetColors() {
      setForegroundColor([0, 0, 0, 255]);
      setBackgroundColor([255, 255, 255, 255]);
    },
  });

  useEffect(() => {
    if (!openMenu) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuBarRef.current?.contains(event.target as Node)) {
        setOpenMenu(null);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpenMenu(null);
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [openMenu]);

  const hasDocument = (render?.uiMeta.documentWidth ?? 0) > 0;
  const documentSize = render
    ? `${render.uiMeta.documentWidth} x ${render.uiMeta.documentHeight}`
    : "No document";
  const zoomPercent = render
    ? `${Math.round(render.viewport.zoom * 100)}%`
    : "0%";
  const cursorText = cursor ? `${cursor.x}, ${cursor.y}` : "—";

  function handleZoomClick() {
    if (zoomClickTimerRef.current) return;
    zoomClickTimerRef.current = setTimeout(() => {
      zoomClickTimerRef.current = null;
      setZoomMenuOpen((prev) => !prev);
    }, 200);
  }

  function handleZoomDoubleClick() {
    if (zoomClickTimerRef.current) {
      clearTimeout(zoomClickTimerRef.current);
      zoomClickTimerRef.current = null;
    }
    engine.setZoom(1);
    setZoomMenuOpen(false);
  }
  const historyEntries = render?.uiMeta.history ?? [];
  const currentHistoryIndex = render?.uiMeta.currentHistoryIndex ?? 0;
  const widthValue = formatDimension(
    pixelsToUnit(draft.width, draft.resolution, documentUnit),
    documentUnit,
  );
  const heightValue = formatDimension(
    pixelsToUnit(draft.height, draft.resolution, documentUnit),
    documentUnit,
  );
  const activeColor = colorPickerTarget === "foreground" ? foregroundColor : backgroundColor;
  const setActiveColor = (next: Rgba) => applyColorToTarget(colorPickerTarget, next);
  const selectionToolOptions =
    activeTool === "move" ? (
      <ToolChoiceButton
        active={moveAutoSelectGroup}
        onClick={() => setMoveAutoSelectGroup((v) => !v)}
      >
        Groups
      </ToolChoiceButton>
    ) : activeTool === "marquee" ? (
      <>
        <ToolOptionGroup label="Shape">
          <ToolChoiceButton
            active={marqueeShape === "rect"}
            onClick={() => setMarqueeShape("rect")}
          >
            Rect
          </ToolChoiceButton>
          <ToolChoiceButton
            active={marqueeShape === "ellipse"}
            onClick={() => setMarqueeShape("ellipse")}
          >
            Ellipse
          </ToolChoiceButton>
          <ToolChoiceButton
            active={marqueeShape === "row"}
            onClick={() => setMarqueeShape("row")}
          >
            Row
          </ToolChoiceButton>
          <ToolChoiceButton
            active={marqueeShape === "col"}
            onClick={() => setMarqueeShape("col")}
          >
            Col
          </ToolChoiceButton>
        </ToolOptionGroup>
        {(marqueeShape === "rect" || marqueeShape === "ellipse") ? (
          <ToolOptionGroup label="Style">
            <ToolChoiceButton
              active={marqueeStyle === "normal"}
              onClick={() => setMarqueeStyle("normal")}
            >
              Normal
            </ToolChoiceButton>
            <ToolChoiceButton
              active={marqueeStyle === "fixed-ratio"}
              onClick={() => setMarqueeStyle("fixed-ratio")}
            >
              Fixed Ratio
            </ToolChoiceButton>
            <ToolChoiceButton
              active={marqueeStyle === "fixed-size"}
              onClick={() => setMarqueeStyle("fixed-size")}
            >
              Fixed Size
            </ToolChoiceButton>
          </ToolOptionGroup>
        ) : null}
        {marqueeStyle === "fixed-ratio" && (marqueeShape === "rect" || marqueeShape === "ellipse") ? (
          <>
            <ToolNumberField
              label="W"
              min={0.01}
              max={9999}
              step={1}
              value={marqueeRatioW}
              onChange={setMarqueeRatioW}
            />
            <ToolNumberField
              label="H"
              min={0.01}
              max={9999}
              step={1}
              value={marqueeRatioH}
              onChange={setMarqueeRatioH}
            />
          </>
        ) : null}
        {marqueeStyle === "fixed-size" && (marqueeShape === "rect" || marqueeShape === "ellipse") ? (
          <>
            <ToolNumberField
              label="W px"
              min={1}
              max={99999}
              step={1}
              value={marqueeSizeW}
              onChange={setMarqueeSizeW}
            />
            <ToolNumberField
              label="H px"
              min={1}
              max={99999}
              step={1}
              value={marqueeSizeH}
              onChange={setMarqueeSizeH}
            />
          </>
        ) : null}
        <ToolNumberField
          label="Feather"
          min={0}
          max={128}
          step={1}
          value={selectionFeatherRadius}
          onChange={setSelectionFeatherRadius}
        />
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
      </>
    ) : activeTool === "crop" ? (
      <>
        <ToolNumberField
          label="W"
          min={1}
          max={99999}
          step={1}
          value={Math.round(render?.uiMeta.crop?.w ?? 0)}
          onChange={(v) => {
            if (render?.uiMeta.crop) {
              engine.dispatchCommand(CommandID.UpdateCrop, {
                ...render.uiMeta.crop,
                w: v,
                rotation: render.uiMeta.crop.rotation ?? 0,
                deletePixels: cropDeletePixels,
              });
            }
          }}
        />
        <ToolNumberField
          label="H"
          min={1}
          max={99999}
          step={1}
          value={Math.round(render?.uiMeta.crop?.h ?? 0)}
          onChange={(v) => {
            if (render?.uiMeta.crop) {
              engine.dispatchCommand(CommandID.UpdateCrop, {
                ...render.uiMeta.crop,
                h: v,
                rotation: render.uiMeta.crop.rotation ?? 0,
                deletePixels: cropDeletePixels,
              });
            }
          }}
        />
        <label className="ml-3 flex items-center gap-1 text-[10px]">
          <input
            type="checkbox"
            checked={cropDeletePixels}
            onChange={(e) => {
              setCropDeletePixels(e.target.checked);
              if (render?.uiMeta.crop) {
                engine.dispatchCommand(CommandID.UpdateCrop, {
                  ...render.uiMeta.crop,
                  deletePixels: e.target.checked,
                });
              }
            }}
          />
          Delete
        </label>
        <Button
          size="sm"
          className="ml-2 h-6 px-3 text-[10px]"
          onClick={() => engine.dispatchCommand(CommandID.CommitCrop)}
        >
          Apply
        </Button>
        <Button
          variant="secondary"
          size="sm"
          className="ml-1 h-6 px-3 text-[10px]"
          onClick={() => engine.dispatchCommand(CommandID.CancelCrop)}
        >
          Cancel
        </Button>
      </>
    ) : activeTool === "lasso" ? (
      <>
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={lassoMode === "freehand"}
            onClick={() => setLassoMode("freehand")}
          >
            Freehand
          </ToolChoiceButton>
          <ToolChoiceButton
            active={lassoMode === "polygon"}
            onClick={() => setLassoMode("polygon")}
          >
            Polygon
          </ToolChoiceButton>
          <ToolChoiceButton
            active={lassoMode === "magnetic"}
            onClick={() => setLassoMode("magnetic")}
          >
            Magnetic
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Feather"
          min={0}
          max={128}
          step={1}
          value={selectionFeatherRadius}
          onChange={setSelectionFeatherRadius}
        />
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
      </>
    ) : activeTool === "wand" ? (
      <>
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={wandMode === "magic"}
            onClick={() => setWandMode("magic")}
          >
            Magic
          </ToolChoiceButton>
          <ToolChoiceButton
            active={wandMode === "quick"}
            onClick={() => setWandMode("quick")}
          >
            Quick
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Tolerance"
          min={0}
          max={255}
          step={1}
          value={wandTolerance}
          onChange={setWandTolerance}
        />
        {wandMode === "magic" ? (
          <ToolChoiceButton
            active={wandContiguous}
            onClick={() => setWandContiguous((current) => !current)}
          >
            Contiguous
          </ToolChoiceButton>
        ) : null}
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
        <ToolChoiceButton
          active={wandSampleMerged}
          onClick={() => setWandSampleMerged((current) => !current)}
        >
          Sample all layers
        </ToolChoiceButton>
      </>
    ) : activeTool === "transform" ? (
      render?.uiMeta.freeTransform?.active ? (
        <>
          <TransformRefGrid
            active={transformRefPoint}
            onChange={(row, col) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const [px, py] = refPointToPivot(ft.corners, row, col);
              setTransformRefPoint([row, col]);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, {
                a: ft.a, b: ft.b, c: ft.c, d: ft.d,
                tx: ft.tx, ty: ft.ty,
                pivotX: px, pivotY: py,
                interpolation: ft.interpolation as InterpolMode,
              });
            }}
          />
          <ToolNumberField
            label="X"
            min={-99999}
            max={99999}
            step={1}
            value={Math.round(render.uiMeta.freeTransform.tx)}
            onChange={(value) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const updated = applyTransformFieldChange(ft, "x", value);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, { ...updated, pivotX: ft.pivotX, pivotY: ft.pivotY, interpolation: ft.interpolation as InterpolMode, ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}) });
            }}
          />
          <ToolNumberField
            label="Y"
            min={-99999}
            max={99999}
            step={1}
            value={Math.round(render.uiMeta.freeTransform.ty)}
            onChange={(value) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const updated = applyTransformFieldChange(ft, "y", value);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, { ...updated, pivotX: ft.pivotX, pivotY: ft.pivotY, interpolation: ft.interpolation as InterpolMode, ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}) });
            }}
          />
          <ToolNumberField
            label="W%"
            min={-99999}
            max={99999}
            step={1}
            value={Math.round(render.uiMeta.freeTransform.scaleX * 100)}
            onChange={(value) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const updated = applyTransformFieldChange(ft, "w", value);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, { ...updated, pivotX: ft.pivotX, pivotY: ft.pivotY, interpolation: ft.interpolation as InterpolMode, ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}) });
            }}
          />
          <ToolNumberField
            label="H%"
            min={-99999}
            max={99999}
            step={1}
            value={Math.round(render.uiMeta.freeTransform.scaleY * 100)}
            onChange={(value) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const updated = applyTransformFieldChange(ft, "h", value);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, { ...updated, pivotX: ft.pivotX, pivotY: ft.pivotY, interpolation: ft.interpolation as InterpolMode, ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}) });
            }}
          />
          <ToolNumberField
            label="°"
            min={-360}
            max={360}
            step={0.1}
            value={Math.round(render.uiMeta.freeTransform.rotation * 10) / 10}
            onChange={(value) => {
              const ft = render.uiMeta.freeTransform;
              if (!ft) return;
              const updated = applyTransformFieldChange(ft, "r", value);
              engine.dispatchCommand(CommandID.UpdateFreeTransform, { ...updated, pivotX: ft.pivotX, pivotY: ft.pivotY, interpolation: ft.interpolation as InterpolMode, ...(ft.warpGrid ? { warpGrid: ft.warpGrid } : {}) });
            }}
          />
          <ToolOptionGroup label="Interp">
            {(["nearest", "bilinear", "bicubic"] as InterpolMode[]).map((mode) => (
              <ToolChoiceButton
                key={mode}
                active={render.uiMeta.freeTransform?.interpolation === mode}
                onClick={() => {
                  const ft = render.uiMeta.freeTransform;
                  if (!ft) return;
                  engine.dispatchCommand(CommandID.UpdateFreeTransform, {
                    a: ft.a, b: ft.b, c: ft.c, d: ft.d,
                    tx: ft.tx, ty: ft.ty,
                    pivotX: ft.pivotX, pivotY: ft.pivotY,
                    interpolation: mode,
                  });
                }}
              >
                {mode.charAt(0).toUpperCase() + mode.slice(1)}
              </ToolChoiceButton>
            ))}
          </ToolOptionGroup>
          <ToolChoiceButton
            active={!!render?.uiMeta.freeTransform?.warpGrid}
            onClick={() => {
              const ft = render?.uiMeta.freeTransform;
              if (!ft) return;
              if (ft.warpGrid) {
                // Exit warp mode → back to affine.
                engine.dispatchCommand(CommandID.UpdateFreeTransform, {
                  a: ft.a, b: ft.b, c: ft.c, d: ft.d,
                  tx: ft.tx, ty: ft.ty,
                  pivotX: ft.pivotX, pivotY: ft.pivotY,
                  interpolation: ft.interpolation as InterpolMode,
                });
              } else {
                // Enter warp mode: initialize grid from corners.
                engine.dispatchCommand(CommandID.UpdateFreeTransform, {
                  a: ft.a, b: ft.b, c: ft.c, d: ft.d,
                  tx: ft.tx, ty: ft.ty,
                  pivotX: ft.pivotX, pivotY: ft.pivotY,
                  interpolation: ft.interpolation as InterpolMode,
                  warpGrid: buildWarpGrid(ft),
                });
              }
            }}
          >
            Warp
          </ToolChoiceButton>
          <button
            type="button"
            className="rounded border border-green-600/50 bg-green-600/20 px-2 py-0.5 text-[11px] text-green-300 hover:bg-green-600/30 focus-visible:outline-none"
            onClick={() => engine.dispatchCommand(CommandID.CommitFreeTransform, {})}
          >
            ✓ Commit
          </button>
          <button
            type="button"
            className="rounded border border-red-600/50 bg-red-600/20 px-2 py-0.5 text-[11px] text-red-300 hover:bg-red-600/30 focus-visible:outline-none"
            onClick={() => engine.dispatchCommand(CommandID.CancelFreeTransform, {})}
          >
            ✗ Cancel
          </button>
        </>
      ) : (
        <span className="text-[11px] text-slate-400">
          Click a layer to begin free transform · Enter to commit · Esc to cancel
        </span>
      )
    ) : activeTool === "brush" || activeTool === "pencil" || activeTool === "mixerBrush" || activeTool === "cloneStamp" || activeTool === "historyBrush" ? (
      <>
        <BrushPresetPicker
          selectedPresetId={brushPresetId}
          onSelectPreset={(preset) => {
            setBrushPresetId(preset.id);
            setBrushTipShape(preset.tipShape);
            setBrushHardness(preset.hardness);
            setBrushSpacing(preset.spacing);
            setBrushAngle(preset.angle);
          }}
        />
        <ToolNumberField
          label="Size"
          min={1}
          max={2500}
          step={1}
          value={brushSize}
          onChange={setBrushSize}
        />
        {activeTool === "mixerBrush" ? (
          <>
            <ToolNumberField
              label="Mix"
              min={0}
              max={1}
              step={0.05}
              value={mixerBrushMix}
              onChange={setMixerBrushMix}
            />
            <ToolChoiceButton
              active={mixerBrushSampleMerged}
              onClick={() => setMixerBrushSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
          </>
        ) : activeTool === "cloneStamp" ? (
          <>
            <ToolChoiceButton
              active={cloneStampSampleMerged}
              onClick={() => setCloneStampSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
            <div className="text-[11px] text-slate-400">
              {cloneStampSource
                ? `Source set at ${Math.round(cloneStampSource.x)}, ${Math.round(cloneStampSource.y)}`
                : "Alt-click the canvas to set a clone source."}
            </div>
          </>
        ) : activeTool === "historyBrush" ? (
          <>
            <ToolChoiceButton
              active={historyBrushSampleMerged}
              onClick={() => setHistoryBrushSampleMerged((value) => !value)}
            >
              Sample Merged
            </ToolChoiceButton>
            <div className="text-[11px] text-slate-400">
              Paints from the previous history state.
            </div>
          </>
        ) : null}
        {activeTool === "pencil" ? (
          <label className="flex items-center gap-1 text-[10px]">
            <input
              type="checkbox"
              checked={pencilAutoErase}
              onChange={(e) => setPencilAutoErase(e.target.checked)}
            />
            Auto-erase
          </label>
        ) : null}
      </>
    ) : activeTool === "eraser" ? (
      <>
        <BrushPresetPicker
          selectedPresetId={brushPresetId}
          onSelectPreset={(preset) => {
            setBrushPresetId(preset.id);
            setBrushTipShape(preset.tipShape);
            setBrushHardness(preset.hardness);
            setBrushSpacing(preset.spacing);
            setBrushAngle(preset.angle);
          }}
        />
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={eraserMode === "normal"}
            onClick={() => setEraserMode("normal")}
          >
            Normal
          </ToolChoiceButton>
          <ToolChoiceButton
            active={eraserMode === "background"}
            onClick={() => setEraserMode("background")}
          >
            Background
          </ToolChoiceButton>
          <ToolChoiceButton
            active={eraserMode === "magic"}
            onClick={() => setEraserMode("magic")}
          >
            Magic
          </ToolChoiceButton>
        </ToolOptionGroup>
        {eraserMode !== "magic" ? (
          <>
            <ToolNumberField
              label="Size"
              min={1}
              max={2500}
              step={1}
              value={brushSize}
              onChange={setBrushSize}
            />
            <ToolNumberField
              label="Opacity"
              min={0}
              max={1}
              step={0.05}
              value={brushFlow}
              onChange={setBrushFlow}
            />
          </>
        ) : null}
        {eraserMode !== "normal" ? (
          <ToolNumberField
            label="Tolerance"
            min={0}
            max={255}
            step={1}
            value={eraserTolerance}
            onChange={setEraserTolerance}
          />
        ) : null}
      </>
        ) : activeTool === "fill" ? (
          <>
            <ToolOptionGroup label="Source">
              <ToolChoiceButton active={fillSource === "foreground"} onClick={() => setFillSource("foreground")}>
                Color
              </ToolChoiceButton>
              <ToolChoiceButton active={fillSource === "background"} onClick={() => setFillSource("background")}>
                Background
              </ToolChoiceButton>
              <ToolChoiceButton active={fillSource === "pattern"} onClick={() => setFillSource("pattern")}>
                Pattern
              </ToolChoiceButton>
            </ToolOptionGroup>
        <ToolNumberField
          label="Tolerance"
          min={0}
          max={255}
          step={1}
          value={fillTolerance}
          onChange={setFillTolerance}
        />
        <ToolChoiceButton active={fillContiguous} onClick={() => setFillContiguous((v) => !v)}>
          Contiguous
        </ToolChoiceButton>
        <ToolChoiceButton active={fillSampleMerged} onClick={() => setFillSampleMerged((v) => !v)}>
          Sample Merged
        </ToolChoiceButton>
        <ToolChoiceButton active={fillCreateLayer} onClick={() => setFillCreateLayer((v) => !v)}>
          New Layer
        </ToolChoiceButton>
        <div className="flex items-center gap-2 text-[11px] text-slate-400">
          <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">Preview</span>
          <span
            className="h-4 w-12 rounded border border-white/10"
            style={
              fillSource === "pattern"
                ? {
                    backgroundColor: "rgba(15, 23, 42, 1)",
                    backgroundImage:
                      "linear-gradient(45deg, rgba(148, 163, 184, 0.35) 25%, transparent 25%, transparent 50%, rgba(148, 163, 184, 0.35) 50%, rgba(148, 163, 184, 0.35) 75%, transparent 75%, transparent)",
                    backgroundSize: "10px 10px",
                  }
                : {
                    backgroundColor:
                      fillSource === "background"
                        ? rgbaToCss(backgroundColor)
                        : rgbaToCss(foregroundColor),
                  }
            }
          />
          <span>{fillModeSummary}</span>
        </div>
        <button
          type="button"
          className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-0.5 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
          onClick={() => setFillDialogOpen(true)}
        >
          Fill Dialog
        </button>
      </>
        ) : activeTool === "gradient" ? (
          <>
            <ToolOptionGroup label="Type">
              {(["linear", "radial", "angle", "reflected", "diamond"] as GradientType[]).map((type) => (
                <ToolChoiceButton key={type} active={gradientType === type} onClick={() => setGradientType(type)}>
                  {type.charAt(0).toUpperCase() + type.slice(1)}
                </ToolChoiceButton>
              ))}
            </ToolOptionGroup>
            <ToolChoiceButton active={gradientReverse} onClick={() => setGradientReverse((v) => !v)}>
              Reverse
            </ToolChoiceButton>
            <ToolChoiceButton active={gradientDither} onClick={() => setGradientDither((v) => !v)}>
              Dither
            </ToolChoiceButton>
            <ToolChoiceButton active={gradientCreateLayer} onClick={() => setGradientCreateLayer((v) => !v)}>
              New Layer
            </ToolChoiceButton>
            <div className="flex items-center gap-2 text-[11px] text-slate-400">
              <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">Preview</span>
              <span
                className="h-4 w-24 rounded border border-white/10"
                style={gradientPreviewStyle}
              />
              <span>{gradientModeSummary}</span>
            </div>
            <button
              type="button"
              className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-0.5 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
              onClick={() => setGradientEditorOpen(true)}
            >
              Edit Gradient
            </button>
            <span className="text-[11px] text-slate-400">Drag on the canvas to set the gradient.</span>
          </>
        ) : activeTool === "eyedropper" ? (
          <>
            <ToolOptionGroup label="Sample">
          {[1, 3, 5, 11, 31, 51, 101].map((size) => (
            <ToolChoiceButton key={size} active={eyedropperSampleSize === size} onClick={() => setEyedropperSampleSize(size)}>
              {size === 1 ? "Point" : `${size}x${size}`}
            </ToolChoiceButton>
          ))}
            </ToolOptionGroup>
            <ToolChoiceButton active={eyedropperSampleMerged} onClick={() => setEyedropperSampleMerged((v) => !v)}>
              Sample Merged
            </ToolChoiceButton>
            <ToolChoiceButton active={eyedropperSampleAllLayersNoAdj} onClick={() => setEyedropperSampleAllLayersNoAdj((v) => !v)}>
              No Adj
            </ToolChoiceButton>
            <span className="text-[11px] text-slate-400">{eyedropperModeSummary}</span>
            <span className="text-[11px] text-slate-400">Click sets foreground; Alt+click sets background.</span>
          </>
        ) : null;

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#202329_0%,#171a1f_100%)] text-slate-100">
      <input
        ref={projectInputRef}
        type="file"
        accept=".agp,application/json,image/png,image/jpeg,image/gif,image/webp,image/bmp"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          if (!file) {
            return;
          }

          if (file.name.endsWith(".agp") || file.type === "application/json") {
            await openProject(file);
          } else if (file.type.startsWith("image/")) {
            await openImageFile(file);
          }
          event.target.value = "";
        }}
      />

      <div className="mx-auto min-h-screen max-w-[1920px] px-0">
        <div className="flex min-h-screen flex-col bg-[#1d2026]">
          <header className="editor-titlebar flex h-[34px] items-center justify-between gap-3 border-b border-border px-2">
            <div
              ref={menuBarRef}
              className="flex min-w-0 flex-nowrap items-center gap-3 overflow-visible"
            >
              <div className="flex shrink-0 items-center gap-2 pr-3">
                <div className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] bg-cyan-400/95 text-[11px] font-black text-slate-950">
                  A
                </div>
                <span className="font-serif text-[12px] font-semibold italic tracking-[0.01em] text-white">
                  Agogo Studio
                </span>
              </div>

              <nav className="flex min-w-0 flex-nowrap items-center gap-1 border-l border-white/8 pl-3">
                {menuItems.map((menu) => {
                  const isOpen = openMenu === menu.label;
                  return (
                    <div key={menu.label} className="relative shrink-0">
                      <button
                        type="button"
                        className={[
                          "px-1.5 py-1 text-[12px] transition focus-visible:bg-white/6 focus-visible:outline-none",
                          isOpen
                            ? "text-white"
                            : "text-slate-400 hover:text-slate-100",
                        ].join(" ")}
                        aria-expanded={isOpen}
                        aria-haspopup="menu"
                        onClick={() =>
                          setOpenMenu((current) =>
                            current === menu.label ? null : menu.label,
                          )
                        }
                        onPointerEnter={() => {
                          if (openMenu) {
                            setOpenMenu(menu.label);
                          }
                        }}
                      >
                        {menu.label}
                      </button>

                      {isOpen ? (
                        <MenuPreviewPanel
                          menu={menu}
                          isItemDisabled={isMenuItemDisabled}
                          onAction={handleMenuAction}
                          checkedActionIds={checkedMenuActionIds}
                        />
                      ) : null}
                    </div>
                  );
                })}
              </nav>
            </div>

            <div className="flex shrink-0 items-center gap-1">
              <Button variant="ghost" size="sm" onClick={openProjectPicker}>
                <OpenFolderIcon className="mr-1.5 h-3.5 w-3.5" />
                Open
              </Button>
              <Button variant="ghost" size="sm" onClick={saveProject}>
                <SaveIcon className="mr-1.5 h-3.5 w-3.5" />
                Save
              </Button>
              <Button variant="ghost" size="sm" onClick={openNewDocumentDialog}>
                <NewDocumentIcon className="mr-1.5 h-3.5 w-3.5" />
                New
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => engine.fitToView()}
              >
                <FitScreenIcon className="mr-1.5 h-3.5 w-3.5" />
                Fit
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => engine.undo()}
                disabled={!render?.uiMeta.canUndo}
              >
                <UndoIcon className="mr-1.5 h-3.5 w-3.5" />
                Undo
              </Button>
              <Button
                size="sm"
                onClick={() => engine.redo()}
                disabled={!render?.uiMeta.canRedo}
              >
                <RedoIcon className="mr-1.5 h-3.5 w-3.5" />
                Redo
              </Button>
            </div>
          </header>

          {hasAutosave && engine.status === "ready" ? (
            <div className="flex items-center gap-2 border-b border-amber-500/30 bg-amber-950/40 px-3 py-1.5 text-[12px] text-amber-200">
              <span>Unsaved session detected.</span>
              <button
                type="button"
                className="rounded bg-amber-600 px-2 py-0.5 text-white hover:bg-amber-500 focus-visible:outline-none"
                onClick={recoverAutosave}
              >
                Restore
              </button>
              <button
                type="button"
                className="rounded px-2 py-0.5 text-amber-400 hover:text-amber-200 focus-visible:outline-none"
                onClick={dismissAutosave}
              >
                Discard
              </button>
            </div>
          ) : null}

          {selectionToolOptions ? (
            <div className="editor-chrome flex min-h-[38px] items-center justify-between gap-3 border-b border-border px-2 py-1.5">
              <div className="flex min-w-0 items-center gap-2 overflow-x-auto pb-0.5">
                {selectionToolOptions}
              </div>
              <div className="hidden shrink-0 text-[11px] text-slate-400 xl:block">
                Shift add, Alt subtract, Shift+Alt intersect
              </div>
            </div>
          ) : null}

          <section
            className="grid min-h-0 flex-1"
            style={{
              gridTemplateColumns: `46px minmax(0,1fr) ${panelCollapsed ? "34px" : `${panelWidth}px`}`,
            }}
          >
            <aside className="editor-chrome editor-toolrail flex min-h-[36rem] flex-col items-center gap-[var(--ui-gap-1)] border-r border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
              {toolItems.map((tool) => {
                const active =
                  (isPanMode && tool.id === "hand") || activeTool === tool.id;
                const ToolIcon = tool.Icon;
                return (
                  <button
                    key={tool.id}
                    type="button"
                    className={[
                      "flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] font-semibold transition focus-visible:outline-none",
                      active
                        ? "bg-cyan-400/14 text-cyan-100"
                        : "bg-transparent text-slate-400 hover:bg-white/6 hover:text-slate-100",
                    ].join(" ")}
                    title={tool.label}
                    onClick={() => activateTool(tool.id)}
                  >
                    <ToolIcon className="h-4 w-4" />
                  </button>
                );
              })}
              {/* Foreground / background color swatches */}
              <div className="relative mt-auto mb-1 flex h-10 w-10 flex-shrink-0 items-end justify-end">
                {/* Background swatch (behind) */}
                <button
                  type="button"
                  className="absolute bottom-0 right-0 h-6 w-6 rounded-sm border border-border"
                  style={{ backgroundColor: rgbaToCss(backgroundColor) }}
                  title="Background color"
                  onClick={() => openColorPicker("background")}
                />
                {/* Foreground swatch (front) */}
                <button
                  type="button"
                  className="absolute left-0 top-0 h-6 w-6 rounded-sm border border-border"
                  style={{ backgroundColor: rgbaToCss(foregroundColor) }}
                  title="Foreground color"
                  onClick={() => openColorPicker("foreground")}
                />
              </div>
            </aside>

            <main className="editor-stage flex min-w-0 min-h-[36rem] flex-col p-[var(--ui-gap-2)]">
              <section
                className={`min-h-0 flex-1 pt-[var(--ui-gap-2)]${isDragOver && hasDocument ? " ring-2 ring-inset ring-blue-500" : ""}`}
                aria-label="Canvas drop zone"
                onDragOver={hasDocument ? handleDragOver : undefined}
                onDragLeave={hasDocument ? handleDragLeave : undefined}
                onDrop={hasDocument ? handleDrop : undefined}
              >
                {hasDocument ? (
                  <EditorCanvas
                    activeTool={activeTool}
                    isPanMode={isPanMode || activeTool === "hand"}
                    isZoomTool={activeTool === "zoom"}
                    selectionOptions={{
                      marqueeShape,
                      marqueeStyle,
                      marqueeRatioW,
                      marqueeRatioH,
                      marqueeSizeW,
                      marqueeSizeH,
                      lassoMode,
                      antiAlias: selectionAntiAlias,
                      featherRadius: selectionFeatherRadius,
                      wandMode,
                      wandTolerance,
                      wandContiguous,
                      wandSampleMerged,
                    }}
                    moveAutoSelectGroup={moveAutoSelectGroup}
                    selectedLayerIds={selectedLayerIds}
                    onCursorChange={setCursor}
                    brushSize={brushSize}
                    brushHardness={brushHardness}
                    brushFlow={brushFlow}
                    mixerBrushMix={mixerBrushMix}
                    mixerBrushSampleMerged={mixerBrushSampleMerged}
                    cloneStampSampleMerged={cloneStampSampleMerged}
                    cloneStampSource={cloneStampSource}
                    onCloneStampSourceChange={setCloneStampSource}
                    historyBrushSampleMerged={historyBrushSampleMerged}
                    pencilAutoErase={pencilAutoErase}
                    eraserMode={eraserMode}
                    eraserTolerance={eraserTolerance}
                    foregroundColor={foregroundColor}
                    onForegroundColorChange={setForegroundColor}
                    onBackgroundColorChange={setBackgroundColor}
                    fillSource={fillSource}
                    fillTolerance={fillTolerance}
                    fillContiguous={fillContiguous}
                    fillSampleMerged={fillSampleMerged}
                    fillCreateLayer={fillCreateLayer}
                    gradientType={gradientType}
                    gradientReverse={gradientReverse}
                    gradientDither={gradientDither}
                    gradientCreateLayer={gradientCreateLayer}
                    gradientStops={gradientStops}
                    eyedropperSampleSize={eyedropperSampleSize}
                    eyedropperSampleMerged={eyedropperSampleMerged}
                    eyedropperSampleAllLayersNoAdj={eyedropperSampleAllLayersNoAdj}
                    cropDeletePixels={cropDeletePixels}
                    transformSelectionActive={transformSelectionActive}
                    onTransformSelectionCommit={(a, b, c, d, tx, ty) => {
                      engine.dispatchCommand(CommandID.TransformSelection, { a, b, c, d, tx, ty });
                      setTransformSelectionActive(false);
                    }}
                    onTransformSelectionCancel={() => setTransformSelectionActive(false)}
                  />
                ) : (
                  <WelcomeScreen
                    isDragOver={isDragOver}
                    hasAutosave={hasAutosave}
                    onNew={() => setNewDocumentOpen(true)}
                    onOpen={() => projectInputRef.current?.click()}
                    onResume={recoverAutosave}
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                  />
                )}
              </section>
            </main>

            <aside className="relative min-h-[36rem]">
              <div
                className="absolute inset-y-[var(--ui-gap-2)] left-0 z-10 w-2 -translate-x-1/2 cursor-col-resize"
                onPointerDown={(event) => {
                  if (panelCollapsed) {
                    return;
                  }
                  const startX = event.clientX;
                  const startWidth = panelWidth;
                  const handleMove = (moveEvent: PointerEvent) => {
                    setPanelWidth(
                      Math.min(
                        420,
                        Math.max(
                          280,
                          startWidth - (moveEvent.clientX - startX),
                        ),
                      ),
                    );
                  };
                  const handleUp = () => {
                    window.removeEventListener("pointermove", handleMove);
                    window.removeEventListener("pointerup", handleUp);
                  };
                  window.addEventListener("pointermove", handleMove);
                  window.addEventListener("pointerup", handleUp);
                }}
              />

              {panelCollapsed ? (
                <div className="editor-panel flex h-full flex-col items-center gap-[var(--ui-gap-1)] border-l border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-[11px]"
                    onClick={() => setPanelCollapsed(false)}
                  >
                    »
                  </Button>
                  {["P", "C", "H", "N", "L"].map((label) => (
                    <div
                      key={label}
                      className="flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] text-slate-400"
                    >
                      {label}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="editor-panel flex h-full flex-col overflow-hidden border-l border-border">
                  <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-[var(--ui-gap-1)]">
                        {[
                          ["properties", "Properties"],
                          ["adjustments", "Adjust"],
                          ["brush", "Brush"],
                          ["color", "Color"],
                          ["swatches", "Swatches"],
                          ["channels", "Channels"],
                          ["history", "History"],
                          ["navigator", "Navigator"],
                        ].map(([id, label]) => (
                          <button
                            key={id}
                            type="button"
                            className={[
                              "rounded-[1px] border px-2 py-1 text-[11px] transition focus-visible:outline-none",
                              activeAuxPanel === id
                                ? "border-white/12 bg-panel-soft text-slate-100"
                                : "border-transparent text-slate-400 hover:border-white/8 hover:bg-white/5 hover:text-slate-200",
                            ].join(" ")}
                            onClick={() => setActiveAuxPanel(id as AuxPanel)}
                          >
                            {label}
                          </button>
                        ))}
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-[11px]"
                        onClick={() => setPanelCollapsed(true)}
                      >
                        «
                      </Button>
                    </div>
                  </div>

                  <div className="grid min-h-0 flex-1 grid-rows-[minmax(15rem,18rem)_minmax(0,1fr)]">
                    <DockSection title={dockTitle(activeAuxPanel)}>
                      {activeAuxPanel === "properties" ? (
                        <AdjPropertiesPanel
                          engine={engine}
                          layers={render?.uiMeta.layers ?? []}
                          activeLayerId={render?.uiMeta.activeLayerId ?? null}
                          fallback={
                            <div className="space-y-[var(--ui-gap-3)]">
                              <PropertyGridRow
                                label="Document"
                                value={documentSize}
                              />
                              <PropertyGridRow label="Zoom" value={zoomPercent} />
                              <PropertyGridRow
                                label="Rotation"
                                value={`${render?.viewport.rotation.toFixed(0) ?? 0}°`}
                              />
                              <PropertyGridRow
                                label="DPI"
                                value={draft.resolution.toString()}
                              />
                              <CompactRange
                                id="rotate-view-range"
                                label="Rotate View"
                                min={0}
                                max={360}
                                step={1}
                                value={render?.viewport.rotation ?? 0}
                                onChange={(value) => engine.setRotation(value)}
                              />
                            </div>
                          }
                        />
                      ) : null}

                      {activeAuxPanel === "adjustments" ? (
                        <AdjustmentsPanel
                          engine={engine}
                          layers={render?.uiMeta.layers ?? []}
                          activeLayerId={render?.uiMeta.activeLayerId ?? null}
                        />
                      ) : null}

                      {activeAuxPanel === "brush" ? (
                        <div className="space-y-3">
                          <BrushSettingsPanel
                            selectedPresetId={brushPresetId}
                            onSelectPreset={(preset: BrushPreset) => {
                              setBrushPresetId(preset.id);
                              setBrushTipShape(preset.tipShape);
                              setBrushHardness(preset.hardness);
                              setBrushSpacing(preset.spacing);
                              setBrushAngle(preset.angle);
                            }}
                            tipShape={brushTipShape}
                            onTipShapeChange={setBrushTipShape}
                            size={brushSize}
                            onSizeChange={setBrushSize}
                            hardness={brushHardness}
                            onHardnessChange={setBrushHardness}
                            angle={brushAngle}
                            onAngleChange={setBrushAngle}
                            roundness={brushRoundness}
                            onRoundnessChange={setBrushRoundness}
                            spacing={brushSpacing}
                            onSpacingChange={setBrushSpacing}
                            sizeJitter={brushSizeJitter}
                            onSizeJitterChange={setBrushSizeJitter}
                            opacityJitter={brushOpacityJitter}
                            onOpacityJitterChange={setBrushOpacityJitter}
                            flowJitter={brushFlowJitter}
                            onFlowJitterChange={setBrushFlowJitter}
                            controlSource={brushControlSource}
                            onControlSourceChange={setBrushControlSource}
                          />
                          {activeTool === "mixerBrush" ? (
                            <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                              <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                                Mixer Brush
                              </p>
                              <div className="flex items-center gap-2">
                                <ToolNumberField
                                  label="Mix"
                                  min={0}
                                  max={1}
                                  step={0.05}
                                  value={mixerBrushMix}
                                  onChange={setMixerBrushMix}
                                />
                                <ToolChoiceButton
                                  active={mixerBrushSampleMerged}
                                  onClick={() => setMixerBrushSampleMerged((value) => !value)}
                                >
                                  Sample Merged
                                </ToolChoiceButton>
                              </div>
                            </div>
                          ) : activeTool === "cloneStamp" ? (
                            <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                              <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                                Clone Stamp
                              </p>
                              <div className="space-y-2 text-[11px] text-slate-400">
                                <div>
                                  {cloneStampSource
                                    ? `Source: ${Math.round(cloneStampSource.x)}, ${Math.round(cloneStampSource.y)}`
                                    : "Alt-click on the canvas to define the source point."}
                                </div>
                                <ToolChoiceButton
                                  active={cloneStampSampleMerged}
                                  onClick={() => setCloneStampSampleMerged((value) => !value)}
                                >
                                  Sample Merged
                                </ToolChoiceButton>
                              </div>
                            </div>
                          ) : activeTool === "historyBrush" ? (
                            <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/10 p-3">
                              <p className="mb-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
                                History Brush
                              </p>
                              <div className="space-y-2 text-[11px] text-slate-400">
                                <div>
                                  Paints from the previous history state. The source selection is still implicit in this first draft.
                                </div>
                                <ToolChoiceButton
                                  active={historyBrushSampleMerged}
                                  onClick={() => setHistoryBrushSampleMerged((value) => !value)}
                                >
                                  Sample Merged
                                </ToolChoiceButton>
                              </div>
                            </div>
                          ) : null}
                        </div>
                      ) : null}

                      {activeAuxPanel === "color" ? (
                        <div className="space-y-3">
                          <div className="flex items-center justify-between gap-2">
                            <div className="flex items-center gap-1">
                              <button
                                type="button"
                                className={[
                                  "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                                  colorPickerTarget === "foreground"
                                    ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                                    : "border-white/8 text-slate-400 hover:bg-white/5",
                                ].join(" ")}
                                onClick={() => setColorPickerTarget("foreground")}
                              >
                                Foreground
                              </button>
                              <button
                                type="button"
                                className={[
                                  "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                                  colorPickerTarget === "background"
                                    ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                                    : "border-white/8 text-slate-400 hover:bg-white/5",
                                ].join(" ")}
                                onClick={() => setColorPickerTarget("background")}
                              >
                                Background
                              </button>
                            </div>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-[11px]"
                              onClick={() => setColorPickerOpen(true)}
                            >
                              Open picker
                            </Button>
                          </div>
                          <ColorPanel
                            color={activeColor}
                            onChange={setActiveColor}
                            channelMode={colorChannelMode}
                            onChannelModeChange={setColorChannelMode}
                            onlyWebColors={onlyWebColors}
                            onOnlyWebColorsChange={setOnlyWebColors}
                            recentColors={recentColors}
                            onRecentColorSelect={(color) => setActiveColor(color)}
                          />
                        </div>
                      ) : null}

                      {activeAuxPanel === "swatches" ? (
                        <SwatchesPanel
                          swatches={swatches}
                          activeColor={activeColor}
                          onPickForeground={(color) => applyColorToTarget("foreground", color)}
                          onPickBackground={(color) => applyColorToTarget("background", color)}
                          onAddSwatch={() => setSwatches((current) => [foregroundColor, ...current].slice(0, 24))}
                          onDeleteSwatch={(index) =>
                            setSwatches((current) => current.filter((_, swatchIndex) => swatchIndex !== index))
                          }
                        />
                      ) : null}

                      {activeAuxPanel === "history" ? (
                        <div className="flex h-full min-h-0 flex-col gap-[var(--ui-gap-2)]">
                          <div className="flex items-center justify-end">
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={historyEntries.length === 0}
                              onClick={() => engine.clearHistory()}
                            >
                              Clear
                            </Button>
                          </div>
                          <div className="min-h-0 flex-1 overflow-auto">
                            <div className="space-y-[var(--ui-gap-1)]">
                              {historyEntries.length === 0 ? (
                                <p className="text-[12px] text-slate-400">
                                  No history entries yet.
                                </p>
                              ) : (
                                historyEntries.map((entry) => (
                                  <button
                                    key={entry.id}
                                    type="button"
                                    className={[
                                      "w-full rounded-[var(--ui-radius-sm)] border px-2 py-1.5 text-left text-[12px] transition focus-visible:outline-none",
                                      entry.id === currentHistoryIndex
                                        ? "border-cyan-400/35 bg-cyan-400/10 text-slate-100"
                                        : entry.state === "undone"
                                          ? "border-white/8 bg-black/10 text-slate-500 hover:text-slate-300"
                                          : "border-white/8 bg-black/10 text-slate-200 hover:border-white/12 hover:bg-black/20",
                                    ].join(" ")}
                                    onClick={() => engine.jumpHistory(entry.id)}
                                  >
                                    {entry.description}
                                  </button>
                                ))
                              )}
                            </div>
                          </div>
                        </div>
                      ) : null}

                      {activeAuxPanel === "navigator" ? (
                        <div className="space-y-[var(--ui-gap-3)]">
                          <div className="border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0.01))] p-[var(--ui-gap-2)]">
                            <div className="aspect-[4/3] border border-white/8 bg-[linear-gradient(135deg,rgba(56,189,248,0.18),rgba(15,23,42,0.82))]" />
                          </div>
                          <CompactRange
                            id="navigator-zoom-range"
                            label="Zoom"
                            min={5}
                            max={3200}
                            step={5}
                            value={Math.round(
                              (render?.viewport.zoom ?? 1) * 100,
                            )}
                            onChange={(value) => engine.setZoom(value / 100)}
                          />
                        </div>
                      ) : null}

                      {activeAuxPanel === "channels" ? <ChannelsPanel /> : null}
                    </DockSection>

                    <DockSection
                      title="Layers"
                      className="border-t border-border"
                    >
                      <LayersPanel
                        engine={engine}
                        layers={render?.uiMeta.layers ?? []}
                        activeLayerId={render?.uiMeta.activeLayerId ?? null}
                        maskEditLayerId={render?.uiMeta.maskEditLayerId ?? null}
                        documentWidth={
                          render?.uiMeta.documentWidth ?? draft.width
                        }
                        documentHeight={
                          render?.uiMeta.documentHeight ?? draft.height
                        }
                        thumbnails={layerThumbnails}
                        selectedLayerIds={selectedLayerIds}
                        onSelectedLayerIdsChange={setSelectedLayerIds}
                      />
                    </DockSection>
                  </div>
                </div>
              )}
            </aside>
          </section>

          <footer className="editor-footerbar flex h-[28px] items-center justify-between gap-3 border-t border-white/8 px-2 text-[11px] text-slate-500">
            <div className="flex items-center gap-2 overflow-hidden">
              <span className="truncate text-slate-300">
                {draft.name}.agp
              </span>
              <Separator orientation="vertical" className="h-3 bg-white/8" />
              <span>{documentSize}</span>
              <Separator orientation="vertical" className="h-3 bg-white/8" />
              <span>{cursorText}</span>
            </div>
            <div className="relative flex items-center gap-2">
              {zoomMenuOpen && (
                <>
                  <button
                    type="button"
                    className="fixed inset-0 z-40"
                    aria-label="Close zoom menu"
                    onClick={() => setZoomMenuOpen(false)}
                  />
                  <DropdownMenuContent className="absolute bottom-full right-0 z-50 mb-1 min-w-[100px] rounded-xl p-1">
                    {[25, 50, 75, 100, 150, 200, 300, 400].map((level) => (
                      <DropdownMenuItem
                        key={level}
                        className={`text-[11px] py-1 px-3 rounded-lg ${Math.round((render?.viewport.zoom ?? 1) * 100) === level ? "text-blue-400" : ""}`}
                        onClick={() => {
                          engine.setZoom(level / 100);
                          setZoomMenuOpen(false);
                        }}
                      >
                        {level}%
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </>
              )}
              <button
                type="button"
                className="cursor-pointer select-none tabular-nums text-slate-200 hover:text-white"
                onClick={handleZoomClick}
                onDoubleClick={handleZoomDoubleClick}
                title="Click for zoom options · Double-click to reset to 100%"
              >
                {zoomPercent}
              </button>
            </div>
          </footer>
        </div>
      </div>

      <Dialog
        open={newDocumentOpen}
        title="Create Document"
        description="Presets, dimensions, resolution, color mode, bit depth, and background feed the Go engine document manager."
      >
        <div className="grid gap-4 md:grid-cols-[11rem_minmax(0,1fr)]">
          <div className="space-y-[var(--ui-gap-2)]">
            {presets.map((preset) => (
              <button
                key={preset.id}
                type="button"
                className="w-full rounded-[var(--ui-radius-sm)] border border-white/8 bg-panel-soft px-3 py-2 text-left transition hover:border-cyan-400/30 hover:bg-cyan-400/8"
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    width: preset.width,
                    height: preset.height,
                    resolution: preset.resolution,
                  }))
                }
              >
                <div className="text-[12px] font-medium text-slate-100">
                  {preset.label}
                </div>
                <div className="mt-1 text-[11px] text-slate-400">
                  {preset.width} x {preset.height} · {preset.resolution} DPI
                </div>
              </button>
            ))}
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Name">
              <input
                className={fieldClassName}
                value={draft.name}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label="Background">
              <select
                className={fieldClassName}
                value={draft.background}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    background: event.target
                      .value as CreateDocumentCommand["background"],
                  }))
                }
              >
                <option value="transparent">Transparent</option>
                <option value="white">White</option>
                <option value="color">Color</option>
              </select>
            </Field>
            <Field label="Units">
              <select
                className={fieldClassName}
                value={documentUnit}
                onChange={(event) =>
                  setDocumentUnit(event.target.value as DocumentUnit)
                }
              >
                <option value="px">Pixels</option>
                <option value="in">Inches</option>
                <option value="cm">Centimeters</option>
                <option value="mm">Millimeters</option>
              </select>
            </Field>
            <Field label={`Width (${documentUnit})`}>
              <input
                className={fieldClassName}
                type="number"
                min={documentUnit === "px" ? 1 : 0.01}
                step={unitSteps[documentUnit]}
                value={widthValue}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    width: Math.max(
                      1,
                      Math.round(
                        unitToPixels(
                          Number(event.target.value),
                          current.resolution,
                          documentUnit,
                        ),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label={`Height (${documentUnit})`}>
              <input
                className={fieldClassName}
                type="number"
                min={documentUnit === "px" ? 1 : 0.01}
                step={unitSteps[documentUnit]}
                value={heightValue}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    height: Math.max(
                      1,
                      Math.round(
                        unitToPixels(
                          Number(event.target.value),
                          current.resolution,
                          documentUnit,
                        ),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label="Resolution (DPI)">
              <input
                className={fieldClassName}
                type="number"
                min={1}
                value={draft.resolution}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    resolution: Number(event.target.value),
                  }))
                }
              />
            </Field>
            <Field label="Bit Depth">
              <select
                className={fieldClassName}
                value={draft.bitDepth}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    bitDepth: Number(event.target.value) as 8 | 16 | 32,
                  }))
                }
              >
                <option value={8}>8-bit</option>
                <option value={16}>16-bit</option>
                <option value={32}>32-bit</option>
              </select>
            </Field>
            <Field label="Color Mode">
              <select
                className={fieldClassName}
                value={draft.colorMode}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    colorMode: event.target
                      .value as CreateDocumentCommand["colorMode"],
                  }))
                }
              >
                <option value="rgb">RGB</option>
                <option value="gray">Grayscale</option>
              </select>
            </Field>
          </div>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setNewDocumentOpen(false)}
          >
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              engine.createDocument(draft);
              setNewDocumentOpen(false);
            }}
          >
            Create Document
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={openRecentOpen}
        title="Open Recent"
        description="The browser build cannot reopen local files automatically yet, so recent documents are informational only for now."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <p>
            Recent document tracking needs a persistent file-access layer. That
            is not wired into the web shell yet.
          </p>
          <p className="text-slate-400">
            Use Open to pick an .agp archive or legacy JSON project from disk.
          </p>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setOpenRecentOpen(false)}
          >
            Close
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setOpenRecentOpen(false);
              openProjectPicker();
            }}
          >
            Open...
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={exportDialogOpen}
        title="Export As"
        description="Project archive export is available now. Flattened image exports still need dedicated engine support."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
            <div className="text-[12px] font-medium text-slate-100">
              Project Archive (.agp)
            </div>
            <div className="mt-1 text-[12px] text-slate-400">
              Saves the current document state, layer tree, and history as{" "}
              {activeDocumentName}.agp.
            </div>
          </div>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setExportDialogOpen(false)}
          >
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              saveProject();
              setExportDialogOpen(false);
            }}
          >
            Export Archive
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={canvasSizeOpen}
        title="Canvas Size"
        description="Resizing the canvas shifts layers relative to the selected anchor."
        className="max-w-md"
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Width">
            <input
              type="number"
              className={fieldClassName}
              value={canvasSizeDraft.width}
              onChange={(e) =>
                setCanvasSizeDraft((c) => ({ ...c, width: Number(e.target.value) }))
              }
            />
          </Field>
          <Field label="Height">
            <input
              type="number"
              className={fieldClassName}
              value={canvasSizeDraft.height}
              onChange={(e) =>
                setCanvasSizeDraft((c) => ({ ...c, height: Number(e.target.value) }))
              }
            />
          </Field>
        </div>
        <div className="mt-4">
          <Field label="Anchor">
            <div className="grid grid-cols-3 gap-1 w-24 h-24 mt-1">
              {(
                [
                  "top-left",
                  "top-center",
                  "top-right",
                  "middle-left",
                  "center",
                  "middle-right",
                  "bottom-left",
                  "bottom-center",
                  "bottom-right",
                ] as const
              ).map((a) => (
                <button
                  key={a}
                  type="button"
                  className={[
                    "w-full h-full border transition",
                    canvasSizeDraft.anchor === a
                      ? "border-cyan-400 bg-cyan-400/20"
                      : "border-white/10 bg-black/20 hover:border-white/20",
                  ].join(" ")}
                  onClick={() => setCanvasSizeDraft((c) => ({ ...c, anchor: a }))}
                />
              ))}
            </div>
          </Field>
        </div>
        <div className="mt-6 flex justify-end gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setCanvasSizeOpen(false)}
          >
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              engine.dispatchCommand(CommandID.ResizeCanvas, canvasSizeDraft);
              setCanvasSizeOpen(false);
            }}
          >
            Resize
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={featherDialogOpen}
        title="Feather Selection"
        description="Softens the selection edges by blurring."
        className="max-w-xs"
      >
        <div className="space-y-3">
          <Field label="Feather Radius (px)">
            <input
              type="number"
              className={fieldClassName}
              min={0}
              max={250}
              step={0.5}
              value={featherDialogValue}
              onChange={(e) => setFeatherDialogValue(Number(e.target.value))}
            />
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setFeatherDialogOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitFeather}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={modifyDialog.open}
        title={{ expand: "Expand Selection", contract: "Contract Selection", smooth: "Smooth Selection", border: "Border Selection" }[modifyDialog.kind]}
        description={{ expand: "Grow the selection outward.", contract: "Shrink the selection inward.", smooth: "Smooth the selection edges.", border: "Create a border of the specified width." }[modifyDialog.kind]}
        className="max-w-xs"
      >
        <div className="space-y-3">
          <Field label={{ expand: "Expand By (px)", contract: "Contract By (px)", smooth: "Radius (px)", border: "Width (px)" }[modifyDialog.kind]}>
            <input
              type="number"
              className={fieldClassName}
              min={1}
              max={500}
              step={1}
              value={modifyDialog.value}
              onChange={(e) => setModifyDialog((d) => ({ ...d, value: Number(e.target.value) }))}
            />
          </Field>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setModifyDialog((d) => ({ ...d, open: false }))}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitModify}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={fillDialogOpen}
        title="Fill"
        description={fillModeSummary}
        className="max-w-sm"
      >
        <div className="space-y-4">
          <ToolOptionGroup label="Source">
            <ToolChoiceButton active={fillSource === "foreground"} onClick={() => setFillSource("foreground")}>
              Color
            </ToolChoiceButton>
            <ToolChoiceButton active={fillSource === "background"} onClick={() => setFillSource("background")}>
              Background
            </ToolChoiceButton>
            <ToolChoiceButton active={fillSource === "pattern"} onClick={() => setFillSource("pattern")}>
              Pattern
            </ToolChoiceButton>
          </ToolOptionGroup>
          <ToolNumberField
            label="Tolerance"
            min={0}
            max={255}
            step={1}
            value={fillTolerance}
            onChange={setFillTolerance}
          />
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillContiguous}
                onChange={(e) => setFillContiguous(e.target.checked)}
              />
              Contiguous
            </label>
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillSampleMerged}
                onChange={(e) => setFillSampleMerged(e.target.checked)}
              />
              Sample Merged
            </label>
            <label className="flex items-center gap-1 text-[10px]">
              <input
                type="checkbox"
                checked={fillCreateLayer}
                onChange={(e) => setFillCreateLayer(e.target.checked)}
              />
              New Layer
            </label>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setFillDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                const activeLayer = render?.uiMeta.activeLayerId;
                if (activeLayer) {
                  engine.dispatchCommand(CommandID.Fill, {
                    hasPoint: false,
                    tolerance: fillTolerance,
                    contiguous: fillContiguous,
                    sampleMerged: fillSampleMerged,
                    source: fillSource,
                    color: toMutableRgba(fillSource === "background" ? backgroundColor : foregroundColor),
                    createLayer: fillCreateLayer,
                  } satisfies FillCommand);
                }
                setFillDialogOpen(false);
              }}
            >
              Fill
            </Button>
          </div>
        </div>
      </Dialog>

      <GradientEditorDialog
        open={gradientEditorOpen}
        description="Edit the stop list, alpha, and reusable presets for the current gradient."
        stops={gradientStops}
        onStopsChange={setGradientStops}
        recentColors={recentColors}
        onRecentColorSelect={pushRecentColor}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        onClose={() => setGradientEditorOpen(false)}
      />

      <Dialog
        open={thresholdDialogOpen}
        title="Threshold"
        description="Threshold uses Rec. 601 luminance: pixels at or above the slider become white, below become black."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
            <div className="mt-2 h-5 overflow-hidden rounded border border-white/10 bg-slate-950">
              <div className="flex h-full w-full">
                <div className="h-full bg-black" style={{ width: `${thresholdValue / 255 * 100}%` }} />
                <div className="h-full flex-1 bg-white" />
              </div>
            </div>
            <div
              className="mt-2 h-1 rounded-full bg-gradient-to-r from-black via-slate-500 to-white"
              style={{
                backgroundImage:
                  "linear-gradient(90deg, rgba(0,0,0,1) 0%, rgba(0,0,0,1) 45%, rgba(255,255,255,1) 55%, rgba(255,255,255,1) 100%)",
              }}
            />
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Threshold</span>
              <span className="text-slate-300">{thresholdValue}</span>
            </div>
            <input
              className="h-2 w-full accent-cyan-400 focus-visible:outline-none"
              type="range"
              min={0}
              max={255}
              step={1}
              value={thresholdValue}
              onChange={(event) => setThresholdValue(Number(event.target.value))}
            />
          </label>
          <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Threshold Value</span>
            <input
              className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
              type="number"
              min={0}
              max={255}
              step={1}
              value={thresholdValue}
              onChange={(event) => setThresholdValue(Math.max(0, Math.min(255, Number(event.target.value) || 0)))}
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setThresholdDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Threshold", "threshold", { threshold: thresholdValue });
                setThresholdDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={posterizeDialogOpen}
        title="Posterize"
        description="Posterize reduces each RGB channel to a fixed number of levels. Alpha is preserved."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
            <div
              className="mt-2 h-5 rounded border border-white/10"
              style={{
                backgroundImage:
                  "linear-gradient(90deg, rgb(0,0,0) 0%, rgb(0,0,0) 14%, rgb(85,85,85) 14%, rgb(85,85,85) 28%, rgb(170,170,170) 28%, rgb(170,170,170) 42%, rgb(255,255,255) 42%, rgb(255,255,255) 100%)",
              }}
            />
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Levels</span>
              <span className="text-slate-300">{posterizeLevels}</span>
            </div>
            <input
              className="h-2 w-full accent-cyan-400 focus-visible:outline-none"
              type="range"
              min={2}
              max={255}
              step={1}
              value={posterizeLevels}
              onChange={(event) => setPosterizeLevels(Number(event.target.value))}
            />
          </label>
          <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            <span>Levels Value</span>
            <input
              className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
              type="number"
              min={2}
              max={255}
              step={1}
              value={posterizeLevels}
              onChange={(event) => setPosterizeLevels(Math.max(2, Math.min(255, Number(event.target.value) || 2)))}
            />
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setPosterizeDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Posterize", "posterize", { levels: posterizeLevels });
                setPosterizeDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={photoFilterDialogOpen}
        title="Photo Filter"
        description="Simulate a gel filter by blending the image toward a tinted filter color. Preserve luminosity keeps the original brightness."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Filter Color</p>
            <div className="mt-2 flex items-center gap-3">
              <input
                type="color"
                className="h-10 w-14 cursor-pointer rounded border border-white/10 bg-transparent"
                value={rgbaToHex(photoFilterColor)}
                onChange={(event) => {
                  const next = hexToRgba(event.target.value);
                  if (next) {
                    setPhotoFilterColor(toMutableRgba(next));
                  }
                }}
              />
              <div className="min-w-0 flex-1">
                <div
                  className="h-10 rounded border border-white/10"
                  style={{ backgroundColor: rgbaToCss(photoFilterColor) }}
                />
                <div className="mt-1 text-[11px] text-slate-500">{rgbaToHex(photoFilterColor).toUpperCase()}</div>
              </div>
            </div>
          </div>
          <label className="block">
            <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
              <span>Density</span>
              <span className="text-slate-300">{photoFilterDensity}</span>
            </div>
            <input
              className="h-2 w-full accent-cyan-400 focus-visible:outline-none"
              type="range"
              min={0}
              max={100}
              step={1}
              value={photoFilterDensity}
              onChange={(event) => setPhotoFilterDensity(Number(event.target.value))}
            />
          </label>
          <label className="flex items-center gap-2 text-[11px] text-slate-300">
            <input
              type="checkbox"
              checked={photoFilterPreserveLuminosity}
              onChange={(event) => setPhotoFilterPreserveLuminosity(event.target.checked)}
            />
            Preserve luminosity
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setPhotoFilterDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Photo Filter", "photo-filter", {
                  color: photoFilterColor,
                  density: photoFilterDensity,
                  preserveLuminosity: photoFilterPreserveLuminosity,
                });
                setPhotoFilterDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={channelMixerDialogOpen}
        title="Channel Mixer"
        description="Mix source RGB into each output channel. Monochrome collapses the mixed result to grayscale."
        className="max-w-4xl"
      >
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            {channelMixerRows.map((row) => (
              <div
                key={row.key}
                className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
              >
                <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                  {row.label}
                </p>
                <div className="mt-3 space-y-3">
                  {channelMixerColumns.map((column) => (
                    <CompactRange
                      key={column.index}
                      id={`channel-mixer-${row.key}-${column.index}`}
                      label={column.label}
                      min={-200}
                      max={200}
                      step={1}
                      value={channelMixerMatrix[row.key][column.index]}
                      onChange={(next) =>
                        setChannelMixerMatrix((current) => ({
                          ...current,
                          [row.key]: current[row.key].map((entry, index) =>
                            index === column.index ? next : entry,
                          ),
                        }))
                      }
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
          <label className="flex items-center gap-2 text-[11px] text-slate-300">
            <input
              type="checkbox"
              checked={channelMixerMonochrome}
              onChange={(event) => setChannelMixerMonochrome(event.target.checked)}
            />
            Monochrome output
          </label>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setChannelMixerDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Channel Mixer", "channel-mixer", {
                  monochrome: channelMixerMonochrome,
                  red: channelMixerMatrix.red,
                  green: channelMixerMatrix.green,
                  blue: channelMixerMatrix.blue,
                });
                setChannelMixerDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog
        open={selectiveColorDialogOpen}
        title="Selective Color"
        description="Adjust CMYK-style components inside named color ranges. Relative mode scales the effect by pixel strength; Absolute applies the full offsets."
        className="max-w-6xl"
      >
        <div className="space-y-4">
          <ToolOptionGroup label="Mode">
            <ToolChoiceButton
              active={selectiveColorMode === "relative"}
              onClick={() => setSelectiveColorMode("relative")}
            >
              Relative
            </ToolChoiceButton>
            <ToolChoiceButton
              active={selectiveColorMode === "absolute"}
              onClick={() => setSelectiveColorMode("absolute")}
            >
              Absolute
            </ToolChoiceButton>
          </ToolOptionGroup>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {selectiveColorRanges.map((range) => (
              <div
                key={range.key}
                className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3"
              >
                <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                  {range.label}
                </p>
                <div className="mt-3 space-y-3">
                  {selectiveColorFields.map((field) => (
                    <CompactRange
                      key={field.key}
                      id={`selective-color-${range.key}-${field.key}`}
                      label={field.label}
                      min={-100}
                      max={100}
                      step={1}
                      value={selectiveColorAdjustments[range.key][field.key]}
                      onChange={(next) =>
                        setSelectiveColorAdjustments((current) => ({
                          ...current,
                          [range.key]: {
                            ...current[range.key],
                            [field.key]: next,
                          },
                        }))
                      }
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
          <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
            <Button variant="secondary" size="sm" onClick={() => setSelectiveColorDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                createAdjustmentLayer("Selective Color", "selective-color", {
                  mode: selectiveColorMode,
                  ...selectiveColorAdjustments,
                });
                setSelectiveColorDialogOpen(false);
              }}
            >
              Create Adjustment Layer
            </Button>
          </div>
        </div>
      </Dialog>

      <GradientEditorDialog
        open={gradientMapDialogOpen}
        title="Gradient Map"
        description="Create an adjustment layer that remaps luminance through the current gradient."
        stops={gradientStops}
        onStopsChange={setGradientStops}
        recentColors={recentColors}
        onRecentColorSelect={pushRecentColor}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        reverse={gradientReverse}
        onReverseChange={setGradientReverse}
        primaryActionLabel="Create Adjustment Layer"
        onPrimaryAction={() => {
          createAdjustmentLayer("Gradient Map", "gradient-map", {
            stops: gradientStops,
            reverse: gradientReverse,
          });
          setGradientMapDialogOpen(false);
        }}
        onClose={() => setGradientMapDialogOpen(false)}
      />

      <SelectAndMaskWorkspace
        open={selectAndMaskOpen}
        onClose={() => setSelectAndMaskOpen(false)}
        engine={engine}
        activeLayerId={render?.uiMeta.activeLayerId ?? null}
      />

      <Dialog
        open={colorRangeOpen}
        title="Color Range"
        description="Select pixels by color similarity."
        className="max-w-sm"
      >
        <div className="space-y-4">
          <Field label="Sample Color">
            <input
              type="color"
              className="h-8 w-full cursor-pointer rounded border border-white/10 bg-transparent"
              value={rgbaToHex(colorRangeColor)}
              onChange={(e) => {
                const next = hexToRgba(e.target.value);
                if (next) {
                  setColorRangeColor(next);
                }
              }}
            />
          </Field>
          <Field label={`Fuzziness: ${colorRangeFuzziness}`}>
            <input
              type="range"
              className="w-full accent-cyan-400"
              min={0}
              max={200}
              step={1}
              value={colorRangeFuzziness}
              onChange={(e) => setColorRangeFuzziness(Number(e.target.value))}
            />
          </Field>
          <label className="flex cursor-pointer select-none items-center gap-2 text-xs text-slate-300">
            <input
              type="checkbox"
              checked={colorRangeSampleMerged}
              onChange={(e) => setColorRangeSampleMerged(e.target.checked)}
            />
            Sample all layers
          </label>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" size="sm" onClick={() => setColorRangeOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={commitColorRange}>
              OK
            </Button>
          </div>
        </div>
      </Dialog>

      <ColorPickerDialog
        open={colorPickerOpen}
        title={colorPickerTarget === "foreground" ? "Foreground Color" : "Background Color"}
        description="Pick a color using RGB or HSB controls. The picker updates the active swatch live."
        color={activeColor}
        onChange={setActiveColor}
        onCommit={() => setColorPickerOpen(false)}
        onClose={() => setColorPickerOpen(false)}
        channelMode={colorChannelMode}
        onChannelModeChange={setColorChannelMode}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={setOnlyWebColors}
        recentColors={recentColors}
        onRecentColorSelect={setActiveColor}
      />
    </div>
  );
}

const fieldClassName =
  "h-[var(--ui-h-md)] w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2.5 text-[13px] text-slate-100 outline-none transition focus:border-cyan-400/40 focus-visible:ring-1 focus-visible:ring-cyan-400/30";

// ---------------------------------------------------------------------------
// Free-transform reference-point grid
// ---------------------------------------------------------------------------

type FreeTransformCorners = [
  [number, number],
  [number, number],
  [number, number],
  [number, number],
];

/** Recompute affine transform after editing one display field in the options bar. */
function applyTransformFieldChange(
  ft: { a: number; b: number; c: number; d: number; tx: number; ty: number; scaleX: number; scaleY: number; rotation: number },
  field: "x" | "y" | "w" | "h" | "r",
  value: number,
): { a: number; b: number; c: number; d: number; tx: number; ty: number } {
  let { a, b, c, d, tx, ty, scaleX, scaleY, rotation } = ft;
  switch (field) {
    case "x":
      tx = value;
      break;
    case "y":
      ty = value;
      break;
    case "w": {
      const newScaleX = value / 100;
      const factor = scaleX !== 0 ? newScaleX / scaleX : 1;
      a *= factor;
      b *= factor;
      break;
    }
    case "h": {
      const newScaleY = value / 100;
      const factor = scaleY !== 0 ? newScaleY / scaleY : 1;
      c *= factor;
      d *= factor;
      break;
    }
    case "r": {
      const deltaRad = ((value - rotation) * Math.PI) / 180;
      const cos = Math.cos(deltaRad);
      const sin = Math.sin(deltaRad);
      const newA = a * cos - c * sin;
      const newC = a * sin + c * cos;
      const newB = b * cos - d * sin;
      const newD = b * sin + d * cos;
      a = newA;
      b = newB;
      c = newC;
      d = newD;
      break;
    }
  }
  return { a, b, c, d, tx, ty };
}

/** Build a 4×4 warp control-point grid by bilinear interpolation of the transform corners. */
function buildWarpGrid(
  ft: FreeTransformMeta,
): [[number, number], [number, number], [number, number], [number, number]][] {
  const [tl, tr, br, bl] = ft.corners;
  const grid: [[number, number], [number, number], [number, number], [number, number]][] = [];
  for (let row = 0; row < 4; row++) {
    const t = row / 3;
    const rowArr: [number, number][] = [];
    for (let col = 0; col < 4; col++) {
      const s = col / 3;
      const x = (1 - t) * ((1 - s) * tl[0] + s * tr[0]) + t * ((1 - s) * bl[0] + s * br[0]);
      const y = (1 - t) * ((1 - s) * tl[1] + s * tr[1]) + t * ((1 - s) * bl[1] + s * br[1]);
      rowArr.push([x, y]);
    }
    grid.push(rowArr as [[number, number], [number, number], [number, number], [number, number]]);
  }
  return grid;
}

/** Compute the pivot doc-space position for a given 3×3 grid cell.
 *  corners: [TL, TR, BR, BL] in document space (from FreeTransformMeta). */
function refPointToPivot(
  corners: FreeTransformCorners,
  row: number,
  col: number,
): [number, number] {
  const t = col / 2; // 0 = left, 0.5 = centre, 1 = right
  const s = row / 2; // 0 = top,  0.5 = middle, 1 = bottom
  const [tl, tr, br, bl] = corners;
  const topX = tl[0] + t * (tr[0] - tl[0]);
  const topY = tl[1] + t * (tr[1] - tl[1]);
  const botX = bl[0] + t * (br[0] - bl[0]);
  const botY = bl[1] + t * (br[1] - bl[1]);
  return [topX + s * (botX - topX), topY + s * (botY - topY)];
}

const REF_POINT_LABELS = [
  ["Top Left", "Top Center", "Top Right"],
  ["Middle Left", "Center", "Middle Right"],
  ["Bottom Left", "Bottom Center", "Bottom Right"],
];

function TransformRefGrid({
  active,
  onChange,
}: {
  active: [number, number];
  onChange(row: number, col: number): void;
}) {
  return (
    <div
      className="grid grid-cols-3 gap-[2px] rounded-[2px] border border-white/20 p-[3px]"
      title="Reference point — sets the pivot for the transform"
    >
      {([0, 1, 2] as const).flatMap((row) =>
        ([0, 1, 2] as const).map((col) => {
          const isActive = active[0] === row && active[1] === col;
          return (
            <button
              key={`${row}-${col}`}
              type="button"
              title={REF_POINT_LABELS[row][col]}
              className={[
                "h-[7px] w-[7px] rounded-[1px] focus-visible:outline-none",
                isActive
                  ? "bg-cyan-400"
                  : "bg-slate-500 hover:bg-slate-300",
              ].join(" ")}
              onClick={() => onChange(row, col)}
            />
          );
        }),
      )}
    </div>
  );
}

function ToolOptionGroup({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="shrink-0 text-[11px] uppercase tracking-[0.18em] text-slate-500">
        {label}
      </span>
      <div className="flex items-center gap-1">{children}</div>
    </div>
  );
}

function ToolChoiceButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={[
        "h-7 rounded-[var(--ui-radius-sm)] border px-2.5 text-[12px] transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/30",
        active
          ? "border-cyan-400/35 bg-cyan-400/14 text-slate-50"
          : "border-white/10 bg-black/20 text-slate-300 hover:border-white/20 hover:bg-black/30",
      ].join(" ")}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function ToolNumberField({
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-[12px] text-slate-300">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
        {label}
      </span>
      <input
        className="h-7 w-20 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-right text-[12px] text-slate-100 outline-none transition focus:border-cyan-400/40 focus-visible:ring-1 focus-visible:ring-cyan-400/30"
        type="number"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function findLayerMetaInTree(
  layers: LayerNodeMeta[],
  targetID: string,
): LayerNodeMeta | null {
  for (const layer of layers) {
    if (layer.id === targetID) {
      return layer;
    }
    const child = findLayerMetaInTree(layer.children ?? [], targetID);
    if (child) {
      return child;
    }
  }
  return null;
}

function findLayerPositionInTree(
  layers: LayerNodeMeta[],
  targetID: string,
  parentId?: string,
): { parentId?: string; index: number } | null {
  for (let index = 0; index < layers.length; index++) {
    const layer = layers[index];
    if (layer.id === targetID) {
      return { parentId, index };
    }
    const child = findLayerPositionInTree(layer.children ?? [], targetID, layer.id);
    if (child) {
      return child;
    }
  }
  return null;
}

function MenuPreviewPanel({
  menu,
  isItemDisabled,
  onAction,
  checkedActionIds,
}: {
  menu: MenuPreviewMenu;
  isItemDisabled(item: MenuPreviewItem): boolean;
  onAction(actionId: MenuActionId): void;
  checkedActionIds?: Set<MenuActionId>;
}) {
  const items = menu.sections.flatMap((section) => section.items);

  return (
    <div
      className={[
        "editor-popup absolute top-[calc(100%+4px)] z-40 w-[18.5rem] max-w-[calc(100vw-1rem)] overflow-hidden",
        menu.align === "right" ? "right-0" : "left-0",
      ].join(" ")}
    >
      <div className="border-b border-white/8 px-2.5 py-2 text-[11px] text-slate-400">
        {menu.caption}
      </div>

      <div className="py-1">
        {items.map((item) => {
          const disabled = isItemDisabled(item);
          const checked = !!(item.actionId && checkedActionIds?.has(item.actionId));
          return (
            <MenuPreviewAction
              key={`${menu.label}-${item.label}`}
              item={item}
              disabled={disabled}
              checked={checked}
              onClick={
                item.actionId
                  ? () => onAction(item.actionId as MenuActionId)
                  : undefined
              }
            />
          );
        })}
      </div>
    </div>
  );
}

function MenuPreviewAction({
  item,
  disabled,
  checked,
  onClick,
}: {
  item: MenuPreviewItem;
  disabled: boolean;
  checked: boolean;
  onClick?: () => void;
}) {
  const ItemIcon = iconForMenuItem(item.label);

  return (
    <button
      type="button"
      className={[
        "flex w-full items-center justify-between px-2.5 py-1.5 text-left text-[12px] transition focus-visible:bg-white/6 focus-visible:outline-none",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:bg-white/6 focus:bg-white/6 focus:outline-none",
      ].join(" ")}
      disabled={disabled}
      aria-disabled={disabled}
      onClick={onClick}
    >
      <span className="flex min-w-0 items-center gap-2">
        <ItemIcon
          className={[
            "h-3.5 w-3.5 shrink-0",
            disabled || item.tone === "muted"
              ? "text-slate-600"
              : item.tone === "accent"
                ? "text-cyan-300"
                : "text-slate-400",
          ].join(" ")}
        />
        <span
          className={
            disabled || item.tone === "muted"
              ? "truncate text-slate-500"
              : "truncate text-slate-100"
          }
        >
          {item.label}
        </span>
      </span>
      {checked ? (
        <span className="ml-4 shrink-0 text-[11px] text-cyan-400">✓</span>
      ) : item.shortcut ? (
        <span className="ml-4 shrink-0 text-[11px] text-slate-500">
          {item.shortcut}
        </span>
      ) : null}
    </button>
  );
}

function iconForMenuItem(label: string) {
  const lower = label.toLowerCase();

  if (lower.includes("new")) {
    return NewDocumentIcon;
  }
  if (lower.includes("open")) {
    return OpenFolderIcon;
  }
  if (
    lower.includes("save") ||
    lower.includes("export") ||
    lower.includes("assets")
  ) {
    return SaveIcon;
  }
  if (lower.includes("undo")) {
    return UndoIcon;
  }
  if (lower.includes("redo")) {
    return RedoIcon;
  }
  if (lower.includes("cut")) {
    return ScissorsIcon;
  }
  if (lower.includes("copy")) {
    return CopyIcon;
  }
  if (lower.includes("paste")) {
    return ClipboardIcon;
  }
  if (
    lower.includes("layer") ||
    lower.includes("rasterize") ||
    lower.includes("merge")
  ) {
    return LayersIcon;
  }
  if (
    lower.includes("select") ||
    lower.includes("feather") ||
    lower.includes("inverse")
  ) {
    return SelectionIcon;
  }
  if (
    lower.includes("levels") ||
    lower.includes("curves") ||
    lower.includes("hue") ||
    lower.includes("invert") ||
    lower.includes("channel mixer") ||
    lower.includes("threshold") ||
    lower.includes("posterize") ||
    lower.includes("selective color") ||
    lower.includes("photo filter") ||
    lower.includes("gradient") ||
    lower.includes("blur") ||
    lower.includes("noise") ||
    lower.includes("stylize") ||
    lower.includes("filter")
  ) {
    return SlidersIcon;
  }
  if (
    lower.includes("zoom") ||
    lower.includes("rulers") ||
    lower.includes("grid") ||
    lower.includes("guides")
  ) {
    return ZoomToolIcon;
  }
  if (
    lower.includes("workspace") ||
    lower.includes("navigator") ||
    lower.includes("history") ||
    lower.includes("panels")
  ) {
    return PanelsIcon;
  }
  return InfoIcon;
}

function DockSection({
  title,
  className,
  children,
}: {
  title: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section className={className}>
      <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
        <h2 className="text-[12px] font-medium text-slate-100">{title}</h2>
      </div>
      <div className="h-[calc(100%-33px)] min-h-0 p-[var(--ui-gap-2)]">
        {children}
      </div>
    </section>
  );
}

function PropertyGridRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 px-2 py-1.5 text-[12px]">
      <span className="text-slate-400">{label}</span>
      <span className="text-slate-100">{value}</span>
    </div>
  );
}

function CompactRange({
  id,
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  id: string;
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{Math.round(value)}</span>
      </div>
      <input
        id={id}
        className="h-2 w-full accent-cyan-400 focus-visible:outline-none"
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function dockTitle(panel: AuxPanel) {
  switch (panel) {
    case "brush":
      return "Brush Settings";
    case "color":
      return "Color";
    case "swatches":
      return "Swatches";
    case "history":
      return "History";
    case "navigator":
      return "Navigator";
    case "channels":
      return "Channels";
    case "adjustments":
      return "Adjustments";
    default:
      return "Properties";
  }
}

const AUTOSAVE_KEY = "agogo:autosave";
const AUTOSAVE_EVERY_N_VERSIONS = 10;

// Channel descriptor: short label, long name, indicator colour class.
const CHANNELS = [
  {
    id: "rgb",
    label: "RGB",
    name: "Composite",
    color: "bg-slate-400",
    shortcut: "~",
  },
  { id: "r", label: "R", name: "Red", color: "bg-rose-400", shortcut: "1" },
  {
    id: "g",
    label: "G",
    name: "Green",
    color: "bg-emerald-400",
    shortcut: "2",
  },
  { id: "b", label: "B", name: "Blue", color: "bg-blue-400", shortcut: "3" },
  { id: "a", label: "A", name: "Alpha", color: "bg-slate-300", shortcut: "4" },
] as const;

function ChannelsPanel() {
  // Channel visibility is cosmetic for now; actual channel isolation is Phase 3+.
  const [visible, setVisible] = useState<Record<string, boolean>>({
    rgb: true,
    r: true,
    g: true,
    b: true,
    a: true,
  });

  return (
    <div className="space-y-[var(--ui-gap-1)]">
      {CHANNELS.map((ch) => (
        <div
          key={ch.id}
          className={[
            "flex items-center gap-2 rounded-[var(--ui-radius-sm)] border px-2 py-1.5 transition",
            visible[ch.id]
              ? "border-white/8 bg-white/[0.02]"
              : "border-white/4 bg-transparent opacity-50",
          ].join(" ")}
        >
          <button
            type="button"
            title={visible[ch.id] ? "Hide channel" : "Show channel"}
            className={[
              "flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] text-[10px] transition",
              visible[ch.id]
                ? "bg-emerald-400/12 text-emerald-100"
                : "bg-black/20 text-slate-500",
            ].join(" ")}
            onClick={() =>
              setVisible((current) => ({
                ...current,
                [ch.id]: !current[ch.id],
              }))
            }
          >
            {visible[ch.id] ? "O" : "-"}
          </button>
          <span className={`h-2.5 w-2.5 rounded-full ${ch.color}`} />
          <span className="flex-1 text-[12px] font-medium text-slate-100">
            {ch.name}
          </span>
          <span className="text-[11px] text-slate-500">{ch.shortcut}</span>
        </div>
      ))}
      <p className="px-1 pt-1 text-[11px] text-slate-600">
        Channel isolation active in Phase 3+.
      </p>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: label wraps its control via children (implicit label pattern)
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
        {label}
      </span>
      {children}
    </label>
  );
}
