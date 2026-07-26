import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MenuPreviewPanel } from "./menu-preview";
import type { MenuPreviewMenu } from "./model";

const filterMenu: MenuPreviewMenu = {
  label: "Filter",
  caption: "Effect categories.",
  sections: [
    {
      title: "Recent",
      items: [{ label: "Last Filter", shortcut: "Ctrl+F", actionId: "filter-last" }],
    },
    {
      title: "Blur",
      items: [
        { label: "Gaussian Blur...", filterId: "gaussian-blur" },
        { label: "Box Blur...", filterId: "box-blur" },
      ],
    },
  ],
};

describe("MenuPreviewPanel", () => {
  it("renders section headers so grouped menus are legible", () => {
    render(
      <MenuPreviewPanel
        menu={filterMenu}
        isItemDisabled={() => false}
        onAction={vi.fn()}
        onFilter={vi.fn()}
      />,
    );
    expect(screen.getByText("Blur")).toBeTruthy();
    expect(screen.getByText("Gaussian Blur...")).toBeTruthy();
  });

  it("routes a filter item click to onFilter with its filter id", () => {
    const onFilter = vi.fn();
    render(
      <MenuPreviewPanel
        menu={filterMenu}
        isItemDisabled={() => false}
        onAction={vi.fn()}
        onFilter={onFilter}
      />,
    );
    fireEvent.click(screen.getByText("Gaussian Blur..."));
    expect(onFilter).toHaveBeenCalledWith("gaussian-blur");
  });

  it("routes an actionId item click to onAction", () => {
    const onAction = vi.fn();
    render(
      <MenuPreviewPanel
        menu={filterMenu}
        isItemDisabled={() => false}
        onAction={onAction}
        onFilter={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByText("Last Filter"));
    expect(onAction).toHaveBeenCalledWith("filter-last");
  });

  it("does not fire a disabled filter item", () => {
    const onFilter = vi.fn();
    render(
      <MenuPreviewPanel
        menu={filterMenu}
        isItemDisabled={(item) => item.filterId === "box-blur"}
        onAction={vi.fn()}
        onFilter={onFilter}
      />,
    );
    fireEvent.click(screen.getByText("Box Blur..."));
    expect(onFilter).not.toHaveBeenCalled();
  });
});
