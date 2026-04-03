import { useEffect } from "react";
import { defaultKeymap, shortcutKey } from "@/lib/keymap";

export type ShortcutTool = "move" | "marquee" | "lasso" | "wand" | "hand" | "zoom" | "crop" | "brush" | "cloneStamp" | "historyBrush" | "pencil" | "eraser" | "fill" | "gradient" | "eyedropper" | "pen" | "directSelect";

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
  onSelectAll(): void;
  onDeselect(): void;
  onInvertSelection(): void;
  onToolSelect(tool: ShortcutTool): void;
  onBeginTransform(): void;
  onNudgeLayer(dx: number, dy: number): void;
  onBrushSizeChange(delta: number): void;
  onBrushHardnessChange(delta: number): void;
  onSwapColors(): void;
  onResetColors(): void;
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
        case "Mod+a":
          event.preventDefault();
          actions.onSelectAll();
          return;
        case "Mod+d":
          event.preventDefault();
          actions.onDeselect();
          return;
        case "Mod+Shift+i":
          event.preventDefault();
          actions.onInvertSelection();
          return;
        case "v":
          event.preventDefault();
          actions.onToolSelect("move");
          return;
        case "m":
          event.preventDefault();
          actions.onToolSelect("marquee");
          return;
        case "l":
          event.preventDefault();
          actions.onToolSelect("lasso");
          return;
        case "w":
          event.preventDefault();
          actions.onToolSelect("wand");
          return;
        case "h":
          event.preventDefault();
          actions.onToolSelect("hand");
          return;
        case "z":
          event.preventDefault();
          actions.onToolSelect("zoom");
          return;
        case "c":
          event.preventDefault();
          actions.onToolSelect("crop");
          return;
        case "b":
          event.preventDefault();
          actions.onToolSelect("brush");
          return;
        case "s":
          event.preventDefault();
          actions.onToolSelect("cloneStamp");
          return;
        case "y":
          event.preventDefault();
          actions.onToolSelect("historyBrush");
          return;
        case "x":
          event.preventDefault();
          actions.onSwapColors();
          return;
        case "d":
          event.preventDefault();
          actions.onResetColors();
          return;
        case "e":
          event.preventDefault();
          actions.onToolSelect("eraser");
          return;
        case "g":
          event.preventDefault();
          actions.onToolSelect("fill");
          return;
        case "i":
          event.preventDefault();
          actions.onToolSelect("eyedropper");
          return;
        case "p":
          event.preventDefault();
          actions.onToolSelect("pen");
          return;
        case "a":
          event.preventDefault();
          actions.onToolSelect("directSelect");
          return;
        case "Mod+t":
          event.preventDefault();
          actions.onBeginTransform();
          return;
        case "ArrowLeft":
          event.preventDefault();
          actions.onNudgeLayer(event.shiftKey ? -10 : -1, 0);
          return;
        case "ArrowRight":
          event.preventDefault();
          actions.onNudgeLayer(event.shiftKey ? 10 : 1, 0);
          return;
        case "ArrowUp":
          event.preventDefault();
          actions.onNudgeLayer(0, event.shiftKey ? -10 : -1);
          return;
        case "ArrowDown":
          event.preventDefault();
          actions.onNudgeLayer(0, event.shiftKey ? 10 : 1);
          return;
        default:
          break;
      }

      // Brush size: [ / ]  (use event.code for layout independence)
      if (event.code === "BracketLeft" && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (event.shiftKey) {
          actions.onBrushHardnessChange(-0.25);
        } else {
          actions.onBrushSizeChange(-1);
        }
        return;
      }
      if (event.code === "BracketRight" && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (event.shiftKey) {
          actions.onBrushHardnessChange(0.25);
        } else {
          actions.onBrushSizeChange(1);
        }
        return;
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
