import type { CropOverlayType } from "@agogo/proto";
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
import { useEngine } from "@/wasm/context";

export type MarqueeShape = "rect" | "ellipse" | "row" | "col";
export type MarqueeStyle = "normal" | "fixed-ratio" | "fixed-size";
export type LassoMode = "freehand" | "polygon" | "magnetic";
export type WandMode = "magic" | "quick";

export interface SelectionToolStateValue {
  marqueeShape: MarqueeShape;
  setMarqueeShape: Dispatch<SetStateAction<MarqueeShape>>;
  marqueeStyle: MarqueeStyle;
  setMarqueeStyle: Dispatch<SetStateAction<MarqueeStyle>>;
  marqueeRatioW: number;
  setMarqueeRatioW: Dispatch<SetStateAction<number>>;
  marqueeRatioH: number;
  setMarqueeRatioH: Dispatch<SetStateAction<number>>;
  marqueeSizeW: number;
  setMarqueeSizeW: Dispatch<SetStateAction<number>>;
  marqueeSizeH: number;
  setMarqueeSizeH: Dispatch<SetStateAction<number>>;
  lassoMode: LassoMode;
  setLassoMode: Dispatch<SetStateAction<LassoMode>>;
  selectionAntiAlias: boolean;
  setSelectionAntiAlias: Dispatch<SetStateAction<boolean>>;
  selectionFeatherRadius: number;
  setSelectionFeatherRadius: Dispatch<SetStateAction<number>>;
  wandMode: WandMode;
  setWandMode: Dispatch<SetStateAction<WandMode>>;
  wandTolerance: number;
  setWandTolerance: Dispatch<SetStateAction<number>>;
  wandContiguous: boolean;
  setWandContiguous: Dispatch<SetStateAction<boolean>>;
  wandSampleMerged: boolean;
  setWandSampleMerged: Dispatch<SetStateAction<boolean>>;
  moveAutoSelectGroup: boolean;
  setMoveAutoSelectGroup: Dispatch<SetStateAction<boolean>>;
  transformRefPoint: [number, number];
  setTransformRefPoint: Dispatch<SetStateAction<[number, number]>>;
  cropDeletePixels: boolean;
  setCropDeletePixels: Dispatch<SetStateAction<boolean>>;
  cropContentAwareFill: boolean;
  setCropContentAwareFill: Dispatch<SetStateAction<boolean>>;
  cropResolution: number;
  setCropResolution: Dispatch<SetStateAction<number>>;
  cropOverlayType: CropOverlayType;
  setCropOverlayType: Dispatch<SetStateAction<CropOverlayType>>;
  cropStraightenActive: boolean;
  setCropStraightenActive: Dispatch<SetStateAction<boolean>>;
  transformSelectionActive: boolean;
  setTransformSelectionActive: Dispatch<SetStateAction<boolean>>;
}

const SelectionToolStateContext = createContext<SelectionToolStateValue | null>(null);

export function SelectionToolStateProvider({ children }: PropsWithChildren) {
  const engine = useEngine();
  const activeCrop = engine.render?.uiMeta.crop;

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
  const [cropContentAwareFill, setCropContentAwareFill] = useState(false);
  const [cropResolution, setCropResolution] = useState(72);
  const [cropOverlayType, setCropOverlayType] = useState<CropOverlayType>("thirds");
  const [cropStraightenActive, setCropStraightenActive] = useState(false);
  const [transformSelectionActive, setTransformSelectionActive] = useState(false);

  // Mirror the engine's active crop parameters into the local crop settings.
  useEffect(() => {
    if (!activeCrop?.active) {
      setCropStraightenActive(false);
      return;
    }
    setCropDeletePixels((current) =>
      current === activeCrop.deletePixels ? current : activeCrop.deletePixels,
    );
    setCropContentAwareFill((current) =>
      current === activeCrop.contentAwareFill ? current : activeCrop.contentAwareFill,
    );
    setCropResolution((current) =>
      current === activeCrop.resolution ? current : activeCrop.resolution,
    );
    setCropOverlayType((current) =>
      current === activeCrop.overlayType ? current : activeCrop.overlayType,
    );
  }, [
    activeCrop?.active,
    activeCrop?.contentAwareFill,
    activeCrop?.deletePixels,
    activeCrop?.overlayType,
    activeCrop?.resolution,
  ]);

  const value = useMemo<SelectionToolStateValue>(
    () => ({
      marqueeShape,
      setMarqueeShape,
      marqueeStyle,
      setMarqueeStyle,
      marqueeRatioW,
      setMarqueeRatioW,
      marqueeRatioH,
      setMarqueeRatioH,
      marqueeSizeW,
      setMarqueeSizeW,
      marqueeSizeH,
      setMarqueeSizeH,
      lassoMode,
      setLassoMode,
      selectionAntiAlias,
      setSelectionAntiAlias,
      selectionFeatherRadius,
      setSelectionFeatherRadius,
      wandMode,
      setWandMode,
      wandTolerance,
      setWandTolerance,
      wandContiguous,
      setWandContiguous,
      wandSampleMerged,
      setWandSampleMerged,
      moveAutoSelectGroup,
      setMoveAutoSelectGroup,
      transformRefPoint,
      setTransformRefPoint,
      cropDeletePixels,
      setCropDeletePixels,
      cropContentAwareFill,
      setCropContentAwareFill,
      cropResolution,
      setCropResolution,
      cropOverlayType,
      setCropOverlayType,
      cropStraightenActive,
      setCropStraightenActive,
      transformSelectionActive,
      setTransformSelectionActive,
    }),
    [
      marqueeShape,
      marqueeStyle,
      marqueeRatioW,
      marqueeRatioH,
      marqueeSizeW,
      marqueeSizeH,
      lassoMode,
      selectionAntiAlias,
      selectionFeatherRadius,
      wandMode,
      wandTolerance,
      wandContiguous,
      wandSampleMerged,
      moveAutoSelectGroup,
      transformRefPoint,
      cropDeletePixels,
      cropContentAwareFill,
      cropResolution,
      cropOverlayType,
      cropStraightenActive,
      transformSelectionActive,
    ],
  );

  return (
    <SelectionToolStateContext.Provider value={value}>
      {children}
    </SelectionToolStateContext.Provider>
  );
}

export function useSelectionToolState() {
  const context = useContext(SelectionToolStateContext);
  if (!context) {
    throw new Error("useSelectionToolState must be used inside <SelectionToolStateProvider>.");
  }

  return context;
}
