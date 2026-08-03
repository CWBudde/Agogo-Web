import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AUTOSAVE_KEY, autosaveKeyForDocument, useAutosave } from "@/hooks/use-autosave";
import { subscribeToasts, type ToastOptions } from "@/lib/toast-bus";

// requestIdleCallback shim: callbacks queue up and only run via flushIdle().
let idleQueue = new Map<number, () => void>();
let nextIdleId = 1;

function flushIdle() {
  const queue = idleQueue;
  idleQueue = new Map();
  for (const callback of queue.values()) {
    callback();
  }
}

function makeEngine(base64Zip: string | null = "ZmFrZS16aXA=") {
  return { exportProject: vi.fn(() => base64Zip) };
}

describe("useAutosave", () => {
  let toasts: ToastOptions[] = [];
  let unsubscribeToasts: () => void;

  beforeEach(() => {
    idleQueue = new Map();
    nextIdleId = 1;
    vi.stubGlobal("requestIdleCallback", (callback: () => void) => {
      const id = nextIdleId;
      nextIdleId += 1;
      idleQueue.set(id, callback);
      return id;
    });
    vi.stubGlobal("cancelIdleCallback", (id: number) => {
      idleQueue.delete(id);
    });
    localStorage.removeItem(AUTOSAVE_KEY);
    toasts = [];
    unsubscribeToasts = subscribeToasts((toast) => toasts.push(toast));
  });

  afterEach(() => {
    unsubscribeToasts();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    localStorage.removeItem(AUTOSAVE_KEY);
  });

  it("does not export before the version threshold is reached", () => {
    const engine = makeEngine();
    const { rerender } = renderHook(
      ({ contentVersion }) => useAutosave({ engine, contentVersion, enabled: true }),
      { initialProps: { contentVersion: 1 } },
    );
    rerender({ contentVersion: 9 });
    flushIdle();

    expect(engine.exportProject).not.toHaveBeenCalled();
  });

  it("coalesces a burst of version bumps into exactly one export", () => {
    const engine = makeEngine();
    const { rerender } = renderHook(
      ({ contentVersion }) => useAutosave({ engine, contentVersion, enabled: true }),
      { initialProps: { contentVersion: 10 } },
    );
    // Newer versions arrive before the idle callback runs.
    rerender({ contentVersion: 11 });
    rerender({ contentVersion: 12 });
    expect(engine.exportProject).not.toHaveBeenCalled();

    flushIdle();

    expect(engine.exportProject).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem(AUTOSAVE_KEY)).toBe("ZmFrZS16aXA=");
  });

  it("cancels a pending save when the active document changes", () => {
    const engine = makeEngine();
    const { rerender } = renderHook(
      ({ contentVersion, documentId }) =>
        useAutosave({ engine, contentVersion, documentId, enabled: true }),
      { initialProps: { contentVersion: 10, documentId: "doc-a" } },
    );

    rerender({ contentVersion: 1, documentId: "doc-b" });
    flushIdle();

    expect(engine.exportProject).not.toHaveBeenCalled();
    expect(localStorage.getItem(autosaveKeyForDocument("doc-a"))).toBeNull();
    expect(localStorage.getItem(autosaveKeyForDocument("doc-b"))).toBeNull();
  });

  it("saves again once the threshold is crossed a second time", () => {
    const engine = makeEngine();
    const { rerender } = renderHook(
      ({ contentVersion }) => useAutosave({ engine, contentVersion, enabled: true }),
      { initialProps: { contentVersion: 10 } },
    );
    flushIdle();
    expect(engine.exportProject).toHaveBeenCalledTimes(1);

    rerender({ contentVersion: 15 });
    flushIdle();
    expect(engine.exportProject).toHaveBeenCalledTimes(1);

    rerender({ contentVersion: 20 });
    flushIdle();
    expect(engine.exportProject).toHaveBeenCalledTimes(2);
  });

  it("does nothing when disabled or without a content version", () => {
    const engine = makeEngine();
    const { rerender } = renderHook(
      (props: Parameters<typeof useAutosave>[0]) => useAutosave(props),
      {
        initialProps: { engine, contentVersion: undefined as number | undefined, enabled: true },
      },
    );
    rerender({ engine, contentVersion: 50, enabled: false });
    flushIdle();

    expect(engine.exportProject).not.toHaveBeenCalled();
  });

  it("falls back to setTimeout when requestIdleCallback is unavailable", () => {
    vi.stubGlobal("requestIdleCallback", undefined);
    vi.stubGlobal("cancelIdleCallback", undefined);
    vi.useFakeTimers();
    try {
      const engine = makeEngine();
      renderHook(() => useAutosave({ engine, contentVersion: 10, enabled: true }));
      expect(engine.exportProject).not.toHaveBeenCalled();

      vi.advanceTimersByTime(1000);
      expect(engine.exportProject).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("emits a warning toast only once per session on storage failure", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("QuotaExceededError");
    });
    const engine = makeEngine();
    const { rerender } = renderHook(
      ({ contentVersion }) => useAutosave({ engine, contentVersion, enabled: true }),
      { initialProps: { contentVersion: 10 } },
    );
    flushIdle();

    const warnings = () => toasts.filter((toast) => toast.kind === "warning");
    expect(warnings()).toHaveLength(1);
    expect(warnings()[0]?.title).toBe("Autosave failed");

    // A second failing save must not toast again.
    rerender({ contentVersion: 20 });
    flushIdle();
    expect(engine.exportProject).toHaveBeenCalledTimes(2);
    expect(warnings()).toHaveLength(1);
  });
});
