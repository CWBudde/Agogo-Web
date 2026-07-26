import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from "react";

/**
 * Filter UI state shared across the Filter menu, the generic filter dialog,
 * the fade dialog, and the Ctrl+F keyboard shortcut.
 *
 * - `activeFilterId` — the filter whose dialog is open (null = none).
 * - `fadeOpen` — the Fade dialog is open.
 * - `lastFilter` — the most recently applied filter, powering "Last Filter"
 *   (reapply) and its menu label. Persists until a different filter runs.
 * - `canFade` — whether Fade is currently available. The engine keeps a
 *   pre-filter snapshot after each apply/reapply and clears it once faded, so
 *   Fade is a one-shot right after applying a filter.
 */
export interface LastFilterRef {
  id: string;
  name: string;
}

export interface FilterState {
  activeFilterId: string | null;
  fadeOpen: boolean;
  lastFilter: LastFilterRef | null;
  canFade: boolean;
}

export const initialFilterState: FilterState = {
  activeFilterId: null,
  fadeOpen: false,
  lastFilter: null,
  canFade: false,
};

export type FilterAction =
  | { type: "open-filter"; id: string }
  | { type: "close-filter" }
  | { type: "open-fade" }
  | { type: "close-fade" }
  | { type: "applied"; id: string; name: string }
  | { type: "faded" };

export function filterReducer(state: FilterState, action: FilterAction): FilterState {
  switch (action.type) {
    case "open-filter":
      return { ...state, activeFilterId: action.id, fadeOpen: false };
    case "close-filter":
      return state.activeFilterId === null ? state : { ...state, activeFilterId: null };
    case "open-fade":
      return { ...state, fadeOpen: true, activeFilterId: null };
    case "close-fade":
      return state.fadeOpen ? { ...state, fadeOpen: false } : state;
    case "applied":
      return { ...state, lastFilter: { id: action.id, name: action.name }, canFade: true };
    case "faded":
      return state.canFade ? { ...state, canFade: false } : state;
    default:
      return state;
  }
}

export interface FilterStateValue extends FilterState {
  openFilter: (id: string) => void;
  closeFilter: () => void;
  openFade: () => void;
  closeFade: () => void;
  /** Record a filter as applied (enables Fade + updates Last Filter). */
  noteFilterApplied: (id: string, name: string) => void;
  /** Record that Fade ran (one-shot: disables Fade until the next apply). */
  noteFaded: () => void;
}

const FilterStateContext = createContext<FilterStateValue | null>(null);

export function FilterStateProvider({ children }: PropsWithChildren) {
  const [state, dispatch] = useReducer(filterReducer, initialFilterState);

  const openFilter = useCallback((id: string) => dispatch({ type: "open-filter", id }), []);
  const closeFilter = useCallback(() => dispatch({ type: "close-filter" }), []);
  const openFade = useCallback(() => dispatch({ type: "open-fade" }), []);
  const closeFade = useCallback(() => dispatch({ type: "close-fade" }), []);
  const noteFilterApplied = useCallback(
    (id: string, name: string) => dispatch({ type: "applied", id, name }),
    [],
  );
  const noteFaded = useCallback(() => dispatch({ type: "faded" }), []);

  const value = useMemo<FilterStateValue>(
    () => ({
      ...state,
      openFilter,
      closeFilter,
      openFade,
      closeFade,
      noteFilterApplied,
      noteFaded,
    }),
    [state, openFilter, closeFilter, openFade, closeFade, noteFilterApplied, noteFaded],
  );

  return <FilterStateContext.Provider value={value}>{children}</FilterStateContext.Provider>;
}

export function useFilterState(): FilterStateValue {
  const value = useContext(FilterStateContext);
  if (!value) {
    throw new Error("useFilterState must be used within a FilterStateProvider");
  }
  return value;
}
