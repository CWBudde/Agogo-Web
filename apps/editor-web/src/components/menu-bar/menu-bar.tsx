import { type KeyboardEvent as ReactKeyboardEvent, useEffect, useRef, useState } from "react";
import {
  FitScreenIcon,
  NewDocumentIcon,
  OpenFolderIcon,
  RedoIcon,
  SaveIcon,
  UndoIcon,
} from "@/components/editor-icons";
import { MenuPreviewPanel } from "@/components/menu-bar/menu-preview";
import { type MenuActionId, menuItems } from "@/components/menu-bar/model";
import { Button } from "@/components/ui/button";
import { type MenuActionIO, useMenuActions } from "@/hooks/use-menu-actions";
import { useDialogState } from "@/state/dialog-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

/**
 * The editor header: application menubar (with roving tabindex + arrow-key
 * navigation), brand mark, and the quick-action buttons on the right.
 */
export function MenuBar({ io }: { io: MenuActionIO }) {
  const engine = useEngine();
  const { setNewDocumentOpen } = useDialogState();
  const canUndo = useUiMeta((meta) => meta?.canUndo ?? false);
  const canRedo = useUiMeta((meta) => meta?.canRedo ?? false);
  const {
    handleMenuAction,
    handleFilter,
    isMenuActionDisabled,
    isMenuItemDisabled,
    checkedMenuActionIds,
  } = useMenuActions(io);

  const menuBarRef = useRef<HTMLDivElement | null>(null);
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [menubarFocusIndex, setMenubarFocusIndex] = useState(0);
  const menuOpenedByKeyboard = useRef(false);

  const openNewDocumentDialog = () => {
    setNewDocumentOpen(true);
  };

  const onMenuAction = (actionId: MenuActionId) => {
    if (isMenuActionDisabled(actionId)) {
      return;
    }
    setOpenMenu(null);
    handleMenuAction(actionId);
  };

  const onFilter = (filterId: string) => {
    setOpenMenu(null);
    handleFilter(filterId);
  };

  useEffect(() => {
    if (!openMenu) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuBarRef.current?.contains(event.target as Node)) {
        setOpenMenu(null);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpenMenu(null);
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [openMenu]);

  // When a menu is opened via the keyboard (ArrowDown), move focus to its first item.
  useEffect(() => {
    if (openMenu && menuOpenedByKeyboard.current) {
      menuOpenedByKeyboard.current = false;
      menuBarRef.current
        ?.querySelector<HTMLElement>('[role="menu"] [role="menuitem"]:not([disabled])')
        ?.focus();
    }
  }, [openMenu]);

  const handleMenubarKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    const target = event.target as HTMLElement;
    const nav = event.currentTarget;
    const dropdown = target.closest<HTMLElement>('[role="menu"]');

    if (!dropdown) {
      // Top-level menubar items: roving focus + ArrowDown opens the menu.
      const topItems = Array.from(
        nav.querySelectorAll<HTMLButtonElement>(':scope > div > button[role="menuitem"]'),
      );
      const index = topItems.indexOf(target as HTMLButtonElement);
      if (index === -1) {
        return;
      }
      if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
        event.preventDefault();
        event.stopPropagation();
        const next =
          event.key === "ArrowRight"
            ? (index + 1) % topItems.length
            : (index - 1 + topItems.length) % topItems.length;
        setMenubarFocusIndex(next);
        topItems[next]?.focus();
        if (openMenu) {
          setOpenMenu(menuItems[next]?.label ?? null);
        }
      } else if (event.key === "ArrowDown") {
        event.preventDefault();
        event.stopPropagation();
        const label = menuItems[index]?.label ?? null;
        if (openMenu === label) {
          // Already open (e.g. via click): the state update below would be a
          // no-op and the [openMenu] effect would never re-fire, so move
          // focus into the menu directly.
          target.parentElement
            ?.querySelector<HTMLElement>('[role="menu"] [role="menuitem"]:not([disabled])')
            ?.focus();
        } else {
          menuOpenedByKeyboard.current = true;
          setOpenMenu(label);
        }
      }
      return;
    }

    // Inside an open dropdown: ArrowUp/ArrowDown cycle, Escape closes and returns focus.
    const items = Array.from(
      dropdown.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not([disabled])'),
    );
    const itemIndex = items.indexOf(target as HTMLButtonElement);
    if (event.key === "ArrowDown") {
      event.preventDefault();
      event.stopPropagation();
      items[(itemIndex + 1) % items.length]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      event.stopPropagation();
      items[(itemIndex - 1 + items.length) % items.length]?.focus();
    } else if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      setOpenMenu(null);
      dropdown.parentElement?.querySelector<HTMLElement>('button[role="menuitem"]')?.focus();
    }
  };

  return (
    <header className="editor-titlebar flex h-[34px] items-center justify-between gap-3 border-b border-border px-2">
      <div
        ref={menuBarRef}
        className="flex min-w-0 flex-nowrap items-center gap-3 overflow-visible"
      >
        <div className="flex shrink-0 items-center gap-2 pr-3">
          <div className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] bg-cyan-400/95 text-[11px] font-black text-slate-950">
            A
          </div>
          <span className="font-serif text-[12px] font-semibold italic tracking-[0.01em] text-white">
            Agogo Studio
          </span>
        </div>

        <div
          role="menubar"
          aria-label="Application menu"
          className="flex min-w-0 flex-nowrap items-center gap-1 border-l border-white/8 pl-3"
          onKeyDown={handleMenubarKeyDown}
        >
          {menuItems.map((menu, menuIndex) => {
            const isOpen = openMenu === menu.label;
            return (
              <div key={menu.label} className="relative shrink-0">
                <button
                  type="button"
                  role="menuitem"
                  tabIndex={menuIndex === menubarFocusIndex ? 0 : -1}
                  className={[
                    "px-1.5 py-1 text-[12px] transition focus-visible:bg-white/6 focus-visible:outline-none",
                    isOpen ? "text-white" : "text-slate-400 hover:text-slate-100",
                  ].join(" ")}
                  aria-expanded={isOpen}
                  aria-haspopup="menu"
                  onClick={() =>
                    setOpenMenu((current) => (current === menu.label ? null : menu.label))
                  }
                  onFocus={() => setMenubarFocusIndex(menuIndex)}
                  onPointerEnter={() => {
                    if (openMenu) {
                      setOpenMenu(menu.label);
                    }
                  }}
                >
                  {menu.label}
                </button>

                {isOpen ? (
                  <MenuPreviewPanel
                    menu={menu}
                    isItemDisabled={isMenuItemDisabled}
                    onAction={onMenuAction}
                    onFilter={onFilter}
                    checkedActionIds={checkedMenuActionIds}
                  />
                ) : null}
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-1">
        <Button variant="ghost" size="sm" onClick={io.openProjectPicker}>
          <OpenFolderIcon className="mr-1.5 h-3.5 w-3.5" />
          Open
        </Button>
        <Button variant="ghost" size="sm" onClick={() => io.saveDocument("archive")}>
          <SaveIcon className="mr-1.5 h-3.5 w-3.5" />
          Save
        </Button>
        <Button variant="ghost" size="sm" onClick={openNewDocumentDialog}>
          <NewDocumentIcon className="mr-1.5 h-3.5 w-3.5" />
          New
        </Button>
        <Button variant="ghost" size="sm" onClick={() => engine.fitToView()}>
          <FitScreenIcon className="mr-1.5 h-3.5 w-3.5" />
          Fit
        </Button>
        <Button variant="ghost" size="sm" onClick={() => engine.undo()} disabled={!canUndo}>
          <UndoIcon className="mr-1.5 h-3.5 w-3.5" />
          Undo
        </Button>
        <Button size="sm" onClick={() => engine.redo()} disabled={!canRedo}>
          <RedoIcon className="mr-1.5 h-3.5 w-3.5" />
          Redo
        </Button>
      </div>
    </header>
  );
}
