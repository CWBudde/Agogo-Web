import {
  CommandID,
  type AddLayerMaskMode,
} from "@agogo/proto";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { EngineContextValue } from "@/wasm/types";

type OutputMode = "selection" | "layer-mask";

export function SelectAndMaskWorkspace({
  open,
  onClose,
  engine,
  activeLayerId,
}: {
  open: boolean;
  onClose: () => void;
  engine: EngineContextValue;
  activeLayerId: string | null;
}) {
  const [smooth, setSmooth] = useState(0);
  const [feather, setFeather] = useState(0);
  const [shiftEdge, setShiftEdge] = useState(0);
  const [output, setOutput] = useState<OutputMode>("selection");

  if (!open) return null;

  const handleApply = () => {
    if (smooth > 0) {
      engine.dispatchCommand(CommandID.SmoothSelection, { radius: smooth });
    }
    if (feather > 0) {
      engine.dispatchCommand(CommandID.FeatherSelection, { radius: feather });
    }
    if (shiftEdge > 0) {
      engine.dispatchCommand(CommandID.ExpandSelection, { pixels: shiftEdge });
    } else if (shiftEdge < 0) {
      engine.dispatchCommand(CommandID.ContractSelection, { pixels: -shiftEdge });
    }
    if (output === "layer-mask" && activeLayerId) {
      engine.dispatchCommand(CommandID.AddLayerMask, {
        layerId: activeLayerId,
        mode: "from-selection" as AddLayerMaskMode,
      });
    }
    onClose();
  };

  const handleCancel = () => {
    setSmooth(0);
    setFeather(0);
    setShiftEdge(0);
    setOutput("selection");
    onClose();
  };

  return (
    <div className="pointer-events-none fixed inset-0 z-50 flex">
      {/* Left panel */}
      <div className="editor-chrome pointer-events-auto flex w-64 shrink-0 flex-col gap-4 border-r border-border p-4">
        <h2 className="text-sm font-semibold text-slate-100">Select and Mask</h2>

        <div className="space-y-5">
          <SliderField
            label={`Smooth: ${smooth}`}
            min={0}
            max={100}
            step={1}
            value={smooth}
            onChange={setSmooth}
          />
          <SliderField
            label={`Feather: ${feather.toFixed(1)} px`}
            min={0}
            max={250}
            step={0.5}
            value={feather}
            onChange={setFeather}
          />
          <SliderField
            label={`Shift Edge: ${shiftEdge > 0 ? "+" : ""}${shiftEdge} px`}
            min={-100}
            max={100}
            step={1}
            value={shiftEdge}
            onChange={setShiftEdge}
          />
        </div>

        <div className="mt-auto space-y-3">
          <label className="flex flex-col gap-1.5">
            <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
              Output To
            </span>
            <select
              className="h-[var(--ui-h-md)] w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2.5 text-[13px] text-slate-100 outline-none transition focus:border-cyan-400/40"
              value={output}
              onChange={(e) => setOutput(e.target.value as OutputMode)}
            >
              <option value="selection">Selection</option>
              <option value="layer-mask">Layer Mask</option>
            </select>
          </label>

          <div className="flex gap-2">
            <Button variant="secondary" size="sm" className="flex-1" onClick={handleCancel}>
              Cancel
            </Button>
            <Button size="sm" className="flex-1" onClick={handleApply}>
              OK
            </Button>
          </div>
        </div>
      </div>

      {/* Right side is transparent — editor shows through */}
      <div className="flex-1" />
    </div>
  );
}

function SliderField({
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className="space-y-1.5">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{label}</span>
      <input
        type="range"
        className="w-full accent-cyan-400"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </div>
  );
}
