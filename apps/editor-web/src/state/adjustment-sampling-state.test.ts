import { CommandID } from "@agogo/proto";
import { act, render } from "@testing-library/react";
import { createElement, type PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  type AdjustmentSamplingRequest,
  AdjustmentSamplingStateProvider,
  type AdjustmentSamplingStateValue,
  adjustmentSamplingReducer,
  isEligibleAdjustmentCanvasSample,
  useAdjustmentSamplingState,
} from "./adjustment-sampling-state";

const mocks = vi.hoisted(() => ({
  activeLayerId: "curves-1" as string | null,
  activeTool: "brush",
  dispatchCommand: vi.fn(),
}));

vi.mock("@/wasm/context", () => ({
  useEngine: () => ({ dispatchCommand: mocks.dispatchCommand }),
}));

vi.mock("@/state/tool-state", () => ({
  useToolState: () => ({ activeTool: mocks.activeTool }),
}));

vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (selector: (meta: { activeLayerId: string | null }) => unknown) =>
    selector({ activeLayerId: mocks.activeLayerId }),
}));

const curvesRequest: AdjustmentSamplingRequest = {
  type: "curves",
  targetLayerId: "curves-1",
  kind: "add-point",
  channel: "red",
  sampleSize: 3,
};

describe("adjustment sampling state", () => {
  beforeEach(() => {
    mocks.activeLayerId = "curves-1";
    mocks.activeTool = "brush";
    mocks.dispatchCommand.mockReset();
  });

  it("starts and cancels a one-shot sampler", () => {
    const active = adjustmentSamplingReducer(null, {
      type: "start",
      request: curvesRequest,
      originTool: "brush",
    });
    expect(active).toEqual({ request: curvesRequest, originTool: "brush" });
    expect(adjustmentSamplingReducer(active, { type: "cancel" })).toBeNull();
  });

  it("replaces an active sampler atomically", () => {
    const active = adjustmentSamplingReducer(null, {
      type: "start",
      request: curvesRequest,
      originTool: "brush",
    });
    const hueRequest: AdjustmentSamplingRequest = {
      type: "hue-range",
      targetLayerId: "hue-1",
      sampleSize: 5,
      sampleMerged: true,
      onIdentifiedRange: vi.fn(),
    };
    const replaced = adjustmentSamplingReducer(active, {
      type: "start",
      request: hueRequest,
      originTool: "marquee",
    });
    expect(replaced?.request).toBe(hueRequest);
    expect(replaced?.originTool).toBe("marquee");
  });

  it("rejects canvas gestures that belong to pan, modifiers, or another button", () => {
    expect(isEligibleAdjustmentCanvasSample({ x: 4, y: 5 })).toBe(true);
    expect(isEligibleAdjustmentCanvasSample({ x: 4, y: 5, isPanMode: true })).toBe(false);
    expect(isEligibleAdjustmentCanvasSample({ x: 4, y: 5, shiftKey: true })).toBe(false);
    expect(isEligibleAdjustmentCanvasSample({ x: 4, y: 5, button: 2 })).toBe(false);
  });

  it("dispatches the selected Curves payload and clears the cursor after one click", () => {
    let samplingState!: AdjustmentSamplingStateValue;
    function Probe() {
      samplingState = useAdjustmentSamplingState();
      return null;
    }
    function Wrapper({ children }: PropsWithChildren) {
      return createElement(AdjustmentSamplingStateProvider, null, children);
    }
    render(createElement(Probe), { wrapper: Wrapper });

    act(() => samplingState.startSampling(curvesRequest));
    expect(samplingState.cursor).toBe("crosshair");
    act(() => {
      expect(samplingState.consumeCanvasSample({ x: 12, y: 34 })).toBe(true);
    });
    expect(mocks.dispatchCommand).toHaveBeenCalledWith(CommandID.SetPointFromSample, {
      layerId: "curves-1",
      x: 12,
      y: 34,
      sampleSize: 3,
      kind: "add-point",
      channel: "red",
    });
    expect(samplingState.cursor).toBeNull();
  });

  it("delivers identified hue ranges and Escape cancels without dispatching", () => {
    let samplingState!: AdjustmentSamplingStateValue;
    function Probe() {
      samplingState = useAdjustmentSamplingState();
      return null;
    }
    mocks.activeLayerId = "hue-1";
    mocks.dispatchCommand.mockReturnValue({ identifiedHueRange: "greens" });
    render(createElement(AdjustmentSamplingStateProvider, null, createElement(Probe)));
    const onIdentifiedRange = vi.fn();
    const request: AdjustmentSamplingRequest = {
      type: "hue-range",
      targetLayerId: "hue-1",
      sampleSize: 5,
      sampleMerged: true,
      onIdentifiedRange,
    };

    act(() => samplingState.startSampling(request));
    act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })));
    expect(samplingState.sampling).toBeNull();
    expect(mocks.dispatchCommand).not.toHaveBeenCalled();

    act(() => samplingState.startSampling(request));
    act(() => {
      samplingState.consumeCanvasSample({ x: 7, y: 9 });
    });
    expect(mocks.dispatchCommand).toHaveBeenCalledWith(CommandID.IdentifyHueRange, {
      x: 7,
      y: 9,
      sampleSize: 5,
      sampleMerged: true,
    });
    expect(onIdentifiedRange).toHaveBeenCalledWith("greens");
  });

  it("cancels when the owning layer or tool changes", () => {
    let samplingState!: AdjustmentSamplingStateValue;
    function Probe() {
      samplingState = useAdjustmentSamplingState();
      return null;
    }
    const tree = () => createElement(AdjustmentSamplingStateProvider, null, createElement(Probe));
    const view = render(tree());

    act(() => samplingState.startSampling(curvesRequest));
    mocks.activeTool = "eraser";
    view.rerender(tree());
    expect(samplingState.sampling).toBeNull();

    act(() => samplingState.startSampling(curvesRequest));
    mocks.activeLayerId = "other-layer";
    view.rerender(tree());
    expect(samplingState.sampling).toBeNull();
  });
});
