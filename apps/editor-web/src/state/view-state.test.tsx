import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";
import { useCursorState, useViewState, ViewStateProvider } from "@/state/view-state";

vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (selector: (m: unknown) => unknown) => selector(null),
}));

function wrapper({ children }: PropsWithChildren) {
  return <ViewStateProvider>{children}</ViewStateProvider>;
}

describe("ViewStateProvider", () => {
  it("throws when hooks are used outside the provider", () => {
    expect(() => renderHook(() => useViewState())).toThrow(
      "useViewState must be used inside <ViewStateProvider>.",
    );
    expect(() => renderHook(() => useCursorState())).toThrow(
      "useCursorState must be used inside <ViewStateProvider>.",
    );
  });

  it("keeps the view-state value identity stable across cursor moves", () => {
    const { result } = renderHook(() => ({ view: useViewState(), cursor: useCursorState() }), {
      wrapper,
    });
    const viewBefore = result.current.view;

    act(() => {
      result.current.cursor.setCursor({ x: 10, y: 20 });
    });
    expect(result.current.cursor.cursor).toEqual({ x: 10, y: 20 });
    // Cursor updates must not invalidate the main view context value —
    // pointer-move should re-render only cursor consumers.
    expect(result.current.view).toBe(viewBefore);

    act(() => {
      result.current.view.setPanelWidth(400);
    });
    expect(result.current.view).not.toBe(viewBefore);
    expect(result.current.view.panelWidth).toBe(400);
  });
});
