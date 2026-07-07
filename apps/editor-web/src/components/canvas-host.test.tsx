import { fireEvent, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppStateProvider } from "@/state/app-state";
import { useBrushState } from "@/state/brush-state";
import { useDialogState } from "@/state/dialog-state";
import type { EngineContextValue } from "@/wasm/types";

// Route useEngine() to a stable per-test stub. The domain providers also call
// useEngine() for their effects; handle=null keeps those effects inert.
const { engineRef, canvasSpy } = vi.hoisted(() => ({
  engineRef: { current: null as unknown as EngineContextValue },
  canvasSpy: { count: 0, lastProps: null as Record<string, unknown> | null },
}));

vi.mock("@/wasm/context", () => ({
  useEngine: () => engineRef.current,
}));

// Replace EditorCanvas with a render-counting stub that captures its props.
vi.mock("@/components/editor-canvas", () => ({
  EditorCanvas: (props: Record<string, unknown>) => {
    canvasSpy.count += 1;
    canvasSpy.lastProps = props;
    return <div data-testid="canvas-stub" />;
  },
}));

import { CanvasHost } from "@/components/canvas-host";

function makeEngine(): EngineContextValue {
  return {
    status: "ready",
    handle: null,
    render: null,
    error: null,
    dispatchCommand: vi.fn(() => null),
  } as unknown as EngineContextValue;
}

// Sibling control that drives domain state changes from inside the providers.
function Controls() {
  const { setBrushSize } = useBrushState();
  const { setNewDocumentOpen } = useDialogState();
  return (
    <>
      <button type="button" data-testid="bump-brush" onClick={() => setBrushSize((s) => s + 1)}>
        brush
      </button>
      <button type="button" data-testid="open-dialog" onClick={() => setNewDocumentOpen(true)}>
        dialog
      </button>
    </>
  );
}

describe("CanvasHost", () => {
  beforeEach(() => {
    engineRef.current = makeEngine();
    canvasSpy.count = 0;
    canvasSpy.lastProps = null;
  });

  it("memoizes the canvas against unrelated state changes but re-renders on canvas-prop changes", () => {
    const { getByTestId } = render(
      <AppStateProvider>
        <Controls />
        <CanvasHost />
      </AppStateProvider>,
    );

    // Baseline after the initial mount.
    const baseline = canvasSpy.count;
    expect(baseline).toBeGreaterThan(0);
    const initialBrushSize = canvasSpy.lastProps?.brushSize as number;
    const selectionOptionsRef = canvasSpy.lastProps?.selectionOptions;

    // Opening a dialog (useDialogState) must NOT re-render the memoized canvas.
    fireEvent.click(getByTestId("open-dialog"));
    expect(canvasSpy.count).toBe(baseline);

    // Changing brushSize MUST re-render the canvas exactly once with the value.
    fireEvent.click(getByTestId("bump-brush"));
    expect(canvasSpy.count).toBe(baseline + 1);
    expect(canvasSpy.lastProps?.brushSize).toBe(initialBrushSize + 1);

    // The selectionOptions bundle keeps a stable reference across the unrelated
    // brushSize update (useMemo with selection-only deps).
    expect(canvasSpy.lastProps?.selectionOptions).toBe(selectionOptionsRef);
  });
});
