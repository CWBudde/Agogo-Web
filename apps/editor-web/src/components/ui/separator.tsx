import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

type SeparatorProps = HTMLAttributes<HTMLHRElement> & {
  orientation?: "horizontal" | "vertical";
};

export function Separator({ orientation = "horizontal", className, ...props }: SeparatorProps) {
  return (
    <hr
      aria-orientation={orientation}
      className={cn(
        orientation === "vertical" ? "h-full w-px" : "h-px w-full",
        "border-0 bg-border/80",
        className,
      )}
      {...props}
    />
  );
}
