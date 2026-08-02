import { fireEvent, render, screen } from "@testing-library/react";
import { CommandID, type LayerNodeMeta } from "@agogo/proto";
import { describe, expect, it, vi } from "vitest";
import { CharacterPanel } from "@/components/character-panel";
import { VectorPropertiesPanel } from "@/components/vector-properties-panel";
import type { EngineContextValue } from "@/wasm/types";

function makeEngine() {
  return {
    beginTransaction: vi.fn(),
    endTransaction: vi.fn(),
    dispatchCommand: vi.fn(),
  } as unknown as EngineContextValue;
}

function layer(layerType: LayerNodeMeta["layerType"], overrides = {}): LayerNodeMeta {
  return {
    id: `${layerType}-1`,
    name: "Layer",
    layerType,
    visible: true,
    lockMode: "none",
    opacity: 1,
    fillOpacity: 1,
    blendMode: "normal",
    clipToBelow: false,
    clippingBase: false,
    hasMask: false,
    maskEnabled: true,
    hasVectorMask: false,
    ...overrides,
  };
}

function colorInput(label: string): HTMLInputElement {
  const labelElement = screen.getByText(label).closest("label");
  const input = labelElement?.querySelector("input");
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`Missing ${label} input`);
  }
  return input;
}

describe("contextual text color", () => {
  it("previews with the original alpha and commits one transaction", () => {
    const engine = makeEngine();
    const textLayer = layer("text", { textColor: [8, 16, 24, 77] });
    render(<CharacterPanel engine={engine} layer={textLayer} availableFonts={[]} />);

    fireEvent.click(screen.getByRole("button", { name: "Text color" }));
    expect(screen.getByRole("dialog", { name: "Text Color" })).not.toBeNull();
    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.beginTransaction).toHaveBeenCalledWith("Change text color");
    expect(colorInput("Alpha").value).toBe("77");

    fireEvent.change(colorInput("Hex"), { target: { value: "#123456" } });
    expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetTextStyle, {
      layerId: "text-1",
      color: [18, 52, 86, 77],
    });

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(engine.endTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(true);
  });

  it("reverts on Escape and restores focus to the invoking swatch", () => {
    const engine = makeEngine();
    const textLayer = layer("text", { textColor: [8, 16, 24, 77] });
    render(<CharacterPanel engine={engine} layer={textLayer} availableFonts={[]} />);
    const swatch = screen.getByRole("button", { name: "Text color" });
    swatch.focus();
    fireEvent.click(swatch);

    const dialog = screen.getByRole("dialog", { name: "Text Color" });
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    fireEvent.keyDown(dialog, { key: "Escape" });

    expect(engine.endTransaction).toHaveBeenCalledWith(false);
    expect(screen.queryByRole("dialog", { name: "Text Color" })).toBeNull();
    expect(document.activeElement).toBe(swatch);
  });
});

describe("contextual vector colors", () => {
  it("previews fill and explicit None without changing stroke or width", () => {
    const engine = makeEngine();
    const vectorLayer = layer("vector", {
      fillColor: [10, 20, 30, 40],
      strokeColor: [50, 60, 70, 80],
      strokeWidth: 7,
    });
    render(<VectorPropertiesPanel engine={engine} layer={vectorLayer} />);

    fireEvent.click(screen.getByRole("button", { name: "Fill color" }));
    fireEvent.change(colorInput("Hex"), { target: { value: "#abcdef" } });
    expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetVectorLayerStyle, {
      layerId: "vector-1",
      fillColor: [171, 205, 239, 40],
      strokeColor: [50, 60, 70, 80],
      strokeWidth: 7,
    });

    fireEvent.click(screen.getByRole("button", { name: "None" }));
    expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetVectorLayerStyle, {
      layerId: "vector-1",
      fillColor: [171, 205, 239, 0],
      strokeColor: [50, 60, 70, 80],
      strokeWidth: 7,
    });

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(engine.beginTransaction).toHaveBeenCalledTimes(1);
    expect(engine.endTransaction).toHaveBeenCalledWith(false);
  });

  it("previews stroke without changing fill or width and applies the transaction", () => {
    const engine = makeEngine();
    const vectorLayer = layer("vector", {
      fillColor: [10, 20, 30, 40],
      strokeColor: [50, 60, 70, 80],
      strokeWidth: 7,
    });
    render(<VectorPropertiesPanel engine={engine} layer={vectorLayer} />);

    fireEvent.click(screen.getByRole("button", { name: "Stroke color" }));
    fireEvent.change(colorInput("Alpha"), { target: { value: "91" } });
    expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetVectorLayerStyle, {
      layerId: "vector-1",
      fillColor: [10, 20, 30, 40],
      strokeColor: [50, 60, 70, 91],
      strokeWidth: 7,
    });

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(engine.beginTransaction).toHaveBeenCalledWith("Change vector stroke");
    expect(engine.endTransaction).toHaveBeenCalledWith(true);
  });
});
