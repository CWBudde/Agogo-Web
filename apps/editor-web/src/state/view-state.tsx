import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  type SetStateAction,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { AuxPanel } from "@/components/dock-section";
import type { DocumentUnit } from "@/lib/units";
import { useUiMeta } from "@/wasm/use-engine-render";

export type CursorPosition = { x: number; y: number } | null;

export interface CursorStateValue {
  cursor: CursorPosition;
  setCursor: Dispatch<SetStateAction<CursorPosition>>;
}

export interface ViewStateValue {
  isPanMode: boolean;
  setIsPanMode: Dispatch<SetStateAction<boolean>>;
  showGuides: boolean;
  setShowGuides: Dispatch<SetStateAction<boolean>>;
  panelCollapsed: boolean;
  setPanelCollapsed: Dispatch<SetStateAction<boolean>>;
  panelWidth: number;
  setPanelWidth: Dispatch<SetStateAction<number>>;
  documentUnit: DocumentUnit;
  setDocumentUnit: Dispatch<SetStateAction<DocumentUnit>>;
  activeAuxPanel: AuxPanel;
  setActiveAuxPanel: Dispatch<SetStateAction<AuxPanel>>;
  selectedLayerIds: string[];
  setSelectedLayerIds: Dispatch<SetStateAction<string[]>>;
}

// Cursor lives in its own context so high-frequency pointer-move updates
// re-render only cursor consumers, not everything reading view state.
const CursorStateContext = createContext<CursorStateValue | null>(null);
const ViewStateContext = createContext<ViewStateValue | null>(null);

export function ViewStateProvider({ children }: PropsWithChildren) {
  const activeLayerId = useUiMeta((meta) => meta?.activeLayerId ?? null);

  const [cursor, setCursor] = useState<CursorPosition>(null);
  const [isPanMode, setIsPanMode] = useState(false);
  const [showGuides, setShowGuides] = useState(false);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [panelWidth, setPanelWidth] = useState(328);
  const [documentUnit, setDocumentUnit] = useState<DocumentUnit>("px");
  const [activeAuxPanel, setActiveAuxPanel] = useState<AuxPanel>("properties");
  const [selectedLayerIds, setSelectedLayerIds] = useState<string[]>([]);

  // Keep the multi-selection in sync with the engine's active layer.
  useEffect(() => {
    if (!activeLayerId) {
      setSelectedLayerIds([]);
      return;
    }
    setSelectedLayerIds((current) =>
      current.length > 0 && current.includes(activeLayerId) ? current : [activeLayerId],
    );
  }, [activeLayerId]);

  const cursorValue = useMemo<CursorStateValue>(() => ({ cursor, setCursor }), [cursor]);

  const value = useMemo<ViewStateValue>(
    () => ({
      isPanMode,
      setIsPanMode,
      showGuides,
      setShowGuides,
      panelCollapsed,
      setPanelCollapsed,
      panelWidth,
      setPanelWidth,
      documentUnit,
      setDocumentUnit,
      activeAuxPanel,
      setActiveAuxPanel,
      selectedLayerIds,
      setSelectedLayerIds,
    }),
    [
      isPanMode,
      showGuides,
      panelCollapsed,
      panelWidth,
      documentUnit,
      activeAuxPanel,
      selectedLayerIds,
    ],
  );

  return (
    <CursorStateContext.Provider value={cursorValue}>
      <ViewStateContext.Provider value={value}>{children}</ViewStateContext.Provider>
    </CursorStateContext.Provider>
  );
}

export function useViewState() {
  const context = useContext(ViewStateContext);
  if (!context) {
    throw new Error("useViewState must be used inside <ViewStateProvider>.");
  }

  return context;
}

export function useCursorState() {
  const context = useContext(CursorStateContext);
  if (!context) {
    throw new Error("useCursorState must be used inside <ViewStateProvider>.");
  }

  return context;
}
