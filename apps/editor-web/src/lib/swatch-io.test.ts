import { describe, expect, it } from "vitest";
import { exportSwatchesAsAco, parseAcoSwatches, parseSwatchSetJSON } from "@/lib/swatch-io";

describe("swatch io", () => {
  it("parses JSON swatch sets", () => {
    const swatches = parseSwatchSetJSON(JSON.stringify({ swatches: [[255, 0, 0, 255], [0, 0, 255, 255]] }));
    expect(swatches).toEqual([
      [255, 0, 0, 255],
      [0, 0, 255, 255],
    ]);
  });

  it("round-trips aco swatches", () => {
    const source = [
      [255, 0, 0, 255],
      [0, 255, 0, 255],
      [0, 0, 255, 255],
    ] as const;
    const encoded = exportSwatchesAsAco([...source]);
    expect(parseAcoSwatches(encoded)).toEqual([...source]);
  });
});
