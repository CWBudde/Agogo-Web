import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";

function makeActions() {
  return {
    onPanModeChange: vi.fn(),
    onNewDocument: vi.fn(),
    onOpenDocument: vi.fn(),
    onSaveDocument: vi.fn(),
    onExportDocument: vi.fn(),
    onZoomIn: vi.fn(),
    onZoomOut: vi.fn(),
    onFitToView: vi.fn(),
    onUndo: vi.fn(),
    onRedo: vi.fn(),
    onSelectAll: vi.fn(),
    onDeselect: vi.fn(),
    onInvertSelection: vi.fn(),
    onToolSelect: vi.fn(),
    onBeginTransform: vi.fn(),
    onNudgeLayer: vi.fn(),
    onBrushSizeChange: vi.fn(),
    onBrushHardnessChange: vi.fn(),
    onSwapColors: vi.fn(),
    onResetColors: vi.fn(),
  };
}

function fireKeyDown(key: string, init: KeyboardEventInit = {}, target: EventTarget = window) {
  target.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, ...init }));
}

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
  // Failure-safe: drop any visibilityState override a test installed, even if
  // that test's assertions threw before it could clean up after itself.
  Reflect.deleteProperty(document, "visibilityState");
});

describe("useKeyboardShortcuts", () => {
  it("zooms out on '-' and in on '+'/'='", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    fireKeyDown("-");
    expect(actions.onZoomOut).toHaveBeenCalledTimes(1);
    expect(actions.onZoomIn).not.toHaveBeenCalled();

    fireKeyDown("+");
    fireKeyDown("=");
    expect(actions.onZoomIn).toHaveBeenCalledTimes(2);
    expect(actions.onZoomOut).toHaveBeenCalledTimes(1);
  });

  it("invokes redo on Mod+Shift+z and undo on Mod+Alt+z", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    fireKeyDown("Z", { ctrlKey: true, shiftKey: true });
    expect(actions.onRedo).toHaveBeenCalledTimes(1);
    expect(actions.onUndo).not.toHaveBeenCalled();

    fireKeyDown("z", { ctrlKey: true, altKey: true });
    expect(actions.onUndo).toHaveBeenCalledTimes(1);
  });

  it("activates pan on Space down and releases it on window blur", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    fireKeyDown(" ");
    expect(actions.onPanModeChange).toHaveBeenLastCalledWith(true);

    window.dispatchEvent(new Event("blur"));
    expect(actions.onPanModeChange).toHaveBeenLastCalledWith(false);
  });

  it("releases pan when the document becomes hidden", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    fireKeyDown(" ");
    expect(actions.onPanModeChange).toHaveBeenLastCalledWith(true);

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    document.dispatchEvent(new Event("visibilitychange"));
    expect(actions.onPanModeChange).toHaveBeenLastCalledWith(false);
  });

  it("ignores tool shortcuts while a <select> has focus", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    const select = document.createElement("select");
    document.body.appendChild(select);
    select.focus();

    fireKeyDown("v", {}, select);
    expect(actions.onToolSelect).not.toHaveBeenCalled();
  });

  it("ignores tool shortcuts while a modal dialog is open", () => {
    const actions = makeActions();
    renderHook(() => useKeyboardShortcuts(actions));

    const dialog = document.createElement("div");
    dialog.setAttribute("role", "dialog");
    dialog.setAttribute("aria-modal", "true");
    document.body.appendChild(dialog);

    fireKeyDown("v");
    expect(actions.onToolSelect).not.toHaveBeenCalled();

    dialog.remove();
    fireKeyDown("v");
    expect(actions.onToolSelect).toHaveBeenCalledWith("move");
  });

  it("registers window listeners once even when actions change identity", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const first = makeActions();

    const { rerender } = renderHook(({ actions }) => useKeyboardShortcuts(actions), {
      initialProps: { actions: first },
    });
    const keydownRegistrations = addSpy.mock.calls.filter(([type]) => type === "keydown").length;

    const second = makeActions();
    rerender({ actions: second });

    const keydownAfterRerender = addSpy.mock.calls.filter(([type]) => type === "keydown").length;
    expect(keydownAfterRerender).toBe(keydownRegistrations);

    fireKeyDown("-");
    expect(second.onZoomOut).toHaveBeenCalledTimes(1);
    expect(first.onZoomOut).not.toHaveBeenCalled();
  });

  it("stops handling keys after unmount", () => {
    const actions = makeActions();
    const { unmount } = renderHook(() => useKeyboardShortcuts(actions));

    unmount();
    fireKeyDown("-");
    expect(actions.onZoomOut).not.toHaveBeenCalled();
  });
});
