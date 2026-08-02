export const CANVAS_INPUT_FLUSH_EVENT = "agogo:flush-canvas-input";

/**
 * Ask the mounted editor canvas to synchronously drain its rAF-batched pointer
 * input before a command dispatched elsewhere in the application runs.
 */
export function flushCanvasInput(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(CANVAS_INPUT_FLUSH_EVENT));
  }
}
