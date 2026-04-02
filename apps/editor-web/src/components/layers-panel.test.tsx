import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LayerPropertiesDialog } from "@/components/layers-panel";
import type { LayerNodeMeta } from "@agogo/proto";

function makeLayer(id: string, name: string): LayerNodeMeta {
  return {
    id,
    name,
    layerType: "adjustment",
    adjustmentKind: "levels",
    params: {},
    visible: true,
    lockMode: "none",
    opacity: 1,
    fillOpacity: 1,
    blendMode: "normal",
    clipToBelow: false,
    clippingBase: false,
    hasMask: false,
    maskEnabled: false,
    hasVectorMask: false,
  };
}

describe("LayerPropertiesDialog", () => {
  it("syncs its draft state when the selected layer changes", () => {
    const onRename = vi.fn();
    const onColorTag = vi.fn();
    const onClose = vi.fn();

    const { rerender } = render(
      <LayerPropertiesDialog
        layer={makeLayer("layer-1", "Levels 1")}
        colorTag="none"
        onRename={onRename}
        onColorTag={onColorTag}
        onClose={onClose}
      />,
    );

    const input = screen.getByLabelText("Name") as HTMLInputElement;
    expect(input.value).toBe("Levels 1");

    fireEvent.change(input, { target: { value: "Edited Name" } });
    expect(input.value).toBe("Edited Name");

    rerender(
      <LayerPropertiesDialog
        layer={makeLayer("layer-2", "Photo Filter 1")}
        colorTag="none"
        onRename={onRename}
        onColorTag={onColorTag}
        onClose={onClose}
      />,
    );

    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Photo Filter 1");
  });
});
