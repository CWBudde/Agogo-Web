import {
  type KeyboardEvent as ReactKeyboardEvent,
  type PropsWithChildren,
  type ReactNode,
  useEffect,
  useId,
  useRef,
} from "react";
import { cn } from "@/lib/utils";

type DialogProps = PropsWithChildren<{
  open?: boolean;
  title?: ReactNode;
  description?: ReactNode;
  className?: string;
  /** Called when the user dismisses the dialog (Escape). */
  onClose?: () => void;
}>;

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable="true"]',
].join(", ");

function isVisible(element: HTMLElement): boolean {
  if (typeof element.checkVisibility === "function") {
    return element.checkVisibility();
  }
  // Fallback for environments without checkVisibility (e.g. jsdom, where
  // offsetParent is also always null): [hidden] + computed style.
  if (element.hidden) {
    return false;
  }
  const style = window.getComputedStyle(element);
  return style.display !== "none" && style.visibility !== "hidden";
}

function getFocusable(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(isVisible);
}

export function Dialog({ open = false, ...contentProps }: DialogProps) {
  if (!open) {
    return null;
  }

  return <DialogContent {...contentProps} />;
}

function DialogContent({
  title,
  description,
  className,
  onClose,
  children,
}: Omit<DialogProps, "open">) {
  const dialogRef = useRef<HTMLDivElement | null>(null);
  const onCloseRef = useRef(onClose);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => {
    onCloseRef.current = onClose;
  });

  // Initial focus on open + focus restore on close/unmount.
  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const dialog = dialogRef.current;
    if (dialog) {
      const focusable = getFocusable(dialog);
      (focusable[0] ?? dialog).focus();
    }
    return () => {
      if (previouslyFocused?.isConnected) {
        previouslyFocused.focus();
      }
    };
  }, []);

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      onCloseRef.current?.();
      return;
    }

    if (event.key !== "Tab") {
      return;
    }

    const dialog = dialogRef.current;
    if (!dialog) {
      return;
    }

    const focusable = getFocusable(dialog);
    if (focusable.length === 0) {
      // Nothing to cycle through — keep focus on the dialog itself.
      event.preventDefault();
      dialog.focus();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement;

    if (event.shiftKey) {
      if (active === first || active === dialog) {
        event.preventDefault();
        last.focus();
      }
    } else if (active === last || active === dialog) {
      event.preventDefault();
      first.focus();
    }
  };

  return (
    <div className="editor-backdrop fixed inset-0 z-50 flex items-center justify-center p-3">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
        onKeyDown={handleKeyDown}
        className={cn("editor-popup w-full max-w-2xl rounded-[var(--ui-radius-lg)] p-4", className)}
      >
        {(title || description) && (
          <header className="mb-3 border-b border-border pb-3">
            {title ? (
              <h2 id={titleId} className="text-sm font-semibold text-slate-100">
                {title}
              </h2>
            ) : null}
            {description ? (
              <p id={descriptionId} className="mt-1 text-xs leading-5 text-slate-400">
                {description}
              </p>
            ) : null}
          </header>
        )}
        {children}
      </div>
    </div>
  );
}
