import { describe, expect, it } from "vitest";
import {
  hexToRgba,
  hsvToRgba,
  isWebSafeColor,
  rgbaToHex,
  rgbaToHsv,
  snapToWebSafeColor,
} from "./color";

describe("color helpers", () => {
  it("round-trips between hex and rgba", () => {
    const color = hexToRgba("#3b82f6");
    expect(color).toEqual([59, 130, 246, 255]);
    expect(rgbaToHex(color ?? [0, 0, 0, 255])).toBe("#3b82f6");
  });

  it("round-trips between hsv and rgba", () => {
    const color = hsvToRgba([210, 0.76, 0.96], 255);
    expect(rgbaToHsv(color)[0]).toBeCloseTo(210, 0);
  });

  it("snaps to the web-safe palette", () => {
    const snapped = snapToWebSafeColor([61, 134, 250, 255]);
    expect(snapped).toEqual([51, 153, 255, 255]);
    expect(isWebSafeColor(snapped)).toBe(true);
  });
});
