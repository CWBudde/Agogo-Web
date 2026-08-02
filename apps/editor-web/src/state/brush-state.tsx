import type { LayerBlendMode } from "@agogo/proto";
import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  BRUSH_PRESETS,
  type BrushControlSource,
  type BrushPreset,
  type BrushTipShape,
  MIXER_BRUSH_PRESETS,
  type MixerBrushPreset,
} from "@/components/brush-color-panels";
import { CUSTOM_BRUSH_PRESETS_KEY, loadBrushPresetList } from "@/lib/persisted-ui";

export type EraserMode = "normal" | "background" | "magic";

export interface BrushStateValue {
  brushSize: number;
  setBrushSize: Dispatch<SetStateAction<number>>;
  brushHardness: number;
  setBrushHardness: Dispatch<SetStateAction<number>>;
  brushAngle: number;
  setBrushAngle: Dispatch<SetStateAction<number>>;
  brushRoundness: number;
  setBrushRoundness: Dispatch<SetStateAction<number>>;
  brushSpacing: number;
  setBrushSpacing: Dispatch<SetStateAction<number>>;
  brushTipShape: BrushTipShape;
  setBrushTipShape: Dispatch<SetStateAction<BrushTipShape>>;
  brushTipResourceId: string | null;
  setBrushTipResourceId: Dispatch<SetStateAction<string | null>>;
  brushPresetId: string;
  setBrushPresetId: Dispatch<SetStateAction<string>>;
  brushBlendMode: LayerBlendMode;
  setBrushBlendMode: Dispatch<SetStateAction<LayerBlendMode>>;
  brushOpacity: number;
  setBrushOpacity: Dispatch<SetStateAction<number>>;
  brushAirbrush: boolean;
  setBrushAirbrush: Dispatch<SetStateAction<boolean>>;
  brushSmoothing: number;
  setBrushSmoothing: Dispatch<SetStateAction<number>>;
  pressureAffectsSize: boolean;
  setPressureAffectsSize: Dispatch<SetStateAction<boolean>>;
  pressureAffectsOpacity: boolean;
  setPressureAffectsOpacity: Dispatch<SetStateAction<boolean>>;
  pressureAffectsFlow: boolean;
  setPressureAffectsFlow: Dispatch<SetStateAction<boolean>>;
  brushSizeJitter: number;
  setBrushSizeJitter: Dispatch<SetStateAction<number>>;
  brushOpacityJitter: number;
  setBrushOpacityJitter: Dispatch<SetStateAction<number>>;
  brushFlowJitter: number;
  setBrushFlowJitter: Dispatch<SetStateAction<number>>;
  brushControlSource: BrushControlSource;
  setBrushControlSource: Dispatch<SetStateAction<BrushControlSource>>;
  brushFadeDabs: number;
  setBrushFadeDabs: Dispatch<SetStateAction<number>>;
  brushFlow: number;
  setBrushFlow: Dispatch<SetStateAction<number>>;
  mixerBrushWetness: number;
  setMixerBrushWetness: Dispatch<SetStateAction<number>>;
  mixerBrushLoad: number;
  setMixerBrushLoad: Dispatch<SetStateAction<number>>;
  mixerBrushSampleMerged: boolean;
  setMixerBrushSampleMerged: Dispatch<SetStateAction<boolean>>;
  cloneStampAligned: boolean;
  setCloneStampAligned: Dispatch<SetStateAction<boolean>>;
  cloneStampAlignedOffset: { x: number; y: number } | null;
  setCloneStampAlignedOffset: Dispatch<SetStateAction<{ x: number; y: number } | null>>;
  cloneStampSampleMerged: boolean;
  setCloneStampSampleMerged: Dispatch<SetStateAction<boolean>>;
  cloneStampOpacity: number;
  setCloneStampOpacity: Dispatch<SetStateAction<number>>;
  cloneStampLoad: number;
  setCloneStampLoad: Dispatch<SetStateAction<number>>;
  cloneStampUseHistorySource: boolean;
  setCloneStampUseHistorySource: Dispatch<SetStateAction<boolean>>;
  cloneStampHistorySourceIndex: number | null;
  setCloneStampHistorySourceIndex: Dispatch<SetStateAction<number | null>>;
  cloneStampSource: { x: number; y: number } | null;
  setCloneStampSource: Dispatch<SetStateAction<{ x: number; y: number } | null>>;
  historyBrushSourceIndex: number | null;
  setHistoryBrushSourceIndex: Dispatch<SetStateAction<number | null>>;
  historyBrushOpacity: number;
  setHistoryBrushOpacity: Dispatch<SetStateAction<number>>;
  historyBrushLoad: number;
  setHistoryBrushLoad: Dispatch<SetStateAction<number>>;
  historyBrushSampleMerged: boolean;
  setHistoryBrushSampleMerged: Dispatch<SetStateAction<boolean>>;
  pencilAutoErase: boolean;
  setPencilAutoErase: Dispatch<SetStateAction<boolean>>;
  eraserMode: EraserMode;
  setEraserMode: Dispatch<SetStateAction<EraserMode>>;
  eraserTolerance: number;
  setEraserTolerance: Dispatch<SetStateAction<number>>;
  customBrushPresets: BrushPreset[];
  setCustomBrushPresets: Dispatch<SetStateAction<BrushPreset[]>>;
  brushPresetStatus: string | null;
  setBrushPresetStatus: Dispatch<SetStateAction<string | null>>;
  /** Built-in presets followed by the user's custom presets. */
  brushPresets: BrushPreset[];
  customBrushPresetIds: string[];
  /** The mixer preset matching the current brush/mixer settings, if any. */
  currentMixerPreset: MixerBrushPreset | null;
  applyBrushPreset: (preset: BrushPreset) => void;
  applyMixerBrushPreset: (preset: MixerBrushPreset) => void;
}

const BrushStateContext = createContext<BrushStateValue | null>(null);

export function BrushStateProvider({ children }: PropsWithChildren) {
  const [brushSize, setBrushSize] = useState(20);
  const [brushHardness, setBrushHardness] = useState(0.8);
  const [brushAngle, setBrushAngle] = useState(0);
  const [brushRoundness, setBrushRoundness] = useState(0.75);
  const [brushSpacing, setBrushSpacing] = useState(0.14);
  const [brushTipShape, setBrushTipShape] = useState<BrushTipShape>("round");
  const [brushTipResourceId, setBrushTipResourceId] = useState<string | null>(null);
  const [brushPresetId, setBrushPresetId] = useState(BRUSH_PRESETS[0].id);
  const [brushBlendMode, setBrushBlendMode] = useState<LayerBlendMode>("normal");
  const [brushOpacity, setBrushOpacity] = useState(1);
  const [brushAirbrush, setBrushAirbrush] = useState(false);
  const [brushSmoothing, setBrushSmoothing] = useState(0);
  const [pressureAffectsSize, setPressureAffectsSize] = useState(true);
  const [pressureAffectsOpacity, setPressureAffectsOpacity] = useState(false);
  const [pressureAffectsFlow, setPressureAffectsFlow] = useState(true);
  const [brushSizeJitter, setBrushSizeJitter] = useState(0);
  const [brushOpacityJitter, setBrushOpacityJitter] = useState(0);
  const [brushFlowJitter, setBrushFlowJitter] = useState(0);
  const [brushControlSource, setBrushControlSource] = useState<BrushControlSource>("pressure");
  const [brushFadeDabs, setBrushFadeDabs] = useState(100);
  const [brushFlow, setBrushFlow] = useState(1.0);
  const [mixerBrushWetness, setMixerBrushWetness] = useState(0.65);
  const [mixerBrushLoad, setMixerBrushLoad] = useState(0.85);
  const [mixerBrushSampleMerged, setMixerBrushSampleMerged] = useState(true);
  const [cloneStampAligned, setCloneStampAligned] = useState(true);
  const [cloneStampAlignedOffset, setCloneStampAlignedOffset] = useState<{
    x: number;
    y: number;
  } | null>(null);
  const [cloneStampSampleMerged, setCloneStampSampleMerged] = useState(true);
  const [cloneStampOpacity, setCloneStampOpacity] = useState(1);
  const [cloneStampLoad, setCloneStampLoad] = useState(1);
  const [cloneStampUseHistorySource, setCloneStampUseHistorySource] = useState(false);
  const [cloneStampHistorySourceIndex, setCloneStampHistorySourceIndex] = useState<number | null>(
    null,
  );
  const [cloneStampSource, setCloneStampSource] = useState<{ x: number; y: number } | null>(null);
  const [historyBrushSourceIndex, setHistoryBrushSourceIndex] = useState<number | null>(null);
  const [historyBrushOpacity, setHistoryBrushOpacity] = useState(1);
  const [historyBrushLoad, setHistoryBrushLoad] = useState(1);
  const [historyBrushSampleMerged, setHistoryBrushSampleMerged] = useState(true);
  const [pencilAutoErase, setPencilAutoErase] = useState(false);
  const [eraserMode, setEraserMode] = useState<EraserMode>("normal");
  const [eraserTolerance, setEraserTolerance] = useState(30);
  const [customBrushPresets, setCustomBrushPresets] = useState<BrushPreset[]>(() =>
    loadBrushPresetList(CUSTOM_BRUSH_PRESETS_KEY),
  );
  const [brushPresetStatus, setBrushPresetStatus] = useState<string | null>(null);

  const brushPresets = useMemo(
    () => [...BRUSH_PRESETS, ...customBrushPresets],
    [customBrushPresets],
  );
  const customBrushPresetIds = useMemo(
    () => customBrushPresets.map((preset) => preset.id),
    [customBrushPresets],
  );

  const currentMixerPreset = useMemo(() => {
    const fuzzyEquals = (a: number, b: number) => Math.abs(a - b) < 0.001;
    return (
      MIXER_BRUSH_PRESETS.find(
        (preset) =>
          preset.baseBrushPresetId === brushPresetId &&
          preset.tipShape === brushTipShape &&
          fuzzyEquals(preset.hardness, brushHardness) &&
          fuzzyEquals(preset.spacing, brushSpacing) &&
          fuzzyEquals(preset.angle, brushAngle) &&
          fuzzyEquals(preset.wetness, mixerBrushWetness) &&
          fuzzyEquals(preset.load, mixerBrushLoad),
      ) ?? null
    );
  }, [
    brushAngle,
    brushHardness,
    brushPresetId,
    brushSpacing,
    brushTipShape,
    mixerBrushLoad,
    mixerBrushWetness,
  ]);

  useEffect(() => {
    try {
      window.localStorage.setItem(CUSTOM_BRUSH_PRESETS_KEY, JSON.stringify(customBrushPresets));
    } catch {
      // Ignore localStorage failures.
    }
  }, [customBrushPresets]);

  // Fall back to the first preset when the selected one disappears
  // (e.g. an imported custom preset was removed).
  useEffect(() => {
    if (brushPresets.some((preset) => preset.id === brushPresetId)) {
      return;
    }
    setBrushPresetId(brushPresets[0]?.id ?? BRUSH_PRESETS[0].id);
  }, [brushPresetId, brushPresets]);

  const applyBrushPreset = useCallback((preset: BrushPreset) => {
    setBrushPresetId(preset.id);
    if (preset.size !== undefined) {
      setBrushSize(preset.size);
    }
    setBrushTipShape(preset.tipShape);
    setBrushTipResourceId(preset.tipResourceId ?? null);
    setBrushHardness(preset.hardness);
    setBrushSpacing(preset.spacing);
    setBrushAngle(preset.angle);
    setBrushRoundness(preset.roundness ?? 1);
    setBrushSizeJitter(preset.sizeJitter ?? 0);
    setBrushOpacityJitter(preset.opacityJitter ?? 0);
    setBrushFlowJitter(preset.flowJitter ?? 0);
    setBrushControlSource(preset.controlSource ?? "pressure");
    setBrushFadeDabs(preset.fadeDabs ?? 100);
  }, []);

  const applyMixerBrushPreset = useCallback((preset: MixerBrushPreset) => {
    setBrushPresetId(preset.baseBrushPresetId);
    setBrushTipShape(preset.tipShape);
    setBrushTipResourceId(null);
    setBrushHardness(preset.hardness);
    setBrushSpacing(preset.spacing);
    setBrushAngle(preset.angle);
    setMixerBrushWetness(preset.wetness);
    setMixerBrushLoad(preset.load);
  }, []);

  const value = useMemo<BrushStateValue>(
    () => ({
      brushSize,
      setBrushSize,
      brushHardness,
      setBrushHardness,
      brushAngle,
      setBrushAngle,
      brushRoundness,
      setBrushRoundness,
      brushSpacing,
      setBrushSpacing,
      brushTipShape,
      setBrushTipShape,
      brushTipResourceId,
      setBrushTipResourceId,
      brushPresetId,
      setBrushPresetId,
      brushBlendMode,
      setBrushBlendMode,
      brushOpacity,
      setBrushOpacity,
      brushAirbrush,
      setBrushAirbrush,
      brushSmoothing,
      setBrushSmoothing,
      pressureAffectsSize,
      setPressureAffectsSize,
      pressureAffectsOpacity,
      setPressureAffectsOpacity,
      pressureAffectsFlow,
      setPressureAffectsFlow,
      brushSizeJitter,
      setBrushSizeJitter,
      brushOpacityJitter,
      setBrushOpacityJitter,
      brushFlowJitter,
      setBrushFlowJitter,
      brushControlSource,
      setBrushControlSource,
      brushFadeDabs,
      setBrushFadeDabs,
      brushFlow,
      setBrushFlow,
      mixerBrushWetness,
      setMixerBrushWetness,
      mixerBrushLoad,
      setMixerBrushLoad,
      mixerBrushSampleMerged,
      setMixerBrushSampleMerged,
      cloneStampAligned,
      setCloneStampAligned,
      cloneStampAlignedOffset,
      setCloneStampAlignedOffset,
      cloneStampSampleMerged,
      setCloneStampSampleMerged,
      cloneStampOpacity,
      setCloneStampOpacity,
      cloneStampLoad,
      setCloneStampLoad,
      cloneStampUseHistorySource,
      setCloneStampUseHistorySource,
      cloneStampHistorySourceIndex,
      setCloneStampHistorySourceIndex,
      cloneStampSource,
      setCloneStampSource,
      historyBrushSourceIndex,
      setHistoryBrushSourceIndex,
      historyBrushOpacity,
      setHistoryBrushOpacity,
      historyBrushLoad,
      setHistoryBrushLoad,
      historyBrushSampleMerged,
      setHistoryBrushSampleMerged,
      pencilAutoErase,
      setPencilAutoErase,
      eraserMode,
      setEraserMode,
      eraserTolerance,
      setEraserTolerance,
      customBrushPresets,
      setCustomBrushPresets,
      brushPresetStatus,
      setBrushPresetStatus,
      brushPresets,
      customBrushPresetIds,
      currentMixerPreset,
      applyBrushPreset,
      applyMixerBrushPreset,
    }),
    [
      brushSize,
      brushHardness,
      brushAngle,
      brushRoundness,
      brushSpacing,
      brushTipShape,
      brushTipResourceId,
      brushPresetId,
      brushBlendMode,
      brushOpacity,
      brushAirbrush,
      brushSmoothing,
      pressureAffectsSize,
      pressureAffectsOpacity,
      pressureAffectsFlow,
      brushSizeJitter,
      brushOpacityJitter,
      brushFlowJitter,
      brushControlSource,
      brushFadeDabs,
      brushFlow,
      mixerBrushWetness,
      mixerBrushLoad,
      mixerBrushSampleMerged,
      cloneStampAligned,
      cloneStampAlignedOffset,
      cloneStampSampleMerged,
      cloneStampOpacity,
      cloneStampLoad,
      cloneStampUseHistorySource,
      cloneStampHistorySourceIndex,
      cloneStampSource,
      historyBrushSourceIndex,
      historyBrushOpacity,
      historyBrushLoad,
      historyBrushSampleMerged,
      pencilAutoErase,
      eraserMode,
      eraserTolerance,
      customBrushPresets,
      brushPresetStatus,
      brushPresets,
      customBrushPresetIds,
      currentMixerPreset,
      applyBrushPreset,
      applyMixerBrushPreset,
    ],
  );

  return <BrushStateContext.Provider value={value}>{children}</BrushStateContext.Provider>;
}

export function useBrushState() {
  const context = useContext(BrushStateContext);
  if (!context) {
    throw new Error("useBrushState must be used inside <BrushStateProvider>.");
  }

  return context;
}
