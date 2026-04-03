import {
  type AdjustmentKind,
  type AdjustmentLayerParams,
  type AdjustmentParamsByKind,
  CommandID,
  type LayerNodeMeta,
} from "@agogo/proto";
import { VectorPropertiesPanel } from "./vector-properties-panel";
import {
  type MouseEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import type { EngineContextValue } from "@/wasm/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface AdjustmentDef {
  kind: AdjustmentKind;
  label: string;
  /** Short abbreviation shown in the grid icon. */
  abbr: string;
  /** Default params when creating a new layer of this type. */
  defaults: AdjustmentLayerParams;
}

// ---------------------------------------------------------------------------
// Adjustment catalog
// ---------------------------------------------------------------------------

const ADJUSTMENTS: AdjustmentDef[] = [
  { kind: "brightness-contrast", label: "Brightness/Contrast", abbr: "B/C", defaults: {} },
  { kind: "levels", label: "Levels", abbr: "Lvl", defaults: {} },
  { kind: "curves", label: "Curves", abbr: "Crv", defaults: {} },
  { kind: "exposure", label: "Exposure", abbr: "Exp", defaults: {} },
  { kind: "vibrance", label: "Vibrance", abbr: "Vib", defaults: {} },
  { kind: "hue-sat", label: "Hue/Saturation", abbr: "H/S", defaults: {} },
  { kind: "color-balance", label: "Color Balance", abbr: "CB", defaults: {} },
  { kind: "black-white", label: "Black & White", abbr: "B&W", defaults: {} },
  { kind: "photo-filter", label: "Photo Filter", abbr: "PF", defaults: { color: [255, 133, 54, 255], density: 25, preserveLuminosity: true } },
  { kind: "channel-mixer", label: "Channel Mixer", abbr: "CM", defaults: {} },
  { kind: "invert", label: "Invert", abbr: "Inv", defaults: {} },
  { kind: "threshold", label: "Threshold", abbr: "Thr", defaults: { threshold: 128 } },
  { kind: "posterize", label: "Posterize", abbr: "Pos", defaults: { levels: 4 } },
  { kind: "selective-color", label: "Selective Color", abbr: "SC", defaults: {} },
  { kind: "gradient-map", label: "Gradient Map", abbr: "GM", defaults: { stops: [{ color: [0, 0, 0, 255], position: 0 }, { color: [255, 255, 255, 255], position: 1 }] } },
];

// ---------------------------------------------------------------------------
// AdjustmentsPanel — grid of adjustment icons
// ---------------------------------------------------------------------------

export function AdjustmentsPanel({
  engine,
  layers,
  activeLayerId,
}: {
  engine: EngineContextValue;
  layers: LayerNodeMeta[];
  activeLayerId: string | null;
}) {
  const createAdjustment = useCallback(
    (def: AdjustmentDef) => {
      if (!activeLayerId) return;
      const position = findLayerPositionInTree(layers, activeLayerId);
      if (!position) return;
      engine.dispatchCommand(CommandID.AddLayer, {
        layerType: "adjustment",
        name: def.label,
        adjustmentKind: def.kind,
        params: def.defaults,
        parentLayerId: position.parentId,
        index: position.index + 1,
      });
    },
    [engine, layers, activeLayerId],
  );

  return (
    <div className="grid grid-cols-5 gap-1">
      {ADJUSTMENTS.map((def) => (
        <button
          key={def.kind}
          type="button"
          title={def.label}
          className="flex aspect-square flex-col items-center justify-center gap-0.5 rounded-[var(--ui-radius-sm)] border border-white/6 bg-white/[0.02] text-slate-400 transition hover:border-cyan-400/30 hover:bg-cyan-400/8 hover:text-slate-100"
          onClick={() => createAdjustment(def)}
        >
          <span className="text-[11px] font-semibold leading-none">
            {def.abbr}
          </span>
          <span className="max-w-full truncate px-0.5 text-[8px] leading-none opacity-60">
            {def.label}
          </span>
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AdjPropertiesPanel — shows adjustment params or fallback content
// ---------------------------------------------------------------------------

export function AdjPropertiesPanel({
  engine,
  layers,
  activeLayerId,
  fallback,
}: {
  engine: EngineContextValue;
  layers: LayerNodeMeta[];
  activeLayerId: string | null;
  fallback: ReactNode;
}) {
  const layer = activeLayerId ? findLayerById(layers, activeLayerId) : null;

  if (layer?.layerType === "vector") {
    return <VectorPropertiesPanel engine={engine} layer={layer} />;
  }

  if (!layer || layer.layerType !== "adjustment" || !layer.adjustmentKind) {
    return <>{fallback}</>;
  }

  return (
    <div className="flex h-full flex-col gap-2 overflow-y-auto">
      <AdjustmentHeader
        engine={engine}
        layer={layer}
      />
      <AdjustmentParamsEditor
        engine={engine}
        layer={layer}
      />
      <MaskSection
        engine={engine}
        layer={layer}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Adjustment header — name, visibility, clip, delete
// ---------------------------------------------------------------------------

function AdjustmentHeader({
  engine,
  layer,
}: {
  engine: EngineContextValue;
  layer: LayerNodeMeta;
}) {
  const [previewOff, setPreviewOff] = useState(false);

  const toggleVisibility = () => {
    const newVisible = !layer.visible;
    engine.dispatchCommand(CommandID.SetLayerVisibility, {
      layerId: layer.id,
      visible: newVisible,
    });
    setPreviewOff(!newVisible);
  };

  const toggleClip = () => {
    engine.dispatchCommand(CommandID.SetLayerClipToBelow, {
      layerId: layer.id,
      clipToBelow: !layer.clipToBelow,
    });
  };

  const deleteLayer = () => {
    engine.dispatchCommand(CommandID.DeleteLayer, { layerId: layer.id });
  };

  // Sync previewOff with actual visibility
  useEffect(() => {
    setPreviewOff(!layer.visible);
  }, [layer.visible]);

  const def = ADJUSTMENTS.find((a) => a.kind === layer.adjustmentKind);

  return (
    <div className="flex items-center gap-1.5 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 px-2 py-1.5">
      <span className="flex-1 truncate text-[12px] font-medium text-slate-100">
        {def?.label ?? layer.name}
      </span>
      <HeaderButton
        title={previewOff ? "Show adjustment (preview)" : "Hide adjustment (preview)"}
        active={!previewOff}
        onClick={toggleVisibility}
      >
        {previewOff ? "OFF" : "ON"}
      </HeaderButton>
      <HeaderButton
        title={layer.clipToBelow ? "Unclip from layer below" : "Clip to layer below"}
        active={layer.clipToBelow}
        onClick={toggleClip}
      >
        Clip
      </HeaderButton>
      <HeaderButton title="Delete adjustment layer" onClick={deleteLayer}>
        Del
      </HeaderButton>
    </div>
  );
}

function HeaderButton({
  title,
  active,
  onClick,
  children,
}: {
  title: string;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      title={title}
      className={[
        "rounded-[var(--ui-radius-sm)] border px-1.5 py-0.5 text-[10px] font-medium transition",
        active
          ? "border-cyan-400/35 bg-cyan-400/12 text-cyan-200"
          : "border-white/8 text-slate-500 hover:bg-white/5 hover:text-slate-300",
      ].join(" ")}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

// ---------------------------------------------------------------------------
// AdjustmentParamsEditor — dispatches to per-type editors
// ---------------------------------------------------------------------------

function AdjustmentParamsEditor({
  engine,
  layer,
}: {
  engine: EngineContextValue;
  layer: LayerNodeMeta;
}) {
  const kind = layer.adjustmentKind as AdjustmentKind;
  const params = (layer.params ?? {}) as AdjustmentLayerParams;

  const updateParams = useCallback(
    (newParams: AdjustmentLayerParams) => {
      engine.dispatchCommand(CommandID.SetAdjustmentParams, {
        layerId: layer.id,
        params: newParams,
      });
    },
    [engine, layer.id],
  );

  switch (kind) {
    case "brightness-contrast":
      return <BrightnessContrastEditor params={params as AdjustmentParamsByKind["brightness-contrast"]} onChange={updateParams} />;
    case "levels":
      return <LevelsEditor params={params as AdjustmentParamsByKind["levels"]} onChange={updateParams} />;
    case "curves":
      return <CurvesEditor params={params as AdjustmentParamsByKind["curves"]} onChange={updateParams} />;
    case "exposure":
      return <ExposureEditor params={params as AdjustmentParamsByKind["exposure"]} onChange={updateParams} />;
    case "vibrance":
      return <VibranceEditor params={params as AdjustmentParamsByKind["vibrance"]} onChange={updateParams} />;
    case "hue-sat":
      return <HueSatEditor params={params as AdjustmentParamsByKind["hue-sat"]} onChange={updateParams} />;
    case "color-balance":
      return <ColorBalanceEditor params={params as AdjustmentParamsByKind["color-balance"]} onChange={updateParams} />;
    case "black-white":
      return <BlackWhiteEditor params={params as AdjustmentParamsByKind["black-white"]} onChange={updateParams} />;
    case "photo-filter":
      return <PhotoFilterEditor params={params as AdjustmentParamsByKind["photo-filter"]} onChange={updateParams} />;
    case "channel-mixer":
      return <ChannelMixerEditor params={params as AdjustmentParamsByKind["channel-mixer"]} onChange={updateParams} />;
    case "threshold":
      return <ThresholdEditor params={params as AdjustmentParamsByKind["threshold"]} onChange={updateParams} />;
    case "posterize":
      return <PosterizeEditor params={params as AdjustmentParamsByKind["posterize"]} onChange={updateParams} />;
    case "selective-color":
      return <SelectiveColorEditor params={params as AdjustmentParamsByKind["selective-color"]} onChange={updateParams} />;
    case "gradient-map":
      return <GradientMapEditor params={params as AdjustmentParamsByKind["gradient-map"]} onChange={updateParams} />;
    case "invert":
      return (
        <div className="px-1 text-[11px] text-slate-500">
          Invert has no adjustable parameters.
        </div>
      );
    default:
      return null;
  }
}

// ---------------------------------------------------------------------------
// Slider / checkbox / select helpers
// ---------------------------------------------------------------------------

function ParamSlider({
  label,
  min,
  max,
  step,
  value,
  onChange,
  formatValue,
}: {
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (v: number) => void;
  formatValue?: (v: number) => string;
}) {
  return (
    <label className="block">
      <div className="mb-0.5 flex items-center justify-between text-[10px] uppercase tracking-[0.15em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">
          {formatValue ? formatValue(value) : Math.round(value)}
        </span>
      </div>
      <input
        className="h-1.5 w-full accent-cyan-400 focus-visible:outline-none"
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </label>
  );
}

function ParamCheckbox({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-1.5 text-[11px] text-slate-300">
      <input
        type="checkbox"
        className="accent-cyan-400"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      {label}
    </label>
  );
}

function ParamSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <label className="block">
      <span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
        {label}
      </span>
      <select
        className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function EditorSection({ children }: { children: ReactNode }) {
  return <div className="space-y-2 px-1">{children}</div>;
}

// ---------------------------------------------------------------------------
// Per-type editors
// ---------------------------------------------------------------------------

function BrightnessContrastEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["brightness-contrast"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["brightness-contrast"]>) =>
    onChange({ ...params, ...patch });

  return (
    <EditorSection>
      <ParamSlider label="Brightness" min={-150} max={150} step={1} value={params.brightness ?? 0} onChange={(v) => update({ brightness: v })} />
      <ParamSlider label="Contrast" min={-150} max={150} step={1} value={params.contrast ?? 0} onChange={(v) => update({ contrast: v })} />
      <ParamCheckbox label="Use Legacy" checked={params.legacy ?? false} onChange={(v) => update({ legacy: v })} />
    </EditorSection>
  );
}

function LevelsEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["levels"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["levels"]>) =>
    onChange({ ...params, ...patch });

  return (
    <EditorSection>
      <ParamSelect
        label="Channel"
        value={params.channel ?? "rgb"}
        options={[
          { value: "rgb", label: "RGB" },
          { value: "red", label: "Red" },
          { value: "green", label: "Green" },
          { value: "blue", label: "Blue" },
        ]}
        onChange={(v) => update({ channel: v })}
      />
      <ParamSlider label="Input Black" min={0} max={255} step={1} value={params.inputBlack ?? 0} onChange={(v) => update({ inputBlack: v })} />
      <ParamSlider label="Input White" min={0} max={255} step={1} value={params.inputWhite ?? 255} onChange={(v) => update({ inputWhite: v })} />
      <ParamSlider label="Gamma" min={0.1} max={9.99} step={0.01} value={params.gamma ?? 1} onChange={(v) => update({ gamma: v })} formatValue={(v) => v.toFixed(2)} />
      <ParamSlider label="Output Black" min={0} max={255} step={1} value={params.outputBlack ?? 0} onChange={(v) => update({ outputBlack: v })} />
      <ParamSlider label="Output White" min={0} max={255} step={1} value={params.outputWhite ?? 255} onChange={(v) => update({ outputWhite: v })} />
      <ParamCheckbox label="Auto" checked={params.auto ?? false} onChange={(v) => update({ auto: v })} />
    </EditorSection>
  );
}

function CurvesEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["curves"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["curves"]>) =>
    onChange({ ...params, ...patch });

  const points = params.points ?? [
    { x: 0, y: 0 },
    { x: 255, y: 255 },
  ];

  return (
    <EditorSection>
      <ParamSelect
        label="Channel"
        value={params.channel ?? "rgb"}
        options={[
          { value: "rgb", label: "RGB" },
          { value: "red", label: "Red" },
          { value: "green", label: "Green" },
          { value: "blue", label: "Blue" },
        ]}
        onChange={(v) => update({ channel: v })}
      />
      <CurvesCanvas points={points} onChange={(pts) => update({ points: pts })} />
    </EditorSection>
  );
}

/** Minimal interactive curves canvas. */
function CurvesCanvas({
  points,
  onChange,
}: {
  points: { x: number; y: number }[];
  onChange: (pts: { x: number; y: number }[]) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [dragging, setDragging] = useState<number | null>(null);
  const SIZE = 128;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.clearRect(0, 0, SIZE, SIZE);

    // Background
    ctx.fillStyle = "rgba(0,0,0,0.3)";
    ctx.fillRect(0, 0, SIZE, SIZE);

    // Grid
    ctx.strokeStyle = "rgba(255,255,255,0.08)";
    ctx.lineWidth = 1;
    for (let i = 1; i < 4; i++) {
      const p = (i / 4) * SIZE;
      ctx.beginPath();
      ctx.moveTo(p, 0);
      ctx.lineTo(p, SIZE);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(0, p);
      ctx.lineTo(SIZE, p);
      ctx.stroke();
    }

    // Diagonal reference
    ctx.strokeStyle = "rgba(255,255,255,0.15)";
    ctx.beginPath();
    ctx.moveTo(0, SIZE);
    ctx.lineTo(SIZE, 0);
    ctx.stroke();

    // Curve
    const sorted = [...points].sort((a, b) => a.x - b.x);
    ctx.strokeStyle = "rgba(103,232,249,0.8)";
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    for (let i = 0; i < sorted.length; i++) {
      const px = (sorted[i].x / 255) * SIZE;
      const py = SIZE - (sorted[i].y / 255) * SIZE;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();

    // Points
    for (const pt of sorted) {
      const px = (pt.x / 255) * SIZE;
      const py = SIZE - (pt.y / 255) * SIZE;
      ctx.fillStyle = "rgba(103,232,249,1)";
      ctx.beginPath();
      ctx.arc(px, py, 3.5, 0, Math.PI * 2);
      ctx.fill();
    }
  }, [points]);

  const getCanvasPos = (e: MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    const x = Math.round(((e.clientX - rect.left) / rect.width) * 255);
    const y = Math.round((1 - (e.clientY - rect.top) / rect.height) * 255);
    return { x: Math.max(0, Math.min(255, x)), y: Math.max(0, Math.min(255, y)) };
  };

  const handleMouseDown = (e: MouseEvent<HTMLCanvasElement>) => {
    const pos = getCanvasPos(e);
    // Find closest point
    let closestIdx = -1;
    let closestDist = 15; // threshold in 0-255 space
    for (let i = 0; i < points.length; i++) {
      const dist = Math.hypot(points[i].x - pos.x, points[i].y - pos.y);
      if (dist < closestDist) {
        closestDist = dist;
        closestIdx = i;
      }
    }
    if (closestIdx >= 0) {
      setDragging(closestIdx);
    } else {
      // Add new point
      const newPoints = [...points, pos].sort((a, b) => a.x - b.x);
      onChange(newPoints);
      const newIdx = newPoints.findIndex(
        (p) => p.x === pos.x && p.y === pos.y,
      );
      setDragging(newIdx >= 0 ? newIdx : null);
    }
  };

  const handleMouseMove = (e: MouseEvent<HTMLCanvasElement>) => {
    if (dragging === null) return;
    const pos = getCanvasPos(e);
    const newPoints = [...points];
    newPoints[dragging] = pos;
    onChange(newPoints.sort((a, b) => a.x - b.x));
  };

  const handleMouseUp = () => {
    setDragging(null);
  };

  return (
    <canvas
      ref={canvasRef}
      width={SIZE}
      height={SIZE}
      className="w-full cursor-crosshair rounded-[var(--ui-radius-sm)] border border-white/10"
      style={{ imageRendering: "auto", aspectRatio: "1" }}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onMouseLeave={handleMouseUp}
    />
  );
}

function ExposureEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["exposure"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["exposure"]>) =>
    onChange({ ...params, ...patch });

  return (
    <EditorSection>
      <ParamSlider label="Exposure" min={-5} max={5} step={0.01} value={params.exposure ?? 0} onChange={(v) => update({ exposure: v })} formatValue={(v) => `${v >= 0 ? "+" : ""}${v.toFixed(2)}`} />
      <ParamSlider label="Offset" min={-0.5} max={0.5} step={0.001} value={params.offset ?? 0} onChange={(v) => update({ offset: v })} formatValue={(v) => v.toFixed(4)} />
      <ParamSlider label="Gamma Correction" min={0.01} max={9.99} step={0.01} value={params.gamma ?? 1} onChange={(v) => update({ gamma: v })} formatValue={(v) => v.toFixed(2)} />
    </EditorSection>
  );
}

function VibranceEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["vibrance"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["vibrance"]>) =>
    onChange({ ...params, ...patch });

  return (
    <EditorSection>
      <ParamSlider label="Vibrance" min={-100} max={100} step={1} value={params.vibrance ?? 0} onChange={(v) => update({ vibrance: v })} />
      <ParamSlider label="Saturation" min={-100} max={100} step={1} value={params.saturation ?? 0} onChange={(v) => update({ saturation: v })} />
    </EditorSection>
  );
}

function HueSatEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["hue-sat"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["hue-sat"]>) =>
    onChange({ ...params, ...patch });

  const [activeRange, setActiveRange] = useState("master");

  const ranges = [
    { value: "master", label: "Master" },
    { value: "reds", label: "Reds" },
    { value: "yellows", label: "Yellows" },
    { value: "greens", label: "Greens" },
    { value: "cyans", label: "Cyans" },
    { value: "blues", label: "Blues" },
    { value: "magentas", label: "Magentas" },
  ];

  const rangeParams =
    activeRange === "master"
      ? { hueShift: params.hueShift ?? 0, saturation: params.saturation ?? 0, lightness: params.lightness ?? 0 }
      : (params[activeRange as keyof typeof params] as { hueShift?: number; saturation?: number; lightness?: number } | undefined) ?? { hueShift: 0, saturation: 0, lightness: 0 };

  const updateRange = (patch: { hueShift?: number; saturation?: number; lightness?: number }) => {
    if (activeRange === "master") {
      update(patch);
    } else {
      const current = (params[activeRange as keyof typeof params] as { hueShift?: number; saturation?: number; lightness?: number } | undefined) ?? {};
      update({ [activeRange]: { ...current, ...patch } } as Partial<AdjustmentParamsByKind["hue-sat"]>);
    }
  };

  return (
    <EditorSection>
      <ParamSelect
        label="Range"
        value={activeRange}
        options={ranges}
        onChange={setActiveRange}
      />
      <ParamSlider label="Hue" min={-180} max={180} step={1} value={rangeParams.hueShift ?? 0} onChange={(v) => updateRange({ hueShift: v })} formatValue={(v) => `${v}°`} />
      <ParamSlider label="Saturation" min={-100} max={100} step={1} value={rangeParams.saturation ?? 0} onChange={(v) => updateRange({ saturation: v })} />
      <ParamSlider label="Lightness" min={-100} max={100} step={1} value={rangeParams.lightness ?? 0} onChange={(v) => updateRange({ lightness: v })} />
      <ParamCheckbox label="Colorize" checked={params.colorize ?? false} onChange={(v) => update({ colorize: v })} />
    </EditorSection>
  );
}

function ColorBalanceEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["color-balance"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["color-balance"]>) =>
    onChange({ ...params, ...patch });

  const [activeTone, setActiveTone] = useState("midtones");

  const tones = [
    { value: "shadows", label: "Shadows" },
    { value: "midtones", label: "Midtones" },
    { value: "highlights", label: "Highlights" },
  ];

  type Tone = { cyanRed?: number; magentaGreen?: number; yellowBlue?: number };
  const toneParams = (params[activeTone as keyof typeof params] as Tone | undefined) ?? {};

  const updateTone = (patch: Tone) => {
    const current = (params[activeTone as keyof typeof params] as Tone | undefined) ?? {};
    update({ [activeTone]: { ...current, ...patch } } as Partial<AdjustmentParamsByKind["color-balance"]>);
  };

  return (
    <EditorSection>
      <ParamSelect label="Tone" value={activeTone} options={tones} onChange={setActiveTone} />
      <ParamSlider label="Cyan / Red" min={-100} max={100} step={1} value={toneParams.cyanRed ?? 0} onChange={(v) => updateTone({ cyanRed: v })} />
      <ParamSlider label="Magenta / Green" min={-100} max={100} step={1} value={toneParams.magentaGreen ?? 0} onChange={(v) => updateTone({ magentaGreen: v })} />
      <ParamSlider label="Yellow / Blue" min={-100} max={100} step={1} value={toneParams.yellowBlue ?? 0} onChange={(v) => updateTone({ yellowBlue: v })} />
      <ParamCheckbox label="Preserve Luminosity" checked={params.preserveLuminosity ?? true} onChange={(v) => update({ preserveLuminosity: v })} />
    </EditorSection>
  );
}

function BlackWhiteEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["black-white"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["black-white"]>) =>
    onChange({ ...params, ...patch });

  return (
    <EditorSection>
      <ParamSlider label="Reds" min={-200} max={300} step={1} value={params.reds ?? 40} onChange={(v) => update({ reds: v })} />
      <ParamSlider label="Yellows" min={-200} max={300} step={1} value={params.yellows ?? 60} onChange={(v) => update({ yellows: v })} />
      <ParamSlider label="Greens" min={-200} max={300} step={1} value={params.greens ?? 40} onChange={(v) => update({ greens: v })} />
      <ParamSlider label="Cyans" min={-200} max={300} step={1} value={params.cyans ?? 60} onChange={(v) => update({ cyans: v })} />
      <ParamSlider label="Blues" min={-200} max={300} step={1} value={params.blues ?? 20} onChange={(v) => update({ blues: v })} />
      <ParamSlider label="Magentas" min={-200} max={300} step={1} value={params.magentas ?? 80} onChange={(v) => update({ magentas: v })} />
      <ParamCheckbox label="Tint" checked={params.tint ?? false} onChange={(v) => update({ tint: v })} />
    </EditorSection>
  );
}

function PhotoFilterEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["photo-filter"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["photo-filter"]>) =>
    onChange({ ...params, ...patch });

  const color = params.color ?? [255, 133, 54, 255];
  const hexColor = `#${color.slice(0, 3).map((c) => c.toString(16).padStart(2, "0")).join("")}`;

  return (
    <EditorSection>
      <label className="block">
        <span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
          Filter Color
        </span>
        <input
          type="color"
          className="h-6 w-full cursor-pointer rounded-[var(--ui-radius-sm)] border border-white/10 bg-transparent"
          value={hexColor}
          onChange={(e) => {
            const hex = e.target.value;
            const r = Number.parseInt(hex.slice(1, 3), 16);
            const g = Number.parseInt(hex.slice(3, 5), 16);
            const b = Number.parseInt(hex.slice(5, 7), 16);
            update({ color: [r, g, b, 255] });
          }}
        />
      </label>
      <ParamSlider label="Density" min={0} max={100} step={1} value={params.density ?? 25} onChange={(v) => update({ density: v })} formatValue={(v) => `${Math.round(v)}%`} />
      <ParamCheckbox label="Preserve Luminosity" checked={params.preserveLuminosity ?? true} onChange={(v) => update({ preserveLuminosity: v })} />
    </EditorSection>
  );
}

function ChannelMixerEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["channel-mixer"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["channel-mixer"]>) =>
    onChange({ ...params, ...patch });

  const [outputChannel, setOutputChannel] = useState("red");

  const channels = [
    { value: "red", label: "Red Output" },
    { value: "green", label: "Green Output" },
    { value: "blue", label: "Blue Output" },
  ];

  type MixerArray = [number, number, number];
  const defaults: Record<string, MixerArray> = {
    red: [100, 0, 0],
    green: [0, 100, 0],
    blue: [0, 0, 100],
  };
  const current = (params[outputChannel as keyof typeof params] as MixerArray | undefined) ?? defaults[outputChannel];

  const updateChannel = (idx: number, v: number) => {
    const arr: MixerArray = [...current] as MixerArray;
    arr[idx] = v;
    update({ [outputChannel]: arr } as Partial<AdjustmentParamsByKind["channel-mixer"]>);
  };

  return (
    <EditorSection>
      <ParamSelect label="Output Channel" value={outputChannel} options={channels} onChange={setOutputChannel} />
      <ParamSlider label="Red" min={-200} max={200} step={1} value={current[0]} onChange={(v) => updateChannel(0, v)} formatValue={(v) => `${v}%`} />
      <ParamSlider label="Green" min={-200} max={200} step={1} value={current[1]} onChange={(v) => updateChannel(1, v)} formatValue={(v) => `${v}%`} />
      <ParamSlider label="Blue" min={-200} max={200} step={1} value={current[2]} onChange={(v) => updateChannel(2, v)} formatValue={(v) => `${v}%`} />
      <ParamCheckbox label="Monochrome" checked={params.monochrome ?? false} onChange={(v) => update({ monochrome: v })} />
    </EditorSection>
  );
}

function ThresholdEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["threshold"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  return (
    <EditorSection>
      <ParamSlider label="Threshold Level" min={1} max={255} step={1} value={params.threshold ?? 128} onChange={(v) => onChange({ threshold: v })} />
    </EditorSection>
  );
}

function PosterizeEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["posterize"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  return (
    <EditorSection>
      <ParamSlider label="Levels" min={2} max={255} step={1} value={params.levels ?? 4} onChange={(v) => onChange({ levels: v })} />
    </EditorSection>
  );
}

function SelectiveColorEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["selective-color"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["selective-color"]>) =>
    onChange({ ...params, ...patch });

  const [activeRange, setActiveRange] = useState("reds");

  const ranges = [
    { value: "reds", label: "Reds" },
    { value: "yellows", label: "Yellows" },
    { value: "greens", label: "Greens" },
    { value: "cyans", label: "Cyans" },
    { value: "blues", label: "Blues" },
    { value: "magentas", label: "Magentas" },
    { value: "whites", label: "Whites" },
    { value: "neutrals", label: "Neutrals" },
    { value: "blacks", label: "Blacks" },
  ];

  type Tone = { cyanRed?: number; magentaGreen?: number; yellowBlue?: number; black?: number };
  const rangeParams = (params[activeRange as keyof typeof params] as Tone | undefined) ?? {};

  const updateRange = (patch: Tone) => {
    const current = (params[activeRange as keyof typeof params] as Tone | undefined) ?? {};
    update({ [activeRange]: { ...current, ...patch } } as Partial<AdjustmentParamsByKind["selective-color"]>);
  };

  return (
    <EditorSection>
      <ParamSelect label="Colors" value={activeRange} options={ranges} onChange={setActiveRange} />
      <ParamSlider label="Cyan" min={-100} max={100} step={1} value={rangeParams.cyanRed ?? 0} onChange={(v) => updateRange({ cyanRed: v })} formatValue={(v) => `${v}%`} />
      <ParamSlider label="Magenta" min={-100} max={100} step={1} value={rangeParams.magentaGreen ?? 0} onChange={(v) => updateRange({ magentaGreen: v })} formatValue={(v) => `${v}%`} />
      <ParamSlider label="Yellow" min={-100} max={100} step={1} value={rangeParams.yellowBlue ?? 0} onChange={(v) => updateRange({ yellowBlue: v })} formatValue={(v) => `${v}%`} />
      <ParamSlider label="Black" min={-100} max={100} step={1} value={rangeParams.black ?? 0} onChange={(v) => updateRange({ black: v })} formatValue={(v) => `${v}%`} />
      <ParamSelect
        label="Method"
        value={params.mode ?? "relative"}
        options={[
          { value: "relative", label: "Relative" },
          { value: "absolute", label: "Absolute" },
        ]}
        onChange={(v) => update({ mode: v })}
      />
    </EditorSection>
  );
}

function GradientMapEditor({
  params,
  onChange,
}: {
  params: AdjustmentParamsByKind["gradient-map"];
  onChange: (p: AdjustmentLayerParams) => void;
}) {
  const update = (patch: Partial<AdjustmentParamsByKind["gradient-map"]>) =>
    onChange({ ...params, ...patch });

  const stops = params.stops ?? [
    { color: [0, 0, 0, 255], position: 0 },
    { color: [255, 255, 255, 255], position: 1 },
  ];

  // Render gradient preview
  const gradientStyle = (() => {
    const sorted = [...stops].sort((a, b) => a.position - b.position);
    const colorStops = sorted.map(
      (s) => `rgba(${s.color[0]},${s.color[1]},${s.color[2]},${(s.color[3] ?? 255) / 255}) ${s.position * 100}%`,
    );
    return { background: `linear-gradient(to right, ${colorStops.join(", ")})` };
  })();

  return (
    <EditorSection>
      <div
        className="h-4 w-full rounded-[var(--ui-radius-sm)] border border-white/10"
        style={gradientStyle}
        title="Gradient preview (edit via Gradient Map dialog)"
      />
      <ParamCheckbox label="Reverse" checked={params.reverse ?? false} onChange={(v) => update({ reverse: v })} />
    </EditorSection>
  );
}

// ---------------------------------------------------------------------------
// Mask section
// ---------------------------------------------------------------------------

function MaskSection({
  engine,
  layer,
}: {
  engine: EngineContextValue;
  layer: LayerNodeMeta;
}) {
  if (!layer.hasMask) return null;

  const toggleMask = () => {
    engine.dispatchCommand(CommandID.SetLayerMaskEnabled, {
      layerId: layer.id,
      enabled: !layer.maskEnabled,
    });
  };

  const deleteMask = () => {
    engine.dispatchCommand(CommandID.DeleteLayerMask, {
      layerId: layer.id,
    });
  };

  const invertMask = () => {
    engine.dispatchCommand(CommandID.InvertLayerMask, {
      layerId: layer.id,
    });
  };

  return (
    <div className="space-y-1.5 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 p-2">
      <div className="flex items-center justify-between">
        <span className="text-[10px] uppercase tracking-[0.15em] text-slate-500">
          Mask
        </span>
        <div className="flex gap-1">
          <HeaderButton
            title={layer.maskEnabled ? "Disable mask" : "Enable mask"}
            active={layer.maskEnabled}
            onClick={toggleMask}
          >
            {layer.maskEnabled ? "On" : "Off"}
          </HeaderButton>
          <HeaderButton title="Invert mask" onClick={invertMask}>
            Inv
          </HeaderButton>
          <HeaderButton title="Delete mask" onClick={deleteMask}>
            Del
          </HeaderButton>
        </div>
      </div>
      <p className="text-[10px] text-slate-600">
        Density and Feather controls require engine support (planned).
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function findLayerById(
  layers: LayerNodeMeta[],
  id: string,
): LayerNodeMeta | null {
  for (const layer of layers) {
    if (layer.id === id) return layer;
    if (layer.children) {
      const found = findLayerById(layer.children, id);
      if (found) return found;
    }
  }
  return null;
}

function findLayerPositionInTree(
  layers: LayerNodeMeta[],
  targetId: string,
  parentId = "",
): { parentId: string; index: number } | null {
  for (let i = 0; i < layers.length; i++) {
    if (layers[i].id === targetId) {
      return { parentId, index: i };
    }
    const children = layers[i].children;
    if (children) {
      const found = findLayerPositionInTree(
        children,
        targetId,
        layers[i].id,
      );
      if (found) return found;
    }
  }
  return null;
}
