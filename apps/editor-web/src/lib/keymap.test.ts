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
    expect(shortcutKey(new KeyboardEvent("keydown", { key: "ArrowLeft" }))).toBe("ArrowLeft");
    expect(shortcutKey(new KeyboardEvent("keydown", { key: " " }))).toBe(" ");
  });
});

describe("defaultKeymap", () => {
  it("maps zoom keys to distinct actions", () => {
    expect(defaultKeymap.get("+")).toBe("zoomIn");
    expect(defaultKeymap.get("=")).toBe("zoomIn");
    expect(defaultKeymap.get("-")).toBe("zoomOut");
    expect(defaultKeymap.get("0")).toBe("fitToView");
  });

  it("maps history and pan shortcuts", () => {
    expect(defaultKeymap.get("Mod+z")).toBe("undo");
    expect(defaultKeymap.get("Mod+Shift+z")).toBe("redo");
    expect(defaultKeymap.get("Mod+Alt+z")).toBe("undo");
    expect(defaultKeymap.get(" ")).toBe("panMode");
  });
});
