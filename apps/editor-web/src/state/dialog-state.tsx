import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from "react";

/**
 * Open FLAGS for App-level dialogs, held in a single reducer keyed by dialog.
 * Dialog DRAFT values (slider positions, names, matrices, ...) intentionally
 * stay outside this provider — they move into the dialog components in B5.
 *
 * The flags are independent booleans (NOT a single activeDialog value):
 * overlap semantics exist, e.g. Select & Mask on top of other UI.
 */
export type DialogKey =
  | "newDocument"
  | "canvasSize"
  | "openRecent"
  | "export"
  | "feather"
  | "colorRange"
  | "saveSelection"
  | "loadSelection"
  | "selectAndMask"
  | "threshold"
  | "posterize"
  | "channelMixer"
  | "selectiveColor"
  | "photoFilter"
  | "gradientMap";

export type DialogFlags = Record<DialogKey, boolean>;

export type DialogAction =
  | { type: "open"; key: DialogKey }
  | { type: "close"; key: DialogKey }
  | { type: "set"; key: DialogKey; open: boolean };

export const initialDialogFlags: DialogFlags = {
  newDocument: false,
  canvasSize: false,
  openRecent: false,
  export: false,
  feather: false,
  colorRange: false,
  saveSelection: false,
  loadSelection: false,
  selectAndMask: false,
  threshold: false,
  posterize: false,
  channelMixer: false,
  selectiveColor: false,
  photoFilter: false,
  gradientMap: false,
};

export function dialogFlagsReducer(state: DialogFlags, action: DialogAction): DialogFlags {
  const open = action.type === "set" ? action.open : action.type === "open";
  if (state[action.key] === open) {
    return state;
  }
  return { ...state, [action.key]: open };
}

export interface DialogStateValue {
  dialogs: DialogFlags;
  openDialog: (key: DialogKey) => void;
  closeDialog: (key: DialogKey) => void;
  setDialogOpen: (key: DialogKey, open: boolean) => void;
  // Per-dialog accessors matching the former App.tsx useState names, so call
  // sites stay untouched. All setters are identity-stable.
  newDocumentOpen: boolean;
  setNewDocumentOpen: (open: boolean) => void;
  canvasSizeOpen: boolean;
  setCanvasSizeOpen: (open: boolean) => void;
  openRecentOpen: boolean;
  setOpenRecentOpen: (open: boolean) => void;
  exportDialogOpen: boolean;
  setExportDialogOpen: (open: boolean) => void;
  featherDialogOpen: boolean;
  setFeatherDialogOpen: (open: boolean) => void;
  colorRangeOpen: boolean;
  setColorRangeOpen: (open: boolean) => void;
  saveSelectionOpen: boolean;
  setSaveSelectionOpen: (open: boolean) => void;
  loadSelectionOpen: boolean;
  setLoadSelectionOpen: (open: boolean) => void;
  selectAndMaskOpen: boolean;
  setSelectAndMaskOpen: (open: boolean) => void;
  thresholdDialogOpen: boolean;
  setThresholdDialogOpen: (open: boolean) => void;
  posterizeDialogOpen: boolean;
  setPosterizeDialogOpen: (open: boolean) => void;
  channelMixerDialogOpen: boolean;
  setChannelMixerDialogOpen: (open: boolean) => void;
  selectiveColorDialogOpen: boolean;
  setSelectiveColorDialogOpen: (open: boolean) => void;
  photoFilterDialogOpen: boolean;
  setPhotoFilterDialogOpen: (open: boolean) => void;
  gradientMapDialogOpen: boolean;
  setGradientMapDialogOpen: (open: boolean) => void;
}

const DialogStateContext = createContext<DialogStateValue | null>(null);

export function DialogStateProvider({ children }: PropsWithChildren) {
  const [dialogs, dispatch] = useReducer(dialogFlagsReducer, initialDialogFlags);

  const openDialog = useCallback((key: DialogKey) => dispatch({ type: "open", key }), []);
  const closeDialog = useCallback((key: DialogKey) => dispatch({ type: "close", key }), []);
  const setDialogOpen = useCallback(
    (key: DialogKey, open: boolean) => dispatch({ type: "set", key, open }),
    [],
  );

  // dispatch is identity-stable, so this memo is created exactly once.
  const setters = useMemo(
    () => ({
      setNewDocumentOpen: (open: boolean) => dispatch({ type: "set", key: "newDocument", open }),
      setCanvasSizeOpen: (open: boolean) => dispatch({ type: "set", key: "canvasSize", open }),
      setOpenRecentOpen: (open: boolean) => dispatch({ type: "set", key: "openRecent", open }),
      setExportDialogOpen: (open: boolean) => dispatch({ type: "set", key: "export", open }),
      setFeatherDialogOpen: (open: boolean) => dispatch({ type: "set", key: "feather", open }),
      setColorRangeOpen: (open: boolean) => dispatch({ type: "set", key: "colorRange", open }),
      setSaveSelectionOpen: (open: boolean) =>
        dispatch({ type: "set", key: "saveSelection", open }),
      setLoadSelectionOpen: (open: boolean) =>
        dispatch({ type: "set", key: "loadSelection", open }),
      setSelectAndMaskOpen: (open: boolean) =>
        dispatch({ type: "set", key: "selectAndMask", open }),
      setThresholdDialogOpen: (open: boolean) => dispatch({ type: "set", key: "threshold", open }),
      setPosterizeDialogOpen: (open: boolean) => dispatch({ type: "set", key: "posterize", open }),
      setChannelMixerDialogOpen: (open: boolean) =>
        dispatch({ type: "set", key: "channelMixer", open }),
      setSelectiveColorDialogOpen: (open: boolean) =>
        dispatch({ type: "set", key: "selectiveColor", open }),
      setPhotoFilterDialogOpen: (open: boolean) =>
        dispatch({ type: "set", key: "photoFilter", open }),
      setGradientMapDialogOpen: (open: boolean) =>
        dispatch({ type: "set", key: "gradientMap", open }),
    }),
    [],
  );

  const value = useMemo<DialogStateValue>(
    () => ({
      dialogs,
      openDialog,
      closeDialog,
      setDialogOpen,
      newDocumentOpen: dialogs.newDocument,
      canvasSizeOpen: dialogs.canvasSize,
      openRecentOpen: dialogs.openRecent,
      exportDialogOpen: dialogs.export,
      featherDialogOpen: dialogs.feather,
      colorRangeOpen: dialogs.colorRange,
      saveSelectionOpen: dialogs.saveSelection,
      loadSelectionOpen: dialogs.loadSelection,
      selectAndMaskOpen: dialogs.selectAndMask,
      thresholdDialogOpen: dialogs.threshold,
      posterizeDialogOpen: dialogs.posterize,
      channelMixerDialogOpen: dialogs.channelMixer,
      selectiveColorDialogOpen: dialogs.selectiveColor,
      photoFilterDialogOpen: dialogs.photoFilter,
      gradientMapDialogOpen: dialogs.gradientMap,
      ...setters,
    }),
    [dialogs, openDialog, closeDialog, setDialogOpen, setters],
  );

  return <DialogStateContext.Provider value={value}>{children}</DialogStateContext.Provider>;
}

export function useDialogState() {
  const context = useContext(DialogStateContext);
  if (!context) {
    throw new Error("useDialogState must be used inside <DialogStateProvider>.");
  }

  return context;
}
