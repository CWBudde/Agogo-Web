import type { FreeTransformMeta } from "@agogo/proto";

export type FreeTransformCorners = [
  [number, number],
  [number, number],
  [number, number],
  [number, number],
];

/** Recompute affine transform after editing one display field in the options bar. */
export function applyTransformFieldChange(
  ft: {
    a: number;
    b: number;
    c: number;
    d: number;
    tx: number;
    ty: number;
    scaleX: number;
    scaleY: number;
    rotation: number;
  },
  field: "x" | "y" | "w" | "h" | "r",
  value: number,
): { a: number; b: number; c: number; d: number; tx: number; ty: number } {
  let { a, b, c, d, tx, ty, scaleX, scaleY, rotation } = ft;
  switch (field) {
    case "x":
      tx = value;
      break;
    case "y":
      ty = value;
      break;
    case "w": {
      const newScaleX = value / 100;
      const factor = scaleX !== 0 ? newScaleX / scaleX : 1;
      a *= factor;
      b *= factor;
      break;
    }
    case "h": {
      const newScaleY = value / 100;
      const factor = scaleY !== 0 ? newScaleY / scaleY : 1;
      c *= factor;
      d *= factor;
      break;
    }
    case "r": {
      const deltaRad = ((value - rotation) * Math.PI) / 180;
      const cos = Math.cos(deltaRad);
      const sin = Math.sin(deltaRad);
      const newA = a * cos - c * sin;
      const newC = a * sin + c * cos;
      const newB = b * cos - d * sin;
      const newD = b * sin + d * cos;
      a = newA;
      b = newB;
      c = newC;
      d = newD;
      break;
    }
  }
  return { a, b, c, d, tx, ty };
}

/** Build a 4×4 warp control-point grid by bilinear interpolation of the transform corners. */
export function buildWarpGrid(
  ft: FreeTransformMeta,
): [[number, number], [number, number], [number, number], [number, number]][] {
  const [tl, tr, br, bl] = ft.corners;
  const grid: [[number, number], [number, number], [number, number], [number, number]][] = [];
  for (let row = 0; row < 4; row++) {
    const t = row / 3;
    const rowArr: [number, number][] = [];
    for (let col = 0; col < 4; col++) {
      const s = col / 3;
      const x = (1 - t) * ((1 - s) * tl[0] + s * tr[0]) + t * ((1 - s) * bl[0] + s * br[0]);
      const y = (1 - t) * ((1 - s) * tl[1] + s * tr[1]) + t * ((1 - s) * bl[1] + s * br[1]);
      rowArr.push([x, y]);
    }
    grid.push(rowArr as [[number, number], [number, number], [number, number], [number, number]]);
  }
  return grid;
}

/** Compute the pivot doc-space position for a given 3×3 grid cell.
 *  corners: [TL, TR, BR, BL] in document space (from FreeTransformMeta). */
export function refPointToPivot(
  corners: FreeTransformCorners,
  row: number,
  col: number,
): [number, number] {
  const t = col / 2; // 0 = left, 0.5 = centre, 1 = right
  const s = row / 2; // 0 = top,  0.5 = middle, 1 = bottom
  const [tl, tr, br, bl] = corners;
  const topX = tl[0] + t * (tr[0] - tl[0]);
  const topY = tl[1] + t * (tr[1] - tl[1]);
  const botX = bl[0] + t * (br[0] - bl[0]);
  const botY = bl[1] + t * (br[1] - bl[1]);
  return [topX + s * (botX - topX), topY + s * (botY - topY)];
}
