import { type Rgba, rgbaToCss, rgbaToHex } from "@/lib/color";

export type ColorSamplerPoint = {
  id: string;
  x: number;
  y: number;
  sampleSize: number;
  sampleMerged: boolean;
  sampleAllLayersNoAdj: boolean;
  color: Rgba | null;
};

type InfoPanelProps = {
  cursorText: string;
  documentSize: string;
  zoomPercent: string;
  samplerPoints: ColorSamplerPoint[];
  onRemoveSampler(id: string): void;
  onClearSamplers(): void;
};

export function InfoPanel({
  cursorText,
  documentSize,
  zoomPercent,
  samplerPoints,
  onRemoveSampler,
  onClearSamplers,
}: InfoPanelProps) {
  return (
    <div className="space-y-3">
      <div className="grid gap-2 sm:grid-cols-3">
        <InfoMetric label="Cursor" value={cursorText} />
        <InfoMetric label="Document" value={documentSize} />
        <InfoMetric label="Zoom" value={zoomPercent} />
      </div>

      <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/14 p-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
              Color Samplers
            </p>
            <p className="mt-1 text-[12px] text-slate-300">
              Shift-click with the Eyedropper to place up to four live sampler points.
            </p>
          </div>
          {samplerPoints.length > 0 ? (
            <button
              type="button"
              className="rounded border border-white/10 px-2 py-1 text-[11px] text-slate-300 transition hover:border-white/20 hover:bg-white/5 hover:text-white"
              onClick={onClearSamplers}
            >
              Clear All
            </button>
          ) : null}
        </div>

        {samplerPoints.length === 0 ? (
          <div className="mt-3 rounded-[var(--ui-radius-sm)] border border-dashed border-white/10 bg-black/10 px-3 py-4 text-[12px] text-slate-500">
            No sampler points yet.
          </div>
        ) : (
          <div className="mt-3 space-y-3">
            {samplerPoints.map((point, index) => (
              <ColorSamplerCard
                key={point.id}
                index={index}
                point={point}
                onRemove={() => onRemoveSampler(point.id)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function InfoMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 px-3 py-2">
      <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">{label}</div>
      <div className="mt-1 text-[12px] text-slate-100">{value}</div>
    </div>
  );
}

function ColorSamplerCard({
  index,
  point,
  onRemove,
}: {
  index: number;
  point: ColorSamplerPoint;
  onRemove(): void;
}) {
  const sampleLabel = point.sampleSize === 1 ? "Point" : `${point.sampleSize}x${point.sampleSize}`;
  const sourceLabel = point.sampleAllLayersNoAdj
    ? "All Layers, No Adj"
    : point.sampleMerged
      ? "Sample Merged"
      : "Current Layer";

  return (
    <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/10 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-cyan-400/35 bg-cyan-400/12 text-[12px] font-semibold text-cyan-100">
            {index + 1}
          </div>
          <div>
            <div className="text-[12px] text-slate-100">{sampleLabel}</div>
            <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
              {sourceLabel}
            </div>
          </div>
        </div>
        <button
          type="button"
          className="rounded border border-white/10 px-2 py-1 text-[11px] text-slate-400 transition hover:border-white/20 hover:bg-white/5 hover:text-white"
          onClick={onRemove}
        >
          Remove
        </button>
      </div>

      <div className="mt-3 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[11px]">
        <span className="uppercase tracking-[0.18em] text-slate-500">X</span>
        <span className="font-mono text-slate-200">{formatCoordinate(point.x)}</span>
        <span className="uppercase tracking-[0.18em] text-slate-500">Y</span>
        <span className="font-mono text-slate-200">{formatCoordinate(point.y)}</span>
        <span className="uppercase tracking-[0.18em] text-slate-500">RGB</span>
        <span className="font-mono text-slate-200">
          {point.color ? `${point.color[0]}, ${point.color[1]}, ${point.color[2]}` : "Unavailable"}
        </span>
        <span className="uppercase tracking-[0.18em] text-slate-500">Hex</span>
        <span className="font-mono text-slate-200">
          {point.color ? rgbaToHex(point.color).toUpperCase() : "Unavailable"}
        </span>
      </div>

      <div className="mt-3 flex items-center gap-2">
        <div className="relative h-5 w-10 overflow-hidden rounded border border-white/10 bg-slate-900">
          <div
            className="absolute inset-0"
            style={{
              backgroundImage:
                "linear-gradient(45deg, rgba(148,163,184,0.28) 25%, transparent 25%, transparent 50%, rgba(148,163,184,0.28) 50%, rgba(148,163,184,0.28) 75%, transparent 75%, transparent)",
              backgroundSize: "8px 8px",
            }}
          />
          <div
            className="absolute inset-0"
            style={{ backgroundColor: point.color ? rgbaToCss(point.color) : "rgba(15, 23, 42, 0.9)" }}
          />
        </div>
        <span className="text-[11px] text-slate-400">
          {point.color ? `Alpha ${point.color[3]}` : "Point is outside the sampled surface."}
        </span>
      </div>
    </div>
  );
}

function formatCoordinate(value: number) {
  const rounded = Math.round(value * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}
