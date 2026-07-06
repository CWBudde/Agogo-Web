import { describe, expect, it } from "vitest";
import { formatDimension, pixelsToUnit, unitSteps, unitToPixels } from "./units";

describe("units helpers", () => {
  it("converts pixels to inches and back at 72 DPI", () => {
    expect(pixelsToUnit(144, 72, "in")).toBeCloseTo(2, 10);
    expect(unitToPixels(pixelsToUnit(144, 72, "in"), 72, "in")).toBeCloseTo(144, 10);
  });

  it("converts pixels to centimeters and back at 300 DPI", () => {
    expect(pixelsToUnit(300, 300, "cm")).toBeCloseTo(2.54, 10);
    expect(unitToPixels(pixelsToUnit(1234, 300, "cm"), 300, "cm")).toBeCloseTo(1234, 10);
  });

  it("converts pixels to millimeters and back at 300 DPI", () => {
    expect(pixelsToUnit(300, 300, "mm")).toBeCloseTo(25.4, 10);
    expect(unitToPixels(pixelsToUnit(600, 300, "mm"), 300, "mm")).toBeCloseTo(600, 10);
  });

  it("returns pixels unchanged for the px unit", () => {
    expect(pixelsToUnit(512, 72, "px")).toBe(512);
    expect(unitToPixels(512, 300, "px")).toBe(512);
  });

  it("formats px and mm as rounded integers", () => {
    expect(formatDimension(511.6, "px")).toBe("512");
    expect(formatDimension(25.4, "mm")).toBe("25");
  });

  it("formats in and cm with two decimals", () => {
    expect(formatDimension(2, "in")).toBe("2.00");
    expect(formatDimension(2.546, "cm")).toBe("2.55");
  });

  it("exposes a step per unit", () => {
    expect(unitSteps.px).toBe(1);
    expect(unitSteps.in).toBe(0.01);
    expect(unitSteps.cm).toBe(0.1);
    expect(unitSteps.mm).toBe(1);
  });
});
