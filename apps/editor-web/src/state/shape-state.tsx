import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { CUSTOM_SHAPE_PRESETS_KEY, loadShapePresetList } from "@/lib/persisted-ui";
import { SHAPE_PRESETS, type ShapePreset } from "@/lib/shape-presets";

export type ShapeSubTool =
  | "rect"
  | "rounded-rect"
  | "ellipse"
  | "polygon"
  | "line"
  | "custom-shape";
export type ShapeMode = "shape" | "path" | "pixels";
export type ArtboardPreset = "custom" | "hd" | "iphone" | "ipad" | "a4";

type RgbaTuple = [number, number, number, number];

export interface ShapeStateValue {
  shapeSubTool: ShapeSubTool;
  setShapeSubTool: Dispatch<SetStateAction<ShapeSubTool>>;
  shapeMode: ShapeMode;
  setShapeMode: Dispatch<SetStateAction<ShapeMode>>;
  shapeCornerRadius: number;
  setShapeCornerRadius: Dispatch<SetStateAction<number>>;
  shapePolygonSides: number;
  setShapePolygonSides: Dispatch<SetStateAction<number>>;
  shapePolygonInnerRadiusPct: number;
  setShapePolygonInnerRadiusPct: Dispatch<SetStateAction<number>>;
  shapeStarMode: boolean;
  setShapeStarMode: Dispatch<SetStateAction<boolean>>;
  shapePresetId: string;
  setShapePresetId: Dispatch<SetStateAction<string>>;
  shapeFillColor: RgbaTuple;
  setShapeFillColor: Dispatch<SetStateAction<RgbaTuple>>;
  shapeStrokeColor: RgbaTuple;
  setShapeStrokeColor: Dispatch<SetStateAction<RgbaTuple>>;
  shapeStrokeWidth: number;
  setShapeStrokeWidth: Dispatch<SetStateAction<number>>;
  customShapePresets: ShapePreset[];
  setCustomShapePresets: Dispatch<SetStateAction<ShapePreset[]>>;
  shapePresetStatus: string | null;
  setShapePresetStatus: Dispatch<SetStateAction<string | null>>;
  /** Built-in presets followed by the user's custom presets. */
  shapePresets: ShapePreset[];
  customShapePresetIds: string[];
  selectedShapePreset: ShapePreset | null;
  artboardPreset: ArtboardPreset;
  setArtboardPreset: Dispatch<SetStateAction<ArtboardPreset>>;
  artboardBackground: RgbaTuple;
  setArtboardBackground: Dispatch<SetStateAction<RgbaTuple>>;
}

const ShapeStateContext = createContext<ShapeStateValue | null>(null);

export function ShapeStateProvider({ children }: PropsWithChildren) {
  const [shapeSubTool, setShapeSubTool] = useState<ShapeSubTool>("rect");
  const [shapeMode, setShapeMode] = useState<ShapeMode>("shape");
  const [shapeCornerRadius, setShapeCornerRadius] = useState(10);
  const [shapePolygonSides, setShapePolygonSides] = useState(6);
  const [shapePolygonInnerRadiusPct, setShapePolygonInnerRadiusPct] = useState(50);
  const [shapeStarMode, setShapeStarMode] = useState(false);
  const [shapePresetId, setShapePresetId] = useState(SHAPE_PRESETS[0]?.id ?? "");
  const [shapeFillColor, setShapeFillColor] = useState<RgbaTuple>([0, 0, 0, 255]);
  const [shapeStrokeColor, setShapeStrokeColor] = useState<RgbaTuple>([0, 0, 0, 0]);
  const [shapeStrokeWidth, setShapeStrokeWidth] = useState(2);
  const [customShapePresets, setCustomShapePresets] = useState<ShapePreset[]>(() =>
    loadShapePresetList(CUSTOM_SHAPE_PRESETS_KEY),
  );
  const [shapePresetStatus, setShapePresetStatus] = useState<string | null>(null);
  const [artboardPreset, setArtboardPreset] = useState<ArtboardPreset>("custom");
  const [artboardBackground, setArtboardBackground] = useState<RgbaTuple>([255, 255, 255, 255]);

  const shapePresets = useMemo(
    () => [...SHAPE_PRESETS, ...customShapePresets],
    [customShapePresets],
  );
  const customShapePresetIds = useMemo(
    () => customShapePresets.map((preset) => preset.id),
    [customShapePresets],
  );
  const selectedShapePreset = useMemo(
    () => shapePresets.find((preset) => preset.id === shapePresetId) ?? shapePresets[0] ?? null,
    [shapePresetId, shapePresets],
  );

  useEffect(() => {
    try {
      window.localStorage.setItem(CUSTOM_SHAPE_PRESETS_KEY, JSON.stringify(customShapePresets));
    } catch {
      // Ignore localStorage failures.
    }
  }, [customShapePresets]);

  // Fall back to the first preset when the selected one disappears
  // (e.g. an imported custom preset was removed).
  useEffect(() => {
    if (shapePresets.some((preset) => preset.id === shapePresetId)) {
      return;
    }
    setShapePresetId(shapePresets[0]?.id ?? SHAPE_PRESETS[0]?.id ?? "");
  }, [shapePresetId, shapePresets]);

  const value = useMemo<ShapeStateValue>(
    () => ({
      shapeSubTool,
      setShapeSubTool,
      shapeMode,
      setShapeMode,
      shapeCornerRadius,
      setShapeCornerRadius,
      shapePolygonSides,
      setShapePolygonSides,
      shapePolygonInnerRadiusPct,
      setShapePolygonInnerRadiusPct,
      shapeStarMode,
      setShapeStarMode,
      shapePresetId,
      setShapePresetId,
      shapeFillColor,
      setShapeFillColor,
      shapeStrokeColor,
      setShapeStrokeColor,
      shapeStrokeWidth,
      setShapeStrokeWidth,
      customShapePresets,
      setCustomShapePresets,
      shapePresetStatus,
      setShapePresetStatus,
      shapePresets,
      customShapePresetIds,
      selectedShapePreset,
      artboardPreset,
      setArtboardPreset,
      artboardBackground,
      setArtboardBackground,
    }),
    [
      shapeSubTool,
      shapeMode,
      shapeCornerRadius,
      shapePolygonSides,
      shapePolygonInnerRadiusPct,
      shapeStarMode,
      shapePresetId,
      shapeFillColor,
      shapeStrokeColor,
      shapeStrokeWidth,
      customShapePresets,
      shapePresetStatus,
      shapePresets,
      customShapePresetIds,
      selectedShapePreset,
      artboardPreset,
      artboardBackground,
    ],
  );

  return <ShapeStateContext.Provider value={value}>{children}</ShapeStateContext.Provider>;
}

export function useShapeState() {
  const context = useContext(ShapeStateContext);
  if (!context) {
    throw new Error("useShapeState must be used inside <ShapeStateProvider>.");
  }

  return context;
}
