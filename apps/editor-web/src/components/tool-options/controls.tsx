import type { ReactNode } from "react";
import { parseNumericInput } from "@/lib/utils";

export function ToolOptionGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <span className="shrink-0 text-[11px] uppercase tracking-[0.18em] text-slate-500">
        {label}
      </span>
      <div className="flex items-center gap-1">{children}</div>
    </div>
  );
}

export function ToolChoiceButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={[
        "h-7 rounded-[var(--ui-radius-sm)] border px-2.5 text-[12px] transition focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-cyan-400/30",
        active
          ? "border-cyan-400/35 bg-cyan-400/14 text-slate-50"
          : "border-white/10 bg-black/20 text-slate-300 hover:border-white/20 hover:bg-black/30",
      ].join(" ")}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function ToolNumberField({
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
  onChange: (value: number) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-[12px] text-slate-300">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{label}</span>
      <input
        className="h-7 w-20 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-right text-[12px] text-slate-100 outline-none transition focus:border-cyan-400/40 focus-visible:ring-1 focus-visible:ring-cyan-400/30"
        type="number"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(parseNumericInput(event.target.value, value))}
      />
    </label>
  );
}

export function ToolSelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="flex items-center gap-2 text-[12px] text-slate-300">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{label}</span>
      <select
        className="h-7 max-w-56 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 text-[12px] text-slate-100 outline-none transition focus:border-cyan-400/40 focus-visible:ring-1 focus-visible:ring-cyan-400/30"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}
