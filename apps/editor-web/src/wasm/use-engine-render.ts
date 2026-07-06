import type { UIMeta, ViewportMeta } from "@agogo/proto";
import { useCallback, useRef, useSyncExternalStore } from "react";
import { useEngineStore } from "./context";
import type { EngineRenderState } from "./types";
import { sameViewport } from "./viewport-equal";

type SelectorCache<T> = {
  hasValue: boolean;
  snapshot: EngineRenderState | null;
  value: T;
};

/**
 * Subscribe to a slice of the engine's render state.
 *
 * Built on `useSyncExternalStore` over the EngineProvider's listener set: the
 * component re-renders only when the selected value changes (per `isEqual`,
 * default `Object.is`), not on every committed frame. The last selected value
 * is cached per snapshot so `getSnapshot` stays referentially stable — a
 * selector may safely return a fresh object each call as long as `isEqual`
 * identifies equivalent results (otherwise every commit re-renders the
 * subscriber, but never loops).
 *
 * The selector runs against `null` until the engine has produced its first
 * render state.
 *
 * Caveat (until B6): the cached value is keyed on the snapshot only — a
 * selector that closes over changing props/state returns the PREVIOUS value
 * until the next engine commit. Do not parameterize selectors with props
 * (e.g. `useUiMeta(m => m?.layers.find(l => l.id === selectedId))`); select
 * the full slice and derive from it in the component instead.
 */
export function useEngineRender<T>(
  selector: (state: EngineRenderState | null) => T,
  isEqual: (a: T, b: T) => boolean = Object.is,
): T {
  const { subscribe, getSnapshot } = useEngineStore();

  // Latest selector/isEqual without destabilizing getSelected's identity —
  // inline arrow functions at call sites are fine.
  const selectorRef = useRef(selector);
  selectorRef.current = selector;
  const isEqualRef = useRef(isEqual);
  isEqualRef.current = isEqual;

  const cacheRef = useRef<SelectorCache<T>>({
    hasValue: false,
    snapshot: null,
    value: undefined as T,
  });

  const getSelected = useCallback(() => {
    const snapshot = getSnapshot();
    const cache = cacheRef.current;
    if (cache.hasValue && cache.snapshot === snapshot) {
      return cache.value;
    }
    const next = selectorRef.current(snapshot);
    if (cache.hasValue && isEqualRef.current(cache.value, next)) {
      // Same logical value for a new snapshot: keep the previous reference so
      // useSyncExternalStore sees no change and skips the re-render.
      cache.snapshot = snapshot;
      return cache.value;
    }
    cache.hasValue = true;
    cache.snapshot = snapshot;
    cache.value = next;
    return next;
  }, [getSnapshot]);

  return useSyncExternalStore(subscribe, getSelected, getSelected);
}

/**
 * Select from the engine's UIMeta (document state). `meta` is `null` until
 * the first full render arrives.
 */
export function useUiMeta<T>(
  selector: (meta: UIMeta | null) => T,
  isEqual?: (a: T, b: T) => boolean,
): T {
  return useEngineRender((state) => selector(state?.uiMeta ?? null), isEqual);
}

/**
 * The current viewport, compared by value: ack merges recreate the viewport
 * object every frame even when nothing moved, so identity comparison would
 * re-render on every cursor/status ack.
 */
export function useViewport(): ViewportMeta | null {
  return useEngineRender((state) => state?.viewport ?? null, sameViewport);
}

/** Whether a document is open (the engine reports a non-zero document size). */
export function useHasDocument(): boolean {
  return useEngineRender((state) => (state?.uiMeta.documentWidth ?? 0) > 0);
}
