export type ShortcutAction = "zoomIn" | "zoomOut" | "fitToView" | "undo" | "redo" | "panMode";

function normalizeKey(key: string) {
  return key.length === 1 ? key.toLowerCase() : key;
}

export function shortcutKey(event: KeyboardEvent) {
  const parts = [
    event.ctrlKey || event.metaKey ? "Mod" : null,
    event.altKey ? "Alt" : null,
    event.shiftKey ? "Shift" : null,
    normalizeKey(event.key),
  ].filter(Boolean);
  return parts.join("+");
}

export const defaultKeymap = new Map<string, ShortcutAction>([
  ["+", "zoomIn"],
  ["=", "zoomIn"],
  ["-", "zoomOut"],
  ["0", "fitToView"],
  ["Mod+z", "undo"],
  ["Mod+Shift+z", "redo"],
  // Photoshop step-backward
  ["Mod+Alt+z", "undo"],
  [" ", "panMode"],
]);
