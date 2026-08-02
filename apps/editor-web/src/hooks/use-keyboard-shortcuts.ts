import { useEffect, useRef } from "react";
import { defaultKeymap, shortcutKey } from "@/lib/keymap";

export type ShortcutTool =
  | "move"
  | "marquee"
  | "lasso"
  | "wand"
  | "hand"
  | "zoom"
  | "crop"
  | "brush"
  | "cloneStamp"
  | "historyBrush"
  | "pencil"
  | "eraser"
  | "fill"
  | "gradient"
  | "eyedropper"
  | "pen"
  | "directSelect";

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
  onCut(): void;
  onCopy(): void;
  onPaste(): void;
  onFill(): void;
  onCanvasSize(): void;
  onSelectAll(): void;
  onDeselect(): void;
  onReselect(): void;
  onInvertSelection(): void;
  onNewLayer(): void;
  onDuplicateLayer(): void;
  onMergeDown(): void;
  onToolSelect(tool: ShortcutTool): void;
  onBeginTransform(): void;
  onTransformAgain(): void;
  onNudgeLayer(dx: number, dy: number): void;
  onBrushSizeChange(delta: number): void;
  onBrushHardnessChange(delta: number): void;
  onSwapColors(): void;
  onResetColors(): void;
  onReapplyFilter(): void;
  onFadeFilter(): void;
};

function isEditableTarget(target: EventTarget | null) {
  const element = target as HTMLElement | null;
  if (!element) {
    return false;
  }
  return (
    element instanceof HTMLInputElement ||
    element instanceof HTMLTextAreaElement ||
    element instanceof HTMLSelectElement ||
    element.isContentEditable
  );
}

function hasOpenModal() {
  return document.querySelector('[role="dialog"][aria-modal="true"]') !== null;
}

export function useKeyboardShortcuts(actions: KeyboardActions) {
  const actionsRef = useRef(actions);
  useEffect(() => {
    actionsRef.current = actions;
  });

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const currentActions = actionsRef.current;
      if ((isEditableTarget(event.target) || hasOpenModal()) && event.key !== "Escape") {
        return;
      }

      const key = shortcutKey(event);
      switch (key) {
        case "Mod+n":
          event.preventDefault();
          currentActions.onNewDocument();
          return;
        case "Mod+o":
          event.preventDefault();
          currentActions.onOpenDocument();
          return;
        case "Mod+s":
          event.preventDefault();
          currentActions.onSaveDocument();
          return;
        case "Mod+Shift+e":
          event.preventDefault();
          currentActions.onExportDocument();
          return;
        case "Mod+x":
          event.preventDefault();
          currentActions.onCut();
          return;
        case "Mod+c":
          event.preventDefault();
          currentActions.onCopy();
          return;
        case "Mod+v":
          event.preventDefault();
          currentActions.onPaste();
          return;
        case "Shift+F5":
          event.preventDefault();
          currentActions.onFill();
          return;
        case "Mod+Alt+c":
          event.preventDefault();
          currentActions.onCanvasSize();
          return;
        case "Mod+Shift+n":
          event.preventDefault();
          currentActions.onNewLayer();
          return;
        case "Mod+j":
          event.preventDefault();
          currentActions.onDuplicateLayer();
          return;
        case "Mod+e":
          event.preventDefault();
          currentActions.onMergeDown();
          return;
        case "Mod+a":
          event.preventDefault();
          currentActions.onSelectAll();
          return;
        case "Mod+d":
          event.preventDefault();
          currentActions.onDeselect();
          return;
        case "Mod+Shift+d":
          event.preventDefault();
          currentActions.onReselect();
          return;
        case "Mod+Shift+i":
          event.preventDefault();
          currentActions.onInvertSelection();
          return;
        case "v":
          event.preventDefault();
          currentActions.onToolSelect("move");
          return;
        case "m":
          event.preventDefault();
          currentActions.onToolSelect("marquee");
          return;
        case "l":
          event.preventDefault();
          currentActions.onToolSelect("lasso");
          return;
        case "w":
          event.preventDefault();
          currentActions.onToolSelect("wand");
          return;
        case "h":
          event.preventDefault();
          currentActions.onToolSelect("hand");
          return;
        case "z":
          event.preventDefault();
          currentActions.onToolSelect("zoom");
          return;
        case "c":
          event.preventDefault();
          currentActions.onToolSelect("crop");
          return;
        case "b":
          event.preventDefault();
          currentActions.onToolSelect("brush");
          return;
        case "s":
          event.preventDefault();
          currentActions.onToolSelect("cloneStamp");
          return;
        case "y":
          event.preventDefault();
          currentActions.onToolSelect("historyBrush");
          return;
        case "x":
          event.preventDefault();
          currentActions.onSwapColors();
          return;
        case "d":
          event.preventDefault();
          currentActions.onResetColors();
          return;
        case "e":
          event.preventDefault();
          currentActions.onToolSelect("eraser");
          return;
        case "g":
          event.preventDefault();
          currentActions.onToolSelect("fill");
          return;
        case "i":
          event.preventDefault();
          currentActions.onToolSelect("eyedropper");
          return;
        case "p":
          event.preventDefault();
          currentActions.onToolSelect("pen");
          return;
        case "a":
          event.preventDefault();
          currentActions.onToolSelect("directSelect");
          return;
        case "Mod+t":
          event.preventDefault();
          currentActions.onBeginTransform();
          return;
        case "Mod+Shift+t":
          event.preventDefault();
          currentActions.onTransformAgain();
          return;
        case "Mod+f":
          event.preventDefault();
          currentActions.onReapplyFilter();
          return;
        case "Mod+Shift+f":
          event.preventDefault();
          currentActions.onFadeFilter();
          return;
        case "ArrowLeft":
          event.preventDefault();
          currentActions.onNudgeLayer(event.shiftKey ? -10 : -1, 0);
          return;
        case "ArrowRight":
          event.preventDefault();
          currentActions.onNudgeLayer(event.shiftKey ? 10 : 1, 0);
          return;
        case "ArrowUp":
          event.preventDefault();
          currentActions.onNudgeLayer(0, event.shiftKey ? -10 : -1);
          return;
        case "ArrowDown":
          event.preventDefault();
          currentActions.onNudgeLayer(0, event.shiftKey ? 10 : 1);
          return;
        default:
          break;
      }

      // Brush size: [ / ]  (use event.code for layout independence)
      if (event.code === "BracketLeft" && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (event.shiftKey) {
          currentActions.onBrushHardnessChange(-0.25);
        } else {
          currentActions.onBrushSizeChange(-1);
        }
        return;
      }
      if (event.code === "BracketRight" && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (event.shiftKey) {
          currentActions.onBrushHardnessChange(0.25);
        } else {
          currentActions.onBrushSizeChange(1);
        }
        return;
      }

      const action = defaultKeymap.get(key);
      switch (action) {
        case "panMode":
          event.preventDefault();
          currentActions.onPanModeChange(true);
          break;
        case "zoomIn":
          event.preventDefault();
          currentActions.onZoomIn();
          break;
        case "zoomOut":
          event.preventDefault();
          currentActions.onZoomOut();
          break;
        case "fitToView":
          event.preventDefault();
          currentActions.onFitToView();
          break;
        case "undo":
          event.preventDefault();
          currentActions.onUndo();
          break;
        case "redo":
          event.preventDefault();
          currentActions.onRedo();
          break;
        default:
          break;
      }
    };

    const handleKeyUp = (event: KeyboardEvent) => {
      if (event.key === " ") {
        actionsRef.current.onPanModeChange(false);
      }
    };

    // Space-pan must not stick when focus leaves the window (alt-tab, tab
    // switch) — the matching keyup never arrives in those cases.
    const releasePanMode = () => {
      actionsRef.current.onPanModeChange(false);
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        releasePanMode();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    window.addEventListener("keyup", handleKeyUp);
    window.addEventListener("blur", releasePanMode);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("keyup", handleKeyUp);
      window.removeEventListener("blur", releasePanMode);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, []);
}
