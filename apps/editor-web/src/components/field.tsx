import type { ReactNode } from "react";

export const fieldClassName =
  "h-[var(--ui-h-md)] w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2.5 text-[13px] text-slate-100 outline-none transition focus:border-cyan-400/40 focus-visible:ring-1 focus-visible:ring-cyan-400/30";

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: label wraps its control via children (implicit label pattern)
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{label}</span>
      {children}
    </label>
  );
}
