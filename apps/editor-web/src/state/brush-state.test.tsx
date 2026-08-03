import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { BRUSH_PRESETS } from "@/components/brush-color-panels";
import { CUSTOM_BRUSH_PRESETS_KEY } from "@/lib/persisted-ui";
import { BrushStateProvider, useBrushState } from "@/state/brush-state";

function wrapper({ children }: PropsWithChildren) {
  return <BrushStateProvider>{children}</BrushStateProvider>;
}

describe("BrushStateProvider", () => {
  beforeEach(() => {
    localStorage.removeItem(CUSTOM_BRUSH_PRESETS_KEY);
  });

  it("throws when used outside the provider", () => {
    expect(() => renderHook(() => useBrushState())).toThrow(
      "useBrushState must be used inside <BrushStateProvider>.",
    );
  });

  it("applyBrushPreset applies all tip parameters at once", () => {
    const preset = { ...BRUSH_PRESETS[1], size: 48 };
    const { result } = renderHook(() => useBrushState(), { wrapper });

    act(() => {
      result.current.applyBrushPreset(preset);
    });
    expect(result.current.brushPresetId).toBe(preset.id);
    expect(result.current.brushSize).toBe(48);
    expect(result.current.brushTipShape).toBe(preset.tipShape);
    expect(result.current.brushHardness).toBe(preset.hardness);
    expect(result.current.brushSpacing).toBe(preset.spacing);
    expect(result.current.brushAngle).toBe(preset.angle);
    expect(result.current.brushRoundness).toBe(preset.roundness ?? 1);
    expect(result.current.brushSizeJitter).toBe(preset.sizeJitter ?? 0);
    expect(result.current.brushOpacityJitter).toBe(preset.opacityJitter ?? 0);
    expect(result.current.brushFlowJitter).toBe(preset.flowJitter ?? 0);
    expect(result.current.brushControlSource).toBe(preset.controlSource ?? "pressure");
    expect(result.current.brushFadeDabs).toBe(preset.fadeDabs ?? 100);
  });

  it("falls back to the first preset when the selected custom preset disappears", () => {
    const custom = {
      ...BRUSH_PRESETS[0],
      id: "custom/test-preset",
      name: "Test Preset",
    };
    const { result } = renderHook(() => useBrushState(), { wrapper });

    act(() => {
      result.current.setCustomBrushPresets([custom]);
    });
    act(() => {
      result.current.applyBrushPreset(custom);
    });
    expect(result.current.brushPresetId).toBe(custom.id);

    act(() => {
      result.current.setCustomBrushPresets([]);
    });
    expect(result.current.brushPresetId).toBe(BRUSH_PRESETS[0].id);
  });
});
