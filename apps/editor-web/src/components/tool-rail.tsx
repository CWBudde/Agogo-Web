import { toolItems } from "@/components/tool-rail-model";
import { useActivateTool } from "@/hooks/use-activate-tool";
import { rgbaToCss } from "@/lib/color";
import { useColorState } from "@/state/color-state";
import { useToolState } from "@/state/tool-state";
import { useViewState } from "@/state/view-state";

/**
 * The left-hand tool rail: one button per editor tool plus the
 * foreground/background color swatches at the bottom.
 */
export function ToolRail() {
  const { activeTool } = useToolState();
  const { isPanMode } = useViewState();
  const { foregroundColor, backgroundColor, openColorPicker } = useColorState();
  const activateTool = useActivateTool();

  return (
    <aside className="editor-chrome editor-toolrail flex min-h-[36rem] flex-col items-center gap-[var(--ui-gap-1)] border-r border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
      {toolItems.map((tool) => {
        const active = (isPanMode && tool.id === "hand") || activeTool === tool.id;
        const ToolIcon = tool.Icon;
        return (
          <button
            key={tool.id}
            type="button"
            className={[
              "flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] font-semibold transition focus-visible:outline-none",
              active
                ? "bg-cyan-400/14 text-cyan-100"
                : "bg-transparent text-slate-400 hover:bg-white/6 hover:text-slate-100",
            ].join(" ")}
            title={tool.label}
            aria-label={tool.label}
            aria-pressed={active}
            onClick={() => activateTool(tool.id)}
          >
            <ToolIcon className="h-4 w-4" />
          </button>
        );
      })}
      {/* Foreground / background color swatches */}
      <div className="relative mt-auto mb-1 flex h-10 w-10 flex-shrink-0 items-end justify-end">
        {/* Background swatch (behind) */}
        <button
          type="button"
          className="absolute bottom-0 right-0 h-6 w-6 rounded-sm border border-border"
          style={{ backgroundColor: rgbaToCss(backgroundColor) }}
          title="Background color"
          aria-label="Background color"
          onClick={() => openColorPicker("background")}
        />
        {/* Foreground swatch (front) */}
        <button
          type="button"
          className="absolute left-0 top-0 h-6 w-6 rounded-sm border border-border"
          style={{ backgroundColor: rgbaToCss(foregroundColor) }}
          title="Foreground color"
          aria-label="Foreground color"
          onClick={() => openColorPicker("foreground")}
        />
      </div>
    </aside>
  );
}
