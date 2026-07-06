const REF_POINT_LABELS = [
  ["Top Left", "Top Center", "Top Right"],
  ["Middle Left", "Center", "Middle Right"],
  ["Bottom Left", "Bottom Center", "Bottom Right"],
];

export function TransformRefGrid({
  active,
  onChange,
}: {
  active: [number, number];
  onChange(row: number, col: number): void;
}) {
  return (
    <div
      className="grid grid-cols-3 gap-[2px] rounded-[2px] border border-white/20 p-[3px]"
      title="Reference point — sets the pivot for the transform"
    >
      {([0, 1, 2] as const).flatMap((row) =>
        ([0, 1, 2] as const).map((col) => {
          const isActive = active[0] === row && active[1] === col;
          return (
            <button
              key={`${row}-${col}`}
              type="button"
              title={REF_POINT_LABELS[row][col]}
              aria-label={REF_POINT_LABELS[row][col]}
              className={[
                "h-[7px] w-[7px] rounded-[1px] focus-visible:outline-none",
                isActive ? "bg-cyan-400" : "bg-slate-500 hover:bg-slate-300",
              ].join(" ")}
              onClick={() => onChange(row, col)}
            />
          );
        }),
      )}
    </div>
  );
}
