import { CommandID, type DefinePatternCommand } from "@agogo/proto";
import type { EngineContextValue } from "@/wasm/types";

export type PatternMetaEntry = { id: string; name: string; width: number; height: number };

type PatternPickerProps = {
  engine: EngineContextValue;
  patterns: PatternMetaEntry[];
  value: string;
  onChange(patternId: string): void;
};

/**
 * Minimal pattern selector for the fill tool: a dropdown over the engine's
 * pattern list (builtins + document-defined) plus a "Define Pattern" button
 * that captures the active layer (or the current selection's bounding rect).
 */
export function PatternPicker({ engine, patterns, value, onChange }: PatternPickerProps) {
  return (
    <div className="flex items-center gap-2">
      <span className="shrink-0 text-[11px] uppercase tracking-[0.18em] text-slate-500">
        Pattern
      </span>
      <select
        className="h-7 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/30"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {patterns.map((pattern) => (
          <option key={pattern.id} value={pattern.id}>
            {pattern.name} ({pattern.width}×{pattern.height})
          </option>
        ))}
      </select>
      <button
        type="button"
        className="h-7 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2.5 text-[12px] text-slate-300 transition hover:border-white/20 hover:bg-black/30 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/30"
        onClick={() =>
          engine.dispatchCommand(CommandID.DefinePattern, {} satisfies DefinePatternCommand)
        }
      >
        Define Pattern
      </button>
    </div>
  );
}
