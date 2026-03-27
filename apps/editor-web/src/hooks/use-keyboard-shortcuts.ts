import { useEffect } from "react";
import { defaultKeymap, shortcutKey } from "@/lib/keymap";

type KeyboardActions = {
  onPanModeChange(active: boolean): void;
  onNewDocument(): void;
  onOpenDocument(): void;
  onSaveDocument(): void;
  onExportDocument(): void;
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

      const key = shortcutKey(event);
      switch (key) {
        case "Mod+n":
          event.preventDefault();
          actions.onNewDocument();
          return;
        case "Mod+o":
          event.preventDefault();
          actions.onOpenDocument();
          return;
        case "Mod+s":
          event.preventDefault();
          actions.onSaveDocument();
          return;
        case "Mod+Shift+e":
          event.preventDefault();
          actions.onExportDocument();
          return;
        default:
          break;
      }

      const command = defaultKeymap.get(key);
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
