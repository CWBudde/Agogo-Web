import type { FreeTransformMeta } from "@agogo/proto";
import { describe, expect, it } from "vitest";
import {
  applyTransformFieldChange,
  buildWarpGrid,
  type FreeTransformCorners,
  refPointToPivot,
} from "./transform-math";

const identityFt = {
  a: 1,
  b: 0,
  c: 0,
  d: 1,
  tx: 10,
  ty: 20,
  scaleX: 1,
  scaleY: 1,
  rotation: 0,
};

describe("applyTransformFieldChange", () => {
  it("updates translation for x and y fields", () => {
    expect(applyTransformFieldChange(identityFt, "x", 42)).toEqual({
      a: 1,
      b: 0,
      c: 0,
      d: 1,
      tx: 42,
      ty: 20,
    });
    expect(applyTransformFieldChange(identityFt, "y", -7).ty).toBe(-7);
  });

  it("scales the first column for a width change", () => {
    const result = applyTransformFieldChange(identityFt, "w", 200);
    expect(result.a).toBeCloseTo(2, 10);
    expect(result.b).toBeCloseTo(0, 10);
    expect(result.d).toBeCloseTo(1, 10);
  });

  it("rotates the matrix by the rotation delta", () => {
    const result = applyTransformFieldChange(identityFt, "r", 90);
    expect(result.a).toBeCloseTo(0, 10);
    expect(result.c).toBeCloseTo(1, 10);
    expect(result.b).toBeCloseTo(-1, 10);
    expect(result.d).toBeCloseTo(0, 10);
  });
});

describe("refPointToPivot", () => {
  const corners: FreeTransformCorners = [
    [0, 0],
    [100, 0],
    [100, 50],
    [0, 50],
  ];

  it("maps grid cells to document-space pivots", () => {
    expect(refPointToPivot(corners, 0, 0)).toEqual([0, 0]);
    expect(refPointToPivot(corners, 1, 1)).toEqual([50, 25]);
    expect(refPointToPivot(corners, 2, 2)).toEqual([100, 50]);
  });
});

describe("buildWarpGrid", () => {
  it("builds a 4x4 grid interpolated between the corners", () => {
    const ft = {
      corners: [
        [0, 0],
        [90, 0],
        [90, 30],
        [0, 30],
      ],
    } as FreeTransformMeta;
    const grid = buildWarpGrid(ft);
    expect(grid).toHaveLength(4);
    expect(grid.every((row) => row.length === 4)).toBe(true);
    expect(grid[0][0]).toEqual([0, 0]);
    expect(grid[3][3]).toEqual([90, 30]);
    expect(grid[1][2][0]).toBeCloseTo(60, 10);
    expect(grid[1][2][1]).toBeCloseTo(10, 10);
  });
});
