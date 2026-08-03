import {
  CommandID,
  type CurvesSampleKind,
  type HistogramChannel,
  type IdentifyHueRangeCommand,
  type SetPointFromSampleCommand,
} from "@agogo/proto";
import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
} from "react";
import type { EditorTool } from "@/components/tool-rail-model";
import { useToolState } from "@/state/tool-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

export type AdjustmentSamplingRequest =
  | {
      type: "curves";
      targetLayerId: string;
      kind: CurvesSampleKind;
      channel: Extract<HistogramChannel, "rgb" | "red" | "green" | "blue">;
      sampleSize: number;
    }
  | {
      type: "hue-range";
      targetLayerId: string;
      sampleSize: number;
      sampleMerged: boolean;
      onIdentifiedRange: (range: string) => void;
    };

export interface AdjustmentCanvasSample {
  x: number;
  y: number;
  button?: number;
  altKey?: boolean;
  ctrlKey?: boolean;
  metaKey?: boolean;
  shiftKey?: boolean;
  isPanMode?: boolean;
}

interface ActiveSampling {
  request: AdjustmentSamplingRequest;
  originTool: EditorTool;
}

type SamplingAction =
  | { type: "start"; request: AdjustmentSamplingRequest; originTool: EditorTool }
  | { type: "cancel" };

export function adjustmentSamplingReducer(
  _state: ActiveSampling | null,
  action: SamplingAction,
): ActiveSampling | null {
  switch (action.type) {
    case "start":
      return { request: action.request, originTool: action.originTool };
    case "cancel":
      return null;
  }
}

export function isEligibleAdjustmentCanvasSample(sample: AdjustmentCanvasSample): boolean {
  return (
    (sample.button ?? 0) === 0 &&
    !sample.altKey &&
    !sample.ctrlKey &&
    !sample.metaKey &&
    !sample.shiftKey &&
    !sample.isPanMode
  );
}

export interface AdjustmentSamplingStateValue {
  sampling: AdjustmentSamplingRequest | null;
  cursor: "crosshair" | null;
  startSampling(request: AdjustmentSamplingRequest): void;
  cancelSampling(): void;
  /**
   * Consumes a one-shot document-space canvas sample. Returns true when the
   * normal canvas gesture should be suppressed, even if the engine rejects
   * the command (the error is surfaced by EngineProvider).
   */
  consumeCanvasSample(sample: AdjustmentCanvasSample): boolean;
}

const AdjustmentSamplingStateContext = createContext<AdjustmentSamplingStateValue | null>(null);

export function AdjustmentSamplingStateProvider({ children }: PropsWithChildren) {
  const engine = useEngine();
  const { activeTool } = useToolState();
  const activeLayerId = useUiMeta((meta) => meta?.activeLayerId ?? null);
  const [active, dispatch] = useReducer(adjustmentSamplingReducer, null);

  const cancelSampling = useCallback(() => dispatch({ type: "cancel" }), []);
  const startSampling = useCallback(
    (request: AdjustmentSamplingRequest) => {
      dispatch({ type: "start", request, originTool: activeTool });
    },
    [activeTool],
  );

  // A sampler belongs to the editor that started it. Switching tools or
  // layers (including deletion of the target layer) invalidates that owner.
  useEffect(() => {
    if (
      active &&
      (active.originTool !== activeTool || active.request.targetLayerId !== activeLayerId)
    ) {
      cancelSampling();
    }
  }, [active, activeLayerId, activeTool, cancelSampling]);

  useEffect(() => {
    if (!active) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      cancelSampling();
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [active, cancelSampling]);

  const consumeCanvasSample = useCallback(
    (sample: AdjustmentCanvasSample) => {
      if (!active || !isEligibleAdjustmentCanvasSample(sample)) return false;

      const { request } = active;
      if (request.type === "curves") {
        const payload: SetPointFromSampleCommand = {
          layerId: request.targetLayerId,
          x: sample.x,
          y: sample.y,
          sampleSize: request.sampleSize,
          kind: request.kind,
          channel: request.channel,
        };
        engine.dispatchCommand(CommandID.SetPointFromSample, payload);
      } else {
        const payload: IdentifyHueRangeCommand = {
          x: sample.x,
          y: sample.y,
          sampleSize: request.sampleSize,
          sampleMerged: request.sampleMerged,
        };
        const result = engine.dispatchCommand(CommandID.IdentifyHueRange, payload);
        if (result?.identifiedHueRange) {
          request.onIdentifiedRange(result.identifiedHueRange);
        }
      }
      cancelSampling();
      return true;
    },
    [active, cancelSampling, engine],
  );

  const value = useMemo<AdjustmentSamplingStateValue>(
    () => ({
      sampling: active?.request ?? null,
      cursor: active ? "crosshair" : null,
      startSampling,
      cancelSampling,
      consumeCanvasSample,
    }),
    [active, cancelSampling, consumeCanvasSample, startSampling],
  );

  return (
    <AdjustmentSamplingStateContext.Provider value={value}>
      {children}
    </AdjustmentSamplingStateContext.Provider>
  );
}

export function useAdjustmentSamplingState(): AdjustmentSamplingStateValue {
  const context = useContext(AdjustmentSamplingStateContext);
  if (!context) {
    throw new Error(
      "useAdjustmentSamplingState must be used inside <AdjustmentSamplingStateProvider>.",
    );
  }
  return context;
}
