import {
  BRUSH_PRESETS,
  type BrushPreset,
  type BrushTipShape,
} from "@/components/brush-color-panels";

type BrushPresetRecord = {
  presets: BrushPreset[];
  sourceName: string;
};

export async function loadBrushPresetFile(file: File): Promise<BrushPresetRecord> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  const sourceName = file.name.replace(/\.[^.]+$/, "") || "Imported Brushes";
  const lowerName = file.name.toLowerCase();
  if (lowerName.endsWith(".json")) {
    return {
      presets: parseBrushPresetJSON(new TextDecoder().decode(bytes), sourceName),
      sourceName,
    };
  }
  throw new Error(
    "Unsupported brush preset file. JSON presets are parsed in the UI; ABR uses the engine importer.",
  );
}

export function parseBrushPresetJSON(json: string, sourceName = "Imported Brushes"): BrushPreset[] {
  const parsed = JSON.parse(json) as unknown;
  const rawPresets = Array.isArray(parsed)
    ? parsed
    : parsed &&
        typeof parsed === "object" &&
        Array.isArray((parsed as { presets?: unknown[] }).presets)
      ? (parsed as { presets: unknown[] }).presets
      : [];
  const presets = rawPresets.flatMap((preset, index) => {
    const sanitized = sanitizeBrushPreset(preset, `${slug(sourceName)}-${index + 1}`);
    return sanitized ? [sanitized] : [];
  });
  if (presets.length === 0) {
    throw new Error("No brush presets found in the imported JSON.");
  }
  return dedupeBrushPresets(presets);
}

function sanitizeBrushPreset(candidate: unknown, fallbackId: string): BrushPreset | null {
  if (!candidate || typeof candidate !== "object") {
    return null;
  }
  const raw = candidate as Partial<BrushPreset>;
  const name = typeof raw.name === "string" ? raw.name.trim() : "";
  if (!name) {
    return null;
  }
  const tipShape = isBrushTipShape(raw.tipShape) ? raw.tipShape : inferTipShape(name);
  const controlSource = isBrushControlSource(raw.controlSource) ? raw.controlSource : "pressure";
  return {
    id: typeof raw.id === "string" && raw.id.trim().length > 0 ? raw.id.trim() : fallbackId,
    name,
    tipShape,
    tipResourceId:
      typeof raw.tipResourceId === "string" && raw.tipResourceId.trim()
        ? raw.tipResourceId.trim()
        : undefined,
    thumbnailRGBA:
      typeof raw.thumbnailRGBA === "string" && raw.thumbnailRGBA.trim()
        ? raw.thumbnailRGBA.trim()
        : undefined,
    size: typeof raw.size === "number" ? clampNumber(raw.size, 1, 2500) : undefined,
    hardness: clampUnitNumber(
      typeof raw.hardness === "number" ? raw.hardness : inferHardness(name),
    ),
    spacing: clampNumber(
      typeof raw.spacing === "number" ? raw.spacing : inferSpacing(name),
      0.01,
      2,
    ),
    angle: clampNumber(typeof raw.angle === "number" ? raw.angle : inferAngle(tipShape), -180, 180),
    roundness: clampNumber(typeof raw.roundness === "number" ? raw.roundness : 1, 0.01, 1),
    sizeJitter: clampUnitNumber(typeof raw.sizeJitter === "number" ? raw.sizeJitter : 0),
    opacityJitter: clampUnitNumber(typeof raw.opacityJitter === "number" ? raw.opacityJitter : 0),
    flowJitter: clampUnitNumber(typeof raw.flowJitter === "number" ? raw.flowJitter : 0),
    controlSource,
    fadeDabs: clampNumber(typeof raw.fadeDabs === "number" ? raw.fadeDabs : 100, 1, 10000),
  };
}

function dedupeBrushPresets(presets: BrushPreset[]) {
  const seen = new Set<string>();
  return presets.filter((preset) => {
    const key = preset.name.toLowerCase();
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function inferTipShape(name: string): BrushTipShape {
  const lower = name.toLowerCase();
  if (/\b(square|box|block)\b/.test(lower)) {
    return "square";
  }
  if (/\b(diamond|lozenge|flat)\b/.test(lower)) {
    return "diamond";
  }
  if (/\b(star|burst|spark)\b/.test(lower)) {
    return "star";
  }
  if (/\b(line|marker|chisel|stroke|flat brush)\b/.test(lower)) {
    return "line";
  }
  return "round";
}

function inferHardness(name: string) {
  const lower = name.toLowerCase();
  if (/\b(soft|feather|air|charcoal)\b/.test(lower)) {
    return 0.22;
  }
  if (/\b(hard|ink|chalk|stamp)\b/.test(lower)) {
    return 0.94;
  }
  if (/\b(marker|line|flat)\b/.test(lower)) {
    return 0.72;
  }
  return 0.6;
}

function inferSpacing(name: string) {
  const lower = name.toLowerCase();
  if (/\b(star|burst)\b/.test(lower)) {
    return 0.1;
  }
  if (/\b(line|marker|chisel)\b/.test(lower)) {
    return 0.28;
  }
  if (/\b(soft|feather)\b/.test(lower)) {
    return 0.08;
  }
  return 0.14;
}

function inferAngle(tipShape: BrushTipShape) {
  if (tipShape === "diamond") {
    return 35;
  }
  return 0;
}

function isBrushTipShape(value: unknown): value is BrushTipShape {
  return (
    value === "round" ||
    value === "square" ||
    value === "diamond" ||
    value === "star" ||
    value === "line"
  );
}

function isBrushControlSource(value: unknown): value is NonNullable<BrushPreset["controlSource"]> {
  return value === "off" || value === "pressure" || value === "tilt" || value === "fade";
}

function clampUnitNumber(value: number) {
  return clampNumber(value, 0, 1);
}

function clampNumber(value: number, min: number, max: number) {
  if (Number.isNaN(value)) {
    return min;
  }
  return Math.max(min, Math.min(max, value));
}

function slug(value: string) {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized || "brush";
}

export function mergeImportedBrushPresets(existing: BrushPreset[], imported: BrushPreset[]) {
  const merged = [...existing];
  const usedIds = new Set([...BRUSH_PRESETS, ...existing].map((preset) => preset.id));
  const usedNames = new Set(
    [...BRUSH_PRESETS, ...existing].map((preset) => preset.name.toLowerCase()),
  );

  for (const preset of imported) {
    const normalizedName = preset.name.trim();
    if (!normalizedName || usedNames.has(normalizedName.toLowerCase())) {
      continue;
    }
    let id = preset.id;
    let suffix = 2;
    while (usedIds.has(id)) {
      id = `${preset.id}-${suffix}`;
      suffix += 1;
    }
    merged.push({ ...preset, id, name: normalizedName });
    usedIds.add(id);
    usedNames.add(normalizedName.toLowerCase());
  }

  return merged;
}
