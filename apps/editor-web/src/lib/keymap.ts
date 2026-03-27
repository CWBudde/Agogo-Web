import { CommandID } from "@agogo/proto";

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

export const defaultKeymap = new Map<string, number>([
  ["+", CommandID.ZoomSet],
  ["=", CommandID.ZoomSet],
  ["-", CommandID.ZoomSet],
  ["0", CommandID.FitToView],
  ["Mod+z", CommandID.Undo],
  ["Mod+Shift+z", CommandID.Redo],
  ["Mod+Alt+z", CommandID.Undo],
  [" ", CommandID.PanSet],
]);
