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
  type ViewportMeta,
} from "@agogo/proto";
import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
} from "react";
import { loadEngine } from "./loader";
import type { EngineContextValue, EngineHandle, EngineRenderState } from "./types";

const EngineContext = createContext<EngineContextValue | null>(null);

type EngineState = {
  status: EngineContextValue["status"];
  handle: EngineHandle | null;
  render: EngineRenderState | null;
  error: Error | null;
};

type EngineAction =
  | { type: "load" }
  | { type: "ready"; handle: EngineHandle; render: EngineRenderState }
  | { type: "render"; render: EngineRenderState }
  | { type: "error"; error: Error };

function isFullRender(result: RenderResult): result is EngineRenderState {
  return result.uiMeta !== undefined;
}

function sameViewport(a: ViewportMeta, b: ViewportMeta): boolean {
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

function reducer(state: EngineState, action: EngineAction): EngineState {
  switch (action.type) {
    case "load":
      return { ...state, status: "loading", error: null };
    case "ready":
      return {
        status: "ready",
        handle: action.handle,
        render: action.render,
        error: null,
      };
    case "render":
      return { ...state, render: action.render, error: null };
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
    render: null,
    error: null,
  });

  // Mutable mirror of the most recent merged render state. run() may fire
  // several times between React commits (rAF-batched pointer input), so the
  // ack merge/change-detection below reads this ref instead of `state.render`.
  const latestRenderRef = useRef<EngineRenderState | null>(null);

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
        dispatch({ type: "ready", handle, render });
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
  }, []);

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
      latestRenderRef.current = render;
      dispatch({ type: "render", render });
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
      const result = handle.dispatchCommand(commandId, payload);
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
        return handleRef.current?.exportProject() ?? null;
      },
      exportDocument(format: string) {
        return handleRef.current?.exportDocument(format) ?? null;
      },
      importProject(projectJSON: string) {
        const handle = handleRef.current;
        if (!handle) {
          return null;
        }
        // ImportProject always returns a full render result.
        const result = handle.importProject(projectJSON) as EngineRenderState;
        latestRenderRef.current = result;
        dispatch({ type: "render", render: result });
        return result;
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
  }, []); // Intentionally empty — functions use handleRef, not closed-over state

  const value = useMemo<EngineContextValue>(
    () => ({
      ...handlers,
      status: state.status,
      handle: state.handle,
      render: state.render,
      error: state.error,
      ready: state.handle ? Promise.resolve(state.handle) : null,
    }),
    [handlers, state.error, state.handle, state.render, state.status],
  );

  return <EngineContext.Provider value={value}>{children}</EngineContext.Provider>;
}

export function useEngine() {
  const context = useContext(EngineContext);
  if (!context) {
    throw new Error("useEngine must be used inside <EngineProvider>.");
  }

  return context;
}
