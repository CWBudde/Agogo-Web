import { CommandID, type LayerNodeMeta } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AdjPropertiesPanel } from "@/components/adjustments-panel";
import { ChannelsPanel } from "@/components/channels-panel";
import type { EngineContextValue } from "@/wasm/types";

vi.mock("@/state/adjustment-sampling-state", () => ({
  useAdjustmentSamplingState: () => ({
    sampling: null,
    startSampling: vi.fn(),
    cancelSampling: vi.fn(),
  }),
}));

function makeEngine() {
  return {
    beginTransaction: vi.fn(),
    endTransaction: vi.fn(),
    dispatchCommand: vi.fn(),
  } as unknown as EngineContextValue;
}

function maskedAdjustmentLayer(): LayerNodeMeta {
  return {
    id: "adjustment-1",
    name: "Invert",
    layerType: "adjustment",
    adjustmentKind: "invert",
    params: {},
    visible: true,
    lockMode: "none",
    opacity: 1,
    fillOpacity: 1,
    blendMode: "normal",
    clipToBelow: false,
    clippingBase: false,
    hasMask: true,
    maskEnabled: true,
    hasVectorMask: false,
  };
}

describe("contextual panel action audit", () => {
  it("shows only real saved channels and exposes no cosmetic channel actions", () => {
    const { rerender } = render(<ChannelsPanel savedSelections={[]} />);
    expect(screen.getByText("No saved selection channels.")).not.toBeNull();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
    expect(screen.queryByText("Composite")).toBeNull();
    expect(screen.queryByText(/channel isolation/i)).toBeNull();

    rerender(<ChannelsPanel savedSelections={[{ name: "Subject", pixelCount: 42 }]} />);
    expect(screen.getByText("Saved Selection Channels")).not.toBeNull();
    expect(screen.getByText("Subject")).not.toBeNull();
    expect(screen.getByText("42px")).not.toBeNull();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("keeps every visible mask action engine-backed and omits deferred controls", () => {
    const engine = makeEngine();
    const layer = maskedAdjustmentLayer();
    render(
      <AdjPropertiesPanel
        engine={engine}
        layers={[layer]}
        activeLayerId={layer.id}
        availableFonts={[]}
        fallback={null}
      />,
    );

    expect(screen.queryByText(/density and feather/i)).toBeNull();
    fireEvent.click(screen.getByTitle("Disable mask"));
    fireEvent.click(screen.getByTitle("Invert mask"));
    fireEvent.click(screen.getByTitle("Delete mask"));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.SetLayerMaskEnabled, {
      layerId: "adjustment-1",
      enabled: false,
    });
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.InvertLayerMask, {
      layerId: "adjustment-1",
    });
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.DeleteLayerMask, {
      layerId: "adjustment-1",
    });
  });
});
