import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { BlendIfSlider, type BlendIfSliderValue } from "@/components/ui/blend-if-slider";

function mockTrackRect(width = 256, left = 0) {
  const track = screen.getByTestId("blend-if-slider-track");
  const rect = {
    x: left,
    y: 0,
    left,
    top: 0,
    right: left + width,
    bottom: 24,
    width,
    height: 24,
    toJSON: () => ({}),
  } as DOMRect;
  vi.spyOn(track, "getBoundingClientRect").mockReturnValue(rect);

  // jsdom does not implement setPointerCapture / releasePointerCapture.
  for (const el of screen.queryAllByTestId(/^blend-if-handle/u)) {
    (el as HTMLElement & { setPointerCapture?: unknown }).setPointerCapture = () => undefined;
    (el as HTMLElement & { releasePointerCapture?: unknown }).releasePointerCapture = () =>
      undefined;
  }

  return { track, width, left };
}

describe("BlendIfSlider", () => {
  it("renders with default-identity values without errors", () => {
    render(
      <BlendIfSlider value={[0, 0, 255, 255]} onChange={() => undefined} label="Gray" />,
    );
    expect(screen.getByTestId("blend-if-slider-track")).toBeTruthy();
    expect(screen.getByTestId("blend-if-handle-low")).toBeTruthy();
    expect(screen.getByTestId("blend-if-handle-high")).toBeTruthy();
  });

  it("exposes label text and aria attributes", () => {
    render(
      <BlendIfSlider value={[10, 20, 200, 230]} onChange={() => undefined} label="Red" />,
    );
    expect(screen.getByText("Red")).toBeTruthy();
    const track = screen.getByTestId("blend-if-slider-track");
    expect(track.getAttribute("aria-valuemin")).toBe("0");
    expect(track.getAttribute("aria-valuemax")).toBe("255");
    expect(track.getAttribute("aria-valuenow")).toBe("20");
    expect(track.getAttribute("aria-label")).toBe("Red");
  });

  it("renders split halves when low or high handles are split", () => {
    render(
      <BlendIfSlider value={[0, 64, 192, 255]} onChange={() => undefined} />,
    );

    // Low is split (0 !== 64) → two halves.
    expect(screen.getByTestId("blend-if-handle-low-hard")).toBeTruthy();
    expect(screen.getByTestId("blend-if-handle-low-soft")).toBeTruthy();
    // High is split (192 !== 255) → two halves.
    expect(screen.getByTestId("blend-if-handle-high-hard")).toBeTruthy();
    expect(screen.getByTestId("blend-if-handle-high-soft")).toBeTruthy();
    // The unified handles should not exist when split.
    expect(screen.queryByTestId("blend-if-handle-low")).toBeNull();
    expect(screen.queryByTestId("blend-if-handle-high")).toBeNull();
  });

  it("invokes onChange with clamped values during a drag on the low handle", () => {
    const onChange = vi.fn();

    function Harness() {
      const [value, setValue] = useState<BlendIfSliderValue>([0, 0, 255, 255]);
      return (
        <BlendIfSlider
          value={value}
          onChange={(next) => {
            onChange(next);
            setValue(next);
          }}
        />
      );
    }

    render(<Harness />);
    mockTrackRect(256, 0);

    const handle = screen.getByTestId("blend-if-handle-low");
    fireEvent.pointerDown(handle, { button: 0, pointerId: 1, clientX: 0 });
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 64 });
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 64 });

    expect(onChange).toHaveBeenCalled();
    const last = onChange.mock.calls.at(-1)?.[0] as BlendIfSliderValue;
    // All four values are integers, ordered, in range.
    for (const v of last) {
      expect(Number.isInteger(v)).toBe(true);
      expect(v).toBeGreaterThanOrEqual(0);
      expect(v).toBeLessThanOrEqual(255);
    }
    expect(last[0]).toBeLessThanOrEqual(last[1]);
    expect(last[1]).toBeLessThanOrEqual(last[2]);
    expect(last[2]).toBeLessThanOrEqual(last[3]);
    // Moving the low handle to ~25% of the track should drag both halves
    // together because we did not hold Alt.
    expect(last[0]).toBeGreaterThan(0);
    expect(last[0]).toBe(last[1]);
  });

  it("splits the low handle when Alt is held during drag", () => {
    const onChange = vi.fn();

    function Harness() {
      const [value, setValue] = useState<BlendIfSliderValue>([0, 0, 255, 255]);
      return (
        <BlendIfSlider
          value={value}
          onChange={(next) => {
            onChange(next);
            setValue(next);
          }}
        />
      );
    }

    render(<Harness />);
    mockTrackRect(256, 0);

    const handle = screen.getByTestId("blend-if-handle-low");
    fireEvent.pointerDown(handle, {
      button: 0,
      pointerId: 1,
      clientX: 0,
      altKey: true,
    });
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 80, altKey: true });
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 80, altKey: true });

    const last = onChange.mock.calls.at(-1)?.[0] as BlendIfSliderValue;
    // The hard edge stays at 0; the soft edge moved right, splitting the handle.
    expect(last[0]).toBe(0);
    expect(last[1]).toBeGreaterThan(0);
    expect(last[1]).toBeLessThanOrEqual(255);
  });
});
