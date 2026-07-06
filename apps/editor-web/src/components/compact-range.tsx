import { parseNumericInput } from "@/lib/utils";

export function CompactRange({
  id,
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  id: string;
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{Math.round(value)}</span>
      </div>
      <input
        id={id}
        className="h-2 w-full accent-accent focus-visible:outline-none"
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(parseNumericInput(event.target.value, value))}
      />
    </label>
  );
}
