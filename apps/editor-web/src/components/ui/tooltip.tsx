import type { PropsWithChildren, ReactNode } from "react";

type TooltipProps = PropsWithChildren<{
  label: ReactNode;
}>;

export function Tooltip({ label, children }: TooltipProps) {
  return <span title={typeof label === "string" ? label : undefined}>{children}</span>;
}
