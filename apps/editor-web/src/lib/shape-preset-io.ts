import type { PathPointCommand, ShapeSubpathCommand } from "@agogo/proto";
import {
  getShapePresetSubpaths,
  SHAPE_PRESETS,
  type ShapePreset,
  type ShapePresetCategory,
} from "@/lib/shape-presets";

type ShapePresetRecord = {
  presets: ShapePreset[];
  sourceName: string;
};

type ResourceBlock = {
  id: number;
  name: string;
  data: Uint8Array;
};

type ParsedPath = {
  subpaths: ShapeSubpathCommand[];
};

const pathRecordLength = 26;

export async function loadShapePresetFile(file: File): Promise<ShapePresetRecord> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  const sourceName = file.name.replace(/\.[^.]+$/, "") || "Imported Shapes";
  const lowerName = file.name.toLowerCase();
  if (lowerName.endsWith(".json")) {
    return {
      presets: parseShapePresetJSON(new TextDecoder().decode(bytes), sourceName),
      sourceName,
    };
  }
  if (lowerName.endsWith(".csh")) {
    return {
      presets: parseCshShapePresets(bytes, sourceName),
      sourceName,
    };
  }
  throw new Error("Unsupported shape file. Use .csh or .json.");
}

export function parseShapePresetJSON(json: string, sourceName = "Imported Shapes"): ShapePreset[] {
  const parsed = JSON.parse(json) as unknown;
  const rawPresets = Array.isArray(parsed)
    ? parsed
    : parsed &&
        typeof parsed === "object" &&
        Array.isArray((parsed as { presets?: unknown[] }).presets)
      ? (parsed as { presets: unknown[] }).presets
      : [];
  const presets = rawPresets.flatMap((preset, index) => {
    const sanitized = sanitizeShapePreset(preset, `${slug(sourceName)}-${index + 1}`);
    return sanitized ? [sanitized] : [];
  });
  if (presets.length === 0) {
    throw new Error("No shape presets found in the imported JSON.");
  }
  return dedupeShapePresets(presets);
}

export function parseCshShapePresets(
  bytes: Uint8Array,
  sourceName = "Imported Shapes",
): ShapePreset[] {
  const presets: ShapePreset[] = [];
  let fallbackIndex = 1;

  for (const block of extractImageResourceBlocks(bytes)) {
    if (block.id < 2000 || block.id > 2997) {
      continue;
    }
    const parsed = parsePathResource(block.data);
    if (!parsed || parsed.subpaths.length === 0) {
      continue;
    }
    const normalizedSubpaths = normalizePresetSubpaths(parsed.subpaths);
    if (normalizedSubpaths.length === 0) {
      continue;
    }
    const name = normalizePresetName(block.name) || `${sourceName} ${fallbackIndex}`;
    const first = normalizedSubpaths[0];
    presets.push({
      id: `${slug(sourceName)}-${fallbackIndex}`,
      name,
      category: "imported",
      closed: first.closed,
      points: first.points,
      subpaths: normalizedSubpaths,
    });
    fallbackIndex += 1;
  }

  if (presets.length === 0) {
    throw new Error("No supported custom shapes were found in the CSH file.");
  }
  return dedupeShapePresets(presets);
}

function sanitizeShapePreset(candidate: unknown, fallbackId: string): ShapePreset | null {
  if (!candidate || typeof candidate !== "object") {
    return null;
  }
  const raw = candidate as Partial<ShapePreset> & { subpaths?: unknown };
  const name = typeof raw.name === "string" ? normalizePresetName(raw.name) : "";
  if (!name) {
    return null;
  }

  const rawSubpaths = Array.isArray(raw.subpaths)
    ? raw.subpaths.flatMap((subpath) => sanitizeSubpath(subpath))
    : [];
  const legacyPoints = sanitizePoints(raw.points);
  const subpaths =
    rawSubpaths.length > 0
      ? rawSubpaths
      : legacyPoints.length > 0
        ? [{ closed: raw.closed ?? true, points: legacyPoints }]
        : [];
  if (subpaths.length === 0) {
    return null;
  }

  const normalizedSubpaths = normalizePresetSubpaths(subpaths);
  if (normalizedSubpaths.length === 0) {
    return null;
  }
  const first = normalizedSubpaths[0];
  return {
    id: typeof raw.id === "string" && raw.id.trim().length > 0 ? raw.id.trim() : fallbackId,
    name,
    category: isShapePresetCategory(raw.category) ? raw.category : "imported",
    closed: first.closed,
    points: first.points,
    subpaths: normalizedSubpaths,
  };
}

function sanitizeSubpath(candidate: unknown): ShapeSubpathCommand[] {
  if (!candidate || typeof candidate !== "object") {
    return [];
  }
  const raw = candidate as Partial<ShapeSubpathCommand>;
  const points = sanitizePoints(raw.points);
  if (points.length === 0) {
    return [];
  }
  return [{ closed: raw.closed ?? true, points }];
}

function sanitizePoints(points: unknown): PathPointCommand[] {
  if (!Array.isArray(points)) {
    return [];
  }
  return points
    .map((point) => sanitizePoint(point))
    .filter((point): point is PathPointCommand => point !== null);
}

function sanitizePoint(point: unknown): PathPointCommand | null {
  if (!point || typeof point !== "object") {
    return null;
  }
  const raw = point as Partial<PathPointCommand>;
  if (!isFiniteNumber(raw.x) || !isFiniteNumber(raw.y)) {
    return null;
  }
  const inX = isFiniteNumber(raw.inX) ? raw.inX : raw.x;
  const inY = isFiniteNumber(raw.inY) ? raw.inY : raw.y;
  const outX = isFiniteNumber(raw.outX) ? raw.outX : raw.x;
  const outY = isFiniteNumber(raw.outY) ? raw.outY : raw.y;
  return {
    x: raw.x,
    y: raw.y,
    inX,
    inY,
    outX,
    outY,
    handleType: clampHandleType(raw.handleType),
  };
}

function extractImageResourceBlocks(bytes: Uint8Array): ResourceBlock[] {
  const blocks: ResourceBlock[] = [];
  let offset = 0;
  while (offset <= bytes.length - 4) {
    if (
      bytes[offset] !== 0x38 ||
      bytes[offset + 1] !== 0x42 ||
      bytes[offset + 2] !== 0x49 ||
      bytes[offset + 3] !== 0x4d
    ) {
      offset += 1;
      continue;
    }

    if (offset + 10 > bytes.length) {
      break;
    }

    const view = new DataView(bytes.buffer, bytes.byteOffset + offset);
    const id = view.getUint16(4, false);
    let cursor = offset + 6;
    const nameLength = bytes[cursor];
    cursor += 1;
    if (cursor + nameLength > bytes.length) {
      offset += 4;
      continue;
    }
    const rawName = bytes.subarray(cursor, cursor + nameLength);
    cursor += nameLength;
    if ((1 + nameLength) % 2 !== 0) {
      cursor += 1;
    }
    if (cursor + 4 > bytes.length) {
      offset += 4;
      continue;
    }
    const size = new DataView(bytes.buffer, bytes.byteOffset + cursor).getUint32(0, false);
    cursor += 4;
    if (cursor + size > bytes.length) {
      offset += 4;
      continue;
    }
    const data = bytes.slice(cursor, cursor + size);
    cursor += size;
    if (size % 2 !== 0) {
      cursor += 1;
    }
    blocks.push({
      id,
      name: new TextDecoder("latin1").decode(rawName),
      data,
    });
    offset = cursor;
  }
  return blocks;
}

function parsePathResource(data: Uint8Array): ParsedPath | null {
  if (data.length < pathRecordLength || data.length % pathRecordLength !== 0) {
    return null;
  }

  const subpaths: ShapeSubpathCommand[] = [];
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  let currentSubpath: ShapeSubpathCommand | null = null;
  let remainingPoints = 0;

  for (let offset = 0; offset < data.length; offset += pathRecordLength) {
    const selector = view.getUint16(offset, false);
    if (selector === 0 || selector === 3) {
      if (currentSubpath && currentSubpath.points.length > 0) {
        subpaths.push(currentSubpath);
      }
      currentSubpath = { closed: selector === 0, points: [] };
      remainingPoints = view.getUint16(offset + 2, false);
      continue;
    }
    if (selector === 6 || selector === 7 || selector === 8) {
      continue;
    }
    if (selector !== 1 && selector !== 2 && selector !== 4 && selector !== 5) {
      continue;
    }
    if (!currentSubpath) {
      continue;
    }

    currentSubpath.points.push(
      parseBezierPoint(view, offset + 2, selector === 1 || selector === 4),
    );
    remainingPoints -= 1;
    if (remainingPoints === 0) {
      subpaths.push(currentSubpath);
      currentSubpath = null;
    }
  }

  if (currentSubpath && currentSubpath.points.length > 0) {
    subpaths.push(currentSubpath);
  }

  return subpaths.length > 0 ? { subpaths } : null;
}

function parseBezierPoint(view: DataView, offset: number, linked: boolean): PathPointCommand {
  const inY = readFixedPoint(view, offset);
  const inX = readFixedPoint(view, offset + 4);
  const y = readFixedPoint(view, offset + 8);
  const x = readFixedPoint(view, offset + 12);
  const outY = readFixedPoint(view, offset + 16);
  const outX = readFixedPoint(view, offset + 20);
  return {
    x,
    y,
    inX,
    inY,
    outX,
    outY,
    handleType: linked && (inX !== x || inY !== y || outX !== x || outY !== y) ? 1 : 0,
  };
}

function normalizePresetSubpaths(subpaths: ShapeSubpathCommand[]): ShapeSubpathCommand[] {
  const coords: number[] = [];
  for (const subpath of subpaths) {
    for (const point of subpath.points) {
      coords.push(
        point.x,
        point.y,
        point.inX ?? point.x,
        point.inY ?? point.y,
        point.outX ?? point.x,
        point.outY ?? point.y,
      );
    }
  }
  if (coords.length === 0) {
    return [];
  }

  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;
  for (let index = 0; index < coords.length; index += 2) {
    minX = Math.min(minX, coords[index]);
    minY = Math.min(minY, coords[index + 1]);
    maxX = Math.max(maxX, coords[index]);
    maxY = Math.max(maxY, coords[index + 1]);
  }

  const width = maxX - minX || 1;
  const height = maxY - minY || 1;
  return subpaths
    .map((subpath) => ({
      closed: subpath.closed,
      points: subpath.points.map((point) => ({
        x: (point.x - minX) / width,
        y: (point.y - minY) / height,
        inX: ((point.inX ?? point.x) - minX) / width,
        inY: ((point.inY ?? point.y) - minY) / height,
        outX: ((point.outX ?? point.x) - minX) / width,
        outY: ((point.outY ?? point.y) - minY) / height,
        handleType: clampHandleType(point.handleType),
      })),
    }))
    .filter((subpath) => subpath.points.length > 0);
}

function dedupeShapePresets(presets: ShapePreset[]) {
  const merged: ShapePreset[] = [];
  const usedIds = new Set<string>();
  const usedNames = new Set<string>();

  for (const preset of presets) {
    const normalizedName = normalizePresetName(preset.name);
    if (!normalizedName || usedNames.has(normalizedName.toLowerCase())) {
      continue;
    }
    let id = preset.id;
    let suffix = 2;
    while (usedIds.has(id)) {
      id = `${preset.id}-${suffix}`;
      suffix += 1;
    }
    const subpaths = normalizePresetSubpaths(getShapePresetSubpaths(preset));
    if (subpaths.length === 0) {
      continue;
    }
    merged.push({
      ...preset,
      id,
      name: normalizedName,
      closed: subpaths[0].closed,
      points: subpaths[0].points,
      subpaths,
      category: preset.category ?? "imported",
    });
    usedIds.add(id);
    usedNames.add(normalizedName.toLowerCase());
  }

  return merged;
}

function readFixedPoint(view: DataView, offset: number) {
  return view.getInt32(offset, false) / 16777216;
}

function normalizePresetName(value: string) {
  return value.replace(/\0/g, "").replace(/\s+/g, " ").trim();
}

function clampHandleType(value: unknown) {
  if (value === 1 || value === 2) {
    return value;
  }
  return 0;
}

function isShapePresetCategory(value: unknown): value is ShapePresetCategory {
  return (
    value === "arrows" ||
    value === "logos" ||
    value === "nature" ||
    value === "ornaments" ||
    value === "imported"
  );
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function slug(value: string) {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized || "shape";
}

export function mergeImportedShapePresets(existing: ShapePreset[], imported: ShapePreset[]) {
  const merged = [...existing];
  const usedIds = new Set([...SHAPE_PRESETS, ...existing].map((preset) => preset.id));
  const usedNames = new Set(
    [...SHAPE_PRESETS, ...existing].map((preset) => preset.name.toLowerCase()),
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
    merged.push({ ...preset, id, name: normalizedName, category: "imported" });
    usedIds.add(id);
    usedNames.add(normalizedName.toLowerCase());
  }

  return merged;
}
