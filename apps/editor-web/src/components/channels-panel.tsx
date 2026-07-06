import { useState } from "react";

// Channel descriptor: short label, long name, indicator colour class.
const CHANNELS = [
  {
    id: "rgb",
    label: "RGB",
    name: "Composite",
    color: "bg-slate-400",
    shortcut: "~",
  },
  { id: "r", label: "R", name: "Red", color: "bg-rose-400", shortcut: "1" },
  {
    id: "g",
    label: "G",
    name: "Green",
    color: "bg-emerald-400",
    shortcut: "2",
  },
  { id: "b", label: "B", name: "Blue", color: "bg-blue-400", shortcut: "3" },
  { id: "a", label: "A", name: "Alpha", color: "bg-slate-300", shortcut: "4" },
] as const;

export function ChannelsPanel({
  savedSelections,
}: {
  savedSelections: Array<{ name: string; pixelCount: number }>;
}) {
  // Channel visibility is cosmetic for now; actual channel isolation is Phase 3+.
  const [visible, setVisible] = useState<Record<string, boolean>>({
    rgb: true,
    r: true,
    g: true,
    b: true,
    a: true,
  });

  return (
    <div className="space-y-[var(--ui-gap-1)]">
      {CHANNELS.map((ch) => (
        <div
          key={ch.id}
          className={[
            "flex items-center gap-2 rounded-[var(--ui-radius-sm)] border px-2 py-1.5 transition",
            visible[ch.id]
              ? "border-white/8 bg-white/[0.02]"
              : "border-white/4 bg-transparent opacity-50",
          ].join(" ")}
        >
          <button
            type="button"
            title={visible[ch.id] ? "Hide channel" : "Show channel"}
            aria-label={visible[ch.id] ? "Hide channel" : "Show channel"}
            className={[
              "flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] text-[10px] transition",
              visible[ch.id] ? "bg-emerald-400/12 text-emerald-100" : "bg-black/20 text-slate-500",
            ].join(" ")}
            onClick={() =>
              setVisible((current) => ({
                ...current,
                [ch.id]: !current[ch.id],
              }))
            }
          >
            {visible[ch.id] ? "O" : "-"}
          </button>
          <span className={`h-2.5 w-2.5 rounded-full ${ch.color}`} />
          <span className="flex-1 text-[12px] font-medium text-slate-100">{ch.name}</span>
          <span className="text-[11px] text-slate-500">{ch.shortcut}</span>
        </div>
      ))}
      {savedSelections.length > 0 ? (
        <>
          <div className="px-1 pt-2 text-[11px] uppercase tracking-[0.18em] text-slate-500">
            Alpha Channels
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
      ) : null}
      <p className="px-1 pt-1 text-[11px] text-slate-600">Channel isolation active in Phase 3+.</p>
    </div>
  );
}
