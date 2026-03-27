import { useEffect } from "react";
import { defaultKeymap, shortcutKey } from "@/lib/keymap";

type KeyboardActions = {
  onPanModeChange(active: boolean): void;
  onZoomIn(): void;
  onZoomOut(): void;
  onFitToView(): void;
  onUndo(): void;
  onRedo(): void;
};

function isEditableTarget(target: EventTarget | null) {
  const element = target as HTMLElement | null;
  if (!element) {
    return false;
  }
  return (
    element instanceof HTMLInputElement ||
    element instanceof HTMLTextAreaElement ||
    element.isContentEditable
  );
}

export function useKeyboardShortcuts(actions: KeyboardActions) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (isEditableTarget(event.target) && event.key !== "Escape") {
        return;
      }

      const command = defaultKeymap.get(shortcutKey(event));
      switch (command) {
        case defaultKeymap.get(" "):
          event.preventDefault();
          actions.onPanModeChange(true);
          break;
        case defaultKeymap.get("+"):
          event.preventDefault();
          actions.onZoomIn();
          break;
        case defaultKeymap.get("="):
          event.preventDefault();
          actions.onZoomIn();
          break;
        case defaultKeymap.get("-"):
          event.preventDefault();
          actions.onZoomOut();
          break;
        case defaultKeymap.get("0"):
          event.preventDefault();
          actions.onFitToView();
          break;
        case defaultKeymap.get("Mod+z"):
          event.preventDefault();
          actions.onUndo();
          break;
        case defaultKeymap.get("Mod+Shift+z"):
          event.preventDefault();
          actions.onRedo();
          break;
        case defaultKeymap.get("Mod+Alt+z"):
          event.preventDefault();
          actions.onUndo();
          break;
        default:
          break;
      }
    };

    const handleKeyUp = (event: KeyboardEvent) => {
      if (event.key === " ") {
        actions.onPanModeChange(false);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("keyup", handleKeyUp);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("keyup", handleKeyUp);
    };
  }, [actions]);
}
