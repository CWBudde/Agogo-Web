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
    <section className={className}>
      <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
        <h2 className="text-[12px] font-medium text-slate-100">{title}</h2>
      </div>
      <div className="h-[calc(100%-33px)] min-h-0 p-[var(--ui-gap-2)]">{children}</div>
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
