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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/74 p-3 backdrop-blur-[2px]">
      <div
        role="dialog"
        aria-modal="true"
        className={cn(
          "editor-panel w-full max-w-2xl rounded-[var(--ui-radius-lg)] p-4 shadow-[0_14px_30px_rgba(0,0,0,0.35)]",
          className,
        )}
      >
        {(title || description) && (
          <header className="mb-3 border-b border-border pb-3">
            {title ? <h2 className="text-sm font-semibold text-slate-100">{title}</h2> : null}
            {description ? (
              <p className="mt-1 text-xs leading-5 text-slate-400">{description}</p>
            ) : null}
          </header>
        )}
        {children}
      </div>
    </div>
  );
}
