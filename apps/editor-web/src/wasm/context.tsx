import {
  CommandID,
  type CreateDocumentCommand,
  type CreateSelectionCommand,
  type PointerEventCommand,
  type QuickSelectCommand,
  type RenderResult,
  type TransformSelectionCommand,
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
import type { EngineContextValue, EngineHandle } from "./types";

const EngineContext = createContext<EngineContextValue | null>(null);

type EngineState = {
  status: EngineContextValue["status"];
  handle: EngineHandle | null;
  render: RenderResult | null;
  error: Error | null;
};

type EngineAction =
  | { type: "load" }
  | { type: "ready"; handle: EngineHandle; render: RenderResult }
  | { type: "render"; render: RenderResult }
  | { type: "error"; error: Error };

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

  useEffect(() => {
    let active = true;
    dispatch({ type: "load" });

    void loadEngine({
      config: {
        documentWidth: 1920,
        documentHeight: 1080,
        background: "transparent",
        resolution: 72,
      },
    })
      .then((handle) => {
        if (!active) {
          return;
        }
        dispatch({ type: "ready", handle, render: handle.renderFrame() });
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
    };
  }, []);

  // Stable ref that always points to the latest handle.
  // Command handlers use this ref so their function identity doesn't change on
  // every render (which would cause useEffect deps to re-fire on every frame).
  const handleRef = useRef(state.handle);
  handleRef.current = state.handle;

  // Stable command handlers — created once, never change identity across renders.
  // All functions call handleRef.current at invocation time so they always reach
  // the live engine handle without capturing stale state in their closure.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const handlers = useMemo(() => {
    const run = (commandId: number, payload?: unknown) => {
      const handle = handleRef.current;
      if (!handle) {
        return null;
      }
      const result = handle.dispatch(commandId, payload);
      dispatch({ type: "render", render: result });
      return result;
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
      quickSelect(command: QuickSelectCommand) {
        return run(CommandID.QuickSelect, command);
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
      exportProject() {
        return handleRef.current?.exportProject() ?? null;
      },
      importProject(projectJSON: string) {
        const handle = handleRef.current;
        if (!handle) {
          return null;
        }
        const result = handle.importProject(projectJSON);
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
