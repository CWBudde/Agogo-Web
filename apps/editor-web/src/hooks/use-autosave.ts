import { useEffect, useRef } from "react";
import { emitToast } from "@/lib/toast-bus";

export const AUTOSAVE_KEY = "agogo:autosave";
export const AUTOSAVE_EVERY_N_VERSIONS = 10;

interface UseAutosaveOptions {
  engine: { exportProject: () => string | null };
  contentVersion: number | undefined;
  enabled: boolean;
}

type ScheduledSave = { cancel: () => void };

function scheduleIdle(callback: () => void): ScheduledSave {
  if (typeof requestIdleCallback === "function") {
    const id = requestIdleCallback(() => callback());
    return { cancel: () => cancelIdleCallback(id) };
  }
  const id = setTimeout(callback, 1000);
  return { cancel: () => clearTimeout(id) };
}

/**
 * Autosaves the project to localStorage every AUTOSAVE_EVERY_N_VERSIONS
 * content versions.
 *
 * The save is scheduled via requestIdleCallback (setTimeout fallback) so it
 * stays off the critical input path, and bursts of version bumps coalesce
 * into a single export: a newer version cancels the pending callback and
 * reschedules.
 *
 * LIMITATION: exportProject() itself is synchronous WASM work on the main
 * thread — only the *scheduling* is deferred to idle time. Moving the export
 * off the main thread would require a second engine instance in a worker,
 * which is out of scope.
 */
export function useAutosave({ engine, contentVersion, enabled }: UseAutosaveOptions): void {
  const lastSavedVersionRef = useRef(0);
  const failureToastShownRef = useRef(false);
  const pendingSaveRef = useRef<ScheduledSave | null>(null);
  const exportProjectRef = useRef(engine.exportProject);
  useEffect(() => {
    exportProjectRef.current = engine.exportProject;
  });

  useEffect(() => {
    if (!enabled || contentVersion === undefined || contentVersion === 0) {
      // Nothing to save (engine gone / document closed) — drop any save that
      // is still pending so it cannot fire against stale state.
      pendingSaveRef.current?.cancel();
      pendingSaveRef.current = null;
      return;
    }
    if (contentVersion - lastSavedVersionRef.current < AUTOSAVE_EVERY_N_VERSIONS) {
      return;
    }

    // Coalesce: a newer contentVersion supersedes any not-yet-run save.
    pendingSaveRef.current?.cancel();
    pendingSaveRef.current = scheduleIdle(() => {
      pendingSaveRef.current = null;
      try {
        const base64Zip = exportProjectRef.current();
        if (!base64Zip) {
          return;
        }
        localStorage.setItem(AUTOSAVE_KEY, base64Zip);
        lastSavedVersionRef.current = contentVersion;
      } catch (error) {
        // Quota exceeded or export failure — warn once per session, then
        // stay quiet so a persistently full localStorage doesn't spam.
        // TODO: lastSavedVersionRef is intentionally NOT advanced on failure,
        // so a persistently failing export reschedules a full synchronous
        // export at idle on every threshold crossing. Advancing the ref (or
        // backing off) after repeated failures is future work.
        if (!failureToastShownRef.current) {
          failureToastShownRef.current = true;
          emitToast({
            kind: "warning",
            title: "Autosave failed",
            message: error instanceof Error ? error.message : String(error),
          });
        }
      }
    });
  }, [contentVersion, enabled]);

  useEffect(() => {
    return () => {
      pendingSaveRef.current?.cancel();
      pendingSaveRef.current = null;
    };
  }, []);
}
