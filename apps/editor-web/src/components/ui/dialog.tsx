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
    <div className="editor-backdrop fixed inset-0 z-50 flex items-center justify-center p-3">
      <div
        role="dialog"
        aria-modal="true"
        className={cn("editor-popup w-full max-w-2xl rounded-[var(--ui-radius-lg)] p-4", className)}
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
