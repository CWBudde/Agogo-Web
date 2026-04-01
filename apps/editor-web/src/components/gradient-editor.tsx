import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import type { GradientStopCommand } from "@agogo/proto";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { ColorPickerDialog, type ColorChannelMode } from "@/components/brush-color-panels";
import { clampUnit, rgbaToCss, rgbaToHex, toMutableRgba, toRgba, type Rgba } from "@/lib/color";

type GradientEditorProps = {
  open: boolean;
  title?: string;
  description?: string;
  stops: GradientStopCommand[];
  onStopsChange(stops: GradientStopCommand[]): void;
  recentColors: Rgba[];
  onRecentColorSelect(color: Rgba): void;
  channelMode: ColorChannelMode;
  onChannelModeChange(mode: ColorChannelMode): void;
  onlyWebColors: boolean;
  onOnlyWebColorsChange(value: boolean): void;
  onClose(): void;
};

type GradientEditorStop = Omit<GradientStopCommand, "color"> & {
  color: Rgba;
  id: string;
};

type GradientPreset = {
  id: string;
  name: string;
  stops: GradientStopCommand[];
};

const GRADIENT_PRESET_STORAGE_KEY = "AGOGO_GRADIENT_PRESETS_V1";

const DEFAULT_GRADIENT_PRESETS: GradientPreset[] = [
  {
    id: "sunset",
    name: "Sunset",
    stops: [
      { position: 0, color: [253, 186, 116, 255] },
      { position: 0.5, color: [244, 114, 182, 255] },
      { position: 1, color: [59, 130, 246, 255] },
    ],
  },
  {
    id: "ice",
    name: "Ice",
    stops: [
      { position: 0, color: [224, 242, 254, 255] },
      { position: 0.5, color: [125, 211, 252, 255] },
      { position: 1, color: [15, 23, 42, 255] },
    ],
  },
  {
    id: "mono",
    name: "Black to White",
    stops: [
      { position: 0, color: [0, 0, 0, 255] },
      { position: 1, color: [255, 255, 255, 255] },
    ],
  },
];

function makeStopId() {
  return globalThis.crypto?.randomUUID?.() ?? `stop-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function clampPosition(value: number) {
  return clampUnit(value);
}

function normalizeStops(stops: GradientStopCommand[]) {
  const normalized = stops.length > 0 ? stops : [
    { position: 0, color: [0, 0, 0, 255] },
    { position: 1, color: [255, 255, 255, 255] },
  ];
  const sorted = normalized
    .map((stop, index) => ({
      id: makeStopId(),
      position: clampPosition(stop.position),
      color: toRgba(stop.color),
      order: index,
    }))
    .sort((a, b) => a.position - b.position || a.order - b.order);
  if (sorted.length === 1) {
    sorted.push({
      id: makeStopId(),
      position: 1,
      color: sorted[0].color,
      order: 1,
    });
  }
  return sorted.map(({ id, position, color }) => ({ id, position, color }));
}

function serializeStops(stops: GradientEditorStop[]): GradientStopCommand[] {
  return stops
    .map((stop) => ({
      position: clampPosition(stop.position),
      color: toMutableRgba(stop.color),
    }))
    .sort((a, b) => a.position - b.position);
}

function mixColors(left: Rgba, right: Rgba, t: number): Rgba {
  const clamped = clampUnit(t);
  return [
    clampByte(left[0] + (right[0] - left[0]) * clamped),
    clampByte(left[1] + (right[1] - left[1]) * clamped),
    clampByte(left[2] + (right[2] - left[2]) * clamped),
    clampByte(left[3] + (right[3] - left[3]) * clamped),
  ];
}

function findInsertColor(stops: GradientEditorStop[], position: number): Rgba {
  if (stops.length === 0) {
    return [0, 0, 0, 255];
  }
  if (position <= stops[0].position) {
    return stops[0].color;
  }
  if (position >= stops[stops.length - 1].position) {
    return stops[stops.length - 1].color;
  }
  for (let i = 1; i < stops.length; i++) {
    const left = stops[i - 1];
    const right = stops[i];
    if (position <= right.position) {
      const span = right.position - left.position || 1;
      return mixColors(left.color, right.color, (position - left.position) / span);
    }
  }
  return stops[stops.length - 1].color;
}

function gradientToCss(stops: GradientStopCommand[]) {
  const normalized = stops
    .map((stop) => ({
      position: clampPosition(stop.position),
      color: toRgba(stop.color),
    }))
    .sort((a, b) => a.position - b.position);
  return `linear-gradient(90deg, ${normalized
    .map((stop) => `${rgbaToCss(stop.color)} ${Math.round(stop.position * 100)}%`)
    .join(", ")})`;
}

function loadUserPresets(): GradientPreset[] {
  if (typeof window === "undefined") {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(GRADIENT_PRESET_STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw) as GradientPreset[];
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.filter((preset) => typeof preset?.name === "string" && Array.isArray(preset?.stops));
  } catch {
    return [];
  }
}

export function GradientEditorDialog({
  open,
  title = "Gradient Editor",
  description = "Edit color stops and load preset gradients.",
  stops,
  onStopsChange,
  recentColors,
  onRecentColorSelect,
  channelMode,
  onChannelModeChange,
  onlyWebColors,
  onOnlyWebColorsChange,
  onClose,
}: GradientEditorProps) {
  const [editorStops, setEditorStops] = useState<GradientEditorStop[]>(() => normalizeStops(stops));
  const [selectedStopId, setSelectedStopId] = useState<string | null>(null);
  const [draggingStopId, setDraggingStopId] = useState<string | null>(null);
  const [stopColorOpen, setStopColorOpen] = useState(false);
  const [stopColorDraft, setStopColorDraft] = useState<Rgba>([0, 0, 0, 255]);
  const [presetName, setPresetName] = useState("");
  const [userPresets, setUserPresets] = useState<GradientPreset[]>(() => loadUserPresets());
  const trackRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const normalized = normalizeStops(stops);
    setEditorStops(normalized);
    setSelectedStopId(normalized[0]?.id ?? null);
    setDraggingStopId(null);
  }, [open, stops]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    try {
      window.localStorage.setItem(GRADIENT_PRESET_STORAGE_KEY, JSON.stringify(userPresets));
    } catch {
      // Ignore storage failures.
    }
  }, [userPresets]);

  const allPresets = useMemo(
    () => [...DEFAULT_GRADIENT_PRESETS, ...userPresets],
    [userPresets],
  );

  const selectedStop = editorStops.find((stop) => stop.id === selectedStopId) ?? editorStops[0] ?? null;

  useEffect(() => {
    if (selectedStop && selectedStop.id !== selectedStopId) {
      setSelectedStopId(selectedStop.id);
    }
  }, [selectedStop, selectedStopId]);

  const commitStops = (nextStops: GradientEditorStop[]) => {
    const normalized = nextStops
      .map((stop) => ({
        ...stop,
        position: clampPosition(stop.position),
        color: toRgba(stop.color),
      }))
      .sort((a, b) => a.position - b.position);
    if (normalized.length === 1) {
      normalized.push({
        id: makeStopId(),
        position: 1,
        color: normalized[0].color,
      });
    }
    setEditorStops(normalized);
    onStopsChange(serializeStops(normalized));
  };

  const updateSelectedStop = (patch: Partial<GradientEditorStop>) => {
    if (!selectedStop) {
      return;
    }
    commitStops(
      editorStops.map((stop) =>
        stop.id === selectedStop.id ? { ...stop, ...patch } : stop,
      ),
    );
  };

  const handleTrackPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect) {
      return;
    }
    const position = clampPosition((event.clientX - rect.left) / rect.width);
    const hit = editorStops.find((stop) => Math.abs(stop.position - position) < 0.035);
    if (hit) {
      setSelectedStopId(hit.id);
      setDraggingStopId(hit.id);
      event.currentTarget.setPointerCapture(event.pointerId);
      return;
    }
    const color = findInsertColor(editorStops, position);
    const nextStop: GradientEditorStop = {
      id: makeStopId(),
      position,
      color,
    };
    setSelectedStopId(nextStop.id);
    commitStops([...editorStops, nextStop]);
    setDraggingStopId(nextStop.id);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handleTrackPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!draggingStopId || !trackRef.current) {
      return;
    }
    const rect = trackRef.current.getBoundingClientRect();
    const position = clampPosition((event.clientX - rect.left) / rect.width);
    commitStops(
      editorStops.map((stop) =>
        stop.id === draggingStopId ? { ...stop, position } : stop,
      ),
    );
  };

  const handleTrackPointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (draggingStopId) {
      setDraggingStopId(null);
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  const handleAddStop = () => {
    const basePosition = selectedStop ? clampPosition(selectedStop.position + 0.1) : 0.5;
    const color = findInsertColor(editorStops, basePosition);
    const nextStop: GradientEditorStop = {
      id: makeStopId(),
      position: basePosition,
      color,
    };
    setSelectedStopId(nextStop.id);
    commitStops([...editorStops, nextStop]);
  };

  const handleDeleteStop = () => {
    if (!selectedStop || editorStops.length <= 2) {
      return;
    }
    const remaining = editorStops.filter((stop) => stop.id !== selectedStop.id);
    setSelectedStopId(remaining[0]?.id ?? null);
    commitStops(remaining);
  };

  const handleLoadPreset = (preset: GradientPreset) => {
    const loaded = normalizeStops(preset.stops);
    setEditorStops(loaded);
    setSelectedStopId(loaded[0]?.id ?? null);
    onStopsChange(serializeStops(loaded));
  };

  const handleSavePreset = () => {
    const name = presetName.trim();
    if (!name) {
      return;
    }
    const nextPreset: GradientPreset = {
      id: name.toLowerCase().replace(/\s+/g, "-"),
      name,
      stops: serializeStops(editorStops),
    };
    setUserPresets((current) => {
      const withoutDuplicate = current.filter((preset) => preset.name.toLowerCase() !== name.toLowerCase());
      return [nextPreset, ...withoutDuplicate];
    });
  };

  const handleDeletePreset = (id: string) => {
    setUserPresets((current) => current.filter((preset) => preset.id !== id));
  };

  const openSelectedStopColor = () => {
    if (!selectedStop) {
      return;
    }
    setStopColorDraft(selectedStop.color);
    setStopColorOpen(true);
  };

  const selectedStopColorLabel = selectedStop ? rgbaToHex(selectedStop.color) : "#000000";

  return (
    <>
      <Dialog open={open} title={title} description={description} className="max-w-4xl">
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_18rem]">
            <div className="space-y-3">
              <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Stops</p>
                    <p className="text-[12px] text-slate-300">Drag to move. Click empty space to add a stop.</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="ghost" size="sm" className="h-7 px-2 text-[11px]" onClick={handleAddStop}>
                      Add
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-[11px]"
                      disabled={!selectedStop || editorStops.length <= 2}
                      onClick={handleDeleteStop}
                    >
                      Remove
                    </Button>
                  </div>
                </div>

                <div
                  ref={trackRef}
                  className="relative mt-3 h-16 rounded border border-white/10 bg-[linear-gradient(90deg,rgba(15,23,42,0.95),rgba(15,23,42,0.7))]"
                  style={{ backgroundImage: gradientToCss(serializeStops(editorStops)) }}
                  onPointerDown={handleTrackPointerDown}
                  onPointerMove={handleTrackPointerMove}
                  onPointerUp={handleTrackPointerUp}
                >
                  {editorStops.map((stop) => {
                    const active = stop.id === selectedStop?.id;
                    return (
                      <button
                        key={stop.id}
                        type="button"
                        className={[
                          "absolute top-0 h-full w-5 -translate-x-1/2 focus-visible:outline-none",
                          active ? "z-20" : "z-10",
                        ].join(" ")}
                        style={{ left: `${stop.position * 100}%` }}
                        onPointerDown={(event) => {
                          setSelectedStopId(stop.id);
                          setDraggingStopId(stop.id);
                          trackRef.current?.setPointerCapture(event.pointerId);
                          event.preventDefault();
                          event.stopPropagation();
                        }}
                        onDoubleClick={() => {
                          setSelectedStopId(stop.id);
                          openSelectedStopColor();
                        }}
                        title={`${Math.round(stop.position * 100)}% ${rgbaToHex(stop.color)}`}
                      >
                        <span
                          className={[
                            "absolute left-1/2 top-1/2 h-6 w-4 -translate-x-1/2 -translate-y-1/2 rounded border",
                            active ? "border-cyan-300" : "border-white/50",
                          ].join(" ")}
                          style={{ backgroundColor: rgbaToCss(stop.color) }}
                        />
                        <span
                          className={[
                            "absolute left-1/2 top-0 h-3 w-3 -translate-x-1/2 rotate-45 border border-white/25 bg-slate-100",
                            active ? "bg-cyan-200" : "",
                          ].join(" ")}
                        />
                      </button>
                    );
                  })}
                </div>
              </div>

              {selectedStop ? (
                <div className="grid gap-3 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3 sm:grid-cols-[10rem_minmax(0,1fr)]">
                  <div className="space-y-2">
                    <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Selected Stop</p>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-full justify-start border border-white/10 px-2 text-[11px] text-slate-200"
                      onClick={openSelectedStopColor}
                    >
                      <span
                        className="mr-2 h-4 w-4 rounded border border-white/20"
                        style={{ backgroundColor: rgbaToCss(selectedStop.color) }}
                      />
                      {selectedStopColorLabel}
                    </Button>
                    <div className="grid gap-2">
                      <label className="flex flex-col gap-1">
                        <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Position</span>
                        <input
                          type="number"
                          min={0}
                          max={100}
                          step={1}
                          className="h-8 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
                          value={Math.round(selectedStop.position * 100)}
                          onChange={(event) => {
                            const nextPosition = clampPosition(Number(event.target.value) / 100);
                            updateSelectedStop({ position: nextPosition });
                          }}
                        />
                      </label>
                      <label className="flex flex-col gap-1">
                        <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Opacity</span>
                        <input
                          type="number"
                          min={0}
                          max={255}
                          step={1}
                          className="h-8 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
                          value={selectedStop.color[3]}
                          onChange={(event) => {
                            const alpha = clampByte(Number(event.target.value));
                            updateSelectedStop({ color: [selectedStop.color[0], selectedStop.color[1], selectedStop.color[2], alpha] });
                          }}
                        />
                      </label>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Stops</p>
                    <div className="max-h-56 overflow-auto rounded border border-white/8">
                      {editorStops.map((stop) => {
                        const active = stop.id === selectedStop.id;
                        return (
                          <button
                            key={stop.id}
                            type="button"
                            className={[
                              "flex w-full items-center gap-2 border-b border-white/8 px-2 py-1.5 text-left text-[12px] last:border-b-0",
                              active ? "bg-cyan-400/12 text-slate-100" : "text-slate-300 hover:bg-white/5",
                            ].join(" ")}
                            onClick={() => setSelectedStopId(stop.id)}
                          >
                            <span className="h-4 w-4 rounded border border-white/10" style={{ backgroundColor: rgbaToCss(stop.color) }} />
                            <span className="w-16 shrink-0">{Math.round(stop.position * 100)}%</span>
                            <span className="font-mono text-[11px] text-slate-500">{rgbaToHex(stop.color)}</span>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </div>
              ) : null}
            </div>

            <div className="space-y-3">
              <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
                <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Preview</p>
                <div className="mt-2 h-24 rounded border border-white/10" style={{ backgroundImage: gradientToCss(serializeStops(editorStops)) }} />
              </div>

              <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/16 p-3">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">Presets</p>
                    <p className="text-[12px] text-slate-300">Save and reuse stop layouts.</p>
                  </div>
                </div>
                <div className="mt-3 flex gap-2">
                  <input
                    className="h-8 flex-1 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none"
                    placeholder="Preset name"
                    value={presetName}
                    onChange={(event) => setPresetName(event.target.value)}
                  />
                  <Button variant="ghost" size="sm" className="h-8 px-2 text-[11px]" onClick={handleSavePreset}>
                    Save
                  </Button>
                </div>
                <div className="mt-3 grid gap-2">
                  {allPresets.map((preset) => {
                    const saved = userPresets.some((entry) => entry.id === preset.id);
                    return (
                      <div key={preset.id} className="flex items-center gap-2 rounded border border-white/8 bg-black/10 px-2 py-1.5">
                        <button
                          type="button"
                          className="flex-1 text-left text-[12px] text-slate-100"
                          onClick={() => handleLoadPreset(preset)}
                        >
                          <div>{preset.name}</div>
                          <div className="text-[11px] text-slate-500">{preset.stops.length} stops</div>
                        </button>
                        {saved ? (
                          <button
                            type="button"
                            className="rounded border border-white/10 px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-slate-400 hover:border-red-400/40 hover:text-red-200"
                            onClick={() => handleDeletePreset(preset.id)}
                          >
                            Delete
                          </button>
                        ) : (
                          <span className="rounded border border-white/8 px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-slate-600">
                            Built-in
                          </span>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-white/8 pt-3">
            <div className="flex items-center gap-2 text-[11px] text-slate-400">
              <span className="uppercase tracking-[0.18em] text-slate-500">Alpha</span>
              <span>Stop opacity is stored in the stop color.</span>
            </div>
            <div className="flex items-center gap-2">
              <label className="flex items-center gap-1 text-[10px] text-slate-400">
                <input
                  type="checkbox"
                  checked={onlyWebColors}
                  onChange={(event) => onOnlyWebColorsChange(event.target.checked)}
                />
                Only Web Colors
              </label>
              <Button variant="ghost" size="sm" onClick={onClose}>
                Close
              </Button>
            </div>
          </div>
        </div>
      </Dialog>

      <ColorPickerDialog
        open={stopColorOpen}
        title="Gradient Stop Color"
        description="Edit the selected stop color and opacity."
        color={stopColorDraft}
        onChange={setStopColorDraft}
        onCommit={() => {
          updateSelectedStop({ color: stopColorDraft });
          setStopColorOpen(false);
        }}
        onClose={() => setStopColorOpen(false)}
        channelMode={channelMode}
        onChannelModeChange={onChannelModeChange}
        onlyWebColors={onlyWebColors}
        onOnlyWebColorsChange={onOnlyWebColorsChange}
        recentColors={recentColors}
        onRecentColorSelect={onRecentColorSelect}
      />
    </>
  );
}
