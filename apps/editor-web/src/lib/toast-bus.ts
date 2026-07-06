/**
 * Module-level toast event bus.
 *
 * Deliberately free of any React dependency: non-React modules (e.g. the
 * wasm engine context) emit toasts through `emitToast`, and the React
 * `<ToastViewport />` subscribes via `subscribeToasts` to render them.
 */

export interface ToastOptions {
  kind: "info" | "warning" | "error";
  title?: string;
  message: string;
  durationMs?: number;
}

type ToastListener = (options: ToastOptions) => void;

const listeners = new Set<ToastListener>();

export function emitToast(options: ToastOptions): void {
  for (const listener of listeners) {
    // Isolate listener failures: emitToast is called from error-handling
    // paths (e.g. the engine context's catch blocks), so a throwing
    // subscriber must never propagate back into them.
    try {
      listener(options);
    } catch (error) {
      console.error("Toast listener failed:", error);
    }
  }
}

export function subscribeToasts(listener: ToastListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
