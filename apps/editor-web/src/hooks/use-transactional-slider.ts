import { useCallback, useEffect, useMemo, useRef } from "react";

/** The subset of the engine context a transactional slider needs. */
export interface TransactionalSliderEngine {
  beginTransaction(description: string): unknown;
  endTransaction(commit?: boolean): unknown;
}

/** Handlers to spread onto the slider input so releasing/leaving it commits. */
export interface TransactionalSliderCommitProps {
  onPointerUp: () => void;
  onLostPointerCapture: () => void;
  onBlur: () => void;
}

export interface TransactionalSlider<T> {
  /** Call with the newest value on every input change (rAF-throttled). */
  change: (value: T) => void;
  /** Flush the pending value and end the open transaction (commit). */
  commit: () => void;
  /** Spread onto the input element: pointerup / lostpointercapture / blur. */
  commitProps: TransactionalSliderCommitProps;
}

/**
 * Groups a slider drag into a single engine history transaction.
 *
 * - Lazy begin: the first `change` opens `engine.beginTransaction(label)`,
 *   which also covers keyboard-driven range changes that never fire
 *   pointerdown.
 * - rAF throttle: the latest value is kept in a ref and dispatched at most
 *   once per animation frame.
 * - Commit: `commitProps` (pointerup / lostpointercapture / blur) flushes the
 *   final value and calls `endTransaction(true)`. Keyboard-only edits commit
 *   on blur.
 * - Unmount with an open transaction commits too — the engine's
 *   `EndTransaction(false)` reverts the live document, so cancelling here
 *   would throw away the user's edit.
 */
export function useTransactionalSlider<T>({
  label,
  engine,
  dispatch,
}: {
  label: string;
  engine: TransactionalSliderEngine;
  dispatch: (value: T) => void;
}): TransactionalSlider<T> {
  const labelRef = useRef(label);
  const engineRef = useRef(engine);
  const dispatchRef = useRef(dispatch);
  useEffect(() => {
    labelRef.current = label;
    engineRef.current = engine;
    dispatchRef.current = dispatch;
  });

  const openRef = useRef(false);
  const pendingRef = useRef<{ value: T } | null>(null);
  const frameRef = useRef<number | null>(null);

  const flush = useCallback(() => {
    if (frameRef.current !== null) {
      cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
    const pending = pendingRef.current;
    pendingRef.current = null;
    if (pending) {
      dispatchRef.current(pending.value);
    }
  }, []);

  const change = useCallback((value: T) => {
    if (!openRef.current) {
      engineRef.current.beginTransaction(labelRef.current);
      openRef.current = true;
    }
    pendingRef.current = { value };
    if (frameRef.current === null) {
      frameRef.current = requestAnimationFrame(() => {
        frameRef.current = null;
        const pending = pendingRef.current;
        pendingRef.current = null;
        if (pending) {
          dispatchRef.current(pending.value);
        }
      });
    }
  }, []);

  const commit = useCallback(() => {
    if (!openRef.current) {
      return;
    }
    flush();
    openRef.current = false;
    engineRef.current.endTransaction(true);
  }, [flush]);

  // Commit (never cancel) a transaction left open at unmount.
  useEffect(() => commit, [commit]);

  return useMemo(
    () => ({
      change,
      commit,
      commitProps: {
        onPointerUp: commit,
        onLostPointerCapture: commit,
        onBlur: commit,
      },
    }),
    [change, commit],
  );
}
