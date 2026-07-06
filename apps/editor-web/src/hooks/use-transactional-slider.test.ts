import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTransactionalSlider } from "@/hooks/use-transactional-slider";

// requestAnimationFrame shim: callbacks queue up and only run via runFrame().
let rafQueue = new Map<number, FrameRequestCallback>();
let nextRafId = 1;

function runFrame() {
  const queue = rafQueue;
  rafQueue = new Map();
  for (const callback of queue.values()) {
    callback(0);
  }
}

function makeEngine() {
  return {
    beginTransaction: vi.fn(() => null),
    endTransaction: vi.fn(() => null),
  };
}

describe("useTransactionalSlider", () => {
  beforeEach(() => {
    rafQueue = new Map();
    nextRafId = 1;
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      const id = nextRafId;
      nextRafId += 1;
      rafQueue.set(id, callback);
      return id;
    });
    vi.stubGlobal("cancelAnimationFrame", (id: number) => {
      rafQueue.delete(id);
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("opens exactly one transaction and dispatches at most once per frame with the latest value", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const { result } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Change layer opacity", engine, dispatch }),
    );

    result.current.change(10);
    result.current.change(20);
    result.current.change(30);

    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.beginTransaction).toHaveBeenCalledWith("Change layer opacity");
    expect(dispatch).not.toHaveBeenCalled();

    runFrame();
    expect(dispatch).toHaveBeenCalledTimes(1);
    expect(dispatch).toHaveBeenLastCalledWith(30);

    result.current.change(40);
    result.current.change(50);
    runFrame();
    expect(dispatch).toHaveBeenCalledTimes(2);
    expect(dispatch).toHaveBeenLastCalledWith(50);

    // Still a single transaction for the whole drag.
    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).not.toHaveBeenCalled();
  });

  it("commits on pointerup: flushes the final value, then ends the transaction once", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const calls: string[] = [];
    dispatch.mockImplementation(() => calls.push("dispatch"));
    engine.endTransaction.mockImplementation(() => {
      calls.push("end");
      return null;
    });

    const { result } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Change fill opacity", engine, dispatch }),
    );

    result.current.change(42);
    result.current.commitProps.onPointerUp();

    expect(dispatch).toHaveBeenCalledTimes(1);
    expect(dispatch).toHaveBeenLastCalledWith(42);
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(true);
    // The final value lands inside the transaction, before it ends.
    expect(calls).toEqual(["dispatch", "end"]);

    // A trailing blur (pointerup then focus loss) must not end a second transaction.
    result.current.commitProps.onBlur();
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);

    // A stale queued frame after commit must not double-dispatch.
    runFrame();
    expect(dispatch).toHaveBeenCalledTimes(1);
  });

  it("transacts keyboard-only edits: change then blur", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const { result } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Adjust levels", engine, dispatch }),
    );

    result.current.change(7);
    result.current.commitProps.onBlur();

    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(true);
    expect(dispatch).toHaveBeenCalledTimes(1);
    expect(dispatch).toHaveBeenLastCalledWith(7);
  });

  it("does nothing on commit handlers when no change happened", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const { result } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Adjust levels", engine, dispatch }),
    );

    result.current.commitProps.onPointerUp();
    result.current.commitProps.onLostPointerCapture();
    result.current.commitProps.onBlur();

    expect(engine.beginTransaction).not.toHaveBeenCalled();
    expect(engine.endTransaction).not.toHaveBeenCalled();
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("commits (never cancels) an open transaction on unmount", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const { result, unmount } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Adjust levels", engine, dispatch }),
    );

    result.current.change(3);
    unmount();

    // EndTransaction(false) reverts the live document, so unmount must commit.
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(true);
    expect(dispatch).toHaveBeenCalledTimes(1);
    expect(dispatch).toHaveBeenLastCalledWith(3);
  });

  it("starts a fresh transaction when the slider changes again after a commit", () => {
    const engine = makeEngine();
    const dispatch = vi.fn();
    const { result } = renderHook(() =>
      useTransactionalSlider<number>({ label: "Adjust levels", engine, dispatch }),
    );

    result.current.change(1);
    result.current.commitProps.onPointerUp();
    result.current.change(2);
    result.current.commitProps.onPointerUp();

    expect(engine.beginTransaction).toHaveBeenCalledTimes(2);
    expect(engine.endTransaction).toHaveBeenCalledTimes(2);
    expect(dispatch).toHaveBeenCalledTimes(2);
    expect(dispatch).toHaveBeenLastCalledWith(2);
  });
});
