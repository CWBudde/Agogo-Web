import type { HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

type ScrollAreaProps = PropsWithChildren<
  HTMLAttributes<HTMLDivElement> & {
    viewportClassName?: string;
  }
>;

export function ScrollArea({ className, viewportClassName, children, ...props }: ScrollAreaProps) {
  return (
    <div className={cn("relative overflow-hidden", className)} {...props}>
      <div className={cn("h-full w-full overflow-auto", viewportClassName)}>{children}</div>
    </div>
  );
}
