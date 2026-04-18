import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ShapesPanel } from "@/components/shapes-panel";
import { SHAPE_PRESETS } from "@/lib/shape-presets";

describe("ShapesPanel", () => {
  it("shows guidance when the custom shape tool is not active", () => {
    render(
      <ShapesPanel
        active={false}
        presets={SHAPE_PRESETS}
        selectedPresetId={SHAPE_PRESETS[0].id}
        onSelectPreset={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Select the Custom Shape subtool to browse and place reusable vector presets.",
      ),
    ).toBeTruthy();
    expect(screen.getByText(SHAPE_PRESETS[0].name)).toBeTruthy();
  });

  it("filters presets by search and category", () => {
    render(
      <ShapesPanel
        active
        presets={SHAPE_PRESETS}
        selectedPresetId={SHAPE_PRESETS[0].id}
        onSelectPreset={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByTestId("shape-preset-search"), {
      target: { value: "leaf" },
    });

    expect(screen.getByTestId("shape-preset-card-leaf")).toBeTruthy();
    expect(screen.queryByTestId("shape-preset-card-arrow-right")).toBeNull();

    fireEvent.change(screen.getByTestId("shape-preset-search"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByTestId("shape-category-ornaments"));

    expect(screen.getByTestId("shape-preset-card-starburst")).toBeTruthy();
    expect(screen.queryByTestId("shape-preset-card-leaf")).toBeNull();
  });

  it("calls onSelectPreset when a card is clicked", () => {
    const onSelectPreset = vi.fn();

    render(
      <ShapesPanel
        active
        presets={SHAPE_PRESETS}
        selectedPresetId={SHAPE_PRESETS[0].id}
        onSelectPreset={onSelectPreset}
      />,
    );

    fireEvent.click(screen.getByTestId("shape-preset-card-leaf"));

    expect(onSelectPreset).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "leaf",
        name: "Leaf",
      }),
    );
  });
});
