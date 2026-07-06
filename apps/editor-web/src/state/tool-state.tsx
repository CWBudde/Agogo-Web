import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
  useContext,
  useMemo,
  useState,
} from "react";
import type { EditorTool } from "@/components/tool-rail-model";

export interface ToolStateValue {
  activeTool: EditorTool;
  setActiveTool: Dispatch<SetStateAction<EditorTool>>;
}

const ToolStateContext = createContext<ToolStateValue | null>(null);

export function ToolStateProvider({ children }: PropsWithChildren) {
  const [activeTool, setActiveTool] = useState<EditorTool>("marquee");

  const value = useMemo<ToolStateValue>(() => ({ activeTool, setActiveTool }), [activeTool]);

  return <ToolStateContext.Provider value={value}>{children}</ToolStateContext.Provider>;
}

export function useToolState() {
  const context = useContext(ToolStateContext);
  if (!context) {
    throw new Error("useToolState must be used inside <ToolStateProvider>.");
  }

  return context;
}
