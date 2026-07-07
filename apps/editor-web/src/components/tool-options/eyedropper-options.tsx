import { useColorState } from "@/state/color-state";
import { ToolChoiceButton, ToolOptionGroup } from "./controls";

export function EyedropperOptions() {
  const {
    eyedropperSampleSize,
    setEyedropperSampleSize,
    eyedropperSampleMerged,
    setEyedropperSampleMerged,
    eyedropperSampleAllLayersNoAdj,
    setEyedropperSampleAllLayersNoAdj,
  } = useColorState();

  const eyedropperModeSummary = `${eyedropperSampleSize === 1 ? "Point sample" : `${eyedropperSampleSize}x${eyedropperSampleSize} average`} · ${eyedropperSampleMerged ? "sample merged" : "active layer"} · ${eyedropperSampleAllLayersNoAdj ? "no adjustments" : "with adjustments"}`;

  return (
    <>
      <ToolOptionGroup label="Sample">
        {[1, 3, 5, 11, 31, 51, 101].map((size) => (
          <ToolChoiceButton
            key={size}
            active={eyedropperSampleSize === size}
            onClick={() => setEyedropperSampleSize(size)}
          >
            {size === 1 ? "Point" : `${size}x${size}`}
          </ToolChoiceButton>
        ))}
      </ToolOptionGroup>
      <ToolChoiceButton
        active={eyedropperSampleMerged}
        onClick={() => setEyedropperSampleMerged((v) => !v)}
      >
        Sample Merged
      </ToolChoiceButton>
      <ToolChoiceButton
        active={eyedropperSampleAllLayersNoAdj}
        onClick={() => setEyedropperSampleAllLayersNoAdj((v) => !v)}
      >
        No Adj
      </ToolChoiceButton>
      <span className="text-[11px] text-slate-400">{eyedropperModeSummary}</span>
      <span className="text-[11px] text-slate-400">
        Click sets foreground; Alt+click sets background.
      </span>
      <span className="text-[11px] text-slate-400">
        Shift+click adds a sampler point; remove them from the Info panel.
      </span>
    </>
  );
}
