import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { CUSTOM_SHAPE_PRESETS_KEY } from "@/lib/persisted-ui";
import { SHAPE_PRESETS } from "@/lib/shape-presets";
import { ShapeStateProvider, useShapeState } from "@/state/shape-state";

function wrapper({ children }: PropsWithChildren) {
  return <ShapeStateProvider>{children}</ShapeStateProvider>;
}

describe("ShapeStateProvider", () => {
  beforeEach(() => {
    localStorage.removeItem(CUSTOM_SHAPE_PRESETS_KEY);
  });

  it("throws when used outside the provider", () => {
    expect(() => renderHook(() => useShapeState())).toThrow(
      "useShapeState must be used inside <ShapeStateProvider>.",
    );
  });

  it("selectedShapePreset tracks the id and falls back when a custom preset disappears", () => {
    const custom = {
      ...SHAPE_PRESETS[0],
      id: "custom/test-shape",
      name: "Test Shape",
    };
    const { result } = renderHook(() => useShapeState(), { wrapper });

    act(() => {
      result.current.setCustomShapePresets([custom]);
    });
    act(() => {
      result.current.setShapePresetId(custom.id);
    });
    expect(result.current.selectedShapePreset?.id).toBe(custom.id);

    act(() => {
      result.current.setCustomShapePresets([]);
    });
    expect(result.current.shapePresetId).toBe(SHAPE_PRESETS[0]?.id ?? "");
    expect(result.current.selectedShapePreset?.id).toBe(SHAPE_PRESETS[0]?.id);
  });
});
