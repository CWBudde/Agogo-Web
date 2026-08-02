import { describe, expect, it } from "vitest";
import { parseBrushPresetJSON } from "@/lib/brush-preset-io";

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

  it("round-trips every engine-backed computed tip and dynamics field", () => {
    const [preset] = parseBrushPresetJSON(
      JSON.stringify([
        {
          id: "complete-tip",
          name: "Complete Tip",
          tipShape: "star",
          tipResourceId: "tip-123",
          hardness: 0.76,
          spacing: 0.19,
          angle: -27,
          roundness: 0.42,
          sizeJitter: 0.31,
          opacityJitter: 0.22,
          flowJitter: 0.13,
          controlSource: "fade",
          fadeDabs: 240,
        },
      ]),
    );
    expect(preset).toMatchObject({
      id: "complete-tip",
      tipShape: "star",
      tipResourceId: "tip-123",
      spacing: 0.19,
      angle: -27,
      roundness: 0.42,
      sizeJitter: 0.31,
      opacityJitter: 0.22,
      flowJitter: 0.13,
      controlSource: "fade",
      fadeDabs: 240,
    });
  });
});
