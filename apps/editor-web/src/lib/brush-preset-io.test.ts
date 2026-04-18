import { describe, expect, it } from "vitest";
import { parseAbrBrushPresets, parseBrushPresetJSON } from "@/lib/brush-preset-io";

describe("brush preset import", () => {
  it("parses JSON brush presets", () => {
    const presets = parseBrushPresetJSON(
      JSON.stringify({
        presets: [
          { name: "Imported Square", tipShape: "square", hardness: 0.9, spacing: 0.2, angle: 15 },
        ],
      }),
    );
    expect(presets).toHaveLength(1);
    expect(presets[0]).toMatchObject({
      name: "Imported Square",
      tipShape: "square",
      hardness: 0.9,
      spacing: 0.2,
      angle: 15,
    });
  });

  it("extracts reasonable preset names from abr-like binary content", () => {
    const payload = new TextEncoder().encode("Soft Round\0Marker Line\0ignored\0");
    const presets = parseAbrBrushPresets(payload, "Test Set");
    expect(presets.map((preset) => preset.name)).toEqual(
      expect.arrayContaining(["Soft Round", "Marker Line"]),
    );
  });

  it("falls back to the file name when no names are found", () => {
    const presets = parseAbrBrushPresets(new Uint8Array([0, 1, 2, 3]), "Imported Set");
    expect(presets).toHaveLength(1);
    expect(presets[0].name).toBe("Imported Set");
  });
});
