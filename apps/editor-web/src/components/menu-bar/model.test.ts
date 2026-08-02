import { describe, expect, it } from "vitest";
import { menuItems } from "./model";

describe("menuItems", () => {
  it("contains no actionless placeholder items", () => {
    const actionlessItems = menuItems.flatMap((menu) =>
      menu.sections.flatMap((section) =>
        section.items
          .filter((item) => !item.actionId && !item.filterId)
          .map((item) => `${menu.label} > ${section.title} > ${item.label}`),
      ),
    );

    expect(actionlessItems).toEqual([]);
  });

  it("omits unsupported menu placeholders", () => {
    const menuLabels = menuItems.map((menu) => menu.label);
    const itemLabels = menuItems.flatMap((menu) =>
      menu.sections.flatMap((section) => section.items.map((item) => item.label)),
    );

    expect(menuLabels).not.toContain("Help");
    expect(itemLabels).not.toEqual(
      expect.arrayContaining([
        "Generate Assets",
        "Image Size...",
        "Trim",
        "Scale",
        "Rotate",
        "Skew",
        "Distort",
        "Perspective",
        "Pixel Grid",
        "Rulers",
        "Essentials",
        "Painting",
        "Reset Workspace",
      ]),
    );
    expect(itemLabels).toEqual(expect.arrayContaining(["Cut", "Copy", "Paste"]));
  });
});
