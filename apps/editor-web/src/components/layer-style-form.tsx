import type {
  BlendIfChannel,
  BlendIfConfig,
  BlendIfRange,
  LayerNodeMeta,
  LayerStyleEntryCommand,
  LayerStyleKind,
} from "@agogo/proto";
import { useState } from "react";
import {
  cloneLayerStyleStack,
  createDefaultLayerStyleStack,
  ensureLayerStyleEntry,
  formatLayerStyleLabel,
  supportsLayerStyles,
} from "@/components/layer-style-model";
import { BlendIfSlider } from "./ui/blend-if-slider";

const blendModeOptions = [
  { value: "normal", label: "Normal" },
  { value: "multiply", label: "Multiply" },
  { value: "screen", label: "Screen" },
  { value: "overlay", label: "Overlay" },
];

const contourOptions = [
  { value: "linear", label: "Linear" },
  { value: "gaussian", label: "Gaussian" },
  { value: "cone", label: "Cone" },
  { value: "rolling-slope", label: "Rolling Slope" },
  { value: "rounded-steps", label: "Rounded Steps" },
];

// Satin's engine-side contour enum has no rounded-steps variant.
const satinContourOptions = contourOptions.filter((option) => option.value !== "rounded-steps");

export interface LayerStylePatternOption {
  id: string;
  name: string;
}

export interface GradientStopParam {
  position: number;
  color: [number, number, number, number];
}

// Fallback when the caller does not supply the UIMeta pattern list: the four
// builtin tiles registered by the engine's pattern registry.
const builtinPatternOptions: LayerStylePatternOption[] = [
  { id: "builtin/checker", name: "Checker" },
  { id: "builtin/stripes", name: "Diagonal Stripes" },
  { id: "builtin/dots", name: "Dots" },
  { id: "builtin/noise", name: "Noise" },
];

export function LayerStyleForm({
  layer,
  styles,
  patterns,
  onEnabledChange,
  onParamsChange,
}: {
  layer: LayerNodeMeta | null;
  styles: LayerStyleEntryCommand[] | undefined;
  /** Pattern list from uiMeta.patterns; falls back to the builtin tiles. */
  patterns?: LayerStylePatternOption[];
  onEnabledChange: (kind: LayerStyleKind, enabled: boolean) => void;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  if (!supportsLayerStyles(layer)) {
    return (
      <div className="space-y-2 p-3 text-[11px] text-slate-400">
        <h2 className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Layer Styles</h2>
        <p>Layer styles are currently available for pixel, text, and vector layers.</p>
      </div>
    );
  }

  let catalog = styles?.length ? cloneLayerStyleStack(styles) : createDefaultLayerStyleStack();
  for (const entry of createDefaultLayerStyleStack()) {
    catalog = ensureLayerStyleEntry(catalog, entry.kind).styles;
  }
  const blendIf = layer?.blendIf;
  const isBlendIfSupported = blendIf !== undefined;

  const updateParams = (kind: LayerStyleKind, patch: Record<string, unknown>) => {
    const { entry } = ensureLayerStyleEntry(catalog, kind);
    onParamsChange(kind, { ...(entry.params ?? {}), ...patch });
  };
  const updateBlendIf = (patch: Partial<BlendIfConfig>) => {
    if (!isBlendIfSupported || !blendIf) {
      return;
    }
    onParamsChange("blend-if" as LayerStyleKind, { ...blendIf, ...patch });
  };

  return (
    <div className="space-y-3 p-3">
      <h2 className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Layer Styles</h2>
      <div className="space-y-2">
        {isBlendIfSupported ? <BlendIfSection blendIf={blendIf} onChange={updateBlendIf} /> : null}
        {catalog.map((entry) => (
          <LayerStyleSection
            key={entry.kind}
            entry={entry}
            patterns={patterns?.length ? patterns : builtinPatternOptions}
            onEnabledChange={onEnabledChange}
            onParamsChange={updateParams}
          />
        ))}
      </div>
    </div>
  );
}

function LayerStyleSection({
  entry,
  patterns,
  onEnabledChange,
  onParamsChange,
}: {
  entry: LayerStyleEntryCommand;
  patterns: LayerStylePatternOption[];
  onEnabledChange: (kind: LayerStyleKind, enabled: boolean) => void;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <section className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-2">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-[11px] font-medium text-slate-100">
          {formatLayerStyleLabel(entry.kind)}
        </h3>
        <label className="flex items-center gap-2 text-[11px] text-slate-300">
          <input
            aria-label={formatLayerStyleLabel(entry.kind)}
            type="checkbox"
            className="accent-cyan-400"
            checked={entry.enabled}
            onChange={(event) => onEnabledChange(entry.kind, event.target.checked)}
          />
          Enabled
        </label>
      </div>
      {entry.enabled ? (
        <div className="mt-3 space-y-2">
          {renderEffectEditor(entry.kind, entry.params ?? {}, patterns, onParamsChange)}
        </div>
      ) : null}
    </section>
  );
}

function renderEffectEditor(
  kind: LayerStyleKind,
  params: Record<string, unknown>,
  patterns: LayerStylePatternOption[],
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void,
) {
  switch (kind) {
    case "drop-shadow":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 0.75)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Angle"
            param="angle"
            value={numberParam(params.angle, 120)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Distance"
            param="distance"
            value={numberParam(params.distance, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Spread"
            param="spread"
            value={numberParam(params.spread, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Size"
            param="size"
            value={numberParam(params.size, 0)}
            onParamsChange={onParamsChange}
          />
          <RangeField
            kind={kind}
            label="Noise"
            param="noise"
            value={numberParam(params.noise, 0)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Layer Knocks Out Drop Shadow"
            param="knockout"
            checked={booleanParam(params.knockout, false)}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "inner-shadow":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 0.75)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Angle"
            param="angle"
            value={numberParam(params.angle, 120)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Distance"
            param="distance"
            value={numberParam(params.distance, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Choke"
            param="choke"
            value={numberParam(params.choke, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Size"
            param="size"
            value={numberParam(params.size, 0)}
            onParamsChange={onParamsChange}
          />
          <RangeField
            kind={kind}
            label="Noise"
            param="noise"
            value={numberParam(params.noise, 0)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "outer-glow":
    case "inner-glow":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 0.75)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Spread"
            param="spread"
            value={numberParam(params.spread, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Size"
            param="size"
            value={numberParam(params.size, 0)}
            onParamsChange={onParamsChange}
          />
          <RangeField
            kind={kind}
            label="Noise"
            param="noise"
            value={numberParam(params.noise, 0)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "bevel-emboss":
      return (
        <>
          <SelectField
            kind={kind}
            label="Style"
            param="style"
            value={stringParam(params.style, "inner-bevel")}
            options={[
              { value: "inner-bevel", label: "Inner Bevel" },
              { value: "outer-bevel", label: "Outer Bevel" },
              { value: "emboss", label: "Emboss" },
              { value: "pillow-emboss", label: "Pillow Emboss" },
              { value: "stroke-emboss", label: "Stroke Emboss" },
            ]}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Technique"
            param="technique"
            value={stringParam(params.technique, "smooth")}
            options={[
              { value: "smooth", label: "Smooth" },
              { value: "chisel-hard", label: "Chisel Hard" },
              { value: "chisel-soft", label: "Chisel Soft" },
            ]}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Depth"
            param="depth"
            value={numberParam(params.depth, 1)}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Direction"
            param="direction"
            value={stringParam(params.direction, "up")}
            options={[
              { value: "up", label: "Up" },
              { value: "down", label: "Down" },
            ]}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Size"
            param="size"
            value={numberParam(params.size, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Soften"
            param="soften"
            value={numberParam(params.soften, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Angle"
            param="angle"
            value={numberParam(params.angle, 120)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Altitude"
            param="altitude"
            value={numberParam(params.altitude, 30)}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Contour"
            param="contour"
            value={stringParam(params.contour, "linear")}
            options={contourOptions}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "satin":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 0.5)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Angle"
            param="angle"
            value={numberParam(params.angle, 19)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Distance"
            param="distance"
            value={numberParam(params.distance, 0)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Size"
            param="size"
            value={numberParam(params.size, 0)}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Invert"
            param="invert"
            checked={booleanParam(params.invert, false)}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Contour"
            param="contour"
            value={stringParam(params.contour, "gaussian")}
            options={satinContourOptions}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "color-overlay":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 1)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "gradient-overlay":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 1)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Angle"
            param="angle"
            value={numberParam(params.angle, 90)}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Scale"
            param="scale"
            value={numberParam(params.scale, 1)}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Reverse"
            param="reverse"
            checked={booleanParam(params.reverse, false)}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Dither"
            param="dither"
            checked={booleanParam(params.dither, false)}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Align"
            param="align"
            checked={booleanParam(params.align, true)}
            onParamsChange={onParamsChange}
          />
          <GradientStopsField
            kind={kind}
            param="stops"
            stops={gradientStopsParam(params.stops)}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "pattern-overlay":
      return (
        <>
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 1)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <NumberField
            kind={kind}
            label="Scale"
            param="scale"
            value={numberParam(params.scale, 1)}
            onParamsChange={onParamsChange}
          />
          <CheckboxField
            kind={kind}
            label="Link With Layer"
            param="link"
            checked={booleanParam(params.link, true)}
            onParamsChange={onParamsChange}
          />
          <PatternSelectField
            kind={kind}
            value={stringParam(params.patternId, "")}
            patterns={patterns}
            onParamsChange={onParamsChange}
          />
        </>
      );
    case "stroke":
      return (
        <>
          <NumberField
            kind={kind}
            label="Stroke Size"
            param="size"
            value={numberParam(params.size, 1)}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Position"
            param="position"
            value={stringParam(params.position, "outside")}
            options={[
              { value: "outside", label: "Outside" },
              { value: "inside", label: "Inside" },
              { value: "center", label: "Center" },
            ]}
            onParamsChange={onParamsChange}
          />
          <BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
          <RangeField
            kind={kind}
            label="Opacity"
            param="opacity"
            value={numberParam(params.opacity, 1)}
            min={0}
            max={1}
            step={0.01}
            onParamsChange={onParamsChange}
          />
          <SelectField
            kind={kind}
            label="Fill Type"
            param="fillType"
            value={stringParam(params.fillType, "color")}
            options={[
              { value: "color", label: "Color" },
              { value: "gradient", label: "Gradient" },
              { value: "pattern", label: "Pattern" },
            ]}
            onParamsChange={onParamsChange}
          />
          {stringParam(params.fillType, "color") === "gradient" ? (
            <>
              <NumberField
                kind={kind}
                label="Gradient Angle"
                param="gradientAngle"
                value={numberParam(params.gradientAngle, 90)}
                onParamsChange={onParamsChange}
              />
              <GradientStopsField
                kind={kind}
                param="stops"
                stops={gradientStopsParam(params.stops)}
                onParamsChange={onParamsChange}
              />
            </>
          ) : null}
          {stringParam(params.fillType, "color") === "pattern" ? (
            <PatternSelectField
              kind={kind}
              value={stringParam(params.patternId, "")}
              patterns={patterns}
              onParamsChange={onParamsChange}
            />
          ) : null}
        </>
      );
  }
}

function BlendModeField({
  kind,
  params,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  params: Record<string, unknown>;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <SelectField
      kind={kind}
      label="Blend Mode"
      param="blendMode"
      value={stringParam(params.blendMode, "normal")}
      options={blendModeOptions}
      onParamsChange={onParamsChange}
    />
  );
}

function NumberField({
  kind,
  label,
  param,
  value,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  label: string;
  param: string;
  value: number;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <label className="block">
      <span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
        {label}
      </span>
      <input
        aria-label={label}
        className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
        type="number"
        value={value}
        onChange={(event) => {
          const nextValue = parseFiniteNumber(event.target.value);
          if (nextValue === null) {
            return;
          }
          onParamsChange(kind, { [param]: nextValue });
        }}
      />
    </label>
  );
}

function RangeField({
  kind,
  label,
  param,
  value,
  min,
  max,
  step,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  label: string;
  param: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <label className="block">
      <div className="mb-0.5 flex items-center justify-between text-[10px] uppercase tracking-[0.15em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{value}</span>
      </div>
      <input
        aria-label={label}
        className="h-1.5 w-full accent-cyan-400 focus-visible:outline-none"
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onParamsChange(kind, { [param]: Number(event.target.value) })}
      />
    </label>
  );
}

function SelectField({
  kind,
  label,
  param,
  value,
  options,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  label: string;
  param: string;
  value: string;
  options: { value: string; label: string }[];
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <label className="block">
      <span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
        {label}
      </span>
      <select
        aria-label={label}
        className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
        value={value}
        onChange={(event) => onParamsChange(kind, { [param]: event.target.value })}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function CheckboxField({
  kind,
  label,
  param,
  checked,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  label: string;
  param: string;
  checked: boolean;
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-[11px] text-slate-300">
      <input
        aria-label={label}
        type="checkbox"
        className="accent-cyan-400"
        checked={checked}
        onChange={(event) => onParamsChange(kind, { [param]: event.target.checked })}
      />
      {label}
    </label>
  );
}

function PatternSelectField({
  kind,
  value,
  patterns,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  value: string;
  patterns: LayerStylePatternOption[];
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  return (
    <SelectField
      kind={kind}
      label="Pattern"
      param="patternId"
      value={value}
      options={[
        { value: "", label: "Default Checker" },
        ...patterns.map((pattern) => ({ value: pattern.id, label: pattern.name })),
      ]}
      onParamsChange={onParamsChange}
    />
  );
}

function GradientStopsField({
  kind,
  param,
  stops,
  onParamsChange,
}: {
  kind: LayerStyleKind;
  param: string;
  stops: GradientStopParam[];
  onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
  const emit = (nextStops: GradientStopParam[]) => {
    onParamsChange(kind, { [param]: nextStops });
  };
  const updateStop = (index: number, patch: Partial<GradientStopParam>) => {
    emit(stops.map((stop, i) => (i === index ? { ...stop, ...patch } : stop)));
  };

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <span className="text-[10px] uppercase tracking-[0.15em] text-slate-500">
          Gradient Stops
        </span>
        <button
          type="button"
          className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-0.5 text-[10px] text-slate-200 hover:bg-black/50"
          onClick={() =>
            emit([...stops, { position: stops.length === 0 ? 0 : 1, color: [255, 255, 255, 255] }])
          }
        >
          Add Stop
        </button>
      </div>
      {stops.length === 0 ? (
        <p className="text-[10px] text-slate-500">No stops: default two-color ramp.</p>
      ) : (
        stops.map((stop, index) => (
          <div
            // biome-ignore lint/suspicious/noArrayIndexKey: stops have no stable identity
            key={index}
            className="flex items-center gap-2"
          >
            <input
              aria-label={`Stop ${index + 1} position`}
              className="w-16 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
              type="number"
              min={0}
              max={1}
              step={0.01}
              value={stop.position}
              onChange={(event) => {
                const next = parseFiniteNumber(event.target.value);
                if (next === null) {
                  return;
                }
                updateStop(index, { position: Math.min(1, Math.max(0, next)) });
              }}
            />
            <input
              aria-label={`Stop ${index + 1} color`}
              className="h-6 w-8 cursor-pointer rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30"
              type="color"
              value={rgbaToHex(stop.color)}
              onChange={(event) => updateStop(index, { color: hexToRgba(event.target.value) })}
            />
            <button
              type="button"
              aria-label={`Remove stop ${index + 1}`}
              className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-0.5 text-[10px] text-slate-400 hover:text-slate-100"
              onClick={() => emit(stops.filter((_, i) => i !== index))}
            >
              Remove
            </button>
          </div>
        ))
      )}
    </div>
  );
}

function gradientStopsParam(value: unknown): GradientStopParam[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const stops: GradientStopParam[] = [];
  for (const entry of value) {
    if (typeof entry !== "object" || entry === null) {
      continue;
    }
    const { position, color } = entry as { position?: unknown; color?: unknown };
    if (typeof position !== "number" || !Array.isArray(color) || color.length !== 4) {
      continue;
    }
    stops.push({
      position,
      color: [Number(color[0]), Number(color[1]), Number(color[2]), Number(color[3])],
    });
  }
  return stops;
}

function rgbaToHex(color: [number, number, number, number]): string {
  const channel = (value: number) =>
    Math.min(255, Math.max(0, Math.round(value)))
      .toString(16)
      .padStart(2, "0");
  return `#${channel(color[0])}${channel(color[1])}${channel(color[2])}`;
}

function hexToRgba(hex: string): [number, number, number, number] {
  const parsed = /^#?([0-9a-f]{6})$/i.exec(hex);
  if (!parsed) {
    return [0, 0, 0, 255];
  }
  return [
    Number.parseInt(parsed[1].slice(0, 2), 16),
    Number.parseInt(parsed[1].slice(2, 4), 16),
    Number.parseInt(parsed[1].slice(4, 6), 16),
    255,
  ];
}

function numberParam(value: unknown, fallback: number): number {
  return typeof value === "number" ? value : fallback;
}

function stringParam(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}

function booleanParam(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function parseFiniteNumber(value: string): number | null {
  if (value.trim() === "") {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

type BlendIfChannelKey = keyof BlendIfRange;

const blendIfChannelOptions: { value: BlendIfChannelKey; label: string }[] = [
  { value: "gray", label: "Gray" },
  { value: "red", label: "Red" },
  { value: "green", label: "Green" },
  { value: "blue", label: "Blue" },
];

function BlendIfSection({
  blendIf,
  onChange,
}: {
  blendIf: BlendIfConfig;
  onChange: (patch: Partial<BlendIfConfig>) => void;
}) {
  const [channel, setChannel] = useState<BlendIfChannelKey>("gray");

  const setThisLayer = (next: BlendIfChannel) => {
    onChange({ thisLayer: { ...blendIf.thisLayer, [channel]: next } });
  };
  const setUnderlyingLayer = (next: BlendIfChannel) => {
    onChange({ underlyingLayer: { ...blendIf.underlyingLayer, [channel]: next } });
  };
  const toggleChannel = (key: "r" | "g" | "b", enabled: boolean) => {
    onChange({ channels: { ...blendIf.channels, [key]: enabled } });
  };

  return (
    <section className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-2">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-[11px] font-medium text-slate-100">Blend If</h3>
        <select
          aria-label="Blend If channel"
          className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
          value={channel}
          onChange={(event) => setChannel(event.target.value as BlendIfChannelKey)}
        >
          {blendIfChannelOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
      <div className="mt-3 space-y-3">
        <BlendIfSlider
          label="This Layer"
          value={blendIf.thisLayer[channel]}
          onChange={setThisLayer}
        />
        <BlendIfSlider
          label="Underlying Layer"
          value={blendIf.underlyingLayer[channel]}
          onChange={setUnderlyingLayer}
        />
        <div className="flex items-center gap-3">
          <span className="text-[10px] uppercase tracking-[0.15em] text-slate-500">Channels</span>
          <label className="flex items-center gap-1.5 text-[11px] text-slate-300">
            <input
              aria-label="Channel R"
              type="checkbox"
              className="accent-cyan-400"
              checked={blendIf.channels.r}
              onChange={(event) => toggleChannel("r", event.target.checked)}
            />
            R
          </label>
          <label className="flex items-center gap-1.5 text-[11px] text-slate-300">
            <input
              aria-label="Channel G"
              type="checkbox"
              className="accent-cyan-400"
              checked={blendIf.channels.g}
              onChange={(event) => toggleChannel("g", event.target.checked)}
            />
            G
          </label>
          <label className="flex items-center gap-1.5 text-[11px] text-slate-300">
            <input
              aria-label="Channel B"
              type="checkbox"
              className="accent-cyan-400"
              checked={blendIf.channels.b}
              onChange={(event) => toggleChannel("b", event.target.checked)}
            />
            B
          </label>
        </div>
      </div>
    </section>
  );
}
