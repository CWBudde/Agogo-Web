import type { ViewportMeta } from "@agogo/proto";

/**
 * Field-wise viewport equality. Ack merges recreate the viewport object every
 * frame even when nothing moved, so identity comparison is useless here. Keep
 * this in sync with ViewportMeta — a missed field means silently missed (or
 * spurious) re-renders for viewport subscribers.
 */
export function sameViewport(a: ViewportMeta | null, b: ViewportMeta | null): boolean {
  if (a === b) {
    return true;
  }
  if (!a || !b) {
    return false;
  }
  return (
    a.centerX === b.centerX &&
    a.centerY === b.centerY &&
    a.zoom === b.zoom &&
    a.rotation === b.rotation &&
    a.canvasW === b.canvasW &&
    a.canvasH === b.canvasH &&
    a.devicePixelRatio === b.devicePixelRatio
  );
}
