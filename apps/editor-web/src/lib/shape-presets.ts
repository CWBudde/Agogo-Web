import type { PathPointCommand, ShapeSubpathCommand } from "@agogo/proto";

export type ShapePresetCategory = "arrows" | "logos" | "nature" | "ornaments" | "imported";

export interface ShapePreset {
  id: string;
  name: string;
  category: ShapePresetCategory;
  closed: boolean;
  points: PathPointCommand[];
  subpaths?: ShapeSubpathCommand[];
}

export interface ShapeBounds {
  x: number;
  y: number;
  w: number;
  h: number;
}

type ShapePoint = {
  x: number;
  y: number;
};

function point(x: number, y: number, options: Partial<PathPointCommand> = {}): PathPointCommand {
  return {
    x,
    y,
    inX: options.inX ?? x,
    inY: options.inY ?? y,
    outX: options.outX ?? x,
    outY: options.outY ?? y,
    handleType: options.handleType ?? 0,
  };
}

function makePreset(
  id: string,
  name: string,
  category: ShapePresetCategory,
  points: PathPointCommand[],
  closed = true,
): ShapePreset {
  return { id, name, category, closed, points };
}

function normalizePoint(point: PathPointCommand) {
  return {
    x: point.x,
    y: point.y,
    inX: point.inX ?? point.x,
    inY: point.inY ?? point.y,
    outX: point.outX ?? point.x,
    outY: point.outY ?? point.y,
  };
}

function hasCurvedSegment(prev: PathPointCommand, next: PathPointCommand) {
  const a = normalizePoint(prev);
  const b = normalizePoint(next);
  return a.outX !== a.x || a.outY !== a.y || b.inX !== b.x || b.inY !== b.y;
}

export const SHAPE_PRESET_CATEGORIES: ShapePresetCategory[] = [
  "arrows",
  "logos",
  "nature",
  "ornaments",
];

export function getShapePresetSubpaths(preset: ShapePreset): ShapeSubpathCommand[] {
  if (preset.subpaths && preset.subpaths.length > 0) {
    return preset.subpaths;
  }
  return [{ closed: preset.closed, points: preset.points }];
}

export const SHAPE_PRESETS: ShapePreset[] = [
  makePreset("arrow-right", "Arrow Right", "arrows", [
    point(0.06, 0.34),
    point(0.58, 0.34),
    point(0.58, 0.14),
    point(0.95, 0.5),
    point(0.58, 0.86),
    point(0.58, 0.66),
    point(0.06, 0.66),
  ]),
  makePreset("arrow-left", "Arrow Left", "arrows", [
    point(0.94, 0.34),
    point(0.42, 0.34),
    point(0.42, 0.14),
    point(0.05, 0.5),
    point(0.42, 0.86),
    point(0.42, 0.66),
    point(0.94, 0.66),
  ]),
  makePreset("double-arrow", "Double Arrow", "arrows", [
    point(0.05, 0.5),
    point(0.25, 0.2),
    point(0.25, 0.36),
    point(0.75, 0.36),
    point(0.75, 0.2),
    point(0.95, 0.5),
    point(0.75, 0.8),
    point(0.75, 0.64),
    point(0.25, 0.64),
    point(0.25, 0.8),
  ]),
  makePreset("ribbon-arrow", "Ribbon Arrow", "arrows", [
    point(0.08, 0.28),
    point(0.46, 0.28),
    point(0.46, 0.12),
    point(0.9, 0.5),
    point(0.46, 0.88),
    point(0.46, 0.72),
    point(0.08, 0.72),
    point(0.22, 0.5),
  ]),
  makePreset("orbit-mark", "Orbit Mark", "logos", [
    point(0.5, 0.08, { inX: 0.22, inY: 0.08, outX: 0.78, outY: 0.08, handleType: 1 }),
    point(0.92, 0.5, { inX: 0.92, inY: 0.2, outX: 0.92, outY: 0.8, handleType: 1 }),
    point(0.5, 0.92, { inX: 0.78, inY: 0.92, outX: 0.22, outY: 0.92, handleType: 1 }),
    point(0.08, 0.5, { inX: 0.08, inY: 0.8, outX: 0.08, outY: 0.2, handleType: 1 }),
    point(0.32, 0.5, { inX: 0.32, inY: 0.62, outX: 0.32, outY: 0.38, handleType: 1 }),
    point(0.5, 0.28, { inX: 0.4, inY: 0.28, outX: 0.6, outY: 0.28, handleType: 1 }),
    point(0.68, 0.5, { inX: 0.68, inY: 0.38, outX: 0.68, outY: 0.62, handleType: 1 }),
    point(0.5, 0.72, { inX: 0.6, inY: 0.72, outX: 0.4, outY: 0.72, handleType: 1 }),
  ]),
  makePreset("prism-badge", "Prism Badge", "logos", [
    point(0.5, 0.06),
    point(0.88, 0.28),
    point(0.88, 0.72),
    point(0.5, 0.94),
    point(0.12, 0.72),
    point(0.12, 0.28),
  ]),
  makePreset("signal-emblem", "Signal Emblem", "logos", [
    point(0.5, 0.08),
    point(0.88, 0.22),
    point(0.78, 0.86),
    point(0.5, 0.72),
    point(0.22, 0.86),
    point(0.12, 0.22),
  ]),
  makePreset("loop-badge", "Loop Badge", "logos", [
    point(0.18, 0.32, { outX: 0.18, outY: 0.12, handleType: 1 }),
    point(0.38, 0.12, { inX: 0.28, inY: 0.12, outX: 0.48, outY: 0.12, handleType: 1 }),
    point(0.62, 0.32, { inX: 0.62, inY: 0.12, outX: 0.62, outY: 0.48, handleType: 1 }),
    point(0.82, 0.18, { inX: 0.72, inY: 0.18, outX: 0.94, outY: 0.18, handleType: 1 }),
    point(0.92, 0.5, { inX: 0.94, inY: 0.32, outX: 0.92, outY: 0.68, handleType: 1 }),
    point(0.82, 0.82, { inX: 0.94, inY: 0.82, outX: 0.72, outY: 0.82, handleType: 1 }),
    point(0.62, 0.68, { inX: 0.62, inY: 0.52, outX: 0.62, outY: 0.88, handleType: 1 }),
    point(0.38, 0.88, { inX: 0.48, inY: 0.88, outX: 0.28, outY: 0.88, handleType: 1 }),
    point(0.18, 0.68, { inX: 0.18, inY: 0.88, outX: 0.18, outY: 0.52, handleType: 1 }),
    point(0.36, 0.5, { inX: 0.24, inY: 0.5, outX: 0.48, outY: 0.5, handleType: 1 }),
    point(0.5, 0.34, { inX: 0.5, inY: 0.42, outX: 0.5, outY: 0.24, handleType: 1 }),
  ]),
  makePreset("leaf", "Leaf", "nature", [
    point(0.5, 0.04, { inX: 0.38, inY: 0.1, outX: 0.62, outY: 0.1, handleType: 1 }),
    point(0.9, 0.48, { inX: 0.92, inY: 0.22, outX: 0.86, outY: 0.74, handleType: 1 }),
    point(0.5, 0.96, { inX: 0.64, inY: 0.88, outX: 0.36, outY: 0.88, handleType: 1 }),
    point(0.1, 0.48, { inX: 0.14, inY: 0.74, outX: 0.08, outY: 0.22, handleType: 1 }),
  ]),
  makePreset("droplet", "Droplet", "nature", [
    point(0.5, 0.04, { inX: 0.42, inY: 0.12, outX: 0.66, outY: 0.18, handleType: 1 }),
    point(0.88, 0.42, { inX: 0.92, inY: 0.24, outX: 0.92, outY: 0.7, handleType: 1 }),
    point(0.5, 0.96, { inX: 0.76, inY: 0.92, outX: 0.24, outY: 0.92, handleType: 1 }),
    point(0.12, 0.42, { inX: 0.08, inY: 0.7, outX: 0.08, outY: 0.24, handleType: 1 }),
  ]),
  makePreset("petal", "Petal", "nature", [
    point(0.5, 0.06, { inX: 0.38, inY: 0.12, outX: 0.62, outY: 0.12, handleType: 1 }),
    point(0.82, 0.46, { inX: 0.88, inY: 0.22, outX: 0.78, outY: 0.72, handleType: 1 }),
    point(0.5, 0.94, { inX: 0.68, inY: 0.88, outX: 0.32, outY: 0.88, handleType: 1 }),
    point(0.18, 0.46, { inX: 0.22, inY: 0.72, outX: 0.12, outY: 0.22, handleType: 1 }),
  ]),
  makePreset("pine", "Pine", "nature", [
    point(0.5, 0.05),
    point(0.74, 0.28),
    point(0.61, 0.28),
    point(0.82, 0.5),
    point(0.68, 0.5),
    point(0.88, 0.76),
    point(0.58, 0.76),
    point(0.58, 0.95),
    point(0.42, 0.95),
    point(0.42, 0.76),
    point(0.12, 0.76),
    point(0.32, 0.5),
    point(0.18, 0.5),
    point(0.39, 0.28),
    point(0.26, 0.28),
  ]),
  makePreset("starburst", "Starburst", "ornaments", [
    point(0.5, 0.04),
    point(0.62, 0.28),
    point(0.9, 0.14),
    point(0.76, 0.42),
    point(0.98, 0.5),
    point(0.76, 0.58),
    point(0.9, 0.86),
    point(0.62, 0.72),
    point(0.5, 0.96),
    point(0.38, 0.72),
    point(0.1, 0.86),
    point(0.24, 0.58),
    point(0.02, 0.5),
    point(0.24, 0.42),
    point(0.1, 0.14),
    point(0.38, 0.28),
  ]),
  makePreset("clover", "Clover", "ornaments", [
    point(0.5, 0.24, { inX: 0.44, inY: 0.06, outX: 0.56, outY: 0.06, handleType: 1 }),
    point(0.72, 0.42, { inX: 0.86, inY: 0.18, outX: 0.86, outY: 0.5, handleType: 1 }),
    point(0.9, 0.5, { inX: 0.82, inY: 0.5, outX: 0.82, outY: 0.62, handleType: 1 }),
    point(0.72, 0.7, { inX: 0.86, inY: 0.62, outX: 0.62, outY: 0.82, handleType: 1 }),
    point(0.54, 0.62, { inX: 0.6, inY: 0.56, outX: 0.58, outY: 0.8, handleType: 1 }),
    point(0.58, 0.96),
    point(0.42, 0.96),
    point(0.46, 0.62, { inX: 0.42, inY: 0.8, outX: 0.4, outY: 0.56, handleType: 1 }),
    point(0.28, 0.7, { inX: 0.38, inY: 0.82, outX: 0.14, outY: 0.62, handleType: 1 }),
    point(0.1, 0.5, { inX: 0.18, inY: 0.62, outX: 0.18, outY: 0.5, handleType: 1 }),
    point(0.28, 0.42, { inX: 0.14, inY: 0.5, outX: 0.14, outY: 0.18, handleType: 1 }),
  ]),
  makePreset("medallion", "Medallion", "ornaments", [
    point(0.5, 0.06),
    point(0.66, 0.18),
    point(0.86, 0.14),
    point(0.82, 0.34),
    point(0.94, 0.5),
    point(0.82, 0.66),
    point(0.86, 0.86),
    point(0.66, 0.82),
    point(0.5, 0.94),
    point(0.34, 0.82),
    point(0.14, 0.86),
    point(0.18, 0.66),
    point(0.06, 0.5),
    point(0.18, 0.34),
    point(0.14, 0.14),
    point(0.34, 0.18),
  ]),
  makePreset("flourish", "Flourish", "ornaments", [
    point(0.12, 0.58, { outX: 0.18, outY: 0.3, handleType: 1 }),
    point(0.34, 0.22, { inX: 0.24, inY: 0.18, outX: 0.5, outY: 0.24, handleType: 1 }),
    point(0.68, 0.12, { inX: 0.58, inY: 0.08, outX: 0.84, outY: 0.18, handleType: 1 }),
    point(0.9, 0.38, { inX: 0.96, inY: 0.24, outX: 0.88, outY: 0.58, handleType: 1 }),
    point(0.72, 0.62, { inX: 0.84, inY: 0.62, outX: 0.56, outY: 0.64, handleType: 1 }),
    point(0.9, 0.92, { inX: 0.82, inY: 0.8, outX: 0.72, outY: 0.96, handleType: 1 }),
    point(0.54, 0.86, { inX: 0.68, inY: 0.98, outX: 0.42, outY: 0.82, handleType: 1 }),
    point(0.18, 0.96, { inX: 0.3, inY: 0.84, outX: 0.06, outY: 0.84, handleType: 1 }),
    point(0.2, 0.68, { inX: 0.06, inY: 0.74, outX: 0.3, outY: 0.68, handleType: 1 }),
    point(0.42, 0.58, { inX: 0.34, inY: 0.54, outX: 0.34, outY: 0.42, handleType: 1 }),
  ]),
] satisfies ShapePreset[];

export function getShapePresetById(id: string) {
  return SHAPE_PRESETS.find((preset) => preset.id === id) ?? null;
}

export function resolveShapeDragBounds(
  start: ShapePoint,
  current: ShapePoint,
  constrain = false,
): ShapeBounds {
  let x = Math.min(start.x, current.x);
  let y = Math.min(start.y, current.y);
  let w = Math.abs(current.x - start.x);
  let h = Math.abs(current.y - start.y);

  if (constrain) {
    const size = Math.max(w, h);
    x = current.x >= start.x ? start.x : start.x - size;
    y = current.y >= start.y ? start.y : start.y - size;
    w = size;
    h = size;
  }

  return { x, y, w, h };
}

export function buildRegularPolygonPoints(
  bounds: ShapeBounds,
  sides: number,
  starMode = false,
  innerRadiusPct = 0.5,
) {
  if (sides < 3 || bounds.w === 0 || bounds.h === 0) {
    return [];
  }

  const cx = bounds.x + bounds.w * 0.5;
  const cy = bounds.y + bounds.h * 0.5;
  const rx = bounds.w * 0.5;
  const ry = bounds.h * 0.5;
  const totalPoints = starMode ? sides * 2 : sides;
  const clampedInnerRadius = Math.min(1, Math.max(0, innerRadiusPct || 0.5));

  return Array.from({ length: totalPoints }, (_, index) => {
    const radiusScale = starMode && index % 2 === 1 ? clampedInnerRadius : 1;
    const angle = -Math.PI / 2 + (index * 2 * Math.PI) / totalPoints;
    return {
      x: cx + rx * radiusScale * Math.cos(angle),
      y: cy + ry * radiusScale * Math.sin(angle),
    };
  });
}

export function mapShapePresetToBounds(preset: ShapePreset, bounds: ShapeBounds) {
  const mapX = (value: number) => bounds.x + value * bounds.w;
  const mapY = (value: number) => bounds.y + value * bounds.h;

  return preset.points.map((presetPoint) => {
    const normalized = normalizePoint(presetPoint);
    return {
      x: mapX(normalized.x),
      y: mapY(normalized.y),
      inX: mapX(normalized.inX),
      inY: mapY(normalized.inY),
      outX: mapX(normalized.outX),
      outY: mapY(normalized.outY),
      handleType: presetPoint.handleType ?? 0,
    } satisfies PathPointCommand;
  });
}

export function mapShapePresetSubpathsToBounds(preset: ShapePreset, bounds: ShapeBounds) {
  const mapX = (value: number) => bounds.x + value * bounds.w;
  const mapY = (value: number) => bounds.y + value * bounds.h;

  return getShapePresetSubpaths(preset).map((subpath) => ({
    closed: subpath.closed,
    points: subpath.points.map((presetPoint) => {
      const normalized = normalizePoint(presetPoint);
      return {
        x: mapX(normalized.x),
        y: mapY(normalized.y),
        inX: mapX(normalized.inX),
        inY: mapY(normalized.inY),
        outX: mapX(normalized.outX),
        outY: mapY(normalized.outY),
        handleType: presetPoint.handleType ?? 0,
      } satisfies PathPointCommand;
    }),
  }));
}

export function pathPointsToSvgPathData(points: PathPointCommand[], closed: boolean) {
  if (points.length === 0) {
    return "";
  }

  const first = normalizePoint(points[0]);
  const parts = [`M ${first.x} ${first.y}`];

  for (let index = 1; index < points.length; index += 1) {
    const prev = points[index - 1];
    const next = points[index];
    const prevPoint = normalizePoint(prev);
    const nextPoint = normalizePoint(next);
    if (hasCurvedSegment(prev, next)) {
      parts.push(
        `C ${prevPoint.outX} ${prevPoint.outY}, ${nextPoint.inX} ${nextPoint.inY}, ${nextPoint.x} ${nextPoint.y}`,
      );
    } else {
      parts.push(`L ${nextPoint.x} ${nextPoint.y}`);
    }
  }

  if (closed && points.length > 1) {
    const last = points[points.length - 1];
    if (hasCurvedSegment(last, points[0])) {
      const lastPoint = normalizePoint(last);
      parts.push(
        `C ${lastPoint.outX} ${lastPoint.outY}, ${first.inX} ${first.inY}, ${first.x} ${first.y}`,
      );
    }
    parts.push("Z");
  }

  return parts.join(" ");
}

export function shapeSubpathsToSvgPathData(subpaths: ShapeSubpathCommand[]) {
  return subpaths
    .map((subpath) => pathPointsToSvgPathData(subpath.points, subpath.closed))
    .filter((segment) => segment.length > 0)
    .join(" ");
}

export function shapePresetToSvgPathData(preset: ShapePreset) {
  return shapeSubpathsToSvgPathData(getShapePresetSubpaths(preset));
}
