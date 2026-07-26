import { CommandID, type SampleMergedColorCommand, type SetColorCommand } from "@agogo/proto";
import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { ColorChannelMode } from "@/components/brush-color-panels";
import type { ColorSamplerPoint } from "@/components/info-panel";
import { type Rgba, snapToWebSafeColor, toMutableRgba, toRgba } from "@/lib/color";
import {
  CUSTOM_SWATCHES_KEY,
  CUSTOM_SWATCHES_NAME_KEY,
  loadColorList,
  loadStoredName,
  RECENT_COLORS_KEY,
} from "@/lib/persisted-ui";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

export type ColorPickerTarget = "foreground" | "background";

export interface ColorStateValue {
  foregroundColor: Rgba;
  setForegroundColor: Dispatch<SetStateAction<Rgba>>;
  backgroundColor: Rgba;
  setBackgroundColor: Dispatch<SetStateAction<Rgba>>;
  colorPickerOpen: boolean;
  setColorPickerOpen: Dispatch<SetStateAction<boolean>>;
  colorPickerTarget: ColorPickerTarget;
  setColorPickerTarget: Dispatch<SetStateAction<ColorPickerTarget>>;
  colorChannelMode: ColorChannelMode;
  setColorChannelMode: Dispatch<SetStateAction<ColorChannelMode>>;
  onlyWebColors: boolean;
  setOnlyWebColors: Dispatch<SetStateAction<boolean>>;
  recentColors: Rgba[];
  setRecentColors: Dispatch<SetStateAction<Rgba[]>>;
  swatches: Rgba[];
  setSwatches: Dispatch<SetStateAction<Rgba[]>>;
  swatchSetName: string;
  setSwatchSetName: Dispatch<SetStateAction<string>>;
  swatchStatus: string | null;
  setSwatchStatus: Dispatch<SetStateAction<string | null>>;
  eyedropperSampleSize: number;
  setEyedropperSampleSize: Dispatch<SetStateAction<number>>;
  eyedropperSampleMerged: boolean;
  setEyedropperSampleMerged: Dispatch<SetStateAction<boolean>>;
  eyedropperSampleAllLayersNoAdj: boolean;
  setEyedropperSampleAllLayersNoAdj: Dispatch<SetStateAction<boolean>>;
  colorSamplerPoints: ColorSamplerPoint[];
  setColorSamplerPoints: Dispatch<SetStateAction<ColorSamplerPoint[]>>;
  pushRecentColor: (color: Rgba) => void;
  applyColorToTarget: (target: ColorPickerTarget, color: Rgba) => void;
  openColorPicker: (target: ColorPickerTarget) => void;
  sampleColorAtPoint: (
    x: number,
    y: number,
    sampleSize: number,
    sampleMerged: boolean,
    sampleAllLayersNoAdj: boolean,
  ) => Rgba | null;
  addColorSamplerPoint: (point: {
    x: number;
    y: number;
    sampleSize: number;
    sampleMerged: boolean;
    sampleAllLayersNoAdj: boolean;
  }) => void;
}

const ColorStateContext = createContext<ColorStateValue | null>(null);

export function ColorStateProvider({ children }: PropsWithChildren) {
  const engine = useEngine();
  const engineHandle = engine.handle;
  // Subscribe only to the two uiMeta slices the color effects depend on; the
  // provider no longer re-renders per committed frame.
  const contentVersion = useUiMeta((meta) => meta?.contentVersion);
  const documentWidth = useUiMeta((meta) => meta?.documentWidth);

  const [foregroundColor, setForegroundColor] = useState<Rgba>([0, 0, 0, 255]);
  const [backgroundColor, setBackgroundColor] = useState<Rgba>([255, 255, 255, 255]);
  const [colorPickerOpen, setColorPickerOpen] = useState(false);
  const [colorPickerTarget, setColorPickerTarget] = useState<ColorPickerTarget>("foreground");
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
  const [swatchSetName, setSwatchSetName] = useState(() =>
    loadStoredName(CUSTOM_SWATCHES_NAME_KEY, "Custom Swatches"),
  );
  const [swatchStatus, setSwatchStatus] = useState<string | null>(null);
  const [eyedropperSampleSize, setEyedropperSampleSize] = useState(1);
  const [eyedropperSampleMerged, setEyedropperSampleMerged] = useState(true);
  const [eyedropperSampleAllLayersNoAdj, setEyedropperSampleAllLayersNoAdj] = useState(false);
  const [colorSamplerPoints, setColorSamplerPoints] = useState<ColorSamplerPoint[]>([]);
  const nextColorSamplerId = useRef(1);

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
      window.localStorage.setItem(CUSTOM_SWATCHES_NAME_KEY, swatchSetName);
    } catch {
      // Ignore localStorage failures.
    }
  }, [swatchSetName]);

  // Clear sampler points when the document goes away.
  useEffect(() => {
    if ((documentWidth ?? 0) > 0 || colorSamplerPoints.length === 0) {
      return;
    }
    setColorSamplerPoints([]);
  }, [colorSamplerPoints.length, documentWidth]);

  // Re-sample sampler point colors whenever document content changes.
  useEffect(() => {
    if (!engineHandle || contentVersion === undefined || colorSamplerPoints.length === 0) {
      return;
    }
    setColorSamplerPoints((current) =>
      current.map((point) => {
        const sampled = engineHandle.dispatchCommand(CommandID.SampleMergedColor, {
          x: point.x,
          y: point.y,
          sampleSize: point.sampleSize,
          sampleMerged: point.sampleMerged || point.sampleAllLayersNoAdj,
        } satisfies SampleMergedColorCommand)?.sampledColor;
        return {
          ...point,
          color: sampled ? toRgba(sampled) : null,
        };
      }),
    );
  }, [colorSamplerPoints.length, contentVersion, engineHandle]);

  const pushRecentColor = useCallback((color: Rgba) => {
    const normalized = color;
    setRecentColors((current) => {
      const withoutDuplicate = current.filter(
        (entry) => !entry.every((value, index) => value === normalized[index]),
      );
      return [normalized, ...withoutDuplicate].slice(0, 10);
    });
  }, []);

  const applyColorToTarget = useCallback(
    (target: ColorPickerTarget, color: Rgba) => {
      const next = onlyWebColors ? snapToWebSafeColor(color) : color;
      if (target === "foreground") {
        setForegroundColor(next);
      } else {
        setBackgroundColor(next);
      }
      pushRecentColor(next);
    },
    [onlyWebColors, pushRecentColor],
  );

  const openColorPicker = useCallback((target: ColorPickerTarget) => {
    setColorPickerTarget(target);
    setColorPickerOpen(true);
  }, []);

  const sampleColorAtPoint = useCallback(
    (
      x: number,
      y: number,
      sampleSize: number,
      sampleMerged: boolean,
      sampleAllLayersNoAdj: boolean,
    ): Rgba | null => {
      const sampled = engineHandle?.dispatchCommand(CommandID.SampleMergedColor, {
        x,
        y,
        sampleSize,
        sampleMerged: sampleMerged || sampleAllLayersNoAdj,
      } satisfies SampleMergedColorCommand)?.sampledColor;
      return sampled ? toRgba(sampled) : null;
    },
    [engineHandle],
  );

  const addColorSamplerPoint = useCallback(
    ({
      x,
      y,
      sampleSize,
      sampleMerged,
      sampleAllLayersNoAdj,
    }: {
      x: number;
      y: number;
      sampleSize: number;
      sampleMerged: boolean;
      sampleAllLayersNoAdj: boolean;
    }) => {
      setColorSamplerPoints((current) => {
        if (current.length >= 4) {
          return current;
        }
        return [
          ...current,
          {
            id: `sampler-${nextColorSamplerId.current++}`,
            x,
            y,
            sampleSize,
            sampleMerged,
            sampleAllLayersNoAdj,
            color: sampleColorAtPoint(x, y, sampleSize, sampleMerged, sampleAllLayersNoAdj),
          },
        ];
      });
    },
    [sampleColorAtPoint],
  );

  const value = useMemo<ColorStateValue>(
    () => ({
      foregroundColor,
      setForegroundColor,
      backgroundColor,
      setBackgroundColor,
      colorPickerOpen,
      setColorPickerOpen,
      colorPickerTarget,
      setColorPickerTarget,
      colorChannelMode,
      setColorChannelMode,
      onlyWebColors,
      setOnlyWebColors,
      recentColors,
      setRecentColors,
      swatches,
      setSwatches,
      swatchSetName,
      setSwatchSetName,
      swatchStatus,
      setSwatchStatus,
      eyedropperSampleSize,
      setEyedropperSampleSize,
      eyedropperSampleMerged,
      setEyedropperSampleMerged,
      eyedropperSampleAllLayersNoAdj,
      setEyedropperSampleAllLayersNoAdj,
      colorSamplerPoints,
      setColorSamplerPoints,
      pushRecentColor,
      applyColorToTarget,
      openColorPicker,
      sampleColorAtPoint,
      addColorSamplerPoint,
    }),
    [
      foregroundColor,
      backgroundColor,
      colorPickerOpen,
      colorPickerTarget,
      colorChannelMode,
      onlyWebColors,
      recentColors,
      swatches,
      swatchSetName,
      swatchStatus,
      eyedropperSampleSize,
      eyedropperSampleMerged,
      eyedropperSampleAllLayersNoAdj,
      colorSamplerPoints,
      pushRecentColor,
      applyColorToTarget,
      openColorPicker,
      sampleColorAtPoint,
      addColorSamplerPoint,
    ],
  );

  return <ColorStateContext.Provider value={value}>{children}</ColorStateContext.Provider>;
}

export function useColorState() {
  const context = useContext(ColorStateContext);
  if (!context) {
    throw new Error("useColorState must be used inside <ColorStateProvider>.");
  }

  return context;
}
