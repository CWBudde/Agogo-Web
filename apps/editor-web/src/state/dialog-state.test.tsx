import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { describe, expect, it } from "vitest";
import {
  DialogStateProvider,
  dialogFlagsReducer,
  initialDialogFlags,
  useDialogState,
} from "@/state/dialog-state";

function wrapper({ children }: PropsWithChildren) {
  return <DialogStateProvider>{children}</DialogStateProvider>;
}

describe("dialogFlagsReducer", () => {
  it("opens and closes dialogs by key", () => {
    let state = dialogFlagsReducer(initialDialogFlags, { type: "open", key: "newDocument" });
    expect(state.newDocument).toBe(true);

    state = dialogFlagsReducer(state, { type: "close", key: "newDocument" });
    expect(state.newDocument).toBe(false);

    state = dialogFlagsReducer(state, { type: "set", key: "export", open: true });
    expect(state.export).toBe(true);
  });

  it("keeps independent flags so dialogs can overlap", () => {
    let state = dialogFlagsReducer(initialDialogFlags, { type: "open", key: "selectAndMask" });
    state = dialogFlagsReducer(state, { type: "open", key: "feather" });
    expect(state.selectAndMask).toBe(true);
    expect(state.feather).toBe(true);

    state = dialogFlagsReducer(state, { type: "close", key: "feather" });
    expect(state.selectAndMask).toBe(true);
    expect(state.feather).toBe(false);
  });

  it("returns the same state object for no-op transitions", () => {
    expect(dialogFlagsReducer(initialDialogFlags, { type: "close", key: "threshold" })).toBe(
      initialDialogFlags,
    );
    const opened = dialogFlagsReducer(initialDialogFlags, { type: "open", key: "threshold" });
    expect(dialogFlagsReducer(opened, { type: "set", key: "threshold", open: true })).toBe(opened);
  });
});

describe("DialogStateProvider", () => {
  it("throws when used outside the provider", () => {
    expect(() => renderHook(() => useDialogState())).toThrow(
      "useDialogState must be used inside <DialogStateProvider>.",
    );
  });

  it("exposes legacy-named flags and stable boolean setters", () => {
    const { result } = renderHook(() => useDialogState(), { wrapper });
    const setter = result.current.setNewDocumentOpen;
    expect(result.current.newDocumentOpen).toBe(false);

    act(() => {
      result.current.setNewDocumentOpen(true);
    });
    expect(result.current.newDocumentOpen).toBe(true);
    expect(result.current.dialogs.newDocument).toBe(true);
    // Setter identity must be stable across state changes.
    expect(result.current.setNewDocumentOpen).toBe(setter);

    act(() => {
      result.current.closeDialog("newDocument");
    });
    expect(result.current.newDocumentOpen).toBe(false);
  });
});
