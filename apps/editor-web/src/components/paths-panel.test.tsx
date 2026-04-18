import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CommandID } from "@agogo/proto";
import { PathsPanel } from "@/components/paths-panel";
import type { EngineContextValue } from "@/wasm/types";

function createEngine(): EngineContextValue & {
  dispatchCommand: ReturnType<typeof vi.fn>;
} {
  const dispatchCommand = vi.fn(() => null);
  return {
    status: "ready",
    handle: null,
    render: null,
    error: null,
    ready: null,
    dispatchCommand,
    createDocument: vi.fn(() => null),
    createSelection: vi.fn(() => null),
    selectAll: vi.fn(() => null),
    deselect: vi.fn(() => null),
    reselect: vi.fn(() => null),
    invertSelection: vi.fn(() => null),
    magicWand: vi.fn(() => null),
    quickSelect: vi.fn(() => null),
    magneticLassoSuggestPath: vi.fn(() => null),
    pickLayerAtPoint: vi.fn(() => null),
    translateLayer: vi.fn(() => null),
    transformSelection: vi.fn(() => null),
    resizeViewport: vi.fn(() => null),
    setZoom: vi.fn(() => null),
    setPan: vi.fn(() => null),
    dispatchPointerEvent: vi.fn(() => null),
    beginTransaction: vi.fn(() => null),
    endTransaction: vi.fn(() => null),
    jumpHistory: vi.fn(() => null),
    clearHistory: vi.fn(() => null),
    setRotation: vi.fn(() => null),
    fitToView: vi.fn(() => null),
    setShowGuides: vi.fn(() => null),
    exportProject: vi.fn(() => null),
    exportDocument: vi.fn(() => null),
    importProject: vi.fn(() => null),
    undo: vi.fn(() => null),
    redo: vi.fn(() => null),
    reload: vi.fn(),
  };
}

describe("PathsPanel", () => {
  it("dispatches boolean path commands from the footer", () => {
    const engine = createEngine();

    render(
      <PathsPanel
        engine={engine}
        paths={[
          { name: "Shape 1", active: true },
          { name: "Shape 2", active: false },
        ]}
      />,
    );

    fireEvent.click(screen.getByTitle("Combine Paths"));
    fireEvent.click(screen.getByTitle("Subtract Paths"));
    fireEvent.click(screen.getByTitle("Exclude Paths"));
    fireEvent.click(screen.getByTitle("Flatten Paths"));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.PathCombine, {});
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.PathSubtract, {});
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.PathExclude, {});
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.FlattenPath, {});
  });

  it("disables boolean buttons when there are not enough paths", () => {
    const engine = createEngine();

    render(<PathsPanel engine={engine} paths={[{ name: "Shape 1", active: true }]} />);

		expect(screen.getByTitle("Combine Paths").hasAttribute("disabled")).toBe(true);
		expect(screen.getByTitle("Subtract Paths").hasAttribute("disabled")).toBe(true);
		expect(screen.getByTitle("Exclude Paths").hasAttribute("disabled")).toBe(true);
		expect(screen.getByTitle("Flatten Paths").hasAttribute("disabled")).toBe(true);
	});
});
