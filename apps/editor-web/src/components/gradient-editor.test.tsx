import type { GradientStopCommand } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { GradientEditorDialog } from "@/components/gradient-editor";

const TRACK_RECT = {
  x: 0,
  y: 0,
  left: 0,
  top: 0,
  right: 100,
  bottom: 16,
  width: 100,
  height: 16,
  toJSON: () => ({}),
} as DOMRect;

beforeAll(() => {
  // jsdom does not implement pointer capture.
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.hasPointerCapture = vi.fn(() => false);
});

beforeEach(() => {
  // Fixed track geometry: 100px wide starting at x=0, so clientX maps 1:1 to percent.
  // Re-applied per test because the config uses restoreMocks: true.
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue(TRACK_RECT);
});

function dialogProps(
  stops: GradientStopCommand[],
  onStopsChange: (s: GradientStopCommand[]) => void,
) {
  return {
    open: true,
    stops,
    onStopsChange,
    recentColors: [],
    onRecentColorSelect: vi.fn(),
    channelMode: "rgb" as const,
    onChannelModeChange: vi.fn(),
    onlyWebColors: false,
    onOnlyWebColorsChange: vi.fn(),
    onClose: vi.fn(),
  };
}

const INITIAL_STOPS: GradientStopCommand[] = [
  { position: 0, color: [0, 0, 0, 255] },
  { position: 1, color: [255, 255, 255, 255] },
];

describe("GradientEditorDialog stop dragging", () => {
  it("keeps a drag alive across consecutive moves when the parent echoes stops back", () => {
    const onStopsChange = vi.fn();
    const view = render(<GradientEditorDialog {...dialogProps(INITIAL_STOPS, onStopsChange)} />);

    // Grab the handle for the black stop at position 0.
    const handle = screen.getByTitle("0% #000000");
    const track = handle.parentElement as HTMLElement;
    expect(track).toBeTruthy();

    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 0, clientY: 8 });

    // First move: drag to 30%.
    fireEvent.pointerMove(track, { pointerId: 1, clientX: 30, clientY: 8 });
    expect(onStopsChange).toHaveBeenCalledTimes(1);
    const firstEmit = onStopsChange.mock.calls[0][0] as GradientStopCommand[];
    expect(firstEmit).toHaveLength(2);
    expect(firstEmit[0].position).toBeCloseTo(0.3, 5);
    expect(firstEmit[1].position).toBeCloseTo(1, 5);

    // Parent echoes the emitted stops back as the `stops` prop (as the real app does).
    view.rerender(<GradientEditorDialog {...dialogProps(firstEmit, onStopsChange)} />);

    // Second move: the drag must still be tracking the same stop.
    fireEvent.pointerMove(track, { pointerId: 1, clientX: 60, clientY: 8 });
    const lastCall = onStopsChange.mock.calls.at(-1);
    expect(lastCall).toBeTruthy();
    const secondEmit = (lastCall as unknown[][])[0] as GradientStopCommand[];
    expect(secondEmit).toHaveLength(2);
    expect(secondEmit[0].position).toBeCloseTo(0.6, 5);
    expect(secondEmit.some((stop) => Math.abs(stop.position - 0.3) < 1e-5)).toBe(false);
  });

  it("re-initializes editor stops from props when the dialog transitions closed to open", () => {
    const onStopsChange = vi.fn();
    const props = dialogProps(INITIAL_STOPS, onStopsChange);
    const view = render(<GradientEditorDialog {...props} open={false} />);
    expect(screen.queryByTitle("0% #000000")).toBeNull();

    const nextStops: GradientStopCommand[] = [
      { position: 0.25, color: [255, 0, 0, 255] },
      { position: 1, color: [0, 0, 255, 255] },
    ];
    view.rerender(<GradientEditorDialog {...dialogProps(nextStops, onStopsChange)} open />);

    expect(screen.getByTitle("25% #ff0000")).toBeTruthy();
    expect(screen.getByTitle("100% #0000ff")).toBeTruthy();
  });
});
