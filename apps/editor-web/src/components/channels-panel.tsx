export function ChannelsPanel({
  savedSelections,
}: {
  savedSelections: Array<{ name: string; pixelCount: number }>;
}) {
  return (
    <div className="space-y-[var(--ui-gap-1)]">
      {savedSelections.length > 0 ? (
        <>
          <div className="px-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            Saved Selection Channels
          </div>
          {savedSelections.map((channel) => (
            <div
              key={channel.name}
              className="flex items-center gap-2 rounded-[var(--ui-radius-sm)] border border-white/8 bg-white/[0.02] px-2 py-1.5"
            >
              <span className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] bg-black/20 text-[10px] text-slate-400">
                A
              </span>
              <span className="flex-1 text-[12px] font-medium text-slate-100">{channel.name}</span>
              <span className="text-[11px] text-slate-500">{channel.pixelCount}px</span>
            </div>
          ))}
        </>
      ) : (
        <p className="px-1 py-2 text-[11px] text-slate-500">No saved selection channels.</p>
      )}
    </div>
  );
}
