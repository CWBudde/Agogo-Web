import { describe, expect, it } from "vitest";
import {
  buildRegularPolygonPoints,
  mapShapePresetToBounds,
  pathPointsToSvgPathData,
  resolveShapeDragBounds,
  type ShapePreset,
} from "@/lib/shape-presets";

describe("shape preset helpers", () => {
  const preset: ShapePreset = {
    id: "test-shape",
    name: "Test Shape",
    category: "logos",
    closed: true,
    points: [
      { x: 0.1, y: 0.2, outX: 0.3, outY: 0.4, handleType: 1 },
      { x: 0.8, y: 0.9, inX: 0.7, inY: 0.6, handleType: 1 },
    ],
  };

  it("maps normalized points and handles into target bounds", () => {
    const mapped = mapShapePresetToBounds(preset, { x: 10, y: 20, w: 100, h: 50 });

    expect(mapped).toEqual([
      {
        x: 20,
        y: 30,
        inX: 20,
        inY: 30,
        outX: 40,
        outY: 40,
        handleType: 1,
      },
      {
        x: 90,
        y: 65,
        inX: 80,
        inY: 50,
        outX: 90,
        outY: 65,
        handleType: 1,
      },
    ]);
  });

  it("resolves constrained drag bounds for forward and reverse drags", () => {
    expect(resolveShapeDragBounds({ x: 10, y: 20 }, { x: 30, y: 45 }, true)).toEqual({
      x: 10,
      y: 20,
      w: 25,
      h: 25,
    });

    expect(resolveShapeDragBounds({ x: 30, y: 40 }, { x: 10, y: 25 }, true)).toEqual({
      x: 10,
      y: 20,
      w: 20,
      h: 20,
    });
  });

  it("emits a closed SVG path when the preset is closed", () => {
    const mapped = mapShapePresetToBounds(preset, { x: 0, y: 0, w: 1, h: 1 });
    expect(pathPointsToSvgPathData(mapped, preset.closed)).toContain("Z");
  });

  it("builds star polygon points with the requested inner radius percentage", () => {
    const points = buildRegularPolygonPoints({ x: 0, y: 0, w: 100, h: 100 }, 4, true, 0.25);

    expect(points).toHaveLength(8);
    expect(points[0]).toEqual({ x: 50, y: 0 });
    expect(points[1].x).toBeCloseTo(58.8388347648);
    expect(points[1].y).toBeCloseTo(41.1611652352);
  });
});
