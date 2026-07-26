import {
  CommandID,
  type CreateDocumentCommand,
  type CreateSelectionCommand,
  type MagicWandCommand,
  type MagneticLassoSuggestPathCommand,
  type PickLayerAtPointCommand,
  type PointerEventCommand,
  type QuickSelectCommand,
  type RenderResult,
  type TransformSelectionCommand,
  type TranslateLayerCommand,
} from "@agogo/proto";
import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
} from "react";
import { emitToast } from "@/lib/toast-bus";
import { loadEngine } from "./loader";
import type { EngineContextValue, EngineHandle, EngineRenderState, EngineStore } from "./types";
import { sameViewport } from "./viewport-equal";

const EngineContext = createContext<EngineContextValue | null>(null);

// Identity-stable store surface for useSyncExternalStore subscribers.
// Kept in a SEPARATE context so subscribing components do not re-render when
// the main EngineContext value changes (it changes on every committed frame);
// they re-render only when their selected slice changes.
const EngineStoreContext = createContext<EngineStore | null>(null);

type EngineState = {
  status: EngineContextValue["status"];
  handle: EngineHandle | null;
  error: Error | null;
};

type EngineAction =
  | { type: "load" }
  | { type: "ready"; handle: EngineHandle }
  | { type: "error"; error: Error };

function isFullRender(result: RenderResult): result is EngineRenderState {
  return result.uiMeta !== undefined;
}

function surfaceEngineError(title: string, error: unknown): null {
  console.error(`${title}:`, error);
  emitToast({
    kind: "error",
    title,
    message: error instanceof Error ? error.message : String(error),
  });
  return null;
}

function reducer(state: EngineState, action: EngineAction): EngineState {
  switch (action.type) {
    case "load":
      return { ...state, status: "loading", error: null };
    case "ready":
      return {
        status: "ready",
        handle: action.handle,
        error: null,
      };
    case "error":
      return { ...state, status: "error", handle: null, error: action.error };
    default:
      return state;
  }
}

export function EngineProvider({ children }: PropsWithChildren) {
  const [state, dispatch] = useReducer(reducer, {
    status: "idle",
    handle: null,
    error: null,
  });

  // Mutable mirror of the most recent merged render state. run() may fire
  // several times between React commits (rAF-batched pointer input), so the
  // ack merge/change-detection below reads this ref (never a reducer field).
  const latestRenderRef = useRef<EngineRenderState | null>(null);

  // Subscribers notified whenever latestRenderRef.current is replaced —
  // i.e. on every commit() (full renders and visible ack merges) and on the
  // initial load. No-change acks never touch the ref, so they notify nobody.
  const listenersRef = useRef(new Set<() => void>());

  const notifyRenderListeners = useCallback(() => {
    for (const listener of listenersRef.current) {
      listener();
    }
  }, []);

  useEffect(() => {
    let active = true;
    let loadedHandle: EngineHandle | null = null;
    dispatch({ type: "load" });

    void loadEngine()
      .then((handle) => {
        if (!active) {
          handle.dispose();
          return;
        }
        loadedHandle = handle;
        // RenderFrame always returns a full result (uiMeta included).
        const render = handle.renderFrame() as EngineRenderState;
        latestRenderRef.current = render;
        notifyRenderListeners();
        dispatch({ type: "ready", handle });
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        dispatch({
          type: "error",
          error: error instanceof Error ? error : new Error("Failed to load the Wasm engine."),
        });
      });

    return () => {
      active = false;
      loadedHandle?.dispose();
      loadedHandle = null;
    };
  }, [notifyRenderListeners]);

  // Stable ref that always points to the latest handle.
  // Command handlers use this ref so their function identity doesn't change on
  // every render (which would cause useEffect deps to re-fire on every frame).
  const handleRef = useRef(state.handle);
  handleRef.current = state.handle;

  // rAF guard for UIMeta refreshes triggered by stale ack versions: at most
  // one RenderFrame per animation frame.
  const uiMetaRefreshRafRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (uiMetaRefreshRafRef.current !== null) {
        cancelAnimationFrame(uiMetaRefreshRafRef.current);
        uiMetaRefreshRafRef.current = null;
      }
    };
  }, []);

  // Stable command handlers — created once, never change identity across renders.
  // All functions call handleRef.current at invocation time so they always reach
  // the live engine handle without capturing stale state in their closure.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const handlers = useMemo(() => {
    const commit = (render: EngineRenderState) => {
      // A committed frame updates the mutable ref and notifies external-store
      // subscribers, but must NOT dispatch a reducer action: re-rendering
      // EngineProvider on every frame is exactly the fan-out B6 removes. The
      // reducer now fires only on status/handle/error transitions.
      latestRenderRef.current = render;
      notifyRenderListeners();
      return render;
    };

    const scheduleUiMetaRefresh = () => {
      if (uiMetaRefreshRafRef.current !== null) {
        return;
      }
      uiMetaRefreshRafRef.current = requestAnimationFrame(() => {
        uiMetaRefreshRafRef.current = null;
        const handle = handleRef.current;
        if (!handle) {
          return;
        }
        commit(handle.renderFrame() as EngineRenderState);
      });
    };

    const run = (commandId: number, payload?: unknown) => {
      const handle = handleRef.current;
      if (!handle) {
        return null;
      }
      let result: RenderResult;
      try {
        result = handle.dispatchCommand(commandId, payload);
      } catch (error) {
        // A failed command must never throw uncaught mid-gesture — callers
        // already tolerate null (same as "no handle yet").
        return surfaceEngineError("Engine command failed", error);
      }
      if (isFullRender(result)) {
        return commit(result);
      }

      // Hot-path ack (no uiMeta): merge into the last full render state so
      // consumers never see missing UIMeta fields. Keep the previous uiMeta
      // object (same reference) unless cursor/status changed, and skip the
      // React state update entirely when nothing visible changed — a steady
      // brush stroke then costs zero React re-renders per frame.
      const prev = latestRenderRef.current;
      if (!prev) {
        // No full render yet (cannot happen after "ready", kept as a safety
        // net): fetch one instead of synthesizing a partial state.
        return commit(handle.renderFrame() as EngineRenderState);
      }

      if (result.uiMetaVersion !== prev.uiMeta.version) {
        // The engine's UI state moved on without us (e.g. a command from
        // another code path); pull a full refresh, at most once per frame.
        scheduleUiMetaRefresh();
      }

      const cursorType = result.cursorType ?? prev.uiMeta.cursorType;
      const statusText = result.statusText ?? prev.uiMeta.statusText;
      const uiMetaChanged =
        cursorType !== prev.uiMeta.cursorType || statusText !== prev.uiMeta.statusText;
      const viewportChanged = !sameViewport(result.viewport, prev.viewport);
      if (!uiMetaChanged && !viewportChanged) {
        return prev;
      }
      return commit({
        ...prev,
        frameId: result.frameId,
        viewport: result.viewport,
        uiMeta: uiMetaChanged ? { ...prev.uiMeta, cursorType, statusText } : prev.uiMeta,
      });
    };

    return {
      dispatchCommand(commandId: number, payload?: unknown) {
        return run(commandId, payload);
      },
      createDocument(command: CreateDocumentCommand) {
        return run(CommandID.CreateDocument, command);
      },
      createSelection(command: CreateSelectionCommand) {
        return run(CommandID.NewSelection, command);
      },
      selectAll() {
        return run(CommandID.SelectAll);
      },
      deselect() {
        return run(CommandID.Deselect);
      },
      reselect() {
        return run(CommandID.Reselect);
      },
      invertSelection() {
        return run(CommandID.InvertSelection);
      },
      magicWand(command: MagicWandCommand) {
        return run(CommandID.MagicWand, command);
      },
      magneticLassoSuggestPath(command: MagneticLassoSuggestPathCommand) {
        return run(CommandID.MagneticLassoSuggestPath, command);
      },
      quickSelect(command: QuickSelectCommand) {
        return run(CommandID.QuickSelect, command);
      },
      pickLayerAtPoint(command: PickLayerAtPointCommand) {
        return run(CommandID.PickLayerAtPoint, command);
      },
      translateLayer(command: TranslateLayerCommand) {
        return run(CommandID.TranslateLayer, command);
      },
      transformSelection(command: TransformSelectionCommand) {
        return run(CommandID.TransformSelection, command);
      },
      resizeViewport(canvasW: number, canvasH: number, devicePixelRatio: number) {
        return run(CommandID.Resize, { canvasW, canvasH, devicePixelRatio });
      },
      setZoom(zoom: number, anchorX?: number, anchorY?: number) {
        return run(CommandID.ZoomSet, {
          zoom,
          hasAnchor: anchorX !== undefined && anchorY !== undefined,
          anchorX,
          anchorY,
        });
      },
      setPan(centerX: number, centerY: number) {
        return run(CommandID.PanSet, { centerX, centerY });
      },
      dispatchPointerEvent(command: PointerEventCommand) {
        return run(CommandID.PointerEvent, command);
      },
      beginTransaction(description: string) {
        return run(CommandID.BeginTransaction, { description });
      },
      endTransaction(commit = true) {
        return run(CommandID.EndTransaction, { commit });
      },
      jumpHistory(historyIndex: number) {
        return run(CommandID.JumpHistory, { historyIndex });
      },
      clearHistory() {
        return run(CommandID.ClearHistory);
      },
      setRotation(rotation: number) {
        return run(CommandID.RotateViewSet, { rotation });
      },
      fitToView() {
        return run(CommandID.FitToView);
      },
      setShowGuides(show: boolean) {
        return run(CommandID.SetShowGuides, { show });
      },
      exportProject() {
        try {
          return handleRef.current?.exportProject() ?? null;
        } catch (error) {
          return surfaceEngineError("Project export failed", error);
        }
      },
      exportDocument(format: string) {
        try {
          return handleRef.current?.exportDocument(format) ?? null;
        } catch (error) {
          return surfaceEngineError("Document export failed", error);
        }
      },
      importProject(projectJSON: string) {
        const handle = handleRef.current;
        if (!handle) {
          return null;
        }
        let result: EngineRenderState;
        try {
          // ImportProject always returns a full render result.
          result = handle.importProject(projectJSON) as EngineRenderState;
        } catch (error) {
          return surfaceEngineError("Project import failed", error);
        }
        return commit(result);
      },
      undo() {
        return run(CommandID.Undo);
      },
      redo() {
        return run(CommandID.Redo);
      },
      reload() {
        window.location.reload();
      },
    };
    // notifyRenderListeners is identity-stable (useCallback with []); all other
    // state is reached through refs, so this memo is created exactly once.
  }, [notifyRenderListeners]);

  // Identity-stable external-store surface (never changes across renders).
  const store = useMemo<EngineStore>(
    () => ({
      subscribe(listener: () => void) {
        listenersRef.current.add(listener);
        return () => {
          listenersRef.current.delete(listener);
        };
      },
      getSnapshot() {
        return latestRenderRef.current;
      },
    }),
    [],
  );

  // The context value is rebuilt only when status/handle/error change — never
  // on a committed frame. handlers and store are identity-stable, so a full
  // render (which no longer touches the reducer) leaves this value untouched,
  // and useEngine() consumers do not re-render per frame. subscribe/getSnapshot
  // ride along (EngineContextValue extends EngineStore) so existing callers and
  // the selector hooks share one identity-stable store.
  const value = useMemo<EngineContextValue>(
    () => ({
      ...handlers,
      ...store,
      status: state.status,
      handle: state.handle,
      error: state.error,
      ready: state.handle ? Promise.resolve(state.handle) : null,
    }),
    [handlers, store, state.error, state.handle, state.status],
  );

  return (
    <EngineContext.Provider value={value}>
      <EngineStoreContext.Provider value={store}>{children}</EngineStoreContext.Provider>
    </EngineContext.Provider>
  );
}

export function useEngine() {
  const context = useContext(EngineContext);
  if (!context) {
    throw new Error("useEngine must be used inside <EngineProvider>.");
  }

  return context;
}

/**
 * Access the identity-stable engine store (subscribe/getSnapshot) without
 * subscribing to the full EngineContext. Components that only need a slice of
 * the render state should use the selector hooks in use-engine-render.ts,
 * which are built on this store and skip re-renders for unrelated changes.
 */
export function useEngineStore(): EngineStore {
  const store = useContext(EngineStoreContext);
  if (!store) {
    throw new Error("useEngineStore must be used inside <EngineProvider>.");
  }

  return store;
}
