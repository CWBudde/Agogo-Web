import {
  ClipboardIcon,
  CopyIcon,
  InfoIcon,
  LayersIcon,
  NewDocumentIcon,
  OpenFolderIcon,
  PanelsIcon,
  RedoIcon,
  SaveIcon,
  ScissorsIcon,
  SelectionIcon,
  SlidersIcon,
  UndoIcon,
  ZoomToolIcon,
} from "@/components/editor-icons";
import type { MenuActionId, MenuPreviewItem, MenuPreviewMenu } from "@/components/menu-bar/model";

export function MenuPreviewPanel({
  menu,
  isItemDisabled,
  onAction,
  checkedActionIds,
}: {
  menu: MenuPreviewMenu;
  isItemDisabled(item: MenuPreviewItem): boolean;
  onAction(actionId: MenuActionId): void;
  checkedActionIds?: Set<MenuActionId>;
}) {
  const items = menu.sections.flatMap((section) => section.items);

  return (
    <div
      role="menu"
      aria-label={menu.label}
      className={[
        "editor-popup absolute top-[calc(100%+4px)] z-40 w-[18.5rem] max-w-[calc(100vw-1rem)] overflow-hidden",
        menu.align === "right" ? "right-0" : "left-0",
      ].join(" ")}
    >
      <div className="border-b border-white/8 px-2.5 py-2 text-[11px] text-slate-400">
        {menu.caption}
      </div>

      <div className="py-1">
        {items.map((item) => {
          const disabled = isItemDisabled(item);
          const checked = !!(item.actionId && checkedActionIds?.has(item.actionId));
          return (
            <MenuPreviewAction
              key={`${menu.label}-${item.label}`}
              item={item}
              disabled={disabled}
              checked={checked}
              onClick={item.actionId ? () => onAction(item.actionId as MenuActionId) : undefined}
            />
          );
        })}
      </div>
    </div>
  );
}

function MenuPreviewAction({
  item,
  disabled,
  checked,
  onClick,
}: {
  item: MenuPreviewItem;
  disabled: boolean;
  checked: boolean;
  onClick?: () => void;
}) {
  const ItemIcon = iconForMenuItem(item.label);

  return (
    <button
      type="button"
      role="menuitem"
      className={[
        "flex w-full items-center justify-between px-2.5 py-1.5 text-left text-[12px] transition focus-visible:bg-white/6 focus-visible:outline-none",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:bg-white/6 focus:bg-white/6 focus:outline-none",
      ].join(" ")}
      disabled={disabled}
      aria-disabled={disabled}
      onClick={onClick}
    >
      <span className="flex min-w-0 items-center gap-2">
        <ItemIcon
          className={[
            "h-3.5 w-3.5 shrink-0",
            disabled || item.tone === "muted"
              ? "text-slate-600"
              : item.tone === "accent"
                ? "text-cyan-300"
                : "text-slate-400",
          ].join(" ")}
        />
        <span
          className={
            disabled || item.tone === "muted"
              ? "truncate text-slate-500"
              : "truncate text-slate-100"
          }
        >
          {item.label}
        </span>
      </span>
      {checked ? (
        <span className="ml-4 shrink-0 text-[11px] text-cyan-400">✓</span>
      ) : item.shortcut ? (
        <span className="ml-4 shrink-0 text-[11px] text-slate-500">{item.shortcut}</span>
      ) : null}
    </button>
  );
}

function iconForMenuItem(label: string) {
  const lower = label.toLowerCase();

  if (lower.includes("new")) {
    return NewDocumentIcon;
  }
  if (lower.includes("open")) {
    return OpenFolderIcon;
  }
  if (lower.includes("save") || lower.includes("export") || lower.includes("assets")) {
    return SaveIcon;
  }
  if (lower.includes("undo")) {
    return UndoIcon;
  }
  if (lower.includes("redo")) {
    return RedoIcon;
  }
  if (lower.includes("cut")) {
    return ScissorsIcon;
  }
  if (lower.includes("copy")) {
    return CopyIcon;
  }
  if (lower.includes("paste")) {
    return ClipboardIcon;
  }
  if (lower.includes("layer") || lower.includes("rasterize") || lower.includes("merge")) {
    return LayersIcon;
  }
  if (lower.includes("select") || lower.includes("feather") || lower.includes("inverse")) {
    return SelectionIcon;
  }
  if (
    lower.includes("levels") ||
    lower.includes("curves") ||
    lower.includes("hue") ||
    lower.includes("invert") ||
    lower.includes("channel mixer") ||
    lower.includes("threshold") ||
    lower.includes("posterize") ||
    lower.includes("selective color") ||
    lower.includes("photo filter") ||
    lower.includes("gradient") ||
    lower.includes("blur") ||
    lower.includes("noise") ||
    lower.includes("stylize") ||
    lower.includes("filter")
  ) {
    return SlidersIcon;
  }
  if (
    lower.includes("zoom") ||
    lower.includes("rulers") ||
    lower.includes("grid") ||
    lower.includes("guides")
  ) {
    return ZoomToolIcon;
  }
  if (
    lower.includes("workspace") ||
    lower.includes("navigator") ||
    lower.includes("history") ||
    lower.includes("panels")
  ) {
    return PanelsIcon;
  }
  return InfoIcon;
}
