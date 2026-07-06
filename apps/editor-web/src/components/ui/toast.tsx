import { useEffect, useRef, useState } from "react";
import { subscribeToasts, type ToastOptions } from "@/lib/toast-bus";

const MAX_TOASTS = 4;
const DEFAULT_DURATION_MS = 6000;

interface ToastEntry extends ToastOptions {
  id: number;
}

const KIND_ACCENT_CLASS: Record<ToastOptions["kind"], string> = {
  info: "border-l-accent",
  warning: "border-l-warning",
  error: "border-l-red-500",
};

/**
 * Fixed bottom-right toast stack. Mount exactly once at the App root; it
 * renders every toast emitted through the toast bus (`emitToast`), keeps at
 * most the four newest, and auto-dismisses each after its duration.
 */
export function ToastViewport() {
  const [toasts, setToasts] = useState<ToastEntry[]>([]);
  const timersRef = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  useEffect(() => {
    const timers = timersRef.current;
    let nextId = 1;

    const remove = (id: number) => {
      const timer = timers.get(id);
      if (timer !== undefined) {
        clearTimeout(timer);
        timers.delete(id);
      }
      setToasts((current) => current.filter((toast) => toast.id !== id));
    };

    const unsubscribe = subscribeToasts((options) => {
      const id = nextId;
      nextId += 1;
      setToasts((current) => {
        const next = [...current, { ...options, id }];
        // Cap the stack: drop the oldest toasts and their pending timers.
        for (const dropped of next.slice(0, Math.max(0, next.length - MAX_TOASTS))) {
          const timer = timers.get(dropped.id);
          if (timer !== undefined) {
            clearTimeout(timer);
            timers.delete(dropped.id);
          }
        }
        return next.slice(-MAX_TOASTS);
      });
      timers.set(
        id,
        setTimeout(() => remove(id), options.durationMs ?? DEFAULT_DURATION_MS),
      );
    });

    return () => {
      unsubscribe();
      for (const timer of timers.values()) {
        clearTimeout(timer);
      }
      timers.clear();
    };
  }, []);

  const dismiss = (id: number) => {
    const timer = timersRef.current.get(id);
    if (timer !== undefined) {
      clearTimeout(timer);
      timersRef.current.delete(id);
    }
    setToasts((current) => current.filter((toast) => toast.id !== id));
  };

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-[var(--ui-gap-4)]">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role="status"
          aria-live={toast.kind === "error" ? "assertive" : "polite"}
          className={`editor-popup pointer-events-auto flex items-start gap-[var(--ui-gap-4)] rounded-[var(--ui-radius-lg)] border-l-2 px-3 py-2.5 ${KIND_ACCENT_CLASS[toast.kind]}`}
        >
          <div className="min-w-0 flex-1 text-[12px]">
            {toast.title ? <p className="font-semibold text-foreground">{toast.title}</p> : null}
            <p className="break-words text-muted-foreground">{toast.message}</p>
          </div>
          <button
            type="button"
            aria-label="Dismiss notification"
            className="shrink-0 rounded-[var(--ui-radius-sm)] px-1 text-[13px] leading-none text-muted-foreground hover:text-foreground focus-visible:outline-none"
            onClick={() => dismiss(toast.id)}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
