import type { PropsWithChildren, ReactNode } from "react";
import { cn } from "@/lib/utils";

type DialogProps = PropsWithChildren<{
  open?: boolean;
  title?: ReactNode;
  description?: ReactNode;
  className?: string;
}>;

export function Dialog({ open = false, title, description, className, children }: DialogProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4 backdrop-blur-sm">
      <div
        role="dialog"
        aria-modal="true"
        className={cn(
          "w-full max-w-2xl rounded-[1.5rem] border border-white/10 bg-slate-950/95 p-6 shadow-2xl",
          className,
        )}
      >
        {(title || description) && (
          <header className="mb-4">
            {title ? <h2 className="text-lg font-semibold">{title}</h2> : null}
            {description ? <p className="mt-1 text-sm text-slate-400">{description}</p> : null}
          </header>
        )}
        {children}
      </div>
    </div>
  );
}
