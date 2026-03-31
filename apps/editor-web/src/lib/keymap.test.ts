import { CommandID } from "@agogo/proto";
import { describe, expect, it } from "vitest";
import { defaultKeymap, shortcutKey } from "@/lib/keymap";

describe("shortcutKey", () => {
  it("normalizes modified printable keys", () => {
    const event = new KeyboardEvent("keydown", {
      key: "Z",
      ctrlKey: true,
      shiftKey: true,
    });

    expect(shortcutKey(event)).toBe("Mod+Shift+z");
  });

  it("preserves non-printable keys and space", () => {
    expect(
      shortcutKey(new KeyboardEvent("keydown", { key: "ArrowLeft" })),
    ).toBe("ArrowLeft");
    expect(shortcutKey(new KeyboardEvent("keydown", { key: " " }))).toBe(" ");
  });
});

describe("defaultKeymap", () => {
  it("contains the core editor shortcuts", () => {
    expect(defaultKeymap.get("Mod+z")).toBe(CommandID.Undo);
    expect(defaultKeymap.get("Mod+Shift+z")).toBe(CommandID.Redo);
    expect(defaultKeymap.get("0")).toBe(CommandID.FitToView);
    expect(defaultKeymap.get(" ")).toBe(CommandID.PanSet);
  });
});
