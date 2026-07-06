import {
  ArtboardToolIcon,
  BrushToolIcon,
  CopyIcon,
  CropToolIcon,
  DirectSelectIcon,
  EraserToolIcon,
  EyedropperToolIcon,
  FillToolIcon,
  GradientToolIcon,
  HandToolIcon,
  LassoToolIcon,
  MarqueeToolIcon,
  MoveToolIcon,
  PencilToolIcon,
  PenToolIcon,
  SelectionIcon,
  ShapeToolIcon,
  SlidersIcon,
  TypeToolIcon,
  UndoIcon,
  ZoomToolIcon,
} from "@/components/editor-icons";
import type { ShortcutTool } from "@/hooks/use-keyboard-shortcuts";

export type EditorTool = ShortcutTool | "mixerBrush" | "type" | "shape" | "transform" | "artboard";

export const toolItems: {
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
  { id: "artboard", label: "Artboard", Icon: ArtboardToolIcon },
  { id: "transform", label: "Transform", Icon: SlidersIcon },
  { id: "hand", label: "Hand", Icon: HandToolIcon },
  { id: "zoom", label: "Zoom", Icon: ZoomToolIcon },
];
