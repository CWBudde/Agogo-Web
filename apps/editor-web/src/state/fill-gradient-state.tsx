import type { FillSource, GradientStopCommand, GradientType } from "@agogo/proto";
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
import { toMutableRgba } from "@/lib/color";
import { GRADIENT_STOPS_KEY, loadGradientStops } from "@/lib/persisted-ui";
import { useColorState } from "@/state/color-state";

export interface FillGradientStateValue {
  fillSource: FillSource;
  setFillSource: Dispatch<SetStateAction<FillSource>>;
  fillPatternId: string;
  setFillPatternId: Dispatch<SetStateAction<string>>;
  fillTolerance: number;
  setFillTolerance: Dispatch<SetStateAction<number>>;
  fillContiguous: boolean;
  setFillContiguous: Dispatch<SetStateAction<boolean>>;
  fillSampleMerged: boolean;
  setFillSampleMerged: Dispatch<SetStateAction<boolean>>;
  fillCreateLayer: boolean;
  setFillCreateLayer: Dispatch<SetStateAction<boolean>>;
  fillDialogOpen: boolean;
  setFillDialogOpen: Dispatch<SetStateAction<boolean>>;
  gradientType: GradientType;
  setGradientType: Dispatch<SetStateAction<GradientType>>;
  gradientReverse: boolean;
  setGradientReverse: Dispatch<SetStateAction<boolean>>;
  gradientDither: boolean;
  setGradientDither: Dispatch<SetStateAction<boolean>>;
  gradientCreateLayer: boolean;
  setGradientCreateLayer: Dispatch<SetStateAction<boolean>>;
  gradientStops: GradientStopCommand[];
  setGradientStops: Dispatch<SetStateAction<GradientStopCommand[]>>;
  gradientEditorOpen: boolean;
  setGradientEditorOpen: Dispatch<SetStateAction<boolean>>;
}

const FillGradientStateContext = createContext<FillGradientStateValue | null>(null);

export function FillGradientStateProvider({ children }: PropsWithChildren) {
  // Default gradient stops derive from the current foreground/background
  // colors at mount time (matching the previous App.tsx initializer).
  const { foregroundColor, backgroundColor } = useColorState();

  const [fillSource, setFillSource] = useState<FillSource>("foreground");
  const [fillPatternId, setFillPatternId] = useState("builtin/checker");
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

  useEffect(() => {
    try {
      window.localStorage.setItem(GRADIENT_STOPS_KEY, JSON.stringify(gradientStops));
    } catch {
      // Ignore localStorage failures.
    }
  }, [gradientStops]);

  const value = useMemo<FillGradientStateValue>(
    () => ({
      fillSource,
      setFillSource,
      fillPatternId,
      setFillPatternId,
      fillTolerance,
      setFillTolerance,
      fillContiguous,
      setFillContiguous,
      fillSampleMerged,
      setFillSampleMerged,
      fillCreateLayer,
      setFillCreateLayer,
      fillDialogOpen,
      setFillDialogOpen,
      gradientType,
      setGradientType,
      gradientReverse,
      setGradientReverse,
      gradientDither,
      setGradientDither,
      gradientCreateLayer,
      setGradientCreateLayer,
      gradientStops,
      setGradientStops,
      gradientEditorOpen,
      setGradientEditorOpen,
    }),
    [
      fillSource,
      fillPatternId,
      fillTolerance,
      fillContiguous,
      fillSampleMerged,
      fillCreateLayer,
      fillDialogOpen,
      gradientType,
      gradientReverse,
      gradientDither,
      gradientCreateLayer,
      gradientStops,
      gradientEditorOpen,
    ],
  );

  return (
    <FillGradientStateContext.Provider value={value}>{children}</FillGradientStateContext.Provider>
  );
}

export function useFillGradientState() {
  const context = useContext(FillGradientStateContext);
  if (!context) {
    throw new Error("useFillGradientState must be used inside <FillGradientStateProvider>.");
  }

  return context;
}
