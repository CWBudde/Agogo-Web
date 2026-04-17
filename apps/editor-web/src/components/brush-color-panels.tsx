import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import {
  clampByte,
  colorEquals,
  formatPercent,
  hexToRgba,
  hsvToRgba,
  isWebSafeColor,
  rgbaToCss,
  rgbaToHex,
  rgbaToHsv,
  snapToWebSafeColor,
  type Hsv,
  type Rgba,
} from "@/lib/color";

export type BrushTipShape = "round" | "square" | "diamond" | "star" | "line";
export type BrushControlSource = "pressure" | "tilt" | "fade";
export type ColorChannelMode = "rgb" | "hsv";

export type BrushPreset = {
  id: string;
  name: string;
  tipShape: BrushTipShape;
  hardness: number;
  spacing: number;
  angle: number;
};

export const BRUSH_PRESETS: BrushPreset[] = [
  { id: "soft-round", name: "Soft Round", tipShape: "round", hardness: 0.2, spacing: 0.08, angle: 0 },
  { id: "painter-round", name: "Painter Round", tipShape: "round", hardness: 0.6, spacing: 0.14, angle: 0 },
  { id: "hard-square", name: "Hard Square", tipShape: "square", hardness: 1, spacing: 0.22, angle: 0 },
  { id: "flat-diamond", name: "Flat Diamond", tipShape: "diamond", hardness: 0.8, spacing: 0.18, angle: 35 },
  { id: "ink-star", name: "Ink Star", tipShape: "star", hardness: 0.95, spacing: 0.1, angle: 0 },
  { id: "marker-line", name: "Marker Line", tipShape: "line", hardness: 0.7, spacing: 0.3, angle: 0 },
];

export type MixerBrushPreset = {
  id: string;
  name: string;
  description: string;
  baseBrushPresetId: BrushPreset["id"];
  tipShape: BrushTipShape;
  hardness: number;
  spacing: number;
  angle: number;
  wetness: number;
  load: number;
};

export const MIXER_BRUSH_PRESETS: MixerBrushPreset[] = [
  {
    id: "feather-blend",
    name: "Feather Blend",
    description: "Soft, wet transitions for colour melting and gentle pickup.",
    baseBrushPresetId: "soft-round",
    tipShape: "round",
    hardness: 0.28,
    spacing: 0.08,
    angle: 0,
    wetness: 0.9,
    load: 0.58,
  },
  {
    id: "loaded-bristle",
    name: "Loaded Bristle",
    description: "Heavy paint charge with firmer edges for visible dragged colour.",
    baseBrushPresetId: "painter-round",
    tipShape: "round",
    hardness: 0.72,
    spacing: 0.14,
    angle: 10,
    wetness: 0.62,
    load: 0.95,
  },
  {
    id: "fan-smear",
    name: "Fan Smear",
    description: "Broad directional streaks that pull sampled colour into ribbons.",
    baseBrushPresetId: "flat-diamond",
    tipShape: "diamond",
    hardness: 0.82,
    spacing: 0.18,
    angle: 32,
    wetness: 0.88,
    load: 0.72,
  },
  {
    id: "dry-drag",
    name: "Dry Drag",
    description: "Low-load, scratchier pulls that preserve broken bristle marks.",
    baseBrushPresetId: "marker-line",
    tipShape: "line",
    hardness: 0.9,
    spacing: 0.26,
    angle: 0,
    wetness: 0.28,
    load: 0.34,
  },
];

type BrushSettingsPanelProps = {
  selectedPresetId: string;
  onSelectPreset: (preset: BrushPreset) => void;
  title?: string;
  subtitle?: string;
  hidePresetPicker?: boolean;
  tipShape: BrushTipShape;
  onTipShapeChange: (shape: BrushTipShape) => void;
  size: number;
  onSizeChange: (size: number) => void;
  hardness: number;
  onHardnessChange: (value: number) => void;
  angle: number;
  onAngleChange: (value: number) => void;
  roundness: number;
  onRoundnessChange: (value: number) => void;
  spacing: number;
  onSpacingChange: (value: number) => void;
  sizeJitter: number;
  onSizeJitterChange: (value: number) => void;
  opacityJitter: number;
  onOpacityJitterChange: (value: number) => void;
  flowJitter: number;
  onFlowJitterChange: (value: number) => void;
  controlSource: BrushControlSource;
  onControlSourceChange: (value: BrushControlSource) => void;
};

export function BrushPresetPicker({
  selectedPresetId,
  onSelectPreset,
}: {
  selectedPresetId: string;
  onSelectPreset: (preset: BrushPreset) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleEscape);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [open]);

  const selectedPreset = useMemo(
    () => BRUSH_PRESETS.find((preset) => preset.id === selectedPresetId) ?? BRUSH_PRESETS[0],
    [selectedPresetId],
  );
  const filteredPresets = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) {
      return BRUSH_PRESETS;
    }
    return BRUSH_PRESETS.filter((preset) => preset.name.toLowerCase().includes(needle));
  }, [query]);

  return (
    <div ref={rootRef} className="relative">
      <Button
        variant="ghost"
        size="sm"
        className="h-7 border border-white/10 bg-black/20 px-2 text-[11px] text-slate-200 hover:bg-white/6"
        onClick={() => setOpen((current) => !current)}
      >
        <span className="mr-2 text-slate-500">Preset</span>
        {selectedPreset.name}
      </Button>

      {open ? (
        <div className="editor-popup absolute left-0 top-[calc(100%+6px)] z-50 w-[22rem] rounded-[var(--ui-radius-md)] p-3">
          <div className="flex items-center gap-2 border-b border-white/8 pb-2">
            <input
              className="h-8 flex-1 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/25 px-2 text-[12px] text-slate-100 outline-none"
              value={query}
              placeholder="Search brush presets"
              onChange={(event) => setQuery(event.target.value)}
            />
            <button
              type="button"
              className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 py-1 text-[11px] text-slate-300 hover:bg-white/6"
              onClick={() => setQuery("")}
            >
              Clear
            </button>
          </div>
          <div className="mt-3 grid grid-cols-2 gap-2">
            {filteredPresets.map((preset) => {
              const active = preset.id === selectedPresetId;
              return (
                <button
                  key={preset.id}
                  type="button"
                  className={[
                    "rounded-[var(--ui-radius-sm)] border p-2 text-left transition focus-visible:outline-none",
                    active
                      ? "border-cyan-400/35 bg-cyan-400/12"
                      : "border-white/8 bg-black/16 hover:border-white/16 hover:bg-white/5",
                  ].join(" ")}
                  onClick={() => {
                    onSelectPreset(preset);
                    setOpen(false);
                  }}
                >
                  <BrushTipPreview shape={preset.tipShape} />
                  <div className="mt-2 text-[12px] text-slate-100">{preset.name}</div>
                  <div className="mt-0.5 text-[11px] text-slate-500">
                    {formatPercent(preset.spacing)} spacing · {Math.round(preset.hardness * 100)}% hard
                  </div>
                </button>
              );
            })}
            {filteredPresets.length === 0 ? (
              <p className="col-span-2 py-8 text-center text-[12px] text-slate-500">
                No presets match that search.
              </p>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function BrushSettingsPanel({
  selectedPresetId,
  onSelectPreset,
  title = "Brush Tip",
  subtitle,
  hidePresetPicker = false,
  tipShape,
  onTipShapeChange,
  size,
  onSizeChange,
  hardness,
  onHardnessChange,
  angle,
  onAngleChange,
  roundness,
  onRoundnessChange,
  spacing,
  onSpacingChange,
  sizeJitter,
  onSizeJitterChange,
  opacityJitter,
  onOpacityJitterChange,
  flowJitter,
  onFlowJitterChange,
  controlSource,
  onControlSourceChange,
}: BrushSettingsPanelProps) {
  const currentPreset = useMemo(
    () => BRUSH_PRESETS.find((preset) => preset.id === selectedPresetId) ?? BRUSH_PRESETS[0],
    [selectedPresetId],
  );

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
            {title}
          </p>
          <p className="text-[12px] text-slate-200">{subtitle ?? currentPreset.name}</p>
        </div>
        {hidePresetPicker ? null : (
          <BrushPresetPicker selectedPresetId={selectedPresetId} onSelectPreset={onSelectPreset} />
        )}
      </div>

      <div className="grid grid-cols-2 gap-2">
        {(["round", "square", "diamond", "star", "line"] satisfies BrushTipShape[]).map((shape) => (
          <button
            key={shape}
            type="button"
            className={[
              "rounded-[var(--ui-radius-sm)] border p-2 text-left transition focus-visible:outline-none",
              tipShape === shape
                ? "border-cyan-400/35 bg-cyan-400/12"
                : "border-white/8 bg-black/16 hover:border-white/16 hover:bg-white/5",
            ].join(" ")}
            onClick={() => onTipShapeChange(shape)}
          >
            <BrushTipPreview shape={shape} />
            <div className="mt-2 text-[12px] text-slate-100">
              {shape.charAt(0).toUpperCase() + shape.slice(1)}
            </div>
          </button>
        ))}
      </div>

      <BrushTipPreview
        shape={tipShape}
        size={size}
        hardness={hardness}
        angle={angle}
        roundness={roundness}
        spacing={spacing}
        className="h-24"
      />

      <div className="grid gap-3">
        <RangeControl label="Size" min={1} max={2500} step={1} value={size} onChange={onSizeChange} />
        <RangeControl label="Hardness" min={0} max={1} step={0.01} value={hardness} onChange={onHardnessChange} />
        <RangeControl label="Angle" min={-180} max={180} step={1} value={angle} onChange={onAngleChange} />
        <RangeControl label="Roundness" min={0} max={1} step={0.01} value={roundness} onChange={onRoundnessChange} />
        <RangeControl label="Spacing" min={0.01} max={2} step={0.01} value={spacing} onChange={onSpacingChange} />
      </div>

      <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
        <div className="mb-2 flex items-center justify-between">
          <div>
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Dynamics</p>
            <p className="text-[12px] text-slate-300">Phase 4.1b controls</p>
          </div>
          <select
            className="h-7 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
            value={controlSource}
            onChange={(event) => onControlSourceChange(event.target.value as BrushControlSource)}
          >
            <option value="pressure">Pressure</option>
            <option value="tilt">Tilt</option>
            <option value="fade">Fade</option>
          </select>
        </div>
        <div className="grid gap-2">
          <RangeControl label="Size Jitter" min={0} max={1} step={0.01} value={sizeJitter} onChange={onSizeJitterChange} />
          <RangeControl label="Opacity Jitter" min={0} max={1} step={0.01} value={opacityJitter} onChange={onOpacityJitterChange} />
          <RangeControl label="Flow Jitter" min={0} max={1} step={0.01} value={flowJitter} onChange={onFlowJitterChange} />
        </div>
      </div>
    </div>
  );
}

type ColorEditorProps = {
  color: Rgba;
  onChange: (color: Rgba) => void;
  channelMode: ColorChannelMode;
  onChannelModeChange: (mode: ColorChannelMode) => void;
  onlyWebColors: boolean;
  onOnlyWebColorsChange: (value: boolean) => void;
  recentColors: Rgba[];
  onRecentColorSelect: (color: Rgba) => void;
};

export function ColorPickerDialog({
  open,
  title,
  description,
  color,
  onChange,
  onCommit,
  onClose,
  channelMode,
  onChannelModeChange,
  onlyWebColors,
  onOnlyWebColorsChange,
  recentColors,
  onRecentColorSelect,
}: {
  open: boolean;
  title: string;
  description?: string;
  color: Rgba;
  onChange: (color: Rgba) => void;
  onCommit: () => void;
  onClose: () => void;
  channelMode: ColorChannelMode;
  onChannelModeChange: (mode: ColorChannelMode) => void;
  onlyWebColors: boolean;
  onOnlyWebColorsChange: (value: boolean) => void;
  recentColors: Rgba[];
  onRecentColorSelect: (color: Rgba) => void;
}) {
  return (
    <Dialog open={open} title={title} description={description} className="max-w-3xl">
      <ColorEditor
        color={color}
        onChange={onChange}
        channelMode={channelMode}
        onChannelModeChange={onChannelModeChange}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={onOnlyWebColorsChange}
        recentColors={recentColors}
        onRecentColorSelect={onRecentColorSelect}
      />

      <div className="mt-4 flex justify-end gap-2 border-t border-border pt-3">
        <Button variant="ghost" size="sm" onClick={onClose}>
          Cancel
        </Button>
        <Button size="sm" onClick={onCommit}>
          Apply
        </Button>
      </div>
    </Dialog>
  );
}

export function ColorPanel({
  color,
  onChange,
  channelMode,
  onChannelModeChange,
  onlyWebColors,
  onOnlyWebColorsChange,
  recentColors,
  onRecentColorSelect,
}: ColorEditorProps) {
  return (
    <div className="space-y-3">
      <ColorEditor
        color={color}
        onChange={onChange}
        channelMode={channelMode}
        onChannelModeChange={onChannelModeChange}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={onOnlyWebColorsChange}
        recentColors={recentColors}
        onRecentColorSelect={onRecentColorSelect}
      />
    </div>
  );
}

export function SwatchesPanel({
  swatches,
  activeColor,
  onPickForeground,
  onPickBackground,
  onAddSwatch,
  onDeleteSwatch,
}: {
  swatches: Rgba[];
  activeColor: Rgba;
  onPickForeground: (color: Rgba) => void;
  onPickBackground: (color: Rgba) => void;
  onAddSwatch: () => void;
  onDeleteSwatch: (index: number) => void;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div>
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Swatches</p>
          <p className="text-[12px] text-slate-300">Click to set foreground. Alt+click sets background.</p>
        </div>
        <Button variant="ghost" size="sm" className="h-7 px-2 text-[11px]" onClick={onAddSwatch}>
          Add current
        </Button>
      </div>

      <div className="grid grid-cols-6 gap-2">
        {swatches.map((swatch, index) => {
          const selected = colorEquals(swatch, activeColor);
          return (
            <div key={swatch.join("-")} className="group relative">
              <button
                type="button"
                className={[
                  "relative h-10 w-full rounded-[var(--ui-radius-sm)] border transition focus-visible:outline-none",
                  selected ? "border-cyan-400/40" : "border-white/10 hover:border-white/20",
                ].join(" ")}
                style={{ backgroundColor: rgbaToCss(swatch) }}
                title="Click to set foreground. Alt+click for background."
                onClick={(event) => {
                  if (event.altKey) {
                    onPickBackground(swatch);
                  } else {
                    onPickForeground(swatch);
                  }
                }}
              >
                <span className="absolute inset-x-1 bottom-1 hidden rounded bg-black/45 px-1 py-0.5 text-[9px] text-white group-hover:block">
                  {rgbaToHex(swatch)}
                </span>
              </button>
                <button
                  type="button"
                  className="absolute right-1 top-1 hidden rounded bg-black/55 px-1 text-[9px] text-white group-hover:block"
                  aria-label="Delete swatch"
                  onClick={() => onDeleteSwatch(index)}
                >
                ×
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ColorEditor({
  color,
  onChange,
  channelMode,
  onChannelModeChange,
  onlyWebColors,
  onOnlyWebColorsChange,
  recentColors,
  onRecentColorSelect,
}: ColorEditorProps) {
  const [hsv, setHsv] = useState<Hsv>(() => rgbaToHsv(color));
  const [hsvBoxDrag, setHsvBoxDrag] = useState(false);
  const boxRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    setHsv(rgbaToHsv(color));
  }, [color]);

  useEffect(() => {
    if (!hsvBoxDrag) {
      return;
    }
    const handleMove = (event: PointerEvent) => {
      const rect = boxRef.current?.getBoundingClientRect();
      if (!rect) {
        return;
      }
      const saturation = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
      const value = Math.max(0, Math.min(1, 1 - (event.clientY - rect.top) / rect.height));
      const next = hsvToRgba([hsv[0], saturation, value], color[3]);
      onChange(onlyWebColors ? snapToWebSafeColor(next) : next);
      setHsv((current) => [current[0], saturation, value]);
    };
    const handleUp = () => setHsvBoxDrag(false);
    window.addEventListener("pointermove", handleMove);
    window.addEventListener("pointerup", handleUp);
    return () => {
      window.removeEventListener("pointermove", handleMove);
      window.removeEventListener("pointerup", handleUp);
    };
  }, [color, hsv, hsvBoxDrag, onChange, onlyWebColors]);

  const handleChange = (next: Rgba) => {
    const snapped = onlyWebColors ? snapToWebSafeColor(next) : next;
    onChange(snapped);
    setHsv(rgbaToHsv(snapped));
  };

  const hexValue = rgbaToHex(color);

  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
      <div className="space-y-3">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Active Color</p>
              <p className="text-[12px] text-slate-300">{isWebSafeColor(color) ? "Web-safe" : "Full gamut"}</p>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                className={[
                  "h-10 w-10 rounded-[var(--ui-radius-sm)] border border-white/10",
                  onlyWebColors ? "ring-1 ring-cyan-400/30" : "",
                ].join(" ")}
                style={{ backgroundColor: rgbaToCss(color) }}
                title="Current color"
              />
              <div className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-3 py-2 text-[12px] text-slate-300">
                <div>{hexValue}</div>
                <div className="text-slate-500">
                  RGB {color.slice(0, 3).join(", ")}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="space-y-2 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1">
              <button
                type="button"
                className={[
                  "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                  channelMode === "rgb"
                    ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                    : "border-white/8 text-slate-400 hover:bg-white/5",
                ].join(" ")}
                onClick={() => onChannelModeChange("rgb")}
              >
                RGB
              </button>
              <button
                type="button"
                className={[
                  "rounded-[var(--ui-radius-sm)] border px-2 py-1 text-[11px] transition",
                  channelMode === "hsv"
                    ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                    : "border-white/8 text-slate-400 hover:bg-white/5",
                ].join(" ")}
                onClick={() => onChannelModeChange("hsv")}
              >
                HSB
              </button>
            </div>
            <label className="flex items-center gap-2 text-[11px] text-slate-400">
              <input
                type="checkbox"
                checked={onlyWebColors}
                onChange={(event) => onOnlyWebColorsChange(event.target.checked)}
              />
              Only Web Colors
            </label>
          </div>

          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/25 p-2">
            <div
              ref={boxRef}
              className="relative aspect-[16/11] overflow-hidden rounded-[var(--ui-radius-sm)] border border-white/8"
              style={{
                backgroundColor: `hsl(${Math.round(hsv[0])} 100% 50%)`,
                touchAction: "none",
              }}
              onPointerDown={(event) => {
                setHsvBoxDrag(true);
                (event.currentTarget as HTMLDivElement).setPointerCapture(event.pointerId);
                const rect = event.currentTarget.getBoundingClientRect();
                const saturation = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
                const value = Math.max(0, Math.min(1, 1 - (event.clientY - rect.top) / rect.height));
                handleChange(hsvToRgba([hsv[0], saturation, value], color[3]));
              }}
            >
              <div className="absolute inset-0 bg-[linear-gradient(90deg,rgba(255,255,255,1),rgba(255,255,255,0))]" />
              <div className="absolute inset-0 bg-[linear-gradient(0deg,rgba(0,0,0,1),rgba(0,0,0,0))]" />
              <div
                className="absolute h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border border-white shadow-[0_0_0_1px_rgba(0,0,0,0.6)]"
                style={{
                  left: `${hsv[1] * 100}%`,
                  top: `${(1 - hsv[2]) * 100}%`,
                  backgroundColor: rgbaToCss(color),
                }}
              />
            </div>
            <div className="mt-2">
              <RangeControl
                label="Hue"
                min={0}
                max={360}
                step={1}
                value={hsv[0]}
                onChange={(value) => handleChange(hsvToRgba([value, hsv[1], hsv[2]], color[3]))}
              />
            </div>
          </div>

          {channelMode === "rgb" ? (
            <div className="grid gap-2 sm:grid-cols-3">
              {(["R", "G", "B"] as const).map((label, index) => (
                <RangeControl
                  key={label}
                  label={label}
                  min={0}
                  max={255}
                  step={1}
                  value={color[index]}
                  onChange={(value) => {
                    const next: Rgba = [
                      index === 0 ? value : color[0],
                      index === 1 ? value : color[1],
                      index === 2 ? value : color[2],
                      color[3],
                    ];
                    handleChange(next);
                  }}
                />
              ))}
            </div>
          ) : (
            <div className="grid gap-2 sm:grid-cols-3">
              <RangeControl
                label="H"
                min={0}
                max={360}
                step={1}
                value={hsv[0]}
                onChange={(value) => handleChange(hsvToRgba([value, hsv[1], hsv[2]], color[3]))}
              />
              <RangeControl
                label="S"
                min={0}
                max={1}
                step={0.01}
                value={hsv[1]}
                onChange={(value) => handleChange(hsvToRgba([hsv[0], value, hsv[2]], color[3]))}
              />
              <RangeControl
                label="B"
                min={0}
                max={1}
                step={0.01}
                value={hsv[2]}
                onChange={(value) => handleChange(hsvToRgba([hsv[0], hsv[1], value], color[3]))}
              />
            </div>
          )}

          <div className="grid gap-2 sm:grid-cols-[8rem_minmax(0,1fr)]">
            <label className="flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Hex</span>
              <input
                className="h-8 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
                value={hexValue}
                onChange={(event) => {
                  const parsed = hexToRgba(event.target.value);
                  if (parsed) {
                    handleChange(parsed);
                  }
                }}
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Alpha</span>
              <input
                className="h-8 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
                type="number"
                min={0}
                max={255}
                value={color[3]}
                onChange={(event) => handleChange([color[0], color[1], color[2], clampByte(Number(event.target.value))])}
              />
            </label>
          </div>
        </div>
      </div>

      <div className="space-y-3">
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Recent Colors</p>
          <div className="mt-2 grid grid-cols-5 gap-2">
            {recentColors.length > 0 ? (
              recentColors.map((recent) => (
                <button
                  key={recent.join("-")}
                  type="button"
                  className="h-10 rounded-[var(--ui-radius-sm)] border border-white/10"
                  style={{ backgroundColor: rgbaToCss(recent) }}
                  onClick={() => onRecentColorSelect(recent)}
                  title={rgbaToHex(recent)}
                />
              ))
            ) : (
              <p className="col-span-5 py-4 text-center text-[12px] text-slate-500">
                No recent colors yet.
              </p>
            )}
          </div>
        </div>

        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
          <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Web Palette</p>
          <div className="mt-2 grid grid-cols-6 gap-2">
            {WEB_SAFE_SWATCHES.map((swatch) => (
              <button
                key={rgbaToHex(swatch)}
                type="button"
                className="h-8 rounded-[var(--ui-radius-sm)] border border-white/10"
                style={{ backgroundColor: rgbaToCss(swatch) }}
                title={rgbaToHex(swatch)}
                onClick={() => handleChange(swatch)}
              />
            ))}
          </div>
          <div className="mt-3 flex items-center gap-2 text-[11px] text-slate-500">
            <span
              className={[
                "h-2.5 w-2.5 rounded-full",
                isWebSafeColor(color) ? "bg-emerald-400" : "bg-amber-400",
              ].join(" ")}
            />
            <span>{isWebSafeColor(color) ? "Color already matches the web-safe palette." : "Color will snap when Only Web Colors is enabled."}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function RangeControl({
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
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{Number.isInteger(value) ? Math.round(value) : value.toFixed(2)}</span>
      </div>
      <input
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

function BrushTipPreview({
  shape,
  size = 72,
  hardness = 0.7,
  angle = 0,
  roundness = 0.75,
  spacing = 0.1,
  className,
}: {
  shape: BrushTipShape;
  size?: number;
  hardness?: number;
  angle?: number;
  roundness?: number;
  spacing?: number;
  className?: string;
}) {
  const opacity = 0.3 + hardness * 0.7;
  const tipClassName =
    shape === "round"
      ? "rounded-full"
      : shape === "square"
        ? "rounded-sm"
        : shape === "diamond"
          ? "rotate-45 rounded-sm"
          : shape === "star"
            ? ""
            : "rounded-full";

  return (
    <div className={["flex items-center justify-center rounded-[var(--ui-radius-sm)] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0.01))]", className ?? "h-16"].join(" ")}>
      <div
        className={[
          "relative flex items-center justify-center overflow-hidden border border-white/10 bg-cyan-400/20",
          shape === "line" ? "w-10 h-4" : "w-10 h-10",
          tipClassName,
        ].join(" ")}
        style={{
          opacity,
          transform: `rotate(${angle}deg) scale(${1 - spacing * 0.1}, ${1 - roundness * 0.15})`,
          boxShadow: `0 0 0 ${Math.max(1, size / 32)}px rgba(34, 211, 238, 0.08)`,
          clipPath:
            shape === "star"
              ? "polygon(50% 0%, 61% 35%, 98% 35%, 68% 57%, 79% 91%, 50% 70%, 21% 91%, 32% 57%, 2% 35%, 39% 35%)"
              : undefined,
        }}
      >
        {shape === "star" ? <div className="h-6 w-6 rotate-45 bg-cyan-200/80" /> : null}
      </div>
    </div>
  );
}

const WEB_SAFE_SWATCHES: Rgba[] = [
  [0, 0, 0, 255],
  [255, 255, 255, 255],
  [255, 0, 0, 255],
  [0, 255, 0, 255],
  [0, 0, 255, 255],
  [255, 255, 0, 255],
  [255, 0, 255, 255],
  [0, 255, 255, 255],
  [153, 102, 51, 255],
  [102, 153, 204, 255],
  [204, 102, 153, 255],
  [51, 153, 102, 255],
];
