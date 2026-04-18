import type { BrushPreset, BrushTipShape } from "@/components/brush-color-panels";

type BrushPresetRecord = {
  presets: BrushPreset[];
  sourceName: string;
};

const abrAsciiPattern = /[A-Za-z][A-Za-z0-9 &'()_,./+-]{2,48}/g;
const abrUtf16Pattern = /[A-Za-z][A-Za-z0-9 &'()_,./+\-\u00c0-\u024f]{2,48}/g;
const ignoredAbrTokens = new Set([
  "8bim",
  "desc",
  "drsh",
  "patt",
  "null",
  "objc",
  "text",
  "long",
  "bool",
  "doub",
  "untf",
  "type",
]);

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
  if (lowerName.endsWith(".abr")) {
    return {
      presets: parseAbrBrushPresets(bytes, sourceName),
      sourceName,
    };
  }
  throw new Error("Unsupported brush preset file. Use .abr or .json.");
}

export function parseBrushPresetJSON(json: string, sourceName = "Imported Brushes"): BrushPreset[] {
  const parsed = JSON.parse(json) as unknown;
  const rawPresets =
    Array.isArray(parsed)
      ? parsed
      : parsed && typeof parsed === "object" && Array.isArray((parsed as { presets?: unknown[] }).presets)
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

export function parseAbrBrushPresets(bytes: Uint8Array, sourceName: string): BrushPreset[] {
  const names = extractAbrPresetNames(bytes);
  const presetNames = names.length > 0 ? names : [sourceName];
  return dedupeBrushPresets(
    presetNames.map((name, index) => makePresetFromName(name, `${slug(sourceName)}-${index + 1}`)),
  );
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
  return {
    id: typeof raw.id === "string" && raw.id.trim().length > 0 ? raw.id.trim() : fallbackId,
    name,
    tipShape,
    hardness: clampUnitNumber(typeof raw.hardness === "number" ? raw.hardness : inferHardness(name)),
    spacing: clampNumber(typeof raw.spacing === "number" ? raw.spacing : inferSpacing(name), 0.01, 2),
    angle: clampNumber(typeof raw.angle === "number" ? raw.angle : inferAngle(tipShape), -180, 180),
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

function extractAbrPresetNames(bytes: Uint8Array) {
  const names = new Set<string>();
  for (const candidate of [
    ...extractPatternMatches(new TextDecoder("latin1").decode(bytes), abrAsciiPattern),
    ...extractPatternMatches(new TextDecoder("utf-16be").decode(bytes), abrUtf16Pattern),
  ]) {
    const normalized = normalizeAbrName(candidate);
    if (normalized) {
      names.add(normalized);
    }
  }
  return [...names].slice(0, 64);
}

function extractPatternMatches(content: string, pattern: RegExp) {
  return [...content.matchAll(pattern)].map((match) => match[0]);
}

function normalizeAbrName(value: string) {
  const normalized = value
    .replace(/\0/g, "")
    .replace(/\s+/g, " ")
    .replace(/^[\s\-_.]+|[\s\-_.]+$/g, "")
    .trim();
  if (
    normalized.length < 3 ||
    normalized.length > 48 ||
    !/[A-Za-z]/.test(normalized) ||
    ignoredAbrTokens.has(normalized.toLowerCase())
  ) {
    return null;
  }
  return normalized;
}

function makePresetFromName(name: string, id: string): BrushPreset {
  const tipShape = inferTipShape(name);
  return {
    id,
    name,
    tipShape,
    hardness: inferHardness(name),
    spacing: inferSpacing(name),
    angle: inferAngle(tipShape),
  };
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
