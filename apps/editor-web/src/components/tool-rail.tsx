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
    <aside className="editor-chrome editor-toolrail flex min-h-0 flex-col items-center border-r border-border px-1 py-1.5">
      <div className="flex min-h-0 flex-1 flex-col items-center gap-0.5 overflow-y-auto overflow-x-hidden pb-2">
        {toolItems.map((tool) => {
          const active = (isPanMode && tool.id === "hand") || activeTool === tool.id;
          const ToolIcon = tool.Icon;
          return (
            <button
              key={tool.id}
              type="button"
              className={[
                "relative flex h-9 w-10 shrink-0 items-center justify-center rounded-[var(--ui-radius-md)] transition focus-visible:outline-none",
                active
                  ? "bg-accent/16 text-accent shadow-[inset_2px_0_0_hsl(var(--accent)),0_0_18px_hsl(var(--accent)/0.06)]"
                  : "bg-transparent text-muted-foreground hover:bg-white/8 hover:text-white",
              ].join(" ")}
              title={tool.label}
              aria-label={tool.label}
              aria-pressed={active}
              onClick={() => activateTool(tool.id)}
            >
              <ToolIcon className="h-[18px] w-[18px]" />
            </button>
          );
        })}
      </div>
      {/* Foreground / background color swatches */}
      <div className="relative mt-1 mb-1 flex h-10 w-10 flex-shrink-0 items-end justify-end">
        {/* Background swatch (behind) */}
        <button
          type="button"
          className="absolute right-0 bottom-0 h-6 w-6 rounded-[var(--ui-radius-sm)] border-2 border-panel shadow-md"
          style={{ backgroundColor: rgbaToCss(backgroundColor) }}
          title="Background color"
          aria-label="Background color"
          onClick={() => openColorPicker("background")}
        />
        {/* Foreground swatch (front) */}
        <button
          type="button"
          className="absolute top-0 left-0 h-6 w-6 rounded-[var(--ui-radius-sm)] border-2 border-panel shadow-md"
          style={{ backgroundColor: rgbaToCss(foregroundColor) }}
          title="Foreground color"
          aria-label="Foreground color"
          onClick={() => openColorPicker("foreground")}
        />
      </div>
    </aside>
  );
}
