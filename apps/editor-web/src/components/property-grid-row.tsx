export function PropertyGridRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 px-2 py-1.5 text-[12px]">
      <span className="text-slate-400">{label}</span>
      <span className="text-slate-100">{value}</span>
    </div>
  );
}
