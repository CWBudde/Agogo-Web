import { fireEvent, render } from "@testing-library/react";
import { useState } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { CurvesCanvas } from "@/components/adjustments-panel";

type Point = { x: number; y: number };

const SIZE = 128;

// CurvesCanvas maps clientX/clientY into 0-255 curve space via getBoundingClientRect.
const CANVAS_RECT = {
  x: 0,
  y: 0,
  left: 0,
  top: 0,
  right: SIZE,
  bottom: SIZE,
  width: SIZE,
  height: SIZE,
  toJSON: () => ({}),
} as DOMRect;

const toClientX = (x: number) => (x / 255) * SIZE;
const toClientY = (y: number) => SIZE - (y / 255) * SIZE;

beforeAll(() => {
  // jsdom has no 2D canvas context; the component guards against a null context.
  HTMLCanvasElement.prototype.getContext = vi.fn(
    () => null,
  ) as unknown as HTMLCanvasElement["getContext"];
});

beforeEach(() => {
  // Re-applied per test because the config uses restoreMocks: true.
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue(CANVAS_RECT);
});

/** Controlled harness mirroring how CurvesEditor feeds emitted points back in. */
function Harness({ initial, onEmit }: { initial: Point[]; onEmit: (pts: Point[]) => void }) {
  const [points, setPoints] = useState(initial);
  return (
    <CurvesCanvas
      points={points}
      onChange={(pts) => {
        onEmit(pts);
        setPoints(pts);
      }}
    />
  );
}

function isStrictlyAscendingByX(points: Point[]) {
  return points.every((pt, i) => i === 0 || points[i - 1].x < pt.x);
}

describe("CurvesCanvas dragging", () => {
  it("clamps a dragged point so it cannot cross its neighbors", () => {
    const onEmit = vi.fn();
    const { container } = render(
      <Harness
        initial={[
          { x: 0, y: 0 },
          { x: 128, y: 128 },
          { x: 255, y: 255 },
        ]}
        onEmit={onEmit}
      />,
    );
    const canvas = container.querySelector("canvas") as HTMLCanvasElement;

    // Grab the middle point (128, 128).
    fireEvent.mouseDown(canvas, { clientX: toClientX(128), clientY: toClientY(128) });
    expect(onEmit).not.toHaveBeenCalled();

    // Drag far past the right neighbor at x=255.
    fireEvent.mouseMove(canvas, { clientX: toClientX(255), clientY: toClientY(128) });
    expect(onEmit).toHaveBeenCalledTimes(1);
    const first = onEmit.mock.calls[0][0] as Point[];
    expect(first).toHaveLength(3);
    expect(isStrictlyAscendingByX(first)).toBe(true);
    expect(first[1].x).toBeLessThanOrEqual(254);
    expect(first[0]).toEqual({ x: 0, y: 0 });
    expect(first[2]).toEqual({ x: 255, y: 255 });

    // A second move must still address the middle point.
    fireEvent.mouseMove(canvas, { clientX: toClientX(255), clientY: toClientY(100) });
    expect(onEmit).toHaveBeenCalledTimes(2);
    const second = onEmit.mock.calls[1][0] as Point[];
    expect(second).toHaveLength(3);
    expect(isStrictlyAscendingByX(second)).toBe(true);
    expect(second[1].x).toBeLessThanOrEqual(254);
    expect(second[1].y).toBe(100);
    expect(second[0]).toEqual({ x: 0, y: 0 });
    expect(second[2]).toEqual({ x: 255, y: 255 });
  });

  it("does not corrupt other points when a drag would reorder the array", () => {
    const onEmit = vi.fn();
    const { container } = render(
      <Harness
        initial={[
          { x: 0, y: 0 },
          { x: 64, y: 64 },
          { x: 128, y: 128 },
          { x: 255, y: 255 },
        ]}
        onEmit={onEmit}
      />,
    );
    const canvas = container.querySelector("canvas") as HTMLCanvasElement;

    // Grab the point at (64, 64), then drag it past the neighbor at x=128.
    fireEvent.mouseDown(canvas, { clientX: toClientX(64), clientY: toClientY(64) });
    fireEvent.mouseMove(canvas, { clientX: toClientX(200), clientY: toClientY(64) });
    fireEvent.mouseMove(canvas, { clientX: toClientX(220), clientY: toClientY(80) });

    expect(onEmit).toHaveBeenCalledTimes(2);
    for (const call of onEmit.mock.calls) {
      const pts = call[0] as Point[];
      expect(pts).toHaveLength(4);
      expect(isStrictlyAscendingByX(pts)).toBe(true);
      // The neighbor at (128, 128) must never be overwritten by the drag.
      expect(pts.some((pt) => pt.x === 128 && pt.y === 128)).toBe(true);
      // The dragged point stays on its own side of the neighbor.
      expect(pts[1].x).toBeLessThanOrEqual(127);
      expect(pts[1].x).toBeGreaterThanOrEqual(1);
    }
    const last = onEmit.mock.calls[1][0] as Point[];
    expect(last[1].y).toBe(80);
  });

  it("adds a new point on mousedown in empty space, keeping the array sorted", () => {
    const onEmit = vi.fn();
    const { container } = render(
      <Harness
        initial={[
          { x: 0, y: 0 },
          { x: 255, y: 255 },
        ]}
        onEmit={onEmit}
      />,
    );
    const canvas = container.querySelector("canvas") as HTMLCanvasElement;

    fireEvent.mouseDown(canvas, { clientX: toClientX(128), clientY: toClientY(60) });
    expect(onEmit).toHaveBeenCalledTimes(1);
    const pts = onEmit.mock.calls[0][0] as Point[];
    expect(pts).toHaveLength(3);
    expect(isStrictlyAscendingByX(pts)).toBe(true);
    expect(pts[1]).toEqual({ x: 128, y: 60 });
  });
});
