import { useMemo, useState } from "react";
import {
  SHAPE_PRESET_CATEGORIES,
  shapePresetToSvgPathData,
  type ShapePreset,
  type ShapePresetCategory,
} from "@/lib/shape-presets";

type ShapesPanelProps = {
  active: boolean;
  presets: ShapePreset[];
  customPresetIds?: string[];
  selectedPresetId: string;
  onSelectPreset: (preset: ShapePreset) => void;
  onImportPresets?: () => void;
  importStatus?: string | null;
};

const categoryLabels: Record<"all" | ShapePresetCategory, string> = {
  all: "All",
  arrows: "Arrows",
  imported: "Imported",
  logos: "Logos",
  nature: "Nature",
  ornaments: "Ornaments",
};

export function ShapesPanel({
  active,
  presets,
  customPresetIds = [],
  selectedPresetId,
  onSelectPreset,
  onImportPresets,
  importStatus,
}: ShapesPanelProps) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<"all" | ShapePresetCategory>("all");
  const customPresetIdSet = useMemo(() => new Set(customPresetIds), [customPresetIds]);

  const selectedPreset = useMemo(
    () => presets.find((preset) => preset.id === selectedPresetId) ?? presets[0] ?? null,
    [presets, selectedPresetId],
  );

  const filteredPresets = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return presets.filter((preset) => {
      if (category !== "all" && preset.category !== category) {
        return false;
      }
      if (!needle) {
        return true;
      }
      return (
        preset.name.toLowerCase().includes(needle) || preset.category.toLowerCase().includes(needle)
      );
    });
  }, [category, presets, query]);

  const visibleCategories = useMemo(
    () =>
      [
        ...SHAPE_PRESET_CATEGORIES,
        ...(presets.some((preset) => customPresetIdSet.has(preset.id))
          ? (["imported"] as const)
          : []),
      ] satisfies ShapePresetCategory[],
    [customPresetIdSet, presets],
  );

  const groupedPresets = useMemo(
    () =>
      visibleCategories
        .map((group) => ({
          category: group,
          presets: filteredPresets.filter((preset) => preset.category === group),
        }))
        .filter((group) => group.presets.length > 0),
    [filteredPresets, visibleCategories],
  );

  if (!active) {
    return (
      <div className="space-y-3 p-3 text-[11px] text-slate-400">
        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Shapes</div>
        <p>Select the Custom Shape subtool to browse and place reusable vector presets.</p>
        {selectedPreset ? (
          <div className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-3">
            <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
              Current Preset
            </div>
            <div className="mt-1 text-[12px] text-slate-100">{selectedPreset.name}</div>
            <div className="text-[11px] text-slate-500">
              {categoryLabels[selectedPreset.category]}
            </div>
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-white/10 px-3 py-2">
        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Shape Library</div>
        <div className="mt-1 text-[12px] text-slate-100">
          {selectedPreset ? selectedPreset.name : "Choose a preset"}
        </div>
        <input
          data-testid="shape-preset-search"
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search shapes"
          className="mt-2 w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 py-1.5 text-[12px] text-slate-100 placeholder:text-slate-500 focus-visible:outline-none"
        />
        <div className="mt-2 flex flex-wrap gap-1">
          {(["all", ...visibleCategories] as Array<"all" | ShapePresetCategory>).map((value) => (
            <button
              key={value}
              type="button"
              data-testid={`shape-category-${value}`}
              className={[
                "rounded border px-2 py-1 text-[11px] transition focus-visible:outline-none",
                category === value
                  ? "border-cyan-400/35 bg-cyan-400/12 text-slate-100"
                  : "border-white/10 bg-black/20 text-slate-400 hover:border-white/20 hover:text-slate-200",
              ].join(" ")}
              onClick={() => setCategory(value)}
            >
              {categoryLabels[value]}
            </button>
          ))}
        </div>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          {onImportPresets ? (
            <button
              type="button"
              className="rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-1 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
              onClick={onImportPresets}
            >
              Import Shapes
            </button>
          ) : null}
          {importStatus ? <span className="text-[11px] text-slate-500">{importStatus}</span> : null}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-3">
        {groupedPresets.length === 0 ? (
          <div className="rounded-[var(--ui-radius-sm)] border border-dashed border-white/10 p-4 text-[11px] text-slate-500">
            No presets match the current search.
          </div>
        ) : (
          <div className="space-y-4">
            {groupedPresets.map((group) => (
              <section key={group.category} className="space-y-2">
                <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
                  {categoryLabels[group.category]}
                </div>
                <div className="grid grid-cols-2 gap-2">
                  {group.presets.map((preset) => {
                    const activeCard = preset.id === selectedPresetId;
                    return (
                      <button
                        key={preset.id}
                        type="button"
                        data-testid={`shape-preset-card-${preset.id}`}
                        className={[
                          "rounded-[var(--ui-radius-sm)] border p-2 text-left transition focus-visible:outline-none",
                          activeCard
                            ? "border-cyan-400/35 bg-cyan-400/10"
                            : "border-white/10 bg-black/20 hover:border-white/20 hover:bg-black/30",
                        ].join(" ")}
                        onClick={() => onSelectPreset(preset)}
                      >
                        <div className="flex items-center justify-center rounded-[var(--ui-radius-sm)] border border-white/8 bg-[radial-gradient(circle_at_top,rgba(34,211,238,0.12),transparent_60%),linear-gradient(180deg,rgba(15,23,42,0.95),rgba(2,6,23,0.95))] p-2">
                          <svg
                            aria-hidden="true"
                            viewBox="0 0 1 1"
                            className="h-16 w-16 overflow-visible"
                          >
                            <path
                              d={shapePresetToSvgPathData(preset)}
                              fill={preset.closed ? "rgba(125, 211, 252, 0.2)" : "none"}
                              stroke="rgba(103, 232, 249, 0.95)"
                              strokeWidth="0.06"
                              vectorEffect="non-scaling-stroke"
                            />
                          </svg>
                        </div>
                        <div className="mt-2 text-[12px] text-slate-100">{preset.name}</div>
                        <div className="text-[10px] uppercase tracking-[0.12em] text-slate-500">
                          {categoryLabels[preset.category]}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
