import type { ReactNode } from "react";

export type AuxPanel =
  | "properties"
  | "info"
  | "adjustments"
  | "history"
  | "navigator"
  | "channels"
  | "brush"
  | "color"
  | "swatches"
  | "paths"
  | "shapes"
  | "styles";

export function DockSection({
  title,
  className,
  children,
}: {
  title: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section
      className={["flex min-h-0 flex-col overflow-hidden", className].filter(Boolean).join(" ")}
    >
      <div className="flex min-h-9 items-center border-b border-border bg-black/10 px-3 py-2">
        <h2 className="text-[12px] font-semibold tracking-[0.01em] text-foreground">{title}</h2>
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-1.5">{children}</div>
    </section>
  );
}

export function dockTitle(panel: AuxPanel) {
  switch (panel) {
    case "info":
      return "Info";
    case "brush":
      return "Brush Settings";
    case "color":
      return "Color";
    case "swatches":
      return "Swatches";
    case "history":
      return "History";
    case "navigator":
      return "Navigator";
    case "channels":
      return "Channels";
    case "paths":
      return "Paths";
    case "shapes":
      return "Shapes";
    case "adjustments":
      return "Adjustments";
    case "styles":
      return "Styles";
    default:
      return "Properties";
  }
}
