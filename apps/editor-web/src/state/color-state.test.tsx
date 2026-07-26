import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { snapToWebSafeColor } from "@/lib/color";
import {
  CUSTOM_SWATCHES_KEY,
  CUSTOM_SWATCHES_NAME_KEY,
  RECENT_COLORS_KEY,
} from "@/lib/persisted-ui";
import { ColorStateProvider, useColorState } from "@/state/color-state";

vi.mock("@/wasm/context", () => ({
  useEngine: () => ({
    handle: null,
    dispatchCommand: vi.fn(() => null),
  }),
}));

vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (selector: (m: unknown) => unknown) => selector(null),
}));

function wrapper({ children }: PropsWithChildren) {
  return <ColorStateProvider>{children}</ColorStateProvider>;
}

describe("ColorStateProvider", () => {
  beforeEach(() => {
    localStorage.removeItem(RECENT_COLORS_KEY);
    localStorage.removeItem(CUSTOM_SWATCHES_KEY);
    localStorage.removeItem(CUSTOM_SWATCHES_NAME_KEY);
  });

  it("throws when used outside the provider", () => {
    expect(() => renderHook(() => useColorState())).toThrow(
      "useColorState must be used inside <ColorStateProvider>.",
    );
  });

  it("pushRecentColor dedupes, moves to front, and caps at 10", () => {
    const { result } = renderHook(() => useColorState(), { wrapper });

    act(() => {
      for (let i = 0; i < 12; i++) {
        result.current.pushRecentColor([i, 0, 0, 255]);
      }
    });
    expect(result.current.recentColors).toHaveLength(10);
    expect(result.current.recentColors[0]).toEqual([11, 0, 0, 255]);

    // Re-pushing an existing color moves it to the front without duplicating.
    act(() => {
      result.current.pushRecentColor([5, 0, 0, 255]);
    });
    expect(result.current.recentColors).toHaveLength(10);
    expect(result.current.recentColors[0]).toEqual([5, 0, 0, 255]);
    expect(
      result.current.recentColors.filter((c) => c[0] === 5 && c[1] === 0 && c[2] === 0),
    ).toHaveLength(1);
  });

  it("applyColorToTarget routes to the right slot and records a recent color", () => {
    const { result } = renderHook(() => useColorState(), { wrapper });

    act(() => {
      result.current.applyColorToTarget("background", [10, 20, 30, 255]);
    });
    expect(result.current.backgroundColor).toEqual([10, 20, 30, 255]);
    expect(result.current.foregroundColor).toEqual([0, 0, 0, 255]);
    expect(result.current.recentColors[0]).toEqual([10, 20, 30, 255]);
  });

  it("applyColorToTarget snaps to web-safe colors when onlyWebColors is set", () => {
    const { result } = renderHook(() => useColorState(), { wrapper });

    act(() => {
      result.current.setOnlyWebColors(true);
    });
    act(() => {
      result.current.applyColorToTarget("foreground", [10, 20, 30, 255]);
    });
    expect(result.current.foregroundColor).toEqual(snapToWebSafeColor([10, 20, 30, 255]));
  });
});
