import {
  CommandID,
  type CreateDocumentCommand,
  type PointerEventCommand,
  type RenderResult,
} from "@agogo/proto";
import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useReducer,
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

  const value = useMemo<EngineContextValue>(() => {
    const run = (commandId: number, payload?: unknown) => {
      if (!state.handle) {
        return null;
      }
      const render = state.handle.dispatch(commandId, payload);
      dispatch({ type: "render", render });
      return render;
    };

    return {
      status: state.status,
      handle: state.handle,
      render: state.render,
      error: state.error,
      ready: state.handle ? Promise.resolve(state.handle) : null,
      dispatchCommand(commandId: number, payload?: unknown) {
        return run(commandId, payload);
      },
      createDocument(command: CreateDocumentCommand) {
        return run(CommandID.CreateDocument, command);
      },
      resizeViewport(canvasW, canvasH, devicePixelRatio) {
        return run(CommandID.Resize, { canvasW, canvasH, devicePixelRatio });
      },
      setZoom(zoom, anchorX, anchorY) {
        return run(CommandID.ZoomSet, {
          zoom,
          hasAnchor: anchorX !== undefined && anchorY !== undefined,
          anchorX,
          anchorY,
        });
      },
      setPan(centerX, centerY) {
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
      setRotation(rotation) {
        return run(CommandID.RotateViewSet, { rotation });
      },
      fitToView() {
        return run(CommandID.FitToView);
      },
      exportProject() {
        return state.handle?.exportProject() ?? null;
      },
      importProject(projectJSON: string) {
        if (!state.handle) {
          return null;
        }
        const render = state.handle.importProject(projectJSON);
        dispatch({ type: "render", render });
        return render;
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
  }, [state.error, state.handle, state.render, state.status]);

  return <EngineContext.Provider value={value}>{children}</EngineContext.Provider>;
}

export function useEngine() {
  const context = useContext(EngineContext);
  if (!context) {
    throw new Error("useEngine must be used inside <EngineProvider>.");
  }

  return context;
}
